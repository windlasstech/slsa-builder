package attestation

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"net/url"
	"testing"
	"time"
)

func identityCertificate(t *testing.T, expected IdentityExpectation) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate certificate key: %v", err)
	}
	san, err := url.Parse(expected.SignerURI)
	if err != nil {
		t.Fatalf("parse certificate SAN: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), URIs: []*url.URL{san},
		NotBefore: fixtureTime, NotAfter: fixtureTime.Add(time.Hour),
	}
	// Fulcio v2 OIDs are encoded independently of the production OID constants.
	for _, claim := range []struct {
		suffix int
		value  string
	}{
		{8, expected.Issuer}, {9, expected.SignerURI}, {10, expected.WorkflowSHA},
		{11, expected.RunnerEnvironment}, {12, expected.SourceRepositoryURI},
		{13, expected.SourceDigest}, {14, expected.SourceRef},
		{15, expected.SourceRepositoryID}, {17, expected.SourceRepositoryOwnerID},
		{21, expected.RunInvocationURI},
	} {
		value, marshalErr := asn1.Marshal(claim.value)
		if marshalErr != nil {
			t.Fatalf("marshal certificate claim %d: %v", claim.suffix, marshalErr)
		}
		template.ExtraExtensions = append(template.ExtraExtensions, pkix.Extension{
			Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, claim.suffix}, Value: value,
		})
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create identity certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse identity certificate: %v", err)
	}
	return certificate
}

func TestVerifyIdentityBindsEveryFulcioClaim(t *testing.T) {
	t.Parallel()
	for _, claim := range []struct {
		name   string
		suffix int
		id     string
	}{
		{"issuer", 8, idIssuerMismatch},
		{"build signer URI", 9, idSignerWorkflowPathMismatch},
		{"workflow SHA", 10, idSignerWorkflowSHAMismatch},
		{"runner environment", 11, idSelfHostedRunner},
		{"repository URI", 12, idSourceIdentityMismatch},
		{"source digest", 13, idSourceDigestMismatch},
		{"source ref", 14, idSourceRefMismatch},
		{"repository ID", 15, idSourceNumericIDMismatch},
		{"owner ID", 17, idSourceNumericIDMismatch},
		{"run invocation URI", 21, idRunInvocationURIInvalid},
	} {
		t.Run(claim.name, func(t *testing.T) {
			t.Parallel()
			for _, mutation := range []string{"mismatch", "missing", "malformed", "empty", "trailing ASN1", "duplicate"} {
				t.Run(mutation, func(t *testing.T) {
					t.Parallel()
					expected := fixtureIdentity()
					certificate := identityCertificate(t, expected)
					oid := asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, claim.suffix}
					for index, extension := range certificate.Extensions {
						if !extension.Id.Equal(oid) {
							continue
						}
						switch mutation {
						case "mismatch":
							certificate.Extensions[index].Value = []byte{12, 5, 'o', 't', 'h', 'e', 'r'}
						case "missing":
							certificate.Extensions = append(certificate.Extensions[:index], certificate.Extensions[index+1:]...)
						case "malformed":
							certificate.Extensions[index].Value = []byte{0xff}
						case "empty":
							certificate.Extensions[index].Value = []byte{12, 0}
						case "trailing ASN1":
							certificate.Extensions[index].Value = append(extension.Value, 0)
						case "duplicate":
							certificate.Extensions = append(certificate.Extensions, extension)
						}
						break
					}
					err := verifyIdentity(certificate, expected)
					want := idSignerIdentityClaimMissing
					if mutation == "mismatch" {
						want = claim.id
					}
					requireDiagnosticID(t, err, want)
				})
			}
		})
	}
}

func TestVerifyIdentityCertificateSAN(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*x509.Certificate) *x509.Certificate
		id     string
	}{
		{"matching certificate", func(c *x509.Certificate) *x509.Certificate { return c }, ""},
		{"nil certificate", func(*x509.Certificate) *x509.Certificate { return nil }, idSignerIdentityClaimMissing},
		{"missing SAN", func(c *x509.Certificate) *x509.Certificate { c.URIs = nil; return c }, idSignerWorkflowPathMismatch},
		{"multiple SANs", func(c *x509.Certificate) *x509.Certificate { c.URIs = append(c.URIs, c.URIs[0]); return c }, idSignerWorkflowPathMismatch},
		{"wrong SAN", func(c *x509.Certificate) *x509.Certificate { c.URIs[0].Path = "/other"; return c }, idSignerWorkflowPathMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			expected := fixtureIdentity()
			certificate := test.mutate(identityCertificate(t, expected))
			err := verifyIdentity(certificate, expected)
			if test.id != "" {
				requireDiagnosticID(t, err, test.id)
				return
			}
			if err != nil {
				t.Fatalf("matching certificate rejected: %v", err)
			}
		})
	}
}

func TestIdentityExpectationValidation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name          string
		mutate        func(*IdentityExpectation)
		certificateID string
	}{
		{"valid", func(*IdentityExpectation) {}, ""},
		{"issuer", func(e *IdentityExpectation) { e.Issuer = "https://other.example" }, ""},
		{"signer URI", func(e *IdentityExpectation) { e.SignerURI = "https://example.com/workflow" }, ""},
		{"repository URI", func(e *IdentityExpectation) { e.SourceRepositoryURI += "/" }, idVerificationModeInvalid},
		{"workflow SHA", func(e *IdentityExpectation) { e.WorkflowSHA = "main" }, idVerificationModeInvalid},
		{"source digest", func(e *IdentityExpectation) { e.SourceDigest = "main" }, idVerificationModeInvalid},
		{"repository ID zero", func(e *IdentityExpectation) { e.SourceRepositoryID = "0" }, ""},
		{"owner ID leading zero", func(e *IdentityExpectation) { e.SourceRepositoryOwnerID = "01" }, ""},
		{"short ref", func(e *IdentityExpectation) { e.SourceRef = "main" }, ""},
		{"ref whitespace", func(e *IdentityExpectation) { e.SourceRef += " " }, ""},
		{"self hosted runner", func(e *IdentityExpectation) { e.RunnerEnvironment = "self-hosted" }, ""},
		{"run invocation URI", func(e *IdentityExpectation) { e.RunInvocationURI += "/" }, idRunInvocationURIInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			expected := fixtureIdentity()
			test.mutate(&expected)
			err := validateIdentityExpectation(expected)
			if test.name == "valid" {
				if err != nil {
					t.Fatalf("valid identity rejected: %v", err)
				}
				return
			}
			requireDiagnosticID(t, err, idPolicySchemaInvalid)
			if test.certificateID != "" {
				certificate := identityCertificate(t, expected)
				requireDiagnosticID(t, verifyIdentity(certificate, expected), test.certificateID)
			}
		})
	}
}
