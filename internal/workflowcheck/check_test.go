package workflowcheck

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const validBuildWorkflow = `name: npm package
permissions: {}
on:
  workflow_call:
    inputs:
      package-directory:
        required: true
        type: string
jobs:
  build:
    runs-on: ubuntu-24.04
    permissions:
      contents: read
    concurrency:
      group: npm-build-${{ github.repository }}-${{ github.ref_name }}
      cancel-in-progress: true
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411 # v2.19.4
        with:
          egress-policy: audit
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
        with:
          persist-credentials: false
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0
        with:
          node-version: "24"
      - run: corepack enable
      - run: go run ./cmd/slsa-builder-internal npm-profile-build
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4.6.2
        with:
          name: js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}
          path: .windlass/package/*.tgz
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4.6.2
        with:
          name: js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}
          path: .windlass/metadata/build-metadata.json
`

const validSigningWorkflow = `name: npm package
permissions: {}
jobs:
  build:
    runs-on: ubuntu-24.04
    permissions: {contents: read}
    steps: []
  provenance-sign:
    needs: build
    runs-on: ubuntu-24.04
    permissions:
      contents: read
      id-token: write
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411
        with: {egress-policy: audit}
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          persist-credentials: false
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with:
          name: js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with:
          name: js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}
      - run: go run ./cmd/slsa-builder-internal npm-profile-sign
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with:
          name: js-ts-npm-provenance-bundle-${{ github.run_id }}-${{ github.run_attempt }}
          path: ${{ runner.temp }}/windlass-provenance/*.intoto.jsonl
`

func TestCheckBuildJob(t *testing.T) {
	path := writeWorkflow(t, validBuildWorkflow)

	result, err := CheckBuildJob(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "pass" {
		t.Fatalf("result = %q, want pass", result.Result)
	}
	if result.SigningPermission {
		t.Error("build job must not have signing permission")
	}
	if result.MutationPermission {
		t.Error("build job must not have mutation permission")
	}
	wantArtifacts := []string{
		"js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}",
		"js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}",
	}
	if !slices.Equal(result.ArtifactNames, wantArtifacts) {
		t.Fatalf("artifact names = %q, want %q", result.ArtifactNames, wantArtifacts)
	}
}

func TestCheckBuildJobRejectsAuthorityAndHandoffDrift(t *testing.T) {
	tests := map[string]string{
		"signing permission":  replaceOnce(t, validBuildWorkflow, "contents: read", "contents: read\n      id-token: write"),
		"mutation permission": replaceOnce(t, validBuildWorkflow, "contents: read", "contents: write"),
		"wrong tarball name": replaceOnce(t, validBuildWorkflow,
			"js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}", "caller-controlled-name"),
		"wrong metadata name": replaceOnce(t, validBuildWorkflow,
			"js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}", "build-metadata"),
		"floating action": replaceOnce(t, validBuildWorkflow,
			"actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0", "actions/checkout@v7"),
		"wrong node": replaceOnce(t, validBuildWorkflow, `node-version: "24"`, `node-version: "22"`),
	}

	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := CheckBuildJob(writeWorkflow(t, contents)); err == nil {
				t.Fatal("CheckBuildJob() succeeded, want rejection")
			}
		})
	}
}

func TestCheckProvenanceSignJob(t *testing.T) {
	result, err := CheckProvenanceSignJob(writeWorkflow(t, validSigningWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "pass" || !result.OIDCPermission || result.AttestationStoragePermission || result.MutationPermission {
		t.Fatalf("unexpected signing result: %#v", result)
	}
	if result.BundleArtifactName != "js-ts-npm-provenance-bundle-${{ github.run_id }}-${{ github.run_attempt }}" {
		t.Fatalf("bundle artifact name = %q", result.BundleArtifactName)
	}
}

func TestCheckProvenanceSignJobRejectsBoundaryDrift(t *testing.T) {
	tests := map[string]string{
		"attestation storage":    replaceOnce(t, validSigningWorkflow, "id-token: write", "id-token: write\n      attestations: write"),
		"mutation permission":    replaceOnce(t, validSigningWorkflow, "contents: read\n      id-token: write", "contents: write\n      id-token: write"),
		"missing OIDC":           replaceOnce(t, validSigningWorkflow, "      id-token: write\n", ""),
		"wrong tarball handoff":  replaceOnce(t, validSigningWorkflow, tarballArtifactName, "caller-tarball"),
		"wrong metadata handoff": replaceOnce(t, validSigningWorkflow, metadataArtifactName, "caller-metadata"),
		"wrong bundle artifact":  replaceOnce(t, validSigningWorkflow, provenanceBundleArtifactName, "caller-bundle"),
		"caller checkout":        replaceOnce(t, validSigningWorkflow, "repository: ${{ job.workflow_repository }}", "repository: ${{ github.repository }}"),
	}

	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := CheckProvenanceSignJob(writeWorkflow(t, contents)); err == nil {
				t.Fatal("CheckProvenanceSignJob() succeeded, want rejection")
			}
		})
	}
}

func writeWorkflow(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workflow.yml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func replaceOnce(t *testing.T, contents, old, replacement string) string {
	t.Helper()
	index := strings.Index(contents, old)
	if index < 0 {
		t.Fatalf("test fixture does not contain %q", old)
	}
	return contents[:index] + replacement + contents[index+len(old):]
}
