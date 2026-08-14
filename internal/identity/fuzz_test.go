package identity

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// FuzzCanonicalRepository exercises the repository-locator normalizer against
// attacker-controlled package metadata. Accepted input must canonicalize to the
// exact lowercase https://github.com/owner/repository form, pass
// ValidateCanonicalRepositoryURI, and be a fixed point of CanonicalRepository.
// Rejected input must fail with the registered
// package-repository-identity-mismatch diagnostic, never a panic or bare error.
func FuzzCanonicalRepository(f *testing.F) {
	// Accepted forms from TestCanonicalRepository and the identity fixtures.
	for _, seed := range []string{
		"owner/repository",
		"github:WindlassTech/Example",
		"https://github.com/WindlassTech/Example",
		"git+https://github.com/WindlassTech/Example.git",
		"git://github.com/WindlassTech/Example/",
		"git@github.com:WindlassTech/Example.git",
		"ssh://git@github.com/WindlassTech/Example.git",
		"https://github.com/WindlassTech/repository_with-dashes",
		"example/acme-widget",
		"https://github.com/example/acme-widget",
	} {
		f.Add(seed)
	}
	// Rejected literals from TestCanonicalRepository.
	for _, seed := range []string{
		"",
		"gitlab:WindlassTech/Example",
		"https://gitlab.com/WindlassTech/Example",
		"https://github.com/WindlassTech/Example/releases",
		"https://github.com/WindlassTech/Example?tab=readme",
		"https://token@github.com/WindlassTech/Example",
		"https://github.com:443/WindlassTech/Example",
		"https://github.com/WindlassTech/%2e%2e/Example",
		"https://github.com/WindlassTech//Example",
		"WindlassTech\\Example",
		"WindlassTech/Example.git",
		"github:WindlassTech/Example.git",
		"git@github.com:WindlassTech/Example",
		"git@github.com:WindlassTech/Example.git/",
		"ssh://git@github.com/WindlassTech/Example",
		" owner/repository",
		"owner/repository ",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		canonical, err := CanonicalRepository(raw)
		if err != nil {
			requireDiagnostic(t, err, IDPackageRepositoryIdentityMismatch)
			return
		}
		if !strings.HasPrefix(canonical, canonicalGitHubBase) {
			t.Fatalf("CanonicalRepository(%q) = %q, missing canonical prefix", raw, canonical)
		}
		if canonical != strings.ToLower(canonical) {
			t.Fatalf("CanonicalRepository(%q) = %q, not lowercase", raw, canonical)
		}
		if err := ValidateCanonicalRepositoryURI(canonical); err != nil {
			t.Fatalf("canonical form %q rejected by ValidateCanonicalRepositoryURI: %v", canonical, err)
		}
		again, err := CanonicalRepository(canonical)
		if err != nil {
			t.Fatalf("canonical form %q rejected on re-normalization: %v", canonical, err)
		}
		if again != canonical {
			t.Fatalf("CanonicalRepository not idempotent: %q -> %q -> %q", raw, canonical, again)
		}
	})
}

// FuzzValidateFullSHA checks the immutable workflow/source digest contract:
// accepted values are exactly 40 lowercase hexadecimal characters, and rejected
// values carry the builder-id-not-immutable diagnostic.
func FuzzValidateFullSHA(f *testing.F) {
	for _, seed := range []string{
		workflowSHA,
		sourceSHA,
		"v1.2.3",
		"89abcde",
		"89ABCDEF0123456789ABCDEF0123456789ABCDEF",
		"",
		strings.Repeat("a", 39),
		strings.Repeat("a", 41),
		strings.Repeat("a", 64),
		strings.Repeat("g", 40),
		"0123456789abcdef0123456789abcdef0123456 ",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value string) {
		err := ValidateFullSHA(value)
		if err != nil {
			requireDiagnostic(t, err, IDBuilderIDNotImmutable)
			return
		}
		if len(value) != 40 {
			t.Fatalf("ValidateFullSHA(%q) accepted length %d, want 40", value, len(value))
		}
		for _, character := range value {
			if !strings.ContainsRune("0123456789abcdef", character) {
				t.Fatalf("ValidateFullSHA(%q) accepted non-lowercase-hex %q", value, character)
			}
		}
		if err := ValidateFullSHA(value); err != nil {
			t.Fatalf("ValidateFullSHA(%q) not deterministic: %v", value, err)
		}
	})
}

// FuzzValidateReleaseRef checks the tags-only release ref contract: accepted
// refs start with refs/tags/ and carry a non-empty tag, and rejected refs carry
// the source-ref-mismatch diagnostic.
func FuzzValidateReleaseRef(f *testing.F) {
	for _, seed := range []string{
		"refs/tags/v1.2.3",
		"refs/tags/v1/2.3",
		"refs/heads/main",
		"v1.2.3",
		" refs/tags/v1.2.3",
		"refs/tags/v1.2.3 ",
		"refs/tags/",
		"refs/tags/@",
		"refs/tags//v1",
		"refs/tags/v1/",
		"refs/tags/v1..2",
		"refs/tags/a@{b}",
		"refs/tags/v1.",
		"refs/tags/.hidden",
		"refs/tags/x.lock",
		"refs/tags/~bad",
		"refs/tags/bad\\ref",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, ref string) {
		err := ValidateReleaseRef(ref)
		if err != nil {
			requireDiagnostic(t, err, IDSourceRefMismatch)
			return
		}
		const prefix = "refs/tags/"
		if !strings.HasPrefix(ref, prefix) {
			t.Fatalf("ValidateReleaseRef(%q) accepted ref without %q prefix", ref, prefix)
		}
		if strings.TrimPrefix(ref, prefix) == "" {
			t.Fatalf("ValidateReleaseRef(%q) accepted an empty tag", ref)
		}
		if err := ValidateReleaseRef(ref); err != nil {
			t.Fatalf("ValidateReleaseRef(%q) not deterministic: %v", ref, err)
		}
	})
}

// FuzzValidateBuilderID checks the ADR 0028 immutable builder identity:
// accepted IDs re-validate deterministically and reconstruct exactly through
// NewBuilderID; rejected IDs carry the builder-id-not-immutable diagnostic.
func FuzzValidateBuilderID(f *testing.F) {
	valid, err := NewBuilderID(workflowPath, workflowSHA)
	if err != nil {
		f.Fatalf("NewBuilderID() error = %v", err)
	}
	f.Add(valid)
	for _, ref := range []string{"main", "v1", "v1.2.3", "89abcde", "89ABCDEF0123456789ABCDEF0123456789ABCDEF"} {
		f.Add("https://github.com/windlasstech/slsa-builder/" + workflowPath + "@" + ref)
	}
	for _, seed := range []string{
		"",
		"https://github.com/other/repository/" + workflowPath + "@" + workflowSHA,
		"https://github.com/windlasstech/slsa-builder/" + workflowPath,
		"https://github.com/windlasstech/slsa-builder/.github/workflows/nested/x.yml@" + workflowSHA,
		"https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml%40" + workflowSHA,
		" " + valid,
		valid + " ",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, builderID string) {
		err := ValidateBuilderID(builderID)
		if err != nil {
			requireDiagnostic(t, err, IDBuilderIDNotImmutable)
			return
		}
		if err := ValidateBuilderID(builderID); err != nil {
			t.Fatalf("ValidateBuilderID(%q) not deterministic: %v", builderID, err)
		}
		remainder, found := strings.CutPrefix(builderID, builderRepositoryURI+"/")
		if !found {
			t.Fatalf("ValidateBuilderID(%q) accepted an ID outside the trusted repository", builderID)
		}
		path, sha, found := strings.Cut(remainder, "@")
		if !found {
			t.Fatalf("ValidateBuilderID(%q) accepted an ID without a workflow SHA suffix", builderID)
		}
		reconstructed, err := NewBuilderID(path, sha)
		if err != nil {
			t.Fatalf("accepted builder ID %q does not reconstruct: %v", builderID, err)
		}
		if reconstructed != builderID {
			t.Fatalf("builder ID round-trip mismatch: %q -> %q", builderID, reconstructed)
		}
	})
}

// FuzzValidateBuildTypeURI checks the acquired-domain build type contract:
// accepted URIs re-validate deterministically and reconstruct exactly through
// NewBuildTypeURI; rejected URIs carry the build-type-not-canonical diagnostic.
func FuzzValidateBuildTypeURI(f *testing.F) {
	for _, seed := range []struct {
		profile string
		major   uint64
	}{
		{"js-ts-npm-package", 1},
		{"go", 2},
	} {
		valid, err := NewBuildTypeURI(seed.profile, seed.major)
		if err != nil {
			f.Fatalf("NewBuildTypeURI(%q, %d) error = %v", seed.profile, seed.major, err)
		}
		f.Add(valid)
	}
	for _, seed := range []string{
		"https://windlasstech.github.io/slsa-builder/js-ts-npm-package/v1",
		"https://buildtype.dev/windlass/slsa-builder/js%2fts-npm-package/v1",
		"https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v01",
		"https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v0",
		"https://buildtype.dev/windlass/slsa-builder//v1",
		"https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1/extra",
		"https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v18446744073709551616",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, buildType string) {
		err := ValidateBuildTypeURI(buildType)
		if err != nil {
			requireDiagnostic(t, err, IDBuildTypeNotCanonical)
			return
		}
		if err := ValidateBuildTypeURI(buildType); err != nil {
			t.Fatalf("ValidateBuildTypeURI(%q) not deterministic: %v", buildType, err)
		}
		remainder, found := strings.CutPrefix(buildType, buildTypeNamespace)
		if !found {
			t.Fatalf("ValidateBuildTypeURI(%q) accepted a URI outside the buildtype.dev namespace", buildType)
		}
		profile, majorText, found := strings.Cut(remainder, "/v")
		if !found {
			t.Fatalf("ValidateBuildTypeURI(%q) accepted a URI without a /vN suffix", buildType)
		}
		major, err := parsePositiveDecimal(majorText)
		if err != nil {
			t.Fatalf("accepted build type %q has unparsable major %q: %v", buildType, majorText, err)
		}
		reconstructed, err := NewBuildTypeURI(profile, major)
		if err != nil {
			t.Fatalf("accepted build type %q does not reconstruct: %v", buildType, err)
		}
		if reconstructed != buildType {
			t.Fatalf("build type round-trip mismatch: %q -> %q", buildType, reconstructed)
		}
	})
}

// FuzzParseRunInvocationURI checks the run-identity round trip: any URI built
// by NewRunInvocationURI must parse back to the same components, and any
// rejected invocation URI must carry the run-invocation-uri-invalid diagnostic.
func FuzzParseRunInvocationURI(f *testing.F) {
	const repository = "https://github.com/example/acme-widget"
	f.Add(repository+"/actions/runs/30745570800/attempts/2", "30745570800", "2")
	f.Add("", "0", "2")
	f.Add("", "+1", "2")
	f.Add("", "01", "2")
	f.Add("", "1", "0")
	f.Add("", "", "")
	f.Add(repository+"/actions/runs/0/attempts/2", "1", "1")
	f.Add(repository+"/actions/runs/1/attempts/2?download=1", "1", "2")
	f.Add("https://github.com/other/acme-widget/actions/runs/1/attempts/2", "1", "2")
	f.Add("https://user@github.com/example/acme-widget/actions/runs/1/attempts/2", "1", "2")
	f.Add("https://github.com:443/example/acme-widget/actions/runs/1/attempts/2", "1", "2")

	f.Fuzz(func(t *testing.T, invocationURI, runID, attempt string) {
		const repository = "https://github.com/example/acme-widget"

		// Constructed URIs must always round-trip.
		constructed, err := NewRunInvocationURI(repository, runID, attempt)
		if err != nil {
			requireDiagnostic(t, err, IDRunInvocationURIInvalid)
		} else {
			parsed, err := ParseRunInvocationURI(constructed, repository)
			if err != nil {
				t.Fatalf("constructed URI %q rejected: %v", constructed, err)
			}
			if parsed.URI != constructed || parsed.RepositoryURI != repository ||
				parsed.RunID != runID || parsed.Attempt != attempt {
				t.Fatalf("round-trip mismatch for %q: %#v", constructed, parsed)
			}
		}

		// Raw URIs must never panic and must fail with the classified diagnostic.
		parsed, err := ParseRunInvocationURI(invocationURI, repository)
		if err != nil {
			requireDiagnostic(t, err, IDRunInvocationURIInvalid)
			return
		}
		if parsed.URI != invocationURI || parsed.RepositoryURI != repository {
			t.Fatalf("ParseRunInvocationURI(%q) returned inconsistent identity %#v", invocationURI, parsed)
		}
		reconstructed, err := NewRunInvocationURI(repository, parsed.RunID, parsed.Attempt)
		if err != nil {
			t.Fatalf("accepted invocation URI %q does not reconstruct: %v", invocationURI, err)
		}
		if reconstructed != invocationURI {
			t.Fatalf("invocation URI round-trip mismatch: %q -> %q", invocationURI, reconstructed)
		}
	})
}

// parsePositiveDecimal converts the canonical positive decimal form used by run
// IDs, attempts, and build type major versions. It guards with the production
// grammar predicate so the fuzz targets only convert values the validators
// themselves accept, rather than re-implementing the grammar in the test.
func parsePositiveDecimal(text string) (uint64, error) {
	if !validPositiveDecimal(text) {
		return 0, errNonCanonicalDecimal
	}
	return strconv.ParseUint(text, 10, 64)
}

var errNonCanonicalDecimal = errors.New("not a canonical positive decimal")
