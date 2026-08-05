package policy

// ExplicitPolicy is the closed schema-version-1 verifier policy.
type ExplicitPolicy struct {
	SchemaVersion string         `json:"schema_version"`
	Source        SourcePolicy   `json:"source"`
	Producer      ProducerPolicy `json:"producer"`
	TrustRoot     TrustRoot      `json:"trust_root"`
}

// SourcePolicy contains immutable caller source expectations.
type SourcePolicy struct {
	RepositoryURI     string `json:"repository_uri"`
	RepositoryID      string `json:"repository_id"`
	RepositoryOwnerID string `json:"repository_owner_id"`
	Digest            string `json:"digest"`
	Ref               string `json:"ref"`
}

// ProducerPolicy contains immutable producer workflow expectations.
type ProducerPolicy struct {
	WorkflowPath      string `json:"workflow_path"`
	WorkflowSHA       string `json:"workflow_sha"`
	RunnerEnvironment string `json:"runner_environment"`
}

// TrustRoot is the union of the closed TUF and pinned-root member sets.
type TrustRoot struct {
	Mode          string  `json:"mode"`
	Instance      string  `json:"instance"`
	Path          *string `json:"path,omitempty"`
	SHA256        *string `json:"sha256,omitempty"`
	TUFRepository *string `json:"tuf_repository,omitempty"`
	RevalidatedAt *string `json:"revalidated_at,omitempty"`
	RefreshBefore *string `json:"refresh_before,omitempty"`
}

// ReleaseManifestExpectation authenticates a manifest signer and selects one producer profile.
type ReleaseManifestExpectation struct {
	SchemaVersion   string                  `json:"schema_version"`
	ReleaseManifest ReleaseManifestIdentity `json:"release_manifest"`
	ProducerProfile ProducerProfile         `json:"producer_profile"`
}

// ReleaseManifestIdentity contains immutable release-manifest signer expectations.
type ReleaseManifestIdentity struct {
	SourceRepositoryURI     string `json:"source_repository_uri"`
	SourceRepositoryID      string `json:"source_repository_id"`
	SourceRepositoryOwnerID string `json:"source_repository_owner_id"`
	WorkflowPath            string `json:"workflow_path"`
	WorkflowSHA             string `json:"workflow_sha"`
}

// ProducerProfile selects one registered producer workflow mapping.
type ProducerProfile struct {
	Profile      string `json:"profile"`
	WorkflowPath string `json:"workflow_path"`
	WorkflowSHA  string `json:"workflow_sha"`
}

// ProducerProfileRegistry resolves the closed set of policy-registered producer profiles.
type ProducerProfileRegistry interface {
	IsRegisteredProducerProfile(string) bool
}
