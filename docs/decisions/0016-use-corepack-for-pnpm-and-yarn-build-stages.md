---
parent: Decisions
nav_order: 16
status: accepted
date: 12026-06-28
decision-makers: Yunseo Kim
---

# Use Corepack for pnpm and Yarn Build Stages

## Context and Problem Statement

ADR 0014 decided that the initial JS/TS npm package profile supports npm, pnpm, and Yarn for
install, build, and pack stages. ADR 0015 decided how the profile selects one of those package
managers from manifest metadata and lockfiles. The next decision is how the trusted reusable
workflow obtains and enforces the selected package manager version.

Corepack provides package-manager version dispatch for pnpm and Yarn through project metadata such
as `packageManager`. However, npm shims are not installed by default in Corepack, and npm is already
bundled with the Node.js toolchain available on GitHub-hosted runners or installed by Node setup
steps.

Should the initial JS/TS npm package profile use Corepack for all package managers, use Corepack
only for pnpm and Yarn, or avoid Corepack entirely?

## Decision Drivers

- Enforce package-manager versions for pnpm and Yarn in a way that matches common Node.js project
  metadata.
- Avoid unnecessary npm shim complexity when npm is already available from the selected Node.js
  toolchain.
- Keep the trusted workflow deterministic and auditable.
- Preserve strict Corepack behavior rather than silently falling back to unrelated global
  package-manager versions.
- Align version enforcement with ADR 0015's manifest-first selection policy.
- Keep publish behavior separate: package publishing still uses `npm publish` as decided in
  ADR 0013.

## Considered Options

- Use Corepack for pnpm and Yarn, and use the runner or selected Node.js toolchain's npm directly
  for npm.
- Use Corepack for npm, pnpm, and Yarn.
- Avoid Corepack and install pnpm and Yarn manually.
- Use repository-bundled package-manager binaries.

## Decision Outcome

Chosen option: "Use Corepack for pnpm and Yarn, and use the runner or selected Node.js toolchain's
npm directly for npm", because it gives pnpm and Yarn project-version enforcement without relying on
npm Corepack shims that are not enabled by default.

The initial JS/TS npm package profile should:

- use Corepack in strict mode for pnpm and Yarn install, build, and pack stages;
- respect the package-manager selection policy from ADR 0015;
- use the npm CLI provided by the selected Node.js toolchain or GitHub-hosted runner for npm
  install, build, pack, and publish stages;
- avoid disabling Corepack project-spec enforcement for pnpm and Yarn;
- fail if the selected pnpm or Yarn version cannot be prepared or does not match the selected
  package metadata;
- record the selected package manager and version in logs and verifier-relevant provenance
  parameters where appropriate.

This decision does not require Corepack for npm. If future Node.js or Corepack behavior changes
enough to make npm shim enforcement safe and useful, that can be reconsidered in a follow-up ADR.

### Consequences

- Good, because pnpm and Yarn versions can be enforced through the package metadata model those
  tools already support.
- Good, because npm keeps the simplest path: the npm CLI from the selected Node.js toolchain.
- Good, because the workflow avoids relying on Corepack npm shims that are not enabled by default.
- Good, because Corepack strict mode helps prevent a globally installed pnpm or Yarn from bypassing
  repository metadata.
- Good, because the behavior aligns with ADR 0015's `packageManager`-first selection model.
- Neutral, because npm version enforcement is tied to Node.js toolchain selection rather than a
  separate npm package-manager pin.
- Bad, because pnpm and Yarn require Corepack setup and cache/download behavior to be specified in
  the implementation.
- Bad, because environments without network access may need a prepackaged Corepack cache or
  documented preparation path.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification
defines:

- Corepack usage for pnpm and Yarn install, build, and pack stages;
- direct npm usage from the selected Node.js toolchain for npm install, build, pack, and publish
  stages;
- strict failure when Corepack cannot prepare or enforce the selected pnpm or Yarn version;
- a prohibition on disabling Corepack project-spec enforcement by default;
- logging and provenance capture of the selected package manager and version.

Implementation review should verify that pnpm and Yarn paths do not accidentally use arbitrary
globally installed package-manager versions and that npm publish continues to use the intended npm
CLI.

## Pros and Cons of the Options

### Use Corepack for pnpm and Yarn, direct npm for npm

Use Corepack to prepare and dispatch pnpm and Yarn, while using the npm CLI from the selected
Node.js toolchain for npm.

- Good, because it matches Corepack's practical strengths for pnpm and Yarn version dispatch.
- Good, because it avoids npm shim behavior that is not enabled by default.
- Good, because npm remains available for the final `npm publish` path required by ADR 0013.
- Good, because package-manager enforcement for pnpm and Yarn is tied to package metadata.
- Neutral, because npm version control must be handled through Node.js setup rather than Corepack.
- Bad, because the implementation uses two enforcement mechanisms instead of one uniform mechanism.

### Use Corepack for npm, pnpm, and Yarn

Attempt to route all package managers, including npm, through Corepack.

- Good, because it appears uniform at the policy level.
- Good, because top-level `packageManager` could theoretically describe all three managers in one
  place.
- Bad, because Corepack does not install npm shims by default.
- Bad, because forcing npm through Corepack adds setup complexity without clear benefit for the
  initial profile.
- Bad, because npm is already provided by the Node.js toolchain needed for `npm publish`.

### Avoid Corepack and install pnpm and Yarn manually

Install specific pnpm and Yarn versions through standalone setup steps or scripts.

- Good, because the workflow controls the exact installation commands.
- Good, because it avoids Corepack-specific behavior and cache semantics.
- Bad, because it duplicates package-manager dispatch behavior that Corepack is designed to provide.
- Bad, because it increases workflow surface area and installation policy complexity.
- Bad, because it can drift from the package metadata that developers use locally.

### Use repository-bundled package-manager binaries

Require packages to commit or vendor the package-manager runtime needed for release builds.

- Good, because release builds can avoid runtime package-manager downloads.
- Good, because repository review can inspect committed package-manager artifacts or metadata.
- Bad, because vendoring package managers increases repository size and maintenance burden.
- Bad, because this is unusual for npm package projects and would create contributor friction.
- Bad, because it shifts package-manager update and verification responsibility to every downstream
  repository.

## More Information

This decision complements ADR 0015. ADR 0015 decides what package manager is selected; this ADR
decides how that selected package manager is obtained and enforced during trusted workflow
execution.

Reference points considered:

- Corepack dispatches supported package managers based on project metadata and Known Good Releases
  when no project specification exists.
- Corepack documentation notes that npm shims are not installed by default.
- pnpm and Yarn are commonly enforced through Corepack-aware `packageManager` metadata.
- npm is bundled with Node.js and is required for the final provenance-aware `npm publish` path
  selected by ADR 0013.
