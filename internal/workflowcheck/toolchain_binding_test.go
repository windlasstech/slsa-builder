package workflowcheck

import (
	"strings"
	"testing"
)

const provisionOutput = "${{ steps.npm-provision.outputs.npm-executable }}"

func TestPublishExecutableBinding(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"step", "job"} {
		for _, binding := range []string{provisionOutput, "${{ vars.AMBIENT_NPM }}", "/usr/bin/npm", "${{ steps.other.outputs.npm-executable }}", ""} {
			t.Run(scope+"/"+binding, func(t *testing.T) {
				t.Parallel()
				// Given diagnostics using the same environment variable as production.
				contents := replaceOnce(t, validNPMOnlyWorkflow,
					"      - if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'\n        run: '\""+provisionOutput+"\" --version'",
					"      - if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'\n        run: '\"$NPM_EXECUTABLE\" --version'")
				if scope == "step" {
					contents = replaceOnce(t, contents, "      - if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'\n        run: '\"$NPM_EXECUTABLE\" --version'",
						"      - if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'\n        env:\n          NPM_EXECUTABLE: \""+binding+"\"\n        run: '\"$NPM_EXECUTABLE\" --version'")
				} else {
					contents = replaceOnce(t, contents, "  publish:\n", "  publish:\n    env:\n      NPM_EXECUTABLE: \""+binding+"\"\n")
				}
				// When checked through the public YAML entry point.
				_, err := CheckPublishJob(writeWorkflow(t, contents))
				// Then only the exact provisioning output is accepted.
				if binding == provisionOutput {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "NPM_EXECUTABLE") {
					t.Fatalf("binding error = %v, want NPM_EXECUTABLE rejection", err)
				}
			})
		}
	}
}

func TestPublishExecutableBindingStepPrecedence(t *testing.T) {
	t.Parallel()
	for _, binding := range []string{provisionOutput, "${{ vars.AMBIENT_NPM }}", ""} {
		t.Run(binding, func(t *testing.T) {
			t.Parallel()
			// Given a trusted job binding overridden by the publish step.
			contents := replaceOnce(t, validNPMOnlyWorkflow, "  publish:\n",
				"  publish:\n    env:\n      NPM_EXECUTABLE: "+provisionOutput+"\n")
			contents = replaceOnce(t, contents, "      - id: publish\n",
				"      - id: publish\n        env:\n          NPM_EXECUTABLE: \""+binding+"\"\n")
			contents = replaceOnce(t, contents, `--npm-executable "`+provisionOutput+`"`, `--npm-executable "${{ env.NPM_EXECUTABLE }}"`)
			// When the publish command uses environment indirection.
			_, err := CheckPublishJob(writeWorkflow(t, contents))
			// Then the step binding wins, including an explicitly empty override.
			if (err != nil) != (binding != provisionOutput) {
				t.Fatalf("step override error = %v, binding = %q", err, binding)
			}
		})
	}
}
