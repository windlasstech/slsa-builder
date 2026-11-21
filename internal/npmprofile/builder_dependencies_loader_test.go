package npmprofile

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestPublishCLIBuilderDependencyLoadFailure(t *testing.T) {
	t.Parallel()
	// Given an unavailable embedded trust anchor, not a received descriptor mismatch.
	load := func() (toolchainpins.PublishNPM, error) {
		return toolchainpins.PublishNPM{}, errors.New("unavailable embedded pin")
	}
	// When constructing the publish dependency.
	_, err := loadPublishCLIBuilderDependency(load)
	// Then callers receive the structured invocation failure.
	requireNPMDiagnostic(t, err, "windlass.verify.error.input-unavailable")
}

func TestPublishCLIBuilderDependencyMalformedIntegrity(t *testing.T) {
	t.Parallel()
	for _, integrity := range []string{"sha256-abcd", "sha512-!", "sha512-" + base64.StdEncoding.EncodeToString([]byte("short")), "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64)) + "\n"} {
		t.Run(strings.TrimSpace(integrity), func(t *testing.T) {
			t.Parallel()
			// Given a successfully loaded pin with malformed integrity content.
			load := func() (toolchainpins.PublishNPM, error) {
				return toolchainpins.PublishNPM{Version: "11.21.1", Integrity: integrity}, nil
			}
			// When constructing the descriptor, then retain the content mismatch diagnostic.
			_, err := loadPublishCLIBuilderDependency(load)
			requireNPMDiagnostic(t, err, IDBuilderDependenciesPublishCLIMismatch)
		})
	}
}
