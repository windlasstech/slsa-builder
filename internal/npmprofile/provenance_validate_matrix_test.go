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
		name, field, id string
		mutate          func(*ExternalParameters)
	}{
		{"repository spelling", "source.repository", "", func(p *ExternalParameters) { p.Source.Repository = "https://github.com/Example/project" }},
		{"repository disagreement", "package.repository", IDPackageRepositoryIdentityMismatch, func(p *ExternalParameters) { p.Package.Repository = "https://github.com/other/project" }},
		{"source SHA", "source.revision", "", func(p *ExternalParameters) { p.Source.Revision = "short" }},
		{"workflow SHA", "source.revision", "", func(p *ExternalParameters) { p.Workflow.SHA = strings.ToUpper(testSourceSHA) }},
		{"workflow path", "workflow.path", "", func(p *ExternalParameters) { p.Workflow.Path = "other.yml" }},
		{"builder binding", "workflow.builder_id", "windlass.verify.error.builder-id-not-immutable", func(p *ExternalParameters) { p.Workflow.BuilderID += "0" }},
		{"event missing", "source", "", func(p *ExternalParameters) { p.Source.EventName = "" }},
		{"branch type", "source", "", func(p *ExternalParameters) { p.Source.RefType = "branch" }},
		{"runner", "runtime", "", func(p *ExternalParameters) { p.Runtime.Runner = "ubuntu-latest" }},
		{"node abbreviated", "runtime", "", func(p *ExternalParameters) { p.Runtime.NodeVersion = "24" }},
		{"node below floor", "runtime", "", func(p *ExternalParameters) { p.Runtime.NodeVersion = "23.9.9" }},
		{"npm below floor", "runtime", "", func(p *ExternalParameters) { p.Runtime.NPMVersion = "11.5.0" }},
		{"private package", "package", "", func(p *ExternalParameters) { p.Package.Private = true }},
		{"packed version", "package", "", func(p *ExternalParameters) { p.Package.PackedVersion = "1.2.4" }},
		{"tarball traversal", "package.tarball_name", IDNPMSubjectMismatch, func(p *ExternalParameters) { p.Package.TarballName = "../package.tgz" }},
		{"tarball extension", "package.tarball_name", IDNPMSubjectMismatch, func(p *ExternalParameters) { p.Package.TarballName = "package.zip" }},
		{"package URL", "package.package_url", "windlass.verify.error.package-url-mismatch", func(p *ExternalParameters) { p.Package.PackageURL += "?token=secret" }},
		{"registry credentials", "package.package_url", "windlass.verify.error.package-url-mismatch", func(p *ExternalParameters) { p.Publish.ResolvedRegistryURL = "https://secret@registry.npmjs.org/" }},
		{"unknown manager", "package_manager", "", func(p *ExternalParameters) { p.PackageManager.Name = "bun" }},
		{"manager abbreviated", "package_manager", "", func(p *ExternalParameters) { p.PackageManager.Version = "11" }},
		{"manager root escape", "package_manager.root", "", func(p *ExternalParameters) { p.PackageManager.Root = "../outside" }},
		{"lockfile selection missing path", "package_manager", "", func(p *ExternalParameters) { p.PackageManager.SelectionSource = SelectionLockfile }},
		{"manifest selection missing path", "package_manager", "", func(p *ExternalParameters) { p.PackageManager.SelectionManifestPath = nil }},
		{"npm yarn mode", "package_manager.yarn_install_mode", "", func(p *ExternalParameters) { p.PackageManager.YarnInstallMode = "immutable" }},
		{"token publishing", "publish", "", func(p *ExternalParameters) { p.Publish.TrustedPublishing = false }},
		{"automatic provenance", "publish", "", func(p *ExternalParameters) { p.Publish.ProvenanceFile = false }},
		{"unknown identity", "publish", "", func(p *ExternalParameters) { p.Publish.PackageIdentityPreexisting = nil }},
		{"new identity", "publish", "", func(p *ExternalParameters) { p.Publish.PackageIdentityPreexisting = boolPointer(false) }},
		{"unknown version", "publish", "", func(p *ExternalParameters) { p.Publish.PackageVersionPreexisting = nil }},
		{"existing version", "publish", "", func(p *ExternalParameters) { p.Publish.PackageVersionPreexisting = boolPointer(true) }},
		{"effective access", "publish.effective_access", "", func(p *ExternalParameters) { p.Publish.EffectiveAccess = "private" }},
		{"access option", "publish.publish_access_option", "", func(p *ExternalParameters) { p.Publish.PublishAccessOption = testStringPointer("private") }},
		{"npmjs custom marker", "publish.custom_registry_support", "", func(p *ExternalParameters) { p.Publish.CustomRegistrySupport = "unsupported-but-not-blocked" }},
		{"release missing", "release", "", func(p *ExternalParameters) { p.Release.Ref = "" }},
		{"release sidecar missing", "distribution.provenance_sidecar", "windlass.verify.error.release-asset-mode-schema-error", func(p *ExternalParameters) { p.Distribution.ReleaseAssetMode = true }},
		{"npm only sidecar", "distribution.provenance_sidecar", "windlass.verify.error.release-asset-mode-schema-error", func(p *ExternalParameters) { p.Distribution.ProvenanceSidecar = testStringPointer("required") }},
		{"caller path", "caller.workflow_filename", "windlass.verify.error.trusted-publisher-mismatch", func(p *ExternalParameters) { p.Caller.WorkflowFilename = "nested/release.yml" }},
		{"build unknown result", "build", "", func(p *ExternalParameters) { p.Build.ScriptResult = "unknown" }},
		{"present script skipped", "build", "", func(p *ExternalParameters) { p.Build.ScriptResult = BuildScriptSkippedAbsent }},
		{"consumer field", "package.consumer_surface", "", func(p *ExternalParameters) {
			p.Package.ConsumerSurface = map[string]json.RawMessage{"scripts": json.RawMessage(`{}`)}
		}},
		{"packed file escape", "package.packed_files", "", func(p *ExternalParameters) { p.Package.PackedFiles = []string{"../secret"} }},
		{"partial source context", "source", "", func(p *ExternalParameters) { p.Source.InputRef = testStringPointer(p.Source.Ref) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given a valid record with one violated semantic constraint.
			parameters := validExternalParameters(ManagerNPM)
			test.mutate(&parameters)
			id := test.id
			if id == "" {
				id = IDUnexpectedExternalParameters
			}
			// When encoding through the public validation boundary.
			_, err := EncodeExternalParameters(parameters)
			// Then the rejection identifies the violated constraint.
			requireProvenanceDiagnostic(t, err, [2]string{id, "externalParameters." + test.field})
		})
	}
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
		{"lockfile annotations", "lockfile", ".annotations", func(d *provenance.ResourceDescriptor) { d.Annotations["package_manager"] = json.RawMessage(`"npm"`) }},
		{"lockfile malformed annotation", "lockfile", ".annotations", func(d *provenance.ResourceDescriptor) { d.Annotations["package_manager"] = json.RawMessage(`{`) }},
		{"distribution insecure", "package-manager-distribution", "", func(d *provenance.ResourceDescriptor) { d.URI = "http://registry.npmjs.org/pnpm.tgz" }},
		{"distribution digest", "package-manager-distribution", "", func(d *provenance.ResourceDescriptor) { d.Digest["sha512"] = strings.Repeat("F", 128) }},
		{"distribution authority", "package-manager-distribution", ".annotations", func(d *provenance.ResourceDescriptor) {
			d.Annotations["digest_authority"] = json.RawMessage(`"download-hash"`)
		}},
		{"runner malformed URI", "runner-image", ".uri", func(d *provenance.ResourceDescriptor) { d.URI = "https://%" }},
		{"runner host", "runner-image", ".uri", func(d *provenance.ResourceDescriptor) { d.URI = "https://example.com/actions/runner-images/report" }},
		{"runner path", "runner-image", ".uri", func(d *provenance.ResourceDescriptor) { d.URI = "https://github.com/other/report" }},
		{"runner empty annotation", "runner-image", ".annotations", func(d *provenance.ResourceDescriptor) { d.Annotations["image_os"] = json.RawMessage(`""`) }},
		{"runner annotation type", "runner-image", ".annotations", func(d *provenance.ResourceDescriptor) { d.Annotations["image_version"] = json.RawMessage(`123`) }},
		{"runner node mismatch", "runner-image", ".annotations.node_version", func(d *provenance.ResourceDescriptor) { d.Annotations["node_version"] = json.RawMessage(`"v24.1.0"`) }},
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
				p := validExternalParameters(ManagerNPM)
				switch field {
				case "package.packed_files":
					p.Package.PackedFiles = []string{value}
				case "package_manager.root":
					p.PackageManager.Root = value
				}
				_, err := EncodeExternalParameters(p)
				requireProvenanceDiagnostic(t, err, [2]string{IDUnexpectedExternalParameters, "externalParameters." + field})
			})
		}
	}
}
