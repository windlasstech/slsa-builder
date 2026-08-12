package npmprofile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveSourceRef(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/example/project/git/ref/tags/v1.2.3" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		if _, err := response.Write([]byte(`{"object":{"type":"commit","sha":"` + testSourceSHA + `"}}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	resolved, err := ResolveSourceRef(context.Background(), SourceRefResolutionConfig{
		HTTPClient: server.Client(), APIURL: server.URL, Repository: "example/project",
		SourceRef: "refs/tags/v1.2.3", EventRef: "refs/heads/main", EventRevision: testAttestSHA,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Ref != "refs/tags/v1.2.3" || resolved.Revision != testSourceSHA || resolved.RefType != "tag" {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
}

func TestResolveSourceRefPreservesUnsetEventSource(t *testing.T) {
	resolved, err := ResolveSourceRef(context.Background(), SourceRefResolutionConfig{
		Repository: "example/project", EventRef: "refs/tags/v1.2.3", EventRevision: testSourceSHA,
		EventRefType: "tag",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Ref != "refs/tags/v1.2.3" || resolved.Revision != testSourceSHA || resolved.RefType != "tag" {
		t.Fatalf("unexpected event resolution: %#v", resolved)
	}
}

func TestResolveSourceRefRejectsInvalidSelection(t *testing.T) {
	tests := map[string]SourceRefResolutionConfig{
		"non-tag":      {Repository: "example/project", SourceRef: "refs/heads/main"},
		"unresolvable": {HTTPClient: http.DefaultClient, APIURL: "http://127.0.0.1:1", Repository: "example/project", SourceRef: "refs/tags/v1.2.3"},
	}
	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ResolveSourceRef(context.Background(), config); err == nil {
				t.Fatal("ResolveSourceRef() succeeded, want rejection")
			}
		})
	}
}
