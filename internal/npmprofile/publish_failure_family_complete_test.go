package npmprofile

import (
	"strings"
	"testing"
)

func routedFamilyMessage(body, packageName string) string {
	escaped := strings.NewReplacer("@", "%40", "/", "%2f").Replace(packageName)
	return "npm error 422 Unprocessable Entity - PUT https://registry.npmjs.org/" + escaped + " - " + strings.ReplaceAll(body, "\n", "\nnpm error 422 ")
}

func TestNPMFailureFamiliesRoutedCapacity(t *testing.T) {
	t.Parallel()
	maxPackage := "@" + strings.Repeat("s", 100) + "/" + strings.Repeat("p", 112)
	if len(maxPackage) != 214 {
		t.Fatal("invalid maximum-length fixture")
	}
	if got := len(strings.ReplaceAll(predicateFamilyMessage, "\n", " ")); got != 1019 {
		t.Fatalf("catalog template size = %d; update bound derivation", got)
	}
	for _, fixture := range npmFailureFamilyBodies() {
		t.Run(fixture.family, func(t *testing.T) {
			t.Parallel()
			for _, pkg := range []string{"@windlass/slsa-builder", maxPackage} {
				for _, body := range []string{fixture.body, strings.ReplaceAll(fixture.body, "\n", " ")} {
					input := routedFamilyMessage(body, pkg)
					for _, chunk := range []int{1, 37, 4096} {
						got := recognizeNPMFailureForTest(t, input, chunk)
						want := npmFailureSummaryPrefix + " family=" + fixture.family + " http_status=422"
						if got != want {
							t.Fatalf("routed %d-byte input: got %q, want %q", len(input), got, want)
						}
					}
				}
			}
		})
	}
}

func TestNPMFailureFamiliesRejectIncompleteTemplates(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`Error verifying sigstore provenance bundle: Invalid GitHub Actions SLSA v1 provenance predicate:not a rejection`,
		`Error verifying sigstore provenance bundle: Failed to validate repository information: package.json: "repository.url" is "`,
		"Error verifying sigstore provenance bundle:\nUnsupported GitHub Actions runner environment: \"self-hosted\".\nActually supported now.",
		"Error verifying sigstore provenance bundle: Invalid GitHub Actions SLSA v1 provenance predicate:",
		"Error verifying sigstore provenance bundle: Invalid GitHub Actions SLSA v1 provenance predicate: not a rejection",
		"Error verifying sigstore provenance bundle: Invalid GitHub Actions SLSA v1 provenance predicate:\nbuildDefinition.buildType: x does not match expected value for buildType: ",
		"Error verifying sigstore provenance bundle: Invalid GitHub Actions SLSA v1 provenance predicate:\nbuildDefinition.buildType: x does not match expected value for buildType: y,",
		strings.TrimSuffix(runnerFamilyMessage, "\nOnly \"github-hosted\" runners are supported when publishing with provenance."),
		strings.TrimSuffix(repositoryFamilyMessage, " from provenance"),
		repositoryFamilyMessage + " altered",
	} {
		t.Run(body[:min(len(body), 85)], func(t *testing.T) {
			t.Parallel()
			for _, input := range []string{frameNPMFamilyMessage(body), routedFamilyMessage(body, "@scope/pkg")} {
				got := recognizeNPMFailureForTest(t, input, 1)
				if strings.Contains(got, " family=") {
					t.Fatalf("incomplete template produced %q", got)
				}
			}
		})
	}
}

func TestNPMFailureFamiliesLineBoundFailsSafe(t *testing.T) {
	t.Parallel()
	prefix := `Error verifying sigstore provenance bundle: Failed to validate repository information: package.json: "repository.url" is "`
	suffix := `", expected to match "r" from provenance`
	base := routedFamilyMessage(prefix+suffix, "@scope/pkg")
	for _, extra := range []int{0, 1} {
		input := routedFamilyMessage(prefix+strings.Repeat("x", maxNPMFailurePendingLineLength-len(base)+extra)+suffix, "@scope/pkg")
		got := recognizeNPMFailureForTest(t, input, 1)
		want := npmFailureSummaryPrefix + " http_status=422"
		if extra == 0 {
			want = npmFailureSummaryPrefix + " family=repository-metadata-mismatch http_status=422"
		}
		if got != want {
			t.Fatalf("bound+%d: got %q, want %q", extra, got, want)
		}
		if extra == 1 {
			input += "\nnpm error code E403\n" + frameNPMFamilyMessage(privateFamilyMessage)
			want = npmFailureSummaryPrefix + " family=private-source-repository code=E403 http_status=422"
			if got := recognizeNPMFailureForTest(t, input, 1); got != want {
				t.Fatalf("overflow corrupted following state: %q", got)
			}
		}
	}
}
