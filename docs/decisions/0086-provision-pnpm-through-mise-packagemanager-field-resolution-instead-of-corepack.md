---
parent: Decisions
nav_order: 86
status: accepted
date: 12026-10-04
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0012
    scope:
      'the pnpm provisioning mechanism implied by the Decision Outcome''s declaration that a root
      mise.toml pins the pnpm version (''pnpm version (matching package.json)'') and by the
      Confirmation criterion that mise.toml pins pnpm: under this ADR the pnpm version pin lives
      solely in package.json''s devEngines.packageManager and mise resolves and installs pnpm
      through its packageManager-field resolution (idiomatic version file) mode; the
      Corepack-mediated installation used at ADR 0012 implementation time (the node tool''s
      ''postinstall = "corepack enable"'' in mise.toml) is withdrawn. mise as the unified
      development-tool runtime, the mise/pnpm tool-management boundary, the aqua/ubi CLI tool
      backends, and the lockfile policy all remain in force'
  - type: amends
    target: ADR-0010
    scope:
      "the pinned installation path clause ('pinned through the packageManager field in
      package.json, with Corepack or an equivalent pinned installation path used in CI'): this ADR
      designates mise's packageManager-field resolution as the equivalent pinned installation path
      for both local development and CI, and withdraws Corepack from that role. The choice of pnpm
      itself and every other ADR 0010 clause remain in force"
  - type: see-also
    target: ADR-0009
  - type: see-also
    target: ADR-0087
  - type: see-also
    target: ADR-0088
---

# Provision pnpm Through mise packageManager-Field Resolution Instead of Corepack

## Context and Problem Statement

ADR 0009 chose Node.js as the development-only JavaScript runtime. ADR 0010 chose pnpm for Node.js
development tooling and required its version to be "pinned through the `packageManager` field in
`package.json`, with Corepack or an equivalent pinned installation path used in CI". ADR 0012
adopted mise as the unified development-tool runtime and bootstrap. As implemented, `mise.toml`
provisions Node.js with `postinstall = "corepack enable"`, `package.json` declares the pin via
`devEngines.packageManager` (`onFail: download`), and Corepack performs the actual pnpm
provisioning. ADR 0012 itself never names Corepack — the Corepack-mediated installation was an
implementation-time choice under the ADR's "pnpm version (matching `package.json`)" declaration —
and the conditions around that choice have shifted:

1. **Node.js is unbundling Corepack — a pre-existing decision whose weight has grown.** The Node.js
   TSC voted on 2025-03-19 to stop distributing Corepack in Node.js 25 and later
   ([nodejs/corepack#688](https://github.com/nodejs/corepack/issues/688)); Node.js 24.x keeps it
   only as an experimental feature. This vote predates ADR 0012, which records no consideration of
   it either way. Whether it went unconsidered or was accepted as a tolerable limitation at the
   time, the risk it carries is materially larger today — any future development-runtime bump breaks
   the `postinstall = "corepack enable"` hook — particularly in combination with the next change.
2. **The pnpm project removed Corepack from its recommended paths (after ADR 0012).** The
   installation documentation's "Using Corepack" section was deleted
   ([pnpm/pnpm.io@`83fbee83`](https://github.com/pnpm/pnpm.io/commit/83fbee83), 2026-08-09), and the
   CLI itself stopped pointing users at Corepack for updates
   ([pnpm/pnpm#14115](https://github.com/pnpm/pnpm/pull/14115), merged 2026-08-24: the update
   notification no longer prints `corepack use pnpm@<version>`, and `pnpm self-update` under
   Corepack now names the standalone install script instead). pnpm 12's move to native-executable
   packaging initially broke Corepack dispatch — Corepack hardcodes the `bin/pnpm.mjs` entry point
   and neither installs optional dependencies nor runs lifecycle scripts
   ([pnpm/pnpm#13018](https://github.com/pnpm/pnpm/issues/13018)) — but pnpm restored compatibility
   on its own side: since 12.0.0-rc.6 the package ships `bin/pnpm.mjs` shims that download the
   pinned native binary on first use ([pnpm/pnpm#13922](https://github.com/pnpm/pnpm/pull/13922),
   merged 2026-08-15), while the Corepack-side fix
   ([nodejs/corepack#887](https://github.com/nodejs/corepack/pull/887)) was closed unmerged.
   Corepack dispatch of pnpm 12 therefore works today, but only through a shim the pnpm project
   itself does not recommend.
3. **mise closed the gap itself (after ADR 0012).** Since v2026.8.7, mise reads
   `devEngines.packageManager` (then the top-level `packageManager` field) as a version source for
   npm, pnpm, and Yarn when the idiomatic version file is enabled
   (`idiomatic_version_file_enable_tools`); since v2026.8.11 it also honors `+sha...` checksum
   suffixes in those fields, acting as a Corepack replacement. The aqua registry verifies GitHub
   artifact attestations for pnpm releases (signer workflow
   `pnpm/pnpm/.github/workflows/release.yml`), and this repository already commits `mise.lock` and
   sets `locked_verify_provenance = true`.

Independently, pnpm itself now enforces its own pin: `devEngines.packageManager` with
`onFail: download` makes pnpm download the pinned version when the running one mismatches, and the
resolution is recorded in `pnpm-lock.yaml` (`packageManagerDependencies`). Exact-version enforcement
therefore no longer needs Corepack at all. What the bootstrap must still provide is a trustworthy
first pnpm binary on a fresh machine, with `package.json` as the single declaration of the intended
version.

Removing Corepack also lifts a constraint that shaped the current `package.json`: Corepack reads
only the top-level `packageManager` field and accepts only an exact version (optionally with a
`+sha...` checksum), which is why the manifest pin names an exact patch version today. pnpm itself
has always supported version ranges in `devEngines.packageManager`, recording the resolved version
in `pnpm-lock.yaml` (`packageManagerDependencies`) and reusing it while it satisfies the range
([pnpm documentation](https://pnpm.io/package_json#devenginespackagemanager)). With Corepack
withdrawn, the declaration grammar — an exact pin, a major-only pin, or a range with the concrete
resolution living in the lockfile — becomes an implementation-level choice rather than a
Corepack-mandated form.

Which mechanism should provision the pinned pnpm for local development and CI?

## Decision Drivers

- **Single source of truth.** The pnpm version must be declared exactly once, in `package.json`; a
  second pin in `mise.toml` is a drift hazard.
- **Trusted first binary.** Provisioning must be checksum- and attestation-verified and recorded in
  the committed `mise.lock`, per the repository's lockfile and provenance re-verification policy.
- **No dead-man's dependency.** The mechanism must not rely on a component that Node.js is
  unbundling and that pnpm has removed from its recommended install and update paths.
- **Local/CI unity.** One bootstrap mechanism (mise) covers both local development and CI, per ADR
  0012; introducing a CI-only provisioning path would require its own prior decision.
- **Forward compatibility.** The mechanism must work for pnpm 11 today and for pnpm 12 if a
  follow-on ADR adopts it.
- **Unchanged contributor friction.** `mise install && pnpm install` must keep working from a fresh
  clone.

## Considered Options

- Pin the pnpm version directly in `mise.toml` (`[tools] pnpm = "x.y.z"`, aqua backend).
- Use mise's packageManager-field resolution mode (idiomatic version file), with `package.json` as
  the sole pin source.
- Keep Corepack (bundled with Node.js 24, or installed via `npm install --global corepack`).
- Use pnpm's standalone install script (`get.pnpm.io/install.sh`).
- Use `npx get-pnpm` / `npm install --global pnpm`.

## Decision Outcome

Chosen option: "Use mise's packageManager-field resolution mode", because it keeps a single pin
source in `package.json` while preserving mise's verified-download and committed-lockfile
guarantees, and because it removes the Corepack dependency without waiting on any other decision.

Configuration consequences (implementation detail, recorded here for the implementer):

- `mise.toml`: remove `postinstall = "corepack enable"` from the `node` tool; add
  `idiomatic_version_file_enable_tools = ["pnpm"]` under `[settings]`; keep
  `[settings.npm] package_manager = "pnpm"` (it governs mise's `npm:` backend behavior); declare no
  `pnpm` entry under `[tools]`.
- `package.json`: `devEngines.packageManager` remains the sole declaration of the pnpm version. This
  ADR does not change its value.
- `mise.lock` records the resolved pnpm with checksum and provenance metadata;
  `locked_verify_provenance = true` re-verifies at install time.
- CI keeps running `MISE_LOCKED=1 mise install`; no workflow change is required.
- pnpm's own `devEngines.packageManager` / `onFail: download` self-provisioning remains as a
  complementary safety net inside the project, not as the primary provisioning path.

### Consequences

- Good, because the pnpm version is declared exactly once (`package.json`), eliminating the
  `mise.toml` ↔ `package.json` pin-drift hazard.
- Good, because Corepack's exact-version-only field constraint disappears with it: the manifest is
  no longer forced to name an exact patch version, so the declaration grammar (exact pin versus a
  range whose concrete resolution lives in `pnpm-lock.yaml`) is free for the implementation to
  choose per update.
- Good, because Corepack is fully removed from the bootstrap; a future Node.js 26 development
  runtime is no longer blocked by the `corepack enable` postinstall.
- Good, because the first binary is verified through the aqua registry's GitHub artifact attestation
  check plus the committed `mise.lock` checksum and `locked_verify_provenance` re-verification — a
  stronger chain than Corepack's signature flow, which has itself shipped outdated-signature
  failures ([nodejs/corepack#612](https://github.com/nodejs/corepack/issues/612)).
- Good, because the mechanism is version-agnostic across pnpm 11 and pnpm 12, so the pnpm-major
  decision can proceed independently.
- Good, because local development and CI keep one identical bootstrap path.
- Neutral, because idiomatic version files are opt-in in mise; the repository enables the feature in
  its own `mise.toml`, but a contributor's global mise configuration could in principle interfere —
  mitigated by the committed repository-local settings.
- Bad, because the feature requires a recent mise (v2026.8.7 or newer); older mise versions will not
  provision pnpm from `package.json`, so bootstrap documentation must state the mise version floor.
- Bad, because `mise.toml` no longer self-documents the pnpm version; a contributor must look at
  `package.json` to see it.

### Confirmation

This decision is confirmed when:

- `mise.toml` contains no pnpm version pin and no `corepack` postinstall, and
  `[settings] idiomatic_version_file_enable_tools` includes `pnpm`.
- `package.json`'s `devEngines.packageManager` is the only place the pnpm version is declared.
- `mise install` on a fresh clone provisions the pnpm version declared in `package.json` (with
  `mise ls` showing the version source as `package.json`) without any Corepack invocation.
- `mise.lock` records pnpm with checksum and provenance metadata, and `MISE_LOCKED=1 mise install`
  succeeds in CI.
- `pnpm --version` after bootstrap matches the `package.json` pin, and `pnpm install` succeeds.
- No `corepack` invocation remains in repository configuration or documentation.

## Pros and Cons of the Options

### Pin the pnpm version directly in `mise.toml`

Declare `pnpm = "x.y.z"` under `[tools]`, installed from the `aqua:pnpm/pnpm` backend.

- Good, because the aqua backend verifies GitHub artifact attestations for pnpm releases and
  `mise.lock` records the checksum.
- Good, because the pin is explicit and self-documenting in `mise.toml`.
- Good, because it needs no idiomatic-version-file opt-in and works on older mise versions.
- Bad, because the pnpm version is then declared twice (`mise.toml` and
  `package.json`/`devEngines.packageManager`), and the two can drift.
- Bad, because `package.json` remains the field pnpm itself reads, so the `mise.toml` pin is
  redundant for in-project enforcement while adding maintenance surface.
- Neutral, because it remains an acceptable fallback if the field-resolution mode misbehaves.

### Use mise's packageManager-field resolution mode

Enable `idiomatic_version_file_enable_tools = ["pnpm"]` and let mise resolve and install pnpm from
`devEngines.packageManager` (falling back to the top-level `packageManager` field).

- Good, because `package.json` is the single source of truth for the pnpm version.
- Good, because download integrity keeps the aqua attestation + `mise.lock` checksum +
  `locked_verify_provenance` chain.
- Good, because mise additionally honors `+sha...` checksum suffixes in the field, matching and
  exceeding Corepack's verification model.
- Good, because it removes Corepack without coupling to the pnpm-major or Node-version decisions.
- Neutral, because the feature is opt-in and must be enabled in the committed `mise.toml`.
- Bad, because it requires mise v2026.8.7 or newer.
- Bad, because the indirection hides the pnpm version from `mise.toml` readers.

### Keep Corepack

Retain `postinstall = "corepack enable"` (Node.js 24 bundled Corepack, or
`npm install --global corepack` where unbundled).

- Good, because it requires no change today on Node.js 24.
- Bad, because Node.js 25+ no longer distributes Corepack, so the mechanism has a known expiry.
- Bad, because the pnpm project has removed Corepack from its recommended install and update paths —
  pnpm 12 runs under Corepack again, but only via a first-use-download shim that is not a supported
  acquisition path — so this option would keep the bootstrap on a trajectory the upstream project is
  actively abandoning.
- Bad, because Corepack's Known Good Release fallback and signature-verification history
  (outdated-signature incidents) keep a verification surface this project otherwise closes.

### Use pnpm's standalone install script

Install via `curl -fsSL https://get.pnpm.io/install.sh | sh -` with `PNPM_VERSION` pinned.

- Good, because it is pnpm's own first-choice installation path and needs no Node.js.
- Good, because it works in minimal environments (for example, slim containers) where mise is
  absent.
- Bad, because the initial fetch's integrity rests on the HTTPS channel alone, outside the committed
  `mise.lock` and attestation re-verification policy.
- Bad, because it bypasses the repository's single-bootstrap model and pins the version through a
  third location (an environment variable).

### Use `npx get-pnpm` / `npm install --global pnpm`

Install pnpm through npm (registry `dist.integrity` and npm signatures apply).

- Good, because it uses npm-registry integrity and signature metadata.
- Good, because pnpm documents it as the recommended path on Windows.
- Bad, because it requires Node.js first and then manages pnpm outside `mise.lock`, splitting the
  tool inventory.
- Bad, because a global npm installation is not version-pinned by the repository and can drift
  independently of `package.json`.

## More Information

Deferred to follow-on decisions:

- **pnpm major adoption (11 versus 12)** — a follow-on ADR, informed by Dependabot's pnpm 12 support
  status and a smoke test of this repository's Dependabot workflow.
- **Development Node.js version (24 versus 26)** — a follow-on decision; removing Corepack here
  eliminates the only hard blocker for Node.js 26.
- **Concrete pin values** — the pnpm version in `devEngines.packageManager` is selected at
  implementation time through the normal update procedure, not by an ADR.

Field-naming note: the repository declares the pin in `devEngines.packageManager` (supported since
pnpm v11.0.0; accepts ranges and `onFail`), which mise reads preferentially before the top-level
`packageManager` field. Corepack reads only the top-level field. ADR 0010's wording
"`packageManager` field" is therefore read as covering the `devEngines.packageManager` form in force
in this repository.

References consulted:

- Node.js Corepack unbundling vote: <https://github.com/nodejs/corepack/issues/688>
- pnpm installation documentation history (Corepack section removal):
  <https://github.com/pnpm/pnpm.io/commit/83fbee83>
- pnpm CLI stops pointing users at Corepack: <https://github.com/pnpm/pnpm/pull/14115>
- pnpm 12.0 release notes: <https://pnpm.io/blog/releases/12.0>
- pnpm `devEngines.packageManager` documentation:
  <https://pnpm.io/package_json#devenginespackagemanager>
- mise Node.js backend documentation: <https://mise.jdx.dev/lang/node.html>
- mise cookbook replacing the Corepack recipe: <https://github.com/jdx/mise/pull/12213>
- mise changelog (v2026.8.7, v2026.8.11): <https://github.com/jdx/mise/blob/main/CHANGELOG.md>
- aqua registry pnpm entry (GitHub artifact attestations):
  <https://github.com/aquaproj/aqua-registry/blob/main/pkgs/pnpm/pnpm/registry.yaml>
