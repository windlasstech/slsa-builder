package npmprofile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	maxCommandOutput = 1 << 20
	commandTimeout   = 10 * time.Minute
	commandWaitDelay = 5 * time.Second
)

const idTrustedCoreBoundaryViolation = "windlass.verify.error.trusted-core-boundary-violation"

// allowedExecutableBasenames is the closed set of toolchain binaries the
// execution boundary may run. The resolved executable must also be contained
// by one of the independently derived trusted roots below.
var allowedExecutableBasenames = map[string]bool{
	"node":     true,
	"npm":      true,
	"npx":      true,
	"corepack": true,
	"pnpm":     true,
	"yarn":     true,
}

type commandBoundaryOptions struct {
	shimDirectory        string
	maximumOutput        int
	rejectOutputOverflow bool
}

type commandBoundaryError struct{ cause error }

func (boundaryError *commandBoundaryError) Error() string {
	return idTrustedCoreBoundaryViolation + ": " + boundaryError.cause.Error()
}

func (boundaryError *commandBoundaryError) Unwrap() error { return boundaryError.cause }

func (boundaryError *commandBoundaryError) DiagnosticID() string {
	return idTrustedCoreBoundaryViolation
}

func runCommand(ctx context.Context, directory, executable string, environment, arguments []string) (string, error) {
	return runCommandAtBoundary(ctx, directory, executable, environment, arguments, commandBoundaryOptions{
		maximumOutput:        maxCommandOutput,
		rejectOutputOverflow: true,
	})
}

func runCommandWithShimRoot(ctx context.Context, directory, executable string, environment, arguments []string, shimDirectory string) (string, error) {
	return runCommandAtBoundary(ctx, directory, executable, environment, arguments, commandBoundaryOptions{
		shimDirectory:        shimDirectory,
		maximumOutput:        maxCommandOutput,
		rejectOutputOverflow: true,
	})
}

func runPublishCommand(ctx context.Context, directory, executable string, environment, arguments []string) (string, error) {
	return runCommandAtBoundary(ctx, directory, executable, environment, arguments, commandBoundaryOptions{
		maximumOutput: maxNPMOutputSize,
	})
}

func runCommandAtBoundary(ctx context.Context, directory, executable string, environment, arguments []string, options commandBoundaryOptions) (string, error) {
	resolvedExecutable, err := validateExecutableContainment(executable, options.shimDirectory)
	if err != nil {
		return "", err
	}
	maximumOutput := options.maximumOutput
	if maximumOutput <= 0 {
		maximumOutput = maxCommandOutput
	}
	commandContext, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command -- this is the single audited process boundary; the executable has an absolute allowlisted name, its symlinks resolve inside an ordered trusted toolchain root, and argv is supplied only by package-owned callers.
	command := exec.CommandContext(commandContext, resolvedExecutable, arguments...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = commandWaitDelay
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	command.Dir = directory
	command.Env = environment
	output := limitedBuffer{maximum: maximumOutput}
	command.Stdout = &output
	command.Stderr = &output
	runErr := command.Run()
	if command.Process != nil {
		cleanupErr := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if cleanupErr != nil && !errors.Is(cleanupErr, syscall.ESRCH) && runErr == nil {
			runErr = fmt.Errorf("terminate command descendants: %w", cleanupErr)
		}
	}
	trimmedOutput := strings.TrimSpace(output.String())
	if runErr != nil {
		if errors.Is(commandContext.Err(), context.DeadlineExceeded) {
			return trimmedOutput, fmt.Errorf("command timed out: %w", commandContext.Err())
		}
		return trimmedOutput, fmt.Errorf("%s %v failed: %w: %s", filepath.Base(executable), arguments, runErr, trimmedOutput)
	}
	if options.rejectOutputOverflow && output.total > maximumOutput {
		return "", fmt.Errorf("command output exceeds %d bytes", maximumOutput)
	}
	return trimmedOutput, nil
}

func validateExecutableContainment(executable, shimDirectory string) (string, error) {
	if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable {
		return "", &commandBoundaryError{cause: fmt.Errorf("executable path must be absolute and canonical: %q", executable)}
	}
	if !allowedExecutableBasenames[filepath.Base(executable)] {
		return "", &commandBoundaryError{cause: fmt.Errorf("executable is not in the toolchain allowlist: %q", filepath.Base(executable))}
	}
	resolvedExecutable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", &commandBoundaryError{cause: fmt.Errorf("resolve executable symlinks: %w", err)}
	}
	resolvedExecutable, err = filepath.Abs(resolvedExecutable)
	if err != nil {
		return "", &commandBoundaryError{cause: fmt.Errorf("resolve executable path: %w", err)}
	}
	roots := trustedToolchainRoots(shimDirectory)
	for _, root := range roots {
		if pathWithinRoot(resolvedExecutable, root) {
			return resolvedExecutable, nil
		}
	}
	return "", &commandBoundaryError{cause: fmt.Errorf("executable %q resolves outside all trusted toolchain roots", executable)}
}

func trustedToolchainRoots(shimDirectory string) []string {
	// Root order is deliberate: prefer the per-build Corepack shim we create,
	// then the GitHub runner cache, then mise's managed installs for local
	// development, and finally node's resolved bin directory as a same-toolchain
	// anchor for adjacent npm/Corepack shims on hosted runners.
	candidates := []string{shimDirectory, os.Getenv("RUNNER_TOOL_CACHE")}
	miseRoot := os.Getenv("MISE_DATA_DIR")
	if miseRoot == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			miseRoot = filepath.Join(home, ".local", "share", "mise")
		}
	}
	candidates = append(candidates, miseRoot)
	if nodePath, err := exec.LookPath("node"); err == nil {
		if resolvedNode, resolveErr := filepath.EvalSymlinks(nodePath); resolveErr == nil {
			candidates = append(candidates, filepath.Dir(resolvedNode))
		}
	}

	roots := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" || !filepath.IsAbs(candidate) {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			roots = append(roots, resolved)
		}
	}
	return roots
}

func pathWithinRoot(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

type limitedBuffer struct {
	buffer  bytes.Buffer
	total   int
	maximum int
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	buffer.total += len(data)
	if buffer.buffer.Len() < buffer.maximum {
		remaining := buffer.maximum - buffer.buffer.Len()
		_, _ = buffer.buffer.Write(data[:min(len(data), remaining)])
	}
	return len(data), nil
}

func (buffer *limitedBuffer) String() string { return buffer.buffer.String() }
