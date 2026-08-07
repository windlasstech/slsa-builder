package provenance_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	attestSHA = "0123456789abcdef0123456789abcdef01234567"
	sha256Hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func TestValidSLSAv1(t *testing.T) {
	t.Parallel()

	statement := loadStatement(t, "valid-statement.json")
	diagnostics, err := statement.Validate(expectations(nil), nil)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("Validate() diagnostics = %#v, want none", diagnostics)
	}

	got, err := statement.Predicate.CanonicalJSON()
	if err != nil {
		t.Fatalf("Predicate.CanonicalJSON() error = %v", err)
	}
	goldenPath := filepath.Join("..", "..", "testdata", "provenance", "valid-predicate.jcs.json")
	if os.Getenv("UPDATE_PROVENANCE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, got, 0o600); err != nil {
			t.Fatalf("write golden predicate: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden predicate: %v", err)
	}
	if !bytes.Equal(got, bytes.TrimSpace(want)) {
		t.Fatalf("canonical predicate mismatch\n got: %s\nwant: %s", got, bytes.TrimSpace(want))
	}

	parsed, present, err := statement.Subject[0].SHA256()
	if err != nil || !present {
		t.Fatalf("Subject.SHA256() = %q, %t, %v", parsed, present, err)
	}
	if parsed.String() != sha256Hex {
		t.Fatalf("Subject.SHA256() = %q, want %q", parsed, sha256Hex)
	}

	statement.Predicate.BuildDefinition.ResolvedDependencies = []provenance.ResourceDescriptor{
		{
			Name:        "profile-owned",
			Digest:      map[string]string{"sha256": sha256Hex},
			Annotations: map[string]json.RawMessage{"authority": json.RawMessage(`"original"`)},
		},
	}
	if _, err := statement.Validate(expectations(nil), mutatingProfileValidator{}); err != nil {
		t.Fatalf("Validate() with profile error = %v", err)
	}
	if statement.Subject[0].Digest["sha256"] != sha256Hex {
		t.Fatal("profile validator mutated the original subject digest")
	}
	dependency := statement.Predicate.BuildDefinition.ResolvedDependencies[0]
	if dependency.Digest["sha256"] != sha256Hex || string(dependency.Annotations["authority"]) != `"original"` {
		t.Fatal("profile validator mutated the original resolved dependency")
	}
}

func TestCommonRejectionMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fixture  string
		wantID   string
		decodeID bool
	}{
		{name: "duplicate JSON member", fixture: "reject-duplicate-json-member.json", wantID: "windlass.verify.error.duplicate-json-member", decodeID: true},
		{name: "statement type", fixture: "reject-statement-type.json", wantID: "windlass.verify.error.statement-type-invalid"},
		{name: "predicate type", fixture: "reject-predicate-type.json", wantID: "windlass.verify.error.predicate-type-invalid"},
		{name: "unexpected internal parameters", fixture: "reject-internal-parameters.json", wantID: "windlass.verify.error.unexpected-internal-parameters"},
		{name: "zero subjects", fixture: "reject-zero-subjects.json", wantID: "windlass.verify.error.subject-cardinality-invalid"},
		{name: "multiple subjects", fixture: "reject-multiple-subjects.json", wantID: "windlass.verify.error.subject-cardinality-invalid"},
		{name: "timestamp format", fixture: "reject-timestamp-format.json", wantID: "windlass.verify.error.timestamp-format-invalid"},
		{name: "timestamp ordering", fixture: "reject-timestamp-ordering.json", wantID: "windlass.verify.error.timestamp-ordering-invalid"},
		{name: "builder version", fixture: "reject-builder-version.json", wantID: "windlass.verify.error.builder-version-mismatch"},
		{name: "builder dependency", fixture: "reject-builder-dependency.json", wantID: "windlass.verify.error.builder-dependencies-signing-adapter-mismatch"},
		{name: "run invocation", fixture: "reject-run-invocation.json", wantID: "windlass.verify.error.run-invocation-uri-invalid"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := readFixture(t, test.fixture)
			statement, err := provenance.DecodeStatement(data)
			if test.decodeID {
				requireDiagnosticID(t, err, test.wantID)
				return
			}
			if err != nil {
				t.Fatalf("DecodeStatement() error = %v", err)
			}
			if test.name != "builder dependency" {
				useSigstoreGoDependency(&statement)
				updateStatementFixture(t, test.fixture, statement)
			}
			_, err = statement.Validate(expectations(nil), nil)
			requireDiagnosticID(t, err, test.wantID)
		})
	}

	t.Run("bounded clock skew warns and passes", func(t *testing.T) {
		t.Parallel()
		statement := loadStatement(t, "valid-clock-skew.json")
		diagnostics, err := statement.Validate(expectations(nil), nil)
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if len(diagnostics) != 1 || diagnostics[0].ID != diagnostic.IDTimestampClockSkew {
			t.Fatalf("Validate() diagnostics = %#v, want timestamp clock-skew warning", diagnostics)
		}
	})
}

func TestBuilderFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		fixture         string
		corepackVersion *string
	}{
		{name: "direct npm", fixture: "valid-statement.json"},
		{name: "corepack", fixture: "valid-statement-corepack.json", corepackVersion: stringPointer("0.29.4")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			statement := loadStatement(t, test.fixture)
			if _, err := statement.Validate(expectations(test.corepackVersion), nil); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func expectations(corepackVersion *string) provenance.Expectations {
	return provenance.Expectations{
		SourceRepositoryURI: "https://github.com/example/acme-widget",
		Builder: provenance.BuilderExpectations{
			NodeJSVersion:   "v24.0.0",
			CorepackVersion: corepackVersion,
		},
	}
}

func loadStatement(t *testing.T, name string) provenance.Statement {
	t.Helper()
	statement, err := provenance.DecodeStatement(readFixture(t, name))
	if err != nil {
		t.Fatalf("DecodeStatement(%q) error = %v", name, err)
	}
	useSigstoreGoDependency(&statement)
	updateStatementFixture(t, name, statement)
	return statement
}

func useSigstoreGoDependency(statement *provenance.Statement) {
	statement.Predicate.RunDetails.Builder.BuilderDependencies = []provenance.BuilderDependency{{
		URI:         "pkg:golang/github.com/sigstore/sigstore-go@v1.3.0",
		Digest:      map[string]string{"h1": "hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE="},
		Annotations: map[string]string{"role": "signing-adapter"},
	}}
}

func updateStatementFixture(t *testing.T, name string, statement provenance.Statement) {
	t.Helper()
	if os.Getenv("UPDATE_PROVENANCE_FIXTURES") != "1" {
		return
	}
	encoded, err := statement.CanonicalJSON()
	if err != nil {
		t.Fatalf("canonicalize fixture %q: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join("..", "..", "testdata", "provenance", name), encoded, 0o600); err != nil {
		t.Fatalf("write fixture %q: %v", name, err)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "provenance", name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return data
}

func requireDiagnosticID(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected diagnostic %q, got nil", want)
	}
	var identified interface{ DiagnosticID() string }
	if !errors.As(err, &identified) {
		t.Fatalf("error %T does not expose a diagnostic ID: %v", err, err)
	}
	if got := identified.DiagnosticID(); got != want {
		t.Fatalf("diagnostic ID = %q, want %q: %v", got, want, err)
	}
}

func stringPointer(value string) *string {
	return &value
}

type mutatingProfileValidator struct{}

func (mutatingProfileValidator) ValidateSubject(subject provenance.Subject) error {
	subject.Digest["sha256"] = "mutated"
	return nil
}

func (mutatingProfileValidator) ValidateExternalParameters(parameters json.RawMessage) error {
	if len(parameters) > 0 {
		parameters[0] = '['
	}
	return nil
}

func (mutatingProfileValidator) ValidateResolvedDependencies(dependencies []provenance.ResourceDescriptor) error {
	dependencies[0].Digest["sha256"] = "mutated"
	dependencies[0].Annotations["authority"] = json.RawMessage(`"mutated"`)
	return nil
}
