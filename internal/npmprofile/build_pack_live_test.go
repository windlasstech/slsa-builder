package npmprofile

// Gated live integration tests: these exercise the real package-manager
// toolchain and real network (Corepack acquisition, registry fetches,
// npm/pnpm/yarn install + pack). They preserve the real-toolchain coverage the
// BuildPack tests had before their hermetic conversion. They never run in CI
// and never run by default: each test skips under `go test -short` and unless
// SLSA_BUILDER_LIVE_TOOLCHAIN=1 is set (see docs/testing-guide.md).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// requireLiveToolchain double-gates live tests: skip in -short mode and skip
// unless the operator explicitly opts in with SLSA_BUILDER_LIVE_TOOLCHAIN=1.
func requireLiveToolchain(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("live toolchain test skipped in -short mode: set SLSA_BUILDER_LIVE_TOOLCHAIN=1 to run")
	}
	if os.Getenv("SLSA_BUILDER_LIVE_TOOLCHAIN") != "1" {
		t.Skip("live toolchain test: set SLSA_BUILDER_LIVE_TOOLCHAIN=1 to run")
	}
}

func TestBuildPackNPMLive(t *testing.T) {
	requireLiveToolchain(t)
	result := runBuildPackFixtureLive(t, "npm-root-valid")
	assertBuildPackResult(t, result, ManagerNPM, "pkg:npm/windlass-fixture-unscoped@1.0.0")
	if result.Toolchain.NodeVersion == "" || result.Toolchain.NPMVersion == "" {
		t.Fatalf("tool capture = %#v", result.Toolchain)
	}
}

func TestBuildPackPNPMLive(t *testing.T) {
	requireLiveToolchain(t)
	result := runBuildPackFixtureLive(t, "scoped-valid")
	assertBuildPackResult(t, result, ManagerPNPM, "pkg:npm/%40windlass-fixtures/scoped@1.0.0")
	if result.Toolchain.CorepackVersion == "" {
		t.Fatal("Corepack version was not captured")
	}
	assertDistributionCapture(t, result.Toolchain.Distribution, ManagerPNPM, "10.14.0", "registry-integrity")
}

func TestBuildPackYarnLive(t *testing.T) {
	requireLiveToolchain(t)
	result := runBuildPackFixtureLive(t, "yarn-valid")
	assertBuildPackResult(t, result, ManagerYarn, "pkg:npm/windlass-fixture-yarn@1.0.0")
	if result.Toolchain.CorepackVersion == "" {
		t.Fatal("Corepack version was not captured")
	}
	assertDistributionCapture(t, result.Toolchain.Distribution, ManagerYarn, "4.9.2", "download-hash")
}

// runBuildPackFixtureLive mirrors runBuildPackFixtureWithSetup but runs the
// real toolchain: no fake toolchain on PATH and no distributionFetcher
// override, so BuildPack defaults to fetchHTTPS and the ambient node/npm/
// corepack installation.
func runBuildPackFixtureLive(t *testing.T, fixture string) BuildPackResult {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "repository")
	source := filepath.Join(testRepositoryRoot(t), "testdata", "npm", "packages", fixture)
	if err := os.CopyFS(repository, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	selection := analyze(t, repository, ".")
	output := filepath.Join(t.TempDir(), "output")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	result, err := BuildPack(ctx, BuildPackConfig{
		Selection:          selection,
		OutputDirectory:    output,
		ArtifactName:       "js-ts-npm-package-tarball-123456789-1",
		ExternalParameters: json.RawMessage(`{"test_case":"live-toolchain-build-pack"}`),
	})
	if err != nil {
		t.Fatalf("BuildPack() error: %v", err)
	}
	assertOneArtifact(t, output, ".tgz")
	assertOneArtifact(t, output, "build-metadata.json")
	return result
}
