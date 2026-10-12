package npmprofile

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

func TestBuildPackFlowFailuresCleanOutputs(t *testing.T) {
	// These cases replace PATH and therefore must remain serial.
	for _, test := range []struct{ name, command, want string }{
		{"install", "ci", "install dependencies: npm [ci] failed: exit status 7: stopped"},
		{"build", "run", "run declared build script: npm [run build] failed: exit status 7: stopped"},
		{"pack", "pack", "pack selected package: npm [pack --pack-destination %s] failed: exit status 7: stopped"},
		{"empty pack", "empty", IDPackagePackFailed + ": pack output contains 0 entries, want exactly one"},
	} {
		t.Run(test.name, func(t *testing.T) {
			clearToolchainRootEnvironment(t)
			installFakeToolchain(t)
			t.Setenv("ImageOS", "")
			t.Setenv("ImageVersion", "")
			npm, err := exec.LookPath("npm")
			if err != nil {
				t.Fatal(err)
			}
			script := "#!/bin/sh\ncase \"$1\" in\n--version) printf '11.5.1';;\n" + test.command + ") printf stopped; exit 7;;\nesac\n"
			writeScript(t, filepath.Dir(npm), "npm", script)
			root := createRepository(t, map[string]string{
				"package.json":      `{"name":"example","version":"1.0.0","packageManager":"npm@11.5.1","repository":"windlasstech/slsa-builder","scripts":{"build":"ignored"}}`,
				"package-lock.json": `{}`,
			})
			output := t.TempDir()
			_, err = BuildPack(t.Context(), BuildPackConfig{Selection: analyze(t, root, "."), OutputDirectory: output, ArtifactName: "artifact", ExternalParameters: []byte(`{}`)})
			want := test.want
			if test.command == "pack" {
				realOutput, resolveErr := filepath.EvalSymlinks(output)
				if resolveErr != nil {
					t.Fatal(resolveErr)
				}
				want = "pack selected package: npm [pack --pack-destination " + filepath.Join(realOutput, "package") + "] failed: exit status 7: stopped"
			}
			if err == nil || err.Error() != want {
				t.Fatalf("BuildPack() error = %v; want %q", err, want)
			}
			entries, readErr := os.ReadDir(output)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("failed build outputs = %v, %v; want empty directory", entries, readErr)
			}
		})
	}
}

func TestBuildPackInvalidYarnOutputName(t *testing.T) {
	t.Parallel()
	selection := Result{}
	selection.Report.Result = diagnostic.ResultPass
	selection.Package.RealDirectory, selection.Package.RealManagerRoot = "/root", "/root"
	selection.Package.Name, selection.Package.Version = "example", "../escape"
	selection.Manager.Name = ManagerYarn
	err := validateBuildPackConfig(BuildPackConfig{Selection: selection, ArtifactName: "artifact", ExternalParameters: []byte(`{}`)})
	if err == nil || err.Error() != `invalid Yarn pack output name: payload file name "example-../escape.tgz" contains a path separator` {
		t.Fatalf("invalid Yarn filename = %v", err)
	}
}

func TestRunCommandManagerRechecksSelection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, want string }{
		{"empty argv", "unsupported package manager"},
		{"changed root", "package manager root changed after package selection"},
		{"new credentials", `package-manager configuration ".npmrc" contains credential material`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := createRepository(t, map[string]string{".npmrc": "_authToken=secret"})
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			selection := Result{}
			selection.Package.RealManagerRoot, selection.Package.RealDirectory = root, root
			args := []string{"ci"}
			switch test.name {
			case "empty argv":
				args = nil
			case "changed root":
				selection.Package.RealManagerRoot = filepath.Join(root, "missing")
			}
			err = runManagerCommand(t.Context(), selection, "must-not-run", nil, args)
			if err == nil || err.Error() != test.want {
				t.Fatalf("runManagerCommand() = %v; want %q", err, test.want)
			}
		})
	}
}

func TestBuildPackExecutionRootRejections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, want string }{
		{"missing package", "package directory changed after package selection"},
		{"outside package", "package directory is outside the package manager root"},
		{"file manager", "package manager root is not a directory"},
		{"file package", "package directory is not a directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := createRepository(t, map[string]string{"file": "x"})
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			selection := Result{}
			selection.Package.RealManagerRoot = root
			selection.Package.RealDirectory = root
			switch test.name {
			case "missing package":
				selection.Package.RealDirectory = filepath.Join(root, "missing")
			case "outside package":
				selection.Package.RealDirectory, err = filepath.EvalSymlinks(createRepository(t, nil))
				if err != nil {
					t.Fatal(err)
				}
			case "file manager":
				selection.Package.RealManagerRoot = filepath.Join(root, "file")
				selection.Package.RealDirectory = selection.Package.RealManagerRoot
			case "file package":
				selection.Package.RealDirectory = filepath.Join(root, "file")
			}
			if err := validateExecutionRoots(selection); err == nil || err.Error() != test.want {
				t.Fatalf("validateExecutionRoots() = %v; want %q", err, test.want)
			}
		})
	}
}

func TestControlledEnvironmentPinsCorepackEnvFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	environment, err := controlledEnvironment(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(environment, "COREPACK_ENV_FILE=0") {
		t.Fatalf("controlled environment lacks the COREPACK_ENV_FILE=0 pin: %v", environment)
	}
}

func TestBuildPackEnvironmentCollision(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"home", "tmp", "cache", "corepack", "empty-npmrc", "empty-global-npmrc"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := createRepository(t, map[string]string{name: "preserve"})
			_, err := controlledEnvironment(root)
			if !errors.Is(err, os.ErrExist) {
				t.Fatalf("controlledEnvironment collision = %v; want os.ErrExist", err)
			}
		})
	}
}

func TestBuildPackCommandTemplates(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		manager       Manager
		relative      string
		install, pack []string
	}{
		{ManagerNPM, ".", []string{"ci"}, []string{"pack", "--pack-destination", "/output"}},
		{ManagerPNPM, "packages/a", []string{"install", "--frozen-lockfile"}, []string{"--filter", "{./packages/a}", "pack", "--pack-destination", "/output"}},
		{ManagerYarn, ".", []string{"install", "--immutable"}, []string{"pack", "--out", "/output/scope-a-1.0.0.tgz"}},
		{ManagerYarn, "packages/a", []string{"install", "--immutable"}, []string{"workspace", "@scope/a", "pack", "--out", "/output/scope-a-1.0.0.tgz"}},
		{Manager("unsupported"), ".", nil, nil},
	} {
		t.Run(string(test.manager)+test.relative, func(t *testing.T) {
			t.Parallel()
			selection := Result{}
			selection.Manager.Name = test.manager
			selection.Package.Name, selection.Package.Version = "@scope/a", "1.0.0"
			selection.Package.ManagerRootRelativeDirectory = test.relative
			if got := installArguments(test.manager); !reflect.DeepEqual(got, test.install) {
				t.Fatalf("install argv = %v; want %v", got, test.install)
			}
			if got := packArguments(selection, "/output"); !reflect.DeepEqual(got, test.pack) {
				t.Fatalf("pack argv = %v; want %v", got, test.pack)
			}
		})
	}
}

func TestBuildPackMetadataExclusiveCreate(t *testing.T) {
	t.Parallel()
	root := createRepository(t, map[string]string{"metadata.json": "preserve"})
	err := writeBuildMetadata(filepath.Join(root, "metadata.json"), BuildMetadata{})
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("writeBuildMetadata() = %v; want os.ErrExist", err)
	}
	if got := string(readFixture(t, root, "metadata.json")); got != "preserve" {
		t.Fatalf("existing metadata = %q; want preserve", got)
	}
}
