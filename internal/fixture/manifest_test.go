package fixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestSchema(t *testing.T) {
	t.Parallel()

	validAccepted := `{
		"name":"fixture-harness-schema-valid",
		"type":"accepted",
		"surface":"npm",
		"artifact":"testdata/fixtures/data/artifact.json",
		"provenance":"testdata/fixtures/data/provenance.json",
		"release-manifest":null,
		"expected-result":"pass",
		"expected-failure-category":null,
		"expected-primary-id":null,
		"expected-secondary-ids":[],
		"covered-requirement":"ARCH-verification-policy-and-fixtures.fixture-manifest-schema"
	}`
	validRejected := `{
		"name":"fixture-harness-schema-rejected",
		"type":"rejected",
		"surface":"npm",
		"artifact":"testdata/fixtures/data/artifact.json",
		"provenance":"testdata/fixtures/data/provenance.json",
		"release-manifest":null,
		"expected-result":"fail",
		"expected-failure-category":"diagnostics-contract-invalid",
		"expected-primary-id":"windlass.verify.error.diagnostics-contract-invalid",
		"expected-secondary-ids":[],
		"covered-requirement":"ARCH-verification-policy-and-fixtures.fixture-manifest-schema"
	}`

	tests := []struct {
		name    string
		index   string
		wantErr bool
	}{
		{name: "valid", index: `{"fixtures":[` + validAccepted + `,` + validRejected + `]}`},
		{name: "duplicate fixture names", index: `{"fixtures":[` + validAccepted + `,` + validAccepted + `]}`, wantErr: true},
		{name: "unknown manifest field", index: `{"fixtures":[` + strings.Replace(validAccepted, `"name":`, `"unknown":true,"name":`, 1) + `]}`, wantErr: true},
		{name: "duplicate JSON member", index: `{"fixtures":[` + strings.Replace(validAccepted, `"name":`, `"name":"shadowed","name":`, 1) + `]}`, wantErr: true},
		{name: "path escapes testdata", index: `{"fixtures":[` + strings.Replace(validAccepted, `testdata/fixtures/data/artifact.json`, `../artifact.json`, 1) + `]}`, wantErr: true},
		{name: "rejected fixture missing primary ID", index: `{"fixtures":[` + strings.Replace(validRejected, `"expected-primary-id":"windlass.verify.error.diagnostics-contract-invalid"`, `"expected-primary-id":null`, 1) + `]}`, wantErr: true},
		{name: "accepted result disagreement", index: `{"fixtures":[` + strings.Replace(validAccepted, `"expected-result":"pass"`, `"expected-result":"fail"`, 1) + `]}`, wantErr: true},
		{name: "rejected category and primary ID disagree", index: `{"fixtures":[` + strings.Replace(validRejected, `windlass.verify.error.diagnostics-contract-invalid`, `windlass.verify.error.policy-schema-invalid`, 1) + `]}`, wantErr: true},
		{name: "unmapped requirement", index: `{"fixtures":[` + strings.Replace(validAccepted, `ARCH-verification-policy-and-fixtures.fixture-manifest-schema`, `ARCH-unknown.missing`, 1) + `]}`, wantErr: true},
		{name: "invalid requirement format", index: `{"fixtures":[` + strings.Replace(validAccepted, `ARCH-verification-policy-and-fixtures.fixture-manifest-schema`, `verification policy`, 1) + `]}`, wantErr: true},
		{name: "warning secondary ID", index: `{"fixtures":[` + validAccepted + `,` + strings.Replace(validRejected, `"expected-secondary-ids":[]`, `"expected-secondary-ids":["windlass.verify.warning.npm-version-mismatch"]`, 1) + `]}`},
		{name: "malformed warning secondary ID", index: `{"fixtures":[` + strings.Replace(validRejected, `"expected-secondary-ids":[]`, `"expected-secondary-ids":["windlass.verify.warning.NPM-version-mismatch"]`, 1) + `]}`, wantErr: true},
		{name: "pnpm malformed digest rejection category", index: `{"fixtures":[` + validAccepted + `,` + strings.Replace(strings.Replace(validRejected, `diagnostics-contract-invalid`, `package-manager-digest-malformed`, 2), `ARCH-verification-policy-and-fixtures.fixture-manifest-schema`, `ADR-0092.descriptor-digest-rejections`, 1) + `]}`},
		{name: "pnpm digest mismatch rejection category", index: `{"fixtures":[` + validAccepted + `,` + strings.Replace(strings.Replace(validRejected, `diagnostics-contract-invalid`, `package-manager-digest-mismatch`, 2), `ARCH-verification-policy-and-fixtures.fixture-manifest-schema`, `ADR-0092.descriptor-integrity-digests`, 1) + `]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "index.json")
			if err := os.WriteFile(path, []byte(test.index), 0o600); err != nil {
				t.Fatalf("write index: %v", err)
			}

			index, err := Load(path)
			if test.wantErr {
				if err == nil {
					t.Fatal("Load() error = nil, want schema validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := len(index.Fixtures); got != 2 {
				t.Fatalf("len(Index.Fixtures) = %d, want 2", got)
			}
		})
	}
}

func Test_RequirementIDs_validate_and_match_build_pack(t *testing.T) {
	t.Parallel()

	// Given
	requirements := map[string]string{
		"ADR-0090.pnpm-10x-support":             "pnpm 10.x pins are accepted from both manifest selection sources",
		"ADR-0091.uniform-selection-source":     "any supported package manager is selectable from devEngines.packageManager",
		"ADR-0092.descriptor-integrity-digests": "pnpm and Yarn descriptors may declare an integrity digest reconciled fail-closed",
		"ADR-0092.descriptor-digest-rejections": "malformed descriptor digest suffixes and npm digest suffixes are rejected",
		"ADR-0095.npm-declared-version":         "npm version declarations are non-authoritative with a mismatch warning",
	}

	// When / Then
	for requirement, description := range requirements {
		if !IsRegisteredRequirement(requirement) {
			t.Errorf("IsRegisteredRequirement(%q) = false", requirement)
		}
		if !requirementMatchesPhase(requirement, "build-pack") {
			t.Errorf("requirement %q does not match build-pack", requirement)
		}
		if got := requirementRegistry[requirement]; got != description {
			t.Errorf("requirementRegistry[%q] = %q, want %q", requirement, got, description)
		}
	}
	if got := requirementRegistry["ADR-0088.corepack-window-version-bounds"]; got != "Corepack-window pnpm 10.x/11.x and Yarn v4/v5 version bounds" {
		t.Errorf("ADR-0088 description = %q", got)
	}
}
