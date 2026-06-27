---
parent: Decisions
nav_order: 10
status: accepted
date: 12026-06-24
decision-makers: Yunseo Kim
---

# Use pnpm for Node.js Development Tooling

## Context and Problem Statement

ADR 0008 chose Prettier for Markdown formatting.
ADR 0009 chose Node.js as the development-only JavaScript runtime needed to run Prettier and explicitly deferred package-manager selection.
The project now needs a Node.js package manager for exact project-local Prettier installation, lockfile review, local execution, CI execution, and dependency update automation.

Which package manager should `slsa-builder` use for Node.js development tooling dependencies?

## Decision Drivers

- Keep Node.js package management limited to development tooling and out of the trusted runtime selected in ADR 0004.
- Install Prettier as an exact project-local development dependency so editors and CI use the same formatter version.
- Commit a lockfile for reproducible installs and reviewable dependency graph changes.
- Prefer package-manager-native supply-chain controls, especially a minimum release age for newly published packages.
- Support CI installs that fail when the lockfile and manifest disagree.
- Preserve compatibility with GitHub dependency tooling, Dependabot, Dependency Review, and OSV Scanner.
- Avoid introducing a non-Node runtime after ADR 0009 selected Node.js.

## Considered Options

- Use pnpm for Node.js development tooling.
- Use npm for Node.js development tooling.
- Use Yarn Berry for Node.js development tooling.
- Use Bun as the package manager for Node.js development tooling.

## Decision Outcome

Chosen option: "Use pnpm for Node.js development tooling", because pnpm provides a strong balance of Node.js compatibility, exact project-local Prettier installation, committed lockfile reproducibility, strict dependency isolation, and native supply-chain controls such as `minimumReleaseAge`.

pnpm should be used only for development tooling dependencies unless a future ADR expands the JavaScript or TypeScript surface.
The trusted implementation remains Go, as decided in ADR 0004.
Prettier remains a development-only formatter dependency, as decided in ADR 0008 and ADR 0009.

The repository should commit `pnpm-lock.yaml`.
CI should use a frozen-lockfile install mode so manifest and lockfile drift fails fast.
The pnpm version should be pinned through the `packageManager` field in `package.json`, with Corepack or an equivalent pinned installation path used in CI.

The initial pnpm policy should explicitly configure `minimumReleaseAge` for dependency resolution.
At the time of this decision, pnpm supports `minimumReleaseAge` in minutes, applies it to all dependencies including transitive dependencies, and supports exclusions through `minimumReleaseAgeExclude`.
Any exclusion should be narrow and justified.

### Consequences

- Good, because pnpm has package-manager-native minimum-release-age controls that align with the repository's supply-chain risk model.
- Good, because `pnpm-lock.yaml` provides a committed, reviewable dependency graph as required by organization dependency-security policy.
- Good, because pnpm's strict dependency layout helps expose undeclared dependency use.
- Good, because pnpm can run the project-local Prettier dependency without relying on an implicitly downloaded latest version.
- Good, because the pnpm version can be pinned through `packageManager` and managed consistently through Corepack-aware workflows.
- Neutral, because pnpm introduces a package-manager bootstrap step that npm would not need.
- Bad, because contributors who only know npm may need to install or enable pnpm through Corepack.
- Bad, because pnpm's stricter dependency layout can expose compatibility issues in some JavaScript tooling, although Prettier is expected to be straightforward.

### Confirmation

This decision is confirmed when development tooling configuration and CI usage:

- declare pnpm as the package manager in `package.json` with a pinned version;
- install Prettier as an exact project-local development dependency;
- commit `pnpm-lock.yaml`;
- configure pnpm `minimumReleaseAge` for newly resolved packages;
- document any `minimumReleaseAgeExclude` entries with narrow justification;
- run CI installs with frozen lockfile behavior;
- run Prettier through pnpm from the project-local installation;
- keep Node.js and pnpm out of the trusted runtime and release artifact execution path.

Implementation review should verify that dependency updates preserve lockfile reviewability, respect cooldown policy, and remain compatible with the organization's Dependency Review and OSV Scanner requirements.

## Pros and Cons of the Options

### Use pnpm for Node.js development tooling

Use pnpm to install and run Prettier and any future accepted Node.js development-only tools.

- Good, because pnpm supports `minimumReleaseAge` for newly published package versions.
- Good, because `minimumReleaseAge` applies to all dependencies, including transitive dependencies.
- Good, because `minimumReleaseAgeExclude` allows narrow exceptions when an urgent or trusted dependency must bypass the age gate.
- Good, because pnpm's content-addressable store and strict dependency layout improve install efficiency and dependency hygiene.
- Good, because `pnpm-lock.yaml` is supported by organization lockfile policy and common GitHub dependency tooling.
- Good, because Corepack can enforce the pinned pnpm version from the package manifest.
- Neutral, because pnpm is less default than npm but still widely used in the Node.js ecosystem.
- Bad, because contributors may need to enable Corepack or install pnpm before running local formatting commands.
- Bad, because pnpm's configuration surface is larger than npm's for a small Prettier-only dependency set.

### Use npm for Node.js development tooling

Use npm, which is bundled with Node.js, to install and run Prettier.

- Good, because npm is included with Node.js and has the lowest contributor bootstrap cost.
- Good, because Prettier's project-local installation documentation works directly with npm.
- Good, because `package-lock.json` is broadly supported by GitHub dependency tooling.
- Good, because recent npm versions support `min-release-age` for package resolution.
- Neutral, because npm's cooldown support reduces the gap with pnpm for basic age-gating.
- Bad, because npm's native cooldown policy is less mature and less granular than pnpm's current `minimumReleaseAge` controls.
- Bad, because npm's dependency layout is less strict about undeclared dependency access.
- Bad, because package-manager version pinning is less explicit than a Corepack-managed pnpm workflow unless additional policy is added.

### Use Yarn Berry for Node.js development tooling

Use modern Yarn for Prettier and development-only Node.js dependencies.

- Good, because Yarn Berry supports `npmMinimalAgeGate` as a package age gate.
- Good, because Yarn Berry has strong install immutability, hardened mode, and package-manager pinning features.
- Good, because Yarn can be managed through Corepack.
- Neutral, because Yarn Berry can be configured with `nodeLinker: node-modules` to avoid Plug'n'Play when needed.
- Bad, because Yarn Classic and Yarn Berry differ materially, which increases policy and contributor explanation cost.
- Bad, because Plug'n'Play, zero-install, linker selection, and cache policy are more machinery than the current Prettier-focused need requires.
- Bad, because pnpm provides the desired age-gate and strictness benefits with a simpler fit for this repository.

### Use Bun as the package manager for Node.js development tooling

Use Bun's package manager to install and run Prettier while Node.js remains the selected runtime.

- Good, because Bun supports `minimumReleaseAge` in `bunfig.toml` and through CLI flags.
- Good, because Bun installs are fast and Bun limits dependency lifecycle scripts by default.
- Good, because Bun has a lockfile and frozen-install mode.
- Neutral, because Bun may be reconsidered if a future profile explicitly targets Bun users.
- Bad, because choosing Bun would introduce a Bun toolchain after ADR 0009 selected Node.js as the development tool runtime.
- Bad, because Bun's runtime and package-manager roles are coupled, making this less clean as a Node.js package-manager decision.
- Bad, because GitHub, editor, and contributor expectations remain more conventional for npm, pnpm, or Yarn in a Node.js development-tooling context.

## More Information

This decision follows ADR 0008 and ADR 0009.
It does not authorize JavaScript or TypeScript trusted runtime implementation.

Organization dependency-security policy requires committed lockfiles and layered dependency controls.
For this decision, that means `pnpm-lock.yaml` should be committed, dependency update automation should preserve cooldown behavior, and Dependency Review and OSV Scanner should continue to review dependency changes.

Workflow implementation should also follow organization workflow-hardening policy.
Any GitHub Actions workflow that installs pnpm dependencies should use minimal permissions, hardened runners where required by policy, and pinned action references or approved internal reusable workflow references.
