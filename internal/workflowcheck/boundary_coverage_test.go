package workflowcheck

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestJobBoundaryRejections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, workflow, old, replacement, want string
		check                                  func(string) error
	}{
		{"build top permissions", validBuildWorkflow, "permissions: {}", "permissions: {contents: read}", "top-level permissions must be an explicit empty mapping", checkBuildError},
		{"build missing", validBuildWorkflow, "  build:", "  other:", "workflow has no build job", checkBuildError},
		{"build extra read permission", validBuildWorkflow, "contents: read", "contents: read\n      issues: read", "build job permissions must be exactly contents: read", checkBuildError},
		{"build runner", validBuildWorkflow, "ubuntu-24.04", "ubuntu-latest", "build job runs-on must be ubuntu-24.04", checkBuildError},
		{"build hardening first", validBuildWorkflow, "step-security/harden-runner@", "other/action@", "harden-runner must be the first build step", checkBuildError},
		{"build egress", validBuildWorkflow, "egress-policy: audit", "egress-policy: block", "harden-runner must use egress-policy audit", checkBuildError},
		{"build corepack", validBuildWorkflow, "corepack enable", "true", "build job must enable Corepack", checkBuildError},
		{"build invocation", validBuildWorkflow, "npm-profile-build", "other-command", "build job must invoke npm-profile-build", checkBuildError},
		{"build resolver identity", validBuildWorkflow, "id: source-ref", "id: untrusted", "source-ref resolver must expose trusted built-source outputs from invocation observations", checkBuildError},
		{"build trusted checkout", validBuildWorkflow, "repository: ${{ job.workflow_repository }}", "repository: ${{ github.repository }}", "build job must checkout the trusted builder at job.workflow_sha without credentials", checkBuildError},
		{"sign top permissions", validSigningWorkflow, "permissions: {}", "permissions: {contents: read}", "top-level permissions must be an explicit empty mapping", checkSignError},
		{"sign missing", validSigningWorkflow, "  provenance-sign:", "  other:", "workflow has no provenance-sign job", checkSignError},
		{"sign runner", validSigningWorkflow, "needs: build\n    runs-on: ubuntu-24.04", "needs: build\n    runs-on: ubuntu-latest", "provenance-sign runs-on must be ubuntu-24.04", checkSignError},
		{"sign dependency", validSigningWorkflow, "needs: build", "needs: other", "provenance-sign must depend only on build", checkSignError},
		{"sign hardening", validSigningWorkflow, "egress-policy: audit", "egress-policy: block", "harden-runner with audit egress must be the first provenance-sign step", checkSignError},
		{"sign floating action", validSigningWorkflow, "actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0", "actions/checkout@main", "action reference \"actions/checkout@main\" is not pinned to a full SHA", checkSignError},
		{"sign forbidden attest", validSigningWorkflow, "actions/checkout@", "actions/attest@", "actions/attest is forbidden in provenance-sign", checkSignError},
		{"sign bundle path", validSigningWorkflow, "/*.intoto.jsonl", "/*", "provenance bundle upload must select only the deterministic bundle file", checkSignError},
		{"publish top permissions", validNPMOnlyWorkflow, "permissions: {}", "permissions: {contents: read}", "top-level permissions must be an explicit empty mapping", checkPublishError},
		{"publish missing", validNPMOnlyWorkflow, "  publish:", "  other:", "workflow has no publish job", checkPublishError},
		{"publish runner", validNPMOnlyWorkflow, "needs: [build, provenance-sign]\n    runs-on: ubuntu-24.04", "needs: [build, provenance-sign]\n    runs-on: ubuntu-latest", "publish job runs-on must be ubuntu-24.04", checkPublishError},
		{"publish conditional", validNPMOnlyWorkflow, "    if: always()", "    if: success()", "publish job must run always so it can emit the outcome report", checkPublishError},
		{"publish invocation", validNPMOnlyWorkflow, "npm-profile-publish", "other-command", "publish job must invoke npm-profile-publish", checkPublishError},
		{"publish hardening", validNPMOnlyWorkflow, "    steps:\n      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411\n        with: {egress-policy: audit}\n      - uses: actions/download-artifact@", "    steps:\n      - uses: step-security/harden-runner@9af89fc71515a100421586dfdb3dc9c984fbf411\n        with: {egress-policy: block}\n      - uses: actions/download-artifact@", "harden-runner with audit egress must be the first publish step", checkPublishError},
		{"publish floating action", validNPMOnlyWorkflow, "      - id: publish", "      - uses: actions/attest@main\n      - id: publish", "action reference \"actions/attest@main\" is not pinned to a full SHA", checkPublishError},
		{"publish forbidden attest", validNPMOnlyWorkflow, "      - id: publish", "      - uses: actions/attest@9af89fc71515a100421586dfdb3dc9c984fbf411\n      - id: publish", "actions/attest is forbidden in publish", checkPublishError},
		{"publish extra download", validNPMOnlyWorkflow, "      - id: publish", "      - uses: actions/download-artifact@9af89fc71515a100421586dfdb3dc9c984fbf411\n        with: {name: extra}\n      - id: publish", `publish handoff downloads are ["extra" "js-ts-npm-package-tarball-${{ github.run_id }}-${{ github.run_attempt }}" "js-ts-npm-provenance-bundle-${{ github.run_id }}-${{ github.run_attempt }}"]`, checkPublishError},
		{"public input type", validNPMOnlyWorkflow, "access: {required: false, type: string}", "access: {required: false, type: boolean}", "workflow_call input \"access\" has the wrong type or required state", checkProfileError},
		{"public boolean default", validNPMOnlyWorkflow, "release-asset-mode: {required: false, type: boolean, default: false}", "release-asset-mode: {required: false, type: boolean, default: true}", "boolean input \"release-asset-mode\" must default to false", checkProfileError},
		{"public output binding", validNPMOnlyWorkflow, "jobs.publish.outputs.package-name", "jobs.build.outputs.package-name", "workflow_call output \"package-name\" must map to the publish job", checkProfileError},
		{"profile sign propagation", validNPMOnlyWorkflow, "needs: build", "needs: other", "provenance-sign contract: provenance-sign must depend only on build", checkProfileError},
		{"profile publish propagation", validNPMOnlyWorkflow, "npm-profile-publish", "other-command", "publish contract: publish job must invoke npm-profile-publish", checkProfileError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := writeWorkflow(t, replaceOnce(t, test.workflow, test.old, test.replacement))
			err := test.check(path)
			if err == nil || err.Error() != test.want {
				t.Fatalf("check workflow error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestChecksPreserveMissingFileError(t *testing.T) {
	t.Parallel()
	for name, check := range map[string]func(string) error{
		"build": checkBuildError, "sign": checkSignError, "publish": checkPublishError, "profile": checkProfileError,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := check(filepath.Join(t.TempDir(), "missing.yml"))
			if !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("missing workflow error = %v, want fs.ErrNotExist", err)
			}
		})
	}
}

func checkBuildError(path string) error   { _, err := CheckBuildJob(path); return err }
func checkSignError(path string) error    { _, err := CheckProvenanceSignJob(path); return err }
func checkPublishError(path string) error { _, err := CheckPublishJob(path); return err }
func checkProfileError(path string) error { _, err := CheckNPMOnlyProfile(path); return err }
