// Package workflowcheck performs static conformance checks on trusted workflows.
package workflowcheck

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	tarballArtifactName  = "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}"
	metadataArtifactName = "js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}"
)

var pinnedActionPattern = regexp.MustCompile(`^[^@\s]+@[0-9a-f]{40}$`)

// BuildJobResult is the machine-readable N04 build-job conformance result.
type BuildJobResult struct {
	Result             string   `json:"result"`
	SigningPermission  bool     `json:"signing_permission"`
	MutationPermission bool     `json:"mutation_permission"`
	ArtifactNames      []string `json:"artifact_names"`
}

type workflowDocument struct {
	Permissions map[string]string   `yaml:"permissions"`
	Jobs        map[string]buildJob `yaml:"jobs"`
}

type buildJob struct {
	RunsOn      string            `yaml:"runs-on"`
	Permissions map[string]string `yaml:"permissions"`
	Concurrency concurrency       `yaml:"concurrency"`
	Steps       []workflowStep    `yaml:"steps"`
}

type concurrency struct {
	Group            string `yaml:"group"`
	CancelInProgress bool   `yaml:"cancel-in-progress"`
}

type workflowStep struct {
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
}

// CheckBuildJob verifies the permission, runtime, hardening, and handoff contract for job build.
func CheckBuildJob(path string) (BuildJobResult, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return BuildJobResult{}, fmt.Errorf("read workflow: %w", err)
	}
	var document workflowDocument
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return BuildJobResult{}, fmt.Errorf("decode workflow: %w", err)
	}
	if document.Permissions == nil || len(document.Permissions) != 0 {
		return BuildJobResult{}, fmt.Errorf("top-level permissions must be an explicit empty mapping")
	}
	build, ok := document.Jobs["build"]
	if !ok {
		return BuildJobResult{}, fmt.Errorf("workflow has no build job")
	}

	result := BuildJobResult{
		SigningPermission:  hasWritePermission(build.Permissions, "id-token", "attestations"),
		MutationPermission: hasWritePermission(build.Permissions, "contents", "packages", "artifact-metadata"),
	}
	if result.SigningPermission {
		return BuildJobResult{}, fmt.Errorf("build job must not have signing permission")
	}
	if result.MutationPermission {
		return BuildJobResult{}, fmt.Errorf("build job must not have mutation permission")
	}
	if len(build.Permissions) != 1 || build.Permissions["contents"] != "read" {
		return BuildJobResult{}, fmt.Errorf("build job permissions must be exactly contents: read")
	}
	if build.RunsOn != "ubuntu-24.04" {
		return BuildJobResult{}, fmt.Errorf("build job runs-on must be ubuntu-24.04")
	}
	if build.Concurrency.Group != "npm-build-${{ github.repository }}-${{ github.ref_name }}" || !build.Concurrency.CancelInProgress {
		return BuildJobResult{}, fmt.Errorf("build job must use the pre-mutation concurrency contract")
	}
	if err := checkSteps(build.Steps, &result); err != nil {
		return BuildJobResult{}, err
	}
	slices.Sort(result.ArtifactNames)
	want := []string{metadataArtifactName, tarballArtifactName}
	if !slices.Equal(result.ArtifactNames, want) {
		return BuildJobResult{}, fmt.Errorf("build job artifact names are %q, want %q", result.ArtifactNames, want)
	}
	result.Result = "pass"
	return result, nil
}

func checkSteps(steps []workflowStep, result *BuildJobResult) error {
	if len(steps) == 0 || !strings.HasPrefix(steps[0].Uses, "step-security/harden-runner@") {
		return fmt.Errorf("harden-runner must be the first build step")
	}
	if scalar(steps[0].With["egress-policy"]) != "audit" {
		return fmt.Errorf("harden-runner must use egress-policy audit")
	}
	hasCheckout := false
	hasNode24 := false
	hasCorepack := false
	hasBuild := false
	for _, current := range steps {
		if current.Uses != "" && !pinnedActionPattern.MatchString(current.Uses) {
			return fmt.Errorf("action reference %q is not pinned to a full SHA", current.Uses)
		}
		switch {
		case strings.HasPrefix(current.Uses, "actions/checkout@"):
			hasCheckout = scalar(current.With["persist-credentials"]) == "false"
		case strings.HasPrefix(current.Uses, "actions/setup-node@"):
			hasNode24 = scalar(current.With["node-version"]) == "24"
		case strings.HasPrefix(current.Uses, "actions/upload-artifact@"):
			result.ArtifactNames = append(result.ArtifactNames, scalar(current.With["name"]))
		}
		if strings.Contains(current.Run, "corepack enable") {
			hasCorepack = true
		}
		if strings.Contains(current.Run, "npm-profile-build") {
			hasBuild = true
		}
	}
	if !hasCheckout {
		return fmt.Errorf("build job must checkout without persisted credentials")
	}
	if !hasNode24 {
		return fmt.Errorf("build job must set up Node.js 24")
	}
	if !hasCorepack {
		return fmt.Errorf("build job must enable Corepack")
	}
	if !hasBuild {
		return fmt.Errorf("build job must invoke npm-profile-build")
	}
	return nil
}

func hasWritePermission(permissions map[string]string, names ...string) bool {
	for _, name := range names {
		if permissions[name] == "write" {
			return true
		}
	}
	return false
}

func scalar(value any) string {
	return fmt.Sprint(value)
}
