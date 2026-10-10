package npmprofile

import (
	"strings"
	"testing"
)

// TestParseDescriptorVersion pins the closed +<algorithm>.<hex> digest-suffix
// grammar of ADRs 0092-0095: pnpm and Yarn descriptors may carry exactly one
// SRI-algorithm digest suffix with lowercase hex of the algorithm's length;
// npm descriptors must not carry any suffix; every other build-metadata shape
// is rejected with package-manager-digest-malformed.
func TestParseDescriptorVersion(t *testing.T) {
	t.Parallel()
	sha256Hex := strings.Repeat("a1", 32)
	sha384Hex := strings.Repeat("b2", 48)
	sha512Hex := strings.Repeat("c3", 64)

	tests := []struct {
		name        string
		manager     Manager
		version     string
		wantPlain   string
		wantDigest  *DeclaredDigest
		wantFailure string
	}{
		{name: "pnpm plain", manager: ManagerPNPM, version: "11.9.0", wantPlain: "11.9.0"},
		{name: "pnpm prerelease plain", manager: ManagerPNPM, version: "11.1.0-alpha.1", wantPlain: "11.1.0-alpha.1"},
		{name: "pnpm sha256", manager: ManagerPNPM, version: "11.9.0+sha256." + sha256Hex, wantPlain: "11.9.0",
			wantDigest: &DeclaredDigest{Algorithm: "sha256", Hex: sha256Hex}},
		{name: "pnpm sha384", manager: ManagerPNPM, version: "11.9.0+sha384." + sha384Hex, wantPlain: "11.9.0",
			wantDigest: &DeclaredDigest{Algorithm: "sha384", Hex: sha384Hex}},
		{name: "yarn sha512", manager: ManagerYarn, version: "4.1.0+sha512." + sha512Hex, wantPlain: "4.1.0",
			wantDigest: &DeclaredDigest{Algorithm: "sha512", Hex: sha512Hex}},
		{name: "yarn prerelease sha256", manager: ManagerYarn, version: "5.0.0-rc.1+sha256." + sha256Hex, wantPlain: "5.0.0-rc.1",
			wantDigest: &DeclaredDigest{Algorithm: "sha256", Hex: sha256Hex}},
		{name: "npm plain exact", manager: ManagerNPM, version: "11.5.1", wantPlain: "11.5.1"},
		{name: "npm range", manager: ManagerNPM, version: "^11.5.1", wantPlain: "^11.5.1"},
		{name: "npm garbage", manager: ManagerNPM, version: "garbage", wantPlain: "garbage"},
		{name: "npm empty", manager: ManagerNPM, version: "", wantPlain: ""},

		// Issue #122: non-digest SemVer build metadata is not a declaration.
		{name: "yarn non-digest metadata", manager: ManagerYarn, version: "4.1.0+build123", wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm unknown algorithm", manager: ManagerPNPM, version: "11.9.0+sha224." + sha256Hex, wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm sha1 algorithm", manager: ManagerYarn, version: "4.1.0+sha1." + sha256Hex, wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm uppercase hex", manager: ManagerPNPM, version: "11.9.0+sha256." + strings.ToUpper(sha256Hex), wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm short hex", manager: ManagerPNPM, version: "11.9.0+sha256." + sha256Hex[:63], wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm long hex", manager: ManagerPNPM, version: "11.9.0+sha256." + sha256Hex + "0", wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm sha384 with sha256 length", manager: ManagerPNPM, version: "11.9.0+sha384." + sha256Hex, wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm sha512 with sha384 length", manager: ManagerPNPM, version: "11.9.0+sha512." + sha384Hex, wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm missing hex", manager: ManagerPNPM, version: "11.9.0+sha512.", wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm extra identifier", manager: ManagerPNPM, version: "11.9.0+sha512." + sha512Hex + ".build1", wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm empty suffix", manager: ManagerPNPM, version: "11.9.0+", wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm single identifier", manager: ManagerPNPM, version: "11.9.0+sha256", wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm second plus", manager: ManagerPNPM, version: "11.9.0+sha256." + sha256Hex + "+x", wantFailure: IDPackageManagerDigestMalformed},
		{name: "pnpm non-hex character", manager: ManagerPNPM, version: "11.9.0+sha256.g" + sha256Hex[1:], wantFailure: IDPackageManagerDigestMalformed},
		{name: "npm well-formed suffix", manager: ManagerNPM, version: "11.5.1+sha512." + sha512Hex, wantFailure: IDPackageManagerDigestMalformed},
		{name: "npm arbitrary suffix", manager: ManagerNPM, version: "11.5.1+build123", wantFailure: IDPackageManagerDigestMalformed},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			plain, digest, failureID := parseDescriptorVersion(test.manager, test.version)
			if failureID != test.wantFailure {
				t.Fatalf("failureID = %q, want %q", failureID, test.wantFailure)
			}
			if test.wantFailure != "" {
				return
			}
			if plain != test.wantPlain {
				t.Fatalf("plain = %q, want %q", plain, test.wantPlain)
			}
			if test.wantDigest == nil {
				if digest != nil {
					t.Fatalf("digest = %#v, want nil", digest)
				}
				return
			}
			if digest == nil || *digest != *test.wantDigest {
				t.Fatalf("digest = %#v, want %#v", digest, test.wantDigest)
			}
		})
	}
}

// TestManagerSelectionDescriptor pins the Descriptor rendering: the plain
// version alone, or version + "+" + algorithm + "." + hex when a digest is
// declared. Version always stays the plain version.
func TestManagerSelectionDescriptor(t *testing.T) {
	t.Parallel()
	plain := ManagerSelection{Name: ManagerPNPM, Version: "11.9.0"}
	if plain.Descriptor() != "11.9.0" {
		t.Fatalf("plain descriptor = %q", plain.Descriptor())
	}
	withDigest := ManagerSelection{
		Name:    ManagerYarn,
		Version: "4.9.2",
		Digest:  &DeclaredDigest{Algorithm: "sha256", Hex: strings.Repeat("a1", 32)},
	}
	want := "4.9.2+sha256." + strings.Repeat("a1", 32)
	if withDigest.Descriptor() != want {
		t.Fatalf("digest descriptor = %q, want %q", withDigest.Descriptor(), want)
	}
	if withDigest.Version != "4.9.2" {
		t.Fatalf("Version = %q, want the plain version", withDigest.Version)
	}
}
