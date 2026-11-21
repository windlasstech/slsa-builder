package dependabot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGitHubModuleBudgetAndCancellation(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"request limit", "byte limit", "cancelled", "invalid path", "invalid origin"} {
		t.Run(scenario, func(t *testing.T) {
			var calls atomic.Int64
			api := testAPI(t, func(w http.ResponseWriter, r *http.Request) { _ = r; calls.Add(1); fmt.Fprint(w, `{}`) })
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			path, want, wantCalls := "/test", ErrInput, int64(0)
			switch scenario {
			case "request limit":
				api.budget = &moduleRequestBudget{requests: maxModuleFiles + 2}
			case "byte limit":
				api.budget = &moduleRequestBudget{bytes: maxModuleResponseBytes - 1}
				wantCalls = 1
			case "cancelled":
				cancel()
				want = ErrGitHub
			case "invalid path":
				path = "//other"
			case "invalid origin":
				api.origin = ":"
				want = ErrGitHub
			}
			_, err := api.request(ctx, path, nil)
			if !errors.Is(err, want) || calls.Load() != wantCalls {
				t.Fatalf("err=%v calls=%d", err, calls.Load())
			}
		})
	}
}

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
	var calls atomic.Int64
	api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/other", http.StatusFound)
	})
	_, err := getJSON[struct{}](t.Context(), api, "/test")
	if !errors.Is(err, ErrGitHub) || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}
