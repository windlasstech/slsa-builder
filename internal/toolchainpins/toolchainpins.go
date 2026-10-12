// Package toolchainpins holds the builder-owned npm publish pin and Node.js runtime pair.
package toolchainpins

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

// PublishNPM is the pinned npm CLI distribution for the publish stage.
type PublishNPM struct {
	Version    string `json:"version"`
	TarballURL string `json:"tarball_url"`
	Integrity  string `json:"integrity"`
}

// NodeRuntime is the pinned Node.js patch and its bundled npm version.
type NodeRuntime struct {
	NodeVersion string `json:"node_version"`
	NPMVersion  string `json:"npm_version"`
}

var (
	//go:embed publish-npm.json
	publishData []byte
	//go:embed node-runtime.json
	runtimeData []byte

	versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	loadPublish    = sync.OnceValues(func() (PublishNPM, error) { return decodePublishPin(publishData) })
	loadRuntime    = sync.OnceValues(func() (NodeRuntime, error) { return decodeNodeRuntimePair(runtimeData) })
)

// PublishPin returns the validated, embedded publish-stage npm pin.
func PublishPin() (PublishNPM, error) {
	return loadPublish()
}

// NodeRuntimePair returns the validated, embedded build-stage Node.js/npm pair.
func NodeRuntimePair() (NodeRuntime, error) {
	return loadRuntime()
}

func decodePublishPin(data []byte) (PublishNPM, error) {
	pin, err := decodeClosed[PublishNPM](data)
	if err != nil {
		return PublishNPM{}, fmt.Errorf("decode publish npm pin: %w", err)
	}
	if !versionPattern.MatchString(pin.Version) {
		return PublishNPM{}, errors.New("version must be an exact three-part SemVer version")
	}
	parsed, err := url.Parse(pin.TarballURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return PublishNPM{}, errors.New("tarball_url must be an uncredentialed HTTPS URL")
	}
	encoded, ok := strings.CutPrefix(pin.Integrity, "sha512-")
	if !ok {
		return PublishNPM{}, errors.New("integrity must start with sha512-")
	}
	digest, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(digest) != 64 {
		return PublishNPM{}, errors.New("integrity must contain a base64-encoded 64-byte SHA-512 digest")
	}
	return pin, nil
}

func decodeNodeRuntimePair(data []byte) (NodeRuntime, error) {
	pair, err := decodeClosed[NodeRuntime](data)
	if err != nil {
		return NodeRuntime{}, fmt.Errorf("decode Node.js runtime pair: %w", err)
	}
	if !versionPattern.MatchString(pair.NodeVersion) {
		return NodeRuntime{}, errors.New("node_version must be an exact three-part SemVer version")
	}
	if !versionPattern.MatchString(pair.NPMVersion) {
		return NodeRuntime{}, errors.New("npm_version must be an exact three-part SemVer version")
	}
	return pair, nil
}

func decodeClosed[T any](data []byte) (T, error) {
	var value T
	if err := canonicaljson.Validate(data); err != nil {
		return value, fmt.Errorf("validate JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("decode closed JSON schema: %w", err)
	}
	return value, nil
}
