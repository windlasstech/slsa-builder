package fixture

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestNPMFixturesNeverPublish(t *testing.T) {
	t.Parallel()

	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	index, err := Load(filepath.Join(repositoryRoot, "testdata", "fixtures", "index.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantCases := map[string]string{
		"npm-root-valid":                                            "",
		"npm-purl-publish-11-21-0-scoped-valid":                     "",
		"npm-purl-publish-11-21-0-unscoped-valid":                   "",
		"npm-purl-build-11-19-0-scoped-valid":                       "",
		"npm-purl-build-11-19-0-unscoped-valid":                     "",
		"pnpm-root-valid":                                           "",
		"pnpm-settings-only-root-valid":                             "",
		"yarn-v4-root-valid":                                        "",
		"npm-workspace-valid":                                       "",
		"npm-stale-lockfiles-valid":                                 "",
		"pnpm-stale-lockfiles-valid":                                "",
		"yarn-stale-lockfiles-valid":                                "",
		"lockfile-only-ambiguous":                                   "windlass.verify.error.package-manager-conflict",
		"selected-lockfile-missing":                                 "windlass.verify.error.required-lockfile-missing",
		"pnpm-version-missing":                                      "windlass.verify.error.package-manager-version-required",
		"pnpm-bare-major":                                           "windlass.verify.error.package-manager-version-required",
		"pnpm-bare-minor":                                           "windlass.verify.error.package-manager-version-required",
		"pnpm-devengines-bare-major":                                "windlass.verify.error.package-manager-version-required",
		"pnpm-12-unsupported":                                       "windlass.verify.error.pnpm-version-unsupported",
		"pnpm-13-unsupported":                                       "windlass.verify.error.pnpm-version-unsupported",
		"pnpm-12-devengines-unsupported":                            "windlass.verify.error.pnpm-version-unsupported",
		"pnpm-13-devengines-unsupported":                            "windlass.verify.error.pnpm-version-unsupported",
		"yarn-6-unsupported":                                        "windlass.verify.error.yarn-version-unsupported",
		"yarn-v5-root-valid":                                        "",
		"pnpm-devengines-root-valid":                                "",
		"pnpm-10-root-valid":                                        "",
		"pnpm-10-devengines-root-valid":                             "",
		"yarn-devengines-root-valid":                                "",
		"pnpm-digest-unknown-algorithm":                             "windlass.verify.error.package-manager-digest-malformed",
		"pnpm-digest-uppercase-hex":                                 "windlass.verify.error.package-manager-digest-malformed",
		"pnpm-digest-wrong-length":                                  "windlass.verify.error.package-manager-digest-malformed",
		"pnpm-digest-devengines-missing-hex":                        "windlass.verify.error.package-manager-digest-malformed",
		"pnpm-digest-extra-identifiers":                             "windlass.verify.error.package-manager-digest-malformed",
		"pnpm-build-metadata":                                       "windlass.verify.error.package-manager-digest-malformed",
		"yarn-build-metadata-devengines":                            "windlass.verify.error.package-manager-digest-malformed",
		"yarn-digest-unknown-algorithm":                             "windlass.verify.error.package-manager-digest-malformed",
		"npm-digest-suffix":                                         "windlass.verify.error.package-manager-digest-malformed",
		"npm-digest-devengines":                                     "windlass.verify.error.package-manager-digest-malformed",
		"pnpm-9-unsupported":                                        "windlass.verify.error.pnpm-version-unsupported",
		"pnpm-9-devengines-unsupported":                             "windlass.verify.error.pnpm-version-unsupported",
		"pnpm-digest-sha512-valid":                                  "",
		"pnpm-digest-sha256-devengines-valid":                       "",
		"yarn-digest-sha384-valid":                                  "",
		"yarn-digest-sha512-devengines-valid":                       "",
		"npm-declared-version-match-valid":                          "",
		"npm-declared-version-mismatch-valid":                       "",
		"npm-corepack-env-ignored-valid":                            "",
		"yarn-classic-rejected":                                     "windlass.verify.error.yarn-selection-invalid",
		"yarn-bare-major":                                           "windlass.verify.error.yarn-selection-invalid",
		"yarn-bare-minor":                                           "windlass.verify.error.yarn-selection-invalid",
		"private-package-rejected":                                  "windlass.verify.error.package-private",
		"package-metadata-missing-rejected":                         "windlass.verify.error.package-metadata-required",
		"unsupported-manager-rejected":                              "windlass.verify.error.package-manager-conflict",
		"yarn-lockfile-only-rejected":                               "windlass.verify.error.yarn-selection-invalid",
		"pnpm-workspace-packages-wrong-type":                        "windlass.verify.error.package-resolution-invalid",
		"pnpm-settings-only-subdirectory":                           "windlass.verify.error.package-resolution-invalid",
		"npm-source-ref-non-tag-rejected":                           "windlass.verify.error.source-ref-invalid",
		"npm-source-ref-unresolvable-rejected":                      "windlass.verify.error.source-ref-invalid",
		"npm-source-ref-version-mismatch-rejected":                  "windlass.verify.error.source-ref-invalid",
		"npm-source-ref-invocation-conflict-rejected":               "windlass.verify.error.source-ref-invalid",
		"npm-source-ref-dispatch-retry-valid":                       "",
		"npm-source-ref-omitted-valid":                              "",
		"npm-source-ref-input-ref-mismatch-rejected":                "windlass.verify.error.source-ref-invalid",
		"npm-source-ref-invocation-without-input-ref-rejected":      "windlass.verify.error.unexpected-external-parameters",
		"npm-source-ref-invocation-missing-with-input-ref-rejected": "windlass.verify.error.unexpected-external-parameters",
		"npm-source-ref-certificate-invocation-mismatch-rejected":   "windlass.verify.error.source-ref-mismatch",
		"npm-source-ref-built-policy-mismatch-rejected":             "windlass.verify.error.source-digest-mismatch",
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

func TestNPMPURLCompatibilityFixtures(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name        string
		cliVersion  string
		subject     string
		requirement string
	}{
		"npm-purl-publish-11-21-0-scoped-valid":   {"@windlass/purl-compat", "11.21.0", "pkg:npm/%40windlass/purl-compat@0.1.0", "ADR-0082.publish-npm-purl-compat"},
		"npm-purl-publish-11-21-0-unscoped-valid": {"purl-compat", "11.21.0", "pkg:npm/purl-compat@0.1.0", "ADR-0082.publish-npm-purl-compat"},
		"npm-purl-build-11-19-0-scoped-valid":     {"@windlass/purl-compat", "11.19.0", "pkg:npm/%40windlass/purl-compat@0.1.0", "ADR-0085.build-npm-purl-compat"},
		"npm-purl-build-11-19-0-unscoped-valid":   {"purl-compat", "11.19.0", "pkg:npm/purl-compat@0.1.0", "ADR-0085.build-npm-purl-compat"},
	}

	root := filepath.Join("..", "..")
	index, err := Load(filepath.Join(root, "testdata", "fixtures", "index.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, manifest := range index.Fixtures {
		want, ok := cases[manifest.Name]
		if !ok {
			continue
		}
		delete(cases, manifest.Name)
		t.Run(manifest.Name, func(t *testing.T) {
			if manifest.CoveredRequirement != want.requirement {
				t.Fatalf("covered requirement = %q, want %q", manifest.CoveredRequirement, want.requirement)
			}
			packageBytes, err := os.ReadFile(filepath.Join(root, manifest.Artifact))
			if err != nil {
				t.Fatal(err)
			}
			var pkg struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}
			if err := json.Unmarshal(packageBytes, &pkg); err != nil {
				t.Fatal(err)
			}
			fixtureBytes, err := os.ReadFile(filepath.Join(root, manifest.Provenance))
			if err != nil {
				t.Fatal(err)
			}
			var fixture struct {
				Phase              string `json:"phase"`
				VerifiedNPMVersion string `json:"verified-npm-cli-version"`
				ExpectedSubject    string `json:"expected-subject"`
			}
			if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
				t.Fatal(err)
			}
			if fixture.Phase != "build-pack" || fixture.VerifiedNPMVersion != want.cliVersion || pkg.Name != want.name || pkg.Version != "0.1.0" || fixture.ExpectedSubject != want.subject {
				t.Fatalf("fixture metadata = (%q, %q, %q, %q@%q), want (build-pack, %q, %q, %q@0.1.0)", fixture.Phase, fixture.VerifiedNPMVersion, fixture.ExpectedSubject, pkg.Name, pkg.Version, want.cliVersion, want.subject, want.name)
			}
			got, err := npmprofile.NPMPackagePURL(pkg.Name, pkg.Version)
			if err != nil {
				t.Fatal(err)
			}
			if got != want.subject {
				t.Errorf("NPMPackagePURL(%q, %q) = %q, want %q", pkg.Name, pkg.Version, got, want.subject)
			}
		})
	}
	for name := range cases {
		t.Errorf("fixture %q is not registered", name)
	}
}
