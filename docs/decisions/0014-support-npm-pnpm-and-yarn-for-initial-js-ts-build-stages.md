---
parent: Decisions
nav_order: 14
status: accepted
date: 12026-06-28
decision-makers: Yunseo Kim
---

# Support npm, pnpm, and Yarn for Initial JS/TS Build Stages

## Context and Problem Statement

ADR 0013 scoped the initial JS/TS profile to npm registry package releases. That decision made npm
package identity the first release artifact model and chose `npm publish` as the initial
provenance-aware publish mechanism. It also left package-manager behavior for install, build, and
pack stages to a later decision.

The initial JS/TS npm package profile must now decide which package managers it officially supports
before publish. This decision concerns only install, build, and pack stages. The criteria used to
select a package manager for a given repository, such as `devEngines.packageManager`,
`packageManager`, lockfiles, or explicit workflow inputs, are intentionally out of scope and should
be decided in a follow-up ADR.

Which package managers should the initial JS/TS npm package profile officially support for install,
build, and pack: npm, pnpm, Yarn, Bun, and/or Deno?

## Decision Drivers

- Respect common JS/TS package development workflows while keeping the first profile auditable.
- Support npm package releases without requiring all projects to use npm for install, build, or
  pack.
- Include package managers with mature lockfile and frozen or immutable install behavior.
- Include package managers with npm-package-oriented pack behavior suitable for producing the
  tarball passed to `npm publish`.
- Avoid expanding the initial npm package profile into non-npm package ecosystems or
  application-deployment workflows.
- Keep Bun and Deno paths available for future explicit decisions without treating them as
  first-class initial scope.
- Keep package-manager detection and precedence rules separate from this support-scope decision.

## Considered Options

- Support npm only for install, build, and pack.
- Support npm and pnpm for install, build, and pack.
- Support npm, pnpm, and Yarn for install, build, and pack.
- Support npm, pnpm, Yarn, and Bun for install, build, and pack.
- Support npm, pnpm, Yarn, Bun, and Deno for install, build, and pack.

## Decision Outcome

Chosen option: "Support npm, pnpm, and Yarn for install, build, and pack", because these package
managers cover the mainstream npm package development workflows while keeping the initial profile
focused on npm-compatible package releases.

The initial JS/TS npm package profile should officially support:

- npm for install, build, and pack;
- pnpm for install, build, and pack;
- Yarn for install, build, and pack.

The initial profile should not officially support Bun as part of the stable first scope. Bun may be
added later as an experimental or opt-in adapter, or as an officially supported package manager
through a follow-up ADR once its setup, version pinning, lockfile, and pack behavior are specified
for this builder.

Deno should be excluded from the initial JS/TS npm package profile because it is not an npm package
manager in the same sense as npm, pnpm, Yarn, or Bun. If Windlass needs Deno-first projects that
produce npm-compatible packages, they should be handled by a separate Deno-to-npm profile or adapter
decision.

This decision does not decide how the workflow selects among npm, pnpm, and Yarn for a particular
package. A follow-up ADR or architecture specification should define the precedence and validation
rules for `devEngines.packageManager`, `packageManager`, lockfiles, workflow inputs, and conflict
handling.

### Consequences

- Good, because npm remains the baseline package manager and publish mechanism for the npm package
  profile.
- Good, because pnpm projects can use their native install, build, and pack behavior before the
  final `npm publish` step.
- Good, because Yarn projects can be supported without forcing conversion to npm or pnpm install
  semantics.
- Good, because the supported set covers the most common npm package development workflows without
  treating every JavaScript runtime as an initial package manager target.
- Good, because Bun and Deno remain available for future explicit expansion instead of being
  silently unsupported edge cases.
- Neutral, because Bun users will need a later adapter or temporary unsupported path.
- Bad, because the initial implementation must maintain command mappings, fixtures, and verification
  behavior for three package managers rather than one.
- Bad, because Yarn has multiple generations and installation modes, so the architecture
  specification must define which Yarn behavior is supported.
- Bad, because repositories using Bun or Deno cannot use the initial stable profile without changing
  their pre-publish build path or waiting for a follow-up profile or adapter.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification
defines:

- npm, pnpm, and Yarn as the official package managers for install, build, and pack stages;
- `npm publish` as the publish stage regardless of the package manager used before publish,
  consistent with ADR 0013;
- Bun as excluded from the stable initial scope unless a follow-up ADR adds it;
- Deno as excluded from the initial npm package profile and deferred to a separate Deno-to-npm
  profile or adapter if needed;
- a separate decision or specification section for package-manager selection, metadata precedence,
  lockfile validation, and conflict handling.

Implementation review should reject implicit Bun or Deno support in the stable initial JS/TS npm
package profile unless a follow-up ADR changes this decision.

## Pros and Cons of the Options

### Support npm only

Use npm for install, build, pack, and publish.

- Good, because it is the smallest and simplest implementation.
- Good, because npm is the native package manager for npm registry package publishing.
- Good, because `npm ci`, `npm run`, `npm pack`, and `npm publish` provide one consistent command
  family.
- Bad, because it does not respect common pnpm and Yarn development workflows.
- Bad, because projects that pin pnpm or Yarn would need to build under different dependency
  resolution and workspace semantics from their normal development path.
- Bad, because it conflicts with the profile goal of supporting non-npm package-manager use before
  publish.

### Support npm and pnpm

Use npm and pnpm for install, build, and pack, while publishing with npm.

- Good, because it covers npm plus the package manager already selected for this repository's
  Node.js development tooling.
- Good, because pnpm provides strong lockfile, frozen install, workspace, and pack behavior for npm
  packages.
- Good, because `pnpm pack` followed by `npm publish` is a practical path when npm publish behavior
  is required.
- Neutral, because it is a smaller stable support set than including Yarn.
- Bad, because Yarn remains a major npm package development workflow and would be unsupported.
- Bad, because adding Yarn later could still require changing package-manager selection and fixture
  assumptions.

### Support npm, pnpm, and Yarn

Use npm, pnpm, and Yarn for install, build, and pack, while publishing with npm.

- Good, because it supports the mainstream Node.js package-manager families used for npm package
  development.
- Good, because all three can be mapped to frozen or immutable install behavior suitable for CI and
  trusted reusable workflows.
- Good, because all three support npm-package-oriented build and pack workflows.
- Good, because it keeps the first profile broad enough for realistic JS/TS package projects without
  adding newer runtime-coupled package managers.
- Neutral, because Yarn-specific behavior must be scoped carefully in the architecture
  specification.
- Bad, because the implementation and fixture matrix is larger than npm-only or npm-plus-pnpm.

### Support npm, pnpm, Yarn, and Bun

Use Bun as an additional supported package manager for install, build, and pack.

- Good, because Bun is increasingly used for JS/TS package development and can install dependencies,
  run scripts, use workspaces, and produce package archives.
- Good, because it would reduce friction for Bun-managed packages.
- Neutral, because Bun may become appropriate as an opt-in adapter once its builder contract is
  specified.
- Bad, because Bun couples package-manager behavior with a separate JavaScript runtime, which
  broadens the initial profile surface.
- Bad, because Bun setup, version pinning, lockfile validation, and pack semantics need their own
  specification before stable support.
- Bad, because adding Bun initially increases the support matrix before the npm, pnpm, and Yarn path
  is stabilized.

### Support npm, pnpm, Yarn, Bun, and Deno

Treat Deno as another supported install, build, and pack tool for the initial npm package profile.

- Good, because it would maximize JavaScript and TypeScript ecosystem coverage.
- Good, because Deno-first projects may need a path to npm-compatible package output.
- Bad, because Deno is a separate runtime and package ecosystem rather than a conventional npm
  package manager.
- Bad, because Deno package output and publish behavior do not share the same assumptions as npm,
  pnpm, Yarn, or Bun package-manager workflows.
- Bad, because this would turn the initial npm package profile into a broader Deno-to-npm
  transformation profile.
- Bad, because Deno support should be decided through a separate adapter or profile with its own
  artifact, pack, and verification contract.

## More Information

This decision follows ADR 0013 and intentionally leaves package-manager selection rules undecided.

Future specifications or ADRs should define:

- how the builder detects or selects npm, pnpm, or Yarn for a target package;
- whether `devEngines.packageManager`, `packageManager`, lockfiles, or workflow inputs are
  authoritative;
- how conflicts between package metadata and lockfiles are handled;
- which Yarn generations and linker modes are supported;
- whether and how Bun can graduate from a future opt-in adapter to official support;
- whether Deno-first npm-compatible packages deserve a dedicated profile.

Reference points considered:

- npm supports `npm ci`, `npm pack`, `npm publish`, workspaces, and package metadata fields relevant
  to package-manager consistency.
- pnpm supports frozen installs, workspaces, `pnpm pack`, and npm-compatible package workflows, but
  `pnpm publish` is not the initial provenance-aware publish path from ADR 0013.
- Yarn supports immutable installs, workspaces, package packing, and npm registry publishing
  workflows.
- Bun supports package-manager behavior but requires separate treatment for stable builder setup and
  versioning.
- Deno supports npm-compatible package output in some cases but is not a conventional npm package
  manager and should not be included in the initial npm package profile.
