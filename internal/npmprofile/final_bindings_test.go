package npmprofile

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestFinalSourceAndManifestBindings(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		role   string
		id     string
		mutate func(*provenance.ResourceDescriptor)
	}{
		{"invocation uri", "invocation-source", IDInvocationSource, func(d *provenance.ResourceDescriptor) { d.URI += "#file" }},
		{"invocation digest", "invocation-source", IDInvocationSource, func(d *provenance.ResourceDescriptor) { d.Digest["gitCommit"] = testSourceSHA }},
		{"invocation annotations", "invocation-source", IDInvocationSource, func(d *provenance.ResourceDescriptor) { d.Annotations = map[string]json.RawMessage{} }},
		{"invocation media type", "invocation-source", IDInvocationSource, func(d *provenance.ResourceDescriptor) { d.MediaType = "application/gzip" }},
		{"built uri", "built-source", IDBuiltSource, func(d *provenance.ResourceDescriptor) {
			d.URI = "git+https://github.com/example/project@refs/heads/main"
		}},
		{"built digest", "built-source", IDBuiltSource, func(d *provenance.ResourceDescriptor) { d.Digest["gitCommit"] = testAttestSHA }},
		{"built annotation missing", "built-source", IDBuiltSource, func(d *provenance.ResourceDescriptor) { d.Annotations = nil }},
		{"built annotation classifier", "built-source", IDBuiltSource, func(d *provenance.ResourceDescriptor) { d.Annotations["ref_type"] = mustJSON("branch") }},
		{"built annotation extra", "built-source", IDBuiltSource, func(d *provenance.ResourceDescriptor) { d.Annotations["extra"] = mustJSON("tag") }},
		{"built media type", "built-source", IDBuiltSource, func(d *provenance.ResourceDescriptor) { d.MediaType = "application/json" }},
		{"manifest uri", "package-manifest", IDPackageManifest, func(d *provenance.ResourceDescriptor) { d.URI += "/other" }},
		{"manifest digest shape", "package-manifest", IDPackageManifest, func(d *provenance.ResourceDescriptor) { d.Digest["sha256"] = "short" }},
		{"manifest digest binding", "package-manifest", IDPackageManifestEvidence, func(d *provenance.ResourceDescriptor) { d.Digest["sha256"] = strings.Repeat("0", 64) }},
		{"manifest media type", "package-manifest", IDPackageManifest, func(d *provenance.ResourceDescriptor) { d.MediaType = "text/plain" }},
		{"manifest workspace classification", "package-manifest", IDPackageManifest, func(d *provenance.ResourceDescriptor) { d.Annotations["workspace_root"] = mustJSON(".") }},
		{"manifest extra annotation", "package-manifest", IDPackageManifest, func(d *provenance.ResourceDescriptor) { d.Annotations["extra"] = mustJSON(false) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := sourceRefDispatchRetryProvenanceInput(t)
			for index := range input.BuildMetadata.ResolvedDependencies {
				if input.BuildMetadata.ResolvedDependencies[index].Name == test.role {
					test.mutate(&input.BuildMetadata.ResolvedDependencies[index])
				}
			}
			requireNPMDiagnostic(t, newProvenanceInputError(input), test.id)
		})
	}
	for role, id := range map[string]string{"invocation-source": IDInvocationSource, "built-source": IDBuiltSource, "package-manifest": IDPackageManifest} {
		for _, duplicate := range []bool{false, true} {
			t.Run(role+"/duplicate="+map[bool]string{false: "false", true: "true"}[duplicate], func(t *testing.T) {
				t.Parallel()
				input := validProvenanceInput(t, ManagerNPM)
				values := input.BuildMetadata.ResolvedDependencies
				if duplicate {
					for _, value := range values {
						if value.Name == role {
							input.BuildMetadata.ResolvedDependencies = append(values, value)
							break
						}
					}
				} else {
					input.BuildMetadata.ResolvedDependencies = slices.DeleteFunc(values, func(value provenance.ResourceDescriptor) bool { return value.Name == role })
				}
				requireNPMDiagnostic(t, newProvenanceInputError(input), id)
			})
		}
	}
}

func TestPackageManifestEvidenceAuthentication(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*ManifestEvidence)
	}{
		{"missing", func(e *ManifestEvidence) { *e = ManifestEvidence{} }},
		{"wrong path", func(e *ManifestEvidence) { e.Path = "other/package.json" }},
		{"replaced bytes", func(e *ManifestEvidence) {
			e.ContentBase64 = base64.StdEncoding.EncodeToString([]byte(`{"name":"other","version":"1.2.3"}`))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := validProvenanceInput(t, ManagerNPM)
			test.mutate(&input.BuildMetadata.ManifestEvidence)
			requireNPMDiagnostic(t, newProvenanceInputError(input), IDPackageManifestEvidence)
		})
	}
}

func TestSourceDescriptorRejectsExplicitForbiddenNull(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"invocation-source", "built-source"} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			input := validProvenanceInput(t, ManagerNPM)
			for index, d := range input.BuildMetadata.ResolvedDependencies {
				if d.Name != role {
					continue
				}
				raw := string(mustJSON(d))
				raw = raw[:len(raw)-1] + `,"mediaType":null}`
				if err := json.Unmarshal([]byte(raw), &input.BuildMetadata.ResolvedDependencies[index]); err != nil {
					t.Fatal(err)
				}
			}
			id := IDInvocationSource
			if role == "built-source" {
				id = IDBuiltSource
			}
			requireNPMDiagnostic(t, newProvenanceInputError(input), id)
		})
	}
}
