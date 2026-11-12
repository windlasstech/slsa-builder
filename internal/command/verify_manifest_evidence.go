package command

import (
	"path/filepath"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func verifyOfflineNPMInputs(statement provenance.Statement, identity attestation.IdentityExpectation, evidencePath, tarballPath string) error {
	evidenceBytes, err := readRegularFileBounded(evidencePath, 24<<20)
	if err != nil {
		return newIdentityPolicyBindingError(npmprofile.IDPackageManifestEvidence, "offline original manifest evidence is unavailable")
	}
	evidence, err := decodeTypedJSON[npmprofile.ManifestEvidence](evidenceBytes, nil)
	if err != nil {
		return newIdentityPolicyBindingError(npmprofile.IDPackageManifestEvidence, "offline original manifest evidence is malformed")
	}
	tarball, err := readRegularFileBounded(tarballPath, 512<<20)
	if err != nil {
		return newIdentityPolicyBindingError("windlass.verify.error.input-unavailable", "npm tarball is unavailable")
	}
	primary := npmprofile.PrimaryArtifact{ArtifactName: "offline-tarball", PayloadFileName: filepath.Base(tarballPath), SHA256: digest.SumSHA256(tarball).String(), SHA512: digest.SumSHA512(tarball).String()}
	packed, err := npmprofile.VerifyPackedArtifact(tarballPath, primary)
	if err != nil {
		return err
	}
	metadata := npmprofile.BuildMetadata{
		SchemaVersion: "1", PrimaryArtifact: primary, ManifestEvidence: evidence,
		ExternalParameters:     statement.Predicate.BuildDefinition.ExternalParameters,
		ResolvedDependencies:   statement.Predicate.BuildDefinition.ResolvedDependencies,
		BuildScriptDisposition: npmprofile.BuildScriptResult(statement.Predicate.RunDetails.Metadata.BuildScriptDisposition),
	}
	return npmprofile.ValidateConsumerStatement(statement, npmprofile.SigstorePublishVerifierConfig{Identity: identity, BuildMetadata: metadata, Packed: packed})
}
