package npmprofile

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

func TestBuildPackNPM(t *testing.T) {
	t.Run("declared build", func(t *testing.T) {
		result := runBuildPackFixture(t, "npm-root-valid")
		assertBuildPackResult(t, result, ManagerNPM, "pkg:npm/windlass-fixture-unscoped@1.0.0")
		if !strings.HasPrefix(result.Toolchain.NodeVersion, "v24.") {
			t.Fatalf("Node.js version = %q, want Node.js 24", result.Toolchain.NodeVersion)
		}
	})
	t.Run("absent build is explicit no-op", func(t *testing.T) {
		result := runBuildPackFixtureWithSetup(t, "npm-root-valid", removeBuildScript)
		if result.BuildScript.Present || result.BuildScript.Result != BuildScriptSkippedAbsent {
			t.Fatalf("build script = %#v", result.BuildScript)
		}
	})
}

func TestBuildPackPNPM(t *testing.T) {
	result := runBuildPackFixture(t, "scoped-valid")
	assertBuildPackResult(t, result, ManagerPNPM, "pkg:npm/%40windlass-fixtures/scoped@1.0.0")
	if result.Toolchain.CorepackVersion == "" {
		t.Fatal("Corepack version was not captured")
	}
	assertDistributionCapture(t, result.Toolchain.Distribution, ManagerPNPM, "10.14.0", "registry-integrity")
}

func TestBuildPackYarn(t *testing.T) {
	result := runBuildPackFixture(t, "yarn-valid")
	assertBuildPackResult(t, result, ManagerYarn, "pkg:npm/windlass-fixture-yarn@1.0.0")
	if result.Toolchain.CorepackVersion == "" {
		t.Fatal("Corepack version was not captured")
	}
	assertDistributionCapture(t, result.Toolchain.Distribution, ManagerYarn, "4.9.2", "download-hash")
}

func TestPackedMetadata(t *testing.T) {
	result := runBuildPackFixture(t, "npm-root-valid")
	if result.Packed.Name != result.PackageName || result.Packed.Version != result.PackageVersion {
		t.Fatalf("packed identity = %s@%s, source = %s@%s", result.Packed.Name, result.Packed.Version, result.PackageName, result.PackageVersion)
	}
	if !contains(result.Packed.Files, "package/package.json") || !contains(result.Packed.Files, "package/dist/index.js") {
		t.Fatalf("packed files = %#v", result.Packed.Files)
	}
}

func TestBuildMetadata(t *testing.T) {
	result := runBuildPackFixture(t, "npm-root-valid")
	encoded, err := os.ReadFile(result.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata BuildMetadata
	if err := json.Unmarshal(encoded, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.SchemaVersion != "1" || metadata.PrimaryArtifact.PayloadFileName != filepath.Base(result.TarballPath) {
		t.Fatalf("build metadata = %#v", metadata)
	}
	if metadata.PrimaryArtifact.SHA256 != result.SHA256.String() || metadata.PrimaryArtifact.SHA512 != result.SHA512.String() {
		t.Fatalf("metadata digests = %#v", metadata.PrimaryArtifact)
	}
	if len(metadata.ResolvedDependencies) != 0 {
		t.Fatalf("resolved dependencies = %#v, want empty test input", metadata.ResolvedDependencies)
	}
	if string(metadata.ExternalParameters) != `{"test_case":"real-tool-build-pack"}` {
		t.Fatalf("external parameters = %s", metadata.ExternalParameters)
	}
}

func runBuildPackFixture(t *testing.T, fixture string) BuildPackResult {
	t.Helper()
	return runBuildPackFixtureWithSetup(t, fixture, nil)
}

func runBuildPackFixtureWithSetup(t *testing.T, fixture string, setup func(*testing.T, string)) BuildPackResult {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "repository")
	source := filepath.Join(testRepositoryRoot(t), "testdata", "npm", "packages", fixture)
	if err := os.CopyFS(repository, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	if setup != nil {
		setup(t, repository)
	}
	selection := analyze(t, repository, ".")
	output := filepath.Join(t.TempDir(), "output")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	result, err := BuildPack(ctx, BuildPackConfig{
		Selection:          selection,
		OutputDirectory:    output,
		ArtifactName:       "js-ts-npm-package-tarball-123456789-1",
		ExternalParameters: json.RawMessage(`{"test_case":"real-tool-build-pack"}`),
	})
	if err != nil {
		t.Fatalf("BuildPack() error: %v", err)
	}
	assertOneArtifact(t, output, ".tgz")
	assertOneArtifact(t, output, "build-metadata.json")
	return result
}

func removeBuildScript(t *testing.T, repository string) {
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
	delete(object, "scripts")
	encoded, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertBuildPackResult(t *testing.T, result BuildPackResult, manager Manager, wantPURL string) {
	t.Helper()
	t.Logf("manager=%s version=%s node=%s npm=%s corepack=%s tarball=%s sha256=%s sha512=%s metadata=%s", result.Manager, result.Toolchain.PackageManagerVersion, result.Toolchain.NodeVersion, result.Toolchain.NPMVersion, result.Toolchain.CorepackVersion, filepath.Base(result.TarballPath), result.SHA256, result.SHA512, filepath.Base(result.MetadataPath))
	if result.Manager != manager || result.Toolchain.PackageManagerVersion == "" {
		t.Fatalf("tool capture = %#v", result.Toolchain)
	}
	if result.PackagePURL != wantPURL {
		t.Fatalf("PURL = %q, want %q", result.PackagePURL, wantPURL)
	}
	if result.SHA256.String() == strings.Repeat("0", 64) || result.SHA512.String() == strings.Repeat("0", 128) {
		t.Fatal("tarball digest is zero")
	}
	tarball, err := os.ReadFile(result.TarballPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.SHA256 != digest.SumSHA256(tarball) || result.SHA512 != digest.SumSHA512(tarball) {
		t.Fatal("tarball digests do not match the immutable packed bytes")
	}
	if !result.BuildScript.Present || result.BuildScript.Result != BuildScriptExecuted {
		t.Fatalf("build script = %#v", result.BuildScript)
	}
}

func assertDistributionCapture(t *testing.T, capture *DistributionCapture, manager Manager, version, authority string) {
	t.Helper()
	if capture == nil {
		t.Fatal("Corepack distribution was not captured")
	}
	wantURL := "https://registry.npmjs.org/pnpm/-/pnpm-" + version + ".tgz"
	if manager == ManagerYarn {
		wantURL = "https://repo.yarnpkg.com/" + version + "/packages/yarnpkg-cli/bin/yarn.js"
	}
	if capture.URL != wantURL || capture.DigestAuthority != authority || len(capture.SHA512) != 128 ||
		capture.PackageManager != manager || capture.PackageManagerVer != version || capture.AcquisitionSource != "corepack" {
		t.Fatalf("distribution capture = %#v", capture)
	}
}

func assertOneArtifact(t *testing.T, root, suffix string) {
	t.Helper()
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && (filepath.Base(path) == suffix || strings.HasSuffix(entry.Name(), suffix)) {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("artifact count for %q = %d, want 1", suffix, count)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
