package npmprofile

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestEscapeLockfileFragment(t *testing.T) {
	t.Parallel()
	// Given paths covering identity, delimiters, literal escapes, and fragment text.
	for _, test := range []struct{ path, want string }{
		{"package-lock.json", "package-lock.json"},
		{"pnpm-lock.yaml", "pnpm-lock.yaml"},
		{"yarn.lock", "yarn.lock"},
		{"packages/foo/package-lock.json", "packages/foo/package-lock.json"},
		{"packages/foo bar#100%/package-lock.json", "packages/foo%20bar%23100%25/package-lock.json"},
		{"packages/./../foo/package-lock.json", "packages/./../foo/package-lock.json"},
		{"packages/a:b@c+$&=._~/package-lock.json", "packages/a:b@c+$&=._~/package-lock.json"},
		{"packages/! '()*;,?/package-lock.json", "packages/%21%20%27%28%29%2A%3B%2C%3F/package-lock.json"},
		{"packages/%2F/package-lock.json", "packages/%252F/package-lock.json"},
		{"packages//foo/package-lock.json", "packages//foo/package-lock.json"},
	} {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()
			// When encoding each segment without cleaning the path.
			got := escapeLockfileFragment(test.path)
			// Then the spelling is exact and percent-decoding preserves the input.
			if got != test.want {
				t.Fatalf("fragment = %q, want %q", got, test.want)
			}
			decoded, err := url.PathUnescape(got)
			if err != nil || decoded != test.path {
				t.Fatalf("decoded fragment = %q, error = %v, want %q", decoded, err, test.path)
			}
		})
	}
}

func TestWorkflowResolvedDependenciesEscapesLockfileFragment(t *testing.T) {
	t.Parallel()
	// Given a selected logical path whose manager root contains reserved characters.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	selection := Result{
		Package: Package{RealManagerRoot: root, ManagerRoot: "packages/foo bar#100%"},
		Manager: ManagerSelection{Name: ManagerNPM, Source: SelectionPackageManager,
			SelectionManifestPath: "packages/foo bar#100%/package.json",
			SelectedLockfilePath:  "packages/foo bar#100%/package-lock.json"},
	}
	parameters := validExternalParameters(ManagerNPM)

	// When producing the resolved dependencies.
	dependencies, err := workflowResolvedDependencies(selection, BuildPackResult{}, parameters)

	// Then the URI contains the encoded path, not raw fragment delimiters.
	if err != nil {
		t.Fatal(err)
	}
	want := "git+https://github.com/example/project@" + testSourceSHA + "#packages/foo%20bar%23100%25/package-lock.json"
	if got := dependencies[0].URI; got != want {
		t.Fatalf("lockfile URI = %q, want %q", got, want)
	}
}

func TestValidateLockfileDescriptorRequiresEncodedFragment(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, fragment string
		accepted       bool
	}{
		{"encoded", "packages/foo%20bar%23100%25/package-lock.json", true},
		{"raw", "packages/foo bar#100%/package-lock.json", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given matching metadata and annotations, with only the URI spelling varied.
			parameters := validExternalParameters(ManagerNPM)
			parameters.PackageManager.Root = "packages/foo bar#100%"
			manifest := "packages/foo bar#100%/package.json"
			parameters.PackageManager.SelectionManifestPath = &manifest
			descriptor := validDependencies(ManagerNPM, parameters)[0]
			descriptor.URI = "git+https://github.com/example/project@" + testSourceSHA + "#" + test.fragment
			descriptor.Annotations["package_manager_root"] = mustJSON(parameters.PackageManager.Root)
			descriptor.Annotations["selection_manifest_path"] = mustJSON(manifest)
			descriptor.Annotations["selection_lockfile_path"] = mustJSON("packages/foo bar#100%/package-lock.json")

			// When validating the descriptor without canonicalizing its URI.
			err := validateLockfileDescriptor(descriptor, parameters)

			// Then only the encoded spelling is accepted.
			if test.accepted {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				requireNPMDiagnostic(t, err, IDResolvedDependenciesLockfile)
			}
		})
	}
}
