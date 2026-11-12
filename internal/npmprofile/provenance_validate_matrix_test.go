package npmprofile

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func requireProvenanceDiagnostic(t *testing.T, err error, want [2]string) {
	t.Helper()
	requireNPMDiagnostic(t, err, want[0])
	var npmError *npmProvenanceValidationError
	var commonError *provenance.ValidationError
	var field string
	switch {
	case errors.As(err, &npmError):
		field = npmError.diagnostic.Field
	case errors.As(err, &commonError):
		field = commonError.Diagnostic.Field
	default:
		t.Fatalf("error %T does not expose an expected diagnostic field: %v", err, err)
	}
	if field != want[1] {
		t.Fatalf("diagnostic field = %q, want %q: %v", field, want[1], err)
	}
}

func TestProvenanceMatrixExternalSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, group, member, value string
	}{
		{"repository spelling", "source", "repository", `"https://github.com/Example/project"`},
		{"repository disagreement", "package", "repository", `"https://github.com/other/project"`},
		{"source SHA", "source", "revision", `"short"`},
		{"workflow ref", "workflow", "ref", `"main"`},
		{"workflow path", "workflow", "path", `"other.yml"`},
		{"workflow repository", "workflow", "repository", `"https://github.com/Example/project"`},
		{"event missing", "source", "event_name", `""`},
		{"branch type", "source", "ref_type", `"branch"`},
		{"runner", "runtime", "runner", `"ubuntu-latest"`},
		{"node abbreviated", "runtime", "node_version", `"24"`},
		{"node below floor", "runtime", "node_version", `"23.9.9"`},
		{"npm below floor", "runtime", "npm_version", `"11.5.0"`},
		{"private package", "package", "private", `true`},
		{"packed version", "package", "packed_version", `"1.2.4"`},
		{"tarball traversal", "package", "tarball_name", `"../package.tgz"`},
		{"tarball extension", "package", "tarball_name", `"package.zip"`},
		{"package URL", "package", "package_url", `"https://registry.npmjs.org/?token=secret"`},
		{"registry credentials", "publish", "resolved_registry_url", `"https://secret@registry.npmjs.org/"`},
		{"unknown manager", "package_manager", "name", `"bun"`},
		{"manager abbreviated", "package_manager", "version", `"11"`},
		{"manager root escape", "package_manager", "root", `"../outside"`},
		{"lockfile selection missing path", "package_manager", "selection_source", `"lockfile"`},
		{"manifest selection missing path", "package_manager", "selection_manifest_path", `null`},
		{"npm yarn mode", "package_manager", "yarn_install_mode", `"immutable"`},
		{"token publishing", "publish", "trusted_publishing", `false`},
		{"automatic provenance", "publish", "provenance_file", `false`},
		{"unknown identity", "publish", "package_identity_preexisting", `null`},
		{"new identity", "publish", "package_identity_preexisting", `false`},
		{"unknown version", "publish", "package_version_preexisting", `null`},
		{"existing version", "publish", "package_version_preexisting", `true`},
		{"effective access", "publish", "effective_access", `"private"`},
		{"access option", "publish", "publish_access_option", `"private"`},
		{"npmjs custom marker", "publish", "custom_registry_support", `"unsupported-but-not-blocked"`},
		{"release missing", "release", "ref", `""`},
		{"release sidecar missing", "distribution", "provenance_sidecar", `null`},
		{"npm only sidecar", "distribution", "provenance_sidecar", `"required"`},
		{"caller path", "caller", "workflow_filename", `"nested/release.yml"`},
		{"build unknown result", "build", "script_result", `"unknown"`},
		{"present script skipped", "build", "script_result", `"skipped-absent"`},
		{"consumer field", "package", "consumer_surface", `{"scripts":{}}`},
		{"packed file escape", "package", "packed_files", `["../secret"]`},
		{"partial source context", "source", "input_ref", `"refs/tags/v1.2.3"`},
		{"workflow ref differs from invocation ref", "source", "invocation_ref", `"refs/heads/other"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var object map[string]map[string]json.RawMessage
			if err := json.Unmarshal(mustJSON(validExternalParameters(ManagerNPM)), &object); err != nil {
				t.Fatal(err)
			}
			if object[test.group] == nil {
				object[test.group] = make(map[string]json.RawMessage)
			}
			if test.group == "source" {
				object[test.group]["ref"] = json.RawMessage(`"refs/tags/v1.2.3"`)
			}
			object[test.group][test.member] = json.RawMessage(test.value)
			_, err := DecodeExternalParameters(mustJSON(object))
			requireNPMDiagnostic(t, err, IDUnexpectedExternalParameters)
		})
	}
	t.Run("workflow ref differs from built ref", func(t *testing.T) {
		t.Parallel()
		input := validProvenanceInput(t, ManagerNPM)
		parameters := validExternalParameters(ManagerNPM)
		parameters.Workflow.Ref = "refs/heads/main"
		input.BuildMetadata.ExternalParameters = mustJSON(parameters)
		requireNPMDiagnostic(t, newProvenanceInputError(input), IDInvocationSource)
	})
}

func TestProvenanceMatrixDependencyRejections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, dependency, suffix string
		mutate                   func(*provenance.ResourceDescriptor)
	}{
		{"lockfile URI", "lockfile", "", func(d *provenance.ResourceDescriptor) { d.URI += "-other" }},
		{"runtime URI", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.URI += ".other" }},
		{"runtime wrong digest", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Digest["sha256"] = strings.Repeat("0", 64) }},
		{"runtime uppercase digest", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Digest["sha256"] = strings.ToUpper(d.Digest["sha256"]) }},
		{"runtime short digest", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Digest["sha256"] = d.Digest["sha256"][:63] }},
		{"runtime nonhex digest", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Digest["sha256"] = strings.Repeat("g", 64) }},
		{"runtime missing digest", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Digest = nil }},
		{"runtime missing sha256", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Digest = map[string]string{"sha512": testSHA512} }},
		{"runtime extra digest", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Digest["sha512"] = testSHA512 }},
		{"runtime missing annotations", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Annotations = nil }},
		{"runtime missing authority", "node-runtime", "", func(d *provenance.ResourceDescriptor) { delete(d.Annotations, "digest_authority") }},
		{"runtime missing platform", "node-runtime", "", func(d *provenance.ResourceDescriptor) { delete(d.Annotations, "platform") }},
		{"runtime unknown annotation", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Annotations["role"] = mustJSON("runtime") }},
		{"runtime authority type", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Annotations["digest_authority"] = mustJSON(42) }},
		{"runtime platform type", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Annotations["platform"] = mustJSON([]string{"linux-x64"}) }},
		{"runtime authority value", "node-runtime", "", func(d *provenance.ResourceDescriptor) {
			d.Annotations["digest_authority"] = mustJSON("registry-integrity")
		}},
		{"runtime platform value", "node-runtime", "", func(d *provenance.ResourceDescriptor) { d.Annotations["platform"] = mustJSON("linux-arm64") }},
		{"lockfile digest length", "lockfile", "", func(d *provenance.ResourceDescriptor) { d.Digest["sha256"] = "00" }},
		{"lockfile annotations", "lockfile", "", func(d *provenance.ResourceDescriptor) { d.Annotations["package_manager"] = json.RawMessage(`"npm"`) }},
		{"lockfile malformed annotation", "lockfile", "", func(d *provenance.ResourceDescriptor) { d.Annotations["package_manager"] = json.RawMessage(`{`) }},
		{"distribution insecure", "package-manager-distribution", "", func(d *provenance.ResourceDescriptor) { d.URI = "http://registry.npmjs.org/pnpm.tgz" }},
		{"distribution digest", "package-manager-distribution", "", func(d *provenance.ResourceDescriptor) { d.Digest["sha512"] = strings.Repeat("F", 128) }},
		{"distribution authority", "package-manager-distribution", "", func(d *provenance.ResourceDescriptor) {
			d.Annotations["digest_authority"] = json.RawMessage(`"download-hash"`)
		}},
		{"runner malformed URI", "runner-image", ".uri", func(d *provenance.ResourceDescriptor) { d.URI = "https://%" }},
		{"runner host", "runner-image", ".uri", func(d *provenance.ResourceDescriptor) { d.URI = "https://example.com/actions/runner-images/report" }},
		{"runner path", "runner-image", ".uri", func(d *provenance.ResourceDescriptor) { d.URI = "https://github.com/other/report" }},
		{"runner empty annotation", "runner-image", ".annotations", func(d *provenance.ResourceDescriptor) { d.Annotations["operating_system"] = json.RawMessage(`""`) }},
		{"runner annotation type", "runner-image", ".annotations", func(d *provenance.ResourceDescriptor) { d.Annotations["image_version"] = json.RawMessage(`123`) }},
		{"runner node annotation forbidden", "runner-image", "", func(d *provenance.ResourceDescriptor) { d.Annotations["node_version"] = json.RawMessage(`"v24.1.0"`) }},
		{"runner missing deployment annotation", "runner-image", "", func(d *provenance.ResourceDescriptor) {
			delete(d.Annotations, "deployment_version")
			d.Annotations["unrelated"] = json.RawMessage(`"x"`)
		}},
		{"runner deployment annotation type", "runner-image", ".annotations", func(d *provenance.ResourceDescriptor) { d.Annotations["deployment_version"] = json.RawMessage(`123`) }},
		{"runner deployment empty annotation", "runner-image", ".annotations", func(d *provenance.ResourceDescriptor) { d.Annotations["deployment_version"] = json.RawMessage(`""`) }},
	}
	ids := map[string]string{"lockfile": IDResolvedDependenciesLockfile, "node-runtime": "windlass.verify.error.resolved-dependencies-node-runtime", "package-manager-distribution": IDResolvedDependenciesDistribution, "runner-image": IDResolvedDependenciesRunnerImage}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := validProvenanceInput(t, ManagerPNPM)
			for index := range input.BuildMetadata.ResolvedDependencies {
				if input.BuildMetadata.ResolvedDependencies[index].Name == test.dependency {
					test.mutate(&input.BuildMetadata.ResolvedDependencies[index])
				}
			}
			_, err := NewProvenanceSigningInput(input)
			requireProvenanceDiagnostic(t, err, [2]string{ids[test.dependency], "resolvedDependencies." + test.dependency + test.suffix})
		})
	}
	for _, name := range []string{"lockfile", "runner-image"} {
		for _, duplicate := range []bool{false, true} {
			t.Run(name+"/duplicate="+map[bool]string{true: "yes", false: "no"}[duplicate], func(t *testing.T) {
				t.Parallel()
				input := validProvenanceInput(t, ManagerNPM)
				var dependencies []provenance.ResourceDescriptor
				for _, value := range input.BuildMetadata.ResolvedDependencies {
					if value.Name != name || duplicate {
						dependencies = append(dependencies, value)
					}
					if value.Name == name && duplicate {
						dependencies = append(dependencies, value)
					}
				}
				input.BuildMetadata.ResolvedDependencies = dependencies
				_, err := NewProvenanceSigningInput(input)
				requireProvenanceDiagnostic(t, err, [2]string{ids[name], "resolvedDependencies." + name})
			})
		}
	}
}

func TestProvenanceMatrixSubjectRejections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, id, field string
		mutate          func(*provenance.Subject)
	}{
		{"name substitution", IDNPMSubjectMismatch, "name", func(s *provenance.Subject) { s.Name = "pkg:npm/other@1.2.3" }},
		{"sha256 missing", IDMissingSubjectSHA256, "digest.sha256", func(s *provenance.Subject) { delete(s.Digest, "sha256"); s.Digest["sha384"] = testSHA256 }},
		{"sha256 malformed", IDMissingSubjectSHA256, "digest.sha256", func(s *provenance.Subject) { s.Digest["sha256"] = strings.Repeat("G", 64) }},
		{"sha512 missing", IDMissingSubjectSHA512, "digest.sha512", func(s *provenance.Subject) { delete(s.Digest, "sha512"); s.Digest["sha384"] = testSHA512 }},
		{"sha512 malformed", IDMissingSubjectSHA512, "digest.sha512", func(s *provenance.Subject) { s.Digest["sha512"] = "00" }},
		{"sha512 substitution", "windlass.verify.error.digest-mismatch", "digest", func(s *provenance.Subject) { s.Digest["sha512"] = strings.Repeat("0", 128) }},
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
			test.mutate(&statement.Subject[0])
			err = ValidateNPMStatement(statement, input)
			requireProvenanceDiagnostic(t, err, [2]string{test.id, "subject[0]." + test.field})
		})
	}
}

func TestProvenanceMatrixRelativePaths(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "/absolute", `nested\file`, "nested/../../outside"} {
		for _, field := range []string{"package.packed_files", "package_manager.root"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				t.Parallel()
				var object map[string]map[string]json.RawMessage
				if err := json.Unmarshal(mustJSON(validExternalParameters(ManagerNPM)), &object); err != nil {
					t.Fatal(err)
				}
				group, member, _ := strings.Cut(field, ".")
				if object[group] == nil {
					object[group] = make(map[string]json.RawMessage)
				}
				object[group][member] = mustJSON(value)
				_, err := DecodeExternalParameters(mustJSON(object))
				requireNPMDiagnostic(t, err, IDUnexpectedExternalParameters)
			})
		}
	}
}
