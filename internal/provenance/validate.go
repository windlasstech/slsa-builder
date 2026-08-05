package provenance

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/identity"
)

const (
	idStatementTypeInvalid           = "windlass.verify.error.statement-type-invalid"
	idPredicateTypeInvalid           = "windlass.verify.error.predicate-type-invalid"
	idUnexpectedInternalParameters   = "windlass.verify.error.unexpected-internal-parameters"
	idSubjectCardinalityInvalid      = "windlass.verify.error.subject-cardinality-invalid"
	idTimestampFormatInvalid         = "windlass.verify.error.timestamp-format-invalid"
	idTimestampOrderingInvalid       = "windlass.verify.error.timestamp-ordering-invalid"
	idBuilderVersionMismatch         = "windlass.verify.error.builder-version-mismatch"
	idBuilderDependenciesMismatch    = "windlass.verify.error.builder-dependencies-signing-adapter-mismatch"
	signingAdapterURIPrefix          = "git+https://github.com/actions/attest@"
	canonicalTimestampLayout         = "2006-01-02T15:04:05Z"
	maximumAcceptedNegativeClockSkew = 5 * time.Second
)

var (
	canonicalTimestampPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
	nodeVersionPattern        = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	componentVersionPattern   = regexp.MustCompile(`^(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

// BuilderExpectations are authenticated or locally observed common builder values.
type BuilderExpectations struct {
	NodeJSVersion     string
	CorepackVersion   *string
	SigningAdapterSHA string
}

// Expectations supplies common validation values that are outside the predicate itself.
type Expectations struct {
	SourceRepositoryURI string
	Builder             BuilderExpectations
}

// ProfileValidator closes the profile-owned semantic extension points without adding profile rules
// to the common package.
type ProfileValidator interface {
	ValidateSubject(Subject) error
	ValidateExternalParameters(json.RawMessage) error
	ValidateResolvedDependencies([]ResourceDescriptor) error
}

// ValidationError carries one stable diagnostic object for a failed common structural check.
type ValidationError struct {
	Diagnostic diagnostic.Diagnostic
}

func (err *ValidationError) Error() string {
	if err.Diagnostic.Field == "" {
		return err.Diagnostic.ID + ": " + err.Diagnostic.Message
	}
	return err.Diagnostic.ID + ": " + err.Diagnostic.Field + ": " + err.Diagnostic.Message
}

// DiagnosticID returns the stable ID consumed by the shared diagnostics report model.
func (err *ValidationError) DiagnosticID() string {
	return err.Diagnostic.ID
}

// Validate enforces the common structural contract, then invokes the profile semantic validator.
func (statement Statement) Validate(expectations Expectations, profile ProfileValidator) ([]diagnostic.Diagnostic, error) {
	if statement.Type != StatementType {
		return nil, validationError(idStatementTypeInvalid, "_type", "Statement type must be in-toto Statement v1")
	}
	if statement.PredicateType != PredicateType {
		return nil, validationError(idPredicateTypeInvalid, "predicateType", "predicate type must be SLSA provenance v1")
	}
	if len(statement.Subject) != 1 {
		return nil, validationError(idSubjectCardinalityInvalid, "subject", "Statement must contain exactly one subject")
	}
	if err := validateInternalParameters(statement.Predicate.BuildDefinition.InternalParameters); err != nil {
		return nil, err
	}
	if err := validateBuilder(statement.Predicate.RunDetails.Builder, expectations.Builder); err != nil {
		return nil, err
	}
	metadata := statement.Predicate.RunDetails.Metadata
	if _, err := identity.ParseRunInvocationURI(metadata.InvocationID, expectations.SourceRepositoryURI); err != nil {
		return nil, err
	}
	diagnostics, err := validateTimestamps(metadata)
	if err != nil {
		return nil, err
	}
	if profile != nil {
		if err := profile.ValidateSubject(cloneSubject(statement.Subject[0])); err != nil {
			return nil, err
		}
		if err := profile.ValidateExternalParameters(append(json.RawMessage(nil), statement.Predicate.BuildDefinition.ExternalParameters...)); err != nil {
			return nil, err
		}
		dependencies := cloneResourceDescriptors(statement.Predicate.BuildDefinition.ResolvedDependencies)
		if err := profile.ValidateResolvedDependencies(dependencies); err != nil {
			return nil, err
		}
	}
	return diagnostics, nil
}

func cloneSubject(subject Subject) Subject {
	return Subject{Name: subject.Name, Digest: cloneStringMap(subject.Digest)}
}

func cloneResourceDescriptors(descriptors []ResourceDescriptor) []ResourceDescriptor {
	cloned := make([]ResourceDescriptor, len(descriptors))
	for index, descriptor := range descriptors {
		annotations := make(map[string]json.RawMessage, len(descriptor.Annotations))
		for key, value := range descriptor.Annotations {
			annotations[key] = append(json.RawMessage(nil), value...)
		}
		cloned[index] = ResourceDescriptor{
			Name:        descriptor.Name,
			URI:         descriptor.URI,
			Digest:      cloneStringMap(descriptor.Digest),
			Annotations: annotations,
		}
	}
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func validateInternalParameters(parameters json.RawMessage) error {
	if len(parameters) == 0 {
		return validationError(idUnexpectedInternalParameters, "predicate.buildDefinition.internalParameters", "internalParameters must be exactly the empty object")
	}
	equal, err := canonicaljson.Equal(parameters, []byte(`{}`))
	if err != nil || !equal {
		return validationError(idUnexpectedInternalParameters, "predicate.buildDefinition.internalParameters", "internalParameters must be exactly the empty object")
	}
	return nil
}

func validateBuilder(builder Builder, expected BuilderExpectations) error {
	if err := validateBuilderVersion(builder.Version, expected); err != nil {
		return err
	}
	return validateBuilderDependencies(builder.BuilderDependencies, expected.SigningAdapterSHA)
}

func validateBuilderVersion(version map[string]string, expected BuilderExpectations) error {
	wantKeys := 1
	if expected.CorepackVersion != nil {
		wantKeys++
	}
	nodeVersion, hasNode := version["nodejs"]
	if len(version) != wantKeys || !hasNode || !nodeVersionPattern.MatchString(nodeVersion) ||
		!nodeVersionPattern.MatchString(expected.NodeJSVersion) || nodeVersion != expected.NodeJSVersion {
		return validationError(idBuilderVersionMismatch, "predicate.runDetails.builder.version", "builder.version must contain the exact observed nodejs and conditional corepack versions")
	}
	corepackVersion, hasCorepack := version["corepack"]
	if expected.CorepackVersion == nil {
		if hasCorepack {
			return validationError(idBuilderVersionMismatch, "predicate.runDetails.builder.version.corepack", "corepack must be absent when it did not supply the package manager")
		}
		return nil
	}
	if !hasCorepack || !componentVersionPattern.MatchString(corepackVersion) ||
		!componentVersionPattern.MatchString(*expected.CorepackVersion) || corepackVersion != *expected.CorepackVersion {
		return validationError(idBuilderVersionMismatch, "predicate.runDetails.builder.version.corepack", "corepack must equal the exact observed version")
	}
	return nil
}

func validateBuilderDependencies(dependencies []BuilderDependency, expectedSHA string) error {
	if err := identity.ValidateFullSHA(expectedSHA); err != nil {
		return validationError(idBuilderDependenciesMismatch, "expectations.builder.signingAdapterSHA", "expected signing adapter revision must be a full lowercase SHA")
	}
	if len(dependencies) != 1 {
		return validationError(idBuilderDependenciesMismatch, "predicate.runDetails.builder.builderDependencies", "builderDependencies must contain exactly one signing adapter")
	}
	dependency := dependencies[0]
	if dependency.URI != signingAdapterURIPrefix+expectedSHA || len(dependency.Digest) != 1 ||
		dependency.Digest["gitCommit"] != expectedSHA || len(dependency.Annotations) != 1 ||
		dependency.Annotations["role"] != "signing-adapter" {
		return validationError(idBuilderDependenciesMismatch, "predicate.runDetails.builder.builderDependencies[0]", "signing adapter URI, digest, role, and expected revision must match")
	}
	return nil
}

func validateTimestamps(metadata Metadata) ([]diagnostic.Diagnostic, error) {
	started, err := parseCanonicalTimestamp(metadata.StartedOn)
	if err != nil {
		return nil, validationError(idTimestampFormatInvalid, "predicate.runDetails.metadata.startedOn", "startedOn must use whole-second UTC RFC 3339")
	}
	finished, err := parseCanonicalTimestamp(metadata.FinishedOn)
	if err != nil {
		return nil, validationError(idTimestampFormatInvalid, "predicate.runDetails.metadata.finishedOn", "finishedOn must use whole-second UTC RFC 3339")
	}
	duration := finished.Sub(started)
	if duration < -maximumAcceptedNegativeClockSkew {
		return nil, validationError(idTimestampOrderingInvalid, "predicate.runDetails.metadata.finishedOn", "finishedOn precedes startedOn beyond the five-second tolerance")
	}
	if duration < 0 {
		warning, err := diagnostic.New(diagnostic.IDTimestampClockSkew, "predicate.runDetails.metadata", "A bounded timestamp clock skew was observed.")
		if err != nil {
			return nil, fmt.Errorf("construct timestamp warning: %w", err)
		}
		warning.Field = "predicate.runDetails.metadata.finishedOn"
		return []diagnostic.Diagnostic{warning}, nil
	}
	return nil, nil
}

func parseCanonicalTimestamp(value string) (time.Time, error) {
	if !canonicalTimestampPattern.MatchString(value) {
		return time.Time{}, fmt.Errorf("timestamp is not canonical whole-second UTC")
	}
	parsed, err := time.Parse(canonicalTimestampLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse canonical timestamp: %w", err)
	}
	return parsed, nil
}

func validationError(id, field, message string) error {
	entry, err := diagnostic.New(id, field, message)
	if err != nil {
		return fmt.Errorf("construct provenance diagnostic %q: %w", id, err)
	}
	entry.Field = field
	return &ValidationError{Diagnostic: entry}
}
