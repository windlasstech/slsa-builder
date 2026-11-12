package npmprofile

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func validateRunnerDescriptor(value provenance.ResourceDescriptor) error {
	if value.Name != "runner-image" || value.URI == "" || value.HasDigest() || value.HasMediaType() || value.HasUnexpectedFields() ||
		!exactObjectKeys(value.Annotations, []string{"deployment_version", "operating_system", "image_version", "runner_label"}, nil) {
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image", "runner-image must have exactly name, uri, and the four governed annotations")
	}
	if !validRunnerSoftwareURI(value.URI) {
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image.uri", "runner-image URI must be the captured actions/runner-images software report")
	}

	var operatingSystem, imageVersion, runnerLabel string
	for name, target := range map[string]*string{
		"operating_system": &operatingSystem,
		"image_version":    &imageVersion,
		"runner_label":     &runnerLabel,
	} {
		if err := json.Unmarshal(value.Annotations[name], target); err != nil {
			return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image.annotations", "runner-image annotations must be strings")
		}
	}
	if strings.TrimSpace(operatingSystem) == "" || strings.TrimSpace(runnerLabel) == "" {
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image.annotations", "runner-image operating_system and runner_label must be non-empty")
	}

	rawDeployment := value.Annotations["deployment_version"]
	if string(rawDeployment) != "null" {
		var deployment string
		if err := json.Unmarshal(rawDeployment, &deployment); err != nil || deployment == "" {
			return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image.annotations", "runner-image deployment_version must be null or a non-empty string")
		}
	}

	projection, validVersion := runnerImageReleaseProjection(imageVersion)
	if !validVersion {
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image.annotations.image_version", "runner-image image_version must contain three numeric components")
	}
	releaseTag, validURI := runnerIncludedSoftwareReleaseTag(value.URI)
	if !validURI || projection != releaseTag {
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image.uri", "runner-image URI release tag must equal the offline image_version projection")
	}
	return nil
}

func runnerIncludedSoftwareReleaseTag(value string) (string, bool) {
	if !validRunnerSoftwareURI(value) {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", false
	}
	const reportPrefix = "/actions/runner-images/blob/"
	if !strings.HasPrefix(parsed.Path, reportPrefix) {
		return "", false
	}
	segments := strings.Split(strings.TrimPrefix(parsed.Path, reportPrefix), "/")
	if len(segments) != 5 || segments[0] != "ubuntu24" || segments[2] != "images" || segments[3] != "ubuntu" || segments[4] != "Ubuntu2404-Readme.md" {
		return "", false
	}
	components := strings.Split(segments[1], ".")
	if len(components) != 2 || !runnerNumericComponents(components) {
		return "", false
	}
	return segments[1], true
}

func runnerNumericComponents(components []string) bool {
	for _, component := range components {
		if component == "" || strings.IndexFunc(component, func(value rune) bool { return value < '0' || value > '9' }) >= 0 {
			return false
		}
	}
	return true
}
