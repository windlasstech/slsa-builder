package dependabot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestRemotePlanBindsWorkflowPullRequest(t *testing.T) {
	t.Parallel()
	f := newRemoteFixture(t)
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(fixtureJSON(t, f.event)), &payload); err != nil {
		t.Fatal(err)
	}
	var run map[string]json.RawMessage
	if err := json.Unmarshal(payload["workflow_run"], &run); err != nil {
		t.Fatal(err)
	}
	run["pull_requests"] = json.RawMessage("[" + f.responses[repositoryAPI+"/pulls/123"] + "]")
	payload["workflow_run"] = json.RawMessage(fixtureJSON(t, run))
	raw := fixtureJSON(t, payload)
	if err := os.WriteFile(f.inputs.EventPath, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == repositoryAPI+"/pulls" {
			t.Error("unexpected fallback lookup")
			w.WriteHeader(500)
			return true
		}
		return false
	}
	first, err := f.remote.Plan(t.Context(), f.inputs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.remote.Plan(t.Context(), f.inputs)
	if err != nil || !reflect.DeepEqual(first, second) || first.Number != 123 || f.posts.Load() != 0 {
		t.Fatalf("err=%v plans differ=%t", err, !reflect.DeepEqual(first, second))
	}
}

func TestRemotePlanRejectsUnboundEvents(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, old, replacement string }{
		{"incomplete", `"status":"completed"`, `"status":"in_progress"`},
		{"wrong workflow", `"name":"Lint"`, `"name":"Other"`},
		{"missing conclusion", `"conclusion":"failure"`, `"conclusion":null`},
		{"wrong source", `"event":"pull_request"`, `"event":"push"`},
		{"wrong head", `"head_sha":"` + strings.Repeat("b", 40) + `"`, `"head_sha":"invalid"`},
		{"duplicate", `"action":"completed"`, `"action":"completed","action":"requested"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newRemoteFixture(t)
			raw := strings.Replace(fixtureJSON(t, f.event), test.old, test.replacement, 1)
			if raw == fixtureJSON(t, f.event) {
				t.Fatal("fixture mutation did not apply")
			}
			if err := os.WriteFile(f.inputs.EventPath, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := f.remote.Plan(t.Context(), f.inputs)
			if err == nil || f.reads.Load() != 0 || f.posts.Load() != 0 {
				t.Fatalf("err=%v requests=%d", err, f.reads.Load())
			}
		})
	}
}

func TestRemotePlanRejectsLaterPageCompanionEdit(t *testing.T) {
	t.Parallel()
	f := newRemoteFixture(t)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == repositoryAPI+"/pulls/123/files" && r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `[{"filename":"CHANGELOG.md","status":"modified"}]`)
			return true
		}
		return false
	}
	_, err := f.remote.Plan(t.Context(), f.inputs)
	if err == nil || f.posts.Load() != 0 {
		t.Fatalf("err=%v posts=%d", err, f.posts.Load())
	}
}
