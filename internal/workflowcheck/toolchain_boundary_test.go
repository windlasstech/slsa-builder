package workflowcheck

import (
	"strings"
	"testing"
)

func TestPublishToolchainStepBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func([]workflowStep)
		want   string
	}{
		{
			name: "provisioning references its own output",
			mutate: func(steps []workflowStep) {
				for index, step := range steps {
					if step.ID == "npm-provision" {
						steps[index].Run += ` && echo "` + provisionOutput + `"`
					}
				}
			},
		},
		{
			name: "consumer precedes provisioning",
			mutate: func(steps []workflowStep) {
				for index, step := range steps {
					if step.ID == "npm-provision" {
						steps[index], steps[index+1] = steps[index+1], steps[index]
						return
					}
				}
			},
			want: "npm provisioning must precede publish diagnostics and npm-profile-publish",
		},
		{
			name: "combined assertion and provisioning violates assertion shape",
			mutate: func(steps []workflowStep) {
				for index, step := range steps {
					if step.ID == "npm-provision" {
						steps[index-1].Run += " && " + step.Run
					}
				}
			},
			want: "publish job must invoke npm-profile-toolchain-assert --node-only --report-path after Node.js setup",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(document.Jobs["publish"].Steps)
			_, err = CheckPublishJob(writeWorkflowDocument(t, document))
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestBuildToolchainSameStepNPMBeforeAssertion(t *testing.T) {
	t.Parallel()
	document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	for index, step := range document.Jobs["build"].Steps {
		if strings.Contains(step.Run, "npm-profile-toolchain-assert") {
			document.Jobs["build"].Steps[index].Run += " && npm --version"
		}
	}
	_, err = CheckBuildJob(writeWorkflowDocument(t, document))
	const want = "build toolchain pair assertion must precede every npm invocation"
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}
