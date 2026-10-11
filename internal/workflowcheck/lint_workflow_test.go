package workflowcheck

import (
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

type workflow struct {
	Permissions map[string]any `yaml:"permissions"`
	Jobs        map[string]job `yaml:"jobs"`
}

type job struct {
	Steps []step `yaml:"steps"`
}

type step struct {
	If   string `yaml:"if"`
	Run  string `yaml:"run"`
	Uses string `yaml:"uses"`
}

func TestLintWorkflowHardening(t *testing.T) {
	contents, err := os.ReadFile("../../.github/workflows/lint.yml")
	if err != nil {
		t.Fatal(err)
	}

	var lint workflow
	if err := yaml.Unmarshal(contents, &lint); err != nil {
		t.Fatal(err)
	}

	if lint.Permissions == nil || len(lint.Permissions) != 0 {
		t.Errorf("top-level permissions must be an explicit empty mapping, got %#v", lint.Permissions)
	}

	hasGoTest := false
	hasCodecovUpload := false
	for name, job := range lint.Jobs {
		if len(job.Steps) == 0 || !strings.HasPrefix(job.Steps[0].Uses, "step-security/harden-runner@") {
			t.Errorf("job %q must use harden-runner as its first step", name)
		}

		for _, step := range job.Steps {
			if strings.Contains(step.If, "hashFiles") && strings.Contains(step.If, ".go") {
				t.Errorf("job %q contains a Go-file skip condition: %s", name, step.If)
			}
			if strings.Contains(step.Run, "go test") && strings.Contains(step.Run, "./...") {
				hasGoTest = true
				if !strings.Contains(step.Run, "-race") {
					t.Errorf("job %q go test step must enable the race detector per docs/testing-guide.md quality gates: %s", name, step.Run)
				}
				if !strings.Contains(step.Run, "-coverprofile=coverage.txt") {
					t.Errorf("job %q go test step must emit coverage.txt for the Codecov upload: %s", name, step.Run)
				}
			}
			if strings.HasPrefix(step.Uses, "step-security/codecov-action@") {
				hasCodecovUpload = true
			}
		}
	}

	if !hasGoTest {
		t.Error("lint workflow must run go test ./...")
	}
	if !hasCodecovUpload {
		t.Error("lint workflow must upload the coverage profile with step-security/codecov-action")
	}
}
