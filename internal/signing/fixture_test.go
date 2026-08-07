package signing

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/digest"
)

func TestProductionSignerFixtureOffline(t *testing.T) {
	originalTransport := http.DefaultTransport
	transport := &denyTransport{}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	fixtureDirectory := filepath.Join("..", "..", "testdata", "signing")
	bundleBytes := readSigningFixture(t, filepath.Join(fixtureDirectory, "npm-go-signer-bundle-valid.intoto.jsonl"))
	parsed, err := attestation.ParseBundle(bundleBytes)
	if err != nil {
		t.Fatalf("parse production signer fixture: %v", err)
	}
	statementPath := filepath.Join(fixtureDirectory, "npm-go-signer-statement.jcs.json")
	if os.Getenv("UPDATE_P02_FIXTURE") == "1" {
		if err := os.WriteFile(statementPath, parsed.StatementBytes(), 0o600); err != nil {
			t.Fatalf("write production signer Statement: %v", err)
		}
	}
	statementBytes := readSigningFixture(t, statementPath)
	identityBytes := readSigningFixture(t, filepath.Join(fixtureDirectory, "npm-go-signer-identity.json"))
	var identity attestation.IdentityExpectation
	if err := json.Unmarshal(identityBytes, &identity); err != nil {
		t.Fatalf("decode production signer identity: %v", err)
	}
	rootPath := filepath.Join("..", "..", "testdata", "attestation", "trusted_root.json")
	rootBytes := readSigningFixture(t, rootPath)
	if got := digest.SumSHA256(rootBytes).String(); got != "4364d7724c04cc912ce2a6c45ed2610e8d8d1c4dc857fb500292738d4d9c8d2c" {
		t.Fatalf("trusted root digest = %s", got)
	}
	trustedMaterial, err := root.NewTrustedRootFromJSON(rootBytes)
	if err != nil {
		t.Fatalf("load fixture trusted root: %v", err)
	}
	result, err := attestation.VerifyWithTrustedMaterial(context.Background(), attestation.Request{
		Bundle:                bundleBytes,
		Identity:              identity,
		ExpectedStatementJSON: statementBytes,
	}, trustedMaterial)
	if err != nil {
		t.Fatalf("offline verify production signer fixture: %v", err)
	}
	if string(result.BundleBytes()) != string(bundleBytes) || string(result.StatementBytes()) != string(statementBytes) {
		t.Fatal("production fixture verification changed exact bytes")
	}
	if calls := transport.calls.Load(); calls != 0 {
		t.Fatalf("offline production fixture verification made %d network calls", calls)
	}
}

func readSigningFixture(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read signing fixture %s: %v", path, err)
	}
	return contents
}
