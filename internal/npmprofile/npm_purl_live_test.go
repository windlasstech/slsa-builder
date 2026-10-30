package npmprofile

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestNPMPURLCLICompatibilityLive(t *testing.T) {
	requireLiveToolchain(t)
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	// Resolve the actual Node binary even when PATH contains a mise shim.
	nodeOutput, err := exec.CommandContext(ctx, "node", "-p", "process.execPath").Output()
	if err != nil {
		t.Fatalf("resolve live node: %v", err)
	}
	node := strings.TrimSpace(string(nodeOutput))
	pair, err := toolchainpins.NodeRuntimePair()
	if err != nil {
		t.Fatal(err)
	}
	versionOutput, err := exec.CommandContext(ctx, node, "--version").Output()
	if err != nil || strings.TrimSpace(string(versionOutput)) != "v"+pair.NodeVersion {
		t.Fatalf("node version = %q, %v; want v%s", versionOutput, err, pair.NodeVersion)
	}
	if pair.NodeVersion != "24.22.1" || pair.NPMVersion != "11.19.1" {
		t.Fatalf("build-stage pair = %#v; update live compatibility fixtures for the new pin", pair)
	}
	publishPin, err := toolchainpins.PublishPin()
	if err != nil {
		t.Fatal(err)
	}
	if publishPin.Version != "11.21.1" {
		t.Fatalf("publish npm pin = %q; update live compatibility fixtures for the new pin", publishPin.Version)
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := ProvisionNPM(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("provision pinned npm %s: %v", publishPin.Version, err)
	}
	buildRoot := filepath.Join(filepath.Dir(filepath.Dir(node)), "lib", "node_modules", "npm")
	publishRoot := filepath.Join(filepath.Dir(launcher), "package")
	for _, cli := range []struct {
		name     string
		root     string
		version  string
		fixtures [2]string
	}{
		{"build", buildRoot, pair.NPMVersion, [2]string{"npm-purl-build-11-19-1-scoped-valid", "npm-purl-build-11-19-1-unscoped-valid"}},
		{"publish", publishRoot, publishPin.Version, [2]string{"npm-purl-publish-11-21-1-scoped-valid", "npm-purl-publish-11-21-1-unscoped-valid"}},
	} {
		t.Run(cli.name+"/npm-"+cli.version, func(t *testing.T) {
			cliVersion, err := exec.CommandContext(ctx, node, filepath.Join(cli.root, "bin", "npm-cli.js"), "--version").Output()
			if err != nil || strings.TrimSpace(string(cliVersion)) != cli.version {
				t.Fatalf("npm version = %q, %v; want %s", cliVersion, err, cli.version)
			}
			npaPath := filepath.Join(cli.root, "node_modules", "npm-package-arg")
			for _, fixtureName := range cli.fixtures {
				t.Run(fixtureName, func(t *testing.T) {
					// Given the registered compatibility fixture's package and recorded subject.
					root := filepath.Join(repositoryRoot, "testdata", "npm", "packages", fixtureName)
					packageBytes, err := os.ReadFile(filepath.Join(root, "package.json"))
					if err != nil {
						t.Fatal(err)
					}
					var pkg struct {
						Name    string `json:"name"`
						Version string `json:"version"`
					}
					if err := json.Unmarshal(packageBytes, &pkg); err != nil {
						t.Fatal(err)
					}
					fixtureBytes, err := os.ReadFile(filepath.Join(root, "fixture.json"))
					if err != nil {
						t.Fatal(err)
					}
					var fixture struct {
						VerifiedNPMVersion string `json:"verified-npm-cli-version"`
						ExpectedSubject    string `json:"expected-subject"`
					}
					if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
						t.Fatal(err)
					}
					if fixture.VerifiedNPMVersion != cli.version {
						t.Fatalf("fixture npm version = %q, want %q", fixture.VerifiedNPMVersion, cli.version)
					}
					// When libnpmpublish's provenance subject expression runs against its own bundled npa.
					const subjectExpression = "const npa = require(process.argv[1]); process.stdout.write(npa.toPurl(npa.resolve(process.argv[2], process.argv[3])))"
					cmd := exec.CommandContext(ctx, node, "-e", subjectExpression, npaPath, pkg.Name, pkg.Version)
					cmd.Dir = t.TempDir() // Never run npm code from this repository's devEngines context.
					cliSubject, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("npm %s provenance subject derivation: %v: %s", cli.version, err, cliSubject)
					}
					goSubject, err := NPMPackagePURL(pkg.Name, pkg.Version)
					if err != nil {
						t.Fatal(err)
					}
					// Then the real CLI, Go derivation, and fixture agree byte-for-byte.
					if string(cliSubject) != fixture.ExpectedSubject || goSubject != fixture.ExpectedSubject {
						t.Fatalf("npm %s subject = %q; Go subject = %q; fixture subject = %q", cli.version, cliSubject, goSubject, fixture.ExpectedSubject)
					}
					t.Logf("npm %s provenance subject: %s", cli.version, cliSubject)
				})
			}
		})
	}
}
