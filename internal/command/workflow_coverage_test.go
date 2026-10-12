package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/workflowcheck"
)

func TestWorkflowCheckAcceptsProductionWorkflow(t *testing.T) {
	t.Parallel()
	// Given the real production reusable workflow, the documented workflow-check contract gate target.
	production := filepath.Join("..", "..", ".github", "workflows", "js-ts-npm-package-slsa3.yml")
	var output bytes.Buffer
	// When the full npm-only profile contract runs against it.
	err := NewWorkflowCheckCommand().Execute(t.Context(), []string{"--workflow", production, "--profile", "npm-only"}, &output)
	// Then the contract reports no findings.
	if err != nil {
		t.Fatalf("production workflow conformance = %v", err)
	}
	var result workflowcheck.NPMOnlyProfileResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("output = %q: %v", output.Bytes(), err)
	}
	if result.Result != "pass" || result.Profile != "npm-only" {
		t.Fatalf("result = %+v, want a passing npm-only profile", result)
	}
}

func TestWorkflowCheckRejectsUnsupportedTargets(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"--job", "--profile"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			err := NewWorkflowCheckCommand().Execute(t.Context(), []string{"--workflow", "unused.yml", target, "unsupported"}, &output)
			if err == nil || !strings.Contains(err.Error(), "unsupported workflow") || output.Len() != 0 {
				t.Fatalf("unsupported target = %v, output %q", err, output.String())
			}
		})
	}
}

func TestWorkflowCheckPropagatesOutputAndInputFailures(t *testing.T) {
	t.Parallel()
	for _, job := range []string{"build", "provenance-sign", "publish"} {
		t.Run(job, func(t *testing.T) {
			t.Parallel()
			cause := errors.New("workflow output failed")
			err := NewWorkflowCheckCommand().Execute(t.Context(), []string{"--workflow", writeWorkflowCheckFixture(t), "--job", job}, &stagedWriter{err: cause})
			if !errors.Is(err, cause) {
				t.Fatalf("writer error = %v, want original cause", err)
			}
		})
	}
	t.Run("missing workflow", func(t *testing.T) {
		t.Parallel()
		var output bytes.Buffer
		err := NewWorkflowCheckCommand().Execute(t.Context(), []string{"--workflow", filepath.Join(t.TempDir(), "missing"), "--job", "build"}, &output)
		if err == nil || output.Len() != 0 {
			t.Fatalf("missing workflow = %v, output %q", err, output.String())
		}
	})
}

func TestFixtureCheckWriterFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		args []string
	}{
		{"missing index", []string{"--index", "missing-index.json"}},
		{"invalid filter", []string{"--index", "../../testdata/fixtures/index.json", "--surface", "invalid"}},
		{"valid index", []string{"--index", "../../testdata/fixtures/index.json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cause := errors.New("fixture report failed")
			if err := NewFixtureCheckCommand().Execute(t.Context(), test.args, &stagedWriter{err: cause}); !errors.Is(err, cause) {
				t.Fatalf("error = %v, want writer cause", err)
			}
		})
	}
}
