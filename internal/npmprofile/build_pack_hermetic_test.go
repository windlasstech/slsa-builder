package npmprofile

// Hermetic harness for the BuildPack tests. The fake toolchain scripts on PATH
// intercept every subprocess call, and fakeDistributionFetcher intercepts the
// only in-process HTTP (pnpm registry metadata + Yarn distribution download).
// No network access or real package-manager installation is required.

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	fakeNodeVersion     = "v24.11.0"
	fakeNPMVersion      = "11.5.1"
	fakeCorepackVersion = "0.34.1"
	fakePNPMVersion     = "10.14.0"
	fakeYarnVersion     = "4.9.2"

	fakePNPMDistributionURL = "https://registry.npmjs.org/pnpm/-/pnpm-" + fakePNPMVersion + ".tgz"
	fakeYarnDistributionURL = "https://repo.yarnpkg.com/" + fakeYarnVersion + "/packages/yarnpkg-cli/bin/yarn.js"
	fakePNPMPackumentURL    = "https://registry.npmjs.org/pnpm/" + fakePNPMVersion
)

// installFakeToolchain writes scripted node/npm/corepack fakes plus the
// Corepack-managed pnpm/yarn shims into a private fakebin directory and puts
// fakebin first on PATH. It also pre-computes the Corepack `.corepack`
// distribution metadata from the committed fixture bytes so the shims only
// copy files; no digest is hardcoded anywhere.
func installFakeToolchain(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	fakebin := filepath.Join(root, "fakebin")
	data := filepath.Join(root, "data")
	for _, directory := range []string{fakebin, data} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	fixtures := filepath.Join(testRepositoryRoot(t), "testdata", "npm", "buildpack")

	pnpmTGZ := readFixture(t, fixtures, "pnpm-"+fakePNPMVersion+".tgz")
	yarnJS := readFixture(t, fixtures, "yarn-"+fakeYarnVersion+".js")
	writeCorepackMetadata(t, data, ManagerPNPM, fakePNPMVersion, pnpmTGZ)
	writeCorepackMetadata(t, data, ManagerYarn, fakeYarnVersion, yarnJS)

	expand := func(script string) string {
		replacer := strings.NewReplacer(
			"@DATA_DIR@", data,
			"@FIXTURE_DIR@", fixtures,
		)
		return replacer.Replace(script)
	}
	writeScript(t, fakebin, "node", expand(fakeNodeScript))
	writeScript(t, fakebin, "npm", expand(fakeNPMScript))
	writeScript(t, fakebin, "corepack", expand(fakeCorepackScript))
	// Corepack's `enable` copies these shims into the per-build shim directory.
	writeScript(t, data, "pnpm-shim", expand(fakePNPMShimScript))
	writeScript(t, data, "yarn-shim", expand(fakeYarnShimScript))

	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// fakeDistributionFetcher replaces fetchHTTPS in BuildPackConfig. It answers
// exactly the two URLs the real distribution capture fetches and fails the
// test on any other URL, proving no unexpected fetch happens.
func fakeDistributionFetcher(t *testing.T) distributionFetcher {
	t.Helper()
	fixtures := filepath.Join(testRepositoryRoot(t), "testdata", "npm", "buildpack")
	return func(fetchContext context.Context, rawURL string, maximum int64, accept string) ([]byte, error) {
		if fetchContext == nil || maximum <= 0 {
			t.Fatalf("distribution fetch called with invalid bounds: maximum=%d", maximum)
		}
		switch rawURL {
		case fakePNPMPackumentURL:
			if accept != "application/json" {
				t.Fatalf("pnpm packument fetch accept = %q, want application/json", accept)
			}
			payload := readFixture(t, fixtures, "pnpm-"+fakePNPMVersion+".tgz")
			sum := sha512.Sum512(payload)
			integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
			return []byte(fmt.Sprintf(
				`{"name":"pnpm","version":%q,"dist":{"tarball":%q,"integrity":%q}}`,
				fakePNPMVersion, fakePNPMDistributionURL, integrity,
			)), nil
		case fakeYarnDistributionURL:
			if accept != "application/octet-stream" {
				t.Fatalf("Yarn distribution fetch accept = %q, want application/octet-stream", accept)
			}
			return readFixture(t, fixtures, "yarn-"+fakeYarnVersion+".js"), nil
		default:
			t.Fatalf("unexpected distribution fetch: %q", rawURL)
			return nil, fmt.Errorf("unexpected distribution fetch: %q", rawURL)
		}
	}
}

// writeCorepackMetadata computes the SHA-512 of the distribution fixture bytes
// at runtime and writes the .corepack metadata file in the exact shape
// captureDistribution reads: {"locator":{"name","reference"},"hash":"sha512.<hex>"}.
func writeCorepackMetadata(t *testing.T, data string, manager Manager, version string, payload []byte) {
	t.Helper()
	sum := sha512.Sum512(payload)
	metadata := fmt.Sprintf(
		`{"locator":{"name":%q,"reference":%q},"hash":"sha512.%s"}`,
		string(manager), version, hex.EncodeToString(sum[:]),
	)
	if err := os.WriteFile(filepath.Join(data, string(manager)+".corepack"), []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFixture(t *testing.T, directory, name string) []byte {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(directory, name))
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func writeScript(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

const fakeNodeScript = `#!/bin/sh
if [ "$1" = "--version" ]; then
  printf '` + fakeNodeVersion + `\n'
  exit 0
fi
echo "fake node: unsupported arguments: $*" >&2
exit 1
`

const fakeNPMScript = `#!/bin/sh
case "$1" in
  --version)
    printf '` + fakeNPMVersion + `\n'
    ;;
  ci)
    exit 0
    ;;
  run)
    if [ "$2" = "build" ]; then
      exit 0
    fi
    exit 1
    ;;
  pack)
    destination=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "--pack-destination" ]; then
        destination="$2"
        shift 2
        continue
      fi
      shift
    done
    if [ -z "$destination" ]; then
      exit 1
    fi
    cp "@FIXTURE_DIR@/npm-root-valid-1.0.0.tgz" "$destination/npm-root-valid-1.0.0.tgz"
    ;;
  *)
    echo "fake npm: unsupported arguments: $*" >&2
    exit 1
    ;;
esac
exit 0
`

const fakeCorepackScript = `#!/bin/sh
case "$1" in
  --version)
    printf '` + fakeCorepackVersion + `\n'
    ;;
  enable)
    destination=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "--install-directory" ]; then
        destination="$2"
        shift 2
        continue
      fi
      shift
    done
    if [ -z "$destination" ]; then
      exit 1
    fi
    cp "@DATA_DIR@/pnpm-shim" "$destination/pnpm"
    cp "@DATA_DIR@/yarn-shim" "$destination/yarn"
    chmod 0700 "$destination/pnpm" "$destination/yarn"
    ;;
  *)
    echo "fake corepack: unsupported arguments: $*" >&2
    exit 1
    ;;
esac
exit 0
`

const fakePNPMShimScript = `#!/bin/sh
case "$1" in
  --version)
    mkdir -p "$COREPACK_HOME/v1/pnpm/` + fakePNPMVersion + `"
    cp "@DATA_DIR@/pnpm.corepack" "$COREPACK_HOME/v1/pnpm/` + fakePNPMVersion + `/.corepack"
    printf 'Corepack DEBUG: resolving pnpm@` + fakePNPMVersion + `\n'
    printf ' corepack Installing pnpm@` + fakePNPMVersion + ` from ` + fakePNPMDistributionURL + `\n'
    printf '` + fakePNPMVersion + `\n'
    ;;
  install)
    if [ "$2" = "--frozen-lockfile" ]; then
      exit 0
    fi
    exit 1
    ;;
  run)
    if [ "$2" = "build" ]; then
      exit 0
    fi
    exit 1
    ;;
  pack)
    destination=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "--pack-destination" ]; then
        destination="$2"
        shift 2
        continue
      fi
      shift
    done
    if [ -z "$destination" ]; then
      exit 1
    fi
    cp "@FIXTURE_DIR@/scoped-1.0.0.tgz" "$destination/scoped-1.0.0.tgz"
    ;;
  *)
    echo "fake pnpm: unsupported arguments: $*" >&2
    exit 1
    ;;
esac
exit 0
`

const fakeYarnShimScript = `#!/bin/sh
case "$1" in
  --version)
    mkdir -p "$COREPACK_HOME/v1/yarn/` + fakeYarnVersion + `"
    cp "@DATA_DIR@/yarn.corepack" "$COREPACK_HOME/v1/yarn/` + fakeYarnVersion + `/.corepack"
    printf 'Corepack DEBUG: resolving yarn@` + fakeYarnVersion + `\n'
    printf ' corepack Installing yarn@` + fakeYarnVersion + ` from ` + fakeYarnDistributionURL + `\n'
    printf '` + fakeYarnVersion + `\n'
    ;;
  install)
    if [ "$2" = "--immutable" ]; then
      exit 0
    fi
    exit 1
    ;;
  run)
    if [ "$2" = "build" ]; then
      exit 0
    fi
    exit 1
    ;;
  pack)
    output=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "--out" ]; then
        output="$2"
        shift 2
        continue
      fi
      shift
    done
    if [ -z "$output" ]; then
      exit 1
    fi
    cp "@FIXTURE_DIR@/yarn-valid-1.0.0.tgz" "$output"
    ;;
  *)
    echo "fake yarn: unsupported arguments: $*" >&2
    exit 1
    ;;
esac
exit 0
`
