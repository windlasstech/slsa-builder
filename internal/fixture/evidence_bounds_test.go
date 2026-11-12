package fixture

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestFixtureEvidenceDecodedBound(t *testing.T) {
	t.Parallel()
	for _, extra := range []int{0, 1} {
		content := []byte(`{}` + strings.Repeat(" ", maxManifestEvidenceBytes-2+extra))
		statementBytes, _ := evidenceFixtureBytes(t)
		statement, err := provenance.DecodeStatement(statementBytes)
		if err != nil {
			t.Fatal(err)
		}
		statement.Predicate.BuildDefinition.ResolvedDependencies[1].Digest["sha256"] = digest.SumSHA256(content).String()
		statementBytes, err = statement.CanonicalJSON()
		if err != nil {
			t.Fatal(err)
		}
		evidence, err := json.Marshal(map[string]string{"path": "packages/a/package.json", "content_base64": base64.StdEncoding.EncodeToString(content)})
		if err != nil {
			t.Fatal(err)
		}
		manifest := validAcceptedManifest()
		manifest.ManifestEvidence = stringPointer("testdata/npm/evidence.json")
		err = ValidateManifestEvidence(manifest, statementBytes, evidence)
		if (err != nil) != (extra > 0) {
			t.Fatalf("decoded length %d: %v", len(content), err)
		}
	}
}

func TestFixtureEvidenceCanonicalPathEncoding(t *testing.T) {
	t.Parallel()
	statement, evidence := evidenceFixtureBytes(t)
	statement = []byte(strings.Replace(string(statement), `"directory":"packages/a"`, `"directory":"packages/a b#%"`, 1))
	statement = []byte(strings.Replace(string(statement), "#packages/a/package.json", "#packages/a%20b%23%25/package.json", 1))
	evidence = []byte(strings.Replace(string(evidence), "packages/a/package.json", "packages/a b#%/package.json", 1))
	manifest := validAcceptedManifest()
	manifest.ManifestEvidence = stringPointer("testdata/npm/evidence.json")
	if err := ValidateManifestEvidence(manifest, statement, evidence); err != nil {
		t.Fatal(err)
	}
	statement = []byte(strings.Replace(string(statement), "#packages/a%20b%23%25/package.json", "#packages/a b#%/package.json", 1))
	if err := ValidateManifestEvidence(manifest, statement, evidence); err == nil {
		t.Fatal("unescaped manifest URI was accepted")
	}
}
