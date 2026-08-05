package handoff_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
)

type handoffFixture struct {
	ArtifactName    string            `json:"artifact_name"`
	PayloadFileName string            `json:"payload_file_name"`
	PayloadKind     string            `json:"payload_kind"`
	SHA256          string            `json:"sha256"`
	Files           map[string]string `json:"files"`
	ExpectedResult  string            `json:"expected_result"`
}

func TestVerifyOneFileHandoff(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		wantErr bool
	}{
		{name: "pass", fixture: "valid.json"},
		{name: "reject zero files", fixture: "zero-files.json", wantErr: true},
		{name: "reject multiple files", fixture: "multiple-files.json", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := loadFixture(t, tt.fixture)
			directory := materialize(t, fixture.Files)
			contract := contractFromFixture(t, fixture)

			verifiedPath, err := handoff.VerifyOneFile(directory, contract)
			if tt.wantErr {
				if err == nil {
					t.Fatal("VerifyOneFile unexpectedly succeeded")
				}
				return
			}
			if err != nil {
				t.Fatalf("VerifyOneFile: %v", err)
			}
			if fixture.ExpectedResult != "pass" {
				t.Fatalf("fixture expected_result = %q, want pass", fixture.ExpectedResult)
			}
			if filepath.Base(verifiedPath) != fixture.PayloadFileName {
				t.Fatalf("verified payload = %q, want basename %q", verifiedPath, fixture.PayloadFileName)
			}

			encoded, err := json.Marshal(contract)
			if err != nil {
				t.Fatalf("marshal handoff: %v", err)
			}
			want := `{"transport":"github-actions-artifact","artifact_name":"profile-payload-123456789-1","payload_file_name":"artifact.tar.gz","payload_kind":"primary-artifact","digest":{"algorithm":"sha256","value":"8458d7a633b4cb9f781d4afbb11abc6be59f135c4b7798f7b9421e776683350e"}}`
			if string(encoded) != want {
				t.Fatalf("handoff JSON = %s, want %s", encoded, want)
			}
		})
	}
}

func TestRejectTraversal(t *testing.T) {
	fixtureBytes, err := os.ReadFile("../../testdata/handoff/traversal.json")
	if err != nil {
		t.Fatalf("read traversal fixture: %v", err)
	}

	var names []string
	if err := json.Unmarshal(fixtureBytes, &names); err != nil {
		t.Fatalf("decode traversal fixture: %v", err)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if err := handoff.ValidateSafeBasename(name); err == nil {
				t.Fatalf("ValidateSafeBasename(%q) unexpectedly succeeded", name)
			}
		})
	}
}

func TestDigestMismatch(t *testing.T) {
	fixture := loadFixture(t, "digest-mismatch.json")
	directory := materialize(t, fixture.Files)

	_, err := handoff.VerifyOneFile(directory, contractFromFixture(t, fixture))
	if err == nil {
		t.Fatal("VerifyOneFile unexpectedly succeeded")
	}
	if !errors.Is(err, handoff.ErrDigestMismatch) {
		t.Fatalf("error %v does not match ErrDigestMismatch", err)
	}
	if got := handoff.ErrorIDOf(err); got != handoff.DigestMismatchID {
		t.Fatalf("primary ID = %q, want %q", got, handoff.DigestMismatchID)
	}
}

func loadFixture(t *testing.T, name string) handoffFixture {
	t.Helper()

	fixtureBytes, err := os.ReadFile(filepath.Join("../../testdata/handoff", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var fixture handoffFixture
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return fixture
}

func materialize(t *testing.T, files map[string]string) string {
	t.Helper()

	directory := t.TempDir()
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600); err != nil {
			t.Fatalf("write fixture file %q: %v", name, err)
		}
	}
	return directory
}

func contractFromFixture(t *testing.T, fixture handoffFixture) handoff.Handoff {
	t.Helper()

	expected, err := digest.ParseSHA256(fixture.SHA256)
	if err != nil {
		t.Fatalf("parse fixture SHA-256: %v", err)
	}
	return handoff.Handoff{
		Transport:       handoff.TransportGitHubActionsArtifact,
		ArtifactName:    fixture.ArtifactName,
		PayloadFileName: fixture.PayloadFileName,
		PayloadKind:     fixture.PayloadKind,
		Digest: handoff.Digest{
			Algorithm: handoff.AlgorithmSHA256,
			Value:     expected,
		},
	}
}
