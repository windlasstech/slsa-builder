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
	artifactURL := "https://nodejs.org/dist/v24.20.0/node-v24.20.0-linux-x64.tar.gz"
	artifactSHA := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	pairRow := "| `24.20.0` | `11.18.0` |\n"
	artifactRow := "| `linux-x64` | `" + artifactURL + "` | `" + artifactSHA + "` |\n"
	artifactTable := "| Platform | Tarball URL | SHA-256 (hex) |\n| --- | --- | --- |\n" + artifactRow
	build := "## Node.js and npm toolchain pair\n\n### Pinned runtime table\n\n" +
		"| Node.js version | Bundled npm version |\n| --- | --- |\n" + pairRow + "\n" + artifactTable
	pin := fmt.Sprintf(`{"version":"11.20.0","tarball_url":"https://registry.npmjs.org/npm/-/npm-11.20.0.tgz","integrity":%q}`, integrity)
	pair := fmt.Sprintf(`{"node_version":"24.20.0","npm_version":"11.18.0","artifacts":{"linux-x64":{"url":%q,"sha256":%q}}}`, artifactURL, artifactSHA)
	descriptor := `{"name":"publish-cli","uri":"pkg:npm/npm@11.20.0","digest":{"sha512":"` + strings.Repeat("0", 128) + `"},"mediaType":"application/gzip","annotations":{"digest_authority":"builder-pinned-record"}}`
	example := "### JS/TS npm `builderDependencies` schema\n\nThe exact current array is:\n\n```json\n[" + descriptor + "]\n```\n"
	publish = example + "\n" + publish
	tests := []struct {
		name, publish, build, pin, pair string
		pass                            bool
	}{
		{name: "matching disk records differ from embeds", pass: true},
		{name: "prose after table", publish: publish + "The allowlist above is reviewed on every bump.\n", pass: true},
		{name: "prettier padding", publish: strings.ReplaceAll(publish, " |", "       |"), pass: true},
		{name: "plain cells", publish: strings.NewReplacer("`11.20.0`", "11.20.0", "`https://registry.npmjs.org/npm/-/npm-11.20.0.tgz`", "https://registry.npmjs.org/npm/-/npm-11.20.0.tgz", "`"+integrity+"`", integrity).Replace(publish), pass: true},
		{name: "CRLF", publish: strings.ReplaceAll(publish, "\n", "\r\n"), pass: true},
		{name: "version drift", publish: strings.Replace(publish, "`11.20.0`", "`11.20.1`", 1)},
		{name: "URL drift", publish: strings.Replace(publish, "npm-11.20.0.tgz", "npm-11.20.1.tgz", 1)},
		{name: "SRI drift", publish: strings.Replace(publish, "sha512-A", "sha512-B", 1)},
		{name: "node drift", build: strings.Replace(build, "24.20.0", "24.20.1", 1)},
		{name: "bundled npm drift", build: strings.Replace(build, "11.18.0", "11.18.1", 1)},
		{name: "artifact URL drift", build: strings.Replace(build, artifactURL, strings.Replace(artifactURL, "v24.20.0", "v24.20.1", 1), 1)},
		{name: "artifact SHA drift", build: strings.Replace(build, artifactSHA, "f"+artifactSHA[1:], 1)},
		{name: "artifact platform drift", build: strings.Replace(build, "`linux-x64`", "`linux-arm64`", 1)},
		{name: "missing artifact table", build: strings.Replace(build, "\n"+artifactTable, "\n", 1)},
		{name: "extra artifact row", build: strings.Replace(build, artifactRow, artifactRow+artifactRow, 1)},
		{name: "missing publish section", publish: strings.Replace(publish, "## Publish-stage npm CLI allowlist", "## Other", 1)},
		{name: "missing build section", build: strings.Replace(build, "## Node.js and npm toolchain pair", "## Other", 1)},
		{name: "missing publish table", publish: "## Publish-stage npm CLI allowlist\n"},
		{name: "missing build tables", build: "## Node.js and npm toolchain pair\n"},
		{name: "table in another section", publish: strings.Replace(publish, "## Publish-stage npm CLI allowlist\n", "## Publish-stage npm CLI allowlist\n\n## Other\n", 1)},
		{name: "malformed row", publish: strings.Replace(publish, " | `sha512-", " `sha512-", 1)},
		{name: "empty cell", build: strings.Replace(build, "`24.20.0`", "", 1)},
		{name: "unbalanced code", build: strings.Replace(build, "`24.20.0`", "`24.20.0", 1)},
		{name: "malformed separator", build: strings.Replace(build, "| --- | --- |", "| -- | --- |", 1)},
		{name: "extra row", build: strings.Replace(build, pairRow, pairRow+pairRow, 1)},
		{name: "malformed trailing row", build: build + "| broken |\n"},
		{name: "malformed pipe row after table", publish: publish + "| `11.20.0` |\n"},
		{name: "duplicate tables", build: build + "\n" + strings.SplitN(build, "### Pinned runtime table\n\n", 2)[1]},
		{name: "record drift", pin: strings.Replace(pin, "11.20.0", "11.20.1", 1)},
		{name: "pair record drift", pair: strings.Replace(pair, "24.20.0", "24.20.1", 1)},
		{name: "pair record artifact drift", pair: strings.Replace(pair, artifactSHA, "f"+artifactSHA[1:], 1)},
		{name: "invalid record", pin: "{}"},
		{name: "invalid pair", pair: "{}"},
		{name: "pair without artifacts", pair: `{"node_version":"24.20.0","npm_version":"11.18.0"}`},
		{name: "duplicate JSON member", pair: `{"node_version":"24.20.0","node_version":"24.20.0","npm_version":"11.18.0"}`},
		{name: "descriptor version drift", publish: strings.Replace(publish, "pkg:npm/npm@11.20.0", "pkg:npm/npm@11.20.1", 1)},
		{name: "descriptor digest drift", publish: strings.Replace(publish, strings.Repeat("0", 128), strings.Repeat("1", 128), 1)},
		{name: "missing example", publish: strings.TrimPrefix(publish, example)},
		{name: "duplicate example", publish: example + publish},
		{name: "duplicate designation", publish: strings.Replace(publish, "\n\n```json", "\nThe exact current array is:\n\n```json", 1)},
		{name: "duplicate descriptor", publish: strings.Replace(publish, "["+descriptor+"]", "["+descriptor+","+descriptor+"]", 1)},
		{name: "missing descriptor", publish: strings.Replace(publish, "["+descriptor+"]", "[]", 1)},
		{name: "malformed example", publish: strings.Replace(publish, "["+descriptor+"]", "[{", 1)},
		{name: "duplicate nested JSON", publish: strings.Replace(publish, `"sha512":`, `"sha512":"ignored","sha512":`, 1)},
		{name: "wrong fence", publish: strings.Replace(publish, "```json", "```text", 1)},
		{name: "unterminated example", publish: strings.Replace(publish, "]\n```", "]", 1)},
		{name: "SRI and hex disagreement", pin: strings.Replace(pin, "sha512-A", "sha512-B", 1), publish: strings.Replace(publish, "sha512-A", "sha512-B", 1)},
		{name: "negative example is not designated", publish: strings.Replace(publish, example, example+"\nInvalid example:\n```json\n[{\"name\":\"publish-cli\",\"uri\":\"wrong\"}]\n```\n", 1), pass: true},
		{name: "oversized specification", publish: publish + strings.Repeat(" ", 16<<20)},
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
