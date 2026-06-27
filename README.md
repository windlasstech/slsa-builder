<div align="center">

# slsa-builder

[![GitHub License](https://img.shields.io/github/license/windlasstech/slsa-builder)](LICENSE)
[![SemVer Versioning](https://img.shields.io/badge/version_scheme-SemVer-0097a7)](https://semver.org/)
[![SLSA Build L3](slsa-build-l3-badge.svg)](https://slsa.dev/spec/v1.2/build-requirements#build-platform)
[![GitHub Release](https://img.shields.io/github/v/release/windlasstech/slsa-builder)](https://github.com/windlasstech/slsa-builder/releases)
[![GitHub Release Date](https://img.shields.io/github/release-date/windlasstech/slsa-builder)](https://github.com/windlasstech/slsa-builder/releases)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-3.0-4baaaa.svg)](https://github.com/windlasstech/.github/blob/main/CODE_OF_CONDUCT.md)
[![GitHub issues](https://img.shields.io/badge/issue_tracking-GitHub-blue.svg)](https://github.com/windlasstech/slsa-builder/issues)

[![Lint](https://github.com/windlasstech/slsa-builder/actions/workflows/lint.yml/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/lint.yml)
[![CodeQL](https://github.com/windlasstech/slsa-builder/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/github-code-scanning/codeql)
[![OSV Scanner](https://github.com/windlasstech/slsa-builder/actions/workflows/osv-scanner.yml/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/osv-scanner.yml)
[![Dependency Review](https://github.com/windlasstech/slsa-builder/actions/workflows/dependency-review.yml/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/dependency-review.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/windlasstech/slsa-builder/badge)](https://scorecard.dev/viewer/?uri=github.com/windlasstech/slsa-builder)

English | [한국어](README.ko.md)

</div>

A reusable, profile-extensible SLSA provenance builder foundation.

## Development setup

This repository uses [mise](https://mise.jdx.dev/) to install and pin the development-tool runtime versions. Go is the primary implementation language; Node.js and pnpm are used only for development tooling such as Prettier and Lefthook.

### Prerequisites

- [mise](https://mise.jdx.dev/getting-started.html) installed
- Git with a configured user name and email

### Bootstrap

```bash
mise install
pnpm install
```

This installs the pinned versions of Go, Node.js, pnpm, and the CLI tools defined in `mise.toml`. Lefthook hooks are installed automatically as a `postinstall` step when mise installs Lefthook. The `pnpm install` step then installs the project-local development dependencies declared in `package.json`.

In CI, run mise with locked mode to avoid API calls to registries:

```bash
MISE_LOCKED=1 mise install
pnpm install
```

### What mise installs versus what pnpm installs

mise installs language runtimes and standalone CLI binaries:

- Go, Node.js, and pnpm
- `golangci-lint`, `shellcheck`, `shfmt`, `lefthook`, `actionlint`

Go source formatting and import normalization are handled by `golangci-lint` formatters (`gofmt`, `goimports`), configured in `.golangci.yml`, rather than by standalone formatter binaries.

pnpm installs Node.js-based development dependencies that are coupled to repository configuration files:

- `prettier` (configured by `.prettierrc`)
- `markdownlint-cli2` (configured by `.markdownlint-cli2.jsonc`)

Keeping Prettier and `markdownlint-cli2` as project-local pnpm dependencies preserves their full dependency graph in `pnpm-lock.yaml` and keeps them aligned with editor integrations and the organization's dependency-review workflow.

After bootstrap, the following commands are available through mise:

```bash
go version
node --version
pnpm --version
golangci-lint --version
shellcheck --version
shfmt --version
lefthook --version
actionlint --version
```

### Tool versions

Tool versions are declared in `mise.toml`. A `mise.lock` file is committed to ensure reproducible installs across platforms. If you change a tool version in `mise.toml`, regenerate the lockfile with:

```bash
mise lock
```

### Conventional commits and sign-off

This project requires a `Signed-off-by:` trailer on every commit (DCO). Lefthook will be configured to enforce this locally; CI and branch protection provide the authoritative check.
