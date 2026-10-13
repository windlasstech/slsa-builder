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

type toolchainSequence struct {
	provisionID    string
	bound          bool
	asserted       bool
	npmProvisionID string
}

// toolchainPolicy states a job's toolchain requirements explicitly, so the
// shared sequencing walk never branches on a free-form job name.
type toolchainPolicy struct {
	job     string
	publish bool
}

func checkToolchainSequence(job workflowJob, policy toolchainPolicy) error {
	if err := checkToolchainConditions(job.Steps); err != nil {
		return err
	}
	if err := checkNodeEnvironment(job.Env); err != nil {
		return err
	}
	var sequence toolchainSequence
	// Preserve the former validator order: Node.js errors, publish gate errors,
	// then the first assertion/npm error. Only sequence state is shared.
	var gateErr, usageErr error
	for _, step := range job.Steps {
		step.Run = toolchainShellText(step.Run)
		if err := sequence.checkNodeStep(step, policy); err != nil {
			return err
		}
		if policy.publish && (runtimeReferencePattern.MatchString(step.Run) || strings.Contains(step.Run, "npm-profile-provision-node") ||
			strings.Contains(step.Run, "GITHUB_PATH") || strings.Contains(step.Run, "npm-profile-toolchain-assert") ||
			strings.Contains(step.Run, "npm-profile-provision-npm") || strings.Contains(step.Run, "npm-profile-build") ||
			strings.Contains(step.Run, "npm-profile-sign") || strings.Contains(step.Run, "npm-profile-publish") ||
			strings.Contains(step.Run, "npm-executable") || strings.Contains(step.Run, "NPM_EXECUTABLE")) &&
			normalizeExpression(strings.Join(strings.Fields(normalizeExpression(step.If)), " ")) != publishToolchainGate {
			gateErr = fmt.Errorf("publish toolchain step conditions must use the upstream-success gate")
		}
		if usageErr == nil {
			usageErr = sequence.checkUsageStep(step, job, policy)
		}
	}
	if !sequence.bound {
		return fmt.Errorf("%s job must provision and bind Node.js", policy.job)
	}
	if gateErr != nil {
		return gateErr
	}
	if usageErr == nil {
		switch {
		case policy.publish && (!sequence.asserted || sequence.npmProvisionID == ""):
			usageErr = fmt.Errorf("publish job must invoke npm-profile-toolchain-assert --node-only and npm-profile-provision-npm")
		case !policy.publish && !sequence.asserted:
			usageErr = fmt.Errorf("%s job must invoke npm-profile-toolchain-assert before npm", policy.job)
		}
	}
	if usageErr != nil {
		return &toolchainUsageError{usageErr}
	}
	return nil
}

func (sequence *toolchainSequence) checkNodeStep(step workflowStep, policy toolchainPolicy) error {
	run := strings.TrimSpace(step.Run)
	if strings.Contains(strings.ToLower(step.Uses), "setup-node@") || runnerToolcacheReference(run) {
		return fmt.Errorf("%s job must not obtain Node.js from a setup action or runner toolcache", policy.job)
	}
	if runtimePathOverridePattern.MatchString(run) {
		return fmt.Errorf("%s runtime use must resolve through the provisioned PATH", policy.job)
	}
	if err := checkNodeEnvironment(step.Env); err != nil {
		return err
	}
	if strings.Contains(run, "npm-profile-provision-node") {
		want := provisionNodeCommand
		if policy.publish {
			want += ` --report-path "$REPORT_PATH"`
			if step.Env["REPORT_PATH"] != outcomeReportPath {
				return fmt.Errorf("publish Node.js provisioning must persist the outcome report")
			}
		}
		if sequence.provisionID != "" || step.ID == "" || step.WorkingDirectory != ".slsa-builder" ||
			strings.Join(strings.Fields(run), " ") != want || (!policy.publish && strings.TrimSpace(step.If) != "") {
			return fmt.Errorf("%s job must provision Node.js unconditionally from the trusted checkout with an id, output directory, and GitHub output", policy.job)
		}
		*sequence = toolchainSequence{provisionID: step.ID}
		return nil
	}
	if strings.Contains(run, "GITHUB_PATH") || strings.Contains(run, ".outputs.node-bin") {
		if sequence.provisionID == "" || sequence.bound || run != nodeBindingCommand(sequence.provisionID) {
			return fmt.Errorf("%s job must bind PATH exactly to the Node.js provisioning output", policy.job)
		}
		sequence.bound = true
		return nil
	}
	if !sequence.bound && (runtimeReferencePattern.MatchString(run) || strings.Contains(run, "npm-profile-toolchain-assert") ||
		strings.Contains(run, "npm-profile-provision-npm") || strings.Contains(run, "npm-profile-build") ||
		strings.Contains(run, "npm-profile-sign") || strings.Contains(run, "npm-profile-publish")) {
		return fmt.Errorf("%s job must provision and bind Node.js before runtime use", policy.job)
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
