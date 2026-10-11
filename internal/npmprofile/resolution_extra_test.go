package npmprofile

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolvePackageRepositoryMismatch pins the identity-comparison failure:
// a well-formed, canonicalizable package repository that names a different
// repository than the observed caller repository fails closed with
// package-repository-identity-mismatch.
func TestResolvePackageRepositoryMismatch(t *testing.T) {
	t.Parallel()
	root := createRepository(t, map[string]string{
		"package.json":      `{"name":"example","version":"1.0.0","packageManager":"npm@11.5.1","repository":"otherorg/other-project"}`,
		"package-lock.json": `{}`,
	})
	result := analyze(t, root, ".")
	assertRejected(t, result, IDPackageRepositoryIdentityMismatch)
}

// TestResolveSelectedDirectoryRejections pins the filesystem-resolution
// failure forms of resolveSelectedDirectory: every unresolvable selection is
// the closed package-resolution-invalid rejection, never a panic or a
// silently accepted path.
func TestResolveSelectedDirectoryRejections(t *testing.T) {
	t.Parallel()
	manifest := `{"name":"example","version":"1.0.0","packageManager":"npm@11.5.1","repository":"windlasstech/slsa-builder"}`

	t.Run("repository root missing", func(t *testing.T) {
		t.Parallel()
		result := analyze(t, filepath.Join(t.TempDir(), "missing"), ".")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})

	t.Run("selected directory missing", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{"package.json": manifest, "package-lock.json": `{}`})
		result := analyze(t, root, "missing")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})

	t.Run("case-mismatched segment", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{
			"real/package.json": manifest,
			"package.json":      `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"npm@11.5.1","workspaces":["real"]}`,
			"package-lock.json": `{}`,
		})
		result := analyze(t, root, "REAL")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})

	t.Run("intermediate component not a directory", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{"file.txt": "not a directory"})
		result := analyze(t, root, "file.txt/deeper")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})

	t.Run("selected path is a file", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{"file.txt": "not a directory"})
		result := analyze(t, root, "file.txt")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})

	t.Run("selected path is a dangling symlink", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, nil)
		if err := os.Symlink(filepath.Join(root, "missing-target"), filepath.Join(root, "dangling")); err != nil {
			t.Fatal(err)
		}
		result := analyze(t, root, "dangling")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})
}

// TestReadManifestRejections pins the two readManifest decode failures beyond
// a missing or symlinked manifest: syntactically invalid JSON and strict JSON
// whose field types violate the manifest shape both reject as
// package-manifest-invalid.
func TestReadManifestRejections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		manifest string
	}{
		{name: "syntactically invalid JSON", manifest: `{"name":"example",`},
		{name: "wrong-typed name field", manifest: `{"name":123,"version":"1.0.0","repository":"windlasstech/slsa-builder"}`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := createRepository(t, map[string]string{"package.json": test.manifest})
			result := analyze(t, root, ".")
			assertRejected(t, result, IDPackageManifestInvalid)
		})
	}
}

// TestNormalizeRepositoryRejections pins the closed repository-identity
// grammar: the repository member is required, and only a string or a
// two-to-three-member {type:"git",url,directory?} object survives; every
// other shape fails closed with package-repository-identity-mismatch.
func TestNormalizeRepositoryRejections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		repository string
	}{
		{name: "repository member missing", repository: ""},
		{name: "repository not a string or object", repository: `,"repository":42`},
		{name: "object with too few members", repository: `,"repository":{"type":"git"}`},
		{name: "object with an unknown member", repository: `,"repository":{"type":"git","url":"windlasstech/slsa-builder","bogus":true}`},
		{name: "object type not git", repository: `,"repository":{"type":"svn","url":"windlasstech/slsa-builder"}`},
		{name: "object type not a string", repository: `,"repository":{"type":42,"url":"windlasstech/slsa-builder"}`},
		{name: "object url not a string", repository: `,"repository":{"type":"git","url":42}`},
		{name: "object directory not a string", repository: `,"repository":{"type":"git","url":"windlasstech/slsa-builder","directory":42}`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := createRepository(t, map[string]string{
				"package.json": `{"name":"example","version":"1.0.0","packageManager":"npm@11.5.1"` + test.repository + `}`,
			})
			result := analyze(t, root, ".")
			assertRejected(t, result, IDPackageRepositoryIdentityMismatch)
		})
	}
}
