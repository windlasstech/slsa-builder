package npmprofile

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestProvenanceExtraDecodeShape(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, group, member, value, field string }{
		{"array root", "", "", `[]`, "externalParameters"},
		{"duplicate root", "", "", `{"source":{},"source":{}}`, "externalParameters"},
		{"trailing value", "", "", `{} {}`, "externalParameters"},
		{"source scalar", "source", "", `true`, "externalParameters.source"},
		{"source null", "source", "", `null`, "externalParameters.source"},
		{"wrong scalar type", "runtime", "node_version", `24`, "externalParameters"},
		{"missing required with extra member", "workflow", "", `{"path":"x","sha":"x","injected":"x"}`, "externalParameters.workflow"},
		{"unknown optional field", "package", "injected", `true`, "externalParameters.package"},
		{"consumer array", "package", "consumer_surface", `[]`, "externalParameters.package.consumer_surface"},
		{"consumer null", "package", "consumer_surface", `null`, "externalParameters.package.consumer_surface"},
		{"consumer unknown", "package", "consumer_surface", `{"scripts":{}}`, "externalParameters.package.consumer_surface"},
		{"publish config array", "publish", "publish_config", `[]`, "externalParameters.publish.publish_config"},
		{"publish config unknown", "publish", "publish_config", `{"token":"secret"}`, "externalParameters.publish.publish_config"},
		{"publish config type", "publish", "publish_config", `{"registry":false}`, "externalParameters"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			encoded := json.RawMessage(test.value)
			if test.group != "" {
				var root map[string]json.RawMessage
				if err := json.Unmarshal(mustJSON(validExternalParameters(ManagerNPM)), &root); err != nil {
					t.Fatal(err)
				}
				if test.member != "" {
					var group map[string]json.RawMessage
					if err := json.Unmarshal(root[test.group], &group); err != nil {
						t.Fatal(err)
					}
					group[test.member] = encoded
					encoded = mustJSON(group)
				}
				root[test.group] = encoded
				encoded = mustJSON(root)
			}
			_, err := DecodeExternalParameters(encoded)
			requireProvenanceDiagnostic(t, err, [2]string{IDUnexpectedExternalParameters, test.field})
		})
	}
}

func TestProvenanceExtraSigningBindings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, id, field string
		mutate          func(*NPMProvenanceInput)
	}{
		{"bad sha256", IDMissingSubjectSHA256, "buildMetadata.primary_artifact.sha256", func(i *NPMProvenanceInput) { i.BuildMetadata.PrimaryArtifact.SHA256 = strings.Repeat("A", 64) }},
		{"bad sha512", IDMissingSubjectSHA512, "buildMetadata.primary_artifact.sha512", func(i *NPMProvenanceInput) { i.BuildMetadata.PrimaryArtifact.SHA512 = "00" }},
		{"release revision mismatch", IDReleaseRefMismatch, "externalParameters.source.revision", func(i *NPMProvenanceInput) { i.PeeledReleaseRevision = testAttestSHA }},
		{"bad finished timestamp", "windlass.verify.error.timestamp-format-invalid", "predicate.runDetails.metadata.finishedOn", func(i *NPMProvenanceInput) { i.FinishedOn = "not-a-time" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := validProvenanceInput(t, ManagerNPM)
			test.mutate(&input)
			_, err := NewProvenanceSigningInput(input)
			requireProvenanceDiagnostic(t, err, [2]string{test.id, test.field})
		})
	}
}

func TestProvenanceExtraStatementBindings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, id, field string
		mutate          func(*provenance.Statement)
	}{
		{"build type", "windlass.verify.error.wrong-build-type", "predicate.buildDefinition.buildType", func(s *provenance.Statement) { s.Predicate.BuildDefinition.BuildType += "/other" }},
		{"builder ID", "windlass.verify.error.wrong-builder-id", "predicate.runDetails.builder.id", func(s *provenance.Statement) { s.Predicate.RunDetails.Builder.ID += "0" }},
		{"invocation", "windlass.verify.error.statement-assembly-mismatch", "predicate.runDetails.metadata", func(s *provenance.Statement) { s.Predicate.RunDetails.Metadata.InvocationID += "0" }},
		{"start", "windlass.verify.error.statement-assembly-mismatch", "predicate.runDetails.metadata", func(s *provenance.Statement) { s.Predicate.RunDetails.Metadata.StartedOn = "2026-08-06T12:00:01Z" }},
		{"finish", "windlass.verify.error.statement-assembly-mismatch", "predicate.runDetails.metadata", func(s *provenance.Statement) { s.Predicate.RunDetails.Metadata.FinishedOn = "2026-08-06T12:00:04Z" }},
		{"valid external substitution", IDUnexpectedExternalParameters, "predicate.buildDefinition.externalParameters", func(s *provenance.Statement) {
			p := validExternalParameters(ManagerNPM)
			p.Publish.ResolvedDistTag = "next"
			s.Predicate.BuildDefinition.ExternalParameters = mustJSON(p)
		}},
		{"source repository substitution", IDUnexpectedExternalParameters, "externalParameters.source.repository", func(s *provenance.Statement) {
			p := validExternalParameters(ManagerNPM)
			p.Source.Repository = "https://github.com/other/project"
			p.Package.Repository = p.Source.Repository
			s.Predicate.BuildDefinition.ExternalParameters = mustJSON(p)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := validProvenanceInput(t, ManagerNPM)
			signing, err := NewProvenanceSigningInput(input)
			if err != nil {
				t.Fatalf("valid signing input: %v", err)
			}
			statement := signing.Statement()
			test.mutate(&statement)
			err = ValidateNPMStatement(statement, input)
			requireProvenanceDiagnostic(t, err, [2]string{test.id, test.field})
		})
	}
	t.Run("malformed pre-sign parameters", func(t *testing.T) {
		t.Parallel()
		input := validProvenanceInput(t, ManagerNPM)
		input.BuildMetadata.ExternalParameters = json.RawMessage(`{`)
		err := ValidateNPMStatement(provenance.Statement{}, input)
		requireProvenanceDiagnostic(t, err, [2]string{IDUnexpectedExternalParameters, "externalParameters"})
	})
}

func TestProvenanceExtraInvocationRefs(t *testing.T) {
	t.Parallel()
	for _, ref := range []string{"refs/heads", "refs/heads/", "refs/heads/.hidden", "refs/heads/main.lock", "refs/heads/main.", "refs//main", "refs/heads/a..b"} {
		t.Run(ref, func(t *testing.T) {
			t.Parallel()
			p := validExternalParameters(ManagerNPM)
			p.Source.InputRef = testStringPointer(p.Source.Ref)
			p.Source.InvocationRef = testStringPointer(ref)
			p.Source.InvocationRevision = testStringPointer(testAttestSHA)
			_, err := EncodeExternalParameters(p)
			requireProvenanceDiagnostic(t, err, [2]string{IDUnexpectedExternalParameters, "externalParameters.source.invocation_ref"})
		})
	}
}

func TestProvenanceExtraPURLVersionRejections(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"", "1.2.3\n", "1.2.3\x7f", "1.2.3\\build"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			_, err := NPMPackagePURL("package", version)
			requireProvenanceDiagnostic(t, err, [2]string{IDNPMSubjectMismatch, "subject[0].name"})
			if got := npmPURL("package", version); got != "" {
				t.Fatalf("npmPURL() = %q, want empty string for invalid version", got)
			}
		})
	}
}

func TestProvenanceExtraExpectedDependencies(t *testing.T) {
	t.Parallel()
	for _, malformed := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing expected dependency", true: "malformed expected annotation"}[malformed], func(t *testing.T) {
			t.Parallel()
			input := validProvenanceInput(t, ManagerNPM)
			signing, err := NewProvenanceSigningInput(input)
			if err != nil {
				t.Fatalf("valid signing input: %v", err)
			}
			want := [2]string{IDResolvedDependenciesUnexpectedEntry, "predicate.buildDefinition.resolvedDependencies"}
			if malformed {
				input.BuildMetadata.ResolvedDependencies[0].Annotations["package_manager"] = json.RawMessage(`{`)
				want = [2]string{IDResolvedDependenciesLockfile, "resolvedDependencies.lockfile"}
			} else {
				input.BuildMetadata.ResolvedDependencies = input.BuildMetadata.ResolvedDependencies[1:]
			}
			err = ValidateNPMStatement(signing.Statement(), input)
			requireProvenanceDiagnostic(t, err, want)
		})
	}
}

func TestProvenanceExtraSourceVersionBinding(t *testing.T) {
	t.Parallel()
	p := validExternalParameters(ManagerNPM)
	p.Source.Ref = "refs/tags/v2.0.0"
	p.Source.InputRef = testStringPointer(p.Source.Ref)
	p.Source.InvocationRef = testStringPointer("refs/heads/main")
	p.Source.InvocationRevision = testStringPointer(testAttestSHA)
	_, err := EncodeExternalParameters(p)
	requireProvenanceDiagnostic(t, err, [2]string{IDSourceRefInvalid, "source-ref"})
}

func TestProvenanceExtraOptionalRoundTrip(t *testing.T) {
	t.Parallel()
	for _, registry := range []string{"https://registry.npmjs.org/", "https://registry.example.com/"} {
		t.Run(registry, func(t *testing.T) {
			t.Parallel()
			p := validExternalParameters(ManagerNPM)
			p.Publish.ResolvedRegistryURL = registry
			p.Package.PackageURL = registry + "%40windlass%2Fslsa-builder/1.2.3"
			if registry != "https://registry.npmjs.org/" {
				p.Publish.CustomRegistrySupport = "unsupported-but-not-blocked"
			}
			p.Publish.PublishConfig = &PublishConfigParameters{Access: "public", Provenance: boolPointer(true)}
			p.Publish.PublishAccessOption = testStringPointer("public")
			p.Publish.EffectiveAccess = "public"
			p.Distribution.ReleaseAssetMode = true
			p.Distribution.ProvenanceSidecar = testStringPointer("required")
			p.Package.ConsumerSurface = map[string]json.RawMessage{"main": json.RawMessage(`"index.js"`)}
			encoded, err := EncodeExternalParameters(p)
			if err != nil {
				t.Fatalf("EncodeExternalParameters() = %v, want success", err)
			}
			got, err := DecodeExternalParameters(encoded)
			if err != nil {
				t.Fatalf("DecodeExternalParameters() = %v, want success", err)
			}
			if string(mustJSON(got)) != string(mustJSON(p)) {
				t.Fatalf("round trip = %s, want %s", mustJSON(got), mustJSON(p))
			}
		})
	}
}
