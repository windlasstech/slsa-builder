package policy_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/policy"
)

const expectedWorkflowSHA = "89abcdef0123456789abcdef0123456789abcdef"

func TestPolicyIntersection(t *testing.T) {
	t.Parallel()

	registry := producerRegistry{"js-ts-npm-package": {}}
	explicit, err := policy.DecodeExplicitPolicy(readPolicyFixture(t, "explicit-valid.json"))
	if err != nil {
		t.Fatalf("DecodeExplicitPolicy() error = %v", err)
	}
	manifest, err := policy.DecodeReleaseManifestExpectation(readPolicyFixture(t, "manifest-expectation-valid.json"), registry)
	if err != nil {
		t.Fatalf("DecodeReleaseManifestExpectation() error = %v", err)
	}
	effective, err := policy.Intersect(explicit.ProducerConstraints(), manifest.ProducerConstraints())
	if err != nil {
		t.Fatalf("Intersect() matching policies error = %v", err)
	}
	if !effective.Allows(policy.FieldProducerWorkflowSHA, expectedWorkflowSHA) {
		t.Fatalf("effective policy does not allow expected workflow SHA")
	}
	if !effective.Allows(policy.FieldProducerRunnerEnvironment, "github-hosted") {
		t.Fatal("effective policy omitted the explicit runner environment constraint")
	}
	if effective.Allows(policy.FieldSourceDigest, "ffffffffffffffffffffffffffffffffffffffff") {
		t.Fatal("effective policy accepted a source digest rejected by explicit policy")
	}
	manifestOnly, err := policy.Intersect(manifest.ProducerConstraints())
	if err != nil {
		t.Fatalf("Intersect() manifest-only policy error = %v", err)
	}
	if manifestOnly.Allows(policy.FieldProducerWorkflowSHA, "0123456789abcdef0123456789abcdef01234567") {
		t.Fatal("manifest-only policy accepted a mismatched producer workflow SHA")
	}

	if _, err := policy.DecodeExplicitPolicy(readPolicyFixture(t, "explicit-pinned-valid.json")); err != nil {
		t.Fatalf("DecodeExplicitPolicy() pinned root error = %v", err)
	}

	t.Run("closed schemas", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			fixture string
			decode  func([]byte) error
			wantID  string
		}{
			{
				name:    "explicit unknown member",
				fixture: "explicit-unknown-member.json",
				decode: func(data []byte) error {
					_, decodeErr := policy.DecodeExplicitPolicy(data)
					return decodeErr
				},
				wantID: "windlass.verify.error.policy-schema-invalid",
			},
			{
				name:    "manifest unregistered profile",
				fixture: "manifest-expectation-unregistered-profile.json",
				decode: func(data []byte) error {
					_, decodeErr := policy.DecodeReleaseManifestExpectation(data, registry)
					return decodeErr
				},
				wantID: "windlass.verify.error.policy-schema-invalid",
			},
			{
				name:    "duplicate member",
				fixture: "explicit-duplicate-member.json",
				decode: func(data []byte) error {
					_, decodeErr := policy.DecodeExplicitPolicy(data)
					return decodeErr
				},
				wantID: "windlass.verify.error.duplicate-json-member",
			},
			{
				name:    "TUF root present empty pinned member",
				fixture: "explicit-tuf-empty-pinned-member.json",
				decode: func(data []byte) error {
					_, decodeErr := policy.DecodeExplicitPolicy(data)
					return decodeErr
				},
				wantID: "windlass.verify.error.policy-schema-invalid",
			},
			{
				name:    "TUF root present null pinned member",
				fixture: "explicit-tuf-null-pinned-member.json",
				decode: func(data []byte) error {
					_, decodeErr := policy.DecodeExplicitPolicy(data)
					return decodeErr
				},
				wantID: "windlass.verify.error.policy-schema-invalid",
			},
			{
				name:    "explicit missing required field",
				fixture: "explicit-missing-source-ref.json",
				decode: func(data []byte) error {
					_, decodeErr := policy.DecodeExplicitPolicy(data)
					return decodeErr
				},
				wantID: "windlass.verify.error.policy-schema-invalid",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				requirePolicyDiagnosticID(t, test.decode(readPolicyFixture(t, test.fixture)), test.wantID)
			})
		}
	})

	t.Run("widening and empty intersections fail closed", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"intersection-union.json", "intersection-empty.json"} {
			fixture := loadIntersectionFixture(t, name)
			baseline, err := policy.NewConstraintSet(fixture.Baseline.Source, fixture.Baseline.Constraints...)
			if err != nil {
				t.Fatalf("NewConstraintSet() baseline error = %v", err)
			}
			narrowing, err := policy.NewConstraintSet(fixture.Narrowing.Source, fixture.Narrowing.Constraints...)
			if err != nil {
				t.Fatalf("NewConstraintSet() narrowing error = %v", err)
			}
			_, err = policy.Intersect(baseline, narrowing)
			requirePolicyDiagnosticID(t, err, fixture.ExpectedDiagnostic)
			var validationError *policy.ValidationError
			if !errors.As(err, &validationError) {
				t.Fatalf("intersection error = %T, want *policy.ValidationError", err)
			}
			if validationError.Diagnostic.Field != string(fixture.ExpectedField) {
				t.Fatalf("conflict field = %q, want %q", validationError.Diagnostic.Field, fixture.ExpectedField)
			}
			if !slices.Equal(validationError.Diagnostic.PolicySources, fixture.ExpectedSources) {
				t.Fatalf("conflict sources = %#v, want %#v", validationError.Diagnostic.PolicySources, fixture.ExpectedSources)
			}
		}
	})

	t.Run("a strict subset narrows the baseline", func(t *testing.T) {
		t.Parallel()
		fixture := loadIntersectionFixture(t, "intersection-narrowing.json")
		baseline, err := policy.NewConstraintSet(fixture.Baseline.Source, fixture.Baseline.Constraints...)
		if err != nil {
			t.Fatalf("NewConstraintSet() baseline error = %v", err)
		}
		narrowing, err := policy.NewConstraintSet(fixture.Narrowing.Source, fixture.Narrowing.Constraints...)
		if err != nil {
			t.Fatalf("NewConstraintSet() narrowing error = %v", err)
		}
		effective, err := policy.Intersect(baseline, narrowing)
		if err != nil {
			t.Fatalf("Intersect() narrowing error = %v", err)
		}
		allowed := effective.Allowed(policy.FieldProducerWorkflowSHA)
		if len(allowed) != 1 || allowed[0] != expectedWorkflowSHA {
			t.Fatalf("narrowed values = %#v, want only %q", allowed, expectedWorkflowSHA)
		}
	})

	t.Run("single-source observed mismatches remain denied", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"intersection-explicit-only-mismatch.json", "intersection-manifest-only-mismatch.json"} {
			fixture := loadObservationFixture(t, name)
			constraints, err := policy.NewConstraintSet(fixture.Policy.Source, fixture.Policy.Constraints...)
			if err != nil {
				t.Fatalf("NewConstraintSet() error = %v", err)
			}
			effective, err := policy.Intersect(constraints)
			if err != nil {
				t.Fatalf("Intersect() error = %v", err)
			}
			if effective.Allows(fixture.Field, fixture.Observed) {
				t.Fatalf("single-source policy allowed mismatched observed value %q", fixture.Observed)
			}
		}
	})
}

type producerRegistry map[string]struct{}

func (registry producerRegistry) IsRegisteredProducerProfile(profile string) bool {
	_, ok := registry[profile]
	return ok
}

type intersectionFixture struct {
	Baseline           constraintDocument        `json:"baseline"`
	Narrowing          constraintDocument        `json:"narrowing"`
	ExpectedDiagnostic string                    `json:"expected_diagnostic"`
	ExpectedField      policy.Field              `json:"expected_field"`
	ExpectedSources    []diagnostic.PolicySource `json:"expected_sources"`
}

type constraintDocument struct {
	Source      diagnostic.PolicySource  `json:"source"`
	Constraints []policy.FieldConstraint `json:"constraints"`
}

type observationFixture struct {
	Policy   constraintDocument `json:"policy"`
	Field    policy.Field       `json:"field"`
	Observed string             `json:"observed"`
}

func loadIntersectionFixture(t *testing.T, name string) intersectionFixture {
	t.Helper()
	var fixture intersectionFixture
	if err := json.Unmarshal(readPolicyFixture(t, name), &fixture); err != nil {
		t.Fatalf("decode intersection fixture %q: %v", name, err)
	}
	return fixture
}

func loadObservationFixture(t *testing.T, name string) observationFixture {
	t.Helper()
	var fixture observationFixture
	if err := json.Unmarshal(readPolicyFixture(t, name), &fixture); err != nil {
		t.Fatalf("decode observation fixture %q: %v", name, err)
	}
	return fixture
}

func readPolicyFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "policy", name))
	if err != nil {
		t.Fatalf("read policy fixture %q: %v", name, err)
	}
	return data
}

func requirePolicyDiagnosticID(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected diagnostic %q, got nil", want)
	}
	var identified interface{ DiagnosticID() string }
	if !errors.As(err, &identified) {
		t.Fatalf("error %T does not expose a diagnostic ID: %v", err, err)
	}
	if got := identified.DiagnosticID(); got != want {
		t.Fatalf("diagnostic ID = %q, want %q: %v", got, want, err)
	}
}
