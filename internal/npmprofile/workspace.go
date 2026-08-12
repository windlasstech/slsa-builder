package npmprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

func discoverWorkspaceRoot(repositoryRoot, selectedDirectory string) (string, *manifest, error) {
	for candidate := selectedDirectory; ; candidate = filepath.Dir(candidate) {
		patterns, candidateManifest, metadataPresent, pnpmRootOnly, err := workspacePatterns(candidate)
		if err != nil {
			return "", nil, err
		}
		if metadataPresent {
			relative, relErr := filepath.Rel(candidate, selectedDirectory)
			if relErr != nil {
				return "", nil, relErr
			}
			if pnpmRootOnly {
				if relative == "." {
					return candidate, candidateManifest, nil
				}
				return "", nil, errors.New("settings-only pnpm workspace cannot claim a subdirectory")
			}
			matched := false
			for _, pattern := range patterns {
				if matchWorkspacePattern(pattern, canonicalRelative(relative)) {
					matched = true
				}
			}
			if matched {
				return candidate, candidateManifest, nil
			}
		}
		if candidate == repositoryRoot {
			break
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", nil, errors.New("workspace search escaped repository")
		}
	}
	return selectedDirectory, nil, nil
}

func workspacePatterns(candidate string) ([]string, *manifest, bool, bool, error) {
	var patterns []string
	var candidateManifest *manifest
	metadataPresent := false
	pnpmRootOnly := false

	manifestPath := filepath.Join(candidate, "package.json")
	if _, err := os.Lstat(manifestPath); err == nil {
		decoded, _, readErr := readManifest(manifestPath)
		if readErr != nil {
			return nil, nil, false, false, readErr
		}
		candidateManifest = &decoded
		if len(decoded.Workspaces) != 0 {
			metadataPresent = true
			workspacePatterns, parseErr := parseJSONWorkspacePatterns(decoded.Workspaces)
			if parseErr != nil {
				return nil, nil, false, false, parseErr
			}
			patterns = append(patterns, workspacePatterns...)
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, false, false, err
	}

	pnpmPath := filepath.Join(candidate, "pnpm-workspace.yaml")
	if encoded, fileErr := readRegularNonSymlink(pnpmPath); fileErr == nil {
		metadataPresent = true
		pnpmPatterns, rootOnly, parseErr := parsePNPMWorkspacePatterns(encoded)
		if parseErr != nil {
			return nil, nil, false, false, parseErr
		}
		pnpmRootOnly = rootOnly
		patterns = append(patterns, pnpmPatterns...)
	} else if !os.IsNotExist(fileErr) {
		return nil, nil, false, false, fileErr
	}

	for _, pattern := range patterns {
		if err := validateWorkspacePattern(pattern); err != nil {
			return nil, nil, false, false, err
		}
	}
	return patterns, candidateManifest, metadataPresent, pnpmRootOnly, nil
}

func parseJSONWorkspacePatterns(raw jsonRaw) ([]string, error) {
	var direct []string
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || len(object) != 1 {
		return nil, errors.New("unsupported workspaces shape")
	}
	packages, ok := object["packages"]
	if !ok || json.Unmarshal(packages, &direct) != nil {
		return nil, errors.New("workspace packages must be an array of strings")
	}
	return direct, nil
}

func parsePNPMWorkspacePatterns(encoded []byte) ([]string, bool, error) {
	var object map[string]any
	decoder := yaml.NewDecoder(bytes.NewReader(encoded), yaml.Strict())
	if err := decoder.Decode(&object); err != nil || object == nil {
		return nil, false, errors.New("pnpm workspace must be an object")
	}
	rawPackages, ok := object["packages"]
	if !ok {
		return nil, true, nil
	}
	values, ok := rawPackages.([]any)
	if !ok {
		return nil, false, errors.New("pnpm workspace packages must be an array")
	}
	patterns := make([]string, 0, len(values))
	for _, value := range values {
		pattern, ok := value.(string)
		if !ok {
			return nil, false, errors.New("pnpm workspace patterns must be strings")
		}
		patterns = append(patterns, pattern)
	}
	return patterns, false, nil
}

func validateWorkspacePattern(pattern string) error {
	if pattern == "" || strings.HasPrefix(pattern, "/") || strings.Contains(pattern, `\`) ||
		strings.ContainsAny(pattern, "!{}[]()?") {
		return errors.New("unsupported workspace pattern")
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "" || segment == "." || segment == ".." ||
			(strings.Contains(segment, "*") && segment != "*" && segment != "**") {
			return errors.New("unsupported workspace pattern segment")
		}
	}
	return nil
}

func matchWorkspacePattern(pattern, relative string) bool {
	patternSegments := strings.Split(pattern, "/")
	var relativeSegments []string
	if relative != "." {
		relativeSegments = strings.Split(relative, "/")
	}
	var match func(int, int) bool
	match = func(patternIndex, relativeIndex int) bool {
		if patternIndex == len(patternSegments) {
			return relativeIndex == len(relativeSegments)
		}
		segment := patternSegments[patternIndex]
		if segment == "**" {
			for next := relativeIndex; next <= len(relativeSegments); next++ {
				if match(patternIndex+1, next) {
					return true
				}
			}
			return false
		}
		if relativeIndex == len(relativeSegments) {
			return false
		}
		if segment != "*" && segment != relativeSegments[relativeIndex] {
			return false
		}
		return match(patternIndex+1, relativeIndex+1)
	}
	return match(0, 0)
}
