package npmprofile

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

// TestReconcileDeclaredDigestFetchFailure pins the fail-closed download path:
// when a declared pnpm sha256 or sha384 requires the tarball bytes and the
// download fails, reconciliation errors instead of skipping verification.
func TestReconcileDeclaredDigestFetchFailure(t *testing.T) {
	t.Parallel()
	selection := ManagerSelection{
		Name:   ManagerPNPM,
		Digest: &DeclaredDigest{Algorithm: "sha256", Hex: strings.Repeat("a1", 32)},
	}
	fetcher := distributionFetcher(func(context.Context, string, int64, string) ([]byte, error) {
		return nil, errors.New("distribution endpoint unreachable")
	})
	err := reconcileDeclaredDigest(t.Context(), selection, "https://example.invalid/pnpm.tgz", strings.Repeat("c3", 64), nil, fetcher)
	if err == nil || !strings.Contains(err.Error(), "download pnpm distribution for declared digest reconciliation") {
		t.Fatalf("reconcileDeclaredDigest() error = %v, want the download-failure wrap", err)
	}
}

// TestReconcileDeclaredDigestReboundMismatch pins the evidence-rebinding
// check: when the downloaded tarball's SHA-512 disagrees with the acquisition
// evidence, reconciliation fails closed with the classified digest-mismatch
// error before the declared digest is even consulted.
func TestReconcileDeclaredDigestReboundMismatch(t *testing.T) {
	t.Parallel()
	selection := ManagerSelection{
		Name:   ManagerPNPM,
		Digest: &DeclaredDigest{Algorithm: "sha256", Hex: strings.Repeat("a1", 32)},
	}
	bundle := []byte("pnpm distribution bytes")
	authoritativeHash := digest.SumSHA512([]byte("different bytes")).String()
	fetcher := distributionFetcher(func(ctx context.Context, rawURL string, maximum int64, accept string) ([]byte, error) {
		if ctx == nil {
			t.Error("fetcher received a nil context")
		}
		if rawURL != "https://example.invalid/pnpm.tgz" || maximum != maxPNPMDistribution || accept != "application/octet-stream" {
			t.Errorf("fetcher called with (%q, %d, %q)", rawURL, maximum, accept)
		}
		return bundle, nil
	})
	err := reconcileDeclaredDigest(t.Context(), selection, "https://example.invalid/pnpm.tgz", authoritativeHash, nil, fetcher)
	if err == nil {
		t.Fatal("reconcileDeclaredDigest() error = nil, want the classified mismatch")
	}
	diagnosticError, ok := err.(interface{ DiagnosticID() string })
	if !ok || diagnosticError.DiagnosticID() != IDPackageManagerDigestMismatch {
		t.Fatalf("reconcileDeclaredDigest() error = %v, want %s", err, IDPackageManagerDigestMismatch)
	}
}

// TestReconcileDeclaredDigestReboundSuccess pins the happy path of the
// sha256/sha384 reconciliation: the downloaded bytes rebind to the
// acquisition evidence and the declared digest computes over those bytes.
func TestReconcileDeclaredDigestReboundSuccess(t *testing.T) {
	t.Parallel()
	bundle := []byte("pnpm distribution bytes")
	selection := ManagerSelection{
		Name:   ManagerPNPM,
		Digest: &DeclaredDigest{Algorithm: "sha256", Hex: digest.SumSHA256(bundle).String()},
	}
	fetcher := distributionFetcher(func(context.Context, string, int64, string) ([]byte, error) {
		return bundle, nil
	})
	authoritativeHash := digest.SumSHA512(bundle).String()
	if err := reconcileDeclaredDigest(t.Context(), selection, "https://example.invalid/pnpm.tgz", authoritativeHash, nil, fetcher); err != nil {
		t.Fatalf("reconcileDeclaredDigest() error = %v, want nil", err)
	}
}

// TestComputeDeclaredDigest pins the closed SRI algorithm set: sha256,
// sha384, and sha512 compute lowercase hex digests, and any other algorithm
// yields the empty fail-closed value.
func TestComputeDeclaredDigest(t *testing.T) {
	t.Parallel()
	data := []byte("distribution bytes")
	sha384Sum := sha512.Sum384(data)
	tests := []struct {
		name      string
		algorithm string
		want      string
	}{
		{name: "sha256", algorithm: "sha256", want: digest.SumSHA256(data).String()},
		{name: "sha384", algorithm: "sha384", want: hex.EncodeToString(sha384Sum[:])},
		{name: "sha512", algorithm: "sha512", want: digest.SumSHA512(data).String()},
		{name: "unknown algorithm", algorithm: "sha1", want: ""},
		{name: "empty algorithm", algorithm: "", want: ""},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := computeDeclaredDigest(test.algorithm, data); got != test.want {
				t.Fatalf("computeDeclaredDigest(%q) = %q, want %q", test.algorithm, got, test.want)
			}
		})
	}
}
