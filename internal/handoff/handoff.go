// Package handoff verifies digest-bound, single-file GitHub Actions artifact handoffs.
// A successful verification returns the exact payload bytes whose digest matched the contract;
// callers never receive a mutable pathname as the verified object.
package handoff

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

const (
	// DigestMismatchID is the primary diagnostic for recomputed digest mismatches.
	DigestMismatchID DiagnosticID = "windlass.verify.error.digest-mismatch"
	// HandoffSchemaMismatchID identifies missing or malformed core handoff fields.
	HandoffSchemaMismatchID DiagnosticID = "windlass.verify.error.handoff-schema-mismatch"
)

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

type digestJSON struct {
	Algorithm *string        `json:"algorithm"`
	Value     *digest.SHA256 `json:"value"`
}

type handoffJSON struct {
	Transport       *string     `json:"transport"`
	ArtifactName    *string     `json:"artifact_name"`
	PayloadFileName *string     `json:"payload_file_name"`
	PayloadKind     *string     `json:"payload_kind"`
	Digest          *digestJSON `json:"digest"`
}

// UnmarshalJSON strictly decodes and validates every required core handoff field.
func (contract *Handoff) UnmarshalJSON(encoded []byte) error {
	var decodedJSON handoffJSON
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decodedJSON); err != nil {
		return schemaMismatchError(fmt.Errorf("decode handoff JSON: %w", err))
	}
	if decodedJSON.Transport == nil || decodedJSON.ArtifactName == nil ||
		decodedJSON.PayloadFileName == nil || decodedJSON.PayloadKind == nil ||
		decodedJSON.Digest == nil || decodedJSON.Digest.Algorithm == nil ||
		decodedJSON.Digest.Value == nil {
		return schemaMismatchError(errors.New("handoff JSON is missing a required core field"))
	}

	decoded := Handoff{
		Transport:       *decodedJSON.Transport,
		ArtifactName:    *decodedJSON.ArtifactName,
		PayloadFileName: *decodedJSON.PayloadFileName,
		PayloadKind:     *decodedJSON.PayloadKind,
		Digest: Digest{
			Algorithm: *decodedJSON.Digest.Algorithm,
			Value:     *decodedJSON.Digest.Value,
		},
	}
	if err := decoded.Validate(); err != nil {
		return schemaMismatchError(fmt.Errorf("validate handoff JSON: %w", err))
	}
	*contract = decoded
	return nil
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

// VerifyOneFile validates a one-file artifact directory and returns its verified payload bytes.
func VerifyOneFile(directory string, contract Handoff) ([]byte, error) {
	if err := contract.Validate(); err != nil {
		return nil, schemaMismatchError(fmt.Errorf("validate handoff: %w", err))
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, fmt.Errorf("open artifact directory: %w", err)
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, closeRoot(root, fmt.Errorf("read artifact directory: %w", err))
	}
	if len(entries) != 1 {
		return nil, closeRoot(root, fmt.Errorf("artifact directory contains %d entries, want exactly 1", len(entries)))
	}

	entry := entries[0]
	if entry.Name() != contract.PayloadFileName {
		return nil, closeRoot(root, fmt.Errorf("artifact payload file is %q, want %q", entry.Name(), contract.PayloadFileName))
	}
	info, err := root.Lstat(entry.Name())
	if err != nil {
		return nil, closeRoot(root, fmt.Errorf("inspect artifact payload: %w", err))
	}
	if !info.Mode().IsRegular() {
		return nil, closeRoot(root, fmt.Errorf("artifact payload %q is not a regular file", entry.Name()))
	}

	payload, err := root.Open(entry.Name())
	if err != nil {
		return nil, closeRoot(root, fmt.Errorf("open artifact payload: %w", err))
	}
	if closeErr := closeRoot(root, nil); closeErr != nil {
		if payloadCloseErr := payload.Close(); payloadCloseErr != nil {
			return nil, errors.Join(closeErr, fmt.Errorf("close artifact payload: %w", payloadCloseErr))
		}
		return nil, closeErr
	}

	payloadBytes, readErr := io.ReadAll(payload)
	payloadCloseErr := payload.Close()
	if readErr != nil {
		return nil, errors.Join(
			fmt.Errorf("read artifact payload: %w", readErr),
			wrapCloseError(payloadCloseErr, "close artifact payload"),
		)
	}
	if payloadCloseErr != nil {
		return nil, fmt.Errorf("close artifact payload: %w", payloadCloseErr)
	}
	actual := digest.SumSHA256(payloadBytes)
	if !actual.Equal(contract.Digest.Value) {
		return nil, &VerificationError{
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

	return payloadBytes, nil
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

func schemaMismatchError(err error) error {
	return &VerificationError{
		PrimaryID: HandoffSchemaMismatchID,
		Err:       fmt.Errorf("handoff schema mismatch: %w", err),
	}
}

func closeRoot(root *os.Root, err error) error {
	if closeErr := root.Close(); closeErr != nil {
		return errors.Join(err, fmt.Errorf("close artifact directory: %w", closeErr))
	}
	return err
}

func wrapCloseError(err error, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
