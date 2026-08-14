package canonicaljson_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

func TestDuplicateMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		member string
	}{
		{name: "top level", input: `{"predicate":{},"predicate":{}}`, member: "predicate"},
		{name: "nested object", input: `{"predicate":{"buildType":"first","buildType":"second"}}`, member: "buildType"},
		{name: "object in array", input: `[{"digest":"first","digest":"second"}]`, member: "digest"},
		{name: "extension member", input: `{"extension":{"future":1,"future":2}}`, member: "future"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := canonicaljson.Validate([]byte(test.input))
			requireDuplicateMemberError(t, err, test.member)

			_, err = canonicaljson.Canonicalize([]byte(test.input))
			requireDuplicateMemberError(t, err, test.member)
		})
	}

	if err := canonicaljson.Validate([]byte(`[{"name":1},{"name":2}]`)); err != nil {
		t.Fatalf("members in distinct objects must not conflict: %v", err)
	}
}

func TestEscapedDuplicateMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		member string
	}{
		{name: "ASCII Unicode escape", input: `{"a":1,"\u0061":2}`, member: "a"},
		{name: "escaped then literal", input: `{"\u0061":1,"a":2}`, member: "a"},
		{name: "control escape", input: `{"\n":1,"\u000a":2}`, member: "\n"},
		{name: "surrogate pair", input: `{"😂":1,"\ud83d\ude02":2}`, member: "😂"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requireDuplicateMemberError(t, canonicaljson.Validate([]byte(test.input)), test.member)
		})
	}
}

func TestJCSVectors(t *testing.T) {
	t.Parallel()

	vectorRoot := filepath.Join("..", "..", "testdata", "canonicaljson")
	inputDir := filepath.Join(vectorRoot, "input")
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		t.Fatalf("read JCS vector directory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("JCS vector directory is empty")
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			input, err := os.ReadFile(filepath.Join(inputDir, entry.Name()))
			if err != nil {
				t.Fatalf("read JCS input: %v", err)
			}
			want, err := os.ReadFile(filepath.Join(vectorRoot, "output", entry.Name()))
			if err != nil {
				t.Fatalf("read JCS output: %v", err)
			}
			want = bytes.TrimSpace(want)

			got, err := canonicaljson.Canonicalize(input)
			if err != nil {
				t.Fatalf("canonicalize JCS input: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("canonical form mismatch:\n got: %s\nwant: %s", got, want)
			}

			recycled, err := canonicaljson.Canonicalize(got)
			if err != nil {
				t.Fatalf("canonicalize canonical output: %v", err)
			}
			if !bytes.Equal(recycled, got) {
				t.Fatalf("canonicalization is not idempotent:\n first: %s\nsecond: %s", got, recycled)
			}
		})
	}
}

func TestStrictClosedValueParsing(t *testing.T) {
	t.Parallel()

	valid := [][]byte{
		[]byte("null"),
		[]byte("true"),
		[]byte(`"statement"`),
		[]byte("42.0"),
		[]byte("[1,2,3]"),
		[]byte(" {\"predicate\": {}} \n"),
	}
	for _, input := range valid {
		if err := canonicaljson.Validate(input); err != nil {
			t.Errorf("Validate(%q) returned an error: %v", input, err)
		}
	}

	invalid := [][]byte{
		nil,
		[]byte("null false"),
		[]byte("{}{}"),
		[]byte(`"\ud800"`),
		[]byte("1e400"),
		{0xff},
	}
	for _, input := range invalid {
		if err := canonicaljson.Validate(input); err == nil {
			t.Errorf("Validate(%q) unexpectedly succeeded", input)
		}
	}
}

func TestNestingDepthLimit(t *testing.T) {
	t.Parallel()

	const excessiveDepth = 10_001
	input := make([]byte, 0, excessiveDepth*2)
	input = append(input, bytes.Repeat([]byte{'['}, excessiveDepth)...)
	input = append(input, bytes.Repeat([]byte{']'}, excessiveDepth)...)
	if err := canonicaljson.Validate(input); err == nil {
		t.Fatalf("Validate() accepted JSON nested %d levels deep", excessiveDepth)
	}
}

func TestStructuralEquality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{name: "object member order", left: `{"a":1,"b":2}`, right: `{"b":2,"a":1}`, want: true},
		{name: "number spelling", left: `{"value":1}`, right: `{"value":1.0}`, want: true},
		{name: "string escape", left: `{"value":"a"}`, right: `{"value":"\u0061"}`, want: true},
		{name: "array order", left: `[1,2]`, right: `[2,1]`, want: false},
		{name: "different types", left: `1`, right: `"1"`, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := canonicaljson.Equal([]byte(test.left), []byte(test.right))
			if err != nil {
				t.Fatalf("compare JSON values: %v", err)
			}
			if got != test.want {
				t.Fatalf("Equal() = %t, want %t", got, test.want)
			}
		})
	}

	_, err := canonicaljson.Equal([]byte(`{"a":1,"a":2}`), []byte(`{"a":2}`))
	requireDuplicateMemberError(t, err, "a")
}

func TestCanonicalizePrimitiveValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "null", want: "null"},
		{input: "true", want: "true"},
		{input: `"\u0061"`, want: `"a"`},
		{input: "1.0", want: "1"},
	}

	for _, test := range tests {
		got, err := canonicaljson.Canonicalize([]byte(test.input))
		if err != nil {
			t.Fatalf("Canonicalize(%q): %v", test.input, err)
		}
		if string(got) != test.want {
			t.Fatalf("Canonicalize(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func FuzzStrictJSON(f *testing.F) {
	f.Add([]byte("null false"))
	f.Add([]byte("{}{}"))
	f.Add([]byte(`"\ud800"`))
	f.Add([]byte("1e400"))
	f.Add([]byte{0xff})
	f.Add([]byte(`{"predicate":{},"predicate":{}}`))
	f.Add([]byte(`{"predicate":{"buildType":"first","buildType":"second"}}`))
	f.Add([]byte(`[{"digest":"first","digest":"second"}]`))
	f.Add([]byte(`{"extension":{"future":1,"future":2}}`))
	f.Add([]byte(`{"a":1,"\u0061":2}`))
	f.Add([]byte(`{"\u0061":1,"a":2}`))
	f.Add([]byte(`{"\n":1,"\u000a":2}`))
	f.Add([]byte(`{"😂":1,"\ud83d\ude02":2}`))
	f.Add([]byte(`{"predicate":{"builder":{"id":"trusted"}}}`))
	f.Add([]byte("[1,2,3]"))

	f.Fuzz(func(t *testing.T, input []byte) {
		if err := canonicaljson.Validate(input); err == nil {
			canonical, err := canonicaljson.Canonicalize(input)
			if err != nil {
				t.Fatalf("valid input failed canonicalization: %v", err)
			}
			if err := canonicaljson.Validate(canonical); err != nil {
				t.Fatalf("canonical output is not strict JSON: %v", err)
			}
			equal, err := canonicaljson.Equal(input, canonical)
			if err != nil {
				t.Fatalf("compare input with canonical output: %v", err)
			}
			if !equal {
				t.Fatal("canonicalization changed the parsed JSON value")
			}
		}

		encodedKey, err := json.Marshal(string(input))
		if err != nil {
			t.Fatalf("encode fuzzed object member name: %v", err)
		}
		var normalizedKey string
		if err := json.Unmarshal(encodedKey, &normalizedKey); err != nil {
			t.Fatalf("decode fuzzed object member name: %v", err)
		}
		duplicate := make([]byte, 0, len(encodedKey)*2+9)
		duplicate = append(duplicate, '{')
		duplicate = append(duplicate, encodedKey...)
		duplicate = append(duplicate, ':', '0', ',')
		duplicate = append(duplicate, encodedKey...)
		duplicate = append(duplicate, ':', '1', '}')

		requireDuplicateMemberError(t, canonicaljson.Validate(duplicate), normalizedKey)
		_, err = canonicaljson.Canonicalize(duplicate)
		requireDuplicateMemberError(t, err, normalizedKey)
	})
}

func requireDuplicateMemberError(t *testing.T, err error, member string) {
	t.Helper()

	var duplicate *canonicaljson.DuplicateMemberError
	if !errors.As(err, &duplicate) {
		t.Fatalf("error %v is not a DuplicateMemberError", err)
	}
	if duplicate.Member != member {
		t.Fatalf("duplicate member = %q, want %q", duplicate.Member, member)
	}
	if duplicate.DiagnosticID() != canonicaljson.DuplicateJSONMemberID {
		t.Fatalf("diagnostic ID = %q, want %q", duplicate.DiagnosticID(), canonicaljson.DuplicateJSONMemberID)
	}
}
