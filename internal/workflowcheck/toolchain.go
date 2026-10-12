package workflowcheck

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	toolchainAssertCommand = "go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert"
	provisionNPMCommand    = "go run ./cmd/slsa-builder-internal npm-profile-provision-npm"
)

// Exclude npm-profile subcommands, npm-executable flags, and diagnostic labels (npm:).
var npmReferencePattern = regexp.MustCompile(`(^|[^a-zA-Z0-9_.-])npm($|[^a-zA-Z0-9_.:-])`)

func checkPairAssertion(steps []workflowStep, job string) error {
	asserted := false
	nodeReady := false
	for _, current := range steps {
		if strings.HasPrefix(current.Uses, "actions/setup-node@") {
			nodeReady = true
			asserted = false
		}
		if (npmReferencePattern.MatchString(current.Run) || strings.Contains(current.Run, "npm-profile-build") ||
			strings.Contains(current.Run, "npm-profile-sign")) && !asserted {
			return fmt.Errorf("%s toolchain pair assertion must precede every npm invocation", job)
		}
		if strings.Contains(current.Run, "npm-profile-toolchain-assert") {
			if !nodeReady || current.WorkingDirectory != ".slsa-builder" ||
				strings.TrimSpace(current.Run) != toolchainAssertCommand {
				return fmt.Errorf("%s job must invoke npm-profile-toolchain-assert from the trusted checkout after Node.js setup", job)
			}
			asserted = true
		}
	}
	if !asserted {
		return fmt.Errorf("%s job must invoke npm-profile-toolchain-assert before npm", job)
	}
	return nil
}

func checkPublishToolchain(steps []workflowStep) error {
	nodeReady := false
	asserted := false
	provisionID := ""
	for _, current := range steps {
		if strings.HasPrefix(current.Uses, "actions/setup-node@") {
			nodeReady = true
			asserted = false
			provisionID = ""
		}
		run := strings.ReplaceAll(current.Run, "\\\n", " ")
		if npmReferencePattern.MatchString(run) {
			return fmt.Errorf("publish job must use the provisioning output instead of ambient npm")
		}
		if strings.Contains(run, "npm-profile-toolchain-assert") {
			if !nodeReady || current.WorkingDirectory != ".slsa-builder" ||
				strings.TrimSpace(run) != toolchainAssertCommand+" --node-only" {
				return fmt.Errorf("publish job must invoke npm-profile-toolchain-assert --node-only after Node.js setup")
			}
			asserted = true
		}
		if strings.Contains(run, "npm-profile-provision-npm") {
			if !asserted || current.ID == "" || current.WorkingDirectory != ".slsa-builder" ||
				!strings.HasPrefix(strings.TrimSpace(run), provisionNPMCommand+" ") ||
				!containsAll(run, `--output-directory "${{ runner.temp }}/windlass-npm-cli"`, `--github-output "$GITHUB_OUTPUT"`) {
				return fmt.Errorf("publish npm provisioning must have an id and emit GitHub outputs after the node-only assertion")
			}
			provisionID = current.ID
		}
		if strings.Contains(run, "npm-executable") || strings.Contains(run, "npm-profile-publish") {
			if provisionID == "" {
				return fmt.Errorf("npm provisioning must precede publish diagnostics and npm-profile-publish")
			}
			output := "${{ steps." + provisionID + ".outputs.npm-executable }}"
			if !strings.Contains(run, output) || strings.Contains(strings.ReplaceAll(run, output, ""), ".outputs.npm-executable") {
				return fmt.Errorf("publish npm references must use %s", output)
			}
			if strings.Contains(run, "npm-profile-publish") && !strings.Contains(run, `--npm-executable "`+output+`"`) {
				return fmt.Errorf("npm-profile-publish --npm-executable must use %s", output)
			}
		}
	}
	if !asserted || provisionID == "" {
		return fmt.Errorf("publish job must invoke npm-profile-toolchain-assert --node-only and npm-profile-provision-npm")
	}
	return nil
}
