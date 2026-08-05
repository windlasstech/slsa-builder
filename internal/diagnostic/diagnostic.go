// Package diagnostic implements the closed Windlass diagnostics and report contract.
package diagnostic

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	jsoncanonicalizer "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

const (
	ExitCodePass              = 0
	ExitCodePolicyFailure     = 1
	ExitCodeInvocationFailure = 2
)

const (
	IDDiagnosticsContractInvalid          = "windlass.verify.error.diagnostics-contract-invalid"
	IDDigestMismatch                      = "windlass.verify.error.digest-mismatch"
	IDInputUnavailable                    = "windlass.verify.error.input-unavailable"
	IDManifestPartialJSONUploaded         = "windlass.verify.error.manifest-partial-json-uploaded"
	IDMutationPermissionDenied            = "windlass.verify.error.mutation-permission-denied"
	IDPublisherIndeterminatePrimaryUpload = "windlass.verify.error.publisher-indeterminate-primary-upload"
	IDReleaseTargetImmutable              = "windlass.verify.error.release-target-immutable"
	IDSignatureMismatch                   = "windlass.verify.error.signature-mismatch"
	IDSourceNumericIDMismatch             = "windlass.verify.error.source-numeric-id-mismatch"
	IDTrustedProducerPolicyConflict       = "windlass.verify.error.trusted-producer-policy-conflict"
	IDUnexpectedExternalParameters        = "windlass.verify.error.unexpected-external-parameters"
	IDVerifierExecutionFailure            = "windlass.verify.error.verifier-execution-failure"
	IDStaleNonSelectedLockfile            = "windlass.verify.warning.stale-non-selected-lockfile"
	IDTimestampClockSkew                  = "windlass.verify.warning.timestamp-clock-skew"
)

// Severity is the closed diagnostic severity vocabulary.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Phase identifies the latest processing phase required to emit a diagnostic.
type Phase string

const (
	PhaseInvocation   Phase = "invocation"
	PhasePolicy       Phase = "policy"
	PhaseVerification Phase = "verification"
	PhasePreMutation  Phase = "pre-mutation"
	PhaseMutation     Phase = "mutation"
)

// Result is the closed report result vocabulary.
type Result string

const (
	ResultPass Result = "pass"
	ResultFail Result = "fail"
)

// PolicySource identifies a source that constrained a diagnostic field.
type PolicySource string

const (
	PolicySourceExplicitPolicy        PolicySource = "explicit-policy"
	PolicySourceReleaseManifest       PolicySource = "release-manifest"
	PolicySourceProducerExpectedValue PolicySource = "producer-expected-value"
	PolicySourceDigestVerifiedHandoff PolicySource = "digest-verified-handoff"
)

// Definition is the immutable registry metadata for one diagnostic ID.
type Definition struct {
	ID               string
	Severity         Severity
	Category         string
	Phase            Phase
	ExitCode         int
	MutationPossible bool
	precedence       int
}

// Evidence contains only non-secret scalar local identifiers.
type Evidence map[string]any

// Diagnostic is the closed machine-readable diagnostic object.
type Diagnostic struct {
	ID            string         `json:"id"`
	Severity      Severity       `json:"severity"`
	Category      string         `json:"category"`
	Check         string         `json:"check"`
	Message       string         `json:"message"`
	Field         string         `json:"field,omitempty"`
	Expected      *any           `json:"expected,omitempty"`
	Actual        *any           `json:"actual,omitempty"`
	PolicySources []PolicySource `json:"policy_sources,omitempty"`
	Evidence      Evidence       `json:"evidence,omitempty"`
}

// DiagnosticMetadata is the closed optional non-trust report metadata extension.
type DiagnosticMetadata struct {
	PackageManifest *PackageManifest `json:"package_manifest,omitempty"`
}

// PackageManifest preserves safe-listed raw package.json diagnostic metadata.
type PackageManifest struct {
	Repository  any      `json:"repository,omitempty"`
	License     any      `json:"license,omitempty"`
	Description string   `json:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Author      any      `json:"author,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
}

// Report is the closed schema-version-1 machine-readable report.
type Report struct {
	SchemaVersion      string              `json:"schema_version"`
	Result             Result              `json:"result"`
	ExitCode           int                 `json:"exit_code"`
	PrimaryID          *string             `json:"primary_id"`
	RunInvocation      *string             `json:"run_invocation"`
	Diagnostics        []Diagnostic        `json:"diagnostics"`
	DiagnosticMetadata *DiagnosticMetadata `json:"diagnostic_metadata,omitempty"`
}

// ContractError reports a diagnostics-contract-invalid rejection.
type ContractError struct {
	ID     string
	Reason string
}

func (e *ContractError) Error() string {
	return e.ID + ": " + e.Reason
}

// JSONValue creates an optional typed JSON value. Passing nil represents an explicit JSON null.
func JSONValue(value any) *any {
	return &value
}

// Lookup returns immutable metadata only for a registered diagnostic ID.
func Lookup(id string) (Definition, bool) {
	definition, ok := registry[id]
	return definition, ok
}

// RegisteredIDs returns the closed registry in lexical order.
func RegisteredIDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// New constructs a diagnostic whose identity fields agree with the closed registry.
func New(id, check, message string) (Diagnostic, error) {
	definition, ok := Lookup(id)
	if !ok {
		return Diagnostic{}, contractErrorf("unknown diagnostic ID %q", id)
	}
	diagnostic := Diagnostic{
		ID:       definition.ID,
		Severity: definition.Severity,
		Category: definition.Category,
		Check:    check,
		Message:  message,
	}
	if err := validateDiagnostic(diagnostic); err != nil {
		return Diagnostic{}, err
	}
	return diagnostic, nil
}

// Build validates, orders, and derives the result, primary ID, and exit code.
func Build(runInvocation *string, diagnostics []Diagnostic, metadata *DiagnosticMetadata) (Report, error) {
	ordered := append([]Diagnostic{}, diagnostics...)
	for _, diagnostic := range ordered {
		if err := validateDiagnostic(diagnostic); err != nil {
			return Report{}, err
		}
	}
	if err := validateMetadata(metadata); err != nil {
		return Report{}, err
	}
	if runInvocation != nil && *runInvocation == "" {
		return Report{}, contractErrorf("run_invocation must be non-empty when present")
	}
	sort.Slice(ordered, func(i, j int) bool { return diagnosticLess(ordered[i], ordered[j]) })

	report := Report{
		SchemaVersion:      "1",
		Result:             ResultPass,
		ExitCode:           ExitCodePass,
		RunInvocation:      cloneString(runInvocation),
		Diagnostics:        ordered,
		DiagnosticMetadata: metadata,
	}
	for i := range ordered {
		definition := registry[ordered[i].ID]
		if definition.Severity != SeverityError {
			continue
		}
		report.Result = ResultFail
		report.PrimaryID = stringPointer(ordered[i].ID)
		report.ExitCode = definition.ExitCode
		break
	}
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	return report, nil
}

// Validate checks the closed report schema and all derived report fields.
func (r Report) Validate() error {
	if r.SchemaVersion != "1" {
		return contractErrorf("schema_version must equal %q", "1")
	}
	if r.Diagnostics == nil {
		return contractErrorf("diagnostics must be a non-null array")
	}
	if r.RunInvocation != nil && *r.RunInvocation == "" {
		return contractErrorf("run_invocation must be non-empty when present")
	}
	for i, diagnostic := range r.Diagnostics {
		if err := validateDiagnostic(diagnostic); err != nil {
			return fmt.Errorf("diagnostics[%d]: %w", i, err)
		}
		if i > 0 && diagnosticLess(diagnostic, r.Diagnostics[i-1]) {
			return contractErrorf("diagnostics are not in deterministic precedence order")
		}
	}
	if err := validateMetadata(r.DiagnosticMetadata); err != nil {
		return err
	}

	wantResult := ResultPass
	wantExitCode := ExitCodePass
	var wantPrimary *string
	for i := range r.Diagnostics {
		definition := registry[r.Diagnostics[i].ID]
		if definition.Severity == SeverityError {
			wantResult = ResultFail
			wantExitCode = definition.ExitCode
			wantPrimary = stringPointer(r.Diagnostics[i].ID)
			break
		}
	}
	if r.Result != wantResult || r.ExitCode != wantExitCode || !equalStringPointers(r.PrimaryID, wantPrimary) {
		return contractErrorf("result, exit_code, and primary_id do not match the first ordered error")
	}
	return nil
}

// CanonicalJSON serializes a validated report as RFC 8785 JCS bytes without a trailing newline.
func (r Report) CanonicalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		return nil, contractErrorf("report is not valid JSON: %v", err)
	}
	canonical, err := jsoncanonicalizer.Transform(encoded)
	if err != nil {
		return nil, contractErrorf("report cannot be JCS canonicalized: %v", err)
	}
	return canonical, nil
}

func validateDiagnostic(diagnostic Diagnostic) error {
	definition, ok := Lookup(diagnostic.ID)
	if !ok {
		return contractErrorf("unknown diagnostic ID %q", diagnostic.ID)
	}
	if diagnostic.Severity != definition.Severity || diagnostic.Category != definition.Category {
		return contractErrorf("diagnostic %q severity or category disagrees with the registry", diagnostic.ID)
	}
	if diagnostic.Check == "" || diagnostic.Message == "" || !utf8.ValidString(diagnostic.Check) || !utf8.ValidString(diagnostic.Message) {
		return contractErrorf("diagnostic %q requires non-empty UTF-8 check and message", diagnostic.ID)
	}
	if diagnostic.Field != "" && !utf8.ValidString(diagnostic.Field) {
		return contractErrorf("diagnostic %q field is not valid UTF-8", diagnostic.ID)
	}
	if err := validateOptionalJSON("expected", diagnostic.Expected); err != nil {
		return err
	}
	if err := validateOptionalJSON("actual", diagnostic.Actual); err != nil {
		return err
	}
	seenSources := make(map[PolicySource]struct{}, len(diagnostic.PolicySources))
	for _, source := range diagnostic.PolicySources {
		if !validPolicySource(source) {
			return contractErrorf("diagnostic %q has unknown policy source %q", diagnostic.ID, source)
		}
		if _, exists := seenSources[source]; exists {
			return contractErrorf("diagnostic %q has duplicate policy source %q", diagnostic.ID, source)
		}
		seenSources[source] = struct{}{}
	}
	for key, value := range diagnostic.Evidence {
		if key == "" || isSecretKey(key) {
			return contractErrorf("diagnostic %q evidence contains a secret-like key", diagnostic.ID)
		}
		if !isScalar(value) || containsSecret(value) {
			return contractErrorf("diagnostic %q evidence %q is not a secret-safe scalar", diagnostic.ID, key)
		}
		if _, err := canonicalJSONValue(value); err != nil {
			return contractErrorf("diagnostic %q evidence %q is not an RFC 8785 JSON value", diagnostic.ID, key)
		}
	}
	return nil
}

func validateOptionalJSON(name string, value *any) error {
	if value == nil {
		return nil
	}
	if _, err := canonicalJSONValue(*value); err != nil {
		return contractErrorf("%s is not an RFC 8785 JSON value: %v", name, err)
	}
	encoded, err := json.Marshal(*value)
	if err != nil {
		return contractErrorf("%s is not a JSON value: %v", name, err)
	}
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return contractErrorf("%s is not a JSON value: %v", name, err)
	}
	if containsSecret(normalized) {
		return contractErrorf("%s contains a secret-like value", name)
	}
	return nil
}

func validateMetadata(metadata *DiagnosticMetadata) error {
	if metadata == nil {
		return nil
	}
	if metadata.PackageManifest == nil {
		return contractErrorf("diagnostic_metadata must contain package_manifest")
	}
	manifest := metadata.PackageManifest
	if err := validateRepository(manifest.Repository); err != nil {
		return err
	}
	if err := validateLicense(manifest.License); err != nil {
		return err
	}
	if err := validateAuthor(manifest.Author); err != nil {
		return err
	}
	for _, value := range append([]string{manifest.Description, manifest.Homepage}, manifest.Keywords...) {
		if value != "" && containsSecret(value) {
			return contractErrorf("package_manifest contains secret-like metadata")
		}
	}
	return nil
}

func validateRepository(value any) error {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return validateSafeMetadataString("repository", typed)
	case map[string]any:
		return validateClosedStringObject("repository", typed, []string{"type", "url"}, []string{"directory"})
	default:
		return contractErrorf("package_manifest.repository has an invalid JSON form")
	}
}

func validateLicense(value any) error {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return validateSafeMetadataString("license", typed)
	case map[string]any:
		return validateClosedStringObject("license", typed, []string{"type"}, []string{"url"})
	default:
		return contractErrorf("package_manifest.license has an invalid JSON form")
	}
}

func validateAuthor(value any) error {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return validateSafeMetadataString("author", typed)
	case map[string]any:
		return validateClosedStringObject("author", typed, []string{"name"}, []string{"email", "url"})
	default:
		return contractErrorf("package_manifest.author has an invalid JSON form")
	}
}

func validateClosedStringObject(name string, object map[string]any, required, optional []string) error {
	allowed := append(slices.Clone(required), optional...)
	for _, key := range required {
		value, ok := object[key].(string)
		if !ok || value == "" {
			return contractErrorf("package_manifest.%s requires non-empty string member %q", name, key)
		}
	}
	for key, raw := range object {
		if !slices.Contains(allowed, key) {
			return contractErrorf("package_manifest.%s has unknown member %q", name, key)
		}
		value, ok := raw.(string)
		if !ok || value == "" || containsSecret(value) {
			return contractErrorf("package_manifest.%s member %q is invalid or secret-like", name, key)
		}
		if (key == "url" || key == "repository") && urlHasCredentials(value) {
			return contractErrorf("package_manifest.%s contains a credential-bearing URL", name)
		}
	}
	return nil
}

func validateSafeMetadataString(name, value string) error {
	if value == "" || containsSecret(value) || urlHasCredentials(value) {
		return contractErrorf("package_manifest.%s is empty or secret-like", name)
	}
	return nil
}

func diagnosticLess(left, right Diagnostic) bool {
	leftDefinition := registry[left.ID]
	rightDefinition := registry[right.ID]
	if leftDefinition.precedence != rightDefinition.precedence {
		return leftDefinition.precedence < rightDefinition.precedence
	}
	if left.ID != right.ID {
		return left.ID < right.ID
	}
	if left.Field != right.Field {
		return left.Field < right.Field
	}
	leftActual := canonicalOptional(left.Actual)
	rightActual := canonicalOptional(right.Actual)
	if leftActual != rightActual {
		return leftActual < rightActual
	}
	return canonicalDiagnostic(left) < canonicalDiagnostic(right)
}

func canonicalOptional(value *any) string {
	if value == nil {
		return ""
	}
	canonical, err := canonicalJSONValue(*value)
	if err != nil {
		return ""
	}
	return string(canonical)
}

func canonicalJSONValue(value any) ([]byte, error) {
	encoded, err := json.Marshal(struct {
		Value any `json:"value"`
	}{Value: value})
	if err != nil {
		return nil, err
	}
	canonical, err := jsoncanonicalizer.Transform(encoded)
	if err != nil {
		return nil, err
	}
	const prefix = `{"value":`
	if len(canonical) <= len(prefix) || string(canonical[:len(prefix)]) != prefix || canonical[len(canonical)-1] != '}' {
		return nil, fmt.Errorf("unexpected canonical wrapper")
	}
	return canonical[len(prefix) : len(canonical)-1], nil
}

func canonicalDiagnostic(diagnostic Diagnostic) string {
	encoded, err := json.Marshal(diagnostic)
	if err != nil {
		return ""
	}
	canonical, err := jsoncanonicalizer.Transform(encoded)
	if err != nil {
		return ""
	}
	return string(canonical)
}

func validPolicySource(source PolicySource) bool {
	return source == PolicySourceExplicitPolicy || source == PolicySourceReleaseManifest ||
		source == PolicySourceProducerExpectedValue || source == PolicySourceDigestVerifiedHandoff
}

func isScalar(value any) bool {
	switch value.(type) {
	case nil, string, bool, float64, float32,
		int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
		json.Number:
		return true
	default:
		return false
	}
}

func isSecretKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "_", "-"))
	for _, marker := range []string{"authorization", "credential", "password", "private-key", "secret", "token", "api-key"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func containsSecret(value any) bool {
	switch typed := value.(type) {
	case string:
		lower := strings.ToLower(strings.TrimSpace(typed))
		if strings.HasPrefix(lower, "bearer ") || strings.HasPrefix(lower, "basic ") ||
			strings.HasPrefix(lower, "ghp_") || strings.HasPrefix(lower, "github_pat_") ||
			strings.HasPrefix(lower, "npm_") || strings.Contains(lower, "-----begin private key-----") {
			return true
		}
		return urlHasCredentials(typed)
	case map[string]any:
		for key, nested := range typed {
			if isSecretKey(key) || containsSecret(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if containsSecret(nested) {
				return true
			}
		}
	case []string:
		for _, nested := range typed {
			if containsSecret(nested) {
				return true
			}
		}
	}
	return false
}

func urlHasCredentials(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.User != nil
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	return stringPointer(*value)
}

func stringPointer(value string) *string {
	return &value
}

func equalStringPointers(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func contractErrorf(format string, args ...any) error {
	return &ContractError{ID: IDDiagnosticsContractInvalid, Reason: fmt.Sprintf(format, args...)}
}
