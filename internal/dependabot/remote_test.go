package dependabot

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRemotePlanOnlyReads(t *testing.T) {
	t.Parallel()
	f := newRemoteFixture(t)
	plan, err := f.remote.Plan(t.Context(), f.inputs)
	if err != nil || plan.Status != "planned" || len(plan.Files) != 1 || f.posts.Load() != 0 || f.mutations.Load() != 0 || f.reads.Load() < 10 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func TestRemoteApplyOneMutation(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	f := newRemoteFixture(t)
	plan, err := f.remote.Apply(t.Context(), f.inputs, "companion")
	if err != nil || plan.Status != "planned" || f.mutations.Load() != 1 {
		t.Fatalf("plan=%+v err=%v mutations=%d", plan, err, f.mutations.Load())
	}
	input := <-f.commits
	if input.ExpectedHeadOID != f.event.Run.HeadSHA || input.Branch.RefName != f.event.Run.HeadBranch || input.Branch.Repository != companionRepository || len(input.FileChanges.Additions) != 1 || input.FileChanges.Additions[0].Contents != "bmV3Cg==" || !strings.Contains(input.Message.Headline, "[dependabot skip]") || input.Message.Body != "Signed-off-by: companion[bot] <42+companion[bot]@users.noreply.github.com>\n" {
		t.Fatalf("wrong mutation: %+v", input)
	}
}

func TestRemoteApplyRejectsBeforeMutation(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	for _, test := range []struct{ name, path, body string }{
		{"human", repositoryAPI + "/pulls/123", `{"number":123,"user":{"login":"human","type":"User"}}`},
		{"missing", repositoryAPI + "/pulls/123", `{}`},
		{"duplicate", repositoryAPI + "/pulls/123", `{"state":"open","state":"closed"}`},
		{"mixed", repositoryAPI + "/pulls/123/files", `[{"filename":"evil.go","status":"modified"},{"filename":"go.mod","status":"modified"}]`},
		{"renamed", repositoryAPI + "/pulls/123/files", `[{"filename":"go.mod","status":"renamed"},{"filename":"go.sum","status":"modified"}]`},
		{"incomplete", repositoryAPI + "/pulls/123/files", `[{"filename":"go.mod","status":"modified"}]`},
		{"duplicate-path", repositoryAPI + "/pulls/123/files", `[{"filename":"go.mod","status":"modified"},{"filename":"go.mod","status":"modified"}]`},
		{"ambiguous", repositoryAPI + "/pulls", `[{"number":123},{"number":124}]`},
		{"scope", "/installation/repositories", `{"total_count":2,"repositories":[]}`},
		{"identity", "/graphql", `{"data":{"viewer":{"login":"human","databaseId":42}}}`},
		{"release", "/repos/sigstore/sigstore-go/releases/tags/v1.999.0", `{"tag_name":"v1.999.0","published_at":"invalid"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRemoteFixture(t)
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == test.path {
					fmt.Fprint(w, test.body)
					return true
				}
				return false
			}
			_, err := f.remote.Apply(t.Context(), f.inputs, "companion")
			if err == nil || f.mutations.Load() != 0 {
				t.Fatalf("err=%v mutations=%d", err, f.mutations.Load())
			}
		})
	}
}

func TestRemoteApplyAmbiguityNeverRetries(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	for _, failure := range []string{"mutation-error", "mutation-timeout", "readback-timeout", "missing-oid", "unverified", "wrong-parent", "wrong-content", "missing-commit", "wrong-head"} {
		t.Run(failure, func(t *testing.T) {
			f := newRemoteFixture(t)
			f.remote.api.client.Timeout = time.Second
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				if f.mutations.Load() == 0 {
					return false
				}
				switch {
				case failure == "mutation-timeout" && r.Method == "POST", failure == "readback-timeout" && strings.Contains(r.URL.Path, "/commits/"):
					<-r.Context().Done()
				case failure == "mutation-error" && r.Method == "POST":
					w.WriteHeader(502)
				case failure == "missing-oid" && r.Method == "POST":
					fmt.Fprint(w, `{"data":{}}`)
				case failure == "missing-commit" && strings.Contains(r.URL.Path, "/commits/"):
					w.WriteHeader(404)
				case (failure == "unverified" || failure == "wrong-parent") && strings.Contains(r.URL.Path, "/commits/"):
					input := <-f.commits
					f.commits <- input
					parent := input.ExpectedHeadOID
					if failure == "wrong-parent" {
						parent = strings.Repeat("e", 40)
					}
					fmt.Fprintf(w, `{"sha":%q,"parents":[{"sha":%q}],"commit":{"message":%q,"verification":{"verified":%t}}}`, strings.Repeat("d", 40), parent, input.Message.Headline+"\n\n"+input.Message.Body, failure != "unverified")
				case failure == "wrong-content" && strings.Contains(r.URL.Path, "/contents/"):
					fmt.Fprint(w, contentJSON(t, "internal/provenance/builder_dependencies.go", []byte("wrong")))
				case failure == "wrong-head" && strings.Contains(r.URL.Path, "/git/ref/"):
					fmt.Fprint(w, `{"object":{"sha":"wrong"}}`)
				default:
					return false
				}
				return true
			}
			_, err := f.remote.Apply(t.Context(), f.inputs, "companion")
			if !errors.Is(err, ErrIndeterminate) || f.mutations.Load() != 1 {
				t.Fatalf("err=%v mutations=%d", err, f.mutations.Load())
			}
		})
	}
}

func TestRemoteApplyDisabledAndLocalInputRejected(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "false")
	f := newRemoteFixture(t)
	_, err := f.remote.Apply(t.Context(), f.inputs, "companion")
	if err == nil || f.reads.Load() != 0 || f.mutations.Load() != 0 {
		t.Fatal("disabled apply reached API")
	}
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	if err := os.WriteFile(f.inputs.EventPath, []byte(`{"schema_version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = f.remote.Apply(t.Context(), f.inputs, "companion")
	if err == nil || f.reads.Load() != 0 || f.mutations.Load() != 0 {
		t.Fatal("local input reached API")
	}
}
