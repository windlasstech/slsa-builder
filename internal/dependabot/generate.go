package dependabot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func generateTrusted(ctx context.Context, generation Generation) ([]Change, error) {
	root := generation.inputs.TrustedRoot
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, git, "-C", root, "rev-parse", "HEAD")
	command.Env = trustedEnvironment()
	head, err := command.Output()
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
	var changes []Change
	for path, before := range original {
		if path == "go.mod" || path == "go.sum" {
			continue
		}
		after, err := readBounded(filepath.Join(scratch, path))
		if err != nil {
			return nil, err
		}
		if bytes.Equal(before, after) {
			continue
		}
		if !allowedOutput(path) {
			return nil, fmt.Errorf("unexpected generated change: %s", path)
		}
		changes = append(changes, Change{Path: path, Before: before, After: after})
	}
	return changes, nil
}

func regenerate(ctx context.Context, scratch string, transition Transition) error {
	if err := writeModules(scratch, transition.base); err != nil {
		return err
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return err
	}
	for _, pkg := range []string{"provenance", "npmprofile"} {
		cmd := exec.CommandContext(ctx, goTool, "test", "-c", "-buildvcs=false", "-mod=readonly", "-o", filepath.Join(scratch, pkg+".test"), "./internal/"+pkg)
		cmd.Dir = scratch
		cmd.Env = trustedEnvironment()
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
		cmd := exec.CommandContext(ctx, filepath.Join(scratch, job.pkg+".test"), "-test.run", job.tests, "-test.parallel=1", "-test.timeout=2m")
		cmd.Dir = filepath.Join(scratch, "internal", job.pkg)
		cmd.Env = append(trustedEnvironment(), job.gates...)
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
	for _, name := range []string{"HOME", "PATH", "TMPDIR", "GOCACHE", "GOMODCACHE", "GOPATH"} {
		if value := os.Getenv(name); value != "" {
			env = append(env, name+"="+value)
		}
	}
	return env
}
