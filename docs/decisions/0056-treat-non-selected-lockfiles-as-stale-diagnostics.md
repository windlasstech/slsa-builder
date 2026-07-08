---
parent: Decisions
nav_order: 56
status: accepted
date: 12026-07-09
decision-makers: Yunseo Kim
---

# Treat Non-Selected Lockfiles as Stale Diagnostics

## Context and Problem Statement

ADR 0015 selected manifest-first package manager selection for the initial JS/TS npm package
profile. The selected order is `packageManager`, then `devEngines.packageManager`, then lockfile
inference. ADR 0015 also says the profile should fail when manifest fields and lockfile signals
conflict.

That conflict rule is intentionally strict when the workflow cannot identify one safe package
manager and one dependency graph. However, architecture review surfaced an ambiguity: when manifest
metadata explicitly selects a supported package manager and the matching required lockfile exists,
should additional non-selected lockfiles in the same package manager root be treated as a conflict
or as stale repository diagnostics?

For example, a package may declare `packageManager: "pnpm@10.0.0"` and commit `pnpm-lock.yaml`, but
also retain an old `package-lock.json` from a previous migration. The selected pnpm release install
does not consume `package-lock.json`; failing the release may be stricter than necessary, while
silently ignoring it without provenance can hide migration leftovers.

Should the initial production profile reject every non-selected supported lockfile, ignore them
silently, or allow them only as recorded stale diagnostics when the manifest-selected package
manager has its required lockfile?

## Decision Drivers

- Preserve ADR 0015's manifest-first package manager selection model.
- Preserve ADR 0017's exact version enforcement for pnpm and Yarn.
- Fail closed when the selected package manager cannot run a reproducible frozen install.
- Avoid treating migration leftovers as trusted dependency graph inputs.
- Avoid unnecessary release failures when explicit manifest metadata and the selected lockfile
  already identify one package manager and one dependency graph.
- Make stale lockfile presence visible in provenance and verifier diagnostics.
- Keep lockfile-only inference strict when no manifest metadata selects a package manager.

## Considered Options

- Allow non-selected lockfiles as recorded stale diagnostics when manifest metadata selects a
  manager and the selected manager's required lockfile is present.
- Treat any non-selected supported lockfile as a package-manager conflict.
- Ignore non-selected supported lockfiles silently.
- Add a caller workflow input to choose whether stale lockfiles are allowed.

## Decision Outcome

Chosen option: "Allow non-selected lockfiles as recorded stale diagnostics when manifest metadata
selects a manager and the selected manager's required lockfile is present", because explicit
manifest metadata plus the selected lockfile is sufficient to identify the package manager and
dependency graph consumed by the release install. Non-selected lockfiles in that state are
repository hygiene signals, not competing release dependency graph inputs.

For the initial JS/TS npm package profile, a supported lockfile for a non-selected package manager
is not a conflict when all of the following are true:

1. `packageManager` or `devEngines.packageManager` selected one supported package manager.
2. For pnpm and Yarn, the selected manifest metadata includes an exact version that can be enforced
   through Corepack.
3. The package manager root contains the required lockfile for the selected package manager:
   `package-lock.json` for npm, `pnpm-lock.yaml` for pnpm, or `yarn.lock` for Yarn.
4. The install command uses the selected package manager's frozen or reproducible install mode.
5. The implementation records the ignored non-selected supported lockfile paths in verifier-relevant
   provenance diagnostics.

In that allowed state, the non-selected lockfiles must not be used for package-manager selection,
dependency graph construction, install command selection, provenance dependency claims, or verifier
acceptance beyond the stale-diagnostic field.

The profile must still fail with a package-manager conflict when any of the following are true:

- no manifest metadata selects a package manager and multiple supported lockfiles are present;
- manifest metadata selects a package manager but the selected manager's required lockfile is
  absent;
- manifest fields at different selection priorities select different package managers;
- the selected manifest metadata names an unsupported package manager;
- pnpm or Yarn is selected without an enforceable exact version;
- lockfile-only inference identifies pnpm or Yarn without exact manifest metadata;
- a caller attempts to override the package manager through workflow inputs.

This decision narrows ADR 0015's "manifest fields and lockfile signals conflict" rule. A lockfile
signal conflicts with manifest-selected metadata only when it would make selection, version
enforcement, or the consumed dependency graph ambiguous. A non-selected supported lockfile is not a
conflicting signal when the selected manifest metadata and selected lockfile already define the
release install unambiguously.

### Consequences

- Good, because explicit manifest metadata remains authoritative for package-manager selection.
- Good, because pnpm and Yarn still require exact package-manager versions before release execution.
- Good, because stale lockfiles are visible to reviewers and verifiers without becoming dependency
  graph inputs.
- Good, because package-manager migrations do not fail releases solely because an old lockfile
  remains after the selected lockfile and manifest metadata are correct.
- Neutral, because repositories may still choose to clean up stale lockfiles for maintainability
  even though the profile accepts them as diagnostics.
- Bad, because verifier policy and provenance schema need a dedicated stale-lockfile diagnostic
  field.
- Bad, because users may incorrectly assume all committed lockfiles influenced the release unless
  the documentation and provenance make the selected lockfile clear.

### Confirmation

This decision is confirmed when the architecture specifications, implementation, fixtures, and
verification guidance define:

- selected-manager lockfile requirements for npm, pnpm, and Yarn;
- allowed stale-lockfile behavior only after manifest metadata selects a supported package manager
  and the selected manager's required lockfile is present;
- failure behavior when no manifest metadata exists and multiple supported lockfiles are present;
- failure behavior when the selected manager's required lockfile is absent;
- provenance fields that record ignored non-selected supported lockfile paths as diagnostics;
- verifier behavior that treats recorded ignored lockfiles as diagnostics only, not as selected
  dependency graph inputs;
- fixtures for manifest-selected npm, pnpm, and Yarn packages that include stale non-selected
  lockfiles and still pass;
- fixtures for lockfile-only ambiguity, missing selected lockfile, unsupported manifest manager, and
  pnpm/Yarn without exact manifest version that fail.

## Pros and Cons of the Options

### Allow non-selected lockfiles as recorded stale diagnostics

When manifest metadata selects a supported package manager and the selected manager's required
lockfile exists, non-selected supported lockfiles are ignored for release execution but recorded in
provenance diagnostics.

- Good, because the release dependency graph remains determined by the selected package manager and
  its own lockfile.
- Good, because verifier policy can distinguish selected lockfile inputs from stale repository
  files.
- Good, because this matches package-manager behavior: npm does not consume `pnpm-lock.yaml`, pnpm
  does not consume `package-lock.json`, and Yarn does not consume either as its selected lockfile.
- Bad, because the policy is less strict than rejecting all stale repository state.

### Treat any non-selected supported lockfile as a conflict

Reject a manifest-selected pnpm package that also contains `package-lock.json`, a manifest-selected
npm package that also contains `pnpm-lock.yaml`, and equivalent cases.

- Good, because it gives repositories a clean one-lockfile release surface.
- Good, because it is the strictest reading of ADR 0015's conflict language.
- Bad, because it fails releases even when the selected package manager and dependency graph are not
  ambiguous.
- Bad, because it can block package-manager migrations for stale files that the selected tool does
  not consume.

### Ignore non-selected supported lockfiles silently

Proceed with the selected package manager and do not record other lockfiles.

- Good, because it is operationally simple.
- Bad, because reviewers and verifiers cannot tell whether a non-selected lockfile existed.
- Bad, because it weakens the audit trail for package-manager migrations.

### Add a caller workflow input to choose stale-lockfile behavior

Let callers opt into strict or lenient handling through a workflow input.

- Good, because repositories could choose their preferred hygiene level.
- Bad, because caller-controlled package-manager policy inputs conflict with ADR 0015's small,
  manifest-first trust boundary.
- Bad, because the override would become verifier-relevant and increase public API complexity.

## More Information

This decision follows ADR 0015 and ADR 0017. It does not change package-manager selection order,
Corepack use, exact pnpm/Yarn version enforcement, supported package managers, install command
semantics, or the prohibition on caller package-manager overrides.

Reference points considered:

- npm, pnpm, and Yarn each consume their own lockfile format for frozen or reproducible installs.
- Stale lockfiles can appear during package-manager migration without affecting the selected package
  manager's install command.
- Verifier policy needs to know which lockfile constrained the release install and which lockfiles
  were ignored as diagnostics.
