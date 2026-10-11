package provenance_test

import (
	"errors"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestValidationRemainingBoundaryFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		mutate    func(*provenance.Statement)
		corepack  *string
		id, field string
	}{
		{"missing signing adapter", func(s *provenance.Statement) { s.Predicate.RunDetails.Builder.BuilderDependencies = nil }, nil,
			"windlass.verify.error.builder-dependencies-signing-adapter-mismatch", "predicate.runDetails.builder.builderDependencies"},
		{"mismatched corepack", func(s *provenance.Statement) { s.Predicate.RunDetails.Builder.Version["corepack"] = "0.29.3" }, new("0.29.4"),
			"windlass.verify.error.builder-version-mismatch", "predicate.runDetails.builder.version.corepack"},
		{"invalid calendar date", func(s *provenance.Statement) { s.Predicate.RunDetails.Metadata.StartedOn = "2026-02-30T12:00:00Z" }, nil,
			"windlass.verify.error.timestamp-format-invalid", "predicate.runDetails.metadata.startedOn"},
		{"invalid finished timestamp", func(s *provenance.Statement) { s.Predicate.RunDetails.Metadata.FinishedOn = "2026-08-07T12:00:00.1Z" }, nil,
			"windlass.verify.error.timestamp-format-invalid", "predicate.runDetails.metadata.finishedOn"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			statement := loadStatement(t, "valid-statement.json")
			test.mutate(&statement)
			_, err := statement.Validate(expectations(test.corepack), nil)
			requireDiagnosticID(t, err, test.id)
			var validation *provenance.ValidationError
			if !errors.As(err, &validation) || validation.Diagnostic.Field != test.field {
				t.Fatalf("validation error = %v, want field %q", err, test.field)
			}
		})
	}
}
