---
parent: Decisions
nav_order: 78
status: accepted
date: 12026-08-13
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0018
    scope:
      "pnpm root-package selection when pnpm-workspace.yaml omits packages; one-package-per-run and
      explicitly selected workspace-package behavior remain in force"
---

# Treat Settings-Only pnpm-workspace.yaml as Standalone Root Package Mode

## Context and Problem Statement

ADR 0018 allows one profile run to select either the repository root package or one explicit
workspace package. The JS/TS npm build-pack specification implements that boundary by discovering
workspace metadata and failing closed when package ownership is malformed or ambiguous.

The first live npm dogfood of `vers-js`,
[run 31622651874](https://github.com/windlasstech/vers-js/actions/runs/31622651874), exposed an
over-broad malformed-metadata classification. `vers-js` is a single root package managed by pnpm.
Its `pnpm-workspace.yaml` contains policy settings such as release-age and trust policy controls but
does not contain a `packages` member. The specification classified that omission as malformed, so
package resolution correctly failed closed with `windlass.verify.error.package-resolution-invalid`
and aborted the publish. The fail-closed behavior worked as designed, but the metadata
classification did not match pnpm semantics.

pnpm documents `packages` as optional: when it is omitted, only the root package is included, and
the root package is always included even when package patterns exist. Modern pnpm also reads
non-authentication configuration, including policy settings, from `pnpm-workspace.yaml`; therefore a
settings-only file is valid root-only workspace configuration rather than evidence of malformed
workspace metadata. The N01 fixture corpus did not include this valid shape.

Should Windlass reject every `pnpm-workspace.yaml` without `packages`, ignore the file entirely, or
treat the omission as an explicit root-only package boundary while preserving fail-closed handling
for malformed or ambiguous workspace metadata?

## Decision Drivers

- Match documented pnpm behavior for an omitted optional `packages` member.
- Admit modern pnpm repositories that keep policy settings in `pnpm-workspace.yaml` without defining
  subpackage globs.
- Keep the selected root `package.json` authoritative for root package identity and package-manager
  selection.
- Preserve ADR 0018's one-package-per-run boundary and ADR 0023's exact `package-directory`
  selection.
- Continue to fail closed when workspace membership is malformed or genuinely ambiguous.
- Avoid interpreting arbitrary pnpm-native glob syntax beyond the profile's supported subset.
- Preserve the registered package-resolution diagnostic taxonomy.

## Considered Options

- Treat an absent `packages` member as standalone-root-package mode.
- Continue treating an absent `packages` member as malformed metadata.
- Ignore a `pnpm-workspace.yaml` without `packages` and continue ancestor discovery.
- Adopt pnpm-native workspace discovery and pattern semantics without restriction.

## Decision Outcome

Chosen option: "Treat an absent `packages` member as standalone-root-package mode", because it
matches pnpm's documented root-only behavior without weakening fail-closed handling for malformed or
ambiguous workspace declarations.

A `pnpm-workspace.yaml` that parses as a YAML object and omits `packages` is valid. For Windlass
package resolution, that candidate has zero workspace patterns and selects only the candidate root
package. The caller's selected `package-directory` must resolve to that same candidate root, and the
root `package.json` remains authoritative for package identity and package-manager metadata. The
settings-only file does not claim any subdirectory package and must not cause Windlass to search for
or infer undeclared workspace members.

When `packages` is present, existing workspace behavior remains unchanged. Its value must be an
array of strings, every pattern must use the profile's supported syntax, and ownership must identify
the exact selected package without ambiguity. A non-array value, a non-string member, unsupported
pattern syntax, conflicting ownership, or another malformed shape still fails closed with the
registered package-resolution diagnostic. No diagnostic ID or taxonomy changes as part of this
decision.

The root-only classification is terminal for that candidate: Windlass must not ignore a valid
settings-only file and let a farther ancestor claim a different package. If the selected
`package-directory` is not that candidate root, package resolution fails closed; the settings-only
file cannot claim a subdirectory or be bypassed to reinterpret the root-only repository as a
workspace or standalone subpackage layout. Genuinely ambiguous multi-package resolution remains
rejected.

This decision amends ADR 0018 rather than partially superseding it. ADR 0018's root-package,
selected-workspace-package, and one-package-per-run clauses still govern exactly as written; this
decision only qualifies how one valid pnpm metadata shape maps to the existing root-package case.
ADR 0014's pnpm support set and ADR 0015's manifest-first package-manager precedence are unchanged.

### Consequences

- Good, because settings-only `pnpm-workspace.yaml` files no longer make valid single-root-package
  repositories fail package resolution.
- Good, because Windlass matches pnpm's documented rule that omitting `packages` yields root-only
  membership.
- Good, because the root `package.json` remains the authoritative package and package-manager
  metadata source.
- Good, because malformed present `packages` values, unsupported patterns, and ambiguous ownership
  continue to fail closed.
- Good, because the diagnostic taxonomy remains stable; the existing
  `windlass.verify.error.package-resolution-invalid` ID still reports genuine resolution failures.
- Neutral, because settings in `pnpm-workspace.yaml` do not become workspace package patterns and do
  not add any selected package.
- Bad, because N02 package resolution and its workspace fixtures require a follow-up implementation
  change before the newly admitted shape works in production.
- Bad, because the first dogfood publish must be retried after implementation to close the live
  confirmation criterion.

The N02 follow-up must add an accepted settings-only `pnpm-workspace.yaml` fixture that resolves the
standalone root package and a rejected fixture whose present `packages` value has the wrong type. It
must not modify diagnostic IDs. After that implementation lands, the `vers-js` dogfood must be
retried with `workflow_dispatch` and `release_tag=v0.1.2`.

### Confirmation

This decision is confirmed when:

- an accepted fixture with a settings-only `pnpm-workspace.yaml`, no `packages` member, and a root
  `package.json` resolves exactly the standalone root package;
- a rejected fixture with a present non-array `packages` member fails closed with
  `windlass.verify.error.package-resolution-invalid`;
- present package patterns using unsupported syntax continue to fail closed with the same registered
  diagnostic;
- a valid present `packages` array retains the existing nearest-workspace-root and exact-selected-
  package behavior;
- genuinely ambiguous multi-package resolution remains rejected;
- no diagnostic taxonomy or registered ID changes;
- the N02 resolution/workspace implementation and both required fixtures land in a follow-up PR; and
- the `vers-js` dogfood retry using `workflow_dispatch` with `release_tag=v0.1.2` passes package
  resolution before proceeding to later release stages.

## Pros and Cons of the Options

### Treat absent `packages` as standalone-root-package mode

- Good, because it matches pnpm's documented root-only semantics.
- Good, because valid policy-only configuration remains usable by single-package repositories.
- Good, because it adds no workspace members and preserves exact package selection.
- Bad, because implementation and fixture behavior must change from the current over-rejection.

### Continue treating absent `packages` as malformed

- Good, because it preserves the current implementation unchanged.
- Bad, because it contradicts pnpm's documented optional field semantics.
- Bad, because it rejects common modern pnpm configuration and already blocked live dogfood.

### Ignore the settings-only file and continue ancestor discovery

- Good, because settings that do not enumerate packages would not directly define subpackage
  membership.
- Bad, because ignoring the file loses pnpm's root-only workspace boundary and may let a farther
  ancestor claim the root inconsistently.
- Bad, because it treats valid metadata as absent instead of interpreting its documented meaning.

### Adopt unrestricted pnpm-native workspace semantics

- Good, because pnpm itself would define all membership behavior.
- Bad, because it would expand the trusted profile to unsupported glob features and version-specific
  behavior.
- Bad, because it would weaken the current auditable and deterministic pattern subset.

## More Information

This finding was discovered during the first live publish tracked by
[issue #30](https://github.com/windlasstech/slsa-builder/issues/30). The failed run emitted
`windlass.verify.error.package-resolution-invalid`; that evidence shows the fail-closed path is
effective and only the absent-`packages` classification needs correction.

Reference points:

- pnpm's [`pnpm-workspace.yaml` documentation](https://pnpm.io/pnpm-workspace_yaml) states that when
  `packages` is omitted only the root package is included and that the root package is always
  included;
- the versioned [pnpm 10 workspace documentation](https://pnpm.io/10.x/pnpm-workspace_yaml) records
  the same optional-`packages` behavior;
- pnpm's [settings documentation](https://pnpm.io/settings) identifies `pnpm-workspace.yaml` as a
  configuration source and requires non-authentication settings there or in the global
  configuration; and
- pnpm's [migration documentation](https://pnpm.io/migration) describes the continuing move of
  non-authentication settings into `pnpm-workspace.yaml`.
