package npmprofile

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestProvisionNodeLive(t *testing.T) {
	t.Parallel()
	if testing.Short() || os.Getenv("SLSA_BUILDER_LIVE_TOOLCHAIN") != "1" {
		t.Skip("live toolchain test: set SLSA_BUILDER_LIVE_TOOLCHAIN=1 and omit -short")
	}
	// Given the embedded official runtime pin, with no local artifact cache.
	pin, err := toolchainpins.NodeRuntimePair()
	if err != nil {
		t.Fatal(err)
	}
	// When the production download, digest, extraction, layout and pair checks run.
	bin, err := ProvisionNode(t.Context(), t.TempDir())
	// Then the real artifact provisions, and its programs identify the recorded pair.
	if err != nil {
		t.Fatal(err)
	}
	t.Log("official linux-x64 artifact passed the full execution-free provisioning pipeline")
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("runtime execution requires linux/amd64; download and provisioning passed")
	}
	for _, check := range []struct{ name, version string }{{"node", "v" + pin.NodeVersion}, {"npm", pin.NPMVersion}} {
		t.Run(check.name, func(t *testing.T) {
			t.Parallel()
			command := exec.CommandContext(t.Context(), filepath.Join(bin, check.name), "--version")
			command.Dir = t.TempDir()
			command.Env = []string{"PATH=" + bin, "HOME=" + command.Dir}
			output, err := command.CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != check.version {
				t.Fatalf("%s --version = %q, %v; want %s", check.name, output, err, check.version)
			}
		})
	}
}
