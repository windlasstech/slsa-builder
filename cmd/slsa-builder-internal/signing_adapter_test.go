package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestSelectedSigstoreModuleMatchesCompiledExpectation(t *testing.T) {
	t.Parallel()
	// Given the dependency graph actually linked into this command package.
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("debug.ReadBuildInfo() unavailable")
	}
	// When its selected signer is compared with the production descriptor constructor.
	expected := provenance.SigningAdapterBuilderDependency()
	for _, dependency := range info.Deps {
		if dependency.Path != "github.com/sigstore/sigstore-go" {
			continue
		}
		// Then replacements, version drift, and content checksum drift fail closed.
		if dependency.Replace != nil {
			t.Fatalf("selected sigstore module has replacement: %#v", dependency.Replace)
		}
		if dependency.Version != "v1.4.0" || dependency.Sum != "h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw=" {
			t.Fatalf("selected sigstore module = %s %s, want v1.4.0 h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw=", dependency.Version, dependency.Sum)
		}
		digest, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(dependency.Sum, "h1:"))
		if err != nil {
			t.Fatal(err)
		}
		if expected.URI != "pkg:golang/"+dependency.Path+"@"+dependency.Version || expected.Digest["dirHash1"] != hex.EncodeToString(digest) {
			t.Fatalf("compiled signer %#v differs from selected module %#v", expected, dependency)
		}
		encoded, err := json.Marshal(expected)
		if err != nil {
			t.Fatal(err)
		}
		var members map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &members); err != nil {
			t.Fatal(err)
		}
		if len(members) != 3 || string(members["name"]) != `"signing-adapter"` || len(expected.Digest) != 1 {
			t.Fatalf("compiled signer shape = %s, want exactly name, uri, digest.dirHash1", encoded)
		}
		return
	}
	t.Fatal("selected sigstore-go dependency missing from command build information")
}
