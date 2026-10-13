package npmprofile

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvisionNPMMissingCLIIsVersionMismatchWithoutExecution(t *testing.T) {
	// Given a directory instead of the required regular CLI entry.
	_, marker := provisionFakeNode(t)
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gz)
	manifest := `{"version":"11.21.0"}`
	if err := archive.WriteHeader(&tar.Header{Name: "package/package.json", Mode: 0o600, Size: int64(len(manifest))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := archive.WriteHeader(&tar.Header{Name: "package/bin/npm-cli.js/", Typeflag: tar.TypeDir, Mode: 0o700}); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	// When provisioning checks the extracted CLI.
	_, err := provisionNPM(t.Context(), t.TempDir(), provisionSource(t, buffer.Bytes()))
	// Then the absent regular CLI is a version-mismatch failure, with no execution.
	requireProvisionID(t, err, "npm-provision-version-mismatch")
	requireProvisionNotInvoked(t, marker)
}

func TestProvisionNPMQuotesShellMetacharactersWithoutExecution(t *testing.T) {
	// Given an output path that contains shell expansion syntax and a single quote.
	node, marker := provisionFakeNode(t)
	output := filepath.Join(t.TempDir(), "space ' $(touch marker)")
	data := provisionArchive(t, `{"version":"11.21.0"}`, nil)
	// When writing the launcher.
	launcher, err := provisionNPM(t.Context(), output, provisionSource(t, data))
	// Then literal paths are quoted, not interpolated as shell syntax.
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "exec '"+node+"' '") || !strings.Contains(string(content), "space '\"'\"' $(touch marker)") {
		t.Fatalf("unsafe quoting: %q", content)
	}
	requireProvisionNotInvoked(t, marker)
}

func TestProvisionNPMRejectsCompressedSizeOverflow(t *testing.T) {
	t.Parallel()
	// Given one byte beyond the compressed distribution limit.
	data := make([]byte, maxNPMDistribution+1)
	source := provisionSource(t, nil)
	source.fetch = func(context.Context, string, int64, string) ([]byte, error) { return data, nil }
	// When the fetch boundary returns excessive bytes.
	_, err := provisionNPM(t.Context(), t.TempDir(), source)
	// Then the download is rejected before digest processing or extraction.
	requireProvisionID(t, err, "npm-provision-download")
}

func TestExtractNPMRejectsOversizedEntry(t *testing.T) {
	t.Parallel()
	// Given a declared entry one byte beyond the expanded stream budget.
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gz)
	if err := archive.WriteHeader(&tar.Header{Name: "package/oversized", Size: maxUnpackedSize + 1}); err != nil {
		t.Fatal(err)
	}
	// Deliberately omit the body and tar trailer: the size gate must reject the header.
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	// When extracting the authenticated archive bytes.
	err := extractNPMDistribution(t.Context(), buffer.Bytes(), directory)
	// Then no oversized file is opened.
	if err == nil || err.Error() != "archive exceeds extraction size limit" {
		t.Fatalf("size gate = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "package", "oversized")); !os.IsNotExist(err) {
		t.Fatalf("oversized file created: %v", err)
	}
	t.Run("registered provisioning diagnostic", func(t *testing.T) {
		t.Parallel()
		_, err := provisionNPM(t.Context(), t.TempDir(), provisionSource(t, buffer.Bytes()))
		requireProvisionID(t, err, "npm-provision-archive-unsafe")
	})
}

func TestProvisionNPMRejectsDuplicateManifest(t *testing.T) {
	t.Parallel()
	// Given two entries named package/package.json.
	data := provisionArchive(t, `{"version":"11.21.0"}`, &tar.Header{Name: "package/package.json", Typeflag: tar.TypeReg})
	// When provisioning the authenticated archive.
	_, err := provisionNPM(t.Context(), t.TempDir(), provisionSource(t, data))
	// Then duplicate entries fail with the registered archive diagnostic.
	requireProvisionID(t, err, "npm-provision-archive-unsafe")
}

func TestProvisionNPMRejectsEmptyTar(t *testing.T) {
	t.Parallel()
	// Given a valid gzip stream containing only a tar end-of-archive trailer.
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gz)
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	// When provisioning an archive with no entries.
	_, err := provisionNPM(t.Context(), t.TempDir(), provisionSource(t, buffer.Bytes()))
	// Then the archive is rejected, not classified as a missing manifest.
	requireProvisionID(t, err, "npm-provision-archive-unsafe")
}

func TestProvisionNPMRejectsUnpackedBudgetOverflow(t *testing.T) {
	t.Parallel()
	// Given a nonempty tar followed by padding that fills the expanded budget plus one.
	var tarBytes, compressed bytes.Buffer
	archive := tar.NewWriter(&tarBytes)
	if err := archive.WriteHeader(&tar.Header{Name: "package/", Typeflag: tar.TypeDir, Mode: 0o700}); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gz.Write(tarBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	padding := make([]byte, 1<<20)
	for remaining := maxUnpackedSize + 1 - tarBytes.Len(); remaining > 0; {
		size := min(remaining, len(padding))
		if _, err := gz.Write(padding[:size]); err != nil {
			t.Fatal(err)
		}
		remaining -= size
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	// When the extractor drains past tar EOF.
	_, err = provisionNPM(t.Context(), t.TempDir(), provisionSource(t, compressed.Bytes()))
	// Then the total budget remains enforced even outside entry bodies.
	requireProvisionID(t, err, "npm-provision-archive-unsafe")
}

func FuzzExtractNPMDistribution(f *testing.F) {
	f.Add([]byte("not gzip"))
	f.Add([]byte{})
	f.Add(provisionArchive(f, `{"version":"11.21.0"}`, nil))
	for _, header := range []tar.Header{
		{Name: "../escape", Typeflag: tar.TypeReg},
		{Name: "/absolute", Typeflag: tar.TypeReg},
		{Name: "package/link", Typeflag: tar.TypeSymlink, Linkname: "../../escape"},
		{Name: "package/link", Typeflag: tar.TypeLink, Linkname: "../escape"},
		{Name: "other/file", Typeflag: tar.TypeReg},
	} {
		f.Add(provisionArchive(f, `{"version":"11.21.0"}`, &header))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip("bounded fuzz input")
		}
		directory := t.TempDir()
		if err := extractNPMDistribution(t.Context(), data, directory); err != nil {
			return
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 1 || entries[0].Name() != "package" || !entries[0].IsDir() {
			t.Fatalf("accepted invalid root: %v, %v", entries, err)
		}
		err = filepath.WalkDir(directory, func(name string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
				t.Errorf("accepted special file: %s", name)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}
