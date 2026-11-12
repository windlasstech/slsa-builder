package workflowcheck

import (
	"strings"
	"testing"
)

func TestPublishRequiresAuthenticatedManifestHandoff(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"commented flags", "duplicate digest", "wrong digest binding"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			document, err := decodeWorkflow([]byte(validNPMOnlyWorkflow))
			if err != nil {
				t.Fatal(err)
			}
			for index, step := range document.Jobs["publish"].Steps {
				if step.ID != "publish" {
					continue
				}
				changed := &document.Jobs["publish"].Steps[index]
				switch mutation {
				case "commented flags":
					changed.Run = strings.Replace(changed.Run, " --metadata-artifact-dir", " # --metadata-artifact-dir", 1)
				case "duplicate digest":
					changed.Run += ` --metadata-sha256 "$OTHER_DIGEST"`
				case "wrong digest binding":
					changed.Env["METADATA_SHA256"] = "${{ needs.build.outputs.package-tarball-sha256 }}"
				}
			}
			if _, err := CheckPublishJob(writeWorkflowDocument(t, document)); err == nil {
				t.Fatal("unauthenticated metadata handoff accepted")
			}
		})
	}
}
