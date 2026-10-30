package npmprofile

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"hash/crc32"
	"hash/crc64"
	"testing"
)

// Small uncompressed LZMA2 chunks keep adversarial metadata fixtures cheap.
type nodeXZFixture struct {
	header, payload, check, index []byte
	flags                         byte
}

func nodeXZCRC(data []byte) []byte {
	return binary.LittleEndian.AppendUint32(data, crc32.ChecksumIEEE(data))
}

func nodeXZRecord(unpadded, unpacked uint64) []byte {
	b := binary.AppendUvarint([]byte{0, 1}, unpadded)
	return binary.AppendUvarint(b, unpacked)
}

func newNodeXZFixture(t testing.TB, flags byte) nodeXZFixture {
	t.Helper()
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	for _, entry := range nodeRuntimeEntries() {
		h := entry.header
		h.Size = int64(len(entry.body))
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	data := raw.Bytes()
	if len(data) > 65536 {
		t.Fatal("fixture exceeds one raw chunk")
	}
	payload := []byte{1, byte((len(data) - 1) >> 8), byte(len(data) - 1)}
	payload = append(payload, data...)
	payload = append(payload, 0)
	var check []byte
	switch flags {
	case 0:
	case 1:
		check = binary.LittleEndian.AppendUint32(nil, crc32.ChecksumIEEE(data))
	case 4:
		check = binary.LittleEndian.AppendUint64(nil, crc64.Checksum(data, crc64.MakeTable(crc64.ECMA)))
	case 10:
		sum := sha256.Sum256(data)
		check = sum[:]
	default:
		t.Fatal("unsupported fixture check")
	}
	f := nodeXZFixture{header: []byte{2, 0, 0x21, 1, 0, 0, 0, 0}, payload: payload, check: check, flags: flags}
	f.index = nodeXZRecord(uint64(12+len(payload)+len(check)), uint64(len(data)))
	return f
}

func (f nodeXZFixture) block() []byte {
	b := nodeXZCRC(bytes.Clone(f.header))
	b = append(b, f.payload...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return append(b, f.check...)
}

func (f nodeXZFixture) encode() []byte {
	header := []byte{0xfd, '7', 'z', 'X', 'Z', 0, 0, f.flags}
	header = binary.LittleEndian.AppendUint32(header, crc32.ChecksumIEEE(header[6:]))
	b := append(header, f.block()...)
	index := bytes.Clone(f.index)
	for len(index)%4 != 0 {
		index = append(index, 0)
	}
	index = nodeXZCRC(index)
	b = append(b, index...)
	footer := binary.LittleEndian.AppendUint32(nil, uint32(len(index)/4-1))
	footer = append(footer, 0, f.flags)
	b = binary.LittleEndian.AppendUint32(b, crc32.ChecksumIEEE(footer))
	b = append(b, footer...)
	return append(b, 'Y', 'Z')
}

func nodeXZMultipleBlocks(t testing.TB, excessive bool) []byte {
	t.Helper()
	f := newNodeXZFixture(t, 4)
	first := f.block()
	if excessive {
		f.header[4] = 0x1d
	}
	second := f.block()
	record := bytes.Clone(f.index[2:])
	f.index = append([]byte{0, 2}, record...)
	f.index = append(f.index, record...)
	b := f.encode()
	result := append(bytes.Clone(b[:12]), first...)
	result = append(result, second...)
	return append(result, b[12+len(second):]...)
}
