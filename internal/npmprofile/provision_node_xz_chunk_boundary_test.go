package npmprofile

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestNodeXZExactOutputBoundaryDrivesEOS(t *testing.T) {
	t.Parallel()
	// Given a final compressed chunk and a buffer exactly its declared output size.
	plain := bytes.Repeat([]byte("exact boundary"), 1024)
	payload := append(nodeXZCompressedChunk(t, plain), 0)
	r, err := newNodeXZReader(nodeXZChunkEnvelope(payload, plain))
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(plain))
	// When that single read finishes the declared output.
	n, err := r.Read(got)
	// Then EOS was explicitly consumed without another caller read.
	if n != len(plain) || !errors.Is(err, io.EOF) || !bytes.Equal(got, plain) {
		t.Fatalf("read = %d, %v", n, err)
	}
	if err := r.verify(); err != nil {
		t.Fatal(err)
	}
}

func TestNodeXZChunkFailureIsTerminal(t *testing.T) {
	t.Parallel()
	// Given a forged first extent, stopped at its output boundary.
	cases, plain := nodeXZForgedChunkPayloads(t)
	r, err := newNodeXZReader(nodeXZChunkEnvelope(cases["extent absorbs next chunk"], plain))
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, len(plain))
	if _, err := r.Read(buffer); err == nil {
		t.Fatal("forged extent was accepted")
	}
	// When a caller retries after the fatal failure.
	n, err := r.Read(buffer)
	// Then no following chunk is decoded.
	if n != 0 {
		t.Fatalf("decoded %d bytes after fatal error", n)
	}
	requireNodeProvisionID(t, err, "archive-unsafe")
}
