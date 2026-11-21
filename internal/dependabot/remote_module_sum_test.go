package dependabot

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRemoteModuleAuthentication(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	for _, scenario := range []string{"forged sum", "commit identity", "tree identity", "truncated", "missing completeness", "missing root", "blob identity", "blob bytes", "blob size", "blob encoding", "missing blob content", "root unavailable", "malformed tree"} {
		t.Run(scenario, func(t *testing.T) {
			f := newRemoteFixture(t)
			var generated atomic.Int64
			f.remote.planner.Generate = func(context.Context, Generation) ([]Change, error) {
				generated.Add(1)
				return nil, ErrInput
			}
			want := ErrInput
			if scenario == "root unavailable" {
				want = ErrGitHub
			}
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				p := r.URL.Path
				switch {
				case scenario == "forged sum" && p == repositoryAPI+"/contents/go.sum" && r.URL.Query().Get("ref") == f.event.Run.HeadSHA:
					content, err := decodeGitHub[githubContent]([]byte(f.responses[p+"?ref="+f.event.Run.HeadSHA]))
					if err != nil {
						t.Error(err)
						return true
					}
					data, err := content.bytes("go.sum")
					if err != nil {
						t.Error(err)
						return true
					}
					lines := strings.Split(string(data), "\n")
					for i, line := range lines {
						if strings.HasPrefix(line, signerModule+" v1.999.0 ") {
							lines[i] = signerModule + " v1.999.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
						}
					}
					fmt.Fprint(w, contentJSON(t, "go.sum", []byte(strings.Join(lines, "\n"))))
				case scenario == "commit identity" && strings.Contains(p, "/git/commits/"):
					fmt.Fprintf(w, `{"sha":%q,"tree":{"sha":%q}}`, strings.Repeat("e", 40), strings.Repeat("8", 40))
				case strings.Contains(p, "/git/trees/") && (scenario == "tree identity" || scenario == "truncated" || scenario == "missing completeness" || scenario == "missing root" || scenario == "malformed tree"):
					tree, err := decodeGitHub[moduleTree]([]byte(f.responses[p]))
					if err != nil {
						t.Error(err)
						return true
					}
					switch scenario {
					case "tree identity":
						tree.SHA = strings.Repeat("e", 40)
					case "truncated":
						*tree.Truncated = true
					case "missing completeness":
						tree.Truncated = nil
					case "missing root":
						for i := range tree.Tree {
							if tree.Tree[i].Path == "go.mod" {
								tree.Tree[i].Path = "other.mod"
							}
						}
					case "malformed tree":
						fmt.Fprint(w, `{`)
						return true
					}
					fmt.Fprint(w, fixtureJSON(t, tree))
				case strings.Contains(p, "/git/blobs/") && strings.HasPrefix(scenario, "blob "), strings.Contains(p, "/git/blobs/") && scenario == "missing blob content":
					blob, err := decodeGitHub[map[string]any]([]byte(f.responses[p]))
					if err != nil {
						t.Error(err)
						return true
					}
					switch scenario {
					case "blob identity":
						blob["sha"] = strings.Repeat("e", 40)
					case "blob bytes":
						encoded, ok := blob["content"].(string)
						if !ok {
							t.Fatal("root blob fixture content is missing or not a string")
						}
						data, err := base64.StdEncoding.DecodeString(encoded)
						if err != nil || len(data) == 0 {
							t.Error("invalid root fixture")
							return true
						}
						data[0] ^= 1
						blob["content"] = base64.StdEncoding.EncodeToString(data)
					case "blob size":
						blob["size"] = 0
					case "blob encoding":
						blob["encoding"] = "utf8"
					case "missing blob content":
						delete(blob, "content")
					}
					fmt.Fprint(w, fixtureJSON(t, blob))
				case scenario == "root unavailable" && strings.Contains(p, "/git/blobs/"):
					w.WriteHeader(503)
				default:
					return false
				}
				return true
			}
			_, err := f.remote.Apply(t.Context(), f.inputs, "companion")
			if !errors.Is(err, want) || generated.Load() != 0 || f.mutations.Load() != 0 {
				t.Fatalf("err=%v generated=%d mutations=%d", err, generated.Load(), f.mutations.Load())
			}
			if _, err := os.Stat(f.inputs.Output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("published output: %v", err)
			}
		})
	}
}

func TestModuleSelectionUsesAuthenticatedMemory(t *testing.T) {
	t.Parallel()
	responses := map[string]string{}
	sum := remoteModuleFixture(t, responses)
	var calls atomic.Int64
	api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("unauthenticated request")
		}
		if strings.Contains(r.URL.Path, "/git/trees/") && r.URL.Query().Get("recursive") != "1" {
			t.Error("incomplete enumeration request")
		}
		body, ok := responses[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(404)
			return
		}
		fmt.Fprint(w, body)
	})
	release := resolvedRelease{metadata: releaseMetadata{Tag: "v1.999.0"}, commit: strings.Repeat("c", 40)}
	manifest, err := api.verifyCandidateModule(t.Context(), release, sum)
	if err != nil || !bytes.Equal(manifest, []byte("module github.com/sigstore/sigstore-go\n\ngo 1.27.0\n")) || calls.Load() != 7 {
		t.Fatalf("err=%v manifest=%q calls=%d", err, manifest, calls.Load())
	}
}

func TestModuleVersionRejectedBeforeRequests(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"v1.2", "v1.2.3+other", "v2.0.0", "invalid"} {
		t.Run(v, func(t *testing.T) {
			api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
				_ = r
				t.Error("invalid version reached API")
				w.WriteHeader(500)
			})
			_, err := api.verifyCandidateModule(t.Context(), resolvedRelease{metadata: releaseMetadata{Tag: v}}, "unused")
			if !errors.Is(err, ErrInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
