package dependabot

import (
	"bytes"
	"testing"
)

func TestCommittedTreeRejectsNonregularAndOversizedEntries(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"symlink", "submodule", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			data := []byte("content\n")
			if kind == "oversized" {
				data = bytes.Repeat([]byte("x"), maxInput+1)
			}
			root, sha := NewCommittedTestRoot(t, map[string][]byte{"payload": data})
			if kind != "oversized" {
				mode, object := "120000", testGit(t, root, "rev-parse", sha+":payload")
				if kind == "submodule" {
					mode, object = "160000", sha
				}
				testGit(t, root, "update-index", "--cacheinfo", mode, object, "payload")
				tree := testGit(t, root, "write-tree")
				sha = testGit(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree, "-m", "nonregular fixture")
			}
			_, err := readCommittedFiles(t.Context(), Inputs{TrustedRoot: root, TrustedSHA: sha}, nil)
			if err == nil {
				t.Fatal("accepted invalid committed entry")
			}
			if kind == "oversized" {
				_, err := trustedGit(t.Context(), root, []string{"show", sha + ":payload"})
				if err == nil {
					t.Fatal("accepted oversized Git command output")
				}
			}
		})
	}
}
