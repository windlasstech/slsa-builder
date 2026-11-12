package npmprofile_test

import (
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestPackageManifestEvidenceHandoff(t *testing.T) {
	t.Parallel()
	input := []byte(`{"schema_version":"1","started_on":"2026-08-06T12:00:00Z","finished_on":"2026-08-06T12:00:03Z","primary_artifact":{"artifact_name":"tarball","payload_file_name":"example.tgz","sha256":"abc","sha512":"def"},"external_parameters":{},"resolved_dependencies":[],"manifest_evidence":{"path":"package.json","content_base64":"e30="},"build_script_disposition":"skipped-absent"}`)
	var control npmprofile.BuildMetadata
	if err := json.Unmarshal(input, &control); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}
	for _, field := range []string{"schema_version", "started_on", "finished_on", "primary_artifact", "external_parameters", "resolved_dependencies", "manifest_evidence", "build_script_disposition"} {
		for _, value := range []string{"missing", "null"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				t.Parallel()
				var object map[string]json.RawMessage
				if err := json.Unmarshal(input, &object); err != nil {
					t.Fatal(err)
				}
				if value == "missing" {
					delete(object, field)
				} else {
					object[field] = json.RawMessage(`null`)
				}
				mutated, err := json.Marshal(object)
				if err != nil {
					t.Fatal(err)
				}
				var metadata npmprofile.BuildMetadata
				if err := json.Unmarshal(mutated, &metadata); err == nil {
					t.Fatal("accepted incomplete handoff")
				}
			})
		}
	}
}
