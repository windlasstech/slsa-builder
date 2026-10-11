package attestation

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestVerifyWithTrustedMaterialGuards(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name            string
		mutate          func(*Request)
		cancelled       bool
		missingMaterial bool
		id              string
	}{
		{"cancelled context", func(*Request) {}, true, false, idVerifierExecutionFailure},
		{"neither representation", func(r *Request) { r.ExpectedStatementJSON = nil }, false, false, idInputUnavailable},
		{"both representations", func(r *Request) { r.ExpectedStatement = &provenance.Statement{} }, false, false, idPolicySchemaInvalid},
		{"nil material", func(*Request) {}, false, true, idUngovernedTrustRoot},
		{"invalid identity", func(r *Request) { r.Identity.WorkflowSHA = "main" }, false, false, idPolicySchemaInvalid},
		{"missing bundle", func(r *Request) { r.Bundle = nil }, false, false, idInputUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := offlineRequest(t, fixtureBundle(t))
			material, err := acquirePinnedRoot(request, fixtureTime)
			if err != nil {
				t.Fatalf("load fixture root: %v", err)
			}
			test.mutate(&request)
			if test.missingMaterial {
				material = nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.cancelled {
				cancel()
			}
			_, err = VerifyWithTrustedMaterial(ctx, request, material)
			requireDiagnosticID(t, err, test.id)
			if test.cancelled && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled verification error = %v, want context.Canceled cause", err)
			}
		})
	}
}

func TestVerifyWithTrustedMaterialRejectsLegacyOverrides(t *testing.T) {
	for _, name := range []string{"SIGSTORE_ROOT_FILE", "SIGSTORE_REKOR_PUBLIC_KEY", "SIGSTORE_CT_LOG_PUBLIC_KEY_FILE"} {
		t.Run(name, func(t *testing.T) {
			request := offlineRequest(t, fixtureBundle(t))
			material, err := acquirePinnedRoot(request, fixtureTime)
			if err != nil {
				t.Fatalf("load fixture root: %v", err)
			}
			t.Setenv(name, "")
			_, err = VerifyWithTrustedMaterial(context.Background(), request, material)
			requireDiagnosticID(t, err, idLegacyTrustRootOverride)
			var diagnostic *VerificationError
			if !errors.As(err, &diagnostic) || diagnostic.Diagnostic.Field != "environment."+name {
				t.Fatalf("override error = %v, want field environment.%s", err, name)
			}
		})
	}
}

func TestVerifyWithTrustedMaterialOfflineCrypto(t *testing.T) {
	original := http.DefaultTransport
	transport := &countingDenyTransport{}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, test := range []struct {
		name      string
		model     bool
		mismatch  bool
		malformed bool
		id        string
	}{
		{"JSON success", false, false, false, ""},
		{"model mismatch", true, true, false, idStatementAssemblyMismatch},
		{"malformed expected JSON", false, false, true, idSignatureInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := offlineRequest(t, fixtureBundle(t))
			material, err := root.NewTrustedRootFromJSON(request.PinnedRoot)
			if err != nil {
				t.Fatalf("parse fixture root: %v", err)
			}
			if test.model {
				statement, decodeErr := provenance.DecodeStatement(readFixture(t, filepath.Join("..", "..", "testdata", "provenance", "valid-statement.json")))
				if decodeErr != nil {
					t.Fatalf("decode expected Statement: %v", decodeErr)
				}
				if test.mismatch {
					statement.Subject[0].Name = "pkg:npm/other@1.0.0"
				}
				request.ExpectedStatement = &statement
				request.ExpectedStatementJSON = nil
			}
			if test.malformed {
				request.ExpectedStatementJSON = []byte("{")
			}
			result, err := VerifyWithTrustedMaterial(context.Background(), request, material)
			if test.id != "" {
				requireDiagnosticID(t, err, test.id)
				return
			}
			if err != nil {
				t.Fatalf("offline verification: %v", err)
			}
			if !bytes.Equal(result.BundleBytes(), request.Bundle) {
				t.Fatal("verified bundle differs from input bytes")
			}
			equal, compareErr := canonicaljson.Equal(result.StatementBytes(), fixtureExpectedStatement(t))
			if compareErr != nil || !equal {
				t.Fatalf("verified Statement equality = %v, error = %v, want true, nil", equal, compareErr)
			}
			if result.Certificate == nil || result.SigningTime.IsZero() {
				t.Fatal("verified evidence lacks certificate or signing time")
			}
			if result.SigningTime.Before(result.Certificate.NotBefore) || result.SigningTime.After(result.Certificate.NotAfter) {
				t.Fatalf("verified signing time %s lies outside certificate validity", result.SigningTime)
			}
		})
	}
	if got := transport.calls.Load(); got != 0 {
		t.Fatalf("network calls = %d, want 0", got)
	}
}

func TestVerifyAtInputGuards(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		cancelled bool
		id        string
	}{
		{"cancelled context", true, idVerifierExecutionFailure},
		{"both statement representations", false, idPolicySchemaInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := offlineRequest(t, fixtureBundle(t))
			request.ExpectedStatement = &provenance.Statement{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.cancelled {
				cancel()
			}
			_, err := verifyAt(ctx, request, fixtureTime)
			requireDiagnosticID(t, err, test.id)
		})
	}
}
