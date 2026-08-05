package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

const maxTypedInputSize = 16 << 20

// RedactedValue replaces secret material in human-readable errors and diagnostics.
const RedactedValue = "[REDACTED]"

var (
	outputNamePattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
	authorizationValue = regexp.MustCompile(`(?i)(authorization\s*:\s*)(?:bearer|basic)\s+[^\s,;]+`)
	tokenValue         = regexp.MustCompile(`(?i)\b(?:github_pat_|gh[oprsu]_|npm_)[A-Za-z0-9_=-]+`)
	privateKeyValue    = regexp.MustCompile(`(?is)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)
)

// OutputAllowlist is the closed set of names a command may append to GITHUB_OUTPUT.
type OutputAllowlist map[string]struct{}

// NewOutputAllowlist constructs and validates a closed GitHub output name set.
func NewOutputAllowlist(names ...string) OutputAllowlist {
	allowlist := make(OutputAllowlist, len(names))
	for _, name := range names {
		if !outputNamePattern.MatchString(name) {
			panic("invalid GitHub output allowlist name: " + name)
		}
		allowlist[name] = struct{}{}
	}
	return allowlist
}

// ReadTypedJSON reads one regular, non-symlink JSON file using a closed schema.
func ReadTypedJSON[T any](path string, validate func(T) error) (T, error) {
	var zero T
	data, err := readRegularFile(path)
	if err != nil {
		return zero, err
	}
	value, err := decodeTypedJSON(data, validate)
	if err != nil {
		return zero, fmt.Errorf("decode typed input %q: %w", path, err)
	}
	return value, nil
}

func decodeTypedJSON[T any](data []byte, validate func(T) error) (T, error) {
	var zero T
	if err := canonicaljson.Validate(data); err != nil {
		return zero, fmt.Errorf("validate JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value T
	if err := decoder.Decode(&value); err != nil {
		return zero, fmt.Errorf("decode closed JSON schema: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return zero, err
	}
	if validate != nil {
		if err := validate(value); err != nil {
			return zero, err
		}
	}
	return value, nil
}

func readRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect typed input %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("typed input %q must be a regular non-symlink file", path)
	}
	if info.Size() > maxTypedInputSize {
		return nil, fmt.Errorf("typed input %q exceeds %d bytes", path, maxTypedInputSize)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open typed input %q: %w", path, err)
	}
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return nil, fmt.Errorf("typed input %q changed while opening", path)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxTypedInputSize+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(data) > maxTypedInputSize {
		return nil, fmt.Errorf("typed input %q exceeds %d bytes", path, maxTypedInputSize)
	}
	return data, nil
}

// WriteFileAtomic replaces path only after the complete output is durable in the same directory.
func WriteFileAtomic(path string, data []byte, permission fs.FileMode) (err error) {
	if permission.Perm() != permission || permission&0o077 != 0 {
		return fmt.Errorf("atomic output permission must not grant group or other access")
	}
	if info, statErr := os.Lstat(path); statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("atomic output %q must be a regular non-symlink file", path)
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("inspect atomic output %q: %w", path, statErr)
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".slsa-builder-*")
	if err != nil {
		return fmt.Errorf("create atomic output in %q: %w", directory, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		closeErr := temporary.Close()
		removeErr := os.Remove(temporaryPath)
		if !errors.Is(removeErr, fs.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
		if err != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if err := temporary.Chmod(permission); err != nil {
		return fmt.Errorf("set atomic output permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write atomic output: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync atomic output: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close atomic output: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace atomic output %q: %w", path, err)
	}
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open atomic output directory: %w", err)
	}
	syncErr := directoryHandle.Sync()
	closeErr := directoryHandle.Close()
	if syncErr != nil || closeErr != nil {
		return errors.Join(syncErr, closeErr)
	}
	return nil
}

// WriteGitHubOutputs appends sorted, allowlisted, single-line, secret-safe file commands.
func WriteGitHubOutputs(path string, allowlist OutputAllowlist, outputs map[string]string) error {
	if path == "" {
		return errors.New("GITHUB_OUTPUT path is required")
	}
	if len(outputs) == 0 {
		return nil
	}
	names := make([]string, 0, len(outputs))
	for name, value := range outputs {
		if _, allowed := allowlist[name]; !allowed {
			return fmt.Errorf("GitHub output %q is not allowlisted", name)
		}
		if !outputNamePattern.MatchString(name) {
			return fmt.Errorf("GitHub output name is invalid")
		}
		if strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("GitHub output %q contains a file-command delimiter", name)
		}
		if containsSecret(value) {
			return fmt.Errorf("GitHub output %q contains secret-like material", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var encoded strings.Builder
	for _, name := range names {
		fmt.Fprintf(&encoded, "%s=%s\n", name, outputs[name])
	}
	file, err := openAppendRegular(path)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
	}
	_, writeErr := io.WriteString(file, encoded.String())
	closeErr := file.Close()
	return errors.Join(writeErr, closeErr)
}

func openAppendRegular(path string) (*os.File, error) {
	for range 2 {
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|os.O_APPEND, 0o600)
			if errors.Is(createErr, fs.ErrExist) {
				continue
			}
			return file, createErr
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("file must be a regular non-symlink file")
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			return nil, err
		}
		openedInfo, statErr := file.Stat()
		if statErr != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
			_ = file.Close()
			return nil, errors.New("file changed while opening")
		}
		return file, nil
	}
	return nil, errors.New("file changed while creating")
}

// RedactSecrets removes explicit secrets and common credential forms from a message.
func RedactSecrets(message string, secrets ...string) string {
	ordered := slices.Clone(secrets)
	sort.Slice(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	for _, secret := range ordered {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, RedactedValue)
		}
	}
	message = authorizationValue.ReplaceAllString(message, `${1}`+RedactedValue)
	message = tokenValue.ReplaceAllString(message, RedactedValue)
	return privateKeyValue.ReplaceAllString(message, RedactedValue)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func containsSecret(value string) bool {
	redacted := RedactSecrets(value)
	if redacted != value {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.User != nil
}
