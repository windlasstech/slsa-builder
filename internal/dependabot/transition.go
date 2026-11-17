// Package dependabot plans local, data-only dependency companion updates.
package dependabot

import (
	"bytes"
	"encoding/base64"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

const signerModule = "github.com/sigstore/sigstore-go"
const maxInput = 16 << 20

// Modules carries files as data, never as an executable dependency graph.
type Modules struct{ Mod, Sum []byte }

// Transition is constructed only by the module boundary parser.
type Transition struct {
	base, head     Modules
	old, candidate toolchainpins.SigningAdapterModule
}

// ParseSigstoreModuleTransition admits only existing grouped requirement bumps.
// A nonempty stable reason requires manual review; no files are written.
func ParseSigstoreModuleTransition(base, head Modules) (Transition, string) {
	if len(base.Mod)+len(base.Sum)+len(head.Mod)+len(head.Sum) > maxInput {
		return Transition{}, "module-input-too-large"
	}
	b, err := modfile.Parse("go.mod", base.Mod, nil)
	if err != nil {
		return Transition{}, "invalid-module-manifest"
	}
	h, err := modfile.Parse("go.mod", head.Mod, nil)
	if err != nil {
		return Transition{}, "invalid-module-manifest"
	}
	if len(b.Replace) != 0 || len(h.Replace) != 0 {
		return Transition{}, "unsupported-module-directive"
	}
	if len(b.Require) != len(h.Require) {
		return Transition{}, "unsupported-transitive-change"
	}
	changed := map[string][2]string{}
	seen := map[string]bool{}
	for i, old := range b.Require {
		candidate := h.Require[i]
		if seen[old.Mod.Path] || candidate.Mod.Path != old.Mod.Path || candidate.Indirect != old.Indirect {
			return Transition{}, "unsupported-transitive-change"
		}
		seen[old.Mod.Path] = true
		if candidate.Mod.Version == old.Mod.Version {
			continue
		}
		if !inGroup(old.Mod.Path) {
			return Transition{}, "unsupported-transitive-change"
		}
		if !semver.IsValid(candidate.Mod.Version) || !semver.IsValid(old.Mod.Version) || semver.Compare(candidate.Mod.Version, old.Mod.Version) <= 0 {
			return Transition{}, "version-not-increased"
		}
		changed[old.Mod.Path] = [2]string{old.Mod.Version, candidate.Mod.Version}
		// Restore only the requirement's version token before comparing all other syntax.
		candidate.Syntax.Token[len(candidate.Syntax.Token)-1] = old.Mod.Version
	}
	if !bytes.Equal(modfile.Format(b.Syntax), modfile.Format(h.Syntax)) {
		return Transition{}, "unsupported-module-directive"
	}
	if _, ok := changed[signerModule]; !ok {
		return Transition{}, "signer-unchanged"
	}
	bs, ok := checksumRecords(base.Sum)
	if !ok {
		return Transition{}, "invalid-module-checksums"
	}
	hs, ok := checksumRecords(head.Sum)
	if !ok {
		return Transition{}, "invalid-module-checksums"
	}
	for module, versions := range changed {
		if bs[module+" "+versions[0]] == "" || hs[module+" "+versions[1]] == "" {
			return Transition{}, "invalid-module-checksums"
		}
	}
	for key, value := range bs {
		if hs[key] != value && !changedChecksum(key, changed) {
			return Transition{}, "unsupported-transitive-change"
		}
	}
	for key, value := range hs {
		if bs[key] != value && !changedChecksum(key, changed) {
			return Transition{}, "unsupported-transitive-change"
		}
	}
	versions := changed[signerModule]
	oldKey, candidateKey := signerModule+" "+versions[0], signerModule+" "+versions[1]
	old, err := toolchainpins.ParseSigningAdapterModule(base.Mod, []byte(oldKey+" "+bs[oldKey]+"\n"))
	if err != nil {
		return Transition{}, "invalid-module-checksums"
	}
	candidate, err := toolchainpins.ParseSigningAdapterModule(head.Mod, []byte(candidateKey+" "+hs[candidateKey]+"\n"))
	if err != nil {
		return Transition{}, "invalid-module-checksums"
	}
	return Transition{base: base, head: head, old: old, candidate: candidate}, ""
}

func inGroup(module string) bool {
	return strings.HasPrefix(module, "github.com/sigstore/") || strings.HasPrefix(module, "github.com/in-toto/")
}

func checksumRecords(data []byte) (map[string]string, bool) {
	records := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 || !semver.IsValid(strings.TrimSuffix(fields[1], "/go.mod")) {
			return nil, false
		}
		encoded, ok := strings.CutPrefix(fields[2], "h1:")
		decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
		key := fields[0] + " " + fields[1]
		if !ok || err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != encoded || records[key] != "" {
			return nil, false
		}
		records[key] = fields[2]
	}
	return records, true
}

func changedChecksum(key string, changed map[string][2]string) bool {
	module, version, _ := strings.Cut(key, " ")
	versions, ok := changed[module]
	version = strings.TrimSuffix(version, "/go.mod")
	return ok && (version == versions[0] || version == versions[1])
}
