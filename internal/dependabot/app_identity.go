package dependabot

import (
	"context"
	"fmt"
	"regexp"
)

var appSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type appSignatory struct {
	login string
	id    int64
}

func (api *githubAPI) authorizeApp(ctx context.Context, slug string) (appSignatory, error) {
	if !appSlugPattern.MatchString(slug) || len(slug) > 100 {
		return appSignatory{}, ErrInput
	}
	installation, err := getJSON[struct {
		Total        int                `json:"total_count"`
		Repositories []githubRepository `json:"repositories"`
	}](ctx, api, "/installation/repositories?per_page=100&page=1")
	if err != nil {
		return appSignatory{}, err
	}
	if installation.Total != 1 || len(installation.Repositories) != 1 || installation.Repositories[0].FullName != companionRepository {
		return appSignatory{}, ErrInput
	}
	raw, err := api.request(ctx, "/graphql", []byte(`{"query":"query { viewer { login databaseId } }"}`))
	if err != nil {
		return appSignatory{}, err
	}
	viewer, err := decodeGitHub[struct {
		Data struct {
			Viewer struct {
				Login string `json:"login"`
				ID    int64  `json:"databaseId"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}](raw)
	if err != nil || len(viewer.Errors) != 0 || viewer.Data.Viewer.Login != slug+"[bot]" || viewer.Data.Viewer.ID <= 0 {
		return appSignatory{}, ErrInput
	}
	user, err := getJSON[struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
	}](ctx, api, "/users/"+slug+"%5Bbot%5D")
	if err != nil {
		return appSignatory{}, err
	}
	if user.Type != "Bot" || user.Login != viewer.Data.Viewer.Login || user.ID != viewer.Data.Viewer.ID {
		return appSignatory{}, ErrInput
	}
	return appSignatory{login: user.Login, id: user.ID}, nil
}

func (signatory appSignatory) dco() string {
	return fmt.Sprintf("%s <%d+%s@users.noreply.github.com>", signatory.login, signatory.id, signatory.login)
}
