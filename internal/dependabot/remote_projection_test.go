package dependabot

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRemoteProjectionRejectsChangedFilesBetweenReads(t *testing.T) {
	t.Parallel()
	for _, phase := range []int64{2, 3} {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			f := newRemoteFixture(t)
			var reads atomic.Int64
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != repositoryAPI+"/pulls/123" {
					return false
				}
				if reads.Add(1) < phase {
					return false
				}
				pr, err := decodeGitHub[githubPR]([]byte(f.responses[r.URL.Path]))
				if err != nil {
					t.Error(err)
					return true
				}
				pr.ChangedFiles = 1
				fmt.Fprint(w, fixtureJSON(t, pr))
				return true
			}
			_, err := f.remote.Plan(t.Context(), f.inputs)
			if !errors.Is(err, ErrInput) || reads.Load() != phase || f.mutations.Load() != 0 {
				t.Fatalf("err=%v reads=%d mutations=%d", err, reads.Load(), f.mutations.Load())
			}
		})
	}
}

func TestRemoteProjectionContentFailures(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"base mod unavailable", "head sum malformed", "wrong content path", "wrong encoding", "wrong size", "bad base64", "license mismatch", "license empty", "license identifier"} {
		t.Run(scenario, func(t *testing.T) {
			f := newRemoteFixture(t)
			want := ErrInput
			if scenario == "base mod unavailable" {
				want = ErrGitHub
			}
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				p, ref := r.URL.Path, r.URL.Query().Get("ref")
				switch {
				case scenario == "base mod unavailable" && p == repositoryAPI+"/contents/go.mod" && ref == f.inputs.TrustedSHA:
					w.WriteHeader(503)
				case scenario == "head sum malformed" && p == repositoryAPI+"/contents/go.sum" && ref == f.event.Run.HeadSHA:
					fmt.Fprint(w, `{`)
				case strings.HasPrefix(scenario, "wrong content") && p == repositoryAPI+"/contents/go.mod":
					fmt.Fprint(w, contentJSON(t, "other", []byte("module x")))
				case (scenario == "wrong encoding" || scenario == "wrong size" || scenario == "bad base64") && p == repositoryAPI+"/contents/go.mod":
					content, err := decodeGitHub[githubContent]([]byte(f.responses[p+"?ref="+ref]))
					if err != nil {
						t.Error(err)
						return true
					}
					switch scenario {
					case "wrong encoding":
						content.Encoding = "utf8"
					case "wrong size":
						content.Size++
					case "bad base64":
						content.Content = "!"
					}
					fmt.Fprint(w, fixtureJSON(t, content))
				case strings.HasPrefix(scenario, "license") && p == signerRepositoryAPI+"/license" && ref == strings.Repeat("c", 40):
					data := []byte("different")
					if scenario == "license empty" {
						data = nil
					}
					body := contentJSON(t, "LICENSE", data)
					if scenario == "license identifier" {
						body = strings.Replace(body, "Apache-2.0", "MIT", 1)
					}
					fmt.Fprint(w, body)
				default:
					return false
				}
				return true
			}
			_, err := f.remote.Plan(t.Context(), f.inputs)
			if !errors.Is(err, want) || f.posts.Load() != 0 {
				t.Fatalf("err=%v posts=%d", err, f.posts.Load())
			}
		})
	}
}

func TestRemoteInputFailuresDoNotReadAPI(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"snapshot", "missing event", "missing root", "wrong sha", "output overlap"} {
		t.Run(scenario, func(t *testing.T) {
			f := newRemoteFixture(t)
			switch scenario {
			case "snapshot":
				f.inputs.Snapshot = t.TempDir()
			case "missing event":
				f.inputs.EventPath = filepath.Join(t.TempDir(), "missing")
			case "missing root":
				f.inputs.TrustedRoot = filepath.Join(t.TempDir(), "missing")
			case "wrong sha":
				f.inputs.TrustedSHA = strings.Repeat("a", 40)
			case "output overlap":
				f.inputs.Output = f.inputs.TrustedRoot
			}
			_, err := f.remote.Plan(t.Context(), f.inputs)
			if err == nil || f.reads.Load() != 0 {
				t.Fatalf("err=%v reads=%d", err, f.reads.Load())
			}
		})
	}
}

func TestRemoteConstructorAndTokenRedaction(t *testing.T) {
	t.Parallel()
	for _, token := range []string{"", "token space", "token\n", "token\t"} {
		if _, err := NewRemote(token); !errors.Is(err, ErrInput) {
			t.Fatalf("err=%v", err)
		}
	}
	remote, err := NewRemote("secret-token")
	if err != nil {
		t.Fatal(err)
	}
	remote.Close()
	for _, format := range []string{"%s", "%v", "%+v", "%#v"} {
		if got := fmt.Sprintf(format, remote.api.token); got != "[REDACTED]" {
			t.Fatalf("unredacted format %s", format)
		}
	}
}
