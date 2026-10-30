package npmprofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishCommandAcceptsProvisionedLauncher(t *testing.T) {
	// Given the real provisioning layout outside every ambient toolchain root.
	node, _ := provisionFakeNode(t)
	if err := os.WriteFile(node, []byte("#!/bin/sh\nprintf 11.21.1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runnerTemp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RUNNER_TEMP", runnerTemp)
	t.Setenv("RUNNER_TOOL_CACHE", "")
	t.Setenv("MISE_DATA_DIR", t.TempDir())
	data := provisionArchive(t, `{"version":"11.21.1"}`, nil)
	launcher, err := provisionNPM(t.Context(), filepath.Join(runnerTemp, "windlass-npm-cli"), provisionSource(t, data))
	if err != nil {
		t.Fatal(err)
	}
	// When the publish version preflight invokes the provisioned launcher.
	prepared := preparedPublish{request: PublishRequest{NPMExecutable: launcher}}
	err = prepared.validateNPMVersion(t.Context())
	// Then the actual process boundary accepts it and observes the pinned version.
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublishCommandRejectsProvisioningSiblings(t *testing.T) {
	// Given isolated toolchain roots and lookalikes outside the exact provisioning layout.
	provisionFakeNode(t)
	runnerTemp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RUNNER_TEMP", runnerTemp)
	t.Setenv("RUNNER_TOOL_CACHE", "")
	t.Setenv("MISE_DATA_DIR", t.TempDir())
	for _, relative := range []string{
		"npm", "npm-provision-123/npm", "windlass-npm-cli/npm",
		"windlass-npm-cli/npm-provision-/npm", "windlass-npm-cli/other/npm",
		"windlass-npm-cli/npm-provision-123/pnpm", "windlass-npm-cli/npm-provision-123/nested/npm",
		"windlass-npm-cli-sibling/npm-provision-123/npm",
	} {
		t.Run(relative, func(t *testing.T) {
			launcher := filepath.Join(runnerTemp, relative)
			if err := os.MkdirAll(filepath.Dir(launcher), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			// When a sibling is passed directly to the publish boundary.
			_, err := runPublishCommand(t.Context(), runnerTemp, launcher, nil, nil)
			// Then it still fails with the closed boundary diagnostic.
			requireProvisionID(t, err, "trusted-core-boundary-violation")
		})
	}
}

func TestPublishCommandRejectsUnsafeProvisionedLayout(t *testing.T) {
	for _, scenario := range []string{"missing runner temp", "relative runner temp", "missing launcher", "launcher symlink", "directory symlink", "public launcher", "public directory", "directory as launcher"} {
		t.Run(scenario, func(t *testing.T) {
			// Given a provisioning-shaped path violating one filesystem constraint.
			provisionFakeNode(t)
			runnerTemp, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("RUNNER_TEMP", runnerTemp)
			t.Setenv("RUNNER_TOOL_CACHE", "")
			t.Setenv("MISE_DATA_DIR", t.TempDir())
			private := filepath.Join(runnerTemp, "windlass-npm-cli", "npm-provision-123")
			if err := os.MkdirAll(private, 0o700); err != nil {
				t.Fatal(err)
			}
			launcher := filepath.Join(private, "npm")
			if scenario != "missing launcher" && scenario != "launcher symlink" && scenario != "directory as launcher" {
				if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "missing runner temp":
				t.Setenv("RUNNER_TEMP", "")
			case "relative runner temp":
				t.Setenv("RUNNER_TEMP", "relative")
			case "missing launcher":
			case "launcher symlink":
				target := filepath.Join(runnerTemp, "npm")
				if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, launcher); err != nil {
					t.Fatal(err)
				}
			case "directory symlink":
				moved := filepath.Join(runnerTemp, "outside")
				if err := os.Rename(private, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, private); err != nil {
					t.Fatal(err)
				}
			case "public launcher":
				if err := os.Chmod(launcher, 0o755); err != nil {
					t.Fatal(err)
				}
			case "public directory":
				if err := os.Chmod(private, 0o755); err != nil {
					t.Fatal(err)
				}
			case "directory as launcher":
				if err := os.Mkdir(launcher, 0o700); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("unhandled scenario %q", scenario)
			}
			// When executing through the publish boundary.
			_, err = runPublishCommand(t.Context(), runnerTemp, launcher, nil, nil)
			// Then no unsafe layout receives a trusted shim root.
			requireProvisionID(t, err, "trusted-core-boundary-violation")
		})
	}
}
