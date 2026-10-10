package npmprofile

// Declared descriptor digest reconciliation (ADR 0092). When the selection
// carries a declared integrity digest, the acquired distribution bytes are
// verified against it before install; any disagreement fails closed with the
// classified windlass.verify.error.package-manager-digest-mismatch error.

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

// packageManagerDigestMismatchError is the classified ADR 0092 fail-closed
// error, mirroring the npmProvenanceValidationError pattern. The message
// names only the manager and algorithm, never distribution bytes or secrets.
type packageManagerDigestMismatchError struct {
	message string
}

func (err *packageManagerDigestMismatchError) Error() string { return err.message }

func (err *packageManagerDigestMismatchError) DiagnosticID() string {
	return IDPackageManagerDigestMismatch
}

func digestMismatch(manager Manager, algorithm, detail string) error {
	return &packageManagerDigestMismatchError{
		message: fmt.Sprintf("%s distribution %s digest %s", manager, algorithm, detail),
	}
}

// reconcileDeclaredDigest verifies the acquired distribution against the
// selection's declared digest (ADR 0092). authoritativeHash is the captured
// source-native SHA-512 evidence (registry integrity for pnpm, download hash
// for Yarn) already proven equal to the Corepack hash; bundle carries the
// Yarn distribution bytes downloaded for that evidence.
func reconcileDeclaredDigest(ctx context.Context, selection ManagerSelection, distributionURL, authoritativeHash string, bundle []byte, fetcher distributionFetcher) error {
	declared := selection.Digest
	if declared == nil {
		return nil
	}
	if selection.Name == ManagerPNPM && declared.Algorithm == "sha512" {
		// The registry integrity evidence already binds the exact tarball
		// bytes, so a declared sha512 reconciles without a second download.
		if declared.Hex != authoritativeHash {
			return digestMismatch(selection.Name, declared.Algorithm, "disagrees with the declared descriptor digest")
		}
		return nil
	}
	bytes := bundle
	if selection.Name == ManagerPNPM {
		// A declared sha256 or sha384 cannot derive from the SHA-512 registry
		// evidence, so the tarball bytes are downloaded and rebound to it.
		downloaded, err := fetcher(ctx, distributionURL, maxPNPMDistribution, "application/octet-stream")
		if err != nil {
			return fmt.Errorf("download pnpm distribution for declared digest reconciliation: %w", err)
		}
		if digest.SumSHA512(downloaded).String() != authoritativeHash {
			return digestMismatch(selection.Name, "sha512", "computed over the downloaded bytes disagrees with the acquisition evidence")
		}
		bytes = downloaded
	}
	if computeDeclaredDigest(declared.Algorithm, bytes) != declared.Hex {
		return digestMismatch(selection.Name, declared.Algorithm, "disagrees with the declared descriptor digest")
	}
	return nil
}

// computeDeclaredDigest computes one ADR 0093 SRI algorithm's lowercase hex
// digest over data. The descriptor grammar (ADR 0094) closes the algorithm
// set at selection; an unknown algorithm yields an empty string, which fails
// closed as a mismatch.
func computeDeclaredDigest(algorithm string, data []byte) string {
	switch algorithm {
	case "sha256":
		return digest.SumSHA256(data).String()
	case "sha384":
		sum := sha512.Sum384(data)
		return hex.EncodeToString(sum[:])
	case "sha512":
		return digest.SumSHA512(data).String()
	default:
		return ""
	}
}
