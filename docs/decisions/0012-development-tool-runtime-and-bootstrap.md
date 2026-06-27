---
parent: Decisions
nav_order: 12
status: accepted
date: 12026-06-24
decision-makers: Yunseo Kim
---

# Adopt mise as the Unified Development-Tool Runtime and Bootstrap

## Context and Problem Statement

`slsa-builder` is intentionally polyglot at the development-tool layer:

- The trusted implementation is Go (ADR 0004).
- Markdown formatting, local Git-hook orchestration, and other quality-of-life tooling may run on Node.js/pnpm (ADR 0009, ADR 0010, ADR 0011).
- Additional Go-ecosystem tools are expected: `gofmt`, `goimports`, `golangci-lint`, `govulncheck`, ShellCheck, and `shfmt` (ADR 0005, ADR 0006, ADR 0007, ADR 0008).

Now the repository pins only the package manager (`pnpm 11.8.0` via `devEngines.packageManager`) and two Node-based developer dependencies (`prettier 3.8.4`, `markdownlint-cli2 ^0.22.1`). It does not yet pin a Node.js version, a Go version, or the versions of the Go ecosystem tools. There is also no single contributor command that installs every required tool.

We need a unified runtime/bootstrap solution so that every contributor and CI job can obtain the same versions of Go, Node.js, pnpm, and the surrounding linters/formatters with minimal friction and strong reproducibility.

## Decision Drivers

- **Polyglot but Go-first.** The solution must install Go and Node.js tooling side by side without making the JavaScript runtime feel like the primary platform.
- **Trust and reproducibility.** Tool versions must be pinned and reviewable; downloads should be checksum- or lockfile-protected where possible.
- **Contributor friction.** A new clone should require at most one or two commands before `pnpm install`, `go build`, and local checks work.
- **CI alignment.** The same configuration must work on GitHub Actions `ubuntu-latest` (and ideally macOS) without containerizing the entire build.
- **Supply-chain posture.** Prefer tools with lockfiles, attestation support, and a policy-friendly update model; avoid plugin ecosystems that execute unreviewed shell scripts.
- **Composability.** The solution should not replace pnpm, Lefthook, or the existing ADR toolchain; it should provide the runtimes those tools run on top of.

## Considered Options

- Use mise as the unified development-tool runtime.
- Use devbox as the unified development environment.
- Use a Nix flake / `nix develop` as the unified environment.
- Use devenv as the Nix-based developer environment.
- Use Flox as the Nix-backed package/environment manager.
- Use asdf as the plugin-based runtime version manager.
- Use proto (moonrepo) as the toolchain manager.
- Use Hermit to distribute a hermetic tool bundle.
- Use a Docker / devcontainer environment.
- Use a Taskfile (Task) plus manual installation instructions.
- Use nvm/fnm for Node.js and gvm/manual installs for Go.

## Decision Outcome

Chosen option: **Use mise as the unified development-tool runtime**, because it provides a single, fast, polyglot version manager that natively supports Go and Node.js, can install pnpm and other CLIs through multiple secure backends, offers a lockfile with checksum/provenance support, and fits the Go-first, low-friction culture of the project without requiring Nix or containers.

We will add a root `mise.toml` that declares:

- Go version
- Node.js version
- pnpm version (matching `package.json`)
- `golangci-lint`, `shellcheck`, `shfmt`, and other CLI tools as `aqua` or `ubi` backend packages

We will also add a `mise.lock` file and document the contributor bootstrap as:

```bash
mise install
pnpm install
```

If mise is not installed locally, contributors may use the official install script or install mise through their system package manager.

This decision does **not** change the trusted runtime boundary from ADR 0004: Go remains the trusted implementation language, and Node.js/pnpm remain development-only. It also does not replace Lefthook (ADR 0011) or pnpm (ADR 0010); it supplies the runtimes they depend on.

### Consequences

- Good, because one configuration file and one command install Go, Node.js, pnpm, and CLI tools.
- Good, because mise supports lockfiles (`mise.lock`) with checksum and provenance metadata, aligning with the project's lockfile discipline.
- Good, because mise has a Go-friendly implementation, supports `aqua` (security-oriented) and `ubi` (single-binary GitHub releases) backends, and can enforce `minimum_release_age`.
- Good, because contributors on Linux and macOS get the same workflow without Docker overhead.
- Good, because CI can run `mise install` and then reuse the same tool versions as local development.
- Neutral, because mise is a newer tool than asdf/nvm; some contributors may need to install it once.
- Bad, because adopting mise adds one more dependency to the bootstrap path.
- Bad, because Windows contributors may need WSL or an alternative path (mise itself is Unix/macOS-first).

### Confirmation

This decision is confirmed when:

- A root `mise.toml` pins Go, Node.js, pnpm, and required CLI tools.
- A `mise.lock` is committed and kept in sync.
- `pnpm install` and `go build` succeed from a fresh clone after `mise install`.
- CI uses mise to install the same tool versions as local development.
- Documentation explains how to install mise and bootstrap the repository.
- The ADR toolchain boundary remains clear: Node.js/pnpm are still development-only.

## Pros and Cons of the Options

### Use mise as the unified development-tool runtime

[mise](https://mise.jdx.dev/) is a Rust-based development environment, version manager, and task runner that supports Go, Node.js, and many CLI tools through multiple backends (`aqua`, `ubi`, `npm`, `cargo`, `asdf`, `github`, `http`).

Although mise positions itself broadly as a development environment tool, this project adopts it specifically as the **development-tool runtime and bootstrap** layer. It will install and pin Go, Node.js, pnpm, and the CLI tools required by the ADR toolchain, but it will not manage application services, containers, or the full developer shell environment.

- Good, because it replaces asdf-style shims with a faster, single-binary model.
- Good, because it supports a `mise.lock` with checksums and provenance metadata.
- Good, because the `aqua` backend maps well to the project's supply-chain concerns (checksums, signed releases).
- Good, because it can install pnpm and Node.js as well as Go and Go-ecosystem binaries in one file.
- Good, because it has `minimum_release_age` and locked-mode options for supply-chain policy alignment.
- Neutral, because mise also includes a task runner; we may or may not use it.
- Bad, because it is less ubiquitous than asdf or nvm; new contributors may not have it.
- Bad, because Windows support is limited compared with WSL/macOS.

### Use devbox as the unified development environment

[devbox](https://www.jetify.com/docs/devbox/) is a Nix-backed shell/environment manager that uses `devbox.json` and `devbox.lock`.

- Good, because it provides strong reproducibility through the Nix store and a committed lockfile.
- Good, because it can install Go, Node.js, pnpm, and virtually any nixpkgs package.
- Good, because it is friendlier than raw Nix for contributors who do not know the Nix language.
- Bad, because it introduces the Nix ecosystem solely for development-tool installation, which is heavier than the current need.
- Bad, because Nix download sizes and store setup can be slow in CI and on contributor machines.
- Bad, because it conflicts less well with the Go-first, lightweight tooling philosophy of the project.

### Use a Nix flake / nix develop as the unified environment

Nix flakes provide a functional, declarative, fully reproducible development environment via `flake.nix` and `flake.lock`.

- Good, because it offers the strongest reproducibility guarantees of any option.
- Good, because it is excellent for supply-chain-sensitive projects.
- Bad, because the learning curve is steep and would exclude many contributors.
- Bad, because maintaining a `flake.nix` for a Go+Node project is more overhead than mise for the same benefit.
- Bad, because CI and local setup require Nix, which is not lightweight.

### Use devenv as the Nix-based developer environment

[devenv](https://devenv.sh/) builds declarative Nix environments with `devenv.nix`/`devenv.yaml` and `devenv.lock`.

- Good, because it abstracts Nix into a simpler developer-environment DSL.
- Good, because it supports languages, services, and tasks.
- Bad, because it still depends on Nix and the Nix store.
- Bad, because the project does not need services or complex environment composition today.

### Use Flox as the Nix-backed package/environment manager

[Flox](https://flox.dev/) provides a catalog-based, language-agnostic environment manager on top of Nix.

- Good, because it has a large catalog and can generate a lockfile.
- Good, because it lowers the Nix learning curve compared with raw flakes.
- Bad, because it adds a proprietary/platform layer and account dependency.
- Bad, because it is still Nix-backed and heavier than a version manager for this use case.

### Use asdf as the plugin-based runtime version manager

[asdf](https://asdf-vm.com/) is a mature, plugin-based runtime version manager using `.tool-versions`.

- Good, because it has a large plugin ecosystem and wide recognition.
- Good, because it supports Go, Node.js, and many CLI tools through plugins.
- Bad, because it has no native lockfile; reproducibility depends on the plugin and the upstream download.
- Bad, because plugins execute third-party shell code, increasing supply-chain exposure.
- Bad, because shim overhead and per-plugin setup are more friction than mise.

### Use proto (moonrepo) as the toolchain manager

[proto](https://moonrepo.dev/proto) is a Rust toolchain manager from the moonrepo project.

- Good, because it is fast, polyglot, and supports Go and Node.js well.
- Good, because it can leverage asdf plugins for additional tools.
- Bad, because its lockfile (`protolock`) is still experimental.
- Bad, because the ecosystem is newer and smaller than mise for standalone developer environments.

### Use Hermit to distribute a hermetic tool bundle

[Hermit](https://cashapp.github.io/hermit/) is a manifest-based package and environment manager from Block.

- Good, because it creates self-contained, hermetic tool bundles.
- Good, because it is written in Go and fits the project's language preference.
- Bad, because package manifests for Node.js/npm CLIs may require custom maintenance.
- Bad, because the ecosystem is smaller than mise/asdf, so some tools may not have first-class manifests.

### Use a Docker / devcontainer environment

[Dev containers](https://containers.dev/) provide a fully containerized development environment.

- Good, because it gives perfect CI/local parity.
- Good, because contributors only need Docker.
- Bad, because it is heavy for a Go+Node CLI project.
- Bad, because it does not provide a lockfile model for tool versions; reproducibility depends on pinned image digests.
- Bad, because it conflicts with the lightweight, native-toolchain contributor experience.

### Use a Taskfile (Task) plus manual installation instructions

[Task](https://taskfile.dev/) is a task runner, not an environment manager.

- Good, because it can document common commands (`task fmt`, `task lint`).
- Bad, because it does not install runtimes or tools; it would still require manual setup.
- Bad, because it does not improve reproducibility by itself.
- Bad, because it should be considered only after a runtime/bootstrap solution is in place.

### Use nvm/fnm for Node.js and gvm/manual installs for Go

A fragmented, language-native approach using nvm or fnm for Node.js and gvm or Go's official version wrappers for Go.

- Good, because it uses familiar, ecosystem-native tools.
- Bad, because it fragments version management across multiple files and tools.
- Bad, because there is no unified lockfile or shared update workflow.
- Bad, because gvm is community-maintained and does not match modern Go toolchain ergonomics.
- Bad, because it has the highest contributor friction and drift risk.

## More Information

This decision follows ADR 0004 (Go as primary language), ADR 0008 (dedicated formatters), ADR 0009 (Node.js as dev-only runtime), ADR 0010 (pnpm as package manager), and ADR 0011 (Lefthook for hooks). It does not supersede them.

### Tool-management boundary

mise installs and pins language runtimes and standalone CLI binaries. pnpm
continues to manage Node.js-based development dependencies such as Prettier and
`markdownlint-cli2`. This split preserves the intent of ADR 0009 and ADR 0010:

- **mise**: Go, Node.js, pnpm, and CLI binaries (`golangci-lint`, `shellcheck`,
  `shfmt`, `lefthook`, `actionlint`). These are single-file executables or
  language runtimes that benefit from a unified, lockfile-protected installer.
- **pnpm**: Prettier and `markdownlint-cli2` remain project-local development
  dependencies in `package.json`. They are tightly coupled to repository
  configuration files (`.prettierrc`, `.markdownlint-cli2.jsonc`) and benefit
  from pnpm's full dependency graph, `pnpm-lock.yaml`, Dependency Review, and
  OSV Scanner integration.

This boundary may be revisited if a future tool is available both as a binary
and as an npm package and one form is clearly superior for supply-chain or
contributor reasons. Any such move should be justified in a follow-up ADR or a
narrow implementation note.

Relevant references consulted:

- mise: <https://mise.jdx.dev/>, lockfile docs <https://mise.jdx.dev/dev-tools/mise-lock.html>, comparison to asdf <https://mise.jdx.dev/dev-tools/comparison-to-asdf.html>
- asdf: <https://asdf-vm.com/guide/getting-started.html>
- proto: <https://moonrepo.dev/proto>, config <https://moonrepo.dev/docs/proto/config>, pin command <https://moonrepo.dev/docs/proto/commands/pin>
- Hermit: <https://cashapp.github.io/hermit/usage/management/>, bundles <https://cashapp.github.io/hermit/usage/bundle/>
- devbox: <https://www.jetify.com/docs/devbox/>, quickstart <https://www.jetify.com/docs/devbox/quickstart/index.md>, pinning <https://www.jetify.com/docs/devbox/guides/pinning-packages/index.md>
- devenv: <https://devenv.sh/getting-started/>
- Flox: <https://flox.dev/docs/>
- Nix flakes: <https://nix.dev/concepts/flakes>
- Dev containers: <https://containers.dev/>
- Task: <https://taskfile.dev/>
