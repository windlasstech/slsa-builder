package npmprofile

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

const idToolchainPairMismatch = "windlass.verify.error.toolchain-pair-mismatch"

// ToolchainPairMismatchError identifies a failed pre-install runtime assertion.
type ToolchainPairMismatchError struct{}

func (*ToolchainPairMismatchError) Error() string { return idToolchainPairMismatch }

// DiagnosticID returns the registered pair-mismatch diagnostic.
func (*ToolchainPairMismatchError) DiagnosticID() string { return idToolchainPairMismatch }

// AssertToolchainPair checks the executing Node.js and, unless nodeOnly, bundled npm against the embedded pair.
func AssertToolchainPair(ctx context.Context, nodeOnly bool) error {
	pair, err := toolchainpins.NodeRuntimePair()
	if err != nil {
		return &ToolchainPairMismatchError{}
	}
	root, err := os.MkdirTemp("", "npm-toolchain-assert-")
	if err != nil {
		return &ToolchainPairMismatchError{}
	}
	defer os.RemoveAll(root)
	environment, err := controlledEnvironment(root)
	if err != nil {
		return &ToolchainPairMismatchError{}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return &ToolchainPairMismatchError{}
	}
	observedNode, err := runCommand(ctx, root, node, environment, []string{"--version"})
	if err != nil || strings.TrimPrefix(observedNode, "v") != pair.NodeVersion {
		return &ToolchainPairMismatchError{}
	}
	if nodeOnly {
		return nil
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		return &ToolchainPairMismatchError{}
	}
	observedNPM, err := runCommand(ctx, root, npm, environment, []string{"--version"})
	if err != nil || observedNPM != pair.NPMVersion {
		return &ToolchainPairMismatchError{}
	}
	return nil
}
