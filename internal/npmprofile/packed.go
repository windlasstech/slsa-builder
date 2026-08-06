package npmprofile

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
)

const (
	maxTarballSize     = 512 << 20
	maxUnpackedSize    = 1 << 30
	maxPackedFileCount = 100_000
)

func oneTarball(directory string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("%s: read pack output: %w", IDPackagePackFailed, err)
	}
	if len(entries) != 1 {
		return "", fmt.Errorf("%s: pack output contains %d entries, want exactly one", IDPackagePackFailed, len(entries))
	}
	entry := entries[0]
	if err := handoff.ValidateSafeBasename(entry.Name()); err != nil || !strings.HasSuffix(entry.Name(), ".tgz") {
		return "", fmt.Errorf("%s: unsafe package tarball basename %q", IDPackagePackFailed, entry.Name())
	}
	info, err := entry.Info()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxTarballSize {
		return "", fmt.Errorf("%s: package tarball is not a bounded regular file", IDPackagePackFailed)
	}
	return filepath.Join(directory, entry.Name()), nil
}

func inspectTarball(tarballPath string) (PackedMetadata, digest.SHA256, digest.SHA512, error) {
	encoded, err := readBoundedRegularFile(tarballPath, maxTarballSize)
	if err != nil {
		return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, fmt.Errorf("%s: %w", IDPackagePackFailed, err)
	}
	sha256Value := digest.SumSHA256(encoded)
	sha512Value := digest.SumSHA512(encoded)
	gzipReader, err := gzip.NewReader(bytes.NewReader(encoded))
	if err != nil {
		return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, fmt.Errorf("%s: open gzip stream: %w", IDPackagePackFailed, err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(io.LimitReader(gzipReader, maxUnpackedSize+1))
	packed := PackedMetadata{Files: make([]string, 0)}
	var unpacked int64
	manifestFound := false
	for {
		header, nextErr := tarReader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, fmt.Errorf("%s: read tar stream: %w", IDPackagePackFailed, nextErr)
		}
		if err := validateArchivePath(header.Name); err != nil {
			return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, fmt.Errorf("%s: %w", IDPackagePackFailed, err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, fmt.Errorf("%s: archive entry %q has unsupported type", IDPackagePackFailed, header.Name)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Size < 0 || header.Size > maxUnpackedSize-unpacked || len(packed.Files) >= maxPackedFileCount {
			return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, fmt.Errorf("%s: package archive exceeds resource limits", IDPackagePackFailed)
		}
		unpacked += header.Size
		packed.Files = append(packed.Files, header.Name)
		if header.Name != "package/package.json" {
			continue
		}
		if manifestFound || header.Size > maxManifestSize {
			return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, fmt.Errorf("%s: packed manifest is duplicate or oversized", IDPackagePackFailed)
		}
		manifestBytes, readErr := io.ReadAll(io.LimitReader(tarReader, maxManifestSize+1))
		if readErr != nil || len(manifestBytes) > maxManifestSize {
			return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, fmt.Errorf("%s: read packed manifest", IDPackagePackFailed)
		}
		decoded, decodeErr := decodePackedManifest(manifestBytes)
		if decodeErr != nil {
			return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, decodeErr
		}
		packed.Name = decoded.Name
		packed.Version = decoded.Version
		packed.ConsumerSurface = decoded.ConsumerSurface
		manifestFound = true
	}
	if !manifestFound || packed.Name == "" || packed.Version == "" {
		return PackedMetadata{}, digest.SHA256{}, digest.SHA512{}, fmt.Errorf("%s: package/package.json is missing or lacks identity", IDPackagePackFailed)
	}
	return packed, sha256Value, sha512Value, nil
}

type packedManifest struct {
	Name            string
	Version         string
	ConsumerSurface map[string]json.RawMessage
}

func decodePackedManifest(encoded []byte) (packedManifest, error) {
	if err := canonicaljson.Validate(encoded); err != nil {
		return packedManifest{}, fmt.Errorf("%s: validate packed manifest: %w", IDPackagePackFailed, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		return packedManifest{}, fmt.Errorf("%s: decode packed manifest: %w", IDPackagePackFailed, err)
	}
	var decoded packedManifest
	if json.Unmarshal(object["name"], &decoded.Name) != nil || json.Unmarshal(object["version"], &decoded.Version) != nil {
		return packedManifest{}, fmt.Errorf("%s: packed manifest identity is invalid", IDPackagePackFailed)
	}
	allowed := []string{"exports", "main", "type", "bin", "types", "typings", "typesVersions", "files"}
	for _, key := range allowed {
		if value, present := object[key]; present {
			if decoded.ConsumerSurface == nil {
				decoded.ConsumerSurface = make(map[string]json.RawMessage)
			}
			decoded.ConsumerSurface[key] = append(json.RawMessage(nil), value...)
		}
	}
	return decoded, nil
}

func validateArchivePath(name string) error {
	if name == "" || strings.Contains(name, `\`) || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") || !strings.HasPrefix(name, "package/") {
		return fmt.Errorf("unsafe package archive path %q", name)
	}
	return nil
}

func readBoundedRegularFile(filePath string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(filePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, errors.New("path is not a bounded regular file")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	if statErr != nil || !os.SameFile(info, opened) {
		_ = file.Close()
		return nil, errors.New("file changed while opening")
	}
	encoded, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if int64(len(encoded)) > maximum {
		return nil, errors.New("file exceeds size limit")
	}
	return encoded, nil
}
