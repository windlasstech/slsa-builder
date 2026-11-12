package npmprofile

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

func TestVerifyProducerPackageRejections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*BuildMetadata)
		wantID string
	}{
		{name: "manifest descriptor digest disagrees with evidence", wantID: IDPackageManifestEvidence, mutate: func(metadata *BuildMetadata) {
			for index := range metadata.ResolvedDependencies {
				if metadata.ResolvedDependencies[index].Name == "package-manifest" {
					metadata.ResolvedDependencies[index].Digest["sha256"] = digest.SumSHA256([]byte("other")).String()
				}
			}
		}},
		{name: "evidence bytes are not a manifest", wantID: IDPackageManifestEvidence, mutate: func(metadata *BuildMetadata) {
			metadata.ManifestEvidence.ContentBase64 = base64.StdEncoding.EncodeToString([]byte(`[1]`))
		}},
		{name: "manifest without identity", wantID: IDPackageMetadataRequired, mutate: func(metadata *BuildMetadata) {
			replaceEvidenceManifest(t, metadata, `{"version":"1.0.0","repository":"windlasstech/slsa-builder","scripts":{"build":"true"}}`)
		}},
		{name: "private manifest is not publishable", wantID: IDPackagePrivate, mutate: func(metadata *BuildMetadata) {
			replaceEvidenceManifest(t, metadata, `{"name":"@windlass/slsa-builder","version":"1.2.3","private":true,"repository":"windlasstech/slsa-builder","scripts":{"build":"true"}}`)
		}},
		{name: "manifest repository differs from caller", wantID: IDPackageRepositoryIdentityMismatch, mutate: func(metadata *BuildMetadata) {
			replaceEvidenceManifest(t, metadata, `{"name":"@windlass/slsa-builder","version":"1.2.3","repository":"other/project","scripts":{"build":"true"}}`)
		}},
		{name: "disposition disagrees with evidence", wantID: IDBuildScriptDisposition, mutate: func(metadata *BuildMetadata) {
			metadata.BuildScriptDisposition = BuildScriptSkippedAbsent
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			metadata := validProvenanceInput(t, ManagerNPM).BuildMetadata
			test.mutate(&metadata)
			_, err := VerifyProducerPackage(metadata)
			requireNPMDiagnostic(t, err, test.wantID)
		})
	}
}

// replaceEvidenceManifest swaps the offline evidence bytes and rebinds the
// signed descriptor digest so the mutated manifest reaches the semantic check.
func replaceEvidenceManifest(t *testing.T, metadata *BuildMetadata, manifest string) {
	t.Helper()
	metadata.ManifestEvidence.ContentBase64 = base64.StdEncoding.EncodeToString([]byte(manifest))
	sum := digest.SumSHA256([]byte(manifest)).String()
	for index := range metadata.ResolvedDependencies {
		if metadata.ResolvedDependencies[index].Name == "package-manifest" {
			metadata.ResolvedDependencies[index].Digest["sha256"] = sum
		}
	}
}

func TestVerifyPackedArtifactRejections(t *testing.T) {
	t.Parallel()
	t.Run("digest mismatch", func(t *testing.T) {
		t.Parallel()
		fixture := filepath.Join(testRepositoryRoot(t), "testdata", "npm", "buildpack", "npm-root-valid-1.0.0.tgz")
		_, err := VerifyPackedArtifact(fixture, PrimaryArtifact{
			ArtifactName: "artifact", PayloadFileName: "npm-root-valid-1.0.0.tgz",
			SHA256: digest.SumSHA256([]byte("other")).String(), SHA512: digest.SumSHA512([]byte("other")).String(),
		})
		requireNPMDiagnostic(t, err, "windlass.verify.error.digest-mismatch")
	})
	t.Run("truncated tarball", func(t *testing.T) {
		t.Parallel()
		fixture := filepath.Join(testRepositoryRoot(t), "testdata", "npm", "buildpack", "npm-root-valid-1.0.0.tgz")
		encoded, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		truncated := filepath.Join(t.TempDir(), "truncated.tgz")
		if err := os.WriteFile(truncated, encoded[:100], 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = VerifyPackedArtifact(truncated, PrimaryArtifact{})
		if err == nil || !strings.HasPrefix(err.Error(), IDPackagePackFailed+": ") {
			t.Fatalf("VerifyPackedArtifact() error = %v, want %s prefix", err, IDPackagePackFailed)
		}
	})
}

// mutateSourceSelector re-encodes external parameters with an explicit caller
// selector pinning the built tag while the invocation ref points elsewhere.
func mutateSourceSelector(t *testing.T, encoded json.RawMessage, sourceRef, invocationRef string) json.RawMessage {
	t.Helper()
	parameters, err := DecodeExternalParameters(encoded)
	if err != nil {
		t.Fatal(err)
	}
	parameters.Source.Ref = sourceRef
	parameters.Workflow.Ref = invocationRef
	mutated, err := EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatal(err)
	}
	return mutated
}

func TestProducerBindingRejections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*NPMProvenanceInput)
		wantID string
	}{
		{name: "packed identity disagrees with source manifest", wantID: IDPackedPackageMetadataMismatch, mutate: func(input *NPMProvenanceInput) {
			input.Packed.Name = "@windlass/other"
		}},
		{name: "unsupported release event", wantID: IDSourceRefInvalid, mutate: func(input *NPMProvenanceInput) {
			input.GitHub.EventName = "pull_request"
		}},
		{name: "push invocation off the built tag", wantID: IDSourceRefInvalid, mutate: func(input *NPMProvenanceInput) {
			input.GitHub.EventName = "push"
			input.BuildMetadata.ExternalParameters = mutateSourceSelector(t, input.BuildMetadata.ExternalParameters, "refs/tags/v1.2.3", "refs/heads/main")
			for index := range input.BuildMetadata.ResolvedDependencies {
				dependency := &input.BuildMetadata.ResolvedDependencies[index]
				if dependency.Name == "invocation-source" {
					dependency.URI = strings.Replace(dependency.URI, "@refs/tags/v1.2.3", "@refs/heads/main", 1)
				}
			}
		}},
		{name: "node version off the pin", wantID: "windlass.verify.error.builder-version-mismatch", mutate: func(input *NPMProvenanceInput) {
			input.NodeJSVersion = "v24.0.0"
		}},
		{name: "built commit off the descriptor", wantID: IDBuiltSource, mutate: func(input *NPMProvenanceInput) {
			input.PeeledReleaseRevision = testAttestSHA
		}},
		{name: "invocation commit off the descriptor", wantID: IDInvocationSource, mutate: func(input *NPMProvenanceInput) {
			input.InvocationRevision = testAttestSHA
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := validProvenanceInput(t, ManagerNPM)
			test.mutate(&input)
			_, err := NewProvenanceSigningInput(input)
			requireNPMDiagnostic(t, err, test.wantID)
		})
	}
}
