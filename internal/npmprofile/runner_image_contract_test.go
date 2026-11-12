package npmprofile

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func runnerProbeCapture() RunnerCapture {
	return RunnerCapture{
		ImageLabel: "ubuntu-24.04", OperatingSystem: "Ubuntu\n24.04.5\nLTS", ImageVersion: "20261025.353.1",
		DeploymentVersion: "20261026.354.1", IncludedSoftwareURL: runnerSoftwareURL, ImageReleaseURL: runnerReleaseURL,
	}
}

func validRunnerImageDescriptor() provenance.ResourceDescriptor {
	return provenance.ResourceDescriptor{
		Name: "runner-image", URI: runnerSoftwareURL,
		Annotations: map[string]json.RawMessage{
			"deployment_version": mustJSON("20261026.354.1"),
			"operating_system":   mustJSON("Ubuntu\n24.04.5\nLTS"),
			"image_version":      mustJSON("20261025.353.1"),
			"runner_label":       mustJSON("ubuntu-24.04"),
		},
	}
}

func requireRunnerImageRejection(t *testing.T, descriptor provenance.ResourceDescriptor) {
	t.Helper()
	requireNPMDiagnostic(t, validateRunnerDescriptor(descriptor), IDResolvedDependenciesRunnerImage)
}

func TestRunnerImageDescriptorContract(t *testing.T) {
	t.Run("producer emits exactly four independent annotations", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0o600); err != nil {
			t.Fatal(err)
		}
		selection := Result{
			Package: Package{RealManagerRoot: root},
			Manager: ManagerSelection{Name: ManagerNPM, SelectedLockfilePath: "package-lock.json"},
		}
		build := BuildPackResult{Toolchain: ToolchainCapture{Runner: runnerProbeCapture()}}

		dependencies, err := workflowResolvedDependencies(selection, build, resolvedWorkflowSource{Repository: "https://github.com/example/project", Revision: testSourceSHA})

		if err != nil {
			t.Fatalf("workflowResolvedDependencies() error = %v; want nil", err)
		}
		got := dependencies[len(dependencies)-1]
		want := validRunnerImageDescriptor()
		equal, err := equalJSONValues(got, want)
		if err != nil || !equal {
			t.Fatalf("runner-image = %#v; want exact descriptor %#v", got, want)
		}
	})

	t.Run("consumer accepts the probe descriptor and rollout divergence", func(t *testing.T) {
		t.Parallel()
		if err := validateRunnerDescriptor(validRunnerImageDescriptor()); err != nil {
			t.Fatalf("validateRunnerDescriptor() error = %v; want nil", err)
		}
	})

	t.Run("consumer accepts null deployment and meaningful OS whitespace", func(t *testing.T) {
		t.Parallel()
		descriptor := validRunnerImageDescriptor()
		descriptor.Annotations["deployment_version"] = json.RawMessage("null")
		descriptor.Annotations["operating_system"] = mustJSON(" Ubuntu\r\n24.04.5\nLTS ")
		if err := validateRunnerDescriptor(descriptor); err != nil {
			t.Fatalf("validateRunnerDescriptor() error = %v; want nil", err)
		}
	})

	for _, test := range []struct {
		name   string
		mutate func(*provenance.ResourceDescriptor)
	}{
		{name: "image_os is rejected", mutate: func(descriptor *provenance.ResourceDescriptor) {
			delete(descriptor.Annotations, "operating_system")
			descriptor.Annotations["image_os"] = mustJSON("ubuntu24")
		}},
		{name: "missing operating_system is rejected", mutate: func(descriptor *provenance.ResourceDescriptor) {
			delete(descriptor.Annotations, "operating_system")
		}},
		{name: "arbitrary fifth annotation is rejected", mutate: func(descriptor *provenance.ResourceDescriptor) {
			descriptor.Annotations["vendor_evidence"] = mustJSON("raw")
		}},
		{name: "empty operating_system is rejected", mutate: func(descriptor *provenance.ResourceDescriptor) {
			descriptor.Annotations["operating_system"] = mustJSON("")
		}},
		{name: "whitespace operating_system is rejected", mutate: func(descriptor *provenance.ResourceDescriptor) {
			descriptor.Annotations["operating_system"] = mustJSON(" \t\r\n")
		}},
		{name: "non-string operating_system is rejected", mutate: func(descriptor *provenance.ResourceDescriptor) {
			descriptor.Annotations["operating_system"] = mustJSON(42)
		}},
		{name: "missing runner_label is rejected", mutate: func(descriptor *provenance.ResourceDescriptor) {
			delete(descriptor.Annotations, "runner_label")
		}},
		{name: "empty runner_label is rejected", mutate: func(descriptor *provenance.ResourceDescriptor) {
			descriptor.Annotations["runner_label"] = mustJSON("")
		}},
		{name: "whitespace runner_label is rejected", mutate: func(descriptor *provenance.ResourceDescriptor) {
			descriptor.Annotations["runner_label"] = mustJSON(" \t\r\n")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			descriptor := validRunnerImageDescriptor()
			test.mutate(&descriptor)
			requireRunnerImageRejection(t, descriptor)
		})
	}
}

func TestRunnerImageOfflineProjection(t *testing.T) {
	t.Run("matching major minor projection preserves both captured values", func(t *testing.T) {
		t.Parallel()
		descriptor := validRunnerImageDescriptor()
		originalURI := descriptor.URI
		originalVersion := append(json.RawMessage(nil), descriptor.Annotations["image_version"]...)

		if err := validateRunnerDescriptor(descriptor); err != nil {
			t.Fatalf("validateRunnerDescriptor() error = %v; want nil", err)
		}
		if descriptor.URI != originalURI || string(descriptor.Annotations["image_version"]) != string(originalVersion) {
			t.Fatalf("projection rewrote captured identity: %#v", descriptor)
		}
	})

	t.Run("mismatched projection is rejected without rewriting", func(t *testing.T) {
		t.Parallel()
		descriptor := validRunnerImageDescriptor()
		descriptor.URI = "https://github.com/actions/runner-images/blob/ubuntu24/20261026.354/images/ubuntu/Ubuntu2404-Readme.md"
		originalURI := descriptor.URI
		originalVersion := append(json.RawMessage(nil), descriptor.Annotations["image_version"]...)

		requireRunnerImageRejection(t, descriptor)
		if descriptor.URI != originalURI || string(descriptor.Annotations["image_version"]) != string(originalVersion) {
			t.Fatalf("failed projection rewrote captured identity: %#v", descriptor)
		}
	})

	for _, test := range []struct {
		name    string
		version string
	}{
		{name: "truncated version", version: "20261025.353"},
		{name: "extra version component", version: "20261025.353.1.4"},
		{name: "non-numeric version", version: "20261025.353.x"},
		{name: "empty version", version: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			descriptor := validRunnerImageDescriptor()
			descriptor.Annotations["image_version"] = mustJSON(test.version)
			requireRunnerImageRejection(t, descriptor)
		})
	}

	for _, test := range []struct {
		name string
		uri  string
	}{
		{name: "moving main report", uri: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md"},
		{name: "release instead of report", uri: "https://github.com/actions/runner-images/releases/tag/ubuntu24/20261025.353"},
		{name: "query carrying report", uri: runnerSoftwareURL + "?download=1"},
		{name: "wrong report path", uri: "https://github.com/actions/runner-images/blob/ubuntu24/20261025.353/images/ubuntu/Ubuntu2204-Readme.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			descriptor := validRunnerImageDescriptor()
			descriptor.URI = test.uri
			requireRunnerImageRejection(t, descriptor)
		})
	}
}

type runnerNotFoundTransport struct {
	calls int
}

func (transport *runnerNotFoundTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls++
	return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
}

func TestRunnerImageProjectionDoesNotCheckPublicAvailability(t *testing.T) {
	// Given a transport whose report and release responses are always HTTP 404.
	transport := &runnerNotFoundTransport{}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })

	// When validating the signed pair offline, then no request or 404-driven rejection occurs.
	if err := validateRunnerDescriptor(validRunnerImageDescriptor()); err != nil {
		t.Fatalf("validateRunnerDescriptor() error = %v; want nil despite public 404", err)
	}
	if transport.calls != 0 {
		t.Fatalf("runner-image validation made %d network calls; want zero", transport.calls)
	}
}
