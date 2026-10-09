---
parent: Decisions
nav_order: 90
status: accepted
date: 12026-10-08
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0088
    scope:
      "the supported pnpm consumer set boundary in the Rollout section and the
      package-manager-specific acquisition-shape list: the framing of pnpm 11.x as the sole
      supported pnpm line during the Corepack window ('pnpm 11 and Yarn Berry consumers are
      unaffected by this boundary'; the 'pnpm 11.x' acquisition-shape entry; the 'pnpm 11' dogfood
      entry in Confirmation). The supported pnpm range becomes [10.0.0, 12.0.0). The
      registry-tarball provisioning mechanism design, the pnpm 12+ exclusion during the Corepack
      window and its record/execution-gap rationale, the Yarn 6+ pre-emptive exclusion, the Yarn
      Berry v4+ boundary, and the remaining confirmation criteria stay in force"
  - type: see-also
    target: ADR-0015
  - type: see-also
    target: ADR-0017
  - type: see-also
    target: ADR-0078
  - type: see-also
    target: ADR-0088
  - type: partially-superseded-by
    target: ADR-0091
    scope:
      "the deferred selection-source axis recorded in Decision Outcome
      ('devEngines.packageManager-sourced pins of pnpm 10.x remain outside the supported set and
      keep rejecting with windlass.verify.error.pnpm-version-unsupported until the follow-up ADR
      decides that axis'), the corresponding More Information note, and the Confirmation clause
      requiring the fixture corpus to retain rejected coverage for devEngines.packageManager-sourced
      pnpm 10.x pins: the axis is decided in the opposite direction —
      devEngines.packageManager-sourced pins of pnpm 10.x are accepted. The [10.0.0, 12.0.0)
      supported pnpm range, the boundary rule, and every other ADR 0090 clause remain in force"
---

# Extend the Supported pnpm Range to the 10.x Line

## Context and Problem Statement

ADR 0088 bounded the supported pnpm consumer set to the 11.x line for as long as the ADR 0016
Corepack mechanism is the production provisioning path, excluding pnpm 12 and newer because the
Corepack shim's first-use native-binary download escapes the builder's distribution capture. For
older majors the JS/TS npm build-and-pack specification records only that they are "outside the
tested support set" — a fact, not a principled boundary.

New evidence reframes the lower bound:

- pnpm's official security policy lists `10.x` as supported through 2027-04-30, with the same
  horizon as `11.x` (`12.x ✅`; `11.x ✅ till 2027 April 30`; `10.x ✅ till 2027 April 30`;
  `<= 9.x ❌`). The 10.x line receives releases and backports (10.34.6 published 2026-09-28; a
  maintainer backport to `release/10` on 2026-08-25). Earlier majors were patched for only two to
  three months after their successor shipped; the published table is a deliberate two-line
  maintenance commitment.
- pnpm 10.x shares the acquisition shape this project can stand behind: a single JS-bundle npm
  registry tarball with SHA-512 integrity, executed through a `node` launcher, with no evidence of
  the pnpm 12-style shim that downloads a native binary at first use.
- The 11.x floor imposes a real migration cost on pnpm 10 consumers: pnpm v11 stops reading
  configuration from `.npmrc` (except auth/registry) and from `package.json#pnpm`, so moving from 10
  to 11 requires relocating settings into `pnpm-workspace.yaml`. Under the current floor, a pnpm 10
  consumer must complete that package-manager migration before it can obtain provenance.

One selection-source asymmetry bounds this decision: pnpm 10 reads and enforces the top-level
`packageManager` field (`packageManagerStrict` defaults to true; `packageManagerStrictVersion` and
`managePackageManagerVersions` are available, the latter documented as "the same field used by
Corepack"), but `devEngines.packageManager` is documented as "Added in: v11.0.0" and is unknown to
pnpm 10. Whether the builder should accept a `devEngines.packageManager`-sourced pin for a package
manager that does not implement that field is a distinct decision axis and is deferred to a
follow-up ADR.

## Decision Drivers

- The project does not issue provenance it cannot stand behind: every supported package-manager line
  must have an acquisition shape whose executed bits the builder records, under both the current
  Corepack mechanism and the ADR 0088 registry-tarball mechanism.
- The supported set should anchor to the upstream's own maintenance commitment, not to an incidental
  tested-set snapshot.
- Consumer cost matters: a support boundary that forces an unrelated configuration migration
  excludes users without a supply-chain justification.
- ADR atomicity: this ADR decides the version-floor axis only.

## Considered Options

- Keep the supported pnpm range at `[11.0.0, 12.0.0)`.
- Extend the supported pnpm range to `[10.0.0, 12.0.0)` and state the boundary rule that produces
  the set.
- State a floating rule only — support every pnpm major listed as supported upstream that has the
  registry JS-bundle shape — without fixing the set in an ADR.
- Defer the decision to the ADR 0088 mechanism rollout.

## Decision Outcome

Chosen option: "Extend the supported pnpm range to `[10.0.0, 12.0.0)`", because both decision
criteria are affirmatively satisfied today — pnpm 10.x is officially maintained through 2027-04-30,
and its registry JS-bundle shape is provisionable without design change under both the Corepack
mechanism and the ADR 0088 mechanism — and because the boundary becomes principled rather than
incidental.

### Boundary rule

The supported pnpm major set is the intersection of:

- majors marked supported in pnpm's official security policy; and
- majors with the registry JS-bundle acquisition shape (a single integrity-bearing npm registry
  tarball executed through a `node` launcher, with no out-of-band executable acquisition).

Applying this rule today yields `{10.x, 11.x}`. Re-applying the rule when the upstream table changes
— in particular at the 10.x horizon on 2027-04-30 — is a specification-phase restatement of the
supported version set under ADR 0088's existing rule and does not require a new ADR. Adding or
removing a line for reasons outside this rule does.

### What changes and what does not

- The supported pnpm range becomes `[10.0.0, 12.0.0)`. The ADR 0017 exact-version contract is
  unchanged: pins must be full three-part SemVer versions, and shortened, ranged, URL, and
  hash-suffixed descriptors remain rejected.
- pnpm 12 and newer remain excluded for the duration of the Corepack window under ADR 0088's
  unchanged record/execution-gap rationale; majors older than 10 remain outside the supported set
  (they fail the boundary rule's maintenance leg: `<= 9.x` is explicitly unsupported upstream).
- pnpm 10.x support applies to pins declared in the top-level `packageManager` field, which pnpm 10
  reads and enforces. `devEngines.packageManager`-sourced pins of pnpm 10.x remain outside the
  supported set and keep rejecting with `windlass.verify.error.pnpm-version-unsupported` until the
  follow-up ADR decides that axis; `devEngines.packageManager`-sourced pins of pnpm 11.x are
  unaffected.
- The Yarn boundaries, the ADR 0088 provisioning design, and every other support-window clause are
  unchanged.

### Technical basis for "no design change"

- Distribution shape: pnpm 10.x publishes the `pnpm` registry tarball with SHA-512 integrity and
  JS-bundle entry points (`bin/*.cjs`, `dist/pnpm.cjs`; 11.x uses `*.mjs`). Both are executed
  through a `node` launcher, the shape ADR 0088 records for pnpm 11.x; the launcher absorbs the
  module-format difference. No native-binary shim acquisition exists in either line.
- Builder coupling: the trusted core has no pnpm lockfile parser (presence check only; both lines
  write `lockfileVersion` 9.0) and no settings-value interpreter (structural workspace parsing
  only), and the reusable workflow pins Node.js 24, so pnpm 10's lower `engines.node` floor is
  irrelevant. ADR 0078 validated the settings-only workspace semantics against the pnpm 10
  documentation.
- The 10↔11 differences that do exist — `patchedDependencies` lockfile serialization, settings
  reading locations, and some install behaviors — do not intersect the builder's code; they are
  consumer-documentation concerns.

### Verification obligations for the specification and implementation phase

- A live verification of pnpm 10.x install, build, and pack on the builder's Node.js 24 runtime,
  using a real pnpm 10-generated lockfile.
- Confirmation of Corepack's `pnpm@10.x` preparation path and its download hosts.
- Review of the reusable workflow's `pnpm config list` redaction comment (written against pnpm v11)
  for v10 accuracy.
- Fixture taxonomy: convert the rejected `pnpm-10-unsupported` coverage into accepted coverage, add
  accepted pnpm 10.x fixtures (including a real pnpm 10 lockfile), and keep rejected coverage for
  `devEngines`-sourced 10.x pins, for pnpm 12+, and for pre-10 majors.
- Specification sweep: the JS/TS npm build-and-pack specification, the package-profile
  specification, the verification-policy and fixture requirements, and the diagnostics table must
  restate the bound and the boundary rule.

### Confirmation

This decision is confirmed when:

- the JS/TS npm build-and-pack specification and the package-profile specification state the
  `[10.0.0, 12.0.0)` pnpm bound, the boundary rule, and the top-level-`packageManager` source
  restriction for 10.x pins;
- the verification policy and fixture corpus cover accepted pnpm 10.x pins and retain rejected
  coverage for `devEngines`-sourced 10.x pins, pre-10 majors, and pnpm 12+;
- the live verification items above pass, including a pnpm 10 consumer dogfood with records that
  bind the executed distribution;
- a standing item tracks the 2027-04-30 pnpm 10.x support horizon, at which the boundary rule is
  re-applied and the floor restated.

## Pros and Cons of the Options

### Keep the supported pnpm range at `[11.0.0, 12.0.0)`

- Good, because the tested support set stays minimal: one pnpm line of fixtures, dogfood runs, and
  patch-cadence watching, and no new ADR.
- Bad, because it excludes an officially maintained user base whose security-support horizon runs to
  2027-04-30.
- Bad, because pnpm 10 consumers must complete an unrelated settings migration
  (`.npmrc`/`package.json#pnpm` → `pnpm-workspace.yaml`) before they can obtain provenance.
- Bad, because "outside the tested support set" remains a fact rather than a principled boundary.

### Extend the supported pnpm range to `[10.0.0, 12.0.0)` (chosen)

- Good, because the supported set coincides with pnpm's official support table — an external,
  verifiable anchor shared with consumers.
- Good, because both lines share the registry JS-bundle shape, so neither the Corepack mechanism nor
  the ADR 0088 mechanism needs a design change.
- Good, because the boundary rule makes future set changes (the 2027-04-30 horizon, future majors)
  mechanical rule applications instead of ad-hoc decisions.
- Bad, because the pnpm verification surface doubles: accepted fixtures and a live dogfood for a
  second line, plus an ongoing watch on two patch cadences.
- Bad, because a scheduled support-set change is now on the calendar: the floor is restated when
  10.x support ends.

### State a floating rule only

- Good, because no per-major decision record is needed and the rule self-executes on upstream table
  changes.
- Bad, because the supported set moves without a reviewed restatement, hurting consumer
  predictability and the project's evidence-bound posture.
- Bad, because acquisition-shape judgement is still per-major manual work (pnpm 12's shim is the
  counterexample), so the rule cannot actually run unattended.

### Defer the decision to the ADR 0088 mechanism rollout

- Good, because no work happens now and acquisition shapes will be re-verified at the transition
  anyway.
- Bad, because the 10.x support horizon (2027-04-30) may expire before the rollout, making the
  question moot while consumers are excluded in the meantime.
- Bad, because both decision criteria are confirmed today, so delay buys no information.

## More Information

- pnpm security policy (support table): <https://github.com/pnpm/pnpm/security/policy> —
  `10.x ✅ till 2027 April 30`, checked 12026-10-08.
- pnpm 10.x settings (`packageManagerStrict`, `packageManagerStrictVersion`,
  `managePackageManagerVersions`): <https://pnpm.io/10.x/settings> — the top-level `packageManager`
  field is read and name-enforced by default.
- pnpm 11.x `package.json` documentation: <https://pnpm.io/11.x/package_json> —
  `devEngines.packageManager` is "Added in: v11.0.0" and supports version ranges resolved into
  `pnpm-lock.yaml`.
- pnpm v10→v11 migration guide: <https://pnpm.io/11.x/migration> — configuration reading changes and
  the `pmOnFail` consolidation of the v10 strictness settings.
- npm registry facts checked 12026-10-08: `latest-10` = 10.34.6 (2026-09-28), `latest-11` = 11.28.2
  / `next-11` = 11.28.5 (2026-10-06), `latest` = 12.10.1; active 10.x backport evidence in
  pnpm/pnpm#14169 (2026-08-25).
- Investigated but undecided (follow-up ADR candidate): whether `devEngines.packageManager` may
  select pnpm 10.x — acceptance keeps the source set uniform across versions (the field is a
  builder/Corepack provisioning contract and pnpm 10 ignores unknown manifest fields), rejection
  keeps selection sources to fields the selected package manager implements.
- Not a differentiator and therefore not decided here: pnpm 10's `engines.runtime` /
  `devEngines.runtime` runtime-download feature exists from v10.14/v10.21 and in later majors alike,
  so it does not distinguish 10.x from the currently supported 11.x.
