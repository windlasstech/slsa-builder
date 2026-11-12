package fixture

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateFixtureEvidenceSchema(t *testing.T) {
	if os.Getenv("UPDATE_FIXTURE_INDEX") != "1" {
		t.Skip("set UPDATE_FIXTURE_INDEX=1 to migrate the registry schema")
	}
	root := filepath.Join("..", "..", "testdata", "fixtures")
	indexPath := filepath.Join(root, "index.json")
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	var index Index
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filename == indexPath || !strings.HasSuffix(filename, ".json") {
			return nil
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(`"manifest-evidence"`)) || !bytes.Contains(data, []byte(`"release-manifest":`)) {
			return nil
		}
		data = bytes.ReplaceAll(data, []byte(`"release-manifest":`), []byte(`"manifest-evidence":null,"release-manifest":`))
		return os.WriteFile(filename, data, 0o600)
	}); err != nil {
		t.Fatal(err)
	}
}
