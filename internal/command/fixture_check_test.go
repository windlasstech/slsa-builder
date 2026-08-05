package command

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFixtureCheck(t *testing.T) {
	t.Parallel()

	validManifest := `{
		"name":"valid",
		"type":"accepted",
		"surface":"npm",
		"artifact":"testdata/fixtures/artifact",
		"provenance":"testdata/fixtures/provenance",
		"release-manifest":null,
		"expected-result":"pass",
		"expected-failure-category":null,
		"expected-primary-id":null,
		"expected-secondary-ids":[],
		"covered-requirement":"ARCH-verification-policy-and-fixtures.fixture-manifest-schema"
	}`

	tests := []struct {
		name          string
		contents      string
		wantExitCode  int
		wantResult    string
		wantPrimaryID *string
	}{
		{name: "pass", contents: `{"fixtures":[` + validManifest + `]}`, wantExitCode: 0, wantResult: "pass"},
		{name: "policy failure", contents: `{"fixtures":[],"fixtures":[]}`, wantExitCode: 1, wantResult: "fail", wantPrimaryID: testStringPointer(diagnosticsContractInvalidID)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "index.json")
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatalf("write fixture index: %v", err)
			}

			var output bytes.Buffer
			result := NewDispatcher(NewFixtureCheckCommand()).Dispatch(context.Background(), []string{"fixture-check", "--index", path}, &output)
			if result.ExitCode != test.wantExitCode {
				t.Fatalf("exit code = %d, want %d; output=%s", result.ExitCode, test.wantExitCode, output.String())
			}

			var report Report
			if err := json.Unmarshal(output.Bytes(), &report); err != nil {
				t.Fatalf("decode report: %v; output=%s", err, output.String())
			}
			if string(report.Result) != test.wantResult {
				t.Errorf("result = %q, want %q", report.Result, test.wantResult)
			}
			if !equalOptionalString(report.PrimaryID, test.wantPrimaryID) {
				t.Errorf("primary ID = %v, want %v", report.PrimaryID, test.wantPrimaryID)
			}
		})
	}
}

func TestFixtureCheckMissingIndex(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing-index.json")
	var output bytes.Buffer
	result := NewDispatcher(NewFixtureCheckCommand()).Dispatch(context.Background(), []string{"fixture-check", "--index", path}, &output)
	if result.ExitCode != ExitCodeInvocationFailure {
		t.Fatalf("exit code = %d, want %d; output=%s", result.ExitCode, ExitCodeInvocationFailure, output.String())
	}

	var report Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v; output=%s", err, output.String())
	}
	if !equalOptionalString(report.PrimaryID, testStringPointer(inputUnavailableID)) {
		t.Errorf("primary ID = %v, want %q", report.PrimaryID, inputUnavailableID)
	}
}

func testStringPointer(value string) *string {
	return &value
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
