package npmprofile

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func TestNodeXZOptionalSizes(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"correct", "compressed mismatch", "uncompressed mismatch", "overflow", "truncated varint"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given both optional sizes in a CRC-valid block header.
			f := newNodeXZFixture(t, 4)
			compressed, unpacked := uint64(len(f.payload)), uint64(len(f.payload)-4)
			switch name {
			case "compressed mismatch":
				compressed++
			case "uncompressed mismatch":
				unpacked++
			case "overflow":
				compressed = 1 << 63
			}
			f.header = binary.AppendUvarint([]byte{0, 0xc0}, compressed)
			f.header = binary.AppendUvarint(f.header, unpacked)
			f.header = append(f.header, 0x21, 1, 0)
			for len(f.header)%4 != 0 {
				f.header = append(f.header, 0)
			}
			f.header[0] = byte(len(f.header) / 4)
			if name == "truncated varint" {
				for i := 2; i < len(f.header); i++ {
					f.header[i] = 0x80
				}
			}
			f.index = nodeXZRecord(uint64(len(f.header)+4+len(f.payload)+len(f.check)), uint64(len(f.payload)-4))
			// When extraction compares header sizes with the index and payload.
			_, err := provisionNode(t.Context(), t.TempDir(), nodeProvisionFixture(t, f.encode()))
			// Then only exact sizes are accepted.
			if name == "correct" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				requireNodeProvisionID(t, err, "archive-unsafe")
			}
		})
	}
}

func TestNodeXZEnvelopeFlagsAndPadding(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"stream flags", "unknown check", "footer flags", "block padding"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Given corrupt flags with repaired CRCs, or nonzero physical block padding.
			f := newNodeXZFixture(t, 4)
			if name == "block padding" {
				f.payload = append(f.payload, 0)
				f.index = nodeXZRecord(uint64(12+len(f.payload)+len(f.check)), uint64(len(f.payload)-5))
			}
			b := bytes.Clone(f.encode())
			switch name {
			case "stream flags":
				b[6] = 1
				binary.LittleEndian.PutUint32(b[8:], crc32.ChecksumIEEE(b[6:8]))
			case "unknown check":
				b[7] = 2
				binary.LittleEndian.PutUint32(b[8:], crc32.ChecksumIEEE(b[6:8]))
			case "footer flags":
				footer := b[len(b)-12:]
				footer[9] = 1
				binary.LittleEndian.PutUint32(footer, crc32.ChecksumIEEE(footer[4:10]))
			case "block padding":
				b[24+len(f.payload)] = 1
			}
			// When parsing the envelope before decoding any payload.
			_, err := parseNodeXZEnvelope(b)
			// Then inconsistent metadata is rejected independently of payload validity.
			requireNodeProvisionID(t, err, "archive-unsafe")
		})
	}
}
