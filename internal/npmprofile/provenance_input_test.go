package npmprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	testSourceSHA = "0123456789abcdef0123456789abcdef01234567"
	testAttestSHA = "89abcdef0123456789abcdef0123456789abcdef"
	testSHA256    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testSHA512    = testSHA256 + testSHA256
)

func TestNPMProvenanceInput(t *testing.T) {
	t.Parallel()

	input := validProvenanceInput(t, ManagerPNPM)
	signing, err := NewProvenanceSigningInput(input)
	if err != nil {
		t.Fatalf("NewProvenanceSigningInput() error = %v", err)
	}
	if signing.PredicateType != provenance.PredicateType || signing.StatementFileName != NPMProvenanceStatementFile {
		t.Fatalf("signing input = %#v", signing)
	}
	if signing.Subject.Name != "pkg:npm/%40windlass/slsa-builder@1.2.3" {
		t.Fatalf("subject name = %q", signing.Subject.Name)
	}
	if len(signing.Subject.Digest) != 2 || signing.Subject.Digest["sha256"] != testSHA256 || signing.Subject.Digest["sha512"] != testSHA512 {
		t.Fatalf("subject digest = %#v", signing.Subject.Digest)
	}
	goldenPath := filepath.Join("..", "..", "testdata", "npm", "provenance", "npm-predicate.jcs.json")
	if os.Getenv("UPDATE_P01_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o700); err != nil {
			t.Fatalf("create golden directory: %v", err)
		}
		if err := os.WriteFile(goldenPath, signing.PredicateJSON, 0o600); err != nil {
			t.Fatalf("write generated golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden predicate: %v", err)
	}
	if !bytes.Equal(signing.PredicateJSON, want) {
		t.Fatalf("predicate differs from JCS golden\n got: %s\nwant: %s", signing.PredicateJSON, want)
	}

	statement := signing.Statement()
	wantStatement, err := statement.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(signing.StatementJSON, wantStatement) {
		t.Fatal("StatementJSON does not preserve the exact preassembled Statement bytes")
	}
	if err := ValidateNPMStatement(statement, input); err != nil {
		t.Fatalf("ValidateNPMStatement() error = %v", err)
	}

	t.Run("unknown external parameter", func(t *testing.T) {
		mutated := statement
		mutated.Predicate.BuildDefinition.ExternalParameters = addJSONMember(t, mutated.Predicate.BuildDefinition.ExternalParameters, "injected", true)
		requireNPMDiagnostic(t, ValidateNPMStatement(mutated, input), IDUnexpectedExternalParameters)
	})
	t.Run("missing nested external parameter", func(t *testing.T) {
		mutated := statement
		mutated.Predicate.BuildDefinition.ExternalParameters = removeNestedJSONMember(t, mutated.Predicate.BuildDefinition.ExternalParameters, "distribution", "linked_artifact_metadata")
		requireNPMDiagnostic(t, ValidateNPMStatement(mutated, input), IDUnexpectedExternalParameters)
	})
	t.Run("subject digest label confusion", func(t *testing.T) {
		mutated := statement
		mutated.Subject = cloneSubjects(statement.Subject)
		mutated.Subject[0].Digest = map[string]string{"sha256": testSHA256, "sha512": testSHA512, "sha-256": testSHA256}
		requireNPMDiagnostic(t, ValidateNPMStatement(mutated, input), IDSubjectDigestScopeInvalid)
	})
	t.Run("subject digest substitution", func(t *testing.T) {
		mutated := statement
		mutated.Subject = cloneSubjects(statement.Subject)
		mutated.Subject[0].Digest["sha256"] = strings.Repeat("0", 64)
		requireNPMDiagnostic(t, ValidateNPMStatement(mutated, input), "windlass.verify.error.digest-mismatch")
	})
	t.Run("lockfile digest", func(t *testing.T) {
		mutated := statement
		mutated.Predicate.BuildDefinition.ResolvedDependencies = cloneDependencies(statement.Predicate.BuildDefinition.ResolvedDependencies)
		mutated.Predicate.BuildDefinition.ResolvedDependencies[0].Digest["sha256"] = strings.Repeat("0", 64)
		requireNPMDiagnostic(t, ValidateNPMStatement(mutated, input), IDResolvedDependenciesLockfile)
	})
}

func TestScopedPURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "plain-package", version: "1.2.3", want: "pkg:npm/plain-package@1.2.3"},
		{name: "@scope/name", version: "1.2.3", want: "pkg:npm/%40scope/name@1.2.3"},
		{name: "@scope/name", version: "1.2.3+build/meta", want: "pkg:npm/%40scope/name@1.2.3%2Bbuild%2Fmeta"},
		{name: "@scöpe/name", version: "1.0.0", want: "pkg:npm/%40sc%C3%B6pe/name@1.0.0"},
	}
	for _, test := range tests {
		t.Run(test.name+"@"+test.version, func(t *testing.T) {
			got, err := NPMPackagePURL(test.name, test.version)
			if err != nil {
				t.Fatalf("NPMPackagePURL() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("NPMPackagePURL() = %q, want %q", got, test.want)
			}
		})
	}
	for _, invalid := range []string{"", "@scope", "@/name", "@scope/name/extra", "name/extra", "name\n"} {
		if _, err := NPMPackagePURL(invalid, "1.0.0"); err == nil {
			t.Errorf("NPMPackagePURL(%q) accepted malformed package name", invalid)
		}
	}
}

func TestResolvedDependencies(t *testing.T) {
	t.Parallel()

	for _, manager := range []Manager{ManagerNPM, ManagerPNPM, ManagerYarn} {
		t.Run(string(manager), func(t *testing.T) {
			input := validProvenanceInput(t, manager)
			signing, err := NewProvenanceSigningInput(input)
			if err != nil {
				t.Fatalf("NewProvenanceSigningInput() error = %v", err)
			}
			want := 2
			if manager != ManagerNPM {
				want = 3
			}
			if len(signing.Predicate.BuildDefinition.ResolvedDependencies) != want {
				t.Fatalf("dependency count = %d, want %d", len(signing.Predicate.BuildDefinition.ResolvedDependencies), want)
			}
		})
	}

	input := validProvenanceInput(t, ManagerPNPM)
	tests := []struct {
		name   string
		mutate func([]provenance.ResourceDescriptor) []provenance.ResourceDescriptor
		wantID string
	}{
		{name: "unknown", wantID: IDResolvedDependenciesUnexpectedEntry, mutate: func(values []provenance.ResourceDescriptor) []provenance.ResourceDescriptor {
			return append(values, provenance.ResourceDescriptor{Name: "transitive-package"})
		}},
		{name: "missing distribution", wantID: IDResolvedDependenciesDistribution, mutate: func(values []provenance.ResourceDescriptor) []provenance.ResourceDescriptor {
			return append(values[:1:1], values[2:]...)
		}},
		{name: "runner digest", wantID: IDResolvedDependenciesRunnerImage, mutate: func(values []provenance.ResourceDescriptor) []provenance.ResourceDescriptor {
			values[2].Digest = map[string]string{"sha256": testSHA256}
			return values
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := input
			mutated.BuildMetadata.ResolvedDependencies = test.mutate(cloneDependencies(input.BuildMetadata.ResolvedDependencies))
			requireNPMDiagnostic(t, newProvenanceInputError(mutated), test.wantID)
		})
	}
}

func TestBuilderFields(t *testing.T) {
	t.Parallel()

	for _, manager := range []Manager{ManagerNPM, ManagerPNPM} {
		t.Run(string(manager), func(t *testing.T) {
			input := validProvenanceInput(t, manager)
			signing, err := NewProvenanceSigningInput(input)
			if err != nil {
				t.Fatalf("NewProvenanceSigningInput() error = %v", err)
			}
			version := signing.Predicate.RunDetails.Builder.Version
			if version["nodejs"] != "v24.0.0" {
				t.Fatalf("builder.version = %#v", version)
			}
			_, hasCorepack := version["corepack"]
			if hasCorepack != (manager != ManagerNPM) {
				t.Fatalf("corepack presence = %t for %s", hasCorepack, manager)
			}
			dependencies := signing.Predicate.RunDetails.Builder.BuilderDependencies
			if len(dependencies) != 1 ||
				dependencies[0].URI != "pkg:golang/github.com/sigstore/sigstore-go@v1.3.0" ||
				dependencies[0].Digest["h1"] != "hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE=" ||
				dependencies[0].Annotations["role"] != "signing-adapter" {
				t.Fatalf("builderDependencies = %#v", dependencies)
			}
		})
	}
}

func TestReleaseRefEquality(t *testing.T) {
	t.Parallel()

	if err := ValidateReleaseRefEquality("refs/tags/v1.2.3", "refs/tags/v1.2.3", "refs/tags/v1.2.3", "v1.2.3", testSourceSHA, testSourceSHA); err != nil {
		t.Fatalf("ValidateReleaseRefEquality() error = %v", err)
	}
	for _, test := range []struct {
		name       string
		sourceRef  string
		releaseRef string
		runtimeRef string
		versionTag string
		peel       string
	}{
		{name: "short tag", sourceRef: "v1.2.3", releaseRef: "refs/tags/v1.2.3", runtimeRef: "refs/tags/v1.2.3", versionTag: "v1.2.3", peel: testSourceSHA},
		{name: "branch", sourceRef: "refs/heads/main", releaseRef: "refs/heads/main", runtimeRef: "refs/heads/main", versionTag: "main", peel: testSourceSHA},
		{name: "different runtime ref", sourceRef: "refs/tags/v1.2.3", releaseRef: "refs/tags/v1.2.3", runtimeRef: "refs/tags/v1.2.4", versionTag: "v1.2.3", peel: testSourceSHA},
		{name: "version tag mismatch", sourceRef: "refs/tags/v1.2.3", releaseRef: "refs/tags/v1.2.3", runtimeRef: "refs/tags/v1.2.3", versionTag: "v1.2.4", peel: testSourceSHA},
		{name: "annotated tag peel mismatch", sourceRef: "refs/tags/v1.2.3", releaseRef: "refs/tags/v1.2.3", runtimeRef: "refs/tags/v1.2.3", versionTag: "v1.2.3", peel: testAttestSHA},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireNPMDiagnostic(t, ValidateReleaseRefEquality(test.sourceRef, test.releaseRef, test.runtimeRef, test.versionTag, testSourceSHA, test.peel), IDReleaseRefMismatch)
		})
	}
}

func TestNPMProvenanceInputSourceRefDispatchRetry(t *testing.T) {
	t.Parallel()

	input := validProvenanceInput(t, ManagerNPM)
	parameters, err := DecodeExternalParameters(input.BuildMetadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	parameters.Source.EventName = "workflow_dispatch"
	parameters.Source.InputRef = testStringPointer("refs/tags/v1.2.3")
	parameters.Source.InvocationRef = testStringPointer("refs/heads/main")
	parameters.Source.InvocationRevision = testStringPointer(testAttestSHA)
	encoded, err := EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatal(err)
	}
	input.BuildMetadata.ExternalParameters = encoded
	signing, err := NewProvenanceSigningInput(input)
	if err != nil {
		t.Fatalf("NewProvenanceSigningInput() error = %v", err)
	}
	goldenPath := filepath.Join("..", "..", "testdata", "npm", "provenance", "npm-predicate-source-ref-dispatch-retry.jcs.json")
	if os.Getenv("UPDATE_P01_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, signing.PredicateJSON, 0o600); err != nil {
			t.Fatalf("write generated golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden predicate: %v", err)
	}
	if !bytes.Equal(signing.PredicateJSON, want) {
		t.Fatalf("dispatch-retry predicate differs from JCS golden\n got: %s\nwant: %s", signing.PredicateJSON, want)
	}
	if err := ValidateNPMStatement(signing.Statement(), input); err != nil {
		t.Fatalf("ValidateNPMStatement() dispatch retry error = %v", err)
	}
}

func TestSourceInvocationRecordValidation(t *testing.T) {
	t.Parallel()

	dispatchRetry := func() ExternalParameters {
		parameters := validExternalParameters(ManagerNPM)
		parameters.Source.InputRef = testStringPointer("refs/tags/v1.2.3")
		parameters.Source.InvocationRef = testStringPointer("refs/heads/main")
		parameters.Source.InvocationRevision = testStringPointer(testAttestSHA)
		return parameters
	}
	if err := validateExternalParameters(dispatchRetry(), ""); err != nil {
		t.Fatalf("validateExternalParameters() dispatch retry error = %v", err)
	}
	if err := validateExternalParameters(validExternalParameters(ManagerNPM), ""); err != nil {
		t.Fatalf("validateExternalParameters() single-identity error = %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*ExternalParameters)
		want   string
	}{
		{name: "input_ref unequal to built ref", want: IDSourceRefInvalid, mutate: func(parameters *ExternalParameters) {
			parameters.Source.InputRef = testStringPointer("refs/tags/v9.9.9")
		}},
		{name: "input_ref not a full tag ref", want: IDSourceRefInvalid, mutate: func(parameters *ExternalParameters) {
			parameters.Source.InputRef = testStringPointer("v1.2.3")
		}},
		{name: "input_ref without invocation record", want: IDUnexpectedExternalParameters, mutate: func(parameters *ExternalParameters) {
			parameters.Source.InvocationRef = nil
			parameters.Source.InvocationRevision = nil
		}},
		{name: "invocation revision malformed", want: IDUnexpectedExternalParameters, mutate: func(parameters *ExternalParameters) {
			parameters.Source.InvocationRevision = testStringPointer("main")
		}},
		{name: "invocation ref not full", want: IDUnexpectedExternalParameters, mutate: func(parameters *ExternalParameters) {
			parameters.Source.InvocationRef = testStringPointer("main")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parameters := dispatchRetry()
			test.mutate(&parameters)
			requireNPMDiagnostic(t, validateExternalParameters(parameters, ""), test.want)
		})
	}
	t.Run("invocation record without input_ref", func(t *testing.T) {
		parameters := validExternalParameters(ManagerNPM)
		parameters.Source.InvocationRef = testStringPointer("refs/tags/v1.2.3")
		parameters.Source.InvocationRevision = testStringPointer(testSourceSHA)
		requireNPMDiagnostic(t, validateExternalParameters(parameters, ""), IDUnexpectedExternalParameters)
	})
}

func validProvenanceInput(t *testing.T, manager Manager) NPMProvenanceInput {
	t.Helper()
	parameters := validExternalParameters(manager)
	encoded, err := EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatalf("EncodeExternalParameters() error = %v", err)
	}
	dependencies := validDependencies(manager, parameters)
	corepackVersion := (*string)(nil)
	if manager != ManagerNPM {
		value := "0.34.0"
		corepackVersion = &value
	}
	builderID := parameters.Workflow.BuilderID
	return NPMProvenanceInput{
		BuildMetadata: BuildMetadata{
			SchemaVersion: "1",
			PrimaryArtifact: PrimaryArtifact{
				ArtifactName:    "js-ts-npm-package-tarball-123456789-1",
				PayloadFileName: "windlass-slsa-builder-1.2.3.tgz",
				SHA256:          testSHA256,
				SHA512:          testSHA512,
			},
			ExternalParameters:   encoded,
			ResolvedDependencies: dependencies,
		},
		BuilderID:             builderID,
		NodeJSVersion:         "v24.0.0",
		CorepackVersion:       corepackVersion,
		InvocationID:          "https://github.com/example/project/actions/runs/123456789/attempts/1",
		StartedOn:             "2026-08-06T12:00:00Z",
		FinishedOn:            "2026-08-06T12:00:03Z",
		RuntimeReleaseRef:     "refs/tags/v1.2.3",
		PeeledReleaseRevision: testSourceSHA,
	}
}

func validExternalParameters(manager Manager) ExternalParameters {
	builderID := "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@" + testSourceSHA
	selectionManifest := testStringPointer("package.json")
	packageManagerVersion := "10.14.0"
	if manager == ManagerNPM {
		packageManagerVersion = "11.5.1"
	}
	if manager == ManagerYarn {
		packageManagerVersion = "4.9.2"
	}
	parameters := ExternalParameters{
		Source:         SourceParameters{Repository: "https://github.com/example/project", Ref: "refs/tags/v1.2.3", Revision: testSourceSHA, EventName: "push", RefType: "tag"},
		Workflow:       WorkflowParameters{Path: NPMWorkflowPath, SHA: testSourceSHA, BuilderID: builderID},
		Runtime:        RuntimeParameters{Runner: "ubuntu-24.04", NodeVersion: "24.0.0", NPMVersion: "11.5.1"},
		Package:        PackageParameters{Directory: ".", WorkspaceRoot: nil, SourceManifest: "package.json", Name: "@windlass/slsa-builder", Version: "1.2.3", Private: false, Repository: "https://github.com/example/project", TarballName: "windlass-slsa-builder-1.2.3.tgz", PackageURL: "https://registry.npmjs.org/%40windlass%2Fslsa-builder/1.2.3", PackedName: "@windlass/slsa-builder", PackedVersion: "1.2.3"},
		PackageManager: PackageManagerParameters{Name: manager, Version: packageManagerVersion, SelectionSource: SelectionPackageManager, SelectionManifest: selectionManifest, SelectionManifestPath: selectionManifest, SelectionLockfilePath: nil, Root: "."},
		Publish:        PublishParameters{ResolvedRegistryURL: "https://registry.npmjs.org/", ResolvedDistTag: "latest", EffectiveAccess: "existing-package-access", TrustedPublishing: true, ProvenanceFile: true, PackageIdentityPreexisting: boolPointer(true), PackageVersionPreexisting: boolPointer(false)},
		Release:        ReleaseParameters{Ref: "refs/tags/v1.2.3", VersionTag: "v1.2.3"},
		Distribution:   DistributionParameters{ReleaseAssetMode: false, ReleaseTagSupplied: false, ProvenanceSidecar: nil, LinkedArtifactMetadata: false},
		Caller:         CallerParameters{WorkflowFilename: "release.yml"},
		Build:          BuildParameters{ScriptPresent: true, ScriptResult: BuildScriptExecuted},
	}
	if manager == ManagerYarn {
		parameters.PackageManager.YarnInstallMode = "immutable"
	}
	return parameters
}

func validDependencies(manager Manager, parameters ExternalParameters) []provenance.ResourceDescriptor {
	lockfile := "package-lock.json"
	if manager == ManagerPNPM {
		lockfile = "pnpm-lock.yaml"
	}
	if manager == ManagerYarn {
		lockfile = "yarn.lock"
	}
	dependencies := []provenance.ResourceDescriptor{{
		Name:   "lockfile",
		URI:    "git+https://github.com/example/project@" + testSourceSHA + "#" + lockfile,
		Digest: map[string]string{"sha256": testSHA256},
		Annotations: map[string]json.RawMessage{
			"package_manager":              mustJSON(manager),
			"package_manager_root":         mustJSON("."),
			"selection_source":             mustJSON(SelectionPackageManager),
			"selection_manifest_path":      mustJSON("package.json"),
			"selection_lockfile_path":      mustJSON(lockfile),
			"stale_non_selected_lockfiles": mustJSON([]string{}),
		},
	}}
	if manager != ManagerNPM {
		authority := "registry-integrity"
		uri := "https://registry.npmjs.org/pnpm/-/pnpm-10.14.0.tgz"
		if manager == ManagerYarn {
			authority = "download-hash"
			uri = "https://repo.yarnpkg.com/4.9.2/packages/yarnpkg-cli/bin/yarn.js"
		}
		dependencies = append(dependencies, provenance.ResourceDescriptor{
			Name: "package-manager-distribution", URI: uri, Digest: map[string]string{"sha512": testSHA512},
			Annotations: map[string]json.RawMessage{"digest_authority": mustJSON(authority), "package_manager": mustJSON(manager), "package_manager_version": mustJSON(parameters.PackageManager.Version), "acquisition_source": mustJSON("corepack")},
		})
	}
	dependencies = append(dependencies, provenance.ResourceDescriptor{
		Name: "runner-image", URI: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md",
		Annotations: map[string]json.RawMessage{"image_os": mustJSON("ubuntu24"), "image_version": mustJSON("20260801.1.0"), "node_version": mustJSON("v24.0.0")},
	})
	return dependencies
}

func newProvenanceInputError(input NPMProvenanceInput) error {
	_, err := NewProvenanceSigningInput(input)
	return err
}

func requireNPMDiagnostic(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected diagnostic %q, got nil", want)
	}
	var identified interface{ DiagnosticID() string }
	if !errors.As(err, &identified) {
		t.Fatalf("error %T does not expose a diagnostic ID: %v", err, err)
	}
	if identified.DiagnosticID() != want {
		t.Fatalf("diagnostic ID = %q, want %q: %v", identified.DiagnosticID(), want, err)
	}
}

func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func boolPointer(value bool) *bool { return &value }

func testStringPointer(value string) *string { return &value }

func addJSONMember(t *testing.T, encoded json.RawMessage, name string, value any) json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	object[name] = mustJSON(value)
	mutated, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return mutated
}

func removeNestedJSONMember(t *testing.T, encoded json.RawMessage, objectName, memberName string) json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(object[objectName], &nested); err != nil {
		t.Fatal(err)
	}
	delete(nested, memberName)
	object[objectName] = mustJSON(nested)
	mutated, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return mutated
}

func cloneSubjects(subjects []provenance.Subject) []provenance.Subject {
	cloned := make([]provenance.Subject, len(subjects))
	for index, subject := range subjects {
		cloned[index] = provenance.Subject{Name: subject.Name, Digest: make(map[string]string, len(subject.Digest))}
		for name, value := range subject.Digest {
			cloned[index].Digest[name] = value
		}
	}
	return cloned
}

func cloneDependencies(values []provenance.ResourceDescriptor) []provenance.ResourceDescriptor {
	cloned := make([]provenance.ResourceDescriptor, len(values))
	for index, value := range values {
		cloned[index] = provenance.ResourceDescriptor{Name: value.Name, URI: value.URI, Digest: make(map[string]string, len(value.Digest)), Annotations: make(map[string]json.RawMessage, len(value.Annotations))}
		for name, digestValue := range value.Digest {
			cloned[index].Digest[name] = digestValue
		}
		for name, annotation := range value.Annotations {
			cloned[index].Annotations[name] = append(json.RawMessage(nil), annotation...)
		}
	}
	return cloned
}
