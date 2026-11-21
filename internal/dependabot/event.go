package dependabot

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"slices"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

var ErrInput = errors.New("invalid companion input")
var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Event is a closed local projection, not a raw GitHub webhook. W4B must bind
// this projection to authenticated API reads at the exact base/head commits.
type Event struct {
	SchemaVersion  int      `json:"schema_version"`
	Repository     string   `json:"repository"`
	Number         int      `json:"number"`
	Author         string   `json:"author"`
	State          string   `json:"state"`
	BaseRef        string   `json:"base_ref"`
	BaseSHA        string   `json:"base_sha"`
	HeadRepository string   `json:"head_repository"`
	HeadSHA        string   `json:"head_sha"`
	TrustedSHA     string   `json:"trusted_sha"`
	ChangedPaths   []string `json:"changed_paths"`
	LintConclusion string   `json:"lint_conclusion"`
}

func ParseEvent(raw []byte) (Event, error) {
	event, err := decodeClosed[Event](raw)
	if err != nil {
		return Event{}, err
	}
	if event.SchemaVersion != 1 || event.Number <= 0 || event.Repository == "" || event.Author == "" || event.State == "" || event.BaseRef == "" || event.HeadRepository == "" || event.LintConclusion == "" || event.ChangedPaths == nil ||
		!shaPattern.MatchString(event.BaseSHA) || !shaPattern.MatchString(event.HeadSHA) || !shaPattern.MatchString(event.TrustedSHA) {
		return Event{}, ErrInput
	}
	slices.Sort(event.ChangedPaths)
	return event, nil
}

func decodeClosed[T any](raw []byte) (T, error) {
	var result T
	if len(raw) > maxInput || canonicaljson.Validate(raw) != nil {
		return result, ErrInput
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return result, ErrInput
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil {
		return result, ErrInput
	}
	return result, nil
}

func eventReason(event, snapshot Event, trustedSHA string) string {
	switch {
	case event.Repository != "windlasstech/slsa-builder", snapshot.Repository != event.Repository:
		return "wrong-repository"
	case event.Author != "dependabot[bot]", snapshot.Author != event.Author:
		return "not-dependabot"
	case event.State != "open", snapshot.State != "open":
		return "pull-request-not-open"
	case event.HeadRepository != event.Repository, snapshot.HeadRepository != event.Repository:
		return "fork-pull-request"
	case event.BaseRef != "main", snapshot.BaseRef != "main":
		return "wrong-base"
	case event.TrustedSHA != trustedSHA, snapshot.TrustedSHA != trustedSHA, event.BaseSHA != trustedSHA, snapshot.BaseSHA != event.BaseSHA:
		return "wrong-trusted-base"
	case event.HeadSHA != snapshot.HeadSHA, event.Number != snapshot.Number, event.HeadSHA == event.BaseSHA:
		return "stale-head"
	case !slices.Equal(event.ChangedPaths, snapshot.ChangedPaths):
		return "changed-paths-mismatch"
	}
	if len(event.ChangedPaths) < 1 || len(event.ChangedPaths) > 2 {
		return "mixed-files-or-companion-edits"
	}
	for i, path := range event.ChangedPaths {
		if (path != "go.mod" && path != "go.sum") || (i > 0 && path == event.ChangedPaths[i-1]) {
			return "mixed-files-or-companion-edits"
		}
	}
	return ""
}
