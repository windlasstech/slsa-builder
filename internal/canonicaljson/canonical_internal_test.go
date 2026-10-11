package canonicaljson

import "testing"

// TestErrorsUnexpectedCanonicalWrapper pins the exact formatted message of the defensive error
// used when the vetted RFC 8785 transformer returns output that is not the expected wrapped array.
// The Canonicalize call site that reaches this error is intentionally uncovered: triggering it
// would require a seam to replace the transformer, and the branch is defensive only.
func TestErrorsUnexpectedCanonicalWrapper(t *testing.T) {
	t.Parallel()

	err := errorsUnexpectedCanonicalWrapper([]byte(`{"a":1}`))
	want := `RFC 8785 transformer returned invalid wrapped output "{\"a\":1}"`
	if err.Error() != want {
		t.Fatalf("errorsUnexpectedCanonicalWrapper() = %q, want %q", err, want)
	}
}
