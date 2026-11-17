package dependabot

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Env = append(trustedEnvironment(), "GIT_AUTHOR_DATE=2026-11-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-11-01T00:00:00Z")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// Exported only in the test binary for the external planner tests.
func NewCommittedTestRoot(t *testing.T, files map[string][]byte) (string, string) {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "--initial-branch=main", "--template=")
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "add", "--", ".")
	tree := testGit(t, root, "write-tree")
	sha := testGit(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree, "-m", "fixture")
	testGit(t, root, "update-ref", "refs/heads/main", sha)
	return root, sha
}

func CloneCommittedTestRoot(t *testing.T, source string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "trusted")
	testGit(t, source, "clone", "--local", "--no-hardlinks", "--no-checkout", "--template=", source, root)
	sha := testGit(t, source, "rev-parse", "HEAD")
	testGit(t, root, "checkout", "--detach", sha)
	return root
}
