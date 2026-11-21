package dependabot

import (
	"bytes"
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type boundedGitOutput struct{ buffer bytes.Buffer }

const maxTreeFiles = 10000
const maxTreeBytes = 128 << 20

type committedEntry struct {
	path, object string
	size         int
}

func parseCommittedTree(listed []byte) ([]committedEntry, error) {
	if len(listed) == 0 || listed[len(listed)-1] != 0 {
		return nil, ErrInput
	}
	var entries []committedEntry
	seen := map[string]bool{}
	total := 0
	for raw := range strings.SplitSeq(string(listed[:len(listed)-1]), "\x00") {
		metadata, path, ok := strings.Cut(raw, "\t")
		fields := strings.Fields(metadata)
		key := strings.ToLower(path)
		if !ok || len(fields) != 4 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || !shaPattern.MatchString(fields[2]) || path == "." || !fs.ValidPath(path) || !filepath.IsLocal(path) || strings.ContainsAny(path, "\\\x00\r\n\t:") || seen[key] {
			return nil, ErrInput
		}
		size, err := strconv.Atoi(fields[3])
		if err != nil || size < 0 || size > maxInput || len(entries) == maxTreeFiles || size > maxTreeBytes-total {
			return nil, ErrInput
		}
		seen[key] = true
		total += size
		entries = append(entries, committedEntry{path: path, object: fields[2], size: size})
	}
	directories := map[string]string{}
	for _, entry := range entries {
		for parent := filepath.ToSlash(filepath.Dir(entry.path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			key := strings.ToLower(parent)
			previous, exists := directories[key]
			if seen[key] || (exists && previous != parent) {
				return nil, ErrInput
			}
			directories[key] = parent
		}
	}
	return entries, nil
}

func (output *boundedGitOutput) Write(data []byte) (int, error) {
	if len(data) > maxInput-output.buffer.Len() {
		return 0, ErrInput
	}
	return output.buffer.Write(data)
}

func trustedGit(ctx context.Context, root string, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root, "-c", "core.fsmonitor=false"}, args...)...)
	command.Env = trustedEnvironment()
	command.WaitDelay = time.Second
	var output boundedGitOutput
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return nil, ErrInput
	}
	return output.buffer.Bytes(), nil
}

func readCommittedFiles(ctx context.Context, inputs Inputs, paths []string) (map[string][]byte, error) {
	if !shaPattern.MatchString(inputs.TrustedSHA) {
		return nil, ErrInput
	}
	args := []string{"ls-tree", "-r", "-z", "-l", inputs.TrustedSHA, "--"}
	listed, err := trustedGit(ctx, inputs.TrustedRoot, append(args, paths...))
	if err != nil {
		return nil, err
	}
	entries, err := parseCommittedTree(listed)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		data, err := trustedGit(ctx, inputs.TrustedRoot, []string{"cat-file", "blob", entry.object})
		if err != nil {
			return nil, err
		}
		if len(data) != entry.size {
			return nil, ErrInput
		}
		files[entry.path] = data
	}
	for _, path := range paths {
		if _, ok := files[path]; !ok {
			return nil, ErrInput
		}
	}
	return files, nil
}
