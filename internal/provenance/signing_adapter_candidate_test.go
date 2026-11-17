package provenance_test

import (
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestSigningAdapterCandidateDrift(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, version, sum string }{
		{"version-only drift", "v1.3.0", "h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw="},
		{"digest-only drift", "v1.4.0", "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given a syntactically valid candidate independent of the compiled signer pin.
			const module = "github.com/sigstore/sigstore-go"
			mod := []byte("module example.test/builder\nrequire " + module + " " + test.version + "\n")
			sum := []byte(module + " " + test.version + " " + test.sum + "\n")
			candidate, err := toolchainpins.ParseSigningAdapterModule(mod, sum)
			if err != nil {
				t.Fatalf("valid generation candidate rejected: %v", err)
			}
			expected := provenance.SigningAdapterBuilderDependency()
			uri := "pkg:golang/" + module + "@" + candidate.Version
			if uri == expected.URI && candidate.DirHash1 == expected.Digest["dirHash1"] {
				t.Fatal("alternate candidate unexpectedly matches compiled signer expectation")
			}
			statement := loadStatement(t, "valid-statement.json")
			statement.Predicate.RunDetails.Builder.BuilderDependencies[0].URI = uri
			statement.Predicate.RunDetails.Builder.BuilderDependencies[0].Digest = map[string]string{"dirHash1": candidate.DirHash1}
			// When the parsed candidate is used in a Statement without updating the compiled pin.
			_, err = statement.Validate(expectations(nil), nil)
			// Then validation, not parsing, rejects the drift.
			requireDiagnosticID(t, err, signerMismatchID)
		})
	}
}
