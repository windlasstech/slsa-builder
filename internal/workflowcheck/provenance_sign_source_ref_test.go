package workflowcheck

import "testing"

func TestCheckProvenanceSignJobBuiltSourceHandoff(t *testing.T) {
	valid := provenanceSignBuiltSourceWorkflow(t)
	if _, err := CheckProvenanceSignJob(writeWorkflow(t, valid)); err != nil {
		t.Fatalf("CheckProvenanceSignJob() rejected built-source handoff: %v", err)
	}

	tests := map[string]string{
		"missing built source arguments": replaceOnce(t, valid,
			` --built-ref "$BUILT_REF" --built-revision "$BUILT_REVISION"`, ""),
		"sign command uses invocation ref": replaceOnce(t, valid,
			`--built-ref "$BUILT_REF"`, `--built-ref "$GITHUB_REF"`),
		"built ref reads invocation ref": replaceOnce(t, valid,
			"BUILT_REF: ${{ needs.build.outputs.built-ref }}", "BUILT_REF: ${{ github.ref }}"),
		"built revision reads invocation revision": replaceOnce(t, valid,
			"BUILT_REVISION: ${{ needs.build.outputs.built-revision }}", "BUILT_REVISION: ${{ github.sha }}"),
		"build output bypasses resolver": replaceOnce(t, valid,
			"built-ref: ${{ steps.source-ref.outputs.built-ref }}", "built-ref: ${{ github.ref }}"),
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := CheckProvenanceSignJob(writeWorkflow(t, contents)); err == nil {
				t.Fatal("CheckProvenanceSignJob() succeeded, want rejection")
			}
		})
	}
}

func provenanceSignBuiltSourceWorkflow(t *testing.T) string {
	t.Helper()
	return validSigningWorkflow
}
