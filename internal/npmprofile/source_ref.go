package npmprofile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const tagRefPrefix = "refs/tags/"

const sourceRefResolutionTimeout = 30 * time.Second

// NormalizeSourceRefInput trims the ASCII whitespace accepted around source-ref input.
func NormalizeSourceRefInput(sourceRef string) string {
	return strings.Trim(sourceRef, " \t\n\r\v\f")
}

// ValidateSourceRefInput validates the tags-only caller-selected build source intent.
func ValidateSourceRefInput(sourceRef, invocationRef, packageVersion string) error {
	sourceRef = NormalizeSourceRefInput(sourceRef)
	if sourceRef == "" {
		return nil
	}
	if !strings.HasPrefix(sourceRef, tagRefPrefix) || !validTagName(strings.TrimPrefix(sourceRef, tagRefPrefix)) {
		return sourceRefError("source-ref must be a full, well-formed tag ref")
	}
	if strings.HasPrefix(invocationRef, tagRefPrefix) && sourceRef != invocationRef {
		return sourceRefError("source-ref conflicts with the tag invocation ref")
	}
	if packageVersion != "" && sourceRef != tagRefPrefix+"v"+packageVersion {
		return sourceRefError("source-ref does not match the package version tag")
	}
	return nil
}

// ResolveSourceRefTag resolves an exact caller-selected tag and proves that it peels to a commit.
func ResolveSourceRefTag(ctx context.Context, sourceRef, invocationRef, remote string) (string, error) {
	if err := ValidateSourceRefInput(sourceRef, invocationRef, ""); err != nil {
		return "", err
	}
	if sourceRef == "" || remote == "" {
		return "", sourceRefError("source-ref and remote are required for resolution")
	}
	if ctx == nil {
		return "", sourceRefError("source-ref resolution context is required")
	}

	bounded, cancel := context.WithTimeout(ctx, sourceRefResolutionTimeout)
	defer cancel()
	repository, err := os.MkdirTemp("", "windlass-source-ref-")
	if err != nil {
		return "", sourceRefError("create isolated source-ref resolver")
	}
	defer func() { _ = os.RemoveAll(repository) }()

	if _, err := executeSourceRefGit(bounded, repository, "init", "--bare", "."); err != nil {
		return "", sourceRefError("initialize isolated source-ref resolver")
	}
	if _, err := executeSourceRefGit(bounded, repository, "fetch", "--no-tags", "--depth=1", remote, sourceRef); err != nil {
		return "", sourceRefError("source-ref tag is missing or unreachable")
	}
	commit, err := executeSourceRefGit(bounded, repository, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", sourceRefError("source-ref does not peel to a commit")
	}
	commit = strings.TrimSpace(commit)
	objectType, err := executeSourceRefGit(bounded, repository, "cat-file", "-t", commit)
	if err != nil || strings.TrimSpace(objectType) != "commit" {
		return "", sourceRefError("source-ref resolved object is not a commit")
	}
	if len(commit) != 40 || strings.Trim(commit, "0123456789abcdef") != "" {
		return "", sourceRefError("source-ref resolved commit is malformed")
	}
	return commit, nil
}

func executeSourceRefGit(ctx context.Context, directory string, arguments ...string) (string, error) {
	commandArguments := append([]string{"-c", "credential.helper="}, arguments...)
	command := exec.CommandContext(ctx, "git", commandArguments...)
	command.Dir = filepath.Clean(directory)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", errors.Join(err, fmt.Errorf("git operation failed: %s", strings.TrimSpace(string(output))))
	}
	return string(output), nil
}

func validTagName(name string) bool {
	if name == "" || name == "@" || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") ||
		strings.HasSuffix(name, ".") || strings.Contains(name, "//") || strings.Contains(name, "..") ||
		strings.Contains(name, "@{") || strings.HasSuffix(name, ".lock") {
		return false
	}
	for _, character := range name {
		if character < 0x20 || character == 0x7f || strings.ContainsRune(" ~^:?*[\\", character) {
			return false
		}
	}
	return true
}

func sourceRefError(message string) error {
	return npmValidationError(IDSourceRefInvalid, "source-ref", message)
}
