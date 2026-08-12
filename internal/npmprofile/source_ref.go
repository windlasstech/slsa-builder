package npmprofile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/identity"
)

const maxTagPeelDepth = 16

// SourceRefResolutionConfig contains caller selection and trusted event fallback inputs.
type SourceRefResolutionConfig struct {
	HTTPClient                  *http.Client
	APIURL, Token, Repository   string
	SourceRef, EventRef         string
	EventRevision, EventRefType string
}

// ResolvedSource identifies the immutable content selected for the build.
type ResolvedSource struct {
	Ref, Revision, RefType string
}

// ResolveSourceRef validates and resolves an optional full tag ref without changing unset behavior.
func ResolveSourceRef(ctx context.Context, config SourceRefResolutionConfig) (ResolvedSource, error) {
	if config.SourceRef == "" {
		if config.EventRef == "" || config.EventRevision == "" || config.EventRefType == "" {
			return ResolvedSource{}, sourceRefError("event source identity is incomplete")
		}
		return ResolvedSource{Ref: config.EventRef, Revision: config.EventRevision, RefType: config.EventRefType}, nil
	}
	if identity.ValidateReleaseRef(config.SourceRef) != nil {
		return ResolvedSource{}, sourceRefError("source-ref must be a full refs/tags reference")
	}
	if config.Repository == "" {
		return ResolvedSource{}, sourceRefError("source repository is unavailable")
	}
	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	apiURL := strings.TrimSuffix(config.APIURL, "/")
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	tagName := strings.TrimPrefix(config.SourceRef, "refs/tags/")
	object, err := fetchGitObject(ctx, client, config.Token, apiURL+"/repos/"+config.Repository+"/git/ref/tags/"+escapePath(tagName))
	if err != nil {
		return ResolvedSource{}, sourceRefError("source-ref does not resolve: " + err.Error())
	}
	for range maxTagPeelDepth {
		switch object.Type {
		case "commit":
			if identity.ValidateFullSHA(object.SHA) != nil {
				return ResolvedSource{}, sourceRefError("source-ref resolved to an invalid commit SHA")
			}
			return ResolvedSource{Ref: config.SourceRef, Revision: object.SHA, RefType: "tag"}, nil
		case "tag":
			object, err = fetchGitObject(ctx, client, config.Token, apiURL+"/repos/"+config.Repository+"/git/tags/"+object.SHA)
			if err != nil {
				return ResolvedSource{}, sourceRefError("source-ref tag cannot be peeled: " + err.Error())
			}
		default:
			return ResolvedSource{}, sourceRefError("source-ref does not peel to a commit")
		}
	}
	return ResolvedSource{}, sourceRefError("source-ref tag peel depth exceeded")
}

type gitObject struct {
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

func fetchGitObject(ctx context.Context, client *http.Client, token, endpoint string) (gitObject, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return gitObject{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return gitObject{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return gitObject{}, fmt.Errorf("GitHub API returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		Object gitObject `json:"object"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return gitObject{}, errors.New("GitHub API returned malformed JSON")
	}
	if payload.Object.Type == "" || payload.Object.SHA == "" {
		return gitObject{}, errors.New("GitHub API response omitted the Git object")
	}
	return payload.Object, nil
}

func escapePath(value string) string {
	parts := strings.Split(value, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func sourceRefError(message string) error {
	return fmt.Errorf("%s: %s", IDReleaseRefMismatch, message)
}
