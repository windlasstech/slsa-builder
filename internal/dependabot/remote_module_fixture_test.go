package dependabot

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func remoteModuleFixture(t *testing.T, responses map[string]string) string {
	t.Helper()
	return remoteModuleManifestFixture(t, responses, "module github.com/sigstore/sigstore-go\n\ngo 1.27.0\n")
}

func remoteModuleManifestFixture(t *testing.T, responses map[string]string, manifest string) string {
	t.Helper()
	files := map[string]string{
		"go.mod":                     manifest,
		"LICENSE":                    "Apache License, Version 2.0\n",
		"main.go":                    "package sigstore\n",
		"run.sh":                     "#!/bin/sh\nexit 0\n",
		"empty":                      "",
		"nested/go.mod":              "module example.com/nested\n",
		"nested/omit.go":             "package nested\n",
		"vendor/example.com/omit.go": "package vendored\n",
	}
	var entries []map[string]any
	for _, p := range []string{"nested", "vendor", "vendor/example.com"} {
		entries = append(entries, map[string]any{"path": p, "mode": "040000", "type": "tree", "sha": strings.Repeat("9", 40)})
	}
	archive := filepath.Join(t.TempDir(), "module.zip")
	out, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	for p, data := range files {
		oid := fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(data), data))))
		mode := "100644"
		if p == "run.sh" {
			mode = "100755"
		}
		entries = append(entries, map[string]any{"path": p, "mode": mode, "type": "blob", "sha": oid, "size": len(data)})
		responses[signerRepositoryAPI+"/git/blobs/"+oid] = fixtureJSON(t, map[string]any{"sha": oid, "size": len(data), "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(data))})
		// The expected zip independently lists the root-module files, not CheckFiles' output.
		if strings.HasPrefix(p, "nested/") || strings.HasPrefix(p, "vendor/") {
			continue
		}
		writer, err := zw.Create(signerModule + "@v1.999.0/" + p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	sum, err := dirhash.HashZip(archive, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}
	tree := strings.Repeat("8", 40)
	responses[signerRepositoryAPI+"/git/commits/"+strings.Repeat("c", 40)] = fmt.Sprintf(`{"sha":%q,"tree":{"sha":%q}}`, strings.Repeat("c", 40), tree)
	responses[signerRepositoryAPI+"/git/trees/"+tree] = fixtureJSON(t, map[string]any{"sha": tree, "truncated": false, "tree": entries})
	return sum
}
