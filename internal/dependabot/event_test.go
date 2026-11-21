package dependabot

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestDependabotEventClosed(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{}`, `null`, `{"repository":"x","repository":"y"}`, `{"unknown":true}`} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			// Given malformed or non-closed event bytes, when parsing, then reject.
			if _, err := ParseEvent([]byte(raw)); !errors.Is(err, ErrInput) {
				t.Fatalf("error=%v, want ErrInput", err)
			}
		})
	}
}

func FuzzDependabotEvent(f *testing.F) {
	data, err := os.ReadFile("testdata/event.json")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	workflow, err := os.ReadFile("testdata/workflow-event.json")
	if err != nil {
		f.Fatal(err)
	}
	if _, err := ParseEvent(data); err != nil {
		f.Fatalf("local seed rejected: %v", err)
	}
	if _, err := parseWorkflowEvent(workflow); err != nil {
		f.Fatalf("workflow seed rejected: %v", err)
	}
	f.Add(workflow)
	f.Add([]byte(`null`))
	f.Add([]byte(`{"schema_version":1,"schema_version":2}`))
	f.Add([]byte(`{"repository":"a","repository":"b"}`))
	f.Add([]byte(`{"action":"completed","workflow_run":{"name":"Lint","status":"completed"}}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		event, localErr := ParseEvent(raw)
		remote, remoteErr := parseWorkflowEvent(raw)
		if localErr == nil && remoteErr == nil {
			t.Fatal("closed local projection and workflow webhook domains overlap")
		}
		if localErr == nil {
			encoded, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			roundTrip, err := ParseEvent(encoded)
			if err != nil || !reflect.DeepEqual(event, roundTrip) {
				t.Fatalf("local round trip changed event: %+v / %+v, %v", event, roundTrip, err)
			}
		}
		if remoteErr == nil {
			encoded, err := json.Marshal(remote)
			if err != nil {
				t.Fatal(err)
			}
			roundTrip, err := parseWorkflowEvent(encoded)
			if err != nil || !reflect.DeepEqual(remote, roundTrip) {
				t.Fatalf("workflow round trip changed bindings: %+v / %+v, %v", remote, roundTrip, err)
			}
		}
	})
}

func TestWorkflowEventBindings(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/workflow-event.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func(*workflowEvent)
		valid  bool
	}{
		{"complete seed", func(*workflowEvent) {}, true},
		{"lookup required", func(e *workflowEvent) { e.Run.PullRequests = nil }, true},
		{"action", func(e *workflowEvent) { e.Action = "requested" }, false},
		{"repository", func(e *workflowEvent) { e.Repository.FullName = "other/repo" }, false},
		{"name", func(e *workflowEvent) { e.Run.Name = "Other" }, false},
		{"status", func(e *workflowEvent) { e.Run.Status = "in_progress" }, false},
		{"conclusion", func(e *workflowEvent) { e.Run.Conclusion = "" }, false},
		{"event", func(e *workflowEvent) { e.Run.Event = "push" }, false},
		{"head repository", func(e *workflowEvent) { e.Run.HeadRepository.FullName = "fork/repo" }, false},
		{"head sha", func(e *workflowEvent) { e.Run.HeadSHA = "invalid" }, false},
		{"head branch", func(e *workflowEvent) { e.Run.HeadBranch = "feature" }, false},
		{"multiple prs", func(e *workflowEvent) { e.Run.PullRequests = append(e.Run.PullRequests, e.Run.PullRequests[0]) }, false},
		{"pr number", func(e *workflowEvent) { e.Run.PullRequests[0].Number = 0 }, false},
		{"pr head sha", func(e *workflowEvent) { e.Run.PullRequests[0].Head.SHA = "cccccccccccccccccccccccccccccccccccccccc" }, false},
		{"pr head branch", func(e *workflowEvent) { e.Run.PullRequests[0].Head.Ref = "dependabot/other" }, false},
		{"pr base", func(e *workflowEvent) { e.Run.PullRequests[0].Base.Ref = "feature" }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var input workflowEvent
			if err := json.Unmarshal(raw, &input); err != nil {
				t.Fatal(err)
			}
			test.change(&input)
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := parseWorkflowEvent(encoded)
			if test.valid {
				if err != nil || !reflect.DeepEqual(got, input) {
					t.Fatalf("workflow bindings changed: got=%+v want=%+v err=%v", got, input, err)
				}
			} else if !errors.Is(err, ErrInput) {
				t.Fatalf("error=%v, want ErrInput for %s", err, test.name)
			}
		})
	}
}
