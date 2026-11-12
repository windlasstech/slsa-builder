package fixture

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestFixtureManifestEvidenceMember(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(validAcceptedManifest())
	if err != nil {
		t.Fatal(err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		t.Fatal(err)
	}
	for _, present := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "explicit null"}[present], func(t *testing.T) {
			t.Parallel()
			copy := make(map[string]json.RawMessage, len(members)+1)
			for key, value := range members {
				copy[key] = value
			}
			delete(copy, "manifest-evidence")
			if present {
				copy["manifest-evidence"] = json.RawMessage(`null`)
			}
			raw, err := json.Marshal(copy)
			if err != nil {
				t.Fatal(err)
			}
			_, err = decodeManifest(raw)
			if (err == nil) != present {
				t.Fatalf("present=%v error=%v", present, err)
			}
		})
	}
}

func TestFixtureManifestEvidenceBinding(t *testing.T) {
	t.Parallel()
	statement, evidence := evidenceFixtureBytes(t)
	type input struct {
		manifest            Manifest
		statement, evidence []byte
	}
	for _, test := range []struct {
		name   string
		mutate func(*input)
		valid  bool
	}{
		{"bound snapshot", func(*input) {}, true},
		{"missing path", func(i *input) { i.manifest.ManifestEvidence = nil }, false},
		{"missing bytes", func(i *input) { i.evidence = nil }, false},
		{"extra member", func(i *input) {
			i.evidence = []byte(strings.TrimSuffix(string(i.evidence), "}") + `,"extra":true}`)
		}, false},
		{"duplicate member", func(i *input) {
			i.evidence = []byte(strings.TrimSuffix(string(i.evidence), "}") + `,"path":"other.json"}`)
		}, false},
		{"wrong path", func(i *input) {
			i.evidence = []byte(strings.Replace(string(i.evidence), "packages/a/package.json", "other/package.json", 1))
		}, false},
		{"wrong digest", func(i *input) {
			i.statement = []byte(strings.Replace(string(i.statement), digest.SumSHA256([]byte(`{"name":"a","version":"1.0.0"}`)).String(), strings.Repeat("0", 64), 1))
		}, false},
		{"wrong commit", func(i *input) {
			i.statement = []byte(strings.Replace(string(i.statement), "@"+strings.Repeat("a", 40)+"#", "@"+strings.Repeat("b", 40)+"#", 1))
		}, false},
		{"noncanonical base64", func(i *input) {
			i.evidence = []byte(`{"path":"packages/a/package.json","content_base64":"e31="}`)
		}, false},
		{"static metadata", func(i *input) {
			i.manifest.ManifestEvidence = nil
			i.statement = []byte(`{"phase":"build-pack"}`)
			i.evidence = nil
		}, true},
		{"static with evidence", func(i *input) { i.statement = []byte(`{"phase":"build-pack"}`) }, false},
		{"release manifest", func(i *input) {
			i.manifest.Surface = "release-manifest"
			i.manifest.ManifestEvidence = nil
			i.statement = []byte(`{"schema_version":"1"}`)
			i.evidence = nil
		}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			manifest := validAcceptedManifest()
			manifest.ManifestEvidence = stringPointer("testdata/npm/evidence.json")
			value := input{manifest: manifest, statement: append([]byte(nil), statement...), evidence: append([]byte(nil), evidence...)}
			test.mutate(&value)
			err := ValidateManifestEvidence(value.manifest, value.statement, value.evidence)
			if test.valid {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var identified interface{ DiagnosticID() string }
			if !errors.As(err, &identified) || identified.DiagnosticID() != "windlass.verify.error.package-manifest-evidence" {
				t.Fatalf("evidence error = %v", err)
			}
		})
	}
}

func evidenceFixtureBytes(t testing.TB) ([]byte, []byte) {
	t.Helper()
	content := []byte(`{"name":"a","version":"1.0.0"}`)
	statement := provenance.Statement{Type: provenance.StatementType, PredicateType: provenance.PredicateType, Predicate: provenance.Predicate{BuildDefinition: provenance.BuildDefinition{
		BuildType:          "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1",
		ExternalParameters: json.RawMessage(`{"workflow":{"repository":"https://github.com/example/project"},"package":{"directory":"packages/a"}}`),
		ResolvedDependencies: []provenance.ResourceDescriptor{
			{Name: "built-source", Digest: map[string]string{"gitCommit": strings.Repeat("a", 40)}},
			{Name: "package-manifest", URI: "git+https://github.com/example/project@" + strings.Repeat("a", 40) + "#packages/a/package.json", MediaType: "application/json", Digest: map[string]string{"sha256": digest.SumSHA256(content).String()}},
		},
	}}}
	s, err := statement.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	e, err := json.Marshal(struct {
		Path    string `json:"path"`
		Content string `json:"content_base64"`
	}{"packages/a/package.json", base64.StdEncoding.EncodeToString(content)})
	if err != nil {
		t.Fatal(err)
	}
	return s, e
}
