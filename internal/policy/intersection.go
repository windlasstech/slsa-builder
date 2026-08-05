package policy

import (
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

const idTrustedProducerPolicyConflict = "windlass.verify.error.trusted-producer-policy-conflict"

// Field is a verifier-visible producer constraint key.
type Field string

const (
	// FieldProducerProfile selects a registered producer profile.
	FieldProducerProfile Field = "producer.profile"
	// FieldProducerWorkflowPath constrains the producer workflow path.
	FieldProducerWorkflowPath Field = "producer.workflow_path"
	// FieldProducerWorkflowSHA constrains the immutable producer workflow revision.
	FieldProducerWorkflowSHA Field = "producer.workflow_sha"
	// FieldProducerRunnerEnvironment constrains the platform-signed runner class.
	FieldProducerRunnerEnvironment Field = "producer.runner_environment"
	// FieldProducerBuilderID constrains a producer builder identity.
	FieldProducerBuilderID Field = "producer.builder_id"
	// FieldProducerBuildType constrains a producer build type.
	FieldProducerBuildType Field = "producer.build_type"
	// FieldSourceRepositoryURI constrains the canonical source repository URI.
	FieldSourceRepositoryURI Field = "source.repository_uri"
	// FieldSourceRepositoryID constrains the authoritative source repository ID.
	FieldSourceRepositoryID Field = "source.repository_id"
	// FieldSourceRepositoryOwnerID constrains the authoritative source owner ID.
	FieldSourceRepositoryOwnerID Field = "source.repository_owner_id"
	// FieldSourceDigest constrains the immutable source revision.
	FieldSourceDigest Field = "source.digest"
	// FieldSourceRef constrains the full release ref.
	FieldSourceRef Field = "source.ref"
)

// FieldConstraint allows a finite set of values for one field.
type FieldConstraint struct {
	Field   Field    `json:"field"`
	Allowed []string `json:"allowed"`
}

// ConstraintSet is one authenticated policy source's finite constraints.
type ConstraintSet struct {
	source diagnostic.PolicySource
	values map[Field][]string
}

// EffectiveConstraints is the monotonic intersection of authenticated policy sources.
type EffectiveConstraints struct {
	values  map[Field][]string
	sources map[Field][]diagnostic.PolicySource
}

// NewConstraintSet validates and normalizes one authenticated policy source.
func NewConstraintSet(source diagnostic.PolicySource, constraints ...FieldConstraint) (ConstraintSet, error) {
	if !validPolicySource(source) {
		return ConstraintSet{}, policySchemaError("policy.source", "policy source %q is not registered", source)
	}
	values := make(map[Field][]string, len(constraints))
	for _, constraint := range constraints {
		if constraint.Field == "" || strings.TrimSpace(string(constraint.Field)) != string(constraint.Field) {
			return ConstraintSet{}, policySchemaError("policy.constraints.field", "constraint field must be non-empty and canonical")
		}
		if _, duplicate := values[constraint.Field]; duplicate {
			return ConstraintSet{}, policySchemaError(string(constraint.Field), "constraint field is duplicated")
		}
		normalized, err := normalizeAllowed(constraint.Allowed)
		if err != nil {
			return ConstraintSet{}, policySchemaError(string(constraint.Field), "%v", err)
		}
		values[constraint.Field] = normalized
	}
	return ConstraintSet{source: source, values: values}, nil
}

// ProducerConstraints returns the explicit policy fields that constrain a producer.
func (policy ExplicitPolicy) ProducerConstraints() ConstraintSet {
	return trustedConstraintSet(diagnostic.PolicySourceExplicitPolicy,
		FieldConstraint{Field: FieldProducerWorkflowPath, Allowed: []string{policy.Producer.WorkflowPath}},
		FieldConstraint{Field: FieldProducerWorkflowSHA, Allowed: []string{policy.Producer.WorkflowSHA}},
		FieldConstraint{Field: FieldProducerRunnerEnvironment, Allowed: []string{policy.Producer.RunnerEnvironment}},
		FieldConstraint{Field: FieldSourceRepositoryURI, Allowed: []string{policy.Source.RepositoryURI}},
		FieldConstraint{Field: FieldSourceRepositoryID, Allowed: []string{policy.Source.RepositoryID}},
		FieldConstraint{Field: FieldSourceRepositoryOwnerID, Allowed: []string{policy.Source.RepositoryOwnerID}},
		FieldConstraint{Field: FieldSourceDigest, Allowed: []string{policy.Source.Digest}},
		FieldConstraint{Field: FieldSourceRef, Allowed: []string{policy.Source.Ref}},
	)
}

// ProducerConstraints returns only the manifest-expectation fields represented for a producer.
func (expectation ReleaseManifestExpectation) ProducerConstraints() ConstraintSet {
	profile := expectation.ProducerProfile
	return trustedConstraintSet(diagnostic.PolicySourceReleaseManifest,
		FieldConstraint{Field: FieldProducerProfile, Allowed: []string{profile.Profile}},
		FieldConstraint{Field: FieldProducerWorkflowPath, Allowed: []string{profile.WorkflowPath}},
		FieldConstraint{Field: FieldProducerWorkflowSHA, Allowed: []string{profile.WorkflowSHA}},
	)
}

// Intersect applies each later source only when it narrows the current effective policy.
func Intersect(baseline ConstraintSet, narrowing ...ConstraintSet) (EffectiveConstraints, error) {
	if !validPolicySource(baseline.source) || baseline.values == nil {
		return EffectiveConstraints{}, policySchemaError("policy.baseline", "baseline constraint set is not initialized")
	}
	effective := EffectiveConstraints{
		values:  cloneValues(baseline.values),
		sources: make(map[Field][]diagnostic.PolicySource, len(baseline.values)),
	}
	for field := range baseline.values {
		effective.sources[field] = []diagnostic.PolicySource{baseline.source}
	}
	for _, candidate := range narrowing {
		if !validPolicySource(candidate.source) || candidate.values == nil {
			return EffectiveConstraints{}, policySchemaError("policy.narrowing", "narrowing constraint set is not initialized")
		}
		for field, candidateAllowed := range candidate.values {
			currentAllowed, constrained := effective.values[field]
			if !constrained {
				effective.values[field] = slices.Clone(candidateAllowed)
				effective.sources[field] = []diagnostic.PolicySource{candidate.source}
				continue
			}
			sources := append(slices.Clone(effective.sources[field]), candidate.source)
			if !isSubset(candidateAllowed, currentAllowed) {
				return EffectiveConstraints{}, newValidationError(
					idTrustedProducerPolicyConflict,
					string(field),
					sources,
					"policy source %q attempts to widen or conflicts with existing constraints",
					candidate.source,
				)
			}
			intersection := intersectValues(currentAllowed, candidateAllowed)
			if len(intersection) == 0 {
				return EffectiveConstraints{}, newValidationError(
					idTrustedProducerPolicyConflict,
					string(field),
					sources,
					"policy intersection is empty",
				)
			}
			effective.values[field] = intersection
			effective.sources[field] = sources
		}
	}
	return effective, nil
}

// Allowed returns a copy of the effective finite set for a field.
func (constraints EffectiveConstraints) Allowed(field Field) []string {
	return slices.Clone(constraints.values[field])
}

// Allows reports whether the effective policy allows one value for a constrained field.
func (constraints EffectiveConstraints) Allows(field Field, value string) bool {
	return slices.Contains(constraints.values[field], value)
}

func trustedConstraintSet(source diagnostic.PolicySource, constraints ...FieldConstraint) ConstraintSet {
	set, err := NewConstraintSet(source, constraints...)
	if err != nil {
		return ConstraintSet{}
	}
	return set
}

func normalizeAllowed(allowed []string) ([]string, error) {
	if len(allowed) == 0 {
		return nil, errors.New("constraint must allow at least one value")
	}
	normalized := slices.Clone(allowed)
	for _, value := range normalized {
		if value == "" {
			return nil, &constraintValueError{message: "constraint allowed values must be non-empty"}
		}
	}
	sort.Strings(normalized)
	for index := 1; index < len(normalized); index++ {
		if normalized[index] == normalized[index-1] {
			return nil, &constraintValueError{message: "constraint allowed values must be unique"}
		}
	}
	return normalized, nil
}

type constraintValueError struct {
	message string
}

func (err *constraintValueError) Error() string {
	return err.message
}

func validPolicySource(source diagnostic.PolicySource) bool {
	switch source {
	case diagnostic.PolicySourceExplicitPolicy,
		diagnostic.PolicySourceReleaseManifest,
		diagnostic.PolicySourceProducerExpectedValue,
		diagnostic.PolicySourceDigestVerifiedHandoff:
		return true
	default:
		return false
	}
}

func cloneValues(values map[Field][]string) map[Field][]string {
	clone := make(map[Field][]string, len(values))
	for field, allowed := range values {
		clone[field] = slices.Clone(allowed)
	}
	return clone
}

func isSubset(candidate, current []string) bool {
	for _, value := range candidate {
		if !slices.Contains(current, value) {
			return false
		}
	}
	return true
}

func intersectValues(left, right []string) []string {
	intersection := make([]string, 0, min(len(left), len(right)))
	for _, value := range left {
		if slices.Contains(right, value) {
			intersection = append(intersection, value)
		}
	}
	return intersection
}
