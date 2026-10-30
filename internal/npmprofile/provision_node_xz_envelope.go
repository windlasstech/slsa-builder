package npmprofile

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

type nodeXZEnvelope struct {
	payload, check []byte
	unpacked       uint64
	dictionary     int
	checkType      byte
}

// XZ VLIs are canonical, at most nine bytes, and limited to 63 bits.
func nodeXZVLI(data *[]byte) (uint64, bool) {
	var value uint64
	for i := 0; i < 9 && len(*data) > 0; i++ {
		b := (*data)[0]
		*data = (*data)[1:]
		value |= uint64(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return value, i == 0 || b != 0
		}
	}
	return 0, false
}

func nodeXZZeros(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

func nodeXZValidCRC(data []byte) bool {
	return len(data) >= 4 && crc32.ChecksumIEEE(data[:len(data)-4]) == binary.LittleEndian.Uint32(data[len(data)-4:])
}

func parseNodeXZEnvelope(data []byte) (nodeXZEnvelope, error) {
	bad := nodeProvisionFailure("archive-unsafe")
	if len(data) < 36 || !xz.ValidHeader(data[:12]) {
		return nodeXZEnvelope{}, bad
	}
	checkSize := 0
	switch data[7] {
	case xz.None:
	case xz.CRC32:
		checkSize = 4
	case xz.CRC64:
		checkSize = 8
	case xz.SHA256:
		checkSize = 32
	default:
		return nodeXZEnvelope{}, bad
	}
	footer := data[len(data)-12:]
	if !bytes.Equal(footer[10:], []byte("YZ")) || !bytes.Equal(footer[8:10], data[6:8]) ||
		binary.LittleEndian.Uint32(footer) != crc32.ChecksumIEEE(footer[4:10]) {
		return nodeXZEnvelope{}, bad
	}
	indexSize := (uint64(binary.LittleEndian.Uint32(footer[4:])) + 1) * 4
	// One indicator, one count, two 9-byte VLIs, alignment and CRC need at most 24 bytes.
	if indexSize < 8 || indexSize > 24 || indexSize > uint64(len(data)-24) {
		return nodeXZEnvelope{}, bad
	}
	indexStart := len(data) - 12 - int(indexSize)
	index := data[indexStart : len(data)-12]
	if index[0] != 0 || !nodeXZValidCRC(index) {
		return nodeXZEnvelope{}, bad
	}
	fields := index[1 : len(index)-4]
	count, ok := nodeXZVLI(&fields)
	if !ok || count != 1 {
		return nodeXZEnvelope{}, bad
	}
	unpadded, ok := nodeXZVLI(&fields)
	if !ok {
		return nodeXZEnvelope{}, bad
	}
	unpacked, ok := nodeXZVLI(&fields)
	if !ok || len(fields) > 3 || !nodeXZZeros(fields) {
		return nodeXZEnvelope{}, bad
	}
	// Bound before alignment/addition and before converting attacker sizes to int.
	if unpadded > uint64(indexStart-12) || (unpadded+3)&^uint64(3) != uint64(indexStart-12) {
		return nodeXZEnvelope{}, bad
	}
	headerSize := (int(data[12]) + 1) * 4
	if data[12] == 0 || headerSize > indexStart-12 || unpadded <= uint64(headerSize+checkSize) {
		return nodeXZEnvelope{}, bad
	}
	payloadSize := int(unpadded) - headerSize - checkSize
	header := data[12 : 12+headerSize]
	dictionary, err := parseNodeXZBlockHeader(header, uint64(payloadSize), unpacked)
	if err != nil {
		return nodeXZEnvelope{}, err
	}
	payloadEnd := 12 + headerSize + payloadSize
	checkStart := indexStart - checkSize
	if !nodeXZZeros(data[payloadEnd:checkStart]) {
		return nodeXZEnvelope{}, bad
	}
	return nodeXZEnvelope{
		payload: data[12+headerSize : payloadEnd], check: data[checkStart:indexStart],
		unpacked: unpacked, dictionary: dictionary, checkType: data[7],
	}, nil
}

func parseNodeXZBlockHeader(header []byte, payloadSize, unpacked uint64) (int, error) {
	bad := nodeProvisionFailure("archive-unsafe")
	if !nodeXZValidCRC(header) || header[1]&0x3f != 0 {
		return 0, bad
	}
	fields := header[2 : len(header)-4]
	for _, size := range []struct {
		flag byte
		want uint64
	}{{0x40, payloadSize}, {0x80, unpacked}} {
		if header[1]&size.flag != 0 {
			value, ok := nodeXZVLI(&fields)
			if !ok || value != size.want {
				return 0, bad
			}
		}
	}
	filter, ok := nodeXZVLI(&fields)
	if !ok || filter != 0x21 {
		return 0, bad
	}
	properties, ok := nodeXZVLI(&fields)
	if !ok || properties != 1 || len(fields) == 0 {
		return 0, bad
	}
	dictionary, err := lzma.DecodeDictCap(fields[0])
	if err != nil || dictionary > 64<<20 || !nodeXZZeros(fields[1:]) {
		return 0, bad
	}
	return int(dictionary), nil
}
