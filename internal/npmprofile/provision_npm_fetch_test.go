package npmprofile

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestProvisionNPMDownloadFailures(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			// Given a local TLS endpoint that fails or redirects.
			_, marker := provisionFakeNode(t)
			server := newFetchTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/distribution" {
					t.Errorf("redirect was followed: %s", r.URL.Path)
				}
				w.Header().Set("Location", "/redirect-target")
				w.WriteHeader(status)
			}))
			source := provisionSource(t, nil)
			source.fetch = func(ctx context.Context, url string, maximum int64, accept string) ([]byte, error) {
				if url != source.pin.TarballURL {
					t.Errorf("URL = %q", url)
				}
				return fetchHTTPS(ctx, server.URL+"/distribution", maximum, accept)
			}
			output := t.TempDir()
			// When the production fetch implementation handles the response.
			_, err := provisionNPM(t.Context(), output, source)
			// Then download failure precedes extraction and execution.
			requireProvisionID(t, err, "npm-provision-download")
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 0 {
				t.Fatalf("extraction occurred: %v, %v", entries, err)
			}
			requireProvisionNotInvoked(t, marker)
		})
	}
}

func TestProvisionNpmLive(t *testing.T) {
	if testing.Short() || os.Getenv("SLSA_BUILDER_LIVE_TOOLCHAIN") != "1" {
		t.Skip("live toolchain test: set SLSA_BUILDER_LIVE_TOOLCHAIN=1 to run")
	}
	// Given the real embedded pin and an isolated execution environment.
	pin, err := toolchainpins.PublishPin()
	if err != nil {
		t.Fatal(err)
	}
	environment, err := controlledEnvironment(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// When the public provisioning entrypoint downloads and authenticates the tarball.
	launcher, err := ProvisionNPM(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Then, only after success, exercise the launcher as live evidence.
	version, err := runCommandWithShimRoot(t.Context(), filepath.Dir(launcher), launcher, environment, []string{"--version"}, filepath.Dir(launcher))
	if err != nil || version != pin.Version {
		t.Fatalf("executed version = %q, %v; want %q", version, err, pin.Version)
	}
	t.Logf("verified URL: %s\nverified SRI: %s\nlauncher: %s\nexecuted npm version: %s", pin.TarballURL, pin.Integrity, launcher, version)
}
