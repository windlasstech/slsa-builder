package workflowcheck

import (
	"fmt"
	"regexp"
	"strings"
)

const provisionNodeCommand = `go run ./cmd/slsa-builder-internal npm-profile-provision-node --output-directory "${{ runner.temp }}/windlass-node-runtime" --github-output "$GITHUB_OUTPUT"`

// Exclude subcommand names, output names, flags, and diagnostic labels.
var runtimeReferencePattern = regexp.MustCompile(`(^|[^a-zA-Z0-9_.-])(node|npm|npx|corepack)($|[^a-zA-Z0-9_.:-])`)

var runtimePathOverridePattern = regexp.MustCompile(`(^|[^a-zA-Z0-9_])PATH\s*\+?=|/(node|npm|npx|corepack)($|[\s"';|&)])`)

func nodeBindingCommand(id string) string {
	return `echo "${{ steps.` + id + `.outputs.node-bin }}" >> "$GITHUB_PATH"`
}

type toolchainSpine struct {
	provisionIndex int
	bindIndex      int
}

// toolchainPolicy states a job's toolchain requirements explicitly, so the
// validators never branch on a free-form job name.
type toolchainPolicy struct {
	job     string
	publish bool
}

func checkNodeProvisioning(job workflowJob, policy toolchainPolicy) (toolchainSpine, error) {
	if err := checkToolchainConditions(job.Steps); err != nil {
		return toolchainSpine{}, err
	}
	if err := checkNodeEnvironment(job.Env); err != nil {
		return toolchainSpine{}, err
	}
	spine := toolchainSpine{provisionIndex: -1, bindIndex: -1}
	for index, step := range job.Steps {
		run := exactToolchainCommand(toolchainShellText(step.Run))
		scan := toolchainInvocationText(run)
		if strings.Contains(strings.ToLower(step.Uses), "setup-node@") || runnerToolcacheReference(scan) {
			return toolchainSpine{}, fmt.Errorf("%s job must not obtain Node.js from a setup action or runner toolcache", policy.job)
		}
		if runtimePathOverridePattern.MatchString(scan) {
			return toolchainSpine{}, fmt.Errorf("%s runtime use must resolve through the provisioned PATH", policy.job)
		}
		if err := checkNodeEnvironment(step.Env); err != nil {
			return toolchainSpine{}, err
		}
		if strings.Contains(scan, "npm-profile-provision-node") {
			want := provisionNodeCommand
			if policy.publish {
				want += ` --report-path "$REPORT_PATH"`
				if step.Env["REPORT_PATH"] != outcomeReportPath {
					return toolchainSpine{}, fmt.Errorf("publish Node.js provisioning must persist the outcome report")
				}
			}
			if spine.provisionIndex >= 0 || step.ID == "" || step.WorkingDirectory != ".slsa-builder" ||
				strings.Join(strings.Fields(run), " ") != want || (!policy.publish && strings.TrimSpace(step.If) != "") {
				return toolchainSpine{}, fmt.Errorf("%s job must provision Node.js unconditionally from the trusted checkout with an id, output directory, and GitHub output", policy.job)
			}
			spine.provisionIndex = index
			continue
		}
		if strings.Contains(scan, "GITHUB_PATH") || strings.Contains(scan, ".outputs.node-bin") {
			if spine.provisionIndex < 0 || spine.bindIndex >= 0 || run != nodeBindingCommand(job.Steps[spine.provisionIndex].ID) {
				return toolchainSpine{}, fmt.Errorf("%s job must bind PATH exactly to the Node.js provisioning output", policy.job)
			}
			spine.bindIndex = index
			continue
		}
		if spine.bindIndex < 0 && (runtimeReferencePattern.MatchString(scan) || strings.Contains(scan, "npm-profile-toolchain-assert") ||
			strings.Contains(scan, "npm-profile-provision-npm") || strings.Contains(scan, "npm-profile-build") ||
			strings.Contains(scan, "npm-profile-sign") || strings.Contains(scan, "npm-profile-publish")) {
			return toolchainSpine{}, fmt.Errorf("%s job must provision and bind Node.js before runtime use", policy.job)
		}
	}
	if spine.bindIndex < 0 {
		return toolchainSpine{}, fmt.Errorf("%s job must provision and bind Node.js", policy.job)
	}
	return spine, nil
}

func runnerToolcacheReference(value string) bool {
	return strings.Contains(value, "RUNNER_TOOL_CACHE") || strings.Contains(value, "runner.tool_cache") ||
		strings.Contains(value, "/opt/hostedtoolcache")
}

func checkNodeEnvironment(environment map[string]string) error {
	for key, value := range environment {
		if key == "PATH" || key == "RUNNER_TOOL_CACHE" || runnerToolcacheReference(value) {
			return fmt.Errorf("runtime environment must preserve the provisioned PATH, not a runner toolcache binding")
		}
	}
	return nil
}
