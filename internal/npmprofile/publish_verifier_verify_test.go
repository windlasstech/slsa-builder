package npmprofile

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/policy"
)

func TestSigstorePublishVerifyRejections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name         string
		changeConfig func(*SigstorePublishVerifierConfig)
		bundle       []byte
		invocation   string
		wantID       string
	}{
		{name: "empty bundle", bundle: []byte{}, wantID: "windlass.verify.error.input-unavailable"},
		{name: "malformed bundle", bundle: []byte(`{"mediaType":`), wantID: "windlass.verify.error.signature-invalid"},
		{name: "repository mismatch", changeConfig: func(config *SigstorePublishVerifierConfig) {
			config.Identity.SourceRepositoryURI = "https://github.com/other/project"
		}, wantID: "windlass.verify.error.source-identity-mismatch"},
		{name: "ref mismatch", changeConfig: func(config *SigstorePublishVerifierConfig) {
			config.Identity.SourceRef = "refs/tags/v9.0.0"
		}, wantID: "windlass.verify.error.source-ref-mismatch"},
		{name: "digest mismatch", changeConfig: func(config *SigstorePublishVerifierConfig) {
			config.Identity.SourceDigest = testAttestSHA
		}, wantID: "windlass.verify.error.source-digest-mismatch"},
		{name: "invalid run URI", invocation: "https://github.com/example/project/actions/runs/0/attempts/1", wantID: "windlass.verify.error.run-invocation-uri-invalid"},
		{name: "invalid verification mode", changeConfig: func(config *SigstorePublishVerifierConfig) {
			config.Mode = "invalid"
		}, wantID: "windlass.verify.error.verification-mode-invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config, signing := publishVerifyInputs(t)
			if test.changeConfig != nil {
				test.changeConfig(&config)
			}
			if test.invocation != "" {
				signing.Predicate.RunDetails.Metadata.InvocationID = test.invocation
			}
			bundle := test.bundle
			if bundle == nil {
				statement, err := signing.Statement().CanonicalJSON()
				if err != nil {
					t.Fatalf("encode test Statement: %v", err)
				}
				bundle = publishBundleWithPayload(t, statement)
			}
			got, err := NewSigstorePublishBundleVerifier(config).Verify(t.Context(), bundle)
			requireNPMDiagnostic(t, err, test.wantID)
			if !reflect.DeepEqual(got, VerifiedPublishBundle{}) {
				t.Fatalf("rejected bundle returned verified evidence: %#v", got)
			}
		})
	}
}

func TestSigstorePublishVerifyStatementDecodeFailure(t *testing.T) {
	t.Parallel()
	bundle := publishBundleWithPayload(t, []byte(`{"subject":42}`))
	_, err := NewSigstorePublishBundleVerifier(SigstorePublishVerifierConfig{}).Verify(t.Context(), bundle)
	// DecodeStatement wraps encoding/json errors without assigning a diagnostic ID.
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) || typeErr.Field != "subject" || typeErr.Value != "number" {
		t.Fatalf("Verify() error = %v; want numeric subject type rejection", err)
	}
}

func TestSigstorePublishVerifierCopiesPinnedRoot(t *testing.T) {
	t.Parallel()
	config, signing := publishVerifyInputs(t)
	statement, err := signing.Statement().CanonicalJSON()
	if err != nil {
		t.Fatalf("encode test Statement: %v", err)
	}
	bundle := publishBundleWithPayload(t, statement)
	// A malformed policy timestamp stops after the root digest check, before
	// cryptography or wall-clock freshness. Mutating aliased bytes changes the ID.
	config.TrustRoot.RevalidatedAt = testStringPointer("not-a-timestamp")
	verifier := NewSigstorePublishBundleVerifier(config)
	_, before := verifier.Verify(t.Context(), bundle)
	requireNPMDiagnostic(t, before, "windlass.verify.error.verification-mode-invalid")
	clear(config.PinnedRoot)
	_, after := verifier.Verify(t.Context(), bundle)
	requireNPMDiagnostic(t, after, "windlass.verify.error.verification-mode-invalid")
	if before.Error() != after.Error() {
		t.Fatalf("caller mutation changed verification: before %v; after %v", before, after)
	}
	_, mutated := NewSigstorePublishBundleVerifier(config).Verify(t.Context(), bundle)
	requireNPMDiagnostic(t, mutated, "windlass.verify.error.ungoverned-trust-root")
}

func TestSigstorePublishVerifyOfflineSignerFixture(t *testing.T) {
	t.Parallel()
	config, _ := publishVerifyInputs(t)
	identityBytes := readPublishVerifyFixture(t, "signing/npm-go-signer-identity.json")
	if err := json.Unmarshal(identityBytes, &config.Identity); err != nil {
		t.Fatalf("decode fixture identity: %v", err)
	}
	bundle := readPublishVerifyFixture(t, "signing/npm-go-signer-bundle-valid.intoto.jsonl")
	_, err := NewSigstorePublishBundleVerifier(config).Verify(t.Context(), bundle)
	// This signer fixture has externalParameters: {}, not the closed npm schema.
	// Its tag-form conformance builder is also not a Windlass SHA-pinned builder.
	// Thus publish Verify cannot succeed, regardless of the compatible Sigstore
	// trusted_root.json format. Do not rewrite signed bytes or bypass these checks.
	requireNPMDiagnostic(t, err, IDUnexpectedExternalParameters)
}

func publishVerifyInputs(t *testing.T) (SigstorePublishVerifierConfig, ProvenanceSigningInput) {
	t.Helper()
	signing, err := NewProvenanceSigningInput(validProvenanceInput(t, ManagerNPM))
	if err != nil {
		t.Fatalf("assemble valid npm Statement: %v", err)
	}
	rootBytes := readPublishVerifyFixture(t, "attestation/trusted_root.json")
	identity := publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA)
	identity.Issuer = "https://token.actions.githubusercontent.com"
	identity.SourceRepositoryID = "123"
	identity.SourceRepositoryOwnerID = "456"
	identity.RunnerEnvironment = "github-hosted"
	return SigstorePublishVerifierConfig{
		Mode: attestation.ModeOffline, PinnedRoot: rootBytes, Identity: identity,
		TrustRoot: policy.TrustRoot{
			Mode: "pinned", Instance: "sigstore-public-good",
			Path: testStringPointer("trusted_root.json"), SHA256: testStringPointer(digest.SumSHA256(rootBytes).String()),
			TUFRepository: testStringPointer("https://tuf-repo-cdn.sigstore.dev"),
			RevalidatedAt: testStringPointer("2026-08-07T00:00:00Z"), RefreshBefore: testStringPointer("2026-08-08T00:00:00Z"),
		},
	}, signing
}

func publishBundleWithPayload(t *testing.T, payload []byte) []byte {
	t.Helper()
	original := readPublishVerifyFixture(t, "signing/npm-go-signer-bundle-valid.intoto.jsonl")
	parsed, err := attestation.ParseBundle(original)
	if err != nil {
		t.Fatalf("parse signer fixture: %v", err)
	}
	oldPayload := []byte(base64.StdEncoding.EncodeToString(parsed.StatementBytes()))
	if bytes.Count(original, oldPayload) != 1 {
		t.Fatal("signer fixture must contain exactly one encoded payload")
	}
	// Only pre-crypto rejection tests use this unsigned payload substitution.
	return bytes.Replace(original, oldPayload, []byte(base64.StdEncoding.EncodeToString(payload)), 1)
}

func readPublishVerifyFixture(t *testing.T, name string) []byte {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return contents
}
