package npmprofile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/fixture"
)

const observedRepository = "https://github.com/windlasstech/slsa-builder"

func TestPackageResolution(t *testing.T) {
	t.Parallel()
	repositoryRoot := filepath.Join(testRepositoryRoot(t), "testdata", "npm", "packages", "npm-root-valid")

	result := analyze(t, repositoryRoot, "./")
	assertPass(t, result)
	if result.Package.Directory != "." {
		t.Fatalf("package directory = %q", result.Package.Directory)
	}
	if result.Package.Repository != observedRepository {
		t.Fatalf("repository = %q", result.Package.Repository)
	}

	for _, packageDirectory := range []string{"", "../outside", "/tmp/outside", `testdata\npm\packages\npm-root-valid`} {
		packageDirectory := packageDirectory
		t.Run("reject "+packageDirectory, func(t *testing.T) {
			t.Parallel()
			result, err := Analyze(Config{
				RepositoryRoot:     repositoryRoot,
				PackageDirectory:   packageDirectory,
				ObservedRepository: observedRepository,
			})
			if err != nil {
				t.Fatalf("Analyze() internal error: %v", err)
			}
			assertRejected(t, result, IDPackageResolutionInvalid)
		})
	}

	t.Run("repository normalization", func(t *testing.T) {
		t.Parallel()
		for _, repository := range []string{
			`"WindlassTech/SLSA-Builder"`,
			`"github:WindlassTech/SLSA-Builder"`,
			`"git+https://github.com/WindlassTech/SLSA-Builder.git"`,
			`"git@github.com:WindlassTech/SLSA-Builder.git"`,
			`{"type":"git","url":"ssh://git@github.com/WindlassTech/SLSA-Builder.git","directory":"packages/example"}`,
		} {
			root := createRepository(t, map[string]string{
				"package.json":      `{"name":"example","version":"1.0.0","packageManager":"npm@11.5.1","repository":` + repository + `}`,
				"package-lock.json": `{}`,
			})
			result, err := Analyze(Config{RepositoryRoot: root, PackageDirectory: ".", ObservedRepository: "windlasstech/slsa-builder"})
			if err != nil {
				t.Fatal(err)
			}
			assertPass(t, result)
		}
	})

	t.Run("repository credential rejection", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{
			"package.json":      `{"name":"example","version":"1.0.0","packageManager":"npm@11.5.1","repository":"https://token@github.com/windlasstech/slsa-builder"}`,
			"package-lock.json": `{}`,
		})
		result := analyze(t, root, ".")
		assertRejected(t, result, IDPackageRepositoryIdentityMismatch)
	})

	t.Run("symlink escape", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, nil)
		outside := createRepository(t, map[string]string{"package.json": `{}`})
		if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
			t.Fatal(err)
		}
		result := analyze(t, root, "outside")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})

	t.Run("manifest symlink escape", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{"package-lock.json": `{}`})
		outside := createRepository(t, map[string]string{
			"package.json": `{"name":"outside","version":"1.0.0","packageManager":"npm@11.5.1","repository":"windlasstech/slsa-builder"}`,
		})
		if err := os.Symlink(filepath.Join(outside, "package.json"), filepath.Join(root, "package.json")); err != nil {
			t.Fatal(err)
		}
		result := analyze(t, root, ".")
		assertRejected(t, result, IDPackageManifestInvalid)
	})
}

func TestWorkspaceDiscovery(t *testing.T) {
	t.Parallel()
	result := analyzeFixture(t, testRepositoryRoot(t), "testdata/npm/packages/workspace-valid/packages/selected")
	assertPass(t, result)

	if result.Package.ManagerRoot != "." {
		t.Fatalf("manager root = %q", result.Package.ManagerRoot)
	}
	if result.Package.ManagerRootRelativeDirectory != "packages/selected" {
		t.Fatalf("manager-root-relative directory = %q", result.Package.ManagerRootRelativeDirectory)
	}
	if result.Manager.SelectionManifestPath != "package.json" {
		t.Fatalf("selection manifest = %q", result.Manager.SelectionManifestPath)
	}
	if result.Manager.Name != ManagerNPM || result.Manager.Source != SelectionPackageManager {
		t.Fatalf("manager selection = %#v", result.Manager)
	}

	t.Run("pnpm settings-only root", func(t *testing.T) {
		t.Parallel()
		result := analyzeFixture(t, testRepositoryRoot(t), "testdata/npm/packages/pnpm-settings-only-valid")
		assertPass(t, result)
		if result.Package.Directory != "." || result.Package.ManagerRoot != "." {
			t.Fatalf("standalone root package = %#v", result.Package)
		}
	})

	t.Run("pnpm settings-only subdirectory fails closed", func(t *testing.T) {
		t.Parallel()
		result := analyzeFixture(t, testRepositoryRoot(t), "testdata/npm/packages/rejected/pnpm-settings-only-subdirectory/packages/undeclared")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})

	t.Run("pnpm recursive pattern", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{
			"package.json":                   `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"pnpm@11.28.3"}`,
			"pnpm-workspace.yaml":            "packages:\n  - packages/**\nsharedWorkspaceLockfile: true\n",
			"pnpm-lock.yaml":                 "lockfileVersion: '9.0'\n",
			"packages/nested/a/package.json": `{"name":"a","version":"1.0.0","repository":"windlasstech/slsa-builder"}`,
		})
		result := analyze(t, root, "packages/nested/a")
		assertPass(t, result)
		if result.Package.ManagerRoot != "." || result.Package.ManagerRootRelativeDirectory != "packages/nested/a" {
			t.Fatalf("workspace package = %#v", result.Package)
		}
	})

	t.Run("malformed pattern fails closed", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{
			"package.json":            `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"npm@11.5.1","workspaces":["packages/{a,b}"]}`,
			"package-lock.json":       `{}`,
			"packages/a/package.json": `{"name":"a","version":"1.0.0","repository":"windlasstech/slsa-builder"}`,
		})
		result := analyze(t, root, "packages/a")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})

	t.Run("workspace metadata symlink escape", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{
			"package.json":            `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"pnpm@11.28.3"}`,
			"pnpm-lock.yaml":          "lockfileVersion: '9.0'\n",
			"packages/a/package.json": `{"name":"a","version":"1.0.0","repository":"windlasstech/slsa-builder"}`,
		})
		outside := createRepository(t, map[string]string{"pnpm-workspace.yaml": "packages:\n  - packages/*\n"})
		if err := os.Symlink(filepath.Join(outside, "pnpm-workspace.yaml"), filepath.Join(root, "pnpm-workspace.yaml")); err != nil {
			t.Fatal(err)
		}
		result := analyze(t, root, "packages/a")
		assertRejected(t, result, IDPackageResolutionInvalid)
	})
}

func TestManagerSelection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		packageDirectory string
		manager          Manager
		version          string
	}{
		{name: "npm", packageDirectory: "testdata/npm/packages/npm-root-valid", manager: ManagerNPM, version: ""},
		{name: "pnpm", packageDirectory: "testdata/npm/packages/scoped-valid", manager: ManagerPNPM, version: "11.28.3"},
		{name: "pnpm devEngines", packageDirectory: "testdata/npm/packages/pnpm-devengines-valid", manager: ManagerPNPM, version: "11.28.3"},
		{name: "pnpm 10", packageDirectory: "testdata/npm/packages/pnpm-10-valid", manager: ManagerPNPM, version: "10.34.6"},
		{name: "pnpm 10 devEngines", packageDirectory: "testdata/npm/packages/pnpm-10-devengines-valid", manager: ManagerPNPM, version: "10.34.6"},
		{name: "yarn", packageDirectory: "testdata/npm/packages/yarn-valid", manager: ManagerYarn, version: "4.9.2"},
		{name: "yarn devEngines", packageDirectory: "testdata/npm/packages/yarn-devengines-valid", manager: ManagerYarn, version: "4.9.2"},
		{name: "yarn 5", packageDirectory: "testdata/npm/packages/yarn-5-valid", manager: ManagerYarn, version: "5.0.0"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeFixture(t, testRepositoryRoot(t), test.packageDirectory)
			assertPass(t, result)
			if result.Manager.Name != test.manager || result.Manager.Version != test.version {
				t.Fatalf("manager = %#v, want %s@%s", result.Manager, test.manager, test.version)
			}
		})
	}

	t.Run("pnpm version conflict", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{
			"package.json":            `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"pnpm@11.28.3","workspaces":["packages/*"]}`,
			"pnpm-lock.yaml":          "lockfileVersion: '9.0'\n",
			"packages/a/package.json": `{"name":"a","version":"1.0.0","repository":"windlasstech/slsa-builder","packageManager":"pnpm@11.0.0"}`,
		})
		result := analyze(t, root, "packages/a")
		assertRejected(t, result, IDPackageManagerConflict)
	})
}

func TestLockfileRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		packageDirectory string
		selected         string
		ignored          []string
	}{
		{
			name:             "npm stale pnpm lockfile",
			packageDirectory: "testdata/npm/packages/stale-lockfiles/npm",
			selected:         "package-lock.json",
			ignored:          []string{"pnpm-lock.yaml"},
		},
		{
			name:             "pnpm stale npm lockfile",
			packageDirectory: "testdata/npm/packages/stale-lockfiles/pnpm",
			selected:         "pnpm-lock.yaml",
			ignored:          []string{"package-lock.json"},
		},
		{
			name:             "yarn stale npm lockfile",
			packageDirectory: "testdata/npm/packages/stale-lockfiles/yarn",
			selected:         "yarn.lock",
			ignored:          []string{"package-lock.json"},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeFixture(t, testRepositoryRoot(t), test.packageDirectory)
			assertPass(t, result)
			if result.Manager.SelectedLockfilePath != test.selected {
				t.Fatalf("selected lockfile = %q", result.Manager.SelectedLockfilePath)
			}
			if !reflect.DeepEqual(result.Manager.IgnoredLockfilePaths, test.ignored) {
				t.Fatalf("ignored lockfiles = %#v, want %#v", result.Manager.IgnoredLockfilePaths, test.ignored)
			}
			if len(result.Report.Diagnostics) != 1 || result.Report.Diagnostics[0].ID != diagnostic.IDStaleNonSelectedLockfile {
				t.Fatalf("diagnostics = %#v", result.Report.Diagnostics)
			}
			if result.Report.Diagnostics[0].Field != "externalParameters.package_manager.ignored_lockfile_paths" {
				t.Fatalf("warning field = %q", result.Report.Diagnostics[0].Field)
			}
		})
	}

	for _, fixture := range loadRejectedFixtures(t) {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			result := analyzeFixture(t, testRepositoryRoot(t), filepath.ToSlash(filepath.Dir(fixture.Artifact)))
			assertRejected(t, result, fixture.ExpectedPrimaryID)
		})
	}

	t.Run("lockfile symlink escape", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{
			"package.json": `{"name":"example","version":"1.0.0","packageManager":"npm@11.5.1","repository":"windlasstech/slsa-builder"}`,
		})
		outside := createRepository(t, map[string]string{"package-lock.json": `{}`})
		if err := os.Symlink(filepath.Join(outside, "package-lock.json"), filepath.Join(root, "package-lock.json")); err != nil {
			t.Fatal(err)
		}
		result := analyze(t, root, ".")
		assertRejected(t, result, IDRequiredLockfileMissing)
	})
}

func TestYarnV4(t *testing.T) {
	t.Parallel()
	valid := analyzeFixture(t, testRepositoryRoot(t), "testdata/npm/packages/yarn-valid")
	assertPass(t, valid)
	if valid.Manager.Name != ManagerYarn || valid.Manager.Version != "4.9.2" {
		t.Fatalf("valid Yarn selection = %#v", valid.Manager)
	}

	for _, test := range []struct {
		name             string
		packageDirectory string
	}{
		{name: "classic", packageDirectory: "testdata/npm/packages/rejected/yarn-classic"},
		{name: "lockfile only", packageDirectory: "testdata/npm/packages/rejected/yarn-lockfile-only"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeFixture(t, testRepositoryRoot(t), test.packageDirectory)
			assertRejected(t, result, IDYarnSelectionInvalid)
		})
	}
}

func TestPackageManagerVersionBounds(t *testing.T) {
	t.Parallel()

	t.Run("pnpm 10.x and 11.x accepted from both manifest sources", func(t *testing.T) {
		t.Parallel()
		for _, version := range []string{"10.0.0", "10.14.0", "10.34.6", "10.1.0-alpha.1", "11.0.0", "11.28.3", "11.1.0-alpha.1"} {
			for _, declaration := range []string{
				`"packageManager":"pnpm@` + version + `"`,
				`"devEngines":{"packageManager":{"name":"pnpm","version":"` + version + `"}}`,
			} {
				root := createRepository(t, map[string]string{
					"package.json":   `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + declaration + `}`,
					"pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
				})
				result := analyze(t, root, ".")
				assertPass(t, result)
				if result.Manager.Name != ManagerPNPM || result.Manager.Version != version {
					t.Fatalf("manager = %#v, want pnpm@%s", result.Manager, version)
				}
			}
		}
	})

	t.Run("pnpm outside the 10.x and 11.x lines rejected from both manifest sources", func(t *testing.T) {
		t.Parallel()
		for _, version := range []string{"9.15.9", "10.0.0-alpha.1", "12.0.0", "12.0.0-rc.6", "13.0.0"} {
			for _, declaration := range []string{
				`"packageManager":"pnpm@` + version + `"`,
				`"devEngines":{"packageManager":{"name":"pnpm","version":"` + version + `"}}`,
			} {
				root := createRepository(t, map[string]string{
					"package.json":   `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + declaration + `}`,
					"pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
				})
				result := analyze(t, root, ".")
				assertRejected(t, result, IDPnpmVersionUnsupported)
			}
		}
	})

	t.Run("pnpm non-exact versions keep version-required", func(t *testing.T) {
		t.Parallel()
		for _, declaration := range []string{
			`"packageManager":"pnpm@^11.0.0"`,
			`"packageManager":"pnpm@latest"`,
			`"packageManager":"pnpm@11"`,
			`"packageManager":"pnpm@11.0"`,
			`"devEngines":{"packageManager":{"name":"pnpm","version":"^11.0.0"}}`,
			`"devEngines":{"packageManager":{"name":"pnpm","version":"11"}}`,
			`"devEngines":{"packageManager":{"name":"pnpm","version":"11.0"}}`,
			`"devEngines":{"packageManager":{"name":"pnpm"}}`,
		} {
			root := createRepository(t, map[string]string{
				"package.json":   `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + declaration + `}`,
				"pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
			})
			result := analyze(t, root, ".")
			assertRejected(t, result, IDPackageManagerVersionRequired)
		}
	})

	t.Run("yarn Berry v4 and v5 accepted from both manifest sources", func(t *testing.T) {
		t.Parallel()
		for _, version := range []string{"4.9.2", "5.0.0", "5.0.0-rc.1"} {
			for _, declaration := range []string{
				`"packageManager":"yarn@` + version + `"`,
				`"devEngines":{"packageManager":{"name":"yarn","version":"` + version + `"}}`,
			} {
				root := createRepository(t, map[string]string{
					"package.json": `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + declaration + `}`,
					"yarn.lock":    "# yarn lockfile\n",
				})
				result := analyze(t, root, ".")
				assertPass(t, result)
				if result.Manager.Name != ManagerYarn || result.Manager.Version != version {
					t.Fatalf("manager = %#v, want yarn@%s", result.Manager, version)
				}
			}
		}
	})

	t.Run("yarn 6 or newer rejected from both manifest sources", func(t *testing.T) {
		t.Parallel()
		for _, version := range []string{"6.0.0", "6.0.0-alpha.1", "6.1.0", "7.0.0", "10.0.0"} {
			for _, declaration := range []string{
				`"packageManager":"yarn@` + version + `"`,
				`"devEngines":{"packageManager":{"name":"yarn","version":"` + version + `"}}`,
			} {
				root := createRepository(t, map[string]string{
					"package.json": `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + declaration + `}`,
					"yarn.lock":    "# yarn lockfile\n",
				})
				result := analyze(t, root, ".")
				assertRejected(t, result, IDYarnVersionUnsupported)
			}
		}
	})

	t.Run("yarn below v4 and non-exact forms keep selection-invalid from both manifest sources", func(t *testing.T) {
		t.Parallel()
		for _, declaration := range []string{
			`"packageManager":"yarn@3.6.4"`,
			`"packageManager":"yarn@^4.0.0"`,
			`"packageManager":"yarn@4"`,
			`"packageManager":"yarn@4.9"`,
			`"packageManager":"yarn@5"`,
			`"devEngines":{"packageManager":{"name":"yarn","version":"3.6.4"}}`,
			`"devEngines":{"packageManager":{"name":"yarn","version":"^4.0.0"}}`,
			`"devEngines":{"packageManager":{"name":"yarn","version":"4"}}`,
			`"devEngines":{"packageManager":{"name":"yarn"}}`,
		} {
			root := createRepository(t, map[string]string{
				"package.json": `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + declaration + `}`,
				"yarn.lock":    "# yarn lockfile\n",
			})
			result := analyze(t, root, ".")
			assertRejected(t, result, IDYarnSelectionInvalid)
		}
	})
}

func TestDescriptorDigests(t *testing.T) {
	t.Parallel()
	sha256Hex := strings.Repeat("a1", 32)
	sha512Hex := strings.Repeat("c3", 64)

	t.Run("digest declarations accepted from both manifest sources", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			manager     Manager
			lockfile    string
			version     string
			algorithm   string
			hex         string
			declaration func(string) string
		}{
			{manager: ManagerPNPM, lockfile: "pnpm-lock.yaml", version: "11.9.0", algorithm: "sha512", hex: sha512Hex,
				declaration: func(version string) string { return `"packageManager":"pnpm@` + version + `"` }},
			{manager: ManagerPNPM, lockfile: "pnpm-lock.yaml", version: "10.34.6", algorithm: "sha256", hex: sha256Hex,
				declaration: func(version string) string {
					return `"devEngines":{"packageManager":{"name":"pnpm","version":"` + version + `"}}`
				}},
			{manager: ManagerYarn, lockfile: "yarn.lock", version: "4.9.2", algorithm: "sha256", hex: sha256Hex,
				declaration: func(version string) string { return `"packageManager":"yarn@` + version + `"` }},
			{manager: ManagerYarn, lockfile: "yarn.lock", version: "4.9.2", algorithm: "sha512", hex: sha512Hex,
				declaration: func(version string) string {
					return `"devEngines":{"packageManager":{"name":"yarn","version":"` + version + `"}}`
				}},
		} {
			test := test
			t.Run(string(test.manager)+"/"+test.algorithm, func(t *testing.T) {
				t.Parallel()
				declared := test.version + "+" + test.algorithm + "." + test.hex
				root := createRepository(t, map[string]string{
					"package.json": `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + test.declaration(declared) + `}`,
					test.lockfile:  "lockfile\n",
				})
				result := analyze(t, root, ".")
				assertPass(t, result)
				if result.Manager.Name != test.manager || result.Manager.Version != test.version {
					t.Fatalf("manager = %#v, want %s@%s", result.Manager, test.manager, test.version)
				}
				if result.Manager.Digest == nil ||
					*result.Manager.Digest != (DeclaredDigest{Algorithm: test.algorithm, Hex: test.hex}) {
					t.Fatalf("digest = %#v, want %s.%s", result.Manager.Digest, test.algorithm, test.hex)
				}
				if result.Manager.Descriptor() != declared {
					t.Fatalf("descriptor = %q, want %q", result.Manager.Descriptor(), declared)
				}
			})
		}
	})

	t.Run("malformed digest suffixes rejected from both manifest sources", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			lockfile    string
			declaration string
		}{
			{"pnpm-lock.yaml", `"packageManager":"pnpm@11.9.0+build123"`},
			{"pnpm-lock.yaml", `"packageManager":"pnpm@11.9.0+sha224.` + sha256Hex + `"`},
			{"yarn.lock", `"packageManager":"yarn@4.1.0+build123"`},
			{"yarn.lock", `"devEngines":{"packageManager":{"name":"yarn","version":"4.1.0+sha512."}}`},
			{"pnpm-lock.yaml", `"devEngines":{"packageManager":{"name":"pnpm","version":"11.9.0+sha512.` + sha512Hex + `.build1"}}`},
			{"package-lock.json", `"packageManager":"npm@11.5.1+sha512.` + sha512Hex + `"`},
			{"package-lock.json", `"devEngines":{"packageManager":{"name":"npm","version":"11.5.1+sha256.` + sha256Hex + `"}}`},
		} {
			root := createRepository(t, map[string]string{
				"package.json": `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + test.declaration + `}`,
				test.lockfile:  "lockfile\n",
			})
			result := analyze(t, root, ".")
			assertRejected(t, result, IDPackageManagerDigestMalformed)
		}
	})

	t.Run("digest grammar is checked before version exactness and bounds", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			declaration string
			wantID      string
		}{
			// Malformed digest on an out-of-range version: grammar wins.
			{`"packageManager":"pnpm@9.15.9+sha256.xyz"`, IDPackageManagerDigestMalformed},
			// Well-formed digest on a range: exactness next.
			{`"packageManager":"pnpm@^11.0.0+sha256.` + sha256Hex + `"`, IDPackageManagerVersionRequired},
			// Well-formed digest on an exact out-of-range version: bound last.
			{`"packageManager":"pnpm@9.15.9+sha256.` + sha256Hex + `"`, IDPnpmVersionUnsupported},
		} {
			root := createRepository(t, map[string]string{
				"package.json":   `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + test.declaration + `}`,
				"pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
			})
			result := analyze(t, root, ".")
			assertRejected(t, result, test.wantID)
		}
	})

	t.Run("differing digests across manifest fields conflict", func(t *testing.T) {
		t.Parallel()
		otherSha256Hex := strings.Repeat("b2", 32)
		for _, test := range []struct {
			name          string
			rootDeclared  string
			childDeclared string
		}{
			{name: "different digest hex", rootDeclared: "11.0.0+sha256." + sha256Hex, childDeclared: "11.0.0+sha256." + otherSha256Hex},
			{name: "different digest algorithm", rootDeclared: "11.0.0+sha256." + sha256Hex, childDeclared: "11.0.0+sha384." + strings.Repeat("a1", 48)},
			{name: "digest only on one field", rootDeclared: "11.0.0", childDeclared: "11.0.0+sha256." + sha256Hex},
		} {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				root := createRepository(t, map[string]string{
					"package.json":            `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"pnpm@` + test.rootDeclared + `","workspaces":["packages/*"]}`,
					"pnpm-lock.yaml":          "lockfileVersion: '9.0'\n",
					"packages/a/package.json": `{"name":"a","version":"1.0.0","repository":"windlasstech/slsa-builder","devEngines":{"packageManager":{"name":"pnpm","version":"` + test.childDeclared + `"}}}`,
				})
				result := analyze(t, root, "packages/a")
				assertRejected(t, result, IDPackageManagerConflict)
			})
		}
	})

	t.Run("matching digests across manifest fields do not conflict", func(t *testing.T) {
		t.Parallel()
		declared := "11.0.0+sha256." + sha256Hex
		root := createRepository(t, map[string]string{
			"package.json":            `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"pnpm@` + declared + `","workspaces":["packages/*"]}`,
			"pnpm-lock.yaml":          "lockfileVersion: '9.0'\n",
			"packages/a/package.json": `{"name":"a","version":"1.0.0","repository":"windlasstech/slsa-builder","devEngines":{"packageManager":{"name":"pnpm","version":"` + declared + `"}}}`,
		})
		result := analyze(t, root, "packages/a")
		assertPass(t, result)
		if result.Manager.Digest == nil || result.Manager.Descriptor() != declared {
			t.Fatalf("selection = %#v, want digest descriptor %q", result.Manager, declared)
		}
	})
}

func TestNPMDeclaredVersion(t *testing.T) {
	t.Parallel()

	t.Run("declared npm versions accepted non-authoritative", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name        string
			declaration string
			want        string
		}{
			{name: "exact top-level", declaration: `"packageManager":"npm@11.5.1"`, want: "11.5.1"},
			{name: "range top-level", declaration: `"packageManager":"npm@^11.5.1"`, want: "^11.5.1"},
			{name: "garbage top-level", declaration: `"packageManager":"npm@garbage"`, want: "garbage"},
			{name: "exact devEngines", declaration: `"devEngines":{"packageManager":{"name":"npm","version":"11.5.1"}}`, want: "11.5.1"},
			{name: "range devEngines", declaration: `"devEngines":{"packageManager":{"name":"npm","version":"^11.5.1"}}`, want: "^11.5.1"},
			{name: "omitted devEngines", declaration: `"devEngines":{"packageManager":{"name":"npm"}}`, want: ""},
		} {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				root := createRepository(t, map[string]string{
					"package.json":      `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder",` + test.declaration + `}`,
					"package-lock.json": `{}`,
				})
				result := analyze(t, root, ".")
				assertPass(t, result)
				if result.Manager.Name != ManagerNPM || result.Manager.Version != "" {
					t.Fatalf("manager = %#v, want npm with a toolchain-owned version", result.Manager)
				}
				if result.Manager.DeclaredVersion != test.want {
					t.Fatalf("declared version = %q, want %q", result.Manager.DeclaredVersion, test.want)
				}
				if result.Manager.Digest != nil {
					t.Fatalf("digest = %#v, want nil for npm", result.Manager.Digest)
				}
			})
		}
	})

	t.Run("lockfile inference leaves the declared version empty", func(t *testing.T) {
		t.Parallel()
		root := createRepository(t, map[string]string{
			"package.json":      `{"name":"example","version":"1.0.0","repository":"windlasstech/slsa-builder"}`,
			"package-lock.json": `{}`,
		})
		result := analyze(t, root, ".")
		assertPass(t, result)
		if result.Manager.Name != ManagerNPM || result.Manager.DeclaredVersion != "" {
			t.Fatalf("manager = %#v, want npm with empty declared version", result.Manager)
		}
	})
}

func loadRejectedFixtures(t *testing.T) []struct {
	Name              string
	Artifact          string
	ExpectedPrimaryID string
} {
	t.Helper()
	index, err := fixture.Load(filepath.Join(testRepositoryRoot(t), "testdata", "fixtures", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	selected, err := fixture.Select(index, "npm", "build-pack")
	if err != nil {
		t.Fatal(err)
	}
	fixtures := make([]struct {
		Name              string
		Artifact          string
		ExpectedPrimaryID string
	}, 0)
	for _, manifest := range selected.Fixtures {
		if manifest.Type != "rejected" || manifest.ExpectedPrimaryID == nil {
			continue
		}
		fixtures = append(fixtures, struct {
			Name              string
			Artifact          string
			ExpectedPrimaryID string
		}{manifest.Name, manifest.Artifact, *manifest.ExpectedPrimaryID})
	}
	return fixtures
}

func analyzeFixture(t *testing.T, repositoryRoot, packageDirectory string) Result {
	t.Helper()
	fixtureRelative := strings.TrimPrefix(packageDirectory, "testdata/npm/packages/")
	parts := strings.Split(fixtureRelative, "/")
	fixtureRootParts := 1
	selectedDirectory := "."
	if len(parts) > 0 && parts[0] == "stale-lockfiles" {
		fixtureRootParts = 2
	}
	if len(parts) > 0 && parts[0] == "rejected" {
		fixtureRootParts = 2
		selectedDirectory = strings.Join(parts[2:], "/")
		if selectedDirectory == "" {
			selectedDirectory = "."
		}
	}
	if len(parts) > 0 && parts[0] == "workspace-valid" {
		fixtureRootParts = 1
		selectedDirectory = strings.Join(parts[1:], "/")
	}
	fixtureRoot := filepath.Join(repositoryRoot, "testdata", "npm", "packages", filepath.FromSlash(strings.Join(parts[:fixtureRootParts], "/")))
	return analyze(t, fixtureRoot, selectedDirectory)
}

func analyze(t *testing.T, repositoryRoot, packageDirectory string) Result {
	t.Helper()
	result, err := Analyze(Config{
		RepositoryRoot:     repositoryRoot,
		PackageDirectory:   packageDirectory,
		ObservedRepository: observedRepository,
	})
	if err != nil {
		t.Fatalf("Analyze() internal error: %v", err)
	}
	return result
}

func assertPass(t *testing.T, result Result) {
	t.Helper()
	if result.Report.Result != diagnostic.ResultPass || result.Report.ExitCode != diagnostic.ExitCodePass || result.Report.PrimaryID != nil {
		t.Fatalf("report = %#v, want pass", result.Report)
	}
}

func assertRejected(t *testing.T, result Result, expectedID string) {
	t.Helper()
	if result.Report.Result != diagnostic.ResultFail || result.Report.ExitCode != diagnostic.ExitCodePolicyFailure {
		t.Fatalf("report = %#v, want policy rejection", result.Report)
	}
	if result.Report.PrimaryID == nil || *result.Report.PrimaryID != expectedID {
		t.Fatalf("primary ID = %#v, want %q", result.Report.PrimaryID, expectedID)
	}
}

func testRepositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func createRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, contents := range files {
		filePath := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filePath, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
