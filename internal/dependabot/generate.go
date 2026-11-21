package dependabot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func generateTrusted(ctx context.Context, generation Generation) ([]Change, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	head, err := trustedGit(ctx, generation.inputs.TrustedRoot, []string{"rev-parse", "HEAD"})
	if err != nil {
		return nil, errors.New("cannot identify trusted checkout")
	}
	if strings.TrimSpace(string(head)) != generation.inputs.TrustedSHA {
		return nil, errors.New("trusted checkout SHA mismatch")
	}
	original, err := readCommittedFiles(ctx, generation.inputs, nil)
	if err != nil {
		return nil, err
	}
	for _, path := range []string{"go.mod", "go.sum"} {
		if _, ok := original[path]; !ok {
			return nil, ErrInput
		}
	}
	scratch, err := os.MkdirTemp("", "dependabot-companion-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	for path, data := range original {
		target := filepath.Join(scratch, path)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return nil, err
		}
	}
	for _, path := range sourceSites {
		data, ok := original[path]
		if !ok {
			return nil, fmt.Errorf("missing trusted site %s", path)
		}
		edited, err := editSite(path, data, generation.prepared)
		if err != nil {
			return nil, fmt.Errorf("edit %s: %w", path, err)
		}
		if err := os.WriteFile(filepath.Join(scratch, path), edited, 0600); err != nil {
			return nil, err
		}
	}
	if err := regenerate(ctx, scratch, generation.prepared.transition); err != nil {
		return nil, err
	}
	expected := make(map[string][]byte, len(original))
	for path, data := range original {
		expected[path] = data
	}
	expected["go.mod"], expected["go.sum"] = generation.prepared.transition.head.Mod, generation.prepared.transition.head.Sum
	return generatedChanges(ctx, scratch, expected)
}

func generatedChanges(ctx context.Context, scratch string, original map[string][]byte) ([]Change, error) {
	var paths []string
	seen := map[string]bool{}
	var total int64
	err := filepath.WalkDir(scratch, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == scratch {
			return nil
		}
		rel, err := filepath.Rel(scratch, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		key := strings.ToLower(rel)
		if !fs.ValidPath(rel) || strings.ContainsAny(rel, "\\\x00\r\n\t:") || seen[key] {
			return ErrInput
		}
		seen[key] = true
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxInput || len(paths) == maxTreeFiles || info.Size() > maxTreeBytes-total {
			return ErrInput
		}
		total += info.Size()
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	var changes []Change
	remaining := make(map[string]bool, len(original))
	for path := range original {
		remaining[path] = true
	}
	for _, path := range paths {
		before, existed := original[path]
		delete(remaining, path)
		after, err := readBounded(filepath.Join(scratch, path))
		if err != nil {
			return nil, err
		}
		if existed && bytes.Equal(before, after) {
			continue
		}
		if !allowedOutput(path) {
			return nil, fmt.Errorf("unexpected generated change: %s", path)
		}
		changes = append(changes, Change{Path: path, Before: before, After: after})
	}
	if len(remaining) != 0 {
		return nil, errors.New("generated tree deleted committed files")
	}
	return changes, nil
}

func regenerate(ctx context.Context, scratch string, transition Transition) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	binaries, err := os.MkdirTemp("", "companion-binaries-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(binaries)
	if err := writeModules(scratch, transition.base); err != nil {
		return err
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return err
	}
	for _, pkg := range []string{"provenance", "npmprofile"} {
		cmd := exec.CommandContext(ctx, goTool, "test", "-c", "-buildvcs=false", "-mod=readonly", "-o", filepath.Join(binaries, pkg+".test"), "./internal/"+pkg)
		cmd.Dir = scratch
		cmd.Env = trustedEnvironment()
		cmd.WaitDelay = time.Second
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("trusted %s compilation failed: %w: %s", pkg, err, output)
		}
	}
	// Both binaries are linked to BASE dependencies before candidate files exist.
	// From here onward only those binaries execute; candidate modules stay data.
	if err := writeModules(scratch, transition.head); err != nil {
		return err
	}
	jobs := []struct {
		pkg, tests string
		gates      []string
	}{
		{"provenance", "^(TestValidSLSAv1|TestCommonRejectionMatrix|TestBuilderFields|TestSigningAdapterCandidateDrift)$", []string{"UPDATE_PROVENANCE_FIXTURES=1", "UPDATE_PROVENANCE_GOLDEN=1"}},
		{"npmprofile", "^(TestNPMProvenanceInput|TestNPMProvenanceInputSourceRefDispatchRetryGolden|TestFinalContractCorpus)$", []string{"UPDATE_P01_GOLDEN=1", "UPDATE_P02_GOLDEN=1", "UPDATE_CONTRACT_FIXTURES=1"}},
	}
	for _, job := range jobs {
		cmd := exec.CommandContext(ctx, filepath.Join(binaries, job.pkg+".test"), "-test.run", job.tests, "-test.parallel=1", "-test.timeout=2m")
		cmd.Dir = filepath.Join(scratch, "internal", job.pkg)
		cmd.Env = append(trustedEnvironment(), job.gates...)
		cmd.WaitDelay = time.Second
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("trusted %s generation failed: %w: %s", job.pkg, err, output)
		}
	}
	return nil
}

func writeModules(root string, modules Modules) error {
	if err := os.WriteFile(filepath.Join(root, "go.mod"), modules.Mod, 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "go.sum"), modules.Sum, 0600)
}

func trustedEnvironment() []string {
	env := []string{"GIT_MASTER=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GOENV=off", "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOVCS=*:off", "CGO_ENABLED=0", "GOFLAGS=-mod=readonly"}
	env = append(env, "GIT_OPTIONAL_LOCKS=0")
	for _, name := range []string{"HOME", "PATH", "TMPDIR", "GOCACHE", "GOMODCACHE", "GOPATH", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY"} {
		if value := os.Getenv(name); value != "" {
			env = append(env, name+"="+value)
		}
	}
	return env
}
