package provenance_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func FuzzDecodeInternalParameters(f *testing.F) {
	f.Add([]byte(`{"github":{"event_name":"workflow_dispatch","repository_id":"123456789","repository_owner_id":"987654321"}}`))
	f.Add([]byte(`{"github":{"event_name":"push","repository_id":123,"repository_owner_id":"456"}}`))
	f.Add([]byte(`{"github":{"event_name":"push","repository_id":"123","repository_owner_id":"456","actor_id":"42"}}`))
	f.Add([]byte(`{"github":{"event_name":"push","event_name":"workflow_dispatch","repository_id":"123","repository_owner_id":"456"}}`))
	f.Add([]byte(`{"github":{"event_name":"push","repository_id":"123","repository_owner_id":"456"}} {}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		decoded, err := provenance.DecodeInternalParameters(input)
		if err != nil {
			var identified interface{ DiagnosticID() string }
			if !errors.As(err, &identified) || identified.DiagnosticID() != "windlass.verify.error.unexpected-internal-parameters" {
				t.Fatalf("rejection = %v, want unexpected-internal-parameters", err)
			}
			return
		}
		if decoded.GitHub.EventName == "" || decoded.GitHub.RepositoryID == "" || decoded.GitHub.RepositoryOwnerID == "" {
			t.Fatal("successful decode contains an empty required field")
		}
		encoded, err := json.Marshal(decoded)
		if err != nil {
			t.Fatalf("marshal accepted internal parameters: %v", err)
		}
		roundTrip, err := provenance.DecodeInternalParameters(encoded)
		if err != nil {
			t.Fatalf("redecode accepted internal parameters: %v", err)
		}
		if !reflect.DeepEqual(decoded, roundTrip) {
			t.Fatalf("round trip changed internal parameters: first %#v, second %#v", decoded, roundTrip)
		}
	})
}
