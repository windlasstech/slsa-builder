package command_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/command"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

func TestPinCheckCommand(t *testing.T) {
	t.Parallel()
	for _, drift := range []bool{false, true} {
		t.Run(fmt.Sprintf("drift=%t", drift), func(t *testing.T) {
			t.Parallel()
			// Given matching temp records and tables, optionally with a changed SRI cell.
			args := pinCheckArgs(t, drift)
			var out bytes.Buffer
			// When dispatched through the real adapter and disk checker.
			result := command.NewDispatcher(command.NewPinCheckCommand()).Dispatch(t.Context(), args, &out)
			// Then success is exit 0; drift emits the canonical registered exit-1 report.
			if !drift {
				if result.ExitCode != 0 {
					t.Fatalf("result = %+v, output = %s", result, out.Bytes())
				}
				return
			}
			if result.ExitCode != 1 {
				t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
			}
			var report diagnostic.Report
			if err := json.Unmarshal(out.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if report.ExitCode != 1 || report.PrimaryID == nil || *report.PrimaryID != "windlass.verify.error.toolchain-pin-drift" ||
				len(report.Diagnostics) != 1 || report.Diagnostics[0].Check != "toolchain.pins.drift" {
				t.Fatalf("report = %+v", report)
			}
			canonical, err := report.CanonicalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out.Bytes(), append(canonical, '\n')) {
				t.Fatal("report is not canonical JSON plus newline")
			}
		})
	}
}

func TestPinCheckCommandMissingFlags(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"all", "--publish-pin", "--pair-record", "--publish-spec", "--build-spec"} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()
			// Given an invocation missing a required flag.
			args := []string{"pin-check"}
			for _, flag := range []string{"--publish-pin", "--pair-record", "--publish-spec", "--build-spec"} {
				if missing != "all" && flag != missing {
					args = append(args, flag, "unused")
				}
			}
			var out bytes.Buffer
			// When dispatched without complete input paths.
			result := command.NewDispatcher(command.NewPinCheckCommand()).Dispatch(t.Context(), args, &out)
			// Then validation fails before disk access with exit 2.
			if result.ExitCode != 2 {
				t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
			}
		})
	}
}

type failingWriter struct{ cause error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.cause }

func TestPinCheckCommandReportWriteFailure(t *testing.T) {
	t.Parallel()
	// Given drifted records and a sink that rejects the drift report.
	args := pinCheckArgs(t, true)
	cause := errors.New("report sink failed")
	// When the drift report cannot be written.
	err := command.NewPinCheckCommand().Execute(t.Context(), args[1:], failingWriter{cause: cause})
	// Then the sink cause surfaces instead of the verification-failure sentinel.
	if !errors.Is(err, cause) || errors.Is(err, command.ErrVerificationFailure) {
		t.Fatalf("Execute() = %v, want sink cause without report sentinel", err)
	}
}

func pinCheckArgs(t *testing.T, drift bool) []string {
	t.Helper()
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
	specIntegrity := integrity
	if drift {
		specIntegrity = strings.Replace(integrity, "sha512-A", "sha512-B", 1)
	}
	files := []struct{ flag, name, content string }{
		{"--publish-pin", "pin.json", fmt.Sprintf(`{"version":"11.20.0","tarball_url":"https://registry.npmjs.org/npm/-/npm-11.20.0.tgz","integrity":%q}`, integrity)},
		{"--pair-record", "pair.json", `{"node_version":"24.20.0","npm_version":"11.18.0"}`},
		{"--publish-spec", "publish.md", "## Publish-stage npm CLI allowlist\n\n| Version | Tarball URL | SHA-512 (SRI) |\n| --- | --- | --- |\n| 11.20.0 | https://registry.npmjs.org/npm/-/npm-11.20.0.tgz | " + specIntegrity + " |\n"},
		{"--build-spec", "build.md", "## Node.js and npm toolchain pair\n\n### Pinned runtime table\n\n| Node.js version | Bundled npm version |\n| --- | --- |\n| 24.20.0 | 11.18.0 |\n"},
	}
	directory := t.TempDir()
	args := []string{"pin-check"}
	for _, file := range files {
		path := filepath.Join(directory, file.name)
		if err := os.WriteFile(path, []byte(file.content), 0o600); err != nil {
			t.Fatal(err)
		}
		args = append(args, file.flag, path)
	}
	return args
}

func TestPinCheckCLIRejectsSpecDrift(t *testing.T) {
	t.Parallel()
	// Given a temporary copy of the real publish spec with one SRI character flipped.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join(root, "docs/architecture/js-ts-npm-provenance-publish.md"))
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(spec, []byte("| `sha512-"))
	if start < 0 {
		t.Fatal("SRI cell not found")
	}
	index := start + len("| `sha512-")
	if spec[index] == 'A' {
		spec[index] = 'B'
	} else {
		spec[index] = 'A'
	}
	path := filepath.Join(t.TempDir(), "publish.md")
	if err := os.WriteFile(path, spec, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	args := []string{"run", "./cmd/slsa-builder-internal", "pin-check",
		"--publish-pin", "internal/toolchainpins/publish-npm.json",
		"--pair-record", "internal/toolchainpins/node-runtime.json",
		"--publish-spec", path,
		"--build-spec", "docs/architecture/js-ts-npm-build-pack.md"}
	process := exec.CommandContext(ctx, "go", args...)
	process.Dir = root
	process.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	var stdout, stderr bytes.Buffer
	process.Stdout, process.Stderr = &stdout, &stderr
	// When the registered executable runs from the repository root.
	err = process.Run()
	// Then the real CLI fails with exit 1 and the registered drift report.
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("run = %v, stderr = %s", err, stderr.Bytes())
	}
	var report diagnostic.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.ExitCode != 1 || report.PrimaryID == nil || *report.PrimaryID != "windlass.verify.error.toolchain-pin-drift" {
		t.Fatalf("report = %+v", report)
	}
	t.Logf("go %s\nstdout: %s\nstderr: %s\nexit: %d", strings.Join(args, " "), stdout.Bytes(), stderr.Bytes(), exit.ExitCode())
}
