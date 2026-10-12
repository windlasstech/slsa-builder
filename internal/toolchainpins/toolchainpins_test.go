package toolchainpins

import (
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

const (
	wantTarballURL = "https://registry.npmjs.org/npm/-/npm-11.21.0.tgz"
	wantIntegrity  = "sha512-Zov8KhamNneiLdELtj5YALtNmJW4L4fCLTzjfpzXG2w6MSHcf0UxgdlK5uloCuksWT+7mGUU7wi79cO6RqivPg=="
)

func TestPublishPinReturnsEmbeddedRecord(t *testing.T) {
	t.Parallel()

	got, err := PublishPin()
	if err != nil {
		t.Fatalf("load publish pin: %v", err)
	}
	if got.Version != "11.21.0" || got.TarballURL != wantTarballURL || got.Integrity != wantIntegrity {
		t.Fatalf("publish pin = %+v, want version 11.21.0, URL %q, integrity %q", got, wantTarballURL, wantIntegrity)
	}
}

func TestNodeRuntimePairReturnsEmbeddedRecord(t *testing.T) {
	t.Parallel()

	got, err := NodeRuntimePair()
	if err != nil {
		t.Fatalf("load Node.js runtime pair: %v", err)
	}
	if got.NodeVersion != "24.21.0" || got.NPMVersion != "11.19.0" {
		t.Fatalf("runtime pair = %+v, want Node.js 24.21.0 / npm 11.19.0", got)
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

	tests := []struct {
		name      string
		input     string
		wantError string
	}{
		{"duplicate member", `{"node_version":"24.21.0","npm_version":"11.19.0","npm_version":"11.20.0"}`, "duplicate JSON object member"},
		{"unknown member", `{"node_version":"24.21.0","npm_version":"11.19.0","extra":1}`, "unknown field"},
		{"invalid Node.js version", `{"node_version":"24.21","npm_version":"11.19.0"}`, "node_version"},
		{"invalid npm version", `{"node_version":"24.21.0","npm_version":"v11.19.0"}`, "npm_version"},
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

func TestEmbeddedLoadersAreConcurrent(t *testing.T) {
	t.Parallel()

	for range 32 {
		t.Run("parallel caller", func(t *testing.T) {
			t.Parallel()

			publish, err := PublishPin()
			if err != nil || publish.Version != "11.21.0" {
				t.Fatalf("concurrent publish pin = %+v, error = %v", publish, err)
			}
			runtime, err := NodeRuntimePair()
			if err != nil || runtime.NodeVersion != "24.21.0" {
				t.Fatalf("concurrent runtime pair = %+v, error = %v", runtime, err)
			}
		})
	}
}
