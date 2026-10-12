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

const workflowCheckFixture = `permissions: {}
on:
  workflow_call:
    inputs:
      package-directory: {required: true, type: string}
      source-ref: {type: string}
      registry-url: {type: string}
      dist-tag: {type: string}
      access: {type: string}
      release-asset-mode: {type: boolean, default: false}
      release-tag: {type: string}
      provenance-sidecar: {type: string}
      linked-artifact-metadata: {type: boolean, default: false}
    outputs:
      package-name: {value: "${{ jobs.publish.outputs.package-name }}"}
      package-version: {value: "${{ jobs.publish.outputs.package-version }}"}
      package-registry-url: {value: "${{ jobs.publish.outputs.package-registry-url }}"}
      package-url: {value: "${{ jobs.publish.outputs.package-url }}"}
      package-tarball-name: {value: "${{ jobs.publish.outputs.package-tarball-name }}"}
      package-tarball-sha256: {value: "${{ jobs.publish.outputs.package-tarball-sha256 }}"}
      package-tarball-sha512: {value: "${{ jobs.publish.outputs.package-tarball-sha512 }}"}
jobs:
  build:
    runs-on: ubuntu-24.04
    permissions: {contents: read}
    outputs:
      built-ref: ${{ steps.source-ref.outputs.built-ref }}
      built-revision: ${{ steps.source-ref.outputs.built-revision }}
    concurrency:
      group: npm-build-${{ github.repository }}-${{ inputs.source-ref || github.ref }}
      cancel-in-progress: true
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411
        with: {egress-policy: audit}
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .slsa-builder
          persist-credentials: false
      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16
        with: {go-version: "1.26.5", cache: false}
      - id: source-ref
        working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-source-ref --source-ref "$SOURCE_REF" --ref "$INVOCATION_REF" --ref-type "$INVOCATION_REF_TYPE" --revision "$INVOCATION_REVISION" --observed-repository "$OBSERVED_REPOSITORY" --github-output "$GITHUB_OUTPUT"
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          ref: ${{ steps.source-ref.outputs.built-revision }}
          path: source
          persist-credentials: false
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
        with: {node-version: "24.21.0"}
      - working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert
      - run: corepack enable
      - run: go run ./cmd/slsa-builder-internal npm-profile-build --source-ref "$SOURCE_REF" --invocation-ref "$INVOCATION_REF" --invocation-revision "$INVOCATION_REVISION" --ref "$REF" --revision "$REVISION"
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with: {name: "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}"}
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with: {name: "js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}"}
  provenance-sign:
    needs: build
    runs-on: ubuntu-24.04
    permissions: {contents: read, id-token: write}
    concurrency:
      group: npm-provenance-sign-${{ github.repository }}-${{ inputs.source-ref || github.ref }}
      cancel-in-progress: true
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411
        with: {egress-policy: audit}
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .slsa-builder
          persist-credentials: false
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
        with: {node-version: "24.21.0"}
      - working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with: {name: "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}"}
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with: {name: "js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}"}
      - env:
          BUILT_REF: ${{ needs.build.outputs.built-ref }}
          BUILT_REVISION: ${{ needs.build.outputs.built-revision }}
        run: go run ./cmd/slsa-builder-internal npm-profile-sign --built-ref "$BUILT_REF" --built-revision "$BUILT_REVISION"
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with:
          name: js-ts-npm-provenance-bundle-${{ github.run_id }}-${{ github.run_attempt }}
          path: ${{ runner.temp }}/windlass-provenance/*.intoto.jsonl
  publish:
    if: always()
    needs: [build, provenance-sign]
    runs-on: ubuntu-24.04
    permissions: {contents: read, id-token: write}
    concurrency:
      group: release-mutation-${{ github.repository }}-${{ inputs.source-ref || github.ref }}
      cancel-in-progress: false
      queue: max
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411
        with: {egress-policy: audit}
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
        with: {node-version: "24.21.0"}
      - working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert --node-only
      - id: npm-provision
        working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-provision-npm --output-directory "${{ runner.temp }}/windlass-npm-cli" --github-output "$GITHUB_OUTPUT"
      - run: '"${{ steps.npm-provision.outputs.npm-executable }}" --version'
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with: {name: "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}"}
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with: {name: "js-ts-npm-provenance-bundle-${{ github.run_id }}-${{ github.run_attempt }}"}
      - run: go run ./cmd/slsa-builder-internal npm-profile-publish --npm-executable "${{ steps.npm-provision.outputs.npm-executable }}"
      - if: always()
        run: go run ./cmd/slsa-builder-internal npm-profile-report
      - if: always()
        uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with: {name: "js-ts-npm-outcome-report-${{ github.run_id }}-${{ github.run_attempt }}"}
`

func writeWorkflowCheckFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workflow.yml")
	if err := os.WriteFile(path, []byte(workflowCheckFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWorkflowCheckCommand(t *testing.T) {
	path := writeWorkflowCheckFixture(t)
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
	if err := NewWorkflowCheckCommand().Execute(context.Background(), []string{"--workflow", "workflow.yml", "--job", "unknown"}, &output); err == nil {
		t.Fatal("Execute() succeeded, want unsupported job error")
	}
}

func TestWorkflowCheckCommandRejectsAmbiguousMode(t *testing.T) {
	var output bytes.Buffer
	if err := NewWorkflowCheckCommand().Execute(context.Background(), []string{"--workflow", "workflow.yml", "--job", "build", "--profile", "npm-only"}, &output); err == nil {
		t.Fatal("Execute() succeeded, want mutually exclusive mode error")
	}
}
