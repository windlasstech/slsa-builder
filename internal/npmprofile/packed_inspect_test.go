package npmprofile

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

type inspectArchiveEntry struct {
	name, body string
	kind       byte
}

func TestPackedOneTarballOutputRejections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, want string }{
		{"wrong suffix", `unsafe package tarball basename "package.tar.gz"`},
		{"directory", "package tarball is not a bounded regular file"},
		{"symlink", "package tarball is not a bounded regular file"},
		{"multiple", "pack output contains 2 entries, want exactly one"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			switch test.name {
			case "wrong suffix":
				writeScript(t, root, "package.tar.gz", "bytes")
			case "directory":
				if err := os.Mkdir(filepath.Join(root, "package.tgz"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("missing", filepath.Join(root, "package.tgz")); err != nil {
					t.Fatal(err)
				}
			case "multiple":
				writeScript(t, root, "one.tgz", "one")
				writeScript(t, root, "two.tgz", "two")
			}
			got, err := oneTarball(root)
			if got != "" || err == nil || err.Error() != IDPackagePackFailed+": "+test.want {
				t.Fatalf("oneTarball() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestPackedOneTarballMissingDirectory(t *testing.T) {
	t.Parallel()
	_, err := oneTarball(filepath.Join(t.TempDir(), "missing"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing pack output = %v; want os.ErrNotExist", err)
	}
}

func inspectArchiveBytes(t *testing.T, entries []inspectArchiveEntry) []byte {
	t.Helper()
	var encoded bytes.Buffer
	gz := gzip.NewWriter(&encoded)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Typeflag: entry.kind, Mode: 0o600, Size: int64(len(entry.body))}); err != nil {
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
	return encoded.Bytes()
}

func TestPackedInspectArchiveContracts(t *testing.T) {
	t.Parallel()
	manifest := inspectArchiveEntry{"package/package.json", `{"name":"example","version":"1.0.0","main":"index.js"}`, tar.TypeReg}
	for _, test := range []struct {
		name    string
		entries []inspectArchiveEntry
		want    string
	}{
		{"valid directory and files", []inspectArchiveEntry{{"package/lib", "", tar.TypeDir}, manifest, {"package/index.js", "export {}", tar.TypeReg}}, ""},
		{"missing manifest", nil, "package/package.json is missing or lacks identity"},
		{"empty identity", []inspectArchiveEntry{{"package/package.json", `{"name":"","version":"1.0.0"}`, tar.TypeReg}}, "package/package.json is missing or lacks identity"},
		{"duplicate manifest", []inspectArchiveEntry{manifest, manifest}, "packed manifest is duplicate or oversized"},
		{"symlink", []inspectArchiveEntry{{"package/link", "", tar.TypeSymlink}}, `archive entry "package/link" has unsupported type`},
		{"hardlink", []inspectArchiveEntry{{"package/link", "", tar.TypeLink}}, `archive entry "package/link" has unsupported type`},
		{"traversal", []inspectArchiveEntry{{"package/../escape", "", tar.TypeReg}}, `unsafe package archive path "package/../escape"`},
		{"wrong identity type", []inspectArchiveEntry{{"package/package.json", `{"name":3,"version":"1.0.0"}`, tar.TypeReg}}, "packed manifest identity is invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			encoded := inspectArchiveBytes(t, test.entries)
			root := createRepository(t, map[string]string{"archive.tgz": string(encoded)})
			got, sha256Value, sha512Value, err := inspectTarball(filepath.Join(root, "archive.tgz"))
			if test.want != "" {
				want := IDPackagePackFailed + ": " + test.want
				if err == nil || err.Error() != want {
					t.Fatalf("inspectTarball() error = %v; want %q", err, want)
				}
				if sha256Value != (digest.SHA256{}) || sha512Value != (digest.SHA512{}) {
					t.Fatal("rejected archive returned usable digests")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != "example" || got.Version != "1.0.0" || !reflect.DeepEqual(got.Files, []string{"package/package.json", "package/index.js"}) || string(got.ConsumerSurface["main"]) != `"index.js"` {
				t.Fatalf("packed metadata = %#v", got)
			}
			if sha256Value != digest.SumSHA256(encoded) || sha512Value != digest.SumSHA512(encoded) {
				t.Fatal("archive digests differ from exact compressed bytes")
			}
		})
	}
}

func TestPackedBoundedRegularFile(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		maximum int64
		want    string
	}{
		{"exact bound", 3, ""}, {"one byte over", 2, "path is not a bounded regular file"},
		{"missing", 3, "path is not a bounded regular file"}, {"directory", 3, "path is not a bounded regular file"},
		{"symlink", 3, "path is not a bounded regular file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := createRepository(t, map[string]string{"file": "abc"})
			path := filepath.Join(root, "file")
			switch test.name {
			case "missing":
				path = filepath.Join(root, "missing")
			case "directory":
				path = root
			case "symlink":
				path = filepath.Join(root, "link")
				if err := os.Symlink("file", path); err != nil {
					t.Fatal(err)
				}
			}
			got, err := readBoundedRegularFile(path, test.maximum)
			if test.want == "" {
				if err != nil || string(got) != "abc" {
					t.Fatalf("bounded read = %q, %v; want abc, nil", got, err)
				}
			} else if err == nil || err.Error() != test.want || got != nil {
				t.Fatalf("bounded read = %q, %v; want nil, %q", got, err, test.want)
			}
		})
	}
}

func TestPackedManifestConsumerSurface(t *testing.T) {
	t.Parallel()
	encoded := []byte(`{"name":"example","version":"1.0.0","exports":{".":"./index.js"},"main":"index.js","type":"module","bin":{"example":"cli.js"},"types":"index.d.ts","typings":"legacy.d.ts","typesVersions":{},"files":["lib"],"scripts":{"build":"ignored"}}`)
	got, err := decodePackedManifest(encoded)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]json.RawMessage{"exports": []byte(`{".":"./index.js"}`), "main": []byte(`"index.js"`), "type": []byte(`"module"`), "bin": []byte(`{"example":"cli.js"}`), "types": []byte(`"index.d.ts"`), "typings": []byte(`"legacy.d.ts"`), "typesVersions": []byte(`{}`), "files": []byte(`["lib"]`)}
	if !reflect.DeepEqual(got.ConsumerSurface, want) {
		t.Fatalf("consumer surface = %s; want %s", got.ConsumerSurface, want)
	}
	clear(encoded)
	if !reflect.DeepEqual(got.ConsumerSurface, want) {
		t.Fatal("consumer surface aliases input bytes")
	}
}

func TestRejectPackedCorepackEnvironment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files []string
		want  bool
	}{
		{"absent", []string{"package/package.json", "package/dist/index.js"}, false},
		{"root entry", []string{"package/package.json", "package/.corepack.env"}, true},
		{"nested entry", []string{"package/package.json", "package/nested/.corepack.env"}, true},
		{"unprefixed entry", []string{".corepack.env", "package/package.json"}, true},
		{"similarly suffixed names pass", []string{"package/.corepack.env.example", "package/corepack.env", "package/dist/.corepack.env.bak"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := rejectPackedCorepackEnvironment(tt.files)
			if tt.want && (err == nil || !strings.Contains(err.Error(), IDPackedCorepackEnvironmentProhibited)) {
				t.Fatalf("rejectPackedCorepackEnvironment(%v) = %v, want %s", tt.files, err, IDPackedCorepackEnvironmentProhibited)
			}
			if !tt.want && err != nil {
				t.Fatalf("rejectPackedCorepackEnvironment(%v) = %v, want nil", tt.files, err)
			}
		})
	}
}
