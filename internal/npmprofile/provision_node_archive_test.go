package npmprofile

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const nodeFixtureRoot = "node-v24.22.1-linux-x64"

type nodeArchiveEntry struct {
	header tar.Header
	body   string
}

func nodeRuntimeEntries() []nodeArchiveEntry {
	entries := []nodeArchiveEntry{}
	for _, name := range []string{"", "bin", "include", "lib", "share"} {
		entries = append(entries, nodeArchiveEntry{header: tar.Header{Name: nodeFixtureRoot + "/" + name, Typeflag: tar.TypeDir, Mode: 0o755}})
	}
	for _, name := range []string{"CHANGELOG.md", "LICENSE", "README.md", "bin/node", "lib/node_modules/npm/bin/npm-cli.js", "lib/node_modules/npm/bin/npx-cli.js", "lib/node_modules/corepack/dist/corepack.js"} {
		entries = append(entries, nodeArchiveEntry{header: tar.Header{Name: nodeFixtureRoot + "/" + name, Typeflag: tar.TypeReg, Mode: 0o755}, body: "not executable runtime bytes"})
	}
	entries = append(entries, nodeArchiveEntry{header: tar.Header{Name: nodeFixtureRoot + "/lib/node_modules/npm/package.json", Typeflag: tar.TypeReg, Mode: 0o644}, body: `{"version":"11.19.1"}`})
	for name, target := range map[string]string{
		"npm":      "../lib/node_modules/npm/bin/npm-cli.js",
		"npx":      "../lib/node_modules/npm/bin/npx-cli.js",
		"corepack": "../lib/node_modules/corepack/dist/corepack.js",
	} {
		entries = append(entries, nodeArchiveEntry{header: tar.Header{Name: nodeFixtureRoot + "/bin/" + name, Typeflag: tar.TypeSymlink, Linkname: target}})
	}
	return entries
}

func nodeRuntimeArchive(t testing.TB, entries []nodeArchiveEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		header := entry.header
		header.Size = int64(len(entry.body))
		if err := tw.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestExtractNodeRuntimeAcceptsDocumentedLinks(t *testing.T) {
	t.Parallel()
	// Given the official root structure and its three documented symlinks.
	data := nodeRuntimeArchive(t, nodeRuntimeEntries())
	// When extracting without invoking any files.
	_, err := extractNodeRuntime(t.Context(), data, t.TempDir())
	// Then the dedicated Node policy admits the links.
	if err != nil {
		t.Fatal(err)
	}
}

func TestExtractNodeRuntimeRejectsUnsafeArchives(t *testing.T) {
	t.Parallel()
	for name, header := range map[string]tar.Header{
		"absolute":   {Name: "/absolute", Typeflag: tar.TypeReg},
		"parent":     {Name: nodeFixtureRoot + "/../escape", Typeflag: tar.TypeReg},
		"backslash":  {Name: nodeFixtureRoot + `/bin\escape`, Typeflag: tar.TypeReg},
		"hardlink":   {Name: nodeFixtureRoot + "/hard", Typeflag: tar.TypeLink, Linkname: nodeFixtureRoot + "/LICENSE"},
		"device":     {Name: nodeFixtureRoot + "/device", Typeflag: tar.TypeChar},
		"fifo":       {Name: nodeFixtureRoot + "/fifo", Typeflag: tar.TypeFifo},
		"multi root": {Name: "node-v0.0.1-linux-x64/file", Typeflag: tar.TypeReg},
		"wrong root": {Name: "package/file", Typeflag: tar.TypeReg},
		"duplicate":  {Name: nodeFixtureRoot + "/LICENSE", Typeflag: tar.TypeReg},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given an unsafe entry before otherwise valid content (including duplicate names).
			data := nodeRuntimeArchive(t, append([]nodeArchiveEntry{{header: header}}, nodeRuntimeEntries()...))
			output := t.TempDir()
			// When the authenticated stream crosses the extraction boundary.
			_, err := provisionNode(t.Context(), output, nodeProvisionFixture(t, data))
			// Then the archive diagnostic wins and all partial output is removed.
			requireNodeProvisionID(t, err, "archive-unsafe")
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 0 {
				t.Fatalf("partial output: %v, %v", entries, err)
			}
		})
	}
}

func TestExtractNodeRuntimeRejectsInvalidStreams(t *testing.T) {
	t.Parallel()
	valid := nodeRuntimeArchive(t, nodeRuntimeEntries())
	badTrailer := bytes.Clone(valid)
	badTrailer[len(badTrailer)-8] ^= 1
	var oversized bytes.Buffer
	gz := gzip.NewWriter(&oversized)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: nodeFixtureRoot + "/huge", Typeflag: tar.TypeReg, Size: maxUnpackedSize + 2}); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"not gzip": []byte("invalid"), "empty tar": nodeRuntimeArchive(t, nil), "oversize": oversized.Bytes(), "gzip trailer": badTrailer, "truncated": valid[:len(valid)-4]} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given an authenticated but invalid compressed stream.
			source := nodeProvisionFixture(t, data)
			// When provisioning drains and verifies the stream.
			_, err := provisionNode(t.Context(), t.TempDir(), source)
			// Then malformed or oversized bytes cannot be accepted.
			requireNodeProvisionID(t, err, "archive-unsafe")
		})
	}
}

func TestProvisionNodeRejectsSymlinkLayout(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		path   string
		target string
	}{
		{name: "wrong in-root target", path: "bin/npm", target: "../lib/node_modules/npm/bin/npx-cli.js"},
		{name: "unknown symlink", path: "bin/foo", target: "../lib/node_modules/npm/bin/npm-cli.js"},
		{name: "escaping target", path: "bin/npm", target: "../../escape"},
		{name: "absolute target", path: "bin/npm", target: "/escape"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given one invalid symlink in an otherwise valid authenticated archive.
			entries := nodeRuntimeEntries()
			for i := range entries {
				if entries[i].header.Name == nodeFixtureRoot+"/bin/npm" {
					entries[i].header.Name = nodeFixtureRoot + "/" + test.path
					entries[i].header.Linkname = test.target
				}
			}
			output := t.TempDir()
			// When provisioning encounters the invalid symlink.
			_, err := provisionNode(t.Context(), output, nodeProvisionFixture(t, nodeRuntimeArchive(t, entries)))
			// Then the exact layout diagnostic is returned and no partial runtime remains.
			requireNodeProvisionID(t, err, "layout-violation")
			files, err := os.ReadDir(output)
			if err != nil || len(files) != 0 {
				t.Fatalf("partial output: %v, %v", files, err)
			}
		})
	}
}

func FuzzExtractNodeRuntime(f *testing.F) {
	f.Add([]byte("not gzip"))
	f.Add([]byte{})
	f.Add(nodeRuntimeArchive(f, nodeRuntimeEntries()))
	for _, header := range []tar.Header{
		{Name: "../escape", Typeflag: tar.TypeReg},
		{Name: "/absolute", Typeflag: tar.TypeReg},
		{Name: nodeFixtureRoot + "/link", Typeflag: tar.TypeSymlink, Linkname: "../../escape"},
		{Name: nodeFixtureRoot + "/bin/npm", Typeflag: tar.TypeSymlink, Linkname: "../lib/node_modules/npm/bin/npm-cli.js"},
		{Name: nodeFixtureRoot + "/link", Typeflag: tar.TypeLink, Linkname: "../escape"},
		{Name: "other/file", Typeflag: tar.TypeReg},
	} {
		f.Add(nodeRuntimeArchive(f, []nodeArchiveEntry{{header: header}}))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip("bounded fuzz input")
		}
		directory := t.TempDir()
		name, err := extractNodeRuntime(t.Context(), data, directory)
		if err != nil {
			return
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 1 || entries[0].Name() != name || !entries[0].IsDir() || !strings.HasPrefix(name, "node-v") || !strings.HasSuffix(name, "-linux-x64") {
			t.Fatalf("invalid root: %v, %v", entries, err)
		}
		err = filepath.WalkDir(filepath.Join(directory, name), func(file string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				relative, err := filepath.Rel(filepath.Join(directory, name), file)
				if err != nil {
					return err
				}
				target, err := os.Readlink(file)
				want := map[string]string{"bin/npm": "../lib/node_modules/npm/bin/npm-cli.js", "bin/npx": "../lib/node_modules/npm/bin/npx-cli.js", "bin/corepack": "../lib/node_modules/corepack/dist/corepack.js"}[filepath.ToSlash(relative)]
				if err != nil || want == "" || target != want {
					t.Fatalf("unexpected link: %s -> %s (%v)", file, target, err)
				}
			} else if !entry.IsDir() && !entry.Type().IsRegular() {
				t.Fatalf("special file: %s", file)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestExtractNodeRuntimeAggregateBounds(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"entry count", "trailing unpacked bytes"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given a stream exceeding the shared entry or decompressed-byte budget.
			var buffer bytes.Buffer
			gz := gzip.NewWriter(&buffer)
			tw := tar.NewWriter(gz)
			if name == "entry count" {
				for i := 0; i <= maxPackedFileCount; i++ {
					if err := tw.WriteHeader(&tar.Header{Name: fmt.Sprintf("%s/file-%d", nodeFixtureRoot, i), Typeflag: tar.TypeReg}); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				if err := tw.WriteHeader(&tar.Header{Name: nodeFixtureRoot, Typeflag: tar.TypeDir}); err != nil {
					t.Fatal(err)
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if name == "trailing unpacked bytes" {
				block := make([]byte, 1<<20)
				for i := int64(0); i < maxUnpackedSize; i += int64(len(block)) {
					if _, err := gz.Write(block); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			// When extraction consumes the stream, including bytes after tar EOF.
			_, err := extractNodeRuntime(t.Context(), buffer.Bytes(), t.TempDir())
			// Then neither budget can be bypassed by individually small entries.
			if err == nil {
				t.Fatal("accepted archive exceeding aggregate bound")
			}
		})
	}
}
