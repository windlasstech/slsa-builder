package digest_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

type encodingFixture struct {
	SHA256        string   `json:"sha256"`
	SHA512        string   `json:"sha512"`
	InvalidSHA256 []string `json:"invalid_sha256"`
	InvalidSHA512 []string `json:"invalid_sha512"`
}

func TestDigestEncoding(t *testing.T) {
	fixtureBytes, err := os.ReadFile("../../testdata/handoff/malformed-digests.json")
	if err != nil {
		t.Fatalf("read digest fixture: %v", err)
	}

	var fixture encodingFixture
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		t.Fatalf("decode digest fixture: %v", err)
	}

	t.Run("sha256 round trip", func(t *testing.T) {
		got, err := digest.ParseSHA256(fixture.SHA256)
		if err != nil {
			t.Fatalf("parse SHA-256: %v", err)
		}
		if got.String() != fixture.SHA256 {
			t.Fatalf("SHA-256 string = %q, want %q", got, fixture.SHA256)
		}
		assertJSONEncoding(t, got, fixture.SHA256)
		if !got.Equal(digest.SumSHA256([]byte("trusted payload\n"))) {
			t.Fatal("parsed and computed SHA-256 values differ")
		}
	})

	t.Run("sha512 round trip", func(t *testing.T) {
		got, err := digest.ParseSHA512(fixture.SHA512)
		if err != nil {
			t.Fatalf("parse SHA-512: %v", err)
		}
		if got.String() != fixture.SHA512 {
			t.Fatalf("SHA-512 string = %q, want %q", got, fixture.SHA512)
		}
		assertJSONEncoding(t, got, fixture.SHA512)
		if !got.Equal(digest.SumSHA512([]byte("trusted payload\n"))) {
			t.Fatal("parsed and computed SHA-512 values differ")
		}
	})

	for _, encoded := range fixture.InvalidSHA256 {
		t.Run("reject sha256 "+encoded, func(t *testing.T) {
			if _, err := digest.ParseSHA256(encoded); err == nil {
				t.Fatalf("ParseSHA256(%q) unexpectedly succeeded", encoded)
			}
		})
	}
	for _, encoded := range fixture.InvalidSHA512 {
		t.Run("reject sha512 "+encoded, func(t *testing.T) {
			if _, err := digest.ParseSHA512(encoded); err == nil {
				t.Fatalf("ParseSHA512(%q) unexpectedly succeeded", encoded)
			}
		})
	}
}

func assertJSONEncoding(t *testing.T, value any, want string) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal digest: %v", err)
	}
	if string(encoded) != `"`+want+`"` {
		t.Fatalf("JSON encoding = %s, want quoted lowercase hex", encoded)
	}
}
