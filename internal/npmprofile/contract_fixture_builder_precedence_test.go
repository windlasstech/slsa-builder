package npmprofile

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

func TestBuilderContractReportPrecedence(t *testing.T) {
	t.Parallel()
	// Given independently validated failures, not invented secondary diagnostics.
	var entries []diagnostic.Diagnostic
	for _, mutation := range []string{"builder-common-shape", "builder-signer-version", "builder-publish-version"} {
		spec := contractFixtureSpec{manager: ManagerNPM, scenario: "tag", mutation: mutation}
		failure := runContractVariant(t, spec, builderContractVariants(t, spec)[0])
		var identified interface{ DiagnosticID() string }
		if !errors.As(failure, &identified) {
			t.Fatalf("%s did not produce an identified failure: %v", mutation, failure)
		}
		entry, err := diagnostic.New(identified.DiagnosticID(), publishCLIBuilderField, failure.Error())
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	for _, order := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}, {0, 1}, {1, 0}} {
		t.Run(string(mustJSON(order)), func(t *testing.T) {
			t.Parallel()
			var input []diagnostic.Diagnostic
			for _, index := range order {
				input = append(input, entries[index])
			}
			// When combining genuine failures into a report.
			report, err := diagnostic.Build(nil, input, nil)
			if err != nil {
				t.Fatal(err)
			}
			// Then publish outranks signer, which outranks common shape, regardless of input order.
			want := []string{entries[1].ID, entries[0].ID}
			if len(order) == 3 {
				want = append([]string{entries[2].ID}, want...)
			}
			var got []string
			for _, entry := range report.Diagnostics {
				got = append(got, entry.ID)
			}
			if !slices.Equal(got, want) || report.PrimaryID == nil || *report.PrimaryID != want[0] {
				t.Fatalf("report order = %v, want %v; primary = %v", got, want, report.PrimaryID)
			}
		})
	}
}

func TestBuilderContractAnnotationsCannotRepairMismatch(t *testing.T) {
	t.Parallel()
	for _, spec := range finalContractFixtures {
		if spec.category != "builder-dependencies-publish-cli-mismatch" && spec.category != "builder-dependencies-signing-adapter-mismatch" {
			continue
		}
		t.Run(spec.name, func(t *testing.T) {
			t.Parallel()
			// Given a wrong descriptor carrying the authentic baseline claims as annotations.
			baseline := decodeContractStatement(t, contractVariant(t, spec.manager, spec.scenario).Statement)
			claims := baseline.Predicate.RunDetails.Builder.BuilderDependencies
			for _, variant := range contractVariants(t, spec) {
				statement := decodeContractStatement(t, variant.Statement)
				for index := range statement.Predicate.RunDetails.Builder.BuilderDependencies {
					statement.Predicate.RunDetails.Builder.BuilderDependencies[index].Annotations = json.RawMessage(mustJSON(claims))
				}
				variant.Statement = mustJSON(statement)
				// When validating, then descriptive claims cannot repair real identity or integrity.
				requireNPMDiagnostic(t, runContractVariant(t, spec, variant), "windlass.verify.error."+spec.category)
			}
		})
	}
}
