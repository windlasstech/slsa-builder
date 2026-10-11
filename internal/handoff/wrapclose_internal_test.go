package handoff

import (
	"errors"
	"testing"
)

// wrapCloseError is unexported and its read-failure call site in VerifyOneFile
// cannot be forced through the public surface, so it is tested here directly.
func TestWrapCloseError(t *testing.T) {
	t.Parallel()

	if got := wrapCloseError(nil, "close artifact payload"); got != nil {
		t.Fatalf("wrapCloseError(nil) = %v, want nil", got)
	}

	sentinel := errors.New("close failed")
	wrapped := wrapCloseError(sentinel, "close artifact payload")
	if wrapped == nil {
		t.Fatal("wrapCloseError(sentinel) = nil, want a wrapped error")
	}
	if !errors.Is(wrapped, sentinel) {
		t.Fatalf("errors.Is(%v, sentinel) = false, want true", wrapped)
	}
	if got, want := wrapped.Error(), "close artifact payload: close failed"; got != want {
		t.Fatalf("wrapCloseError() message = %q, want %q", got, want)
	}
}

// closeRoot's close-failure branch is intentionally untested: there is no
// portable way to force os.Root.Close to fail. Its success path is exercised
// through every VerifyOneFile test.
