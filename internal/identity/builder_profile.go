package identity

import "strings"

// ValidateBuilderIDForWorkflow enforces canonical identity for one authorized profile workflow.
func ValidateBuilderIDForWorkflow(builderID, workflowPath string) error {
	if err := ValidateBuilderID(builderID); err != nil {
		return err
	}
	_, workflowSHA, _ := strings.Cut(builderID, "@")
	canonical, err := NewBuilderID(workflowPath, workflowSHA)
	if err != nil {
		return err
	}
	if builderID != canonical {
		return validationError(IDBuilderIDNotImmutable, "builder.id", "builder ID does not use the authorized profile workflow path")
	}
	return nil
}
