package npmprofile

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

// UnmarshalJSON requires observed timestamps while leaving their policy validation to provenance.
func (metadata *BuildMetadata) UnmarshalJSON(encoded []byte) error {
	if err := canonicaljson.Validate(encoded); err != nil {
		return fmt.Errorf("validate build metadata JSON: %w", err)
	}
	type metadataJSON BuildMetadata
	var decoded struct {
		metadataJSON
		StartedOn  *string `json:"started_on"`
		FinishedOn *string `json:"finished_on"`
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return fmt.Errorf("decode build metadata JSON: %w", err)
	}
	if decoded.StartedOn == nil {
		return fmt.Errorf("build metadata requires started_on")
	}
	if decoded.FinishedOn == nil {
		return fmt.Errorf("build metadata requires finished_on")
	}
	decoded.metadataJSON.StartedOn = *decoded.StartedOn
	decoded.metadataJSON.FinishedOn = *decoded.FinishedOn
	*metadata = BuildMetadata(decoded.metadataJSON)
	return nil
}
