package npmprofile

import (
	"context"
	"strings"
	"testing"
)

// TestResolveSourceRefTagArgumentRejections pins the fail-closed argument
// contract of ResolveSourceRefTag: a well-formed tag still requires a remote
// and a resolution context, and every rejection carries the classified
// source-ref-invalid diagnostic before any git operation runs.
func TestResolveSourceRefTagArgumentRejections(t *testing.T) {
	t.Parallel()

	t.Run("source-ref omitted with a remote", func(t *testing.T) {
		t.Parallel()
		_, err := ResolveSourceRefTag(t.Context(), "", "refs/heads/main", "https://example.invalid/remote.git")
		requireSourceRefInvalid(t, err)
	})

	t.Run("remote required", func(t *testing.T) {
		t.Parallel()
		_, err := ResolveSourceRefTag(t.Context(), "refs/tags/v1.2.3", "refs/heads/main", "")
		requireSourceRefInvalid(t, err)
	})

	t.Run("context required", func(t *testing.T) {
		t.Parallel()
		var nilContext context.Context
		_, err := ResolveSourceRefTag(nilContext, "refs/tags/v1.2.3", "refs/heads/main", "https://example.invalid/remote.git")
		requireSourceRefInvalid(t, err)
	})
}

func requireSourceRefInvalid(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), IDSourceRefInvalid) {
		t.Fatalf("ResolveSourceRefTag() error = %v, want %s", err, IDSourceRefInvalid)
	}
}

// TestValidTagName pins the closed git tag-name grammar behind the tags-only
// source-ref contract: the structural rejections (empty, @, slash and dot
// edges, //, .., @{, .lock) and the per-character rejections (control bytes,
// DEL, space, and ~^:?*[ backslash) all fail, while ordinary release tags
// pass.
func TestValidTagName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		tag   string
		valid bool
	}{
		{name: "empty", tag: "", valid: false},
		{name: "lone at", tag: "@", valid: false},
		{name: "leading slash", tag: "/v1.2.3", valid: false},
		{name: "trailing slash", tag: "v1.2.3/", valid: false},
		{name: "trailing dot", tag: "v1.2.3.", valid: false},
		{name: "double slash", tag: "release//v1.2.3", valid: false},
		{name: "double dot", tag: "v1..2", valid: false},
		{name: "reflog syntax", tag: "v1@{0}", valid: false},
		{name: "lock suffix", tag: "v1.2.3.lock", valid: false},
		{name: "space", tag: "v1 2", valid: false},
		{name: "tilde", tag: "v1~2", valid: false},
		{name: "caret", tag: "v1^2", valid: false},
		{name: "colon", tag: "v1:2", valid: false},
		{name: "question mark", tag: "v1?2", valid: false},
		{name: "asterisk", tag: "v1*2", valid: false},
		{name: "open bracket", tag: "v1[2", valid: false},
		{name: "backslash", tag: `v1\2`, valid: false},
		{name: "control character", tag: "v1\x1f2", valid: false},
		{name: "delete character", tag: "v1\x7f2", valid: false},
		{name: "plain release tag", tag: "v1.2.3", valid: true},
		{name: "nested release tag", tag: "release/2026.08", valid: true},
		{name: "prerelease tag", tag: "v1.2.3-rc.1", valid: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := validTagName(test.tag); got != test.valid {
				t.Fatalf("validTagName(%q) = %v, want %v", test.tag, got, test.valid)
			}
		})
	}
}
