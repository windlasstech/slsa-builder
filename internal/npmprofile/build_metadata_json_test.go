package npmprofile

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildMetadataRejectsDescriptorUnknownMember(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"node-runtime", "lockfile"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given otherwise valid metadata with an unknown descriptor member.
			metadata := validProvenanceInput(t, ManagerNPM).BuildMetadata
			raw := string(mustJSON(metadata))
			needle := `"name":"` + name + `"`
			if strings.Count(raw, needle) != 1 {
				t.Fatal("expected one descriptor mutation target")
			}
			raw = strings.Replace(raw, needle, needle+`,"mediaType":"application/gzip"`, 1)
			// When decoding at the strict BuildMetadata JSON boundary.
			var decoded BuildMetadata
			err := json.Unmarshal([]byte(raw), &decoded)
			// Then rejection occurs during decode, not later semantic validation.
			if err == nil || !strings.Contains(err.Error(), `unknown field "mediaType"`) {
				t.Fatalf("BuildMetadata decode error = %v, want unknown field mediaType", err)
			}
		})
	}
}

func TestBuildMetadataRejectsCaseVariantMember(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"Started_On", "PRIMARY_ARTIFACT"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			// Given valid metadata with one top-level member's casing changed.
			metadata := validProvenanceInput(t, ManagerNPM).BuildMetadata
			raw := string(mustJSON(metadata))
			needle := `"` + strings.ToLower(key) + `":`
			if strings.Count(raw, needle) != 1 {
				t.Fatal("expected one top-level member mutation target")
			}
			raw = strings.Replace(raw, needle, `"`+key+`":`, 1)
			// When decoding at the strict BuildMetadata JSON boundary.
			var decoded BuildMetadata
			err := json.Unmarshal([]byte(raw), &decoded)
			// Then the non-closed member name is rejected, not case-insensitively bound.
			if err == nil || !strings.Contains(err.Error(), `unknown field "`+key+`"`) {
				t.Fatalf("BuildMetadata decode error = %v, want unknown field %s", err, key)
			}
		})
	}
}
