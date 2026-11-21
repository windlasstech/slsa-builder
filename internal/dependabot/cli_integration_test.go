package dependabot_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/dependabot"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestCompanionCLIIntegration(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	buildRoot := root
	root = dependabot.CloneCommittedTestRoot(t, root)
	work := t.TempDir()
	if evidence := os.Getenv("COMPANION_TEST_EVIDENCE"); evidence != "" {
		work = evidence
		if err := os.MkdirAll(work, 0700); err != nil {
			t.Fatal(err)
		}
	}
	git := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "HEAD")
	git.Env = append(os.Environ(), "GIT_MASTER=1")
	sha, err := runOfflineChild(t, git)
	if err != nil {
		t.Fatal(err)
	}
	baseMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	baseSum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	old, err := toolchainpins.ParseSigningAdapterModule(baseMod, baseSum)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(old.Version, ".")
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	candidate := parts[0] + "." + strconv.Itoa(minor+1) + ".99"
	headMod := bytes.ReplaceAll(baseMod, []byte("github.com/sigstore/sigstore-go "+old.Version), []byte("github.com/sigstore/sigstore-go "+candidate))
	headSum := bytes.ReplaceAll(baseSum, []byte("github.com/sigstore/sigstore-go "+old.Version), []byte("github.com/sigstore/sigstore-go "+candidate))
	headSum = bytes.ReplaceAll(headSum, []byte(old.Sum), []byte("h1:DGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw="))
	for path, data := range map[string][]byte{"base/go.mod": baseMod, "base/go.sum": baseSum, "head/go.mod": headMod, "head/go.sum": headSum} {
		writeTestFile(t, filepath.Join(work, "snapshot", path), data)
	}
	event := dependabot.Event{SchemaVersion: 1, Repository: "windlasstech/slsa-builder", Number: 123, Author: "dependabot[bot]", State: "open", BaseRef: "main", BaseSHA: strings.TrimSpace(string(sha)), TrustedSHA: strings.TrimSpace(string(sha)), HeadRepository: "windlasstech/slsa-builder", HeadSHA: strings.Repeat("b", 40), ChangedPaths: []string{"go.mod", "go.sum"}, LintConclusion: "failure"}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(work, "snapshot/event.json"), raw)
	release := dependabot.Release{Module: "github.com/sigstore/sigstore-go", Version: candidate, ReleaseDate: "2026-11-10", GoVersion: "1.27.0", BaseLicense: "Apache-2.0", License: "Apache-2.0"}
	raw, err = json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(work, "snapshot/release.json"), raw)
	binary := filepath.Join(work, "slsa-builder-internal")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./cmd/slsa-builder-internal")
	build.Dir = buildRoot
	build.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
	if output, err := runOfflineChild(t, build); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	args := []string{"dependabot-companion", "--mode", "dry-run", "--event", filepath.Join(work, "snapshot/event.json"), "--snapshot", filepath.Join(work, "snapshot"), "--trusted-root", root, "--trusted-sha", event.TrustedSHA, "--output", filepath.Join(work, "output")}
	executable, invocation := binary, args
	if sandbox, err := exec.LookPath("sandbox-exec"); err == nil {
		executable = sandbox
		invocation = append([]string{"-p", "(version 1)(allow default)(deny network*)", binary}, args...)
		t.Log("CLI and all subprocesses run with OS-level network denial")
	}
	for i := 0; i < 2; i++ {
		cmd := exec.CommandContext(t.Context(), executable, invocation...)
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
		if output, err := runOfflineChild(t, cmd); err != nil {
			t.Fatalf("dry-run: %v\n%s", err, output)
		} else {
			t.Logf("run %d: %s", i+1, output)
		}
	}
	raw, err = os.ReadFile(filepath.Join(work, "output/plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plan dependabot.Plan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Status != "planned" || len(plan.Files) < 30 {
		t.Fatalf("incomplete plan: %s", raw)
	}
	for _, file := range plan.Files {
		before, err := os.ReadFile(filepath.Join(root, file.Path))
		if err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(filepath.Join(work, "output/files", file.Path))
		if err != nil {
			t.Fatal(err)
		}
		oldHash, newHash := sha256.Sum256(before), sha256.Sum256(after)
		if hex.EncodeToString(oldHash[:]) != file.BeforeSHA256 || hex.EncodeToString(newHash[:]) != file.SHA256 {
			t.Fatalf("hash or trusted-root mutation at %s", file.Path)
		}
		if strings.HasSuffix(file.Path, ".tgz") || strings.HasPrefix(file.Path, "docs/decisions/") || strings.HasPrefix(file.Path, "docs/dogfood/") {
			t.Fatalf("forbidden output: %s", file.Path)
		}
	}
	check := exec.CommandContext(t.Context(), "git", "-C", root, "apply", "--check", filepath.Join(work, "output/patch.diff"))
	check.Env = append(os.Environ(), "GIT_MASTER=1")
	if output, err := runOfflineChild(t, check); err != nil {
		t.Fatalf("patch does not apply: %v\n%s", err, output)
	}
	writeTestFile(t, filepath.Join(work, "malformed.json"), []byte(`{"number":1,"number":2}`))
	badArgs := append([]string(nil), args...)
	badArgs[4] = filepath.Join(work, "malformed.json")
	badArgs[len(badArgs)-1] = filepath.Join(work, "rejected-output")
	badInvocation := badArgs
	if executable != binary {
		badInvocation = append([]string{"-p", "(version 1)(allow default)(deny network*)", binary}, badArgs...)
	}
	bad := exec.CommandContext(t.Context(), executable, badInvocation...)
	if _, err := runOfflineChild(t, bad); err == nil {
		t.Fatal("malformed CLI input succeeded")
	} else if errors.Is(err, errChildNetwork) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(work, "rejected-output")); !os.IsNotExist(err) {
		t.Fatalf("malformed CLI left output: %v", err)
	}
	t.Logf("CLI: %s %s", binary, strings.Join(args, " "))
	t.Logf("files=%d patch_sha256=%s", len(plan.Files), plan.PatchSHA256)
}
