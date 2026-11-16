package npmprofile

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	publishCLIWantSigner     = `{"name":"signing-adapter","uri":"pkg:golang/github.com/sigstore/sigstore-go@v1.4.0","digest":{"dirHash1":"0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac"}}`
	publishCLIWantDescriptor = `{"name":"publish-cli","uri":"pkg:npm/npm@11.21.1","digest":{"sha512":"613ccbe90044257c87713bb426251ced128ca7e9b3949b65eeb18b98445a5d441e14720c73d6ad76be86f4cf42f2f0d66efe347a9a13400ccd2f6fcfb5954385"},"mediaType":"application/gzip","annotations":{"digest_authority":"builder-pinned-record"}}`
	publishCLIMismatch       = "windlass.verify.error.builder-dependencies-publish-cli-mismatch"
	commonBuilderMismatch    = "windlass.verify.error.builder-dependencies-descriptor-invalid"
)

func TestBuilderFields(t *testing.T) {
	t.Parallel()
	for _, manager := range []Manager{ManagerNPM, ManagerPNPM, ManagerYarn} {
		t.Run(string(manager), func(t *testing.T) {
			t.Parallel()
			// Given build-stage versions distinct from the publish CLI pin.
			input := validProvenanceInput(t, manager)
			// When producing the exact signing payload.
			signing, err := NewProvenanceSigningInput(input)
			if err != nil {
				t.Fatal(err)
			}
			// Then build-stage versions and dependency cardinality remain unchanged.
			version := signing.Predicate.RunDetails.Builder.Version
			wantCount, wantVersions := 7, 3
			if manager == ManagerNPM {
				wantCount, wantVersions = 6, 2
			}
			if len(version) != wantVersions || version["nodejs"] != input.NodeJSVersion || version["npm"] != input.NPMVersion || version["npm"] == "11.21.1" {
				t.Errorf("builder.version = %#v, want build-stage versions", version)
			}
			if input.CorepackVersion != nil && version["corepack"] != *input.CorepackVersion {
				t.Errorf("corepack = %q, want %q", version["corepack"], *input.CorepackVersion)
			}
			resolved := signing.Predicate.BuildDefinition.ResolvedDependencies
			if len(resolved) != wantCount {
				t.Errorf("resolvedDependencies count = %d, want %d", len(resolved), wantCount)
			}
			for _, dependency := range resolved {
				if dependency.Name == "npm-cli" || dependency.Name == "publish-cli" {
					t.Errorf("publish dependency leaked into resolvedDependencies: %q", dependency.Name)
				}
			}
			requirePublishBuilderJSON(t, signing.PredicateJSON)
		})
	}
}

func requirePublishBuilderJSON(t *testing.T, raw []byte) {
	t.Helper()
	var predicate struct {
		RunDetails struct {
			Builder struct {
				Dependencies json.RawMessage `json:"builderDependencies"`
			} `json:"builder"`
		} `json:"runDetails"`
	}
	if err := json.Unmarshal(raw, &predicate); err != nil {
		t.Fatal(err)
	}
	want := []byte("[" + publishCLIWantSigner + "," + publishCLIWantDescriptor + "]")
	got := predicate.RunDetails.Builder.Dependencies
	equal, err := canonicaljson.Equal(got, want)
	if err != nil || !equal {
		t.Fatalf("builderDependencies raw JSON = %s, want %s; error = %v", got, want, err)
	}
}

// Replace raw JSON, not typed fields: malformed types and ignored annotations must
// reach the production decoder unchanged, including duplicate object members.
func publishCLIStatementBytes(t *testing.T, input NPMProvenanceInput, descriptors string) []byte {
	t.Helper()
	statement := runtimeTestStatement(t, input)
	raw := mustJSON(statement)
	original := mustJSON(statement.Predicate.RunDetails.Builder.BuilderDependencies)
	if bytes.Count(raw, original) != 1 {
		t.Fatal("expected one builder dependency array")
	}
	return bytes.Replace(raw, original, []byte(descriptors), 1)
}

func validatePublishCLIBytes(t *testing.T, input NPMProvenanceInput, raw []byte) error {
	t.Helper()
	statement, err := provenance.DecodeStatement(raw)
	if err != nil {
		return err
	}
	return ValidateConsumerStatement(statement, SigstorePublishVerifierConfig{
		Identity:      publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA),
		BuildMetadata: input.BuildMetadata, Packed: input.Packed,
	})
}
