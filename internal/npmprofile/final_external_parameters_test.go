package npmprofile_test

import (
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestFinalExternalParameters(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		ref    string
		source string
	}{
		{name: "omitted selector", ref: "refs/tags/v1.2.3"},
		{name: "dispatch retry", ref: "refs/heads/main", source: `,"source":{"ref":"refs/tags/v1.2.3"}`},
		{name: "explicit equal tag", ref: "refs/tags/v1.2.3", source: `,"source":{"ref":"refs/tags/v1.2.3"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given: only actual interface inputs, with exact selector suppliedness.
			input := finalExternalParametersJSON(test.ref, test.source)
			// When: decode and re-encode the closed interface.
			parameters, err := npmprofile.DecodeExternalParameters(input)
			if err != nil {
				t.Fatalf("decode final external parameters: %v", err)
			}
			encoded, err := npmprofile.EncodeExternalParameters(parameters)
			if err != nil {
				t.Fatalf("encode final external parameters: %v", err)
			}
			// Then: no derived groups or invented source selector appear.
			equal, err := canonicaljson.Equal(input, encoded)
			if err != nil || !equal {
				t.Fatalf("interface round trip changed supplied inputs: %s (error: %v)", encoded, err)
			}
		})
	}
}

func TestFinalExternalParametersRejectsRemovedMembers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		group  string
		member string
	}{
		{group: "runtime"}, {group: "package_manager"}, {group: "release"},
		{group: "caller"}, {group: "build"},
		{group: "source", member: "repository"}, {group: "source", member: "revision"},
		{group: "source", member: "ref_type"}, {group: "source", member: "input_ref"},
		{group: "source", member: "invocation_ref"}, {group: "source", member: "invocation_revision"},
		{group: "source", member: "event_name"},
		{group: "package", member: "name"}, {group: "package", member: "version"},
		{group: "package", member: "private"}, {group: "package", member: "repository"},
		{group: "package", member: "workspace_root"}, {group: "package", member: "source_manifest"},
		{group: "package", member: "tarball_name"}, {group: "package", member: "package_url"},
		{group: "package", member: "packed_name"}, {group: "package", member: "packed_version"},
		{group: "package", member: "packed_files"}, {group: "package", member: "consumer_surface"},
		{group: "package", member: "publish_config_raw"},
		{group: "publish", member: "publish_config"}, {group: "publish", member: "resolved_registry_url"},
		{group: "publish", member: "resolved_dist_tag"}, {group: "publish", member: "publish_access_option"},
		{group: "publish", member: "effective_access"}, {group: "publish", member: "trusted_publishing"},
		{group: "publish", member: "provenance_file"}, {group: "publish", member: "custom_registry_support"},
		{group: "publish", member: "package_identity_preexisting"}, {group: "publish", member: "package_version_preexisting"},
		{group: "distribution", member: "release_tag_supplied"}, {group: "distribution", member: "provenance_sidecar"},
	} {
		t.Run(test.group+"/"+test.member, func(t *testing.T) {
			t.Parallel()
			// Given: a valid final interface, then one forbidden member.
			input := finalExternalParametersJSON("refs/heads/main", `,"source":{"ref":"refs/tags/v1.2.3"}`)
			if _, err := npmprofile.DecodeExternalParameters(input); err != nil {
				t.Fatalf("valid control rejected before mutation: %v", err)
			}
			var object map[string]map[string]json.RawMessage
			if err := json.Unmarshal(input, &object); err != nil {
				t.Fatal(err)
			}
			if test.member == "" {
				object[test.group] = map[string]json.RawMessage{}
			} else {
				object[test.group][test.member] = json.RawMessage(`null`)
			}
			mutated, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			// When: the forbidden value crosses the decoder boundary.
			_, err = npmprofile.DecodeExternalParameters(mutated)
			// Then: even null-valued removed members must not be silently ignored.
			if err == nil {
				t.Fatal("accepted removed external parameter")
			}
		})
	}
}

func finalExternalParametersJSON(ref, source string) []byte {
	return []byte(`{"workflow":{"ref":"` + ref + `","repository":"https://github.com/example/project","path":".github/workflows/publish.yml"},"package":{"directory":"."},"publish":{"input_registry_url":null,"input_dist_tag":null,"input_access":null},"distribution":{"release_asset_mode":false,"linked_artifact_metadata":false}` + source + `}`)
}
