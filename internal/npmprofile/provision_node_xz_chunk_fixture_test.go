package npmprofile

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ulikunitz/xz/lzma"
)

func nodeXZCompressedChunk(t testing.TB, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w, err := (lzma.Writer2Config{DictCap: 4096}).NewWriter2(&b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	p := b.Bytes()
	if len(p) < 7 || p[0] < 0xe0 || 6+int(binary.BigEndian.Uint16(p[3:5]))+1 != len(p)-1 {
		t.Fatal("fixture must contain exactly one genuinely compressed chunk")
	}
	return bytes.Clone(p[:len(p)-1])
}

func nodeXZRawChunk(data []byte) []byte {
	b := binary.BigEndian.AppendUint16([]byte{1}, uint16(len(data)-1))
	return append(b, data...)
}

func nodeXZContinuingChunks(t testing.TB, plain []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w, err := (lzma.Writer2Config{DictCap: 4096}).NewWriter2(&b)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := w.Write(plain); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	p := b.Bytes()
	second := 6 + int(binary.BigEndian.Uint16(p[3:5])) + 1
	if p[second] < 0x80 || p[second] >= 0xc0 {
		t.Fatal("expected five-byte continuation header")
	}
	return p
}

func nodeXZChunkEnvelope(payload, unpacked []byte) []byte {
	f := nodeXZFixture{
		header: []byte{2, 0, 0x21, 1, 0, 0, 0, 0}, payload: payload,
		index: nodeXZRecord(uint64(12+len(payload)), uint64(len(unpacked))),
	}
	return f.encode()
}

func nodeXZForgedChunkPayloads(t testing.TB) (map[string][]byte, []byte) {
	t.Helper()
	f := newNodeXZFixture(t, 0)
	plain := bytes.Clone(f.payload[3 : len(f.payload)-1])
	first := nodeXZCompressedChunk(t, plain[:len(plain)/2])
	second := nodeXZCompressedChunk(t, plain[len(plain)/2:])
	final := nodeXZCompressedChunk(t, plain)
	binary.BigEndian.PutUint16(final[3:5], binary.BigEndian.Uint16(final[3:5])+1)
	final = append(final, 0)
	binary.BigEndian.PutUint16(first[3:5], binary.BigEndian.Uint16(first[3:5])+uint16(len(second)))
	absorbed := append(first, second...)
	absorbed = append(absorbed, 0)
	// A declaration-only walk skips the second chunk and lands on a valid final EOS.
	if end := 6 + int(binary.BigEndian.Uint16(absorbed[3:5])) + 1; end != len(absorbed)-1 || absorbed[end] != 0 {
		t.Fatal("absorbed-chunk counterexample does not fool a declaration-only walk")
	}
	return map[string][]byte{"extent includes EOS": final, "extent absorbs next chunk": absorbed}, plain
}
