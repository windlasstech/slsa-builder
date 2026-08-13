package npmprofile

import (
	"encoding/json"

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
	IDResolvedDependenciesDistribution    = "windlass.verify.error.resolved-dependencies-package-manager-distribution"
	IDResolvedDependenciesRunnerImage     = "windlass.verify.error.resolved-dependencies-runner-image"
	IDResolvedDependenciesUnexpectedEntry = "windlass.verify.error.resolved-dependencies-unexpected-entry"
	IDBuilderDependenciesMismatch         = "windlass.verify.error.builder-dependencies-signing-adapter-mismatch"
	IDReleaseRefMismatch                  = "windlass.verify.error.release-ref-mismatch"
	IDSourceRefInvalid                    = "windlass.verify.error.source-ref-invalid"
)

// ExternalParameters is the closed npm v1 buildType external interface.
type ExternalParameters struct {
	Source         SourceParameters         `json:"source"`
	Workflow       WorkflowParameters       `json:"workflow"`
	Runtime        RuntimeParameters        `json:"runtime"`
	Package        PackageParameters        `json:"package"`
	PackageManager PackageManagerParameters `json:"package_manager"`
	Publish        PublishParameters        `json:"publish"`
	Release        ReleaseParameters        `json:"release"`
	Distribution   DistributionParameters   `json:"distribution"`
	Caller         CallerParameters         `json:"caller"`
	Build          BuildParameters          `json:"build"`
}

type SourceParameters struct {
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	Revision   string `json:"revision"`
	EventName  string `json:"event_name"`
	RefType    string `json:"ref_type"`
}

type WorkflowParameters struct {
	Path      string `json:"path"`
	SHA       string `json:"sha"`
	BuilderID string `json:"builder_id"`
}

type RuntimeParameters struct {
	Runner      string `json:"runner"`
	NodeVersion string `json:"node_version"`
	NPMVersion  string `json:"npm_version"`
}

type PackageParameters struct {
	Directory        string                     `json:"directory"`
	WorkspaceRoot    *string                    `json:"workspace_root"`
	SourceManifest   string                     `json:"source_manifest"`
	Name             string                     `json:"name"`
	Version          string                     `json:"version"`
	Private          bool                       `json:"private"`
	Repository       string                     `json:"repository"`
	TarballName      string                     `json:"tarball_name"`
	PackageURL       string                     `json:"package_url"`
	PackedName       string                     `json:"packed_name"`
	PackedVersion    string                     `json:"packed_version"`
	PublishConfigRaw map[string]json.RawMessage `json:"publish_config_raw,omitempty"`
	PackedFiles      []string                   `json:"packed_files,omitempty"`
	ConsumerSurface  map[string]json.RawMessage `json:"consumer_surface,omitempty"`
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
	InputRegistryURL           *string                  `json:"input_registry_url"`
	InputDistTag               *string                  `json:"input_dist_tag"`
	InputAccess                *string                  `json:"input_access"`
	PublishConfig              *PublishConfigParameters `json:"publish_config"`
	ResolvedRegistryURL        string                   `json:"resolved_registry_url"`
	ResolvedDistTag            string                   `json:"resolved_dist_tag"`
	PublishAccessOption        *string                  `json:"publish_access_option"`
	EffectiveAccess            string                   `json:"effective_access"`
	TrustedPublishing          bool                     `json:"trusted_publishing"`
	ProvenanceFile             bool                     `json:"provenance_file"`
	PackageIdentityPreexisting *bool                    `json:"package_identity_preexisting"`
	PackageVersionPreexisting  *bool                    `json:"package_version_preexisting"`
	CustomRegistrySupport      string                   `json:"custom_registry_support,omitempty"`
}

type ReleaseParameters struct {
	Ref        string `json:"ref"`
	VersionTag string `json:"version_tag"`
}

type DistributionParameters struct {
	ReleaseAssetMode       bool    `json:"release_asset_mode"`
	ReleaseTagSupplied     bool    `json:"release_tag_supplied"`
	ProvenanceSidecar      *string `json:"provenance_sidecar"`
	LinkedArtifactMetadata bool    `json:"linked_artifact_metadata"`
}

type CallerParameters struct {
	WorkflowFilename string `json:"workflow_filename"`
}

type BuildParameters struct {
	ScriptPresent bool              `json:"script_present"`
	ScriptResult  BuildScriptResult `json:"script_result"`
}

// NPMProvenanceInput combines the digest-bound N03 metadata with signing-job observations.
type NPMProvenanceInput struct {
	BuildMetadata         BuildMetadata
	BuilderID             string
	NodeJSVersion         string
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
