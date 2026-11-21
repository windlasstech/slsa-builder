package dependabot

import (
	"bytes"
	"context"
	"fmt"
	"go/version"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/windlasstech/slsa-builder/internal/provenance"
	"golang.org/x/mod/modfile"
)

type Inputs struct {
	EventPath, Snapshot, TrustedRoot, TrustedSHA, Output string
}

type Release struct {
	Module      string `json:"module"`
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
	GoVersion   string `json:"go_version"`
	BaseLicense string `json:"base_license"`
	License     string `json:"license"`
}

type prepared struct {
	event      Event
	transition Transition
	release    Release
}

func load(ctx context.Context, inputs Inputs) (prepared, string, error) {
	var result prepared
	if !shaPattern.MatchString(inputs.TrustedSHA) {
		return result, "", ErrInput
	}
	raw, err := readBounded(inputs.EventPath)
	if err != nil {
		return result, "", err
	}
	result.event, err = ParseEvent(raw)
	if err != nil {
		return result, "", err
	}
	raw, err = readBounded(filepath.Join(inputs.Snapshot, "event.json"))
	if err != nil {
		return result, "", err
	}
	snapshot, err := ParseEvent(raw)
	if err != nil {
		return result, "", err
	}
	if reason := eventReason(result.event, snapshot, inputs.TrustedSHA); reason != "" {
		return result, reason, nil
	}
	for _, name := range []string{"base", "head"} {
		entries, err := os.ReadDir(filepath.Join(inputs.Snapshot, name))
		if err != nil {
			return result, "", err
		}
		for _, entry := range entries {
			if entry.Name() != "go.mod" && entry.Name() != "go.sum" {
				return result, "mixed-files-or-companion-edits", nil
			}
		}
	}
	base, err := readModules(filepath.Join(inputs.Snapshot, "base"))
	if err != nil {
		return result, "", err
	}
	head, err := readModules(filepath.Join(inputs.Snapshot, "head"))
	if err != nil {
		return result, "", err
	}
	trusted, err := readCommittedFiles(ctx, inputs, []string{"go.mod", "go.sum"})
	if err != nil {
		return result, "", err
	}
	if !bytes.Equal(base.Mod, trusted["go.mod"]) || !bytes.Equal(base.Sum, trusted["go.sum"]) {
		return result, "trusted-module-mismatch", nil
	}
	var paths []string
	if !bytes.Equal(base.Mod, head.Mod) {
		paths = append(paths, "go.mod")
	}
	if !bytes.Equal(base.Sum, head.Sum) {
		paths = append(paths, "go.sum")
	}
	if !slices.Equal(paths, result.event.ChangedPaths) {
		return result, "changed-paths-mismatch", nil
	}
	transition, reason := ParseSigstoreModuleTransition(base, head)
	if reason != "" {
		return result, reason, nil
	}
	compiled := provenance.SigningAdapterBuilderDependency()
	if compiled.URI != "pkg:golang/"+signerModule+"@"+transition.old.Version || compiled.Digest["dirHash1"] != transition.old.DirHash1 {
		return result, "compiled-signer-mismatch", nil
	}
	result.transition = transition
	raw, err = readBounded(filepath.Join(inputs.Snapshot, "release.json"))
	if err != nil {
		return result, "release-metadata-required", nil
	}
	result.release, err = decodeClosed[Release](raw)
	if err != nil {
		return result, "release-metadata-invalid", nil
	}
	if !validRelease(result.release, transition) {
		return result, "release-metadata-unconfirmed", nil
	}
	return result, "", nil
}

func validRelease(release Release, transition Transition) bool {
	date, err := time.Parse("2006-01-02", release.ReleaseDate)
	manifest, parseErr := modfile.Parse("go.mod", transition.base.Mod, nil)
	return err == nil && date.Format("2006-01-02") == release.ReleaseDate && parseErr == nil && manifest.Go != nil &&
		release.Module == signerModule && release.Version == transition.candidate.Version &&
		release.License == "Apache-2.0" && release.BaseLicense == release.License &&
		version.IsValid("go"+release.GoVersion) && version.Compare("go"+release.GoVersion, "go"+manifest.Go.Version) <= 0
}

func readModules(root string) (Modules, error) {
	mod, err := readBounded(filepath.Join(root, "go.mod"))
	if err != nil {
		return Modules{}, err
	}
	sum, err := readBounded(filepath.Join(root, "go.sum"))
	if err != nil {
		return Modules{}, err
	}
	return Modules{Mod: mod, Sum: sum}, nil
}

func readBounded(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("read companion input: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxInput {
		return nil, ErrInput
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return nil, ErrInput
	}
	data, err := io.ReadAll(io.LimitReader(file, maxInput+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxInput {
		return nil, ErrInput
	}
	return data, nil
}
