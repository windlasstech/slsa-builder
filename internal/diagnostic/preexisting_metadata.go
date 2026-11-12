package diagnostic

import (
	"encoding/json"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

// PublishMetadata is unsigned preflight evidence and never authorizes publication.
type PublishMetadata struct {
	PackageIdentityPreexisting *bool `json:"package_identity_preexisting"`
	PackageVersionPreexisting  *bool `json:"package_version_preexisting"`
}

func (metadata *PublishMetadata) UnmarshalJSON(encoded []byte) error {
	if err := canonicaljson.Validate(encoded); err != nil {
		return contractErrorf("invalid publish diagnostic metadata: %v", err)
	}
	var members map[string]json.RawMessage
	if json.Unmarshal(encoded, &members) != nil || members == nil {
		return contractErrorf("publish diagnostic metadata must be an object")
	}
	for key := range members {
		if key != "package_identity_preexisting" && key != "package_version_preexisting" {
			return contractErrorf("publish diagnostic metadata has unknown member %q", key)
		}
	}
	type wire PublishMetadata
	var decoded wire
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return contractErrorf("publish diagnostic observations must be boolean or null: %v", err)
	}
	*metadata = PublishMetadata(decoded)
	return nil
}
