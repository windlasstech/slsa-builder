package workflowcheck

import (
	"slices"
	"strings"
	"testing"
)

func TestNodeProvisioningAcceptsBoundRuntime(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"node-provision", "verified-runtime"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			// Given independently named provisioning outputs and a uniform publish gate.
			document, err := decodeWorkflow([]byte(strings.ReplaceAll(validNPMOnlyWorkflow, "node-provision", id)))
			if err != nil {
				t.Fatal(err)
			}
			for index, step := range document.Jobs["publish"].Steps {
				if step.If != "always()" {
					document.Jobs["publish"].Steps[index].If = "needs.build.result == 'success' && needs.provenance-sign.result == 'success'"
				}
			}
			// When the whole workflow is checked.
			result, err := CheckNPMOnlyProfile(writeWorkflowDocument(t, document))
			// Then provisioner and assertion subcommand names are not runtime invocations.
			if err != nil || result.Result != "pass" {
				t.Fatalf("bound runtime rejected: result=%+v error=%v", result, err)
			}
		})
	}
}

func TestNodeProvisioningRejectsContractDrift(t *testing.T) {
	t.Parallel()
	for _, jobName := range []string{"build", "provenance-sign", "publish"} {
		for _, mutation := range []string{
			"missing provision", "conditional provision", "missing bind", "conditional bind",
			"wrong output id", "wrong output name", "missing github output", "missing directory",
			"wrong working directory", "missing id", "setup action", "toolcache",
			"node before bind", "npm before bind", "npx before bind", "corepack before bind",
			"assertion before provision", "npm provisioning before node", "bind before provision",
			"commented provision", "conditional shell provision", "extra provision command",
			"job PATH override", "step PATH override", "shell PATH override", "absolute runtime",
			"missing node report", "wrong node report", "duplicate provision", "duplicate bind",
		} {
			t.Run(jobName+"/"+mutation, func(t *testing.T) {
				t.Parallel()
				// Given one changed runtime boundary in an otherwise valid workflow.
				document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
				if err != nil {
					t.Fatal(err)
				}
				job := document.Jobs[jobName]
				index := slices.IndexFunc(job.Steps, func(step workflowStep) bool { return step.ID == "node-provision" })
				if index < 0 {
					t.Fatal("fixture has no provisioner")
				}
				switch mutation {
				case "missing provision":
					job.Steps = slices.Delete(job.Steps, index, index+1)
				case "conditional provision":
					job.Steps[index].If = "success()"
				case "missing bind":
					job.Steps = slices.Delete(job.Steps, index+1, index+2)
				case "conditional bind":
					job.Steps[index+1].If = "success()"
				case "wrong output id":
					job.Steps[index+1].Run = strings.ReplaceAll(job.Steps[index+1].Run, "steps.node-provision", "steps.other")
				case "wrong output name":
					job.Steps[index+1].Run = strings.ReplaceAll(job.Steps[index+1].Run, "node-bin", "node-path")
				case "missing github output":
					job.Steps[index].Run = strings.ReplaceAll(job.Steps[index].Run, ` --github-output "$GITHUB_OUTPUT"`, "")
				case "missing directory":
					job.Steps[index].Run = strings.ReplaceAll(job.Steps[index].Run, ` --output-directory "${{ runner.temp }}/windlass-node-runtime"`, "")
				case "wrong working directory":
					job.Steps[index].WorkingDirectory = "source"
				case "missing id":
					job.Steps[index].ID = ""
				case "setup action":
					job.Steps = append(job.Steps, workflowStep{Uses: "actions/setup-node@820762786026740c76f36085b0efc47a31fe5020"})
				case "toolcache":
					job.Steps = append(job.Steps, workflowStep{Run: `echo "$RUNNER_TOOL_CACHE/node/24.21.0/x64/bin" >> "$GITHUB_PATH"`})
				case "node before bind", "npm before bind", "npx before bind", "corepack before bind":
					job.Steps = slices.Insert(job.Steps, index+1, workflowStep{Run: strings.Fields(mutation)[0] + " --version"})
				case "assertion before provision":
					assertion := job.Steps[index+2]
					job.Steps = slices.Delete(job.Steps, index+2, index+3)
					job.Steps = slices.Insert(job.Steps, index, assertion)
				case "npm provisioning before node":
					job.Steps = slices.Insert(job.Steps, index, workflowStep{Run: "go run ./cmd/slsa-builder-internal npm-profile-provision-npm"})
				case "bind before provision":
					job.Steps[index], job.Steps[index+1] = job.Steps[index+1], job.Steps[index]
				case "commented provision":
					job.Steps[index].Run = "# " + job.Steps[index].Run
				case "conditional shell provision":
					job.Steps[index].Run = "false && " + job.Steps[index].Run
				case "extra provision command":
					job.Steps[index].Run += "; node --version"
				case "job PATH override":
					job.Env = map[string]string{"PATH": "/usr/bin"}
				case "step PATH override":
					job.Steps[index+2].Env = map[string]string{"PATH": "/usr/bin"}
				case "shell PATH override":
					job.Steps = append(job.Steps, workflowStep{Run: "PATH=/usr/bin node --version"})
				case "absolute runtime":
					job.Steps = append(job.Steps, workflowStep{Run: "/usr/bin/node --version"})
				case "missing node report", "wrong node report":
					if jobName != "publish" {
						t.Skip("only publish persists provisioning failure reports")
					}
					if mutation == "missing node report" {
						job.Steps[index].Run = strings.ReplaceAll(job.Steps[index].Run, ` --report-path "$REPORT_PATH"`, "")
					} else {
						job.Steps[index].Env["REPORT_PATH"] = "other.json"
					}
				case "duplicate provision":
					job.Steps = append(job.Steps, job.Steps[index])
				case "duplicate bind":
					job.Steps = append(job.Steps, job.Steps[index+1])
				}
				document.Jobs[jobName] = job
				// When checked through the public YAML entry point.
				_, err = CheckNPMOnlyProfile(writeWorkflowDocument(t, document))
				// Then no drift is accepted.
				if err == nil {
					t.Fatal("runtime provisioning drift accepted")
				}
			})
		}
	}
}

func TestNodeProvisioningProductionWorkflow(t *testing.T) {
	// Given the checked-in production artifact.
	path := "../../.github/workflows/js-ts-npm-package-slsa3.yml"
	// When checked through the public entry point.
	result, err := CheckNPMOnlyProfile(path)
	// Then the migrated production shape passes.
	if err != nil || result.Result != "pass" {
		t.Fatalf("production workflow: result=%+v error=%v", result, err)
	}
}
