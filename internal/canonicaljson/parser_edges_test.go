package canonicaljson_test

import (
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

func TestStrictEscapeRejections(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, input, want string }{
		{"unfinished escape", "\"\\", "unterminated JSON escape at byte offset 1"},
		{"short unicode", `"\u12`, "incomplete Unicode escape at byte offset 1"},
		{"invalid hex", `"\u00xz"`, "invalid Unicode escape at byte offset 1"},
		{"invalid second surrogate escape", `"\ud800\uZZZZ"`, "invalid Unicode escape at byte offset 7"},
		{"high surrogate followed by ordinary codepoint", `"\ud800\u0041"`, "unpaired high surrogate at byte offset 1"},
		{"low surrogate", `"\uDC00"`, "unpaired low surrogate at byte offset 1"},
		{"high surrogate followed by text", `"\uD800abcdef"`, "unpaired high surrogate at byte offset 1"},
		{"escape outside string", `\`, `parse JSON value: invalid character '\\' looking for beginning of value`},
		{"invalid trailing token", `{}x`, "parse trailing JSON data: invalid character 'x' looking for beginning of value"},
		{"invalid object key", `{1:0}`, "parse JSON value: object member name must be a string"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := canonicaljson.Validate([]byte(test.input))
			if err == nil || err.Error() != test.want {
				t.Fatalf("Validate(%q) error = %v, want %q", test.input, err, test.want)
			}
		})
	}
}

func TestTruncatedContainersPreserveClassification(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`{`, `[`, `{"a":`, `{"a":1`, `[1`, `[{`} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			err := canonicaljson.Validate([]byte(input))
			if input == `{"a":` {
				if !errors.Is(err, io.EOF) {
					t.Fatalf("Validate(%q) error = %v, want io.EOF", input, err)
				}
				return
			}
			var syntax *json.SyntaxError
			if !errors.As(err, &syntax) || syntax.Error() != "unexpected end of JSON input" {
				t.Fatalf("Validate(%q) error = %v, want truncated-input SyntaxError", input, err)
			}
		})
	}
}

func TestStructuralEqualityTypeAndShapeEdges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, left, right string
		want              bool
	}{
		{"null equal", `null`, `null`, true},
		{"null differs", `null`, `false`, false},
		{"boolean equal", `true`, `true`, true},
		{"boolean differs", `true`, `false`, false},
		{"boolean type differs", `true`, `1`, false},
		{"array type differs", `[]`, `{}`, false},
		{"array length differs", `[1]`, `[]`, false},
		{"nested array equal", `[[1],null]`, `[[1.0],null]`, true},
		{"object type differs", `{}`, `[]`, false},
		{"object length differs", `{"a":1}`, `{}`, false},
		{"object keys differ", `{"a":1}`, `{"b":1}`, false},
		{"object values differ", `{"a":1}`, `{"a":2}`, false},
		{"uppercase unicode", `"\uABCD"`, `"ꯍ"`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicaljson.Equal([]byte(test.left), []byte(test.right))
			if err != nil || got != test.want {
				t.Fatalf("Equal(%s, %s) = %t, %v, want %t, nil", test.left, test.right, got, err, test.want)
			}
		})
	}
}

func TestEqualityRejectsDuplicateRightOperand(t *testing.T) {
	t.Parallel()
	_, err := canonicaljson.Equal([]byte(`{}`), []byte(`{"a":1,"a":2}`))
	requireDuplicateMemberError(t, err, "a")
}

func TestMismatchedContainerClosers(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`[}`, `{]`} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			err := canonicaljson.Validate([]byte(input))
			var syntax *json.SyntaxError
			if !errors.As(err, &syntax) {
				t.Fatalf("Validate(%q) error = %v, want SyntaxError", input, err)
			}
		})
	}
}
