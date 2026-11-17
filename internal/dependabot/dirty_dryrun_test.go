package dependabot_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/dependabot"
)

func TestDryRunIgnoresDirtyTrustedModules(t *testing.T) {
	t.Parallel()
	inputs := planInputs(t)
	for _, path := range []string{"go.mod", "go.sum"} {
		if err := os.WriteFile(filepath.Join(inputs.TrustedRoot, path), []byte("dirty working-tree bytes\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) {
		return []dependabot.Change{{Path: "internal/provenance/builder_dependencies.go", Before: []byte("old\n"), After: []byte("new\n")}}, nil
	}}
	plan, err := planner.DryRun(t.Context(), inputs)
	if err != nil || plan.Status != "planned" {
		t.Fatalf("status=%s reason=%s error=%v", plan.Status, plan.Reason, err)
	}
}
