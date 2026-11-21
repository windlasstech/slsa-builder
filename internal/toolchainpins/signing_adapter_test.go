package toolchainpins_test

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

const sigstoreModule = "github.com/sigstore/sigstore-go"
const sigstoreSum = "h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw="
const sigstoreDirHash = "0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac"

func TestSigningAdapterModuleGraphConsistency(t *testing.T) {
	t.Parallel()
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile("../../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	got, err := toolchainpins.ParseSigningAdapterModule(mod, sum)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "v1.4.0" || got.Sum != sigstoreSum || got.DirHash1 != sigstoreDirHash {
		t.Fatalf("go.mod/go.sum signer = %#v, want v1.4.0 / %s / %s", got, sigstoreSum, sigstoreDirHash)
	}
}

func TestParseSigningAdapterModule(t *testing.T) {
	t.Parallel()
	mod := "module example.test/builder\n\ngo 1.27\nrequire " + sigstoreModule + " v1.4.0\n"
	sum := sigstoreModule + " v1.4.0 " + sigstoreSum + "\n"
	for _, test := range []struct {
		name, mod, sum       string
		wantVersion, wantSum string
	}{
		{"exact module content", mod, sum, "v1.4.0", sigstoreSum},
		{"older content before match", mod, strings.ReplaceAll(sum, "v1.4.0", "v1.3.0") + sum, "v1.4.0", sigstoreSum},
		{"newer content after match", mod, sum + strings.ReplaceAll(sum, "v1.4.0", "v1.5.0"), "v1.4.0", sigstoreSum},
		{"other version go.mod", mod, strings.ReplaceAll(sum, "v1.4.0", "v1.3.0/go.mod") + sum, "v1.4.0", sigstoreSum},
		{"identical duplicate", mod, sum + sum, "", ""},
		{"malformed matching after valid", mod, sum + sigstoreModule + " v1.4.0\n", "", ""},
		{"require block", "module example.test/builder\nrequire (\n\t" + sigstoreModule + " v1.4.0\n)\n", sum, "v1.4.0", sigstoreSum},
		{"go.mod sum does not override content", mod, sum + sigstoreModule + " v1.4.0/go.mod h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n", "v1.4.0", sigstoreSum},
		{"alternate exact version", strings.ReplaceAll(mod, "v1.4.0", "v1.3.0"), strings.ReplaceAll(sum, "v1.4.0", "v1.3.0"), "v1.3.0", sigstoreSum},
		{"future candidate version", strings.ReplaceAll(mod, "v1.4.0", "v1.5.0"), strings.ReplaceAll(sum, "v1.4.0", "v1.5.0"), "v1.5.0", sigstoreSum},
		{"go.mod only", mod, "", "", ""},
		{"go.sum only", "module example.test/builder\n", sum, "", ""},
		{"go.mod-sum substitution", mod, strings.ReplaceAll(sum, "v1.4.0 ", "v1.4.0/go.mod "), "", ""},
		{"wrong version sum", mod, strings.ReplaceAll(sum, "v1.4.0", "v1.3.0"), "", ""},
		{"malformed base64", mod, strings.ReplaceAll(sum, sigstoreSum, "h1:!"), "", ""},
		{"short directory hash", mod, strings.ReplaceAll(sum, sigstoreSum, "h1:YQ=="), "", ""},
		{"wrong algorithm", mod, strings.ReplaceAll(sum, "h1:", "h2:"), "", ""},
		{"missing checksum", mod, sigstoreModule + " v1.4.0\n", "", ""},
		{"conflicting sums", mod, sum + sigstoreModule + " v1.4.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n", "", ""},
		{"alternate valid content sum", mod, sigstoreModule + " v1.4.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n", "v1.4.0", "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
		{"local replacement", mod + "replace " + sigstoreModule + " => ../signer\n", sum, "", ""},
		{"version replacement", mod + "replace " + sigstoreModule + " v1.4.0 => " + sigstoreModule + " v1.3.0\n", sum, "", ""},
		{"replace block", mod + "replace (\n" + sigstoreModule + " => ../signer\n)\n", sum, "", ""},
		{"malformed go.mod", mod + "require (\n", sum, "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given independent manifest bytes used only by generation/drift tooling.
			// When strict module parsing resolves the candidate's matching content checksum.
			got, err := toolchainpins.ParseSigningAdapterModule([]byte(test.mod), []byte(test.sum))
			// Then conversion preserves candidate values independently of compiled pin policy.
			if test.wantVersion != "" {
				decoded, decodeErr := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(test.wantSum, "h1:"))
				if decodeErr != nil {
					t.Fatalf("invalid expected checksum: %v", decodeErr)
				}
				wantHex := hex.EncodeToString(decoded)
				if err != nil || got.Version != test.wantVersion || got.Sum != test.wantSum || got.DirHash1 != wantHex {
					t.Fatalf("ParseSigningAdapterModule() = %#v, %v; want %s / %s / %s", got, err, test.wantVersion, test.wantSum, wantHex)
				}
				return
			}
			var drift *toolchainpins.DriftError
			if !errors.As(err, &drift) || drift.DiagnosticID() != "windlass.verify.error.toolchain-pin-drift" {
				t.Fatalf("ParseSigningAdapterModule() = %#v, %v; want toolchain-pin-drift", got, err)
			}
		})
	}
}

func FuzzParseSigningAdapterModule(f *testing.F) {
	// Given a required candidate version and both valid and hostile checksum snapshots.
	mod := "module example.test/builder\nrequire " + sigstoreModule + " v1.4.0\n"
	sum := sigstoreModule + " v1.4.0 " + sigstoreSum + "\n"
	for _, seed := range []string{sum, strings.ReplaceAll(sum, "v1.4.0", "v1.3.0") + sum,
		sum + sum, sigstoreModule + " v1.4.0\n", strings.ReplaceAll(sum, "h1:", "h2:"),
		strings.ReplaceAll(sum, "v1.4.0", "v1.4.0/go.mod"), strings.ReplaceAll(sum, sigstoreSum, "h1:!")} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, snapshot string) {
		// When parsing arbitrary checksum records against the explicit requirement.
		got, err := toolchainpins.ParseSigningAdapterModule([]byte(mod), []byte(snapshot))
		// Then failure is classified, or the required version carries canonical equivalent digests.
		if err != nil {
			var drift *toolchainpins.DriftError
			if !errors.As(err, &drift) || drift.DiagnosticID() != "windlass.verify.error.toolchain-pin-drift" {
				t.Fatalf("unclassified error: %v", err)
			}
			return
		}
		encoded, ok := strings.CutPrefix(got.Sum, "h1:")
		digest, decodeErr := base64.StdEncoding.Strict().DecodeString(encoded)
		if !ok || decodeErr != nil || len(digest) != 32 || base64.StdEncoding.EncodeToString(digest) != encoded ||
			got.Version != "v1.4.0" || got.DirHash1 != hex.EncodeToString(digest) {
			t.Fatalf("invalid successful result: %#v", got)
		}
		matches := 0
		for _, line := range strings.Split(snapshot, "\n") {
			fields := strings.Fields(line)
			if len(fields) == 3 && fields[0] == sigstoreModule && fields[1] == "v1.4.0" && fields[2] == got.Sum {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("successful checksum has %d matching source records", matches)
		}
	})
}
