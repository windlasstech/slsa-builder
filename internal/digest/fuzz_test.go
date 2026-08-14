package digest_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

// FuzzParseSHA256 exercises ParseSHA256 with property-based invariants: on
// accept the value round-trips through String and MarshalText/UnmarshalText,
// and the accepted string is exact-length lowercase hexadecimal; on reject the
// error is always classified as ErrInvalidEncoding.
func FuzzParseSHA256(f *testing.F) {
	fixture := readDigestFixture(f)

	f.Add(fixture.SHA256)
	f.Add(digest.SumSHA256([]byte("trusted payload\n")).String())
	for _, encoded := range fixture.InvalidSHA256 {
		f.Add(encoded)
	}
	f.Add("")
	f.Add("8458D7A633B4CB9F781D4AFBB11ABC6BE59F135C4B7798F7B9421E776683350E")
	f.Add("8458d7a633b4cb9f781d4afbb11abc6be59f135c4b7798f7b9421e776683350")
	f.Add("g458d7a633b4cb9f781d4afbb11abc6be59f135c4b7798f7b9421e776683350e")

	f.Fuzz(func(t *testing.T, value string) {
		parsed, err := digest.ParseSHA256(value)
		if err != nil {
			if !errors.Is(err, digest.ErrInvalidEncoding) {
				t.Fatalf("ParseSHA256(%q) rejected with unclassified error: %v", value, err)
			}
			return
		}

		if !isLowerHex(value, 64) {
			t.Fatalf("ParseSHA256 accepted non-canonical encoding %q", value)
		}
		if parsed.String() != value {
			t.Fatalf("ParseSHA256(%q).String() = %q, want canonical identity", value, parsed.String())
		}

		reparsed, err := digest.ParseSHA256(parsed.String())
		if err != nil {
			t.Fatalf("re-parse of String() output %q failed: %v", parsed.String(), err)
		}
		if !parsed.Equal(reparsed) {
			t.Fatalf("String() round trip changed the value: %q became %q", value, reparsed.String())
		}

		text, err := parsed.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed for accepted value %q: %v", value, err)
		}
		var unmarshaled digest.SHA256
		if err := unmarshaled.UnmarshalText(text); err != nil {
			t.Fatalf("UnmarshalText(%q) failed: %v", text, err)
		}
		if !parsed.Equal(unmarshaled) {
			t.Fatalf("text round trip changed the value: %q became %q", value, unmarshaled.String())
		}
	})
}

// FuzzParseSHA512 exercises ParseSHA512 with the same invariants as
// FuzzParseSHA256 over the 128-character SHA-512 encoding.
func FuzzParseSHA512(f *testing.F) {
	fixture := readDigestFixture(f)

	f.Add(fixture.SHA512)
	f.Add(digest.SumSHA512([]byte("trusted payload\n")).String())
	for _, encoded := range fixture.InvalidSHA512 {
		f.Add(encoded)
	}
	f.Add("")
	f.Add("466A72510EDEB9C753C0E0950AE2BFF687963B24AA16E86F0670A510D512102F9FFDA10AD4169BF164C145D30F4D38180155070F287C62E841C5CAE602D5E562")
	f.Add("466a72510edeb9c753c0e0950ae2bff687963b24aa16e86f0670a510d512102f9ffda10ad4169bf164c145d30f4d38180155070f287c62e841c5cae602d5e56")
	f.Add("z66a72510edeb9c753c0e0950ae2bff687963b24aa16e86f0670a510d512102f9ffda10ad4169bf164c145d30f4d38180155070f287c62e841c5cae602d5e562")

	f.Fuzz(func(t *testing.T, value string) {
		parsed, err := digest.ParseSHA512(value)
		if err != nil {
			if !errors.Is(err, digest.ErrInvalidEncoding) {
				t.Fatalf("ParseSHA512(%q) rejected with unclassified error: %v", value, err)
			}
			return
		}

		if !isLowerHex(value, 128) {
			t.Fatalf("ParseSHA512 accepted non-canonical encoding %q", value)
		}
		if parsed.String() != value {
			t.Fatalf("ParseSHA512(%q).String() = %q, want canonical identity", value, parsed.String())
		}

		reparsed, err := digest.ParseSHA512(parsed.String())
		if err != nil {
			t.Fatalf("re-parse of String() output %q failed: %v", parsed.String(), err)
		}
		if !parsed.Equal(reparsed) {
			t.Fatalf("String() round trip changed the value: %q became %q", value, reparsed.String())
		}

		text, err := parsed.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed for accepted value %q: %v", value, err)
		}
		var unmarshaled digest.SHA512
		if err := unmarshaled.UnmarshalText(text); err != nil {
			t.Fatalf("UnmarshalText(%q) failed: %v", text, err)
		}
		if !parsed.Equal(unmarshaled) {
			t.Fatalf("text round trip changed the value: %q became %q", value, unmarshaled.String())
		}
	})
}

// readDigestFixture loads the malformed-digest fixture used for seed values.
func readDigestFixture(f *testing.F) encodingFixture {
	f.Helper()

	fixtureBytes, err := os.ReadFile("../../testdata/handoff/malformed-digests.json")
	if err != nil {
		f.Fatalf("read digest fixture: %v", err)
	}

	var fixture encodingFixture
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		f.Fatalf("decode digest fixture: %v", err)
	}
	return fixture
}

// isLowerHex reports whether value is exactly length characters of lowercase
// hexadecimal.
func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range []byte(value) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
