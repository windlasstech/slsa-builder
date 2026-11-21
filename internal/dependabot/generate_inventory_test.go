package dependabot

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedInventoryAccountsForEveryFile(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"new allowed", "new ignored", "symlink", "deleted", "modified forbidden", "module drift"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			// Given a complete scratch tree and its committed preimages.
			root := t.TempDir()
			original := map[string][]byte{"go.mod": []byte("module base"), "go.sum": []byte("sum"), ".gitignore": []byte("ignored\n")}
			for path, data := range original {
				if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			path := "testdata/npm/provenance/contract-new.json"
			switch kind {
			case "new ignored":
				path = "ignored"
			case "modified forbidden":
				path = ".gitignore"
			case "module drift":
				path = "go.mod"
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "symlink":
				if err := os.Symlink(filepath.Join(root, "go.mod"), filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if err := os.Remove(filepath.Join(root, "go.sum")); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.WriteFile(filepath.Join(root, path), []byte("{}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// When comparing the entire tree, then only allowlisted additions survive.
			changes, err := generatedChanges(t.Context(), root, original)
			if kind != "new allowed" {
				if err == nil {
					t.Fatal("unexpected file change accepted")
				}
				return
			}
			if err != nil || len(changes) != 1 || len(changes[0].Before) != 0 || !bytes.Equal(changes[0].After, []byte("{}\n")) {
				t.Fatalf("changes=%v error=%v", changes, err)
			}
			if !bytes.Contains(unifiedPatch(changes[0]), []byte("--- /dev/null\n")) {
				t.Fatal("new file patch lacks creation header")
			}
		})
	}
}
