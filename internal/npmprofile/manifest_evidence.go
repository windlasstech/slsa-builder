package npmprofile

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"path"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

const IDPackageManifestEvidence = "windlass.verify.error.package-manifest-evidence"
const IDBuildScriptDisposition = "windlass.verify.error.build-script-disposition-mismatch"

// ManifestEvidence transports original source bytes, not an authenticated manifest by itself.
type ManifestEvidence struct {
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
}

func (evidence *ManifestEvidence) UnmarshalJSON(encoded []byte) error {
	invalid := func() error {
		return npmValidationError(IDPackageManifestEvidence, "manifest_evidence", "manifest evidence requires exactly path and content_base64 strings")
	}
	if canonicaljson.Validate(encoded) != nil {
		return invalid()
	}
	var members map[string]json.RawMessage
	if json.Unmarshal(encoded, &members) != nil || !exactObjectKeys(members, []string{"path", "content_base64"}, nil) {
		return invalid()
	}
	for _, name := range []string{"path", "content_base64"} {
		if bytes.Equal(bytes.TrimSpace(members[name]), []byte("null")) {
			return invalid()
		}
	}
	type wire ManifestEvidence
	var decoded wire
	if json.Unmarshal(encoded, &decoded) != nil {
		return invalid()
	}
	*evidence = ManifestEvidence(decoded)
	return nil
}

// Decode checks the bounded transport and manifest syntax; descriptor authentication is separate.
func (evidence ManifestEvidence) Decode(directory string) (SourceManifestSnapshot, error) {
	invalid := func() (SourceManifestSnapshot, error) {
		return SourceManifestSnapshot{}, npmValidationError(IDPackageManifestEvidence, "manifest_evidence", "manifest evidence has invalid path, encoding, size, or JSON")
	}
	if directory == "" || path.IsAbs(directory) || path.Clean(directory) != directory || strings.ContainsAny(directory, "\\\x00") {
		return invalid()
	}
	for _, segment := range strings.Split(directory, "/") {
		if segment == ".." {
			return invalid()
		}
	}
	if evidence.Path != packageManifestPath(directory) || len(evidence.ContentBase64) > base64.StdEncoding.EncodedLen(maxManifestSize) {
		return invalid()
	}
	content, err := base64.StdEncoding.Strict().DecodeString(evidence.ContentBase64)
	if err != nil || len(content) > maxManifestSize || base64.StdEncoding.EncodeToString(content) != evidence.ContentBase64 {
		return invalid()
	}
	if len(bytes.TrimSpace(content)) == 0 || bytes.TrimSpace(content)[0] != '{' {
		return invalid()
	}
	if _, err := decodeManifest(content); err != nil {
		return invalid()
	}
	return SourceManifestSnapshot{Path: evidence.Path, Content: content}, nil
}

func captureManifestEvidence(snapshot SourceManifestSnapshot, directory string) (ManifestEvidence, error) {
	if len(snapshot.Content) > maxManifestSize {
		return ManifestEvidence{}, npmValidationError(IDPackageManifestEvidence, "manifest_evidence", "source manifest exceeds the decoded bound")
	}
	evidence := ManifestEvidence{Path: snapshot.Path, ContentBase64: base64.StdEncoding.EncodeToString(snapshot.Content)}
	if _, err := evidence.Decode(directory); err != nil {
		return ManifestEvidence{}, err
	}
	return evidence, nil
}

func validateBuildDisposition(evidence ManifestEvidence, directory string, disposition BuildScriptResult) error {
	snapshot, err := evidence.Decode(directory)
	if err != nil {
		return err
	}
	manifest, err := decodeSourceManifestSnapshot(snapshot)
	if err != nil {
		return npmValidationError(IDPackageManifestEvidence, "manifest_evidence", "source manifest is invalid")
	}
	want := BuildScriptSkippedAbsent
	if manifest.Scripts["build"] != "" {
		want = BuildScriptExecuted
	}
	if disposition != want {
		return npmValidationError(IDBuildScriptDisposition, "build_script_disposition", "disposition disagrees with the original source manifest")
	}
	return nil
}
