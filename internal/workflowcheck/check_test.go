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
      group: npm-build-${{ github.repository }}-${{ inputs.source-ref || github.ref }}
      cancel-in-progress: true
    steps:
      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411 # v2.19.4
        with:
          egress-policy: audit
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
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
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
        with:
          ref: ${{ steps.source-ref.outputs.built-revision }}
          path: source
          persist-credentials: false
      - id: node-provision
        working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-provision-node --output-directory "${{ runner.temp }}/windlass-node-runtime" --github-output "$GITHUB_OUTPUT"
      - run: echo "${{ steps.node-provision.outputs.node-bin }}" >> "$GITHUB_PATH"
      - working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert
      - run: npm --version
      - run: corepack enable
      - run: go run ./cmd/slsa-builder-internal npm-profile-build --source-ref "$SOURCE_REF" --invocation-ref "$INVOCATION_REF" --invocation-revision "$INVOCATION_REVISION" --ref "$REF" --revision "$REVISION"
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
    outputs:
      built-ref: ${{ steps.source-ref.outputs.built-ref }}
      built-revision: ${{ steps.source-ref.outputs.built-revision }}
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
      - id: node-provision
        working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-provision-node --output-directory "${{ runner.temp }}/windlass-node-runtime" --github-output "$GITHUB_OUTPUT"
      - run: echo "${{ steps.node-provision.outputs.node-bin }}" >> "$GITHUB_PATH"
      - working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert
      - run: npm --version
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with:
          name: js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}
      - uses: actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093
        with:
          name: js-ts-npm-build-metadata-${{ github.run_id }}-${{ github.run_attempt }}
      - env:
          BUILT_REF: ${{ needs.build.outputs.built-ref }}
          BUILT_REVISION: ${{ needs.build.outputs.built-revision }}
        run: go run ./cmd/slsa-builder-internal npm-profile-sign --built-ref "$BUILT_REF" --built-revision "$BUILT_REVISION"
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
      source-ref: {required: false, type: string}
      registry-url: {required: false, type: string}
      dist-tag: {required: false, type: string}
      access: {required: false, type: string}
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
      group: npm-build-${{ github.repository }}-${{ inputs.source-ref || github.ref }}
      cancel-in-progress: true
    outputs:
      built-ref: ${{ steps.source-ref.outputs.built-ref }}
      built-revision: ${{ steps.source-ref.outputs.built-revision }}
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
      - id: node-provision
        working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-provision-node --output-directory "${{ runner.temp }}/windlass-node-runtime" --github-output "$GITHUB_OUTPUT"
      - run: echo "${{ steps.node-provision.outputs.node-bin }}" >> "$GITHUB_PATH"
      - working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert
      - run: npm --version
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
          persist-credentials: false
      - id: node-provision
        working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-provision-node --output-directory "${{ runner.temp }}/windlass-node-runtime" --github-output "$GITHUB_OUTPUT"
      - run: echo "${{ steps.node-provision.outputs.node-bin }}" >> "$GITHUB_PATH"
      - working-directory: .slsa-builder
        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert
      - run: npm --version
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
          name: "js-ts-npm-provenance-bundle-${{ github.run_id }}-${{ github.run_attempt }}"
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
      - id: node-provision
        if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'
        working-directory: .slsa-builder
        env: {REPORT_PATH: "${{ runner.temp }}/windlass-npm-outcome-report.json"}
        run: go run ./cmd/slsa-builder-internal npm-profile-provision-node --output-directory "${{ runner.temp }}/windlass-node-runtime" --github-output "$GITHUB_OUTPUT" --report-path "$REPORT_PATH"
      - if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'
        run: echo "${{ steps.node-provision.outputs.node-bin }}" >> "$GITHUB_PATH"
      - working-directory: .slsa-builder
        if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'
        env: {REPORT_PATH: "${{ runner.temp }}/windlass-npm-outcome-report.json"}
        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert --node-only --report-path "$REPORT_PATH"
      - id: npm-provision
        if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'
        working-directory: .slsa-builder
        env: {REPORT_PATH: "${{ runner.temp }}/windlass-npm-outcome-report.json"}
        run: go run ./cmd/slsa-builder-internal npm-profile-provision-npm --output-directory "${{ runner.temp }}/windlass-npm-cli" --github-output "$GITHUB_OUTPUT" --report-path "$REPORT_PATH"
      - if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'
        run: '"${{ steps.npm-provision.outputs.npm-executable }}" --version'
      - id: publish
        if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'
        run: go run ./cmd/slsa-builder-internal npm-profile-publish --npm-executable "${{ steps.npm-provision.outputs.npm-executable }}"
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
		"wrong node output": replaceOnce(t, validBuildWorkflow, "outputs.node-bin", "outputs.other"),
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
	if result.MutationKey != "release-mutation-${{ github.repository }}-${{ inputs.source-ref || github.ref }}" || result.CancelInProgress || result.Queue != "max" {
		t.Fatalf("unexpected publish concurrency: %#v", result)
	}
}

func TestCheckPublishJobRejectsBoundaryDrift(t *testing.T) {
	tests := map[string]string{
		"wrong dependency":     replaceOnce(t, validNPMOnlyWorkflow, "needs: [build, provenance-sign]", "needs: build"),
		"cancellable mutation": replaceOnce(t, validNPMOnlyWorkflow, "cancel-in-progress: false\n      queue: max", "cancel-in-progress: true\n      queue: max"),
		"missing queue":        replaceOnce(t, validNPMOnlyWorkflow, "      queue: max\n", ""),
		"wrong mutation key":   replaceOnce(t, validNPMOnlyWorkflow, "release-mutation-${{ github.repository }}-${{ inputs.source-ref || github.ref }}", "release-mutation-${{ github.workflow }}"),
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
		"missing input":      replaceOnce(t, validNPMOnlyWorkflow, "      registry-url: {required: false, type: string}\n", ""),
		"string default":     replaceOnce(t, validNPMOnlyWorkflow, "registry-url: {required: false, type: string}", "registry-url: {required: false, type: string, default: https://registry.npmjs.org/}"),
		"missing output":     replaceOnce(t, validNPMOnlyWorkflow, "      package-url: {value: \"${{ jobs.publish.outputs.package-url }}\"}\n", ""),
		"extra job":          replaceOnce(t, validNPMOnlyWorkflow, "jobs:\n", "jobs:\n  extra:\n    runs-on: ubuntu-24.04\n    steps: []\n"),
		"pre-mutation queue": replaceOnce(t, validNPMOnlyWorkflow, "      cancel-in-progress: true\n    steps:", "      cancel-in-progress: true\n      queue: max\n    steps:"),
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

func TestToolchainContractRejectsDrift(t *testing.T) {
	t.Parallel()
	const pairStep = "      - working-directory: .slsa-builder\n        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert\n"
	const publishGate = "if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'\n        "
	const nodeOnlyStep = "      - working-directory: .slsa-builder\n        " + publishGate + "env: {REPORT_PATH: \"${{ runner.temp }}/windlass-npm-outcome-report.json\"}\n        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert --node-only --report-path \"$REPORT_PATH\"\n"
	const provisionStep = "      - id: npm-provision\n        " + publishGate + "working-directory: .slsa-builder\n        env: {REPORT_PATH: \"${{ runner.temp }}/windlass-npm-outcome-report.json\"}\n        run: go run ./cmd/slsa-builder-internal npm-profile-provision-npm --output-directory \"${{ runner.temp }}/windlass-npm-cli\" --github-output \"$GITHUB_OUTPUT\" --report-path \"$REPORT_PATH\"\n"
	const diagnostics = "      - if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'\n        run: '\"${{ steps.npm-provision.outputs.npm-executable }}\" --version'\n"
	const output = "${{ steps.npm-provision.outputs.npm-executable }}"

	for _, job := range []string{"build", "provenance-sign", "publish"} {
		t.Run(job, func(t *testing.T) {
			t.Parallel()
			// Given a valid job, change only its toolchain contract.
			prefix, body, ok := strings.Cut(validNPMOnlyWorkflow, "  "+job+":\n")
			if !ok {
				t.Fatal("missing fixture job")
			}
			tests := map[string]string{
				"wrong node output":            replaceOnce(t, body, "outputs.node-bin", "outputs.other"),
				"missing runtime provisioning": replaceOnce(t, body, "npm-profile-provision-node", "other-command"),
			}
			if job == "publish" {
				tests["missing provisioning"] = replaceOnce(t, body, provisionStep, "")
				tests["late provisioning"] = replaceOnce(t, body, provisionStep+diagnostics, diagnostics+provisionStep)
				tests["missing provision id"] = replaceOnce(t, body, "id: npm-provision", "name: Provision npm")
				tests["missing node-only assertion"] = replaceOnce(t, body, nodeOnlyStep, "")
				tests["pair assertion in publish"] = replaceOnce(t, body, " --node-only", "")
				tests["late node-only assertion"] = replaceOnce(t, body, nodeOnlyStep+provisionStep, provisionStep+nodeOnlyStep)
				tests["ambient diagnostics"] = replaceOnce(t, body, diagnostics, "      - run: npm --version\n")
				tests["ambient alongside output"] = replaceOnce(t, body, diagnostics, "      - run: |\n          \""+output+"\" --version\n          npm --version\n")
				tests["command lookup"] = replaceOnce(t, body, `--npm-executable "`+output+`"`, `--npm-executable "$(command -v npm)"`)
				tests["wrong output"] = replaceOnce(t, body, `--npm-executable "`+output+`"`, `--npm-executable "${{ steps.other.outputs.npm-executable }}"`)
				tests["missing executable flag"] = replaceOnce(t, body, ` --npm-executable "`+output+`"`, "")
				tests["missing github output"] = replaceOnce(t, body, provisionStep, strings.ReplaceAll(provisionStep, ` --github-output "$GITHUB_OUTPUT"`, ""))
				tests["missing assert report-path"] = replaceOnce(t, body, ` --node-only --report-path "$REPORT_PATH"`, " --node-only")
				tests["missing assert report env"] = replaceOnce(t, body, "        env: {REPORT_PATH: \"${{ runner.temp }}/windlass-npm-outcome-report.json\"}\n        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert", "        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert")
				tests["wrong assert report env"] = replaceOnce(t, body, "env: {REPORT_PATH: \"${{ runner.temp }}/windlass-npm-outcome-report.json\"}\n        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert", "env: {REPORT_PATH: \"${{ runner.temp }}/other.json\"}\n        run: go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert")
				tests["missing provision report-path"] = replaceOnce(t, body, provisionStep, strings.ReplaceAll(provisionStep, ` --report-path "$REPORT_PATH"`, ""))
				tests["wrong provision report env"] = replaceOnce(t, body, "env: {REPORT_PATH: \"${{ runner.temp }}/windlass-npm-outcome-report.json\"}\n        run: go run ./cmd/slsa-builder-internal npm-profile-provision-npm", "env: {REPORT_PATH: \"${{ runner.temp }}/other.json\"}\n        run: go run ./cmd/slsa-builder-internal npm-profile-provision-npm")
			} else {
				command := "npm-profile-build"
				if job == "provenance-sign" {
					command = "npm-profile-sign"
				}
				tests["npm command before assertion"] = replaceOnce(t, body, pairStep, "      - run: go run ./cmd/slsa-builder-internal "+command+"\n"+pairStep)
				tests["missing pair assertion"] = replaceOnce(t, body, pairStep, "")
				tests["late pair assertion"] = replaceOnce(t, body, pairStep+"      - run: npm --version\n", "      - run: npm --version\n"+pairStep)
				tests["node-only instead of pair"] = replaceOnce(t, body, pairStep, strings.ReplaceAll(pairStep, "toolchain-assert\n", "toolchain-assert --node-only\n"))
				tests["commented assertion"] = replaceOnce(t, body, pairStep, "      - working-directory: .slsa-builder\n        run: '# go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert'\n")
			}
			for name, changed := range tests {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					path := writeWorkflow(t, prefix+"  "+job+":\n"+changed)
					// When
					var err error
					switch job {
					case "build":
						_, err = CheckBuildJob(path)
					case "provenance-sign":
						_, err = CheckProvenanceSignJob(path)
					case "publish":
						_, err = CheckPublishJob(path)
					}
					// Then
					if err == nil {
						t.Fatal("toolchain drift accepted, want rejection")
					}
				})
			}
		})
	}
}

func TestPublishToolchainAcceptsProvisionOutput(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"npm-provision", "verified-cli"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			// Given
			contents := strings.ReplaceAll(validNPMOnlyWorkflow, "npm-provision", id)
			contents = replaceOnce(t, contents, "      - if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'\n        run: '\"${{ steps."+id+".outputs.npm-executable }}\" --version'",
				"      - if: needs.build.result == 'success' && needs.provenance-sign.result == 'success'\n        run: |\n          echo \"node: $(node --version), npm: $(\"${{ steps."+id+".outputs.npm-executable }}\" --version)\"\n          \"${{ steps."+id+".outputs.npm-executable }}\" config ls")
			// When
			_, err := CheckPublishJob(writeWorkflow(t, contents))
			// Then
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func replaceOnce(t *testing.T, contents, old, replacement string) string {
	t.Helper()
	index := strings.Index(contents, old)
	if index < 0 {
		t.Fatalf("test fixture does not contain %q", old)
	}
	return contents[:index] + replacement + contents[index+len(old):]
}
