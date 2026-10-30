package npmprofile

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
)

var nodeArchiveRootPattern = regexp.MustCompile(`^node-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-linux-x64$`)

var errNodeLayout = errors.New("node runtime symlink layout violation")

func nodeRuntimeLink(name string) string {
	switch name {
	case "bin/npm":
		return "../lib/node_modules/npm/bin/npm-cli.js"
	case "bin/npx":
		return "../lib/node_modules/npm/bin/npx-cli.js"
	case "bin/corepack":
		return "../lib/node_modules/corepack/dist/corepack.js"
	default:
		return ""
	}
}

// Extraction checks root syntax; recorded-version equality belongs to pair verification.
func extractNodeRuntime(ctx context.Context, encoded []byte, directory string) (string, error) {
	decoded, err := newNodeXZReader(encoded)
	if err != nil {
		return "", err
	}
	limited := &io.LimitedReader{R: decoded, N: maxUnpackedSize + 1}
	archive := tar.NewReader(limited)
	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", err
	}
	defer root.Close()
	seen := make(map[string]bool)
	rootName := ""
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		name := header.Name
		if header.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if name == "" || strings.Contains(name, "\\") {
			return "", nodeProvisionFailure("archive-unsafe")
		}
		parts := strings.Split(name, "/")
		for _, part := range parts {
			if part == "" || part == "." || part == ".." {
				return "", nodeProvisionFailure("archive-unsafe")
			}
		}
		if !nodeArchiveRootPattern.MatchString(parts[0]) || (rootName != "" && rootName != parts[0]) {
			return "", nodeProvisionFailure("archive-unsafe")
		}
		rootName = parts[0]
		if len(parts) == 1 && header.Typeflag != tar.TypeDir {
			return "", nodeProvisionFailure("archive-unsafe")
		}
		if seen[name] || len(seen) >= maxPackedFileCount {
			return "", nodeProvisionFailure("archive-unsafe")
		}
		seen[name] = true
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o700); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if header.Size < 0 || header.Size > limited.N {
				return "", nodeProvisionFailure("archive-unsafe")
			}
			if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
				return "", err
			}
			// Retain owner execute only; never reproduce archive ownership or special mode bits.
			mode := os.FileMode(0o600)
			if header.Mode&0o100 != 0 {
				mode = 0o700
			}
			file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return "", err
			}
			_, copyErr := io.CopyN(file, archive, header.Size)
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return "", err
			}
		case tar.TypeSymlink:
			target := nodeRuntimeLink(strings.TrimPrefix(name, rootName+"/"))
			if target == "" || header.Linkname != target {
				return "", errNodeLayout
			}
			if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
				return "", err
			}
			if err := root.Symlink(target, name); err != nil {
				return "", err
			}
		default:
			return "", nodeProvisionFailure("archive-unsafe")
		}
	}
	// Tar EOF alone does not validate the xz trailer or bound trailing decompressed bytes.
	if _, err := io.CopyN(io.Discard, limited, limited.N); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if limited.N == 0 || len(seen) == 0 {
		return "", nodeProvisionFailure("archive-unsafe")
	}
	if err := decoded.verify(); err != nil {
		return "", err
	}
	return rootName, nil
}
