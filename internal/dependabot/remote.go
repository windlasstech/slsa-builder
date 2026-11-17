package dependabot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type Remote struct {
	api     *githubAPI
	planner Planner
}

func NewRemote(token string) (*Remote, error) {
	api, err := newGitHub(token)
	if err != nil {
		return nil, err
	}
	return &Remote{api: api}, nil
}

func (remote *Remote) Close() { remote.api.client.CloseIdleConnections() }

func bindInputs(ctx context.Context, inputs Inputs) (workflowEvent, error) {
	if inputs.Snapshot != "" || inputs.EventPath == "" || inputs.Output == "" || inputs.TrustedRoot == "" || !shaPattern.MatchString(inputs.TrustedSHA) {
		return workflowEvent{}, ErrInput
	}
	raw, err := readBounded(inputs.EventPath)
	if err != nil {
		return workflowEvent{}, err
	}
	event, err := parseWorkflowEvent(raw)
	if err != nil {
		return workflowEvent{}, err
	}
	command := exec.CommandContext(ctx, "git", "-C", inputs.TrustedRoot, "rev-parse", "HEAD")
	command.Env = trustedEnvironment()
	command.WaitDelay = time.Second
	sha, err := command.Output()
	if err != nil || strings.TrimSpace(string(sha)) != inputs.TrustedSHA {
		return workflowEvent{}, ErrInput
	}
	status, err := trustedGit(ctx, inputs.TrustedRoot, []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching", "--ignore-submodules=none"})
	if err != nil || len(status) != 0 {
		return workflowEvent{}, fmt.Errorf("%w: trusted checkout must be clean", ErrInput)
	}
	boundary := inputs
	boundary.Snapshot = inputs.EventPath
	if err := outputBoundary(boundary); err != nil {
		return workflowEvent{}, err
	}
	return event, nil
}

func (remote *Remote) Plan(ctx context.Context, inputs Inputs) (Plan, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	event, err := bindInputs(ctx, inputs)
	if err != nil {
		return Plan{}, err
	}
	number, err := remote.api.bindPR(ctx, event)
	if err != nil {
		return Plan{}, err
	}
	return remote.plan(ctx, inputs, remoteBinding{event, number, inputs.TrustedSHA})
}

func (remote *Remote) plan(ctx context.Context, inputs Inputs, binding remoteBinding) (Plan, error) {
	snapshot, err := os.MkdirTemp("", "companion-projection-")
	if err != nil {
		return Plan{}, err
	}
	defer os.RemoveAll(snapshot)
	event, err := remote.api.project(ctx, binding, snapshot)
	if err != nil {
		return Plan{}, err
	}
	local := inputs
	local.Snapshot = snapshot
	local.EventPath = filepath.Join(snapshot, "event.json")
	plan, err := remote.planner.DryRun(ctx, local)
	if err != nil {
		return Plan{}, err
	}
	fresh, err := remote.api.readPR(ctx, binding)
	if err != nil {
		return Plan{}, err
	}
	if !reflect.DeepEqual(event, fresh) {
		return Plan{}, ErrInput
	}
	return plan, nil
}

func (remote *Remote) Apply(ctx context.Context, inputs Inputs, appSlug string) (Plan, error) {
	if os.Getenv("DEPENDABOT_COMPANION_APPLY_ENABLED") != "true" {
		return Plan{}, fmt.Errorf("%w: apply disabled", ErrInput)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	event, err := bindInputs(ctx, inputs)
	if err != nil {
		return Plan{}, err
	}
	signatory, err := remote.api.authorizeApp(ctx, appSlug)
	if err != nil {
		return Plan{}, err
	}
	number, err := remote.api.bindPR(ctx, event)
	if err != nil {
		return Plan{}, err
	}
	binding := remoteBinding{event, number, inputs.TrustedSHA}
	initial, err := remote.api.readPR(ctx, binding)
	if err != nil {
		return Plan{}, err
	}
	first, err := remote.plan(ctx, inputs, binding)
	if err != nil {
		return Plan{}, err
	}
	if first.Status != "planned" {
		return first, nil
	}
	scratch, err := os.MkdirTemp("", "companion-revalidation-")
	if err != nil {
		return Plan{}, err
	}
	defer os.RemoveAll(scratch)
	secondInputs := inputs
	secondInputs.Output = filepath.Join(scratch, "output")
	second, err := remote.plan(ctx, secondInputs, binding)
	if err != nil {
		return Plan{}, err
	}
	a, err := json.Marshal(first)
	if err != nil {
		return Plan{}, err
	}
	b, err := json.Marshal(second)
	if err != nil {
		return Plan{}, err
	}
	if hash(a) != hash(b) {
		return Plan{}, fmt.Errorf("%w: regenerated plan differs", ErrInput)
	}
	commit, err := prepareCommit(second, secondInputs.Output, signatory)
	if err != nil {
		return Plan{}, err
	}
	if err := remote.api.checkPreimages(ctx, second); err != nil {
		return Plan{}, err
	}
	fresh, err := remote.api.readPR(ctx, binding)
	if err != nil {
		return Plan{}, err
	}
	if !reflect.DeepEqual(initial, fresh) {
		return Plan{}, ErrInput
	}
	commit.Branch.RefName = event.Run.HeadBranch
	if err := remote.api.commitAndReconcile(ctx, commit); err != nil {
		return Plan{}, err
	}
	return second, nil
}
