package dependabot

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestCompanionWorkflowRejectsExecutionDrift(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, old, replacement, path string }{
		{"workflow env", "permissions: {}", "env: {BASH_ENV: payload.sh}\npermissions: {}", "workflow.env"},
		{"workflow defaults", "permissions: {}", "defaults: {run: {shell: sh}}\npermissions: {}", "workflow.defaults"},
		{"workflow future field", "permissions: {}", "future-execution: true\npermissions: {}", "workflow.future-execution"},
		{"trigger filter", "types: [completed]", "types: [completed]\n    branches: [main]", "workflow.on.workflow_run.branches"},
		{"concurrency field", "cancel-in-progress: false", "cancel-in-progress: false\n  queue: max", "workflow.concurrency.queue"},
		{"concurrency prefix", "dependabot-companion-${{", "unrelated-${{", "workflow.concurrency.group"},
		{"missing permissions", "permissions: {}", "", "workflow.permissions"},
		{"null permissions", "permissions: {}", "permissions: null", "workflow.permissions"},
		{"missing cancellation", "  cancel-in-progress: false", "", "workflow.concurrency.cancel-in-progress"},
		{"job container", "  plan:", "  plan:\n    container: attacker/image", "workflow.jobs.plan.container"},
		{"job services", "  plan:", "  plan:\n    services: {db: {image: attacker/image}}", "workflow.jobs.plan.services"},
		{"job defaults", "  plan:", "  plan:\n    defaults: {run: {shell: sh}}", "workflow.jobs.plan.defaults"},
		{"job env", "  apply:", "  apply:\n    env: {BASH_ENV: payload.sh}", "workflow.jobs.apply.env"},
		{"job future field", "  apply:", "  apply:\n    future-execution: true", "workflow.jobs.apply.future-execution"},
		{"job continue on error", "  plan:", "  plan:\n    continue-on-error: true", "workflow.jobs.plan.continue-on-error"},
		{"step shell", "run: go mod download", "shell: sh\n        run: go mod download", "workflow.jobs.plan.steps[3].shell"},
		{"step directory", "run: go mod download", "working-directory: candidate\n        run: go mod download", "workflow.jobs.plan.steps[3].working-directory"},
		{"step condition", "run: go mod download", "if: always()\n        run: go mod download", "workflow.jobs.plan.steps[3].if"},
		{"step future field", "run: go mod download", "future-execution: true\n        run: go mod download", "workflow.jobs.plan.steps[3].future-execution"},
		{"extra app permission", "permission-contents: write", "permission-issues: write\n          permission-contents: write", "workflow.jobs.apply.steps[4].with.permission-issues"},
		{"extra ordinary endpoint", "api.github.com:443 github.com:443", "attacker.example:443 api.github.com:443 github.com:443", "workflow.jobs.plan.steps[0].with.allowed-endpoints"},
		{"missing ordinary endpoint", "mise.jdx.dev:443 ", "", "workflow.jobs.plan.steps[0].with.allowed-endpoints"},
		{"substituted ordinary endpoint", "mise.jdx.dev:443", "attacker.example:443", "workflow.jobs.plan.steps[0].with.allowed-endpoints"},
		{"missing trusted repository predicate", "      github.event.workflow_run.head_repository.full_name == github.repository &&\n", "", "workflow.jobs.plan.if"},
		{"missing dependabot predicate", " &&\n      startsWith(github.event.workflow_run.head_branch, 'dependabot/')", "", "workflow.jobs.plan.if"},
		{"weakened plan conjunction", "'pull_request' &&", "'pull_request' ||", "workflow.jobs.plan.if"},
		{"checkout token", "ref: main", "token: ${{ secrets.TOKEN }}\n          ref: main", "workflow.jobs.plan.steps[1].with.token"},
		{"checkout path", "ref: main", "path: candidate\n          ref: main", "workflow.jobs.plan.steps[1].with.path"},
		{"checkout id", "id: trusted", "id: candidate", "workflow.jobs.plan.steps[1].id"},
		{"harden input", "egress-policy: block", "disable-sudo: false\n          egress-policy: block", "workflow.jobs.plan.steps[0].with.disable-sudo"},
		{"mise input", "install_args: \"--locked go\"", "install_args: \"go\"", "workflow.jobs.plan.steps[2].with.install_args"},
		{"module environment", "run: go mod download", "env: {GOPROXY: 'https://attacker.example'}\n        run: go mod download", "workflow.jobs.plan.steps[3].env"},
		{"invocation environment", "TRUSTED_SHA: ${{ steps.trusted.outputs.commit }}", "BASH_ENV: payload.sh\n          TRUSTED_SHA: ${{ steps.trusted.outputs.commit }}", "workflow.jobs.plan.steps[4].env.BASH_ENV"},
		{"plan token source", "GITHUB_TOKEN: ${{ github.token }}", "GITHUB_TOKEN: other", "workflow.jobs.plan.steps[4].env.GITHUB_TOKEN"},
		{"app id source", "app-id: ${{ vars.DEPENDABOT_COMPANION_APP_ID }}", "app-id: 123", "workflow.jobs.apply.steps[4].with.app-id"},
		{"app key source", "private-key: ${{ secrets.DEPENDABOT_COMPANION_APP_PRIVATE_KEY }}", "private-key: other", "workflow.jobs.apply.steps[4].with.private-key"},
		{"app step id", "id: app", "id: other", "workflow.jobs.apply.steps[4].id"},
		{"other pinned checkout", "3d3c42e5aac5ba805825da76410c181273ba90b1", "0000000000000000000000000000000000000000", "workflow.jobs.plan.steps[1].uses"},
		{"other pinned hardener", "1d88384fd173af6ecb012dfd0264a306b711241a", "0000000000000000000000000000000000000000", "workflow.jobs.plan.steps[0].uses"},
		{"other pinned mise", "a225bbd7c202ddf57dd76dff22bb7992f6314173", "0000000000000000000000000000000000000000", "workflow.jobs.plan.steps[2].uses"},
		{"changed timeout", "timeout-minutes: 15", "timeout-minutes: 16", "workflow.jobs.plan.timeout-minutes"},
		{"string timeout", "timeout-minutes: 15", "timeout-minutes: '15'", "workflow.jobs.plan.timeout-minutes"},
		{"string cancellation", "cancel-in-progress: false", "cancel-in-progress: 'false'", "workflow.concurrency.cancel-in-progress"},
		{"extra trigger", "on:\n", "on:\n  workflow_dispatch:\n", "workflow.on.workflow_dispatch"},
		{"missing event types", "    types: [completed]\n", "", "workflow.on.workflow_run.types"},
		{"trigger scalar", "workflows: [Lint]", "workflows: Lint", "workflow.on.workflow_run.workflows"},
		{"extra source workflow", "workflows: [Lint]", "workflows: [Lint, Other]", "workflow.on.workflow_run.workflows"},
		{"extra job", "jobs:\n", "jobs:\n  extra: {}\n", "workflow.jobs.extra"},
		{"runner sequence", "runs-on: ubuntu-24.04", "runs-on: [ubuntu-24.04]", "workflow.jobs.plan.runs-on"},
		{"missing apply dependency", "    needs: plan\n", "", "workflow.jobs.apply.needs"},
		{"apply always", "if: vars.DEPENDABOT_COMPANION_APPLY_ENABLED == 'true'", "if: always() || vars.DEPENDABOT_COMPANION_APPLY_ENABLED == 'true'", "workflow.jobs.apply.if"},
		{"extra job permission", "      contents: read", "      issues: write\n      contents: read", "workflow.jobs.plan.permissions.issues"},
		{"missing job permission", "      contents: read\n", "", "workflow.jobs.plan.permissions.contents"},
		{"permission shape", "permissions: {}", "permissions: []", "workflow.permissions"},
		{"extra step", "      - name: Read-only plan", "      - run: go mod download\n      - name: Read-only plan", "workflow.jobs.plan.steps"},
		{"missing module step", "      - name: Cache trusted base modules\n        run: go mod download\n", "", "workflow.jobs.plan.steps"},
		{"missing checkout input", "          ref: main\n", "", "workflow.jobs.plan.steps[1].with.ref"},
		{"with shape", "with:\n          install_args: \"--locked go\"", "with: [go]", "workflow.jobs.plan.steps[2].with"},
		{"missing env binding", "          TRUSTED_SHA: ${{ steps.trusted.outputs.commit }}\n", "", "workflow.jobs.plan.steps[4].env.TRUSTED_SHA"},
		{"env shape", "env:\n          GITHUB_TOKEN: ${{ github.token }}\n          TRUSTED_SHA: ${{ steps.trusted.outputs.commit }}", "env: [TOKEN]", "workflow.jobs.plan.steps[4].env"},
		{"upload hidden files", "if-no-files-found: error", "if-no-files-found: error\n          include-hidden-files: true", "workflow.jobs.plan.steps[5].with.include-hidden-files"},
		{"upload name", "dependabot-companion-plan-${{", "other-plan-${{", "workflow.jobs.plan.steps[5].with.name"},
		{"missing app scope", "          repositories: slsa-builder\n", "", "workflow.jobs.apply.steps[4].with.repositories"},
		{"run shape", "run: go mod download", "run: [go, mod, download]", "workflow.jobs.plan.steps[3].run"},
		{"run newline command split", "run: go mod download", "run: |\n          go mod\n          download", "workflow.jobs.plan.steps[3].run"},
		{"quoted argument whitespace", "--event\n          \"$GITHUB_EVENT_PATH\"", "--event\n          \" $GITHUB_EVENT_PATH\"", "workflow.jobs.plan.steps[4].run"},
		{"literal predicate whitespace", "'dependabot/'", "'dependabot/ '", "workflow.jobs.plan.if"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given: a single syntactically valid drift from the checked-in workflow.
			raw, err := os.ReadFile("../../.github/workflows/dependabot-companion.yml")
			if err != nil {
				t.Fatal(err)
			}
			mutated := strings.Replace(string(raw), test.old, test.replacement, 1)
			if mutated == string(raw) {
				t.Fatalf("mutation target %q absent", test.old)
			}
			var document map[string]any
			if err := yaml.Unmarshal([]byte(mutated), &document); err != nil {
				t.Fatalf("mutation is not valid YAML: %v", err)
			}
			// When: check the mutated workflow, without executing any step.
			err = checkCompanionWorkflow([]byte(mutated))
			// Then: identify the exact contract field, not an incidental parse failure.
			var violation *companionWorkflowViolation
			if !errors.As(err, &violation) || violation.path != test.path {
				t.Fatalf("check = %v; want rejection at %s", err, test.path)
			}
		})
	}
}
