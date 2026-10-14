package npmprofile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

// Official runtime tarballs are roughly 30-50 MiB; leave bounded growth headroom.
const maxNodeDistribution = 128 << 20

type nodeProvisionSource struct {
	pin   toolchainpins.NodeRuntime
	fetch distributionFetcher
}

// NodeProvisionError identifies a provisioning failure without exposing input bytes.
type NodeProvisionError struct{ id string }

func (err *NodeProvisionError) Error() string { return err.id }

// DiagnosticID returns the registered runtime provisioning diagnostic.
func (err *NodeProvisionError) DiagnosticID() string { return err.id }

func nodeProvisionFailure(suffix string) error {
	return &NodeProvisionError{id: "windlass.verify.error.node-runtime-provision-" + suffix}
}

// ProvisionNode authenticates and provisions the embedded runtime without executing it.
// The caller retains ownership of the successful private directory for the job.
func ProvisionNode(ctx context.Context, outputDirectory string) (string, error) {
	pin, err := toolchainpins.NodeRuntimePair()
	if err != nil {
		return "", nodeProvisionFailure("download")
	}
	return provisionNode(ctx, outputDirectory, nodeProvisionSource{pin: pin, fetch: fetchHTTPS})
}

func provisionNode(ctx context.Context, outputDirectory string, source nodeProvisionSource) (binDir string, resultErr error) {
	if err := ctx.Err(); err != nil {
		return "", nodeProvisionFailure("download")
	}
	artifact := source.pin.Artifacts["linux-x64"]
	encoded, err := source.fetch(ctx, artifact.URL, maxNodeDistribution, "application/octet-stream")
	if err != nil || len(encoded) > maxNodeDistribution {
		return "", nodeProvisionFailure("download")
	}
	sum := sha256.Sum256(encoded)
	if hex.EncodeToString(sum[:]) != artifact.SHA256 {
		return "", nodeProvisionFailure("digest-mismatch")
	}
	if outputDirectory == "" {
		return "", nodeProvisionFailure("archive-unsafe")
	}
	if err := os.MkdirAll(outputDirectory, 0o700); err != nil {
		return "", nodeProvisionFailure("archive-unsafe")
	}
	root, err := filepath.Abs(outputDirectory)
	if err != nil {
		return "", nodeProvisionFailure("archive-unsafe")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", nodeProvisionFailure("archive-unsafe")
	}
	private, err := os.MkdirTemp(root, "node-provision-")
	if err != nil {
		return "", nodeProvisionFailure("archive-unsafe")
	}
	defer func() {
		if resultErr != nil {
			if err := os.RemoveAll(private); err != nil {
				resultErr = errors.Join(resultErr, errors.New("remove partial Node.js provisioning directory failed"))
			}
		}
	}()
	name, err := extractNodeRuntime(ctx, encoded, private)
	if err != nil {
		if errors.Is(err, errNodeLayout) {
			return "", nodeProvisionFailure("layout-violation")
		}
		return "", nodeProvisionFailure("archive-unsafe")
	}
	runtimeRoot := filepath.Join(private, name)
	if err := verifyNodeRuntimeLayout(runtimeRoot); err != nil {
		return "", err
	}
	if name != "node-v"+source.pin.NodeVersion+"-linux-x64" {
		return "", nodeProvisionFailure("version-mismatch")
	}
	manifest, err := readBoundedRegularFile(filepath.Join(runtimeRoot, "lib/node_modules/npm/package.json"), maxManifestSize)
	if err != nil || canonicaljson.Validate(manifest) != nil {
		return "", nodeProvisionFailure("version-mismatch")
	}
	// The upstream manifest is extensible; consume only its strict, duplicate-free version member.
	var object map[string]json.RawMessage
	var version string
	if json.Unmarshal(manifest, &object) != nil || json.Unmarshal(object["version"], &version) != nil || version != source.pin.NPMVersion {
		return "", nodeProvisionFailure("version-mismatch")
	}
	return filepath.Join(private, name, "bin"), nil
}

func verifyNodeRuntimeLayout(directory string) error {
	for subdir, expected := range map[string]map[string]bool{
		"":    {"bin": true, "include": true, "lib": true, "share": true, "CHANGELOG.md": false, "LICENSE": false, "README.md": false},
		"bin": {"node": false, "npm": false, "npx": false, "corepack": false},
	} {
		entries, err := os.ReadDir(filepath.Join(directory, subdir))
		if err != nil || len(entries) != len(expected) {
			return nodeProvisionFailure("layout-violation")
		}
		for _, entry := range entries {
			isDir, found := expected[entry.Name()]
			if !found || entry.IsDir() != isDir {
				return nodeProvisionFailure("layout-violation")
			}
			info, err := entry.Info()
			if err != nil {
				return nodeProvisionFailure("layout-violation")
			}
			if subdir == "bin" && entry.Name() != "node" {
				if info.Mode()&os.ModeSymlink == 0 {
					return nodeProvisionFailure("layout-violation")
				}
			} else if !isDir && !info.Mode().IsRegular() {
				return nodeProvisionFailure("layout-violation")
			}
			if subdir == "bin" && entry.Name() == "node" && info.Mode().Perm()&0o100 == 0 {
				return nodeProvisionFailure("layout-violation")
			}
		}
	}
	return filepath.WalkDir(directory, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nodeProvisionFailure("layout-violation")
		}
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			relative, err := filepath.Rel(directory, name)
			if err != nil {
				return nodeProvisionFailure("layout-violation")
			}
			expected := nodeRuntimeLink(filepath.ToSlash(relative))
			target, err := os.Readlink(name)
			if err != nil || expected == "" || target != expected {
				return nodeProvisionFailure("layout-violation")
			}
			resolved, err := filepath.EvalSymlinks(name)
			if err != nil {
				return nodeProvisionFailure("layout-violation")
			}
			within, err := filepath.Rel(directory, resolved)
			if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
				return nodeProvisionFailure("layout-violation")
			}
			info, err := os.Stat(resolved)
			if err != nil || !info.Mode().IsRegular() {
				return nodeProvisionFailure("layout-violation")
			}
		case entry.IsDir(), entry.Type().IsRegular():
		default:
			return nodeProvisionFailure("layout-violation")
		}
		return nil
	})
}
