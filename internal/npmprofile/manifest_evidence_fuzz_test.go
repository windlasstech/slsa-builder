package npmprofile

import (
	"encoding/base64"
	"encoding/json"
	"path"
	"testing"
)

func FuzzManifestEvidence(f *testing.F) {
	for _, seed := range []string{
		`{"path":"package.json","content_base64":"e30="}`,
		`{"path":"packages/a/package.json","content_base64":"e30="}`,
		`{"path":"package.json","content_base64":"e31="}`,
		`{"path":"package.json","content_base64":"e30=\n"}`,
		`{"path":"package.json","path":"other.json","content_base64":"e30="}`,
		`null`, `[]`, `{}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		var evidence ManifestEvidence
		if err := json.Unmarshal(raw, &evidence); err != nil {
			return
		}
		snapshot, err := evidence.Decode(path.Dir(evidence.Path))
		if err != nil {
			requireNPMDiagnostic(t, err, IDPackageManifestEvidence)
			return
		}
		if len(snapshot.Content) > maxManifestSize || base64.StdEncoding.EncodeToString(snapshot.Content) != evidence.ContentBase64 || snapshot.Path != evidence.Path {
			t.Fatal("accepted evidence violates the bounded exact-byte transport")
		}
		encoded, err := json.Marshal(evidence)
		if err != nil {
			t.Fatal(err)
		}
		var repeated ManifestEvidence
		if err := json.Unmarshal(encoded, &repeated); err != nil || repeated != evidence {
			t.Fatalf("evidence round trip changed value: %v", err)
		}
	})
}
