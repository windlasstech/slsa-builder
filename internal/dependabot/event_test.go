package dependabot

import (
	"os"
	"testing"
)

func TestDependabotEventClosed(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{}`, `null`, `{"repository":"x","repository":"y"}`, `{"unknown":true}`} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			// Given malformed or non-closed event bytes, when parsing, then reject.
			if _, err := ParseEvent([]byte(raw)); err == nil {
				t.Fatal("accepted malformed event")
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
	f.Add([]byte(`{"repository":"a","repository":"b"}`))
	f.Add([]byte(`{"action":"completed","workflow_run":{"name":"Lint","status":"completed"}}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		event, err := ParseEvent(raw)
		if err == nil && event.SchemaVersion != 1 {
			t.Fatal("accepted unsupported schema")
		}
		remote, err := parseWorkflowEvent(raw)
		if err == nil && (remote.Action != "completed" || remote.Run.Name != "Lint" || remote.Repository.FullName != companionRepository) {
			t.Fatal("accepted unbound workflow event")
		}
	})
}
