package command

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestBuildRejectsInputsBeforeRegistryAccess(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, flag, value, want string }{
		{"tarball traversal", "--artifact-name", "../escape", "invalid tarball artifact name"},
		{"metadata traversal", "--metadata-artifact-name", "../escape", "invalid build metadata artifact name"},
		{"publish access", "--access", "invalid", "access is invalid"},
		{"insecure registry", "--registry-url", "http://registry.npmjs.org", "credential-free HTTPS"},
		{"unsafe HTTP transport", "--access", "public", "custom HTTP transport has an unsupported type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			transport := &countingRejectTransport{}
			args := npmBuildSourceRefArguments(copyNPMBuildFixture(t), t.TempDir(), filepath.Join(t.TempDir(), "output"), signRegistryURL, "refs/tags/v1.0.0", signInvocationRef, invocationRevision)
			args = append(args, test.flag, test.value)
			var output bytes.Buffer
			err := (npmProfileBuildCommand{httpClient: &http.Client{Transport: transport}}).Execute(t.Context(), args, &output)
			if err == nil || !strings.Contains(err.Error(), test.want) || output.Len() != 0 || transport.requests != 0 {
				t.Fatalf("Execute() = %v, output = %q, requests = %d; want %q before registry access", err, output.String(), transport.requests, test.want)
			}
		})
	}
}

func TestBuildSelectionAndSourceReportsPropagateWriterFailure(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"selection", "source"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			repository := copyNPMBuildFixture(t)
			if stage == "selection" {
				repository = t.TempDir()
			}
			args := npmBuildSourceRefArguments(repository, t.TempDir(), filepath.Join(t.TempDir(), "output"), signRegistryURL, "refs/tags/v9.0.0", signInvocationRef, invocationRevision)
			cause := errors.New("report sink unavailable")
			err := NewNPMProfileBuildCommand().Execute(t.Context(), args, &stagedWriter{err: cause})
			if !errors.Is(err, cause) || errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("Execute() = %v, want sink cause without verification sentinel", err)
			}
		})
	}
}

func TestBuildStopsAtRegistryOrOutputFailure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		status int
		want   string
	}{
		{"registry denied", http.StatusForbidden, "registry packument returned HTTP 403"},
		{"missing build output", http.StatusNotFound, "output directory must be an existing non-symlink directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewTLSServer(http.HandlerFunc(func(out http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || request.URL.Path != "/windlass-fixture-unscoped" {
					t.Errorf("registry request = %s %s", request.Method, request.URL.Path)
				}
				out.WriteHeader(test.status)
			}))
			t.Cleanup(server.Close)
			args := npmBuildSourceRefArguments(copyNPMBuildFixture(t), filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "output"), server.URL, "refs/tags/v1.0.0", signInvocationRef, invocationRevision)
			var output bytes.Buffer
			err := (npmProfileBuildCommand{httpClient: server.Client()}).Execute(t.Context(), args, &output)
			if err == nil || err.Error() != test.want || output.Len() != 0 {
				t.Fatalf("Execute() = %v, output = %q, want %q without report", err, output.String(), test.want)
			}
		})
	}
}

func TestBuildSelectionFailureEmitsPolicyReport(t *testing.T) {
	t.Parallel()
	args := npmBuildSourceRefArguments(t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "output"), signRegistryURL, "refs/tags/v1.0.0", signInvocationRef, invocationRevision)
	var output bytes.Buffer
	err := NewNPMProfileBuildCommand().Execute(t.Context(), args, &output)
	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("Execute() = %v, want verification sentinel", err)
	}
	assertPrimaryDiagnostic(t, output.Bytes(), 1, npmprofile.IDPackageManifestInvalid)
}

func TestMetadataPackageURLRejectsMalformedParameters(t *testing.T) {
	t.Parallel()
	if got := metadataPackageURL(npmprofile.BuildMetadata{ExternalParameters: []byte(`{}`)}); got != "" {
		t.Fatalf("metadataPackageURL() = %q, want empty for malformed parameters", got)
	}
}

func TestSelectionAdaptersRejectMalformedManifest(t *testing.T) {
	t.Parallel()
	for _, command := range []Command{NewNPMProfileBuildCommand(), NewNPMProfileSelectCommand()} {
		t.Run(command.Name(), func(t *testing.T) {
			t.Parallel()
			repository := copyNPMBuildFixture(t)
			manifest := `{"name":"windlass-fixture-unscoped","version":42}`
			if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"--repository-root", repository, "--package-directory", ".", "--observed-repository", "windlasstech/slsa-builder"}
			if command.Name() == "npm-profile-build" {
				args = npmBuildSourceRefArguments(repository, t.TempDir(), filepath.Join(t.TempDir(), "output"), signRegistryURL, "refs/tags/v1.0.0", signInvocationRef, invocationRevision)
			}
			var output bytes.Buffer
			err := command.Execute(t.Context(), args, &output)
			if !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("Execute() = %v, want manifest verification failure", err)
			}
			assertPrimaryDiagnostic(t, output.Bytes(), 1, npmprofile.IDPackageManifestInvalid)
		})
	}
}
