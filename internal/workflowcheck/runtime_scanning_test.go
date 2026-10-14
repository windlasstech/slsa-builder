package workflowcheck

import (
	"slices"
	"strings"
	"testing"
)

func TestRuntimeScanningRejectsMutations(t *testing.T) {
	t.Parallel()
	for _, jobName := range []string{"build", "provenance-sign", "publish"} {
		for _, mutation := range []string{"job toolcache key", "step toolcache key", "PATH append", "quoted node", "escaped node", "mixed quoted node", "quoted subcommand"} {
			t.Run(jobName+"/"+mutation, func(t *testing.T) {
				t.Parallel()
				// Given a valid workflow with one runtime boundary mutation.
				document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
				if err != nil {
					t.Fatal(err)
				}
				job := document.Jobs[jobName]
				index := slices.IndexFunc(job.Steps, func(step workflowStep) bool { return step.ID == "node-provision" })
				step := workflowStep{If: job.Steps[index].If}
				switch mutation {
				case "job toolcache key":
					job.Env = map[string]string{"RUNNER_TOOL_CACHE": "/tmp"}
				case "step toolcache key":
					if job.Steps[index].Env == nil {
						job.Steps[index].Env = make(map[string]string)
					}
					job.Steps[index].Env["RUNNER_TOOL_CACHE"] = "/tmp"
				case "PATH append":
					step.Run = "PATH+=:/tmp"
				case "quoted node":
					step.Run = "n''ode --version"
				case "escaped node":
					step.Run = `n\ode --version`
				case "mixed quoted node":
					step.Run = `"n"ode --version`
				case "quoted subcommand":
					step.Run = "go run ./cmd/slsa-builder-internal npm-profile-bu''ild"
				}
				job.Steps = slices.Insert(job.Steps, index, step)
				document.Jobs[jobName] = job
				// When checked through the public YAML entry point.
				_, err = CheckNPMOnlyProfile(writeWorkflowDocument(t, document))
				// Then the runtime boundary violation is rejected.
				if err == nil {
					t.Fatal("runtime scanning mutation accepted")
				}
			})
		}
	}
}

func TestPublishRejectsDuplicateNPMProvisioning(t *testing.T) {
	t.Parallel()
	// Given a second provisioner before the original, which still supplies consumers.
	document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	job := document.Jobs["publish"]
	index := slices.IndexFunc(job.Steps, func(step workflowStep) bool {
		return strings.Contains(step.Run, "npm-profile-provision-npm")
	})
	duplicate := job.Steps[index]
	duplicate.ID = "extra-npm-provision"
	job.Steps = slices.Insert(job.Steps, index, duplicate)
	document.Jobs["publish"] = job
	// When checked through the publish entry point.
	_, err = CheckPublishJob(writeWorkflowDocument(t, document))
	// Then silently replacing the provisioning output is forbidden.
	if err == nil {
		t.Fatal("duplicate npm provisioning accepted")
	}
}

func TestWorkflowEnvironmentRejectsRuntimeOverrides(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"PATH", "RUNNER_TOOL_CACHE"} {
		for _, check := range []struct {
			name string
			run  func(string) error
		}{
			{name: "build", run: checkBuildError},
			{name: "sign", run: checkSignError},
			{name: "publish", run: checkPublishError},
		} {
			t.Run(check.name+"/"+key, func(t *testing.T) {
				t.Parallel()
				// Given a global override in otherwise valid workflow YAML.
				path := writeWorkflow(t, "env:\n  "+key+": /tmp\n"+validNPMOnlyWorkflow)
				// When checking an individual governed profile.
				err := check.run(path)
				// Then workflow inheritance cannot bypass the environment policy.
				if err == nil {
					t.Fatal("workflow runtime environment override accepted")
				}
			})
		}
	}
}

func TestToolchainAcceptsOperatorComments(t *testing.T) {
	t.Parallel()
	// Given a required exact command terminated by a semicolon and comment.
	document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range document.Jobs {
		for index, step := range job.Steps {
			if step.ID == "node-provision" || strings.Contains(step.Run, "npm-profile-toolchain-assert") || strings.Contains(step.Run, "GITHUB_PATH") {
				job.Steps[index].Run = strings.TrimSpace(step.Run) + ";# node --version"
			}
		}
	}
	// When checking the full workflow.
	_, err = CheckNPMOnlyProfile(writeWorkflowDocument(t, document))
	// Then comments supply no invocation evidence and do not alter the command.
	if err != nil {
		t.Fatalf("operator comment rejected: %v", err)
	}
}

func TestToolchainShellTextOperatorBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ run, want string }{
		{run: "echo a#b", want: "echo a#b"},
		{run: "echo ok;# comment", want: "echo ok;"},
		{run: "echo ok|# comment", want: "echo ok|"},
		{run: "echo ok&# comment", want: "echo ok&"},
		{run: "(# comment", want: "("},
		{run: "(echo ok)# comment", want: "(echo ok)"},
		{run: `echo \;#suffix`, want: `echo \;#suffix`},
		{run: "echo \"line\n\"#suffix", want: "echo \"line\n\"#suffix"},
	} {
		t.Run(test.run, func(t *testing.T) {
			t.Parallel()
			// Given shell word and operator boundaries.
			// When stripping comments.
			got := toolchainShellText(test.run)
			// Then only an unquoted, unescaped word-start hash is a comment.
			if got != test.want {
				t.Fatalf("shell text = %q, want %q", got, test.want)
			}
		})
	}
}
