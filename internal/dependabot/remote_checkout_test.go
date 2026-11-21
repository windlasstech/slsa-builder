package dependabot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteRejectsDirtyCheckoutBeforeNetwork(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	for _, mode := range []string{"plan", "apply"} {
		for _, state := range []string{"unstaged", "staged", "untracked", "ignored"} {
			t.Run(mode+"/"+state, func(t *testing.T) {
				f := newRemoteFixture(t)
				root, sha := NewCommittedTestRoot(t, map[string][]byte{"source.go": []byte("committed\n"), ".gitignore": []byte("ignored.go\n")})
				f.inputs.TrustedRoot, f.inputs.TrustedSHA = root, sha
				path := "source.go"
				if state == "untracked" {
					path = "extra.go"
				}
				if state == "ignored" {
					path = "ignored.go"
				}
				if err := os.WriteFile(filepath.Join(root, path), []byte("dirty\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if state == "staged" {
					testGit(t, root, "add", "--", path)
				}
				var err error
				switch mode {
				case "plan":
					_, err = f.remote.Plan(t.Context(), f.inputs)
				case "apply":
					_, err = f.remote.Apply(t.Context(), f.inputs, "companion")
				}
				if err == nil || f.reads.Load() != 0 || f.posts.Load() != 0 || f.mutations.Load() != 0 {
					t.Fatalf("error=%v reads=%d posts=%d mutations=%d", err, f.reads.Load(), f.posts.Load(), f.mutations.Load())
				}
				t.Log("dirty checkout: reads=0 posts=0 mutations=0")
			})
		}
	}
}
