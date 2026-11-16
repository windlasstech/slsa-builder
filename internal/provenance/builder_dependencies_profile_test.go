package provenance_test

import (
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const admittedDescriptorJSON = `{"name":"publish-cli","uri":"pkg:npm/npm@11.21.1","digest":{"sha512":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef01234567"},"mediaType":"application/gzip","annotations":{"digest_authority":"builder-pinned-record"}}`

func TestBuilderDependencyProfileAdmission(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, raw, id string }{
		{"signer first", "[" + signerDescriptorJSON + "," + admittedDescriptorJSON + "]", ""},
		{"signer last", "[" + admittedDescriptorJSON + "," + signerDescriptorJSON + "]", ""},
		{"cannot replace signer", "[" + admittedDescriptorJSON + "]", signerMismatchID},
		{"cannot duplicate signer", "[" + signerDescriptorJSON + "," + signerDescriptorJSON + "," + admittedDescriptorJSON + "]", signerMismatchID},
		{"unknown remains forbidden", "[" + signerDescriptorJSON + "," + admittedDescriptorJSON + "," + extraDescriptorJSON + "]", descriptorInvalidID},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given an explicit profile admission, without npm-specific pin validation.
			statement, err := decodeDependencyStatement(t, test.raw)
			// When common verification runs with that profile.
			if err == nil {
				_, err = statement.Validate(expectations(nil), builderDependencyProfile{})
			}
			// Then admission changes only the additional descriptor set.
			if test.id != "" {
				requireDiagnosticID(t, err, test.id)
			} else if err != nil {
				t.Fatalf("profile-admitted set rejected: %v", err)
			}
		})
	}
}

type builderDependencyProfile struct{}

func (builderDependencyProfile) ValidateSubject(provenance.Subject) error         { return nil }
func (builderDependencyProfile) ValidateExternalParameters(json.RawMessage) error { return nil }
func (builderDependencyProfile) ValidateResolvedDependencies([]provenance.ResourceDescriptor) error {
	return nil
}
func (builderDependencyProfile) BuilderDependencyNames() []string { return []string{"publish-cli"} }
func (builderDependencyProfile) ValidateBuilderDependencies([]provenance.BuilderDependency) error {
	return nil
}
