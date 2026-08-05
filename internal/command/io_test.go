package command

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type typedInput struct {
	Name string `json:"name"`
}

func TestTypedInputs(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	validPath := filepath.Join(directory, "input.json")
	if err := os.WriteFile(validPath, []byte(`{"name":"artifact"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := ReadTypedJSON(validPath, func(value typedInput) error {
		if value.Name == "" {
			return errors.New("name is required")
		}
		return nil
	})
	if err != nil || input.Name != "artifact" {
		t.Fatalf("ReadTypedJSON() = %#v, %v", input, err)
	}

	unknownPath := filepath.Join(directory, "unknown.json")
	if err := os.WriteFile(unknownPath, []byte(`{"name":"artifact","extra":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTypedJSON[typedInput](unknownPath, nil); err == nil {
		t.Fatal("ReadTypedJSON() accepted an unknown member")
	}

	symlinkPath := filepath.Join(directory, "link.json")
	if err := os.Symlink(validPath, symlinkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTypedJSON[typedInput](symlinkPath, nil); err == nil {
		t.Fatal("ReadTypedJSON() accepted a symlink")
	}
}

func TestAtomicOutputs(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "result.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "new" {
		t.Fatalf("contents = %q, want new", contents)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory contains %d entries after atomic write", len(entries))
	}
}

func TestGitHubOutputAllowlist(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "github-output")
	allowlist := NewOutputAllowlist("report-path", "result")
	if err := WriteGitHubOutputs(path, allowlist, map[string]string{
		"result":      "pass",
		"report-path": "result.json",
	}); err != nil {
		t.Fatalf("WriteGitHubOutputs() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "report-path=result.json\nresult=pass\n" {
		t.Fatalf("output contents = %q", contents)
	}
	if err := WriteGitHubOutputs(path, allowlist, map[string]string{"unexpected": "value"}); err == nil {
		t.Fatal("WriteGitHubOutputs() accepted an output outside the allowlist")
	}
	if err := WriteGitHubOutputs(path, allowlist, map[string]string{"result": "pass\ninjected=true"}); err == nil {
		t.Fatal("WriteGitHubOutputs() accepted a file-command injection")
	}
}

func TestSecretRedaction(t *testing.T) {
	t.Parallel()

	secret := "github_pat_1234567890abcdef"
	redacted := RedactSecrets("request failed with Authorization: Bearer "+secret, secret)
	if strings.Contains(redacted, secret) || strings.Contains(strings.ToLower(redacted), "bearer") {
		t.Fatalf("RedactSecrets() leaked secret material: %q", redacted)
	}
	if !strings.Contains(redacted, RedactedValue) {
		t.Fatalf("RedactSecrets() = %q, want redaction marker", redacted)
	}

	path := filepath.Join(t.TempDir(), "github-output")
	err := WriteGitHubOutputs(path, NewOutputAllowlist("result"), map[string]string{"result": secret})
	if err == nil {
		t.Fatal("WriteGitHubOutputs() accepted secret-like output")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("WriteGitHubOutputs() error leaked secret: %v", err)
	}
}
