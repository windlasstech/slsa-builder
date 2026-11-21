package dependabot

import (
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestRemoteRejectsWrongBotLoginAtPRBoundary(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	f := newRemoteFixture(t)
	var fileReads atomic.Int64
	var prReads atomic.Int64
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == repositoryAPI+"/pulls/123/files" {
			fileReads.Add(1)
		}
		if r.URL.Path != repositoryAPI+"/pulls/123" {
			return false
		}
		pr, err := decodeGitHub[githubPR]([]byte(f.responses[r.URL.Path]))
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return true
		}
		pr.User.Login = "other[bot]"
		prReads.Add(1)
		fmt.Fprint(w, fixtureJSON(t, pr))
		return true
	}
	_, err := f.remote.Apply(t.Context(), f.inputs, "companion")
	if !errors.Is(err, ErrInput) || prReads.Load() != 1 || fileReads.Load() != 0 || f.mutations.Load() != 0 {
		t.Fatalf("error=%v pr reads=%d file reads=%d mutations=%d", err, prReads.Load(), fileReads.Load(), f.mutations.Load())
	}
	t.Log("wrong-login Bot: rejected before file enumeration; mutations=0")
}
