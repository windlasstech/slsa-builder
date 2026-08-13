package workflowcheck

import (
	"strings"
	"testing"
)

const sourceRefExpression = "${{ inputs.source-ref || github.ref }}"

func TestNPMWorkflowSourceRefContract(t *testing.T) {
	valid := sourceRefWorkflow(t)
	if _, err := CheckNPMOnlyProfile(writeWorkflow(t, valid)); err != nil {
		t.Fatalf("CheckNPMOnlyProfile() rejected the source-ref contract: %v", err)
	}

	tests := map[string]string{
		"missing source-ref input": replaceOnce(t, valid,
			"      source-ref: {required: false, type: string}\n", ""),
		"string default on source-ref": replaceOnce(t, valid,
			"source-ref: {required: false, type: string}",
			"source-ref: {required: false, type: string, default: refs/tags/v1.0.0}"),
		"build group uses ref name": replaceOnce(t, valid,
			"npm-build-${{ github.repository }}-"+sourceRefExpression,
			"npm-build-${{ github.repository }}-${{ github.ref_name }}"),
		"sign group uses ref name": replaceOnce(t, valid,
			"npm-provenance-sign-${{ github.repository }}-"+sourceRefExpression,
			"npm-provenance-sign-${{ github.repository }}-${{ github.ref_name }}"),
		"publish group uses ref name": replaceOnce(t, valid,
			"release-mutation-${{ github.repository }}-"+sourceRefExpression,
			"release-mutation-${{ github.repository }}-${{ github.ref_name }}"),
		"dispatch retry group uses invocation ref": replaceOnce(t, valid,
			"npm-build-${{ github.repository }}-"+sourceRefExpression,
			"npm-build-${{ github.repository }}-${{ github.ref }}"),
		"group uses workflow name": replaceOnce(t, valid,
			"release-mutation-${{ github.repository }}-"+sourceRefExpression,
			"release-mutation-${{ github.workflow }}-"+sourceRefExpression),
		"package checkout has no ref": replaceOnce(t, valid,
			"          ref: ${{ steps.source-ref.outputs.built-revision }}\n", ""),
		"package checkout uses tag": replaceOnce(t, valid,
			"ref: ${{ steps.source-ref.outputs.built-revision }}", "ref: ${{ inputs.source-ref }}"),
		"resolver follows package checkout": resolverAfterCheckout(t, valid),
		"build omits invocation arguments": replaceOnce(t, valid,
			" --source-ref \"$SOURCE_REF\" --invocation-ref \"$INVOCATION_REF\" --invocation-revision \"$INVOCATION_REVISION\"", ""),
		"build job has queue": replaceOnce(t, valid,
			"      cancel-in-progress: true\n    steps:", "      cancel-in-progress: true\n      queue: max\n    steps:"),
		"sign job has queue": replaceOnce(t, valid,
			"npm-provenance-sign-${{ github.repository }}-"+sourceRefExpression+"\n      cancel-in-progress: true",
			"npm-provenance-sign-${{ github.repository }}-"+sourceRefExpression+"\n      cancel-in-progress: true\n      queue: max"),
		"publish is cancellable": replaceOnce(t, valid,
			"cancel-in-progress: false\n      queue: max", "cancel-in-progress: true\n      queue: max"),
		"publish has no queue": replaceOnce(t, valid, "      queue: max\n", ""),
	}

	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := CheckNPMOnlyProfile(writeWorkflow(t, contents)); err == nil {
				t.Fatal("CheckNPMOnlyProfile() succeeded, want rejection")
			}
		})
	}
}

func resolverAfterCheckout(t *testing.T, workflow string) string {
	t.Helper()
	resolverStart := strings.Index(workflow, "      - id: source-ref\n")
	checkoutStart := strings.Index(workflow[resolverStart:], "      - uses: actions/checkout@") + resolverStart
	setupNodeStart := strings.Index(workflow[checkoutStart:], "      - uses: actions/setup-node@") + checkoutStart
	if resolverStart < 0 || checkoutStart < resolverStart || setupNodeStart < checkoutStart {
		t.Fatal("source-ref fixture does not contain the expected resolver and checkout sequence")
	}
	resolver := workflow[resolverStart:checkoutStart]
	return workflow[:resolverStart] + workflow[checkoutStart:setupNodeStart] + resolver + workflow[setupNodeStart:]
}

const sourceResolverSteps = `      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          repository: ${{ job.workflow_repository }}
          ref: ${{ job.workflow_sha }}
          path: .slsa-builder
          persist-credentials: false
      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16
        with: {go-version: "1.26.5", cache: false}
      - id: source-ref
        working-directory: .slsa-builder
        env:
          SOURCE_REF: ${{ inputs.source-ref }}
          INVOCATION_REF: ${{ github.ref }}
          INVOCATION_REF_TYPE: ${{ github.ref_type }}
          INVOCATION_REVISION: ${{ github.sha }}
          OBSERVED_REPOSITORY: ${{ github.repository }}
        run: go run ./cmd/slsa-builder-internal npm-profile-source-ref --source-ref "$SOURCE_REF" --ref "$INVOCATION_REF" --ref-type "$INVOCATION_REF_TYPE" --revision "$INVOCATION_REVISION" --observed-repository "$OBSERVED_REPOSITORY" --github-output
`

const packageCheckoutStep = `      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0
        with:
          ref: ${{ steps.source-ref.outputs.built-revision }}
          path: source
          persist-credentials: false
`

func sourceRefWorkflow(t *testing.T) string {
	t.Helper()
	if strings.Contains(validNPMOnlyWorkflow, "source-ref: {required: false, type: string}") {
		return validNPMOnlyWorkflow
	}
	workflow := replaceOnce(t, validNPMOnlyWorkflow,
		"      registry-url: {required: false, type: string}\n",
		"      source-ref: {required: false, type: string}\n      registry-url: {required: false, type: string}\n")
	workflow = strings.ReplaceAll(workflow, "${{ github.ref_name }}", sourceRefExpression)
	workflow = replaceOnce(t, workflow,
		"      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0\n        with: {persist-credentials: false}\n",
		sourceResolverSteps+packageCheckoutStep)
	workflow = replaceOnce(t, workflow,
		"      - run: go run ./cmd/slsa-builder-internal npm-profile-build\n",
		`      - env:
          SOURCE_REF: ${{ inputs.source-ref }}
          INVOCATION_REF: ${{ github.ref }}
          INVOCATION_REVISION: ${{ github.sha }}
          REF: ${{ steps.source-ref.outputs.built-ref }}
          REVISION: ${{ steps.source-ref.outputs.built-revision }}
        run: go run ./cmd/slsa-builder-internal npm-profile-build --source-ref "$SOURCE_REF" --invocation-ref "$INVOCATION_REF" --invocation-revision "$INVOCATION_REVISION" --ref "$REF" --revision "$REVISION"
`)
	return workflow
}
