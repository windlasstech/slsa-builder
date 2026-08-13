package npmprofile

import (
	"bytes"
	"sync"
	"testing"
)

func TestNormalizeSourceRefInput(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "omitted", input: "", want: ""},
		{name: "ASCII whitespace only remains invalid", input: " \t\n\r\v\f", want: " \t\n\r\v\f"},
		{name: "padded tag remains noncanonical", input: " \trefs/tags/v1.2.3\r\n", want: " \trefs/tags/v1.2.3\r\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeSourceRefInput(test.input); got != test.want {
				t.Fatalf("NormalizeSourceRefInput(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestValidateSourceRefInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		sourceRef     string
		invocationRef string
		version       string
		wantID        string
	}{
		{name: "omitted tag run", invocationRef: "refs/tags/v1.2.3", version: "1.2.3"},
		{name: "ASCII whitespace only", sourceRef: " \t\n\r\v\f", invocationRef: "refs/tags/v1.2.3", version: "1.2.3", wantID: IDSourceRefInvalid},
		{name: "dispatch retry", sourceRef: "refs/tags/v1.2.3", invocationRef: "refs/heads/main", version: "1.2.3"},
		{name: "leading space", sourceRef: " refs/tags/v1.2.3", invocationRef: "refs/heads/main", version: "1.2.3", wantID: IDSourceRefInvalid},
		{name: "trailing space", sourceRef: "refs/tags/v1.2.3 ", invocationRef: "refs/heads/main", version: "1.2.3", wantID: IDSourceRefInvalid},
		{name: "tab padded", sourceRef: "\trefs/tags/v1.2.3\t", invocationRef: "refs/heads/main", version: "1.2.3", wantID: IDSourceRefInvalid},
		{name: "branch", sourceRef: "refs/heads/main", invocationRef: "refs/heads/main", version: "1.2.3", wantID: IDSourceRefInvalid},
		{name: "short tag", sourceRef: "v1.2.3", invocationRef: "refs/heads/main", version: "1.2.3", wantID: IDSourceRefInvalid},
		{name: "commit SHA", sourceRef: "0123456789abcdef0123456789abcdef01234567", invocationRef: "refs/heads/main", version: "1.2.3", wantID: IDSourceRefInvalid},
		{name: "wrong version tag", sourceRef: "refs/tags/v1.2.4", invocationRef: "refs/heads/main", version: "1.2.3", wantID: IDSourceRefInvalid},
		{name: "tag conflict", sourceRef: "refs/tags/v1.2.4", invocationRef: "refs/tags/v1.2.3", version: "1.2.4", wantID: IDSourceRefInvalid},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateSourceRefInput(test.sourceRef, test.invocationRef, test.version)
			if test.wantID == "" {
				if err != nil {
					t.Fatalf("ValidateSourceRefInput() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateSourceRefInput() error = nil")
			}
			diagnosticError, ok := err.(interface{ DiagnosticID() string })
			if !ok {
				t.Fatalf("ValidateSourceRefInput() error type = %T, want typed diagnostic error", err)
			}
			if got := diagnosticError.DiagnosticID(); got != test.wantID {
				t.Fatalf("ValidateSourceRefInput() diagnostic ID = %q, want %q", got, test.wantID)
			}
		})
	}
}

func TestBoundedOutput(t *testing.T) {
	t.Parallel()

	t.Run("retains exactly the limit", func(t *testing.T) {
		t.Parallel()
		output := newBoundedOutput(sourceRefGitOutputLimit)
		payload := bytes.Repeat([]byte("a"), sourceRefGitOutputLimit+1)
		written, err := output.Write(payload)
		if err != nil || written != len(payload) {
			t.Fatalf("Write() = (%d, %v), want (%d, nil)", written, err, len(payload))
		}
		if got := output.String(); len(got) != sourceRefGitOutputLimit || got != string(payload[:sourceRefGitOutputLimit]) {
			t.Fatalf("retained output length = %d, want %d", len(got), sourceRefGitOutputLimit)
		}
	})

	t.Run("reports full writes after saturation", func(t *testing.T) {
		t.Parallel()
		output := newBoundedOutput(1)
		if written, err := output.Write([]byte("a")); err != nil || written != 1 {
			t.Fatalf("initial Write() = (%d, %v), want (1, nil)", written, err)
		}
		payload := []byte("discarded after saturation")
		if written, err := output.Write(payload); err != nil || written != len(payload) {
			t.Fatalf("saturated Write() = (%d, %v), want (%d, nil)", written, err, len(payload))
		}
		if got := output.String(); got != "a" {
			t.Fatalf("retained output = %q, want %q", got, "a")
		}
	})

	t.Run("concurrent writes are bounded", func(t *testing.T) {
		t.Parallel()
		const writers = 32
		const chunkSize = 64 << 10
		output := newBoundedOutput(sourceRefGitOutputLimit)
		payload := bytes.Repeat([]byte("x"), chunkSize)
		var wait sync.WaitGroup
		wait.Add(writers)
		for range writers {
			go func() {
				defer wait.Done()
				if written, err := output.Write(payload); err != nil || written != len(payload) {
					t.Errorf("Write() = (%d, %v), want (%d, nil)", written, err, len(payload))
				}
			}()
		}
		wait.Wait()
		if got := len(output.String()); got != sourceRefGitOutputLimit {
			t.Fatalf("retained output length = %d, want %d", got, sourceRefGitOutputLimit)
		}
	})
}
