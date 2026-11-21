package dependabot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

var ErrGitHub = errors.New("GitHub companion request failed")
var ErrIndeterminate = errors.New("post-mutation indeterminate; inspect the branch, do not retry or roll back")

type githubToken string

func (githubToken) String() string   { return "[REDACTED]" }
func (githubToken) GoString() string { return "[REDACTED]" }

type githubAPI struct {
	client *http.Client
	origin string
	token  githubToken
	budget *moduleRequestBudget
}

type githubStatusError struct{ Code int }

func (err *githubStatusError) Error() string { return fmt.Sprintf("%s: HTTP %d", ErrGitHub, err.Code) }
func (*githubStatusError) Unwrap() error     { return ErrGitHub }

func newGitHub(token string) (*githubAPI, error) {
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return nil, ErrInput
	}
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 20 * time.Second,
		IdleConnTimeout: 30 * time.Second, MaxIdleConns: 2, MaxConnsPerHost: 2,
		MaxResponseHeaderBytes: 64 << 10,
	}
	return &githubAPI{origin: "https://api.github.com", token: githubToken(token),
		client: &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: refuseRedirect}}, nil
}

func refuseRedirect(*http.Request, []*http.Request) error { return ErrGitHub }

func (api *githubAPI) request(ctx context.Context, path string, body []byte) ([]byte, error) {
	if api.budget != nil {
		if api.budget.requests >= maxModuleFiles+2 || api.budget.bytes >= maxModuleResponseBytes {
			return nil, ErrInput
		}
		api.budget.requests++
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, ErrInput
	}
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	request, err := http.NewRequestWithContext(ctx, method, api.origin+path, bytes.NewReader(body))
	if err != nil {
		return nil, ErrGitHub
	}
	request.Header.Set("Authorization", "Bearer "+string(api.token))
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := api.client.Do(request)
	if err != nil {
		return nil, ErrGitHub
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &githubStatusError{Code: response.StatusCode}
	}
	limit := maxInput
	if api.budget != nil {
		limit = min(limit, maxModuleResponseBytes-api.budget.bytes)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, int64(limit)+1))
	if err != nil || len(raw) > maxInput {
		return nil, ErrGitHub
	}
	if api.budget != nil {
		api.budget.bytes += len(raw)
		if api.budget.bytes > maxModuleResponseBytes {
			return nil, ErrInput
		}
	}
	return raw, nil
}

func decodeGitHub[T any](raw []byte) (T, error) {
	var result T
	trimmed := bytes.TrimSpace(raw)
	if len(raw) > maxInput || len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') || canonicaljson.Validate(raw) != nil {
		return result, ErrInput
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, ErrInput
	}
	return result, nil
}

func getJSON[T any](ctx context.Context, api *githubAPI, path string) (T, error) {
	raw, err := api.request(ctx, path, nil)
	if err != nil {
		var zero T
		return zero, err
	}
	return decodeGitHub[T](raw)
}
