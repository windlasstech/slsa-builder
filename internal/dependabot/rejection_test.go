package dependabot_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/dependabot"
)

func TestManualActionNeverGenerates(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, file, old, replacement, reason string }{
		{"out of group", "head/go.mod", "example.org/other v1.0.0", "example.org/other v1.1.0", "unsupported-transitive-change"},
		{"replacement", "head/go.mod", "go 1.27.3", "go 1.27.3\nreplace example.org/a => example.org/b v1.0.0", "unsupported-module-directive"},
		{"go directive", "head/go.mod", "go 1.27.3", "go 1.28.0", "unsupported-module-directive"},
		{"missing checksum", "head/go.sum", "github.com/sigstore/sigstore-go v1.5.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "", "invalid-module-checksums"},
		{"malformed checksum", "head/go.sum", "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "h1:not-base64", "invalid-module-checksums"},
		{"conflicting checksum", "head/go.sum", "github.com/sigstore/sigstore-go v1.5.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "github.com/sigstore/sigstore-go v1.5.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\ngithub.com/sigstore/sigstore-go v1.5.0 h1:DGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw=", "invalid-module-checksums"},
		{"license", "release.json", `"license":"Apache-2.0"`, `"license":"GPL-3.0"`, "release-metadata-unconfirmed"},
		{"toolchain", "release.json", `"go_version":"1.27.0"`, `"go_version":"1.28.0"`, "release-metadata-unconfirmed"},
		{"date", "release.json", "2026-11-10", "not-a-date", "release-metadata-unconfirmed"},
		{"tag", "release.json", `"version":"v1.5.0"`, `"version":"v1.6.0"`, "release-metadata-unconfirmed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			inputs := planInputs(t)
			path := filepath.Join(inputs.Snapshot, test.file)
			releaseBytes, err := os.ReadFile(filepath.Join(inputs.Snapshot, "release.json"))
			if err != nil {
				t.Fatal(err)
			}
			var release dependabot.Release
			if err := json.Unmarshal(releaseBytes, &release); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			old := strings.ReplaceAll(test.old, "v1.5.0", release.Version)
			replacement := strings.ReplaceAll(test.replacement, "v1.5.0", release.Version)
			writeTestFile(t, path, []byte(strings.ReplaceAll(string(raw), old, replacement)))
			calls := 0
			planner := dependabot.Planner{Generate: func(context.Context, dependabot.Generation) ([]dependabot.Change, error) { calls++; return nil, nil }}
			plan, err := planner.DryRun(t.Context(), inputs)
			if err != nil || plan.Status != "manual-action-required" || plan.Reason != test.reason || calls != 0 || len(plan.Files) != 0 {
				t.Fatalf("plan=%+v calls=%d err=%v", plan, calls, err)
			}
		})
	}
}

func TestSumOnlyWithUnchangedSignerSkips(t *testing.T) {
	t.Parallel()
	inputs := planInputs(t)
	baseMod, err := os.ReadFile(filepath.Join(inputs.Snapshot, "base/go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	baseSum, err := os.ReadFile(filepath.Join(inputs.Snapshot, "base/go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	base := dependabot.Modules{Mod: baseMod, Sum: baseSum}
	writeTestFile(t, filepath.Join(inputs.Snapshot, "head/go.mod"), base.Mod)
	writeTestFile(t, filepath.Join(inputs.Snapshot, "head/go.sum"), append(base.Sum, '\n'))
	raw, err := os.ReadFile(inputs.EventPath)
	if err != nil {
		t.Fatal(err)
	}
	event, err := dependabot.ParseEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	event.ChangedPaths = []string{"go.sum"}
	raw, err = json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, inputs.EventPath, raw)
	writeTestFile(t, filepath.Join(inputs.Snapshot, "event.json"), raw)
	plan, err := (dependabot.Planner{}).DryRun(t.Context(), inputs)
	if err != nil || plan.Status != "skip" || plan.Reason != "signer-unchanged" {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}
