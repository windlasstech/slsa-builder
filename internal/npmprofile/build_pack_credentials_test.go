package npmprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

func TestValidateCredentialConfiguration(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, filename, content, want string }{
		{"missing files", "", "", ""},
		{"comments", ".npmrc", "\n # _auth=ignored\n ; username=ignored\nregistry=https://registry.npmjs.org/\n", ""},
		{"benign yarn", ".yarnrc.yml", "nodeLinker: node-modules\n", ""},
		{"benign yaml values", ".yarnrc.yml", "nodeLinker: pnpm\nregistry: \"https://registry.npmjs.org/\"\n", ""},
		{"yaml comments", ".yarnrc.yml", " # httpsKey: secret\n ; httpsCert: secret\n", ""},
		{"auth", ".npmrc", "//registry.npmjs.org/:_auth=secret", `package-manager configuration ".npmrc" contains credential material`},
		{"scoped auth token", ".npmrc", "//host/:_authToken=secret", `package-manager configuration ".npmrc" contains credential material`},
		{"auth token", ".yarnrc.yml", "npmAuthToken: secret", `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"auth identity", ".yarnrc.yml", "npmAuthIdent: secret", `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"username", ".npmrc", " USERNAME =secret", `package-manager configuration ".npmrc" contains credential material`},
		{"password", ".npmrc", "_password=secret", `package-manager configuration ".npmrc" contains credential material`},
		{"yaml username", ".yarnrc.yml", " username: foo", `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"yaml password", ".yarnrc.yml", "_password: x", `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"yaml empty key value", ".yarnrc.yml", "httpsKey:", `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"yaml uppercase key", ".yarnrc.yml", " HTTPSKEY: secret", `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"https key", ".yarnrc.yml", "httpsKey: secret", `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"https certificate", ".yarnrc.yml", "httpsCert: secret", `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"yaml quoted key", ".yarnrc.yml", `"httpsKey": secret`, `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"yaml single-quoted key", ".yarnrc.yml", `'httpsCert': secret`, `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"yaml value with equals", ".yarnrc.yml", "httpsKey: abc=def", `package-manager configuration ".yarnrc.yml" contains credential material`},
		{"yaml quoted benign key", ".yarnrc.yml", `"nodeLinker": pnpm`, ""},
		{"corepack environment", ".corepack.env", "", "project .corepack.env is prohibited for release builds"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, nested := range []bool{false, true} {
				root := t.TempDir()
				pkg := root
				if nested {
					pkg = filepath.Join(root, "package")
					if err := os.Mkdir(pkg, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				if test.filename != "" {
					directory := pkg
					if test.filename == ".corepack.env" {
						directory = root
					}
					if err := os.WriteFile(filepath.Join(directory, test.filename), []byte(test.content), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				err := validateCredentialConfiguration(Result{Package: Package{RealManagerRoot: root, RealDirectory: pkg}})
				requireHelperError(t, err, test.want)
			}
		})
	}
}

func TestValidateCredentialConfigurationRejectsNonRegular(t *testing.T) {
	t.Parallel()
	for _, filename := range []string{".npmrc", ".yarnrc.yml"} {
		for _, symlink := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/symlink=%t", filename, symlink), func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				path := filepath.Join(root, filename)
				if symlink {
					if err := os.Symlink(filepath.Join(root, "missing"), path); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				err := validateCredentialConfiguration(Result{Package: Package{RealManagerRoot: root, RealDirectory: root}})
				requireHelperError(t, err, fmt.Sprintf("package-manager configuration %q must be a regular non-symlink file", filename))
			})
		}
	}
}

func TestValidateBuildPackConfigHelpers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*BuildPackConfig)
		want   string
	}{
		{"valid", func(*BuildPackConfig) {}, ""},
		{"selection failed", func(c *BuildPackConfig) { c.Selection.Report.Result = diagnostic.ResultFail }, "package selection did not pass"},
		{"package root missing", func(c *BuildPackConfig) { c.Selection.Package.RealDirectory = "" }, "package selection lacks resolved real directories"},
		{"manager root missing", func(c *BuildPackConfig) { c.Selection.Package.RealManagerRoot = "" }, "package selection lacks resolved real directories"},
		{"artifact missing", func(c *BuildPackConfig) { c.ArtifactName = "" }, "invalid artifact name: payload file name is empty"},
		{"parameters missing", func(c *BuildPackConfig) { c.ExternalParameters = nil }, "external parameters are required"},
		{"parameters null", func(c *BuildPackConfig) { c.ExternalParameters = json.RawMessage("null") }, "external parameters must be a JSON object"},
		{"parameters array", func(c *BuildPackConfig) { c.ExternalParameters = json.RawMessage("[]") }, "external parameters must be a JSON object"},
		{"yarn unsafe output", func(c *BuildPackConfig) { c.Selection.Manager.Name = ManagerYarn; c.Selection.Package.Version = "../1" }, `invalid Yarn pack output name: payload file name "package-../1.tgz" contains a path separator`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			config := BuildPackConfig{Selection: Result{Report: diagnostic.Report{Result: diagnostic.ResultPass}, Package: Package{RealDirectory: root, RealManagerRoot: root, Name: "package", Version: "1.0.0"}}, ArtifactName: "artifact", ExternalParameters: json.RawMessage(`{}`)}
			test.mutate(&config)
			requireHelperError(t, validateBuildPackConfig(config), test.want)
		})
	}
}

func TestPrepareOutputHelpers(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"empty", "missing", "file", "symlink", "populated"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			output, want := root, ""
			switch kind {
			case "missing":
				output = filepath.Join(root, "missing")
			case "file", "populated":
				file := filepath.Join(root, "occupied")
				if err := os.WriteFile(file, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if kind == "file" {
					output = file
				} else {
					want = "output directory must be readable and empty"
				}
			case "symlink":
				output = filepath.Join(root, "link")
				if err := os.Symlink(t.TempDir(), output); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "missing" || kind == "file" || kind == "symlink" {
				want = "output directory must be an existing non-symlink directory"
			}
			pkg, metadata, err := prepareOutput(output)
			requireHelperError(t, err, want)
			if want != "" {
				if pkg != "" || metadata != "" {
					t.Fatalf("rejected output returned paths %q, %q", pkg, metadata)
				}
				return
			}
			realRoot, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{pkg, metadata} {
				info, err := os.Stat(path)
				if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 || filepath.Dir(path) != realRoot {
					t.Fatalf("output %q: info=%v, error=%v", path, info, err)
				}
			}
			if filepath.Base(pkg) != "package" || filepath.Base(metadata) != "metadata" {
				t.Fatalf("output paths = %q, %q", pkg, metadata)
			}
		})
	}
}

func TestBuildArgumentsHelpers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		manager  Manager
		relative string
		want     []string
	}{
		{ManagerNPM, ".", []string{"run", "build"}}, {ManagerPNPM, ".", []string{"run", "build"}}, {ManagerYarn, ".", []string{"run", "build"}},
		{ManagerNPM, "packages/app", []string{"--workspace", "packages/app", "run", "build"}},
		{ManagerPNPM, "packages/app", []string{"--filter", "{./packages/app}", "run", "build"}},
		{ManagerYarn, "packages/app", []string{"workspace", "@scope/app", "run", "build"}},
		{Manager("unknown"), ".", nil},
	} {
		t.Run(string(test.manager)+"/"+test.relative, func(t *testing.T) {
			t.Parallel()
			selection := Result{Manager: ManagerSelection{Name: test.manager}, Package: Package{Name: "@scope/app", ManagerRootRelativeDirectory: test.relative}}
			if got := buildArguments(selection); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("buildArguments() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestWriteBuildMetadataHelpers(t *testing.T) {
	t.Parallel()
	t.Run("canonical output", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "metadata.json")
		metadata := BuildMetadata{SchemaVersion: "1", ExternalParameters: json.RawMessage(`{"z":1,"a":2}`)}
		if err := writeBuildMetadata(path, metadata); err != nil {
			t.Fatal(err)
		}
		encoded, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var got BuildMetadata
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatal(err)
		}
		if got.SchemaVersion != "1" || string(got.ExternalParameters) != `{"a":2,"z":1}` {
			t.Fatalf("written metadata = %#v", got)
		}
	})
	t.Run("existing file", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "metadata.json")
		if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := writeBuildMetadata(path, BuildMetadata{}); !errors.Is(err, os.ErrExist) {
			t.Fatalf("existing file error = %v, want ErrExist", err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "original" {
			t.Fatalf("existing file changed: %q, %v", got, err)
		}
	})
	t.Run("invalid raw JSON", func(t *testing.T) {
		t.Parallel()
		err := writeBuildMetadata(filepath.Join(t.TempDir(), "metadata.json"), BuildMetadata{ExternalParameters: json.RawMessage(`{`)})
		var syntax *json.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("invalid JSON error = %v, want SyntaxError", err)
		}
	})
	t.Run("duplicate raw JSON", func(t *testing.T) {
		t.Parallel()
		err := writeBuildMetadata(filepath.Join(t.TempDir(), "metadata.json"), BuildMetadata{ExternalParameters: json.RawMessage(`{"a":1,"a":2}`)})
		var duplicate *canonicaljson.DuplicateMemberError
		if !errors.As(err, &duplicate) || duplicate.Member != "a" {
			t.Fatalf("duplicate JSON error = %v, want member a", err)
		}
	})
}

func requireHelperError(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected helper rejection: %v", err)
		}
		return
	}
	if err == nil || err.Error() != want {
		t.Fatalf("helper error = %v, want %q", err, want)
	}
}
