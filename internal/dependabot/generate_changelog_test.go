package dependabot

import (
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestChangelogPreservesUnrelatedVersions(t *testing.T) {
	t.Parallel()
	for _, signer := range []bool{false, true} {
		t.Run(map[bool]string{false: "append", true: "replace"}[signer], func(t *testing.T) {
			t.Parallel()
			// Given an unrelated same-version mention and a released signer entry.
			input := "## [Unreleased]\n\n### Changed\n\n- Other library v1.4.0.\n"
			want := input
			if signer {
				input += "- Updated sigstore-go from v1.3.0 to v1.4.0.\n"
				want += "- Updated sigstore-go from v1.3.0 to v1.5.0.\n"
			} else {
				want += "- Updated sigstore-go from v1.4.0 to v1.5.0.\n"
			}
			later := "\n## [0.1.0]\n\n- sigstore-go v1.4.0.\n"
			p := prepared{transition: Transition{old: toolchainpins.SigningAdapterModule{Version: "v1.4.0"}, candidate: toolchainpins.SigningAdapterModule{Version: "v1.5.0"}}}
			// When editing, then only the signer bump target changes.
			got, err := editSite("CHANGELOG.md", []byte(input+later), p)
			if err != nil || strings.TrimSpace(string(got)) != strings.TrimSpace(want+later) {
				t.Fatalf("got=%s error=%v", got, err)
			}
		})
	}
}
