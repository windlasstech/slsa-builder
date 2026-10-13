package npmprofile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

const maxNPMDistribution = 64 << 20

type npmProvisionSource struct {
	pin   toolchainpins.PublishNPM
	fetch distributionFetcher
}

// NPMProvisionError identifies a pre-execution provisioning failure without exposing input bytes.
type NPMProvisionError struct{ id string }

func (err *NPMProvisionError) Error() string { return err.id }

// DiagnosticID returns the registered provisioning diagnostic.
func (err *NPMProvisionError) DiagnosticID() string { return err.id }

func npmProvisionFailure(suffix string) error {
	return &NPMProvisionError{id: "windlass.verify.error.npm-provision-" + suffix}
}

// ProvisionNPM authenticates and extracts the embedded npm pin without executing npm or node.
// The caller owns the returned launcher's private directory for the remainder of the job.
func ProvisionNPM(ctx context.Context, outputDirectory string) (string, error) {
	pin, err := toolchainpins.PublishPin()
	if err != nil {
		return "", npmProvisionFailure("download")
	}
	return provisionNPM(ctx, outputDirectory, npmProvisionSource{pin: pin, fetch: fetchHTTPS})
}

func provisionNPM(ctx context.Context, outputDirectory string, source npmProvisionSource) (launcher string, resultErr error) {
	if err := ctx.Err(); err != nil {
		return "", npmProvisionFailure("download")
	}
	encoded, err := source.fetch(ctx, source.pin.TarballURL, maxNPMDistribution, "application/octet-stream")
	if err != nil || len(encoded) > maxNPMDistribution {
		return "", npmProvisionFailure("download")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(source.pin.Integrity, "sha512-"))
	var expected digest.SHA512
	if err != nil || len(decoded) != len(expected) {
		return "", npmProvisionFailure("digest-mismatch")
	}
	copy(expected[:], decoded)
	if !digest.SumSHA512(encoded).Equal(expected) {
		return "", npmProvisionFailure("digest-mismatch")
	}
	if outputDirectory == "" {
		return "", npmProvisionFailure("archive-unsafe")
	}
	if err := os.MkdirAll(outputDirectory, 0o700); err != nil {
		return "", npmProvisionFailure("archive-unsafe")
	}
	root, err := filepath.Abs(outputDirectory)
	if err != nil {
		return "", npmProvisionFailure("archive-unsafe")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", npmProvisionFailure("archive-unsafe")
	}
	private, err := os.MkdirTemp(root, "npm-provision-")
	if err != nil {
		return "", npmProvisionFailure("archive-unsafe")
	}
	defer func() {
		if resultErr != nil {
			if err := os.RemoveAll(private); err != nil {
				resultErr = errors.Join(resultErr, errors.New("remove partial npm provisioning directory failed"))
			}
		}
	}()
	if err := extractNPMDistribution(ctx, encoded, private); err != nil {
		return "", npmProvisionFailure("archive-unsafe")
	}
	manifest, err := readBoundedRegularFile(filepath.Join(private, "package", "package.json"), maxManifestSize)
	if err != nil || canonicaljson.Validate(manifest) != nil {
		return "", npmProvisionFailure("version-mismatch")
	}
	// npm package manifests are extensible; only the exact version member is consumed here.
	var object map[string]json.RawMessage
	var version string
	if json.Unmarshal(manifest, &object) != nil || json.Unmarshal(object["version"], &version) != nil || version != source.pin.Version {
		return "", npmProvisionFailure("version-mismatch")
	}
	cli := filepath.Join(private, "package", "bin", "npm-cli.js")
	info, err := os.Lstat(cli)
	if err != nil || !info.Mode().IsRegular() {
		return "", npmProvisionFailure("version-mismatch")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return "", npmProvisionFailure("version-mismatch")
	}
	node, err = filepath.Abs(node)
	if err != nil {
		return "", npmProvisionFailure("version-mismatch")
	}
	node, err = filepath.EvalSymlinks(node)
	if err != nil {
		return "", npmProvisionFailure("version-mismatch")
	}
	launcher = filepath.Join(private, "npm")
	// Single quoting prevents shell expansion even in caller-supplied directory names.
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nexec " + quote(node) + " " + quote(cli) + " \"$@\"\n"
	if err := os.WriteFile(launcher, []byte(script), 0o700); err != nil {
		return "", npmProvisionFailure("archive-unsafe")
	}
	return launcher, nil
}
