package dependencyproof_test

import (
	"bytes"
	"net/http"
	"testing"

	jsoncanonicalizer "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/goccy/go-yaml/parser"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

type denyNetworkTransport struct{}

func (denyNetworkTransport) RoundTrip(*http.Request) (*http.Response, error) {
	panic("offline verifier construction attempted a network call")
}

func TestOfflineSigstoreVerifierConstructionPath(t *testing.T) {
	originalTransport := http.DefaultTransport
	http.DefaultTransport = denyNetworkTransport{}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	const pinnedRoot = `{
		"mediaType":"application/vnd.dev.sigstore.trustedroot+json;version=0.1",
		"tlogs":[],
		"certificateAuthorities":[],
		"ctlogs":[],
		"timestampAuthorities":[]
	}`

	trustedRoot, err := root.NewTrustedRootFromJSON([]byte(pinnedRoot))
	if err != nil {
		t.Fatalf("load local trusted root: %v", err)
	}

	_, err = verify.NewVerifier(
		trustedRoot,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		t.Fatalf("construct offline verifier: %v", err)
	}
}

func TestJCSCanonicalizesRFC8785Sample(t *testing.T) {
	input := []byte(`{
		"numbers": [333333333.33333329, 1E30, 4.50, 2e-3, 0.000000000000000000000000001],
		"string": "\u20ac$\u000F\nA'B\"\\\\\"/",
		"literals": [null, true, false]
	}`)
	want := []byte(`{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`)

	got, err := jsoncanonicalizer.Transform(input)
	if err != nil {
		t.Fatalf("canonicalize RFC 8785 sample: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("canonical form mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestYAMLParserLoadsWorkflowSyntax(t *testing.T) {
	workflow := []byte("name: proof\non:\n  workflow_call:\njobs: {}\n")

	if _, err := parser.ParseBytes(workflow, 0); err != nil {
		t.Fatalf("parse workflow YAML: %v", err)
	}
}
