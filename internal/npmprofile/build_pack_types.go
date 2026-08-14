package npmprofile

import (
	"encoding/json"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

// BuildScriptResult records the observable optional build-stage outcome.
type BuildScriptResult string

const (
	BuildScriptExecuted      BuildScriptResult = "executed"
	BuildScriptSkippedAbsent BuildScriptResult = "skipped-absent"
)

// BuildScriptCapture records whether scripts.build was declared and executed.
type BuildScriptCapture struct {
	Present bool              `json:"present"`
	Result  BuildScriptResult `json:"result"`
}

// RunnerCapture records the GitHub-hosted runner observations available to the build.
type RunnerCapture struct {
	ImageLabel          string `json:"image_label,omitempty"`
	ImageOS             string `json:"image_os,omitempty"`
	ImageVersion        string `json:"image_version,omitempty"`
	IncludedSoftwareURL string `json:"included_software_url,omitempty"`
	ImageReleaseURL     string `json:"image_release_url,omitempty"`
}

// DistributionCapture records Corepack's evidence for the executed pnpm or Yarn distribution.
type DistributionCapture struct {
	URL               string  `json:"url"`
	SHA512            string  `json:"sha512"`
	DigestAuthority   string  `json:"digest_authority"`
	PackageManager    Manager `json:"package_manager"`
	PackageManagerVer string  `json:"package_manager_version"`
	AcquisitionSource string  `json:"acquisition_source"`
}

// ToolchainCapture records exact versions observed from the executables used by the build.
type ToolchainCapture struct {
	NodeVersion           string               `json:"node_version"`
	NPMVersion            string               `json:"npm_version"`
	CorepackVersion       string               `json:"corepack_version,omitempty"`
	PackageManagerVersion string               `json:"package_manager_version"`
	Distribution          *DistributionCapture `json:"distribution,omitempty"`
	Runner                RunnerCapture        `json:"runner"`
}

// PackedMetadata is the authoritative identity and file list read from the .tgz.
type PackedMetadata struct {
	Name            string                     `json:"name"`
	Version         string                     `json:"version"`
	Files           []string                   `json:"files"`
	ConsumerSurface map[string]json.RawMessage `json:"consumer_surface,omitempty"`
}

// PrimaryArtifact is the digest-bound one-file tarball handoff recorded in build metadata.
type PrimaryArtifact struct {
	ArtifactName    string `json:"artifact_name"`
	PayloadFileName string `json:"payload_file_name"`
	SHA256          string `json:"sha256"`
	SHA512          string `json:"sha512"`
}

// BuildMetadata is the closed same-run build metadata envelope.
type BuildMetadata struct {
	SchemaVersion        string                          `json:"schema_version"`
	PrimaryArtifact      PrimaryArtifact                 `json:"primary_artifact"`
	ExternalParameters   json.RawMessage                 `json:"external_parameters"`
	ResolvedDependencies []provenance.ResourceDescriptor `json:"resolved_dependencies"`
}

// BuildPackConfig supplies trusted selection state and same-run metadata destinations.
type BuildPackConfig struct {
	Selection            Result
	OutputDirectory      string
	ArtifactName         string
	ExternalParameters   json.RawMessage
	ResolvedDependencies []provenance.ResourceDescriptor
	fetcher              distributionFetcher
}

// BuildPackResult contains the one packed artifact, metadata, and observed tool state.
type BuildPackResult struct {
	Manager        Manager
	PackageName    string
	PackageVersion string
	PackagePURL    string
	TarballPath    string
	MetadataPath   string
	SHA256         digest.SHA256
	SHA512         digest.SHA512
	Packed         PackedMetadata
	BuildScript    BuildScriptCapture
	Toolchain      ToolchainCapture
}
