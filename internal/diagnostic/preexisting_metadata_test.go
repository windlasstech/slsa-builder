package diagnostic_test

import (
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

func TestPreexistingFlagsDiagnosticMetadata(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		valid bool
	}{
		{`{"publish":{"package_identity_preexisting":true,"package_version_preexisting":false}}`, true},
		{`{"publish":{"package_identity_preexisting":null,"package_version_preexisting":null}}`, true},
		{`{"publish":{"package_version_preexisting":true}}`, true},
		{`{"publish":{}}`, true},
		{`{"publish":null}`, false},
		{`{"publish":{"package_identity_preexisting":"true"}}`, false},
		{`{"publish":{"package_identity_preexisting":1}}`, false},
		{`{"publish":{"package_identity_preexisting":true,"package_identity_preexisting":false}}`, false},
		{`{"publish":{"trusted_publishing":true}}`, false},
	} {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			var metadata diagnostic.DiagnosticMetadata
			err := json.Unmarshal([]byte(test.input), &metadata)
			if (err == nil) != test.valid {
				t.Fatalf("decode error = %v, valid = %v", err, test.valid)
			}
			if !test.valid {
				return
			}
			report, err := diagnostic.Build(nil, nil, &metadata)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := report.CanonicalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var decoded diagnostic.Report
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if err := decoded.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
