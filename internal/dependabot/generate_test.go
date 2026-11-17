package dependabot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegenerateCompilesBaseBeforeInstallingCandidateData(t *testing.T) {
	root := t.TempDir()
	tools := t.TempDir()
	base, err := readModules("testdata/base")
	if err != nil {
		t.Fatal(err)
	}
	head, err := readModules("testdata/head")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"provenance", "npmprofile"} {
		if err := os.MkdirAll(filepath.Join(root, "internal", pkg), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), base.Mod, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), base.Sum, 0600); err != nil {
		t.Fatal(err)
	}
	// This fake compiler refuses candidate input and emits a runner that requires it.
	script := `#!/bin/sh
set -eu
grep -q 'sigstore-go v1.4.0' go.mod
test "$GOPROXY" = off
test "$GOTOOLCHAIN" = local
test "${UPDATE_FIXTURE_INDEX-unset}" = unset
test "${UPDATE_P02_FIXTURE-unset}" = unset
while test "$1" != -o; do shift; done
shift
printf '%s\n' '#!/bin/sh' 'set -eu' "grep -q 'sigstore-go v1.5.0' ../../go.mod" 'test "$GOPROXY" = off' 'test "${UPDATE_FIXTURE_INDEX-unset}" = unset' 'test "${UPDATE_P02_FIXTURE-unset}" = unset' > "$1"
if test "${1##*/}" = provenance.test; then
  printf '%s\n' 'case "$*" in *TestSigningAdapterCandidateDrift*) ;; *) exit 1 ;; esac' >> "$1"
fi
chmod 700 "$1"
`
	if err := os.WriteFile(filepath.Join(tools, "go"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("UPDATE_FIXTURE_INDEX", "1")
	t.Setenv("UPDATE_P02_FIXTURE", "1")
	if err := regenerate(t.Context(), root, Transition{base: base, head: head}); err != nil {
		t.Fatal(err)
	}
}
