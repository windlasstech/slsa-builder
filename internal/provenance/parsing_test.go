package provenance_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestDecodeStatementDropsUnrecognizedMember(t *testing.T) {
	t.Parallel()

	want := bytes.TrimSpace(readFixture(t, "valid-statement.json"))
	input := append([]byte(`{"extra":true,`), want[1:]...)

	statement, err := provenance.DecodeStatement(input)
	if err != nil {
		t.Fatalf("DecodeStatement() error = %v", err)
	}
	got, err := statement.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded Statement retained unrecognized member or changed known fields\n got: %s\nwant: %s", got, want)
	}
}

func TestValidateRejectsNonemptyInternalParameters(t *testing.T) {
	t.Parallel()

	for _, parameters := range []string{`{"debug":true}`, `[1]`, `[]`, `true`, `1`, `""`, `"null"`, ` `, `{`} {
		t.Run(parameters, func(t *testing.T) {
			t.Parallel()

			statement := loadStatement(t, "valid-statement.json")
			statement.Predicate.BuildDefinition.InternalParameters = json.RawMessage(parameters)
			_, err := statement.Validate(expectations(nil), nil)
			requireDiagnosticID(t, err, "windlass.verify.error.unexpected-internal-parameters")
		})
	}
}

func TestValidateRejectsInternalParametersFixture(t *testing.T) {
	t.Parallel()

	statement := loadStatement(t, "reject-internal-parameters.json")
	_, err := statement.Validate(expectations(nil), nil)
	requireDiagnosticID(t, err, "windlass.verify.error.unexpected-internal-parameters")
}
