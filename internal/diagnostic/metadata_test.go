package diagnostic

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildMetadataForms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		manifest PackageManifest
	}{
		{name: "absent source fields"},
		{name: "string alternatives", manifest: PackageManifest{Repository: "github:example/widget", License: "MIT", Author: "Maintainer"}},
		{name: "required object members", manifest: PackageManifest{
			Repository: map[string]any{"type": "git", "url": "https://github.com/example/widget.git"},
			License:    map[string]any{"type": "MIT"}, Author: map[string]any{"name": "Maintainer"},
		}},
		{name: "all optional members", manifest: PackageManifest{
			Repository:  map[string]any{"type": "git", "url": "https://github.com/example/widget.git", "directory": "packages/widget"},
			License:     map[string]any{"type": "MIT", "url": "https://example.com/license"},
			Author:      map[string]any{"name": "Maintainer", "email": "maintainer@example.com", "url": "https://example.com"},
			Description: "Example package.", Homepage: "https://example.com/widget", Keywords: []string{"example", "widget"},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			metadata := &DiagnosticMetadata{PackageManifest: &test.manifest}
			report, err := Build(nil, nil, metadata)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			encoded, err := report.CanonicalJSON()
			if err != nil {
				t.Fatalf("CanonicalJSON() error = %v", err)
			}
			var decoded Report
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("decode canonical report: %v", err)
			}
			if !reflect.DeepEqual(decoded.DiagnosticMetadata, metadata) {
				t.Fatalf("serialized metadata = %#v, want %#v", decoded.DiagnosticMetadata, metadata)
			}
			if decoded.Result != ResultPass || decoded.ExitCode != ExitCodePass || decoded.PrimaryID != nil {
				t.Fatalf("metadata-only result = %q/%d/%v, want pass/0/nil", decoded.Result, decoded.ExitCode, decoded.PrimaryID)
			}
		})
	}
}

func TestBuildRejectsMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		manifest PackageManifest
		reason   string
	}{
		{"repository number", PackageManifest{Repository: 1}, "package_manifest.repository has an invalid JSON form"},
		{"license array", PackageManifest{License: []string{"MIT"}}, "package_manifest.license has an invalid JSON form"},
		{"author boolean", PackageManifest{Author: true}, "package_manifest.author has an invalid JSON form"},
		{"empty repository", PackageManifest{Repository: ""}, "package_manifest.repository is empty or secret-like"},
		{"empty license", PackageManifest{License: ""}, "package_manifest.license is empty or secret-like"},
		{"secret author", PackageManifest{Author: "Bearer fake"}, "package_manifest.author is empty or secret-like"},
		{"repository credentials", PackageManifest{Repository: "https://fake:fake@example.com/repo"}, "package_manifest.repository is empty or secret-like"},
		{"repository missing type", PackageManifest{Repository: map[string]any{"url": "https://example.com"}}, `package_manifest.repository requires non-empty string member "type"`},
		{"repository missing url", PackageManifest{Repository: map[string]any{"type": "git"}}, `package_manifest.repository requires non-empty string member "url"`},
		{"license empty type", PackageManifest{License: map[string]any{"type": ""}}, `package_manifest.license requires non-empty string member "type"`},
		{"author nonstring name", PackageManifest{Author: map[string]any{"name": 1}}, `package_manifest.author requires non-empty string member "name"`},
		{"repository unknown member", PackageManifest{Repository: map[string]any{"type": "git", "url": "https://example.com", "extra": "x"}}, `package_manifest.repository has unknown member "extra"`},
		{"license unknown member", PackageManifest{License: map[string]any{"type": "MIT", "extra": "x"}}, `package_manifest.license has unknown member "extra"`},
		{"author unknown member", PackageManifest{Author: map[string]any{"name": "Maintainer", "extra": "x"}}, `package_manifest.author has unknown member "extra"`},
		{"repository empty directory", PackageManifest{Repository: map[string]any{"type": "git", "url": "https://example.com", "directory": ""}}, `package_manifest.repository member "directory" is invalid or secret-like`},
		{"license nonstring url", PackageManifest{License: map[string]any{"type": "MIT", "url": false}}, `package_manifest.license member "url" is invalid or secret-like`},
		{"author secret email", PackageManifest{Author: map[string]any{"name": "Maintainer", "email": "npm_fake"}}, `package_manifest.author member "email" is invalid or secret-like`},
		{"repository object credentials", PackageManifest{Repository: map[string]any{"type": "git", "url": "https://fake@example.com"}}, `package_manifest.repository member "url" is invalid or secret-like`},
		{"secret description", PackageManifest{Description: "Basic fake"}, "package_manifest contains secret-like metadata"},
		{"secret homepage", PackageManifest{Homepage: "https://fake@example.com"}, "package_manifest contains secret-like metadata"},
		{"secret keyword after safe keyword", PackageManifest{Keywords: []string{"safe", "github_pat_fake"}}, "package_manifest contains secret-like metadata"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Build(nil, nil, &DiagnosticMetadata{PackageManifest: &test.manifest})
			requireContractReason(t, err, test.reason)
		})
	}
}
