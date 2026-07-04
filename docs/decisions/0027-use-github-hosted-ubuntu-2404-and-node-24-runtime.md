---
parent: Decisions
nav_order: 27
status: accepted
date: 12026-07-04
decision-makers: Yunseo Kim
---

# Use GitHub-Hosted Ubuntu 24.04 and Node.js 24 Runtime

## Context and Problem Statement

ADR 0002 chose a trusted reusable workflow foundation oriented around GitHub-hosted runners. ADR
0014 through ADR 0017 selected npm, pnpm, and Yarn for JS/TS install, build, and pack stages;
selected manifest-first package-manager detection; chose Corepack for pnpm and Yarn; and required
exact pnpm and Yarn versions for release builds. ADR 0024 selected OIDC trusted publishing without
publish secrets and documented that npm trusted publishing requires GitHub-hosted runners, npm CLI
11.5.1 or later, and Node.js 22.14.0 or later. ADR 0026 then constrained supported release caller
patterns.

The initial JS/TS npm package profile still needs an explicit runtime policy for the production
SLSA3 reusable workflow: which runner class, operating system image, Node.js major version, npm CLI
requirements, and package-manager version rules are part of the supported public contract.

Should the profile use a fixed builder-owned runtime, allow caller-selected runtimes, or track broad
LTS/runtime ranges?

## Decision Drivers

- Keep the production builder runtime deterministic enough for downstream verification.
- Match npm trusted publishing requirements for GitHub Actions OIDC publishes.
- Preserve the GitHub-hosted runner trust boundary already assumed by ADR 0002, ADR 0020, and
  ADR 0024.
- Avoid adding runner, OS, Node.js, or package-manager override inputs to the public `workflow_call`
  contract.
- Align the default Node.js runtime with GitHub-maintained and major ecosystem actions that have
  moved to Node.js 24 runtimes.
- Avoid `ubuntu-latest` drift in a release-producing builder identity.
- Keep ADR 0016 and ADR 0017's stricter pnpm and Yarn Corepack/version policy intact.
- Leave future lower-assurance, self-hosted, or alternate-runtime modes available through distinct
  decisions and builder identities.

## Considered Options

- Use GitHub-hosted `ubuntu-24.04`, Node.js 24, npm version checks, and existing strict
  package-manager enforcement.
- Use GitHub-hosted `ubuntu-24.04` with active and maintenance LTS Node.js versions.
- Use GitHub-hosted `ubuntu-latest` with Node.js 24.
- Let callers select runner, OS, Node.js, and package-manager versions through workflow inputs.
- Support self-hosted runners in the production SLSA3 profile.

## Decision Outcome

Chosen option: "Use GitHub-hosted `ubuntu-24.04`, Node.js 24, npm version checks, and existing
strict package-manager enforcement", because it gives the initial production SLSA3 npm package
profile a small, builder-owned runtime contract while matching the ecosystem move toward Node.js 24
action runtimes.

The production JS/TS npm package profile should run on GitHub-hosted Linux runners using the
explicit Ubuntu 24.04 image:

```yaml
runs-on: ubuntu-24.04
```

The initial production profile should not support self-hosted runners, macOS runners, Windows
runners, container jobs, or caller-selected runner labels. Those environments have different trust,
isolation, filesystem, shell, path, and native-build properties and should require a later ADR and a
distinct builder identity if they become supported.

The profile should install and use Node.js 24 as the builder-owned JS/TS release runtime. Node.js 24
is the only supported Node.js major version for the initial production SLSA3 profile. The reusable
workflow should not expose a `node-version`, `node-version-file`, `runner`, `runs-on`, `os`, or
equivalent caller input for the production profile.

The exact Node.js patch version is selected by the builder implementation and its pinned setup
mechanism for a given builder release. The workflow should record the actual Node.js version in logs
and verifier-relevant provenance parameters where appropriate. A future change to another Node.js
major version should be treated as a public runtime-policy change and should be recorded by a later
decision or distinct builder mode before production use.

The profile should use the npm CLI provided by the selected Node.js 24 toolchain for npm install,
pack, and publish stages. Before publishing, it should verify that the actual npm CLI version
satisfies npm trusted publishing requirements, including npm CLI 11.5.1 or later, and fail clearly
if it does not. The workflow should record the actual npm version used.

Package-manager support remains the policy already selected by ADR 0014, ADR 0016, and ADR 0017:

- npm, pnpm, and Yarn are supported for install, build, and pack stages;
- final registry publishing uses `npm publish`;
- pnpm and Yarn are obtained and enforced through Corepack;
- pnpm and Yarn release builds require exact package-manager versions from selected package
  metadata;
- Corepack Known Good Release fallback, package-manager version ranges, and arbitrary global pnpm or
  Yarn installations are not supported in production release builds;
- npm's actual version is captured from the selected Node.js 24 toolchain rather than required as a
  separate package metadata pin.

Packages whose `engines`, native dependencies, lifecycle scripts, or package-manager metadata cannot
build under the selected GitHub-hosted Ubuntu 24.04 and Node.js 24 runtime are outside the initial
production profile until a later ADR defines a broader runtime policy or separate builder mode.

### Consequences

- Good, because the production runtime is explicit: GitHub-hosted Ubuntu 24.04 with Node.js 24.
- Good, because using a fixed Ubuntu image avoids `ubuntu-latest` drift in release-producing builds.
- Good, because Node.js 24 aligns the profile with the current direction of GitHub-maintained and
  major ecosystem actions.
- Good, because the npm CLI can satisfy trusted publishing requirements while remaining coupled to
  the selected Node.js toolchain.
- Good, because callers cannot silently change verifier-relevant runtime behavior through workflow
  inputs.
- Good, because self-hosted and non-Linux runners do not accidentally share the same production
  SLSA3 builder identity.
- Neutral, because exact Node.js patch selection still depends on the builder release's setup
  mechanism and should be recorded.
- Bad, because packages that only support Node.js 22 or older cannot use the initial production
  profile without updating or waiting for a later builder mode.
- Bad, because the project must intentionally review future Node.js major and Ubuntu image changes
  rather than inheriting them automatically.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification and
implementation define:

- GitHub-hosted `ubuntu-24.04` as the production runner image;
- no support for self-hosted, macOS, Windows, container-job, or caller-selected runner labels in the
  production SLSA3 profile;
- Node.js 24 as the only supported production Node.js major version;
- no `node-version`, `node-version-file`, `runner`, `runs-on`, `os`, or equivalent runtime override
  input in the production `workflow_call` contract;
- actual Node.js and npm version capture in logs and verifier-relevant provenance parameters where
  appropriate;
- clear failure when npm CLI is older than the trusted publishing minimum;
- preservation of ADR 0016 and ADR 0017 Corepack and exact-version requirements for pnpm and Yarn;
- documentation that packages incompatible with Ubuntu 24.04 or Node.js 24 are outside the initial
  production profile.

Implementation review should verify that release builds cannot proceed on an unsupported runner
class, operating system, Node.js major version, npm CLI version, or package-manager version policy
under the production SLSA3 builder identity.

## Pros and Cons of the Options

### Use GitHub-hosted Ubuntu 24.04, Node.js 24, and strict package-manager enforcement

Run the production profile on GitHub-hosted `ubuntu-24.04`, install Node.js 24 through the builder,
verify npm trusted publishing compatibility, and keep exact Corepack-enforced pnpm/Yarn versions.

- Good, because runner image and Node.js major version are explicit and verifier-friendly.
- Good, because it avoids `ubuntu-latest` image drift.
- Good, because Node.js 24 matches the current direction of GitHub-maintained and major ecosystem
  action runtimes.
- Good, because it preserves the small public input contract selected by ADR 0023.
- Good, because npm, pnpm, and Yarn behavior remains governed by earlier package-manager ADRs.
- Bad, because packages not yet compatible with Node.js 24 or Ubuntu 24.04 are unsupported.

### Use GitHub-hosted Ubuntu 24.04 with active and maintenance LTS Node.js versions

Run on explicit Ubuntu 24.04 but allow the builder to select among supported Node.js LTS majors such
as Node.js 22 and Node.js 24.

- Good, because it accepts more existing packages that have not yet moved to Node.js 24.
- Good, because Node.js 22 can still satisfy npm trusted publishing's minimum when the npm CLI is
  new enough.
- Bad, because Node.js selection rules become verifier-relevant and more complex.
- Bad, because the workflow would need to define how `engines`, `devEngines.runtime`, `.nvmrc`, or
  other version files interact with the builder-owned runtime.
- Bad, because supporting multiple Node.js majors expands the fixture and compatibility matrix.

### Use GitHub-hosted ubuntu-latest with Node.js 24

Run on GitHub-hosted `ubuntu-latest` and install Node.js 24.

- Good, because `ubuntu-latest` is common in GitHub Actions examples and several SLSA generator
  examples.
- Good, because the profile would inherit GitHub's default Linux image updates automatically.
- Bad, because `ubuntu-latest` can change the runner image under the same builder contract.
- Bad, because release artifacts involving native dependencies, shell tools, or system libraries can
  drift when GitHub updates the image alias.
- Bad, because verifier documentation must explain a moving OS image rather than a fixed one.

### Let callers select runner, OS, Node.js, and package-manager versions

Expose runtime override inputs such as `runs-on`, `node-version`, `node-version-file`, or
package-manager version overrides.

- Good, because it maximizes compatibility with legacy or specialized packages.
- Good, because downstream repositories can align releases with their existing CI matrices.
- Bad, because caller-selected runtime values become verifier-relevant external parameters.
- Bad, because the public `workflow_call` API grows substantially.
- Bad, because the same `builder.id` would cover builds with materially different runtime and trust
  properties unless every mode is separated.
- Bad, because it conflicts with the profile's existing small-input and strict package-manager
  direction.

### Support self-hosted runners in the production SLSA3 profile

Allow caller repositories to run the production reusable workflow on self-hosted runners.

- Good, because it supports organizations with private infrastructure, custom hardware, or
  restricted network environments.
- Good, because it can reduce migration friction for repositories already using self-hosted release
  runners.
- Bad, because npm trusted publishing does not support self-hosted runners for the selected OIDC
  publishing path.
- Bad, because SLSA Build L3 runner isolation and trust properties differ materially from
  GitHub-hosted runners.
- Bad, because a self-hosted mode would need separate governance, documentation, verification, and
  likely a distinct builder identity or lower-assurance claim.

## More Information

This decision follows ADR 0014, ADR 0015, ADR 0016, ADR 0017, ADR 0023, ADR 0024, and ADR 0026. It
decides the production runner, OS image, Node.js major version, npm version check, and how existing
package-manager version decisions apply under that runtime. It does not decide package versioning,
release tag naming, private dependency credentials, native dependency support, or future validation
matrix behavior outside production publishes.

Reference points considered:

- SLSA Build L3 requires a hosted, isolated build platform with provenance authenticity protected
  from tenant-controlled build steps.
- npm trusted publishing for GitHub Actions requires GitHub-hosted runners, OIDC permissions, npm
  CLI 11.5.1 or later, and Node.js 22.14.0 or later.
- npm trusted publishing and provenance examples commonly use GitHub-hosted Ubuntu runners and
  modern Node.js setup.
- SLSA GitHub Generator examples commonly use GitHub-hosted Ubuntu runners, often through
  `ubuntu-latest`, but this profile chooses explicit `ubuntu-24.04` to avoid image alias drift.
- GitHub-maintained and major ecosystem actions have moved or are moving their action runtime
  baseline to Node.js 24, making Node.js 24 the appropriate default for this new builder profile.
- Corepack supports pnpm and Yarn version dispatch from project metadata, while npm remains coupled
  to the selected Node.js toolchain for this profile.
