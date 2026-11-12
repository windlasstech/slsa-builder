package npmprofile

import (
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	// NPMWorkflowPath is the immutable npm producer workflow path.
	NPMWorkflowPath = ".github/workflows/js-ts-npm-package-slsa3.yml"
	// NPMBuildType is the acquired-domain npm producer build type.
	NPMBuildType = "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1"
	// NPMProvenanceStatementFile is the deterministic exact-byte DSSE payload basename.
	NPMProvenanceStatementFile = "slsa-provenance-statement.json"

	IDUnexpectedExternalParameters        = "windlass.verify.error.unexpected-external-parameters"
	IDNPMSubjectMismatch                  = "windlass.verify.error.npm-purl-subject-mismatch"
	IDSubjectDigestScopeInvalid           = "windlass.verify.error.subject-digest-scope-invalid"
	IDMissingSubjectSHA256                = "windlass.verify.error.missing-subject-sha256"
	IDMissingSubjectSHA512                = "windlass.verify.error.missing-subject-sha512"
	IDResolvedDependenciesLockfile        = "windlass.verify.error.resolved-dependencies-lockfile"
	IDResolvedDependenciesNodeRuntime     = "windlass.verify.error.resolved-dependencies-node-runtime"
	IDResolvedDependenciesDistribution    = "windlass.verify.error.resolved-dependencies-package-manager-distribution"
	IDResolvedDependenciesRunnerImage     = "windlass.verify.error.resolved-dependencies-runner-image"
	IDResolvedDependenciesUnexpectedEntry = "windlass.verify.error.resolved-dependencies-unexpected-entry"
	IDBuilderDependenciesMismatch         = "windlass.verify.error.builder-dependencies-signing-adapter-mismatch"
	IDReleaseRefMismatch                  = "windlass.verify.error.release-ref-mismatch"
	IDSourceRefInvalid                    = "windlass.verify.error.source-ref-invalid"
)

// ExternalParameters is the closed npm v1 buildType external interface.
type ExternalParameters struct {
	Source       SourceParameters       `json:"source,omitzero"`
	Workflow     WorkflowParameters     `json:"workflow"`
	Package      PackageParameters      `json:"package"`
	Publish      PublishParameters      `json:"publish"`
	Distribution DistributionParameters `json:"distribution"`
}

type SourceParameters struct {
	Ref string `json:"ref"`
}

type WorkflowParameters struct {
	Ref        string `json:"ref"`
	Repository string `json:"repository"`
	Path       string `json:"path"`
}

type PackageParameters struct {
	Directory string `json:"directory"`
}

type PackageManagerParameters struct {
	Name                  Manager         `json:"name"`
	Version               string          `json:"version"`
	SelectionSource       SelectionSource `json:"selection_source"`
	SelectionManifest     *string         `json:"selection_manifest"`
	SelectionManifestPath *string         `json:"selection_manifest_path"`
	SelectionLockfilePath *string         `json:"selection_lockfile_path"`
	Root                  string          `json:"root"`
	IgnoredLockfilePaths  []string        `json:"ignored_lockfile_paths,omitempty"`
	YarnInstallMode       string          `json:"yarn_install_mode,omitempty"`
}

type PublishConfigParameters struct {
	Registry   string `json:"registry,omitempty"`
	Access     string `json:"access,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Provenance *bool  `json:"provenance,omitempty"`
}

type PublishParameters struct {
	InputRegistryURL *string `json:"input_registry_url"`
	InputDistTag     *string `json:"input_dist_tag"`
	InputAccess      *string `json:"input_access"`
}

type PublishIntent struct {
	InputRegistryURL      *string
	InputDistTag          *string
	InputAccess           *string
	PublishConfig         *PublishConfigParameters `json:"publish_config"`
	ResolvedRegistryURL   string                   `json:"resolved_registry_url"`
	ResolvedDistTag       string                   `json:"resolved_dist_tag"`
	PublishAccessOption   *string                  `json:"publish_access_option"`
	EffectiveAccess       string                   `json:"effective_access"`
	TrustedPublishing     bool                     `json:"trusted_publishing"`
	ProvenanceFile        bool                     `json:"provenance_file"`
	CustomRegistrySupport string                   `json:"custom_registry_support,omitempty"`
}

type DistributionParameters struct {
	ReleaseAssetMode       bool `json:"release_asset_mode"`
	LinkedArtifactMetadata bool `json:"linked_artifact_metadata"`
}

// NPMProvenanceInput combines the digest-bound N03 metadata with signing-job observations.
type NPMProvenanceInput struct {
	BuildMetadata         BuildMetadata
	BuilderID             string
	GitHub                provenance.GitHubParameters
	NodeJSVersion         string
	NPMVersion            string
	InvocationRevision    string
	Packed                PackedMetadata
	CorepackVersion       *string
	InvocationID          string
	StartedOn             string
	FinishedOn            string
	RuntimeReleaseRef     string
	PeeledReleaseRevision string
}

// ProvenanceSigningInput contains the exact Statement supplied to the Go-native DSSE signer.
type ProvenanceSigningInput struct {
	Subject           provenance.Subject
	PredicateType     string
	Predicate         provenance.Predicate
	PredicateJSON     []byte
	StatementJSON     []byte
	StatementFileName string
}

// Statement returns the in-toto Statement implied by the custom-mode signing inputs.
func (input ProvenanceSigningInput) Statement() provenance.Statement {
	return provenance.Statement{
		Type:          provenance.StatementType,
		Subject:       []provenance.Subject{input.Subject},
		PredicateType: input.PredicateType,
		Predicate:     input.Predicate,
	}
}
