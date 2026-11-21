package dependabot_test

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/dependabot"
)

func TestTransitionPreservesExistingChecksumIdentity(t *testing.T) {
	t.Parallel()
	for _, module := range []string{"github.com/sigstore/sigstore-go", "github.com/in-toto/attestation", "example.org/other"} {
		for _, suffix := range []string{"", "/go.mod"} {
			for _, version := range []string{"v1.0.0", "v1.1.0", "v1.4.0", "v1.5.0"} {
				t.Run(module+"/"+version+suffix, func(t *testing.T) {
					t.Parallel()
					// Given a record present on both sides of an otherwise valid bump.
					base, head := fixtureModules(t, "base"), fixtureModules(t, "head")
					key := module + " " + version + suffix + " "
					oldHash := "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
					newHash := "h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw="
					for _, m := range []*dependabot.Modules{&base, &head} {
						lines := strings.Split(strings.TrimSpace(string(m.Sum)), "\n")
						found := false
						for i, line := range lines {
							if strings.HasPrefix(line, key) {
								lines[i] = key + oldHash
								found = true
							}
						}
						if !found {
							lines = append(lines, key+oldHash)
						}
						m.Sum = []byte(strings.Join(lines, "\n") + "\n")
					}
					head.Sum = bytes.ReplaceAll(head.Sum, []byte(key+oldHash), []byte(key+newHash))
					// When only that record's hash changes, then fail closed.
					_, reason := dependabot.ParseSigstoreModuleTransition(base, head)
					if reason != "unsupported-transitive-change" {
						t.Fatalf("changed %s: reason=%q, want unsupported-transitive-change", key, reason)
					}
				})
			}
		}
	}
}

func TestTransitionRejectsWrongDirectionChecksumChanges(t *testing.T) {
	t.Parallel()
	for _, removeCandidate := range []bool{false, true} {
		t.Run(strconv.FormatBool(removeCandidate), func(t *testing.T) {
			t.Parallel()
			// Given an old-version addition or a preexisting candidate-version removal.
			base, head := fixtureModules(t, "base"), fixtureModules(t, "head")
			record := "github.com/sigstore/sigstore-go v1.4.0/go.mod h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n"
			if removeCandidate {
				base.Sum = append(base.Sum, []byte(strings.ReplaceAll(record, "v1.4.0", "v1.5.0"))...)
			} else {
				head.Sum = append(head.Sum, []byte(record)...)
			}
			// When parsing, then only old removal and new addition are permitted.
			_, reason := dependabot.ParseSigstoreModuleTransition(base, head)
			if reason != "unsupported-transitive-change" {
				t.Fatalf("reason=%q, want unsupported-transitive-change", reason)
			}
		})
	}
}

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
		{"unchanged signer hash mutation", func(m *dependabot.Modules) {
			*m = base
			m.Sum = bytes.ReplaceAll(base.Sum, []byte("h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw="), []byte("h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="))
		}, "unsupported-transitive-change"},
		{"downgrade", func(m *dependabot.Modules) { m.Mod = []byte(strings.ReplaceAll(string(m.Mod), "v1.5.0", "v1.2.0")) }, "version-not-increased"},
		{"replacement", func(m *dependabot.Modules) {
			m.Mod = append(m.Mod, []byte("replace example.org/a => example.org/b v1.0.0\n")...)
		}, "unsupported-module-directive"},
		{"transitive", func(m *dependabot.Modules) {
			m.Mod = []byte(strings.ReplaceAll(string(m.Mod), "example.org/other v1.0.0", "example.org/other v1.1.0"))
		}, "unsupported-transitive-change"},
		{"missing sum", func(m *dependabot.Modules) { m.Sum = nil }, "invalid-module-checksums"},
		{"conflicting sum", func(m *dependabot.Modules) { m.Sum = append(m.Sum, m.Sum...) }, "invalid-module-checksums"},
		{"malformed manifest", func(m *dependabot.Modules) { m.Mod = []byte("require (\n") }, "invalid-module-manifest"},
		{"oversized manifest", func(m *dependabot.Modules) { m.Mod = make([]byte, (16<<20)+1) }, "module-input-too-large"},
		{"go directive", func(m *dependabot.Modules) { m.Mod = bytes.ReplaceAll(m.Mod, []byte("go 1.27.3"), []byte("go 1.28.0")) }, "unsupported-module-directive"},
		{"toolchain directive", func(m *dependabot.Modules) { m.Mod = append(m.Mod, []byte("toolchain go1.28.0\n")...) }, "unsupported-module-directive"},
		{"exclude directive", func(m *dependabot.Modules) { m.Mod = append(m.Mod, []byte("exclude example.org/other v1.0.0\n")...) }, "unsupported-module-directive"},
		{"missing signer", func(m *dependabot.Modules) {
			m.Mod = bytes.ReplaceAll(m.Mod, []byte("\tgithub.com/sigstore/sigstore-go v1.5.0\n"), nil)
		}, "unsupported-transitive-change"},
		{"indirect changed", func(m *dependabot.Modules) { m.Mod = bytes.ReplaceAll(m.Mod, []byte(" // indirect"), nil) }, "unsupported-transitive-change"},
		{"same signer version", func(m *dependabot.Modules) {
			m.Mod = bytes.ReplaceAll(m.Mod, []byte("v1.5.0"), []byte("v1.4.0"))
			m.Sum = bytes.ReplaceAll(m.Sum, []byte("github.com/sigstore/sigstore-go v1.5.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="), []byte(strings.Split(string(base.Sum), "\n")[0]))
		}, "signer-unchanged"},
		{"invalid checksum version", func(m *dependabot.Modules) {
			m.Sum = bytes.ReplaceAll(m.Sum, []byte("v1.5.0"), []byte("not-a-version"))
		}, "invalid-module-checksums"},
		{"checksum field count", func(m *dependabot.Modules) { m.Sum = append(m.Sum, []byte("extra field\n")...) }, "invalid-module-checksums"},
		{"checksum algorithm", func(m *dependabot.Modules) { m.Sum = bytes.ReplaceAll(m.Sum, []byte("h1:"), []byte("h2:")) }, "invalid-module-checksums"},
		{"checksum length", func(m *dependabot.Modules) {
			m.Sum = bytes.ReplaceAll(m.Sum, []byte("h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="), []byte("h1:YQ=="))
		}, "invalid-module-checksums"},
		{"checksum base64", func(m *dependabot.Modules) {
			m.Sum = bytes.ReplaceAll(m.Sum, []byte("h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="), []byte("h1:!"))
		}, "invalid-module-checksums"},
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

func TestTransitionRejectsInvalidBase(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mod    bool
		data   []byte
		reason string
	}{
		{"manifest", true, []byte("require (\n"), "invalid-module-manifest"},
		{"checksum", false, []byte("bad\n"), "invalid-module-checksums"},
		{"oversized sum", false, make([]byte, (16<<20)+1), "module-input-too-large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			base, head := fixtureModules(t, "base"), fixtureModules(t, "head")
			if test.mod {
				base.Mod = test.data
			} else {
				base.Sum = test.data
			}
			_, reason := dependabot.ParseSigstoreModuleTransition(base, head)
			if reason != test.reason {
				t.Fatalf("reason=%q, want %q", reason, test.reason)
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
		originalMod, originalSum := bytes.Clone(mod), bytes.Clone(sum)
		_, reason := dependabot.ParseSigstoreModuleTransition(base, dependabot.Modules{Mod: mod, Sum: sum})
		if !bytes.Equal(mod, originalMod) || !bytes.Equal(sum, originalSum) {
			t.Fatal("transition parser mutated input bytes")
		}
		if reason == "" {
			for _, old := range strings.Split(string(base.Sum), "\n") {
				before := strings.Fields(old)
				if len(before) != 3 {
					continue
				}
				for _, candidate := range strings.Split(string(sum), "\n") {
					after := strings.Fields(candidate)
					if len(after) == 3 && before[0] == after[0] && before[1] == after[1] && before[2] != after[2] {
						t.Fatalf("accepted mutation of existing checksum %s %s", before[0], before[1])
					}
				}
			}
		}
	})
}
