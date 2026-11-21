package dependabot

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestModuleInventoryRejectsAmbiguity(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"duplicate", "case collision", "unicode collision", "file directory", "missing directory", "dot path", "parent path", "absolute", "backslash", "missing path", "invalid oid", "missing size", "negative size", "type mismatch", "symlink", "gitlink", "directory type", "entry bound"} {
		t.Run(scenario, func(t *testing.T) {
			responses := map[string]string{}
			remoteModuleFixture(t, responses)
			tree, err := decodeGitHub[moduleTree]([]byte(responses[signerRepositoryAPI+"/git/trees/"+strings.Repeat("8", 40)]))
			if err != nil {
				t.Fatal(err)
			}
			size := int64(0)
			entry := moduleTreeEntry{Path: "extra", Mode: "100644", Type: "blob", SHA: strings.Repeat("a", 40), Size: &size}
			switch scenario {
			case "duplicate":
				entry.Path = "go.mod"
			case "case collision":
				entry.Path = "GO.mod"
			case "unicode collision":
				entry.Path = "S"
				tree.Tree = append(tree.Tree, entry)
				entry.Path = "ſ"
			case "file directory":
				entry.Path = "go.mod/child"
			case "missing directory":
				entry.Path = "missing/child"
			case "dot path":
				entry.Path = "nested/./child"
			case "parent path":
				entry.Path = "../child"
			case "absolute":
				entry.Path = "/child"
			case "backslash":
				entry.Path = `nested\child`
			case "missing path":
				entry.Path = ""
			case "invalid oid":
				entry.SHA = "bad"
			case "missing size":
				entry.Size = nil
			case "negative size":
				size = -1
			case "type mismatch":
				entry.Type = "tree"
			case "symlink":
				entry.Mode = "120000"
			case "gitlink":
				entry.Mode = "160000"
				entry.Type = "commit"
			case "directory type":
				entry.Mode = "040000"
			case "entry bound":
				for i := len(tree.Tree); i < maxModuleEntries; i++ {
					clone := entry
					clone.Path = fmt.Sprintf("extra-%d", i)
					tree.Tree = append(tree.Tree, clone)
				}
			}
			tree.Tree = append(tree.Tree, entry)
			if _, err := moduleInventory(tree); !errors.Is(err, ErrInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestModuleContentBudgets(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"individual", "selected count", "aggregate", "root bound"} {
		t.Run(scenario, func(t *testing.T) {
			responses := map[string]string{}
			sum := remoteModuleFixture(t, responses)
			key := signerRepositoryAPI + "/git/trees/" + strings.Repeat("8", 40)
			tree, err := decodeGitHub[moduleTree]([]byte(responses[key]))
			if err != nil {
				t.Fatal(err)
			}
			size := int64(maxModuleBlob + 1)
			entry := moduleTreeEntry{Path: "large", Mode: "100644", Type: "blob", SHA: strings.Repeat("a", 40), Size: &size}
			switch scenario {
			case "individual":
				tree.Tree = append(tree.Tree, entry)
			case "root bound":
				for i := range tree.Tree {
					if tree.Tree[i].Path == "go.mod" {
						tree.Tree[i].Size = &size
					}
				}
			case "selected count", "aggregate":
				count := maxModuleFiles
				size = 0
				if scenario == "aggregate" {
					count = 17
					size = maxModuleBlob
				}
				for i := 0; i < count; i++ {
					clone := entry
					clone.Path = fmt.Sprintf("file-%d", i)
					tree.Tree = append(tree.Tree, clone)
				}
			}
			responses[key] = fixtureJSON(t, tree)
			api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
				body, ok := responses[r.URL.Path]
				if !ok {
					t.Errorf("unbounded blob fetch %s", r.URL)
					w.WriteHeader(500)
					return
				}
				fmt.Fprint(w, body)
			})
			_, err = api.verifyCandidateModule(t.Context(), resolvedRelease{metadata: releaseMetadata{Tag: "v1.999.0"}, commit: strings.Repeat("c", 40)}, sum)
			if !errors.Is(err, ErrInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
