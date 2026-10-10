// Package npmprofile resolves the selected npm package and its release package manager.
package npmprofile

import "github.com/windlasstech/slsa-builder/internal/diagnostic"

const (
	IDPackageManifestInvalid            = "windlass.verify.error.package-manifest-invalid"
	IDPackageMetadataRequired           = "windlass.verify.error.package-metadata-required"
	IDPackagePrivate                    = "windlass.verify.error.package-private"
	IDPackageResolutionInvalid          = "windlass.verify.error.package-resolution-invalid"
	IDPackageManagerConflict            = "windlass.verify.error.package-manager-conflict"
	IDPackageManagerVersionRequired     = "windlass.verify.error.package-manager-version-required"
	IDPackageManagerDigestMalformed     = "windlass.verify.error.package-manager-digest-malformed"
	IDPackageManagerDigestMismatch      = "windlass.verify.error.package-manager-digest-mismatch"
	IDYarnSelectionInvalid              = "windlass.verify.error.yarn-selection-invalid"
	IDPnpmVersionUnsupported            = "windlass.verify.error.pnpm-version-unsupported"
	IDYarnVersionUnsupported            = "windlass.verify.error.yarn-version-unsupported"
	IDRequiredLockfileMissing           = "windlass.verify.error.required-lockfile-missing"
	IDPackageRepositoryIdentityMismatch = "windlass.verify.error.package-repository-identity-mismatch"
	IDPackedPackageMetadataMismatch     = "windlass.verify.error.packed-package-metadata-mismatch"
	IDPackagePackFailed                 = "windlass.verify.error.package-pack-failed"
	IDNPMVersionMismatch                = "windlass.verify.warning.npm-version-mismatch"
)

// Manager is one supported build-stage package manager.
type Manager string

const (
	ManagerNPM  Manager = "npm"
	ManagerPNPM Manager = "pnpm"
	ManagerYarn Manager = "yarn"
)

// SelectionSource identifies the manifest field or lockfile that selected a manager.
type SelectionSource string

const (
	SelectionPackageManager SelectionSource = "packageManager"
	SelectionDevEngines     SelectionSource = "devEngines.packageManager"
	SelectionLockfile       SelectionSource = "lockfile"
)

// Config contains trusted repository observations and the caller's package selector.
type Config struct {
	RepositoryRoot     string
	PackageDirectory   string
	ObservedRepository string
}

// Package describes the one selected package and its workspace context.
type Package struct {
	Directory                    string
	RealDirectory                string
	RealManagerRoot              string
	ManagerRoot                  string
	ManagerRootRelativeDirectory string
	Name                         string
	Version                      string
	Repository                   string
}

// DeclaredDigest is the optional integrity digest a pnpm or Yarn descriptor
// declares for its package-manager distribution (ADRs 0092-0094).
type DeclaredDigest struct {
	Algorithm string
	Hex       string
}

// ManagerSelection is the deterministic package-manager selection result.
type ManagerSelection struct {
	Name                  Manager
	Version               string
	Digest                *DeclaredDigest
	DeclaredVersion       string
	DeclaredVersionSet    bool
	Source                SelectionSource
	SelectionManifestPath string
	SelectionLockfilePath string
	SelectedLockfilePath  string
	IgnoredLockfilePaths  []string
}

// Descriptor renders the selected descriptor version: the plain version, or
// the version with its declared digest suffix when a digest was declared.
// Version always carries the plain version without the suffix.
func (selection ManagerSelection) Descriptor() string {
	if selection.Digest == nil {
		return selection.Version
	}
	return selection.Version + "+" + selection.Digest.Algorithm + "." + selection.Digest.Hex
}

// Result contains the resolved policy state and its machine-readable report.
type Result struct {
	Package Package
	Manager ManagerSelection
	Report  diagnostic.Report
}

type manifest struct {
	Name           string            `json:"name"`
	Version        string            `json:"version"`
	Private        bool              `json:"private"`
	PackageManager jsonRaw           `json:"packageManager"`
	DevEngines     jsonRaw           `json:"devEngines"`
	PublishConfig  jsonRaw           `json:"publishConfig"`
	Workspaces     jsonRaw           `json:"workspaces"`
	Repository     jsonRaw           `json:"repository"`
	License        jsonRaw           `json:"license"`
	Description    string            `json:"description"`
	Keywords       []string          `json:"keywords"`
	Author         jsonRaw           `json:"author"`
	Homepage       string            `json:"homepage"`
	Scripts        map[string]string `json:"scripts"`
}

type jsonRaw []byte

func (raw *jsonRaw) UnmarshalJSON(encoded []byte) error {
	*raw = append((*raw)[:0], encoded...)
	return nil
}

type resolvedPackage struct {
	repositoryRootReal string
	directoryReal      string
	directory          string
	managerRootReal    string
	managerRoot        string
	manifest           manifest
	manifestPath       string
	rootManifest       *manifest
	rootManifestPath   string
}

type managerCandidate struct {
	name            Manager
	version         string
	versionDeclared bool
	digest          *DeclaredDigest
	source          SelectionSource
	manifestPath    string
}
