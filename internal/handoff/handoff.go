// Package handoff verifies digest-bound, single-file GitHub Actions artifact handoffs.
package handoff

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

const (
	// TransportGitHubActionsArtifact is the initial same-run handoff transport.
	TransportGitHubActionsArtifact = "github-actions-artifact"
	// AlgorithmSHA256 is the required cross-job handoff digest algorithm.
	AlgorithmSHA256 = "sha256"
)

// DiagnosticID is a stable verification diagnostic identifier.
type DiagnosticID string

// DigestMismatchID is the primary diagnostic for recomputed digest mismatches.
const DigestMismatchID DiagnosticID = "windlass.verify.error.digest-mismatch"

// ErrDigestMismatch identifies payload bytes that differ from the expected digest.
var ErrDigestMismatch = errors.New("handoff digest mismatch")

// Digest is the exact digest object in the core handoff JSON contract.
type Digest struct {
	Algorithm string        `json:"algorithm"`
	Value     digest.SHA256 `json:"value"`
}

// Handoff is the semantic minimum same-run artifact handoff contract.
type Handoff struct {
	Transport       string `json:"transport"`
	ArtifactName    string `json:"artifact_name"`
	PayloadFileName string `json:"payload_file_name"`
	PayloadKind     string `json:"payload_kind"`
	Digest          Digest `json:"digest"`
}

// VerificationError carries the stable diagnostic ID for a handoff failure.
type VerificationError struct {
	PrimaryID DiagnosticID
	Err       error
}

// Error implements error.
func (verificationError *VerificationError) Error() string {
	return verificationError.Err.Error()
}

// Unwrap exposes the underlying failure for errors.Is and errors.As.
func (verificationError *VerificationError) Unwrap() error {
	return verificationError.Err
}

// DiagnosticID returns the stable primary diagnostic identifier.
func (verificationError *VerificationError) DiagnosticID() DiagnosticID {
	return verificationError.PrimaryID
}

// ErrorIDOf returns a handoff verification error's diagnostic ID, or an empty ID.
func ErrorIDOf(err error) DiagnosticID {
	var identified interface {
		DiagnosticID() DiagnosticID
	}
	if errors.As(err, &identified) {
		return identified.DiagnosticID()
	}
	return ""
}

// ValidateSafeBasename rejects path traversal, separators, absolute paths, and non-canonical names.
func ValidateSafeBasename(name string) error {
	if name == "" {
		return errors.New("payload file name is empty")
	}
	if strings.ContainsRune(name, '\x00') {
		return errors.New("payload file name contains NUL")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("payload file name %q is not a file basename", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("payload file name %q contains a path separator", name)
	}
	if filepath.IsAbs(name) || filepath.Clean(name) != name || hasWindowsDrivePrefix(name) {
		return fmt.Errorf("payload file name %q is not canonical", name)
	}
	return nil
}

// Validate checks the core handoff fields that can be proven without retrieving an artifact.
func (contract Handoff) Validate() error {
	if contract.Transport != TransportGitHubActionsArtifact {
		return fmt.Errorf("handoff transport %q is not allowed", contract.Transport)
	}
	if err := validateArtifactName(contract.ArtifactName); err != nil {
		return err
	}
	if err := ValidateSafeBasename(contract.PayloadFileName); err != nil {
		return err
	}
	if contract.PayloadKind == "" {
		return errors.New("payload kind is empty")
	}
	if contract.Digest.Algorithm != AlgorithmSHA256 {
		return fmt.Errorf("handoff digest algorithm %q is not allowed", contract.Digest.Algorithm)
	}
	return nil
}

// VerifyOneFile validates a one-file artifact directory and recomputes its SHA-256 digest.
func VerifyOneFile(directory string, contract Handoff) (string, error) {
	if err := contract.Validate(); err != nil {
		return "", fmt.Errorf("validate handoff: %w", err)
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", fmt.Errorf("open artifact directory: %w", err)
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return closeRoot(root, "", fmt.Errorf("read artifact directory: %w", err))
	}
	if len(entries) != 1 {
		return closeRoot(root, "", fmt.Errorf("artifact directory contains %d entries, want exactly 1", len(entries)))
	}

	entry := entries[0]
	if entry.Name() != contract.PayloadFileName {
		return closeRoot(root, "", fmt.Errorf("artifact payload file is %q, want %q", entry.Name(), contract.PayloadFileName))
	}
	info, err := root.Lstat(entry.Name())
	if err != nil {
		return closeRoot(root, "", fmt.Errorf("inspect artifact payload: %w", err))
	}
	if !info.Mode().IsRegular() {
		return closeRoot(root, "", fmt.Errorf("artifact payload %q is not a regular file", entry.Name()))
	}

	payload, err := root.Open(entry.Name())
	if err != nil {
		return closeRoot(root, "", fmt.Errorf("open artifact payload: %w", err))
	}
	if _, closeErr := closeRoot(root, "", nil); closeErr != nil {
		if payloadCloseErr := payload.Close(); payloadCloseErr != nil {
			return "", errors.Join(closeErr, fmt.Errorf("close artifact payload: %w", payloadCloseErr))
		}
		return "", closeErr
	}

	actual, digestErr := digest.SumSHA256Reader(payload)
	payloadCloseErr := payload.Close()
	if digestErr != nil {
		return "", errors.Join(digestErr, wrapCloseError(payloadCloseErr, "close artifact payload"))
	}
	if payloadCloseErr != nil {
		return "", fmt.Errorf("close artifact payload: %w", payloadCloseErr)
	}
	if !actual.Equal(contract.Digest.Value) {
		return "", &VerificationError{
			PrimaryID: DigestMismatchID,
			Err: fmt.Errorf(
				"%w: payload %q computed %s, expected %s",
				ErrDigestMismatch,
				entry.Name(),
				actual,
				contract.Digest.Value,
			),
		}
	}

	return filepath.Join(directory, entry.Name()), nil
}

func validateArtifactName(name string) error {
	if err := ValidateSafeBasename(name); err != nil {
		return fmt.Errorf("invalid artifact name: %w", err)
	}
	if strings.ContainsRune(name, ':') {
		return fmt.Errorf("artifact name %q resembles a URL or volume path", name)
	}
	allDigits := true
	for _, character := range name {
		if !unicode.IsDigit(character) {
			allDigits = false
			break
		}
	}
	if allDigits {
		return fmt.Errorf("artifact name %q is an ID-only handle", name)
	}
	return nil
}

func hasWindowsDrivePrefix(name string) bool {
	return len(name) >= 2 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) && name[1] == ':'
}

func closeRoot(root *os.Root, value string, err error) (string, error) {
	if closeErr := root.Close(); closeErr != nil {
		return "", errors.Join(err, fmt.Errorf("close artifact directory: %w", closeErr))
	}
	return value, err
}

func wrapCloseError(err error, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
