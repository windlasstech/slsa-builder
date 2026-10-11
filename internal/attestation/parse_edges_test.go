package attestation

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestParseBundleRejectsWrongPayloadType(t *testing.T) {
	t.Parallel()
	raw := fixtureBundle(t)
	old := []byte(`application/vnd.in-toto+json`)
	if !bytes.Contains(raw, old) {
		t.Fatal("fixture has no in-toto payload type")
	}
	raw = bytes.Replace(raw, old, []byte(`application/json`), 1)

	_, err := ParseBundle(raw)

	requireDiagnosticID(t, err, idSignatureInvalid)
	var verification *VerificationError
	if !errors.As(err, &verification) || verification.Diagnostic.Field != "bundle.dsseEnvelope.payloadType" {
		t.Fatalf("error = %v, want payloadType rejection", err)
	}
}

func TestCompareStatementRejectsUnserializableExpectation(t *testing.T) {
	t.Parallel()
	expected := provenance.Statement{Predicate: provenance.Predicate{BuildDefinition: provenance.BuildDefinition{ExternalParameters: json.RawMessage(`{`)}}}
	err := CompareStatement([]byte(`{}`), expected)
	requireDiagnosticID(t, err, idVerifierExecutionFailure)
	var verification *VerificationError
	if !errors.As(err, &verification) || verification.Diagnostic.Field != "expected_statement" {
		t.Fatalf("error = %v, want expected_statement rejection", err)
	}
}

func TestOfflineVerifyPropagatesBundleParseFailure(t *testing.T) {
	t.Parallel()
	request := offlineRequest(t, []byte(`{"mediaType":`))
	_, err := verifyAt(t.Context(), request, fixtureTime)
	requireDiagnosticID(t, err, idSignatureInvalid)
}

func TestParseBundleRejectsMessageSignatureInsteadOfDSSE(t *testing.T) {
	t.Parallel()
	var document map[string]json.RawMessage
	if err := json.Unmarshal(fixtureBundle(t), &document); err != nil {
		t.Fatalf("decode bundle fixture: %v", err)
	}
	delete(document, "dsseEnvelope")
	document["messageSignature"] = json.RawMessage(`{"messageDigest":{"algorithm":"SHA2_256","digest":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},"signature":"AQ=="}`)
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode message-signature bundle: %v", err)
	}

	_, err = ParseBundle(raw)

	requireDiagnosticID(t, err, idSignatureInvalid)
	var verification *VerificationError
	if !errors.As(err, &verification) || verification.Diagnostic.Field != "bundle.dsseEnvelope" {
		t.Fatalf("error = %v, want DSSE-envelope rejection", err)
	}
}
