---
parent: Decisions
nav_order: 17
status: accepted
date: 12026-06-28
decision-makers: Yunseo Kim
---

# Require Explicit Package Manager Version Enforcement

## Context and Problem Statement

ADR 0014 decided that the initial JS/TS npm package profile supports npm, pnpm, and Yarn for
install, build, and pack stages. ADR 0015 defined the package-manager selection precedence:
`packageManager`, then `devEngines.packageManager`, then lockfile inference, with conflict failure.
ADR 0016 decided that pnpm and Yarn use Corepack while npm uses the npm CLI from the selected
Node.js toolchain or runner.

Those decisions define what package manager is selected and how the workflow obtains pnpm and Yarn.
They do not fully define version enforcement policy: exact versions versus ranges, Corepack fallback
behavior, npm version handling, and what should be recorded for verification.

How should the initial JS/TS npm package profile enforce selected package-manager versions during
release builds?

## Decision Drivers

- Keep release builds deterministic and auditable.
- Avoid silently resolving mutable package-manager ranges during trusted release builds.
- Prevent Corepack Known Good Release fallback from choosing a package-manager version not declared
  by the target package.
- Preserve compatibility with npm, whose CLI is tied to the selected Node.js toolchain in this
  profile.
- Keep pnpm and Yarn version enforcement strict enough for SLSA provenance and verification.
- Record the package manager and version used so downstream users can understand the build
  environment.

## Considered Options

- Require explicit exact package-manager versions for pnpm and Yarn, disallow release-time fallback,
  and record npm's actual version.
- Allow package-manager version ranges and trust Corepack or package-manager resolution.
- Allow Corepack Known Good Release fallback when no package-manager version is declared.
- Require exact package-manager versions for npm, pnpm, and Yarn.
- Treat package-manager versions as non-verifier-relevant implementation detail.

## Decision Outcome

Chosen option: "Require explicit exact package-manager versions for pnpm and Yarn, disallow
release-time fallback, and record npm's actual version", because pnpm and Yarn can be
version-dispatched through Corepack while npm is already bound to the selected Node.js toolchain
used for `npm publish`.

For pnpm and Yarn release builds, the initial JS/TS npm package profile should require an exact
package-manager version from the selected manifest source. The preferred source is top-level
`packageManager`, consistent with ADR 0015. If `packageManager` is absent and
`devEngines.packageManager` is used, it must resolve to an exact version suitable for strict release
execution. Package-manager ranges should not be resolved during release builds unless a later ADR
defines a lockfile-backed resolution policy.

Corepack Known Good Release fallback should not be used for release builds. If pnpm or Yarn is
selected and no explicit version can be enforced, the release build should fail.

For npm release builds, the profile should use the npm CLI from the selected Node.js toolchain or
runner as decided in ADR 0016. The profile should record the actual npm version used, but it should
not require a separate npm package-manager pin in the target package for the initial profile.
Node.js version selection and enforcement should be decided separately if needed.

The selected package manager, package-manager version, and selection source should be logged and
recorded in verifier-relevant provenance parameters where appropriate.

### Consequences

- Good, because pnpm and Yarn release builds use explicit versions rather than mutable ranges or
  ambient global installs.
- Good, because Corepack cannot silently choose a Known Good Release when the package has not
  declared a release package-manager version.
- Good, because npm remains operationally simple and aligned with the final `npm publish` path.
- Good, because provenance can report the exact package manager and version used for install, build,
  and pack.
- Neutral, because npm version determinism depends on the selected Node.js toolchain instead of a
  separate npm pin.
- Bad, because pnpm or Yarn packages that only declare version ranges must add exact release
  metadata before using the stable profile.
- Bad, because projects relying on Corepack fallback or package-manager auto-resolution must make
  their release intent explicit.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification
defines:

- exact package-manager version requirements for pnpm and Yarn release builds;
- failure behavior when pnpm or Yarn is selected without an enforceable exact version;
- release-build prohibition of Corepack Known Good Release fallback;
- npm version capture from the selected Node.js toolchain or runner;
- provenance/log fields for package manager name, version, and selection source;
- a separate decision boundary for Node.js version enforcement and any future npm package-manager
  pinning.

Implementation review should verify that pnpm and Yarn release builds never proceed with ambient
global versions, unpinned Corepack fallback, or unresolved version ranges.

## Pros and Cons of the Options

### Require exact pnpm and Yarn versions, disallow fallback, record npm

Require exact pnpm and Yarn versions for release builds, enforce them through Corepack, and record
the actual npm version when npm is selected.

- Good, because it matches the Corepack enforcement path chosen for pnpm and Yarn.
- Good, because exact release versions are easier to audit, reproduce, and record in provenance.
- Good, because release behavior does not depend on mutable range resolution or Corepack fallback
  defaults.
- Good, because npm remains tied to the selected Node.js toolchain and final `npm publish` path.
- Neutral, because this creates asymmetric handling between npm and pnpm/Yarn.
- Bad, because pnpm and Yarn users must maintain exact package-manager metadata.

### Allow package-manager version ranges

Allow ranges such as `pnpm@^10` or equivalent `devEngines.packageManager.version` ranges and resolve
them during release builds.

- Good, because it reduces maintenance for downstream repositories.
- Good, because pnpm has package-manager range handling semantics.
- Bad, because release builds could use different package-manager versions over time without a
  manifest change.
- Bad, because the resolved version source and cache behavior become verifier-relevant and more
  complex to specify.
- Bad, because range resolution weakens deterministic release behavior.

### Allow Corepack Known Good Release fallback

Let Corepack select a Known Good Release when no project package-manager version is declared.

- Good, because simple packages can release without package-manager metadata.
- Good, because Corepack provides a supported fallback path for missing project specifications.
- Bad, because the trusted release build would depend on Corepack's ambient fallback set rather than
  repository-declared intent.
- Bad, because downstream verification cannot rely on package metadata to explain why that
  package-manager version was used.
- Bad, because fallback makes release builds less reproducible across time and environments.

### Require exact versions for npm, pnpm, and Yarn

Require every selected package manager, including npm, to be declared with an exact version in
package metadata.

- Good, because it creates a symmetric version-enforcement policy.
- Good, because npm version would be explicitly declared by the package.
- Bad, because npm is already coupled to the selected Node.js toolchain in common GitHub runner
  usage.
- Bad, because Corepack npm shims are not the selected enforcement path from ADR 0016.
- Bad, because this creates more adoption friction than the initial profile needs.

### Treat package-manager versions as implementation detail

Select npm, pnpm, or Yarn, but do not require or record the exact version.

- Good, because it minimizes specification and implementation complexity.
- Good, because it is easy for legacy packages to adopt.
- Bad, because package-manager behavior can affect dependency installation, lifecycle scripts,
  workspace handling, and tarball contents.
- Bad, because missing package-manager versions reduce release reproducibility.
- Bad, because SLSA-oriented provenance should expose verifier-relevant build environment choices.

## More Information

This decision complements ADR 0015 and ADR 0016. ADR 0015 selects the package manager. ADR 0016
defines the enforcement mechanism for pnpm, Yarn, and npm. This ADR defines the release-build
version policy.

Future decisions may refine:

- Node.js version selection and enforcement;
- whether npm should ever be separately pinned from Node.js;
- whether pnpm `devEngines.packageManager` ranges can be accepted when a resolved version is locked
  and verifier-visible;
- whether non-release validation builds may allow looser package-manager version behavior than
  release builds.
