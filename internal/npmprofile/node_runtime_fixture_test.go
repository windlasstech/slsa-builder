package npmprofile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/fixture"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestNodeRuntimeDependencyFixtures(t *testing.T) {
	t.Parallel()

	const (
		descriptorRequirement = "ADR-0099.node-runtime-descriptor"
		rejectionRequirement  = "ADR-0099.node-runtime-rejections"
	)
	expectedNames := []string{
		"npm-resolved-dependencies-npm-valid",
		"npm-resolved-dependencies-pnpm-valid",
		"npm-resolved-dependencies-yarn-valid",
		"npm-resolved-dependencies-reordered-valid",
		"npm-resolved-dependencies-node-runtime-missing",
		"npm-resolved-dependencies-node-runtime-duplicate",
		"npm-resolved-dependencies-node-runtime-wrong-uri",
		"npm-resolved-dependencies-node-runtime-wrong-digest",
		"npm-resolved-dependencies-node-runtime-malformed-digest",
		"npm-resolved-dependencies-node-runtime-annotations",
		"npm-resolved-dependencies-unknown-entry",
		"npm-resolved-dependencies-npm-manager-distribution",
		"npm-resolved-dependencies-node-runtime-unknown-member",
	}

	// Given the complete ADR 0099 fixture set discovered through the governed index.
	root := filepath.Clean(filepath.Join("..", ".."))
	index, err := fixture.Load(filepath.Join(root, "testdata", "fixtures", "index.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	manifests := make(map[string]fixture.Manifest, len(expectedNames))
	actualNames := make([]string, 0, len(expectedNames))
	for _, manifest := range index.Fixtures {
		if manifest.CoveredRequirement != descriptorRequirement && manifest.CoveredRequirement != rejectionRequirement {
			continue
		}
		manifests[manifest.Name] = manifest
		actualNames = append(actualNames, manifest.Name)
	}
	sortedExpectedNames := slices.Clone(expectedNames)
	slices.Sort(sortedExpectedNames)
	slices.Sort(actualNames)
	if !slices.Equal(actualNames, sortedExpectedNames) {
		t.Fatalf("ADR 0099 fixture names = %v, want %v", actualNames, sortedExpectedNames)
	}

	for _, name := range expectedNames {
		manifest := manifests[name]
		t.Run(name, func(t *testing.T) {
			// Given a strictly decoded fixture containing the complete dependency array.
			data, err := os.ReadFile(filepath.Join(root, manifest.Provenance))
			if err != nil {
				t.Fatal(err)
			}
			if err := canonicaljson.Validate(data); err != nil {
				t.Fatalf("strict fixture JSON: %v", err)
			}
			var document struct {
				PackageManager       Manager         `json:"package_manager"`
				ResolvedDependencies json.RawMessage `json:"resolved_dependencies"`
			}
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&document); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			input := validProvenanceInput(t, document.PackageManager)
			// Decode descriptor bytes as received Statement data, not as closed BuildMetadata.
			var dependencies []provenance.ResourceDescriptor
			if err := json.Unmarshal(document.ResolvedDependencies, &dependencies); err != nil {
				t.Fatalf("decode descriptors: %v", err)
			}
			input.BuildMetadata.ResolvedDependencies = dependencies

			// When constructing the real signing input from the fixture dependencies.
			signing, signingErr := NewProvenanceSigningInput(input)

			// Then the fixture's governed accepted result or exact primary diagnostic is observed.
			switch manifest.Type {
			case "accepted":
				if manifest.CoveredRequirement != descriptorRequirement {
					t.Fatalf("covered requirement = %q, want %q", manifest.CoveredRequirement, descriptorRequirement)
				}
				if signingErr != nil {
					t.Fatalf("NewProvenanceSigningInput() error = %v", signingErr)
				}
				if err := ValidateNPMStatement(signing.Statement(), input); err != nil {
					t.Fatalf("ValidateNPMStatement() error = %v", err)
				}
			case "rejected":
				if manifest.CoveredRequirement != rejectionRequirement {
					t.Fatalf("covered requirement = %q, want %q", manifest.CoveredRequirement, rejectionRequirement)
				}
				if manifest.ExpectedPrimaryID == nil {
					t.Fatal("rejected fixture has no registered primary diagnostic")
				}
				requireNPMDiagnostic(t, signingErr, *manifest.ExpectedPrimaryID)
			default:
				t.Fatalf("fixture type = %q, want accepted or rejected", manifest.Type)
			}
		})
	}
}
