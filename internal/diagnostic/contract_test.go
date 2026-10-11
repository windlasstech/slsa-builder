package diagnostic

import (
	"errors"
	"fmt"
	"testing"
)

func requireContractReason(t *testing.T, err error, reason string) {
	t.Helper()
	var contract *ContractError
	if !errors.As(err, &contract) {
		t.Fatalf("error = %v, want *ContractError", err)
	}
	if contract.ID != IDDiagnosticsContractInvalid || contract.Reason != reason {
		t.Fatalf("contract = %#v, want ID %q, reason %q", contract, IDDiagnosticsContractInvalid, reason)
	}
	if want := IDDiagnosticsContractInvalid + ": " + reason; contract.Error() != want {
		t.Fatalf("Error() = %q, want %q", contract.Error(), want)
	}
}

func TestContractErrorWrapping(t *testing.T) {
	t.Parallel()
	cause := &ContractError{ID: IDDiagnosticsContractInvalid, Reason: "invalid metadata"}
	err := fmt.Errorf("report: %w", cause)
	if want := IDDiagnosticsContractInvalid + ": invalid metadata"; cause.Error() != want {
		t.Fatalf("Error() = %q, want %q", cause.Error(), want)
	}
	var got *ContractError
	if !errors.Is(err, cause) || !errors.As(err, &got) || got != cause {
		t.Fatalf("wrapped error = %v, want original cause %v through Is and As", err, cause)
	}
	if want := "report: " + IDDiagnosticsContractInvalid + ": invalid metadata"; err.Error() != want {
		t.Fatalf("wrapped Error() = %q, want %q", err.Error(), want)
	}
}

func TestBuildRejectsInvalidDiagnosticFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*Diagnostic)
		reason string
	}{
		{"empty check", func(d *Diagnostic) { d.Check = "" }, fmt.Sprintf("diagnostic %q requires non-empty UTF-8 check and message", IDSignatureMismatch)},
		{"invalid message UTF8", func(d *Diagnostic) { d.Message = "\xff" }, fmt.Sprintf("diagnostic %q requires non-empty UTF-8 check and message", IDSignatureMismatch)},
		{"invalid field UTF8", func(d *Diagnostic) { d.Field = "\xff" }, fmt.Sprintf("diagnostic %q field is not valid UTF-8", IDSignatureMismatch)},
		{"unknown policy source", func(d *Diagnostic) { d.PolicySources = []PolicySource{"unknown"} }, fmt.Sprintf("diagnostic %q has unknown policy source %q", IDSignatureMismatch, "unknown")},
		{"duplicate policy source", func(d *Diagnostic) {
			d.PolicySources = []PolicySource{PolicySourceExplicitPolicy, PolicySourceExplicitPolicy}
		}, fmt.Sprintf("diagnostic %q has duplicate policy source %q", IDSignatureMismatch, PolicySourceExplicitPolicy)},
		{"non JSON expected", func(d *Diagnostic) { d.Expected = JSONValue(make(chan int)) }, "expected is not an RFC 8785 JSON value: json: unsupported type: chan int"},
		{"non JSON actual", func(d *Diagnostic) { d.Actual = JSONValue(make(chan int)) }, "actual is not an RFC 8785 JSON value: json: unsupported type: chan int"},
		{"nested evidence", func(d *Diagnostic) { d.Evidence = Evidence{"observed": []string{"safe"}} }, fmt.Sprintf("diagnostic %q evidence %q is not a secret-safe scalar", IDSignatureMismatch, "observed")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			d := mustDiagnostic(t, IDSignatureMismatch, "signature", "Mismatch.")
			test.change(&d)
			_, err := Build(nil, []Diagnostic{d}, nil)
			requireContractReason(t, err, test.reason)
		})
	}
}

func TestCanonicalJSONRejectsInvalidReport(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*Report)
		reason string
	}{
		{"schema version", func(r *Report) { r.SchemaVersion = "2" }, `schema_version must equal "1"`},
		{"null diagnostics", func(r *Report) { r.Diagnostics = nil }, "diagnostics must be a non-null array"},
		{"empty invocation", func(r *Report) { r.RunInvocation = new("") }, "run_invocation must be non-empty when present"},
		{"wrong result", func(r *Report) { r.Result = ResultFail }, "result, exit_code, and primary_id do not match the first ordered error"},
		{"wrong exit code", func(r *Report) { r.ExitCode = 1 }, "result, exit_code, and primary_id do not match the first ordered error"},
		{"wrong primary", func(r *Report) { r.PrimaryID = new(IDSignatureMismatch) }, "result, exit_code, and primary_id do not match the first ordered error"},
		{"missing metadata manifest", func(r *Report) { r.DiagnosticMetadata = &DiagnosticMetadata{} }, "diagnostic_metadata must contain package_manifest"},
		{"invalid diagnostic", func(r *Report) { r.Diagnostics = []Diagnostic{{ID: "unknown"}} }, `unknown diagnostic ID "unknown"`},
		{"reverse order", func(r *Report) {
			r.Diagnostics = []Diagnostic{
				mustDiagnostic(t, IDSignatureMismatch, "signature", "Mismatch."),
				mustDiagnostic(t, IDDigestMismatch, "digest", "Mismatch."),
			}
		}, "diagnostics are not in deterministic precedence order"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			report := Report{SchemaVersion: "1", Result: ResultPass, Diagnostics: []Diagnostic{}}
			test.change(&report)
			got, err := report.CanonicalJSON()
			requireContractReason(t, err, test.reason)
			if got != nil {
				t.Fatalf("CanonicalJSON() bytes = %q, want nil on rejection", got)
			}
			if test.name == "invalid diagnostic" {
				want := `diagnostics[0]: ` + IDDiagnosticsContractInvalid + `: unknown diagnostic ID "unknown"`
				if err.Error() != want {
					t.Fatalf("error = %q, want %q", err.Error(), want)
				}
			}
		})
	}
}

func TestRegistryRejectsInvalidRegistration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		run  func()
		want string
	}{
		{"duplicate", func() {
			register(map[string]Definition{IDDigestMismatch: {ID: IDDigestMismatch}}, Definition{ID: IDDigestMismatch})
		}, "duplicate diagnostic registry entry: " + IDDigestMismatch},
		{"unregistered override", func() { set(map[string]Definition{}, PhasePolicy, 1, false, 1, "unknown") }, "diagnostic override is not registered: windlass.verify.error.unknown"},
		{"unregistered precedence", func() { setPrecedence(map[string]Definition{}, 1, "unknown") }, "diagnostic precedence entry is not registered: windlass.verify.error.unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if got := recover(); got != test.want {
					t.Errorf("panic = %v, want %q", got, test.want)
				}
			}()
			test.run()
		})
	}
}
