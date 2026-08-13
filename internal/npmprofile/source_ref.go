package npmprofile

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/windlasstech/slsa-builder/internal/identity"
)

// ResolveSourceRefTag resolves a caller-supplied source-ref release tag to its commit SHA through
// an unauthenticated git ls-remote against the canonical caller repository (ADR 0079). Annotated
// tags are peeled to their commit. A tag that is missing, unresolvable, or unreachable fails
// closed with windlass.verify.error.source-ref-invalid.
func ResolveSourceRefTag(ctx context.Context, remoteURL, sourceRef string) (string, error) {
	if err := identity.ValidateReleaseRef(sourceRef); err != nil {
		return "", npmValidationError(IDSourceRefInvalid, "inputs.source-ref", "source-ref must be a full refs/tags/ release tag ref")
	}
	output, err := runGitLsRemote(ctx, remoteURL, sourceRef)
	if err != nil {
		return "", npmValidationError(IDSourceRefInvalid, "inputs.source-ref", "source-ref tag could not be listed from the caller repository")
	}
	direct, peeled := "", ""
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		switch fields[1] {
		case sourceRef:
			direct = fields[0]
		case sourceRef + "^{}":
			peeled = fields[0]
		}
	}
	commit := peeled
	if commit == "" {
		commit = direct
	}
	if commit == "" {
		return "", npmValidationError(IDSourceRefInvalid, "inputs.source-ref", "source-ref tag does not exist in the caller repository")
	}
	if identity.ValidateFullSHA(commit) != nil {
		return "", npmValidationError(IDSourceRefInvalid, "inputs.source-ref", "source-ref tag does not resolve to a commit")
	}
	return commit, nil
}

func runGitLsRemote(ctx context.Context, remoteURL, sourceRef string) (string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}
	commandContext, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command -- the executable is the resolved git binary, argv is fixed apart from the validated release ref and the canonical caller repository URL, and prompting is disabled.
	command := exec.CommandContext(commandContext, gitPath, "ls-remote", remoteURL, sourceRef, sourceRef+"^{}")
	command.Env = []string{"GIT_TERMINAL_PROMPT=0", "PATH=" + filepath.Dir(gitPath)}
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
