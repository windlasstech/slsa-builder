package attestation

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

// FuzzParseBundle asserts the ParseBundle contract for arbitrary attacker-controlled bytes:
// it never panics, every failure carries a classified diagnostic ID, empty input is always
// input-unavailable, and success preserves the input bytes byte-for-byte while exposing a
// strict-JSON Statement payload.
func FuzzParseBundle(f *testing.F) {
	validBundle := readFuzzFixture(f, filepath.Join("..", "..", "testdata", "platform", "contracts", "valid.intoto.jsonl"))
	duplicateBundle := append(
		[]byte(`{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json",`),
		validBundle[1:]...,
	)
	duplicateStatementBundle := mutateFuzzStatementPayload(f, validBundle, func(statement []byte) []byte {
		return append([]byte(`{"_type":"https://in-toto.io/Statement/v1",`), statement[1:]...)
	})

	f.Add(validBundle)
	f.Add(duplicateBundle)
	f.Add(duplicateStatementBundle)
	f.Add(validBundle[:len(validBundle)/2])
	f.Add([]byte{})
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		parsed, err := ParseBundle(raw)
		if err == nil {
			if !bytes.Equal(parsed.BundleBytes(), raw) {
				t.Fatal("ParseBundle success did not preserve the exact input bytes")
			}
			if validateErr := canonicaljson.Validate(parsed.StatementBytes()); validateErr != nil {
				t.Fatalf("ParseBundle success exposed non-strict Statement bytes: %v", validateErr)
			}
			return
		}
		id := fuzzDiagnosticID(t, err)
		switch id {
		case idInputUnavailable, idDuplicateJSONMember, idSignatureInvalid:
		default:
			t.Fatalf("ParseBundle error diagnostic ID = %s, want a classified parse failure", id)
		}
		if len(raw) == 0 && id != idInputUnavailable {
			t.Fatalf("empty input diagnostic ID = %s, want %s", id, idInputUnavailable)
		}
	})
}

// FuzzCompareStatement asserts the CompareStatement contract for arbitrary signed payload bytes
// against a fixed expected Statement: it never panics, every failure carries a classified
// diagnostic ID, and the exact expected Statement bytes always compare equal.
func FuzzCompareStatement(f *testing.F) {
	validStatement := readFuzzFixture(f, filepath.Join("..", "..", "testdata", "provenance", "valid-statement.json"))
	expected, err := provenance.DecodeStatement(validStatement)
	if err != nil {
		f.Fatalf("decode expected Statement fixture: %v", err)
	}

	f.Add(validStatement)
	// bytes.Replace mismatch mutation in the shape of TestVerifyStatementMismatch; the
	// deterministic-f03 token lives in expected-statement.json, so the replacement targets
	// the subject name token present in this fixture.
	f.Add(bytes.Replace(validStatement, []byte("pkg:generic/example@1.0.0"), []byte("different-fixture"), 1))
	f.Add(append([]byte(`{"_type":"https://in-toto.io/Statement/v1",`), validStatement[1:]...))
	f.Add(validStatement[:len(validStatement)/2])
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, actual []byte) {
		err := CompareStatement(actual, expected)
		if bytes.Equal(actual, validStatement) && err != nil {
			t.Fatalf("exact valid Statement bytes failed comparison: %v", err)
		}
		if err == nil {
			return
		}
		switch fuzzDiagnosticID(t, err) {
		case idStatementAssemblyMismatch, idDuplicateJSONMember, idSignatureInvalid:
		default:
			t.Fatalf("CompareStatement error diagnostic ID = %s, want a classified comparison failure", fuzzDiagnosticID(t, err))
		}
	})
}

func fuzzDiagnosticID(t *testing.T, err error) string {
	t.Helper()
	var diagnosticError interface{ DiagnosticID() string }
	if !errors.As(err, &diagnosticError) {
		t.Fatalf("error %T does not expose a diagnostic ID: %v", err, err)
	}
	return diagnosticError.DiagnosticID()
}

func readFuzzFixture(f *testing.F, path string) []byte {
	f.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		f.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func mutateFuzzStatementPayload(f *testing.F, data []byte, mutate func([]byte) []byte) []byte {
	f.Helper()
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		f.Fatalf("decode bundle mutation fixture: %v", err)
	}
	envelope, ok := document["dsseEnvelope"].(map[string]any)
	if !ok {
		f.Fatal("fixture dsseEnvelope is not an object")
	}
	payload, ok := envelope["payload"].(string)
	if !ok {
		f.Fatal("fixture DSSE payload is not a string")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		f.Fatalf("decode fixture DSSE payload: %v", err)
	}
	envelope["payload"] = base64.StdEncoding.EncodeToString(mutate(decoded))
	mutated, err := json.Marshal(document)
	if err != nil {
		f.Fatalf("encode bundle mutation fixture: %v", err)
	}
	return mutated
}
