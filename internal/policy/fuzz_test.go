package policy_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/policy"
)

// policySchemaInvalidID is the stable diagnostic ID for closed-schema and semantic rejections.
const policySchemaInvalidID = "windlass.verify.error.policy-schema-invalid"

// FuzzDecodeExplicitPolicy asserts the trust-boundary invariants of the closed explicit verifier
// policy decoder: it never panics, every rejection carries a classified diagnostic ID, and every
// accepted policy round-trips through json.Marshal back into an accepted policy.
func FuzzDecodeExplicitPolicy(f *testing.F) {
	for _, name := range []string{
		"explicit-valid.json",
		"explicit-pinned-valid.json",
		"explicit-unknown-member.json",
		"explicit-duplicate-member.json",
		"explicit-tuf-null-pinned-member.json",
		"explicit-tuf-empty-pinned-member.json",
		"explicit-missing-source-ref.json",
	} {
		f.Add(readFuzzPolicyFixture(f, name))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		decoded, err := policy.DecodeExplicitPolicy(data)
		if err != nil {
			requireClassifiedPolicyError(t, err)
			return
		}
		encoded, err := json.Marshal(decoded)
		if err != nil {
			t.Fatalf("marshal decoded explicit policy: %v", err)
		}
		if _, err := policy.DecodeExplicitPolicy(encoded); err != nil {
			t.Fatalf("re-decode marshaled explicit policy: %v", err)
		}
	})
}

// FuzzDecodeReleaseManifestExpectation asserts the same invariants for the closed release manifest
// expectation decoder, plus the registered-profile contract: a successful decode must name a
// producer profile registered in the supplied registry.
func FuzzDecodeReleaseManifestExpectation(f *testing.F) {
	for _, name := range []string{
		"manifest-expectation-valid.json",
		"manifest-expectation-unregistered-profile.json",
	} {
		f.Add(readFuzzPolicyFixture(f, name))
	}
	registry := producerRegistry{"js-ts-npm-package": {}}
	f.Fuzz(func(t *testing.T, data []byte) {
		decoded, err := policy.DecodeReleaseManifestExpectation(data, registry)
		if err != nil {
			requireClassifiedPolicyError(t, err)
			return
		}
		if !registry.IsRegisteredProducerProfile(decoded.ProducerProfile.Profile) {
			t.Fatalf("decode succeeded for unregistered producer profile %q", decoded.ProducerProfile.Profile)
		}
		encoded, err := json.Marshal(decoded)
		if err != nil {
			t.Fatalf("marshal decoded manifest expectation: %v", err)
		}
		if _, err := policy.DecodeReleaseManifestExpectation(encoded, registry); err != nil {
			t.Fatalf("re-decode marshaled manifest expectation: %v", err)
		}
	})
}

// readFuzzPolicyFixture loads a seed corpus entry from the shared policy fixture directory.
func readFuzzPolicyFixture(f *testing.F, name string) []byte {
	f.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "policy", name))
	if err != nil {
		f.Fatalf("read policy fixture %q: %v", name, err)
	}
	return data
}

// requireClassifiedPolicyError requires every decoder rejection to expose one of the two stable
// diagnostic IDs the policy decode pipeline may produce: the closed-schema ID or the
// duplicate-JSON-member ID surfaced from canonicaljson validation.
func requireClassifiedPolicyError(t *testing.T, err error) {
	t.Helper()
	var identified interface{ DiagnosticID() string }
	if !errors.As(err, &identified) {
		t.Fatalf("error %T does not expose a diagnostic ID: %v", err, err)
	}
	switch id := identified.DiagnosticID(); id {
	case policySchemaInvalidID, canonicaljson.DuplicateJSONMemberID:
	default:
		t.Fatalf("diagnostic ID = %q, want %q or %q: %v", id, policySchemaInvalidID, canonicaljson.DuplicateJSONMemberID, err)
	}
}
