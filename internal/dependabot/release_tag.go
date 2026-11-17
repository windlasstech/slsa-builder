package dependabot

import (
	"context"
	"net/url"
	"strings"
	"time"
)

const signerRepositoryAPI = "/repos/sigstore/sigstore-go"

type releaseMetadata struct {
	Tag        string `json:"tag_name"`
	Target     string `json:"target_commitish"`
	Published  string `json:"published_at"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

type gitTarget struct {
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

type gitTagRef struct {
	Ref    string    `json:"ref"`
	Object gitTarget `json:"object"`
}

type resolvedRelease struct {
	metadata releaseMetadata
	ref      gitTagRef
	commit   string
	date     time.Time
}

func (api *githubAPI) resolveRelease(ctx context.Context, version string) (resolvedRelease, error) {
	metadata, err := getJSON[releaseMetadata](ctx, api, signerRepositoryAPI+"/releases/tags/"+url.PathEscape(version))
	if err != nil {
		return resolvedRelease{}, err
	}
	date, err := time.Parse(time.RFC3339, metadata.Published)
	if err != nil || metadata.Tag != version || metadata.Draft || metadata.Prerelease {
		return resolvedRelease{}, ErrInput
	}
	ref, err := api.tagRef(ctx, version)
	if err != nil {
		return resolvedRelease{}, err
	}
	target := ref.Object
	seen := map[string]bool{}
	for depth := 0; ; depth++ {
		if !shaPattern.MatchString(target.SHA) || seen[target.SHA] {
			return resolvedRelease{}, ErrInput
		}
		seen[target.SHA] = true
		switch target.Type {
		case "commit":
			// A branch-valued target_commitish is only a creation hint, not an immutable release binding.
			if expected := strings.ToLower(metadata.Target); shaPattern.MatchString(expected) && expected != target.SHA {
				return resolvedRelease{}, ErrInput
			}
			return resolvedRelease{metadata: metadata, ref: ref, commit: target.SHA, date: date}, nil
		case "tag":
			if depth >= 8 {
				return resolvedRelease{}, ErrInput
			}
			tag, err := getJSON[struct {
				SHA    string    `json:"sha"`
				Object gitTarget `json:"object"`
			}](ctx, api, signerRepositoryAPI+"/git/tags/"+target.SHA)
			if err != nil {
				return resolvedRelease{}, err
			}
			if tag.SHA != target.SHA {
				return resolvedRelease{}, ErrInput
			}
			target = tag.Object
		default:
			return resolvedRelease{}, ErrInput
		}
	}
}

func (api *githubAPI) tagRef(ctx context.Context, version string) (gitTagRef, error) {
	ref, err := getJSON[gitTagRef](ctx, api, signerRepositoryAPI+"/git/ref/tags/"+url.PathEscape(version))
	if err != nil {
		return gitTagRef{}, err
	}
	if ref.Ref != "refs/tags/"+version || !shaPattern.MatchString(ref.Object.SHA) {
		return gitTagRef{}, ErrInput
	}
	return ref, nil
}
