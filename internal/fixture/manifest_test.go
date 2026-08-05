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
