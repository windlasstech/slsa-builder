package dependabot

import (
	"bytes"
	"context"
	"crypto/sha1" // Git object IDs use SHA-1; the module checksum uses SHA-256.
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
	"unicode"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"
)

const (
	maxModuleEntries       = 10000
	maxModuleFiles         = 2048
	maxModuleBlob          = 8 << 20
	maxModuleBytes         = 128 << 20
	maxModuleResponseBytes = 192 << 20
)

type moduleRequestBudget struct{ requests, bytes int }
type moduleTreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size *int64 `json:"size"`
}
type moduleTree struct {
	SHA       string            `json:"sha"`
	Truncated *bool             `json:"truncated"`
	Tree      []moduleTreeEntry `json:"tree"`
}

type moduleFile struct {
	entry moduleTreeEntry
	data  []byte
}

func (f *moduleFile) Path() string { return f.entry.Path }
func (f *moduleFile) Name() string { return path.Base(f.entry.Path) }
func (f *moduleFile) Size() int64  { return *f.entry.Size }
func (f *moduleFile) Mode() fs.FileMode {
	if f.entry.Mode == "100755" {
		return 0755
	}
	return 0644
}
func (*moduleFile) ModTime() time.Time            { return time.Time{} }
func (*moduleFile) IsDir() bool                   { return false }
func (*moduleFile) Sys() any                      { return nil }
func (f *moduleFile) Lstat() (fs.FileInfo, error) { return f, nil }
func (f *moduleFile) Open() (io.ReadCloser, error) {
	if f.data == nil {
		return nil, ErrInput
	}
	return io.NopCloser(bytes.NewReader(f.data)), nil
}

func modulePathFold(p string) string {
	return strings.Map(func(r rune) rune {
		lowest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < lowest {
				lowest = next
			}
		}
		return lowest
	}, p)
}

func moduleInventory(tree moduleTree) (map[string]*moduleFile, error) {
	if tree.Truncated == nil || *tree.Truncated || len(tree.Tree) == 0 || len(tree.Tree) > maxModuleEntries {
		return nil, ErrInput
	}
	entries := make(map[string]moduleTreeEntry, len(tree.Tree))
	folded := make(map[string]string, len(tree.Tree))
	files := make(map[string]*moduleFile)
	for _, entry := range tree.Tree {
		p := entry.Path
		if p == "." || p != path.Clean(p) || strings.Contains(p, "\\") || module.CheckFilePath(p) != nil || !shaPattern.MatchString(entry.SHA) {
			return nil, ErrInput
		}
		if _, exists := entries[p]; exists {
			return nil, ErrInput
		}
		key := modulePathFold(p)
		if _, exists := folded[key]; exists {
			return nil, ErrInput
		}
		folded[key], entries[p] = p, entry
		switch entry.Mode {
		case "040000":
			if entry.Type != "tree" {
				return nil, ErrInput
			}
		case "100644", "100755":
			if entry.Type != "blob" || entry.Size == nil || *entry.Size < 0 {
				return nil, ErrInput
			}
			files[p] = &moduleFile{entry: entry}
		default:
			return nil, ErrInput
		}
	}
	for p := range entries {
		for parent := path.Dir(p); parent != "."; parent = path.Dir(parent) {
			if entry, exists := entries[parent]; !exists || entry.Type != "tree" {
				return nil, ErrInput
			}
		}
	}
	if files["go.mod"] == nil {
		return nil, ErrInput
	}
	return files, nil
}

func (api *githubAPI) moduleBlob(ctx context.Context, file *moduleFile) error {
	if file.Size() > maxModuleBlob {
		return ErrInput
	}
	blob, err := getJSON[struct {
		SHA      string  `json:"sha"`
		Encoding string  `json:"encoding"`
		Size     *int64  `json:"size"`
		Content  *string `json:"content"`
	}](ctx, api, signerRepositoryAPI+"/git/blobs/"+file.entry.SHA)
	if err != nil {
		return err
	}
	if blob.SHA != file.entry.SHA || blob.Encoding != "base64" || blob.Size == nil || *blob.Size != file.Size() || blob.Content == nil {
		return ErrInput
	}
	data, err := base64.StdEncoding.Strict().DecodeString(*blob.Content)
	if err != nil || int64(len(data)) != file.Size() {
		return ErrInput
	}
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	if fmt.Sprintf("%x", h.Sum(nil)) != file.entry.SHA {
		return ErrInput
	}
	file.data = data
	return nil
}

func (api *githubAPI) verifyCandidateModule(ctx context.Context, release resolvedRelease, expected string) ([]byte, error) {
	v := release.metadata.Tag
	if module.CanonicalVersion(v) != v || module.Check(signerModule, v) != nil {
		return nil, ErrInput
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	bounded := *api
	bounded.budget = &moduleRequestBudget{}
	commit, err := getJSON[struct {
		SHA  string `json:"sha"`
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}](ctx, &bounded, signerRepositoryAPI+"/git/commits/"+release.commit)
	if err != nil {
		return nil, err
	}
	if commit.SHA != release.commit || !shaPattern.MatchString(commit.Tree.SHA) {
		return nil, ErrInput
	}
	tree, err := getJSON[moduleTree](ctx, &bounded, signerRepositoryAPI+"/git/trees/"+commit.Tree.SHA+"?recursive=1")
	if err != nil {
		return nil, err
	}
	if tree.SHA != commit.Tree.SHA {
		return nil, ErrInput
	}
	files, err := moduleInventory(tree)
	if err != nil {
		return nil, err
	}
	root := files["go.mod"]
	if root.Size() > modzip.MaxGoMod {
		return nil, ErrInput
	}
	if err := bounded.moduleBlob(ctx, root); err != nil {
		return nil, err
	}
	// CheckFiles opens root go.mod in x/mod v0.42.0 and suppresses read errors.
	// Supply authenticated, repeatable memory only; metadata exposes nested modules.
	metadata := make([]modzip.File, 0, len(files))
	for _, file := range files {
		metadata = append(metadata, file)
	}
	checked, err := modzip.CheckFiles(metadata)
	if err != nil || len(checked.Valid) > maxModuleFiles {
		return nil, ErrInput
	}
	var total int64
	for _, p := range checked.Valid {
		size := files[p].Size()
		if size > maxModuleBlob || size > maxModuleBytes-total {
			return nil, ErrInput
		}
		total += size
	}
	prefix := signerModule + "@" + v + "/"
	names := make([]string, 0, len(checked.Valid))
	for _, p := range checked.Valid {
		if p != "go.mod" {
			if err := bounded.moduleBlob(ctx, files[p]); err != nil {
				return nil, err
			}
		}
		names = append(names, prefix+p)
	}
	sum, err := dirhash.Hash1(names, func(name string) (io.ReadCloser, error) {
		return files[strings.TrimPrefix(name, prefix)].Open()
	})
	if err != nil || sum != expected {
		return nil, ErrInput
	}
	return root.data, nil
}
