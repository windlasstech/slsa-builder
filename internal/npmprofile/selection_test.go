package npmprofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
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
			"package.json":                   `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"pnpm@10.14.0"}`,
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
			"package.json":            `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"pnpm@10.14.0"}`,
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
		{name: "pnpm", packageDirectory: "testdata/npm/packages/scoped-valid", manager: ManagerPNPM, version: "10.14.0"},
		{name: "yarn", packageDirectory: "testdata/npm/packages/yarn-valid", manager: ManagerYarn, version: "4.9.2"},
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
			"package.json":            `{"name":"root","version":"1.0.0","private":true,"repository":"windlasstech/slsa-builder","packageManager":"pnpm@10.15.0","workspaces":["packages/*"]}`,
			"pnpm-lock.yaml":          "lockfileVersion: '9.0'\n",
			"packages/a/package.json": `{"name":"a","version":"1.0.0","repository":"windlasstech/slsa-builder","packageManager":"pnpm@10.14.0"}`,
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
		{name: "devEngines", packageDirectory: "testdata/npm/packages/rejected/yarn-devengines"},
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

type rejectedFixture struct {
	Name              string  `json:"name"`
	Type              string  `json:"type"`
	Surface           string  `json:"surface"`
	Artifact          string  `json:"artifact"`
	ExpectedPrimaryID *string `json:"expected-primary-id"`
}

func loadRejectedFixtures(t *testing.T) []struct {
	Name              string
	Artifact          string
	ExpectedPrimaryID string
} {
	t.Helper()
	encoded, err := os.ReadFile(filepath.Join(testRepositoryRoot(t), "testdata", "fixtures", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index struct {
		Fixtures []rejectedFixture `json:"fixtures"`
	}
	if err := json.Unmarshal(encoded, &index); err != nil {
		t.Fatal(err)
	}
	fixtures := make([]struct {
		Name              string
		Artifact          string
		ExpectedPrimaryID string
	}, 0)
	for _, fixture := range index.Fixtures {
		if fixture.Type != "rejected" || fixture.Surface != "npm" || fixture.ExpectedPrimaryID == nil {
			continue
		}
		fixtures = append(fixtures, struct {
			Name              string
			Artifact          string
			ExpectedPrimaryID string
		}{fixture.Name, fixture.Artifact, *fixture.ExpectedPrimaryID})
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
