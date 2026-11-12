package attestation

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"testing"
)

func TestVerifyIdentityBuildInvocationClaims(t *testing.T) {
	t.Parallel()

	for _, claim := range []struct {
		name        string
		suffix      int
		value       string
		expectation string
		mismatchID  string
	}{
		{
			name:        "build trigger",
			suffix:      16,
			value:       "push",
			expectation: `{"build_trigger":"push"}`,
			mismatchID:  "windlass.verify.error.build-trigger-mismatch",
		},
		{
			name:        "build config URI",
			suffix:      18,
			value:       "yunseo-kim/slsa-builder-attest-spike/.github/workflows/publish.yml@refs/heads/main",
			expectation: `{"build_config_uri":"yunseo-kim/slsa-builder-attest-spike/.github/workflows/publish.yml@refs/heads/main"}`,
			mismatchID:  "windlass.verify.error.build-config-uri-mismatch",
		},
	} {
		t.Run(claim.name, func(t *testing.T) {
			t.Parallel()
			for _, mutation := range []string{"match", "mismatch", "missing", "malformed", "duplicate"} {
				t.Run(mutation, func(t *testing.T) {
					t.Parallel()

					// Given a certificate carrying the invocation claim and its statement-side expectation.
					expected := fixtureIdentity()
					if err := json.Unmarshal([]byte(claim.expectation), &expected); err != nil {
						t.Fatalf("decode invocation expectation: %v", err)
					}
					certificate := identityCertificate(t, expected)
					oid := asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, claim.suffix}
					encoded, err := asn1.Marshal(claim.value)
					if err != nil {
						t.Fatalf("marshal invocation claim: %v", err)
					}
					certificate.Extensions = append(certificate.Extensions, pkix.Extension{Id: oid, Value: encoded})
					index := len(certificate.Extensions) - 1
					switch mutation {
					case "mismatch":
						certificate.Extensions[index].Value = []byte{12, 5, 'o', 't', 'h', 'e', 'r'}
					case "missing":
						certificate.Extensions = certificate.Extensions[:index]
					case "malformed":
						certificate.Extensions[index].Value = []byte{0xff}
					case "duplicate":
						certificate.Extensions = append(certificate.Extensions, certificate.Extensions[index])
					}

					// When certificate identity is verified against the invocation expectation.
					err = verifyIdentity(certificate, expected)

					// Then matching claims pass and every mismatch or unusable claim fails closed.
					if mutation == "match" {
						if err != nil {
							t.Fatalf("matching %s claim rejected: %v", claim.name, err)
						}
						return
					}
					want := idSignerIdentityClaimMissing
					if mutation == "mismatch" {
						want = claim.mismatchID
					}
					requireDiagnosticID(t, err, want)
				})
			}
		})
	}
}
