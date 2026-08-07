package npmprofile

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryMetadataPreflight(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.EscapedPath() != "/%40windlass%2Fslsa-builder" {
			t.Errorf("request = %s %s", request.Method, request.URL.EscapedPath())
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"name":"@windlass/slsa-builder","versions":{"1.2.2":{"name":"@windlass/slsa-builder","version":"1.2.2","dist":{"integrity":"sha512-YQ==","tarball":"https://registry.npmjs.org/example.tgz"}}}}`)
	}))
	defer server.Close()

	client, err := NewRegistryClient(RegistryClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	state, err := client.Preflight(context.Background(), "@windlass/slsa-builder", "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if !state.PackageExists || state.VersionExists {
		t.Fatalf("state = %#v, want existing package and absent version", state)
	}
}

func TestRegistryMetadataPackageAbsent(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", request.Method)
		}
		writer.WriteHeader(http.StatusNotFound)
		if _, err := writer.Write([]byte(`{"error":"Not found"}`)); err != nil {
			t.Errorf("write package-not-found response: %v", err)
		}
	}))
	defer server.Close()
	client, err := NewRegistryClient(RegistryClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	state, err := client.Preflight(context.Background(), "missing-package", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if state.PackageExists || state.VersionExists {
		t.Fatalf("state = %#v, want absent package", state)
	}
}

func TestRegistryAttestations(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.EscapedPath() != "/-/npm/v1/attestations/%40windlass%2Fslsa-builder@1.2.3" {
			t.Errorf("request = %s %s", request.Method, request.URL.EscapedPath())
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"attestations":[{"predicateType":"https://slsa.dev/provenance/v1","bundle":{"exact":"bytes"}},{"predicateType":"https://github.com/npm/attestation/tree/main/specs/publish/v0.1","bundle":{"ignored":true}}]}`)
	}))
	defer server.Close()

	client, err := NewRegistryClient(RegistryClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	state, err := client.Attestations(context.Background(), "@windlass/slsa-builder", "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if !state.Found || len(state.Attestations) != 2 || string(state.Attestations[0].Bundle) != `{"exact":"bytes"}` {
		t.Fatalf("attestation state = %#v", state)
	}
}

func TestRegistryClientRejectsInsecureURL(t *testing.T) {
	t.Parallel()
	for _, rawURL := range []string{
		"http://registry.npmjs.org/",
		"https://token@registry.npmjs.org/",
		"https://registry.npmjs.org/path/",
		"https://registry.npmjs.org/?token=secret",
		"https://registry.npmjs.org/#fragment",
	} {
		if _, err := NewRegistryClient(RegistryClientConfig{RegistryURL: rawURL}); err == nil {
			t.Errorf("NewRegistryClient(%q) accepted unsafe URL", rawURL)
		}
	}
}

func TestRegistryClientRejectsInvalidPackageNames(t *testing.T) {
	t.Parallel()

	client, err := NewRegistryClient(RegistryClientConfig{RegistryURL: "https://registry.npmjs.org/"})
	if err != nil {
		t.Fatal(err)
	}
	for _, packageName := range []string{"foo bar", "foo@bar", "@scope/name@extra", strings.Repeat("a", 215)} {
		if _, err := client.Preflight(context.Background(), packageName, "1.0.0"); err == nil {
			t.Errorf("Preflight() accepted invalid package name %q", packageName)
		}
	}
}

func TestRegistryClientRejectsRedirect(t *testing.T) {
	t.Parallel()

	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer target.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer server.Close()
	client, err := NewRegistryClient(RegistryClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Preflight(context.Background(), "safe-package", "1.0.0"); err == nil {
		t.Fatal("registry metadata client followed or accepted a redirect")
	}
}
