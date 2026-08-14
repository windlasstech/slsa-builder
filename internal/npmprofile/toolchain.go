package npmprofile

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"golang.org/x/mod/semver"
)

const (
	maxCorepackMetadata  = 64 << 10
	maxRegistryMetadata  = 1 << 20
	maxYarnDistribution  = 128 << 20
	distributionTimeout  = 2 * time.Minute
	corepackHashPrefix   = "sha512."
	corepackDebugMarker  = " corepack Installing "
	runnerMetadataPath   = "/imagegeneration/imagedata.json"
	maxRunnerMetadata    = 1 << 20
	acquisitionSource    = "corepack"
	registryDigestSource = "registry-integrity"
	downloadDigestSource = "download-hash"
)

func prepareToolchain(ctx context.Context, selection Result, root string, environment []string, fetcher distributionFetcher) (ToolchainCapture, string, error) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return ToolchainCapture{}, "", errors.New("node executable is unavailable")
	}
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		return ToolchainCapture{}, "", errors.New("npm executable is unavailable")
	}
	nodeVersion, err := runCommand(ctx, selection.Package.RealManagerRoot, nodePath, environment, []string{"--version"})
	if err != nil {
		return ToolchainCapture{}, "", err
	}
	npmVersion, err := runCommand(ctx, selection.Package.RealManagerRoot, npmPath, environment, []string{"--version"})
	if err != nil {
		return ToolchainCapture{}, "", err
	}
	if !semver.IsValid(nodeVersion) || semver.Major(nodeVersion) != "v24" {
		return ToolchainCapture{}, "", fmt.Errorf("required Node.js major is 24; executed %q", nodeVersion)
	}
	if !semver.IsValid("v"+npmVersion) || semver.Compare("v"+npmVersion, "v11.5.1") < 0 {
		return ToolchainCapture{}, "", fmt.Errorf("npm 11.5.1 or newer is required, executed %q", npmVersion)
	}
	runner, err := captureRunner()
	if err != nil {
		return ToolchainCapture{}, "", err
	}
	capture := ToolchainCapture{NodeVersion: nodeVersion, NPMVersion: npmVersion, Runner: runner}
	if selection.Manager.Name == ManagerNPM {
		capture.PackageManagerVersion = npmVersion
		return capture, npmPath, nil
	}

	corepackPath, err := exec.LookPath("corepack")
	if err != nil {
		return ToolchainCapture{}, "", errors.New("corepack executable is unavailable")
	}
	corepackVersion, err := runCommand(ctx, selection.Package.RealManagerRoot, corepackPath, environment, []string{"--version"})
	if err != nil {
		return ToolchainCapture{}, "", err
	}
	shimDirectory := filepath.Join(root, "shims")
	if err := os.Mkdir(shimDirectory, 0o700); err != nil {
		return ToolchainCapture{}, "", fmt.Errorf("create Corepack shim directory: %w", err)
	}
	if _, err := runCommand(ctx, selection.Package.RealManagerRoot, corepackPath, environment, []string{"enable", "--install-directory", shimDirectory}); err != nil {
		return ToolchainCapture{}, "", fmt.Errorf("enable Corepack: %w", err)
	}
	managerPath := filepath.Join(shimDirectory, string(selection.Manager.Name))
	acquisitionEnvironment := append(append([]string(nil), environment...), "DEBUG=corepack")
	managerOutput, err := runCommandWithShimRoot(ctx, selection.Package.RealManagerRoot, managerPath, acquisitionEnvironment, []string{"--version"}, shimDirectory)
	if err != nil {
		return ToolchainCapture{}, "", err
	}
	managerVersion := lastOutputLine(managerOutput)
	if managerVersion != selection.Manager.Version {
		return ToolchainCapture{}, "", fmt.Errorf("selected %s@%s but Corepack executed %s", selection.Manager.Name, selection.Manager.Version, managerVersion)
	}
	distribution, err := captureDistribution(ctx, filepath.Join(root, "corepack"), selection.Manager.Name, managerVersion, managerOutput, fetcher)
	if err != nil {
		return ToolchainCapture{}, "", err
	}
	capture.CorepackVersion = corepackVersion
	capture.PackageManagerVersion = managerVersion
	capture.Distribution = distribution
	return capture, managerPath, nil
}

func lastOutputLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func captureDistribution(ctx context.Context, corepackHome string, manager Manager, version, commandOutput string, fetcher distributionFetcher) (*DistributionCapture, error) {
	distributionURL, err := corepackDistributionURL(commandOutput, manager, version)
	if err != nil {
		return nil, err
	}
	wantURL := expectedDistributionURL(manager, version)
	if distributionURL != wantURL {
		return nil, fmt.Errorf("corepack acquired %s@%s from unexpected URL %q", manager, version, distributionURL)
	}
	metadataPath := filepath.Join(corepackHome, "v1", string(manager), version, ".corepack")
	encoded, err := readBoundedRegularFile(metadataPath, maxCorepackMetadata)
	if err != nil {
		return nil, fmt.Errorf("read Corepack distribution metadata: %w", err)
	}
	if err := canonicaljson.Validate(encoded); err != nil {
		return nil, fmt.Errorf("validate Corepack distribution metadata: %w", err)
	}
	var metadata struct {
		Locator struct {
			Name      string `json:"name"`
			Reference string `json:"reference"`
		} `json:"locator"`
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(encoded, &metadata); err != nil || metadata.Locator.Name != string(manager) || metadata.Locator.Reference != version {
		return nil, errors.New("corepack distribution metadata disagrees with the executed manager")
	}
	corepackHash := strings.TrimPrefix(metadata.Hash, corepackHashPrefix)
	decodedHash, err := hex.DecodeString(corepackHash)
	if err != nil || len(decodedHash) != 64 || metadata.Hash != corepackHashPrefix+corepackHash || strings.ToLower(corepackHash) != corepackHash {
		return nil, errors.New("corepack distribution metadata lacks a canonical SHA-512 hash")
	}

	authority := registryDigestSource
	authoritativeHash := ""
	switch manager {
	case ManagerPNPM:
		authoritativeHash, err = pnpmRegistryIntegrity(ctx, version, distributionURL, fetcher)
	case ManagerYarn:
		authority = downloadDigestSource
		authoritativeHash, err = hashYarnDistribution(ctx, distributionURL, fetcher)
	default:
		err = errors.New("unsupported Corepack package manager")
	}
	if err != nil {
		return nil, err
	}
	if authoritativeHash != corepackHash {
		return nil, errors.New("corepack distribution hash disagrees with acquisition evidence")
	}
	return &DistributionCapture{
		URL:               distributionURL,
		SHA512:            corepackHash,
		DigestAuthority:   authority,
		PackageManager:    manager,
		PackageManagerVer: version,
		AcquisitionSource: acquisitionSource,
	}, nil
}

func corepackDistributionURL(output string, manager Manager, version string) (string, error) {
	prefix := corepackDebugMarker + string(manager) + "@" + version + " from "
	for _, line := range strings.Split(output, "\n") {
		index := strings.Index(line, prefix)
		if index < 0 {
			continue
		}
		value := strings.TrimSpace(line[index+len(prefix):])
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", errors.New("corepack reported an invalid distribution URL")
		}
		return value, nil
	}
	return "", errors.New("corepack did not report the acquired distribution URL")
}

func expectedDistributionURL(manager Manager, version string) string {
	if manager == ManagerYarn {
		return "https://repo.yarnpkg.com/" + version + "/packages/yarnpkg-cli/bin/yarn.js"
	}
	return "https://registry.npmjs.org/pnpm/-/pnpm-" + version + ".tgz"
}

func pnpmRegistryIntegrity(ctx context.Context, version, distributionURL string, fetcher distributionFetcher) (string, error) {
	metadataURL := "https://registry.npmjs.org/pnpm/" + url.PathEscape(version)
	encoded, err := fetcher(ctx, metadataURL, maxRegistryMetadata, "application/json")
	if err != nil {
		return "", fmt.Errorf("fetch pnpm registry metadata: %w", err)
	}
	if err := canonicaljson.Validate(encoded); err != nil {
		return "", fmt.Errorf("validate pnpm registry metadata: %w", err)
	}
	var packument struct {
		Dist struct {
			Integrity string `json:"integrity"`
			Tarball   string `json:"tarball"`
		} `json:"dist"`
	}
	if err := json.Unmarshal(encoded, &packument); err != nil || packument.Dist.Tarball != distributionURL {
		return "", errors.New("pnpm registry metadata disagrees with the Corepack distribution URL")
	}
	encodedDigest, found := strings.CutPrefix(packument.Dist.Integrity, "sha512-")
	if !found || strings.ContainsAny(encodedDigest, " \t\r\n") {
		return "", errors.New("pnpm registry metadata lacks one SHA-512 integrity value")
	}
	digestBytes, err := base64.StdEncoding.DecodeString(encodedDigest)
	if err != nil || len(digestBytes) != 64 {
		return "", errors.New("pnpm registry SHA-512 integrity value is malformed")
	}
	return hex.EncodeToString(digestBytes), nil
}

func hashYarnDistribution(ctx context.Context, distributionURL string, fetcher distributionFetcher) (string, error) {
	encoded, err := fetcher(ctx, distributionURL, maxYarnDistribution, "application/octet-stream")
	if err != nil {
		return "", fmt.Errorf("download Yarn distribution evidence: %w", err)
	}
	return digest.SumSHA512(encoded).String(), nil
}

//nolint:unused // parameter names document the fetcher contract
type distributionFetcher func(ctx context.Context, rawURL string, maximum int64, accept string) ([]byte, error)

func fetchHTTPS(ctx context.Context, rawURL string, maximum int64, accept string) ([]byte, error) {
	requestContext, cancel := context.WithTimeout(ctx, distributionTimeout)
	defer cancel()
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("default HTTP transport has an unsupported type")
	}
	transport := baseTransport.Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("distribution endpoint redirect is prohibited")
		},
	}
	defer client.CloseIdleConnections()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(requestContext, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", accept)
		request.Header.Set("User-Agent", "windlass-slsa-builder")
		if transport.Proxy != nil {
			proxyURL, proxyErr := transport.Proxy(request)
			if proxyErr != nil {
				return nil, fmt.Errorf("resolve HTTPS proxy: %w", proxyErr)
			}
			if proxyURL != nil && proxyURL.User != nil {
				return nil, errors.New("credential-bearing HTTPS proxies are prohibited")
			}
		}
		response, requestErr := client.Do(request)
		if requestErr == nil {
			encoded, readErr := io.ReadAll(io.LimitReader(response.Body, maximum+1))
			closeErr := response.Body.Close()
			if response.StatusCode == http.StatusOK && readErr == nil && closeErr == nil {
				if int64(len(encoded)) > maximum {
					return nil, errors.New("distribution response exceeds size limit")
				}
				return encoded, nil
			}
			if response.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("distribution endpoint returned HTTP %d", response.StatusCode)
			} else {
				lastErr = errors.Join(readErr, closeErr)
			}
		} else {
			lastErr = requestErr
		}
		if attempt == 2 {
			break
		}
		delay := time.NewTimer(time.Duration(attempt+1) * time.Second)
		select {
		case <-requestContext.Done():
			delay.Stop()
			return nil, errors.Join(lastErr, requestContext.Err())
		case <-delay.C:
		}
	}
	return nil, lastErr
}

func captureRunner() (RunnerCapture, error) {
	capture := RunnerCapture{ImageOS: os.Getenv("ImageOS"), ImageVersion: os.Getenv("ImageVersion")}
	if capture.ImageOS == "" && capture.ImageVersion == "" {
		return capture, nil
	}
	if capture.ImageOS == "" || capture.ImageVersion == "" {
		return RunnerCapture{}, errors.New("runner image observations from GitHub are incomplete")
	}
	encoded, err := readBoundedRegularFile(runnerMetadataPath, maxRunnerMetadata)
	if err != nil {
		return RunnerCapture{}, fmt.Errorf("read GitHub runner image metadata: %w", err)
	}
	if err := canonicaljson.Validate(encoded); err != nil {
		return RunnerCapture{}, fmt.Errorf("validate GitHub runner image metadata: %w", err)
	}
	var groups []struct {
		Group  string `json:"group"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(encoded, &groups); err != nil {
		return RunnerCapture{}, fmt.Errorf("decode GitHub runner image metadata: %w", err)
	}
	for _, group := range groups {
		if group.Group != "Runner Image" {
			continue
		}
		for _, line := range strings.Split(group.Detail, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "Image: "):
				capture.ImageLabel = strings.TrimSpace(strings.TrimPrefix(line, "Image: "))
			case strings.HasPrefix(line, "Version: "):
				if strings.TrimSpace(strings.TrimPrefix(line, "Version: ")) != capture.ImageVersion {
					return RunnerCapture{}, errors.New("runner image metadata version disagrees with ImageVersion")
				}
			case strings.HasPrefix(line, "Included Software: "):
				capture.IncludedSoftwareURL = strings.TrimSpace(strings.TrimPrefix(line, "Included Software: "))
			case strings.HasPrefix(line, "Image Release: "):
				capture.ImageReleaseURL = strings.TrimSpace(strings.TrimPrefix(line, "Image Release: "))
			}
		}
		if capture.ImageLabel == "" || capture.IncludedSoftwareURL == "" || capture.ImageReleaseURL == "" {
			return RunnerCapture{}, errors.New("runner image metadata lacks required detail lines")
		}
		return capture, nil
	}
	return RunnerCapture{}, errors.New("runner image metadata lacks the Runner Image group")
}
