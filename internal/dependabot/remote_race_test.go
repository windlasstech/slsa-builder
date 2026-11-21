package dependabot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRemoteApplyRevalidatesBeforeMutation(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	for _, scenario := range []string{"stale-head", "closed", "base-moved", "author-changed", "fork", "paths-changed", "plan-changed", "preimage-changed"} {
		t.Run(scenario, func(t *testing.T) {
			f := newRemoteFixture(t)
			var generated atomic.Int64
			f.remote.planner.Generate = func(context.Context, Generation) ([]Change, error) {
				count := generated.Add(1)
				after := "new\n"
				if scenario == "plan-changed" && count > 1 {
					after = "changed\n"
				}
				return []Change{{Path: "internal/provenance/builder_dependencies.go", Before: []byte("old\n"), After: []byte(after)}}, nil
			}
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				if generated.Load() < 2 {
					return false
				}
				if scenario == "preimage-changed" && strings.Contains(r.URL.Path, "/contents/internal/") {
					fmt.Fprint(w, contentJSON(t, "internal/provenance/builder_dependencies.go", []byte("different")))
					return true
				}
				if scenario == "paths-changed" && strings.HasSuffix(r.URL.Path, "/files") {
					fmt.Fprint(w, `[{"filename":"evil.go","status":"modified"}]`)
					return true
				}
				if r.URL.Path != repositoryAPI+"/pulls/123" {
					return false
				}
				pr, err := decodeGitHub[githubPR]([]byte(f.responses[r.URL.Path]))
				if err != nil {
					t.Error(err)
					return false
				}
				switch scenario {
				case "stale-head":
					pr.Head.SHA = strings.Repeat("e", 40)
				case "closed":
					pr.State = "closed"
				case "base-moved":
					pr.Base.SHA = strings.Repeat("e", 40)
				case "author-changed":
					pr.User.Login = "human"
				case "fork":
					pr.Head.Repo.FullName = "other/repository"
				default:
					return false
				}
				fmt.Fprint(w, fixtureJSON(t, pr))
				return true
			}
			_, err := f.remote.Apply(t.Context(), f.inputs, "companion")
			want := ErrInput.Error()
			switch scenario {
			case "closed":
				want += ": pull-request-not-open"
			case "base-moved":
				want += ": wrong-trusted-base"
			case "fork":
				want += ": fork-pull-request"
			case "plan-changed":
				want += ": regenerated plan differs"
			case "preimage-changed":
				want += ": remote preimage differs"
			}
			if !errors.Is(err, ErrInput) || err.Error() != want || generated.Load() != 2 || f.mutations.Load() != 0 {
				t.Fatalf("err=%v mutations=%d", err, f.mutations.Load())
			}
		})
	}
}

func TestGitHubPaginationIsCompleteAndBounded(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"later-page", "overflow"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if scenario == "overflow" {
					fmt.Fprint(w, "["+strings.Repeat(`{"number":1},`, 99)+`{"number":1}]`)
					return
				}
				switch r.URL.Query().Get("page") {
				case "1":
					fmt.Fprint(w, `[{"number":1}]`)
				case "2":
					fmt.Fprint(w, `[{"number":2}]`)
				default:
					fmt.Fprint(w, `[]`)
				}
			})
			items, err := listGitHub[githubPR](t.Context(), api, "/pulls?state=open")
			if scenario == "overflow" {
				if err == nil || calls.Load() != 3 {
					t.Fatalf("err=%v calls=%d", err, calls.Load())
				}
			} else if err != nil || len(items) != 2 || calls.Load() != 3 {
				t.Fatalf("items=%v err=%v calls=%d", items, err, calls.Load())
			}
		})
	}
}

func TestGitHubProductionTransport(t *testing.T) {
	t.Parallel()
	api, err := newGitHub("test-token")
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := api.client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.TLSClientConfig != nil || transport.DialTLSContext != nil || api.origin != "https://api.github.com" || api.client.Timeout <= 0 || api.client.CheckRedirect == nil {
		t.Fatal("unsafe production transport")
	}
	if err := api.client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, ErrGitHub) {
		t.Fatalf("redirect callback=%v", err)
	}
}
