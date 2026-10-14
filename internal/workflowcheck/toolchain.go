package workflowcheck

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	toolchainAssertCommand = "go run ./cmd/slsa-builder-internal npm-profile-toolchain-assert"
	provisionNPMCommand    = "go run ./cmd/slsa-builder-internal npm-profile-provision-npm"
	outcomeReportPath      = "${{ runner.temp }}/windlass-npm-outcome-report.json"
	publishToolchainGate   = "needs.build.result == 'success' && needs.provenance-sign.result == 'success'"
)

// Exclude npm-profile subcommands, npm-executable flags, and diagnostic labels (npm:).
var npmReferencePattern = regexp.MustCompile(`(^|[^a-zA-Z0-9_.-])npm($|[^a-zA-Z0-9_.:-])`)

// Capture the whole shell word, including concatenated quoted/unquoted fragments.
var npmExecutableArgumentPattern = regexp.MustCompile(`(?:^|[\s;|&])--npm-executable(?:[ \t]+|=)((?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s"';|&])+)`)

func checkPairAssertion(steps []workflowStep, policy toolchainPolicy, spine toolchainSpine) error {
	firstAssertIndex := len(steps)
	for index, current := range steps {
		run := toolchainShellText(current.Run)
		scan := toolchainInvocationText(run)
		if (npmReferencePattern.MatchString(scan) || strings.Contains(scan, "npm-profile-build") ||
			strings.Contains(scan, "npm-profile-sign")) && index <= firstAssertIndex {
			return fmt.Errorf("%s toolchain pair assertion must precede every npm invocation", policy.job)
		}
		if strings.Contains(scan, "npm-profile-toolchain-assert") {
			if index <= spine.bindIndex || current.WorkingDirectory != ".slsa-builder" || exactToolchainCommand(run) != toolchainAssertCommand {
				return fmt.Errorf("%s job must invoke npm-profile-toolchain-assert from the trusted checkout after Node.js setup", policy.job)
			}
			firstAssertIndex = min(firstAssertIndex, index)
		}
	}
	if firstAssertIndex == len(steps) {
		return fmt.Errorf("%s job must invoke npm-profile-toolchain-assert before npm", policy.job)
	}
	return nil
}

func checkPublishToolchain(job workflowJob, spine toolchainSpine) error {
	for _, step := range job.Steps {
		run := toolchainInvocationText(toolchainShellText(step.Run))
		if (runtimeReferencePattern.MatchString(run) || strings.Contains(run, "npm-profile-provision-node") ||
			strings.Contains(run, "GITHUB_PATH") || strings.Contains(run, "npm-profile-toolchain-assert") ||
			strings.Contains(run, "npm-profile-provision-npm") || strings.Contains(run, "npm-profile-build") ||
			strings.Contains(run, "npm-profile-sign") || strings.Contains(run, "npm-profile-publish") ||
			strings.Contains(run, "npm-executable") || strings.Contains(run, "NPM_EXECUTABLE")) &&
			normalizeExpression(strings.Join(strings.Fields(normalizeExpression(step.If)), " ")) != publishToolchainGate {
			return fmt.Errorf("publish toolchain step conditions must use the upstream-success gate")
		}
	}
	firstAssertIndex := len(job.Steps)
	npmProvisionIndex := len(job.Steps)
	npmProvisionID := ""
	for index, current := range job.Steps {
		run := toolchainShellText(current.Run)
		scan := toolchainInvocationText(run)
		if npmReferencePattern.MatchString(scan) {
			return fmt.Errorf("publish job must use the provisioning output instead of ambient npm")
		}
		if strings.Contains(scan, "npm-profile-toolchain-assert") {
			if index <= spine.bindIndex || current.WorkingDirectory != ".slsa-builder" ||
				exactToolchainCommand(run) != toolchainAssertCommand+` --node-only --report-path "$REPORT_PATH"` ||
				current.Env["REPORT_PATH"] != outcomeReportPath {
				return fmt.Errorf("publish job must invoke npm-profile-toolchain-assert --node-only --report-path after Node.js setup")
			}
			firstAssertIndex = min(firstAssertIndex, index)
		}
		if strings.Contains(scan, "npm-profile-provision-npm") {
			if npmProvisionID != "" {
				return fmt.Errorf("publish job must contain exactly one npm-profile-provision-npm step")
			}
			if index < firstAssertIndex || current.ID == "" || current.WorkingDirectory != ".slsa-builder" ||
				!strings.HasPrefix(strings.TrimSpace(run), provisionNPMCommand+" ") ||
				!containsAll(run, `--output-directory "${{ runner.temp }}/windlass-npm-cli"`, `--github-output "$GITHUB_OUTPUT"`, `--report-path "$REPORT_PATH"`) ||
				current.Env["REPORT_PATH"] != outcomeReportPath {
				return fmt.Errorf("publish npm provisioning must have an id, emit GitHub outputs, and persist a failure report after the node-only assertion")
			}
			npmProvisionID = current.ID
			npmProvisionIndex = index
		}
		if strings.Contains(scan, "npm-executable") || strings.Contains(scan, "npm-profile-publish") || strings.Contains(scan, "NPM_EXECUTABLE") {
			if index < npmProvisionIndex {
				return fmt.Errorf("npm provisioning must precede publish diagnostics and npm-profile-publish")
			}
			output := "${{ steps." + npmProvisionID + ".outputs.npm-executable }}"
			argumentRun := run
			if strings.Contains(scan, "NPM_EXECUTABLE") {
				binding, present := current.Env["NPM_EXECUTABLE"]
				if !present {
					binding = job.Env["NPM_EXECUTABLE"]
				}
				if binding != output {
					return fmt.Errorf("NPM_EXECUTABLE must bind to %s", output)
				}
				run = strings.NewReplacer("${{ env.NPM_EXECUTABLE }}", output,
					"$NPM_EXECUTABLE", output, "${NPM_EXECUTABLE}", output).Replace(run)
			}
			if !strings.Contains(run, output) || strings.Contains(strings.ReplaceAll(run, output, ""), ".outputs.npm-executable") {
				return fmt.Errorf("publish npm references must use %s", output)
			}
			if strings.Contains(scan, "npm-profile-publish") {
				arguments := npmExecutableArgumentPattern.FindAllStringSubmatch(argumentRun, -1)
				if len(arguments) != 1 || strings.Count(argumentRun, "--npm-executable") != 1 ||
					!slices.Contains([]string{`"` + output + `"`, "'" + output + "'",
						`"${{ env.NPM_EXECUTABLE }}"`, `'${{ env.NPM_EXECUTABLE }}'`,
						`"$NPM_EXECUTABLE"`, `"${NPM_EXECUTABLE}"`}, arguments[0][1]) {
					return fmt.Errorf("npm-profile-publish --npm-executable must use exactly one pinned output argument: %s", output)
				}
			}
		}
	}
	if firstAssertIndex == len(job.Steps) || npmProvisionID == "" {
		return fmt.Errorf("publish job must invoke npm-profile-toolchain-assert --node-only and npm-profile-provision-npm")
	}
	return nil
}

// Conditions are compared as whitespace-normalized strings, not evaluated.
// Missing gates must also agree so npm cannot run while its assertion is skipped.
func checkToolchainConditions(steps []workflowStep) error {
	gate := ""
	seen := false
	for _, step := range steps {
		step.Run = toolchainInvocationText(toolchainShellText(step.Run))
		if !runtimeReferencePattern.MatchString(step.Run) &&
			!strings.Contains(step.Run, "npm-profile-provision-node") &&
			!strings.Contains(step.Run, "GITHUB_PATH") &&
			!strings.Contains(step.Run, "npm-profile-toolchain-assert") &&
			!strings.Contains(step.Run, "npm-profile-provision-npm") &&
			!strings.Contains(step.Run, "npm-profile-build") &&
			!strings.Contains(step.Run, "npm-profile-sign") &&
			!strings.Contains(step.Run, "npm-profile-publish") &&
			!strings.Contains(step.Run, "npm-executable") &&
			!strings.Contains(step.Run, "NPM_EXECUTABLE") {
			continue
		}
		condition := strings.Join(strings.Fields(normalizeExpression(step.If)), " ")
		if normalizeExpression(condition) == "false" || (seen && condition != gate) {
			return fmt.Errorf("toolchain step conditions must be identical and not constant false")
		}
		gate = condition
		seen = true
	}
	return nil
}

// Strip shell comments before joining continuations so comments cannot supply
// evidence, while preserving quoted, escaped, or parameter-expansion-internal
// hashes in executable shell text.
func toolchainShellText(run string) string {
	lines := strings.Split(run, "\n")
	var quote rune
	expansion := 0
	for index, line := range lines {
		escaped := false
		wordStart := true
		for offset, char := range line {
			if escaped {
				escaped = false
				wordStart = false
				continue
			}
			if char == '\\' && quote != '\'' {
				escaped = true
				wordStart = false
				continue
			}
			if quote != 0 {
				wordStart = false
				if char == quote {
					quote = 0
				}
				continue
			}
			if expansion > 0 {
				wordStart = false
				if char == '$' && offset+1 < len(line) && line[offset+1] == '{' {
					expansion++
				} else if char == '}' {
					expansion--
				}
				continue
			}
			if char == '$' && offset+1 < len(line) && line[offset+1] == '{' {
				expansion++
				wordStart = false
			} else if char == '\'' || char == '"' {
				quote = char
				wordStart = false
			} else if char == '#' && wordStart {
				lines[index] = line[:offset]
				break
			} else {
				wordStart = strings.ContainsRune(" \t\r\v\f;|&()<>", char)
			}
		}
	}
	return strings.ReplaceAll(strings.Join(lines, "\n"), "\\\n", " ")
}
