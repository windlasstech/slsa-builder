package dependabot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

type remoteFixture struct {
	remote    *Remote
	inputs    Inputs
	event     workflowEvent
	responses map[string]string
	mutations atomic.Int64
	posts     atomic.Int64
	reads     atomic.Int64
	commitMu  sync.Mutex
	commit    commitInput
	override  func(http.ResponseWriter, *http.Request) bool
}

func fixtureJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func contentJSON(t *testing.T, path string, data []byte) string {
	t.Helper()
	content := githubContent{Type: "file", Path: path, Size: len(data), Encoding: "base64", Content: base64.StdEncoding.EncodeToString(data)}
	content.License.SPDX = "Apache-2.0"
	return fixtureJSON(t, content)
}

func newRemoteFixture(t *testing.T) *remoteFixture {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	base, err := readModules(root)
	if err != nil {
		t.Fatal(err)
	}
	old, err := toolchainpins.ParseSigningAdapterModule(base.Mod, base.Sum)
	if err != nil {
		t.Fatal(err)
	}
	head := Modules{Mod: bytes.ReplaceAll(base.Mod, []byte("sigstore-go "+old.Version), []byte("sigstore-go v1.999.0")),
		Sum: bytes.ReplaceAll(base.Sum, []byte("sigstore-go "+old.Version), []byte("sigstore-go v1.999.0"))}
	root, sha := NewCommittedTestRoot(t, map[string][]byte{"go.mod": base.Mod, "go.sum": base.Sum})
	f := &remoteFixture{responses: map[string]string{}}
	sum := remoteModuleFixture(t, f.responses)
	head.Sum = bytes.ReplaceAll(head.Sum, []byte(signerModule+" v1.999.0 "+old.Sum), []byte(signerModule+" v1.999.0 "+sum))
	f.inputs = Inputs{TrustedRoot: root, TrustedSHA: sha, EventPath: filepath.Join(t.TempDir(), "event.json"), Output: filepath.Join(t.TempDir(), "output")}
	f.event.Action = "completed"
	f.event.Repository.FullName = companionRepository
	f.event.Run.Name, f.event.Run.Status, f.event.Run.Conclusion, f.event.Run.Event = "Lint", "completed", "failure", "pull_request"
	f.event.Run.HeadSHA, f.event.Run.HeadBranch = strings.Repeat("b", 40), "dependabot/go_modules/sigstore"
	f.event.Run.HeadRepository.FullName = companionRepository
	if err := os.WriteFile(f.inputs.EventPath, []byte(fixtureJSON(t, f.event)), 0600); err != nil {
		t.Fatal(err)
	}
	pr := githubPR{Number: 123, State: "open", ChangedFiles: 2}
	pr.User.Login, pr.User.Type = "dependabot[bot]", "Bot"
	pr.Base = githubRef{SHA: f.inputs.TrustedSHA, Ref: "main", Repo: githubRepository{companionRepository}}
	pr.Head = githubRef{SHA: f.event.Run.HeadSHA, Ref: f.event.Run.HeadBranch, Repo: githubRepository{companionRepository}}
	f.responses[repositoryAPI+"/pulls"] = "[" + fixtureJSON(t, pr) + "]"
	f.responses[repositoryAPI+"/pulls/123"] = fixtureJSON(t, pr)
	f.responses[repositoryAPI+"/pulls/123/files"] = `[{"filename":"go.mod","status":"modified"},{"filename":"go.sum","status":"modified"}]`
	for i, ref := range []string{pr.Base.SHA, pr.Head.SHA} {
		module := []Modules{base, head}[i]
		f.responses[repositoryAPI+"/contents/go.mod?ref="+ref] = contentJSON(t, "go.mod", module.Mod)
		f.responses[repositoryAPI+"/contents/go.sum?ref="+ref] = contentJSON(t, "go.sum", module.Sum)
	}
	for i, version := range []string{old.Version, "v1.999.0"} {
		commit := []string{strings.Repeat("f", 40), strings.Repeat("c", 40)}[i]
		f.responses["/repos/sigstore/sigstore-go/releases/tags/"+version] = fmt.Sprintf(`{"tag_name":%q,"target_commitish":%q,"published_at":"2026-11-10T00:00:00Z","draft":false,"prerelease":false}`, version, commit)
		f.responses["/repos/sigstore/sigstore-go/git/ref/tags/"+version] = fmt.Sprintf(`{"ref":%q,"object":{"type":"commit","sha":%q}}`, "refs/tags/"+version, commit)
		f.responses["/repos/sigstore/sigstore-go/license?ref="+commit] = contentJSON(t, "LICENSE", []byte("Apache License, Version 2.0\n"))
	}
	f.responses["/repos/sigstore/sigstore-go/contents/go.mod?ref="+strings.Repeat("c", 40)] = contentJSON(t, "go.mod", []byte("module github.com/sigstore/sigstore-go\n\ngo 1.27.0\n"))
	f.responses["/installation/repositories"] = `{"total_count":1,"repositories":[{"full_name":"windlasstech/slsa-builder"}]}`
	f.responses["/users/companion[bot]"] = `{"login":"companion[bot]","id":42,"type":"Bot"}`
	f.responses[repositoryAPI+"/git/ref/heads/"+pr.Head.Ref] = `{"object":{"sha":"` + strings.Repeat("d", 40) + `"}}`
	f.responses[repositoryAPI+"/contents/internal/provenance/builder_dependencies.go?ref="+strings.Repeat("d", 40)] = contentJSON(t, "internal/provenance/builder_dependencies.go", []byte("new\n"))
	f.responses[repositoryAPI+"/contents/internal/provenance/builder_dependencies.go?ref="+strings.Repeat("b", 40)] = contentJSON(t, "internal/provenance/builder_dependencies.go", []byte("old\n"))
	api := testAPI(t, f.handler(t))
	f.remote = &Remote{api: api, planner: Planner{Generate: func(context.Context, Generation) ([]Change, error) {
		return []Change{{Path: "internal/provenance/builder_dependencies.go", Before: []byte("old\n"), After: []byte("new\n")}}, nil
	}}}
	return f
}

func (f *remoteFixture) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing authentication")
		}
		if r.Method == http.MethodPost {
			f.posts.Add(1)
			var request struct {
				Query     string
				Variables struct{ Input commitInput }
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				return
			}
			if strings.HasPrefix(request.Query, "mutation") {
				f.mutations.Add(1)
				f.commitMu.Lock()
				f.commit = request.Variables.Input
				f.commitMu.Unlock()
				if f.override != nil && f.override(w, r) {
					return
				}
				fmt.Fprintf(w, `{"data":{"createCommitOnBranch":{"commit":{"oid":%q}}}}`, strings.Repeat("d", 40))
				return
			}
			if f.override != nil && f.override(w, r) {
				return
			}
			fmt.Fprint(w, `{"data":{"viewer":{"login":"companion[bot]","databaseId":42}}}`)
			return
		}
		f.reads.Add(1)
		if f.override != nil && f.override(w, r) {
			return
		}
		if strings.Contains(r.URL.Path, "/commits/"+strings.Repeat("d", 40)) {
			input := f.committed()
			fmt.Fprintf(w, `{"sha":%q,"parents":[{"sha":%q}],"commit":{"message":%q,"verification":{"verified":true}}}`, strings.Repeat("d", 40), input.ExpectedHeadOID, input.Message.Headline+"\n\n"+input.Message.Body)
			return
		}
		if page := r.URL.Query().Get("page"); page != "" && page != "1" {
			fmt.Fprint(w, `[]`)
			return
		}
		key := r.URL.Path
		if ref := r.URL.Query().Get("ref"); ref != "" {
			key += "?ref=" + ref
		}
		if body, ok := f.responses[key]; ok {
			fmt.Fprint(w, body)
			return
		}
		t.Errorf("unexpected API read: %s", r.URL)
		http.NotFound(w, r)
	}
}

func (f *remoteFixture) committed() commitInput {
	f.commitMu.Lock()
	defer f.commitMu.Unlock()
	return f.commit
}
