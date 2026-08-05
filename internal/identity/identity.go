// Package identity validates immutable GitHub Actions builder and source identities.
package identity

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	githubHost              = "github.com"
	canonicalGitHubBase     = "https://github.com/"
	builderRepositoryURI    = "https://github.com/windlasstech/slsa-builder"
	buildTypeNamespace      = "https://buildtype.dev/windlass/slsa-builder/"
	trustedGitHubOIDCIssuer = "https://token.actions.githubusercontent.com"
)

var (
	fullSHAExpression           = regexp.MustCompile(`^[0-9a-f]{40}$`)
	positiveDecimalExpression   = regexp.MustCompile(`^[1-9][0-9]*$`)
	repositorySegmentExpression = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	profileSegmentExpression    = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
)

// DiagnosticID is directly convertible to the string ID accepted by internal/diagnostic.
type DiagnosticID string

const (
	IDBuildTypeNotCanonical             DiagnosticID = "windlass.verify.error.build-type-not-canonical"
	IDBuilderIDNotImmutable             DiagnosticID = "windlass.verify.error.builder-id-not-immutable"
	IDIssuerMismatch                    DiagnosticID = "windlass.verify.error.issuer-mismatch"
	IDPackageRepositoryIdentityMismatch DiagnosticID = "windlass.verify.error.package-repository-identity-mismatch"
	IDRunInvocationURIInvalid           DiagnosticID = "windlass.verify.error.run-invocation-uri-invalid"
	IDSelfHostedRunner                  DiagnosticID = "windlass.verify.error.self-hosted-runner"
	IDSignerIdentityClaimMissing        DiagnosticID = "windlass.verify.error.signer-identity-claim-missing"
	IDSignerWorkflowPathMismatch        DiagnosticID = "windlass.verify.error.signer-workflow-path-mismatch"
	IDSignerWorkflowSHAMismatch         DiagnosticID = "windlass.verify.error.signer-workflow-sha-mismatch"
	IDSourceDigestMismatch              DiagnosticID = "windlass.verify.error.source-digest-mismatch"
	IDSourceIdentityMismatch            DiagnosticID = "windlass.verify.error.source-identity-mismatch"
	IDSourceNumericIDMismatch           DiagnosticID = "windlass.verify.error.source-numeric-id-mismatch"
	IDSourceRefMismatch                 DiagnosticID = "windlass.verify.error.source-ref-mismatch"
)

// ValidationError carries the stable diagnostic ID for a failed identity check.
type ValidationError struct {
	ID      DiagnosticID
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", e.ID, e.Message)
	}
	return fmt.Sprintf("%s: %s: %s", e.ID, e.Field, e.Message)
}

// DiagnosticID returns the string form consumed by internal/diagnostic.New.
func (e *ValidationError) DiagnosticID() string {
	return string(e.ID)
}

func validationError(id DiagnosticID, field, format string, arguments ...any) error {
	return &ValidationError{ID: id, Field: field, Message: fmt.Sprintf(format, arguments...)}
}

// CanonicalRepository normalizes a closed set of GitHub repository locators.
func CanonicalRepository(raw string) (string, error) {
	owner, repository, ok := repositoryCoordinates(raw)
	if !ok {
		return "", validationError(
			IDPackageRepositoryIdentityMismatch,
			"repository",
			"repository locator is not a supported uncredentialed GitHub form",
		)
	}
	return canonicalGitHubBase + strings.ToLower(owner) + "/" + strings.ToLower(repository), nil
}

// ValidateCanonicalRepositoryURI requires the exact lowercase GitHub URI form used by policy.
func ValidateCanonicalRepositoryURI(repositoryURI string) error {
	canonical, err := CanonicalRepository(repositoryURI)
	if err != nil || canonical != repositoryURI || !strings.HasPrefix(repositoryURI, canonicalGitHubBase) {
		return validationError(
			IDSourceIdentityMismatch,
			"source.repository_uri",
			"repository URI must use the exact canonical lowercase GitHub form",
		)
	}
	return nil
}

func repositoryCoordinates(raw string) (string, string, bool) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\\\x00") || strings.Contains(raw, "%") {
		return "", "", false
	}

	if remainder, found := strings.CutPrefix(raw, "github:"); found {
		return splitRepositoryPath(remainder, false, false, false)
	}
	if remainder, found := strings.CutPrefix(raw, "git@github.com:"); found {
		return splitRepositoryPath(remainder, false, true, true)
	}
	if !strings.Contains(raw, ":") {
		return splitRepositoryPath(raw, false, false, false)
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Hostname() != githubHost || parsed.Port() != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", "", false
	}

	switch parsed.Scheme {
	case "https", "git+https", "git":
		if parsed.User != nil {
			return "", "", false
		}
	case "ssh":
		if parsed.User == nil || parsed.User.Username() != "git" {
			return "", "", false
		}
		if _, hasPassword := parsed.User.Password(); hasPassword {
			return "", "", false
		}
	default:
		return "", "", false
	}

	return splitRepositoryPath(
		strings.TrimPrefix(parsed.Path, "/"),
		true,
		true,
		parsed.Scheme == "ssh",
	)
}

func splitRepositoryPath(path string, allowTrailingSlash, allowGitSuffix, requireGitSuffix bool) (string, string, bool) {
	if allowTrailingSlash && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	hasGitSuffix := strings.HasSuffix(path, ".git")
	if requireGitSuffix && !hasGitSuffix || !allowGitSuffix && hasGitSuffix {
		return "", "", false
	}
	if allowGitSuffix && hasGitSuffix {
		path = strings.TrimSuffix(path, ".git")
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || !validRepositorySegment(parts[0]) || !validRepositorySegment(parts[1]) {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func validRepositorySegment(segment string) bool {
	return segment != "." && segment != ".." && repositorySegmentExpression.MatchString(segment)
}

// ValidateFullSHA requires the lowercase full Git commit form used by builder references.
func ValidateFullSHA(sha string) error {
	if !validFullSHA(sha) {
		return validationError(
			IDBuilderIDNotImmutable,
			"workflow_sha",
			"workflow revision must be exactly 40 lowercase hexadecimal characters",
		)
	}
	return nil
}

func validFullSHA(sha string) bool {
	return fullSHAExpression.MatchString(sha)
}

// ValidateReleaseRef requires a complete release tag reference.
func ValidateReleaseRef(ref string) error {
	if !validReleaseRef(ref) {
		return validationError(IDSourceRefMismatch, "source.ref", "source ref must be a valid full refs/tags reference")
	}
	return nil
}

func validReleaseRef(ref string) bool {
	const prefix = "refs/tags/"
	if !strings.HasPrefix(ref, prefix) {
		return false
	}
	tag := strings.TrimPrefix(ref, prefix)
	if tag == "" || tag == "@" || strings.HasPrefix(tag, "/") || strings.HasSuffix(tag, "/") ||
		strings.Contains(tag, "//") || strings.Contains(tag, "..") || strings.Contains(tag, "@{") ||
		strings.HasSuffix(tag, ".") {
		return false
	}
	for _, component := range strings.Split(tag, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	for _, character := range tag {
		if unicode.IsControl(character) || unicode.IsSpace(character) || strings.ContainsRune("~^:?*[\\", character) {
			return false
		}
	}
	return true
}

func validPositiveDecimal(value string) bool {
	return positiveDecimalExpression.MatchString(value)
}

func validWorkflowPath(path string) bool {
	const prefix = ".github/workflows/"
	if !strings.HasPrefix(path, prefix) || strings.Contains(path, "%") || strings.Contains(path, "\\") {
		return false
	}
	filename := strings.TrimPrefix(path, prefix)
	if filename == "" || strings.Contains(filename, "/") || strings.HasPrefix(filename, ".") {
		return false
	}
	stem, extensionFound := strings.CutSuffix(filename, ".yaml")
	if !extensionFound {
		stem, extensionFound = strings.CutSuffix(filename, ".yml")
	}
	return extensionFound && stem != "" && repositorySegmentExpression.MatchString(stem)
}

// NewBuilderID constructs the immutable ADR 0028 reusable-workflow identity.
func NewBuilderID(workflowPath, workflowSHA string) (string, error) {
	if !validWorkflowPath(workflowPath) {
		return "", validationError(
			IDBuilderIDNotImmutable,
			"builder.id",
			"workflow path must identify one file directly under .github/workflows",
		)
	}
	if err := ValidateFullSHA(workflowSHA); err != nil {
		return "", err
	}
	return builderRepositoryURI + "/" + workflowPath + "@" + workflowSHA, nil
}

// ValidateBuilderID requires the exact Windlass repository, workflow path, and lowercase full SHA.
func ValidateBuilderID(builderID string) error {
	if strings.Contains(builderID, "%") || strings.TrimSpace(builderID) != builderID {
		return validationError(IDBuilderIDNotImmutable, "builder.id", "builder ID is not canonical")
	}
	prefix := builderRepositoryURI + "/"
	remainder, found := strings.CutPrefix(builderID, prefix)
	if !found {
		return validationError(IDBuilderIDNotImmutable, "builder.id", "builder ID uses an untrusted repository")
	}
	workflowPath, workflowSHA, found := strings.Cut(remainder, "@")
	if !found || strings.Contains(workflowSHA, "@") || !validWorkflowPath(workflowPath) || !validFullSHA(workflowSHA) {
		return validationError(
			IDBuilderIDNotImmutable,
			"builder.id",
			"builder ID must contain one trusted workflow path and a full lowercase SHA",
		)
	}
	canonical, err := NewBuilderID(workflowPath, workflowSHA)
	if err != nil || canonical != builderID {
		return validationError(IDBuilderIDNotImmutable, "builder.id", "builder ID is not canonical")
	}
	return nil
}

// NewBuildTypeURI constructs a canonical acquired-domain producer build type identifier.
func NewBuildTypeURI(profile string, major uint64) (string, error) {
	if !profileSegmentExpression.MatchString(profile) || major == 0 {
		return "", validationError(
			IDBuildTypeNotCanonical,
			"build_type",
			"profile must be one URI-safe segment and major version must be positive",
		)
	}
	return buildTypeNamespace + profile + "/v" + strconv.FormatUint(major, 10), nil
}

// ValidateBuildTypeURI requires the exact buildtype.dev namespace and canonical major version.
func ValidateBuildTypeURI(buildType string) error {
	if strings.Contains(buildType, "%") || strings.TrimSpace(buildType) != buildType {
		return validationError(IDBuildTypeNotCanonical, "build_type", "build type URI is not canonical")
	}
	remainder, found := strings.CutPrefix(buildType, buildTypeNamespace)
	if !found {
		return validationError(IDBuildTypeNotCanonical, "build_type", "build type URI uses another namespace")
	}
	profile, majorText, found := strings.Cut(remainder, "/v")
	if !found || strings.Contains(majorText, "/") || !profileSegmentExpression.MatchString(profile) ||
		!validPositiveDecimal(majorText) {
		return validationError(IDBuildTypeNotCanonical, "build_type", "build type URI has a non-canonical profile or version")
	}
	major, err := strconv.ParseUint(majorText, 10, 64)
	if err != nil {
		return validationError(IDBuildTypeNotCanonical, "build_type", "build type major version is out of range")
	}
	canonical, err := NewBuildTypeURI(profile, major)
	if err != nil || canonical != buildType {
		return validationError(IDBuildTypeNotCanonical, "build_type", "build type URI is not canonical")
	}
	return nil
}

// RunInvocation is the parsed GitHub Actions run and attempt identity.
type RunInvocation struct {
	URI           string
	RepositoryURI string
	RunID         string
	Attempt       string
}

// NewRunInvocationURI constructs the canonical URI carried by GitHub and SLSA metadata.
func NewRunInvocationURI(repositoryURI, runID, attempt string) (string, error) {
	if err := ValidateCanonicalRepositoryURI(repositoryURI); err != nil {
		return "", validationError(
			IDRunInvocationURIInvalid,
			"run_invocation",
			"source repository is not canonical",
		)
	}
	if !validPositiveDecimal(runID) || !validPositiveDecimal(attempt) {
		return "", validationError(
			IDRunInvocationURIInvalid,
			"run_invocation",
			"run ID and attempt must be positive canonical decimal integers",
		)
	}
	return repositoryURI + "/actions/runs/" + runID + "/attempts/" + attempt, nil
}

// ParseRunInvocationURI validates repository ownership and extracts the run identity.
func ParseRunInvocationURI(invocationURI, expectedRepositoryURI string) (RunInvocation, error) {
	if err := ValidateCanonicalRepositoryURI(expectedRepositoryURI); err != nil {
		return RunInvocation{}, validationError(
			IDRunInvocationURIInvalid,
			"run_invocation",
			"expected source repository is not canonical",
		)
	}
	if strings.Contains(invocationURI, "%") || strings.TrimSpace(invocationURI) != invocationURI {
		return RunInvocation{}, validationError(IDRunInvocationURIInvalid, "run_invocation", "invocation URI is not canonical")
	}
	parsed, err := url.Parse(invocationURI)
	if err != nil || parsed.Scheme != "https" || parsed.Host != githubHost || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return RunInvocation{}, validationError(IDRunInvocationURIInvalid, "run_invocation", "invocation URI has forbidden URI components")
	}
	prefix := expectedRepositoryURI + "/actions/runs/"
	remainder, found := strings.CutPrefix(invocationURI, prefix)
	if !found {
		return RunInvocation{}, validationError(IDRunInvocationURIInvalid, "run_invocation", "invocation URI identifies another repository")
	}
	parts := strings.Split(remainder, "/")
	if len(parts) != 3 || parts[1] != "attempts" || !validPositiveDecimal(parts[0]) || !validPositiveDecimal(parts[2]) {
		return RunInvocation{}, validationError(IDRunInvocationURIInvalid, "run_invocation", "invocation URI has an invalid run ID or attempt")
	}
	canonical, err := NewRunInvocationURI(expectedRepositoryURI, parts[0], parts[2])
	if err != nil || canonical != invocationURI {
		return RunInvocation{}, validationError(IDRunInvocationURIInvalid, "run_invocation", "invocation URI is not canonical")
	}
	return RunInvocation{
		URI:           invocationURI,
		RepositoryURI: expectedRepositoryURI,
		RunID:         parts[0],
		Attempt:       parts[2],
	}, nil
}
