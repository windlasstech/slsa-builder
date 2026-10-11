package npmprofile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestClassifiedHelperErrors(t *testing.T) {
	t.Parallel()
	err := digestMismatch(ManagerPNPM, "sha512", "disagrees")
	requireNPMDiagnostic(t, err, IDPackageManagerDigestMismatch)
	if err.Error() != "pnpm distribution sha512 digest disagrees" {
		t.Fatalf("digest error = %q", err.Error())
	}
	boundary := &commandBoundaryError{cause: context.Canceled}
	wrapped := fmt.Errorf("execute: %w", boundary)
	requireNPMDiagnostic(t, wrapped, idTrustedCoreBoundaryViolation)
	var got *commandBoundaryError
	if !errors.Is(wrapped, context.Canceled) || !errors.As(wrapped, &got) || got != boundary {
		t.Fatalf("boundary error chain = %v", wrapped)
	}
}

func TestDependencyMismatch(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, id, field string }{
		{"lockfile", IDResolvedDependenciesLockfile, "resolvedDependencies.lockfile"},
		{"package-manager-distribution", IDResolvedDependenciesDistribution, "resolvedDependencies.package-manager-distribution"},
		{"runner-image", IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image"},
		{"unknown", IDResolvedDependenciesUnexpectedEntry, "resolvedDependencies"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := dependencyMismatch(test.name)
			requireNPMDiagnostic(t, err, test.id)
			var classified *npmProvenanceValidationError
			if !errors.As(err, &classified) || classified.diagnostic.Field != test.field {
				t.Fatalf("dependencyMismatch() = %v, want field %q", err, test.field)
			}
		})
	}
}

func TestDigestEncodingInvalid(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value   string
		length  int
		invalid bool
	}{
		{"0123456789abcdef", 16, false}, {"abc", 4, true}, {"ab/c", 4, true},
		{"ab:c", 4, true}, {"abAc", 4, true}, {"abgc", 4, true},
	} {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()
			if got := digestEncodingInvalid(test.value, test.length); got != test.invalid {
				t.Fatalf("digestEncodingInvalid(%q, %d) = %t, want %t", test.value, test.length, got, test.invalid)
			}
		})
	}
}

func TestProvenanceCollectionHelpers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		parameters PackageManagerParameters
		want       string
	}{
		{"explicit wins", PackageManagerParameters{Name: ManagerNPM, Root: "other", SelectionLockfilePath: testStringPointer("explicit/npm-shrinkwrap.json")}, "explicit/npm-shrinkwrap.json"},
		{"npm root", PackageManagerParameters{Name: ManagerNPM, Root: "."}, "package-lock.json"},
		{"pnpm nested", PackageManagerParameters{Name: ManagerPNPM, Root: "packages"}, "packages/pnpm-lock.yaml"},
		{"yarn nested", PackageManagerParameters{Name: ManagerYarn, Root: "packages"}, "packages/yarn.lock"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := selectedLockfilePath(test.parameters); got != test.want {
				t.Fatalf("selectedLockfilePath() = %q, want %q", got, test.want)
			}
		})
	}
	if got := nonNilStrings(nil); got == nil || len(got) != 0 {
		t.Fatalf("nonNilStrings(nil) = %#v, want non-nil empty slice", got)
	}
	values := []string{"yarn.lock"}
	if got := nonNilStrings(values); !reflect.DeepEqual(got, values) {
		t.Fatalf("nonNilStrings() = %v, want %v", got, values)
	}
}

func TestEncodeExternalParametersMalformedRawJSON(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{`, `{"duplicate":1,"duplicate":2}`} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			parameters := validExternalParameters(ManagerNPM)
			parameters.Package.ConsumerSurface = map[string]json.RawMessage{"exports": json.RawMessage(raw)}
			encoded, err := EncodeExternalParameters(parameters)
			requireNPMDiagnostic(t, err, IDUnexpectedExternalParameters)
			if encoded != nil {
				t.Fatalf("rejected parameters returned bytes: %s", encoded)
			}
		})
	}
}

func TestValidateInputBindings(t *testing.T) {
	t.Parallel()
	type bindings struct {
		input      NPMProvenanceInput
		parameters ExternalParameters
	}
	for _, test := range []struct {
		name   string
		mutate func(*bindings)
		id     string
	}{
		{"valid", func(*bindings) {}, ""},
		{"schema", func(b *bindings) { b.input.BuildMetadata.SchemaVersion = "2" }, IDUnexpectedExternalParameters},
		{"artifact name", func(b *bindings) { b.input.BuildMetadata.PrimaryArtifact.ArtifactName = "" }, IDNPMSubjectMismatch},
		{"payload empty", func(b *bindings) {
			b.input.BuildMetadata.PrimaryArtifact.PayloadFileName = ""
		}, IDNPMSubjectMismatch},
		{"payload path", func(b *bindings) {
			b.input.BuildMetadata.PrimaryArtifact.PayloadFileName = "nested/pkg.tgz"
		}, IDNPMSubjectMismatch},
		{"payload suffix", func(b *bindings) {
			b.input.BuildMetadata.PrimaryArtifact.PayloadFileName = "pkg.zip"
		}, IDNPMSubjectMismatch},
		{"payload mismatch", func(b *bindings) {
			b.input.BuildMetadata.PrimaryArtifact.PayloadFileName = "other.tgz"
		}, IDNPMSubjectMismatch},
		{"builder mismatch", func(b *bindings) { b.input.BuilderID = "other" }, "windlass.verify.error.builder-id-not-immutable"},
		{"builder invalid", func(b *bindings) {
			b.input.BuilderID = "invalid"
			b.parameters.Workflow.BuilderID = "invalid"
		}, "windlass.verify.error.builder-id-not-immutable"},
		{"node mismatch", func(b *bindings) { b.input.NodeJSVersion = "v22.0.0" }, "windlass.verify.error.builder-version-mismatch"},
		{"unexpected corepack", func(b *bindings) { b.input.CorepackVersion = testStringPointer("0.34.0") }, "windlass.verify.error.builder-version-mismatch"},
		{"missing corepack", func(b *bindings) { b.parameters.PackageManager.Name = ManagerPNPM }, "windlass.verify.error.builder-version-mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := bindings{input: validProvenanceInput(t, ManagerNPM), parameters: validExternalParameters(ManagerNPM)}
			test.mutate(&value)
			err := validateInputBindings(value.input, value.parameters)
			if test.id != "" {
				requireNPMDiagnostic(t, err, test.id)
			} else if err != nil {
				t.Fatalf("valid bindings rejected: %v", err)
			}
		})
	}
}
