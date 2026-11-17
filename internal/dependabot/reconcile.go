package dependabot

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
)

func (api *githubAPI) reconcile(ctx context.Context, oid string, input commitInput) error {
	commit, err := getJSON[struct {
		SHA     string `json:"sha"`
		Parents []struct {
			SHA string `json:"sha"`
		} `json:"parents"`
		Commit struct {
			Message      string `json:"message"`
			Verification struct {
				Verified bool `json:"verified"`
			} `json:"verification"`
		} `json:"commit"`
	}](ctx, api, repositoryAPI+"/commits/"+oid)
	if err != nil || commit.SHA != oid || len(commit.Parents) != 1 || commit.Parents[0].SHA != input.ExpectedHeadOID || !commit.Commit.Verification.Verified || strings.TrimSuffix(commit.Commit.Message, "\n") != strings.TrimSuffix(input.Message.Headline+"\n\n"+input.Message.Body, "\n") {
		return ErrIndeterminate
	}
	for _, file := range input.FileChanges.Additions {
		data, err := api.contents(ctx, remoteFile{companionRepository, oid, file.Path})
		if err != nil || base64.StdEncoding.EncodeToString(data) != file.Contents {
			return ErrIndeterminate
		}
	}
	ref, err := getJSON[struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}](ctx, api, repositoryAPI+"/git/ref/heads/"+url.PathEscape(input.Branch.RefName))
	if err != nil || ref.Object.SHA != oid {
		return ErrIndeterminate
	}
	return nil
}
