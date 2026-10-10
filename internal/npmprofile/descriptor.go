package npmprofile

import (
	"strings"

	"golang.org/x/mod/semver"
)

// digestHexLengths pins the closed W3C Subresource Integrity algorithm set
// (ADR 0093) to each algorithm's lowercase hex encoding length (ADR 0094).
var digestHexLengths = map[string]int{
	"sha256": 64,
	"sha384": 96,
	"sha512": 128,
}

// parseDescriptorVersion splits a declared descriptor version into its plain
// version and optional declared integrity digest per ADRs 0092-0095. A version
// without "+" has no suffix. With "+", the build metadata must be exactly two
// dot-separated identifiers, <algorithm>.<hex>, with an SRI-set algorithm and
// lowercase hex of that algorithm's length; npm descriptors must not carry a
// suffix at all (ADR 0095). Every other shape is rejected with
// package-manager-digest-malformed.
func parseDescriptorVersion(manager Manager, version string) (plain string, digest *DeclaredDigest, failureID string) {
	core, suffix, hasSuffix := strings.Cut(version, "+")
	if !hasSuffix {
		return version, nil, ""
	}
	if manager == ManagerNPM {
		return "", nil, IDPackageManagerDigestMalformed
	}
	// SemVer permits one build-metadata suffix; a second "+" or any
	// identifier beyond the algorithm-hex pair violates the closed grammar.
	if strings.Contains(suffix, "+") {
		return "", nil, IDPackageManagerDigestMalformed
	}
	identifiers := strings.Split(suffix, ".")
	if len(identifiers) != 2 {
		return "", nil, IDPackageManagerDigestMalformed
	}
	wantLength, known := digestHexLengths[identifiers[0]]
	if !known || len(identifiers[1]) != wantLength || !isLowerHex(identifiers[1]) {
		return "", nil, IDPackageManagerDigestMalformed
	}
	return core, &DeclaredDigest{Algorithm: identifiers[0], Hex: identifiers[1]}, ""
}

// classifyDescriptorVersion applies the per-manager descriptor contract to a
// declared version: the digest grammar first, then exact-version and
// version-bound rules for pnpm and Yarn (ADR 0090, ADR 0091), and
// non-authoritative acceptance for npm (ADR 0095).
func classifyDescriptorVersion(manager Manager, version string) (plain string, digest *DeclaredDigest, failureID string) {
	// The digest grammar applies only to supported manager names: an
	// unsupported name stays a package-manager conflict even when its
	// descriptor carries a malformed suffix.
	switch manager {
	case ManagerNPM, ManagerPNPM, ManagerYarn:
	default:
		return "", nil, IDPackageManagerConflict
	}
	plain, digest, failureID = parseDescriptorVersion(manager, version)
	if failureID != "" {
		return "", nil, failureID
	}
	switch manager {
	case ManagerNPM:
		return plain, nil, ""
	case ManagerPNPM:
		if !exactSemver(plain) {
			return "", nil, IDPackageManagerVersionRequired
		}
		if pnpmVersionUnsupported(plain) {
			return "", nil, IDPnpmVersionUnsupported
		}
		return plain, digest, ""
	case ManagerYarn:
		if !exactSemver(plain) || semver.Compare("v"+plain, "v4.0.0") < 0 {
			return "", nil, IDYarnSelectionInvalid
		}
		if yarnVersionUnsupported(plain) {
			return "", nil, IDYarnVersionUnsupported
		}
		return plain, digest, ""
	default:
		return "", nil, IDPackageManagerConflict
	}
}

func isLowerHex(hex string) bool {
	for _, char := range hex {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
