package handoff_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
)

// Every VerificationError constructor (schemaMismatchError and the
// digest-mismatch literal in VerifyOneFile) assigns a non-nil Err, so a nil
// inner error cannot occur through the package surface; this test pins the
// passthrough format for the non-nil case.
func TestVerificationErrorPassthrough(t *testing.T) {
	t.Parallel()

	inner := errors.New("inner failure detail")
	identified := &handoff.VerificationError{
		PrimaryID: handoff.DigestMismatchID,
		Err:       inner,
	}

	if got := identified.Error(); got != inner.Error() {
		t.Fatalf("Error() = %q, want inner error text %q", got, inner.Error())
	}
	if !errors.Is(identified, inner) {
		t.Fatalf("errors.Is(%v, inner) = false, want true", identified)
	}
	if got := identified.DiagnosticID(); got != handoff.DigestMismatchID {
		t.Fatalf("DiagnosticID() = %q, want %q", got, handoff.DigestMismatchID)
	}
}

func TestErrorIDOfPlainError(t *testing.T) {
	t.Parallel()

	if got := handoff.ErrorIDOf(errors.New("plain failure")); got != "" {
		t.Fatalf("ErrorIDOf() = %q, want empty ID for an unidentified error", got)
	}
	if got := handoff.ErrorIDOf(nil); got != "" {
		t.Fatalf("ErrorIDOf(nil) = %q, want empty ID", got)
	}
}

func TestValidateOrderedFailures(t *testing.T) {
	t.Parallel()

	valid := validContract("payload.bin", digest.SumSHA256([]byte("payload bytes")))

	tests := []struct {
		name    string
		mutate  func(*handoff.Handoff)
		wantErr string
	}{
		{name: "valid contract", mutate: func(*handoff.Handoff) {}},
		{
			name:    "transport not allowed",
			mutate:  func(contract *handoff.Handoff) { contract.Transport = "s3" },
			wantErr: `handoff transport "s3" is not allowed`,
		},
		{
			name:    "artifact name empty",
			mutate:  func(contract *handoff.Handoff) { contract.ArtifactName = "" },
			wantErr: "invalid artifact name: payload file name is empty",
		},
		{
			name:    "artifact name resembles URL or volume path",
			mutate:  func(contract *handoff.Handoff) { contract.ArtifactName = "artifact:1" },
			wantErr: `artifact name "artifact:1" resembles a URL or volume path`,
		},
		{
			name:    "artifact name is an ID-only handle",
			mutate:  func(contract *handoff.Handoff) { contract.ArtifactName = "123456789" },
			wantErr: `artifact name "123456789" is an ID-only handle`,
		},
		{
			name:    "payload file name empty",
			mutate:  func(contract *handoff.Handoff) { contract.PayloadFileName = "" },
			wantErr: "payload file name is empty",
		},
		{
			name:    "payload file name contains a separator",
			mutate:  func(contract *handoff.Handoff) { contract.PayloadFileName = "../payload.bin" },
			wantErr: `payload file name "../payload.bin" contains a path separator`,
		},
		{
			name:    "payload kind empty",
			mutate:  func(contract *handoff.Handoff) { contract.PayloadKind = "" },
			wantErr: "payload kind is empty",
		},
		{
			name:    "digest algorithm not allowed",
			mutate:  func(contract *handoff.Handoff) { contract.Digest.Algorithm = "sha512" },
			wantErr: `handoff digest algorithm "sha512" is not allowed`,
		},
		{
			name: "earliest failing field wins",
			mutate: func(contract *handoff.Handoff) {
				contract.Transport = "s3"
				contract.PayloadKind = ""
				contract.Digest.Algorithm = "sha512"
			},
			wantErr: `handoff transport "s3" is not allowed`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			contract := valid
			test.mutate(&contract)
			err := contract.Validate()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", test.wantErr)
			}
			if err.Error() != test.wantErr {
				t.Fatalf("Validate() error = %q, want %q", err.Error(), test.wantErr)
			}
		})
	}
}

func TestVerifyOneFileRejections(t *testing.T) {
	t.Parallel()

	expected := digest.SumSHA256([]byte("payload bytes"))

	t.Run("wrong payload file name", func(t *testing.T) {
		t.Parallel()

		directory := materialize(t, map[string]string{"other.bin": "payload bytes"})
		_, err := handoff.VerifyOneFile(directory, validContract("payload.bin", expected))
		if err == nil {
			t.Fatal("VerifyOneFile() error = nil, want a file name mismatch error")
		}
		want := `artifact payload file is "other.bin", want "payload.bin"`
		if err.Error() != want {
			t.Fatalf("VerifyOneFile() error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("directory as payload", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		if err := os.Mkdir(filepath.Join(directory, "payload.bin"), 0o700); err != nil {
			t.Fatalf("create payload directory: %v", err)
		}
		_, err := handoff.VerifyOneFile(directory, validContract("payload.bin", expected))
		if err == nil {
			t.Fatal("VerifyOneFile() error = nil, want a non-regular-file rejection")
		}
		want := `artifact payload "payload.bin" is not a regular file`
		if err.Error() != want {
			t.Fatalf("VerifyOneFile() error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("symlink payload", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		target := filepath.Join(directory, "target.bin")
		if err := os.WriteFile(target, []byte("payload bytes"), 0o600); err != nil {
			t.Fatalf("write symlink target: %v", err)
		}
		if err := os.Symlink(target, filepath.Join(directory, "payload.bin")); err != nil {
			t.Fatalf("create payload symlink: %v", err)
		}
		// Remove the target so the symlink is the sole directory entry.
		if err := os.Remove(target); err != nil {
			t.Fatalf("remove symlink target: %v", err)
		}

		_, err := handoff.VerifyOneFile(directory, validContract("payload.bin", expected))
		if err == nil {
			t.Fatal("VerifyOneFile() error = nil, want a non-regular-file rejection")
		}
		want := `artifact payload "payload.bin" is not a regular file`
		if err.Error() != want {
			t.Fatalf("VerifyOneFile() error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("invalid contract rejected before filesystem access", func(t *testing.T) {
		t.Parallel()

		contract := validContract("payload.bin", expected)
		contract.Transport = "s3"
		missing := filepath.Join(t.TempDir(), "must-not-open")

		_, err := handoff.VerifyOneFile(missing, contract)
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("invalid contract reached filesystem access: %v", err)
		}
		assertSchemaMismatch(t, err)
	})

	t.Run("missing artifact directory", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "absent")
		_, err := handoff.VerifyOneFile(missing, validContract("payload.bin", expected))
		if err == nil {
			t.Fatal("VerifyOneFile() error = nil, want an open-directory error")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("VerifyOneFile() error = %v, want errors.Is match for os.ErrNotExist", err)
		}
		if got := handoff.ErrorIDOf(err); got != "" {
			t.Fatalf("ErrorIDOf() = %q, want empty ID for a filesystem error", got)
		}
	})
}
