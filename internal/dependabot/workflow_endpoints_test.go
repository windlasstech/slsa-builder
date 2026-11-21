package dependabot

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func mutateCompanionEndpoints(section, endpoint, replacement string) string {
	block := regexp.MustCompile(`(?m)^(          allowed-endpoints: [>|][+-]?)[ \t]*\r?\n((?:            [^\r\n]*\r?\n)+)`)
	return block.ReplaceAllStringFunc(section, func(value string) string {
		lines := strings.SplitN(value, "\n", 2)
		list := strings.TrimSuffix(lines[1], "\n")
		endpoints := strings.Fields(list)
		for i, item := range endpoints {
			if item == endpoint {
				endpoints[i] = replacement
			}
		}
		return strings.TrimSuffix(lines[0], "\r") + "\n            " + strings.TrimSpace(strings.Join(endpoints, " ")) + "\n"
	})
}

func TestCompanionWorkflowEndpointSets(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../.github/workflows/dependabot-companion.yml")
	if err != nil {
		t.Fatal(err)
	}
	plan, apply, ok := strings.Cut(string(raw), "\n  apply:")
	if !ok {
		t.Fatal("missing apply job boundary")
	}
	for _, job := range []string{"plan", "apply"} {
		endpoints := strings.Fields("api.github.com:443 github.com:443 mise.jdx.dev:443 mise-versions.jdx.dev:443 dl.google.com:443 release-assets.githubusercontent.com:443 proxy.golang.org:443 sum.golang.org:443 storage.googleapis.com:443")
		if job == "plan" {
			endpoints = append(endpoints, "results-receiver.actions.githubusercontent.com:443", "*.blob.core.windows.net:443")
		}
		for _, endpoint := range endpoints {
			for _, replacement := range []string{"", "attacker.example:443", endpoint + " " + endpoint} {
				t.Run(job+"/"+endpoint+"/"+replacement, func(t *testing.T) {
					t.Parallel()
					changedPlan, changedApply := plan, apply
					if job == "plan" {
						changedPlan = mutateCompanionEndpoints(plan, endpoint, replacement)
					} else {
						changedApply = mutateCompanionEndpoints(apply, endpoint, replacement)
					}
					mutated := changedPlan + "\n  apply:" + changedApply
					if mutated == string(raw) {
						t.Fatalf("%s endpoint %q absent", job, endpoint)
					}
					var document map[string]any
					if err := yaml.Unmarshal([]byte(mutated), &document); err != nil {
						t.Fatalf("endpoint mutation is invalid YAML: %v", err)
					}
					err := checkCompanionWorkflow([]byte(mutated))
					path := "workflow.jobs." + job + ".steps[0].with.allowed-endpoints"
					var violation *companionWorkflowViolation
					if !errors.As(err, &violation) || violation.path != path {
						t.Fatalf("check = %v; want exact endpoint-set rejection at %s", err, path)
					}
				})
			}
		}
		for _, extra := range []string{"attacker.example:443", "*.example:443", "results-receiver.actions.githubusercontent.com:443", "*.blob.core.windows.net:443"} {
			t.Run(job+"/extra/"+extra, func(t *testing.T) {
				t.Parallel()
				changedPlan, changedApply := plan, apply
				if job == "plan" {
					changedPlan = strings.Replace(plan, "api.github.com:443", "api.github.com:443 "+extra, 1)
				} else {
					changedApply = strings.Replace(apply, "api.github.com:443", "api.github.com:443 "+extra, 1)
				}
				err := checkCompanionWorkflow([]byte(changedPlan + "\n  apply:" + changedApply))
				path := "workflow.jobs." + job + ".steps[0].with.allowed-endpoints"
				var violation *companionWorkflowViolation
				if !errors.As(err, &violation) || violation.path != path {
					t.Fatalf("check = %v; want endpoint addition rejection at %s", err, path)
				}
			})
		}
	}
}

func TestCompanionWorkflowAcceptsPresentationChanges(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../.github/workflows/dependabot-companion.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, old, replacement string }{
		{"workflow label", "name: Dependabot companion", "name: Reviewed companion"},
		{"step label", "name: Harden runner", "name: Restrict egress"},
		{"comment", "# v7.0.1", "# reviewed checkout"},
		{"endpoint order", "api.github.com:443 github.com:443", "github.com:443   api.github.com:443"},
		{"predicate spacing", "'pull_request' &&", "'pull_request'   &&"},
		{"apply spacing", "ENABLED == 'true'", "ENABLED   ==   'true'"},
		{"quoted runner", "runs-on: ubuntu-24.04", "runs-on: 'ubuntu-24.04'"},
		{"stripped folded endpoint block", "allowed-endpoints: >", "allowed-endpoints: >-"},
		{"literal endpoint block", "allowed-endpoints: >", "allowed-endpoints: |"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mutated := strings.Replace(string(raw), test.old, test.replacement, 1)
			if mutated == string(raw) {
				t.Fatalf("presentation target %q absent", test.old)
			}
			if err := checkCompanionWorkflow([]byte(mutated)); err != nil {
				t.Fatalf("presentation-only change rejected: %v", err)
			}
		})
	}
}
