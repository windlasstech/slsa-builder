package dependabot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestRemotePreimagesUseCandidateOnlyForRootModuleFiles(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, path, served string
		accepted           bool
	}{
		{"module candidate", "go.mod", "candidate\n", true},
		{"checksum candidate", "go.sum", "candidate\n", true},
		{"module base rejected", "go.mod", "base\n", false},
		{"checksum base rejected", "go.sum", "base\n", false},
		{"module corruption", "go.mod", "wrong\n", false},
		{"checksum corruption", "go.sum", "wrong\n", false},
		{"companion base", "internal/provenance/builder_dependencies.go", "base\n", true},
		{"companion candidate rejected", "internal/provenance/builder_dependencies.go", "candidate\n", false},
		{"nested module remains preimage", "nested/go.mod", "base\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			plan := Plan{HeadSHA: strings.Repeat("b", 40), Files: []FileHash{{Path: test.path, BeforeSHA256: hash([]byte("base\n")), SHA256: hash([]byte("candidate\n"))}}}
			api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != repositoryAPI+"/contents/"+test.path || r.URL.Query().Get("ref") != plan.HeadSHA {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				fmt.Fprint(w, contentJSON(t, test.path, []byte(test.served)))
			})
			err := api.checkPreimages(t.Context(), plan)
			if (err == nil) != test.accepted || (!test.accepted && !errors.Is(err, ErrInput)) {
				t.Fatalf("accepted=%t error=%v", test.accepted, err)
			}
		})
	}
}

func TestRemotePreimageCancellationRetainsTransportClass(t *testing.T) {
	t.Parallel()
	api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r
		t.Error("cancelled request reached server")
		w.WriteHeader(500)
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := api.checkPreimages(ctx, Plan{HeadSHA: strings.Repeat("b", 40), Files: []FileHash{{Path: "go.mod"}}})
	if !errors.Is(err, ErrGitHub) || errors.Is(err, ErrInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestRemoteApplyAcceptsChangedHeadModules(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	f := newRemoteFixture(t)
	for _, path := range []string{"go.mod", "go.sum"} {
		base, err := decodeGitHub[githubContent]([]byte(f.responses[repositoryAPI+"/contents/"+path+"?ref="+f.inputs.TrustedSHA]))
		if err != nil {
			t.Fatal(err)
		}
		head, err := decodeGitHub[githubContent]([]byte(f.responses[repositoryAPI+"/contents/"+path+"?ref="+f.event.Run.HeadSHA]))
		if err != nil {
			t.Fatal(err)
		}
		baseBytes, err := base.bytes(path)
		if err != nil {
			t.Fatal(err)
		}
		headBytes, err := head.bytes(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(baseBytes, headBytes) {
			t.Fatalf("%s fixture must distinguish base from candidate", path)
		}
	}
	plan, err := f.remote.Apply(t.Context(), f.inputs, "companion")
	if err != nil || plan.Status != "planned" || f.mutations.Load() != 1 {
		t.Fatalf("status=%s mutations=%d error=%v", plan.Status, f.mutations.Load(), err)
	}
	t.Logf("differing base/head go.mod and go.sum: mutations=%d, status=%s", f.mutations.Load(), plan.Status)
}
