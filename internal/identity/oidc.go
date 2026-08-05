package identity

import "strings"

// Platform identifies the runtime surface that supplies reusable-workflow identity.
type Platform string

const (
	// PlatformGitHubDotCom is the only platform with the required job.workflow_* delivery surface.
	PlatformGitHubDotCom Platform = githubHost
)

// OIDCClaims is the validated target model for GitHub-issued identity claims.
// JWT acquisition and signature verification are intentionally outside this package.
type OIDCClaims struct {
	Issuer            string `json:"iss"`
	JobWorkflowRef    string `json:"job_workflow_ref"`
	JobWorkflowSHA    string `json:"job_workflow_sha"`
	Repository        string `json:"repository"`
	RepositoryID      string `json:"repository_id"`
	RepositoryOwnerID string `json:"repository_owner_id"`
	SHA               string `json:"sha"`
	Ref               string `json:"ref"`
	RunID             string `json:"run_id"`
	RunAttempt        string `json:"run_attempt"`
	RunnerEnvironment string `json:"runner_environment"`
}

// BindingPolicy contains the expected immutable values for ADR 0068 validation.
type BindingPolicy struct {
	Platform                Platform `json:"platform"`
	WorkflowPath            string   `json:"workflow_path"`
	WorkflowSHA             string   `json:"workflow_sha"`
	SourceRepositoryURI     string   `json:"source_repository_uri"`
	SourceRepositoryID      string   `json:"source_repository_id"`
	SourceRepositoryOwnerID string   `json:"source_repository_owner_id"`
	SourceDigest            string   `json:"source_digest"`
	SourceRef               string   `json:"source_ref"`
}

// RuntimeIdentity contains the normalized values safe to pass to provenance construction.
type RuntimeIdentity struct {
	BuilderID           string
	WorkflowSHA         string
	SourceRepositoryURI string
	SourceDigest        string
	SourceRef           string
	RunInvocationURI    string
}

// ValidateMaximalOIDCBinding enforces all six ADR 0068 identity bindings.
func ValidateMaximalOIDCBinding(claims OIDCClaims, policy BindingPolicy) (RuntimeIdentity, error) {
	if err := validateBindingPolicy(policy); err != nil {
		return RuntimeIdentity{}, err
	}
	if claims.Issuer != trustedGitHubOIDCIssuer {
		return RuntimeIdentity{}, validationError(IDIssuerMismatch, "iss", "issuer must exactly match the GitHub Actions issuer")
	}

	expectedWorkflowRefPrefix := "windlasstech/slsa-builder/" + policy.WorkflowPath + "@"
	workflowRefSHA, found := strings.CutPrefix(claims.JobWorkflowRef, expectedWorkflowRefPrefix)
	if !found {
		return RuntimeIdentity{}, validationError(
			IDSignerWorkflowPathMismatch,
			"job_workflow_ref",
			"called workflow repository and path do not match policy",
		)
	}
	if !validFullSHA(workflowRefSHA) || !validFullSHA(claims.JobWorkflowSHA) ||
		workflowRefSHA != claims.JobWorkflowSHA || claims.JobWorkflowSHA != policy.WorkflowSHA {
		return RuntimeIdentity{}, validationError(
			IDSignerWorkflowSHAMismatch,
			"job_workflow_sha",
			"resolved called-workflow SHA, ref suffix, and policy SHA must match",
		)
	}

	builderID, err := NewBuilderID(policy.WorkflowPath, claims.JobWorkflowSHA)
	if err != nil {
		return RuntimeIdentity{}, validationError(IDSignerWorkflowSHAMismatch, "job_workflow_sha", "cannot construct immutable builder identity")
	}

	repositoryURI, err := CanonicalRepository(claims.Repository)
	if err != nil || repositoryURI != policy.SourceRepositoryURI {
		return RuntimeIdentity{}, validationError(
			IDSourceIdentityMismatch,
			"repository",
			"source repository does not match the canonical policy URI",
		)
	}
	if !validPositiveDecimal(claims.RepositoryID) || !validPositiveDecimal(claims.RepositoryOwnerID) ||
		claims.RepositoryID != policy.SourceRepositoryID || claims.RepositoryOwnerID != policy.SourceRepositoryOwnerID {
		return RuntimeIdentity{}, validationError(
			IDSourceNumericIDMismatch,
			"repository_id",
			"repository and owner numeric IDs must match policy; names are not authoritative",
		)
	}
	if !validFullSHA(claims.SHA) || claims.SHA != policy.SourceDigest {
		return RuntimeIdentity{}, validationError(IDSourceDigestMismatch, "sha", "source digest does not match policy")
	}
	if !validReleaseRef(claims.Ref) || claims.Ref != policy.SourceRef {
		return RuntimeIdentity{}, validationError(IDSourceRefMismatch, "ref", "source release ref does not match policy")
	}

	runInvocationURI, err := NewRunInvocationURI(repositoryURI, claims.RunID, claims.RunAttempt)
	if err != nil {
		return RuntimeIdentity{}, err
	}
	if err := ValidateHostedRunner(policy.Platform, claims.RunnerEnvironment); err != nil {
		return RuntimeIdentity{}, err
	}

	return RuntimeIdentity{
		BuilderID:           builderID,
		WorkflowSHA:         claims.JobWorkflowSHA,
		SourceRepositoryURI: repositoryURI,
		SourceDigest:        claims.SHA,
		SourceRef:           claims.Ref,
		RunInvocationURI:    runInvocationURI,
	}, nil
}

// ValidateHostedRunner rejects GHES and any runner identity other than platform-signed github-hosted.
func ValidateHostedRunner(platform Platform, runnerEnvironment string) error {
	if platform != PlatformGitHubDotCom {
		return validationError(
			IDSignerIdentityClaimMissing,
			"platform",
			"job.workflow_* identity acquisition is supported only on github.com, not GHES",
		)
	}
	if runnerEnvironment != "github-hosted" {
		return validationError(
			IDSelfHostedRunner,
			"runner_environment",
			"runner environment must be the platform-signed github-hosted value",
		)
	}
	return nil
}

func validateBindingPolicy(policy BindingPolicy) error {
	if policy.Platform != PlatformGitHubDotCom {
		return ValidateHostedRunner(policy.Platform, "")
	}
	if !validWorkflowPath(policy.WorkflowPath) {
		return validationError(IDSignerWorkflowPathMismatch, "workflow_path", "policy workflow path is not canonical")
	}
	if !validFullSHA(policy.WorkflowSHA) {
		return validationError(IDSignerWorkflowSHAMismatch, "workflow_sha", "policy workflow SHA is not a full lowercase SHA")
	}
	if err := ValidateCanonicalRepositoryURI(policy.SourceRepositoryURI); err != nil {
		return err
	}
	if !validPositiveDecimal(policy.SourceRepositoryID) || !validPositiveDecimal(policy.SourceRepositoryOwnerID) {
		return validationError(IDSourceNumericIDMismatch, "source.repository_id", "policy numeric IDs must be positive decimal strings")
	}
	if !validFullSHA(policy.SourceDigest) {
		return validationError(IDSourceDigestMismatch, "source.digest", "policy source digest is not a full lowercase SHA")
	}
	if !validReleaseRef(policy.SourceRef) {
		return validationError(IDSourceRefMismatch, "source.ref", "policy source ref is not a full release tag ref")
	}
	return nil
}
