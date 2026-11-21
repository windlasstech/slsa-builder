package provenance_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
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

func TestDecodeStatementIgnoresUnknownField(t *testing.T) {
	t.Parallel()

	statement, err := provenance.DecodeStatement([]byte(`{"_type":"https://in-toto.io/Statement/v1","surprise":1}`))
	if err != nil {
		t.Fatalf("DecodeStatement() error = %v", err)
	}
	if statement.Type != provenance.StatementType {
		t.Fatalf("Statement.Type = %q, want %q", statement.Type, provenance.StatementType)
	}
}

func TestResourceDescriptorJSONContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		encoded              string
		wantUnexpectedFields bool
	}{
		{
			name:    "known members",
			encoded: `{"name":"lockfile","uri":"file:///package.json","digest":{"sha256":"` + sha256Hex + `"},"annotations":{"authority":"registry"}}`,
		},
		{
			name:                 "unknown member",
			encoded:              `{"name":"lockfile","uri":"file:///package.json","digest":{"sha256":"` + sha256Hex + `"},"annotations":{"authority":"registry"},"downloadLocation":"https://example.com/package.json"}`,
			wantUnexpectedFields: true,
		},
		{
			name:    "known media type",
			encoded: `{"name":"lockfile","uri":"file:///package.json","digest":{"sha256":"` + sha256Hex + `"},"annotations":{"authority":"registry"},"mediaType":"application/json"}`,
		},
		{
			name:                 "case-variant member",
			encoded:              `{"Name":"lockfile","uri":"file:///package.json","digest":{"sha256":"` + sha256Hex + `"},"annotations":{"authority":"registry"}}`,
			wantUnexpectedFields: true,
		},
	}
	wantDigest := map[string]string{"sha256": sha256Hex}
	wantAnnotations := map[string]json.RawMessage{"authority": json.RawMessage(`"registry"`)}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var descriptor provenance.ResourceDescriptor
			if err := json.Unmarshal([]byte(test.encoded), &descriptor); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if descriptor.Name != "lockfile" {
				t.Fatalf("ResourceDescriptor.Name = %q, want %q", descriptor.Name, "lockfile")
			}
			if descriptor.URI != "file:///package.json" {
				t.Fatalf("ResourceDescriptor.URI = %q, want %q", descriptor.URI, "file:///package.json")
			}
			if !reflect.DeepEqual(descriptor.Digest, wantDigest) {
				t.Fatalf("ResourceDescriptor.Digest = %#v, want %#v", descriptor.Digest, wantDigest)
			}
			if !reflect.DeepEqual(descriptor.Annotations, wantAnnotations) {
				t.Fatalf("ResourceDescriptor.Annotations = %#v, want %#v", descriptor.Annotations, wantAnnotations)
			}
			if descriptor.HasUnexpectedFields() != test.wantUnexpectedFields {
				t.Fatalf(
					"ResourceDescriptor.HasUnexpectedFields() = %t, want %t",
					descriptor.HasUnexpectedFields(),
					test.wantUnexpectedFields,
				)
			}
		})
	}
}

func TestResourceDescriptorJSONRejectsWrongFieldType(t *testing.T) {
	t.Parallel()

	descriptor := provenance.ResourceDescriptor{
		Name:        "lockfile",
		URI:         "file:///package.json",
		Digest:      map[string]string{"sha256": sha256Hex},
		Annotations: map[string]json.RawMessage{"authority": json.RawMessage(`"registry"`)},
	}
	original := descriptor
	original.Digest = make(map[string]string, len(descriptor.Digest))
	for algorithm, value := range descriptor.Digest {
		original.Digest[algorithm] = value
	}
	original.Annotations = make(map[string]json.RawMessage, len(descriptor.Annotations))
	for name, value := range descriptor.Annotations {
		original.Annotations[name] = append(json.RawMessage(nil), value...)
	}

	err := json.Unmarshal([]byte(`{"name":1,"uri":"file:///changed.json"}`), &descriptor)
	if err == nil {
		t.Fatal("json.Unmarshal() unexpectedly succeeded")
	}
	if !strings.HasPrefix(err.Error(), "decode resource descriptor: ") {
		t.Fatalf("json.Unmarshal() error = %q, want decode resource descriptor wrap", err)
	}
	var typeError *json.UnmarshalTypeError
	if !errors.As(err, &typeError) {
		t.Fatalf("json.Unmarshal() error = %v, want *json.UnmarshalTypeError", err)
	}
	if typeError.Value != "number" {
		t.Fatalf("json.UnmarshalTypeError.Value = %q, want %q", typeError.Value, "number")
	}
	if !reflect.DeepEqual(descriptor, original) {
		t.Fatalf("ResourceDescriptor after failed decode = %#v, want original %#v", descriptor, original)
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

func TestValidateInternalParametersGitHubContract(t *testing.T) {
	t.Parallel()

	const valid = `{"github":{"event_name":"workflow_dispatch","repository_id":"123456789","repository_owner_id":"987654321"}}`
	for _, test := range []struct {
		name       string
		parameters string
		valid      bool
	}{
		{name: "closed github object", parameters: valid, valid: true},
		{name: "whitespace around closed github object", parameters: " \n" + valid + "\t", valid: true},
		{name: "absent", parameters: ""},
		{name: "null", parameters: "null"},
		{name: "empty object", parameters: "{}"},
		{name: "null github", parameters: `{"github":null}`},
		{name: "empty github", parameters: `{"github":{}}`},
		{name: "additional top-level member", parameters: `{"github":{"event_name":"push","repository_id":"123456789","repository_owner_id":"987654321"},"debug":true}`},
		{name: "additional github member", parameters: `{"github":{"event_name":"push","repository_id":"123456789","repository_owner_id":"987654321","runner":"ubuntu-24.04"}}`},
		{name: "missing owner identifier", parameters: `{"github":{"event_name":"push","repository_id":"123456789"}}`},
		{name: "non-string repository identifier", parameters: `{"github":{"event_name":"push","repository_id":123456789,"repository_owner_id":"987654321"}}`},
		{name: "actor field", parameters: `{"github":{"event_name":"push","repository_id":"123456789","repository_owner_id":"987654321","actor_id":"42"}}`},
		{name: "triggering actor field", parameters: `{"github":{"event_name":"push","repository_id":"123456789","repository_owner_id":"987654321","triggering_actor_id":"42"}}`},
		{name: "case-variant member", parameters: `{"github":{"event_name":"push","Repository_Id":"123456789","repository_owner_id":"987654321"}}`},
		{name: "non-object github", parameters: `{"github":[]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Given a Statement carrying the case's internalParameters value.
			statement := loadStatement(t, "valid-statement.json")
			statement.Predicate.BuildDefinition.InternalParameters = json.RawMessage(test.parameters)

			// When common validation evaluates the closed GitHub invocation record.
			_, err := statement.Validate(expectations(nil), nil)

			// Then only the exact three-string GitHub object is accepted.
			if test.valid {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			requireDiagnosticID(t, err, "windlass.verify.error.unexpected-internal-parameters")
		})
	}
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
