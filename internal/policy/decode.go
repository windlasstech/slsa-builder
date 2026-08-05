package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/identity"
)

const (
	idPolicySchemaInvalid      = "windlass.verify.error.policy-schema-invalid"
	sigstorePublicGoodInstance = "sigstore-public-good"
	sigstoreTUFRepository      = "https://tuf-repo-cdn.sigstore.dev"
	policyTimestampLayout      = "2006-01-02T15:04:05Z"
)

var (
	positiveDecimalPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
	policyTimestampPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
)

// ValidationError carries one stable diagnostic object for policy schema or intersection failure.
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

// DecodeExplicitPolicy strictly decodes and validates the closed explicit verifier policy schema.
func DecodeExplicitPolicy(data []byte) (ExplicitPolicy, error) {
	var policy ExplicitPolicy
	if err := decodeClosed(data, &policy); err != nil {
		return ExplicitPolicy{}, err
	}
	if err := rejectNullTrustRootMembers(data); err != nil {
		return ExplicitPolicy{}, err
	}
	if err := validateExplicitPolicy(policy); err != nil {
		return ExplicitPolicy{}, err
	}
	return policy, nil
}

func rejectNullTrustRootMembers(data []byte) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return policySchemaError("trust_root", "decode trust root member presence: %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(document["trust_root"], &members); err != nil {
		return nil
	}
	for _, field := range []string{"mode", "instance", "path", "sha256", "tuf_repository", "revalidated_at", "refresh_before"} {
		value, present := members[field]
		if present && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return policySchemaError("trust_root."+field, "trust root members must not be null")
		}
	}
	return nil
}

// DecodeReleaseManifestExpectation strictly decodes the closed manifest expectation schema.
func DecodeReleaseManifestExpectation(data []byte, registry ProducerProfileRegistry) (ReleaseManifestExpectation, error) {
	var expectation ReleaseManifestExpectation
	if err := decodeClosed(data, &expectation); err != nil {
		return ReleaseManifestExpectation{}, err
	}
	if err := validateReleaseManifestExpectation(expectation, registry); err != nil {
		return ReleaseManifestExpectation{}, err
	}
	return expectation, nil
}

func decodeClosed(data []byte, target any) error {
	if err := canonicaljson.Validate(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return policySchemaError("policy", "decode closed policy schema: %v", err)
	}
	return nil
}

func validateExplicitPolicy(policy ExplicitPolicy) error {
	if policy.SchemaVersion != "1" {
		return policySchemaError("schema_version", "schema_version must equal %q", "1")
	}
	if err := validateSourcePolicy(policy.Source); err != nil {
		return err
	}
	if err := validateWorkflow(policy.Producer.WorkflowPath, policy.Producer.WorkflowSHA, "producer"); err != nil {
		return err
	}
	if policy.Producer.RunnerEnvironment != "github-hosted" {
		return policySchemaError("producer.runner_environment", "runner_environment must equal %q", "github-hosted")
	}
	return validateTrustRoot(policy.TrustRoot)
}

func validateSourcePolicy(source SourcePolicy) error {
	if err := identity.ValidateCanonicalRepositoryURI(source.RepositoryURI); err != nil {
		return policySchemaError("source.repository_uri", "repository URI is not canonical")
	}
	if !positiveDecimalPattern.MatchString(source.RepositoryID) {
		return policySchemaError("source.repository_id", "repository ID must be a positive decimal string")
	}
	if !positiveDecimalPattern.MatchString(source.RepositoryOwnerID) {
		return policySchemaError("source.repository_owner_id", "repository owner ID must be a positive decimal string")
	}
	if err := identity.ValidateFullSHA(source.Digest); err != nil {
		return policySchemaError("source.digest", "source digest must be a full lowercase SHA")
	}
	if err := identity.ValidateReleaseRef(source.Ref); err != nil {
		return policySchemaError("source.ref", "source ref must be a full release tag ref")
	}
	return nil
}

func validateTrustRoot(root TrustRoot) error {
	if root.Instance != sigstorePublicGoodInstance {
		return policySchemaError("trust_root.instance", "trust root instance must equal %q", sigstorePublicGoodInstance)
	}
	switch root.Mode {
	case "tuf":
		if root.Path != nil || root.SHA256 != nil || root.TUFRepository != nil || root.RevalidatedAt != nil || root.RefreshBefore != nil {
			return policySchemaError("trust_root", "TUF trust root contains pinned-root members")
		}
		return nil
	case "pinned":
		return validatePinnedTrustRoot(root)
	default:
		return policySchemaError("trust_root.mode", "trust root mode must equal %q or %q", "tuf", "pinned")
	}
}

func validatePinnedTrustRoot(root TrustRoot) error {
	if root.Path == nil || *root.Path == "" {
		return policySchemaError("trust_root.path", "pinned trust root path is required")
	}
	if root.SHA256 == nil {
		return policySchemaError("trust_root.sha256", "pinned trust root SHA-256 is required")
	}
	if _, err := digest.ParseSHA256(*root.SHA256); err != nil {
		return policySchemaError("trust_root.sha256", "pinned trust root SHA-256 is invalid")
	}
	if root.TUFRepository == nil || *root.TUFRepository != sigstoreTUFRepository {
		return policySchemaError("trust_root.tuf_repository", "pinned trust root TUF repository is not governed")
	}
	if root.RevalidatedAt == nil {
		return policySchemaError("trust_root.revalidated_at", "revalidated_at is required")
	}
	revalidated, err := parsePolicyTimestamp(*root.RevalidatedAt)
	if err != nil {
		return policySchemaError("trust_root.revalidated_at", "revalidated_at must use whole-second UTC RFC 3339")
	}
	if root.RefreshBefore == nil {
		return policySchemaError("trust_root.refresh_before", "refresh_before is required")
	}
	refreshBefore, err := parsePolicyTimestamp(*root.RefreshBefore)
	if err != nil {
		return policySchemaError("trust_root.refresh_before", "refresh_before must use whole-second UTC RFC 3339")
	}
	if !refreshBefore.After(revalidated) {
		return policySchemaError("trust_root.refresh_before", "refresh_before must be later than revalidated_at")
	}
	return nil
}

func validateReleaseManifestExpectation(expectation ReleaseManifestExpectation, registry ProducerProfileRegistry) error {
	if expectation.SchemaVersion != "1" {
		return policySchemaError("schema_version", "schema_version must equal %q", "1")
	}
	manifest := expectation.ReleaseManifest
	if err := identity.ValidateCanonicalRepositoryURI(manifest.SourceRepositoryURI); err != nil {
		return policySchemaError("release_manifest.source_repository_uri", "source repository URI is not canonical")
	}
	if !positiveDecimalPattern.MatchString(manifest.SourceRepositoryID) {
		return policySchemaError("release_manifest.source_repository_id", "source repository ID must be a positive decimal string")
	}
	if !positiveDecimalPattern.MatchString(manifest.SourceRepositoryOwnerID) {
		return policySchemaError("release_manifest.source_repository_owner_id", "source owner ID must be a positive decimal string")
	}
	if err := validateWorkflow(manifest.WorkflowPath, manifest.WorkflowSHA, "release_manifest"); err != nil {
		return err
	}
	profile := expectation.ProducerProfile
	if registry == nil || profile.Profile == "" || !registry.IsRegisteredProducerProfile(profile.Profile) {
		return policySchemaError("producer_profile.profile", "producer profile is not registered")
	}
	return validateWorkflow(profile.WorkflowPath, profile.WorkflowSHA, "producer_profile")
}

func validateWorkflow(path, sha, fieldPrefix string) error {
	if _, err := identity.NewBuilderID(path, sha); err != nil {
		return policySchemaError(fieldPrefix, "workflow path or SHA is not canonical")
	}
	return nil
}

func parsePolicyTimestamp(value string) (time.Time, error) {
	if !policyTimestampPattern.MatchString(value) {
		return time.Time{}, fmt.Errorf("timestamp is not canonical whole-second UTC")
	}
	parsed, err := time.Parse(policyTimestampLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse policy timestamp: %w", err)
	}
	return parsed, nil
}

func policySchemaError(field, format string, arguments ...any) error {
	return newValidationError(idPolicySchemaInvalid, field, nil, format, arguments...)
}

func newValidationError(id, field string, sources []diagnostic.PolicySource, format string, arguments ...any) error {
	message := fmt.Sprintf(format, arguments...)
	entry, err := diagnostic.New(id, field, message)
	if err != nil {
		return fmt.Errorf("construct policy diagnostic %q: %w", id, err)
	}
	entry.Field = field
	entry.PolicySources = append([]diagnostic.PolicySource(nil), sources...)
	return &ValidationError{Diagnostic: entry}
}
