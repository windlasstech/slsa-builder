package npmprofile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestNodeXZRejectsForgedChunkExtents(t *testing.T) {
	t.Parallel()
	cases, plain := nodeXZForgedChunkPayloads(t)
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given valid compressed data with an enlarged extent and consistent XZ index.
			data := nodeXZChunkEnvelope(payload, plain)
			// When provisioning decodes the authenticated archive.
			_, err := provisionNode(t.Context(), t.TempDir(), nodeProvisionFixture(t, data))
			// Then actual consumption, not declared chunk boundaries, rejects the forgery.
			requireNodeProvisionID(t, err, "archive-unsafe")
		})
	}
}

func TestNodeXZChunkTransitions(t *testing.T) {
	t.Parallel()
	plain := bytes.Repeat([]byte("chunk-boundary-\x00"), 1024)
	compressed := nodeXZCompressedChunk(t, plain)
	uncompressed := nodeXZRawChunk(plain)
	propertiesReset := bytes.Clone(compressed)
	propertiesReset[0] = 0xc0
	stateReset := append(bytes.Clone(compressed[:5]), compressed[6:]...)
	stateReset[0] = 0xa0
	rawContinuation := bytes.Clone(uncompressed)
	rawContinuation[0] = 2
	continuing := nodeXZContinuingChunks(t, plain)
	for name, chunks := range map[string][][]byte{
		"compressed compressed":   {compressed, compressed},
		"compressed uncompressed": {compressed, uncompressed},
		"uncompressed compressed": {uncompressed, compressed},
		"properties reset":        {compressed, propertiesReset},
		"state reset":             {compressed, stateReset},
		"raw continuation":        {uncompressed, rawContinuation},
		"continuing compressed":   {continuing[:len(continuing)-1]},
	} {
		for _, size := range []int{1, len(plain), 1 << 16} {
			t.Run(fmt.Sprintf("%s/read-%d", name, size), func(t *testing.T) {
				t.Parallel()
				// Given two chunks, each producing more than the 4 KiB dictionary capacity.
				payload := append(bytes.Join(chunks, nil), 0)
				want := bytes.Repeat(plain, 2)
				r, err := newNodeXZReader(nodeXZChunkEnvelope(payload, want))
				if err != nil {
					t.Fatal(err)
				}
				var got bytes.Buffer
				buffer := make([]byte, size)
				// When caller reads span, exactly match, or subdivide the output boundaries.
				for {
					n, err := r.Read(buffer)
					got.Write(buffer[:n])
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					if n == 0 {
						t.Fatal("reader made no progress")
					}
				}

				// Then all output and explicit decoder EOS are verified.
				if !bytes.Equal(got.Bytes(), want) {
					t.Fatal("decoded bytes differ")
				}
				if err := r.verify(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestNodeXZChunkInvalidStates(t *testing.T) {
	t.Parallel()
	plain := bytes.Repeat([]byte("compress me"), 1024)
	chunk := nodeXZCompressedChunk(t, plain)
	for name, mutate := range map[string]func([]byte) []byte{
		"smaller compressed size":     func(p []byte) []byte { binary.BigEndian.PutUint16(p[3:5], binary.BigEndian.Uint16(p[3:5])-1); return p },
		"larger compressed size":      func(p []byte) []byte { binary.BigEndian.PutUint16(p[3:5], binary.BigEndian.Uint16(p[3:5])+2); return p },
		"smaller output size":         func(p []byte) []byte { p[2]--; return p },
		"larger output size":          func(p []byte) []byte { p[2]++; return p },
		"early EOS":                   func(p []byte) []byte { p[0] = 0; return p },
		"missing EOS":                 func(p []byte) []byte { return p[:len(p)-1] },
		"bytes after EOS":             func(p []byte) []byte { return append(p, 0) },
		"reserved control":            func(p []byte) []byte { p[0] = 3; return p },
		"missing dictionary reset":    func(p []byte) []byte { p[0] = 0xc0; return p },
		"missing properties":          func(p []byte) []byte { p[0] = 0x80; return append(p[:5], p[6:]...) },
		"invalid properties":          func(p []byte) []byte { p[5] = 0xff; return p },
		"truncated raw header":        func([]byte) []byte { return []byte{1, 0} },
		"truncated compressed header": func([]byte) []byte { return []byte{0xe0, 0, 0, 0, 0} },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given a malformed chunk sequence in an otherwise valid XZ envelope.
			payload := mutate(append(bytes.Clone(chunk), 0))
			// When the reader drains the payload and verifies its result.
			r, err := newNodeXZReader(nodeXZChunkEnvelope(payload, plain))
			if err == nil {
				_, err = io.Copy(io.Discard, r)
				if err == nil {
					err = r.verify()
				}
			}
			// Then no malformed sequence can be accepted.
			if err == nil {
				t.Fatal("accepted invalid chunk sequence")
			}
		})
	}
}

func TestNodeXZRejectsLZMA2PropertySumAboveFour(t *testing.T) {
	t.Parallel()
	t.Run("first chunk", func(t *testing.T) {
		t.Parallel()
		// Given an otherwise decodable chunk whose properties declare lc=5 and lp=0.
		payload := []byte{0xe0, 0, 0, 0, 5, 5, 0, 0, 0, 0, 0, 0, 0}
		r, err := newNodeXZReader(nodeXZChunkEnvelope(payload, []byte{0}))
		if err == nil {
			_, err = io.Copy(io.Discard, r)
			if err == nil {
				err = r.verify()
			}
		}
		// Then LZMA2's lc+lp<=4 restriction rejects it before acceptance.
		if err == nil {
			t.Fatal("accepted properties with lc+lp=5")
		}
	})
	t.Run("properties reset", func(t *testing.T) {
		t.Parallel()
		// Given a later properties-reset header carrying the same invalid combination.
		first := nodeXZRawChunk([]byte{0})
		reset := []byte{0xc0, 0, 0, 0, 5, 5, 0, 0, 0, 0, 0, 0, 0}
		payload := append(first, reset...)
		// When parsing the reset chunk directly.
		_, err := parseNodeXZChunk(payload, len(first))
		// Then the reset is rejected with the same archive-unsafe classification.
		if err == nil {
			t.Fatal("accepted properties-reset with lc+lp=5")
		}
	})
}
