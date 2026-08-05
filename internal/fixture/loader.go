package fixture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var diagnosticIDPattern = regexp.MustCompile(`^windlass\.verify\.error\.[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ErrInputUnavailable marks an unreadable local fixture index.
var ErrInputUnavailable = errors.New("fixture input unavailable")

// ErrInvalidIndex marks fixture JSON or manifest contract violations.
var ErrInvalidIndex = errors.New("fixture index invalid")

var requiredManifestFields = []string{
	"name",
	"type",
	"surface",
	"artifact",
	"provenance",
	"release-manifest",
	"expected-result",
	"expected-failure-category",
	"expected-primary-id",
	"expected-secondary-ids",
	"covered-requirement",
}

// Load reads a fixture index, rejects ambiguous JSON, and validates its closed schema.
func Load(path string) (Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Index{}, fmt.Errorf("%w: read fixture index: %w", ErrInputUnavailable, err)
	}
	if err := rejectDuplicateMembers(data); err != nil {
		return Index{}, fmt.Errorf("%w: strict fixture index JSON: %w", ErrInvalidIndex, err)
	}

	index, err := decodeIndex(data)
	if err != nil {
		return Index{}, fmt.Errorf("%w: %w", ErrInvalidIndex, err)
	}
	if err := Validate(index); err != nil {
		return Index{}, fmt.Errorf("%w: %w", ErrInvalidIndex, err)
	}
	return index, nil
}

func decodeIndex(data []byte) (Index, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return Index{}, fmt.Errorf("decode fixture index: %w", err)
	}
	if len(document) != 1 {
		return Index{}, fmt.Errorf("fixture index must contain only the fixtures member")
	}
	rawFixtures, ok := document["fixtures"]
	if !ok {
		return Index{}, fmt.Errorf("fixture index is missing required field %q", "fixtures")
	}

	var manifests []json.RawMessage
	if err := json.Unmarshal(rawFixtures, &manifests); err != nil || manifests == nil {
		return Index{}, fmt.Errorf("fixture index field %q must be an array", "fixtures")
	}

	index := Index{Fixtures: make([]Manifest, 0, len(manifests))}
	for position, raw := range manifests {
		manifest, err := decodeManifest(raw)
		if err != nil {
			return Index{}, fmt.Errorf("fixture %d: %w", position, err)
		}
		index.Fixtures = append(index.Fixtures, manifest)
	}
	return index, nil
}

func decodeManifest(data []byte) (Manifest, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Manifest{}, fmt.Errorf("manifest must be an object: %w", err)
	}
	for _, field := range requiredManifestFields {
		if _, ok := fields[field]; !ok {
			return Manifest{}, fmt.Errorf("manifest is missing required field %q", field)
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode closed manifest schema: %w", err)
	}
	return manifest, nil
}

// Validate checks fixture uniqueness, paths, requirement mappings, and expected-result consistency.
func Validate(index Index) error {
	names := make(map[string]struct{}, len(index.Fixtures))
	for position := range index.Fixtures {
		manifest := &index.Fixtures[position]
		if err := validateManifest(*manifest); err != nil {
			return fmt.Errorf("fixture %d: %w", position, err)
		}
		if _, duplicate := names[manifest.Name]; duplicate {
			return fmt.Errorf("duplicate fixture name %q", manifest.Name)
		}
		names[manifest.Name] = struct{}{}
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if manifest.Name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if manifest.Type != "accepted" && manifest.Type != "rejected" {
		return fmt.Errorf("type %q is not accepted or rejected", manifest.Type)
	}
	if !validSurface(manifest.Surface) {
		return fmt.Errorf("surface %q is not registered", manifest.Surface)
	}
	if err := validateTestdataPath("artifact", manifest.Artifact); err != nil {
		return err
	}
	if err := validateTestdataPath("provenance", manifest.Provenance); err != nil {
		return err
	}
	if manifest.ReleaseManifest != nil {
		if err := validateTestdataPath("release-manifest", *manifest.ReleaseManifest); err != nil {
			return err
		}
	}
	if !IsRegisteredRequirement(manifest.CoveredRequirement) {
		return fmt.Errorf("covered-requirement %q is not a mapped requirement ID", manifest.CoveredRequirement)
	}
	if err := validateExpectations(manifest); err != nil {
		return err
	}
	return validateSecondaryIDs(manifest)
}

func validSurface(surface string) bool {
	switch surface {
	case "npm", "publisher", "composition", "release-manifest":
		return true
	default:
		return false
	}
}

func validateTestdataPath(field, path string) error {
	if path == "" || strings.Contains(path, `\`) || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return fmt.Errorf("%s path %q must be a relative path under testdata", field, path)
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	testdataPrefix := "testdata" + string(filepath.Separator)
	if clean == "testdata" || !strings.HasPrefix(clean, testdataPrefix) {
		return fmt.Errorf("%s path %q escapes testdata", field, path)
	}
	return nil
}

func validateExpectations(manifest Manifest) error {
	if manifest.Type == "accepted" {
		if manifest.ExpectedResult != "pass" || manifest.ExpectedFailureCategory != nil || manifest.ExpectedPrimaryID != nil {
			return fmt.Errorf("accepted fixture result, failure category, and primary ID disagree")
		}
		return nil
	}

	if manifest.ExpectedResult != "fail" || manifest.ExpectedFailureCategory == nil || manifest.ExpectedPrimaryID == nil {
		return fmt.Errorf("rejected fixture requires fail result, failure category, and primary ID")
	}
	category := *manifest.ExpectedFailureCategory
	if _, ok := failureCategoryRegistry[category]; !ok {
		return fmt.Errorf("failure category %q is not registered", category)
	}
	expectedID := "windlass.verify.error." + category
	if *manifest.ExpectedPrimaryID != expectedID {
		return fmt.Errorf("expected-primary-id %q does not correspond to category %q", *manifest.ExpectedPrimaryID, category)
	}
	return nil
}

func validateSecondaryIDs(manifest Manifest) error {
	if manifest.ExpectedSecondaryIDs == nil {
		return fmt.Errorf("expected-secondary-ids must be an array")
	}
	seen := make(map[string]struct{}, len(manifest.ExpectedSecondaryIDs))
	for _, id := range manifest.ExpectedSecondaryIDs {
		if !diagnosticIDPattern.MatchString(id) {
			return fmt.Errorf("secondary diagnostic ID %q is not canonical", id)
		}
		if manifest.ExpectedPrimaryID != nil && id == *manifest.ExpectedPrimaryID {
			return fmt.Errorf("secondary diagnostic ID %q duplicates the primary ID", id)
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("duplicate secondary diagnostic ID %q", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}
