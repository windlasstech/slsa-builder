package npmprofile

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSelectManagerNameConflict pins the cross-manifest name conflict: when
// the selected package and the workspace root declare different package
// managers, selection fails closed with package-manager-conflict.
func TestSelectManagerNameConflict(t *testing.T) {
	t.Parallel()
	root := createRepository(t, map[string]string{
		"package.json":            `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"pnpm@11.28.3","workspaces":["packages/*"]}`,
		"pnpm-lock.yaml":          "lockfileVersion: '9.0'\n",
		"packages/a/package.json": `{"name":"a","version":"1.0.0","repository":"windlasstech/slsa-builder","devEngines":{"packageManager":{"name":"yarn","version":"4.9.2"}}}`,
	})
	result := analyze(t, root, "packages/a")
	assertRejected(t, result, IDPackageManagerConflict)
}

// TestParsePackageManagerRejections pins the per-manager classification of a
// versionless packageManager descriptor: yarn keeps yarn-selection-invalid,
// pnpm keeps package-manager-version-required, and every other versionless
// or unclassifiable descriptor is a package-manager conflict.
func TestParsePackageManagerRejections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		declaration string
		wantID      string
	}{
		{name: "yarn without version", declaration: `"packageManager":"yarn"`, wantID: IDYarnSelectionInvalid},
		{name: "pnpm without version", declaration: `"packageManager":"pnpm"`, wantID: IDPackageManagerVersionRequired},
		{name: "npm without version", declaration: `"packageManager":"npm"`, wantID: IDPackageManagerConflict},
		{name: "empty manager name", declaration: `"packageManager":"@11.0.0"`, wantID: IDPackageManagerConflict},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := createRepository(t, map[string]string{
				"package.json": `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + test.declaration + `}`,
			})
			result := analyze(t, root, ".")
			assertRejected(t, result, test.wantID)
		})
	}
}

// TestParseDevEnginesRejections pins the closed devEngines.packageManager
// shape: a non-object devEngines, a non-object packageManager member, unknown
// members, a missing or empty name, a non-string version, and an out-of-set
// onFail all fail closed with package-manager-conflict.
func TestParseDevEnginesRejections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		devEngine string
	}{
		{name: "devEngines not an object", devEngine: `"pnpm"`},
		{name: "packageManager not an object", devEngine: `{"packageManager":"pnpm@11.0.0"}`},
		{name: "unknown member", devEngine: `{"packageManager":{"name":"pnpm","version":"11.0.0","bogus":true}}`},
		{name: "name missing", devEngine: `{"packageManager":{"version":"11.0.0"}}`},
		{name: "name empty", devEngine: `{"packageManager":{"name":""}}`},
		{name: "name not a string", devEngine: `{"packageManager":{"name":42}}`},
		{name: "version not a string", devEngine: `{"packageManager":{"name":"pnpm","version":42}}`},
		{name: "onFail outside the closed set", devEngine: `{"packageManager":{"name":"pnpm","version":"11.0.0","onFail":"explode"}}`},
		{name: "onFail not a string", devEngine: `{"packageManager":{"name":"pnpm","version":"11.0.0","onFail":42}}`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := createRepository(t, map[string]string{
				"package.json": `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder","devEngines":` + test.devEngine + `}`,
			})
			result := analyze(t, root, ".")
			assertRejected(t, result, IDPackageManagerConflict)
		})
	}
}

// TestInferFromLockfileRejections pins the lockfile-inference failure forms:
// without manifest declarations, zero or multiple present lockfiles (or an
// unreadable lockfile set) leave the manager ambiguous and fail closed with
// package-manager-conflict.
func TestInferFromLockfileRejections(t *testing.T) {
	t.Parallel()
	manifest := `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder"}`

	t.Run("no lockfile", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{"package.json": manifest})
		result := analyze(t, root, ".")
		assertRejected(t, result, IDPackageManagerConflict)
	})

	t.Run("multiple lockfiles", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{
			"package.json":      manifest,
			"package-lock.json": `{}`,
			"pnpm-lock.yaml":    "lockfileVersion: '9.0'\n",
		})
		result := analyze(t, root, ".")
		assertRejected(t, result, IDPackageManagerConflict)
	})

	t.Run("lockfile not regular", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{"package.json": manifest})
		outside := createRepository(t, map[string]string{"package-lock.json": `{}`})
		if err := os.Symlink(filepath.Join(outside, "package-lock.json"), filepath.Join(root, "package-lock.json")); err != nil {
			t.Fatal(err)
		}
		result := analyze(t, root, ".")
		assertRejected(t, result, IDPackageManagerConflict)
	})
}

// TestValidateLockfilesWithoutManifestSelection pins the inference-side
// lockfile check: when no manifest declaration selected the manager, a
// present required lockfile resolves the selection without stale-lockfile
// warnings.
func TestValidateLockfilesWithoutManifestSelection(t *testing.T) {
	t.Parallel()
	root := createRepository(t, map[string]string{"package-lock.json": `{}`})
	resolved := resolvedPackage{managerRootReal: root, managerRoot: "."}
	selection, diagnostics, failureID := validateLockfiles(resolved, ManagerSelection{Name: ManagerNPM}, false)
	if failureID != "" {
		t.Fatalf("failureID = %q, want none", failureID)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if selection.SelectedLockfilePath != "package-lock.json" {
		t.Fatalf("selected lockfile = %q, want %q", selection.SelectedLockfilePath, "package-lock.json")
	}
	if len(selection.IgnoredLockfilePaths) != 0 {
		t.Fatalf("ignored lockfiles = %#v, want none", selection.IgnoredLockfilePaths)
	}
}

// TestPresentLockfilesRootNotDirectory pins the non-ENOENT stat failure: a
// manager root that cannot be enumerated surfaces an error rather than an
// empty lockfile set.
func TestPresentLockfilesRootNotDirectory(t *testing.T) {
	t.Parallel()
	root := createRepository(t, map[string]string{"file.txt": "not a directory"})
	if _, err := presentLockfiles(filepath.Join(root, "file.txt")); err == nil {
		t.Fatal("presentLockfiles() error = nil, want an enumeration failure")
	}
}

// TestRejectionMessageFallback pins the closed rejection taxonomy: an ID
// outside the registered selection rejections renders the generic fallback
// message.
func TestRejectionMessageFallback(t *testing.T) {
	t.Parallel()
	if got := rejectionMessage("windlass.verify.error.unregistered-test"); got != "The npm profile selection policy rejected the package." {
		t.Fatalf("rejectionMessage() = %q, want the fallback message", got)
	}
}

// TestManagerForLockfileUnknown pins the closed lockfile mapping: an
// unrecognized lockfile name selects no manager.
func TestManagerForLockfileUnknown(t *testing.T) {
	t.Parallel()
	if got := managerForLockfile("Gemfile.lock"); got != "" {
		t.Fatalf("managerForLockfile() = %q, want empty", got)
	}
}
