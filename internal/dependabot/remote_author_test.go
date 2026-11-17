package dependabot

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestRemoteRejectsWrongBotLoginAtPRBoundary(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	f := newRemoteFixture(t)
	var fileReads atomic.Int64
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
		fmt.Fprint(w, fixtureJSON(t, pr))
		return true
	}
	_, err := f.remote.Apply(t.Context(), f.inputs, "companion")
	if err == nil || fileReads.Load() != 0 || f.mutations.Load() != 0 {
		t.Fatalf("error=%v file reads=%d mutations=%d", err, fileReads.Load(), f.mutations.Load())
	}
	t.Log("wrong-login Bot: rejected before file enumeration; mutations=0")
}
