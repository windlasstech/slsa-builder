package npmprofile

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

// Fixed auxiliary inputs for the registry and OIDC decoder fuzz targets. The
// package and version reuse the registry_client_test.go packument fixture
// identity; the workflow ref reuses the oidc_client_test.go caller identity.
const (
	fuzzRegistryPackage = "@windlass/slsa-builder"
	fuzzRegistryVersion = "1.2.2"
	fuzzWorkflowRef     = "windlasstech/caller/.github/workflows/release.yml@refs/tags/v1.2.3"
)

// fuzzRegistryPackumentErrors is the closed set of plain errors
// decodeRegistryPackumentResponse may return: every rejection path in the
// decoder is one of these three errors.New values.
var fuzzRegistryPackumentErrors = map[string]bool{
	"registry packument is malformed":                 true,
	"registry packument identity is malformed":        true,
	"registry version metadata identity is malformed": true,
}

// fuzzRegistryAttestationErrors is the closed set of plain errors
// decodeRegistryAttestationResponse may return.
var fuzzRegistryAttestationErrors = map[string]bool{
	"registry attestation response is malformed":           true,
	"registry attestation response has an invalid shape":   true,
	"registry attestation response contains trailing data": true,
	"registry attestation entry is malformed":              true,
}

// fuzzGitHubOIDCErrors is the closed set of plain errors
// decodeGitHubOIDCResponse may return once the input is strict JSON. Inputs
// that fail strict-JSON validation return the canonicaljson error instead.
var fuzzGitHubOIDCErrors = map[string]bool{
	"invalid GitHub OIDC response":     true,
	"invalid GitHub OIDC token":        true,
	"invalid GitHub OIDC claims":       true,
	"caller workflow claim mismatch":   true,
	"caller workflow claim has no ref": true,
	"caller workflow path is invalid":  true,
}

// fuzzNPMExchangeErrors is the closed set of plain errors
// decodeNPMExchangeResponse may return once the input is strict JSON. A JSON
// value of the wrong type surfaces as a *json.UnmarshalTypeError instead.
var fuzzNPMExchangeErrors = map[string]bool{
	"multiple JSON values":               true,
	"invalid npm OIDC exchange response": true,
}

// FuzzDecodeRegistryPackumentResponse feeds attacker-controlled bytes to the
// pure packument decoder with the fixture package identity. Properties
// asserted for every input:
//   - decoding never panics and is deterministic;
//   - every rejection is one of the decoder's three documented plain errors;
//   - on success the package exists, and when the version also exists the
//     returned identity matches the fixed package name and version exactly.
func FuzzDecodeRegistryPackumentResponse(f *testing.F) {
	// Valid packument (registry_client_test.go TestRegistryMetadataPreflight).
	f.Add([]byte(`{"name":"@windlass/slsa-builder","versions":{"1.2.2":{"name":"@windlass/slsa-builder","version":"1.2.2","dist":{"integrity":"sha512-YQ==","tarball":"https://registry.npmjs.org/example.tgz"}}}}`))
	// Version-absent variant: existing package, no matching version member.
	f.Add([]byte(`{"name":"@windlass/slsa-builder","versions":{}}`))
	// Name-mismatch variant: packument identity does not match the request.
	f.Add([]byte(`{"name":"@windlass/other","versions":{}}`))
	// Malformed JSON.
	f.Add([]byte(`{"name":"@windlass/slsa-builder","versions":`))

	f.Fuzz(func(t *testing.T, data []byte) {
		state, err := decodeRegistryPackumentResponse(data, fuzzRegistryPackage, fuzzRegistryVersion)
		secondState, secondErr := decodeRegistryPackumentResponse(data, fuzzRegistryPackage, fuzzRegistryVersion)
		if (err == nil) != (secondErr == nil) {
			t.Fatalf("decodeRegistryPackumentResponse is nondeterministic for %q", data)
		}
		if err != nil {
			if secondErr == nil || err.Error() != secondErr.Error() {
				t.Fatalf("decodeRegistryPackumentResponse error is nondeterministic for %q", data)
			}
			if !fuzzRegistryPackumentErrors[err.Error()] {
				t.Fatalf("rejection is not a documented decoder error: %v", err)
			}
			return
		}
		if state.PackageExists != secondState.PackageExists || state.VersionExists != secondState.VersionExists {
			t.Fatalf("decodeRegistryPackumentResponse state is nondeterministic for %q", data)
		}
		if (state.Version == nil) != (secondState.Version == nil) {
			t.Fatalf("decodeRegistryPackumentResponse version metadata is nondeterministic for %q", data)
		}
		if state.Version != nil && *state.Version != *secondState.Version {
			t.Fatalf("decodeRegistryPackumentResponse version metadata is nondeterministic for %q", data)
		}
		if !state.PackageExists {
			t.Fatal("successful decode reported an absent package")
		}
		if state.VersionExists {
			if state.Version == nil {
				t.Fatal("version exists but carries no metadata")
			}
			if state.Version.Name != fuzzRegistryPackage || state.Version.Version != fuzzRegistryVersion {
				t.Fatalf("accepted version identity (%q, %q), want (%q, %q)",
					state.Version.Name, state.Version.Version, fuzzRegistryPackage, fuzzRegistryVersion)
			}
		}
	})
}

// FuzzDecodeRegistryAttestationResponse feeds attacker-controlled bytes to the
// pure attestation decoder. Properties asserted for every input:
//   - decoding never panics and is deterministic;
//   - every rejection is one of the decoder's four documented plain errors;
//   - on success every entry carries a non-empty predicate type and a
//     non-empty bundle.
func FuzzDecodeRegistryAttestationResponse(f *testing.F) {
	// Valid response (registry_client_test.go TestRegistryAttestations).
	f.Add([]byte(`{"attestations":[{"predicateType":"https://slsa.dev/provenance/v1","bundle":{"exact":"bytes"}},{"predicateType":"https://github.com/npm/attestation/tree/main/specs/publish/v0.1","bundle":{"ignored":true}}]}`))
	// Empty attestations array: an observed version with no attestations.
	f.Add([]byte(`{"attestations":[]}`))
	// Missing attestations member.
	f.Add([]byte(`{}`))
	// Entry with an empty predicateType.
	f.Add([]byte(`{"attestations":[{"predicateType":"","bundle":{"exact":"bytes"}}]}`))
	// Entry with a null bundle.
	f.Add([]byte(`{"attestations":[{"predicateType":"https://slsa.dev/provenance/v1","bundle":null}]}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		state, err := decodeRegistryAttestationResponse(data)
		secondState, secondErr := decodeRegistryAttestationResponse(data)
		if (err == nil) != (secondErr == nil) {
			t.Fatalf("decodeRegistryAttestationResponse is nondeterministic for %q", data)
		}
		if err != nil {
			if secondErr == nil || err.Error() != secondErr.Error() {
				t.Fatalf("decodeRegistryAttestationResponse error is nondeterministic for %q", data)
			}
			if !fuzzRegistryAttestationErrors[err.Error()] {
				t.Fatalf("rejection is not a documented decoder error: %v", err)
			}
			return
		}
		if state.Found != secondState.Found || len(state.Attestations) != len(secondState.Attestations) {
			t.Fatalf("decodeRegistryAttestationResponse state is nondeterministic for %q", data)
		}
		for index, attestation := range state.Attestations {
			other := secondState.Attestations[index]
			if attestation.PredicateType != other.PredicateType || string(attestation.Bundle) != string(other.Bundle) {
				t.Fatalf("decodeRegistryAttestationResponse entry %d is nondeterministic for %q", index, data)
			}
		}
		if !state.Found {
			t.Fatal("successful decode reported attestations as not found")
		}
		for index, attestation := range state.Attestations {
			if attestation.PredicateType == "" {
				t.Fatalf("accepted attestation %d has an empty predicate type", index)
			}
			if len(attestation.Bundle) == 0 {
				t.Fatalf("accepted attestation %d has an empty bundle", index)
			}
		}
	})
}

// FuzzNormalizeRegistryURL feeds attacker-controlled strings to the registry
// root URL validator. Properties asserted for every input:
//   - normalization never panics and is deterministic;
//   - an accepted URL is HTTPS, credential-free, query-free, fragment-free,
//     and rooted at path "/";
//   - normalization is idempotent: re-normalizing the accepted rendering
//     succeeds and yields an equal URL.
func FuzzNormalizeRegistryURL(f *testing.F) {
	f.Add("https://registry.npmjs.org/")
	// Rejection set from registry_client_test.go TestRegistryClientRejectsInsecureURL.
	f.Add("http://registry.npmjs.org/")
	f.Add("https://token@registry.npmjs.org/")
	f.Add("https://registry.npmjs.org/path/")
	f.Add("https://registry.npmjs.org/?token=secret")
	f.Add("https://registry.npmjs.org/#fragment")
	// No trailing slash: accepted and normalized to path "/".
	f.Add("https://registry.npmjs.org")
	// Case normalization: scheme and host case fold to lowercase.
	f.Add("HTTPS://REGISTRY.NPMJS.ORG/")

	f.Fuzz(func(t *testing.T, raw string) {
		normalized, err := normalizeRegistryURL(raw)
		second, secondErr := normalizeRegistryURL(raw)
		if (err == nil) != (secondErr == nil) {
			t.Fatalf("normalizeRegistryURL(%q) is nondeterministic", raw)
		}
		if err != nil {
			return
		}
		if normalized.String() != second.String() {
			t.Fatalf("normalizeRegistryURL(%q) rendered nondeterministically", raw)
		}
		if normalized.Scheme != "https" {
			t.Fatalf("accepted non-HTTPS registry URL %q", raw)
		}
		if normalized.User != nil {
			t.Fatalf("accepted credential-bearing registry URL %q", raw)
		}
		if normalized.RawQuery != "" || normalized.Fragment != "" {
			t.Fatalf("accepted registry URL %q with query or fragment", raw)
		}
		if normalized.Path != "/" {
			t.Fatalf("accepted registry URL %q with path %q", raw, normalized.Path)
		}
		renormalized, err := normalizeRegistryURL(normalized.String())
		if err != nil {
			t.Fatalf("re-normalizing accepted URL %q failed: %v", normalized.String(), err)
		}
		if renormalized.String() != normalized.String() {
			t.Fatalf("normalizeRegistryURL is not idempotent: %q became %q", normalized.String(), renormalized.String())
		}
	})
}

// FuzzDecodeGitHubOIDCResponse feeds attacker-controlled bytes to the GitHub
// OIDC identity decoder with the fixture caller workflow ref. Properties
// asserted for every input:
//   - decoding never panics and is deterministic;
//   - a strict-JSON rejection is one of the decoder's documented plain errors
//     (non-strict JSON returns the canonicaljson validation error);
//   - on success the token is a three-segment JWT and the observed workflow
//     filename is the fixture's release.yml.
func FuzzDecodeGitHubOIDCResponse(f *testing.F) {
	encodeClaims := func(claims string) string {
		return "header." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".signature"
	}
	// Valid response, built exactly as oidc_client_test.go:169 does.
	validJWT := encodeClaims(`{"workflow_ref":"windlasstech/caller/.github/workflows/release.yml@refs/tags/v1.2.3"}`)
	f.Add([]byte(`{"value":"` + validJWT + `"}`))
	// Claim-mismatch variant: a different caller workflow.
	f.Add([]byte(`{"value":"` + encodeClaims(`{"workflow_ref":"windlasstech/caller/.github/workflows/other.yml@refs/tags/v1.2.3"}`) + `"}`))
	// Two-segment token.
	f.Add([]byte(`{"value":"header.payload"}`))
	// Non-canonical JSON: duplicate member rejected by strict validation.
	f.Add([]byte(`{"value":"a.b.c","value":"d.e.f"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		token, filename, err := decodeGitHubOIDCResponse(data, fuzzWorkflowRef)
		secondToken, secondFilename, secondErr := decodeGitHubOIDCResponse(data, fuzzWorkflowRef)
		if (err == nil) != (secondErr == nil) {
			t.Fatalf("decodeGitHubOIDCResponse is nondeterministic for %q", data)
		}
		if err != nil {
			if secondErr == nil || err.Error() != secondErr.Error() {
				t.Fatalf("decodeGitHubOIDCResponse error is nondeterministic for %q", data)
			}
			if canonicalErr := canonicaljson.Validate(data); canonicalErr == nil && !fuzzGitHubOIDCErrors[err.Error()] {
				t.Fatalf("strict-JSON rejection is not a documented decoder error: %v", err)
			}
			return
		}
		if token.value() != secondToken.value() || filename != secondFilename {
			t.Fatalf("decodeGitHubOIDCResponse output is nondeterministic for %q", data)
		}
		if segments := strings.Split(token.value(), "."); len(segments) != 3 {
			t.Fatalf("accepted token does not have exactly three segments: %d", len(segments))
		}
		if filename != "release.yml" {
			t.Fatalf("accepted workflow filename %q, want release.yml", filename)
		}
	})
}

// FuzzDecodeNPMExchangeResponse feeds attacker-controlled bytes to the npm
// OIDC exchange decoder with the fixed testExchangeNow clock. Properties
// asserted for every input:
//   - decoding never panics and is deterministic;
//   - a strict-JSON rejection is either a *json.UnmarshalTypeError or one of
//     the decoder's documented plain errors;
//   - on success expires is strictly after created and strictly after now.
func FuzzDecodeNPMExchangeResponse(f *testing.F) {
	// Valid rows: documented RFC 3339 representation and observed live-registry
	// integral epoch representation (ADR 0081).
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":"2026-08-07T12:00:00Z","expires":"2026-08-07T13:00:00Z"}`))
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":1786704000,"expires":1786704900}`))
	// Contract rejection rows from oidc_client_test.go
	// TestOIDCExchangeResponseContractRejections.
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":"2026-08-07T12:00:00Z","expires":"2026-08-07T12:15:00Z","scope":"publish"}`))
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":"2026-08-07T12:00:00Z"}`))
	f.Add([]byte(`{"token_type":"bearer","token":"x","created":"2026-08-07T12:00:00Z","expires":"2026-08-07T12:15:00Z"}`))
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":"2026-08-07T12:15:00Z","expires":"2026-08-07T12:00:00Z"}`))
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":"2026-08-07T11:45:00Z","expires":"2026-08-07T12:00:00Z"}`))
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":"not-a-timestamp","expires":"2026-08-07T12:15:00Z"}`))
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":1786705013,"expires":1786705913.5}`))
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":1786705013,"expires":0}`))
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":"1786705013","expires":"2026-08-07T12:15:00Z"}`))
	f.Add([]byte(`{"token_type":"oidc","token":"x","created":1786705013,"expires":"not-a-timestamp"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, created, expires, err := decodeNPMExchangeResponse(data, testExchangeNow)
		_, secondCreated, secondExpires, secondErr := decodeNPMExchangeResponse(data, testExchangeNow)
		if (err == nil) != (secondErr == nil) {
			t.Fatalf("decodeNPMExchangeResponse is nondeterministic for %q", data)
		}
		if err != nil {
			if secondErr == nil || err.Error() != secondErr.Error() {
				t.Fatalf("decodeNPMExchangeResponse error is nondeterministic for %q", data)
			}
			if canonicalErr := canonicaljson.Validate(data); canonicalErr == nil {
				// decodeNPMExchangeResponse surfaces decoder.Decode failures
				// raw: member type mismatches as *json.UnmarshalTypeError and,
				// under DisallowUnknownFields, unknown members as
				// "json: unknown field ..." errors.
				var typeErr *json.UnmarshalTypeError
				if !errors.As(err, &typeErr) && !strings.HasPrefix(err.Error(), "json: unknown field ") &&
					!fuzzNPMExchangeErrors[err.Error()] {
					t.Fatalf("strict-JSON rejection is not a documented decoder error: %v", err)
				}
			}
			return
		}
		if !created.Equal(secondCreated) || !expires.Equal(secondExpires) {
			t.Fatalf("decodeNPMExchangeResponse timestamps are nondeterministic for %q", data)
		}
		if !expires.After(created) {
			t.Fatalf("accepted exchange response with expires %s not after created %s", expires, created)
		}
		if !expires.After(testExchangeNow) {
			t.Fatalf("accepted exchange response with expires %s not after now %s", expires, testExchangeNow)
		}
	})
}

// FuzzParseExchangeTimestamp feeds attacker-controlled bytes to the ADR 0081
// exchange timestamp decoder. Properties asserted for every input:
//   - decoding never panics and is deterministic;
//   - an accepted integral JSON number n decodes to time.Unix(n, 0).UTC();
//   - an accepted string decodes to its RFC 3339 parse;
//   - zero, negative, fractional, quoted-number, object, array, null, and
//     boolean inputs are rejected.
func FuzzParseExchangeTimestamp(f *testing.F) {
	f.Add([]byte("1786705013"))
	f.Add([]byte(`"2026-08-07T12:00:00Z"`))
	f.Add([]byte("0"))
	f.Add([]byte("-5"))
	f.Add([]byte("1786705913.5"))
	f.Add([]byte(`"1786705013"`))
	f.Add([]byte("null"))
	f.Add([]byte("true"))
	f.Add([]byte("[1]"))

	f.Fuzz(func(t *testing.T, data []byte) {
		instant, err := parseExchangeTimestamp(json.RawMessage(data))
		secondInstant, secondErr := parseExchangeTimestamp(json.RawMessage(data))
		if (err == nil) != (secondErr == nil) {
			t.Fatalf("parseExchangeTimestamp is nondeterministic for %q", data)
		}
		if err == nil && !instant.Equal(secondInstant) {
			t.Fatalf("parseExchangeTimestamp instant is nondeterministic for %q", data)
		}

		var text string
		if unmarshalErr := json.Unmarshal(data, &text); unmarshalErr == nil && string(json.RawMessage(data)) != "null" {
			// String representation: accepted exactly when it is a valid
			// RFC 3339 timestamp, and the result equals that parse.
			parsed, parseErr := time.Parse(time.RFC3339, text)
			if parseErr != nil {
				if err == nil {
					t.Fatalf("accepted non-RFC3339 string %q", data)
				}
				return
			}
			if err != nil {
				t.Fatalf("rejected valid RFC 3339 string %q: %v", data, err)
			}
			if !instant.Equal(parsed) {
				t.Fatalf("string %q decoded to %s, want RFC 3339 parse %s", data, instant, parsed)
			}
			return
		}

		var number json.Number
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.UseNumber()
		if decoder.Decode(&number) == nil {
			// Number representation: accepted exactly when it is an integral,
			// positive int64 of epoch seconds, normalized to a UTC instant.
			seconds, parseErr := strconv.ParseInt(number.String(), 10, 64)
			if parseErr != nil || seconds <= 0 {
				if err == nil {
					t.Fatalf("accepted non-integral or non-positive epoch %q", data)
				}
				return
			}
			if err != nil {
				t.Fatalf("rejected integral positive epoch %q: %v", data, err)
			}
			if want := time.Unix(seconds, 0).UTC(); !instant.Equal(want) {
				t.Fatalf("epoch %q decoded to %s, want %s", data, instant, want)
			}
			return
		}

		// Anything else (bool, null, object, array, invalid JSON) must reject.
		if err == nil {
			t.Fatalf("accepted non-string non-number timestamp %q", data)
		}
	})
}
