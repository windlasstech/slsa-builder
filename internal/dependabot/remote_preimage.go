package dependabot

import (
	"context"
	"fmt"
)

func (api *githubAPI) checkPreimages(ctx context.Context, plan Plan) error {
	for _, file := range plan.Files {
		expectedHash := file.BeforeSHA256
		switch file.Path {
		case "go.mod", "go.sum":
			expectedHash = file.SHA256
		}
		content, err := api.contents(ctx, remoteFile{companionRepository, plan.HeadSHA, file.Path})
		if err != nil {
			return err
		}
		if hash(content) != expectedHash {
			return fmt.Errorf("%w: remote preimage differs", ErrInput)
		}
	}
	return nil
}
