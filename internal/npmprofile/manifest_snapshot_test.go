package npmprofile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeCapturesOriginalManifestSnapshot(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name             string
		packageDirectory string
		manifestPath     string
		lockfilePath     string
	}{
		{name: "root package", packageDirectory: ".", manifestPath: "package.json", lockfilePath: "package-lock.json"},
		{name: "nested package", packageDirectory: "packages/selected", manifestPath: "packages/selected/package.json", lockfilePath: "packages/selected/package-lock.json"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Given a selected manifest whose formatting and trailing newline are significant.
			original := []byte("{\n  \"name\": \"example\",\n  \"version\": \"1.0.0\",\n  \"private\": false,\n  \"packageManager\": \"npm@11.5.1\",\n  \"repository\": \"windlasstech/slsa-builder\"\n}\n")
			repository := createRepository(t, map[string]string{
				test.manifestPath: string(original),
				test.lockfilePath: `{}`,
			})

			// When analysis captures the manifest and the working tree later changes.
			selection := analyze(t, repository, test.packageDirectory)
			assertPass(t, selection)
			mutated := []byte(`{"name":"replacement","version":"9.9.9","private":true,"packageManager":"npm@11.5.1","repository":"windlasstech/slsa-builder"}`)
			if err := os.WriteFile(filepath.Join(repository, filepath.FromSlash(test.manifestPath)), mutated, 0o600); err != nil {
				t.Fatal(err)
			}

			// Then the snapshot retains the exact pre-mutation bytes and repository-relative path.
			snapshot := selection.Package.SourceManifest
			if snapshot.Path != test.manifestPath {
				t.Fatalf("source manifest path = %q, want %q", snapshot.Path, test.manifestPath)
			}
			if !bytes.Equal(snapshot.Content, original) {
				t.Fatalf("source manifest content changed after capture:\n got: %q\nwant: %q", snapshot.Content, original)
			}
		})
	}
}

func TestBuildPackUsesPreBuildManifestSnapshot(t *testing.T) {
	// Given analysis of a package that declared a build script before any tool ran.
	clearToolchainRootEnvironment(t)
	installFakeToolchain(t)
	repository := filepath.Join(t.TempDir(), "repository")
	source := filepath.Join(testRepositoryRoot(t), "testdata", "npm", "packages", "npm-root-valid")
	if err := os.CopyFS(repository, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	selection := analyze(t, repository, ".")
	assertPass(t, selection)

	// When a post-capture mutation removes the script and the build runs.
	removeBuildScript(t, repository)
	fetcher, _ := fakeDistributionFetcher(t)
	result, err := BuildPack(t.Context(), BuildPackConfig{
		Selection:          selection,
		OutputDirectory:    t.TempDir(),
		ArtifactName:       "artifact",
		ExternalParameters: []byte(`{}`),
		fetcher:            fetcher,
	})
	if err != nil {
		t.Fatalf("BuildPack() error: %v", err)
	}

	// Then detection and disposition still describe the captured manifest.
	if !result.BuildScript.Present || result.BuildScript.Result != BuildScriptExecuted {
		t.Fatalf("build script = %#v, want present and executed from the original snapshot", result.BuildScript)
	}
}

func TestResolveWorkflowPublishIntentUsesCapturedManifest(t *testing.T) {
	t.Parallel()

	// Given analysis of a public package with source publishConfig intent.
	original := []byte(`{"name":"example","version":"1.0.0","private":false,"packageManager":"npm@11.5.1","repository":"windlasstech/slsa-builder","publishConfig":{"tag":"next","access":"public"}}`)
	repository := createRepository(t, map[string]string{
		"package.json":      string(original),
		"package-lock.json": `{}`,
	})
	selection := analyze(t, repository, ".")
	assertPass(t, selection)

	// When all manifest-dependent values change on disk before publish resolution.
	mutated := []byte(`{"name":"replacement","version":"9.9.9","private":true,"packageManager":"npm@11.5.1","repository":"windlasstech/slsa-builder","publishConfig":{"tag":"mutated","access":"restricted"}}`)
	if err := os.WriteFile(filepath.Join(repository, "package.json"), mutated, 0o600); err != nil {
		t.Fatal(err)
	}
	intent, err := ResolveWorkflowPublishIntent(selection, "", "", "")
	if err != nil {
		t.Fatalf("ResolveWorkflowPublishIntent() error: %v", err)
	}

	// Then identity, privacy, and publish intent still come from the captured bytes.
	if selection.Package.Name != "example" || selection.Package.Version != "1.0.0" {
		t.Fatalf("selected identity = %s@%s, want example@1.0.0", selection.Package.Name, selection.Package.Version)
	}
	if intent.ResolvedDistTag != "next" || intent.EffectiveAccess != "public" {
		t.Fatalf("publish intent = %#v, want tag next and public access from the original snapshot", intent)
	}
	var snapshot manifest
	if err := json.Unmarshal(selection.Package.SourceManifest.Content, &snapshot); err != nil {
		t.Fatalf("decode captured manifest: %v", err)
	}
	if snapshot.Name != "example" || snapshot.Version != "1.0.0" || snapshot.Private {
		t.Fatalf("captured identity and privacy = %#v, want public example@1.0.0", snapshot)
	}
}
