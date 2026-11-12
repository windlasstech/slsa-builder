package identity

import "testing"

const (
	profileWorkflowPath = ".github/workflows/js-ts-npm-package-slsa3.yml"
	profileBuilderID    = builderRepositoryURI + "/" + profileWorkflowPath + "@" + workflowSHA
)

func TestValidateBuilderIDForWorkflow(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, builderID, workflowPath, wantID string
	}{
		{name: "authorized path accepted", builderID: profileBuilderID, workflowPath: profileWorkflowPath},
		{name: "untrusted workflow path", builderID: builderRepositoryURI + "/.github/workflows/other.yml@" + workflowSHA, workflowPath: profileWorkflowPath, wantID: "windlass.verify.error.wrong-builder-id"},
		{name: "mutable tag reference", builderID: builderRepositoryURI + "/" + profileWorkflowPath + "@v1.2.3", workflowPath: profileWorkflowPath, wantID: "windlass.verify.error.builder-id-not-immutable"},
		{name: "malformed builder id", builderID: "not-a-builder-id", workflowPath: profileWorkflowPath, wantID: "windlass.verify.error.builder-id-not-immutable"},
		{name: "untrusted repository", builderID: "https://github.com/other/repo/" + profileWorkflowPath + "@" + workflowSHA, workflowPath: profileWorkflowPath, wantID: "windlass.verify.error.builder-id-not-immutable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateBuilderIDForWorkflow(test.builderID, test.workflowPath)
			if test.wantID == "" {
				if err != nil {
					t.Fatalf("ValidateBuilderIDForWorkflow() error = %v", err)
				}
				return
			}
			requireDiagnostic(t, err, DiagnosticID(test.wantID))
		})
	}
}
