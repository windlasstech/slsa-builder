package policy_test

import (
	"slices"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/policy"
)

func TestConstraintSetRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		source      diagnostic.PolicySource
		constraints []policy.FieldConstraint
		wantField   string
		wantMessage string
	}{
		{
			name:   "unregistered policy source",
			source: diagnostic.PolicySource("unregistered"),
			constraints: []policy.FieldConstraint{
				{Field: policy.FieldProducerWorkflowSHA, Allowed: []string{expectedWorkflowSHA}},
			},
			wantField: "policy.source",
		},
		{
			name:   "empty constraint field",
			source: diagnostic.PolicySourceExplicitPolicy,
			constraints: []policy.FieldConstraint{
				{Field: "", Allowed: []string{"value"}},
			},
			wantField: "policy.constraints.field",
		},
		{
			name:   "whitespace-padded constraint field",
			source: diagnostic.PolicySourceExplicitPolicy,
			constraints: []policy.FieldConstraint{
				{Field: " producer.workflow_sha", Allowed: []string{"value"}},
			},
			wantField: "policy.constraints.field",
		},
		{
			name:   "duplicate constraint field",
			source: diagnostic.PolicySourceExplicitPolicy,
			constraints: []policy.FieldConstraint{
				{Field: policy.FieldProducerWorkflowSHA, Allowed: []string{expectedWorkflowSHA}},
				{Field: policy.FieldProducerWorkflowSHA, Allowed: []string{"0123456789abcdef0123456789abcdef01234567"}},
			},
			wantField: string(policy.FieldProducerWorkflowSHA),
		},
		{
			name:   "empty allowed list",
			source: diagnostic.PolicySourceExplicitPolicy,
			constraints: []policy.FieldConstraint{
				{Field: policy.FieldProducerWorkflowSHA, Allowed: nil},
			},
			wantField:   string(policy.FieldProducerWorkflowSHA),
			wantMessage: "constraint must allow at least one value",
		},
		{
			name:   "empty allowed value",
			source: diagnostic.PolicySourceExplicitPolicy,
			constraints: []policy.FieldConstraint{
				{Field: policy.FieldProducerWorkflowSHA, Allowed: []string{expectedWorkflowSHA, ""}},
			},
			wantField:   string(policy.FieldProducerWorkflowSHA),
			wantMessage: "constraint allowed values must be non-empty",
		},
		{
			name:   "duplicate allowed values",
			source: diagnostic.PolicySourceExplicitPolicy,
			constraints: []policy.FieldConstraint{
				{Field: policy.FieldProducerWorkflowSHA, Allowed: []string{"b", "a", "b"}},
			},
			wantField:   string(policy.FieldProducerWorkflowSHA),
			wantMessage: "constraint allowed values must be unique",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := policy.NewConstraintSet(test.source, test.constraints...)
			requirePolicyDiagnostic(t, err, policySchemaInvalidID, test.wantField, test.wantMessage)
		})
	}
}

func TestConstraintSetAcceptsAllRegisteredSources(t *testing.T) {
	t.Parallel()

	sources := []diagnostic.PolicySource{
		diagnostic.PolicySourceExplicitPolicy,
		diagnostic.PolicySourceReleaseManifest,
		diagnostic.PolicySourceProducerExpectedValue,
		diagnostic.PolicySourceDigestVerifiedHandoff,
	}
	for _, source := range sources {
		t.Run(string(source), func(t *testing.T) {
			t.Parallel()
			_, err := policy.NewConstraintSet(source,
				policy.FieldConstraint{Field: policy.FieldProducerWorkflowSHA, Allowed: []string{expectedWorkflowSHA}},
			)
			if err != nil {
				t.Fatalf("NewConstraintSet() registered source %q error = %v", source, err)
			}
		})
	}
}

func TestConstraintSetSortsAllowedValues(t *testing.T) {
	t.Parallel()

	baseline, err := policy.NewConstraintSet(diagnostic.PolicySourceExplicitPolicy,
		policy.FieldConstraint{Field: policy.FieldSourceDigest, Allowed: []string{"digest-c", "digest-a", "digest-b"}},
	)
	if err != nil {
		t.Fatalf("NewConstraintSet() error = %v", err)
	}
	effective, err := policy.Intersect(baseline)
	if err != nil {
		t.Fatalf("Intersect() error = %v", err)
	}
	allowed := effective.Allowed(policy.FieldSourceDigest)
	want := []string{"digest-a", "digest-b", "digest-c"}
	if !slices.Equal(allowed, want) {
		t.Fatalf("Allowed() = %#v, want sorted %#v", allowed, want)
	}
}

func TestIntersectRejectsUninitializedConstraintSets(t *testing.T) {
	t.Parallel()

	initialized, err := policy.NewConstraintSet(diagnostic.PolicySourceExplicitPolicy,
		policy.FieldConstraint{Field: policy.FieldProducerWorkflowSHA, Allowed: []string{expectedWorkflowSHA}},
	)
	if err != nil {
		t.Fatalf("NewConstraintSet() error = %v", err)
	}

	t.Run("uninitialized baseline", func(t *testing.T) {
		t.Parallel()
		_, err := policy.Intersect(policy.ConstraintSet{}, initialized)
		requirePolicyDiagnostic(t, err, policySchemaInvalidID, "policy.baseline", "")
	})

	t.Run("uninitialized narrowing", func(t *testing.T) {
		t.Parallel()
		_, err := policy.Intersect(initialized, policy.ConstraintSet{})
		requirePolicyDiagnostic(t, err, policySchemaInvalidID, "policy.narrowing", "")
	})
}
