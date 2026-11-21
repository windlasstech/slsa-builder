package dependabot

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func outputBoundary(inputs Inputs) error {
	if inputs.Output == "" || inputs.TrustedRoot == "" || inputs.Snapshot == "" || inputs.EventPath == "" {
		return ErrInput
	}
	clean := filepath.Clean(inputs.Output)
	if info, err := os.Lstat(clean); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrInput
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(clean))
	if err != nil {
		return err
	}
	output, err := filepath.Abs(filepath.Join(parent, filepath.Base(clean)))
	if err != nil {
		return err
	}
	for _, input := range []string{inputs.TrustedRoot, inputs.Snapshot, inputs.EventPath} {
		resolved, err := filepath.EvalSymlinks(input)
		if err != nil {
			return err
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return err
		}
		if within(resolved, output) || within(output, resolved) {
			return errors.New("output must be separate from trusted root and inputs")
		}
	}
	return nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}

func publishOutputs(output string, files map[string][]byte) error {
	output = filepath.Clean(output)
	if info, err := os.Lstat(output); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrInput
		}
		seen := 0
		err = filepath.WalkDir(output, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path == output {
					return nil
				}
				rel, err := filepath.Rel(output, path)
				if err != nil {
					return err
				}
				for name := range files {
					if strings.HasPrefix(name, filepath.ToSlash(rel)+"/") {
						return nil
					}
				}
				return errors.New("existing output has unexpected directory")
			}
			rel, err := filepath.Rel(output, path)
			if err != nil {
				return err
			}
			want, ok := files[filepath.ToSlash(rel)]
			if !ok {
				return errors.New("existing output differs")
			}
			got, err := readBounded(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(got, want) {
				return errors.New("existing output differs")
			}
			seen++
			return nil
		})
		if err != nil {
			return err
		}
		if seen == 0 {
			return publishNewOutputs(output, files)
		}
		if seen != len(files) {
			return errors.New("existing output incomplete")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return publishNewOutputs(output, files)
}

func publishNewOutputs(output string, files map[string][]byte) error {
	output = filepath.Clean(output)
	stage, err := os.MkdirTemp(filepath.Dir(output), ".companion-output-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for path, data := range files {
		target := filepath.Join(stage, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return err
		}
	}
	if err := os.Remove(output); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(stage, output)
}

func unifiedPatch(change Change) []byte {
	var result bytes.Buffer
	old := strings.SplitAfter(string(change.Before), "\n")
	newLines := strings.SplitAfter(string(change.After), "\n")
	if old[len(old)-1] == "" {
		old = old[:len(old)-1]
	}
	if newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}
	fmt.Fprintf(&result, "diff --git a/%s b/%s\n", change.Path, change.Path)
	oldPath, oldStart := "a/"+change.Path, 1
	if len(old) == 0 {
		oldStart = 0
	}
	if change.Before == nil {
		result.WriteString("new file mode 100644\n")
		oldPath, oldStart = "/dev/null", 0
	}
	newStart := 1
	if len(newLines) == 0 {
		newStart = 0
	}
	fmt.Fprintf(&result, "--- %s\n+++ b/%s\n@@ -%d,%d +%d,%d @@\n", oldPath, change.Path, oldStart, len(old), newStart, len(newLines))
	for _, group := range []struct {
		prefix byte
		lines  []string
	}{{'-', old}, {'+', newLines}} {
		for _, line := range group.lines {
			result.WriteByte(group.prefix)
			result.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				result.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return result.Bytes()
}
