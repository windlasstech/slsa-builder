package dependabot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

type FileHash struct {
	Path         string `json:"path"`
	BeforeSHA256 string `json:"before_sha256"`
	SHA256       string `json:"sha256"`
}

type Plan struct {
	SchemaVersion int        `json:"schema_version"`
	Status        string     `json:"status"`
	Reason        string     `json:"reason"`
	Repository    string     `json:"repository"`
	Number        int        `json:"number"`
	BaseSHA       string     `json:"base_sha"`
	HeadSHA       string     `json:"head_sha"`
	TrustedSHA    string     `json:"trusted_sha"`
	Files         []FileHash `json:"files"`
	PatchSHA256   string     `json:"patch_sha256"`
	CommitMessage string     `json:"commit_message"`
	DCOPolicy     string     `json:"dco_policy"`
}

type Change struct {
	Path          string
	Before, After []byte
}
type Generation struct {
	inputs   Inputs
	prepared prepared
}
type Generator func(context.Context, Generation) ([]Change, error)
type Planner struct{ Generate Generator }

func (planner Planner) DryRun(ctx context.Context, inputs Inputs) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	if err := outputBoundary(inputs); err != nil {
		return Plan{}, err
	}
	prepared, reason, err := load(ctx, inputs)
	if err != nil {
		return Plan{}, err
	}
	event := prepared.event
	plan := Plan{SchemaVersion: 1, Status: "skip", Reason: reason, Repository: event.Repository, Number: event.Number, BaseSHA: event.BaseSHA, HeadSHA: event.HeadSHA, TrustedSHA: inputs.TrustedSHA, Files: []FileHash{}, DCOPolicy: "W4B must obtain an explicitly authorized DCO signatory; bot identity alone is not authorization. Placeholder must be replaced before commit."}
	var changes []Change
	if reason == "" {
		generate := planner.Generate
		if generate == nil {
			generate = generateTrusted
		}
		changes, err = generate(ctx, Generation{inputs: inputs, prepared: prepared})
		if err != nil {
			return Plan{}, err
		}
		if len(changes) == 0 {
			return Plan{}, errors.New("companion generator produced no changes")
		}
		plan.Status = "planned"
		plan.Reason = "eligible-sigstore-transition"
		plan.CommitMessage = fmt.Sprintf("chore(deps): synchronize sigstore-go %s companion files [dependabot skip]\n\nSigned-off-by: <W4B_AUTHORIZED_DCO_SIGNATORY>\n", prepared.transition.candidate.Version)
	} else if manualReason(reason) {
		plan.Status = "manual-action-required"
	}
	slices.SortFunc(changes, func(a, b Change) int {
		if a.Path < b.Path {
			return -1
		}
		if a.Path > b.Path {
			return 1
		}
		return 0
	})
	outputs := map[string][]byte{}
	var patch []byte
	for i, change := range changes {
		if !allowedOutput(change.Path) || (i > 0 && changes[i-1].Path == change.Path) {
			return Plan{}, errors.New("generator output outside closed allowlist")
		}
		plan.Files = append(plan.Files, FileHash{Path: change.Path, BeforeSHA256: hash(change.Before), SHA256: hash(change.After)})
		outputs["files/"+change.Path] = change.After
		patch = append(patch, unifiedPatch(change)...)
	}
	plan.PatchSHA256 = hash(patch)
	encoded, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return Plan{}, err
	}
	outputs["plan.json"] = append(encoded, '\n')
	outputs["patch.diff"] = patch
	if err := publishOutputs(inputs.Output, outputs); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func manualReason(reason string) bool {
	switch reason {
	case "unsupported-module-directive", "unsupported-transitive-change", "version-not-increased", "invalid-module-checksums", "invalid-module-manifest", "module-input-too-large", "release-metadata-required", "release-metadata-invalid", "release-metadata-unconfirmed", "compiled-signer-mismatch", "trusted-module-mismatch":
		return true
	default:
		return false
	}
}

func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
