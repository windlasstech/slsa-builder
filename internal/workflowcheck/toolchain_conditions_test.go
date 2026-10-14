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
			// Then build and sign provisioning cannot be conditional, even uniformly.
			if err == nil {
				t.Fatal("uniform gate was accepted, want rejection")
			}
		})
	}
}

func TestPublishToolchainRequiresUpstreamSuccessGate(t *testing.T) {
	t.Parallel()
	const upstreamGate = "needs.build.result == 'success' && needs.provenance-sign.result == 'success'"
	for _, test := range []struct {
		name      string
		condition string
		wantError bool
	}{
		{name: "uniform success", condition: "success()", wantError: true},
		{name: "uniform omission", condition: "", wantError: true},
		{name: "partial gate", condition: "needs.build.result == 'success'", wantError: true},
		{name: "expression wrapper and outer whitespace", condition: "  ${{  " + upstreamGate + "  }}  "},
		{name: "production gate", condition: upstreamGate},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given an otherwise valid workflow with the same condition on all publish toolchain steps.
			document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
			if err != nil {
				t.Fatal(err)
			}
			for index, step := range document.Jobs["publish"].Steps {
				if step.If == upstreamGate && (runtimeReferencePattern.MatchString(step.Run) ||
					strings.Contains(step.Run, "npm-profile-provision-node") ||
					strings.Contains(step.Run, "GITHUB_PATH") ||
					strings.Contains(step.Run, "npm-profile-toolchain-assert") ||
					strings.Contains(step.Run, "npm-profile-provision-npm") ||
					strings.Contains(step.Run, "npm-profile-build") || strings.Contains(step.Run, "npm-profile-sign") ||
					strings.Contains(step.Run, "npm-profile-publish") || strings.Contains(step.Run, "npm-executable") ||
					strings.Contains(step.Run, "NPM_EXECUTABLE")) {
					document.Jobs["publish"].Steps[index].If = test.condition
				}
			}
			// When the public workflow contract is checked.
			_, err = CheckNPMOnlyProfile(writeWorkflowDocument(t, document))
			// Then only the normalized upstream-success gate is accepted.
			if (err != nil) != test.wantError {
				t.Fatalf("publish toolchain condition error = %v, want error %v", err, test.wantError)
			}
		})
	}
}

func TestProductionWorkflowAcceptsPublishToolchainGate(t *testing.T) {
	t.Parallel()
	// Given the production workflow fixture with its checked-in upstream-success gates.
	// When the public workflow contract is checked.
	_, err := CheckNPMOnlyProfile("../../.github/workflows/js-ts-npm-package-slsa3.yml")
	// Then the production gate remains accepted.
	if err != nil {
		t.Fatalf("production workflow rejected: %v", err)
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
