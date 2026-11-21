package dependabot_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/dependabot"
)

func TestInputRequiresModuleDirectories(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"base", "head"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inputs := planInputs(t)
			if err := os.RemoveAll(filepath.Join(inputs.Snapshot, name)); err != nil {
				t.Fatal(err)
			}
			calls := 0
			planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) { calls++; return nil, nil }}
			_, err := planner.DryRun(t.Context(), inputs)
			if !errors.Is(err, os.ErrNotExist) || calls != 0 {
				t.Fatalf("error=%v calls=%d, want missing directory and zero calls", err, calls)
			}
			if _, err := os.Stat(inputs.Output); !os.IsNotExist(err) {
				t.Fatalf("partial output: %v", err)
			}
		})
	}
}

func TestInputRejectsCommittedSignerMismatch(t *testing.T) {
	t.Parallel()
	inputs := planInputs(t)
	mod, err := os.ReadFile(filepath.Join(inputs.Snapshot, "base/go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(inputs.Snapshot, "base/go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(sum, []byte("\n"))
	fields := bytes.Fields(lines[0])
	lines[0] = append(bytes.Join(fields[:2], []byte(" ")), []byte(" h1:AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")...)
	sum = bytes.Join(lines, []byte("\n"))
	writeTestFile(t, filepath.Join(inputs.Snapshot, "base/go.sum"), sum)
	root, sha := dependabot.NewCommittedTestRoot(t, map[string][]byte{"go.mod": mod, "go.sum": sum})
	for _, path := range []string{inputs.EventPath, filepath.Join(inputs.Snapshot, "event.json")} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, path, bytes.ReplaceAll(raw, []byte(inputs.TrustedSHA), []byte(sha)))
	}
	inputs.TrustedRoot, inputs.TrustedSHA = root, sha
	calls := 0
	planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) { calls++; return nil, nil }}
	plan, err := planner.DryRun(t.Context(), inputs)
	if err != nil || calls != 0 || plan.Status != "manual-action-required" || plan.Reason != "compiled-signer-mismatch" {
		t.Fatalf("plan=%+v calls=%d err=%v, want compiled-signer-mismatch", plan, calls, err)
	}
	requireEmptyPlanOutput(t, inputs.Output, plan)
}
