package dependabot

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRemoteRejectsAuthenticatedInvalidManifest(t *testing.T) {
	t.Parallel()
	for _, manifest := range []string{"invalid (", "module example.com/wrong\n\ngo 1.27.0\n", "module " + signerModule + "\n", "module " + signerModule + "\n\ngo 1.999.0\n"} {
		t.Run(manifest, func(t *testing.T) {
			f := newRemoteFixture(t)
			sum := remoteModuleManifestFixture(t, f.responses, manifest)
			transition := Transition{}
			transition.old.Version = "v1.4.0"
			for key := range f.responses {
				if version, ok := strings.CutPrefix(key, signerRepositoryAPI+"/releases/tags/"); ok && version != "v1.999.0" {
					transition.old.Version = version
				}
			}
			transition.candidate.Version, transition.candidate.Sum = "v1.999.0", sum
			transition.base.Mod = []byte("module example.com/trusted\n\ngo 1.27.0\n")
			_, err := f.remote.api.release(t.Context(), transition)
			if !errors.Is(err, ErrInput) || f.posts.Load() != 0 {
				t.Fatalf("err=%v posts=%d", err, f.posts.Load())
			}
		})
	}
}

func TestRemoteReadsExactRefsAndStopsOnLateAPIFailure(t *testing.T) {
	t.Parallel()
	for _, failAt := range []int64{0, 2, 3} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			f := newRemoteFixture(t)
			var prReads, contentReads atomic.Int64
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == repositoryAPI+"/pulls/123" && prReads.Add(1) == failAt {
					w.WriteHeader(503)
					return true
				}
				if strings.HasPrefix(r.URL.Path, repositoryAPI+"/contents/") {
					contentReads.Add(1)
					ref := r.URL.Query().Get("ref")
					if ref != f.inputs.TrustedSHA && ref != f.event.Run.HeadSHA {
						t.Errorf("mutable/wrong ref: %q", ref)
					}
				}
				return false
			}
			plan, err := f.remote.Plan(t.Context(), f.inputs)
			if failAt == 0 {
				if err != nil || plan.Status != "planned" || contentReads.Load() != 4 {
					t.Fatalf("err=%v plan=%s contents=%d", err, plan.Status, contentReads.Load())
				}
			} else if !errors.Is(err, ErrGitHub) || prReads.Load() != failAt {
				t.Fatalf("err=%v reads=%d", err, prReads.Load())
			}
			if f.posts.Load() != 0 {
				t.Fatal("plan mutated remote state")
			}
		})
	}
}
