package npmprofile

import (
	"strings"
	"testing"
)

// Verbatim catalog bodies; only npm's per-line framing is added by the fixtures.
const (
	predicateFamilyMessage = `Error verifying sigstore provenance bundle: Invalid GitHub Actions SLSA v1 provenance predicate:
buildDefinition.buildType: <observed> does not match expected value for buildType: <expected>,
buildDefinition.externalParameters.workflow.ref: does not match expected value for SourceRepositoryRef: <expected>,
buildDefinition.externalParameters.workflow.repository: does not match expected value for SourceRepositoryURI: <expected>,
buildDefinition.externalParameters.workflow.path: does not match expected value for BuildConfigURI: <expected>,
buildDefinition.internalParameters.github.event_name: does not match expected value for BuildTrigger: <expected>,
buildDefinition.internalParameters.github.repository_id: does not match expected value for SourceRepositoryIdentifier: <expected>,
buildDefinition.internalParameters.github.repository_owner_id: does not match expected value for SourceRepositoryOwnerIdentifier: <expected>,
runDetails.builder.id: <observed> does not match expected value for builderId: <expected>`
	repositoryFamilyMessage = `Error verifying sigstore provenance bundle: Failed to validate repository information: package.json: "repository.url" is "<manifest value>", expected to match "<provenance repository URL>" from provenance`
	privateFamilyMessage    = `Error verifying sigstore provenance bundle: Unsupported GitHub Actions source repository visibility: "private". Only public source repositories are supported when publishing with provenance.`
	runnerFamilyMessage     = `Error verifying sigstore provenance bundle:
Unsupported GitHub Actions runner environment: "self-hosted".
Only "github-hosted" runners are supported when publishing with provenance.`
	missingClaimFamilyLine = `npm error 422 Unprocessable Entity - PUT https://registry.npmjs.org/<escaped package> - Error verifying sigstore provenance bundle: Missing SourceRepositoryRef in signing certificate`
)

func npmFailureFamilyFixtures() []struct{ family, input string } {
	return []struct{ family, input string }{
		{"predicate-field-mismatch", frameNPMFamilyMessage(predicateFamilyMessage)},
		{"repository-metadata-mismatch", frameNPMFamilyMessage(repositoryFamilyMessage)},
		{"private-source-repository", frameNPMFamilyMessage(privateFamilyMessage)},
		{"unsupported-runner-environment", frameNPMFamilyMessage(runnerFamilyMessage)},
		{"missing-certificate-claim", missingClaimFamilyLine},
	}
}

func frameNPMFamilyMessage(message string) string {
	return "npm error 422 " + strings.ReplaceAll(message, "\n", "\nnpm error 422 ")
}

func TestNPMFailureRecognizerFamilies(t *testing.T) {
	t.Parallel()
	for _, fixture := range npmFailureFamilyFixtures() {
		t.Run(fixture.family, func(t *testing.T) {
			t.Parallel()
			// Given: catalog text plus existing independently recognized fields.
			input := "npm error code E422\n" + fixture.input + "\n" +
				"npm error 422 predicate.buildDefinition.buildType expected SECRET_CANARY\n" +
				"npm error 422 predicate.runDetails.builder.id actual SECRET_CANARY"
			want := npmFailureSummaryPrefix + " family=" + fixture.family +
				" code=E422 http_status=422 predicate_paths=predicate.buildDefinition.buildType,predicate.runDetails.builder.id"
			// When: every framing style is consumed with split and whole writes.
			for _, prefix := range []string{"npm error ", "npm err! "} {
				for _, chunk := range []int{1, 7, 4096} {
					got := recognizeNPMFailureForTest(t, strings.ReplaceAll(input, "npm error ", prefix), chunk)
					// Then: family precedes the unchanged categorical fields, with no raw values.
					if got != want {
						t.Fatalf("Summary() = %q, want %q", got, want)
					}
				}
			}
		})
	}
}

func TestNPMFailureRecognizerFamilyAnchors(t *testing.T) {
	t.Parallel()
	for _, message := range []string{
		"novel registry rejection",
		"Unsupported OIDC token issuer",
		"quoted: " + predicateFamilyMessage,
		strings.ReplaceAll(predicateFamilyMessage, "predicate:", "predicate-like:"),
		strings.ReplaceAll(repositoryFamilyMessage, `"repository.url"`, `"repository.other"`),
		privateFamilyMessage + " not a rejection",
		`Unsupported GitHub Actions runner environment: "self-hosted".`,
		"Error verifying sigstore provenance bundle:\nintervening line\n" + `Unsupported GitHub Actions runner environment: "self-hosted".`,
		"quoted: " + missingClaimFamilyLine,
		strings.ReplaceAll(missingClaimFamilyLine, "Missing SourceRepositoryRef", "Missing UnknownClaim"),
		missingClaimFamilyLine + "Extra",
		strings.Repeat("x", maxNPMFailurePendingLineLength) + privateFamilyMessage,
	} {
		t.Run(message[:min(len(message), 70)], func(t *testing.T) {
			t.Parallel()
			input := message
			if !strings.HasPrefix(input, "npm error ") {
				input = frameNPMFamilyMessage(input)
			}
			got := recognizeNPMFailureForTest(t, input, 1, 31)
			if strings.Contains(got, " family=") {
				t.Fatalf("near miss classified: %q", got)
			}
		})
	}
}

func TestNPMFailureRecognizerFirstFamilyWins(t *testing.T) {
	t.Parallel()
	fixtures := npmFailureFamilyFixtures()
	for i, fixture := range fixtures {
		t.Run(fixture.family, func(t *testing.T) {
			t.Parallel()
			input := fixture.input + "\n" + fixtures[(i+1)%len(fixtures)].input
			got := recognizeNPMFailureForTest(t, input, 1)
			want := npmFailureSummaryPrefix + " family=" + fixture.family + " http_status=422"
			if got != want {
				t.Fatalf("Summary() = %q, want first family %q", got, want)
			}
		})
	}
}

func TestNPMFailureRecognizerFamilyOnlyAndFinalLine(t *testing.T) {
	t.Parallel()
	for _, ending := range []string{"", "\n", "\r\n"} {
		got := recognizeNPMFailureForTest(t, "npm error "+privateFamilyMessage+ending, 1)
		if got != npmFailureSummaryPrefix+" family=private-source-repository" {
			t.Fatalf("Summary() = %q, want family without inferred code/status", got)
		}
	}
}
