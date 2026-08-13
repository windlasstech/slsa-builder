package npmprofile

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSourceRefTag(t *testing.T) {
	t.Parallel()

	remote, commit := sourceRefTestRemote(t)
	cases := []struct {
		name      string
		remote    string
		sourceRef string
		want      string
		wantError bool
	}{
		{name: "lightweight tag", remote: remote, sourceRef: "refs/tags/v1.2.3", want: commit},
		{name: "annotated tag", remote: remote, sourceRef: "refs/tags/v1.2.4", want: commit},
		{name: "missing tag", remote: remote, sourceRef: "refs/tags/v9.9.9", wantError: true},
		{name: "unreachable remote", remote: filepath.Join(t.TempDir(), "missing.git"), sourceRef: "refs/tags/v1.2.3", wantError: true},
		{name: "non-commit tag", remote: remote, sourceRef: "refs/tags/blob", wantError: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveSourceRefTag(context.Background(), test.sourceRef, "refs/heads/main", test.remote)
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), IDSourceRefInvalid) {
					t.Fatalf("ResolveSourceRefTag() error = %v, want %s", err, IDSourceRefInvalid)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveSourceRefTag() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ResolveSourceRefTag() = %q, want %q", got, test.want)
			}
		})
	}
}

func sourceRefTestRemote(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	remote := filepath.Join(root, "remote.git")
	runSourceRefGit(t, root, "init", "--bare", remote)
	runSourceRefGit(t, root, "init", work)
	runSourceRefGit(t, work, "config", "user.name", "Fixture")
	runSourceRefGit(t, work, "config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(work, "README"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runSourceRefGit(t, work, "add", "README")
	runSourceRefGit(t, work, "commit", "-m", "fixture")
	commit := strings.TrimSpace(runSourceRefGit(t, work, "rev-parse", "HEAD"))
	runSourceRefGit(t, work, "tag", "v1.2.3")
	runSourceRefGit(t, work, "tag", "-a", "v1.2.4", "-m", "annotated fixture")
	blob := strings.TrimSpace(runSourceRefGitInput(t, work, "blob fixture\n", "hash-object", "-w", "--stdin"))
	runSourceRefGit(t, work, "tag", "blob", blob)
	runSourceRefGit(t, work, "remote", "add", "origin", remote)
	runSourceRefGit(t, work, "push", "origin", "HEAD", "refs/tags/v1.2.3", "refs/tags/v1.2.4", "refs/tags/blob")
	return remote, commit
}

func runSourceRefGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	return runSourceRefGitInput(t, directory, "", args...)
}

func runSourceRefGitInput(t *testing.T, directory, input string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	command.Stdin = strings.NewReader(input)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(errors.Join(err, errors.New(string(output))))
	}
	return string(output)
}
