package dependabot

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

const companionRepository = "windlasstech/slsa-builder"
const repositoryAPI = "/repos/" + companionRepository

type githubRepository struct {
	FullName string `json:"full_name"`
}
type githubRef struct {
	SHA  string           `json:"sha"`
	Ref  string           `json:"ref"`
	Repo githubRepository `json:"repo"`
}
type workflowEvent struct {
	Action     string           `json:"action"`
	Repository githubRepository `json:"repository"`
	Run        struct {
		Name           string           `json:"name"`
		Status         string           `json:"status"`
		Conclusion     string           `json:"conclusion"`
		Event          string           `json:"event"`
		HeadSHA        string           `json:"head_sha"`
		HeadBranch     string           `json:"head_branch"`
		HeadRepository githubRepository `json:"head_repository"`
		PullRequests   []struct {
			Number int       `json:"number"`
			Head   githubRef `json:"head"`
			Base   githubRef `json:"base"`
		} `json:"pull_requests"`
	} `json:"workflow_run"`
}

func parseWorkflowEvent(raw []byte) (workflowEvent, error) {
	event, err := decodeGitHub[workflowEvent](raw)
	if err != nil {
		return event, err
	}
	run := event.Run
	if event.Action != "completed" || event.Repository.FullName != companionRepository || run.Name != "Lint" || run.Status != "completed" || run.Conclusion == "" || run.Event != "pull_request" || run.HeadRepository.FullName != companionRepository || !shaPattern.MatchString(run.HeadSHA) || !validHeadBranch(run.HeadBranch) || len(run.PullRequests) > 1 {
		return event, ErrInput
	}
	if len(run.PullRequests) == 1 {
		pr := run.PullRequests[0]
		if pr.Number <= 0 || pr.Head.SHA != run.HeadSHA || pr.Head.Ref != run.HeadBranch || pr.Base.Ref != "main" {
			return event, ErrInput
		}
	}
	return event, nil
}

func validHeadBranch(branch string) bool {
	return strings.HasPrefix(branch, "dependabot/") && len(branch) <= 250 &&
		!strings.ContainsAny(branch, " \t\r\n\\~^:?*[") && !strings.Contains(branch, "..") &&
		!strings.Contains(branch, "@{") && !strings.HasSuffix(branch, "/") && !strings.HasSuffix(branch, ".lock")
}

func (api *githubAPI) bindPR(ctx context.Context, event workflowEvent) (int, error) {
	if len(event.Run.PullRequests) == 1 {
		return event.Run.PullRequests[0].Number, nil
	}
	path := repositoryAPI + "/pulls?state=open&head=" + url.QueryEscape("windlasstech:"+event.Run.HeadBranch)
	prs, err := listGitHub[githubPR](ctx, api, path)
	if err != nil {
		return 0, err
	}
	if len(prs) != 1 || prs[0].Number <= 0 || prs[0].Head.Repo.FullName != companionRepository || prs[0].Head.Ref != event.Run.HeadBranch || prs[0].Head.SHA != event.Run.HeadSHA {
		return 0, ErrInput
	}
	return prs[0].Number, nil
}

// Enumerate until an empty page, rather than trusting a possibly truncated first page.
func listGitHub[T any](ctx context.Context, api *githubAPI, path string) ([]T, error) {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	var all []T
	for page := 1; page <= 3; page++ {
		items, err := getJSON[[]T](ctx, api, fmt.Sprintf("%s%sper_page=100&page=%d", path, separator, page))
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return all, nil
		}
		if len(items) > 100 || len(all)+len(items) > 200 {
			return nil, ErrInput
		}
		all = append(all, items...)
	}
	return nil, ErrInput
}
