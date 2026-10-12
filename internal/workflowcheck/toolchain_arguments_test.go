package workflowcheck

import (
	"strings"
	"testing"
)

func TestPublishExecutableArgumentRejectsSpoofing(t *testing.T) {
	t.Parallel()
	const expectedFlag = `--npm-executable "` + provisionOutput + `"`
	for name, argument := range map[string]string{
		"comment only":             `--npm-executable "$AMBIENT" # ` + expectedFlag,
		"comment on separate line": `--npm-executable "$AMBIENT"` + "\n# " + expectedFlag,
		"unknown variable":         `--npm-executable "$AMBIENT"`,
		"appended shell token":     expectedFlag + `"$AMBIENT"`,
		"duplicate flag":           expectedFlag + ` --npm-executable "$AMBIENT"`,
		"wrong first flag":         `--npm-executable "$AMBIENT" ` + expectedFlag,
		"different flag name":      `--other-` + expectedFlag,
		"literal environment name": `--npm-executable '$NPM_EXECUTABLE'`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given a publish invocation whose actual argument is not the pinned output.
			document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
			if err != nil {
				t.Fatal(err)
			}
			for index, step := range document.Jobs["publish"].Steps {
				if step.ID == "publish" {
					document.Jobs["publish"].Steps[index].Env = map[string]string{"NPM_EXECUTABLE": provisionOutput}
					document.Jobs["publish"].Steps[index].Run = "go run ./cmd/slsa-builder-internal npm-profile-publish " + argument
				}
			}
			// When checked through the YAML contract.
			_, err = CheckPublishJob(writeWorkflowDocument(t, document))
			// Then comments and substrings cannot substitute for the argument token.
			if err == nil {
				t.Fatal("untrusted executable argument accepted, want rejection")
			}
		})
	}
}

func TestPublishExecutableArgumentAcceptsPinnedToken(t *testing.T) {
	t.Parallel()
	for _, argument := range []string{
		`--npm-executable "` + provisionOutput + `"`,
		`--npm-executable='` + provisionOutput + `'`,
		"--npm-executable \\\n\"" + provisionOutput + "\" # trusted CLI",
	} {
		t.Run(argument, func(t *testing.T) {
			t.Parallel()
			// Given a quoted pinned token with shell formatting variations.
			document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
			if err != nil {
				t.Fatal(err)
			}
			for index, step := range document.Jobs["publish"].Steps {
				if step.ID == "publish" {
					document.Jobs["publish"].Steps[index].Run = "go run ./cmd/slsa-builder-internal npm-profile-publish " + argument
				}
			}
			// When checked.
			_, err = CheckPublishJob(writeWorkflowDocument(t, document))
			// Then only shell formatting, not the executable identity, differs.
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPublishCommentsCannotSupplyProvisioning(t *testing.T) {
	t.Parallel()
	// Given an inert provisioning command in a shell comment.
	contents := replaceOnce(t, validNPMOnlyWorkflow, "        run: "+provisionNPMCommand,
		"        run: |\n          # "+provisionNPMCommand)
	// When checked.
	_, err := CheckPublishJob(writeWorkflow(t, contents))
	// Then diagnostics have no executed provisioner.
	if err == nil || !strings.Contains(err.Error(), "provision") {
		t.Fatalf("commented provisioner error = %v, want provisioning rejection", err)
	}
}
