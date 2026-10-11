package diagnostic

import (
	"reflect"
	"testing"
)

func TestBuildOrderingTieBreakers(t *testing.T) {
	t.Parallel()
	base := mustDiagnostic(t, IDSignatureMismatch, "signature", "Mismatch.")
	tests := []struct {
		name          string
		first, second Diagnostic
	}{
		{"precedence before lexical ID", mustDiagnostic(t, IDInputUnavailable, "input", "Unavailable."), mustDiagnostic(t, "windlass.verify.error.builder-id-not-immutable", "builder", "Mutable.")},
		{"ID within precedence", mustDiagnostic(t, IDDigestMismatch, "digest", "Mismatch."), mustDiagnostic(t, IDInputUnavailable, "input", "Unavailable.")},
		{"absent field before present", base, base},
		{"field before actual", base, base},
		{"actual canonical lexical not numeric", base, base},
		{"absent actual before explicit null", base, base},
		{"full canonical diagnostic", base, base},
		{"equivalent canonical objects", base, base},
	}
	tests[2].second.Field = "signature"
	tests[3].first.Field, tests[3].second.Field = "a", "z"
	tests[3].first.Actual, tests[3].second.Actual = JSONValue("z"), JSONValue("a")
	tests[4].first.Actual, tests[4].second.Actual = JSONValue(10), JSONValue(2)
	tests[5].second.Actual = JSONValue(nil)
	tests[6].first.Message, tests[6].second.Message = "Alpha.", "Zulu."
	tests[7].first.Actual = JSONValue(map[string]any{"z": 2, "a": 1})
	tests[7].second.Actual = JSONValue(map[string]any{"a": 1, "z": 2})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := []Diagnostic{test.second, test.first}
			report, err := Build(nil, input, nil)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			want := []Diagnostic{test.first, test.second}
			if !reflect.DeepEqual(report.Diagnostics, want) {
				t.Fatalf("ordered diagnostics = %#v, want %#v", report.Diagnostics, want)
			}
			if !reflect.DeepEqual(input, []Diagnostic{test.second, test.first}) {
				t.Fatal("Build() reordered caller's diagnostics")
			}
			if report.PrimaryID == nil || *report.PrimaryID != test.first.ID {
				t.Fatalf("primary = %v, want %q", report.PrimaryID, test.first.ID)
			}
		})
	}
}
