package npmprofile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"testing"
)

func TestNodeXZAcceptedEnvelope(t *testing.T) {
	t.Parallel()
	for _, flags := range []byte{0, 1, 4, 10} {
		t.Run(fmt.Sprint(flags), func(t *testing.T) {
			t.Parallel()
			// Given each supported check, including CRC64 with the maximum dictionary.
			f := newNodeXZFixture(t, flags)
			if flags == 4 {
				f.header[4] = 0x1c
			}
			// When the full provisioning boundary extracts the archive.
			_, err := provisionNode(t.Context(), t.TempDir(), nodeProvisionFixture(t, f.encode()))
			// Then the envelope and official layout are accepted.
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func nodeXZInvalidEnvelopes(t testing.TB) map[string][]byte {
	t.Helper()
	f := newNodeXZFixture(t, 4)
	valid := f.encode()
	cases := map[string][]byte{
		"two blocks":                        nodeXZMultipleBlocks(t, false),
		"safe first excessive second block": nodeXZMultipleBlocks(t, true),
		"two streams":                       append(bytes.Clone(valid), valid...),
		"stream padding":                    append(bytes.Clone(valid), 0, 0, 0, 0),
		"trailing byte":                     append(bytes.Clone(valid), 1),
	}
	forged, plain := nodeXZForgedChunkPayloads(t)
	for name, payload := range forged {
		cases[name] = nodeXZChunkEnvelope(payload, plain)
	}
	for name, mutate := range map[string]func(*nodeXZFixture){
		"dictionary 1d":         func(f *nodeXZFixture) { f.header[4] = 0x1d },
		"invalid dictionary":    func(f *nodeXZFixture) { f.header[4] = 0xff },
		"maximum dictionary":    func(f *nodeXZFixture) { f.header[4] = 0x28 },
		"multiple filters":      func(f *nodeXZFixture) { f.header[1] = 1 },
		"reserved block flags":  func(f *nodeXZFixture) { f.header[1] = 4 },
		"wrong filter":          func(f *nodeXZFixture) { f.header[2] = 0x20 },
		"wrong property length": func(f *nodeXZFixture) { f.header[3] = 2 },
		"header padding":        func(f *nodeXZFixture) { f.header[7] = 1 },
		"zero records":          func(f *nodeXZFixture) { f.index[1] = 0 },
		"wrong unpadded":        func(f *nodeXZFixture) { f.index[2] ^= 1 },
		"unpadded underflow":    func(f *nodeXZFixture) { f.index = nodeXZRecord(1, 0) },
		"unpadded overflow":     func(f *nodeXZFixture) { f.index = nodeXZRecord(1<<63-1, 0) },
		"varint overflow":       func(f *nodeXZFixture) { f.index = nodeXZRecord(1<<63, 0) },
		"overlong varint":       func(f *nodeXZFixture) { f.index = append([]byte{0, 0x81, 0}, f.index[2:]...) },
		"wrong uncompressed":    func(f *nodeXZFixture) { f.index = nodeXZRecord(uint64(12+len(f.payload)+len(f.check)), 1) },
		"extra index padding":   func(f *nodeXZFixture) { f.index = append(f.index, 0, 0, 0, 0) },
		"index padding":         func(f *nodeXZFixture) { f.index = append(f.index, 1) },
		"early termination":     func(f *nodeXZFixture) { f.payload[0] = 0 },
		"unused payload": func(f *nodeXZFixture) {
			f.payload = append(f.payload, 0)
			f.index = nodeXZRecord(uint64(12+len(f.payload)+len(f.check)), uint64(len(f.payload)-5))
		},
		"missing EOS": func(f *nodeXZFixture) {
			f.payload = f.payload[:len(f.payload)-1]
			f.index = nodeXZRecord(uint64(12+len(f.payload)+len(f.check)), uint64(len(f.payload)-3))
		},
		"block check": func(f *nodeXZFixture) { f.check[0] ^= 1 },
	} {
		f := newNodeXZFixture(t, 4)
		mutate(&f)
		cases[name] = f.encode()
	}
	for name, offset := range map[string]int{
		"stream CRC": 8, "block CRC": 20, "index CRC": len(valid) - 13,
		"footer CRC": len(valid) - 12, "footer magic": len(valid) - 1,
	} {
		b := bytes.Clone(valid)
		b[offset] ^= 1
		cases[name] = b
	}
	for name, value := range map[string]uint32{"backward oversized": 0xffffffff, "backward zero": 0} {
		b := bytes.Clone(valid)
		footer := b[len(b)-12:]
		binary.LittleEndian.PutUint32(footer[4:], value)
		binary.LittleEndian.PutUint32(footer, crc32.ChecksumIEEE(footer[4:10]))
		cases[name] = b
	}
	second := newNodeXZFixture(t, 4)
	second.header[4] = 0x1d
	cases["safe first excessive second stream"] = append(bytes.Clone(valid), second.encode()...)
	for _, n := range []int{0, 1, 11, 12, 23, len(valid) - 13, len(valid) - 1} {
		cases[fmt.Sprintf("truncated %d", n)] = valid[:n]
	}
	return cases
}

func TestNodeXZRejectsEnvelope(t *testing.T) {
	t.Parallel()
	for name, data := range nodeXZInvalidEnvelopes(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given a small archive with one malformed field, repairing metadata CRCs.
			source := nodeProvisionFixture(t, data)
			// When the authenticated archive reaches provisioning.
			_, err := provisionNode(t.Context(), t.TempDir(), source)
			// Then it fails closed with the existing archive diagnostic.
			requireNodeProvisionID(t, err, "archive-unsafe")
		})
	}
}
