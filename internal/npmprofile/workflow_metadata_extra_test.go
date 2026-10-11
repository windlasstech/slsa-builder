package npmprofile

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDecodePublishConfig(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, raw, wantError string
		want                 *PublishConfigParameters
	}{
		{name: "absent"},
		{name: "null", raw: "null"},
		{name: "array", raw: "[]", wantError: "publishConfig must be an object"},
		{name: "scalar", raw: "true", wantError: "publishConfig must be an object"},
		{name: "malformed", raw: "{", wantError: "publishConfig must be an object"},
		{name: "directory even null", raw: `{"directory":null}`, wantError: "publishConfig.directory is unsupported"},
		{name: "registry type", raw: `{"registry":1}`, wantError: "publishConfig.registry must be a string"},
		{name: "tag type", raw: `{"tag":[]}`, wantError: "publishConfig.tag must be a string"},
		{name: "access type", raw: `{"access":false}`, wantError: "publishConfig.access must be a string"},
		{name: "provenance type", raw: `{"provenance":"true"}`, wantError: "publishConfig.provenance must be boolean"},
		{name: "empty object", raw: `{}`, want: &PublishConfigParameters{}},
		{name: "unknown ignored", raw: `{"unknown":{"nested":[1]}}`, want: &PublishConfigParameters{}},
		{name: "registry", raw: `{"registry":"https://registry.npmjs.org/"}`, want: &PublishConfigParameters{Registry: "https://registry.npmjs.org/"}},
		{name: "tag", raw: `{"tag":"next"}`, want: &PublishConfigParameters{Tag: "next"}},
		{name: "access", raw: `{"access":"public"}`, want: &PublishConfigParameters{Access: "public"}},
		{name: "false preserved", raw: `{"provenance":false}`, want: &PublishConfigParameters{Provenance: boolPointer(false)}},
		{name: "all fields", raw: `{"registry":"https://registry.npmjs.org/","tag":"next","access":"public","provenance":true,"ignored":42}`, want: &PublishConfigParameters{Registry: "https://registry.npmjs.org/", Tag: "next", Access: "public", Provenance: boolPointer(true)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodePublishConfig(jsonRaw(test.raw))
			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError || got != nil {
					t.Fatalf("decodePublishConfig() = %#v, %v; want nil, %q", got, err, test.wantError)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("decodePublishConfig() = %#v, %v; want %#v", got, err, test.want)
			}
		})
	}
}

func TestCloneRawMapIndependence(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		source map[string]json.RawMessage
	}{
		{"nil", nil},
		{"empty", map[string]json.RawMessage{}},
		{"populated", map[string]json.RawMessage{"value": json.RawMessage(`"first"`)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := cloneRawMap(test.source)
			if !reflect.DeepEqual(got, test.source) {
				t.Fatalf("clone = %#v, want %#v", got, test.source)
			}
			if test.source == nil {
				return
			}
			if raw, present := test.source["value"]; present {
				raw[1] = 'X'
				if string(got["value"]) != `"first"` {
					t.Fatal("clone aliases source byte slice")
				}
				delete(test.source, "value")
				if _, present := got["value"]; !present {
					t.Fatal("clone aliases source map")
				}
			}
			test.source["added"] = json.RawMessage("true")
			if _, present := got["added"]; present {
				t.Fatal("source insertion changed clone")
			}
		})
	}
}

func TestWorkflowPathHelpers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, directory, root, manifest string
		workspace                       *string
	}{
		{"root", ".", ".", "package.json", nil},
		{"standalone nested", "nested", "nested", "nested/package.json", nil},
		{"workspace", "packages/app", ".", "packages/app/package.json", testStringPointer(".")},
		{"trailing slash", "packages/app/", ".", "packages/app/package.json", testStringPointer(".")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pkg := Package{Directory: test.directory, ManagerRoot: test.root}
			if got := workspaceRoot(pkg); !reflect.DeepEqual(got, test.workspace) {
				t.Fatalf("workspaceRoot() = %v, want %v", got, test.workspace)
			}
			if got := packageManifestPath(test.directory); got != test.manifest {
				t.Fatalf("packageManifestPath() = %q, want %q", got, test.manifest)
			}
		})
	}
	if got := nullableString(""); got != nil {
		t.Fatalf("nullableString(empty) = %#v, want nil", got)
	}
	if got := nullableString("package.json"); got != "package.json" {
		t.Fatalf("nullableString(nonempty) = %#v", got)
	}
}
