package npmprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

const (
	maxPackumentResponse   int64 = 8 << 20
	maxAttestationResponse int64 = 16 << 20
)

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

// RegistryAttestation is one exact bundle selected by its declared predicate type.
type RegistryAttestation struct {
	PredicateType string
	Bundle        []byte
}

// RegistryAttestationState records one exact-version attestation observation.
type RegistryAttestationState struct {
	Found        bool
	Attestations []RegistryAttestation
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
	return decodeRegistryPackumentResponse(response.body, packageName, version)
}

// URL returns the normalized credential-free registry root used by this client.
func (client *RegistryClient) URL() string {
	if client == nil || client.registryURL == nil {
		return ""
	}
	return client.registryURL.String()
}

// Attestations reads npm's exact-version public attestation surface without credentials.
func (client *RegistryClient) Attestations(ctx context.Context, packageName, version string) (RegistryAttestationState, error) {
	if client == nil || client.registryURL == nil || !validRegistryPackageName(packageName) || invalidPURLText(version) || strings.Contains(version, "/") {
		return RegistryAttestationState{}, errors.New("package name or version is invalid")
	}
	endpoint := *client.registryURL
	identifier := packageName + "@" + version
	endpoint.Path = "/-/npm/v1/attestations/" + identifier
	endpoint.RawPath = "/-/npm/v1/attestations/" + percentEncode(packageName) + "@" + percentEncode(version)
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("User-Agent", "windlass-slsa-builder")
	response, err := performBoundedRequest(ctx, client.httpClient, http.MethodGet, &endpoint, headers, maxAttestationResponse)
	if err != nil {
		return RegistryAttestationState{}, errors.New("registry attestation request failed")
	}
	if response.status == http.StatusNotFound {
		return RegistryAttestationState{}, nil
	}
	if response.status != http.StatusOK {
		return RegistryAttestationState{}, fmt.Errorf("registry attestation endpoint returned HTTP %d", response.status)
	}
	return decodeRegistryAttestationResponse(response.body)
}

func decodeRegistryPackumentResponse(body []byte, packageName, version string) (RegistryPreflightState, error) {
	if err := canonicaljson.Validate(body); err != nil {
		return RegistryPreflightState{}, errors.New("registry packument is malformed")
	}
	var packument struct {
		Name     string                     `json:"name"`
		Versions map[string]json.RawMessage `json:"versions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
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

func decodeRegistryAttestationResponse(body []byte) (RegistryAttestationState, error) {
	if err := canonicaljson.Validate(body); err != nil {
		return RegistryAttestationState{}, errors.New("registry attestation response is malformed")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var document struct {
		Attestations []struct {
			PredicateType string          `json:"predicateType"`
			Bundle        json.RawMessage `json:"bundle"`
		} `json:"attestations"`
	}
	if err := decoder.Decode(&document); err != nil || document.Attestations == nil {
		return RegistryAttestationState{}, errors.New("registry attestation response has an invalid shape")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return RegistryAttestationState{}, errors.New("registry attestation response contains trailing data")
	}
	attestations := make([]RegistryAttestation, 0, len(document.Attestations))
	for _, candidate := range document.Attestations {
		if candidate.PredicateType == "" || len(candidate.Bundle) == 0 || string(candidate.Bundle) == "null" {
			return RegistryAttestationState{}, errors.New("registry attestation entry is malformed")
		}
		attestations = append(attestations, RegistryAttestation{
			PredicateType: candidate.PredicateType,
			Bundle:        append([]byte(nil), candidate.Bundle...),
		})
	}
	return RegistryAttestationState{Found: true, Attestations: attestations}, nil
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
