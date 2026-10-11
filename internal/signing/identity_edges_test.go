package signing

import (
	"encoding/base64"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestBindOIDCSignerIdentityRejectsMalformedClaims(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, token, want string }{
		{"missing segments", "token", "GitHub Actions OIDC token shape is invalid"},
		{"invalid base64", "header.!.signature", "GitHub Actions OIDC claims are invalid"},
		{"invalid JSON", "header." + base64.RawURLEncoding.EncodeToString([]byte(`{`)) + ".signature", "GitHub Actions OIDC workflow claims are unavailable"},
		{"missing workflow", "header.e30.signature", "GitHub Actions OIDC workflow revision differs from trusted runtime"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := bindOIDCSignerIdentity(nil, attestation.IdentityExpectation{WorkflowSHA: "trusted"}, test.token)
			if err == nil || err.Error() != test.want {
				t.Fatalf("bind error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestBindOIDCSignerIdentityWorkflowBinding(t *testing.T) {
	t.Parallel()
	const sha = "d2d916c6d6694c82c79d15c0393139b4084d4acc"
	const path = "owner/builder/.github/workflows/build.yml"
	for _, test := range []struct{ name, claims, builder, statement, want string }{
		{"fallback workflow", `{"workflow_ref":"` + path + `@refs/heads/main","workflow_sha":"` + sha + `"}`, "https://github.com/" + path + "@" + sha, "", ""},
		{"partial job claims cannot fall back", `{"job_workflow_ref":"` + path + `@` + sha + `","workflow_ref":"` + path + `@main","workflow_sha":"` + sha + `"}`, "", "", "GitHub Actions OIDC workflow revision differs from trusted runtime"},
		{"different workflow path", `{"job_workflow_ref":"` + path + `@` + sha + `","job_workflow_sha":"` + sha + `"}`, "https://github.com/other/builder/.github/workflows/build.yml@" + sha, "", "OIDC signer workflow differs from the Statement builder identity"},
		{"missing ref separator", `{"job_workflow_ref":"` + path + `","job_workflow_sha":"` + sha + `"}`, "", "", "called workflow reference is not pinned by its full commit SHA"},
		{"invalid statement", `{"job_workflow_ref":"` + path + `@` + sha + `","job_workflow_sha":"` + sha + `"}`, "", `{"extra":true}`, `decode signed Statement identity: decode closed SLSA Statement: json: unknown field "extra"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			statement := []byte(test.statement)
			if test.statement == "" {
				var err error
				statement, err = (provenance.Statement{Predicate: provenance.Predicate{RunDetails: provenance.RunDetails{Builder: provenance.Builder{ID: test.builder}}}}).CanonicalJSON()
				if err != nil {
					t.Fatalf("encode statement: %v", err)
				}
			}
			token := "header." + base64.RawURLEncoding.EncodeToString([]byte(test.claims)) + ".signature"
			expected := attestation.IdentityExpectation{WorkflowSHA: sha}
			got, err := bindOIDCSignerIdentity(statement, expected, token)
			if test.want != "" {
				if err == nil || err.Error() != test.want {
					t.Fatalf("bind error = %v, want %q", err, test.want)
				}
				return
			}
			expected.SignerURI = "https://github.com/" + path + "@refs/heads/main"
			if err != nil || got != expected {
				t.Fatalf("bound identity = %#v, %v, want %#v, nil", got, err, expected)
			}
		})
	}
}

func TestPrepareDSSERejectsNoncanonicalBytes(t *testing.T) {
	t.Parallel()
	_, err := PrepareDSSE([]byte(`{ }`))
	if err == nil || err.Error() != "statement must be exact RFC 8785 bytes" {
		t.Fatalf("PrepareDSSE error = %v, want exact-byte rejection", err)
	}
}
