package toolchainpins

import (
	"encoding/base64"
	"encoding/hex"
	"strings"

	"golang.org/x/mod/modfile"
)

const signingAdapterModulePath = "github.com/sigstore/sigstore-go"

// SigningAdapterModule is a candidate module-content checksum, not runtime policy.
type SigningAdapterModule struct {
	Version  string
	Sum      string
	DirHash1 string
}

// ParseSigningAdapterModule parses explicit generation/drift inputs. It does not
// compare the candidate against the compiled production signing descriptor.
func ParseSigningAdapterModule(goMod, goSum []byte) (SigningAdapterModule, error) {
	manifest, err := modfile.Parse("go.mod", goMod, nil)
	if err != nil {
		return SigningAdapterModule{}, &DriftError{message: "invalid signing adapter module manifest"}
	}
	for _, replacement := range manifest.Replace {
		if replacement.Old.Path == signingAdapterModulePath {
			return SigningAdapterModule{}, &DriftError{message: "signing adapter module replacement is forbidden"}
		}
	}
	var version string
	for _, requirement := range manifest.Require {
		if requirement.Mod.Path != signingAdapterModulePath {
			continue
		}
		if version != "" {
			return SigningAdapterModule{}, &DriftError{message: "duplicate signing adapter requirement"}
		}
		version = requirement.Mod.Version
	}
	if version == "" {
		return SigningAdapterModule{}, &DriftError{message: "missing signing adapter requirement"}
	}
	var sum string
	for _, line := range strings.Split(string(goSum), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != signingAdapterModulePath {
			continue
		}
		if len(fields) != 3 {
			return SigningAdapterModule{}, &DriftError{message: "malformed signing adapter checksum record"}
		}
		if strings.HasSuffix(fields[1], "/go.mod") {
			continue
		}
		if fields[1] != version || sum != "" {
			return SigningAdapterModule{}, &DriftError{message: "signing adapter content checksum must uniquely match the required version"}
		}
		sum = fields[2]
	}
	encoded, hasAlgorithm := strings.CutPrefix(sum, "h1:")
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if !hasAlgorithm || err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != encoded {
		return SigningAdapterModule{}, &DriftError{message: "signing adapter content checksum must be a canonical 32-byte h1 digest"}
	}
	return SigningAdapterModule{Version: version, Sum: sum, DirHash1: hex.EncodeToString(decoded)}, nil
}
