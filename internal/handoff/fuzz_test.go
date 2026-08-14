package handoff_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
)

// FuzzHandoffUnmarshalJSON feeds attacker-controlled bytes to the handoff
// contract decoder. Properties asserted for every input:
//   - decoding never panics;
//   - every error carries the handoff-schema-mismatch diagnostic ID;
//   - a failed decode leaves the receiver unchanged;
//   - a successful decode round-trips through marshal and re-decode.
func FuzzHandoffUnmarshalJSON(f *testing.F) {
	for _, fixture := range []string{"valid.json", "digest-mismatch.json", "malformed-digests.json"} {
		contents, err := os.ReadFile(filepath.Join("../../testdata/handoff", fixture))
		if err != nil {
			f.Fatalf("read %s: %v", fixture, err)
		}
		f.Add(contents)
	}

	// Missing digest.value (handoff_test.go TestRejectMissingDigestValueBeforeOpeningPayload).
	f.Add([]byte(`{
  "transport":"github-actions-artifact",
  "artifact_name":"artifact-1",
  "payload_file_name":"payload.bin",
  "payload_kind":"primary-artifact",
  "digest":{"algorithm":"sha256"}
}`))

	// Malformed digest.value spellings (handoff_test.go
	// TestRejectMalformedHandoffDigestWithSchemaDiagnostic).
	for _, value := range []string{
		`"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"`,
		`"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
		`"gaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
		`null`,
		`123`,
	} {
		f.Add([]byte(`{
  "transport":"github-actions-artifact",
  "artifact_name":"artifact-1",
  "payload_file_name":"payload.bin",
  "payload_kind":"primary-artifact",
  "digest":{"algorithm":"sha256","value":` + value + `}
}`))
	}

	// A fully valid contract exercises the success path.
	f.Add([]byte(`{"transport":"github-actions-artifact","artifact_name":"profile-payload-123456789-1","payload_file_name":"artifact.tar.gz","payload_kind":"primary-artifact","digest":{"algorithm":"sha256","value":"8458d7a633b4cb9f781d4afbb11abc6be59f135c4b7798f7b9421e776683350e"}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		sentinel := handoff.Handoff{
			Transport:       "sentinel-transport",
			ArtifactName:    "sentinel-artifact",
			PayloadFileName: "sentinel-payload",
			PayloadKind:     "sentinel-kind",
			Digest: handoff.Digest{
				Algorithm: "sentinel-algorithm",
				Value:     digest.SumSHA256([]byte("sentinel")),
			},
		}

		// Call UnmarshalJSON directly: json.Unmarshal reports syntax errors
		// without invoking the custom unmarshaler, bypassing the diagnostic
		// classification this target asserts.
		contract := sentinel
		err := contract.UnmarshalJSON(data)
		if err != nil {
			if got := handoff.ErrorIDOf(err); got != handoff.HandoffSchemaMismatchID {
				t.Fatalf("primary ID = %q, want %q (error: %v)", got, handoff.HandoffSchemaMismatchID, err)
			}
			if contract != sentinel {
				t.Fatalf("failed decode mutated the receiver: got %+v, want %+v", contract, sentinel)
			}
			return
		}

		encoded, err := json.Marshal(contract)
		if err != nil {
			t.Fatalf("marshal decoded contract: %v", err)
		}
		var reparsed handoff.Handoff
		if err := json.Unmarshal(encoded, &reparsed); err != nil {
			t.Fatalf("re-decode marshaled contract %s: %v", encoded, err)
		}
		if reparsed != contract {
			t.Fatalf("round-trip mismatch: got %+v, want %+v", reparsed, contract)
		}
	})
}

// FuzzValidateSafeBasename asserts that accepted names are canonical
// separator-free basenames and that rejection is deterministic.
func FuzzValidateSafeBasename(f *testing.F) {
	fixtureBytes, err := os.ReadFile("../../testdata/handoff/traversal.json")
	if err != nil {
		f.Fatalf("read traversal fixture: %v", err)
	}
	var traversalNames []string
	if err := json.Unmarshal(fixtureBytes, &traversalNames); err != nil {
		f.Fatalf("decode traversal fixture: %v", err)
	}
	for _, name := range traversalNames {
		f.Add(name)
	}

	// Valid names from testdata/handoff/valid.json.
	f.Add("profile-payload-123456789-1")
	f.Add("artifact.tar.gz")

	f.Fuzz(func(t *testing.T, name string) {
		first := handoff.ValidateSafeBasename(name)
		second := handoff.ValidateSafeBasename(name)
		if (first == nil) != (second == nil) {
			t.Fatalf("ValidateSafeBasename(%q) is nondeterministic: first %v, second %v", name, first, second)
		}
		if first != nil {
			return
		}

		if filepath.Clean(name) != name {
			t.Fatalf("accepted name %q is not canonical (clean: %q)", name, filepath.Clean(name))
		}
		if strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, '\x00') {
			t.Fatalf("accepted name %q contains a path separator or NUL", name)
		}
		if name == "." || name == ".." {
			t.Fatalf("accepted name %q is not a file basename", name)
		}
	})
}
