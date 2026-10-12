package diagnostic

import (
	"bytes"
	"encoding/json"
	"regexp"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

var publishNPMVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// UnmarshalJSON preserves the closed metadata schema and rejects explicit null members.
func (metadata *DiagnosticMetadata) UnmarshalJSON(data []byte) error {
	if err := canonicaljson.Validate(data); err != nil {
		return contractErrorf("invalid diagnostic_metadata JSON: %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil || members == nil {
		return contractErrorf("diagnostic_metadata must be an object")
	}
	for key, value := range members {
		if key != "package_manifest" && key != "publish_npm_version" {
			return contractErrorf("diagnostic_metadata has unknown member %q", key)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return contractErrorf("diagnostic_metadata member %q must not be null", key)
		}
		if containsJSONNull(value) {
			return contractErrorf("diagnostic_metadata member %q must not contain null values", key)
		}
	}
	type wireMetadata DiagnosticMetadata
	var decoded wireMetadata
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return contractErrorf("invalid diagnostic_metadata: %v", err)
	}
	value := DiagnosticMetadata(decoded)
	if err := validateMetadata(&value); err != nil {
		return err
	}
	*metadata = value
	return nil
}

// containsJSONNull reports whether raw JSON holds a null value at any depth. The input has
// already passed canonicaljson.Validate, so a decode error cannot hide a null and rejection
// simply defers to the typed decoder below.
func containsJSONNull(data json.RawMessage) bool {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false
	}
	return jsonValueHasNull(value)
}

func jsonValueHasNull(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case map[string]any:
		for _, nested := range typed {
			if jsonValueHasNull(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if jsonValueHasNull(nested) {
				return true
			}
		}
	}
	return false
}
