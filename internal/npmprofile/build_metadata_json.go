package npmprofile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

// UnmarshalJSON requires observed timestamps while leaving their policy validation to provenance.
func (metadata *BuildMetadata) UnmarshalJSON(encoded []byte) error {
	if err := canonicaljson.Validate(encoded); err != nil {
		return fmt.Errorf("validate build metadata JSON: %w", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		return fmt.Errorf("decode build metadata JSON: %w", err)
	}
	for key := range members {
		switch key {
		case "schema_version", "started_on", "finished_on", "primary_artifact", "external_parameters", "resolved_dependencies", "manifest_evidence", "build_script_disposition":
		default:
			return fmt.Errorf("decode build metadata JSON: json: unknown field %q", key)
		}
	}
	if raw, present := members["manifest_evidence"]; !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return npmValidationError(IDPackageManifestEvidence, "manifest_evidence", "original manifest evidence is required")
	}
	if raw, present := members["build_script_disposition"]; !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return npmValidationError(IDBuildScriptDisposition, "build_script_disposition", "build script disposition is required")
	}
	for _, key := range []string{"schema_version", "started_on", "finished_on", "primary_artifact", "external_parameters", "resolved_dependencies", "manifest_evidence"} {
		raw, present := members[key]
		if !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("build metadata requires non-null %s", key)
		}
	}
	var artifact map[string]json.RawMessage
	if json.Unmarshal(members["primary_artifact"], &artifact) != nil || !exactObjectKeys(artifact, []string{"artifact_name", "payload_file_name", "sha256", "sha512"}, nil) {
		return fmt.Errorf("build metadata primary_artifact violates its closed shape")
	}
	for _, key := range []string{"artifact_name", "payload_file_name", "sha256", "sha512"} {
		var value *string
		if json.Unmarshal(artifact[key], &value) != nil || value == nil {
			return fmt.Errorf("build metadata primary_artifact requires string %s", key)
		}
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
	// ResourceDescriptor captures unknown members instead of rejecting them itself.
	for index, descriptor := range decoded.ResolvedDependencies {
		if descriptor.HasUnexpectedFields() {
			type descriptorJSON provenance.ResourceDescriptor
			var strict struct {
				metadataJSON
				ResolvedDependencies []descriptorJSON `json:"resolved_dependencies"`
			}
			decoder := json.NewDecoder(bytes.NewReader(encoded))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&strict); err != nil {
				return fmt.Errorf("decode build metadata JSON: %w", err)
			}
			return fmt.Errorf("decode build metadata JSON: resolved_dependencies[%d] contains unrecognized members", index)
		}
	}
	if decoded.StartedOn == nil {
		return fmt.Errorf("build metadata requires started_on")
	}
	if decoded.FinishedOn == nil {
		return fmt.Errorf("build metadata requires finished_on")
	}
	decoded.metadataJSON.StartedOn = *decoded.StartedOn
	decoded.metadataJSON.FinishedOn = *decoded.FinishedOn
	if decoded.SchemaVersion != "1" {
		return fmt.Errorf("build metadata schema_version must equal 1")
	}
	var external map[string]json.RawMessage
	if json.Unmarshal(decoded.ExternalParameters, &external) != nil || external == nil {
		return fmt.Errorf("build metadata external_parameters must be an object")
	}
	if err := validateBuildDisposition(decoded.ManifestEvidence, path.Dir(decoded.ManifestEvidence.Path), decoded.BuildScriptDisposition); err != nil {
		return err
	}
	*metadata = BuildMetadata(decoded.metadataJSON)
	return nil
}
