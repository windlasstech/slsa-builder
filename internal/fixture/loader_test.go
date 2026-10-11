package fixture

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stringPointer(value string) *string {
	return &value
}

func validAcceptedManifest() Manifest {
	return Manifest{
		Name:                 "case",
		Type:                 "accepted",
		Surface:              "npm",
		Artifact:             "testdata/npm/case/artifact.tgz",
		Provenance:           "testdata/npm/case/provenance.json",
		ExpectedResult:       "pass",
		ExpectedSecondaryIDs: []string{},
		CoveredRequirement:   "ADR-0014.package-manager-matrix",
	}
}

func validRejectedManifest() Manifest {
	manifest := validAcceptedManifest()
	manifest.Type = "rejected"
	manifest.ExpectedResult = "fail"
	manifest.ExpectedFailureCategory = stringPointer("diagnostics-contract-invalid")
	manifest.ExpectedPrimaryID = stringPointer("windlass.verify.error.diagnostics-contract-invalid")
	return manifest
}

func TestValidateManifestEdges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Manifest)
		wantErr string
	}{
		{name: "valid accepted", mutate: func(*Manifest) {}},
		{
			name: "valid rejected with secondary IDs",
			mutate: func(manifest *Manifest) {
				*manifest = validRejectedManifest()
				manifest.ExpectedSecondaryIDs = []string{"windlass.verify.warning.npm-version-mismatch"}
			},
		},
		{
			name:    "empty name",
			mutate:  func(manifest *Manifest) { manifest.Name = "" },
			wantErr: "fixture 0: name must not be empty",
		},
		{
			name:    "unknown type",
			mutate:  func(manifest *Manifest) { manifest.Type = "unknown" },
			wantErr: `fixture 0: type "unknown" is not accepted or rejected`,
		},
		{
			name:    "unregistered surface",
			mutate:  func(manifest *Manifest) { manifest.Surface = "maven" },
			wantErr: `fixture 0: surface "maven" is not registered`,
		},
		{
			name:    "empty artifact path",
			mutate:  func(manifest *Manifest) { manifest.Artifact = "" },
			wantErr: `fixture 0: artifact path "" must be a relative path under testdata`,
		},
		{
			name:    "artifact path with backslash",
			mutate:  func(manifest *Manifest) { manifest.Artifact = `testdata\npm\case\artifact.tgz` },
			wantErr: `fixture 0: artifact path "testdata\\npm\\case\\artifact.tgz" must be a relative path under testdata`,
		},
		{
			name:    "absolute artifact path",
			mutate:  func(manifest *Manifest) { manifest.Artifact = "/testdata/npm/case/artifact.tgz" },
			wantErr: `fixture 0: artifact path "/testdata/npm/case/artifact.tgz" must be a relative path under testdata`,
		},
		{
			name:    "artifact path escapes testdata",
			mutate:  func(manifest *Manifest) { manifest.Artifact = "../artifact.tgz" },
			wantErr: `fixture 0: artifact path "../artifact.tgz" escapes testdata`,
		},
		{
			name:    "artifact path is the testdata root",
			mutate:  func(manifest *Manifest) { manifest.Artifact = "testdata" },
			wantErr: `fixture 0: artifact path "testdata" escapes testdata`,
		},
		{
			name:    "provenance path escapes testdata",
			mutate:  func(manifest *Manifest) { manifest.Provenance = "../provenance.json" },
			wantErr: `fixture 0: provenance path "../provenance.json" escapes testdata`,
		},
		{
			name: "release manifest path escapes testdata",
			mutate: func(manifest *Manifest) {
				manifest.ReleaseManifest = stringPointer("../manifest.json")
			},
			wantErr: `fixture 0: release-manifest path "../manifest.json" escapes testdata`,
		},
		{
			name:    "unmapped covered requirement",
			mutate:  func(manifest *Manifest) { manifest.CoveredRequirement = "ARCH-unknown.missing" },
			wantErr: `fixture 0: covered-requirement "ARCH-unknown.missing" is not a mapped requirement ID`,
		},
		{
			name: "unregistered failure category",
			mutate: func(manifest *Manifest) {
				*manifest = validRejectedManifest()
				manifest.ExpectedFailureCategory = stringPointer("not-registered")
				manifest.ExpectedPrimaryID = stringPointer("windlass.verify.error.not-registered")
			},
			wantErr: `fixture 0: failure category "not-registered" is not registered`,
		},
		{
			name:    "missing secondary IDs array",
			mutate:  func(manifest *Manifest) { manifest.ExpectedSecondaryIDs = nil },
			wantErr: "fixture 0: expected-secondary-ids must be an array",
		},
		{
			name: "secondary ID duplicates primary ID",
			mutate: func(manifest *Manifest) {
				*manifest = validRejectedManifest()
				manifest.ExpectedSecondaryIDs = []string{"windlass.verify.error.diagnostics-contract-invalid"}
			},
			wantErr: `fixture 0: secondary diagnostic ID "windlass.verify.error.diagnostics-contract-invalid" duplicates the primary ID`,
		},
		{
			name: "duplicate secondary IDs",
			mutate: func(manifest *Manifest) {
				manifest.ExpectedSecondaryIDs = []string{
					"windlass.verify.warning.npm-version-mismatch",
					"windlass.verify.warning.npm-version-mismatch",
				}
			},
			wantErr: `fixture 0: duplicate secondary diagnostic ID "windlass.verify.warning.npm-version-mismatch"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manifest := validAcceptedManifest()
			test.mutate(&manifest)
			err := Validate(Index{Fixtures: []Manifest{manifest}})
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", test.wantErr)
			}
			if err.Error() != test.wantErr {
				t.Fatalf("Validate() error = %q, want %q", err.Error(), test.wantErr)
			}
		})
	}
}

func loadFromString(t *testing.T, contents string) (Index, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "index.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	return Load(path)
}

func TestLoadRejections(t *testing.T) {
	t.Parallel()

	validManifest := `{
		"name":"case",
		"type":"accepted",
		"surface":"npm",
		"artifact":"testdata/npm/case/artifact.tgz",
		"provenance":"testdata/npm/case/provenance.json",
		"release-manifest":null,
		"expected-result":"pass",
		"expected-failure-category":null,
		"expected-primary-id":null,
		"expected-secondary-ids":[],
		"covered-requirement":"ADR-0014.package-manager-matrix"
	}`

	tests := []struct {
		name    string
		index   string
		wantErr string
	}{
		{
			name:    "trailing JSON value",
			index:   `{"fixtures":[]} {"extra":true}`,
			wantErr: "multiple JSON values",
		},
		{
			name:    "trailing garbage",
			index:   `{"fixtures":[]} trailing`,
			wantErr: "read trailing JSON",
		},
		{
			name:    "unterminated array",
			index:   `[`,
			wantErr: "read JSON token",
		},
		{
			name:    "unterminated object",
			index:   `{"fixtures":[` + validManifest,
			wantErr: "read JSON token",
		},
		{
			name:    "mismatched array closer",
			index:   `[}`,
			wantErr: "close JSON array",
		},
		{
			name:    "mismatched object closer",
			index:   `{]`,
			wantErr: "close JSON object",
		},
		{
			name:    "empty document",
			index:   `{}`,
			wantErr: "fixture index must contain only the fixtures member",
		},
		{
			name:    "extra top-level member",
			index:   `{"fixtures":[],"extra":true}`,
			wantErr: "fixture index must contain only the fixtures member",
		},
		{
			name:    "missing fixtures member",
			index:   `{"manifests":[]}`,
			wantErr: `fixture index is missing required field "fixtures"`,
		},
		{
			name:    "fixtures member is an object",
			index:   `{"fixtures":{}}`,
			wantErr: `fixture index field "fixtures" must be an array`,
		},
		{
			name:    "fixtures member is null",
			index:   `{"fixtures":null}`,
			wantErr: `fixture index field "fixtures" must be an array`,
		},
		{
			name:    "manifest is not an object",
			index:   `{"fixtures":[1]}`,
			wantErr: "fixture 0: manifest must be an object",
		},
		{
			name: "manifest misses a required field",
			index: `{"fixtures":[{
				"name":"case",
				"type":"accepted",
				"surface":"npm",
				"artifact":"testdata/npm/case/artifact.tgz",
				"provenance":"testdata/npm/case/provenance.json",
				"release-manifest":null,
				"expected-result":"pass",
				"expected-failure-category":null,
				"expected-primary-id":null,
				"expected-secondary-ids":[]
			}]}`,
			wantErr: `fixture 0: manifest is missing required field "covered-requirement"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := loadFromString(t, test.index)
			if err == nil {
				t.Fatalf("Load() error = nil, want an error containing %q", test.wantErr)
			}
			if !errors.Is(err, ErrInvalidIndex) {
				t.Fatalf("Load() error = %v, want errors.Is match for ErrInvalidIndex", err)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Load() error = %q, want substring %q", err.Error(), test.wantErr)
			}
		})
	}
}

func TestLoadUnreadableIndex(t *testing.T) {
	t.Parallel()

	_, err := Load(filepath.Join(t.TempDir(), "missing", "index.json"))
	if err == nil {
		t.Fatal("Load() error = nil, want ErrInputUnavailable")
	}
	if !errors.Is(err, ErrInputUnavailable) {
		t.Fatalf("Load() error = %v, want errors.Is match for ErrInputUnavailable", err)
	}
	if errors.Is(err, ErrInvalidIndex) {
		t.Fatalf("Load() error = %v must not also match ErrInvalidIndex", err)
	}
}
