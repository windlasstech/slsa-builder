package npmprofile

import (
	"strings"
	"testing"
)

func TestFinalExternalParametersBoundaryRejections(t *testing.T) {
	t.Parallel()
	encoded, err := EncodeExternalParameters(validExternalParameters(ManagerNPM))
	if err != nil {
		t.Fatal(err)
	}
	base := string(encoded)
	for _, test := range []struct {
		name   string
		mutate func(string) string
		wantID string
	}{
		{name: "workflow ref wrong type", wantID: IDUnexpectedExternalParameters, mutate: func(s string) string {
			return strings.Replace(s, `"ref":"refs/tags/v1.2.3"`, `"ref":1`, 1)
		}},
		{name: "empty source selector", wantID: IDSourceRefInvalid, mutate: func(s string) string {
			return strings.Replace(s, `{"distribution":`, `{"source":{"ref":""},"distribution":`, 1)
		}},
		{name: "absolute package directory", wantID: IDPackageResolutionInvalid, mutate: func(s string) string {
			return strings.Replace(s, `"directory":"."`, `"directory":"/abs"`, 1)
		}},
		{name: "traversing package directory", wantID: IDPackageResolutionInvalid, mutate: func(s string) string {
			return strings.Replace(s, `"directory":"."`, `"directory":"../x"`, 1)
		}},
		{name: "release asset mode is not implemented", wantID: "windlass.verify.error.release-asset-mode-schema-error", mutate: func(s string) string {
			return strings.Replace(s, `"release_asset_mode":false`, `"release_asset_mode":true`, 1)
		}},
		{name: "whitespace dist tag is not an omission", wantID: IDUnexpectedExternalParameters, mutate: func(s string) string {
			return strings.Replace(s, `"input_dist_tag":null`, `"input_dist_tag":" "`, 1)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeExternalParameters([]byte(test.mutate(base)))
			requireNPMDiagnostic(t, err, test.wantID)
		})
	}
}
