package dependabot_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/dependabot"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestPlanRejectsBeforeGeneration(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		change func(*dependabot.Event)
		reason string
	}{
		{"human", func(e *dependabot.Event) { e.Author = "human" }, "not-dependabot"},
		{"fork", func(e *dependabot.Event) { e.HeadRepository = "other/repo" }, "fork-pull-request"},
		{"closed", func(e *dependabot.Event) { e.State = "closed" }, "pull-request-not-open"},
		{"base", func(e *dependabot.Event) { e.BaseRef = "feature" }, "wrong-base"},
		{"repository", func(e *dependabot.Event) { e.Repository = "other/repo" }, "wrong-repository"},
		{"companion", func(e *dependabot.Event) { e.ChangedPaths = append(e.ChangedPaths, "CHANGELOG.md") }, "changed-paths-mismatch"},
		{"stale", func(e *dependabot.Event) { e.HeadSHA = "cccccccccccccccccccccccccccccccccccccccc" }, "stale-head"},
		{"trusted base", func(e *dependabot.Event) { e.BaseSHA = strings.Repeat("c", 40) }, "wrong-trusted-base"},
		{"trusted sha", func(e *dependabot.Event) { e.TrustedSHA = strings.Repeat("c", 40) }, "wrong-trusted-base"},
		{"number", func(e *dependabot.Event) { e.Number++ }, "stale-head"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			inputs := planInputs(t)
			raw, err := os.ReadFile(inputs.EventPath)
			if err != nil {
				t.Fatal(err)
			}
			event, err := dependabot.ParseEvent(raw)
			if err != nil {
				t.Fatal(err)
			}
			test.change(&event)
			raw, err = json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, inputs.EventPath, raw)
			calls := 0
			planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) { calls++; return nil, nil }}
			plan, err := planner.DryRun(t.Context(), inputs)
			if err != nil || plan.Status != "skip" || plan.Reason != test.reason || calls != 0 {
				t.Fatalf("plan=%+v calls=%d err=%v, want skip/%s without generation", plan, calls, err, test.reason)
			}
			requireEmptyPlanOutput(t, inputs.Output, plan)
		})
	}
}

func TestPlanDeterministicDespiteFailedLint(t *testing.T) {
	t.Parallel()
	inputs := planInputs(t)
	planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) {
		return []dependabot.Change{{Path: "internal/provenance/builder_dependencies.go", Before: []byte("old\n"), After: []byte("new\n")}}, nil
	}}
	first, err := planner.DryRun(t.Context(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := planner.DryRun(t.Context(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	a, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) || first.Status != "planned" || len(first.Files) != 1 {
		t.Fatalf("nondeterministic or incomplete plan: %s / %s", a, b)
	}
}

func TestMalformedEventLeavesNoOutput(t *testing.T) {
	t.Parallel()
	inputs := planInputs(t)
	writeTestFile(t, inputs.EventPath, []byte(`{"number":1,"number":2}`))
	_, err := (dependabot.Planner{}).DryRun(t.Context(), inputs)
	if err == nil {
		t.Fatal("accepted duplicate")
	}
	if _, err := os.Stat(inputs.Output); !os.IsNotExist(err) {
		t.Fatalf("partial output: %v", err)
	}
}

func planInputs(t *testing.T) dependabot.Inputs {
	t.Helper()
	root := t.TempDir()
	compiled := provenance.SigningAdapterBuilderDependency()
	_, current, _ := strings.Cut(compiled.URI, "@")
	parts := strings.Split(current, ".")
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	candidate := parts[0] + "." + strconv.Itoa(minor+1) + ".0"
	digest, err := hex.DecodeString(compiled.Digest["dirHash1"])
	if err != nil {
		t.Fatal(err)
	}
	currentSum := "h1:" + base64.StdEncoding.EncodeToString(digest)
	for _, name := range []string{"event.json", "release.json", "base/go.mod", "base/go.sum", "head/go.mod", "head/go.sum"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(data), "v1.5.0", candidate)
		if strings.HasPrefix(name, "base/") {
			text = strings.ReplaceAll(text, "v1.4.0", current)
			text = strings.ReplaceAll(text, "h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw=", currentSum)
		}
		writeTestFile(t, filepath.Join(root, "snapshot", name), []byte(text))
	}
	base := fixtureModules(t, "base")
	base.Mod = []byte(strings.ReplaceAll(string(base.Mod), "v1.4.0", current))
	base.Sum = []byte(strings.ReplaceAll(strings.ReplaceAll(string(base.Sum), "v1.4.0", current), "h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw=", currentSum))
	trusted, sha := dependabot.NewCommittedTestRoot(t, map[string][]byte{"go.mod": base.Mod, "go.sum": base.Sum})
	event, err := os.ReadFile("testdata/event.json")
	if err != nil {
		t.Fatal(err)
	}
	event = bytes.ReplaceAll(event, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []byte(sha))
	writeTestFile(t, filepath.Join(root, "event.json"), event)
	writeTestFile(t, filepath.Join(root, "snapshot", "event.json"), event)
	return dependabot.Inputs{EventPath: filepath.Join(root, "event.json"), Snapshot: filepath.Join(root, "snapshot"), TrustedRoot: trusted, TrustedSHA: sha, Output: filepath.Join(root, "output")}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPlanGeneratorFailuresLeaveNoOutput(t *testing.T) {
	t.Parallel()
	generationErr := errors.New("generation failed")
	for _, test := range []struct {
		name    string
		changes []dependabot.Change
		err     error
		message string
	}{
		{"generation error", nil, generationErr, "generation failed"},
		{"empty generation", nil, nil, "companion generator produced no changes"},
		{"outside allowlist", []dependabot.Change{{Path: "go.mod"}}, nil, "generator output outside closed allowlist"},
		{"duplicate output", []dependabot.Change{{Path: "internal/provenance/builder_dependencies.go"}, {Path: "internal/provenance/builder_dependencies.go"}}, nil, "generator output outside closed allowlist"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			inputs := planInputs(t)
			calls := 0
			planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) {
				calls++
				return test.changes, test.err
			}}
			_, err := planner.DryRun(t.Context(), inputs)
			if err == nil || err.Error() != test.message || calls != 1 {
				t.Fatalf("error=%v calls=%d, want %q and one generation", err, calls, test.message)
			}
			if test.err != nil && !errors.Is(err, test.err) {
				t.Fatalf("lost generator error: %v", err)
			}
			if _, err := os.Stat(inputs.Output); !os.IsNotExist(err) {
				t.Fatalf("partial output: %v", err)
			}
		})
	}
}
