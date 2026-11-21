package dependabot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
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

func TestEditChangelogSynchronizesVersionBump(t *testing.T) {
	t.Parallel()
	transition := Transition{
		old:       toolchainpins.SigningAdapterModule{Version: "v1.4.0", DirHash1: "old-digest", Sum: "h1:old-sum"},
		candidate: toolchainpins.SigningAdapterModule{Version: "v1.5.0", DirHash1: "new-digest", Sum: "h1:new-sum"},
	}
	tests := []struct {
		name, input, want string
	}{
		{
			name: "creates changed section without an existing signer reference",
			input: `## [Unreleased]

### Added

- Existing addition.

### Security

- Existing security entry.

## [0.1.0] - 12026-01-01
`,
			want: `## [Unreleased]

### Added

- Existing addition.

### Changed

- Updated sigstore-go from v1.4.0 to v1.5.0.

### Security

- Existing security entry.

## [0.1.0] - 12026-01-01
`,
		},
		{
			name: "appends after the last existing changed entry",
			input: `## [Unreleased]

### Added

- Existing addition.

### Changed

- First change.
- Last existing change.

### Fixed

- Existing fix.
`,
			want: `## [Unreleased]

### Added

- Existing addition.

### Changed

- First change.
- Last existing change.
- Updated sigstore-go from v1.4.0 to v1.5.0.

### Fixed

- Existing fix.
`,
		},
		{
			name: "advances the existing bump target without adding an entry",
			input: `## [Unreleased]

### Changed

- Updated sigstore-go from v1.3.0 to v1.4.0.
`,
			want: `## [Unreleased]

### Changed

- Updated sigstore-go from v1.3.0 to v1.5.0.
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given unreleased changelog inputs with different category boundaries.
			prepared := prepared{transition: transition}

			// When the companion edits the unreleased changelog.
			edited, err := editSite("CHANGELOG.md", []byte(test.input), prepared)

			// Then the bump entry appears at the required category boundary.
			if err != nil {
				t.Fatal(err)
			}
			if string(edited) != test.want {
				t.Fatalf("changelog mismatch:\n got:\n%s\nwant:\n%s", edited, test.want)
			}
		})
	}
}
