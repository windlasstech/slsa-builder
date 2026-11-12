package npmprofile

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func updatedRuntimeFixture(t *testing.T, name string, manager Manager) []byte {
	t.Helper()
	values := orderedProducerDependencies(validDependencies(manager, validExternalParameters(manager)))
	runtimeIndex := slices.IndexFunc(values, func(value provenance.ResourceDescriptor) bool { return value.Name == "node-runtime" })
	switch name {
	case "npm-resolved-dependencies-npm-valid", "npm-resolved-dependencies-pnpm-valid", "npm-resolved-dependencies-yarn-valid":
	case "npm-resolved-dependencies-reordered-valid":
		slices.Reverse(values)
	case "npm-resolved-dependencies-node-runtime-missing":
		values = slices.Delete(values, runtimeIndex, runtimeIndex+1)
	case "npm-resolved-dependencies-node-runtime-duplicate":
		values = append(values, values[runtimeIndex])
	case "npm-resolved-dependencies-node-runtime-wrong-uri":
		values[runtimeIndex].URI += ".wrong"
	case "npm-resolved-dependencies-node-runtime-wrong-digest":
		values[runtimeIndex].Digest["sha256"] = strings.Repeat("0", 64)
	case "npm-resolved-dependencies-node-runtime-malformed-digest":
		values[runtimeIndex].Digest["sha256"] = "invalid"
	case "npm-resolved-dependencies-node-runtime-annotations":
		values[runtimeIndex].Annotations["platform"] = mustJSON("linux-arm64")
	case "npm-resolved-dependencies-unknown-entry":
		values = append(values, provenance.ResourceDescriptor{Name: "npm-cli"})
	case "npm-resolved-dependencies-npm-manager-distribution":
		values = append(values, validDependencies(ManagerPNPM, validExternalParameters(ManagerPNPM))[1])
	case "npm-resolved-dependencies-node-runtime-unknown-member":
	default:
		t.Fatalf("no generator for runtime fixture %q", name)
	}
	raw, err := json.Marshal(struct {
		PackageManager       Manager                         `json:"package_manager"`
		ResolvedDependencies []provenance.ResourceDescriptor `json:"resolved_dependencies"`
	}{manager, values})
	if err != nil {
		t.Fatal(err)
	}
	if name == "npm-resolved-dependencies-node-runtime-unknown-member" {
		raw = bytes.Replace(raw, []byte(`"name":"node-runtime"`), []byte(`"name":"node-runtime","downloadLocation":"https://example.com/runtime.tar.gz"`), 1)
	}
	encoded, err := canonicaljson.Canonicalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
