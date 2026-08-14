package canonicaljson_test

import (
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

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
