package diagnostic

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestPackageManifestNullMembers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		metadata string
		valid    bool
	}{
		{"manifest without nulls", `{"package_manifest":{"description":"Example package.","keywords":["example","widget"]}}`, true},
		{"empty manifest", `{"package_manifest":{}}`, true},
		{"null description", `{"package_manifest":{"description":null}}`, false},
		{"null keyword", `{"package_manifest":{"keywords":[null]}}`, false},
		{"null keyword after safe keyword", `{"package_manifest":{"keywords":["safe",null]}}`, false},
		{"null repository", `{"package_manifest":{"repository":null}}`, false},
		{"null repository object member", `{"package_manifest":{"repository":{"type":"git","url":null}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Given a report carrying diagnostic_metadata with the candidate package_manifest.
			input := []byte(`{"schema_version":"1","result":"pass","exit_code":0,"primary_id":null,"run_invocation":null,"diagnostics":[],"diagnostic_metadata":` + tc.metadata + `}`)
			var report Report
			// When the report is decoded and validated.
			err := json.Unmarshal(input, &report)
			if err == nil {
				err = report.Validate()
			}
			// Then explicit nulls at any depth fail the closed metadata contract.
			if !tc.valid {
				var contract *ContractError
				if !errors.As(err, &contract) || contract.ID != IDDiagnosticsContractInvalid {
					t.Fatalf("error = %v, want diagnostics-contract-invalid", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPublishNPMMetadataContract(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		metadata string
		valid    bool
	}{
		{"valid", `{"publish_npm_version":"11.21.0"}`, true},
		{"with-manifest", `{"package_manifest":{},"publish_npm_version":"11.21.0"}`, true},
		{"short", `{"publish_npm_version":"11.21"}`, false},
		{"null", `{"package_manifest":{},"publish_npm_version":null}`, false},
		{"number", `{"publish_npm_version":11}`, false},
		{"empty", `{"publish_npm_version":""}`, false},
		{"leading-zero", `{"publish_npm_version":"011.21.0"}`, false},
		{"unknown", `{"package_manifest":{},"extra":"value"}`, false},
		{"duplicate", `{"publish_npm_version":"11.21.0","publish_npm_version":"11.20.0"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Given
			input := []byte(`{"schema_version":"1","result":"pass","exit_code":0,"primary_id":null,"run_invocation":null,"diagnostics":[],"diagnostic_metadata":` + tc.metadata + `}`)
			var report Report
			// When
			err := json.Unmarshal(input, &report)
			if err == nil {
				err = report.Validate()
			}
			// Then
			if !tc.valid {
				var contract *ContractError
				if !errors.As(err, &contract) || contract.ID != IDDiagnosticsContractInvalid {
					t.Fatalf("error = %v, want diagnostics-contract-invalid", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := report.CanonicalJSON()
			if err != nil || !bytes.Contains(encoded, []byte(`"publish_npm_version":"11.21.0"`)) {
				t.Fatalf("round trip = %s, %v", encoded, err)
			}
		})
	}
}
