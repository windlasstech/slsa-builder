package npmprofile

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestProvisionNPMCreatesPrivateLauncherWithoutExecution(t *testing.T) {
	// Given a private fake node and an authenticated distribution.
	node, marker := provisionFakeNode(t)
	data := provisionArchive(t, `{"name":"npm","version":"11.21.0"}`, nil)
	output := t.TempDir()
	output, err := filepath.EvalSymlinks(output)
	if err != nil {
		t.Fatal(err)
	}
	// When provisioning completes.
	launcher, err := provisionNPM(t.Context(), output, provisionSource(t, data))
	// Then the launcher is private, absolute, and binds the resolved node and CLI.
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(launcher) || filepath.Base(launcher) != "npm" || !pathWithinRoot(launcher, output) {
		t.Fatalf("launcher = %q", launcher)
	}
	for _, name := range []string{launcher, filepath.Dir(launcher)} {
		info, statErr := os.Stat(name)
		if statErr != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("mode of %q = %v, %v", name, info, statErr)
		}
	}
	content, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(filepath.Dir(launcher), "package", "bin", "npm-cli.js")
	want := "#!/bin/sh\nexec '" + node + "' '" + cli + "' \"$@\"\n"
	if string(content) != want {
		t.Fatalf("launcher = %q, want %q", content, want)
	}
	requireProvisionNotInvoked(t, marker)
}

func TestProvisionNPMDigestMismatchBeforeDirectoryCreation(t *testing.T) {
	// Given corrupted compressed bytes and the original pin.
	_, marker := provisionFakeNode(t)
	data := provisionArchive(t, `{"name":"npm","version":"11.21.0"}`, nil)
	source := provisionSource(t, data)
	source.fetch = func(context.Context, string, int64, string) ([]byte, error) { return []byte("corrupted"), nil }
	output := filepath.Join(t.TempDir(), "not-created")
	// When the digest does not match.
	_, err := provisionNPM(t.Context(), output, source)
	// Then no extraction or execution has happened.
	requireProvisionID(t, err, "npm-provision-digest-mismatch")
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output created: %v", err)
	}
	requireProvisionNotInvoked(t, marker)
}

func TestProvisionNPMRejectsUnsafeArchive(t *testing.T) {
	for _, header := range []tar.Header{
		{Name: "../escape", Typeflag: tar.TypeReg},
		{Name: "/absolute", Typeflag: tar.TypeReg},
		{Name: "package/link", Typeflag: tar.TypeSymlink, Linkname: "../../escape"},
		{Name: "package/link", Typeflag: tar.TypeLink, Linkname: "../escape"},
		{Name: "other/file", Typeflag: tar.TypeReg},
		{Name: "package/bin/npm-cli.js", Typeflag: tar.TypeReg},
	} {
		t.Run(header.Name+string(header.Typeflag), func(t *testing.T) {
			// Given a digest-matching but unsafe archive.
			_, marker := provisionFakeNode(t)
			data := provisionArchive(t, `{"name":"npm","version":"11.21.0"}`, &header)
			output := t.TempDir()
			// When provisioning checks the archive.
			_, err := provisionNPM(t.Context(), output, provisionSource(t, data))
			// Then it fails closed and removes partial extraction.
			requireProvisionID(t, err, "npm-provision-archive-unsafe")
			entries, readErr := os.ReadDir(output)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("partial extraction: %v, %v", entries, readErr)
			}
			requireProvisionNotInvoked(t, marker)
		})
	}
}

func TestProvisionNPMInvalidManifestIsVersionMismatchWithoutExecution(t *testing.T) {
	for _, manifest := range []string{`{"name":"npm","version":"0.0.1"}`, `{}`, `null`, `{"version":1}`, `{"version":"11.21.0","version":"11.21.0"}`, `invalid`, ""} {
		t.Run(manifest, func(t *testing.T) {
			// Given a missing, malformed, or mismatched version record.
			_, marker := provisionFakeNode(t)
			data := provisionArchive(t, manifest, nil)
			// When provisioning reads the extracted manifest.
			_, err := provisionNPM(t.Context(), t.TempDir(), provisionSource(t, data))
			// Then the failure is consistently a version mismatch, without execution.
			requireProvisionID(t, err, "npm-provision-version-mismatch")
			requireProvisionNotInvoked(t, marker)
		})
	}
}

func provisionSource(t *testing.T, data []byte) npmProvisionSource {
	t.Helper()
	pin, err := toolchainpins.PublishPin()
	if err != nil {
		t.Fatal(err)
	}
	pin.Integrity = sha512SRI(digest.SumSHA512(data))
	return npmProvisionSource{pin: pin, fetch: func(ctx context.Context, url string, maximum int64, accept string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if url != pin.TarballURL || maximum <= 0 || accept != "application/octet-stream" {
			t.Errorf("fetch arguments = %q, %d, %q", url, maximum, accept)
		}
		return data, nil
	}}
}

func provisionArchive(t testing.TB, manifest string, extra *tar.Header) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	for name, content := range map[string]string{"package/bin/npm-cli.js": "// fixture", "package/package.json": manifest} {
		if content == "" {
			continue
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if extra != nil {
		if err := tw.WriteHeader(extra); err != nil {
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

func provisionFakeNode(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	marker := filepath.Join(directory, "invoked")
	node := filepath.Join(directory, "node")
	if err := os.WriteFile(node, []byte("#!/bin/sh\nprintf invoked > '"+marker+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	resolved, err := filepath.EvalSymlinks(node)
	if err != nil {
		t.Fatal(err)
	}
	return resolved, marker
}

func requireProvisionNotInvoked(t *testing.T, marker string) {
	t.Helper()
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("node was invoked: %v", err)
	}
}

func requireProvisionID(t *testing.T, err error, suffix string) {
	t.Helper()
	var identified interface{ DiagnosticID() string }
	if !errors.As(err, &identified) || identified.DiagnosticID() != "windlass.verify.error."+suffix {
		t.Fatalf("error = %v, want %s", err, suffix)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}
