package dependabot_test

import (
	"os"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/dependabot"
)

func TestSigstoreModuleTransition(t *testing.T) {
	t.Parallel()
	base := fixtureModules(t, "base")
	head := fixtureModules(t, "head")
	for _, test := range []struct {
		name   string
		change func(*dependabot.Modules)
		reason string
	}{
		{"grouped", func(*dependabot.Modules) {}, ""},
		{"unchanged", func(m *dependabot.Modules) { *m = base }, "signer-unchanged"},
		{"downgrade", func(m *dependabot.Modules) { m.Mod = []byte(strings.ReplaceAll(string(m.Mod), "v1.5.0", "v1.2.0")) }, "version-not-increased"},
		{"replacement", func(m *dependabot.Modules) {
			m.Mod = append(m.Mod, []byte("replace example.org/a => example.org/b v1.0.0\n")...)
		}, "unsupported-module-directive"},
		{"transitive", func(m *dependabot.Modules) {
			m.Mod = []byte(strings.ReplaceAll(string(m.Mod), "example.org/other v1.0.0", "example.org/other v1.1.0"))
		}, "unsupported-transitive-change"},
		{"missing sum", func(m *dependabot.Modules) { m.Sum = nil }, "invalid-module-checksums"},
		{"conflicting sum", func(m *dependabot.Modules) { m.Sum = append(m.Sum, m.Sum...) }, "invalid-module-checksums"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given an isolated candidate module pair.
			candidate := dependabot.Modules{Mod: append([]byte(nil), head.Mod...), Sum: append([]byte(nil), head.Sum...)}
			test.change(&candidate)
			// When evaluating without execution.
			_, reason := dependabot.ParseSigstoreModuleTransition(base, candidate)
			// Then only the supported transition is admitted.
			if reason != test.reason {
				t.Fatalf("reason = %q, want %q", reason, test.reason)
			}
		})
	}
}

func fixtureModules(t testing.TB, name string) dependabot.Modules {
	t.Helper()
	mod, err := os.ReadFile("testdata/" + name + "/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile("testdata/" + name + "/go.sum")
	if err != nil {
		t.Fatal(err)
	}
	return dependabot.Modules{Mod: mod, Sum: sum}
}

func TestModuleOnlyTransitionWithPreexistingChecksums(t *testing.T) {
	t.Parallel()
	base, head := fixtureModules(t, "base"), fixtureModules(t, "head")
	base.Sum = append(base.Sum, []byte("github.com/sigstore/sigstore-go v1.5.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\ngithub.com/in-toto/attestation v1.1.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n")...)
	head.Sum = base.Sum
	_, reason := dependabot.ParseSigstoreModuleTransition(base, head)
	if reason != "" {
		t.Fatalf("go.mod-only transition rejected: %s", reason)
	}
}

func FuzzSigstoreModuleTransition(f *testing.F) {
	base, head := fixtureModules(f, "base"), fixtureModules(f, "head")
	f.Add(head.Mod, head.Sum)
	f.Add([]byte("module x\n"), []byte("bad"))
	f.Fuzz(func(t *testing.T, mod, sum []byte) {
		t.Helper()
		dependabot.ParseSigstoreModuleTransition(base, dependabot.Modules{Mod: mod, Sum: sum})
	})
}
