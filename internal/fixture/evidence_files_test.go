package fixture

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestFixtureEvidenceFiles(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"valid", "missing", "digest mismatch", "expected rejection", "unobserved rejection", "escape symlink"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			index, indexPath, evidencePath := writeEvidenceFixture(t)
			switch mode {
			case "valid":
			case "missing":
				if err := os.Remove(evidencePath); err != nil {
					t.Fatal(err)
				}
			case "digest mismatch", "expected rejection":
				if err := os.WriteFile(evidencePath, []byte(`{"path":"packages/a/package.json","content_base64":"e30="}`), 0o600); err != nil {
					t.Fatal(err)
				}
			case "escape symlink":
				_, content := evidenceFixtureBytes(t)
				outside := filepath.Join(t.TempDir(), "outside.json")
				if err := os.WriteFile(outside, content, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(evidencePath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, evidencePath); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "expected rejection" || mode == "unobserved rejection" {
				index.Fixtures[0].Type = "rejected"
				index.Fixtures[0].ExpectedResult = "fail"
				index.Fixtures[0].ExpectedPrimaryID = stringPointer(IDPackageManifestEvidence)
				index.Fixtures[0].ExpectedFailureCategory = stringPointer("package-manifest-evidence")
			}
			err := CheckEvidence(index, indexPath)
			if mode == "valid" || mode == "expected rejection" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if mode == "unobserved rejection" {
				if !errors.Is(err, ErrInvalidIndex) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			var identified interface{ DiagnosticID() string }
			if !errors.As(err, &identified) || identified.DiagnosticID() != IDPackageManifestEvidence {
				t.Fatalf("evidence file failure = %v", err)
			}
		})
	}
}

func TestFixtureEvidenceIsOffline(t *testing.T) {
	original := http.DefaultTransport
	denied := &fixtureDenyTransport{}
	http.DefaultTransport = denied
	t.Cleanup(func() { http.DefaultTransport = original })
	index, indexPath, _ := writeEvidenceFixture(t)
	if err := CheckEvidence(index, indexPath); err != nil {
		t.Fatal(err)
	}
	if denied.calls.Load() != 0 {
		t.Fatal("fixture evidence validation attempted a network request")
	}
}

type fixtureDenyTransport struct{ calls atomic.Int64 }

func (transport *fixtureDenyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	return nil, errors.New("network forbidden")
}

func writeEvidenceFixture(t *testing.T) (Index, string, string) {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{"testdata/npm", "testdata/fixtures"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	statement, evidence := evidenceFixtureBytes(t)
	manifest := validAcceptedManifest()
	manifest.Provenance = "testdata/npm/statement.json"
	manifest.ManifestEvidence = stringPointer("testdata/npm/evidence.json")
	for name, data := range map[string][]byte{manifest.Provenance: statement, *manifest.ManifestEvidence: evidence} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	index := Index{Fixtures: []Manifest{manifest}}
	encoded, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(root, "testdata", "fixtures", "index.json")
	if err := os.WriteFile(indexPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	return loaded, indexPath, filepath.Join(root, *manifest.ManifestEvidence)
}
