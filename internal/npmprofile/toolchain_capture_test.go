package npmprofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureRunnerEnvironment(t *testing.T) {
	// These tests redirect runnerMetadataPath to hermetic fixtures and never
	// read the real fixed host path (/imagegeneration/imagedata.json).
	for _, test := range []struct {
		name         string
		imageOS      string
		imageVersion string
		wantErr      string
	}{
		{name: "no GitHub observations"},
		{name: "only ImageVersion", imageVersion: "20260807.1.0", wantErr: "runner image observations from GitHub are incomplete"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ImageOS", test.imageOS)
			t.Setenv("ImageVersion", test.imageVersion)
			got, err := captureRunner()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("captureRunner() error = %v; want nil", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("captureRunner() error = %v; want %q", err, test.wantErr)
				}
				// ADR 0100: failure diagnostics carry both observed operands.
				if !strings.Contains(err.Error(), `ImageOS="`+test.imageOS+`"`) ||
					!strings.Contains(err.Error(), `ImageVersion="`+test.imageVersion+`"`) {
					t.Fatalf("captureRunner() error = %q; want both operands", err)
				}
				requireNPMDiagnostic(t, err, "windlass.verify.error.input-unavailable")
			}
			if got != (RunnerCapture{}) {
				t.Fatalf("captureRunner() = %#v; want empty capture", got)
			}
		})
	}

	t.Run("ImageVersion absent is tolerated", func(t *testing.T) {
		// ADR 0100: deployment_version may be null when the platform supplies no
		// ImageVersion; only the file metadata and ImageOS are load-bearing. The
		// null case is exercised hermetically through parseRunnerMetadata, never
		// by reading the fixed host metadata path.
		got, err := parseRunnerMetadata(runnerMetadataDocument(t, validRunnerDetail()), "ubuntu24", "")
		if err != nil {
			t.Fatalf("parseRunnerMetadata() error = %v; want nil (absent ImageVersion is tolerated)", err)
		}
		if got.DeploymentVersion != "" {
			t.Fatalf("DeploymentVersion = %q; want empty for an absent ImageVersion", got.DeploymentVersion)
		}
		if got.ImageVersion != "20261025.353.1" {
			t.Fatalf("ImageVersion = %q; want the file-side value %q", got.ImageVersion, "20261025.353.1")
		}
	})

	t.Run("capture succeeds through a redirected metadata path", func(t *testing.T) {
		fixture := filepath.Join(t.TempDir(), "imagedata.json")
		if err := os.WriteFile(fixture, runnerMetadataDocument(t, validRunnerDetail()), 0o600); err != nil {
			t.Fatal(err)
		}
		original := runnerMetadataPath
		t.Cleanup(func() { runnerMetadataPath = original })
		runnerMetadataPath = fixture
		t.Setenv("ImageOS", "ubuntu24")
		t.Setenv("ImageVersion", "")
		got, err := captureRunner()
		if err != nil {
			t.Fatalf("captureRunner() error = %v; want nil", err)
		}
		if got.ImageVersion != "20261025.353.1" || got.DeploymentVersion != "" || got.ImageOS != "ubuntu24" {
			t.Fatalf("captureRunner() = %#v; want the file-side image version with an empty deployment version", got)
		}
	})

	t.Run("metadata read failure carries both operands", func(t *testing.T) {
		original := runnerMetadataPath
		t.Cleanup(func() { runnerMetadataPath = original })
		runnerMetadataPath = filepath.Join(t.TempDir(), "absent.json")
		t.Setenv("ImageOS", "ubuntu24")
		t.Setenv("ImageVersion", "20261026.354.1")
		_, err := captureRunner()
		if err == nil || !strings.Contains(err.Error(), "read GitHub runner image metadata") {
			t.Fatalf("captureRunner() error = %v; want a read failure", err)
		}
		if !strings.Contains(err.Error(), `ImageOS="ubuntu24"`) || !strings.Contains(err.Error(), `ImageVersion="20261026.354.1"`) {
			t.Fatalf("captureRunner() error = %q; want both operands", err)
		}
		requireNPMDiagnostic(t, err, "windlass.verify.error.input-unavailable")
	})
}

const runnerSoftwareURL = "https://github.com/actions/runner-images/blob/ubuntu24/20261025.353/images/ubuntu/Ubuntu2404-Readme.md"
const runnerReleaseURL = "https://github.com/actions/runner-images/releases/tag/ubuntu24%2F20261025.353"

func runnerMetadataDocument(t *testing.T, detail string) []byte {
	t.Helper()
	encoded, err := json.Marshal([]map[string]string{
		{"group": "Operating System", "detail": "Ubuntu\n24.04.5\nLTS"},
		{"group": "Runner Image", "detail": detail},
	})
	if err != nil {
		t.Fatalf("marshal runner metadata document: %v", err)
	}
	return encoded
}

func validRunnerDetail() string {
	return "Image: ubuntu-24.04\n" +
		"Version: 20261025.353.1\n" +
		"Included Software: " + runnerSoftwareURL + "\n" +
		"Image Release: " + runnerReleaseURL
}

func TestParseRunnerMetadata(t *testing.T) {
	t.Parallel()

	t.Run("steady state agreement", func(t *testing.T) {
		t.Parallel()
		got, err := parseRunnerMetadata(runnerMetadataDocument(t, validRunnerDetail()), "ubuntu24", "20261025.353.1")
		if err != nil {
			t.Fatalf("parseRunnerMetadata() error = %v; want nil", err)
		}
		want := RunnerCapture{
			ImageLabel: "ubuntu-24.04", ImageOS: "ubuntu24", ImageVersion: "20261025.353.1",
			DeploymentVersion: "20261025.353.1", IncludedSoftwareURL: runnerSoftwareURL, ImageReleaseURL: runnerReleaseURL,
		}
		if got != want {
			t.Fatalf("parseRunnerMetadata() = %#v; want %#v", got, want)
		}
	})

	t.Run("rollout window divergence accepted", func(t *testing.T) {
		t.Parallel()
		// ADR 0100: the on-disk file is authoritative for the image identity; the
		// control-plane deployment version is separate evidence, and a divergence
		// between them is recorded, not failed.
		got, err := parseRunnerMetadata(runnerMetadataDocument(t, validRunnerDetail()), "ubuntu24", "20261026.354.1")
		if err != nil {
			t.Fatalf("parseRunnerMetadata() error = %v; want nil (divergence is recorded, not failed)", err)
		}
		if got.ImageVersion != "20261025.353.1" {
			t.Fatalf("ImageVersion = %q; want the file-side value %q", got.ImageVersion, "20261025.353.1")
		}
		if got.DeploymentVersion != "20261026.354.1" {
			t.Fatalf("DeploymentVersion = %q; want the control-plane value %q", got.DeploymentVersion, "20261026.354.1")
		}
	})

	for _, test := range []struct {
		name    string
		encoded []byte
		wantErr string
	}{
		{
			name:    "malformed JSON",
			encoded: []byte(`[{"group":"Runner Image","detail":`),
			wantErr: "validate GitHub runner image metadata",
		},
		{
			name:    "duplicate member",
			encoded: []byte(`[{"group":"Runner Image","group":"Runner Image","detail":"Image: ubuntu-24.04"}]`),
			wantErr: "validate GitHub runner image metadata",
		},
		{
			name:    "not an array of groups",
			encoded: []byte(`{"group":"Runner Image"}`),
			wantErr: "decode GitHub runner image metadata",
		},
		{
			name:    "missing Runner Image group",
			encoded: []byte(`[{"group":"Operating System","detail":"Ubuntu"}]`),
			wantErr: "runner image metadata lacks the Runner Image group",
		},
		{
			name: "missing Version line",
			encoded: runnerMetadataDocument(t, "Image: ubuntu-24.04\n"+
				"Included Software: "+runnerSoftwareURL+"\n"+
				"Image Release: "+runnerReleaseURL),
			wantErr: "runner image metadata lacks required detail lines",
		},
		{
			name: "missing Included Software URL",
			encoded: runnerMetadataDocument(t, "Image: ubuntu-24.04\nVersion: 20261025.353.1\n"+
				"Image Release: "+runnerReleaseURL),
			wantErr: "runner image metadata lacks required detail lines",
		},
		{
			name: "empty Version line",
			encoded: runnerMetadataDocument(t, "Image: ubuntu-24.04\nVersion:\n"+
				"Included Software: "+runnerSoftwareURL+"\n"+
				"Image Release: "+runnerReleaseURL),
			wantErr: "runner image metadata lacks required detail lines",
		},
		{
			name: "repeated Image line",
			encoded: runnerMetadataDocument(t, "Image: ubuntu-24.04\nImage: ubuntu-22.04\n"+
				"Version: 20261025.353.1\n"+
				"Included Software: "+runnerSoftwareURL+"\n"+
				"Image Release: "+runnerReleaseURL),
			wantErr: "runner image metadata repeats the Image detail line",
		},
		{
			name: "repeated Version line",
			encoded: runnerMetadataDocument(t, "Image: ubuntu-24.04\n"+
				"Version: 20261025.353.1\nVersion: 20261026.354.1\n"+
				"Included Software: "+runnerSoftwareURL+"\n"+
				"Image Release: "+runnerReleaseURL),
			wantErr: "runner image metadata repeats the Version detail line",
		},
		{
			name: "repeated Included Software line",
			encoded: runnerMetadataDocument(t, "Image: ubuntu-24.04\nVersion: 20261025.353.1\n"+
				"Included Software: "+runnerSoftwareURL+"\n"+
				"Included Software: https://example.com/other\n"+
				"Image Release: "+runnerReleaseURL),
			wantErr: "runner image metadata repeats the Included Software detail line",
		},
		{
			name: "repeated Image Release line",
			encoded: runnerMetadataDocument(t, "Image: ubuntu-24.04\nVersion: 20261025.353.1\n"+
				"Included Software: "+runnerSoftwareURL+"\n"+
				"Image Release: "+runnerReleaseURL+"\n"+
				"Image Release: https://example.com/other"),
			wantErr: "runner image metadata repeats the Image Release detail line",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseRunnerMetadata(test.encoded, "ubuntu24", "20261026.354.1")
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("parseRunnerMetadata() error = %v; want %q", err, test.wantErr)
			}
			// ADR 0100: failure diagnostics carry both observed operands.
			if !strings.Contains(err.Error(), `ImageOS="ubuntu24"`) || !strings.Contains(err.Error(), `ImageVersion="20261026.354.1"`) {
				t.Fatalf("parseRunnerMetadata() error = %q; want both operands", err)
			}
			requireNPMDiagnostic(t, err, "windlass.verify.error.input-unavailable")
		})
	}
}
