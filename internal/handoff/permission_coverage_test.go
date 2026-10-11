package handoff_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
)

func TestVerifyOneFilePermissionFailures(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement for an unprivileged user")
	}
	for _, test := range []struct {
		name                       string
		directoryMode, payloadMode os.FileMode
		operation                  string
	}{
		{"directory cannot be searched", 0o400, 0o600, "read artifact directory: "},
		{"payload cannot be read", 0o700, 0, "open artifact payload: "},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := materialize(t, map[string]string{"payload.bin": "payload"})
			payload := filepath.Join(directory, "payload.bin")
			t.Cleanup(func() {
				if err := os.Chmod(directory, 0o700); err != nil {
					t.Errorf("restore directory permissions: %v", err)
				}
				if err := os.Chmod(payload, 0o600); err != nil {
					t.Errorf("restore payload permissions: %v", err)
				}
			})
			if err := os.Chmod(payload, test.payloadMode); err != nil {
				t.Fatalf("set payload permissions: %v", err)
			}
			if err := os.Chmod(directory, test.directoryMode); err != nil {
				t.Fatalf("set directory permissions: %v", err)
			}

			got, err := handoff.VerifyOneFile(directory, validContract("payload.bin", digest.SumSHA256([]byte("payload"))))

			if !errors.Is(err, os.ErrPermission) || !strings.HasPrefix(err.Error(), test.operation) {
				t.Fatalf("VerifyOneFile error = %v, want permission failure at %q", err, test.operation)
			}
			if got != nil || handoff.ErrorIDOf(err) != "" {
				t.Fatalf("filesystem rejection returned payload %q or diagnostic %q", got, handoff.ErrorIDOf(err))
			}
		})
	}
}

func TestDecodeRejectsSemanticallyInvalidContract(t *testing.T) {
	t.Parallel()
	contract := validContract("payload.bin", digest.SumSHA256(nil))
	contract.Transport = "s3"
	encoded, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("encode invalid contract: %v", err)
	}

	var decoded handoff.Handoff
	err = json.Unmarshal(encoded, &decoded)

	assertSchemaMismatch(t, err)
	if want := `handoff schema mismatch: validate handoff JSON: handoff transport "s3" is not allowed`; err.Error() != want {
		t.Fatalf("decode error = %v, want %q", err, want)
	}
}

func TestSafeBasenameRejectsNUL(t *testing.T) {
	t.Parallel()
	err := handoff.ValidateSafeBasename("payload\x00.bin")
	if err == nil || err.Error() != "payload file name contains NUL" {
		t.Fatalf("ValidateSafeBasename error = %v, want NUL rejection", err)
	}
}

func TestSafeBasenameRejectsDriveRelativePath(t *testing.T) {
	t.Parallel()
	err := handoff.ValidateSafeBasename("C:payload.bin")
	if err == nil || err.Error() != `payload file name "C:payload.bin" is not canonical` {
		t.Fatalf("ValidateSafeBasename error = %v, want drive-relative rejection", err)
	}
}
