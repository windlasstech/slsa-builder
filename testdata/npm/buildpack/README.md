# BuildPack hermetic-test fixtures

Static byte fixtures consumed by the hermetic BuildPack tests in
`internal/npmprofile/build_pack_hermetic_test.go`. They let the BuildPack tests run without any
network access or real package-manager installs.

## Contents

| File                       | Role                                                                                                      |
| -------------------------- | --------------------------------------------------------------------------------------------------------- |
| `pnpm-10.14.0.tgz`         | Stand-in pnpm distribution tarball. Bytes are only hashed (SHA-512) and never unpacked.                   |
| `yarn-4.9.2.js`            | Stand-in Yarn CLI bundle. Bytes are only hashed (SHA-512) and never executed.                             |
| `npm-root-valid-1.0.0.tgz` | `npm pack` output for `windlass-fixture-unscoped@1.0.0` (fixture `testdata/npm/packages/npm-root-valid`). |
| `scoped-1.0.0.tgz`         | `pnpm pack` output for `@windlass-fixtures/scoped@1.0.0` (fixture `testdata/npm/packages/scoped-valid`).  |
| `yarn-valid-1.0.0.tgz`     | `yarn pack` output for `windlass-fixture-yarn@1.0.0` (fixture `testdata/npm/packages/yarn-valid`).        |

The three pack outputs each contain `package/package.json` (strict JSON carrying the fixture
package's `name`/`version`) and `package/dist/index.js`. All tar entries are `package/`-prefixed
regular files, matching the `packed.go` archive constraints (`validateArchivePath`, single-tarball
pack output).

## Regeneration

The fixtures are generated with a one-off Go program using `archive/tar` + `compress/gzip`. Save the
program below as `genfixtures.go` outside the module tree (or in a scratch directory) and run it
from the repository root:

```bash
go run /path/to/genfixtures.go
```

```go
// One-off generator for the committed testdata/npm/buildpack fixtures.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const outputDirectory = "testdata/npm/buildpack"

func main() {
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		fail(err)
	}
	writeFile("pnpm-10.14.0.tgz", tarball(map[string]string{
		"package/index.js": "// pnpm 10.14.0 distribution placeholder; bytes are only hashed.\n",
	}))
	writeFile("yarn-4.9.2.js", []byte("#!/usr/bin/env node\n// Yarn 4.9.2 CLI placeholder; bytes are only hashed.\n"))
	writeFile("npm-root-valid-1.0.0.tgz", packTarball("windlass-fixture-unscoped", "1.0.0"))
	writeFile("scoped-1.0.0.tgz", packTarball("@windlass-fixtures/scoped", "1.0.0"))
	writeFile("yarn-valid-1.0.0.tgz", packTarball("windlass-fixture-yarn", "1.0.0"))
	fmt.Println("fixtures written to", outputDirectory)
}

// packTarball builds the pack-output shape the BuildPack pipeline validates:
// package/package.json plus package/dist/index.js, package/-prefixed regular
// entries only.
func packTarball(name, version string) []byte {
	manifest := fmt.Sprintf("{\"name\":%q,\"version\":%q}\n", name, version)
	return tarball(map[string]string{
		"package/package.json":  manifest,
		"package/dist/index.js": "export {};\n",
	})
}

func tarball(files map[string]string) []byte {
	var buffer bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		fail(err)
	}
	tarWriter := tar.NewWriter(gzipWriter)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		body := []byte(files[name])
		if err := tarWriter.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			fail(err)
		}
		if _, err := tarWriter.Write(body); err != nil {
			fail(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		fail(err)
	}
	if err := gzipWriter.Close(); err != nil {
		fail(err)
	}
	return buffer.Bytes()
}

func writeFile(name string, contents []byte) {
	if err := os.WriteFile(filepath.Join(outputDirectory, name), contents, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
```

The pack-output tarball names in this directory are the names the fake toolchain scripts copy into
the pack-destination directory; the tests do not depend on real npm/pnpm/Yarn naming rules.
