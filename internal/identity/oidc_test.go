package identity

import "testing"

func TestOIDCClaimFailureClassification(t *testing.T) {
	fixture := loadFixture[maximalIdentityFixture](t, "npm-maximal-identity-valid.json")
	otherSHA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name   string
		mutate func(*OIDCClaims)
		want   DiagnosticID
	}{
		{"missing_issuer", func(claims *OIDCClaims) { claims.Issuer = "" }, IDSignerIdentityClaimMissing},
		{"malformed_issuer", func(claims *OIDCClaims) { claims.Issuer = "://" }, IDIssuerMismatch},
		{"unequal_issuer", func(claims *OIDCClaims) { claims.Issuer = "https://issuer.example" }, IDIssuerMismatch},
		{"missing_workflow_ref", func(claims *OIDCClaims) { claims.JobWorkflowRef = "" }, IDSignerIdentityClaimMissing},
		{"malformed_workflow_ref", func(claims *OIDCClaims) {
			claims.JobWorkflowRef = "windlasstech/slsa-builder/" + workflowPath + "@main"
		}, IDSignerWorkflowSHAMismatch},
		{"unequal_workflow_ref", func(claims *OIDCClaims) {
			claims.JobWorkflowRef = "windlasstech/slsa-builder/.github/workflows/other.yml@" + workflowSHA
		}, IDSignerWorkflowPathMismatch},
		{"missing_workflow_sha", func(claims *OIDCClaims) { claims.JobWorkflowSHA = "" }, IDSignerIdentityClaimMissing},
		{"malformed_workflow_sha", func(claims *OIDCClaims) { claims.JobWorkflowSHA = "main" }, IDSignerWorkflowSHAMismatch},
		{"unequal_workflow_sha", func(claims *OIDCClaims) { claims.JobWorkflowSHA = otherSHA }, IDSignerWorkflowSHAMismatch},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := fixture.Claims
			test.mutate(&claims)
			_, err := ValidateMaximalOIDCBinding(claims, fixture.Policy)
			requireDiagnostic(t, err, test.want)
		})
	}
}

func TestMalformedBindingPolicyUsesPolicySchemaDiagnostic(t *testing.T) {
	fixture := loadFixture[maximalIdentityFixture](t, "npm-maximal-identity-valid.json")
	const policySchemaInvalid = DiagnosticID("windlass.verify.error.policy-schema-invalid")
	tests := []struct {
		name   string
		mutate func(*BindingPolicy)
	}{
		{"platform", func(policy *BindingPolicy) { policy.Platform = "" }},
		{"workflow_path", func(policy *BindingPolicy) { policy.WorkflowPath = "../workflow.yml" }},
		{"workflow_sha", func(policy *BindingPolicy) { policy.WorkflowSHA = "main" }},
		{"source_repository_uri", func(policy *BindingPolicy) {
			policy.SourceRepositoryURI = "https://GitHub.com/example/acme-widget"
		}},
		{"source_repository_id", func(policy *BindingPolicy) { policy.SourceRepositoryID = "0" }},
		{"source_repository_owner_id", func(policy *BindingPolicy) { policy.SourceRepositoryOwnerID = "owner" }},
		{"source_digest", func(policy *BindingPolicy) { policy.SourceDigest = "main" }},
		{"source_ref", func(policy *BindingPolicy) { policy.SourceRef = "refs/heads/main" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := fixture.Policy
			test.mutate(&policy)
			_, err := ValidateMaximalOIDCBinding(fixture.Claims, policy)
			requireDiagnostic(t, err, policySchemaInvalid)
		})
	}
}
