package npmprofile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProvisionNPMRejectsCancelledContext(t *testing.T) {
	t.Parallel()
	// Given a cancelled request to the public, embedded-pin entry point.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// When provisioning starts.
	_, err := ProvisionNPM(ctx, t.TempDir())
	// Then it fails before the network fetch.
	requireProvisionID(t, err, "npm-provision-download")
}

func TestProvisionNPMRejectsMalformedIntegrity(t *testing.T) {
	t.Parallel()
	for _, integrity := range []string{"sha512-!", "sha512-YQ=="} {
		t.Run(integrity, func(t *testing.T) {
			t.Parallel()
			// Given invalid base64 or a digest of the wrong length.
			source := provisionSource(t, nil)
			source.pin.Integrity = integrity
			// When authenticating the fetched bytes.
			_, err := provisionNPM(t.Context(), t.TempDir(), source)
			// Then the existing digest diagnostic is preserved.
			requireProvisionID(t, err, "npm-provision-digest-mismatch")
		})
	}
}

func TestProvisionNPMRejectsUnusableOutput(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"empty", "regular file"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			// Given authenticated bytes and an unusable output directory.
			output := ""
			if scenario == "regular file" {
				output = filepath.Join(t.TempDir(), "file")
				if err := os.WriteFile(output, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			data := provisionArchive(t, `{"version":"11.21.0"}`, nil)
			// When provisioning attempts to establish its private directory.
			_, err := provisionNPM(t.Context(), output, provisionSource(t, data))
			// Then directory setup fails with the archive diagnostic.
			requireProvisionID(t, err, "npm-provision-archive-unsafe")
		})
	}
}

func TestProvisionNPMRejectsMissingNode(t *testing.T) {
	// Given a valid distribution with no node on PATH.
	t.Setenv("PATH", t.TempDir())
	data := provisionArchive(t, `{"version":"11.21.0"}`, nil)
	// When resolving the launcher runtime.
	_, err := provisionNPM(t.Context(), t.TempDir(), provisionSource(t, data))
	// Then provisioning cannot emit a usable launcher.
	requireProvisionID(t, err, "npm-provision-version-mismatch")
}

func TestProvisionNPMRejectsCorruptGzipTrailer(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"truncated", "checksum"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			// Given a tar whose gzip trailer is truncated or has an invalid checksum.
			data := provisionArchive(t, `{"version":"11.21.0"}`, nil)
			if scenario == "truncated" {
				data = data[:len(data)-1]
			} else {
				data[len(data)-8] ^= 1
			}
			// When draining beyond the tar EOF after digest authentication.
			_, err := provisionNPM(t.Context(), t.TempDir(), provisionSource(t, data))
			// Then the explicit copy bound does not hide gzip integrity errors.
			requireProvisionID(t, err, "npm-provision-archive-unsafe")
		})
	}
}
