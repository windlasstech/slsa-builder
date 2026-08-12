package fixture

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNPMFixturesNeverPublish(t *testing.T) {
	t.Parallel()

	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	index, err := Load(filepath.Join(repositoryRoot, "testdata", "fixtures", "index.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantCases := map[string]string{
		"npm-root-valid":                     "",
		"pnpm-root-valid":                    "",
		"pnpm-settings-only-root-valid":      "",
		"yarn-v4-root-valid":                 "",
		"npm-workspace-valid":                "",
		"npm-stale-lockfiles-valid":          "",
		"pnpm-stale-lockfiles-valid":         "",
		"yarn-stale-lockfiles-valid":         "",
		"lockfile-only-ambiguous":            "windlass.verify.error.package-manager-conflict",
		"selected-lockfile-missing":          "windlass.verify.error.required-lockfile-missing",
		"pnpm-version-missing":               "windlass.verify.error.package-manager-version-required",
		"yarn-classic-rejected":              "windlass.verify.error.yarn-selection-invalid",
		"yarn-devengines-rejected":           "windlass.verify.error.yarn-selection-invalid",
		"private-package-rejected":           "windlass.verify.error.package-private",
		"package-metadata-missing-rejected":  "windlass.verify.error.package-metadata-required",
		"unsupported-manager-rejected":       "windlass.verify.error.package-manager-conflict",
		"yarn-lockfile-only-rejected":        "windlass.verify.error.yarn-selection-invalid",
		"pnpm-workspace-packages-wrong-type": "windlass.verify.error.package-resolution-invalid",
		"pnpm-settings-only-subdirectory":    "windlass.verify.error.package-resolution-invalid",
	}

	for _, manifest := range index.Fixtures {
		wantPrimaryID, wanted := wantCases[manifest.Name]
		if !wanted {
			continue
		}
		delete(wantCases, manifest.Name)

		if manifest.Surface != "npm" {
			t.Errorf("fixture %q surface = %q, want npm", manifest.Name, manifest.Surface)
		}
		if !strings.HasPrefix(manifest.Artifact, "testdata/npm/") ||
			!strings.HasPrefix(manifest.Provenance, "testdata/npm/") {
			t.Errorf("fixture %q paths must stay under testdata/npm", manifest.Name)
		}
		if wantPrimaryID == "" {
			if manifest.Type != "accepted" || manifest.ExpectedPrimaryID != nil {
				t.Errorf("fixture %q must be accepted without a primary diagnostic", manifest.Name)
			}
			continue
		}
		if manifest.Type != "rejected" || manifest.ExpectedPrimaryID == nil || *manifest.ExpectedPrimaryID != wantPrimaryID {
			t.Errorf("fixture %q primary ID = %v, want %q", manifest.Name, manifest.ExpectedPrimaryID, wantPrimaryID)
		}
	}

	for name := range wantCases {
		t.Errorf("fixture %q is not registered", name)
	}

	packagesRoot := filepath.Join(repositoryRoot, "testdata", "npm", "packages")
	err = filepath.WalkDir(packagesRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || (entry.Name() != "package.json" && entry.Name() != "fixture.json") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var manifest struct {
			Scripts  map[string]string   `json:"scripts"`
			Commands map[string][]string `json:"commands"`
		}
		if decodeErr := json.Unmarshal(data, &manifest); decodeErr != nil {
			return nil
		}
		for script, command := range manifest.Scripts {
			if strings.Contains(command, "npm publish") {
				t.Errorf("%s script %q contains forbidden npm publish command", path, script)
			}
		}
		for step, command := range manifest.Commands {
			if strings.Contains(strings.Join(command, " "), "npm publish") {
				t.Errorf("%s command %q contains forbidden npm publish command", path, step)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan npm fixture commands: %v", err)
	}
}
