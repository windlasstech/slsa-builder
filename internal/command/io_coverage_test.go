package command

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAppendRegularFilesystem(t *testing.T) {
	t.Parallel()
	// Concurrent replacement branches require timing races and are intentionally excluded.
	for _, kind := range []string{"new", "existing", "directory", "symlink", "missing parent"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "output")
			prefix := ""
			switch kind {
			case "existing":
				prefix = "original\n"
				if err := os.WriteFile(path, []byte(prefix), 0o600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(filepath.Join(t.TempDir(), "target"), path); err != nil {
					t.Fatal(err)
				}
			case "missing parent":
				path = filepath.Join(path, "child")
			}
			file, err := openAppendRegular(path)
			if kind == "directory" || kind == "symlink" || kind == "missing parent" {
				if err == nil {
					file.Close()
					t.Fatal("openAppendRegular accepted invalid path")
				}
				if kind == "missing parent" && !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("error = %v, want not-exist", err)
				}
				if kind != "missing parent" && err.Error() != "file must be a regular non-symlink file" {
					t.Fatalf("rejection = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := io.WriteString(file, "appended\n"); err != nil {
				t.Fatal(err)
			}
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("permissions = %o, want 600", info.Mode().Perm())
			}
			if got := string(mustRead(t, path)); got != prefix+"appended\n" {
				t.Fatalf("content = %q, want %q", got, prefix+"appended\n")
			}
		})
	}
}

func TestRequireJSONEOF(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, suffix string
		wantError    bool
	}{
		{"clean EOF", "", false}, {"whitespace", " \n\t", false},
		{"second value", "{}", true}, {"malformed trailing bytes", "!", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			decoder := json.NewDecoder(strings.NewReader(`{}` + test.suffix))
			var first struct{}
			if err := decoder.Decode(&first); err != nil {
				t.Fatal(err)
			}
			err := requireJSONEOF(decoder)
			if (err != nil) != test.wantError {
				t.Fatalf("requireJSONEOF() = %v, want error %t", err, test.wantError)
			}
			if test.name == "second value" && err.Error() != "multiple JSON values are not allowed" {
				t.Fatalf("second-value error = %v", err)
			}
			if test.name == "malformed trailing bytes" {
				var syntax *json.SyntaxError
				if !errors.As(err, &syntax) {
					t.Fatalf("trailing error = %v, want SyntaxError", err)
				}
			}
		})
	}
}
