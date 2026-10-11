package provenance_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const sha512Hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" +
	"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestStatementCanonicalJSON(t *testing.T) {
	t.Parallel()

	t.Run("exact bytes and repeat stability", func(t *testing.T) {
		t.Parallel()

		statement := loadStatement(t, "valid-statement.json")
		got, err := statement.CanonicalJSON()
		if err != nil {
			t.Fatalf("Statement.CanonicalJSON() error = %v", err)
		}
		want := bytes.TrimSpace(readFixture(t, "valid-statement.json"))
		if !bytes.Equal(got, want) {
			t.Fatalf("Statement.CanonicalJSON() mismatch\n got: %s\nwant: %s", got, want)
		}
		repeat, err := statement.CanonicalJSON()
		if err != nil {
			t.Fatalf("Statement.CanonicalJSON() second call error = %v", err)
		}
		if !bytes.Equal(got, repeat) {
			t.Fatalf("Statement.CanonicalJSON() is not stable\nfirst:  %s\nsecond: %s", got, repeat)
		}
	})

	t.Run("marshal error", func(t *testing.T) {
		t.Parallel()

		statement := loadStatement(t, "valid-statement.json")
		statement.Predicate.BuildDefinition.ExternalParameters = json.RawMessage(`{`)
		_, err := statement.CanonicalJSON()
		if err == nil {
			t.Fatal("Statement.CanonicalJSON() unexpectedly succeeded")
		}
		if !strings.HasPrefix(err.Error(), "encode provenance JSON: ") {
			t.Fatalf("Statement.CanonicalJSON() error = %q, want encode provenance JSON wrap", err)
		}
	})
}

func TestSubjectSHA512(t *testing.T) {
	t.Parallel()

	t.Run("digest absent", func(t *testing.T) {
		t.Parallel()

		subject := provenance.Subject{Name: "pkg:generic/example@1.0.0", Digest: map[string]string{"sha256": sha256Hex}}
		parsed, present, err := subject.SHA512()
		if err != nil || present {
			t.Fatalf("Subject.SHA512() = %q, %t, %v, want zero value, absent, nil error", parsed, present, err)
		}
		if parsed != (digest.SHA512{}) {
			t.Fatalf("Subject.SHA512() = %q, want zero digest when absent", parsed)
		}
	})

	t.Run("valid digest", func(t *testing.T) {
		t.Parallel()

		statement := loadStatement(t, "valid-statement.json")
		parsed, present, err := statement.Subject[0].SHA512()
		if err != nil || !present {
			t.Fatalf("Subject.SHA512() = %q, %t, %v", parsed, present, err)
		}
		if parsed.String() != sha512Hex {
			t.Fatalf("Subject.SHA512() = %q, want %q", parsed, sha512Hex)
		}
	})

	t.Run("malformed hex", func(t *testing.T) {
		t.Parallel()

		subject := provenance.Subject{Name: "pkg:generic/example@1.0.0", Digest: map[string]string{"sha512": "zz"}}
		parsed, present, err := subject.SHA512()
		if err == nil {
			t.Fatal("Subject.SHA512() unexpectedly succeeded")
		}
		if !present {
			t.Fatal("Subject.SHA512() present = false, want true when the digest key exists")
		}
		if parsed != (digest.SHA512{}) {
			t.Fatalf("Subject.SHA512() = %q, want zero digest on error", parsed)
		}
		if !errors.Is(err, digest.ErrInvalidEncoding) {
			t.Fatalf("Subject.SHA512() error = %v, want wrapped ErrInvalidEncoding", err)
		}
		if !strings.HasPrefix(err.Error(), "subject sha512: ") {
			t.Fatalf("Subject.SHA512() error = %q, want subject sha512 wrap", err)
		}
	})
}

func TestSubjectSHA256Branches(t *testing.T) {
	t.Parallel()

	t.Run("digest absent", func(t *testing.T) {
		t.Parallel()

		subject := provenance.Subject{Name: "pkg:generic/example@1.0.0", Digest: map[string]string{"sha512": sha512Hex}}
		parsed, present, err := subject.SHA256()
		if err != nil || present {
			t.Fatalf("Subject.SHA256() = %q, %t, %v, want zero value, absent, nil error", parsed, present, err)
		}
		if parsed != (digest.SHA256{}) {
			t.Fatalf("Subject.SHA256() = %q, want zero digest when absent", parsed)
		}
	})

	t.Run("malformed hex", func(t *testing.T) {
		t.Parallel()

		subject := provenance.Subject{Name: "pkg:generic/example@1.0.0", Digest: map[string]string{"sha256": "zz"}}
		parsed, present, err := subject.SHA256()
		if err == nil {
			t.Fatal("Subject.SHA256() unexpectedly succeeded")
		}
		if !present {
			t.Fatal("Subject.SHA256() present = false, want true when the digest key exists")
		}
		if parsed != (digest.SHA256{}) {
			t.Fatalf("Subject.SHA256() = %q, want zero digest on error", parsed)
		}
		if !errors.Is(err, digest.ErrInvalidEncoding) {
			t.Fatalf("Subject.SHA256() error = %v, want wrapped ErrInvalidEncoding", err)
		}
		if !strings.HasPrefix(err.Error(), "subject sha256: ") {
			t.Fatalf("Subject.SHA256() error = %q, want subject sha256 wrap", err)
		}
	})
}

func TestDecodeStatementRejectsUnknownField(t *testing.T) {
	t.Parallel()

	_, err := provenance.DecodeStatement([]byte(`{"_type":"https://in-toto.io/Statement/v1","surprise":1}`))
	if err == nil {
		t.Fatal("DecodeStatement() unexpectedly succeeded")
	}
	if !strings.HasPrefix(err.Error(), "decode closed SLSA Statement: ") {
		t.Fatalf("DecodeStatement() error = %q, want decode closed SLSA Statement wrap", err)
	}
}

func TestValidateProfileErrorPropagation(t *testing.T) {
	t.Parallel()

	errRejected := errors.New("profile rejection")
	tests := []struct {
		name      string
		validator provenance.ProfileValidator
	}{
		{name: "subject", validator: failingProfileValidator{subjectErr: errRejected}},
		{name: "external parameters", validator: failingProfileValidator{parametersErr: errRejected}},
		{name: "resolved dependencies", validator: failingProfileValidator{dependenciesErr: errRejected}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			statement := loadStatement(t, "valid-statement.json")
			_, err := statement.Validate(expectations(nil), test.validator)
			if !errors.Is(err, errRejected) {
				t.Fatalf("Validate() error = %v, want propagated profile rejection", err)
			}
		})
	}
}

func TestValidateRejectsEmptyInternalParameters(t *testing.T) {
	t.Parallel()

	statement := loadStatement(t, "valid-statement.json")
	statement.Predicate.BuildDefinition.InternalParameters = nil
	_, err := statement.Validate(expectations(nil), nil)
	requireDiagnosticID(t, err, "windlass.verify.error.unexpected-internal-parameters")
}

type failingProfileValidator struct {
	subjectErr      error
	parametersErr   error
	dependenciesErr error
}

func (validator failingProfileValidator) ValidateSubject(provenance.Subject) error {
	return validator.subjectErr
}

func (validator failingProfileValidator) ValidateExternalParameters(json.RawMessage) error {
	return validator.parametersErr
}

func (validator failingProfileValidator) ValidateResolvedDependencies([]provenance.ResourceDescriptor) error {
	return validator.dependenciesErr
}

func TestValidationErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		diagnostic diagnostic.Diagnostic
		want       string
	}{
		{
			name: "without field",
			diagnostic: diagnostic.Diagnostic{
				ID:      "windlass.verify.error.digest-mismatch",
				Message: "digests differ",
			},
			want: "windlass.verify.error.digest-mismatch: digests differ",
		},
		{
			name: "with field",
			diagnostic: diagnostic.Diagnostic{
				ID:      "windlass.verify.error.digest-mismatch",
				Field:   "subject[0].digest",
				Message: "digests differ",
			},
			want: "windlass.verify.error.digest-mismatch: subject[0].digest: digests differ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := &provenance.ValidationError{Diagnostic: test.diagnostic}
			if got := err.Error(); got != test.want {
				t.Fatalf("ValidationError.Error() = %q, want %q", got, test.want)
			}
			if got := err.DiagnosticID(); got != test.diagnostic.ID {
				t.Fatalf("ValidationError.DiagnosticID() = %q, want %q", got, test.diagnostic.ID)
			}
		})
	}
}
