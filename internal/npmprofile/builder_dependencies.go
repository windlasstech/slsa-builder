package npmprofile

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/provenance"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

const publishCLIBuilderField = "predicate.runDetails.builder.builderDependencies"

// publishCLIBuilderDependency derives the descriptor from the provisioning trust anchor.
func publishCLIBuilderDependency() (provenance.BuilderDependency, error) {
	pin, err := toolchainpins.PublishPin()
	if err != nil {
		return provenance.BuilderDependency{}, fmt.Errorf("load publish CLI pin: %w", err)
	}
	encoded, ok := strings.CutPrefix(pin.Integrity, "sha512-")
	if !ok {
		return provenance.BuilderDependency{}, npmValidationError(IDBuilderDependenciesPublishCLIMismatch, publishCLIBuilderField, "publish CLI pin must contain sha512 integrity")
	}
	digest, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(digest) != sha512.Size || base64.StdEncoding.EncodeToString(digest) != encoded {
		return provenance.BuilderDependency{}, npmValidationError(IDBuilderDependenciesPublishCLIMismatch, publishCLIBuilderField, "publish CLI pin must contain a canonical base64-encoded 64-byte SHA-512 digest")
	}
	return provenance.BuilderDependency{
		Name:        "publish-cli",
		URI:         "pkg:npm/npm@" + pin.Version,
		Digest:      map[string]string{"sha512": hex.EncodeToString(digest)},
		MediaType:   "application/gzip",
		Annotations: json.RawMessage(`{"digest_authority":"builder-pinned-record"}`),
	}, nil
}

func (npmProfileValidator) BuilderDependencyNames() []string {
	return []string{"publish-cli"}
}

func (npmProfileValidator) ValidateBuilderDependencies(dependencies []provenance.BuilderDependency) error {
	var publish provenance.BuilderDependency
	count := 0
	for _, dependency := range dependencies {
		if dependency.Name == "publish-cli" {
			publish = dependency
			count++
		}
	}
	// Identity is name-only: a renamed lookalike cannot satisfy the required entry.
	if count != 1 {
		return npmValidationError(IDBuilderDependenciesPublishCLIMismatch, publishCLIBuilderField, "exactly one publish-cli descriptor is required")
	}
	want, err := publishCLIBuilderDependency()
	if err != nil {
		return err
	}
	// Annotations are descriptive JSON, never evidence for the pin comparison.
	if publish.URI != want.URI || len(publish.Digest) != 1 || publish.Digest["sha512"] != want.Digest["sha512"] ||
		publish.MediaType != want.MediaType || publish.HasMalformedMembers() || publish.HasUnexpectedFields() {
		return npmValidationError(IDBuilderDependenciesPublishCLIMismatch, publishCLIBuilderField, "publish CLI must match the embedded pin and declared descriptor shape")
	}
	return nil
}
