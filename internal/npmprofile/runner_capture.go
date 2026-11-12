package npmprofile

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

const maxRunnerMetadata = 1 << 20

// runnerMetadataPath is the GitHub-hosted runner's fixed baked image metadata
// path. It is a var so tests can redirect the read to a hermetic fixture;
// production code never reassigns it.
var runnerMetadataPath = "/imagegeneration/imagedata.json"

func captureRunner() (RunnerCapture, error) {
	imageOS := os.Getenv("ImageOS")
	deploymentVersion := os.Getenv("ImageVersion")
	encoded, err := readBoundedRegularFile(runnerMetadataPath, maxRunnerMetadata)
	if err != nil {
		return RunnerCapture{}, runnerCaptureError(fmt.Sprintf("read GitHub runner image metadata: %v", err), imageOS, deploymentVersion)
	}
	return parseRunnerMetadata(encoded, imageOS, deploymentVersion)
}

func runnerCaptureError(message, imageOS, deploymentVersion string) error {
	return npmValidationError("windlass.verify.error.input-unavailable", "resolvedDependencies.runner-image",
		fmt.Sprintf("%s (ImageOS=%q, ImageVersion=%q)", message, imageOS, deploymentVersion))
}

// parseRunnerMetadata reads one bounded on-disk snapshot as the sole authority
// for the reported image identity and OS detail. ImageOS is diagnostic-only;
// deploymentVersion remains separate, non-authoritative rollout evidence.
func parseRunnerMetadata(encoded []byte, imageOS, deploymentVersion string) (RunnerCapture, error) {
	capture := RunnerCapture{DeploymentVersion: deploymentVersion}
	if err := canonicaljson.Validate(encoded); err != nil {
		return RunnerCapture{}, runnerCaptureError(fmt.Sprintf("validate GitHub runner image metadata: %v", err), imageOS, deploymentVersion)
	}
	var groups []struct {
		Group  string `json:"group"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(encoded, &groups); err != nil {
		return RunnerCapture{}, runnerCaptureError(fmt.Sprintf("decode GitHub runner image metadata: %v", err), imageOS, deploymentVersion)
	}
	seenRunnerImage := false
	for _, group := range groups {
		if group.Group != "Runner Image" {
			continue
		}
		if seenRunnerImage {
			return RunnerCapture{}, runnerCaptureError("runner image metadata repeats the Runner Image group", imageOS, deploymentVersion)
		}
		seenRunnerImage = true
		for _, line := range strings.Split(group.Detail, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "Image: "):
				if capture.ImageLabel != "" {
					return RunnerCapture{}, runnerCaptureError("runner image metadata repeats the Image detail line", imageOS, deploymentVersion)
				}
				capture.ImageLabel = strings.TrimSpace(strings.TrimPrefix(line, "Image: "))
			case strings.HasPrefix(line, "Version: "):
				if capture.ImageVersion != "" {
					return RunnerCapture{}, runnerCaptureError("runner image metadata repeats the Version detail line", imageOS, deploymentVersion)
				}
				capture.ImageVersion = strings.TrimSpace(strings.TrimPrefix(line, "Version: "))
			case strings.HasPrefix(line, "Included Software: "):
				if capture.IncludedSoftwareURL != "" {
					return RunnerCapture{}, runnerCaptureError("runner image metadata repeats the Included Software detail line", imageOS, deploymentVersion)
				}
				capture.IncludedSoftwareURL = strings.TrimSpace(strings.TrimPrefix(line, "Included Software: "))
			case strings.HasPrefix(line, "Image Release: "):
				if capture.ImageReleaseURL != "" {
					return RunnerCapture{}, runnerCaptureError("runner image metadata repeats the Image Release detail line", imageOS, deploymentVersion)
				}
				capture.ImageReleaseURL = strings.TrimSpace(strings.TrimPrefix(line, "Image Release: "))
			}
		}
		if capture.ImageLabel == "" || capture.ImageVersion == "" || capture.IncludedSoftwareURL == "" || capture.ImageReleaseURL == "" {
			return RunnerCapture{}, runnerCaptureError("runner image metadata lacks required detail lines", imageOS, deploymentVersion)
		}
	}
	if !seenRunnerImage {
		return RunnerCapture{}, runnerCaptureError("runner image metadata lacks the Runner Image group", imageOS, deploymentVersion)
	}
	if _, ok := runnerImageReleaseProjection(capture.ImageVersion); !ok {
		return RunnerCapture{}, runnerCaptureError("runner image metadata has a malformed full image version", imageOS, deploymentVersion)
	}
	if !validRunnerSoftwareURI(capture.IncludedSoftwareURL) {
		return RunnerCapture{}, runnerCaptureError("runner image metadata has a malformed Included Software URI", imageOS, deploymentVersion)
	}

	seenOperatingSystem := false
	for _, group := range groups {
		if group.Group != "Operating System" {
			continue
		}
		if seenOperatingSystem {
			return RunnerCapture{}, runnerCaptureError("runner image metadata repeats the Operating System group", imageOS, deploymentVersion)
		}
		seenOperatingSystem = true
		if strings.TrimSpace(group.Detail) == "" {
			return RunnerCapture{}, runnerCaptureError("runner image metadata lacks a non-empty Operating System detail", imageOS, deploymentVersion)
		}
		capture.OperatingSystem = group.Detail
	}
	if !seenOperatingSystem {
		return RunnerCapture{}, runnerCaptureError("runner image metadata lacks the Operating System group", imageOS, deploymentVersion)
	}
	return capture, nil
}

func runnerImageReleaseProjection(version string) (string, bool) {
	components := strings.Split(version, ".")
	if len(components) != 3 {
		return "", false
	}
	if !runnerNumericComponents(components) {
		return "", false
	}
	return components[0] + "." + components[1], true
}

func validRunnerSoftwareURI(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host == "github.com" && parsed.User == nil &&
		parsed.RawQuery == "" && parsed.Fragment == "" && strings.HasPrefix(parsed.Path, "/actions/runner-images/")
}
