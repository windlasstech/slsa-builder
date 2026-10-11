package attestation

import (
	"net/http"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/policy"
)

func TestOnlineRootRequiresOnlyPublicGoodTUF(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*policy.TrustRoot)
	}{
		{"valid", func(*policy.TrustRoot) {}},
		{"wrong mode", func(r *policy.TrustRoot) { r.Mode = "pinned" }},
		{"wrong instance", func(r *policy.TrustRoot) { r.Instance = "private" }},
		{"path", func(r *policy.TrustRoot) { r.Path = new("") }},
		{"digest", func(r *policy.TrustRoot) { r.SHA256 = new("") }},
		{"TUF repository", func(r *policy.TrustRoot) { r.TUFRepository = new(publicGoodTUF) }},
		{"revalidated at", func(r *policy.TrustRoot) { r.RevalidatedAt = new("") }},
		{"refresh before", func(r *policy.TrustRoot) { r.RefreshBefore = new("") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := policy.TrustRoot{Mode: "tuf", Instance: publicGoodInstance}
			test.mutate(&root)
			err := validateOnlineRoot(root)
			if test.name == "valid" {
				if err != nil {
					t.Fatalf("public-good TUF root rejected: %v", err)
				}
				return
			}
			requireDiagnosticID(t, err, idVerificationModeInvalid)
		})
	}
}

func TestAcquireTrustedMaterialOfflineAndInvalidModes(t *testing.T) {
	original := http.DefaultTransport
	transport := &countingDenyTransport{}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, test := range []struct {
		name string
		mode Mode
		id   string
	}{
		{"offline delegation", ModeOffline, ""},
		{"missing mode", "", idVerificationModeInvalid},
		{"unknown mode", "other", idVerificationModeInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := offlineRequest(t, fixtureBundle(t))
			request.Mode = test.mode
			material, err := acquireTrustedMaterial(request, fixtureTime)
			if test.id != "" {
				requireDiagnosticID(t, err, test.id)
				if material != nil {
					t.Fatal("rejected acquisition returned trusted material, want nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("acquire offline root: %v", err)
			}
			if material == nil || len(material.FulcioCertificateAuthorities()) == 0 || len(material.RekorLogs()) == 0 {
				t.Fatal("offline root lacks Fulcio or Rekor material, want fixture trust anchors")
			}
		})
	}
	if got := transport.calls.Load(); got != 0 {
		t.Fatalf("network calls = %d, want 0", got)
	}
}

func TestPinnedRootRejectsMalformedInputs(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		mutate   func(*Request)
		zeroTime bool
		id       string
	}{
		{"wrong root mode", func(r *Request) { r.TrustRoot.Mode = "tuf" }, false, idVerificationModeInvalid},
		{"wrong instance", func(r *Request) { r.TrustRoot.Instance = "private" }, false, idVerificationModeInvalid},
		{"missing path", func(r *Request) { r.TrustRoot.Path = nil }, false, idVerificationModeInvalid},
		{"empty path", func(r *Request) { r.TrustRoot.Path = new("") }, false, idVerificationModeInvalid},
		{"missing digest", func(r *Request) { r.TrustRoot.SHA256 = nil }, false, idVerificationModeInvalid},
		{"missing repository", func(r *Request) { r.TrustRoot.TUFRepository = nil }, false, idVerificationModeInvalid},
		{"missing revalidation", func(r *Request) { r.TrustRoot.RevalidatedAt = nil }, false, idVerificationModeInvalid},
		{"missing refresh", func(r *Request) { r.TrustRoot.RefreshBefore = nil }, false, idVerificationModeInvalid},
		{"wrong repository", func(r *Request) { r.TrustRoot.TUFRepository = new("https://other.example") }, false, idUngovernedTrustRoot},
		{"malformed JSON", func(r *Request) { r.PinnedRoot = []byte("{"); r.TrustRoot.SHA256 = new(sha256Hex(r.PinnedRoot)) }, false, idUngovernedTrustRoot},
		{"invalid root schema", func(r *Request) {
			r.PinnedRoot = []byte(`{"mediaType":"unsupported"}`)
			r.TrustRoot.SHA256 = new(sha256Hex(r.PinnedRoot))
		}, false, idUngovernedTrustRoot},
		{"malformed revalidation", func(r *Request) { r.TrustRoot.RevalidatedAt = new("not-a-time") }, false, idVerificationModeInvalid},
		{"malformed refresh", func(r *Request) { r.TrustRoot.RefreshBefore = new("not-a-time") }, false, idVerificationModeInvalid},
		{"equal deadlines", func(r *Request) { r.TrustRoot.RefreshBefore = r.TrustRoot.RevalidatedAt }, false, idVerificationModeInvalid},
		{"reversed deadlines", func(r *Request) { r.TrustRoot.RefreshBefore = new("2026-08-04T00:00:00Z") }, false, idVerificationModeInvalid},
		{"zero verification time", func(*Request) {}, true, idVerificationModeInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := offlineRequest(t, fixtureBundle(t))
			test.mutate(&request)
			now := fixtureTime
			if test.zeroTime {
				now = time.Time{}
			}
			material, err := acquireTrustedMaterial(request, now)
			requireDiagnosticID(t, err, test.id)
			if material != nil {
				t.Fatal("invalid root returned trusted material, want nil")
			}
		})
	}
}
