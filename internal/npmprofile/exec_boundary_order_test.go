package npmprofile

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommandBoundaryPreservesEachStreamOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeScript(t, root, "pnpm", "#!/bin/sh\nprintf 'stderr-A\\n' >&2\nprintf 'stderr-B\\n' >&2\nprintf 'stdout-C\\n'\nprintf 'stdout-D\\n'\n")
	output, err := runCommandAtBoundary(t.Context(), root, filepath.Join(root, "pnpm"), []string{}, nil, commandBoundaryOptions{shimDirectory: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"stderr-A", "stderr-B"}, {"stdout-C", "stdout-D"}} {
		first, second := strings.Index(output, pair[0]), strings.Index(output, pair[1])
		if first < 0 || second < 0 || first >= second {
			t.Fatalf("per-stream order lost in %q", output)
		}
	}
}

func TestPublishStderrCodeOrderIsDeterministic(t *testing.T) {
	for _, test := range []struct{ first, second, id string }{
		{"E403", "E404", IDMutationPermissionDenied},
		{"E404", "E403", IDTrustedPublisherMismatch},
	} {
		t.Run(test.first, func(t *testing.T) {
			fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true, npmExitCode: 1,
				npmError: "npm error code " + test.first + "\nnpm error code " + test.second})
			var log bytes.Buffer
			result, err := Publish(WithOperationalDebugOutput(t.Context(), &log), fixture.request)
			requireNPMDiagnostic(t, err, test.id)
			if result.State != PublishAbsent || !result.MutationAttempted || result.Report.PrimaryID == nil || *result.Report.PrimaryID != test.id {
				t.Fatal("definitive classification changed")
			}
			if !strings.Contains(log.String(), npmFailureSummaryPrefix+" code="+test.first+"\n") {
				t.Fatal("recognizer did not select first stderr code")
			}
			if fixture.packumentRequests != 1 || fixture.attestationRequests != 1 {
				t.Fatal("definitive failure triggered read-back")
			}
			fixture.requireInvocations(t, fixture.expectedMutationInvocations())
		})
	}
}
