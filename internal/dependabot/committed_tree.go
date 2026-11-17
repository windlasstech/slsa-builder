package dependabot

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type boundedGitOutput struct{ buffer bytes.Buffer }

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
	files := map[string][]byte{}
	for _, entry := range bytes.Split(bytes.TrimSuffix(listed, []byte{0}), []byte{0}) {
		metadata, path, ok := strings.Cut(string(entry), "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 4 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || !shaPattern.MatchString(fields[2]) || !filepath.IsLocal(path) {
			return nil, ErrInput
		}
		size, err := strconv.Atoi(fields[3])
		if err != nil || size < 0 || size > maxInput {
			return nil, ErrInput
		}
		data, err := trustedGit(ctx, inputs.TrustedRoot, []string{"cat-file", "blob", fields[2]})
		if err != nil {
			return nil, err
		}
		if len(data) != size {
			return nil, ErrInput
		}
		files[path] = data
	}
	for _, path := range paths {
		if _, ok := files[path]; !ok {
			return nil, ErrInput
		}
	}
	return files, nil
}
