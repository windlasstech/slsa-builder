package diagnostic

import "testing"

func TestNewRejectsInvalidConstruction(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, id, check, message, reason string }{
		{"unknown identity", "unknown", "check", "message", `unknown diagnostic ID "unknown"`},
		{"missing check", IDDigestMismatch, "", "message", `diagnostic "windlass.verify.error.digest-mismatch" requires non-empty UTF-8 check and message`},
		{"invalid message encoding", IDDigestMismatch, "digest", "\xff", `diagnostic "windlass.verify.error.digest-mismatch" requires non-empty UTF-8 check and message`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(test.id, test.check, test.message)
			requireContractReason(t, err, test.reason)
		})
	}
}

func TestBuildRejectsEmptyInvocation(t *testing.T) {
	t.Parallel()
	_, err := Build(new(""), nil, nil)
	requireContractReason(t, err, "run_invocation must be non-empty when present")
}
