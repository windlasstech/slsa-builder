package npmprofile

import "strings"

// Family state is a fixed-size identifier, not retained external message text.
type npmFailureFamily uint8

const (
	npmFailureFamilyNone npmFailureFamily = iota
	npmFailurePredicateMismatch
	npmFailureRepositoryMismatch
	npmFailurePrivateSource
	npmFailureUnsupportedRunner
	npmFailureMissingClaim
	npmFailureBundleWrapper = "Error verifying sigstore provenance bundle:"
)

func (family npmFailureFamily) label() string {
	switch family {
	case npmFailureFamilyNone:
		return ""
	case npmFailurePredicateMismatch:
		return "predicate-field-mismatch"
	case npmFailureRepositoryMismatch:
		return "repository-metadata-mismatch"
	case npmFailurePrivateSource:
		return "private-source-repository"
	case npmFailureUnsupportedRunner:
		return "unsupported-runner-environment"
	case npmFailureMissingClaim:
		return "missing-certificate-claim"
	default:
		return ""
	}
}

func (r *npmFailureRecognizer) observeFamily(rest string, complete bool) {
	if r.family != npmFailureFamilyNone {
		return
	}
	message := strings.TrimPrefix(rest, "422 ")
	if route, ok := strings.CutPrefix(message, "Unprocessable Entity - PUT https://registry.npmjs.org/"); ok {
		_, message, _ = strings.Cut(route, " - ")
	}
	previousWrapper := r.familyWrapper
	r.familyWrapper = complete && message == npmFailureBundleWrapper
	if previousWrapper && complete && message == `Unsupported GitHub Actions runner environment: "self-hosted".` {
		r.family = npmFailureUnsupportedRunner
		return
	}
	body, wrapped := strings.CutPrefix(message, npmFailureBundleWrapper+" ")
	if !wrapped {
		return
	}
	switch {
	case strings.HasPrefix(body, "Invalid GitHub Actions SLSA v1 provenance predicate:"):
		r.family = npmFailurePredicateMismatch
	case strings.HasPrefix(body, `Failed to validate repository information: package.json: "repository.url" is "`):
		r.family = npmFailureRepositoryMismatch
	case complete && body == `Unsupported GitHub Actions source repository visibility: "private". Only public source repositories are supported when publishing with provenance.`:
		r.family = npmFailurePrivateSource
	case complete && body == "Missing SourceRepositoryRef in signing certificate":
		r.family = npmFailureMissingClaim
	}
}
