package signing

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestOIDCBindingRejectsDuplicateClaims(t *testing.T) {
	t.Parallel()
	statement, expected, payload := oidcBindingSeed(t)
	duplicate := []byte(strings.TrimSuffix(string(payload), "}") + `,"repository":"example/project"}`)
	_, err := bindOIDCSignerIdentity(statement, expected, "header."+base64.RawURLEncoding.EncodeToString(duplicate)+".signature")
	if err == nil {
		t.Fatal("duplicate OIDC claims were accepted")
	}
}

func TestOIDCBindingRejectsExtraSignerSeparator(t *testing.T) {
	t.Parallel()
	statement, expected, payload := oidcBindingSeed(t)
	mutated := []byte(strings.Replace(string(payload), ".yml@"+expected.WorkflowSHA, ".yml@extra@"+expected.WorkflowSHA, 1))
	_, err := bindOIDCSignerIdentity(statement, expected, "header."+base64.RawURLEncoding.EncodeToString(mutated)+".signature")
	if err == nil {
		t.Fatal("extra signer ref separator was accepted")
	}
}

func FuzzBindOIDCSignerIdentity(f *testing.F) {
	statement, expected, valid := oidcBindingSeed(f)
	f.Add(valid, false)
	f.Add(valid, true)
	f.Add([]byte(strings.Replace(string(valid), ".yml@"+expected.WorkflowSHA, ".yml@extra@"+expected.WorkflowSHA, 1)), false)
	f.Add([]byte(`{}`), false)
	f.Add([]byte(strings.TrimSuffix(string(valid), "}")+`,"repository":"example/project"}`), false)
	f.Add([]byte(strings.Replace(string(valid), `"event_name":"workflow_dispatch"`, `"event_name":"push"`, 1)), false)
	f.Fuzz(func(t *testing.T, payload []byte, missingTrigger bool) {
		trusted := expected
		if missingTrigger {
			trusted.BuildTrigger = ""
		}
		got, err := bindOIDCSignerIdentity(statement, trusted, "header."+base64.RawURLEncoding.EncodeToString(payload)+".signature")
		if err != nil {
			return
		}
		if missingTrigger || canonicaljson.Validate(payload) != nil {
			t.Fatal("ambiguous claims or missing trusted Build Trigger accepted")
		}
		want := expected
		want.SignerURI = "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@" + expected.WorkflowSHA
		want.BuildConfigURI = "https://github.com/example/project/.github/workflows/release.yml@refs/heads/main"
		if got != want {
			t.Fatalf("OIDC binding changed independent trusted identity: %#v", got)
		}
	})
}

func oidcBindingSeed(t testing.TB) ([]byte, attestation.IdentityExpectation, []byte) {
	t.Helper()
	sha := strings.Repeat("a", 40)
	sourceSHA := strings.Repeat("b", 40)
	builder := "windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@" + sha
	expected := attestation.IdentityExpectation{WorkflowSHA: sha, SourceRepositoryURI: "https://github.com/example/project", SourceRepositoryID: "123", SourceRepositoryOwnerID: "456", SourceRef: "refs/heads/main", SourceDigest: sourceSHA, BuildTrigger: "workflow_dispatch"}
	statement, err := (provenance.Statement{Type: provenance.StatementType, PredicateType: provenance.PredicateType, Predicate: provenance.Predicate{
		BuildDefinition: provenance.BuildDefinition{
			ExternalParameters: json.RawMessage(`{"workflow":{"ref":"refs/heads/main","repository":"https://github.com/example/project","path":".github/workflows/release.yml"},"source":{"ref":"refs/tags/v1.2.3"}}`),
			InternalParameters: json.RawMessage(`{"github":{"event_name":"workflow_dispatch","repository_id":"123","repository_owner_id":"456"}}`),
		}, RunDetails: provenance.RunDetails{Builder: provenance.Builder{ID: "https://github.com/" + builder}},
	}}).CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"job_workflow_ref": builder, "job_workflow_sha": sha, "workflow_ref": "example/project/.github/workflows/release.yml@refs/heads/main", "workflow_sha": sourceSHA, "event_name": "workflow_dispatch", "repository": "example/project", "repository_id": "123", "repository_owner_id": "456", "ref": "refs/heads/main", "sha": sourceSHA})
	if err != nil {
		t.Fatal(err)
	}
	return statement, expected, payload
}
