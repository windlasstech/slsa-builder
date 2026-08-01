---
parent: Decisions
nav_order: 63
status: accepted
date: 12026-07-31
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0014
    scope:
      "Yarn support is narrowed to Yarn Berry v4 or newer executed through Corepack packageManager
      enforcement; the decision to support Yarn remains in force"
  - type: partially-supersedes
    target: ADR-0015
    scope:
      "lockfile-based Yarn inference fallback; Yarn requires an explicit packageManager declaration"
---

# Limit Yarn Support to Berry v4 with Corepack Package Manager Metadata

## Context and Problem Statement

ADR 0014 decided that the initial JS/TS npm package profile supports Yarn for install, build, and
pack stages. ADR 0015 selected package managers from manifest metadata before lockfile inference.
ADR 0016 decided to use Corepack for Yarn execution, and ADR 0017 required exact package-manager
version enforcement for Yarn release builds.

Those decisions intentionally left the supported Yarn generation and installation model open. Yarn
Classic 1.x and Yarn Berry 2, 3, and 4 differ in maintenance status, CLI behavior, configuration
files, lockfile semantics, Corepack integration, plugin availability, and default install strategy.
The profile needs one stable Yarn support boundary before the architecture specification can define
commands, fixtures, provenance parameters, and verifier policy.

Which Yarn generation should the initial JS/TS npm package profile support?

## Decision Drivers

- Keep the Yarn support surface deterministic enough for SLSA provenance and verification.
- Avoid implementing separate behavior for Yarn Classic and multiple Berry major versions in the
  initial profile.
- Prefer the actively maintained Yarn line over the frozen Yarn Classic line.
- Align Yarn execution with Corepack and top-level `packageManager` metadata.
- Require repository-declared package-manager intent rather than lockfile-only Yarn inference.
- Keep future support for older Yarn generations possible through explicit follow-up decisions.

## Considered Options

- Support only Yarn Berry v4 or newer through Corepack and top-level `packageManager` metadata.
- Support Yarn Berry v3 or newer through Corepack.
- Support all Yarn Berry versions.
- Support Yarn Classic and Yarn Berry.
- Do not support Yarn.

## Decision Outcome

Chosen option: "Support only Yarn Berry v4 or newer through Corepack and top-level `packageManager`
metadata", because it keeps Yarn support aligned with the current maintained Yarn model while
avoiding separate Classic, Berry v2, and Berry v3 compatibility branches in the initial profile.

For the initial JS/TS npm package profile, Yarn is a supported package manager only when all of the
following are true:

1. The target package's top-level `package.json` contains `packageManager` with an exact Yarn
   version.
2. The declared version is Yarn Berry v4 or newer, for example `yarn@4.0.0` or a later exact
   version.
3. Corepack can prepare and dispatch that exact Yarn version without falling back to a Known Good
   Release or an unrelated global Yarn installation.
4. The package manager root contains the selected Yarn lockfile and the release install uses Yarn's
   immutable or otherwise specified reproducible install mode.

Yarn Classic 1.x is not supported by the stable initial profile. Yarn Berry v2 and v3 are also not
supported by the stable initial profile. Lockfile-only Yarn inference is not sufficient for stable
Yarn support, because it cannot distinguish the supported Yarn generation and exact release
package-manager version without manifest metadata.

This decision narrows ADR 0014's Yarn support and ADR 0015's lockfile inference fallback for the
Yarn path. It does not change npm or pnpm support. It also does not decide whether Yarn Berry should
run with Plug'n'Play, `nodeLinker: node-modules`, or another linker; those observable install-mode
requirements belong in the JS/TS npm build and pack architecture specification.

### Consequences

- Good, because the initial stable Yarn path targets the currently maintained Yarn generation.
- Good, because Corepack enforcement is tied to the standard top-level `packageManager` field.
- Good, because verifiers can see the exact Yarn version used for install, build, and pack.
- Good, because the implementation avoids separate command and plugin behavior for Yarn Classic,
  Berry v2, and Berry v3.
- Good, because projects using unsupported Yarn generations fail before producing ambiguous release
  provenance.
- Neutral, because projects using Yarn Berry v3 must upgrade or use npm/pnpm before they can use the
  stable Yarn path.
- Bad, because Yarn Classic projects remain unsupported despite their continued presence in the
  ecosystem.
- Bad, because downstream projects must add or maintain exact `packageManager` metadata even when
  they already commit `yarn.lock`.

### Confirmation

This decision is confirmed when the JS/TS npm package profile architecture specification defines:

- Yarn support as Yarn Berry v4 or newer only;
- top-level `packageManager` with an exact `yarn@<version>` value as mandatory for Yarn selection;
- failure behavior for Yarn Classic, Yarn Berry v2 or v3, Yarn version ranges, missing
  `packageManager`, and lockfile-only Yarn inference;
- Corepack strict execution for the selected exact Yarn version;
- the required Yarn lockfile and immutable or reproducible install command semantics;
- provenance/log fields for selected package manager, exact Yarn version, selection source, and Yarn
  install-mode-relevant settings.

Implementation review should verify that Yarn release builds never proceed from an ambient global
Yarn, Corepack fallback, `devEngines.packageManager` alone, a version range, or a `yarn.lock`
without top-level exact `packageManager` metadata.

## Pros and Cons of the Options

### Support only Yarn Berry v4+ through Corepack and `packageManager`

Require exact top-level `packageManager` metadata selecting Yarn Berry v4 or newer, and execute that
version through Corepack.

- Good, because it matches the modern Yarn installation recommendation based on Corepack and
  `packageManager`.
- Good, because Yarn v4 requires a modern Node.js baseline, which aligns with the profile's Node.js
  24 runtime decision.
- Good, because v4 includes official plugins by default, reducing version-specific plugin setup
  ambiguity for npm-oriented commands.
- Good, because exact manifest metadata is the strongest Yarn selection signal for release builds.
- Bad, because it excludes existing Yarn Classic and Berry v2/v3 projects from the stable Yarn path.

### Support Yarn Berry v3+

Support Yarn Berry v3 and newer through Corepack.

- Good, because it includes more existing modern Yarn projects.
- Good, because v3 already uses the modern Berry architecture, `.yarnrc.yml`, `.pnp.cjs`, and
  Corepack-compatible metadata.
- Bad, because v3 and v4 still differ in Node.js requirements, plugin availability, Corepack default
  guidance, and some CLI behavior.
- Bad, because the initial profile would need extra fixtures and specification branches for a Yarn
  generation that is no longer the current major.

### Support all Yarn Berry versions

Support Yarn Berry v2, v3, and v4 or newer.

- Good, because it recognizes that all Berry versions share the modern Yarn architecture.
- Good, because it maximizes compatibility for projects that already migrated away from Classic.
- Bad, because v2 introduced the largest breaking changes and has more migration-era compatibility
  caveats.
- Bad, because command behavior, plugin handling, PnP defaults, and Node.js compatibility differ
  across Berry majors.
- Bad, because testing all Berry majors increases the initial support matrix before the npm package
  profile is implemented.

### Support Yarn Classic and Yarn Berry

Support every active-in-the-wild Yarn generation under one Yarn path.

- Good, because it maximizes Yarn ecosystem coverage.
- Good, because Yarn Classic remains common in older npm package repositories.
- Bad, because Yarn Classic is frozen and mostly kept for historical and migration purposes.
- Bad, because Classic uses different configuration and command behavior from Berry.
- Bad, because a single "Yarn" support claim would hide materially different install, lockfile, and
  provenance behavior.

### Do not support Yarn

Remove Yarn from the initial stable package-manager support set.

- Good, because it would make the package-manager matrix smaller.
- Good, because npm and pnpm already cover a large share of npm package workflows.
- Bad, because it would reverse ADR 0014's decision to include Yarn among mainstream npm package
  development workflows.
- Bad, because Yarn Berry v4 has a clear Corepack-based enforcement model suitable for trusted
  release builds.

## More Information

This decision follows ADR 0014, ADR 0015, ADR 0016, ADR 0017, and ADR 0056. It narrows Yarn support
without changing npm or pnpm behavior.

Reference points considered:

- Yarn Classic 1.x is frozen; the Classic repository directs new features and ordinary bugfixes to
  the Berry repository and recommends migration for most issues.
- Yarn Modern documentation recommends upgrading from Classic where possible and documents Modern
  support for multiple install strategies through `nodeLinker`.
- Yarn v4 recommends Corepack and top-level `packageManager` metadata, requires Node.js 18 or newer,
  includes official plugins by default, and adds supply-chain-oriented hardened install checks.
- Yarn PnP is the default modern installation strategy, but Yarn Modern can also use
  `nodeLinker: node-modules` or a pnpm-style linker. The initial support-scope decision should not
  silently choose among those linker modes; the architecture specification must record the selected
  behavior.
