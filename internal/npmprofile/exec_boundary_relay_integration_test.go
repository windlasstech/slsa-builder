package npmprofile

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommandBoundaryRelaysStderrOnly(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"node", "npm", "npx", "corepack", "pnpm", "yarn"} {
		for _, exit := range []int{0, 9} {
			t.Run(fmt.Sprintf("%s/exit%d", operation, exit), func(t *testing.T) {
				t.Parallel()
				// Given: both output streams and a secret in the actual child environment.
				root := t.TempDir()
				writeScript(t, root, operation, fmt.Sprintf("#!/bin/sh\nprintf 'stdout-only\\n'\nprintf 'public %%s\\nlast' \"$ACTIONS_ID_TOKEN_REQUEST_TOKEN\" >&2\nexit %d\n", exit))
				var log bytes.Buffer
				ctx := WithOperationalDebugOutput(t.Context(), &log)
				// When: an allowlisted process completes, successfully or unsuccessfully.
				output, err := runCommandAtBoundary(ctx, root, filepath.Join(root, operation), []string{"ACTIONS_ID_TOKEN_REQUEST_TOKEN=ambient-canary"}, nil, commandBoundaryOptions{shimDirectory: root})
				// Then: capture remains raw, but only filtered stderr reaches the operational log.
				if (err != nil) != (exit != 0) || !strings.Contains(output, "stdout-only") || !strings.Contains(output, "public ambient-canary\nlast") {
					t.Fatalf("capture = %q, %v", output, err)
				}
				prefix := "windlass operational: operation=" + operation
				want := prefix + " stderr: public [REDACTED]\n" + prefix + " stderr: last\n"
				if !strings.Contains(log.String(), want) {
					t.Fatalf("missing filtered stderr relay %q; log:\n%s", want, log.String())
				}
				if strings.Contains(log.String(), "stdout-only") || strings.Contains(log.String(), "ambient-canary") {
					t.Fatal("stdout or secret leaked into log")
				}
				if !strings.HasPrefix(log.String(), prefix+" start\n"+want+prefix+" started=true") {
					t.Fatalf("incorrect lifecycle ordering: %s", log.String())
				}
			})
		}
	}
}

func TestRunCommandBoundaryRelayConcurrentStreams(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeScript(t, root, "npm", "#!/bin/sh\n(i=0; while [ $i -lt 2000 ]; do printf 'stdout-only\\n'; i=$((i+1)); done) &\ni=0; while [ $i -lt 2000 ]; do printf 'stderr-public\\n' >&2; i=$((i+1)); done\nwait\nprintf '\\nnpm error code E422\\n' >&2\nexit 1\n")
	var log bytes.Buffer
	output, err := runCommandAtBoundary(WithOperationalDebugOutput(t.Context(), &log), root, filepath.Join(root, "npm"), []string{}, []string{"publish"}, commandBoundaryOptions{shimDirectory: root})
	if err == nil || strings.Count(output, "stdout-only") != 2000 || strings.Count(output, "stderr-public") != 2000 {
		t.Fatal("concurrent capture lost output or error")
	}
	if strings.Contains(log.String(), "stdout-only") || strings.Count(log.String(), "stderr: stderr-public\n") != 512 || strings.Count(log.String(), "stderr_truncated=true") != 1 {
		t.Fatal("concurrent relay lost isolation or bounds")
	}
	marker := strings.Index(log.String(), "stderr_truncated=true")
	completion := strings.Index(log.String(), "started=true")
	summary := strings.Index(log.String(), "npm_failure code=E422")
	if marker < 0 || completion < marker || summary < completion {
		t.Fatal("incorrect relay/completion/summary ordering")
	}
}

func TestRunCommandBoundaryRelayWriterFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeScript(t, root, "pnpm", "#!/bin/sh\nprintf 'public error' >&2\nexit 9\n")
	var controlOutput, controlError string
	for i, writer := range []io.Writer{io.Discard, publishOperationalFailingWriter{}} {
		output, err := runCommandAtBoundary(WithOperationalDebugOutput(t.Context(), writer), root, filepath.Join(root, "pnpm"), []string{}, nil, commandBoundaryOptions{shimDirectory: root})
		if err == nil {
			t.Fatal("expected child exit failure")
		}
		if i == 0 {
			controlOutput, controlError = output, err.Error()
			continue
		}
		if output != controlOutput || err.Error() != controlError {
			t.Fatal("operational writer changed result")
		}
	}
}
