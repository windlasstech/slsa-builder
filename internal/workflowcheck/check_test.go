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
      source-ref:
        required: false
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
      - id: source
        env:
          SOURCE_REF: ${{ inputs.source-ref }}
        run: go run ./cmd/slsa-builder-internal npm-profile-source --source-ref "$SOURCE_REF"
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
        with:
          ref: ${{ steps.source.outputs.revision }}
          path: source
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

const validNPMOnlyWorkflow = `name: npm package
permissions: {}
on:
  workflow_call:
    inputs:
      package-directory: {required: true, type: string}
      registry-url: {required: false, type: string}
      dist-tag: {required: false, type: string}
      access: {required: false, type: string}
      source-ref: {required: false, type: string}
      release-asset-mode: {required: false, type: boolean, default: false}
      release-tag: {required: false, type: string}
      provenance-sidecar: {required: false, type: string}
      linked-artifact-metadata: {required: false, type: boolean, default: false}
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
    concurrency:
      group: npm-build-${{ github.repository }}-${{ github.ref_name }}
      cancel-in-progress: true
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411
        with: {egress-policy: audit}
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          persist-credentials: false
      - id: source
        env:
          SOURCE_REF: ${{ inputs.source-ref }}
        run: go run ./cmd/slsa-builder-internal npm-profile-source --source-ref "$SOURCE_REF"
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          ref: ${{ steps.source.outputs.revision }}
          path: source
          persist-credentials: false
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
        with: {node-version: "24"}
      - run: corepack enable
      - run: go run ./cmd/slsa-builder-internal npm-profile-build
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with: {name: "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}"}
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with: {name: "js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}"}
  provenance-sign:
    needs: build
    runs-on: ubuntu-24.04
    permissions: {contents: read, id-token: write}
    concurrency:
      group: npm-provenance-sign-${{ github.repository }}-${{ github.ref_name }}
      cancel-in-progress: true
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411
        with: {egress-policy: audit}
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          persist-credentials: false
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with: {name: "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}"}
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with: {name: "js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}"}
      - run: go run ./cmd/slsa-builder-internal npm-profile-sign
      - uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with:
          name: "js-ts-npm-provenance-bundle-${{ github.run_id }}-${{ github.run_attempt }}"
          path: ${{ runner.temp }}/windlass-provenance/*.intoto.jsonl
  publish:
    if: always()
    needs: [build, provenance-sign]
    runs-on: ubuntu-24.04
    permissions: {contents: read, id-token: write}
    concurrency:
      group: release-mutation-${{ github.repository }}-${{ github.ref_name }}
      cancel-in-progress: false
      queue: max
    outputs:
      package-name: ${{ steps.publish.outputs.package-name }}
      package-version: ${{ steps.publish.outputs.package-version }}
      package-registry-url: ${{ steps.publish.outputs.package-registry-url }}
      package-url: ${{ steps.publish.outputs.package-url }}
      package-tarball-name: ${{ steps.publish.outputs.package-tarball-name }}
      package-tarball-sha256: ${{ steps.publish.outputs.package-tarball-sha256 }}
      package-tarball-sha512: ${{ steps.publish.outputs.package-tarball-sha512 }}
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411
        with: {egress-policy: audit}
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with: {name: "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}"}
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with: {name: "js-ts-npm-provenance-bundle-${{ github.run_id }}-${{ github.run_attempt }}"}
      - id: publish
        run: go run ./cmd/slsa-builder-internal npm-profile-publish
      - if: always()
        run: go run ./cmd/slsa-builder-internal npm-profile-report
      - if: always()
        uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with: {name: "js-ts-npm-outcome-report-${{ github.run_id }}-${{ github.run_attempt }}"}
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

func TestCheckPublishJob(t *testing.T) {
	result, err := CheckPublishJob(writeWorkflow(t, validNPMOnlyWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "pass" || !result.OIDCPermission || result.MutationPermission || !result.AlwaysRunReport {
		t.Fatalf("unexpected publish result: %#v", result)
	}
	if result.MutationKey != "release-mutation-${{ github.repository }}-${{ github.ref_name }}" || result.CancelInProgress || result.Queue != "max" {
		t.Fatalf("unexpected publish concurrency: %#v", result)
	}
}

func TestCheckPublishJobRejectsBoundaryDrift(t *testing.T) {
	tests := map[string]string{
		"wrong dependency":     replaceOnce(t, validNPMOnlyWorkflow, "needs: [build, provenance-sign]", "needs: build"),
		"cancellable mutation": replaceOnce(t, validNPMOnlyWorkflow, "cancel-in-progress: false\n      queue: max", "cancel-in-progress: true\n      queue: max"),
		"missing queue":        replaceOnce(t, validNPMOnlyWorkflow, "      queue: max\n", ""),
		"wrong mutation key":   replaceOnce(t, validNPMOnlyWorkflow, "release-mutation-${{ github.repository }}-${{ github.ref_name }}", "release-mutation-${{ github.workflow }}"),
		"mutation authority": replaceOnce(t, validNPMOnlyWorkflow,
			"permissions: {contents: read, id-token: write}\n    concurrency:\n      group: release-mutation-",
			"permissions: {contents: write, id-token: write}\n    concurrency:\n      group: release-mutation-"),
		"missing report": replaceOnce(t, validNPMOnlyWorkflow, "      - if: always()\n        run: go run ./cmd/slsa-builder-internal npm-profile-report\n", ""),
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := CheckPublishJob(writeWorkflow(t, contents)); err == nil {
				t.Fatal("CheckPublishJob() succeeded, want rejection")
			}
		})
	}
}

func TestCheckNPMOnlyProfile(t *testing.T) {
	result, err := CheckNPMOnlyProfile(writeWorkflow(t, validNPMOnlyWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	wantGraph := []string{"build", "provenance-sign", "publish"}
	if result.Result != "pass" || result.Profile != "npm-only" || !slices.Equal(result.Graph, wantGraph) {
		t.Fatalf("unexpected profile result: %#v", result)
	}
}

func TestCheckNPMOnlyProfileRejectsPublicContractDrift(t *testing.T) {
	tests := map[string]string{
		"missing input":       replaceOnce(t, validNPMOnlyWorkflow, "      registry-url: {required: false, type: string}\n", ""),
		"string default":      replaceOnce(t, validNPMOnlyWorkflow, "registry-url: {required: false, type: string}", "registry-url: {required: false, type: string, default: https://registry.npmjs.org/}"),
		"source ref required": replaceOnce(t, validNPMOnlyWorkflow, "source-ref: {required: false, type: string}", "source-ref: {required: true, type: string}"),
		"source ref default":  replaceOnce(t, validNPMOnlyWorkflow, "source-ref: {required: false, type: string}", "source-ref: {required: false, type: string, default: refs/tags/v1.2.3}"),
		"missing output":      replaceOnce(t, validNPMOnlyWorkflow, "      package-url: {value: \"${{ jobs.publish.outputs.package-url }}\"}\n", ""),
		"extra job":           replaceOnce(t, validNPMOnlyWorkflow, "jobs:\n", "jobs:\n  extra:\n    runs-on: ubuntu-24.04\n    steps: []\n"),
		"pre-mutation queue":  replaceOnce(t, validNPMOnlyWorkflow, "      cancel-in-progress: true\n    steps:", "      cancel-in-progress: true\n      queue: max\n    steps:"),
		"unresolved checkout": replaceOnce(t, validNPMOnlyWorkflow, "ref: ${{ steps.source.outputs.revision }}", "ref: ${{ inputs.source-ref }}"),
		"shell interpolation": replaceOnce(t, validNPMOnlyWorkflow, `--source-ref "$SOURCE_REF"`, `--source-ref "${{ inputs.source-ref }}"`),
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := CheckNPMOnlyProfile(writeWorkflow(t, contents)); err == nil {
				t.Fatal("CheckNPMOnlyProfile() succeeded, want rejection")
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
