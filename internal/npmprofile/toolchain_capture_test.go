package npmprofile

import "testing"

func TestCaptureRunnerEnvironment(t *testing.T) {
	// Metadata decoding is seam-blocked by /imagegeneration/imagedata.json.
	// These tests never read or write that fixed host path.
	for _, test := range []struct {
		name         string
		imageOS      string
		imageVersion string
		wantErr      string
	}{
		{name: "no GitHub observations"},
		{name: "only ImageOS", imageOS: "ubuntu24", wantErr: "runner image observations from GitHub are incomplete"},
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
			} else if err == nil || err.Error() != test.wantErr {
				t.Fatalf("captureRunner() error = %v; want %q", err, test.wantErr)
			}
			if got != (RunnerCapture{}) {
				t.Fatalf("captureRunner() = %#v; want empty capture", got)
			}
		})
	}
}
