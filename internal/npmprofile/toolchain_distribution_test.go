package npmprofile

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPNPMIntegrityDistributionResponses(t *testing.T) {
	// newFetchTLSServer changes http.DefaultTransport, so these cases are serial.
	for _, test := range []struct{ name, body, want string }{
		{"success", `{"dist":{"tarball":"https://example.test/pnpm.tgz","integrity":"sha512-` + base64.StdEncoding.EncodeToString(make([]byte, 64)) + `"}}`, ""},
		{"wrong shape", `{"dist":[]}`, "pnpm registry metadata disagrees with the Corepack distribution URL"},
		{"wrong tarball", `{"dist":{"tarball":"https://other.test/a"}}`, "pnpm registry metadata disagrees with the Corepack distribution URL"},
		{"wrong algorithm", `{"dist":{"tarball":"https://example.test/pnpm.tgz","integrity":"sha256-AA=="}}`, "pnpm registry metadata lacks one SHA-512 integrity value"},
		{"multiple values", `{"dist":{"tarball":"https://example.test/pnpm.tgz","integrity":"sha512-AA== sha512-AA=="}}`, "pnpm registry metadata lacks one SHA-512 integrity value"},
		{"invalid base64", `{"dist":{"tarball":"https://example.test/pnpm.tgz","integrity":"sha512-!"}}`, "pnpm registry SHA-512 integrity value is malformed"},
		{"short digest", `{"dist":{"tarball":"https://example.test/pnpm.tgz","integrity":"sha512-AA=="}}`, "pnpm registry SHA-512 integrity value is malformed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := newFetchTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/pnpm/11.0.0" || r.Header.Get("Accept") != "application/json" {
					t.Errorf("registry request = %s %s %v", r.Method, r.URL, r.Header)
				}
				if _, err := w.Write([]byte(test.body)); err != nil {
					t.Errorf("write metadata: %v", err)
				}
			}))
			fetcher := func(ctx context.Context, rawURL string, maximum int64, accept string) ([]byte, error) {
				if rawURL != "https://registry.npmjs.org/pnpm/11.0.0" || maximum != maxRegistryMetadata {
					t.Fatalf("metadata fetch = %q, %d", rawURL, maximum)
				}
				return fetchHTTPS(ctx, server.URL+"/pnpm/11.0.0", maximum, accept)
			}
			got, err := pnpmRegistryIntegrity(t.Context(), "11.0.0", "https://example.test/pnpm.tgz", fetcher)
			if test.want == "" {
				if err != nil || got != strings.Repeat("0", 128) {
					t.Fatalf("integrity = %q, %v; want zero SHA-512", got, err)
				}
			} else if err == nil || err.Error() != test.want || got != "" {
				t.Fatalf("integrity = %q, %v; want empty, %q", got, err, test.want)
			}
		})
	}
}

func TestCorepackToolchainPreparationFailures(t *testing.T) {
	for _, test := range []struct{ name, node, npm, corepack, want string }{
		{"missing node", "", "", "", "node executable is unavailable"},
		{"missing npm", "printf v24.0.0", "", "", "npm executable is unavailable"},
		{"node failure", "exit 8", "printf 11.5.1", "", "node [--version] failed: exit status 8: "},
		{"npm failure", "printf v24.0.0", "exit 8", "", "npm [--version] failed: exit status 8: "},
		{"old node", "printf v22.0.0", "printf 11.5.1", "", `required Node.js major is 24; executed "v22.0.0"`},
		{"old npm", "printf v24.0.0", "printf 10.0.0", "", `npm 11.5.1 or newer is required, executed "10.0.0"`},
		{"missing corepack", "printf v24.0.0", "printf 11.5.1", "", "corepack executable is unavailable"},
		{"corepack version failure", "printf v24.0.0", "printf 11.5.1", "exit 8", "corepack [--version] failed: exit status 8: "},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("PATH", root)
			t.Setenv("ImageOS", "")
			t.Setenv("ImageVersion", "")
			for name, body := range map[string]string{"node": test.node, "npm": test.npm, "corepack": test.corepack} {
				if body != "" {
					writeScript(t, root, name, "#!/bin/sh\n"+body+"\n")
				}
			}
			selection := Result{Manager: ManagerSelection{Name: ManagerPNPM, Version: "11.0.0"}}
			selection.Package.RealManagerRoot = root
			_, executable, err := prepareToolchain(t.Context(), selection, root, []string{}, nil)
			if err == nil || err.Error() != test.want || executable != "" {
				t.Fatalf("prepareToolchain() executable=%q error=%v; want %q", executable, err, test.want)
			}
		})
	}
}

func TestCaptureDistributionAcquisitionFailure(t *testing.T) {
	t.Parallel()
	for _, manager := range []Manager{ManagerPNPM, ManagerYarn} {
		t.Run(string(manager), func(t *testing.T) {
			t.Parallel()
			root := createRepository(t, map[string]string{"v1/" + string(manager) + "/4.0.0/.corepack": fmt.Sprintf(`{"locator":{"name":%q,"reference":"4.0.0"},"hash":"sha512.%s"}`, manager, strings.Repeat("0", 128))})
			sentinel := errors.New("acquisition unavailable")
			fetcher := func(context.Context, string, int64, string) ([]byte, error) { return nil, sentinel }
			got, err := captureDistribution(t.Context(), root, ManagerSelection{Name: manager, Version: "4.0.0"}, " corepack Installing "+string(manager)+"@4.0.0 from "+expectedDistributionURL(manager, "4.0.0"), fetcher)
			if !errors.Is(err, sentinel) || got != nil {
				t.Fatalf("capture = %v, %v; want wrapped acquisition failure", got, err)
			}
		})
	}
}

func TestCorepackShimDirectoryCollision(t *testing.T) {
	root := t.TempDir()
	writeScript(t, root, "node", "#!/bin/sh\nprintf v24.0.0\n")
	writeScript(t, root, "npm", "#!/bin/sh\nprintf 11.5.1\n")
	writeScript(t, root, "corepack", "#!/bin/sh\nprintf 0.34.1\n")
	t.Setenv("PATH", root)
	t.Setenv("ImageOS", "")
	t.Setenv("ImageVersion", "")
	if err := os.Mkdir(filepath.Join(root, "shims"), 0o700); err != nil {
		t.Fatal(err)
	}
	selection := Result{Manager: ManagerSelection{Name: ManagerPNPM, Version: "11.0.0"}}
	selection.Package.RealManagerRoot = root
	_, _, err := prepareToolchain(t.Context(), selection, root, []string{}, nil)
	if !errors.Is(err, os.ErrExist) || !strings.HasPrefix(err.Error(), "create Corepack shim directory: ") {
		t.Fatalf("shim collision = %v; want wrapped os.ErrExist", err)
	}
}

func TestYarnDistributionDownload(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("canceled=%t", canceled), func(t *testing.T) {
			server := newFetchTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if canceled {
					t.Error("cancelled download reached server")
				}
				if r.Method != http.MethodGet || r.URL.Path != "/yarn.js" || r.Header.Get("Accept") != "application/octet-stream" {
					t.Errorf("Yarn request = %s %s %v", r.Method, r.URL, r.Header)
				}
				if _, err := w.Write([]byte("bundle")); err != nil {
					t.Errorf("write bundle: %v", err)
				}
			}))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if canceled {
				cancel()
			}
			got, err := downloadYarnDistribution(ctx, server.URL+"/yarn.js", fetchHTTPS)
			if canceled {
				if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "download Yarn distribution evidence: ") || got != nil {
					t.Fatalf("cancelled download = %q, %v", got, err)
				}
			} else if err != nil || string(got) != "bundle" {
				t.Fatalf("download = %q, %v; want bundle, nil", got, err)
			}
		})
	}
}

func TestCorepackDistributionURLRejections(t *testing.T) {
	t.Parallel()
	for _, endpoint := range []string{"http://example.test/a", "https://user@example.test/a", "https://example.test/a?token=x", "https://example.test/a#x", "https://%invalid"} {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			got, err := corepackDistributionURL("noise\n corepack Installing pnpm@11.0.0 from "+endpoint, ManagerPNPM, "11.0.0")
			if err == nil || err.Error() != "corepack reported an invalid distribution URL" || got != "" {
				t.Fatalf("distribution URL = %q, %v; want invalid URL rejection", got, err)
			}
		})
	}
}

func TestCaptureDistributionMetadataRejections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, metadata, want string }{
		{"missing", "", "read Corepack distribution metadata: path is not a bounded regular file"},
		{"wrong locator", `{"locator":{"name":"yarn","reference":"11.0.0"}}`, "corepack distribution metadata disagrees with the executed manager"},
		{"wrong shape", `{"locator":[]}`, "corepack distribution metadata disagrees with the executed manager"},
		{"wrong hash", `{"locator":{"name":"pnpm","reference":"11.0.0"},"hash":"sha512.00"}`, "corepack distribution metadata lacks a canonical SHA-512 hash"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{}
			if test.metadata != "" {
				files["v1/pnpm/11.0.0/.corepack"] = test.metadata
			}
			root := createRepository(t, files)
			fetcher := func(context.Context, string, int64, string) ([]byte, error) {
				t.Error("invalid cache triggered network acquisition")
				return nil, errors.New("unexpected fetch")
			}
			got, err := captureDistribution(t.Context(), root, ManagerSelection{Name: ManagerPNPM, Version: "11.0.0"}, " corepack Installing pnpm@11.0.0 from https://registry.npmjs.org/pnpm/-/pnpm-11.0.0.tgz", fetcher)
			if err == nil || err.Error() != test.want || got != nil {
				t.Fatalf("captureDistribution() = %v, %v; want nil, %q", got, err, test.want)
			}
		})
	}
}
