---
parent: Decisions
nav_order: 18
status: accepted
date: 12026-06-28
decision-makers: Yunseo Kim
---

# Publish One JS/TS Package per Profile Run

## Context and Problem Statement

ADR 0013 scoped the initial JS/TS profile to npm registry package releases. ADR 0014 decided that
npm, pnpm, and Yarn are officially supported for install, build, and pack stages. ADR 0015, ADR
0016, and ADR 0017 defined package-manager selection and version enforcement. The next scope
decision is package layout: whether the initial profile supports only a root package, supports
workspaces, and whether it can publish multiple packages in a single run.

Modern JS/TS repositories often use workspaces or monorepos. At the same time, npm package
provenance, package URL, tarball digest, and registry verification are clearest when one release
operation maps to one package version.

Should the initial JS/TS npm package profile support only a root package, support a single selected
workspace package, or publish multiple workspace packages in one run?

## Decision Drivers

- Preserve ADR 0013's npm package release unit: one package name and version with one published
  tarball.
- Support common monorepo layouts without taking on multi-package release orchestration.
- Keep provenance subjects, package URLs, tarball digests, and verification commands unambiguous.
- Avoid partial multi-package release failure modes in the first profile.
- Keep versioning, changelog, dependency bumping, and publish ordering out of the initial trusted
  profile.
- Allow future orchestration profiles to compose multiple single-package profile runs when needed.

## Considered Options

- Support only a single package at the repository root.
- Support one package per run, where the package may be the repository root package or one
  explicitly selected workspace package.
- Support workspace and monorepo repositories with multi-package publish in one run.
- Support root package, selected workspace package, and multi-package publish in the initial
  profile.

## Decision Outcome

Chosen option: "Support one package per run, where the package may be the repository root package or
one explicitly selected workspace package", because it preserves a single npm package provenance
unit while supporting realistic workspace and monorepo repositories.

The initial JS/TS npm package profile should publish exactly one npm package per workflow run. That
package may be:

- the repository root package; or
- one explicitly selected workspace package in a workspace or monorepo repository.

The profile should support package directory or workspace disambiguation inputs consistent with
ADR 0015. Those inputs identify the target package; they should not override package-manager
selection.

Multi-package publish is out of scope for the initial JS/TS npm package profile. Repositories that
need to publish multiple packages should call the single-package profile multiple times through a
higher-level release orchestration workflow, or wait for a future multi-package profile or ADR.

### Consequences

- Good, because every profile run has one package name, one package version, one tarball, and one
  provenance subject set.
- Good, because root-package repositories remain simple.
- Good, because monorepos can still publish a selected workspace package without adopting
  multi-package release semantics.
- Good, because package directory and workspace selection solve the main monorepo layout need
  without caller-controlled package-manager override.
- Good, because multi-package versioning, publish ordering, and partial failure handling are
  deferred to a more appropriate orchestration layer.
- Neutral, because repositories with many packages must run the profile once per package or add
  orchestration.
- Bad, because the initial profile will not provide a one-shot monorepo release experience.
- Bad, because workspace support still requires careful specification of root lockfiles, package
  metadata, and package-manager commands for npm, pnpm, and Yarn.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification
defines:

- a single published npm package as the unit of one profile run;
- root package selection behavior;
- selected workspace package behavior;
- package directory or workspace disambiguation inputs;
- metadata, lockfile, and package-manager resolution rules for selected workspace packages;
- explicit exclusion of multi-package publish from the initial profile.

Implementation review should verify that a single profile run cannot publish multiple npm packages
unless a follow-up ADR changes this decision.

## Pros and Cons of the Options

### Support only a single package at the repository root

Require the publish target to be the root `package.json` package.

- Good, because this is the simplest possible layout.
- Good, because package metadata, lockfile, tarball path, and publish command are easy to reason
  about.
- Good, because no workspace selection inputs are required.
- Bad, because many JS/TS package repositories publish from `packages/*` or another subdirectory.
- Bad, because adding workspace support later would likely change workflow inputs and provenance
  parameters.
- Bad, because it unnecessarily excludes monorepo repositories that still publish one package at a
  time.

### Support one package per run, including a selected workspace package

Publish exactly one package per profile run, but allow that package to be the root package or one
explicitly selected workspace package.

- Good, because it keeps one profile run mapped to one npm package version.
- Good, because monorepo repositories can use the initial profile without requiring multi-package
  release orchestration.
- Good, because package URL, tarball digest, publish result, and verification commands remain
  package-specific.
- Good, because package directory or workspace selection can be recorded as verifier-relevant
  provenance.
- Neutral, because workspace-aware command mapping is required for npm, pnpm, and Yarn.
- Bad, because repositories with coordinated multi-package releases must add an orchestration layer.

### Support multi-package publish in one run

Allow one profile run to publish several workspace packages.

- Good, because it matches monorepo projects that release multiple packages together.
- Good, because one release workflow could coordinate all package publishes.
- Bad, because each package has its own name, version, tarball digest, and verification result.
- Bad, because partial publish failures require explicit rollback or recovery policy.
- Bad, because versioning, changelog, dependency bumping, dependency-order publish, and
  changed-package selection become part of the trusted profile scope.
- Bad, because npm, pnpm, and Yarn provide different workspace and recursive command semantics.

### Support every layout in the initial profile

Support root package publish, selected workspace package publish, and multi-package publish from the
start.

- Good, because it covers the widest range of JS/TS repository layouts.
- Good, because users would not need separate orchestration for monorepo releases.
- Bad, because it makes the initial profile much larger and harder to audit.
- Bad, because single-package and multi-package provenance models would be mixed in one profile.
- Bad, because this expands the first profile beyond the focused npm package release unit selected
  in ADR 0013.

## More Information

This decision refines ADR 0013's npm package artifact scope and ADR 0015's allowance for package
directory or monorepo disambiguation inputs.

Future specifications or ADRs should define:

- how root packages and workspace packages are selected;
- how npm, pnpm, and Yarn commands are mapped for a selected workspace package;
- how root lockfiles and workspace package metadata are validated;
- how workspace-local dependencies are handled before packing;
- whether a higher-level release orchestration workflow should call the single-package profile
  repeatedly for multi-package repositories;
- whether a future multi-package profile should integrate release tools such as Changesets or Rush.

Reference points considered:

- npm supports workspaces and workspace-targeted commands.
- pnpm supports workspace roots, filtering, and recursive workspace publishing, while broader
  workspace versioning often needs external release tooling.
- Yarn supports workspace-targeted commands and workspace iteration.
- Multi-package publishing introduces versioning and orchestration behavior beyond a single package
  provenance unit.
