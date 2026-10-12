package diagnostic

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

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
