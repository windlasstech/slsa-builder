package diagnostic

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	jsoncanonicalizer "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

func TestOrdering(t *testing.T) {
	t.Parallel()

	actualA := JSONValue([]string{"z"})
	actualB := JSONValue([]string{"a"})
	diagnostics := []Diagnostic{
		mustDiagnostic(t, IDTimestampClockSkew, "metadata.finishedOn", "Clock skew observed."),
		mustDiagnostic(t, IDTrustedProducerPolicyConflict, "policy.intersection", "Policies conflict."),
		mustDiagnostic(t, IDSignatureMismatch, "bundle.signature", "Signature mismatch."),
		mustDiagnostic(t, IDDigestMismatch, "subject.digest", "Digest mismatch."),
		{
			ID:       IDUnexpectedExternalParameters,
			Severity: SeverityError,
			Category: "unexpected-external-parameters",
			Check:    "predicate.externalParameters",
			Message:  "Unexpected parameters.",
			Field:    "z-field",
			Actual:   actualA,
		},
		{
			ID:       IDUnexpectedExternalParameters,
			Severity: SeverityError,
			Category: "unexpected-external-parameters",
			Check:    "predicate.externalParameters",
			Message:  "Unexpected parameters.",
			Field:    "a-field",
			Actual:   actualB,
		},
	}

	report, err := Build(nil, diagnostics, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	want := []string{
		IDDigestMismatch,
		IDSignatureMismatch,
		IDTrustedProducerPolicyConflict,
		IDUnexpectedExternalParameters,
		IDUnexpectedExternalParameters,
		IDTimestampClockSkew,
	}
	got := make([]string, len(report.Diagnostics))
	for i := range report.Diagnostics {
		got[i] = report.Diagnostics[i].ID
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered IDs = %#v, want %#v", got, want)
	}
	if report.Diagnostics[3].Field != "a-field" || report.Diagnostics[4].Field != "z-field" {
		t.Fatalf("same-level field ordering = %q, %q", report.Diagnostics[3].Field, report.Diagnostics[4].Field)
	}
	if report.PrimaryID == nil || *report.PrimaryID != IDDigestMismatch {
		t.Fatalf("primary ID = %v, want %q", report.PrimaryID, IDDigestMismatch)
	}
}

func TestExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		diagnostics []Diagnostic
		wantResult  Result
		wantCode    int
		wantPrimary *string
	}{
		{name: "pass", wantResult: ResultPass, wantCode: ExitCodePass},
		{
			name:        "policy failure",
			diagnostics: []Diagnostic{mustDiagnostic(t, IDSignatureMismatch, "bundle.signature", "Signature mismatch.")},
			wantResult:  ResultFail,
			wantCode:    ExitCodePolicyFailure,
			wantPrimary: testStringPointer(IDSignatureMismatch),
		},
		{
			name:        "invocation failure",
			diagnostics: []Diagnostic{mustDiagnostic(t, IDInputUnavailable, "input.bundle", "Bundle is unavailable.")},
			wantResult:  ResultFail,
			wantCode:    ExitCodeInvocationFailure,
			wantPrimary: testStringPointer(IDInputUnavailable),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			report, err := Build(nil, test.diagnostics, nil)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if report.Result != test.wantResult || report.ExitCode != test.wantCode {
				t.Fatalf("result/code = %q/%d, want %q/%d", report.Result, report.ExitCode, test.wantResult, test.wantCode)
			}
			if !reflect.DeepEqual(report.PrimaryID, test.wantPrimary) {
				t.Fatalf("primary ID = %v, want %v", report.PrimaryID, test.wantPrimary)
			}
		})
	}
}

func TestWarningOnlyPass(t *testing.T) {
	t.Parallel()

	runInvocation := "https://github.com/example/acme-widget/actions/runs/123456789/attempts/2"
	warning := mustDiagnostic(t, IDStaleNonSelectedLockfile, "externalParameters.package_manager.ignored_lockfile_paths", "A non-selected pnpm lockfile was recorded.")
	warning.Field = "externalParameters.package_manager.ignored_lockfile_paths"
	warning.Actual = JSONValue([]string{"pnpm-lock.yaml"})
	warning.Evidence = Evidence{"bundle": "package.tgz.intoto.jsonl"}

	report, err := Build(&runInvocation, []Diagnostic{warning}, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.Result != ResultPass || report.ExitCode != ExitCodePass || report.PrimaryID != nil {
		t.Fatalf("warning-only report = result %q, exit %d, primary %v", report.Result, report.ExitCode, report.PrimaryID)
	}
	assertGolden(t, report, "warning-only-report.jcs.json")
}

func TestMutationPossible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id               string
		wantPhase        Phase
		wantMutationFlag bool
	}{
		{IDReleaseTargetImmutable, PhasePreMutation, false},
		{IDMutationPermissionDenied, PhaseMutation, false},
		{IDPublisherIndeterminatePrimaryUpload, PhaseMutation, true},
		{IDManifestPartialJSONUploaded, PhaseMutation, true},
	}
	for _, test := range tests {
		definition, ok := Lookup(test.id)
		if !ok {
			t.Fatalf("Lookup(%q) was not registered", test.id)
		}
		if definition.Phase != test.wantPhase || definition.MutationPossible != test.wantMutationFlag {
			t.Errorf("Lookup(%q) = phase %q, mutation_possible %t; want %q, %t", test.id, definition.Phase, definition.MutationPossible, test.wantPhase, test.wantMutationFlag)
		}
	}
}

func TestClosedRegistryAndContractValidation(t *testing.T) {
	t.Parallel()

	if got := len(RegisteredIDs()); got != 155 {
		t.Fatalf("registered ID count = %d, want 155", got)
	}
	if _, ok := Lookup("windlass.verify.error.not-registered"); ok {
		t.Fatal("unknown ID unexpectedly registered")
	}

	tests := []struct {
		name       string
		diagnostic Diagnostic
	}{
		{
			name:       "unknown ID",
			diagnostic: Diagnostic{ID: "windlass.verify.error.not-registered", Severity: SeverityError, Category: "not-registered", Check: "test", Message: "Unknown."},
		},
		{
			name:       "warning as error",
			diagnostic: Diagnostic{ID: IDTimestampClockSkew, Severity: SeverityError, Category: "timestamp-clock-skew", Check: "test", Message: "Wrong severity."},
		},
		{
			name:       "secret evidence",
			diagnostic: Diagnostic{ID: IDSignatureMismatch, Severity: SeverityError, Category: "signature-mismatch", Check: "test", Message: "Unsafe.", Evidence: Evidence{"authorization": "Bearer secret-value"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Build(nil, []Diagnostic{test.diagnostic}, nil)
			if err == nil {
				t.Fatal("Build() unexpectedly succeeded")
			}
			var contractErr *ContractError
			if !errors.As(err, &contractErr) || contractErr.ID != IDDiagnosticsContractInvalid {
				t.Fatalf("Build() error = %v, want %q contract error", err, IDDiagnosticsContractInvalid)
			}
		})
	}
}

func TestEvidenceCredentialShapeCurrentPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		evidence Evidence
	}{
		{name: "GitHub server token prefix", evidence: Evidence{"observed": "ghs_FAKE000000000000000000000000000000000"}},
		{name: "JWT shape", evidence: Evidence{"observed": "eyJhbGciOiJub25lIn0.eyJzdWIiOiJmYWtlIn0.c2lnbmF0dXJl"}},
		{name: "camel-case apiKey", evidence: Evidence{"apiKey": "fake-value"}},
		{name: "scheme-relative userinfo URL", evidence: Evidence{"observed": "//fake-user:fake-pass@example.com/path"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			diagnostic := mustDiagnostic(t, IDSignatureMismatch, "bundle.signature", "Signature mismatch.")
			diagnostic.Evidence = test.evidence
			if _, err := Build(nil, []Diagnostic{diagnostic}, nil); err != nil {
				t.Fatalf("Build() rejected currently accepted Evidence shape: %v", err)
			}
		})
	}
}

func TestCanonicalReportDeterminism(t *testing.T) {
	t.Parallel()

	diagnostic := mustDiagnostic(t, IDSourceNumericIDMismatch, "fulcio.1.3.6.1.4.1.57264.1.15", "The source repository identifier does not match policy.")
	diagnostic.Field = "source.repository_id"
	diagnostic.Expected = JSONValue("123456789")
	diagnostic.Actual = JSONValue("222222222")
	diagnostic.PolicySources = []PolicySource{PolicySourceExplicitPolicy}
	diagnostic.Evidence = Evidence{
		"oid":                "1.3.6.1.4.1.57264.1.15",
		"certificate_sha256": "7c222fb2927d828af22f592134e8932480637c0d3f9c2072e82716801567e69f",
	}
	runInvocation := "https://github.com/example/acme-widget/actions/runs/123456789/attempts/2"
	report, err := Build(&runInvocation, []Diagnostic{diagnostic}, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	first, err := report.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON() error = %v", err)
	}
	for range 10 {
		next, nextErr := report.CanonicalJSON()
		if nextErr != nil {
			t.Fatalf("CanonicalJSON() repeated error = %v", nextErr)
		}
		if !bytes.Equal(first, next) {
			t.Fatal("CanonicalJSON() changed across runs")
		}
	}
	assertGolden(t, report, "failed-verification-report.jcs.json")
}

func mustDiagnostic(t *testing.T, id, check, message string) Diagnostic {
	t.Helper()
	diagnostic, err := New(id, check, message)
	if err != nil {
		t.Fatalf("New(%q) error = %v", id, err)
	}
	return diagnostic
}

func assertGolden(t *testing.T, report Report, name string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "diagnostics", name))
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", name, err)
	}
	got, err := report.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON() error = %v", err)
	}
	want, err = jsoncanonicalizer.Transform(want)
	if err != nil {
		t.Fatalf("canonicalize golden %q: %v", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("canonical report mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

func testStringPointer(value string) *string {
	return &value
}
