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

type npmFailureFamilyStage uint8

const (
	npmFailureStageNone npmFailureFamilyStage = iota
	npmFailureStageBody
	npmFailureStagePredicate
	npmFailureStageRunnerConclusion
	npmFailurePredicateHeader  = "Invalid GitHub Actions SLSA v1 provenance predicate:"
	npmFailureRunnerIntro      = `Unsupported GitHub Actions runner environment: "self-hosted".`
	npmFailureRunnerConclusion = `Only "github-hosted" runners are supported when publishing with provenance.`
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
	previousStage := r.familyStage
	r.familyStage = npmFailureStageNone
	if !complete {
		return
	}
	message := strings.TrimPrefix(rest, "422 ")
	if route, ok := strings.CutPrefix(message, "Unprocessable Entity - PUT https://registry.npmjs.org/"); ok {
		_, message, _ = strings.Cut(route, " - ")
	}
	if previousStage == npmFailureStageRunnerConclusion && message == npmFailureRunnerConclusion {
		r.family = npmFailureUnsupportedRunner
		return
	}
	if previousStage == npmFailureStagePredicate && r.observePredicateDetails(message) {
		return
	}
	if message == npmFailureBundleWrapper {
		r.familyStage = npmFailureStageBody
		return
	}
	body, wrapped := strings.CutPrefix(message, npmFailureBundleWrapper+" ")
	if !wrapped {
		if previousStage != npmFailureStageBody {
			return
		}
		body = message
	}
	switch {
	case body == npmFailurePredicateHeader:
		r.familyStage = npmFailureStagePredicate
	case strings.HasPrefix(body, npmFailurePredicateHeader+" "):
		r.observePredicateDetails(strings.TrimPrefix(body, npmFailurePredicateHeader+" "))
	case completeRepositoryMismatch(body):
		r.family = npmFailureRepositoryMismatch
	case body == `Unsupported GitHub Actions source repository visibility: "private". Only public source repositories are supported when publishing with provenance.`:
		r.family = npmFailurePrivateSource
	case body == npmFailureRunnerIntro:
		r.familyStage = npmFailureStageRunnerConclusion
	case body == npmFailureRunnerIntro+" "+npmFailureRunnerConclusion:
		r.family = npmFailureUnsupportedRunner
	case body == "Missing SourceRepositoryRef in signing certificate":
		r.family = npmFailureMissingClaim
	}
}

func completeRepositoryMismatch(body string) bool {
	values, ok := strings.CutPrefix(body, `Failed to validate repository information: package.json: "repository.url" is "`)
	if !ok {
		return false
	}
	_, rest, found := strings.Cut(values, `"`)
	if !found {
		return false
	}
	expected, ok := strings.CutPrefix(rest, `, expected to match "`)
	if !ok {
		return false
	}
	value, ending, found := strings.Cut(expected, `"`)
	return found && value != "" && ending == " from provenance"
}

func (r *npmFailureRecognizer) observePredicateDetails(details string) bool {
	text, continued := strings.CutSuffix(details, ",")
	for detail := range strings.SplitSeq(text, ", ") {
		if !completePredicateMismatch(detail) {
			return false
		}
	}
	if continued {
		r.familyStage = npmFailureStagePredicate
	} else {
		r.family = npmFailurePredicateMismatch
	}
	return true
}

func completePredicateMismatch(detail string) bool {
	for _, binding := range [...][2]string{
		{"buildDefinition.buildType", "buildType"},
		{"buildDefinition.externalParameters.workflow.ref", "SourceRepositoryRef"},
		{"buildDefinition.externalParameters.workflow.repository", "SourceRepositoryURI"},
		{"buildDefinition.externalParameters.workflow.path", "BuildConfigURI"},
		{"buildDefinition.internalParameters.github.event_name", "BuildTrigger"},
		{"buildDefinition.internalParameters.github.repository_id", "SourceRepositoryIdentifier"},
		{"buildDefinition.internalParameters.github.repository_owner_id", "SourceRepositoryOwnerIdentifier"},
		{"runDetails.builder.id", "builderId"},
	} {
		value, ok := strings.CutPrefix(detail, binding[0]+": ")
		if !ok {
			continue
		}
		observed, expected, found := strings.Cut(value, "does not match expected value for "+binding[1]+": ")
		return found && strings.TrimSpace(expected) != "" &&
			(observed == "" || strings.HasSuffix(observed, " ") && strings.TrimSpace(observed) != "")
	}
	return false
}
