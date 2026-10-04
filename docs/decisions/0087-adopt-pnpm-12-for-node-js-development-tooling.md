---
parent: Decisions
nav_order: 87
status: accepted
date: 12026-10-04
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0010
    scope:
      "the pnpm version line in force for development tooling: ADR 0010 chose pnpm and its
      implementation initially ran the pnpm 11 line (pinned via devEngines.packageManager); this ADR
      adopts the pnpm 12 line as the development-tooling pnpm major. The choice of pnpm itself, the
      committed-lockfile and cooldown policies, and every other ADR 0010 clause remain in force"
  - type: see-also
    target: ADR-0086
  - type: see-also
    target: ADR-0089
---

# Adopt pnpm 12 for Node.js Development Tooling

## Context and Problem Statement

ADR 0010 chose pnpm for Node.js development tooling (Prettier and markdownlint-cli2 only); its
implementation has run the pnpm 11 line, pinned through `devEngines.packageManager`. ADR 0086 then
replaced Corepack-based provisioning with mise's packageManager-field resolution — a mechanism that
is version-agnostic across pnpm majors and installs pnpm from the aqua backend's standalone
binaries. The remaining question is which pnpm major line the repository should run.

The relevant facts, verified 12026-10-04:

- **pnpm 12 is the current mainline.** pnpm 12.0 shipped 2026-08-26 as a Rust rewrite distributed as
  a standalone native executable that does not require Node.js after installation
  ([release notes](https://pnpm.io/blog/releases/12.0)). The `latest` npm dist-tag now points at the
  12 line (12.8.1 at evaluation time); the 11 line (`latest-11` 11.28.2) continues to receive fixes
  in lockstep — 11.28.3 and 12.9.0 shipped within days of the evaluation — but pnpm has published no
  EOL policy for 11, so staying is an open-ended legacy-line commitment.
- **Configuration compatibility is fully verified.** All nine `pnpm-workspace.yaml` policy keys this
  repository uses — `minimumReleaseAge`, `minimumReleaseAgeIgnoreMissingTime`,
  `minimumReleaseAgeStrict`, `trustPolicy`, `blockExoticSubdeps`, `verifyStoreIntegrity`,
  `lockfile`, `preferFrozenLockfile`, and `allowBuilds` — are documented settings in pnpm 12
  ([settings reference](https://pnpm.io/settings/dependency-resolution)). pnpm 12's new
  unrecognized-workspace-setting error (`ERR_PNPM_UNRECOGNIZED_WORKSPACE_SETTINGS`) therefore does
  not affect this repository and instead guards against future configuration typos.
- **The lockfile contract is unchanged.** pnpm 12 keeps pnpm 11's commands, flags, settings, and
  lockfile format ([What's different in pnpm 12](https://pnpm.io/blog/whats-different-in-pnpm-12)).
  The one documented source of lockfile churn — deterministic cycle breaking rewriting peer variants
  of cyclic dependency graphs on first re-resolve — does not apply to this repository's dependency
  set (Prettier, markdownlint-cli2), which contains no cycles.
- **The known external gap is latent.** GitHub's Dependabot documentation lists pnpm support only
  for v7–v10, with open issues for 12; this repository has no `dependabot.yml`, and its actual
  supply-chain layers (the `pnpm-workspace.yaml` policy above, the committed lockfile, Dependency
  Review, and OSV Scanner) are agnostic to the pnpm version.
- **The risk surface is minimal and reversible.** The pnpm major affects only development-only
  format/lint tooling; the trusted Go core and the production reusable workflow do not consume it,
  and a rollback is a single-line change to the version declaration.

Should the repository stay on the pnpm 11 line or adopt the pnpm 12 line for development tooling?

## Decision Drivers

- Preserve the fail-closed supply-chain policy stack (cooldown with strict mode, trust policy,
  exotic-subdependency blocking, store integrity verification, frozen lockfile) without
  re-expression or weakening.
- Adopt a ground-up rewrite only after verifying configuration compatibility against primary
  sources.
- Keep the blast radius confined to development tooling and trivially reversible.
- Track the actively mainlined release line rather than accreting time on a legacy line with no
  published EOL policy.
- Compose with ADR 0086's provisioning: mise's aqua backend installs pnpm 12's standalone binary
  with GitHub artifact attestation verification and `mise.lock` checksum recording.
- Avoid lockfile churn and keep Dependency Review / OSV Scanner behavior unchanged.

## Considered Options

- Stay on the pnpm 11 line (bumping the pin to the current 11.28.x).
- Adopt the pnpm 12 line (pinning an explicit 12.x version at implementation time).

## Decision Outcome

Chosen option: "Adopt the pnpm 12 line", because configuration compatibility is verified against
primary sources, the lockfile and policy contracts are unchanged, the risk surface is dev-only and
one-line reversible, and staying on 11 would be an unbounded commitment to a legacy line.

Configuration consequences (implementation detail, recorded here for the implementer):

- `package.json`'s `devEngines.packageManager` is updated to declare the pnpm 12 line. The concrete
  declaration value is selected at implementation time through the normal update procedure (mise
  `minimum_release_age` plus the repository cooldown policy apply); note that ADR 0086 removed
  Corepack, whose exact-version-only field syntax was the original reason for pinning an exact patch
  version in the manifest.
- `mise lock` is regenerated so `mise.lock` records the pnpm 12 standalone binary with checksum and
  provenance metadata.
- `pnpm install` is expected to leave `pnpm-lock.yaml` unchanged; any diff is reviewed and committed
  separately as a migration artifact, not silently absorbed.
- The Prettier and markdownlint gates are smoke-tested locally and in CI; `CHANGELOG.md` records the
  change under `[Unreleased]` → `Changed`.
- Rollback path: revert the version declaration to an explicit 11.28.x version.

### Consequences

- Good, because the repository tracks the actively mainlined pnpm line instead of a legacy line with
  no published EOL policy.
- Good, because the pnpm 12 standalone binary composes with ADR 0086's provisioning: no Node.js is
  required to install the package manager itself, and the aqua backend's GitHub artifact attestation
  verification plus `mise.lock` checksums apply unchanged.
- Good, because all nine fail-closed policy keys keep working as configured, and pnpm 12's
  unrecognized-setting error hardens the configuration against future typos.
- Good, because the lockfile format is unchanged, so Dependency Review, OSV Scanner, and the
  committed-lockfile review workflow see no new surface.
- Good, because rollback is a one-line version-declaration revert if a pnpm 12 defect surfaces.
- Neutral, because the young Rust line is patching rapidly (12.8.0 → 12.9.0 within days); pin values
  move through the normal cooldown-respecting update procedure rather than tracking tags.
- Bad, because early-rewrite defects may still surface (for example the `@pnpm/napi` memory
  regression reported in 12.8.0 and fixed in subsequent patches); the dev-only blast radius and the
  rollback path bound this.
- Bad, because if Dependabot version updates are ever configured for this repository, pnpm 12's
  undocumented support status (v7–v10 listed today) must be re-verified first.

### Confirmation

This decision is confirmed when:

- `package.json`'s `devEngines.packageManager` declares the pnpm 12 line.
- `mise install` provisions pnpm 12 from that declaration and `pnpm --version` reports a 12.x
  version.
- `pnpm install` runs without `ERR_PNPM_UNRECOGNIZED_WORKSPACE_SETTINGS`, and any `pnpm-lock.yaml`
  diff is reviewed and committed separately with an explanation.
- All format gates (`pnpm exec prettier --check .`, `pnpm exec markdownlint-cli2 "**/*.md"`) pass
  locally and in CI under pnpm 12.
- `CHANGELOG.md` carries the change under `[Unreleased]` → `Changed`.

## Pros and Cons of the Options

### Stay on the pnpm 11 line

Keep the TypeScript/Node-based pnpm 11 line, bumping the pin to the current 11.28.x.

- Good, because the 11 line is mature and still receives fixes in lockstep with 12 (11.28.3 shipped
  days before this evaluation).
- Good, because no migration step of any kind is required beyond the overdue patch bump.
- Bad, because `latest` has moved to 12: staying is a legacy-line commitment with no published EOL
  policy, so the migration this ADR performs would only grow larger whenever it eventually happens.
- Bad, because it forgoes pnpm 12's improvements that matter to this repository — the standalone
  Node-free binary that matches ADR 0086's provisioning, deterministic lockfiles, and the
  unrecognized-setting guard.

### Adopt the pnpm 12 line

Move development tooling to the Rust-based pnpm 12 line with an explicitly pinned version.

- Good, because all nine repository policy keys are verified as recognized pnpm 12 settings, so the
  fail-closed supply-chain stack carries over unchanged.
- Good, because the lockfile format and commands are unchanged, so there is no lockfile migration
  and no GitHub-tooling impact.
- Good, because the standalone binary removes Node.js from the package-manager bootstrap path
  entirely, aligning with ADR 0086.
- Good, because the change is confined to development tooling and reverts with one line.
- Neutral, because rapid early patching means version bumps arrive frequently; the cooldown policy
  already governs their adoption.
- Bad, because early-rewrite defects may appear between evaluation and adoption; mitigated by the
  dev-only surface, explicit pinning, and the rollback path.
- Bad, because Dependabot — should this repository ever enable it — does not yet document pnpm 12
  support and would need re-verification at that time.

## More Information

Watch items:

- **Dependabot pnpm 12 support.** GitHub's supported-versions list currently names pnpm v7–v10 only
  ([supported package managers](https://docs.github.com/en/code-security/dependabot/dependabot-version-updates/configuration-options-for-the-dependabot.yml-file)).
  If this repository ever adds a `dependabot.yml`, re-verify pnpm 12 support first.
- **pnpm 11 EOL announcement.** If pnpm publishes an EOL date for the 11 line, record it here for
  traceability; this ADR already makes the repository independent of it.

Out of scope, unchanged from ADR 0086's deferral: the development Node.js version (24 versus 26) is
a separate follow-on decision.

References consulted:

- pnpm 12.0 release notes: <https://pnpm.io/blog/releases/12.0>
- What’s different in pnpm 12: <https://pnpm.io/blog/whats-different-in-pnpm-12>
- pnpm settings reference (dependency resolution, store & lockfile, build):
  <https://pnpm.io/settings/dependency-resolution>
- pnpm `devEngines.packageManager` documentation:
  <https://pnpm.io/package_json#devenginespackagemanager>
- pnpm registry release cadence (dist-tags and per-version timestamps):
  <https://registry.npmjs.org/pnpm>
- GitHub Dependabot supported package managers:
  <https://docs.github.com/en/code-security/dependabot/dependabot-version-updates/configuration-options-for-the-dependabot.yml-file>
- mise changelog (v2026.9.17, pnpm 12 minimum-release-age forwarding):
  <https://github.com/jdx/mise/blob/main/CHANGELOG.md>
