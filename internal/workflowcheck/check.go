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
	tarballArtifactName          = "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}"
	metadataArtifactName         = "js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}"
	provenanceBundleArtifactName = "js-ts-npm-provenance-bundle-${{ github.run_id }}-${{ github.run_attempt }}"
)

var pinnedActionPattern = regexp.MustCompile(`^[^@\s]+@[0-9a-f]{40}$`)

// BuildJobResult is the machine-readable N04 build-job conformance result.
type BuildJobResult struct {
	Result             string   `json:"result"`
	SigningPermission  bool     `json:"signing_permission"`
	MutationPermission bool     `json:"mutation_permission"`
	ArtifactNames      []string `json:"artifact_names"`
}

// ProvenanceSignJobResult is the machine-readable P02 signing-job conformance result.
type ProvenanceSignJobResult struct {
	Result                       string `json:"result"`
	OIDCPermission               bool   `json:"oidc_permission"`
	AttestationStoragePermission bool   `json:"attestation_storage_permission"`
	MutationPermission           bool   `json:"mutation_permission"`
	BundleArtifactName           string `json:"bundle_artifact_name"`
}

type workflowDocument struct {
	Permissions map[string]string   `yaml:"permissions"`
	Jobs        map[string]buildJob `yaml:"jobs"`
}

type buildJob struct {
	Needs       any               `yaml:"needs"`
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

// CheckProvenanceSignJob verifies the isolated authority and handoff contract for provenance-sign.
func CheckProvenanceSignJob(path string) (ProvenanceSignJobResult, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return ProvenanceSignJobResult{}, fmt.Errorf("read workflow: %w", err)
	}
	var document workflowDocument
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return ProvenanceSignJobResult{}, fmt.Errorf("decode workflow: %w", err)
	}
	if document.Permissions == nil || len(document.Permissions) != 0 {
		return ProvenanceSignJobResult{}, fmt.Errorf("top-level permissions must be an explicit empty mapping")
	}
	job, ok := document.Jobs["provenance-sign"]
	if !ok {
		return ProvenanceSignJobResult{}, fmt.Errorf("workflow has no provenance-sign job")
	}
	result := ProvenanceSignJobResult{
		OIDCPermission:               job.Permissions["id-token"] == "write",
		AttestationStoragePermission: job.Permissions["attestations"] == "write",
		MutationPermission:           hasWritePermission(job.Permissions, "contents", "packages", "artifact-metadata"),
	}
	if len(job.Permissions) != 2 || job.Permissions["contents"] != "read" || !result.OIDCPermission {
		return ProvenanceSignJobResult{}, fmt.Errorf("provenance-sign permissions must be exactly contents: read and id-token: write")
	}
	if result.AttestationStoragePermission || result.MutationPermission {
		return ProvenanceSignJobResult{}, fmt.Errorf("provenance-sign must not have storage or mutation permission")
	}
	if job.RunsOn != "ubuntu-24.04" {
		return ProvenanceSignJobResult{}, fmt.Errorf("provenance-sign runs-on must be ubuntu-24.04")
	}
	if scalar(job.Needs) != "build" {
		return ProvenanceSignJobResult{}, fmt.Errorf("provenance-sign must depend only on build")
	}
	if err := checkProvenanceSignSteps(job.Steps, &result); err != nil {
		return ProvenanceSignJobResult{}, err
	}
	result.Result = "pass"
	return result, nil
}

func checkProvenanceSignSteps(steps []workflowStep, result *ProvenanceSignJobResult) error {
	if len(steps) == 0 || !strings.HasPrefix(steps[0].Uses, "step-security/harden-runner@") || scalar(steps[0].With["egress-policy"]) != "audit" {
		return fmt.Errorf("harden-runner with audit egress must be the first provenance-sign step")
	}
	downloads := make([]string, 0, 2)
	hasTrustedCheckout := false
	hasSigner := false
	for _, current := range steps {
		if current.Uses != "" && !pinnedActionPattern.MatchString(current.Uses) {
			return fmt.Errorf("action reference %q is not pinned to a full SHA", current.Uses)
		}
		if strings.HasPrefix(current.Uses, "actions/attest@") {
			return fmt.Errorf("actions/attest is forbidden in provenance-sign")
		}
		switch {
		case strings.HasPrefix(current.Uses, "actions/checkout@"):
			hasTrustedCheckout = scalar(current.With["repository"]) == "${{ job.workflow_repository }}" &&
				scalar(current.With["ref"]) == "${{ job.workflow_sha }}" &&
				scalar(current.With["persist-credentials"]) == "false"
		case strings.HasPrefix(current.Uses, "actions/download-artifact@"):
			downloads = append(downloads, scalar(current.With["name"]))
		case strings.HasPrefix(current.Uses, "actions/upload-artifact@"):
			result.BundleArtifactName = scalar(current.With["name"])
			if !strings.HasSuffix(scalar(current.With["path"]), "/*.intoto.jsonl") {
				return fmt.Errorf("provenance bundle upload must select only the deterministic bundle file")
			}
		}
		if strings.Contains(current.Run, "npm-profile-sign") {
			hasSigner = true
		}
	}
	slices.Sort(downloads)
	wantDownloads := []string{metadataArtifactName, tarballArtifactName}
	if !slices.Equal(downloads, wantDownloads) {
		return fmt.Errorf("provenance-sign handoff downloads are %q, want %q", downloads, wantDownloads)
	}
	if !hasTrustedCheckout {
		return fmt.Errorf("provenance-sign must checkout the trusted builder at job.workflow_sha without credentials")
	}
	if !hasSigner {
		return fmt.Errorf("provenance-sign must invoke npm-profile-sign")
	}
	if result.BundleArtifactName != provenanceBundleArtifactName {
		return fmt.Errorf("provenance bundle artifact name is %q, want %q", result.BundleArtifactName, provenanceBundleArtifactName)
	}
	return nil
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
