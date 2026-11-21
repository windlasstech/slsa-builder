package dependabot

import (
	"fmt"
	"strings"
	"testing"
)

func TestCommittedMetadataBounds(t *testing.T) {
	t.Parallel()
	entry := func(path, size string) string {
		return "100644 blob " + strings.Repeat("a", 40) + " " + size + "\t" + path + "\x00"
	}
	var count, aggregate strings.Builder
	for i := range 10000 {
		count.WriteString(entry(fmt.Sprintf("file%d", i), "0"))
	}
	for i := range 8 {
		aggregate.WriteString(entry(fmt.Sprintf("file%d", i), "16777216"))
	}
	for _, test := range []struct {
		name, raw string
		valid     bool
	}{
		{"count boundary", count.String(), true},
		{"count overflow", count.String() + entry("extra", "0"), false},
		{"aggregate boundary", aggregate.String(), true},
		{"aggregate overflow", aggregate.String() + entry("extra", "1"), false},
		{"file overflow", entry("file", "16777217"), false},
		{"duplicate", entry("file", "0") + entry("file", "0"), false},
		{"negative size", entry("file", "-1"), false},
		{"malformed size", entry("file", "x"), false},
		{"truncated", strings.TrimSuffix(entry("file", "0"), "\x00"), false},
		{"ambiguous", entry("a/../file", "0"), false},
		{"backslash", entry("a\\file", "0"), false},
		{"case collision", entry("File", "0") + entry("file", "0"), false},
		{"directory case collision", entry("Dir/a", "0") + entry("dir/b", "0"), false},
		{"file directory collision", entry("dir", "0") + entry("dir/b", "0"), false},
		{"root path", entry(".", "0"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given bounded tree metadata, when parsed, then reject before blob reads.
			_, err := parseCommittedTree([]byte(test.raw))
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}

func TestCommittedFixtureIncludesIgnoredFiles(t *testing.T) {
	t.Parallel()
	// Given an ignored module file in a synthetic trusted commit.
	root, sha := NewCommittedTestRoot(t, map[string][]byte{".gitignore": []byte("go.sum\n"), "go.mod": []byte("module example\n"), "go.sum": []byte("ignored content\n")})
	// When reading committed module inputs, then the ignored file is present.
	files, err := readCommittedFiles(t.Context(), Inputs{TrustedRoot: root, TrustedSHA: sha}, []string{"go.mod", "go.sum"})
	if err != nil || string(files["go.sum"]) != "ignored content\n" {
		t.Fatalf("files=%v error=%v", files, err)
	}
}
