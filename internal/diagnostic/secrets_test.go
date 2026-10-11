package diagnostic

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestBuildRejectsSecretOptionalValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value any
	}{
		{"bearer", "  BeArEr fake "},
		{"basic", "Basic fake"},
		{"GitHub classic token", "ghp_fake"},
		{"GitHub fine grained token", "github_pat_fake"},
		{"npm token", "npm_fake"},
		{"private key", "prefix -----BEGIN PRIVATE KEY----- suffix"},
		{"URL userinfo", "https://fake:fake@example.com/path"},
		{"nested map value", map[string]any{"outer": map[string]any{"value": "npm_fake"}}},
		{"nested array value", []any{nil, "safe", []any{"Basic fake"}}},
		{"string array value", []string{"safe", "ghp_fake"}},
		{"nested secret key", []any{map[string]any{"API_KEY": "fake"}}},
	}
	for _, test := range tests {
		for _, field := range []string{"expected", "actual"} {
			t.Run(test.name+"/"+field, func(t *testing.T) {
				t.Parallel()
				d := mustDiagnostic(t, IDSignatureMismatch, "signature", "Mismatch.")
				if field == "expected" {
					d.Expected = JSONValue(test.value)
				} else {
					d.Actual = JSONValue(test.value)
				}
				_, err := Build(nil, []Diagnostic{d}, nil)
				requireContractReason(t, err, field+" contains a secret-like value")
			})
		}
	}
}

func TestContainsSecretStringSlices(t *testing.T) {
	t.Parallel()
	// Build normalizes optional JSON to []any; exercise the native []string branch directly.
	tests := []struct {
		name   string
		values []string
		want   bool
	}{
		{"empty", []string{}, false},
		{"safe values", []string{"safe", "https://example.com"}, false},
		{"secret after safe value", []string{"safe", "npm_fake"}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := containsSecret(test.values); got != test.want {
				t.Fatalf("containsSecret(%q) = %t, want %t", test.values, got, test.want)
			}
		})
	}
}

func TestBuildPreservesSafeJSONValues(t *testing.T) {
	t.Parallel()
	d := mustDiagnostic(t, IDSignatureMismatch, "signature", "Mismatch.")
	d.Expected = JSONValue(map[string]any{"nested": []any{nil, true, 12, "safe", map[string]any{"path": "bundle.json"}}})
	d.Actual = JSONValue([]string{"safe", "https://example.com"})
	d.PolicySources = []PolicySource{PolicySourceExplicitPolicy, PolicySourceReleaseManifest, PolicySourceProducerExpectedValue, PolicySourceDigestVerifiedHandoff}
	d.Evidence = Evidence{
		"null": nil, "string": "bundle.json", "bool": true,
		"float64": float64(1), "float32": float32(2), "int": int(3), "int8": int8(4), "int16": int16(5),
		"int32": int32(6), "int64": int64(7), "uint": uint(8), "uint8": uint8(9), "uint16": uint16(10),
		"uint32": uint32(11), "uint64": uint64(12), "number": json.Number("13"),
	}
	report, err := Build(nil, []Diagnostic{d}, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !reflect.DeepEqual(report.Diagnostics, []Diagnostic{d}) {
		t.Fatalf("diagnostics = %#v, want %#v", report.Diagnostics, []Diagnostic{d})
	}
	if _, err := report.CanonicalJSON(); err != nil {
		t.Fatalf("CanonicalJSON() error = %v", err)
	}
}

func TestBuildRejectsUnsafeEvidence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		evidence Evidence
		reason   string
	}{
		{"empty key", Evidence{"": "safe"}, fmt.Sprintf("diagnostic %q evidence contains a secret-like key", IDSignatureMismatch)},
		{"normalized secret key", Evidence{"PRIVATE_KEY": "fake"}, fmt.Sprintf("diagnostic %q evidence contains a secret-like key", IDSignatureMismatch)},
		{"secret scalar", Evidence{"observed": "Bearer fake"}, fmt.Sprintf("diagnostic %q evidence %q is not a secret-safe scalar", IDSignatureMismatch, "observed")},
		{"nonfinite scalar", Evidence{"observed": math.Inf(1)}, fmt.Sprintf("diagnostic %q evidence %q is not an RFC 8785 JSON value", IDSignatureMismatch, "observed")},
		{"overflow JSON number", Evidence{"observed": json.Number("1e999")}, fmt.Sprintf("diagnostic %q evidence %q is not an RFC 8785 JSON value", IDSignatureMismatch, "observed")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			d := mustDiagnostic(t, IDSignatureMismatch, "signature", "Mismatch.")
			d.Evidence = test.evidence
			_, err := Build(nil, []Diagnostic{d}, nil)
			requireContractReason(t, err, test.reason)
		})
	}
}
