package npmprofile

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"hash"
	"hash/crc32"
	"hash/crc64"
	"io"
	"slices"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

type nodeXZReader struct {
	envelope nodeXZEnvelope
	payload  *nodeXZInput
	raw      *lzma.Reader2
	hash     hash.Hash
	count    uint64
	chunk    nodeXZChunk
	terminal error
}

func newNodeXZReader(encoded []byte) (*nodeXZReader, error) {
	envelope, err := parseNodeXZEnvelope(encoded)
	if err != nil {
		return nil, err
	}
	chunk, err := parseNodeXZChunk(envelope.payload, 0)
	if err != nil {
		return nil, err
	}
	payload := &nodeXZInput{data: envelope.payload}
	raw, err := (lzma.Reader2Config{DictCap: envelope.dictionary}).NewReader2(payload)
	if err != nil {
		return nil, nodeProvisionFailure("archive-unsafe")
	}
	r := &nodeXZReader{envelope: envelope, payload: payload, raw: raw, chunk: chunk}
	switch envelope.checkType {
	case xz.None:
	case xz.CRC32:
		r.hash = crc32.NewIEEE()
	case xz.CRC64:
		r.hash = crc64.New(crc64.MakeTable(crc64.ECMA))
	case xz.SHA256:
		r.hash = sha256.New()
	}
	return r, nil
}

func (r *nodeXZReader) Read(p []byte) (int, error) {
	n, err := r.readChunk(p)
	r.count += uint64(n)
	if r.hash != nil {
		if _, hashErr := r.hash.Write(p[:n]); hashErr != nil {
			return n, hashErr
		}
	}
	return n, err
}

func (r *nodeXZReader) verify() error {
	if !errors.Is(r.terminal, io.EOF) || !r.raw.EOS() || r.payload.offset != len(r.envelope.payload) || r.count != r.envelope.unpacked {
		return nodeProvisionFailure("archive-unsafe")
	}
	if r.hash != nil {
		sum := r.hash.Sum(nil)
		switch r.envelope.checkType {
		case xz.CRC32, xz.CRC64:
			slices.Reverse(sum)
		case xz.SHA256:
		}
		if !bytes.Equal(sum, r.envelope.check) {
			return nodeProvisionFailure("archive-unsafe")
		}
	}
	return nil
}
