package npmprofile

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"strings"
)

func extractNPMDistribution(ctx context.Context, encoded []byte, directory string) error {
	gz, err := gzip.NewReader(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	defer gz.Close()
	limited := &io.LimitedReader{R: gz, N: maxUnpackedSize + 1}
	archive := tar.NewReader(limited)
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	seen := make(map[string]bool)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name := header.Name
		if header.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if name != "package" || header.Typeflag != tar.TypeDir {
			if err := validateArchivePath(name); err != nil {
				return err
			}
		}
		if seen[name] || len(seen) >= maxPackedFileCount {
			return errors.New("duplicate archive entry or entry limit exceeded")
		}
		seen[name] = true
		// Match the existing packed-archive policy: links and special files are not supported.
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if header.Size < 0 || header.Size > limited.N {
				return errors.New("archive exceeds extraction size limit")
			}
			if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
				return err
			}
			file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, archive)
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return err
			}
		default:
			return errors.New("unsupported archive entry type")
		}
	}
	// Drain the bounded gzip stream so truncation and checksum errors cannot hide behind tar EOF.
	if _, err := io.Copy(io.Discard, limited); err != nil {
		return err
	}
	if limited.N == 0 || len(seen) == 0 {
		return errors.New("empty or oversized archive")
	}
	return nil
}
