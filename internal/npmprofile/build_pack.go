package npmprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/handoff"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const maxCommandOutput = 1 << 20

const commandTimeout = 10 * time.Minute

const commandWaitDelay = 5 * time.Second

// BuildPack executes the selected real package manager and writes one tarball plus one metadata file.
func BuildPack(ctx context.Context, config BuildPackConfig) (BuildPackResult, error) {
	if ctx == nil {
		return BuildPackResult{}, errors.New("build context is required")
	}
	if err := validateBuildPackConfig(config); err != nil {
		return BuildPackResult{}, err
	}
	if err := validateExecutionRoots(config.Selection); err != nil {
		return BuildPackResult{}, err
	}
	if err := validateCredentialConfiguration(config.Selection); err != nil {
		return BuildPackResult{}, err
	}
	packageOutput, metadataOutput, err := prepareOutput(config.OutputDirectory)
	if err != nil {
		return BuildPackResult{}, err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.RemoveAll(packageOutput)
			_ = os.RemoveAll(metadataOutput)
		}
	}()
	environmentRoot, err := os.MkdirTemp("", "windlass-npm-build-")
	if err != nil {
		return BuildPackResult{}, fmt.Errorf("create isolated package-manager environment: %w", err)
	}
	defer os.RemoveAll(environmentRoot)

	environment, err := controlledEnvironment(environmentRoot)
	if err != nil {
		return BuildPackResult{}, err
	}
	toolchain, managerPath, err := prepareToolchain(ctx, config.Selection, environmentRoot, environment)
	if err != nil {
		return BuildPackResult{}, err
	}
	manifest, _, err := readManifest(filepath.Join(config.Selection.Package.RealDirectory, "package.json"))
	if err != nil {
		return BuildPackResult{}, fmt.Errorf("re-read selected package manifest: %w", err)
	}
	buildScript := BuildScriptCapture{Present: manifest.Scripts != nil && manifest.Scripts["build"] != "", Result: BuildScriptSkippedAbsent}

	if err := runManagerCommand(ctx, config.Selection, managerPath, environment, installArguments(config.Selection.Manager.Name)); err != nil {
		return BuildPackResult{}, fmt.Errorf("install dependencies: %w", err)
	}
	if buildScript.Present {
		if err := runManagerCommand(ctx, config.Selection, managerPath, environment, buildArguments(config.Selection)); err != nil {
			return BuildPackResult{}, fmt.Errorf("run declared build script: %w", err)
		}
		buildScript.Result = BuildScriptExecuted
	}
	if err := runManagerCommand(ctx, config.Selection, managerPath, environment, packArguments(config.Selection, packageOutput)); err != nil {
		return BuildPackResult{}, fmt.Errorf("pack selected package: %w", err)
	}

	tarballPath, err := oneTarball(packageOutput)
	if err != nil {
		return BuildPackResult{}, err
	}
	packed, sha256Value, sha512Value, err := inspectTarball(tarballPath)
	if err != nil {
		return BuildPackResult{}, err
	}
	if packed.Name != config.Selection.Package.Name || packed.Version != config.Selection.Package.Version {
		return BuildPackResult{}, fmt.Errorf("%s: packed identity %s@%s differs from source %s@%s", IDPackedPackageMetadataMismatch, packed.Name, packed.Version, config.Selection.Package.Name, config.Selection.Package.Version)
	}

	metadata := BuildMetadata{
		SchemaVersion: "1",
		PrimaryArtifact: PrimaryArtifact{
			ArtifactName:    config.ArtifactName,
			PayloadFileName: filepath.Base(tarballPath),
			SHA256:          sha256Value.String(),
			SHA512:          sha512Value.String(),
		},
		ExternalParameters:   append(json.RawMessage(nil), config.ExternalParameters...),
		ResolvedDependencies: append([]provenance.ResourceDescriptor(nil), config.ResolvedDependencies...),
	}
	metadataPath := filepath.Join(metadataOutput, "build-metadata.json")
	if err := writeBuildMetadata(metadataPath, metadata); err != nil {
		return BuildPackResult{}, err
	}

	result := BuildPackResult{
		Manager:        config.Selection.Manager.Name,
		PackageName:    config.Selection.Package.Name,
		PackageVersion: config.Selection.Package.Version,
		PackagePURL:    npmPURL(config.Selection.Package.Name, config.Selection.Package.Version),
		TarballPath:    tarballPath,
		MetadataPath:   metadataPath,
		SHA256:         sha256Value,
		SHA512:         sha512Value,
		Packed:         packed,
		BuildScript:    buildScript,
		Toolchain:      toolchain,
	}
	succeeded = true
	return result, nil
}

func validateBuildPackConfig(config BuildPackConfig) error {
	if config.Selection.Report.Result != diagnostic.ResultPass {
		return errors.New("package selection did not pass")
	}
	if config.Selection.Package.RealDirectory == "" || config.Selection.Package.RealManagerRoot == "" {
		return errors.New("package selection lacks resolved real directories")
	}
	if err := handoff.ValidateSafeBasename(config.ArtifactName); err != nil {
		return fmt.Errorf("invalid artifact name: %w", err)
	}
	if len(config.ExternalParameters) == 0 {
		return errors.New("external parameters are required")
	}
	if err := canonicaljson.Validate(config.ExternalParameters); err != nil {
		return fmt.Errorf("validate external parameters: %w", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(config.ExternalParameters, &object); err != nil || object == nil {
		return errors.New("external parameters must be a JSON object")
	}
	if config.Selection.Manager.Name == ManagerYarn {
		if err := handoff.ValidateSafeBasename(npmTarballName(config.Selection.Package.Name, config.Selection.Package.Version)); err != nil {
			return fmt.Errorf("invalid Yarn pack output name: %w", err)
		}
	}
	return nil
}

func validateExecutionRoots(selection Result) error {
	managerRoot, err := filepath.EvalSymlinks(selection.Package.RealManagerRoot)
	if err != nil || managerRoot != selection.Package.RealManagerRoot {
		return errors.New("package manager root changed after package selection")
	}
	packageDirectory, err := filepath.EvalSymlinks(selection.Package.RealDirectory)
	if err != nil || packageDirectory != selection.Package.RealDirectory {
		return errors.New("package directory changed after package selection")
	}
	relative, err := filepath.Rel(managerRoot, packageDirectory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return errors.New("package directory is outside the package manager root")
	}
	managerInfo, err := os.Stat(managerRoot)
	if err != nil || !managerInfo.IsDir() {
		return errors.New("package manager root is not a directory")
	}
	packageInfo, err := os.Stat(packageDirectory)
	if err != nil || !packageInfo.IsDir() {
		return errors.New("package directory is not a directory")
	}
	return nil
}

func validateCredentialConfiguration(selection Result) error {
	paths := []string{
		filepath.Join(selection.Package.RealManagerRoot, ".npmrc"),
		filepath.Join(selection.Package.RealDirectory, ".npmrc"),
		filepath.Join(selection.Package.RealManagerRoot, ".yarnrc.yml"),
		filepath.Join(selection.Package.RealDirectory, ".yarnrc.yml"),
	}
	seen := make(map[string]struct{}, len(paths))
	for _, configPath := range paths {
		if _, ok := seen[configPath]; ok {
			continue
		}
		seen[configPath] = struct{}{}
		info, err := os.Lstat(configPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect package-manager configuration %q: %w", filepath.Base(configPath), err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("package-manager configuration %q must be a regular non-symlink file", filepath.Base(configPath))
		}
		encoded, err := readBoundedRegularFile(configPath, maxManifestSize)
		if err != nil {
			return fmt.Errorf("read package-manager configuration %q: %w", filepath.Base(configPath), err)
		}
		for _, line := range strings.Split(string(encoded), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
				continue
			}
			key, _, _ := strings.Cut(line, "=")
			key = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(key, ":")))
			if strings.Contains(key, "_auth") || strings.Contains(key, "authtoken") ||
				strings.Contains(key, "authident") || strings.HasSuffix(key, "username") ||
				strings.HasSuffix(key, "_password") || strings.HasSuffix(key, "httpskey") ||
				strings.HasSuffix(key, "httpscert") {
				return fmt.Errorf("package-manager configuration %q contains credential material", filepath.Base(configPath))
			}
		}
	}
	corepackEnvironment := filepath.Join(selection.Package.RealManagerRoot, ".corepack.env")
	if _, err := os.Lstat(corepackEnvironment); err == nil {
		return errors.New("project .corepack.env is prohibited for release builds")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect project .corepack.env: %w", err)
	}
	return nil
}

func prepareOutput(output string) (string, string, error) {
	info, err := os.Lstat(output)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("output directory must be an existing non-symlink directory")
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 0 {
		return "", "", errors.New("output directory must be readable and empty")
	}
	realOutput, err := filepath.EvalSymlinks(output)
	if err != nil {
		return "", "", fmt.Errorf("resolve output directory: %w", err)
	}
	packageOutput := filepath.Join(realOutput, "package")
	metadataOutput := filepath.Join(realOutput, "metadata")
	if err := os.Mkdir(packageOutput, 0o700); err != nil {
		return "", "", fmt.Errorf("create package output: %w", err)
	}
	if err := os.Mkdir(metadataOutput, 0o700); err != nil {
		_ = os.Remove(packageOutput)
		return "", "", fmt.Errorf("create metadata output: %w", err)
	}
	return packageOutput, metadataOutput, nil
}

func controlledEnvironment(root string) ([]string, error) {
	pathValue := os.Getenv("PATH")
	for _, directory := range []string{"home", "tmp", "cache", "corepack"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0o700); err != nil {
			return nil, fmt.Errorf("create isolated environment directory: %w", err)
		}
	}
	npmrc := filepath.Join(root, "empty-npmrc")
	globalNPMRC := filepath.Join(root, "empty-global-npmrc")
	for _, configPath := range []string{npmrc, globalNPMRC} {
		file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return nil, fmt.Errorf("create isolated npm configuration: %w", err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close isolated npm configuration: %w", err)
		}
	}
	return []string{
		"PATH=" + pathValue,
		"HOME=" + filepath.Join(root, "home"),
		"TMPDIR=" + filepath.Join(root, "tmp"),
		"XDG_CACHE_HOME=" + filepath.Join(root, "cache"),
		"COREPACK_HOME=" + filepath.Join(root, "corepack"),
		"COREPACK_ENABLE_STRICT=1",
		"COREPACK_ENABLE_PROJECT_SPEC=1",
		"COREPACK_DEFAULT_TO_LATEST=0",
		"COREPACK_ENABLE_DOWNLOAD_PROMPT=0",
		"NPM_CONFIG_USERCONFIG=" + npmrc,
		"npm_config_userconfig=" + npmrc,
		"NPM_CONFIG_GLOBALCONFIG=" + globalNPMRC,
		"npm_config_globalconfig=" + globalNPMRC,
		"npm_config_audit=false",
		"npm_config_fund=false",
		"CI=true",
		"NO_UPDATE_NOTIFIER=1",
	}, nil
}

// allowedExecutableBasenames is the closed set of toolchain binaries runCommand
// may execute. Callers resolve these from fixed names via exec.LookPath or from
// the Corepack shim directory; rejecting any other basename is defense in depth
// against a future caller passing influenced input.
var allowedExecutableBasenames = map[string]bool{
	"node":     true,
	"npm":      true,
	"npx":      true,
	"corepack": true,
	"pnpm":     true,
	"yarn":     true,
}

func runCommand(ctx context.Context, directory, executable string, environment, arguments []string) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("executable path must be absolute: %q", executable)
	}
	if !allowedExecutableBasenames[filepath.Base(executable)] {
		return "", fmt.Errorf("executable is not in the toolchain allowlist: %q", filepath.Base(executable))
	}
	commandContext, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command -- executable is allowlist-validated immediately above; all call sites resolve fixed tool names via exec.LookPath or the enum-constrained Corepack shim directory, so pinned-toolchain resolution cannot stay statically literal.
	command := exec.CommandContext(commandContext, executable, arguments...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = commandWaitDelay
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	command.Dir = directory
	command.Env = environment
	var output limitedBuffer
	command.Stdout = &output
	command.Stderr = &output
	runErr := command.Run()
	if command.Process != nil {
		cleanupErr := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if cleanupErr != nil && !errors.Is(cleanupErr, syscall.ESRCH) && runErr == nil {
			runErr = fmt.Errorf("terminate command descendants: %w", cleanupErr)
		}
	}
	if runErr != nil {
		if errors.Is(commandContext.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("command timed out: %w", commandContext.Err())
		}
		return "", fmt.Errorf("%s %v failed: %w: %s", filepath.Base(executable), arguments, runErr, strings.TrimSpace(output.String()))
	}
	if output.total > maxCommandOutput {
		return "", fmt.Errorf("command output exceeds %d bytes", maxCommandOutput)
	}
	return strings.TrimSpace(output.String()), nil
}

type limitedBuffer struct {
	buffer bytes.Buffer
	total  int
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	buffer.total += len(data)
	if buffer.buffer.Len() < maxCommandOutput {
		remaining := maxCommandOutput - buffer.buffer.Len()
		_, _ = buffer.buffer.Write(data[:min(len(data), remaining)])
	}
	return len(data), nil
}

func (buffer *limitedBuffer) String() string {
	return buffer.buffer.String()
}

func installArguments(manager Manager) []string {
	switch manager {
	case ManagerNPM:
		return []string{"ci"}
	case ManagerPNPM:
		return []string{"install", "--frozen-lockfile"}
	case ManagerYarn:
		return []string{"install", "--immutable"}
	default:
		return nil
	}
}

func buildArguments(selection Result) []string {
	relative := selection.Package.ManagerRootRelativeDirectory
	switch selection.Manager.Name {
	case ManagerNPM:
		if relative == "." {
			return []string{"run", "build"}
		}
		return []string{"--workspace", relative, "run", "build"}
	case ManagerPNPM:
		if relative == "." {
			return []string{"run", "build"}
		}
		return []string{"--filter", "{./" + relative + "}", "run", "build"}
	case ManagerYarn:
		if relative == "." {
			return []string{"run", "build"}
		}
		return []string{"workspace", selection.Package.Name, "run", "build"}
	default:
		return nil
	}
}

func packArguments(selection Result, output string) []string {
	relative := selection.Package.ManagerRootRelativeDirectory
	switch selection.Manager.Name {
	case ManagerNPM:
		if relative == "." {
			return []string{"pack", "--pack-destination", output}
		}
		return []string{"pack", "--workspace", relative, "--pack-destination", output}
	case ManagerPNPM:
		if relative == "." {
			return []string{"pack", "--pack-destination", output}
		}
		return []string{"--filter", "{./" + relative + "}", "pack", "--pack-destination", output}
	case ManagerYarn:
		tarball := filepath.Join(output, npmTarballName(selection.Package.Name, selection.Package.Version))
		if relative == "." {
			return []string{"pack", "--out", tarball}
		}
		return []string{"workspace", selection.Package.Name, "pack", "--out", tarball}
	default:
		return nil
	}
}

func runManagerCommand(ctx context.Context, selection Result, executable string, environment, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("unsupported package manager")
	}
	if err := validateExecutionRoots(selection); err != nil {
		return err
	}
	if err := validateCredentialConfiguration(selection); err != nil {
		return err
	}
	_, err := runCommand(ctx, selection.Package.RealManagerRoot, executable, environment, arguments)
	return err
}

func writeBuildMetadata(path string, metadata BuildMetadata) error {
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode build metadata: %w", err)
	}
	encoded, err = canonicaljson.Canonicalize(encoded)
	if err != nil {
		return fmt.Errorf("canonicalize build metadata: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create build metadata: %w", err)
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return fmt.Errorf("write build metadata: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close build metadata: %w", err)
	}
	return nil
}

func npmTarballName(name, version string) string {
	return strings.ReplaceAll(strings.TrimPrefix(name, "@"), "/", "-") + "-" + version + ".tgz"
}
