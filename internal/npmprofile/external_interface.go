package npmprofile

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/identity"
)

func (parameters ExternalParameters) MarshalJSON() ([]byte, error) {
	if err := validateFinalExternalParameters(parameters, ""); err != nil {
		return nil, err
	}
	type wire ExternalParameters
	return json.Marshal(wire(parameters))
}

func validateFinalExternalShape(encoded []byte) error {
	var top map[string]json.RawMessage
	if json.Unmarshal(encoded, &top) != nil || !exactObjectKeys(top, []string{"workflow", "package", "publish", "distribution"}, []string{"source"}) {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters", "external parameter groups are missing or unexpected")
	}
	shapes := map[string][]string{
		"source": {"ref"}, "workflow": {"ref", "repository", "path"},
		"package": {"directory"}, "publish": {"input_registry_url", "input_dist_tag", "input_access"},
		"distribution": {"release_asset_mode", "linked_artifact_metadata"},
	}
	for _, name := range []string{"source", "workflow", "package", "publish", "distribution"} {
		raw, present := top[name]
		if name == "source" && !present {
			continue
		}
		var members map[string]json.RawMessage
		if json.Unmarshal(raw, &members) != nil || !exactObjectKeys(members, shapes[name], nil) {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters."+name, "interface members are missing or unexpected")
		}
		for _, key := range shapes[name] {
			value := members[key]
			if name == "source" && string(value) == `""` {
				return sourceRefError("a supplied source selector must not be empty")
			}
			if name == "publish" && string(value) == "null" {
				continue
			}
			if name == "distribution" {
				if string(value) == "true" || string(value) == "false" {
					continue
				}
			} else if len(value) > 0 && value[0] == '"' {
				continue
			}
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters."+name+"."+key, "interface member has the wrong type")
		}
	}
	return nil
}

func validateFinalExternalParameters(parameters ExternalParameters, expectedRepository string) error {
	if identity.ValidateCanonicalRepositoryURI(parameters.Workflow.Repository) != nil || expectedRepository != "" && parameters.Workflow.Repository != expectedRepository {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.workflow.repository", "workflow repository must match the canonical caller repository")
	}
	if !validFullGitRef(parameters.Workflow.Ref) {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.workflow.ref", "workflow ref must be a full Git ref")
	}
	if parameters.Workflow.Path == NPMWorkflowPath {
		return npmValidationError("windlass.verify.error.build-config-uri-mismatch", "externalParameters.workflow.path", "caller workflow cannot be the reusable builder")
	}
	if !validCallerWorkflowPath(parameters.Workflow.Path) {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.workflow.path", "caller workflow path is invalid")
	}
	if parameters.Source.Ref != "" {
		if identity.ValidateReleaseRef(parameters.Source.Ref) != nil || !validFullGitRef(parameters.Source.Ref) || strings.HasPrefix(parameters.Workflow.Ref, "refs/tags/") && parameters.Source.Ref != parameters.Workflow.Ref {
			return sourceRefError("source selector must be a canonical tag without a tag-invocation conflict")
		}
	}
	directory := parameters.Package.Directory
	if directory == "" || path.IsAbs(directory) || path.Clean(directory) != directory || strings.ContainsAny(directory, "\\\x00") {
		return npmValidationError(IDPackageResolutionInvalid, "externalParameters.package.directory", "package directory must be repository relative and canonical")
	}
	for _, segment := range strings.Split(directory, "/") {
		if segment == ".." {
			return npmValidationError(IDPackageResolutionInvalid, "externalParameters.package.directory", "package directory must not traverse parents")
		}
	}
	if parameters.Distribution.ReleaseAssetMode || parameters.Distribution.LinkedArtifactMetadata {
		return npmValidationError("windlass.verify.error.release-asset-mode-schema-error", "externalParameters.distribution", "distribution modes are not implemented")
	}
	for _, raw := range []*string{parameters.Publish.InputRegistryURL, parameters.Publish.InputDistTag, parameters.Publish.InputAccess} {
		if raw != nil && strings.Trim(*raw, " \t\r\n\v\f") == "" {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.publish", "omitted raw inputs must be null")
		}
	}
	_, err := ResolvePublishIntent(stringOrEmpty(parameters.Publish.InputRegistryURL), stringOrEmpty(parameters.Publish.InputDistTag), stringOrEmpty(parameters.Publish.InputAccess), nil)
	return err
}

func effectiveBuiltRef(parameters ExternalParameters) string {
	if parameters.Source.Ref != "" {
		return parameters.Source.Ref
	}
	return parameters.Workflow.Ref
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
