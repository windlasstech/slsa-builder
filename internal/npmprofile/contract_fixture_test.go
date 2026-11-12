package npmprofile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/fixture"
)

func TestFinalContractCorpus(t *testing.T) {
	if os.Getenv("UPDATE_CONTRACT_FIXTURES") != "1" {
		t.Parallel()
	}
	root := filepath.Join("..", "..")
	if os.Getenv("UPDATE_CONTRACT_FIXTURES") == "1" {
		writeContractFixtures(t, root)
	}
	index, err := fixture.Load(filepath.Join(root, "testdata", "fixtures", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]fixture.Manifest, len(index.Fixtures))
	for _, manifest := range index.Fixtures {
		byName[manifest.Name] = manifest
	}
	for _, spec := range finalContractFixtures {
		t.Run(spec.name, func(t *testing.T) {
			manifest, present := byName[spec.name]
			if !present {
				t.Fatalf("contract fixture %q is not registered", spec.name)
			}
			if manifest.CoveredRequirement != spec.requirement || manifest.Surface != "npm" {
				t.Fatalf("wrong fixture mapping: %#v", manifest)
			}
			if (manifest.Type == "accepted") != (spec.category == "") {
				t.Fatalf("wrong fixture result: %#v", manifest)
			}
			if spec.category != "" && (manifest.ExpectedPrimaryID == nil || *manifest.ExpectedPrimaryID != "windlass.verify.error."+spec.category) {
				t.Fatalf("wrong fixture diagnostic: %#v", manifest)
			}
			raw, err := os.ReadFile(filepath.Join(root, manifest.Provenance))
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := canonicaljson.Canonicalize(raw)
			if err != nil || !bytes.Equal(raw, canonical) {
				t.Fatalf("non-JCS contract vector: %v", err)
			}
			var variants []contractFixtureVariant
			if spec.requirement == "ADR-0109.package-manifest-evidence" {
				variant := contractFixtureVariant{Name: spec.name, Manager: spec.manager, Statement: raw}
				if manifest.ManifestEvidence != nil {
					evidence, err := os.ReadFile(filepath.Join(root, *manifest.ManifestEvidence))
					if err != nil {
						t.Fatal(err)
					}
					variant.EvidenceJSON = string(evidence)
				}
				variants = []contractFixtureVariant{variant}
			} else {
				if manifest.ManifestEvidence != nil {
					t.Fatal("static vector must have null index manifest-evidence")
				}
				var document contractFixtureDocument
				decoder := json.NewDecoder(bytes.NewReader(raw))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&document); err != nil {
					t.Fatal(err)
				}
				if document.Phase != "provenance-inputs" {
					t.Fatalf("phase = %q", document.Phase)
				}
				variants = document.Cases
			}
			wantCount := 1
			if spec.mutation == "matrix" || spec.mutation == "diagnostics" {
				wantCount = 3
			}
			if spec.mutation == "removed" {
				wantCount = len(contractRemovedFields())
			}
			if len(variants) != wantCount {
				t.Fatalf("variant count %d, want %d", len(variants), wantCount)
			}
			for _, variant := range variants {
				t.Run(variant.Name, func(t *testing.T) {
					baseline := contractFixtureInput(t, variant.Manager, spec.scenario)
					packed, err := VerifyPackedArtifact(filepath.Join(root, manifest.Artifact), baseline.BuildMetadata.PrimaryArtifact)
					if err != nil {
						t.Fatalf("contract artifact binding: %v", err)
					}
					if packed.Name != baseline.Packed.Name || packed.Version != baseline.Packed.Version {
						t.Fatal("contract tarball identity differs from original manifest")
					}
					err = runContractVariant(t, spec, variant)
					if spec.category == "" {
						if err != nil {
							t.Fatal(err)
						}
					} else {
						requireNPMDiagnostic(t, err, "windlass.verify.error."+spec.category)
					}
				})
			}
		})
	}
}

func runContractVariant(t *testing.T, spec contractFixtureSpec, variant contractFixtureVariant) error {
	t.Helper()
	input := contractFixtureInput(t, variant.Manager, spec.scenario)
	var evidence ManifestEvidence
	if variant.EvidenceJSON != "" {
		if err := json.Unmarshal([]byte(variant.EvidenceJSON), &evidence); err != nil {
			return err
		}
	}
	input.BuildMetadata.ManifestEvidence = evidence
	var capturedRunner *RunnerCapture
	if variant.RunnerMetadataJSON != "" {
		imageOS := ""
		if variant.ImageOS != nil {
			imageOS = *variant.ImageOS
		}
		capture, captureErr := parseRunnerMetadata([]byte(variant.RunnerMetadataJSON), imageOS, variant.DeploymentVersion)
		if spec.category == "input-unavailable" || captureErr != nil {
			return captureErr
		}
		capturedRunner = &capture
	}
	statement := decodeContractStatement(t, variant.Statement)
	if capturedRunner != nil {
		produced := runnerImageDescriptor(*capturedRunner)
		if err := validateRunnerDescriptor(produced); err != nil {
			return err
		}
		found := false
		for _, dependency := range statement.Predicate.BuildDefinition.ResolvedDependencies {
			if dependency.Name != "runner-image" {
				continue
			}
			found = true
			equal, err := equalJSONValues(dependency, produced)
			if err != nil || !equal {
				return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image", "runner metadata capture must reproduce the signed runner-image descriptor")
			}
		}
		if !found {
			return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image", "runner-image descriptor is required")
		}
	}
	if spec.producer {
		input.BuildMetadata.ResolvedDependencies = statement.Predicate.BuildDefinition.ResolvedDependencies
		_, err := NewProvenanceSigningInput(input)
		if err != nil {
			return err
		}
	}
	if err := ValidateNPMStatement(statement, input); err != nil {
		return err
	}
	if spec.category == "" {
		want := []string{"invocation-source", "built-source", "package-manifest", "lockfile", "node-runtime"}
		if variant.Manager != ManagerNPM {
			want = append(want, "package-manager-distribution")
		}
		want = append(want, "runner-image")
		if len(statement.Predicate.BuildDefinition.ResolvedDependencies) != len(want) {
			t.Fatal("wrong complete descriptor count")
		}
		for index, name := range want {
			if statement.Predicate.BuildDefinition.ResolvedDependencies[index].Name != name {
				t.Fatalf("descriptor %d must be %s", index, name)
			}
		}
	}
	if spec.scenario == "publish" {
		verified, err := VerifyProducerPackage(input.BuildMetadata)
		if err != nil {
			return err
		}
		if verified.Publish.ResolvedRegistryURL != "https://registry.npmjs.org/" || verified.Publish.ResolvedDistTag != "next" || verified.Publish.PublishAccessOption == nil || *verified.Publish.PublishAccessOption != "public" {
			t.Fatal("publish settings did not recompute from original inputs")
		}
	}
	if spec.mutation == "diagnostics" {
		var metadata diagnostic.DiagnosticMetadata
		if err := json.Unmarshal([]byte(variant.DiagnosticJSON), &metadata); err != nil {
			return err
		}
		if metadata.Publish == nil {
			t.Fatal("missing preflight diagnostics")
		}
		if strings.Contains(string(statement.Predicate.BuildDefinition.ExternalParameters), "preexisting") {
			t.Fatal("preflight diagnostics entered signed parameters")
		}
		_, err := diagnostic.Build(nil, nil, &metadata)
		return err
	}
	return nil
}
