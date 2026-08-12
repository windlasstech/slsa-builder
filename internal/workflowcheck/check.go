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

// PublishJobResult is the machine-readable P05 npm mutation-job conformance result.
type PublishJobResult struct {
	Result             string `json:"result"`
	OIDCPermission     bool   `json:"oidc_permission"`
	MutationPermission bool   `json:"mutation_permission"`
	MutationKey        string `json:"mutation_key"`
	CancelInProgress   bool   `json:"cancel_in_progress"`
	Queue              string `json:"queue"`
	AlwaysRunReport    bool   `json:"always_run_report"`
}

// NPMOnlyProfileResult is the complete machine-readable P05 workflow conformance result.
type NPMOnlyProfileResult struct {
	Result           string                       `json:"result"`
	Profile          string                       `json:"profile"`
	Graph            []string                     `json:"graph"`
	MutationKey      string                       `json:"mutation_key"`
	CancelInProgress bool                         `json:"cancel_in_progress"`
	Queue            string                       `json:"queue"`
	Permissions      map[string]map[string]string `json:"permissions"`
	AlwaysRunReport  bool                         `json:"always_run_report"`
}

type workflowDocument struct {
	Permissions map[string]string      `yaml:"permissions"`
	On          workflowTriggers       `yaml:"on"`
	Jobs        map[string]workflowJob `yaml:"jobs"`
}

type workflowTriggers struct {
	WorkflowCall workflowCall `yaml:"workflow_call"`
}

type workflowCall struct {
	Inputs  map[string]workflowInput  `yaml:"inputs"`
	Outputs map[string]workflowOutput `yaml:"outputs"`
}

type workflowInput struct {
	Required bool   `yaml:"required"`
	Type     string `yaml:"type"`
	Default  any    `yaml:"default"`
}

type workflowOutput struct {
	Value string `yaml:"value"`
}

type workflowJob struct {
	If          string            `yaml:"if"`
	Needs       any               `yaml:"needs"`
	RunsOn      string            `yaml:"runs-on"`
	Permissions map[string]string `yaml:"permissions"`
	Concurrency concurrency       `yaml:"concurrency"`
	Outputs     map[string]string `yaml:"outputs"`
	Steps       []workflowStep    `yaml:"steps"`
}

type concurrency struct {
	Group            string `yaml:"group"`
	CancelInProgress bool   `yaml:"cancel-in-progress"`
	Queue            string `yaml:"queue"`
}

type workflowStep struct {
	ID   string         `yaml:"id"`
	If   string         `yaml:"if"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	Env  map[string]any `yaml:"env"`
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
	if build.Concurrency.Group != "npm-build-${{ github.repository }}-${{ github.ref_name }}" || !build.Concurrency.CancelInProgress || build.Concurrency.Queue != "" {
		return BuildJobResult{}, fmt.Errorf("build job must use the pre-mutation concurrency contract")
	}
	_, requiresSourceRef := document.On.WorkflowCall.Inputs["source-ref"]
	if err := checkSteps(build.Steps, &result, requiresSourceRef); err != nil {
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

// CheckPublishJob verifies the isolated npm mutation authority, concurrency, handoffs, and report.
func CheckPublishJob(path string) (PublishJobResult, error) {
	document, err := readWorkflow(path)
	if err != nil {
		return PublishJobResult{}, err
	}
	if document.Permissions == nil || len(document.Permissions) != 0 {
		return PublishJobResult{}, fmt.Errorf("top-level permissions must be an explicit empty mapping")
	}
	job, ok := document.Jobs["publish"]
	if !ok {
		return PublishJobResult{}, fmt.Errorf("workflow has no publish job")
	}
	result := PublishJobResult{
		OIDCPermission:     job.Permissions["id-token"] == "write",
		MutationPermission: hasWritePermission(job.Permissions, "contents", "packages", "attestations", "artifact-metadata"),
		MutationKey:        job.Concurrency.Group,
		CancelInProgress:   job.Concurrency.CancelInProgress,
		Queue:              job.Concurrency.Queue,
	}
	if len(job.Permissions) != 2 || job.Permissions["contents"] != "read" || !result.OIDCPermission {
		return PublishJobResult{}, fmt.Errorf("publish permissions must be exactly contents: read and id-token: write")
	}
	if result.MutationPermission {
		return PublishJobResult{}, fmt.Errorf("publish must not have GitHub storage or repository mutation permission")
	}
	if job.RunsOn != "ubuntu-24.04" {
		return PublishJobResult{}, fmt.Errorf("publish job runs-on must be ubuntu-24.04")
	}
	if !slices.Equal(sortedNeeds(job.Needs), []string{"build", "provenance-sign"}) {
		return PublishJobResult{}, fmt.Errorf("publish must depend exactly on build and provenance-sign")
	}
	if normalizeExpression(job.If) != "always()" {
		return PublishJobResult{}, fmt.Errorf("publish job must run always so it can emit the outcome report")
	}
	if result.MutationKey != "release-mutation-${{ github.repository }}-${{ github.ref_name }}" || result.CancelInProgress || result.Queue != "max" {
		return PublishJobResult{}, fmt.Errorf("publish job must use the exact queued mutation concurrency contract")
	}
	if err := checkPublishSteps(job.Steps, &result); err != nil {
		return PublishJobResult{}, err
	}
	result.Result = "pass"
	return result, nil
}

// CheckNPMOnlyProfile verifies the complete public npm-only reusable workflow contract.
func CheckNPMOnlyProfile(path string) (NPMOnlyProfileResult, error) {
	document, err := readWorkflow(path)
	if err != nil {
		return NPMOnlyProfileResult{}, err
	}
	if err := checkNPMWorkflowCall(document.On.WorkflowCall); err != nil {
		return NPMOnlyProfileResult{}, err
	}
	graph := make([]string, 0, len(document.Jobs))
	for name := range document.Jobs {
		graph = append(graph, name)
	}
	slices.Sort(graph)
	wantGraph := []string{"build", "provenance-sign", "publish"}
	if !slices.Equal(graph, wantGraph) {
		return NPMOnlyProfileResult{}, fmt.Errorf("npm-only graph is %q, want %q", graph, wantGraph)
	}
	if _, err := CheckBuildJob(path); err != nil {
		return NPMOnlyProfileResult{}, fmt.Errorf("build contract: %w", err)
	}
	if _, err := CheckProvenanceSignJob(path); err != nil {
		return NPMOnlyProfileResult{}, fmt.Errorf("provenance-sign contract: %w", err)
	}
	signing := document.Jobs["provenance-sign"]
	if signing.Concurrency.Group != "npm-provenance-sign-${{ github.repository }}-${{ github.ref_name }}" || !signing.Concurrency.CancelInProgress || signing.Concurrency.Queue != "" {
		return NPMOnlyProfileResult{}, fmt.Errorf("provenance-sign must use the pre-mutation concurrency contract")
	}
	publish, err := CheckPublishJob(path)
	if err != nil {
		return NPMOnlyProfileResult{}, fmt.Errorf("publish contract: %w", err)
	}
	return NPMOnlyProfileResult{
		Result: "pass", Profile: "npm-only", Graph: wantGraph,
		MutationKey: publish.MutationKey, CancelInProgress: publish.CancelInProgress,
		Queue: publish.Queue, AlwaysRunReport: publish.AlwaysRunReport,
		Permissions: map[string]map[string]string{
			"build":           document.Jobs["build"].Permissions,
			"provenance-sign": document.Jobs["provenance-sign"].Permissions,
			"publish":         document.Jobs["publish"].Permissions,
		},
	}, nil
}

func readWorkflow(path string) (workflowDocument, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return workflowDocument{}, fmt.Errorf("read workflow: %w", err)
	}
	var document workflowDocument
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return workflowDocument{}, fmt.Errorf("decode workflow: %w", err)
	}
	return document, nil
}

func checkNPMWorkflowCall(call workflowCall) error {
	wantInputs := map[string]struct {
		typeName string
		required bool
		boolean  bool
	}{
		"package-directory": {typeName: "string", required: true},
		"registry-url":      {typeName: "string"}, "dist-tag": {typeName: "string"}, "access": {typeName: "string"},
		"source-ref":         {typeName: "string"},
		"release-asset-mode": {typeName: "boolean", boolean: true}, "release-tag": {typeName: "string"},
		"provenance-sidecar": {typeName: "string"}, "linked-artifact-metadata": {typeName: "boolean", boolean: true},
	}
	if len(call.Inputs) != len(wantInputs) {
		return fmt.Errorf("workflow_call inputs must contain exactly the nine public inputs")
	}
	for name, want := range wantInputs {
		input, ok := call.Inputs[name]
		if !ok || input.Type != want.typeName || input.Required != want.required {
			return fmt.Errorf("workflow_call input %q has the wrong type or required state", name)
		}
		if want.boolean {
			if input.Default != false {
				return fmt.Errorf("boolean input %q must default to false", name)
			}
		} else if input.Default != nil {
			return fmt.Errorf("string input %q must not define a workflow_call default", name)
		}
	}
	wantOutputs := []string{"package-name", "package-registry-url", "package-tarball-name", "package-tarball-sha256", "package-tarball-sha512", "package-url", "package-version"}
	gotOutputs := make([]string, 0, len(call.Outputs))
	for name, output := range call.Outputs {
		gotOutputs = append(gotOutputs, name)
		if output.Value != "${{ jobs.publish.outputs."+name+" }}" {
			return fmt.Errorf("workflow_call output %q must map to the publish job", name)
		}
	}
	slices.Sort(gotOutputs)
	if !slices.Equal(gotOutputs, wantOutputs) {
		return fmt.Errorf("npm-only public outputs are %q, want %q", gotOutputs, wantOutputs)
	}
	return nil
}

func checkPublishSteps(steps []workflowStep, result *PublishJobResult) error {
	if len(steps) == 0 || !strings.HasPrefix(steps[0].Uses, "step-security/harden-runner@") || scalar(steps[0].With["egress-policy"]) != "audit" {
		return fmt.Errorf("harden-runner with audit egress must be the first publish step")
	}
	downloads := make([]string, 0, 2)
	hasPublish := false
	hasReport := false
	hasReportUpload := false
	for _, current := range steps {
		if current.Uses != "" && !pinnedActionPattern.MatchString(current.Uses) {
			return fmt.Errorf("action reference %q is not pinned to a full SHA", current.Uses)
		}
		if strings.HasPrefix(current.Uses, "actions/attest@") {
			return fmt.Errorf("actions/attest is forbidden in publish")
		}
		if strings.HasPrefix(current.Uses, "actions/download-artifact@") {
			downloads = append(downloads, scalar(current.With["name"]))
		}
		if strings.Contains(current.Run, "npm-profile-publish") {
			hasPublish = true
		}
		if strings.Contains(current.Run, "npm-profile-report") && normalizeExpression(current.If) == "always()" {
			hasReport = true
		}
		if strings.HasPrefix(current.Uses, "actions/upload-artifact@") && normalizeExpression(current.If) == "always()" &&
			scalar(current.With["name"]) == "js-ts-npm-outcome-report-${{ github.run_id }}-${{ github.run_attempt }}" {
			hasReportUpload = true
		}
	}
	slices.Sort(downloads)
	if !slices.Equal(downloads, []string{tarballArtifactName, provenanceBundleArtifactName}) {
		return fmt.Errorf("publish handoff downloads are %q", downloads)
	}
	if !hasPublish {
		return fmt.Errorf("publish job must invoke npm-profile-publish")
	}
	result.AlwaysRunReport = hasReport && hasReportUpload
	if !result.AlwaysRunReport {
		return fmt.Errorf("publish job must always generate and upload the outcome report")
	}
	return nil
}

func sortedNeeds(value any) []string {
	var values []string
	switch typed := value.(type) {
	case string:
		values = []string{typed}
	case []any:
		for _, item := range typed {
			values = append(values, scalar(item))
		}
	case []string:
		values = append(values, typed...)
	}
	slices.Sort(values)
	return values
}

func normalizeExpression(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "${{")
	value = strings.TrimSuffix(value, "}}")
	return strings.TrimSpace(value)
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

func checkSteps(steps []workflowStep, result *BuildJobResult, requiresSourceRef bool) error {
	if len(steps) == 0 || !strings.HasPrefix(steps[0].Uses, "step-security/harden-runner@") {
		return fmt.Errorf("harden-runner must be the first build step")
	}
	if scalar(steps[0].With["egress-policy"]) != "audit" {
		return fmt.Errorf("harden-runner must use egress-policy audit")
	}
	hasCheckout := false
	hasSourceResolution := false
	hasNode24 := false
	hasCorepack := false
	hasBuild := false
	for _, current := range steps {
		if current.Uses != "" && !pinnedActionPattern.MatchString(current.Uses) {
			return fmt.Errorf("action reference %q is not pinned to a full SHA", current.Uses)
		}
		switch {
		case strings.HasPrefix(current.Uses, "actions/checkout@"):
			if requiresSourceRef && scalar(current.With["path"]) == "source" {
				hasCheckout = scalar(current.With["persist-credentials"]) == "false" && scalar(current.With["ref"]) == "${{ steps.source.outputs.revision }}"
			} else if !requiresSourceRef {
				hasCheckout = scalar(current.With["persist-credentials"]) == "false"
			}
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
		if current.ID == "source" && strings.Contains(current.Run, "npm-profile-source") &&
			scalar(current.Env["SOURCE_REF"]) == "${{ inputs.source-ref }}" && !strings.Contains(current.Run, "${{ inputs.source-ref }}") {
			hasSourceResolution = true
		}
	}
	if requiresSourceRef && !hasSourceResolution {
		return fmt.Errorf("build job must resolve source-ref through the trusted source command")
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
