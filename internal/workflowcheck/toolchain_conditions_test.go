package workflowcheck

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestToolchainConditionsRejectDrift(t *testing.T) {
	t.Parallel()
	for _, jobName := range []string{"build", "provenance-sign", "publish"} {
		for _, condition := range []string{"success()", "false", "${{ false }}"} {
			t.Run(jobName+"/"+condition, func(t *testing.T) {
				t.Parallel()
				// Given an otherwise valid job with only its assertion gated.
				document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
				if err != nil {
					t.Fatal(err)
				}
				for index, step := range document.Jobs[jobName].Steps {
					if strings.Contains(step.Run, "npm-profile-toolchain-assert") {
						document.Jobs[jobName].Steps[index].If = condition
					}
				}
				// When the public workflow contract is checked.
				_, err = CheckNPMOnlyProfile(writeWorkflowDocument(t, document))
				// Then a skipped assertion cannot protect an executing npm step.
				if err == nil || !strings.Contains(err.Error(), "toolchain step conditions") {
					t.Fatalf("condition drift error = %v, want toolchain step conditions rejection", err)
				}
			})
		}
	}
}

func TestToolchainConditionsUniformGate(t *testing.T) {
	t.Parallel()
	for _, gate := range []string{"", "needs.build.result == 'success'", "false", "${{ false }}"} {
		t.Run(gate, func(t *testing.T) {
			t.Parallel()
			// Given a uniform gate across every step, including the toolchain segment.
			document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
			if err != nil {
				t.Fatal(err)
			}
			for _, job := range document.Jobs {
				for index := range job.Steps {
					if job.Steps[index].If != "always()" {
						job.Steps[index].If = gate
					}
				}
			}
			// When the public workflow contract is checked.
			_, err = CheckNPMOnlyProfile(writeWorkflowDocument(t, document))
			// Then uniform live gates pass but literal false never does.
			wantError := strings.Contains(gate, "false")
			if (err != nil) != wantError {
				t.Fatalf("uniform gate error = %v, want error %v", err, wantError)
			}
		})
	}
}

func writeWorkflowDocument(t *testing.T, document workflowDocument) string {
	t.Helper()
	encoded, err := yaml.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return writeWorkflow(t, string(encoded))
}
