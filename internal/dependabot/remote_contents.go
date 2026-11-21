package dependabot

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/url"
	"strings"

	"golang.org/x/mod/modfile"
)

type remoteFile struct{ repository, ref, path string }
type githubContent struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	Size     int    `json:"size"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
	License  struct {
		SPDX string `json:"spdx_id"`
	} `json:"license"`
}

func (content githubContent) bytes(path string) ([]byte, error) {
	if content.Type != "file" || content.Path != path || content.Size < 0 || content.Size > maxInput || content.Encoding != "base64" {
		return nil, ErrInput
	}
	data, err := base64.StdEncoding.Strict().DecodeString(content.Content)
	if err != nil || len(data) != content.Size {
		return nil, ErrInput
	}
	return data, nil
}

func (api *githubAPI) contents(ctx context.Context, file remoteFile) ([]byte, error) {
	parts := strings.Split(file.path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	content, err := getJSON[githubContent](ctx, api, "/repos/"+file.repository+"/contents/"+strings.Join(parts, "/")+"?ref="+url.QueryEscape(file.ref))
	if err != nil {
		return nil, err
	}
	return content.bytes(file.path)
}

func (api *githubAPI) release(ctx context.Context, transition Transition) (Release, error) {
	old, err := api.resolveRelease(ctx, transition.old.Version)
	if err != nil {
		return Release{}, err
	}
	candidate, err := api.resolveRelease(ctx, transition.candidate.Version)
	if err != nil {
		return Release{}, err
	}
	var licenses [2][]byte
	for i, resolved := range []resolvedRelease{old, candidate} {
		license, err := getJSON[githubContent](ctx, api, signerRepositoryAPI+"/license?ref="+resolved.commit)
		if err != nil {
			return Release{}, err
		}
		if license.License.SPDX != "Apache-2.0" {
			return Release{}, ErrInput
		}
		licenses[i], err = license.bytes("LICENSE")
		if err != nil {
			return Release{}, err
		}
	}
	if len(licenses[0]) == 0 || !bytes.Equal(licenses[0], licenses[1]) {
		return Release{}, ErrInput
	}
	manifest, err := api.verifyCandidateModule(ctx, candidate, transition.candidate.Sum)
	if err != nil {
		return Release{}, err
	}
	parsed, err := modfile.Parse("go.mod", manifest, nil)
	if err != nil || parsed.Go == nil || parsed.Module == nil || parsed.Module.Mod.Path != signerModule {
		return Release{}, ErrInput
	}
	for _, resolved := range []resolvedRelease{old, candidate} {
		fresh, err := api.tagRef(ctx, resolved.metadata.Tag)
		if err != nil {
			return Release{}, err
		}
		if fresh != resolved.ref {
			return Release{}, ErrInput
		}
	}
	release := Release{Module: signerModule, Version: candidate.metadata.Tag, ReleaseDate: candidate.date.UTC().Format("2006-01-02"),
		GoVersion: parsed.Go.Version, BaseLicense: "Apache-2.0", License: "Apache-2.0"}
	if !validRelease(release, transition) {
		return Release{}, ErrInput
	}
	return release, nil
}
