package npmprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

const maxPackumentResponse int64 = 8 << 20

// RegistryClientConfig identifies one npm-compatible metadata endpoint.
type RegistryClientConfig struct {
	HTTPClient  *http.Client
	RegistryURL string
}

// RegistryClient reads package metadata without credentials or redirects.
type RegistryClient struct {
	httpClient  *http.Client
	registryURL *url.URL
}

// RegistryPreflightState records authoritative package and version existence.
type RegistryPreflightState struct {
	PackageExists bool
	VersionExists bool
	Version       *RegistryVersion
}

// RegistryVersion is the preflight subset needed by P04 convergence.
type RegistryVersion struct {
	Name      string
	Version   string
	Integrity string
	Tarball   string
}

// NewRegistryClient validates the root registry URL and installs hardened HTTP behavior.
func NewRegistryClient(config RegistryClientConfig) (*RegistryClient, error) {
	registryURL, err := normalizeRegistryURL(config.RegistryURL)
	if err != nil {
		return nil, err
	}
	client, err := hardenedHTTPClient(config.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &RegistryClient{httpClient: client, registryURL: registryURL}, nil
}

// Preflight reads the full packument and determines package and exact-version existence.
func (client *RegistryClient) Preflight(ctx context.Context, packageName, version string) (RegistryPreflightState, error) {
	if !validRegistryPackageName(packageName) || invalidPURLText(version) || strings.Contains(version, "/") {
		return RegistryPreflightState{}, errors.New("package name or version is invalid")
	}
	endpoint := *client.registryURL
	endpoint.Path = "/" + packageName
	endpoint.RawPath = "/" + percentEncode(packageName)
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("User-Agent", "windlass-slsa-builder")
	response, err := performBoundedRequest(ctx, client.httpClient, http.MethodGet, &endpoint, headers, maxPackumentResponse)
	if err != nil {
		return RegistryPreflightState{}, errors.New("registry packument request failed")
	}
	if response.status == http.StatusNotFound {
		return RegistryPreflightState{}, nil
	}
	if response.status != http.StatusOK {
		return RegistryPreflightState{}, fmt.Errorf("registry packument returned HTTP %d", response.status)
	}
	if err := canonicaljson.Validate(response.body); err != nil {
		return RegistryPreflightState{}, errors.New("registry packument is malformed")
	}
	var packument struct {
		Name     string                     `json:"name"`
		Versions map[string]json.RawMessage `json:"versions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response.body))
	if err := decoder.Decode(&packument); err != nil || packument.Name != packageName || packument.Versions == nil {
		return RegistryPreflightState{}, errors.New("registry packument identity is malformed")
	}
	state := RegistryPreflightState{PackageExists: true}
	rawVersion, exists := packument.Versions[version]
	if !exists {
		return state, nil
	}
	var metadata struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Dist    struct {
			Integrity string `json:"integrity"`
			Tarball   string `json:"tarball"`
		} `json:"dist"`
	}
	if err := json.Unmarshal(rawVersion, &metadata); err != nil || metadata.Name != packageName || metadata.Version != version {
		return RegistryPreflightState{}, errors.New("registry version metadata identity is malformed")
	}
	state.VersionExists = true
	state.Version = &RegistryVersion{
		Name:      metadata.Name,
		Version:   metadata.Version,
		Integrity: metadata.Dist.Integrity,
		Tarball:   metadata.Dist.Tarball,
	}
	return state, nil
}

func normalizeRegistryURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, errors.New("registry URL must be an absolute credential-free HTTPS root URL")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Port() == "443" {
		parsed.Host = parsed.Hostname()
	}
	parsed.Path = "/"
	parsed.RawPath = ""
	return parsed, nil
}

func validRegistryPackageName(name string) bool {
	if invalidPURLText(name) || len([]byte(name)) > 214 || strings.ContainsAny(name, "?#%") {
		return false
	}
	if strings.HasPrefix(name, "@") {
		remainder := strings.TrimPrefix(name, "@")
		scope, packageName, found := strings.Cut(remainder, "/")
		return found && validRegistryPackageNamePart(scope) && validRegistryPackageNamePart(packageName) &&
			!strings.Contains(packageName, "/")
	}
	return validRegistryPackageNamePart(name) && !strings.Contains(name, "/")
}

func validRegistryPackageNamePart(part string) bool {
	if part == "" || strings.Contains(part, "@") {
		return false
	}
	for _, character := range part {
		if unicode.IsSpace(character) {
			return false
		}
	}
	return true
}
