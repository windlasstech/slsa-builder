package workflowcheck

import (
	"fmt"
	"regexp"
	"strings"
)

const provisionNodeCommand = `go run ./cmd/slsa-builder-internal npm-profile-provision-node --output-directory "${{ runner.temp }}/windlass-node-runtime" --github-output "$GITHUB_OUTPUT"`

// Exclude subcommand names, output names, flags, and diagnostic labels.
var runtimeReferencePattern = regexp.MustCompile(`(^|[^a-zA-Z0-9_.-])(node|npm|npx|corepack)($|[^a-zA-Z0-9_.:-])`)

var runtimePathOverridePattern = regexp.MustCompile(`(^|[^a-zA-Z0-9_])PATH\s*=|/(node|npm|npx|corepack)($|[\s"';|&)])`)

func nodeBindingCommand(id string) string {
	return `echo "${{ steps.` + id + `.outputs.node-bin }}" >> "$GITHUB_PATH"`
}

func checkNodeProvisioning(job workflowJob, name string) error {
	if err := checkToolchainConditions(job.Steps); err != nil {
		return err
	}
	if err := checkNodeEnvironment(job.Env); err != nil {
		return err
	}
	provisionID := ""
	bound := false
	for _, step := range job.Steps {
		run := strings.TrimSpace(toolchainShellText(step.Run))
		if strings.Contains(strings.ToLower(step.Uses), "setup-node@") || runnerToolcacheReference(run) {
			return fmt.Errorf("%s job must not obtain Node.js from a setup action or runner toolcache", name)
		}
		if runtimePathOverridePattern.MatchString(run) {
			return fmt.Errorf("%s runtime use must resolve through the provisioned PATH", name)
		}
		if err := checkNodeEnvironment(step.Env); err != nil {
			return err
		}
		if strings.Contains(run, "npm-profile-provision-node") {
			want := provisionNodeCommand
			if name == "publish" {
				want += ` --report-path "$REPORT_PATH"`
				if step.Env["REPORT_PATH"] != outcomeReportPath {
					return fmt.Errorf("publish Node.js provisioning must persist the outcome report")
				}
			}
			if provisionID != "" || step.ID == "" || step.WorkingDirectory != ".slsa-builder" ||
				strings.Join(strings.Fields(run), " ") != want || (name != "publish" && strings.TrimSpace(step.If) != "") {
				return fmt.Errorf("%s job must provision Node.js unconditionally from the trusted checkout with an id, output directory, and GitHub output", name)
			}
			provisionID = step.ID
			continue
		}
		if strings.Contains(run, "GITHUB_PATH") || strings.Contains(run, ".outputs.node-bin") {
			if provisionID == "" || bound || run != nodeBindingCommand(provisionID) {
				return fmt.Errorf("%s job must bind PATH exactly to the Node.js provisioning output", name)
			}
			bound = true
			continue
		}
		if !bound && (runtimeReferencePattern.MatchString(run) || strings.Contains(run, "npm-profile-toolchain-assert") ||
			strings.Contains(run, "npm-profile-provision-npm") || strings.Contains(run, "npm-profile-build") ||
			strings.Contains(run, "npm-profile-sign") || strings.Contains(run, "npm-profile-publish")) {
			return fmt.Errorf("%s job must provision and bind Node.js before runtime use", name)
		}
	}
	if !bound {
		return fmt.Errorf("%s job must provision and bind Node.js", name)
	}
	return nil
}

func runnerToolcacheReference(value string) bool {
	return strings.Contains(value, "RUNNER_TOOL_CACHE") || strings.Contains(value, "runner.tool_cache") ||
		strings.Contains(value, "/opt/hostedtoolcache")
}

func checkNodeEnvironment(environment map[string]string) error {
	for key, value := range environment {
		if key == "PATH" || runnerToolcacheReference(value) {
			return fmt.Errorf("runtime environment must preserve the provisioned PATH, not a runner toolcache binding")
		}
	}
	return nil
}
