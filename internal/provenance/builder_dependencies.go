package provenance

import (
	"net/url"
	"regexp"
)

const (
	signingAdapterURI          = "pkg:golang/github.com/sigstore/sigstore-go@v1.4.0"
	signingAdapterDigest       = "0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac"
	idBuilderDescriptorInvalid = "windlass.verify.error.builder-dependencies-descriptor-invalid"
	builderDependenciesField   = "predicate.runDetails.builder.builderDependencies"
)

var builderDependencyNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var builderDigestHexPattern = regexp.MustCompile(`^[0-9a-f]+$`)

// SigningAdapterBuilderDependency returns a fresh descriptor for the compiled signer.
// Module graph parsing is generation/drift tooling, never a runtime dependency.
func SigningAdapterBuilderDependency() BuilderDependency {
	return BuilderDependency{
		Name:   "signing-adapter",
		URI:    signingAdapterURI,
		Digest: map[string]string{"dirHash1": signingAdapterDigest},
	}
}

// These optional hooks leave existing ProfileValidator implementations compatible.
type builderDependencyNames interface {
	BuilderDependencyNames() []string
}

type builderDependencyValidator interface {
	ValidateBuilderDependencies([]BuilderDependency) error
}

func validateBuilderDependencies(dependencies []BuilderDependency, profile ProfileValidator) error {
	if validator, ok := profile.(builderDependencyValidator); ok {
		if err := validator.ValidateBuilderDependencies(cloneBuilderDependencies(dependencies)); err != nil {
			return err
		}
	}
	signers := 0
	for _, dependency := range dependencies {
		if dependency.Name != "signing-adapter" {
			continue
		}
		signers++
		if dependency.URI != signingAdapterURI || len(dependency.Digest) != 1 ||
			dependency.Digest["dirHash1"] != signingAdapterDigest ||
			dependency.MediaType != "" || dependency.HasMalformedMembers() || dependency.HasUnexpectedFields() {
			return validationError(idBuilderDependenciesMismatch, builderDependenciesField, "signing adapter must match the compiled descriptor exactly")
		}
	}
	if signers != 1 {
		return validationError(idBuilderDependenciesMismatch, builderDependenciesField, "exactly one signing-adapter descriptor is required")
	}
	admitted := map[string]bool{"signing-adapter": true}
	if names, ok := profile.(builderDependencyNames); ok {
		for _, name := range names.BuilderDependencyNames() {
			admitted[name] = true
		}
	}
	seen := make(map[string]bool, len(dependencies))
	for _, dependency := range dependencies {
		if !admitted[dependency.Name] || seen[dependency.Name] || !validBuilderDependencyShape(dependency) {
			return validationError(idBuilderDescriptorInvalid, builderDependenciesField, "builder dependency must have a unique admitted name and valid declared members")
		}
		seen[dependency.Name] = true
	}
	return nil
}

func validBuilderDependencyShape(dependency BuilderDependency) bool {
	if dependency.HasMalformedMembers() || dependency.HasUnexpectedFields() ||
		!builderDependencyNamePattern.MatchString(dependency.Name) || len(dependency.Digest) == 0 {
		return false
	}
	uri, err := url.Parse(dependency.URI)
	if err != nil || !uri.IsAbs() {
		return false
	}
	for algorithm, value := range dependency.Digest {
		if algorithm == "" || value == "" {
			return false
		}
		var size int
		switch algorithm {
		case "sha1":
			size = 40
		case "sha256":
			size = 64
		case "sha384":
			size = 96
		case "sha512":
			size = 128
		}
		if size != 0 && len(value) != size {
			return false
		}
		switch algorithm {
		case "sha256", "sha384", "sha512", "sha1", "gitCommit", "dirHash1":
			if !builderDigestHexPattern.MatchString(value) || len(value)%2 != 0 {
				return false
			}
		}
	}
	return true
}
