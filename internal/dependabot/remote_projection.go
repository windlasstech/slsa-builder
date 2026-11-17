package dependabot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
)

type githubPR struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	User   struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
	Base         githubRef `json:"base"`
	Head         githubRef `json:"head"`
	ChangedFiles int       `json:"changed_files"`
}

type changedFile struct {
	Filename         string `json:"filename"`
	Status           string `json:"status"`
	PreviousFilename string `json:"previous_filename"`
}

type remoteBinding struct {
	event      workflowEvent
	number     int
	trustedSHA string
}

func (api *githubAPI) readPR(ctx context.Context, binding remoteBinding) (Event, error) {
	path := fmt.Sprintf("%s/pulls/%d", repositoryAPI, binding.number)
	pr, err := getJSON[githubPR](ctx, api, path)
	if err != nil {
		return Event{}, err
	}
	if pr.Number != binding.number || pr.User.Login != "dependabot[bot]" || pr.User.Type != "Bot" || pr.Base.Repo.FullName != companionRepository || pr.Head.Ref != binding.event.Run.HeadBranch || pr.Head.SHA != binding.event.Run.HeadSHA || pr.ChangedFiles < 1 || pr.ChangedFiles > 200 {
		return Event{}, ErrInput
	}
	files, err := listGitHub[changedFile](ctx, api, path+"/files")
	if err != nil {
		return Event{}, err
	}
	if len(files) != pr.ChangedFiles {
		return Event{}, ErrInput
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		if file.Status != "modified" || file.PreviousFilename != "" || slices.Contains(paths, file.Filename) {
			return Event{}, ErrInput
		}
		paths = append(paths, file.Filename)
	}
	slices.Sort(paths)
	event := Event{SchemaVersion: 1, Repository: companionRepository, Number: pr.Number, Author: pr.User.Login,
		State: pr.State, BaseRef: pr.Base.Ref, BaseSHA: pr.Base.SHA, HeadRepository: pr.Head.Repo.FullName,
		HeadSHA: pr.Head.SHA, TrustedSHA: binding.trustedSHA, ChangedPaths: paths, LintConclusion: binding.event.Run.Conclusion}
	if reason := eventReason(event, event, binding.trustedSHA); reason != "" {
		return Event{}, fmt.Errorf("%w: %s", ErrInput, reason)
	}
	return event, nil
}

func (api *githubAPI) project(ctx context.Context, binding remoteBinding, directory string) (Event, error) {
	event, err := api.readPR(ctx, binding)
	if err != nil {
		return Event{}, err
	}
	var modules [2]Modules
	for i, ref := range []string{event.BaseSHA, event.HeadSHA} {
		mod, err := api.contents(ctx, remoteFile{companionRepository, ref, "go.mod"})
		if err != nil {
			return Event{}, err
		}
		sum, err := api.contents(ctx, remoteFile{companionRepository, ref, "go.sum"})
		if err != nil {
			return Event{}, err
		}
		modules[i] = Modules{Mod: mod, Sum: sum}
	}
	for i, name := range []string{"base", "head"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0700); err != nil {
			return Event{}, err
		}
		if err := writeModules(filepath.Join(directory, name), modules[i]); err != nil {
			return Event{}, err
		}
	}
	transition, reason := ParseSigstoreModuleTransition(modules[0], modules[1])
	if reason == "" {
		release, err := api.release(ctx, transition)
		if err != nil {
			return Event{}, err
		}
		if err := writeProjection(filepath.Join(directory, "release.json"), release); err != nil {
			return Event{}, err
		}
	}
	fresh, err := api.readPR(ctx, binding)
	if err != nil {
		return Event{}, err
	}
	if !reflect.DeepEqual(fresh, event) {
		return Event{}, ErrInput
	}
	return event, writeProjection(filepath.Join(directory, "event.json"), event)
}

func writeProjection[T any](path string, value T) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}
