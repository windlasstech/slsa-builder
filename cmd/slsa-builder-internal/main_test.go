package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

func TestUnknownSubcommand(t *testing.T) {
	if os.Getenv("SLSA_BUILDER_TEST_HELPER") == "1" {
		os.Args = []string{os.Args[0], "does-not-exist"}
		main()
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestUnknownSubcommand$")
	cmd.Env = append(os.Environ(), "SLSA_BUILDER_TEST_HELPER=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected invocation failure, output=%s", output)
	}

	exitError, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected process exit error, got %T: %v", err, err)
	}
	if exitError.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d; output=%s", exitError.ExitCode(), output)
	}

	var report struct {
		PrimaryID string `json:"primary_id"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(output))), &report); err != nil {
		t.Fatalf("expected JSON report, got %q: %v", output, err)
	}
	if report.PrimaryID != "windlass.verify.error.verifier-execution-failure" {
		t.Fatalf("unexpected primary diagnostic: %q", report.PrimaryID)
	}
}

func TestCLIOperationalDiagnosticsSeparateStdoutAndStderr(t *testing.T) {
	if os.Getenv("SLSA_BUILDER_TEST_HELPER") == "1" {
		os.Args = []string{os.Args[0], "npm-profile-toolchain-assert"}
		main()
	}
	t.Parallel()
	// Given a real CLI subprocess whose fake node produces a pair mismatch and sensitive canaries.
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	const (
		secretCanary = "cli-secret-canary-284c71"
		childCanary  = "cli-child-output-canary-6a39d0"
	)
	node := filepath.Join(bin, "node")
	script := "#!/bin/sh\nprintf 'v0.0.0\\n'\nprintf '" + childCanary + "' >&2\n"
	if err := os.WriteFile(node, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCLIOperationalDiagnosticsSeparateStdoutAndStderr$")
	cmd.Env = []string{
		"SLSA_BUILDER_TEST_HELPER=1",
		"PATH=" + bin,
		"RUNNER_TOOL_CACHE=" + bin,
		"MISE_DATA_DIR=" + filepath.Join(root, "mise"),
		"NPM_TOKEN=" + secretCanary,
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// When the CLI executes node and emits its verification report.
	err := cmd.Run()

	// Then stdout stays canonical JSON while only stderr carries safe human diagnostics.
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 1 {
		t.Fatalf("CLI error = %T %v; want exit code 1, stdout=%q stderr=%q", err, err, stdout.Bytes(), stderr.Bytes())
	}
	reportBytes := bytes.TrimSpace(stdout.Bytes())
	canonical, canonicalErr := canonicaljson.Canonicalize(reportBytes)
	if canonicalErr != nil || !bytes.Equal(canonical, reportBytes) {
		t.Fatalf("stdout is not one canonical report: %q, %v", stdout.Bytes(), canonicalErr)
	}
	var report struct {
		PrimaryID string `json:"primary_id"`
	}
	if err := json.Unmarshal(reportBytes, &report); err != nil || report.PrimaryID != "windlass.verify.error.toolchain-pair-mismatch" {
		t.Fatalf("stdout report = %q, %v; want toolchain-pair-mismatch", stdout.Bytes(), err)
	}
	if strings.Contains(stdout.String(), "windlass operational: ") {
		t.Fatalf("human diagnostics leaked into stdout: %q", stdout.Bytes())
	}
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("stderr operational records = %q; want start and completion", stderr.Bytes())
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "windlass operational: ") {
			t.Fatalf("unexpected stderr line: %q", line)
		}
	}
	if !strings.Contains(lines[0], "operation=node") || !strings.HasSuffix(lines[0], " start") {
		t.Fatalf("stderr start record = %q; want fixed operation label and start notice", lines[0])
	}
	for _, field := range []string{"operation=node", "started=true", "outcome=exit", "exit=0"} {
		if !strings.Contains(lines[1], field) {
			t.Fatalf("stderr completion = %q; missing operational field %q", lines[1], field)
		}
	}
	for _, disclosed := range []string{secretCanary, childCanary, "--version", root, bin, node, "NPM_TOKEN="} {
		if strings.Contains(stderr.String(), disclosed) {
			t.Fatalf("stderr disclosed %q: %q", disclosed, stderr.Bytes())
		}
	}
	if strings.ContainsAny(stderr.String(), "{}") || strings.Contains(stderr.String(), "primary_id") {
		t.Fatalf("stderr contains JSON diagnostics: %q", stderr.Bytes())
	}
}

func TestCLIOperationalDiagnosticsBrokenStderrDoesNotChangeOutcome(t *testing.T) {
	if os.Getenv("SLSA_BUILDER_TEST_HELPER") == "1" {
		os.Args = []string{os.Args[0], "npm-profile-toolchain-assert"}
		main()
	}
	t.Parallel()
	// Given a real CLI subprocess with an executable node fake and a closed diagnostic stderr reader.
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	node := filepath.Join(bin, "node")
	if err := os.WriteFile(node, []byte("#!/bin/sh\nprintf 'v0.0.0\\n'\nprintf 'closed-stderr-canary' >&2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCLIOperationalDiagnosticsBrokenStderrDoesNotChangeOutcome$")
	cmd.Env = []string{
		"SLSA_BUILDER_TEST_HELPER=1",
		"PATH=" + bin,
		"RUNNER_TOOL_CACHE=" + bin,
		"MISE_DATA_DIR=" + filepath.Join(root, "mise"),
	}
	stderrReader, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := stderrReader.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// When the CLI emits diagnostics into the broken pipe.
	err = cmd.Run()

	// Then the original command result and canonical stdout survive without SIGPIPE termination.
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 1 {
		t.Fatalf("CLI error = %T %v; want the original exit code 1, stdout=%q", err, err, stdout.Bytes())
	}
	reportBytes := bytes.TrimSpace(stdout.Bytes())
	canonical, canonicalErr := canonicaljson.Canonicalize(reportBytes)
	if canonicalErr != nil || !bytes.Equal(canonical, reportBytes) {
		t.Fatalf("stdout is not the original canonical report: %q, %v", stdout.Bytes(), canonicalErr)
	}
	var report struct {
		PrimaryID string `json:"primary_id"`
	}
	if err := json.Unmarshal(reportBytes, &report); err != nil || report.PrimaryID != "windlass.verify.error.toolchain-pair-mismatch" {
		t.Fatalf("stdout report = %q, %v; want toolchain-pair-mismatch", stdout.Bytes(), err)
	}
}

func TestOperationalDebugStderrClose(t *testing.T) {
	t.Parallel()
	writer, closeDiagnostics := operationalDebugStderr()
	closeDiagnostics()
	if _, err := writer.Write([]byte("closed descriptor")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after cleanup error = %v; want os.ErrClosed", err)
	}
}

func TestOperationalDebugStderrCloseOnExec(t *testing.T) {
	t.Parallel()
	writer, closeDiagnostics := operationalDebugStderr()
	defer closeDiagnostics()
	file, ok := writer.(*os.File)
	if !ok {
		t.Fatalf("diagnostic writer type = %T; want *os.File", writer)
	}
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, file.Fd(), syscall.F_GETFD, 0)
	if errno != 0 {
		t.Fatalf("read diagnostic descriptor flags: %v", errno)
	}
	if flags&syscall.FD_CLOEXEC == 0 {
		t.Fatal("diagnostic descriptor is inherited by child processes")
	}
}
