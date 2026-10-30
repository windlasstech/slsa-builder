package npmprofile

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

type nodeXZInput struct {
	data   []byte
	offset int
}

func (r *nodeXZInput) Read(p []byte) (int, error) {
	if r.offset == len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

type nodeXZChunk struct {
	end       int
	remaining int
	eos       bool
}

func parseNodeXZChunk(payload []byte, offset int) (nodeXZChunk, error) {
	bad := nodeProvisionFailure("archive-unsafe")
	if offset < 0 || offset >= len(payload) {
		return nodeXZChunk{}, bad
	}
	p := payload[offset:]
	control := p[0]
	headerSize := 0
	switch {
	case control == 0:
		if len(p) != 1 {
			return nodeXZChunk{}, bad
		}
		return nodeXZChunk{end: offset + 1, eos: true}, nil
	case control == 1 || control == 2:
		headerSize = 3
	case control >= 0x80 && control < 0xc0:
		headerSize = 5
	case control >= 0xc0:
		headerSize = 6
	default:
		return nodeXZChunk{}, bad
	}
	if len(p) < headerSize {
		return nodeXZChunk{}, bad
	}
	if control >= 0xc0 {
		properties, err := lzma.PropertiesForCode(p[5])
		if err != nil || properties.LC+properties.LP > 4 {
			return nodeXZChunk{}, bad
		}
	}
	unpacked := int(binary.BigEndian.Uint16(p[1:3])) + 1
	extent := unpacked
	if control >= 0x80 {
		unpacked += int(control&0x1f) << 16
		extent = int(binary.BigEndian.Uint16(p[3:5])) + 1
	}
	if extent > len(p)-headerSize {
		return nodeXZChunk{}, bad
	}
	return nodeXZChunk{end: offset + headerSize + extent, remaining: unpacked}, nil
}

func (r *nodeXZReader) readChunk(p []byte) (int, error) {
	if r.terminal != nil {
		return 0, r.terminal
	}
	if len(p) == 0 {
		return 0, nil
	}
	if r.chunk.eos {
		r.terminal = r.finishEOS()
		return 0, r.terminal
	}
	output := p[:min(len(p), r.chunk.remaining)]
	n, err := r.raw.Read(output)
	r.chunk.remaining -= n
	// Pinned lzma v0.5.17 implementation contract: ByteReader does not buffer input, and Reader2
	// returns a full output buffer before starting another chunk. Recheck both properties and the
	// forged-extent tests on decoder upgrades; they are not public io.Reader guarantees.
	if err != nil || n != len(output) {
		r.terminal = nodeProvisionFailure("archive-unsafe")
		return n, r.terminal
	}
	if r.chunk.remaining == 0 {
		// Check actual consumption before raw can interpret the next physical header.
		if r.payload.offset != r.chunk.end {
			r.terminal = nodeProvisionFailure("archive-unsafe")
			return n, r.terminal
		}
		r.chunk, r.terminal = parseNodeXZChunk(r.envelope.payload, r.payload.offset)
		if r.terminal == nil && r.chunk.eos {
			r.terminal = r.finishEOS()
		}
	}
	return n, r.terminal
}

func (r *nodeXZReader) finishEOS() error {
	var scratch [1]byte
	n, err := r.raw.Read(scratch[:])
	if n != 0 || !errors.Is(err, io.EOF) || !r.raw.EOS() || r.payload.offset != len(r.envelope.payload) {
		return nodeProvisionFailure("archive-unsafe")
	}
	return io.EOF
}
