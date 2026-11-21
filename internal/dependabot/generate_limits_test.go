package dependabot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedTreeBounds(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		count int
		size  int64
		extra int64
		valid bool
	}{
		{"count boundary", 10000, 0, -1, true},
		{"count overflow", 10000, 0, 0, false},
		{"aggregate boundary", 8, 16 << 20, -1, true},
		{"aggregate overflow", 8, 16 << 20, 1, false},
		{"file overflow", 1, (16 << 20) + 1, -1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given sparse regular fixtures at the exact limits or one beyond them.
			root := t.TempDir()
			directory := filepath.Join(root, "testdata/npm/provenance")
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			count := test.count
			if test.extra >= 0 {
				count++
			}
			for i := range count {
				size := test.size
				if i == test.count {
					size = test.extra
				}
				file, err := os.Create(filepath.Join(directory, fmt.Sprintf("contract-%d.json", i)))
				if err != nil {
					t.Fatal(err)
				}
				err = file.Truncate(size)
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("truncate=%v close=%v", err, closeErr)
				}
			}
			// When inventorying all files, then the same limits govern generated output.
			changes, err := generatedChanges(t.Context(), root, nil)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
			if test.valid && len(changes) != test.count {
				t.Fatalf("count=%d", len(changes))
			}
		})
	}
}

func TestTrustedGenerationRejectsMissingModuleFiles(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"go.mod", "go.sum"} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()
			// Given a trusted commit missing a required module file.
			files := map[string][]byte{"go.mod": []byte("module example"), "go.sum": []byte("sum")}
			delete(files, missing)
			root, sha := NewCommittedTestRoot(t, files)
			// When starting generation, then reject before compiling anything.
			_, err := generateTrusted(t.Context(), Generation{inputs: Inputs{TrustedRoot: root, TrustedSHA: sha}})
			if err != ErrInput {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
