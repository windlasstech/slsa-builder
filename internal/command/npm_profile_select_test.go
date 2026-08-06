package command

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

func TestNPMProfileSelectRejectedFixture(t *testing.T) {
	t.Parallel()
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", "testdata", "npm", "packages", "rejected", "private-package"))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	result := NewDispatcher(NewNPMProfileSelectCommand()).Dispatch(context.Background(), []string{
		"npm-profile-select",
		"--repository-root", repositoryRoot,
		"--package-directory", ".",
		"--observed-repository", "windlasstech/slsa-builder",
	}, &output)
	if result.ExitCode != ExitCodeVerificationFailure {
		t.Fatalf("exit code = %d, output = %s", result.ExitCode, output.String())
	}
	var report diagnostic.Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.PrimaryID == nil || *report.PrimaryID != "windlass.verify.error.package-private" {
		t.Fatalf("report = %#v", report)
	}
}
