package dependabot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOutputTrailingSlashAndExactRepublication(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"fresh", "empty", "empty nested directories", "exact", "incomplete", "extra", "mismatch", "symlink", "extra directory"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			// Given a separate output with an optional previous publication.
			base := t.TempDir()
			output := filepath.Join(base, "output")
			files := map[string][]byte{"plan.json": []byte("plan"), "patch.diff": []byte("patch")}
			if kind == "empty nested directories" {
				files["files/nested/deeper/result"] = []byte("exact\x00bytes\n")
			}
			if kind == "symlink" {
				if err := os.Symlink(t.TempDir(), output); err != nil {
					t.Fatal(err)
				}
			} else if kind != "fresh" {
				if err := os.Mkdir(output, 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "empty nested directories" {
					if err := os.MkdirAll(filepath.Join(output, "files", "nested", "deeper"), 0700); err != nil {
						t.Fatal(err)
					}
				} else if kind != "empty" {
					for path, data := range files {
						if kind == "incomplete" && path == "patch.diff" {
							continue
						}
						if kind == "mismatch" {
							data = []byte("wrong")
						}
						if err := os.WriteFile(filepath.Join(output, path), data, 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
				if kind == "extra" {
					if err := os.WriteFile(filepath.Join(output, "extra"), nil, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "extra directory" {
					if err := os.Mkdir(filepath.Join(output, "extra"), 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			// When publishing with a trailing separator, then only exact or empty roots succeed.
			err := publishOutputs(output+string(filepath.Separator), files)
			valid := kind == "fresh" || kind == "empty" || kind == "empty nested directories" || kind == "exact"
			if (err == nil) != valid {
				t.Fatalf("valid=%v error=%v", valid, err)
			}
			if kind == "empty nested directories" {
				for path, want := range files {
					got, readErr := os.ReadFile(filepath.Join(output, path))
					if readErr != nil || string(got) != string(want) {
						t.Fatalf("published %s = %q, %v; want %q", path, got, readErr, want)
					}
				}
			}
		})
	}
}

func TestOutputBoundaryRejectsOverlap(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"separate", "inside", "ancestor", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			// Given distinct inputs and an output path, possibly aliased.
			base := t.TempDir()
			root := filepath.Join(base, "trusted")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(base, "output")
			switch kind {
			case "inside":
				output = filepath.Join(root, "output")
			case "ancestor":
				output = base
			case "symlink":
				if err := os.Symlink(root, output); err != nil {
					t.Fatal(err)
				}
			}
			// When resolving boundaries, then aliases and either overlap direction fail.
			err := outputBoundary(Inputs{Output: output + string(filepath.Separator), TrustedRoot: root, Snapshot: root, EventPath: root})
			if (err == nil) != (kind == "separate") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
