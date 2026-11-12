package npmprofile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func operatingSystemGroup(detail any) string {
	return string(mustJSON(map[string]any{"group": "Operating System", "detail": detail}))
}

func runnerDocumentWithOperatingSystemGroups(t *testing.T, groups ...string) []byte {
	t.Helper()
	encoded := []byte("[")
	for _, group := range groups {
		encoded = append(encoded, group...)
		encoded = append(encoded, ',')
	}
	encoded = append(encoded, `{"group":"Runner Image","detail":`...)
	encoded = append(encoded, mustJSON(validRunnerDetail())...)
	return append(encoded, '}', ']')
}

func runnerCaptureStringField(t *testing.T, capture RunnerCapture, name string) (string, bool) {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(mustJSON(capture), &fields); err != nil {
		t.Fatalf("decode runner capture: %v", err)
	}
	raw, present := fields[name]
	if !present {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("runner capture field %q is not a string: %v", name, err)
	}
	return value, true
}

func requireRunnerOperatingSystem(t *testing.T, capture RunnerCapture, want string) {
	t.Helper()
	got, present := runnerCaptureStringField(t, capture, "operating_system")
	if !present || got != want {
		t.Fatalf("operating_system = %q, present %t; want exact decoded value %q", got, present, want)
	}
	if _, present := runnerCaptureStringField(t, capture, "image_os"); present {
		t.Fatal("runner capture retains signed image_os evidence")
	}
}

func TestCaptureRunnerOperatingSystem(t *testing.T) {
	t.Run("exact multiline detail ignores ImageOS", func(t *testing.T) {
		t.Parallel()
		// Given valid local OS evidence and a deliberately unrelated environment label.
		encoded := runnerMetadataDocument(t, validRunnerDetail())

		// When parsing the single bounded snapshot.
		got, err := parseRunnerMetadata(encoded, "not-an-os-authority", "20261026.354.1")

		// Then only the exact decoded local detail is retained as signed OS evidence.
		if err != nil {
			t.Fatalf("parseRunnerMetadata() error = %v; want nil", err)
		}
		requireRunnerOperatingSystem(t, got, "Ubuntu\n24.04.5\nLTS")
	})

	t.Run("meaningful whitespace and line endings are preserved", func(t *testing.T) {
		t.Parallel()
		const detail = " \tUbuntu\r\n24.04.5\nLTS "

		got, err := parseRunnerMetadata(runnerDocumentWithOperatingSystemGroups(t, operatingSystemGroup(detail)), "ubuntu24", "20261026.354.1")

		if err != nil {
			t.Fatalf("parseRunnerMetadata() error = %v; want nil", err)
		}
		requireRunnerOperatingSystem(t, got, detail)
	})

	for _, test := range []struct {
		name        string
		replacement []byte
	}{
		{name: "escaped newline", replacement: []byte(`Ubuntu\n24.04.5\nLTS`)},
		{name: "unicode escaped newline", replacement: []byte("Ubuntu\\u000a24.04.5\\u000aLTS")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			encoded := bytes.Replace(
				runnerMetadataDocument(t, validRunnerDetail()),
				[]byte(`Ubuntu\n24.04.5\nLTS`),
				test.replacement,
				1,
			)

			got, err := parseRunnerMetadata(encoded, "ubuntu24", "20261026.354.1")

			if err != nil {
				t.Fatalf("parseRunnerMetadata() error = %v; want nil", err)
			}
			requireRunnerOperatingSystem(t, got, "Ubuntu\n24.04.5\nLTS")
		})
	}

	for _, test := range []struct {
		name    string
		encoded []byte
	}{
		{name: "missing Operating System group", encoded: runnerDocumentWithOperatingSystemGroups(t)},
		{name: "duplicate Operating System group", encoded: runnerDocumentWithOperatingSystemGroups(t,
			operatingSystemGroup("Ubuntu"), operatingSystemGroup("Ubuntu"))},
		{name: "malformed Operating System group", encoded: runnerDocumentWithOperatingSystemGroups(t,
			`{"group":42,"detail":"Ubuntu"}`)},
		{name: "missing Operating System detail", encoded: runnerDocumentWithOperatingSystemGroups(t,
			`{"group":"Operating System"}`)},
		{name: "duplicate Operating System detail", encoded: runnerDocumentWithOperatingSystemGroups(t,
			`{"group":"Operating System","detail":"Ubuntu","detail":"Ubuntu"}`)},
		{name: "non-string Operating System detail", encoded: runnerDocumentWithOperatingSystemGroups(t,
			operatingSystemGroup(42))},
		{name: "empty Operating System detail", encoded: runnerDocumentWithOperatingSystemGroups(t,
			operatingSystemGroup(""))},
		{name: "whitespace-only Operating System detail", encoded: runnerDocumentWithOperatingSystemGroups(t,
			operatingSystemGroup(" \t\r\n\u00a0"))},
		{name: "malformed full image version", encoded: bytes.Replace(
			runnerDocumentWithOperatingSystemGroups(t, operatingSystemGroup("Ubuntu\n24.04.5\nLTS")),
			[]byte("20261025.353.1"), []byte("20261025.353"), 1)},
		{name: "malformed Included Software URI", encoded: bytes.Replace(
			runnerDocumentWithOperatingSystemGroups(t, operatingSystemGroup("Ubuntu\n24.04.5\nLTS")),
			[]byte(runnerSoftwareURL), []byte("://bad"), 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given malformed local evidence alongside a valid environment ImageOS.
			// When parsing the snapshot, then capture fails closed with input-unavailable.
			got, err := parseRunnerMetadata(test.encoded, "ubuntu24", "20261026.354.1")
			requireNPMDiagnostic(t, err, "windlass.verify.error.input-unavailable")
			if got != (RunnerCapture{}) {
				t.Fatalf("parseRunnerMetadata() = %#v; want empty capture on rejection", got)
			}
		})
	}
}

func redirectRunnerMetadata(t *testing.T) {
	t.Helper()
	fixture := filepath.Join(t.TempDir(), "imagedata.json")
	if err := os.WriteFile(fixture, runnerMetadataDocument(t, validRunnerDetail()), 0o600); err != nil {
		t.Fatal(err)
	}
	original := runnerMetadataPath
	t.Cleanup(func() { runnerMetadataPath = original })
	runnerMetadataPath = fixture
}

func TestCaptureRunnerOperatingSystemWithoutImageOS(t *testing.T) {
	// Given a complete local snapshot and no ImageOS environment observation.
	fixture := filepath.Join(t.TempDir(), "imagedata.json")
	if err := os.WriteFile(fixture, runnerMetadataDocument(t, validRunnerDetail()), 0o600); err != nil {
		t.Fatal(err)
	}
	original := runnerMetadataPath
	t.Cleanup(func() { runnerMetadataPath = original })
	runnerMetadataPath = fixture
	t.Setenv("ImageOS", "")
	t.Setenv("ImageVersion", "20261026.354.1")

	// When capturing the runner, then the absent environment label is not required.
	got, err := captureRunner()
	if err != nil {
		t.Fatalf("captureRunner() error = %v; want nil without ImageOS", err)
	}
	requireRunnerOperatingSystem(t, got, "Ubuntu\n24.04.5\nLTS")
}
