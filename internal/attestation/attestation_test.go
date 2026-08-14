package attestation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/policy"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	fixtureRootSHA256  = "4364d7724c04cc912ce2a6c45ed2610e8d8d1c4dc857fb500292738d4d9c8d2c"
	fixtureWorkflowSHA = "b39d8bf04a5abe0e1a070d293edd199d0ac2133d"
)

var fixtureTime = time.Date(2026, time.August, 5, 11, 55, 17, 0, time.UTC)

func TestVerifyOnlineBundle(t *testing.T) {
	if os.Getenv("WINDLASS_TEST_ONLINE") != "1" {
		t.Skip("set WINDLASS_TEST_ONLINE=1 to exercise authenticated Sigstore TUF acquisition")
	}

	result, err := Verify(context.Background(), Request{
		Mode:                  ModeOnline,
		Bundle:                fixtureBundle(t),
		TrustRoot:             policy.TrustRoot{Mode: "tuf", Instance: "sigstore-public-good"},
		Identity:              fixtureIdentity(),
		ExpectedStatementJSON: fixtureExpectedStatement(t),
	})
	if err != nil {
		t.Fatalf("verify online bundle: %v", err)
	}
	if len(result.BundleBytes()) == 0 || len(result.StatementBytes()) == 0 {
		t.Fatal("online verification did not preserve bundle and Statement bytes")
	}
}

func TestVerifyOfflineBundle(t *testing.T) {
	bundleBytes := fixtureBundle(t)
	result, err := verifyAt(context.Background(), offlineRequest(t, bundleBytes), fixtureTime)
	if err != nil {
		t.Fatalf("verify offline bundle: %v", err)
	}
	if string(result.BundleBytes()) != string(bundleBytes) {
		t.Fatal("verification changed the exact actions/attest bundle bytes")
	}
	if result.SigningTime.IsZero() {
		t.Fatal("verified SET-covered signing time is zero")
	}
}

func TestMissingSCT(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	request.PinnedRoot = removeJSONMember(t, request.PinnedRoot, "ctlogs")
	request.TrustRoot.SHA256 = stringPointer(sha256Hex(request.PinnedRoot))

	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idMissingSCT)
}

func TestMissingRekor(t *testing.T) {
	request := offlineRequest(t, removeNestedJSONMember(t, fixtureBundle(t), "verificationMaterial", "tlogEntries"))

	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idMissingRekorEntry)
}

func TestSigningTime(t *testing.T) {
	result, err := verifyAt(context.Background(), offlineRequest(t, fixtureBundle(t)), fixtureTime)
	if err != nil {
		t.Fatalf("verify fixture: %v", err)
	}
	var document struct {
		VerificationMaterial struct {
			TLogEntries []struct {
				IntegratedTime json.Number `json:"integratedTime"`
			} `json:"tlogEntries"`
		} `json:"verificationMaterial"`
	}
	if err := json.Unmarshal(fixtureBundle(t), &document); err != nil {
		t.Fatalf("decode fixture signing time: %v", err)
	}
	seconds, err := document.VerificationMaterial.TLogEntries[0].IntegratedTime.Int64()
	if err != nil {
		t.Fatalf("parse fixture signing time: %v", err)
	}
	if !result.SigningTime.Equal(time.Unix(seconds, 0).UTC()) {
		t.Fatalf("signing time = %s, want %s", result.SigningTime, time.Unix(seconds, 0).UTC())
	}

	err = validateSigningTime(result.Certificate, result.Certificate.NotBefore.Add(-time.Second))
	requireDiagnosticID(t, err, idSignatureTimeViolation)
	err = validateSigningTime(result.Certificate, result.Certificate.NotAfter.Add(time.Second))
	requireDiagnosticID(t, err, idSignatureTimeViolation)
}

func TestStatementAssembly(t *testing.T) {
	statementBytes := readFixture(t, filepath.Join("..", "..", "testdata", "provenance", "valid-statement.json"))
	statement, err := provenance.DecodeStatement(statementBytes)
	if err != nil {
		t.Fatalf("decode expected Statement: %v", err)
	}
	if err := CompareStatement(statementBytes, statement); err != nil {
		t.Fatalf("compare matching Statement: %v", err)
	}

	statement.Subject[0].Name = "pkg:npm/example@9.9.9"
	err = CompareStatement(statementBytes, statement)
	requireDiagnosticID(t, err, idStatementAssemblyMismatch)
}

func TestStatementExpectationRequired(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	request.ExpectedStatementJSON = nil
	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idInputUnavailable)
}

func TestVerifyStatementMismatch(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	request.ExpectedStatementJSON = bytes.Replace(
		request.ExpectedStatementJSON,
		[]byte("deterministic-f03"),
		[]byte("different-fixture"),
		1,
	)
	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idStatementAssemblyMismatch)
}

func TestOfflineForbidsNetwork(t *testing.T) {
	originalTransport := http.DefaultTransport
	transport := &countingDenyTransport{}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	if _, err := verifyAt(context.Background(), offlineRequest(t, fixtureBundle(t)), fixtureTime); err != nil {
		t.Fatalf("verify offline bundle: %v", err)
	}
	if calls := transport.calls.Load(); calls != 0 {
		t.Fatalf("offline verification made %d HTTP transport calls, want zero", calls)
	}
}

func TestDuplicateBundleMember(t *testing.T) {
	bundle := fixtureBundle(t)
	bundle = append([]byte(`{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json",`), bundle[1:]...)
	_, err := ParseBundle(bundle)
	requireDiagnosticID(t, err, idDuplicateJSONMember)
}

func TestWrongSignerIdentity(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	request.Identity.SourceRepositoryID = "999999999"
	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idSourceNumericIDMismatch)
}

func TestStalePinnedRoot(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	_, err := verifyAt(context.Background(), request, time.Date(2026, time.August, 12, 0, 0, 1, 0, time.UTC))
	requireDiagnosticID(t, err, idStalePinnedTrustRoot)
}

func TestProductionClockRejectsStalePinnedRoot(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	refreshBefore := "2020-01-02T00:00:00Z"
	revalidatedAt := "2020-01-01T00:00:00Z"
	request.TrustRoot.RefreshBefore = &refreshBefore
	request.TrustRoot.RevalidatedAt = &revalidatedAt
	_, err := Verify(context.Background(), request)
	requireDiagnosticID(t, err, idStalePinnedTrustRoot)
}

func TestMalformedIdentityExpectation(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	request.Identity.WorkflowSHA = "main"
	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idPolicySchemaInvalid)
}

func TestSignerURIExpectationSHAPinForm(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	request.Identity.SignerURI = "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@" + fixtureWorkflowSHA
	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idSignerIdentityUntrusted)
}

func TestSignerURIExpectationCanonicalForms(t *testing.T) {
	base := "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml"
	cases := map[string]string{
		"branch full ref": base + "@refs/heads/main",
		"tag full ref":    base + "@refs/tags/v1.0.0",
		"yaml filename":   "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yaml@refs/heads/main",
	}
	for name, uri := range cases {
		t.Run(name, func(t *testing.T) {
			request := offlineRequest(t, fixtureBundle(t))
			request.Identity.SignerURI = uri
			_, err := verifyAt(context.Background(), request, fixtureTime)
			requireDiagnosticID(t, err, idSignerIdentityUntrusted)
		})
	}
}

func TestSignerURIExpectationGrammarRejections(t *testing.T) {
	base := "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@"
	cases := map[string]string{
		"bare branch name":   base + "main",
		"short SHA":          base + "d2d916c6",
		"39-hex SHA":         base + "d2d916c6d6694c82c79d15c0393139b4084d4ac",
		"41-hex SHA":         base + "d2d916c6d6694c82c79d15c0393139b4084d4acca",
		"uppercase SHA":      base + "D2D916C6D6694C82C79D15C0393139B4084D4ACC",
		"empty ref":          base,
		"missing at-sign":    "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml",
		"double at":          base + "refs/heads/main@extra",
		"ref trailing space": base + "refs/heads/main ",
		"http scheme":        "http://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/heads/main",
		"userinfo":           "https://user@github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/heads/main",
		"query":              base + "refs/heads/main?foo=bar",
		"fragment":           base + "refs/heads/main#frag",
		"non-yaml filename":  "https://github.com/windlasstech/slsa-builder/.github/workflows/build.json@refs/heads/main",
		"extra path segment": "https://github.com/windlasstech/slsa-builder/.github/workflows/sub/js-ts-npm-package-slsa3.yml@refs/heads/main",
		"non-workflow path":  "https://github.com/windlasstech/slsa-builder/actions/build.yml@refs/heads/main",
		"non-github host":    "https://ghe.example.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/heads/main",
	}
	for name, uri := range cases {
		t.Run(name, func(t *testing.T) {
			request := offlineRequest(t, fixtureBundle(t))
			request.Identity.SignerURI = uri
			_, err := verifyAt(context.Background(), request, fixtureTime)
			requireDiagnosticID(t, err, idPolicySchemaInvalid)
		})
	}
}

func TestEmptyPinnedRoot(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	request.PinnedRoot = nil
	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idInputUnavailable)
}

func TestPinnedRootDigestMismatch(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	request.TrustRoot.SHA256 = &digest
	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idUngovernedTrustRoot)
}

func TestLegacyTrustRootOverride(t *testing.T) {
	t.Setenv("SIGSTORE_ROOT_FILE", "/tmp/untrusted-root.json")
	_, err := verifyAt(context.Background(), offlineRequest(t, fixtureBundle(t)), fixtureTime)
	requireDiagnosticID(t, err, idLegacyTrustRootOverride)
}

func TestDuplicateStatementMember(t *testing.T) {
	bundle := mutateStatementPayload(t, fixtureBundle(t), func(statement []byte) []byte {
		return append([]byte(`{"_type":"https://in-toto.io/Statement/v1",`), statement[1:]...)
	})
	_, err := ParseBundle(bundle)
	requireDiagnosticID(t, err, idDuplicateJSONMember)
}

func TestDuplicatePinnedRootMember(t *testing.T) {
	request := offlineRequest(t, fixtureBundle(t))
	request.PinnedRoot = append(
		[]byte(`{"mediaType":"application/vnd.dev.sigstore.trustedroot+json;version=0.1",`),
		request.PinnedRoot[1:]...,
	)
	request.TrustRoot.SHA256 = stringPointer(sha256Hex(request.PinnedRoot))
	_, err := verifyAt(context.Background(), request, fixtureTime)
	requireDiagnosticID(t, err, idDuplicateJSONMember)
}

func offlineRequest(t *testing.T, bundle []byte) Request {
	t.Helper()
	path := "trusted_root.json"
	digest := fixtureRootSHA256
	tufRepository := "https://tuf-repo-cdn.sigstore.dev"
	revalidatedAt := "2026-08-05T00:00:00Z"
	refreshBefore := "2026-08-12T00:00:00Z"
	return Request{
		Mode:       ModeOffline,
		Bundle:     bundle,
		PinnedRoot: readFixture(t, filepath.Join("..", "..", "testdata", "attestation", path)),
		TrustRoot: policy.TrustRoot{
			Mode:          "pinned",
			Instance:      "sigstore-public-good",
			Path:          &path,
			SHA256:        &digest,
			TUFRepository: &tufRepository,
			RevalidatedAt: &revalidatedAt,
			RefreshBefore: &refreshBefore,
		},
		Identity:              fixtureIdentity(),
		ExpectedStatementJSON: fixtureExpectedStatement(t),
	}
}

func fixtureIdentity() IdentityExpectation {
	return IdentityExpectation{
		Issuer:                  "https://token.actions.githubusercontent.com",
		SignerURI:               "https://github.com/yunseo-kim/slsa-builder-attest-spike/.github/workflows/attest-npm-contract-spike.yml@refs/heads/main",
		WorkflowSHA:             fixtureWorkflowSHA,
		SourceRepositoryURI:     "https://github.com/yunseo-kim/slsa-builder-attest-spike",
		SourceRepositoryID:      "1323958651",
		SourceRepositoryOwnerID: "65203374",
		SourceDigest:            fixtureWorkflowSHA,
		SourceRef:               "refs/heads/main",
		RunnerEnvironment:       "github-hosted",
		RunInvocationURI:        "https://github.com/yunseo-kim/slsa-builder-attest-spike/actions/runs/31003035465/attempts/1",
	}
}

func fixtureBundle(t *testing.T) []byte {
	t.Helper()
	return readFixture(t, filepath.Join("..", "..", "testdata", "platform", "contracts", "valid.intoto.jsonl"))
}

func fixtureExpectedStatement(t *testing.T) []byte {
	t.Helper()
	return readFixture(t, filepath.Join("..", "..", "testdata", "attestation", "expected-statement.json"))
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func removeJSONMember(t *testing.T, data []byte, member string) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode JSON mutation fixture: %v", err)
	}
	delete(document, member)
	mutated, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode JSON mutation fixture: %v", err)
	}
	return mutated
}

func removeNestedJSONMember(t *testing.T, data []byte, object, member string) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode nested JSON mutation fixture: %v", err)
	}
	nested, ok := document[object].(map[string]any)
	if !ok {
		t.Fatalf("fixture member %s is not an object", object)
	}
	delete(nested, member)
	mutated, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode nested JSON mutation fixture: %v", err)
	}
	return mutated
}

func mutateStatementPayload(t *testing.T, data []byte, mutate func([]byte) []byte) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode bundle mutation fixture: %v", err)
	}
	envelope, ok := document["dsseEnvelope"].(map[string]any)
	if !ok {
		t.Fatal("fixture dsseEnvelope is not an object")
	}
	payload, ok := envelope["payload"].(string)
	if !ok {
		t.Fatal("fixture DSSE payload is not a string")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode fixture DSSE payload: %v", err)
	}
	envelope["payload"] = base64.StdEncoding.EncodeToString(mutate(decoded))
	mutated, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode bundle mutation fixture: %v", err)
	}
	return mutated
}

func requireDiagnosticID(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want diagnostic %s", want)
	}
	var diagnosticError interface{ DiagnosticID() string }
	if !errors.As(err, &diagnosticError) {
		t.Fatalf("error %T does not expose a diagnostic ID: %v", err, err)
	}
	if got := diagnosticError.DiagnosticID(); got != want {
		t.Fatalf("diagnostic ID = %s, want %s: %v", got, want, err)
	}
}

func stringPointer(value string) *string { return &value }

type countingDenyTransport struct{ calls atomic.Int64 }

func (transport *countingDenyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	return nil, errors.New("network denied by offline verification test")
}
