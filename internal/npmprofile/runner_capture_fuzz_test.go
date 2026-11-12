package npmprofile

import (
	"encoding/json"
	"strings"
	"testing"
)

// FuzzParseRunnerMetadata exercises the on-disk runner image metadata parser
// against untrusted-shaped JSON. Accepted captures must carry the exact
// governed fields, and rejections must fail closed with the input-unavailable
// diagnostic, never a panic.
func FuzzParseRunnerMetadata(f *testing.F) {
	validDocument, err := json.Marshal([]map[string]string{
		{"group": "Operating System", "detail": "Ubuntu\n24.04.5\nLTS"},
		{"group": "Runner Image", "detail": validRunnerDetail()},
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(validDocument, "ubuntu24", "20261026.354.1")
	f.Add([]byte(`[{"group":"Runner Image","detail":"Image: ubuntu-24.04\nVersion: 20261025.353.1"}]`), "", "")
	f.Add([]byte(`{"group":"Runner Image"}`), "", "")
	f.Add([]byte(`not json`), "", "")
	f.Add([]byte{}, "", "")
	f.Fuzz(func(t *testing.T, encoded []byte, imageOS, deploymentVersion string) {
		capture, err := parseRunnerMetadata(encoded, imageOS, deploymentVersion)
		if err != nil {
			requireNPMDiagnostic(t, err, "windlass.verify.error.input-unavailable")
			return
		}
		if capture.ImageLabel == "" || capture.ImageVersion == "" || capture.OperatingSystem == "" ||
			capture.IncludedSoftwareURL == "" || capture.ImageReleaseURL == "" {
			t.Fatalf("accepted capture lacks governed fields: %#v", capture)
		}
		if !validRunnerSoftwareURI(capture.IncludedSoftwareURL) || !validRunnerReleaseURI(capture.ImageReleaseURL) {
			t.Fatalf("accepted capture carries non-canonical URIs: %#v", capture)
		}
		if projection, ok := runnerImageReleaseProjection(capture.ImageVersion); !ok ||
			!strings.HasSuffix(capture.IncludedSoftwareURL, projection+"/images/ubuntu/Ubuntu2404-Readme.md") {
			t.Fatalf("accepted capture identity disagrees with its version: %#v", capture)
		}
	})
}
