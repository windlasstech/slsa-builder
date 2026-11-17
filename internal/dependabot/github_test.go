package dependabot

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Internal tests inject only the httptest transport; production has no injection API.
func testAPI(t *testing.T, handler http.HandlerFunc) *githubAPI {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := server.Client()
	client.CheckRedirect = refuseRedirect
	return &githubAPI{client: client, origin: server.URL, token: githubToken("test-token")}
}

func TestGitHubReadRejectsUnsafeResponses(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"id":1,"id":2}`, `null`, `{`, strings.Repeat(" ", maxInput+1)} {
		t.Run(fmt.Sprintf("bytes-%d", len(body)), func(t *testing.T) {
			t.Parallel()
			// Given an authenticated but malformed upstream response.
			api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("unexpected request")
				}
				fmt.Fprint(w, body)
			})
			// When decoding at the API boundary.
			_, err := getJSON[struct{ ID int }](t.Context(), api, "/test")
			// Then no malformed input crosses the boundary.
			if err == nil {
				t.Fatal("accepted malformed response")
			}
		})
	}
}

func TestGitHubRefusesRedirect(t *testing.T) {
	t.Parallel()
	calls := 0
	api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "/other", http.StatusFound)
	})
	_, err := getJSON[struct{}](t.Context(), api, "/test")
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
