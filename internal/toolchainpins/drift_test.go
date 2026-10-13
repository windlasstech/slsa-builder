package toolchainpins_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestCheckDrift(t *testing.T) {
	t.Parallel()
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
	publish := "## Publish-stage npm CLI allowlist\n\n| Version | Tarball URL | SHA-512 (SRI) |\n| --- | --- | --- |\n" +
		"| `11.20.0` | `https://registry.npmjs.org/npm/-/npm-11.20.0.tgz` | `" + integrity + "` |\n"
	build := "## Node.js and npm toolchain pair\n\n### Pinned runtime table\n\n" +
		"| Node.js version | Bundled npm version |\n| --- | --- |\n| `24.20.0` | `11.18.0` |\n"
	pin := fmt.Sprintf(`{"version":"11.20.0","tarball_url":"https://registry.npmjs.org/npm/-/npm-11.20.0.tgz","integrity":%q}`, integrity)
	pair := `{"node_version":"24.20.0","npm_version":"11.18.0"}`
	tests := []struct {
		name, publish, build, pin, pair string
		pass                            bool
	}{
		{name: "matching disk records differ from embeds", pass: true},
		{name: "prose after table", publish: publish + "The allowlist above is reviewed on every bump.\n", pass: true},
		{name: "prettier padding", publish: strings.ReplaceAll(publish, " |", "       |"), pass: true},
		{name: "plain cells", publish: strings.ReplaceAll(publish, "`", ""), pass: true},
		{name: "CRLF", publish: strings.ReplaceAll(publish, "\n", "\r\n"), pass: true},
		{name: "version drift", publish: strings.Replace(publish, "`11.20.0`", "`11.20.1`", 1)},
		{name: "URL drift", publish: strings.Replace(publish, "npm-11.20.0.tgz", "npm-11.20.1.tgz", 1)},
		{name: "SRI drift", publish: strings.Replace(publish, "sha512-A", "sha512-B", 1)},
		{name: "node drift", build: strings.Replace(build, "24.20.0", "24.20.1", 1)},
		{name: "bundled npm drift", build: strings.Replace(build, "11.18.0", "11.18.1", 1)},
		{name: "missing publish section", publish: strings.Replace(publish, "## Publish-stage npm CLI allowlist", "## Other", 1)},
		{name: "missing build section", build: strings.Replace(build, "## Node.js and npm toolchain pair", "## Other", 1)},
		{name: "missing publish table", publish: "## Publish-stage npm CLI allowlist\n"},
		{name: "missing build table", build: "## Node.js and npm toolchain pair\n"},
		{name: "table in another section", publish: strings.Replace(publish, "\n\n", "\n\n## Other\n", 1)},
		{name: "malformed row", publish: strings.Replace(publish, " | `sha512-", " `sha512-", 1)},
		{name: "empty cell", build: strings.Replace(build, "`24.20.0`", "", 1)},
		{name: "unbalanced code", build: strings.Replace(build, "`24.20.0`", "`24.20.0", 1)},
		{name: "malformed separator", build: strings.Replace(build, "| --- | --- |", "| -- | --- |", 1)},
		{name: "extra row", build: build + "| `24.20.0` | `11.18.0` |\n"},
		{name: "malformed trailing row", build: build + "| broken |\n"},
		{name: "malformed pipe row after table", publish: publish + "| `11.20.0` |\n"},
		{name: "duplicate table", build: build + "\n" + strings.SplitN(build, "### Pinned runtime table\n\n", 2)[1]},
		{name: "record drift", pin: strings.Replace(pin, "11.20.0", "11.20.1", 1)},
		{name: "pair record drift", pair: strings.Replace(pair, "24.20.0", "24.20.1", 1)},
		{name: "invalid record", pin: "{}"},
		{name: "invalid pair", pair: "{}"},
		{name: "duplicate JSON member", pair: `{"node_version":"24.20.0","node_version":"24.20.0","npm_version":"11.18.0"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given four independent on-disk inputs, never the repository's real files.
			inputs := toolchainpins.DriftInputs{
				PublishPin:  writeDriftFile(t, "pin.json", firstNonempty(test.pin, pin)),
				PairRecord:  writeDriftFile(t, "pair.json", firstNonempty(test.pair, pair)),
				PublishSpec: writeDriftFile(t, "publish.md", firstNonempty(test.publish, publish)),
				BuildSpec:   writeDriftFile(t, "build.md", firstNonempty(test.build, build)),
			}
			// When disk records are compared with the specification tables.
			err := toolchainpins.CheckDrift(inputs)
			// Then only matching, well-formed tables pass.
			if test.pass {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var failure *toolchainpins.DriftError
			if !errors.As(err, &failure) || failure.DiagnosticID() != "windlass.verify.error.toolchain-pin-drift" {
				t.Fatalf("CheckDrift() = %v, want toolchain-pin-drift", err)
			}
		})
	}
}

func firstNonempty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func writeDriftFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
