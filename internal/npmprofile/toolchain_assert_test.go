package npmprofile_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestAssertToolchainPair_whenObservationsDiffer(t *testing.T) {
	for _, test := range []struct {
		name     string
		node     string
		npm      string
		nodeOnly bool
		wantFail bool
	}{
		{name: "matching pair", node: "v24.21.0", npm: "11.19.0"},
		{name: "older node", node: "v24.20.0", npm: "11.19.0", wantFail: true},
		{name: "older npm", node: "v24.21.0", npm: "11.18.0", wantFail: true},
		{name: "node-only without npm", node: "v24.21.0", nodeOnly: true},
		{name: "node-only ignores installed npm", node: "v24.21.0", npm: "11.18.0", nodeOnly: true},
		{name: "node-only wrong node", node: "v24.20.0", nodeOnly: true, wantFail: true},
		{name: "unparseable node", node: "not a version", npm: "11.19.0", wantFail: true},
		{name: "unparseable npm", node: "v24.21.0", npm: "not a version", wantFail: true},
		{name: "missing npm", node: "v24.21.0", wantFail: true},
		{name: "missing node", npm: "11.19.0", wantFail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given a private PATH containing only the requested fake executables.
			bin := t.TempDir()
			counter := filepath.Join(t.TempDir(), "npm-invocations")
			if test.node != "" {
				writeVersionTool(t, bin, "node", test.node, "")
			}
			if test.npm != "" {
				writeVersionTool(t, bin, "npm", test.npm, counter)
			}
			t.Setenv("PATH", bin)
			t.Setenv("RUNNER_TOOL_CACHE", "")
			t.Setenv("MISE_DATA_DIR", t.TempDir())
			t.Setenv("NPM_TOKEN", "should-not-reach-tool")
			t.Setenv("NODE_AUTH_TOKEN", "should-not-reach-tool")

			// When the pre-install assertion observes the selected job shape.
			err := npmprofile.AssertToolchainPair(t.Context(), test.nodeOnly)

			// Then only the exact pinned pair passes, and node-only never invokes npm.
			var mismatch *npmprofile.ToolchainPairMismatchError
			if test.wantFail {
				if !errors.As(err, &mismatch) || mismatch.DiagnosticID() != "windlass.verify.error.toolchain-pair-mismatch" {
					t.Fatalf("error = %v, want toolchain-pair-mismatch", err)
				}
			} else if err != nil {
				t.Fatalf("matching toolchain rejected: %v", err)
			}
			if test.nodeOnly {
				if _, err := os.Stat(counter); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("npm was executed in node-only mode: %v", err)
				}
			} else if test.npm != "" && test.node == "v24.21.0" {
				if _, err := os.Stat(counter); err != nil {
					t.Fatalf("npm was not observed in full mode: %v", err)
				}
			}
		})
	}
}

func TestAssertToolchainPair_observesFirstNodeOnPath(t *testing.T) {
	// Given two node executables on PATH, with only the first matching the pin.
	first := t.TempDir()
	second := t.TempDir()
	writeVersionTool(t, first, "node", "v24.21.0", "")
	writeVersionTool(t, second, "node", "v24.20.0", "")
	t.Setenv("PATH", first+string(os.PathListSeparator)+second)
	t.Setenv("RUNNER_TOOL_CACHE", "")
	t.Setenv("MISE_DATA_DIR", t.TempDir())

	// When the full toolchain assertion resolves node.
	err := npmprofile.AssertToolchainPair(t.Context(), true)

	// Then the matching first PATH entry is observed.
	if err != nil {
		t.Fatalf("first node on PATH was not observed: %v", err)
	}
}

func writeVersionTool(t *testing.T, bin, name, version, counter string) {
	t.Helper()
	script := "#!/bin/sh\n"
	if counter != "" {
		script += "printf 'called\\n' >> '" + counter + "'\n"
	}
	script += "[ \"$#\" -eq 1 ] && [ \"$1\" = '--version' ] || exit 7\n"
	script += "[ -z \"${NPM_TOKEN-}\" ] && [ -z \"${NODE_AUTH_TOKEN-}\" ] || exit 8\n"
	script += "printf '%s\\n' '" + strings.ReplaceAll(version, "'", "") + "'\n"
	if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}
