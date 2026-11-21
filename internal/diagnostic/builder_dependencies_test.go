package diagnostic

import (
	"slices"
	"testing"
)

func TestBuilderDependencyPrecedence(t *testing.T) {
	t.Parallel()
	const prefix = "windlass.verify.error."
	for _, parse := range []string{"", "duplicate-json-member", "diagnostics-contract-invalid"} {
		for _, reverse := range []bool{false, true} {
			t.Run(parse+map[bool]string{false: "/forward", true: "/reverse"}[reverse], func(t *testing.T) {
				t.Parallel()
				// Given simultaneous descriptor failures, optionally with a parse failure.
				want := []string{prefix + "builder-dependencies-publish-cli-mismatch", prefix + "builder-dependencies-signing-adapter-mismatch", prefix + "builder-dependencies-descriptor-invalid"}
				if parse != "" {
					want = append([]string{prefix + parse}, want...)
				}
				want = append(want, IDTimestampClockSkew)
				input := slices.Clone(want)
				if reverse {
					slices.Reverse(input)
				}
				var diagnostics []Diagnostic
				for _, id := range input {
					diagnostics = append(diagnostics, mustDiagnostic(t, id, "builderDependencies", "Invalid descriptor."))
				}
				// When a report chooses its primary diagnostic and deterministic order.
				report, err := Build(nil, diagnostics, nil)
				if err != nil {
					t.Fatal(err)
				}
				// Then JSON errors precede publish-cli, signer, common shape, and warnings.
				var got []string
				for _, diagnostic := range report.Diagnostics {
					got = append(got, diagnostic.ID)
				}
				if !slices.Equal(got, want) {
					t.Fatalf("order = %v, want %v", got, want)
				}
				if report.PrimaryID == nil || *report.PrimaryID != want[0] {
					t.Fatalf("primary = %v, want %s", report.PrimaryID, want[0])
				}
			})
		}
	}
}
