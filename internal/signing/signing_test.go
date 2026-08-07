package signing

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestPrepareDSSEPreservesExactStatementBytes(t *testing.T) {
	statement, err := (provenance.Statement{
		Type:          provenance.StatementType,
		Subject:       []provenance.Subject{},
		PredicateType: provenance.PredicateType,
		Predicate: provenance.Predicate{
			BuildDefinition: provenance.BuildDefinition{ExternalParameters: []byte(`{}`), InternalParameters: []byte(`{}`), ResolvedDependencies: []provenance.ResourceDescriptor{}},
			RunDetails:      provenance.RunDetails{Builder: provenance.Builder{Version: map[string]string{}, BuilderDependencies: []provenance.BuilderDependency{}}},
		},
	}).CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := string(statement)
	content, err := PrepareDSSE(statement)
	if err != nil {
		t.Fatal(err)
	}
	if content.PayloadType != bundle.IntotoMediaType {
		t.Fatalf("payload type = %q", content.PayloadType)
	}
	statement[0] = '['
	if string(content.Data) != want {
		t.Fatalf("DSSE payload was not defensively preserved: %s", content.Data)
	}
}

func TestSignGitHubActionsOnline(t *testing.T) {
	if os.Getenv("WINDLASS_TEST_ONLINE") != "1" {
		t.Skip("set WINDLASS_TEST_ONLINE=1 in GitHub Actions to exercise keyless signing")
	}
	repository := requiredEnvironment(t, "GITHUB_REPOSITORY")
	serverURL := requiredEnvironment(t, "GITHUB_SERVER_URL")
	runID := requiredEnvironment(t, "GITHUB_RUN_ID")
	runAttempt := requiredEnvironment(t, "GITHUB_RUN_ATTEMPT")
	workflowRef := requiredEnvironment(t, "GITHUB_WORKFLOW_REF")
	workflowSHA := requiredEnvironment(t, "GITHUB_WORKFLOW_SHA")
	sourceSHA := requiredEnvironment(t, "GITHUB_SHA")
	sourceRef := requiredEnvironment(t, "GITHUB_REF")
	statement, err := (provenance.Statement{
		Type:          provenance.StatementType,
		Subject:       []provenance.Subject{{Name: "pkg:generic/windlass-online-test", Digest: map[string]string{"sha256": strings.Repeat("0", 64)}}},
		PredicateType: provenance.PredicateType,
		Predicate: provenance.Predicate{
			BuildDefinition: provenance.BuildDefinition{BuildType: "https://buildtype.dev/windlass/test/v1", ExternalParameters: []byte(`{}`), InternalParameters: []byte(`{}`), ResolvedDependencies: []provenance.ResourceDescriptor{}},
			RunDetails: provenance.RunDetails{
				Builder:  provenance.Builder{ID: serverURL + "/" + workflowRef, Version: map[string]string{}, BuilderDependencies: []provenance.BuilderDependency{}},
				Metadata: provenance.Metadata{InvocationID: serverURL + "/" + repository + "/actions/runs/" + runID + "/attempts/" + runAttempt, StartedOn: "2026-08-07T00:00:00Z", FinishedOn: "2026-08-07T00:00:00Z"},
			},
		},
	}).CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	identity := attestation.IdentityExpectation{
		Issuer:                  "https://token.actions.githubusercontent.com",
		SignerURI:               serverURL + "/" + workflowRef,
		WorkflowSHA:             workflowSHA,
		SourceRepositoryURI:     serverURL + "/" + repository,
		SourceRepositoryID:      requiredEnvironment(t, "GITHUB_REPOSITORY_ID"),
		SourceRepositoryOwnerID: requiredEnvironment(t, "GITHUB_REPOSITORY_OWNER_ID"),
		SourceDigest:            sourceSHA,
		SourceRef:               sourceRef,
		RunnerEnvironment:       "github-hosted",
		RunInvocationURI:        serverURL + "/" + repository + "/actions/runs/" + runID + "/attempts/" + runAttempt,
	}
	result, err := SignGitHubActions(t.Context(), Request{Statement: statement, Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bundle) == 0 || string(result.Statement) != string(statement) {
		t.Fatal("online signer did not preserve its output bytes")
	}
	if outputPath := os.Getenv("WINDLASS_ONLINE_BUNDLE_PATH"); outputPath != "" {
		if err := os.WriteFile(outputPath, result.Bundle, 0o600); err != nil {
			t.Fatalf("write online signing bundle: %v", err)
		}
	}
}

func requiredEnvironment(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required for online signing", name)
	}
	return value
}

func TestSignRejectsInvalidStatementBeforeNetwork(t *testing.T) {
	original := http.DefaultTransport
	transport := &denyTransport{}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })

	_, err := SignGitHubActions(context.Background(), Request{Statement: []byte(`{"not":"a statement"}`)})
	if err == nil {
		t.Fatal("SignGitHubActions() succeeded, want invalid Statement rejection")
	}
	if calls := transport.calls.Load(); calls != 0 {
		t.Fatalf("invalid signing input made %d network calls, want zero", calls)
	}
}

type denyTransport struct{ calls atomic.Int64 }

func (transport *denyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	return nil, errors.New("network denied")
}
