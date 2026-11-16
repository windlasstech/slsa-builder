package provenance_test

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestSigningAdapterModuleGraphConsistency(t *testing.T) {
	t.Parallel()
	// Given the real governed module-content record, not its /go.mod checksum.
	sums, err := os.ReadFile("../../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "github.com/sigstore/sigstore-go v1.4.0 "
	var sum string
	for _, line := range strings.Split(string(sums), "\n") {
		if value, found := strings.CutPrefix(line, prefix); found {
			if sum != "" {
				t.Fatal("duplicate selected sigstore module-content checksum")
			}
			sum = value
		}
	}
	if sum != "h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw=" {
		t.Fatalf("selected module-content sum = %q, want h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw=", sum)
	}
	// When a current Statement decodes the independently converted directory hash.
	decoded, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(sum, "h1:"))
	if err != nil {
		t.Fatal(err)
	}
	statement, err := decodeDependencyStatement(t, "["+signerDescriptorJSON+"]")
	if err != nil {
		t.Fatal(err)
	}
	// Then both the graph bytes and the common validator agree with ADR 0113.
	dependency := statement.Predicate.RunDetails.Builder.BuilderDependencies[0]
	if dependency.Digest["dirHash1"] != hex.EncodeToString(decoded) {
		t.Fatalf("descriptor directory hash = %q, want %s", dependency.Digest["dirHash1"], hex.EncodeToString(decoded))
	}
	if _, err := statement.Validate(expectations(nil), nil); err != nil {
		t.Fatalf("governed v1.4.0 module descriptor rejected: %v", err)
	}
}
