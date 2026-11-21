package npmprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/fixture"
)

func writeContractFixtures(t *testing.T, root string) {
	t.Helper()
	indexPath := filepath.Join(root, "testdata", "fixtures", "index.json")
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	var index fixture.Index
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	for _, spec := range finalContractFixtures {
		variants := contractVariants(t, spec)
		var primaryID string
		for _, variant := range variants {
			err := runContractVariant(t, spec, variant)
			if spec.category == "" {
				if err != nil {
					t.Fatalf("%s/%s: %v", spec.name, variant.Name, err)
				}
				continue
			}
			requireNPMDiagnostic(t, err, "windlass.verify.error."+spec.category)
			var identified interface{ DiagnosticID() string }
			if !errors.As(err, &identified) {
				t.Fatalf("%s/%s: missing actual diagnostic: %v", spec.name, variant.Name, err)
			}
			primaryID = identified.DiagnosticID()
		}
		filename := "testdata/npm/provenance/contract-" + spec.name + ".json"
		artifact := "testdata/npm/provenance/contract-artifact-" + spec.scenario + ".tgz"
		input := contractFixtureInput(t, spec.manager, spec.scenario)
		parameters, err := DecodeExternalParameters(input.BuildMetadata.ExternalParameters)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := input.BuildMetadata.ManifestEvidence.Decode(parameters.Package.Directory)
		if err != nil {
			t.Fatal(err)
		}
		writeContractBytes(t, filepath.Join(root, artifact), contractArtifactBytes(t, snapshot.Content))
		manifest := fixture.Manifest{Name: spec.name, Type: "accepted", Surface: "npm", Artifact: artifact, Provenance: filename, ExpectedResult: "pass", ExpectedSecondaryIDs: []string{}, CoveredRequirement: spec.requirement}
		var encoded []byte
		if spec.requirement == "ADR-0109.package-manifest-evidence" {
			encoded = variants[0].Statement
			if variants[0].EvidenceJSON != "" {
				evidencePath := "testdata/npm/provenance/contract-" + spec.name + "-evidence.json"
				manifest.ManifestEvidence = &evidencePath
				writeContractBytes(t, filepath.Join(root, evidencePath), []byte(variants[0].EvidenceJSON))
			}
		} else {
			encoded = mustJSON(contractFixtureDocument{Phase: "provenance-inputs", Cases: variants})
		}
		canonical, err := canonicaljson.Canonicalize(encoded)
		if err != nil {
			t.Fatal(err)
		}
		writeContractBytes(t, filepath.Join(root, filename), canonical)
		if spec.category != "" {
			manifest.Type, manifest.ExpectedResult = "rejected", "fail"
			manifest.ExpectedFailureCategory = testStringPointer(spec.category)
			manifest.ExpectedPrimaryID = testStringPointer(primaryID)
		}
		found := false
		for position, existing := range index.Fixtures {
			if existing.Name == spec.name {
				index.Fixtures[position] = manifest
				found = true
				break
			}
		}
		if !found {
			index.Fixtures = append(index.Fixtures, manifest)
		}
	}
	if err := fixture.Validate(index); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeContractBytes(t, indexPath, append(encoded, '\n'))
}

func writeContractBytes(t *testing.T, filename string, data []byte) {
	t.Helper()
	existing, err := os.ReadFile(filename)
	if err == nil && bytes.Equal(existing, data) {
		return
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
