---
parent: Decisions
nav_order: 89
status: accepted
date: 12026-10-04
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0012
    scope:
      "the Node.js development-runtime version declaration implied by the Confirmation criterion
      that a root mise.toml pins Go, Node.js, pnpm, and required CLI tools: under this ADR the
      Node.js development-runtime version is declared solely in package.json's devEngines.runtime
      and mise resolves and installs Node.js through its idiomatic package.json resolution. The Go
      and CLI tool declarations in mise.toml, mise as the unified development-tool runtime, the
      aqua/ubi backends, and the lockfile policy all remain in force"
  - type: see-also
    target: ADR-0086
  - type: see-also
    target: ADR-0087
---

# Provision the Development Node.js Runtime Through mise package.json Field Resolution

## Context and Problem Statement

ADR 0086 moved the development pnpm's version declaration out of `mise.toml` into `package.json`
(`devEngines.packageManager`), establishing mise's idiomatic package.json resolution as the
provisioning mechanism and eliminating the Corepack dependency. The Node.js development runtime was
left declared in `mise.toml` (`node = "24"`), and `devEngines.runtime` was later added to
`package.json` so that pnpm-side script execution would also use a pinned runtime
([pnpm `devEngines.runtime`](https://pnpm.io/package_json#devenginesruntime)).

That leaves the runtime version declared in two provisioning systems whose resolutions can diverge.
Concretely, at the time of this decision `mise.lock` records Node.js 24.17.0 (from `mise.toml`'s
`"24"`) while `pnpm-lock.yaml` records runtime 24.21.0 (from `devEngines.runtime`'s `"24"`): a
`node` invoked through the mise shim and the same command executed through pnpm run different patch
versions. The hazard was surfaced by a local review finding and is the same dual-pin drift class ADR
0086 removed for pnpm.

The development-runtime **version** axis is closed and stays closed: Node.js 24 remains the
development runtime, and this ADR does not reopen it. The open axis here is the **declaration and
provisioning mechanism** for that runtime. The production builder toolchain is out of scope: ADR
0085's per-release Node.js patch pin and bundled-npm assertion are unaffected.

Relevant documented mechanics:

- mise reads "development runtime and package-manager declarations" from `package.json`
  (`devEngines.runtime` first, per the
  [mise Node.js guide](https://mise.jdx.dev/lang/node.html#package-json)); idiomatic fields are
  single version requests — "the version the project is built with" — and floors or ranges are not
  version requests
  ([mise configuration](https://mise.jdx.dev/configuration.html#idiomatic-version-files)).
- pnpm resolves `devEngines.runtime` to a concrete version, records it in `pnpm-lock.yaml`, and uses
  it for script execution (and, since pnpm v12.0.0-rc.2, for bare `node` inside the project).

## Decision Drivers

- One declaration of the development runtime version, eliminating the dual-pin drift class.
- Concrete, identical runtime version recorded in both lockfiles (`mise.lock` and `pnpm-lock.yaml`),
  so every invocation path executes the same Node.js.
- Preserve pnpm's runtime guarantee: scripts and postinstall hooks must execute under the pinned
  runtime even when invoked outside mise.
- Keep the change inside the closed version axis: the Node.js 24 major does not change.
- Follow the established ADR 0086 mechanism rather than inventing a parallel one.

## Considered Options

- Keep both declarations and pin them to the same version manually.
- Provision the runtime solely from `mise.toml` and remove `devEngines.runtime`.
- Provision the runtime from `package.json`'s `devEngines.runtime` as the sole declaration, resolved
  by mise through idiomatic package.json resolution.

## Decision Outcome

Chosen option: "Provision the runtime from `package.json`'s `devEngines.runtime` as the sole
declaration, resolved by mise through idiomatic package.json resolution", because it removes the
dual-pin drift class structurally while preserving pnpm's runtime guarantee, and because it reuses
the mechanism ADR 0086 already established for pnpm instead of creating a second topology.

Configuration consequences (implementation detail, recorded here for the implementer):

- `package.json`: `devEngines.runtime` is the sole declaration of the development Node.js runtime
  version and carries an exact version (selected through the normal update procedure: the newest
  24.x satisfying the repository cooldown at update time).
- `mise.toml`: the `node` entry under `[tools]` is removed, and
  `idiomatic_version_file_enable_tools` gains `"node"` alongside `"pnpm"`.
- `mise.lock` records the same runtime version as `pnpm-lock.yaml`; both are regenerated in the same
  change and on every runtime bump (`mise lock` and `pnpm install`).
- CI keeps running `MISE_LOCKED=1 mise install`; no workflow change is required.

### Consequences

- Good, because the runtime version is declared exactly once (`package.json`), making divergence
  between the two lockfiles structurally impossible rather than procedurally policed.
- Good, because every invocation path — the mise shim, `pnpm exec`, lifecycle scripts, and pnpm's
  bare-`node` handoff — executes the same recorded Node.js version.
- Good, because the mechanism is identical to the one ADR 0086 established for pnpm, so contributors
  learn one provisioning model for both Node.js and pnpm.
- Good, because pnpm's runtime guarantee is preserved: scripts invoked outside mise still run under
  the pinned runtime.
- Neutral, because a runtime bump now edits `package.json` instead of `mise.toml`; the two-lockfile
  regeneration procedure is already documented.
- Bad, because `mise.toml` no longer self-documents the runtime version; a contributor must look at
  `package.json`, the same trade-off ADR 0086 accepted for pnpm.

### Confirmation

This decision is confirmed when:

- `mise.toml` contains no `node` entry under `[tools]`, and `idiomatic_version_file_enable_tools`
  contains `"node"`;
- `package.json`'s `devEngines.runtime` carries an exact Node.js 24.x version and is the only
  declaration of the development runtime version;
- `mise.lock` and `pnpm-lock.yaml` record the same runtime version;
- `mise exec -- node --version` and a pnpm-executed `node --version` report the same version;
- the bootstrap documentation (both languages) describes the runtime as declared in `package.json`
  and provisioned by mise.

## Pros and Cons of the Options

### Keep both declarations and pin them to the same version manually

- Good, because no provisioning topology changes.
- Bad, because the drift hazard is only suppressed, not removed: the next runtime bump must edit two
  declarations, and a missed one silently reintroduces divergent lockfiles.

### Provision the runtime solely from `mise.toml` and remove `devEngines.runtime`

- Good, because a single declaration remains.
- Bad, because pnpm's runtime guarantee is lost: scripts and postinstall hooks invoked outside mise
  would run under whatever Node.js is ambient, weakening exactly the determinism this repository's
  bootstrap exists to provide.

### Provision the runtime from `package.json` as the sole declaration

- Good, because the drift class is removed structurally and the pnpm-side guarantee is kept.
- Good, because it reuses the ADR 0086 mechanism, which mise and pnpm both document as the intended
  joint design.
- Bad, because the version no longer lives beside the other tool pins in `mise.toml`.

## More Information

- Local review finding that surfaced the hazard: `mise.lock` recorded Node.js 24.17.0 while
  `pnpm-lock.yaml` recorded runtime 24.21.0 from the same fuzzy `"24"` declarations.
- [pnpm `devEngines.runtime`](https://pnpm.io/package_json#devenginesruntime) — resolution, lockfile
  recording, and bare-`node` behavior since v12.0.0-rc.2.
- [mise idiomatic version files](https://mise.jdx.dev/configuration.html#idiomatic-version-files) —
  idiomatic fields are single version requests; floors and ranges are not version requests.
