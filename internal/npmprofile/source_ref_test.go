package npmprofile

import "testing"

func TestNormalizeSourceRefInput(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "omitted", input: "", want: ""},
		{name: "ASCII whitespace only", input: " \t\n\r\v\f", want: ""},
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
		{name: "ASCII whitespace only", sourceRef: " \t\n\r\v\f", invocationRef: "refs/tags/v1.2.3", version: "1.2.3"},
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
