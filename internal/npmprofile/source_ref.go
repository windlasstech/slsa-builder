package npmprofile

import "strings"

const tagRefPrefix = "refs/tags/"

// ValidateSourceRefInput validates the tags-only caller-selected build source intent.
func ValidateSourceRefInput(sourceRef, invocationRef, packageVersion string) error {
	if strings.Trim(sourceRef, " \t\n\r\v\f") == "" {
		return nil
	}
	if !strings.HasPrefix(sourceRef, tagRefPrefix) || !validTagName(strings.TrimPrefix(sourceRef, tagRefPrefix)) {
		return sourceRefError("source-ref must be a full, well-formed tag ref")
	}
	if strings.HasPrefix(invocationRef, tagRefPrefix) && sourceRef != invocationRef {
		return sourceRefError("source-ref conflicts with the tag invocation ref")
	}
	if packageVersion != "" && sourceRef != tagRefPrefix+"v"+packageVersion {
		return sourceRefError("source-ref does not match the package version tag")
	}
	return nil
}

func validTagName(name string) bool {
	if name == "" || name == "@" || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") ||
		strings.HasSuffix(name, ".") || strings.Contains(name, "//") || strings.Contains(name, "..") ||
		strings.Contains(name, "@{") || strings.HasSuffix(name, ".lock") {
		return false
	}
	for _, character := range name {
		if character < 0x20 || character == 0x7f || strings.ContainsRune(" ~^:?*[\\", character) {
			return false
		}
	}
	return true
}

func sourceRefError(message string) error {
	return npmValidationError(IDSourceRefInvalid, "source-ref", message)
}
