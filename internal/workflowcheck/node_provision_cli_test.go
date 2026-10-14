package workflowcheck

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNodeProvisioningCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "slsa-builder-internal")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./cmd/slsa-builder-internal")
	build.Dir = "../.."
	build.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build internal executable: %v\n%s", err, output)
	}
	for _, legacySetup := range []bool{false, true} {
		name := "provisioned runtime"
		if legacySetup {
			name = "reintroduced setup-node"
		}
		t.Run(name, func(t *testing.T) {
			// Given a temporary copy of the production workflow, optionally with setup-node restored.
			contents, err := os.ReadFile("../../.github/workflows/js-ts-npm-package-slsa3.yml")
			if err != nil {
				t.Fatal(err)
			}
			if legacySetup {
				marker := []byte("      - name: Provision pinned Node.js runtime")
				if !bytes.Contains(contents, marker) {
					t.Fatal("production workflow is missing runtime provisioning")
				}
				contents = bytes.Replace(contents, marker,
					append([]byte("      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020\n"), marker...), 1)
			}
			path := writeWorkflow(t, string(contents))
			// When the real CLI checks the copied workflow.
			command := exec.CommandContext(t.Context(), binary, "workflow-check", "--workflow", path, "--profile", "npm-only")
			output, err := command.CombinedOutput()
			t.Logf("workflow-check output (error=%v):\n%s", err, output)
			// Then only the provisioned workflow exits successfully; t.TempDir removes both copies.
			if legacySetup {
				if err == nil || !bytes.Contains(output, []byte("setup action or runner toolcache")) {
					t.Fatalf("setup-node workflow was not rejected: %v\n%s", err, output)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var report NPMOnlyProfileResult
			if err := json.Unmarshal(output, &report); err != nil || report.Result != "pass" {
				t.Fatalf("expected pass report: %+v, error=%v", report, err)
			}
		})
	}
}
