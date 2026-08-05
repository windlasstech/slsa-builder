package npmprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/identity"
)

const maxManifestSize = 16 << 20

func resolvePackage(config Config) (resolvedPackage, *diagnostic.DiagnosticMetadata, string, string) {
	rootReal, selectedReal, selectedRelative, err := resolveSelectedDirectory(config.RepositoryRoot, config.PackageDirectory)
	if err != nil {
		return resolvedPackage{}, nil, IDPackageResolutionInvalid, "package-directory does not resolve to one directory inside the repository"
	}

	selectedManifest, _, err := readManifest(filepath.Join(selectedReal, "package.json"))
	if err != nil {
		return resolvedPackage{}, nil, IDPackageManifestInvalid, "selected package.json is missing or invalid"
	}
	if selectedManifest.Name == "" || selectedManifest.Version == "" {
		return resolvedPackage{}, nil, IDPackageMetadataRequired, "selected package.json requires non-empty name and version"
	}
	if selectedManifest.Private {
		return resolvedPackage{}, nil, IDPackagePrivate, "selected package is private"
	}
	canonicalRepository, err := normalizeRepository(selectedManifest.Repository)
	if err != nil {
		return resolvedPackage{}, nil, IDPackageRepositoryIdentityMismatch, "package repository identity is missing, malformed, or unsupported"
	}
	observedRepository, err := identity.CanonicalRepository(config.ObservedRepository)
	if err != nil || canonicalRepository != observedRepository {
		return resolvedPackage{}, nil, IDPackageRepositoryIdentityMismatch, "package repository identity does not match the observed caller repository"
	}

	managerRootReal, rootManifest, err := discoverWorkspaceRoot(rootReal, selectedReal)
	if err != nil {
		return resolvedPackage{}, nil, IDPackageResolutionInvalid, "workspace metadata cannot resolve the selected package unambiguously"
	}
	managerRootRelative, err := filepath.Rel(rootReal, managerRootReal)
	if err != nil {
		return resolvedPackage{}, nil, IDPackageResolutionInvalid, "package manager root cannot be represented inside the repository"
	}
	managerRoot := canonicalRelative(managerRootRelative)

	resolved := resolvedPackage{
		repositoryRootReal: rootReal,
		directoryReal:      selectedReal,
		directory:          selectedRelative,
		managerRootReal:    managerRootReal,
		managerRoot:        managerRoot,
		manifest:           selectedManifest,
		manifestPath:       joinRelative(selectedRelative, "package.json"),
	}
	if rootManifest != nil && managerRootReal != selectedReal {
		resolved.rootManifest = rootManifest
		resolved.rootManifestPath = joinRelative(managerRoot, "package.json")
	}
	return resolved, nil, "", canonicalRepository
}

func resolveSelectedDirectory(repositoryRoot, packageDirectory string) (string, string, string, error) {
	if repositoryRoot == "" || packageDirectory == "" || filepath.IsAbs(packageDirectory) || strings.Contains(packageDirectory, `\`) {
		return "", "", "", errors.New("invalid path form")
	}
	segments := strings.Split(packageDirectory, "/")
	for _, segment := range segments {
		if segment == ".." {
			return "", "", "", errors.New("path traversal")
		}
	}
	cleaned := path.Clean(packageDirectory)
	if cleaned == "" || cleaned == "/" || strings.HasPrefix(cleaned, "../") {
		return "", "", "", errors.New("invalid normalized path")
	}

	rootAbsolute, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return "", "", "", err
	}
	rootReal, err := filepath.EvalSymlinks(rootAbsolute)
	if err != nil {
		return "", "", "", err
	}
	if err := verifyExactCase(rootReal, cleaned); err != nil {
		return "", "", "", err
	}
	selectedReal, err := filepath.EvalSymlinks(filepath.Join(rootReal, filepath.FromSlash(cleaned)))
	if err != nil {
		return "", "", "", err
	}
	info, err := os.Stat(selectedReal)
	if err != nil || !info.IsDir() {
		return "", "", "", errors.New("selected path is not a directory")
	}
	relative, err := filepath.Rel(rootReal, selectedReal)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", "", "", errors.New("selected path escapes repository")
	}
	return rootReal, selectedReal, canonicalRelative(relative), nil
}

func verifyExactCase(root, cleaned string) error {
	if cleaned == "." {
		return nil
	}
	current := root
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "." || segment == "" {
			continue
		}
		entries, err := os.ReadDir(current)
		if err != nil {
			return err
		}
		found := false
		for _, entry := range entries {
			if entry.Name() == segment {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("path segment %q does not match an entry exactly", segment)
		}
		current = filepath.Join(current, segment)
	}
	return nil
}

func readManifest(manifestPath string) (manifest, *diagnostic.DiagnosticMetadata, error) {
	encoded, err := readRegularNonSymlink(manifestPath)
	if err != nil {
		return manifest{}, nil, err
	}
	if err := canonicaljson.Validate(encoded); err != nil {
		return manifest{}, nil, err
	}
	var decoded manifest
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return manifest{}, nil, err
	}
	return decoded, nil, nil
}

func readRegularNonSymlink(filePath string) ([]byte, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxManifestSize {
		return nil, errors.New("path must identify a bounded regular non-symlink file")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return nil, errors.New("file changed while opening")
	}
	encoded, readErr := io.ReadAll(io.LimitReader(file, maxManifestSize+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if len(encoded) > maxManifestSize {
		return nil, errors.New("file exceeds size limit")
	}
	return encoded, nil
}

func normalizeRepository(raw jsonRaw) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("missing repository")
	}
	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err == nil {
		return identity.CanonicalRepository(stringValue)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", err
	}
	if len(object) < 2 || len(object) > 3 {
		return "", errors.New("unsupported repository object shape")
	}
	for key := range object {
		if key != "type" && key != "url" && key != "directory" {
			return "", errors.New("unknown repository object member")
		}
	}
	var repositoryType, repositoryURL string
	if err := json.Unmarshal(object["type"], &repositoryType); err != nil || repositoryType != "git" {
		return "", errors.New("repository object type must be git")
	}
	if err := json.Unmarshal(object["url"], &repositoryURL); err != nil {
		return "", errors.New("repository object URL must be a string")
	}
	if directory, ok := object["directory"]; ok {
		var value string
		if err := json.Unmarshal(directory, &value); err != nil {
			return "", errors.New("repository directory must be a string")
		}
	}
	return identity.CanonicalRepository(repositoryURL)
}

func canonicalRelative(value string) string {
	if value == "." || value == "" {
		return "."
	}
	return filepath.ToSlash(value)
}

func joinRelative(base, name string) string {
	if base == "." {
		return name
	}
	return path.Join(base, name)
}
