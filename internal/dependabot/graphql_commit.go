package dependabot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
)

type fileAddition struct {
	Path     string `json:"path"`
	Contents string `json:"contents"`
}
type commitInput struct {
	Branch struct {
		Repository string `json:"repositoryNameWithOwner"`
		RefName    string `json:"refName"`
	} `json:"branch"`
	ExpectedHeadOID string `json:"expectedHeadOid"`
	FileChanges     struct {
		Additions []fileAddition `json:"additions"`
	} `json:"fileChanges"`
	Message struct {
		Headline string `json:"headline"`
		Body     string `json:"body"`
	} `json:"message"`
}

func prepareCommit(plan Plan, output string, signatory appSignatory) (commitInput, error) {
	var input commitInput
	if plan.Status != "planned" || len(plan.Files) == 0 || len(plan.Files) > 200 || strings.Count(plan.CommitMessage, "<W4B_AUTHORIZED_DCO_SIGNATORY>") != 1 || !strings.Contains(plan.CommitMessage, "[dependabot skip]") {
		return input, ErrInput
	}
	patch, err := readBounded(filepath.Join(output, "patch.diff"))
	if err != nil || hash(patch) != plan.PatchSHA256 {
		return input, ErrInput
	}
	input.Branch.Repository = plan.Repository
	input.ExpectedHeadOID = plan.HeadSHA
	message := strings.Replace(plan.CommitMessage, "<W4B_AUTHORIZED_DCO_SIGNATORY>", signatory.dco(), 1)
	input.Message.Headline, input.Message.Body, _ = strings.Cut(message, "\n\n")
	for _, file := range plan.Files {
		if !allowedOutput(file.Path) {
			return commitInput{}, ErrInput
		}
		data, err := readBounded(filepath.Join(output, "files", filepath.FromSlash(file.Path)))
		if err != nil || hash(data) != file.SHA256 {
			return commitInput{}, ErrInput
		}
		input.FileChanges.Additions = append(input.FileChanges.Additions, fileAddition{file.Path, base64.StdEncoding.EncodeToString(data)})
	}
	return input, nil
}

func (api *githubAPI) commitAndReconcile(ctx context.Context, input commitInput) error {
	request := struct {
		Query     string `json:"query"`
		Variables struct {
			Input commitInput `json:"input"`
		} `json:"variables"`
	}{Query: `mutation($input: CreateCommitOnBranchInput!) { createCommitOnBranch(input: $input) { commit { oid } } }`}
	request.Variables.Input = input
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if len(body) > maxInput {
		return ErrInput
	}
	raw, err := api.request(ctx, "/graphql", body)
	if err != nil {
		return ErrIndeterminate
	}
	result, err := decodeGitHub[struct {
		Data struct {
			Create struct {
				Commit struct {
					OID string `json:"oid"`
				} `json:"commit"`
			} `json:"createCommitOnBranch"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}](raw)
	if err != nil || len(result.Errors) != 0 || !shaPattern.MatchString(result.Data.Create.Commit.OID) {
		return ErrIndeterminate
	}
	return api.reconcile(ctx, result.Data.Create.Commit.OID, input)
}
