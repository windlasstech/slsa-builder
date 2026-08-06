package command

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/workflowcheck"
)

func TestWorkflowCheckCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yml")
	contents := `permissions: {}
jobs:
  build:
    runs-on: ubuntu-24.04
    permissions: {contents: read}
    concurrency:
      group: npm-build-${{ github.repository }}-${{ github.ref_name }}
      cancel-in-progress: true
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411
        with: {egress-policy: audit}
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with: {persist-credentials: false}
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
        with: {node-version: "24"}
      - run: corepack enable
      - run: go run ./cmd/slsa-builder-internal npm-profile-build
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with: {name: "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}"}
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with: {name: "js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}"}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := NewWorkflowCheckCommand().Execute(context.Background(), []string{"--workflow", path, "--job", "build"}, &output); err != nil {
		t.Fatal(err)
	}
	var result workflowcheck.BuildJobResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode output %q: %v", output.String(), err)
	}
	if result.Result != "pass" || result.SigningPermission || result.MutationPermission || len(result.ArtifactNames) != 2 {
		t.Fatalf("unexpected workflow-check result: %#v", result)
	}
}

func TestWorkflowCheckCommandRejectsUnsupportedJob(t *testing.T) {
	var output bytes.Buffer
	if err := NewWorkflowCheckCommand().Execute(context.Background(), []string{"--workflow", "workflow.yml", "--job", "publish"}, &output); err == nil {
		t.Fatal("Execute() succeeded, want unsupported job error")
	}
}
