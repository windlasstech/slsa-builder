package npmprofile

import (
	"strings"
)

// ParseCallerWorkflowPath extracts the repository-relative caller path from GitHub workflow_ref.
func ParseCallerWorkflowPath(reference string) (string, error) {
	if strings.Count(reference, "@") != 1 || strings.ContainsAny(reference, "%\\\x00") || strings.TrimSpace(reference) != reference {
		return "", npmValidationError(IDTrustedPublisherMismatch, "workflow_ref", "caller workflow_ref is not canonical")
	}
	workflow, ref, _ := strings.Cut(reference, "@")
	components := strings.Split(workflow, "/")
	if ref == "" || len(components) != 5 || components[0] == "" || components[1] == "" || components[2] != ".github" || components[3] != "workflows" {
		return "", npmValidationError(IDTrustedPublisherMismatch, "workflow_ref", "caller workflow_ref does not identify one repository workflow")
	}
	callerPath := strings.Join(components[2:], "/")
	if !validCallerWorkflowPath(callerPath) || callerPath == NPMWorkflowPath {
		return "", npmValidationError(IDTrustedPublisherMismatch, "workflow_ref", "caller workflow path is invalid or names the reusable builder")
	}
	return callerPath, nil
}
