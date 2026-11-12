package fixture

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"path"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const IDPackageManifestEvidence = "windlass.verify.error.package-manifest-evidence"
const maxManifestEvidenceBytes = 16 << 20
const npmProducerBuildType = "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1"

type evidenceError struct{ reason string }

func (err *evidenceError) Error() string    { return IDPackageManifestEvidence + ": " + err.reason }
func (*evidenceError) DiagnosticID() string { return IDPackageManifestEvidence }
func evidenceFailure(reason string) error   { return &evidenceError{reason: reason} }

// ValidateManifestEvidence checks local byte bindings, not signature authenticity; crypto fixtures own that check.
func ValidateManifestEvidence(manifest Manifest, document, evidence []byte) error {
	statement, applicable, err := evidenceStatement(document)
	if err != nil {
		return err
	}
	return manifest.validateEvidence(statement, applicable, evidence)
}

func (manifest Manifest) validateEvidence(statement provenance.Statement, applicable bool, evidence []byte) error {
	if manifest.Surface == "release-manifest" || !applicable {
		if manifest.ManifestEvidence != nil || len(evidence) != 0 {
			return evidenceFailure("non-npm-Statement fixtures require null manifest-evidence")
		}
		return nil
	}
	if manifest.ManifestEvidence == nil || *manifest.ManifestEvidence == "" {
		return evidenceFailure("npm Statement requires an evidence path")
	}
	if canonicaljson.Validate(evidence) != nil {
		return evidenceFailure("manifest evidence must be duplicate-free JSON")
	}
	var members map[string]json.RawMessage
	if json.Unmarshal(evidence, &members) != nil || len(members) != 2 {
		return evidenceFailure("manifest evidence must contain exactly path and content_base64")
	}
	var evidencePath, encoded string
	if json.Unmarshal(members["path"], &evidencePath) != nil || json.Unmarshal(members["content_base64"], &encoded) != nil || encoded == "" {
		return evidenceFailure("manifest evidence members must be non-empty strings")
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(maxManifestEvidenceBytes) {
		return evidenceFailure("decoded manifest exceeds 16 MiB")
	}
	content, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(content) > maxManifestEvidenceBytes || base64.StdEncoding.EncodeToString(content) != encoded {
		return evidenceFailure("manifest base64 must be canonical padded RFC 4648")
	}
	if canonicaljson.Validate(content) != nil || len(bytes.TrimSpace(content)) == 0 || bytes.TrimSpace(content)[0] != '{' {
		return evidenceFailure("original manifest must be a bounded duplicate-free JSON object")
	}
	var external struct {
		Workflow struct {
			Repository string `json:"repository"`
		} `json:"workflow"`
		Package struct {
			Directory string `json:"directory"`
		} `json:"package"`
	}
	if json.Unmarshal(statement.Predicate.BuildDefinition.ExternalParameters, &external) != nil || identity.ValidateCanonicalRepositoryURI(external.Workflow.Repository) != nil || !safeEvidencePath(external.Package.Directory) {
		return evidenceFailure("manifest source repository or package directory is invalid")
	}
	wantPath := path.Join(external.Package.Directory, "package.json")
	if evidencePath != wantPath || !safeEvidencePath(evidencePath) {
		return evidenceFailure("manifest evidence path differs from the selected package")
	}
	var selected, built provenance.ResourceDescriptor
	seen := make(map[string]bool)
	for _, descriptor := range statement.Predicate.BuildDefinition.ResolvedDependencies {
		if seen[descriptor.Name] {
			return evidenceFailure("duplicate dependency names prevent evidence binding")
		}
		seen[descriptor.Name] = true
	}
	for _, descriptor := range statement.Predicate.BuildDefinition.ResolvedDependencies {
		switch descriptor.Name {
		case "package-manifest":
			selected = descriptor
		case "built-source":
			built = descriptor
		}
	}
	commit := built.Digest["gitCommit"]
	if identity.ValidateFullSHA(commit) != nil || len(built.Digest) != 1 {
		return evidenceFailure("built-source commit is unavailable")
	}
	wantURI := "git+" + external.Workflow.Repository + "@" + commit + "#" + encodeEvidencePath(evidencePath)
	if selected.HasUnexpectedFields() || selected.MediaType != "application/json" || selected.URI != wantURI || len(selected.Digest) != 1 || selected.Digest["sha256"] != digest.SumSHA256(content).String() {
		return evidenceFailure("manifest bytes, path, repository, or built commit disagree with the descriptor")
	}
	return nil
}

func evidenceStatement(document []byte) (provenance.Statement, bool, error) {
	if canonicaljson.Validate(document) != nil {
		return provenance.Statement{}, false, evidenceFailure("provenance document is not duplicate-free JSON")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(document, &object) != nil || object == nil {
		return provenance.Statement{}, false, evidenceFailure("provenance document must be an object")
	}
	if _, bundle := object["dsseEnvelope"]; bundle {
		parsed, err := attestation.ParseBundle(document)
		if err != nil {
			return provenance.Statement{}, false, evidenceFailure("provenance bundle cannot supply a Statement")
		}
		document = parsed.StatementBytes()
	} else {
		var kind string
		if json.Unmarshal(object["_type"], &kind) != nil || kind != provenance.StatementType {
			return provenance.Statement{}, false, nil
		}
	}
	statement, err := provenance.DecodeStatement(document)
	if err != nil {
		return provenance.Statement{}, false, evidenceFailure("provenance Statement cannot be decoded")
	}
	if statement.Type != provenance.StatementType || statement.PredicateType != provenance.PredicateType || statement.Predicate.BuildDefinition.BuildType != npmProducerBuildType {
		return statement, false, nil
	}
	for _, descriptor := range statement.Predicate.BuildDefinition.ResolvedDependencies {
		if descriptor.Name == "package-manifest" {
			return statement, true, nil
		}
	}
	return statement, false, nil
}

func safeEvidencePath(value string) bool {
	if value == "" || path.IsAbs(value) || path.Clean(value) != value || strings.ContainsAny(value, "\\\x00") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

func encodeEvidencePath(value string) string {
	const hex = "0123456789ABCDEF"
	var encoded strings.Builder
	for _, b := range []byte(value) {
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.ContainsRune("-._~/", rune(b)) {
			encoded.WriteByte(b)
		} else {
			encoded.WriteByte('%')
			encoded.WriteByte(hex[b>>4])
			encoded.WriteByte(hex[b&15])
		}
	}
	return encoded.String()
}
