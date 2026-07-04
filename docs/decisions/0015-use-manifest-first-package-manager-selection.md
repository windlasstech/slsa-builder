---
parent: Decisions
nav_order: 15
status: accepted
date: 12026-06-28
decision-makers: Yunseo Kim
---

# Use Manifest-First Package Manager Selection

## Context and Problem Statement

ADR 0013 scoped the initial JS/TS profile to npm registry package releases. ADR 0014 decided that
the initial JS/TS npm package profile officially supports npm, pnpm, and Yarn for install, build,
and pack stages, while publishing with `npm publish`. The next decision is how the trusted reusable
workflow selects one of the supported package managers for a target package.

The selection policy must respect package metadata, avoid surprising lockfile-only inference, and
prevent caller-controlled inputs from silently changing trusted build behavior. It must also define
how `packageManager`, `devEngines.packageManager`, lockfiles, and workflow inputs interact.

What should be the package manager selection precedence for the initial JS/TS npm package profile?

## Decision Drivers

- Respect the package's declared package-manager intent when it is present in `package.json`.
- Prefer explicit manifest metadata over lockfile heuristics.
- Avoid caller-controlled package-manager overrides that can make the trusted build differ from the
  repository's declared development model.
- Support legacy or minimal packages that only have a lockfile, while failing on ambiguous or
  conflicting signals.
- Make selection behavior explainable in provenance, logs, and verification documentation.
- Keep package directory or workspace disambiguation separate from package-manager override.

## Considered Options

- Use manifest-first selection: `packageManager`, then `devEngines.packageManager`, then lockfile
  inference, with conflict failure.
- Use `devEngines.packageManager` before `packageManager`.
- Use lockfile-first autodetection.
- Use workflow input as the primary selector.
- Require manifest metadata only and reject lockfile inference.

## Decision Outcome

Chosen option: "Use manifest-first selection: `packageManager`, then `devEngines.packageManager`,
then lockfile inference, with conflict failure", because it makes package metadata authoritative
while preserving a limited compatibility path for packages that have not yet declared
package-manager metadata.

The initial JS/TS npm package profile should select the package manager in this order:

1. Use top-level `packageManager` from the target package's `package.json` when present.
2. If `packageManager` is absent, use `devEngines.packageManager` when present.
3. If neither manifest field is present, infer the package manager from the relevant lockfile.
4. If the manifest fields and lockfile signals conflict, fail rather than choosing one silently.
5. If multiple lockfiles make inference ambiguous, fail rather than guessing.

Workflow inputs should not provide a default package-manager override. Inputs may identify a package
directory, workspace, or other monorepo disambiguation value, but they should not normally override
the package manager selected from the target package metadata and lockfile. If a future
implementation needs an override for migration or emergency use, that path should require a
follow-up decision or a separately documented escape hatch that records the override in provenance
and fails on unsafe divergence.

This decision does not define the exact JSON parsing rules for every `devEngines.packageManager`
shape, lockfile search root, or workspace package resolution path. Those details belong in the JS/TS
profile architecture specification.

### Consequences

- Good, because top-level `packageManager` is the clearest project-level package-manager pin for
  Corepack-aware npm package workflows.
- Good, because `devEngines.packageManager` remains useful when a project uses it to declare
  development package-manager expectations.
- Good, because lockfiles can support legacy package inference without becoming the primary trust
  source.
- Good, because conflicting metadata fails loudly instead of making the trusted workflow pick an
  arbitrary winner.
- Good, because caller workflows cannot silently switch a package from npm to pnpm or Yarn through
  an input.
- Neutral, because package directory and workspace selection still require explicit workflow
  contract design.
- Bad, because projects with stale metadata or multiple lockfiles must fix their repository state
  before using the profile.
- Bad, because strict conflict failure may create adoption friction for repositories migrating
  between package managers.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification
defines:

- `packageManager` as the first package-manager selection source;
- `devEngines.packageManager` as the second selection source when `packageManager` is absent;
- lockfile inference as a fallback only when both manifest fields are absent;
- failure behavior for conflicts between manifest fields and lockfiles;
- failure behavior for multiple ambiguous lockfiles;
- workflow inputs limited to package directory or monorepo disambiguation by default, not
  package-manager override.

Implementation review should verify that package-manager selection is deterministic, logged, and
included in verifier-relevant provenance parameters where appropriate.

## Pros and Cons of the Options

### Use manifest-first selection with conflict failure

Select from `packageManager`, then `devEngines.packageManager`, then lockfile inference, failing on
conflicts.

- Good, because manifest metadata expresses repository intent more directly than lockfile presence.
- Good, because `packageManager` aligns with Corepack and Yarn conventions for package-manager
  pinning.
- Good, because `devEngines.packageManager` remains available as a secondary manifest signal.
- Good, because lockfile inference can still support older packages without metadata.
- Good, because conflict failure protects the trusted workflow from ambiguous or stale repository
  state.
- Bad, because repositories with inconsistent metadata and lockfiles must be cleaned up before
  release.

### Use `devEngines.packageManager` before `packageManager`

Prefer `devEngines.packageManager`, then `packageManager`, then lockfiles.

- Good, because it emphasizes the development package-manager expectation.
- Good, because it is attractive for pnpm-oriented workflows where `devEngines.packageManager` can
  carry package-manager policy.
- Bad, because top-level `packageManager` is the clearer Corepack project pin for npm, pnpm, and
  Yarn selection.
- Bad, because it mixes environment validation semantics with primary selection semantics.
- Bad, because it is harder to explain for Yarn projects that use `packageManager` as their standard
  package-manager pin.

### Use lockfile-first autodetection

Infer npm, pnpm, or Yarn from `package-lock.json`, `pnpm-lock.yaml`, or `yarn.lock` before reading
manifest metadata.

- Good, because many existing projects already have lockfiles even when package-manager metadata is
  missing.
- Good, because it can reduce onboarding friction.
- Bad, because lockfiles represent dependency resolution state rather than package-manager intent.
- Bad, because migration leftovers or multiple lockfiles can make autodetection ambiguous.
- Bad, because it can ignore explicit package metadata that should be authoritative.

### Use workflow input as the primary selector

Require or allow caller workflows to set `package-manager: npm`, `pnpm`, or `yarn`.

- Good, because the reusable workflow API is explicit.
- Good, because monorepo callers can disambiguate unusual layouts.
- Bad, because caller-controlled input can make the trusted build use a different package manager
  from repository metadata.
- Bad, because the override becomes verifier-relevant and must be carefully recorded and justified.
- Bad, because it weakens the trust boundary unless every override is validated against manifest and
  lockfile state.

### Require manifest metadata only

Require `packageManager` or `devEngines.packageManager` and reject lockfile-only inference.

- Good, because it is the strictest and most deterministic policy.
- Good, because it avoids all lockfile autodetection ambiguity.
- Bad, because many otherwise valid npm packages do not yet declare package-manager metadata.
- Bad, because it raises initial adoption friction more than the project currently needs.

## More Information

This decision follows ADR 0014 and intentionally does not decide package-manager installation
mechanics. Corepack and version enforcement are covered by a separate decision.

Reference points considered:

- Corepack treats top-level `packageManager` as the project package-manager specification and
  enforces the configured package manager for supported tools.
- npm documents `devEngines.packageManager` as development-environment validation with configurable
  failure behavior.
- pnpm supports `devEngines.packageManager` and can resolve package-manager versions from it.
- Yarn uses top-level `packageManager` as its package-manager pin and supports immutable installs.
- npm, pnpm, and Yarn lockfiles support frozen or immutable dependency installation, but lockfiles
  are dependency graph controls rather than the best primary source for package-manager selection.
