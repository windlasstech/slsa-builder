package npmprofile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorkflowIntentConflicts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, registry, tag, access, message string
		source                               *PublishConfigParameters
	}{
		{"provenance disabled", "", "", "", "publishConfig.provenance must not disable provenance", &PublishConfigParameters{Provenance: boolPointer(false)}},
		{"registry conflict", "https://registry.npmjs.org/", "", "", "registry-url conflicts with publishConfig", &PublishConfigParameters{Registry: "https://example.com/"}},
		{"insecure registry", "http://registry.npmjs.org/", "", "", "registry URL must be an absolute credential-free HTTPS root URL", nil},
		{"tag conflict", "", "latest", "", "dist-tag is invalid or conflicts with publishConfig", &PublishConfigParameters{Tag: "next"}},
		{"invalid tag", "", "../next", "", "dist-tag is invalid or conflicts with publishConfig", nil},
		{"access conflict", "", "", "public", "access is invalid or conflicts with publishConfig", &PublishConfigParameters{Access: "restricted"}},
		{"invalid access", "", "", "private", "access is invalid or conflicts with publishConfig", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolvePublishIntent(test.registry, test.tag, test.access, test.source)
			if err == nil || err.Error() != test.message || !reflect.DeepEqual(got, PublishParameters{}) {
				t.Fatalf("ResolvePublishIntent() = %#v, %v; want empty intent and %q", got, err, test.message)
			}
		})
	}
}

func TestWorkflowIntentNormalizedInputs(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, registry, tag, access string
		want                        PublishParameters
	}{
		{"whitespace omissions", " \t", "\r\n", "\f\v", PublishParameters{ResolvedRegistryURL: "https://registry.npmjs.org/", ResolvedDistTag: "latest", EffectiveAccess: "existing-package-access", TrustedPublishing: true, ProvenanceFile: true}},
		{"custom registry", " https://EXAMPLE.COM:443 ", " next ", " restricted ", PublishParameters{InputRegistryURL: testStringPointer("https://EXAMPLE.COM:443"), InputDistTag: testStringPointer("next"), InputAccess: testStringPointer("restricted"), ResolvedRegistryURL: "https://example.com/", ResolvedDistTag: "next", PublishAccessOption: testStringPointer("restricted"), EffectiveAccess: "restricted", TrustedPublishing: true, ProvenanceFile: true, CustomRegistrySupport: "unsupported-but-not-blocked"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolvePublishIntent(test.registry, test.tag, test.access, nil)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ResolvePublishIntent() = %#v, %v; want %#v", got, err, test.want)
			}
		})
	}
}

func TestWorkflowIntentManifestFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, manifest, message string }{
		{"missing manifest", "", ""},
		{"invalid publish config", `{"publishConfig":42}`, "publishConfig must be an object"},
		{"forbidden directory", `{"publishConfig":{"directory":"dist"}}`, "publishConfig.directory is unsupported"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if test.manifest != "" {
				if err := os.WriteFile(filepath.Join(directory, "package.json"), []byte(test.manifest), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := ResolveWorkflowPublishIntent(Result{Package: Package{RealDirectory: directory}}, "", "", "")
			if test.manifest == "" {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("missing manifest error = %v, want os.ErrNotExist", err)
				}
			} else if err == nil || err.Error() != test.message {
				t.Fatalf("manifest error = %v, want %q", err, test.message)
			}
			if !reflect.DeepEqual(got, PublishParameters{}) {
				t.Fatalf("invalid manifest returned intent: %#v", got)
			}
		})
	}
}

func TestWorkflowIntentFinalizeRejections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, message, id string
		change            func(*Result, *BuildPackResult, *WorkflowBuildMetadataConfig)
	}{
		{"caller path", "caller workflow filename is invalid", "", func(s *Result, b *BuildPackResult, c *WorkflowBuildMetadataConfig) {
			_, _ = s, b
			c.CallerWorkflowFilename = "../release.yml"
		}},
		{"missing artifact", "build result is incomplete or conflicts with selected package", "", func(s *Result, b *BuildPackResult, c *WorkflowBuildMetadataConfig) { _, _ = s, b; c.ArtifactName = "" }},
		{"invalid intent", "access is invalid or conflicts with publishConfig", "", func(s *Result, b *BuildPackResult, c *WorkflowBuildMetadataConfig) {
			_, _ = s, b
			c.AccessInput = "private"
		}},
		{"package URL", "unsupported registry package URL input", "", func(s *Result, b *BuildPackResult, c *WorkflowBuildMetadataConfig) {
			_ = c
			s.Package.Name = "bad\x00name"
			b.PackageName = "bad\x00name"
			b.Packed.Name = "bad\x00name"
		}},
		{"missing distribution", "corepack package-manager distribution capture is required", "", func(s *Result, b *BuildPackResult, c *WorkflowBuildMetadataConfig) {
			_ = c
			s.Manager.Name = ManagerPNPM
			s.Manager.Version = "11.28.3"
			b.Toolchain.PackageManagerVersion = "11.28.3"
		}},
		{"invalid runtime", "", IDUnexpectedExternalParameters, func(s *Result, b *BuildPackResult, c *WorkflowBuildMetadataConfig) {
			_, _ = s, c
			b.Toolchain.NodeVersion = "v22.0.0"
		}},
		{"invalid runner", "", IDResolvedDependenciesRunnerImage, func(s *Result, b *BuildPackResult, c *WorkflowBuildMetadataConfig) {
			_, _ = s, c
			b.Toolchain.Runner.ImageOS = ""
		}},
		{"source ref drift", "", IDSourceRefInvalid, func(s *Result, b *BuildPackResult, c *WorkflowBuildMetadataConfig) {
			_, _ = s, b
			c.SourceRefInput = "refs/tags/v1.2.3"
			c.InvocationRef = "refs/heads/main"
			c.InvocationRevision = testAttestSHA
			c.Ref = "refs/tags/v1.2.4"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given valid build metadata with one changed boundary value.
			selection, build, config := workflowIntentInputs(t)
			test.change(&selection, &build, &config)
			// When finalizing, then no partial metadata escapes a rejected gate.
			got, err := FinalizeWorkflowBuildMetadata(selection, build, config)
			if test.id != "" {
				requireNPMDiagnostic(t, err, test.id)
			} else if err == nil || err.Error() != test.message {
				t.Fatalf("FinalizeWorkflowBuildMetadata() error = %v, want %q", err, test.message)
			}
			if !reflect.DeepEqual(got, BuildMetadata{}) {
				t.Fatalf("rejection returned partial metadata: %#v", got)
			}
		})
	}
}

func TestWorkflowIntentMissingLockfile(t *testing.T) {
	t.Parallel()
	selection, build, config := workflowIntentInputs(t)
	selection.Manager.SelectedLockfilePath = "missing-lock.json"
	_, err := FinalizeWorkflowBuildMetadata(selection, build, config)
	if err == nil || err.Error() != "read selected lockfile: path is not a bounded regular file" {
		t.Fatalf("FinalizeWorkflowBuildMetadata() error = %v, want bounded regular lockfile rejection", err)
	}
}

func workflowIntentInputs(t *testing.T) (Result, BuildPackResult, WorkflowBuildMetadataConfig) {
	t.Helper()
	directory := t.TempDir()
	for name, data := range map[string]string{"package.json": `{"name":"@windlass/slsa-builder","version":"1.2.3"}`, "package-lock.json": `{"lockfileVersion":3}`} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	selection := Result{
		Package: Package{Directory: ".", RealDirectory: directory, RealManagerRoot: directory, ManagerRoot: ".", Name: "@windlass/slsa-builder", Version: "1.2.3", Repository: "https://github.com/example/project"},
		Manager: ManagerSelection{Name: ManagerNPM, Version: "11.5.1", Source: SelectionPackageManager, SelectionManifestPath: "package.json", SelectedLockfilePath: "package-lock.json"},
	}
	build := BuildPackResult{
		PackageName: selection.Package.Name, PackageVersion: "1.2.3", TarballPath: filepath.Join(directory, "windlass-slsa-builder-1.2.3.tgz"),
		SHA256: mustSHA256(t, testSHA256), SHA512: mustSHA512(t, testSHA512),
		Packed: PackedMetadata{Name: selection.Package.Name, Version: "1.2.3"}, BuildScript: BuildScriptCapture{Present: true, Result: BuildScriptExecuted},
		Toolchain: ToolchainCapture{NodeVersion: "v24.0.0", NPMVersion: "11.5.1", PackageManagerVersion: "11.5.1", Runner: RunnerCapture{ImageOS: "ubuntu24", ImageVersion: "20260801.1.0", IncludedSoftwareURL: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md"}},
	}
	config := WorkflowBuildMetadataConfig{ArtifactName: "tarball", EventName: "push", RefType: "tag", Ref: "refs/tags/v1.2.3", Revision: testSourceSHA, WorkflowSHA: testSourceSHA, CallerWorkflowFilename: "release.yml", RegistryState: RegistryPreflightState{PackageExists: true}}
	return selection, build, config
}
