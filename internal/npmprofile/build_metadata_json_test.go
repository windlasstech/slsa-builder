package npmprofile

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
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
	t.Run("case-variant Name", func(t *testing.T) {
		t.Parallel()
		// Given otherwise valid metadata with a case-variant descriptor name member.
		metadata := validProvenanceInput(t, ManagerNPM).BuildMetadata
		raw := string(mustJSON(metadata))
		needle := `"name":"lockfile"`
		if strings.Count(raw, needle) != 1 {
			t.Fatal("expected one descriptor mutation target")
		}
		raw = strings.Replace(raw, needle, `"Name":"lockfile"`, 1)
		// When decoding at the strict BuildMetadata JSON boundary.
		var decoded BuildMetadata
		err := json.Unmarshal([]byte(raw), &decoded)
		// Then the exact-key descriptor fallback rejects the unrecognized member.
		const want = "decode build metadata JSON: resolved_dependencies[0] contains unrecognized members"
		if err == nil || err.Error() != want {
			t.Fatalf("BuildMetadata decode error = %v, want %q", err, want)
		}
	})
}

func TestBuildMetadataRejectsDuplicateMember(t *testing.T) {
	t.Parallel()
	// Given otherwise valid metadata with started_on duplicated.
	metadata := validProvenanceInput(t, ManagerNPM).BuildMetadata
	raw := string(mustJSON(metadata))
	needle := `"started_on":`
	if strings.Count(raw, needle) != 1 {
		t.Fatal("expected one started_on mutation target")
	}
	raw = strings.Replace(raw, needle, `"started_on":null,"started_on":`, 1)
	// When decoding at the strict BuildMetadata JSON boundary.
	var decoded BuildMetadata
	err := json.Unmarshal([]byte(raw), &decoded)
	// Then strict validation reports the duplicate member before decoding.
	if err == nil || !strings.HasPrefix(err.Error(), "validate build metadata JSON: ") {
		t.Fatalf("BuildMetadata decode error = %v, want validate prefix", err)
	}
	var duplicate *canonicaljson.DuplicateMemberError
	if !errors.As(err, &duplicate) {
		t.Fatalf("BuildMetadata decode error = %T, want *canonicaljson.DuplicateMemberError", err)
	}
	if duplicate.Member != "started_on" {
		t.Fatalf("duplicate member = %q, want started_on", duplicate.Member)
	}
}

func TestBuildMetadataRejectsMalformedShape(t *testing.T) {
	t.Parallel()
	// Given an array root and valid metadata with a numeric schema_version.
	metadata := validProvenanceInput(t, ManagerNPM).BuildMetadata
	raw := string(mustJSON(metadata))
	needle := `"schema_version":"1"`
	if strings.Count(raw, needle) != 1 {
		t.Fatal("expected one schema_version mutation target")
	}
	tests := []struct {
		name      string
		input     string
		wantValue string
	}{
		{name: "array root", input: "[]", wantValue: "array"},
		{name: "wrong known field type", input: strings.Replace(raw, needle, `"schema_version":1`, 1), wantValue: "number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// When decoding at the strict BuildMetadata JSON boundary.
			var decoded BuildMetadata
			err := json.Unmarshal([]byte(tt.input), &decoded)
			// Then decoding reports the JSON value type mismatch.
			if err == nil || !strings.HasPrefix(err.Error(), "decode build metadata JSON: ") {
				t.Fatalf("BuildMetadata decode error = %v, want decode prefix", err)
			}
			var typeError *json.UnmarshalTypeError
			if !errors.As(err, &typeError) {
				t.Fatalf("BuildMetadata decode error = %T, want *json.UnmarshalTypeError", err)
			}
			if typeError.Value != tt.wantValue {
				t.Fatalf("JSON value type = %q, want %q", typeError.Value, tt.wantValue)
			}
		})
	}
}

func TestBuildMetadataRequiresTimestampMembers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		member    string
		setNull   bool
		wantError string
	}{
		{name: "missing started_on", member: "started_on", wantError: "build metadata requires started_on"},
		{name: "null started_on", member: "started_on", setNull: true, wantError: "build metadata requires started_on"},
		{name: "missing finished_on", member: "finished_on", wantError: "build metadata requires finished_on"},
		{name: "null finished_on", member: "finished_on", setNull: true, wantError: "build metadata requires finished_on"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Given otherwise valid metadata with a required timestamp absent or null.
			metadata := validProvenanceInput(t, ManagerNPM).BuildMetadata
			var members map[string]json.RawMessage
			if err := json.Unmarshal(mustJSON(metadata), &members); err != nil {
				t.Fatalf("decode metadata members: %v", err)
			}
			if tt.setNull {
				members[tt.member] = json.RawMessage("null")
			} else {
				delete(members, tt.member)
			}
			// When decoding at the strict BuildMetadata JSON boundary.
			var decoded BuildMetadata
			err := json.Unmarshal(mustJSON(members), &decoded)
			// Then the corresponding required timestamp error is returned.
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("BuildMetadata decode error = %v, want %q", err, tt.wantError)
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
