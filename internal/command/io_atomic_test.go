package command

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestAtomicOutputRejectsFilesystemFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, suffix, want string
	}{
		{"invalid path", "\x00", "inspect atomic output"},
		{"missing parent", "missing/output", "create atomic output"},
		{"directory", ".", "must be a regular non-symlink file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given an output path that cannot be replaced safely.
			root := t.TempDir()
			// When the atomic writer attempts to persist bytes.
			err := WriteFileAtomic(filepath.Join(root, test.suffix), []byte("payload"), 0o600)
			// Then it identifies the failed operation and leaves no temporary file.
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("WriteFileAtomic() = %v, want %q", err, test.want)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed write left entries %v, error %v", entries, err)
			}
		})
	}
}

func TestRegularFilesRejectPermissionFailures(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses Unix permission checks")
	}
	for _, operation := range []string{"read", "append", "create"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			// Given a private file or directory without the required access.
			root := t.TempDir()
			path := filepath.Join(root, "output")
			if err := os.WriteFile(path, []byte("original"), 0o000); err != nil {
				t.Fatal(err)
			}
			if operation == "create" {
				if err := os.Chmod(root, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(root, 0o700); err != nil {
						t.Error(err)
					}
				})
			}
			// When the adapter attempts its operation.
			var err error
			switch operation {
			case "read":
				_, err = readRegularFile(path)
			case "append":
				var file *os.File
				file, err = openAppendRegular(path)
				if file != nil {
					if closeErr := file.Close(); closeErr != nil {
						t.Fatal(closeErr)
					}
				}
			case "create":
				err = WriteFileAtomic(path, []byte("replacement"), 0o600)
			}
			// Then callers retain the filesystem permission sentinel.
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("%s error = %v, want permission denied", operation, err)
			}
		})
	}
}

func TestAppendRejectsInvalidPath(t *testing.T) {
	t.Parallel()
	file, err := openAppendRegular("invalid\x00path")
	if file != nil || !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("openAppendRegular() = %v, %v, want nil and invalid path", file, err)
	}
}

func TestGitHubOutputsRejectForgedAllowlist(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "output")
	err := WriteGitHubOutputs(path, OutputAllowlist{"bad=name": {}}, map[string]string{"bad=name": "value"})
	if err == nil || err.Error() != "GitHub output name is invalid" {
		t.Fatalf("forged allowlist error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid output created file: %v", err)
	}
}

func TestOutputAllowlistRejectsInvalidNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "1result", "result=value", "result\n"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if got := recover(); got != "invalid GitHub output allowlist name: "+name {
					t.Fatalf("panic = %v, want invalid allowlist name %q", got, name)
				}
			}()
			NewOutputAllowlist(name)
		})
	}
}

func TestTypedJSONValidatorReturnsZeroOnFailure(t *testing.T) {
	t.Parallel()
	cause := errors.New("semantic rejection")
	value, err := decodeTypedJSON([]byte(`"value"`), func(value string) error {
		if value != "value" {
			t.Fatalf("validator input = %q", value)
		}
		return cause
	})
	if value != "" || !errors.Is(err, cause) {
		t.Fatalf("decodeTypedJSON() = %q, %v, want zero and semantic cause", value, err)
	}
}

func TestRedactionIgnoresEmptyExplicitSecret(t *testing.T) {
	t.Parallel()
	if got := RedactSecrets("safe text", ""); got != "safe text" {
		t.Fatalf("empty secret changed safe message to %q", got)
	}
}

func TestRedactionReplacesLongestExplicitSecretFirst(t *testing.T) {
	t.Parallel()
	if got := RedactSecrets("prefix-secret prefix", "prefix", "prefix-secret", ""); got != "[REDACTED] [REDACTED]" {
		t.Fatalf("overlapping secrets leaked a suffix: %q", got)
	}
}

func TestRegularInputRejectsOneByteOverLimit(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "oversized.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxTypedInputSize + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := readRegularFile(path)
	if err == nil || !strings.Contains(err.Error(), "exceeds 16777216 bytes") || data != nil {
		t.Fatalf("readRegularFile() returned %d bytes, %v; want size rejection", len(data), err)
	}
}

func TestAtomicOutputRejectsUnsafePermissions(t *testing.T) {
	t.Parallel()
	for _, permission := range []os.FileMode{0o640, 0o604, os.ModeSymlink | 0o600} {
		t.Run(permission.String(), func(t *testing.T) {
			t.Parallel()
			err := WriteFileAtomic(filepath.Join(t.TempDir(), "output"), []byte("payload"), permission)
			if err == nil || err.Error() != "atomic output permission must not grant group or other access" {
				t.Fatalf("WriteFileAtomic() = %v for mode %v", err, permission)
			}
		})
	}
}

func TestGitHubOutputsEmptyInputContract(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		missingPath bool
	}{{"missing path", true}, {"empty outputs", false}} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "output")
			if test.missingPath {
				path = ""
			}
			err := WriteGitHubOutputs(path, NewOutputAllowlist("result"), nil)
			if test.missingPath {
				if err == nil || err.Error() != "GITHUB_OUTPUT path is required" {
					t.Fatalf("empty path error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("empty outputs created file: %v", err)
			}
		})
	}
}
