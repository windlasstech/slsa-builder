---
parent: Decisions
nav_order: 33
status: accepted
date: 12026-07-05
decision-makers: Yunseo Kim
---

# Run Build Script Only When Declared

## Context and Problem Statement

ADR 0014 selected npm, pnpm, and Yarn as the supported package managers for install, build, and pack
stages in the initial JS/TS npm package profile. ADR 0015 through ADR 0017 defined package-manager
selection, Corepack usage, and exact version enforcement. ADR 0019 selected source metadata
validation followed by packed artifact inspection, and ADR 0023 rejected arbitrary install, build,
publish, environment, step, service, and default inputs for the production SLSA3 profile.

The remaining build-stage contract needs to define what the reusable workflow does between
dependency installation and package packing. In particular, npm packages vary widely: some require a
build script before packing, while others are source-only, type-only, configuration-only, or rely on
package manager lifecycle scripts that run during pack. Requiring every package to define
`scripts.build` would make release intent explicit but would reject many valid npm packages.
Skipping a build script entirely would improve compatibility but would make it easier to publish
packages that forgot to run their expected build.

Should the initial production JS/TS npm package profile require a build script, skip build entirely,
or run a declared build script while treating an absent build script as an intentional no-op?

## Decision Drivers

- Preserve the production SLSA3 profile's small public input surface and avoid caller-provided
  arbitrary commands.
- Keep install, build, and pack behavior deterministic and package-manager selected from repository
  metadata rather than workflow inputs.
- Support common npm packages that do not need a build step before packing.
- Run a package's declared build step when the package declares one.
- Make the build-step result visible to provenance and verifier policy without making the builder a
  package quality linter.
- Avoid silently expanding the trusted workflow boundary into monorepo orchestration, task-runner
  selection, or caller-defined shell execution.
- Keep future stricter modes possible through a later ADR or distinct builder identity.

## Considered Options

- Use fixed package-manager commands and require `scripts.build`.
- Use fixed package-manager commands, run `scripts.build` when present, and treat an absent build
  script as a successful no-op.
- Use fixed package-manager commands and require an explicit package metadata marker when no build
  script is present.
- Allow a caller-provided build command.
- Omit a separate build stage and rely only on package-manager pack or publish lifecycle behavior.

## Decision Outcome

Chosen option: "Use fixed package-manager commands, run `scripts.build` when present, and treat an
absent build script as a successful no-op", because it preserves the trusted workflow boundary while
supporting valid npm packages that do not require a build step.

The initial production JS/TS npm package profile should use builder-defined commands for install,
build, and pack stages. The concrete command forms for each supported package manager belong in the
JS/TS npm package profile architecture specification, but the public contract should follow these
rules:

- the selected package manager is determined by ADR 0015 through ADR 0017;
- dependency installation uses the selected package manager's frozen or immutable install behavior;
- the build stage checks the selected source package manifest for `scripts.build`;
- if `scripts.build` is present, the workflow runs the selected package manager's normal `build`
  script command for the selected package;
- if `scripts.build` is absent, the build stage succeeds as an explicit no-op;
- package packing then uses the selected package manager's pack behavior, and packed artifact
  inspection remains authoritative as decided by ADR 0019;
- final registry publishing continues to use `npm publish` as decided by ADR 0013 and ADR 0029.

The production profile should not expose `install-command`, `build-command`, `pack-command`,
`publish-command`, task-runner, arbitrary shell, arbitrary environment, arbitrary step, service, or
defaults inputs. Packages that need Turbo, Nx, Changesets, Make, or another project-specific build
orchestration layer should invoke that orchestration through their declared `scripts.build` rather
than through a reusable workflow command override.

The workflow should record whether the build script was present, whether it was executed or skipped,
and the selected package manager used for the build stage in logs and verifier-relevant provenance
or byproduct fields where appropriate. An absent build script should be represented as "skipped
because absent" or an equivalent explicit value, not hidden as an unobservable implementation
detail.

This decision does not guarantee that the package's consumer surface is correct. It does not
semantically validate `exports`, `main`, `bin`, TypeScript declarations, generated files, or package
quality beyond the metadata and packed artifact checks selected by ADR 0019. Repositories that
require tests, type checks, linting, or stricter release validation should run those checks before
calling the production publish profile, or use a future validation profile if one is defined.

### Consequences

- Good, because packages that declare a build script get a real build stage before pack.
- Good, because valid packages without a build script are not forced to add a meaningless script.
- Good, because caller workflows cannot inject arbitrary trusted build commands through workflow
  inputs.
- Good, because monorepo-specific build tools can still be used behind the package's own
  `scripts.build` contract.
- Good, because provenance can distinguish an executed build from an intentionally skipped build.
- Neutral, because packages that require tests or type checks must enforce those outside this
  publish profile unless a later validation profile is added.
- Bad, because the builder cannot detect every accidental omission of `scripts.build`.
- Bad, because some repositories may need to add or adjust `scripts.build` to fit the fixed profile
  contract.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification,
implementation, documentation, and verifier guidance define:

- exact install commands for npm, pnpm, and Yarn using frozen or immutable dependency installation;
- exact build script invocation commands for npm, pnpm, and Yarn when `scripts.build` is present;
- explicit no-op success behavior when `scripts.build` is absent;
- exact pack commands for npm, pnpm, and Yarn;
- stage ordering of install, optional build, pack, packed artifact inspection, provenance
  generation, and publish;
- provenance or byproduct fields that identify build script presence, execution or skip status,
  selected package manager, and selected package-manager version;
- documentation that package tests, type checks, linting, and consumer-surface validation are
  outside this publish profile unless encoded in `scripts.build` or provided by another workflow;
- rejection of arbitrary command, environment, step, service, task-runner, and default inputs in the
  production SLSA3 profile.

Implementation review should verify that the production workflow cannot be made to execute
caller-provided install, build, pack, or publish commands through workflow inputs, and that an
absent `scripts.build` is observable as an explicit skipped build rather than being silently
conflated with an executed build.

## Pros and Cons of the Options

### Use fixed commands and require `scripts.build`

The profile runs fixed package-manager install, build, and pack commands, and fails when the
selected package does not declare `scripts.build`.

- Good, because every release artifact has an explicit build command.
- Good, because a missing expected build script fails before publish.
- Good, because verifier guidance can treat build execution as mandatory.
- Bad, because valid no-build npm packages need dummy scripts or cannot use the initial profile.
- Bad, because the profile becomes a stronger package-shape gate than npm itself.

### Use fixed commands, run build when present, and no-op when absent

The profile runs fixed package-manager install and pack commands. It runs the selected package's
normal build script only when `scripts.build` is present; otherwise the build stage succeeds as an
explicit no-op.

- Good, because it follows common npm package practice without exposing arbitrary commands.
- Good, because build execution remains package-owned through `scripts.build`.
- Good, because no-build packages remain supported.
- Good, because provenance can record whether build was executed or skipped.
- Bad, because a package that forgot to declare `scripts.build` may still publish.

### Require an explicit no-build package metadata marker

The profile runs `scripts.build` when present and fails without a build script unless the package
declares a Windlass-specific no-build marker.

- Good, because a skipped build is explicitly declared by the package author.
- Good, because accidental build omission is less likely than with a default no-op.
- Bad, because it introduces Windlass-specific package metadata into npm packages.
- Bad, because the initial profile and verifier schema become more complex.
- Bad, because it may surprise maintainers whose package already has valid npm semantics.

### Allow a caller-provided build command

The reusable workflow accepts a workflow input such as `build-command` and executes it before pack.

- Good, because it can support complex monorepo, task-runner, and migration workflows.
- Good, because callers can preserve existing release commands more easily.
- Bad, because caller-controlled command execution weakens the production trusted workflow boundary.
- Bad, because verifier-relevant behavior becomes much harder to constrain.
- Bad, because it conflicts with the current small input surface and no arbitrary command decisions.

### Omit a separate build stage

The profile only installs dependencies and packs the package, relying on package-manager lifecycle
scripts such as `prepack`, `prepare`, or equivalent pack behavior.

- Good, because it stays close to native package-manager pack behavior.
- Good, because the reusable workflow has one fewer explicit stage.
- Bad, because build intent is less visible in provenance and documentation.
- Bad, because packages that conventionally use `scripts.build` before pack would not be built
  unless they also wire build behavior into lifecycle scripts.
- Bad, because package-manager lifecycle differences become more important to the trusted profile
  contract.

## More Information

This decision follows ADR 0014 through ADR 0019 and ADR 0023. It decides the build-stage policy for
the initial production JS/TS npm package profile only. It does not change package-manager selection,
Corepack enforcement, packed artifact validation, publish authentication, provenance generation,
registry scope, release trigger policy, or SHA-pinned builder identity.

The architecture specification should still define the exact npm, pnpm, and Yarn command lines,
including frozen install flags, workspace package invocation behavior, pack output parsing,
lifecycle script expectations, and failure handling for unsupported package-manager versions or
package layouts.
