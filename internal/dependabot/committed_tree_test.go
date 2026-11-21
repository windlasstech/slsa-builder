package dependabot

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateTrustedIgnoresDirtyAllowlistedContent(t *testing.T) {
	base, err := readModules("testdata/base")
	if err != nil {
		t.Fatal(err)
	}
	head, err := readModules("testdata/head")
	if err != nil {
		t.Fatal(err)
	}
	transition, reason := ParseSigstoreModuleTransition(base, head)
	if reason != "" {
		t.Fatal(reason)
	}
	files := map[string][]byte{"go.mod": base.Mod, "go.sum": base.Sum}
	for _, path := range sourceSites {
		files[path] = []byte("committed v1.4.0\n")
	}
	files["docs/dependency-selection.md"] = []byte("committed v1.4.0 was released on\n12026-10-29\ndeclares Go 1.27.0, which is compatible\n")
	files["internal/toolchainpins/signing_adapter_test.go"] = []byte("committed v1.4.0\nfunc TestParse() {}\n")
	files["CHANGELOG.md"] = []byte("## [Unreleased]\ncommitted v1.4.0\n")
	root, sha := NewCommittedTestRoot(t, files)
	path := "internal/provenance/builder_dependencies.go"
	if err := os.WriteFile(filepath.Join(root, path), []byte("dirty v1.4.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	compiler := `#!/bin/sh
set -eu
test "$(cat internal/provenance/builder_dependencies.go)" = 'committed v1.5.0'
while test "$1" != -o; do shift; done
shift
printf '%s\n' '#!/bin/sh' 'exit 0' > "$1"
chmod 700 "$1"
`
	if err := os.WriteFile(filepath.Join(tools, "go"), []byte(compiler), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	changes, err := generateTrusted(t.Context(), Generation{inputs: Inputs{TrustedRoot: root, TrustedSHA: sha}, prepared: prepared{transition: transition, release: Release{ReleaseDate: "2026-11-10", GoVersion: "1.27.0"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if change.Path == path {
			if !bytes.Equal(change.Before, files[path]) || string(change.After) != "committed v1.5.0\n" {
				t.Fatalf("dirty content influenced generation: %+v", change)
			}
			t.Log("scratch and original use committed bytes despite dirty allowlisted file")
			return
		}
	}
	t.Fatal("missing generated change")
}
