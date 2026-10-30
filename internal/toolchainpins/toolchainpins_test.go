package toolchainpins

import (
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

const (
	wantTarballURL = "https://registry.npmjs.org/npm/-/npm-11.21.1.tgz"
	wantIntegrity  = "sha512-YTzL6QBEJXyHcTu0JiUc7RKMp+mzlJtl7rGLmERaXUQeFHIMc9atdr6G9M9C8vDWbv40epoTQAzNL2/PtZVDhQ=="
	wantNodeURL    = "https://nodejs.org/download/release/v24.22.1/node-v24.22.1-linux-x64.tar.gz"
	wantNodeSHA256 = "eb6728e77c576a35162c122c45ccadf224d89ff00abc8dae46cc2897c6002d7c"
)

func TestPublishPinReturnsEmbeddedRecord(t *testing.T) {
	t.Parallel()

	got, err := PublishPin()
	if err != nil {
		t.Fatalf("load publish pin: %v", err)
	}
	if got.Version != "11.21.1" || got.TarballURL != wantTarballURL || got.Integrity != wantIntegrity {
		t.Fatalf("publish pin = %+v, want version 11.21.1, URL %q, integrity %q", got, wantTarballURL, wantIntegrity)
	}
}

func TestNodeRuntimePairReturnsEmbeddedRecord(t *testing.T) {
	t.Parallel()

	got, err := NodeRuntimePair()
	if err != nil {
		t.Fatalf("load Node.js runtime pair: %v", err)
	}
	if got.NodeVersion != "24.22.1" || got.NPMVersion != "11.19.1" {
		t.Fatalf("runtime pair = %+v, want Node.js 24.22.1 / npm 11.19.1", got)
	}
	if len(got.Artifacts) != 1 {
		t.Fatalf("artifacts = %+v, want exactly the linux-x64 pin", got.Artifacts)
	}
	artifact, ok := got.Artifacts["linux-x64"]
	if !ok || artifact.URL != wantNodeURL || artifact.SHA256 != wantNodeSHA256 {
		t.Fatalf("linux-x64 artifact = %+v, want URL %q, SHA-256 %q", artifact, wantNodeURL, wantNodeSHA256)
	}
}

func TestNodeRuntimePairIsolatesCallerMutation(t *testing.T) {
	t.Parallel()

	first, err := NodeRuntimePair()
	if err != nil {
		t.Fatalf("load Node.js runtime pair: %v", err)
	}
	// When a caller corrupts its returned artifact map.
	first.Artifacts["linux-x64"] = NodeArtifact{URL: "https://tampered.example.com/node.tgz", SHA256: "f" + wantNodeSHA256[1:]}
	first.Artifacts["darwin-arm64"] = NodeArtifact{}
	// Then a later caller still receives the intact cached pin.
	second, err := NodeRuntimePair()
	if err != nil {
		t.Fatalf("reload Node.js runtime pair: %v", err)
	}
	artifact, ok := second.Artifacts["linux-x64"]
	if len(second.Artifacts) != 1 || !ok || artifact.URL != wantNodeURL || artifact.SHA256 != wantNodeSHA256 {
		t.Fatalf("artifacts after caller mutation = %+v, want the intact linux-x64 pin", second.Artifacts)
	}
}

func TestDecodePublishPinRejectsInvalidRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantError string
	}{
		{"duplicate member", `{"version":"11.21.0","version":"11.22.0","tarball_url":"` + wantTarballURL + `","integrity":"` + wantIntegrity + `"}`, "duplicate JSON object member"},
		{"escaped duplicate member", `{"version":"11.21.0","\u0076ersion":"11.22.0","tarball_url":"` + wantTarballURL + `","integrity":"` + wantIntegrity + `"}`, "duplicate JSON object member"},
		{"unknown member", `{"version":"11.21.0","tarball_url":"` + wantTarballURL + `","integrity":"` + wantIntegrity + `","other":true}`, "unknown field"},
		{"invalid version", `{"version":"11.21","tarball_url":"` + wantTarballURL + `","integrity":"` + wantIntegrity + `"}`, "version"},
		{"http URL", `{"version":"11.21.0","tarball_url":"http://registry.npmjs.org/npm/-/npm-11.21.0.tgz","integrity":"` + wantIntegrity + `"}`, "tarball_url"},
		{"port-only authority URL", `{"version":"11.21.0","tarball_url":"https://:443/npm.tgz","integrity":"` + wantIntegrity + `"}`, "tarball_url"},
		{"wrong SRI prefix", `{"version":"11.21.0","tarball_url":"` + wantTarballURL + `","integrity":"sha256-` + strings.TrimPrefix(wantIntegrity, "sha512-") + `"}`, "integrity"},
		{"bad SRI base64", `{"version":"11.21.0","tarball_url":"` + wantTarballURL + `","integrity":"sha512-!!!"}`, "integrity"},
		{"short SRI digest", `{"version":"11.21.0","tarball_url":"` + wantTarballURL + `","integrity":"sha512-YQ=="}`, "integrity"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := decodePublishPin([]byte(test.input))
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("decode publish pin error = %v, want %q", err, test.wantError)
			}
			if strings.Contains(test.name, "duplicate member") {
				var duplicate *canonicaljson.DuplicateMemberError
				if !errors.As(err, &duplicate) {
					t.Fatalf("duplicate member error = %v, want *canonicaljson.DuplicateMemberError", err)
				}
			}
		})
	}
}

func TestDecodeNodeRuntimePairRejectsInvalidRecords(t *testing.T) {
	t.Parallel()

	artifact := `"url":"` + wantNodeURL + `","sha256":"` + wantNodeSHA256 + `"`
	valid := `{"node_version":"24.21.0","npm_version":"11.19.0","artifacts":{"linux-x64":{` + artifact + `}}}`
	tests := []struct {
		name      string
		input     string
		wantError string
	}{
		{"duplicate member", `{"node_version":"24.21.0","npm_version":"11.19.0","npm_version":"11.20.0"}`, "duplicate JSON object member"},
		{"unknown member", `{"node_version":"24.21.0","npm_version":"11.19.0","extra":1}`, "unknown field"},
		{"invalid Node.js version", `{"node_version":"24.21","npm_version":"11.19.0"}`, "node_version"},
		{"invalid npm version", `{"node_version":"24.21.0","npm_version":"v11.19.0"}`, "npm_version"},
		{"missing artifacts", `{"node_version":"24.21.0","npm_version":"11.19.0"}`, "artifacts"},
		{"empty artifacts", `{"node_version":"24.21.0","npm_version":"11.19.0","artifacts":{}}`, "artifacts"},
		{"two platforms", strings.Replace(valid, `}}`, `},"darwin-arm64":{`+artifact+`}}`, 1), "exactly one platform"},
		{"unsupported platform", strings.Replace(valid, `"linux-x64"`, `"linux-arm64"`, 1), "platform"},
		{"uppercase platform key", strings.Replace(valid, `"linux-x64"`, `"Linux-x64"`, 1), "platform"},
		{"underscore platform key", strings.Replace(valid, `"linux-x64"`, `"linux_x64"`, 1), "platform"},
		{"duplicate platform key", strings.Replace(valid, `}}`, `},"linux-x64":{`+artifact+`}}`, 1), "duplicate JSON object member"},
		{"unknown artifact member", strings.Replace(valid, artifact, artifact+`,"size":1`, 1), "unknown field"},
		{"http artifact URL", strings.Replace(valid, "https://", "http://", 1), "url"},
		{"credentialed artifact URL", strings.Replace(valid, "https://nodejs.org", "https://user:secret@nodejs.org", 1), "url"},
		{"pathless artifact URL", strings.Replace(valid, wantNodeURL, "https://nodejs.org", 1), "url"},
		{"short sha256", strings.Replace(valid, wantNodeSHA256, "abc123", 1), "sha256"},
		{"uppercase sha256", strings.Replace(valid, wantNodeSHA256, strings.ToUpper(wantNodeSHA256), 1), "sha256"},
		{"non-hex sha256", strings.Replace(valid, wantNodeSHA256, "g"+wantNodeSHA256[1:], 1), "sha256"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := decodeNodeRuntimePair([]byte(test.input))
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("decode runtime pair error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestDecodeNodeRuntimePairAcceptsNonNodeJSHost(t *testing.T) {
	t.Parallel()

	// The artifact pin binds the digest, not the host: a mirror URL is valid.
	mirrorURL := "https://mirror.example.com/dist/v24.22.1/node-v24.22.1-linux-x64.tar.gz"
	input := `{"node_version":"24.22.1","npm_version":"11.19.1","artifacts":{"linux-x64":{"url":"` + mirrorURL + `","sha256":"` + wantNodeSHA256 + `"}}}`
	pair, err := decodeNodeRuntimePair([]byte(input))
	if err != nil {
		t.Fatalf("decode runtime pair: %v", err)
	}
	artifact, ok := pair.Artifacts["linux-x64"]
	if !ok || artifact.URL != mirrorURL || artifact.SHA256 != wantNodeSHA256 {
		t.Fatalf("artifacts = %+v, want linux-x64 pin with URL %q", pair.Artifacts, mirrorURL)
	}
}

func TestEmbeddedLoadersAreConcurrent(t *testing.T) {
	t.Parallel()

	for range 32 {
		t.Run("parallel caller", func(t *testing.T) {
			t.Parallel()

			publish, err := PublishPin()
			if err != nil || publish.Version != "11.21.1" {
				t.Fatalf("concurrent publish pin = %+v, error = %v", publish, err)
			}
			runtime, err := NodeRuntimePair()
			if err != nil || runtime.NodeVersion != "24.22.1" {
				t.Fatalf("concurrent runtime pair = %+v, error = %v", runtime, err)
			}
		})
	}
}
