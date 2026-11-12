package npmprofile

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

type contractFixtureDocument struct {
	Phase string                   `json:"phase"`
	Cases []contractFixtureVariant `json:"cases"`
}

type contractFixtureVariant struct {
	Name               string          `json:"name"`
	Manager            Manager         `json:"package_manager"`
	Statement          json.RawMessage `json:"statement"`
	EvidenceJSON       string          `json:"manifest_evidence_json"`
	DiagnosticJSON     string          `json:"diagnostic_metadata_json,omitempty"`
	RunnerMetadataJSON string          `json:"runner_metadata_json,omitempty"`
	ImageOS            *string         `json:"image_os,omitempty"`
	DeploymentVersion  string          `json:"deployment_version,omitempty"`
}

func contractFixtureInput(t *testing.T, manager Manager, scenario string) NPMProvenanceInput {
	t.Helper()
	if scenario == "retry" {
		return bindContractArtifact(t, sourceRefDispatchRetryProvenanceInput(t), scenario)
	}
	input := validProvenanceInput(t, manager)
	if scenario == "tag" {
		return bindContractArtifact(t, input, scenario)
	}
	parameters, err := DecodeExternalParameters(input.BuildMetadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := input.BuildMetadata.ManifestEvidence.Decode(parameters.Package.Directory)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(snapshot.Content, &manifest); err != nil {
		t.Fatal(err)
	}
	switch scenario {
	case "skipped":
		delete(manifest, "scripts")
		input.BuildMetadata.BuildScriptDisposition = BuildScriptSkippedAbsent
	case "publish":
		manifest["publishConfig"] = json.RawMessage(`{"registry":"https://registry.npmjs.org/","tag":"next","access":"public"}`)
		parameters.Publish = PublishParameters{InputRegistryURL: testStringPointer(" HTTPS://REGISTRY.NPMJS.ORG:443 "), InputDistTag: testStringPointer(" next "), InputAccess: testStringPointer(" public ")}
	case "workspace":
		parameters.Package.Directory = "packages/widget"
		snapshot.Path = "packages/widget/package.json"
	default:
		t.Fatalf("unknown contract scenario %q", scenario)
	}
	snapshot.Content = mustJSON(manifest)
	input.BuildMetadata.ManifestEvidence = ManifestEvidence{Path: snapshot.Path, ContentBase64: base64.StdEncoding.EncodeToString(snapshot.Content)}
	input.BuildMetadata.ExternalParameters = mustJSON(parameters)
	for index := range input.BuildMetadata.ResolvedDependencies {
		d := &input.BuildMetadata.ResolvedDependencies[index]
		if d.Name != "package-manifest" {
			continue
		}
		d.URI = "git+" + parameters.Workflow.Repository + "@" + testSourceSHA + "#" + escapeLockfileFragment(snapshot.Path)
		d.Digest["sha256"] = digest.SumSHA256(snapshot.Content).String()
		if scenario == "workspace" {
			d.Annotations["workspace_root"] = mustJSON(".")
		}
	}
	return bindContractArtifact(t, input, scenario)
}

func bindContractArtifact(t *testing.T, input NPMProvenanceInput, scenario string) NPMProvenanceInput {
	t.Helper()
	parameters, err := DecodeExternalParameters(input.BuildMetadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := input.BuildMetadata.ManifestEvidence.Decode(parameters.Package.Directory)
	if err != nil {
		t.Fatal(err)
	}
	encoded := contractArtifactBytes(t, snapshot.Content)
	input.BuildMetadata.PrimaryArtifact.PayloadFileName = "contract-artifact-" + scenario + ".tgz"
	input.BuildMetadata.PrimaryArtifact.SHA256 = digest.SumSHA256(encoded).String()
	input.BuildMetadata.PrimaryArtifact.SHA512 = digest.SumSHA512(encoded).String()
	return input
}

func contractArtifactBytes(t *testing.T, manifest []byte) []byte {
	t.Helper()
	return inspectArchiveBytes(t, []inspectArchiveEntry{{name: "package/package.json", body: string(manifest), kind: '0'}})
}

func contractVariant(t *testing.T, manager Manager, scenario string) contractFixtureVariant {
	t.Helper()
	input := contractFixtureInput(t, manager, scenario)
	signing, err := NewProvenanceSigningInput(input)
	if err != nil {
		t.Fatal(err)
	}
	return contractFixtureVariant{Name: string(manager), Manager: manager, Statement: signing.StatementJSON, EvidenceJSON: string(mustJSON(input.BuildMetadata.ManifestEvidence))}
}

func contractRemovedFields() []string {
	return []string{
		"runtime", "package_manager", "release", "caller", "build",
		"source.repository", "source.revision", "source.ref_type", "source.input_ref", "source.invocation_ref", "source.invocation_revision", "source.event_name",
		"workflow.sha", "workflow.builder_id",
		"package.name", "package.version", "package.private", "package.repository", "package.workspace_root", "package.source_manifest", "package.tarball_name", "package.package_url", "package.packed_name", "package.packed_version", "package.packed_files", "package.consumer_surface", "package.publish_config_raw",
		"publish.publish_config", "publish.resolved_registry_url", "publish.resolved_dist_tag", "publish.publish_access_option", "publish.effective_access", "publish.trusted_publishing", "publish.provenance_file", "publish.custom_registry_support", "publish.package_identity_preexisting", "publish.package_version_preexisting",
		"distribution.release_tag_supplied", "distribution.provenance_sidecar",
	}
}
