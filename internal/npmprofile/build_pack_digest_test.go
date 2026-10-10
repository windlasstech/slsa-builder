package npmprofile

// Hermetic ADR 0092/0095 scenarios: declared descriptor digests are
// reconciled against the acquired distribution bytes before install (failing
// closed with package-manager-digest-mismatch), and a declared npm version
// that disagrees with the toolchain npm produces a warning, never a failure.

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildPackPNPMDeclaredSHA512Match(t *testing.T) {
	declared := fixtureDigestHex(t, "pnpm-"+fakePNPMVersion+".tgz", "sha512")
	reference := fakePNPMVersion + "+sha512." + declared
	result, log, err := runBuildPackDescriptor(t, "scoped-valid", "pnpm@"+reference)
	if err != nil {
		t.Fatalf("BuildPack() error: %v", err)
	}
	if got := log.count(fakePNPMDistributionURL); got != 0 {
		t.Fatalf("pnpm tarball fetches = %d, want 0: a declared sha512 reconciles against registry integrity evidence", got)
	}
	distribution := result.Toolchain.Distribution
	if distribution == nil {
		t.Fatal("Corepack distribution was not captured")
	}
	if distribution.PackageManagerVer != reference {
		t.Fatalf("distribution package_manager_version = %q, want the verbatim descriptor %q", distribution.PackageManagerVer, reference)
	}
	if distribution.SHA512 != declared {
		t.Fatalf("distribution sha512 = %q, want %q", distribution.SHA512, declared)
	}
}

func TestBuildPackPNPMDeclaredSHA512Mismatch(t *testing.T) {
	declared := flipHex(fixtureDigestHex(t, "pnpm-"+fakePNPMVersion+".tgz", "sha512"))
	_, log, err := runBuildPackDescriptor(t, "scoped-valid", "pnpm@"+fakePNPMVersion+"+sha512."+declared)
	requireDigestMismatch(t, err)
	if got := log.count(fakePNPMDistributionURL); got != 0 {
		t.Fatalf("pnpm tarball fetches = %d, want 0 on the sha512 path", got)
	}
}

func TestBuildPackPNPMDeclaredSHA256Match(t *testing.T) {
	declared := fixtureDigestHex(t, "pnpm-"+fakePNPMVersion+".tgz", "sha256")
	reference := fakePNPMVersion + "+sha256." + declared
	result, log, err := runBuildPackDescriptor(t, "scoped-valid", "pnpm@"+reference)
	if err != nil {
		t.Fatalf("BuildPack() error: %v", err)
	}
	if got := log.count(fakePNPMDistributionURL); got != 1 {
		t.Fatalf("pnpm tarball fetches = %d, want 1: non-sha512 declarations reconcile over the downloaded bytes", got)
	}
	if distribution := result.Toolchain.Distribution; distribution == nil || distribution.PackageManagerVer != reference {
		t.Fatalf("distribution capture = %#v, want package_manager_version %q", distribution, reference)
	}
}

func TestBuildPackPNPMDeclaredSHA256Mismatch(t *testing.T) {
	declared := flipHex(fixtureDigestHex(t, "pnpm-"+fakePNPMVersion+".tgz", "sha256"))
	_, log, err := runBuildPackDescriptor(t, "scoped-valid", "pnpm@"+fakePNPMVersion+"+sha256."+declared)
	requireDigestMismatch(t, err)
	if got := log.count(fakePNPMDistributionURL); got != 1 {
		t.Fatalf("pnpm tarball fetches = %d, want 1: the bytes are downloaded before the mismatch is detected", got)
	}
}

func TestBuildPackYarnDeclaredSHA384Match(t *testing.T) {
	declared := fixtureDigestHex(t, "yarn-"+fakeYarnVersion+".js", "sha384")
	reference := fakeYarnVersion + "+sha384." + declared
	result, _, err := runBuildPackDescriptor(t, "yarn-valid", "yarn@"+reference)
	if err != nil {
		t.Fatalf("BuildPack() error: %v", err)
	}
	if distribution := result.Toolchain.Distribution; distribution == nil || distribution.PackageManagerVer != reference {
		t.Fatalf("distribution capture = %#v, want package_manager_version %q", distribution, reference)
	}
}

func TestBuildPackYarnDeclaredSHA384Mismatch(t *testing.T) {
	declared := flipHex(fixtureDigestHex(t, "yarn-"+fakeYarnVersion+".js", "sha384"))
	_, _, err := runBuildPackDescriptor(t, "yarn-valid", "yarn@"+fakeYarnVersion+"+sha384."+declared)
	requireDigestMismatch(t, err)
}

func TestBuildPackNPMDeclaredVersionMatch(t *testing.T) {
	result := runBuildPackFixture(t, "npm-root-valid")
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none: the declared npm version equals the toolchain npm", result.Diagnostics)
	}
}

func TestBuildPackNPMDeclaredVersionMismatch(t *testing.T) {
	for _, declared := range []string{"^10.0.0", "not-a-version"} {
		t.Run(declared, func(t *testing.T) {
			result, _, err := runBuildPackDescriptor(t, "npm-root-valid", "npm@"+declared)
			if err != nil {
				t.Fatalf("BuildPack() error: %v, want success: a declared npm version never fails the build", err)
			}
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].ID != IDNPMVersionMismatch {
				t.Fatalf("diagnostics = %#v, want exactly one %s warning", result.Diagnostics, IDNPMVersionMismatch)
			}
			if result.Diagnostics[0].Actual != nil {
				t.Fatalf("warning actual = %#v, want nil: the untrusted declaration is not embedded in the report", result.Diagnostics[0].Actual)
			}
		})
	}
}

// TestBuildPackNPMDeclaredVersionSecretShaped pins the ADR 0095
// warn-and-continue contract for token-shaped declarations: a declared npm
// version that trips the report's secret redaction rules still produces the
// warning instead of failing report construction.
func TestBuildPackNPMDeclaredVersionSecretShaped(t *testing.T) {
	result, _, err := runBuildPackDescriptor(t, "npm-root-valid", "npm@npm_11_5_1")
	if err != nil {
		t.Fatalf("BuildPack() error: %v, want success: a secret-shaped declaration warns and continues", err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].ID != IDNPMVersionMismatch {
		t.Fatalf("diagnostics = %#v, want exactly one %s warning", result.Diagnostics, IDNPMVersionMismatch)
	}
}

// TestNPMDeclaredVersionWarningPresence pins the presence semantics of ADR
// 0095: an omitted declaration carries no warning, while an explicitly empty
// devEngines version is an unparseable declaration and warns.
func TestNPMDeclaredVersionWarningPresence(t *testing.T) {
	omitted, err := npmDeclaredVersionWarning(ManagerSelection{Name: ManagerNPM}, "11.5.1")
	if err != nil || len(omitted) != 0 {
		t.Fatalf("omitted declaration: diagnostics = %#v, err = %v, want none", omitted, err)
	}
	empty, err := npmDeclaredVersionWarning(ManagerSelection{Name: ManagerNPM, DeclaredVersion: "", DeclaredVersionSet: true}, "11.5.1")
	if err != nil || len(empty) != 1 || empty[0].ID != IDNPMVersionMismatch {
		t.Fatalf("empty declaration: diagnostics = %#v, err = %v, want one %s warning", empty, err, IDNPMVersionMismatch)
	}
}

// runBuildPackDescriptor builds the fixture package after rewriting its
// packageManager field to descriptor, with the fake toolchain keyed on the
// descriptor reference. The BuildPack error is returned, not fatal.
func runBuildPackDescriptor(t *testing.T, fixture, descriptor string) (BuildPackResult, *distributionFetchLog, error) {
	t.Helper()
	name, reference, found := strings.Cut(descriptor, "@")
	if !found {
		t.Fatalf("descriptor %q lacks @", descriptor)
	}
	pnpmReference, yarnReference := fakePNPMVersion, fakeYarnVersion
	switch Manager(name) {
	case ManagerPNPM:
		pnpmReference = reference
	case ManagerYarn:
		yarnReference = reference
	}
	clearToolchainRootEnvironment(t)
	installFakeToolchainWithReferences(t, pnpmReference, yarnReference)
	repository := filepath.Join(t.TempDir(), "repository")
	source := filepath.Join(testRepositoryRoot(t), "testdata", "npm", "packages", fixture)
	if err := os.CopyFS(repository, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	declarePackageManager(t, repository, descriptor)
	selection := analyze(t, repository, ".")
	output := filepath.Join(t.TempDir(), "output")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	fetcher, log := fakeDistributionFetcher(t)
	result, err := BuildPack(ctx, BuildPackConfig{
		Selection:          selection,
		OutputDirectory:    output,
		ArtifactName:       "js-ts-npm-package-tarball-123456789-1",
		ExternalParameters: json.RawMessage(`{"test_case":"declared-digest"}`),
		fetcher:            fetcher,
	})
	return result, log, err
}

// declarePackageManager rewrites the fixture manifest's packageManager field.
func declarePackageManager(t *testing.T, repository, descriptor string) {
	t.Helper()
	manifestPath := filepath.Join(repository, "package.json")
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	descriptorJSON, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	object["packageManager"] = descriptorJSON
	encoded, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

// fixtureDigestHex computes the lowercase hex digest of a buildpack fixture
// under one of the ADR 0093 SRI algorithms.
func fixtureDigestHex(t *testing.T, name, algorithm string) string {
	t.Helper()
	payload := readFixture(t, filepath.Join(testRepositoryRoot(t), "testdata", "npm", "buildpack"), name)
	switch algorithm {
	case "sha256":
		sum := sha256.Sum256(payload)
		return hex.EncodeToString(sum[:])
	case "sha384":
		sum := sha512.Sum384(payload)
		return hex.EncodeToString(sum[:])
	case "sha512":
		sum := sha512.Sum512(payload)
		return hex.EncodeToString(sum[:])
	default:
		t.Fatalf("unsupported algorithm %q", algorithm)
		return ""
	}
}

// flipHex perturbs the first hex digit so the digest provably differs.
func flipHex(value string) string {
	if value[0] == 'a' {
		return "b" + value[1:]
	}
	return "a" + value[1:]
}

// requireDigestMismatch asserts the ADR 0092 fail-closed contract: the error
// is classified package-manager-digest-mismatch via errors.As.
func requireDigestMismatch(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("BuildPack() succeeded, want a declared-digest mismatch failure")
	}
	var identified interface{ DiagnosticID() string }
	if !errors.As(err, &identified) || identified.DiagnosticID() != IDPackageManagerDigestMismatch {
		t.Fatalf("BuildPack() error = %v, want classified %s", err, IDPackageManagerDigestMismatch)
	}
}
