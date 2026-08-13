package npmprofile

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSourceRefTag(t *testing.T) {
	t.Parallel()

	remote := gitRemoteFixture(t)
	lightweight := gitOutput(t, "git", "-C", remote, "rev-parse", "refs/tags/v1.2.3")
	annotated := gitOutput(t, "git", "-C", remote, "rev-parse", "refs/tags/v1.2.4^{}")

	tests := []struct {
		name      string
		sourceRef string
		want      string
	}{
		{name: "lightweight tag", sourceRef: "refs/tags/v1.2.3", want: lightweight},
		{name: "annotated tag peels to commit", sourceRef: "refs/tags/v1.2.4", want: annotated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			revision, err := ResolveSourceRefTag(context.Background(), remote, test.sourceRef)
			if err != nil {
				t.Fatalf("ResolveSourceRefTag() error = %v", err)
			}
			if revision != test.want {
				t.Fatalf("ResolveSourceRefTag() = %q, want %q", revision, test.want)
			}
		})
	}

	t.Run("missing tag", func(t *testing.T) {
		_, err := ResolveSourceRefTag(context.Background(), remote, "refs/tags/v9.9.9")
		requireNPMDiagnostic(t, err, IDSourceRefInvalid)
	})

	t.Run("invalid form", func(t *testing.T) {
		_, err := ResolveSourceRefTag(context.Background(), remote, "refs/heads/main")
		requireNPMDiagnostic(t, err, IDSourceRefInvalid)
	})

	t.Run("unreachable remote", func(t *testing.T) {
		_, err := ResolveSourceRefTag(context.Background(), filepath.Join(t.TempDir(), "absent.git"), "refs/tags/v1.2.3")
		requireNPMDiagnostic(t, err, IDSourceRefInvalid)
	})
}

func TestSourceRefRejectedFixtures(t *testing.T) {
	t.Parallel()

	root := testRepositoryRoot(t)
	descriptors, err := filepath.Glob(filepath.Join(root, "testdata", "npm", "source-ref", "*.json"))
	if err != nil || len(descriptors) != 4 {
		t.Fatalf("source-ref rejected descriptors = %v, want 4 (%v)", descriptors, err)
	}
	for _, descriptorPath := range descriptors {
		descriptorPath := descriptorPath
		t.Run(filepath.Base(descriptorPath), func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(descriptorPath)
			if err != nil {
				t.Fatal(err)
			}
			var descriptor struct {
				Stage          string `json:"stage"`
				SourceRef      string `json:"source-ref"`
				InvocationRef  string `json:"invocation-ref"`
				PackageVersion string `json:"package-version"`
				Expectation    string `json:"expectation"`
			}
			if err := json.Unmarshal(data, &descriptor); err != nil {
				t.Fatal(err)
			}
			if descriptor.Expectation != IDSourceRefInvalid {
				t.Fatalf("descriptor expectation = %q, want %q", descriptor.Expectation, IDSourceRefInvalid)
			}
			switch descriptor.Stage {
			case "input":
				requireNPMDiagnostic(t, ValidateSourceRefInput(descriptor.SourceRef, descriptor.InvocationRef), IDSourceRefInvalid)
			case "resolution":
				remote := filepath.Join(t.TempDir(), "empty.git")
				gitOutput(t, "git", "init", "--bare", remote)
				_, err := ResolveSourceRefTag(context.Background(), remote, descriptor.SourceRef)
				requireNPMDiagnostic(t, err, IDSourceRefInvalid)
			case "metadata":
				selection, build := workflowMetadataFixture(t)
				selection.Package.Version = descriptor.PackageVersion
				build.PackageVersion = descriptor.PackageVersion
				build.Packed.Version = descriptor.PackageVersion
				_, err := FinalizeWorkflowBuildMetadata(selection, build, WorkflowBuildMetadataConfig{
					ArtifactName: "js-ts-npm-package-tarball-123456789-1", EventName: "workflow_dispatch",
					RefType: "branch", Ref: descriptor.InvocationRef, Revision: testSourceSHA,
					SourceRefInput:         descriptor.SourceRef,
					InvocationRef:          descriptor.InvocationRef,
					InvocationRevision:     testAttestSHA,
					WorkflowSHA:            testSourceSHA,
					CallerWorkflowFilename: "release.yml",
				})
				requireNPMDiagnostic(t, err, IDSourceRefInvalid)
			default:
				t.Fatalf("unknown descriptor stage %q", descriptor.Stage)
			}
		})
	}
}

func gitRemoteFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git executable is unavailable")
	}
	root := t.TempDir()
	work := filepath.Join(root, "work")
	remote := filepath.Join(root, "remote.git")
	gitOutput(t, "git", "init", "--bare", "--initial-branch=main", remote)
	gitOutput(t, "git", "clone", remote, work)
	for name, contents := range map[string]string{"package.json": `{"name":"pkg","version":"1.2.3"}` + "\n"} {
		if err := os.WriteFile(filepath.Join(work, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitOutput(t, "git", "-C", work, "-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "add", ".")
	gitOutput(t, "git", "-C", work, "-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "commit", "-m", "initial")
	gitOutput(t, "git", "-C", work, "tag", "v1.2.3")
	gitOutput(t, "git", "-C", work, "-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "tag", "-a", "-m", "release", "v1.2.4")
	gitOutput(t, "git", "-C", work, "push", "--tags", "origin", "main")
	return remote
}

func gitOutput(t *testing.T, name string, arguments ...string) string {
	t.Helper()
	output, err := exec.Command(name, arguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v: %s", name, arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}
