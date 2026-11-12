package identity

import "testing"

func TestCanonicalRepositoryNormalizesSchemeAndHostCase(t *testing.T) {
	const want = "https://github.com/windlasstech/example"
	for _, input := range []string{
		"HTTPS://github.com/WindlassTech/Example/",
		"https://GitHub.com/WindlassTech/Example.git",
	} {
		t.Run(input, func(t *testing.T) {
			got, err := CanonicalRepository(input)
			if err != nil {
				t.Fatalf("CanonicalRepository() error = %v", err)
			}
			if got != want {
				t.Fatalf("CanonicalRepository() = %q, want %q", got, want)
			}
		})
	}
}

func TestValidateCanonicalRepositoryURIRejectsNonCanonicalCase(t *testing.T) {
	for _, input := range []string{
		"HTTPS://github.com/windlasstech/example",
		"https://GitHub.com/windlasstech/example",
	} {
		t.Run(input, func(t *testing.T) {
			requireDiagnostic(t, ValidateCanonicalRepositoryURI(input), IDSourceIdentityMismatch)
		})
	}
}

// A repository segment ending in ".git" survives one normalization pass but is
// stripped as a suffix on the next, so the emitted canonical form fails
// re-validation. The segment must be rejected up front to keep
// CanonicalRepository idempotent. Regression: FuzzCanonicalRepository found
// "https://github.com/0/.git" and "https://github.com/0/0.git" as
// non-idempotent canonical outputs.
func TestCanonicalRepositoryRejectsGitSuffixRepositorySegment(t *testing.T) {
	for _, input := range []string{
		"https://github.com/0/.git.git",
		"https://github.com/0/.GIT.git",
		"https://github.com/0/0.git.git",
		"https://github.com/0/0.GIT.git",
		"git+https://github.com/0/0.git.git",
		"git@github.com:0/0.git.git",
		"ssh://git@github.com/0/0.git.git",
	} {
		t.Run(input, func(t *testing.T) {
			_, err := CanonicalRepository(input)
			requireDiagnostic(t, err, IDPackageRepositoryIdentityMismatch)
		})
	}
}

// The .git ambiguity is unique to the repository segment; an owner carrying a
// literal ".git" suffix is never subject to suffix stripping and must still
// canonicalize.
func TestCanonicalRepositoryAcceptsGitSuffixOwnerSegment(t *testing.T) {
	got, err := CanonicalRepository("https://github.com/example.git/repo")
	if err != nil {
		t.Fatalf("CanonicalRepository() error = %v", err)
	}
	if want := "https://github.com/example.git/repo"; got != want {
		t.Fatalf("CanonicalRepository() = %q, want %q", got, want)
	}
}
