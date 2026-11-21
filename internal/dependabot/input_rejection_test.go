package dependabot_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/dependabot"
)

func TestInputReadFailuresNeverGenerate(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"event", "event.json", "base/go.mod", "base/go.sum", "head/go.mod", "head/go.sum", "release.json"} {
		for _, mode := range []string{"missing", "directory", "symlink", "oversized"} {
			t.Run(file+"/"+mode, func(t *testing.T) {
				t.Parallel()
				// Given one inaccessible or unbounded input in a valid snapshot.
				inputs := planInputs(t)
				path := filepath.Join(inputs.Snapshot, file)
				if file == "event" {
					path = inputs.EventPath
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				wantErr := dependabot.ErrInput
				switch mode {
				case "missing":
					wantErr = os.ErrNotExist
				case "directory":
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					if err := os.Symlink(inputs.TrustedRoot, path); err != nil {
						t.Fatal(err)
					}
				case "oversized":
					writeTestFile(t, path, nil)
					if err := os.Truncate(path, (16<<20)+1); err != nil {
						t.Fatal(err)
					}
				}
				calls := 0
				planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) { calls++; return nil, nil }}
				// When planning, then no generator or partial generated output is allowed.
				plan, err := planner.DryRun(t.Context(), inputs)
				if calls != 0 {
					t.Fatalf("generator called %d times after %s failure", calls, file)
				}
				if file == "release.json" {
					if err != nil || plan.Status != "manual-action-required" || plan.Reason != "release-metadata-required" {
						t.Fatalf("plan=%+v err=%v, want release-metadata-required", plan, err)
					}
					requireEmptyPlanOutput(t, inputs.Output, plan)
				} else {
					if !errors.Is(err, wantErr) {
						t.Fatalf("error=%v, want %v", err, wantErr)
					}
					if _, err := os.Stat(inputs.Output); !os.IsNotExist(err) {
						t.Fatalf("partial output after read failure: %v", err)
					}
				}
			})
		}
	}
}

func TestMalformedInputsNeverGenerate(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ file, reason string }{
		{"event", ""}, {"event.json", ""},
		{"head/go.mod", "invalid-module-manifest"},
		{"head/go.sum", "invalid-module-checksums"},
		{"release.json", "release-metadata-invalid"},
	} {
		t.Run(test.file, func(t *testing.T) {
			t.Parallel()
			inputs := planInputs(t)
			path := filepath.Join(inputs.Snapshot, test.file)
			if test.file == "event" {
				path = inputs.EventPath
			}
			writeTestFile(t, path, []byte("malformed\n"))
			calls := 0
			planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) { calls++; return nil, nil }}
			plan, err := planner.DryRun(t.Context(), inputs)
			if calls != 0 {
				t.Fatalf("generator called %d times", calls)
			}
			if test.reason == "" {
				if !errors.Is(err, dependabot.ErrInput) {
					t.Fatalf("error=%v, want ErrInput", err)
				}
				if _, err := os.Stat(inputs.Output); !os.IsNotExist(err) {
					t.Fatalf("partial output: %v", err)
				}
			} else {
				if err != nil || plan.Status != "manual-action-required" || plan.Reason != test.reason {
					t.Fatalf("plan=%+v err=%v, want %s", plan, err, test.reason)
				}
				requireEmptyPlanOutput(t, inputs.Output, plan)
			}
		})
	}
}

func TestInputBindingsRejectBeforeGeneration(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, reason, status string }{
		{"trusted module", "trusted-module-mismatch", "manual-action-required"},
		{"actual changed paths", "changed-paths-mismatch", "skip"},
		{"extra base file", "mixed-files-or-companion-edits", "skip"},
		{"extra head file", "mixed-files-or-companion-edits", "skip"},
		{"duplicate paths", "mixed-files-or-companion-edits", "skip"},
		{"companion paths", "mixed-files-or-companion-edits", "skip"},
		{"empty paths", "mixed-files-or-companion-edits", "skip"},
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
			switch test.name {
			case "trusted module":
				writeTestFile(t, filepath.Join(inputs.Snapshot, "base/go.sum"), []byte("untrusted"))
			case "actual changed paths":
				event.ChangedPaths = []string{"go.mod"}
			case "extra base file":
				writeTestFile(t, filepath.Join(inputs.Snapshot, "base/extra"), nil)
			case "extra head file":
				writeTestFile(t, filepath.Join(inputs.Snapshot, "head/extra"), nil)
			case "duplicate paths":
				event.ChangedPaths = []string{"go.mod", "go.mod"}
			case "companion paths":
				event.ChangedPaths = []string{"CHANGELOG.md"}
			case "empty paths":
				event.ChangedPaths = []string{}
			}
			raw, err = json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, inputs.EventPath, raw)
			writeTestFile(t, filepath.Join(inputs.Snapshot, "event.json"), raw)
			calls := 0
			planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) { calls++; return nil, nil }}
			plan, err := planner.DryRun(t.Context(), inputs)
			if err != nil || calls != 0 || plan.Reason != test.reason || plan.Status != test.status {
				t.Fatalf("plan=%+v calls=%d err=%v, want %s/%s", plan, calls, err, test.status, test.reason)
			}
			requireEmptyPlanOutput(t, inputs.Output, plan)
		})
	}
}

func TestInvalidTrustedSHAStopsBeforeInputReads(t *testing.T) {
	t.Parallel()
	inputs := planInputs(t)
	inputs.TrustedSHA = strings.Repeat("g", 40)
	calls := 0
	planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) { calls++; return nil, nil }}
	_, err := planner.DryRun(t.Context(), inputs)
	if !errors.Is(err, dependabot.ErrInput) || calls != 0 {
		t.Fatalf("error=%v calls=%d, want ErrInput and zero calls", err, calls)
	}
	if _, err := os.Stat(inputs.Output); !os.IsNotExist(err) {
		t.Fatalf("partial output: %v", err)
	}
}

func TestChecksumMutationNeverGenerates(t *testing.T) {
	t.Parallel()
	for _, module := range []string{"github.com/sigstore/sigstore-go", "github.com/in-toto/attestation", "example.org/other"} {
		t.Run(module, func(t *testing.T) {
			t.Parallel()
			inputs := planInputs(t)
			base, err := os.ReadFile(filepath.Join(inputs.Snapshot, "base/go.sum"))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(inputs.Snapshot, "head/go.sum")
			head, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, line := range strings.Split(string(base), "\n") {
				if !strings.HasPrefix(line, module+" ") {
					continue
				}
				found = true
				fields := strings.Fields(line)
				mutated := fields[0] + " " + fields[1] + " h1:AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
				if module == "example.org/other" {
					head = []byte(strings.ReplaceAll(string(head), line, mutated))
				} else {
					head = append(head, []byte(mutated+"\n")...)
				}
			}
			if !found {
				t.Fatalf("base lacks %s checksum", module)
			}
			writeTestFile(t, path, head)
			calls := 0
			planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) { calls++; return nil, nil }}
			plan, err := planner.DryRun(t.Context(), inputs)
			if err != nil || calls != 0 || plan.Status != "manual-action-required" || plan.Reason != "unsupported-transitive-change" {
				t.Fatalf("plan=%+v calls=%d err=%v, want unsupported-transitive-change without generation", plan, calls, err)
			}
			requireEmptyPlanOutput(t, inputs.Output, plan)
		})
	}
}
