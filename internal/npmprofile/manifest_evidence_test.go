package npmprofile_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestPackageManifestEvidence(t *testing.T) {
	t.Parallel()
	content := []byte(`{"name":"example","version":"1.2.3"}`)
	evidence := npmprofile.ManifestEvidence{Path: "package.json", ContentBase64: base64.StdEncoding.EncodeToString(content)}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	var decoded npmprofile.ManifestEvidence
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	got, err := decoded.Decode(".")
	if err != nil || string(got.Content) != string(content) || got.Path != "package.json" {
		t.Fatalf("snapshot = %+v, error = %v", got, err)
	}
	if digest.SumSHA256(got.Content) != digest.SumSHA256(content) {
		t.Fatal("evidence round trip changed original bytes")
	}
}

func TestPackageManifestEvidenceRejectsMalformedEnvelope(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`null`, `[]`, `{}`, `{"path":"package.json"}`,
		`{"path":null,"content_base64":"e30="}`,
		`{"path":"package.json","content_base64":null}`,
		`{"path":"package.json","content_base64":"e30=","extra":true}`,
		`{"path":"package.json","path":"other.json","content_base64":"e30="}`,
		`{"Path":"package.json","content_base64":"e30="}`,
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			var evidence npmprofile.ManifestEvidence
			if err := json.Unmarshal([]byte(input), &evidence); err == nil {
				t.Fatal("accepted malformed evidence envelope")
			}
		})
	}
}

func TestPackageManifestEvidenceRejectsNoncanonicalContent(t *testing.T) {
	t.Parallel()
	for _, content := range []string{"", "e30", "e30=\n", "e30=\r", "e31=", "e30==", "____", "bnVsbA==", "W10=", base64.StdEncoding.EncodeToString([]byte(`{"name":"a","name":"b"}`))} {
		t.Run(content, func(t *testing.T) {
			t.Parallel()
			evidence := npmprofile.ManifestEvidence{Path: "package.json", ContentBase64: content}
			if _, err := evidence.Decode("."); err == nil {
				t.Fatal("accepted malformed or noncanonical manifest bytes")
			}
		})
	}
}

func TestPackageManifestEvidenceSizeBound(t *testing.T) {
	t.Parallel()
	for _, extra := range []int{0, 1, 3} {
		content := `{}` + strings.Repeat(" ", (16<<20)-2+extra)
		evidence := npmprofile.ManifestEvidence{Path: "package.json", ContentBase64: base64.StdEncoding.EncodeToString([]byte(content))}
		_, err := evidence.Decode(".")
		if (err != nil) != (extra > 0) {
			t.Fatalf("decoded size %d: error = %v", len(content), err)
		}
	}
}

func TestPackageManifestEvidencePath(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"", "/package.json", "../package.json", "packages/../package.json", `packages\widget\package.json`, "other/package.json"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			evidence := npmprofile.ManifestEvidence{Path: path, ContentBase64: "e30="}
			if _, err := evidence.Decode("packages/widget"); err == nil {
				t.Fatal("accepted evidence outside selected manifest path")
			}
		})
	}
}
