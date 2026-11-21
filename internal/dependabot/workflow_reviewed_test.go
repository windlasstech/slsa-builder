package dependabot

type companionDisplayLabel struct{}

// This is the reviewed execution contract, not a snapshot loaded from the workflow
// under test. Every map is closed, including action inputs and invocation env.
func reviewedCompanionWorkflow() map[string]any {
	return map[string]any{
		"name": companionDisplayLabel{},
		"on": map[string]any{"workflow_run": map[string]any{
			"workflows": []any{"Lint"}, "types": []any{"completed"},
		}},
		"permissions": map[string]any{},
		"concurrency": map[string]any{
			"group":              "dependabot-companion-${{ github.repository }}-${{ github.event.workflow_run.head_branch || github.event.workflow_run.id }}",
			"cancel-in-progress": false,
		},
		"jobs": map[string]any{"plan": reviewedCompanionJob("plan"), "apply": reviewedCompanionJob("apply")},
	}
}

func reviewedCompanionJob(mode string) map[string]any {
	endpoints := "api.github.com:443 github.com:443 mise.jdx.dev:443 mise-versions.jdx.dev:443 " +
		"dl.google.com:443 release-assets.githubusercontent.com:443 proxy.golang.org:443 " +
		"sum.golang.org:443 storage.googleapis.com:443"
	job := map[string]any{
		"runs-on":         "ubuntu-24.04",
		"timeout-minutes": uint64(15),
		"if": "github.event.workflow_run.event == 'pull_request' && " +
			"github.event.workflow_run.head_repository.full_name == github.repository && " +
			"startsWith(github.event.workflow_run.head_branch, 'dependabot/')",
		"permissions": map[string]any{"contents": "read", "pull-requests": "read"},
	}
	env := map[string]any{
		"GITHUB_TOKEN": "${{ github.token }}",
		"TRUSTED_SHA":  "${{ steps.trusted.outputs.commit }}",
	}
	if mode == "plan" {
		endpoints += " results-receiver.actions.githubusercontent.com:443 *.blob.core.windows.net:443"
	} else {
		job["needs"] = "plan"
		job["if"] = "vars.DEPENDABOT_COMPANION_APPLY_ENABLED == 'true'"
		job["timeout-minutes"] = uint64(20)
		job["permissions"] = map[string]any{"contents": "write", "pull-requests": "read"}
		env["GITHUB_TOKEN"] = "${{ steps.app.outputs.token }}"
		env["APP_SLUG"] = "${{ steps.app.outputs.app-slug }}"
		env["DEPENDABOT_COMPANION_APPLY_ENABLED"] = "${{ vars.DEPENDABOT_COMPANION_APPLY_ENABLED }}"
	}
	steps := []any{
		map[string]any{
			"name": companionDisplayLabel{},
			"uses": "step-security/harden-runner@1d88384fd173af6ecb012dfd0264a306b711241a",
			"with": map[string]any{"egress-policy": "block", "allowed-endpoints": endpoints},
		},
		map[string]any{
			"name": companionDisplayLabel{}, "id": "trusted",
			"uses": "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
			"with": map[string]any{"ref": "main", "persist-credentials": false},
		},
		map[string]any{
			"name": companionDisplayLabel{},
			"uses": "step-security/mise-action@a225bbd7c202ddf57dd76dff22bb7992f6314173",
			"with": map[string]any{"install_args": "--locked go"},
		},
		map[string]any{"name": companionDisplayLabel{}, "run": "go mod download"},
	}
	if mode == "apply" {
		steps = append(steps, map[string]any{
			"name": companionDisplayLabel{}, "id": "app",
			"uses": "actions/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510",
			"with": map[string]any{
				"app-id":      "${{ vars.DEPENDABOT_COMPANION_APP_ID }}",
				"private-key": "${{ secrets.DEPENDABOT_COMPANION_APP_PRIVATE_KEY }}",
				"owner":       "windlasstech", "repositories": "slsa-builder",
				"permission-contents": "write", "permission-pull-requests": "read",
			},
		})
	}
	steps = append(steps, map[string]any{
		"name": companionDisplayLabel{}, "env": env,
		"run": `go run -mod=readonly ./cmd/slsa-builder-internal dependabot-companion --mode ` + mode +
			` --event "$GITHUB_EVENT_PATH" --trusted-root "$GITHUB_WORKSPACE" --trusted-sha "$TRUSTED_SHA" --output "$RUNNER_TEMP/companion-` + mode + `"`,
	})
	if mode == "plan" {
		steps = append(steps, map[string]any{
			"name": companionDisplayLabel{},
			"uses": "actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9",
			"with": map[string]any{
				"name":              "dependabot-companion-plan-${{ github.event.workflow_run.id }}-${{ github.run_attempt }}",
				"path":              "${{ runner.temp }}/companion-plan/plan.json\n${{ runner.temp }}/companion-plan/patch.diff\n",
				"if-no-files-found": "error",
			},
		})
	}
	job["steps"] = steps
	return job
}
