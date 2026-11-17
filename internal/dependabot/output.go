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
	parent, err := filepath.EvalSymlinks(filepath.Dir(inputs.Output))
	if err != nil {
		return err
	}
	output, err := filepath.Abs(filepath.Join(parent, filepath.Base(inputs.Output)))
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
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func publishOutputs(output string, files map[string][]byte) error {
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
				return nil
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
	fmt.Fprintf(&result, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n", change.Path, change.Path, change.Path, change.Path, len(old), len(newLines))
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
