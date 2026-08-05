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
