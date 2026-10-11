package digest_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

// errStreamFailure is the sentinel error a failing reader returns mid-stream.
var errStreamFailure = errors.New("stream failure")

// failingReader serves one chunk of data and then fails with a sentinel error.
type failingReader struct {
	chunk []byte
}

func (reader *failingReader) Read(p []byte) (int, error) {
	if reader.chunk == nil {
		return 0, errStreamFailure
	}
	n := copy(p, reader.chunk)
	reader.chunk = reader.chunk[n:]
	if len(reader.chunk) == 0 {
		reader.chunk = nil
	}
	return n, nil
}

// largePayload returns a payload bigger than the io.Copy buffer, forcing multi-chunk reads.
func largePayload() []byte {
	return bytes.Repeat([]byte("slsa-builder-reader-payload\n"), 4096)
}

func TestSumSHA256Reader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload []byte
		wantHex string
	}{
		{
			name:    "empty payload",
			payload: nil,
			wantHex: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:    "short ASCII",
			payload: []byte("abc"),
			wantHex: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name:    "multi-chunk large payload",
			payload: largePayload(),
			wantHex: digest.SumSHA256(largePayload()).String(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := digest.SumSHA256Reader(bytes.NewReader(test.payload))
			if err != nil {
				t.Fatalf("SumSHA256Reader() error = %v", err)
			}
			if got.String() != test.wantHex {
				t.Fatalf("SumSHA256Reader() = %q, want %q", got, test.wantHex)
			}
			if want := digest.SumSHA256(test.payload); !got.Equal(want) {
				t.Fatalf("SumSHA256Reader() = %q, slice variant = %q", got, want)
			}
		})
	}

	t.Run("reader error", func(t *testing.T) {
		t.Parallel()

		got, err := digest.SumSHA256Reader(&failingReader{chunk: []byte("partial payload")})
		if err == nil {
			t.Fatal("SumSHA256Reader() unexpectedly succeeded")
		}
		if !errors.Is(err, errStreamFailure) {
			t.Fatalf("SumSHA256Reader() error = %v, want wrapped stream failure", err)
		}
		if !strings.HasPrefix(err.Error(), "compute SHA-256: ") {
			t.Fatalf("SumSHA256Reader() error = %q, want compute SHA-256 wrap", err)
		}
		if got != (digest.SHA256{}) {
			t.Fatalf("SumSHA256Reader() = %q, want zero digest on error", got)
		}
	})
}

func TestSumSHA512Reader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload []byte
		wantHex string
	}{
		{
			name:    "empty payload",
			payload: nil,
			wantHex: "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce" +
				"47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
		},
		{
			name:    "short ASCII",
			payload: []byte("abc"),
			wantHex: "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
				"2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
		},
		{
			name:    "multi-chunk large payload",
			payload: largePayload(),
			wantHex: digest.SumSHA512(largePayload()).String(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := digest.SumSHA512Reader(bytes.NewReader(test.payload))
			if err != nil {
				t.Fatalf("SumSHA512Reader() error = %v", err)
			}
			if got.String() != test.wantHex {
				t.Fatalf("SumSHA512Reader() = %q, want %q", got, test.wantHex)
			}
			if want := digest.SumSHA512(test.payload); !got.Equal(want) {
				t.Fatalf("SumSHA512Reader() = %q, slice variant = %q", got, want)
			}
		})
	}

	t.Run("reader error", func(t *testing.T) {
		t.Parallel()

		got, err := digest.SumSHA512Reader(&failingReader{chunk: []byte("partial payload")})
		if err == nil {
			t.Fatal("SumSHA512Reader() unexpectedly succeeded")
		}
		if !errors.Is(err, errStreamFailure) {
			t.Fatalf("SumSHA512Reader() error = %v, want wrapped stream failure", err)
		}
		if !strings.HasPrefix(err.Error(), "compute SHA-512: ") {
			t.Fatalf("SumSHA512Reader() error = %q, want compute SHA-512 wrap", err)
		}
		if got != (digest.SHA512{}) {
			t.Fatalf("SumSHA512Reader() = %q, want zero digest on error", got)
		}
	})
}
