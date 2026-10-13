package npmprofile

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestProvisionNodeRetainsPrivateBinWithoutExecution(t *testing.T) {
	t.Parallel()
	// Given an authenticated tree containing deliberately non-executable program bytes.
	data := nodeRuntimeArchive(t, nodeRuntimeEntries())
	output := filepath.Join(t.TempDir(), "new-output")
	source := nodeProvisionFixture(t, data)
	fetch := source.fetch
	source.fetch = func(ctx context.Context, url string, maximum int64, accept string) ([]byte, error) {
		if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("directory created before fetch")
		}
		return fetch(ctx, url, maximum, accept)
	}
	// When provisioning completes without execution.
	bin, err := provisionNode(t.Context(), output, source)
	// Then the retained bin directory is absolute and private to this invocation.
	if err != nil {
		t.Fatal(err)
	}
	private := filepath.Dir(filepath.Dir(bin))
	if !filepath.IsAbs(bin) || bin != filepath.Join(private, nodeFixtureRoot, "bin") || !strings.HasPrefix(filepath.Base(private), "node-provision-") {
		t.Fatalf("bin path = %q", bin)
	}
	info, err := os.Stat(private)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("private mode: %v, %v", info, err)
	}
	info, err = os.Lstat(filepath.Join(bin, "node"))
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("node mode: %v, %v", info, err)
	}
}

func TestProvisionNodeDownloadFailures(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"fetch error", "oversize", "cancelled"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given a failed download and an output directory that does not exist.
			source := nodeProvisionFixture(t, nil)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			source.fetch = func(context.Context, string, int64, string) ([]byte, error) {
				if name == "oversize" {
					return make([]byte, (128<<20)+1), nil
				}
				if name == "cancelled" {
					t.Fatal("cancelled request fetched")
				}
				return nil, errors.New("secret fetch failure")
			}
			if name == "cancelled" {
				cancel()
			}
			output := filepath.Join(t.TempDir(), "absent")
			// When provisioning cannot acquire bounded bytes.
			_, err := provisionNode(ctx, output, source)
			// Then download wins before any filesystem mutation.
			requireNodeProvisionID(t, err, "download")
			if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("output created: %v", err)
			}
		})
	}
}

func TestProvisionNodeDigestMismatchBeforeDirectoryCreation(t *testing.T) {
	t.Parallel()
	// Given invalid compressed bytes that differ from the recorded digest.
	source := nodeProvisionFixture(t, []byte("authenticated"))
	source.fetch = func(context.Context, string, int64, string) ([]byte, error) { return []byte("corrupt"), nil }
	output := filepath.Join(t.TempDir(), "absent")
	// When provisioning verifies the compressed digest.
	_, err := provisionNode(t.Context(), output, source)
	// Then neither extraction nor directory creation occurs.
	requireNodeProvisionID(t, err, "digest-mismatch")
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output created: %v", err)
	}
}

func nodeProvisionFixture(t testing.TB, data []byte) nodeProvisionSource {
	t.Helper()
	pin, err := toolchainpins.NodeRuntimePair()
	if err != nil {
		t.Fatal(err)
	}
	artifact := pin.Artifacts["linux-x64"]
	sum := sha256.Sum256(data)
	artifact.SHA256 = hex.EncodeToString(sum[:])
	pin.Artifacts = map[string]toolchainpins.NodeArtifact{"linux-x64": artifact}
	return nodeProvisionSource{pin: pin, fetch: func(ctx context.Context, url string, maximum int64, accept string) ([]byte, error) {
		if ctx == nil || url != artifact.URL || maximum != 128<<20 || accept != "application/octet-stream" {
			t.Fatalf("fetch arguments: %q, %d, %q", url, maximum, accept)
		}
		return data, nil
	}}
}

func requireNodeProvisionID(t *testing.T, err error, suffix string) {
	t.Helper()
	var identified *NodeProvisionError
	want := "windlass.verify.error.node-runtime-provision-" + suffix
	if !errors.As(err, &identified) || identified.DiagnosticID() != want || identified.Error() != want {
		t.Fatalf("error = %v, want %s", err, want)
	}
}

func TestProvisionNodeLayoutViolations(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"missing top entry", "extra top entry", "missing symlink", "extra bin entry", "node directory", "node not executable", "top file directory"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given one layout defect in an otherwise safe authenticated archive.
			entries := nodeRuntimeEntries()
			for i := range entries {
				entry := &entries[i]
				switch {
				case name == "missing top entry" && entry.header.Name == nodeFixtureRoot+"/LICENSE", name == "missing symlink" && entry.header.Name == nodeFixtureRoot+"/bin/npm":
					entries = append(entries[:i], entries[i+1:]...)
				case name == "node directory" && entry.header.Name == nodeFixtureRoot+"/bin/node", name == "top file directory" && entry.header.Name == nodeFixtureRoot+"/LICENSE":
					entry.header.Typeflag, entry.body = tar.TypeDir, ""
				case name == "node not executable" && entry.header.Name == nodeFixtureRoot+"/bin/node":
					entry.header.Mode = 0o644
				default:
					continue
				}
				break
			}
			if name == "extra top entry" {
				entries = append(entries, nodeArchiveEntry{header: tar.Header{Name: nodeFixtureRoot + "/extra", Typeflag: tar.TypeReg}})
			}
			if name == "extra bin entry" {
				entries = append(entries, nodeArchiveEntry{header: tar.Header{Name: nodeFixtureRoot + "/bin/extra", Typeflag: tar.TypeReg}})
			}
			output := t.TempDir()
			// When provisioning checks the extracted layout.
			_, err := provisionNode(t.Context(), output, nodeProvisionFixture(t, nodeRuntimeArchive(t, entries)))
			// Then layout failure removes the private directory.
			requireNodeProvisionID(t, err, "layout-violation")
			files, err := os.ReadDir(output)
			if err != nil || len(files) != 0 {
				t.Fatalf("partial output: %v, %v", files, err)
			}
		})
	}
}

func TestProvisionNodeVersionMismatch(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"root version", "npm version", "duplicate JSON", "invalid JSON", "missing manifest"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given safe layout with a mismatched or unreadable pair identity.
			entries := nodeRuntimeEntries()
			for i := range entries {
				entry := &entries[i]
				if name == "root version" {
					entry.header.Name = strings.Replace(entry.header.Name, "24.21.0", "24.20.0", 1)
				}
				if strings.HasSuffix(entry.header.Name, "/npm/package.json") {
					switch name {
					case "npm version":
						entry.body = `{"version":"0.0.1"}`
					case "duplicate JSON":
						entry.body = `{"version":"11.19.0","version":"11.19.0"}`
					case "invalid JSON":
						entry.body = `null`
					case "missing manifest":
						entry.header.Name += ".absent"
					}
				}
			}
			output := t.TempDir()
			// When checking identity without execution.
			_, err := provisionNode(t.Context(), output, nodeProvisionFixture(t, nodeRuntimeArchive(t, entries)))
			// Then the version diagnostic is exact and partial output is removed.
			requireNodeProvisionID(t, err, "version-mismatch")
			files, err := os.ReadDir(output)
			if err != nil || len(files) != 0 {
				t.Fatalf("partial output: %v, %v", files, err)
			}
		})
	}
}

func TestNodeRuntimeLayoutRejectsChangedSymlink(t *testing.T) {
	t.Parallel()
	// Given an extracted tree whose npm link was changed after extraction.
	directory := t.TempDir()
	name, err := extractNodeRuntime(t.Context(), nodeRuntimeArchive(t, nodeRuntimeEntries()), directory)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(directory, name)
	link := filepath.Join(root, "bin/npm")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../escape", link); err != nil {
		t.Fatal(err)
	}
	// When validating the tree independently of raw archive admission.
	err = verifyNodeRuntimeLayout(root)
	// Then exact link equality is enforced at the layout layer too.
	requireNodeProvisionID(t, err, "layout-violation")
}
