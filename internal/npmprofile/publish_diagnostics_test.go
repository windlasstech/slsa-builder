package npmprofile

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

const npmFailureSummaryPrefix = "windlass operational: npm_failure"

func TestNPMFailureRecognizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "npm error code never infers status", input: "npm error code E422\n", want: npmFailureSummaryPrefix + " code=E422"},
		{name: "npm err code never infers status", input: "npm err! code EPUBLISHCONFLICT\n", want: npmFailureSummaryPrefix + " code=EPUBLISHCONFLICT"},
		{name: "status never infers code", input: "npm err! 400 Bad Request\n", want: npmFailureSummaryPrefix + " http_status=400"},
		{name: "code and differing status are independent", input: "npm error code E403\nnpm error 422 Unprocessable Entity\n", want: npmFailureSummaryPrefix + " code=E403 http_status=422"},
		{name: "status upper bound", input: "npm error 599 Server Error\n", want: npmFailureSummaryPrefix + " http_status=599"},
		{name: "status below range", input: "npm error 399 Redirect\n", want: ""},
		{name: "status above range", input: "npm error 600 Server Error\n", want: ""},
		{
			name: "only dotted and indexed allowlisted paths",
			input: "npm error 422 predicate.buildDefinition.buildType expected EXPECTED_CANARY\n" +
				"npm error 422 predicate.buildDefinition.externalParameters.source.repository=SECRET_CANARY\n" +
				"npm error 422 predicate.buildDefinition.externalParameters[0].name\n" +
				"npm error 422 predicate.buildDefinition.resolvedDependencies[12].annotations[0].name\n" +
				"npm error 422 predicate.runDetails.builder.id\n" +
				"npm error 422 predicate.runDetails.metadata[1].value\n",
			want: npmFailureSummaryPrefix + " http_status=422 predicate_paths=" +
				"predicate.buildDefinition.buildType,predicate.buildDefinition.externalParameters.source.repository," +
				"predicate.buildDefinition.externalParameters[0].name,predicate.buildDefinition.resolvedDependencies[12].annotations[0].name," +
				"predicate.runDetails.builder.id,predicate.runDetails.metadata[1].value",
		},
		{
			name: "unknown prose URL message comparison and secret canaries are omitted",
			input: "npm error unknown UNKNOWN_CANARY\nnpm error prose PROSE_CANARY\n" +
				"npm error message MESSAGE_CANARY\nnpm error expected EXPECTED_CANARY\nnpm error actual ACTUAL_CANARY\n" +
				"npm error 422 https://example.invalid/predicate.buildDefinition.buildType?secret=SECRET_CANARY\n" +
				"npm error 422 predicate.buildDefinition.buildTypeExtra\n" +
				"npm error 422 predicate.buildDefinition.internalParameters.secret\n" +
				"npm error 422 predicate.subject[0].name\n",
			want: npmFailureSummaryPrefix + " http_status=422",
		},
		{name: "overlong code is omitted", input: "npm error code " + strings.Repeat("E", maxNPMFailureCodeLength+1) + "\n", want: ""},
		{
			name:  "overlong path is omitted",
			input: "npm error 422 predicate.buildDefinition.externalParameters." + strings.Repeat("x", maxNPMFailurePredicatePathLength-len("predicate.buildDefinition.externalParameters.")+1) + "\n",
			want:  npmFailureSummaryPrefix + " http_status=422",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Given: an npm failure stream with tokens split at arbitrary byte boundaries.
			// When: the stream is recognized through uneven writes.
			got := recognizeNPMFailureForTest(t, test.input, 1, 3, 2, 64, 5)

			// Then: only independently recognized, grammar-safe fields are summarized.
			if got != test.want {
				t.Fatalf("Summary() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPublishOperationalDiagnosticsLateE422WithTruncationAndSecretCanary(t *testing.T) {
	// Given: a real Publish fixture whose combined output exceeds the unchanged head cap before E422.
	secretCanary := "NPM_FAILURE_SECRET_CANARY_7f3a91"
	noiseLine := "npm error noise " + secretCanary + " MESSAGE_CANARY\n"
	noise := strings.Repeat(noiseLine, maxNPMOutputSize/len(noiseLine)+1)
	if len(noise) <= maxNPMOutputSize {
		t.Fatalf("noise length = %d, want more than %d", len(noise), maxNPMOutputSize)
	}
	fixture := newPublishFixture(t, publishFixtureOptions{
		initialVersionAbsent: true,
		npmExitCode:          1,
		npmError: noise +
			"npm err! code E422\n" +
			"npm err! 422 Unprocessable Entity\n" +
			"npm err! 422 predicate.buildDefinition.buildType expected EXPECTED_CANARY actual ACTUAL_CANARY\n" +
			"npm err! 422 predicate.buildDefinition.externalParameters.source.repository=https://example.invalid/URL_CANARY\n" +
			"npm err! 422 predicate.runDetails.builder.id message MESSAGE_CANARY\n" +
			"npm err! 422 predicate.subject[0].name SUBJECT_CANARY\n",
	})
	var diagnostics bytes.Buffer
	ctx := WithOperationalDebugOutput(t.Context(), &diagnostics)

	// When: operational diagnostics are enabled for the otherwise unchanged publish.
	result, err := Publish(ctx, fixture.request)

	// Then: late recognition is complete while classification and read-back stay unchanged.
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.State != PublishCommittedAsExpected || !result.MutationAttempted || result.Report.PrimaryID != nil {
		t.Fatalf("Publish() result = %#v, want the normal committed read-back", result)
	}
	fixture.requireInvocations(t, fixture.expectedMutationInvocations())
	if fixture.packumentRequests != 2 || fixture.attestationRequests != 2 || fixture.clock.sleeps != 0 {
		t.Fatalf("read-back = packument:%d attestations:%d sleeps:%d, want 2, 2, 0", fixture.packumentRequests, fixture.attestationRequests, fixture.clock.sleeps)
	}
	wantFailure := npmFailureSummaryPrefix + " code=E422 http_status=422 predicate_paths=" +
		"predicate.buildDefinition.buildType,predicate.buildDefinition.externalParameters.source.repository,predicate.runDetails.builder.id"
	gotDiagnostics := diagnostics.String()
	failureLines := 0
	truncatedExitLines := 0
	for _, line := range strings.Split(strings.TrimSpace(gotDiagnostics), "\n") {
		if line == wantFailure {
			failureLines++
		}
		if strings.Contains(line, "operation=npm ") && strings.Contains(line, "outcome=exit exit=1") && strings.Contains(line, "output_truncated=true") {
			truncatedExitLines++
		}
	}
	if failureLines != 1 || truncatedExitLines != 1 {
		t.Fatalf("diagnostics failure lines = %d and truncated exit lines = %d, want one each; diagnostics:\n%s", failureLines, truncatedExitLines, gotDiagnostics)
	}
	for _, prohibited := range []string{
		secretCanary, "EXPECTED_CANARY", "ACTUAL_CANARY", "MESSAGE_CANARY", "URL_CANARY", "SUBJECT_CANARY",
		"https://", "npm err!", "noise", "argv=", "url=", "message=", "expected=", "actual=", "request_phase=", "response_body=",
		"must-not-reach-fake-npm", "must-not-reach-npm", "short-lived-token-must-not-reach-npm-env",
		fixture.request.TarballPath, fixture.request.BundlePath,
	} {
		if strings.Contains(gotDiagnostics, prohibited) {
			t.Errorf("diagnostics contain prohibited raw value %q", prohibited)
		}
	}
}

func FuzzNPMFailureRecognizer(f *testing.F) {
	f.Add([]byte("npm error code E422\nnpm error 422 predicate.runDetails.builder.id\n"), uint8(1))
	f.Add([]byte("npm err! code E403\nnpm err! 499 predicate.subject[0].name=SECRET_CANARY\n"), uint8(7))
	f.Add([]byte(strings.Repeat("noise SECRET_CANARY\n", maxNPMOutputSize/8)+"npm err! code E422\nnpm err! 422 predicate.buildDefinition.buildType\n"), uint8(31))
	summaryPattern := regexp.MustCompile(`^windlass operational: npm_failure( code=[A-Z][A-Z0-9_]{0,63})?( http_status=[45][0-9]{2})?( predicate_paths=[A-Za-z0-9_.\[\],]+)?$`)
	f.Fuzz(func(t *testing.T, input []byte, chunk uint8) {
		// Given: arbitrary bytes and a nonzero arbitrary write size.
		// When: the recognizer consumes every chunk without a panic recovery path.
		got := recognizeNPMFailureForTest(t, string(input), int(chunk)+1)

		// Then: retained state and emitted grammar remain bounded for every input.
		if len(got) > maxNPMFailureSummaryLength {
			t.Fatalf("Summary() length = %d, exceeds %d", len(got), maxNPMFailureSummaryLength)
		}
		if got != "" && (got == npmFailureSummaryPrefix || !summaryPattern.MatchString(got)) {
			t.Fatalf("Summary() violates the diagnostics-only grammar: %q", got)
		}
	})
}

func recognizeNPMFailureForTest(t *testing.T, input string, chunks ...int) string {
	t.Helper()
	if len(chunks) == 0 {
		chunks = []int{1}
	}
	recognizer := newNPMFailureRecognizer()
	for offset, index := 0, 0; offset < len(input); index++ {
		end := min(offset+chunks[index%len(chunks)], len(input))
		written, err := recognizer.Write([]byte(input[offset:end]))
		if err != nil || written != end-offset {
			t.Fatalf("Write() = %d, %v, want %d, nil", written, err, end-offset)
		}
		offset = end
		if size := recognizer.StateSize(); size < 0 || size > maxNPMFailureRecognizerState {
			t.Fatalf("StateSize() = %d after byte %d, want 0..%d", size, offset, maxNPMFailureRecognizerState)
		}
	}
	return recognizer.Summary()
}
