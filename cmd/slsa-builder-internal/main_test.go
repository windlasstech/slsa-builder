package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
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
