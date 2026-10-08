---
parent: Decisions
nav_order: 91
status: accepted
date: 12026-10-08
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0090
    scope:
      "the deferred selection-source axis recorded in Decision Outcome
      ('devEngines.packageManager-sourced pins of pnpm 10.x remain outside the supported set and
      keep rejecting with windlass.verify.error.pnpm-version-unsupported until the follow-up ADR
      decides that axis') and the corresponding More Information note: the axis is decided in the
      opposite direction — devEngines.packageManager-sourced pins of pnpm 10.x are accepted. The
      [10.0.0, 12.0.0) supported pnpm range, the boundary rule, and every other ADR 0090 clause
      remain in force"
  - type: partially-supersedes
    target: ADR-0063
    scope:
      "the exclusion of devEngines.packageManager alone as a Yarn selection source (the Confirmation
      clause that Yarn release builds never proceed from devEngines.packageManager alone, and the
      top-level packageManager metadata requirement as the sole Yarn selection path): Yarn becomes
      selectable from devEngines.packageManager with an exact version in the supported Berry range.
      The Berry v4+ version boundary, the rejection of version ranges and yarn.lock-only inference,
      and the immutable install requirement remain in force"
  - type: see-also
    target: ADR-0015
  - type: see-also
    target: ADR-0017
  - type: see-also
    target: ADR-0092
---

# Accept devEngines.packageManager as a Uniform Selection Source

## Context and Problem Statement

The profile selects the package manager from up to four manifest fields before lockfile inference
(ADR 0015): the top-level `packageManager` field and the `devEngines.packageManager` field, each in
the selected package manifest and in the workspace root manifest. Historically the two fields were
treated asymmetrically: pnpm is selectable from both, but ADR 0063 requires that Yarn release builds
never proceed from `devEngines.packageManager` alone, because Yarn itself does not implement the
field. ADR 0090 extended the supported pnpm range to `[10.0.0, 12.0.0)` and deferred, as an interim
default, whether `devEngines.packageManager`-sourced pnpm 10.x pins are accepted — pnpm 10 predates
the field ("Added in: v11.0.0" per the pnpm 11.x documentation).

Verification after ADR 0090 established that the asymmetry's premise does not hold under the
production provisioning mechanism. The Corepack documentation states: "If the top-level
`packageManager` field is missing, Corepack will use the package manager defined in
`devEngines.packageManager`", and Corepack additionally uses the field to validate a compatible
package manager, with `onFail` governing mismatch behavior (error by default). Live verification
against Corepack 0.36.0 on Node.js 24.21.0 (12026-10-08) confirmed:

- a manifest carrying only `devEngines.packageManager` for Yarn 4.18.1 runs Yarn 4.18.1 through
  Corepack;
- a manifest carrying only `devEngines.packageManager` for pnpm 10.34.6 runs pnpm 10.34.6 through
  Corepack;
- a manifest whose top-level `packageManager` and `devEngines.packageManager` disagree fails with a
  mismatch error.

So under the Corepack mechanism the toolchain chain — the builder plus Corepack — reads, validates,
and provisions from `devEngines.packageManager` for every supported package manager, including the
ones whose binaries do not implement the field. And under the ADR 0088 registry-tarball mechanism,
the trusted core reads the manifest fields itself and provisions exactly the pinned version with
post-provisioning assertion, so pin enforcement is builder-side under both mechanisms. Rejecting
such pins therefore buys contract semantics, not build integrity.

The field's documented purpose is precisely "the package manager and version this project is
developed with", and it has consumers beyond the selected package manager: npm's CLI enforces it
(observed as `EBADDEVENGINES` against this repository's own manifest), and mise resolves development
pins from `package.json` fields (ADR 0089). Consumers in mixed-tooling repositories legitimately
carry it as their single pin declaration.

The question: may any supported package manager be selected from `devEngines.packageManager`, with
the same contract as the top-level field — accepting that Yarn's existing exclusion is superseded
while the project is pre-release, when the reversal is cheapest?

## Decision Drivers

- Align the selection contract with the verified behavior of the production toolchain: the builder
  should not reject a declaration the toolchain would honor.
- One uniform rule over per-manager special cases: every removed special case is a diagnostic, a
  fixture class, and a spec clause the project does not have to maintain.
- Honor the consumer's documented intent: the field exists to declare the project's package manager;
  a consumer writing it means it.
- Do not issue provenance the project cannot stand behind: either way, the executed version is
  pinned, digest-bound at provisioning, asserted after provisioning, and the selection source is
  recorded in provenance — build integrity is identical under both answers.
- Pre-release timing: reversing ADR 0063's Yarn exclusion is inexpensive now and would be a breaking
  change later; if the policies are ever to be unified, 0.x is when.

## Considered Options

- Restrict selection sources to fields the selected package manager implements; pnpm 10.x
  `devEngines.packageManager`-sourced pins stay rejected, and ADR 0063's Yarn posture stands.
- Accept `devEngines.packageManager`-sourced pins for every supported package manager, partially
  superseding ADR 0063 so Yarn is also selectable from `devEngines.packageManager`.
- Accept such pins but record a non-fatal annotation that the selected package manager does not
  implement the field.
- Keep ADR 0090's interim rejection for pnpm 10.x without stating a principle, and revisit on
  consumer demand.

## Decision Outcome

Chosen option: "Accept `devEngines.packageManager`-sourced pins for every supported package
manager", because the verified facts collapse the rejection's central premise — the toolchain reads
and enforces the field for every supported manager throughout the Corepack window, and the builder
enforces the pin itself under both provisioning mechanisms — while the costs of uniform acceptance
(a Yarn policy reversal and a spec sweep) are at their lowest during the pre-release period, and the
uniform rule deletes the Yarn special case instead of adding a pnpm one.

### The uniform selection rule

Any supported package manager may be selected from either manifest field — the top-level
`packageManager` field or `devEngines.packageManager` — in the ADR 0015 precedence order, under the
same per-manager contract regardless of source:

- The ADR 0017 exact-version contract applies identically: full three-part SemVer versions only;
  shortened forms, ranges, tags, URLs, and hash-suffixed descriptors are rejected from both fields,
  even though Corepack can resolve ranges — release builds must not resolve them.
- pnpm requires an exact version in `[10.0.0, 12.0.0)` (ADR 0090), from either field.
- Yarn requires an exact version in `[4.0.0, 6.0.0)`, from either field. Yarn becomes selectable
  from `devEngines.packageManager` alone; `yarn.lock`-only inference without manifest metadata
  remains excluded, and Yarn 6 or newer remains rejected with
  `windlass.verify.error.yarn-version-unsupported`.
- npm remains selectable from either field, with the builder-owned npm runtime override unchanged.
- `onFail` remains diagnostic metadata only and must not weaken release-build enforcement.

### Consequences and commitments

- ADR 0063's Yarn posture is partially superseded as scoped in this ADR's relations: the Berry v4+
  version boundary, the rejection of ranges and `yarn.lock`-only inference, and the immutable
  install requirement remain in force.
- This is a permanent posture, not a provisional convenience: tightening source eligibility after a
  stable release would be a breaking selection change, so acceptance is adopted with the intent to
  keep it.
- Package-manager self-enforcement stays as it is, and the profile does not rely on it for
  correctness in any case: Yarn's binary does not read the top-level `packageManager` field at all
  (verified on 4.18.1 — invoked outside Corepack, it installs past both name and version mismatches;
  Yarn's own pin mechanism is `.yarnrc.yml` `yarnPath`), pnpm 10.x name-checks that field by default
  with exact-version checking opt-in, and pnpm 11.x implements `devEngines.packageManager` while
  folding the v10 strictness settings into `pmOnFail`. Corepack validates both fields for every
  supported manager, including cross-field mismatches.
- The specification sweep scheduled by ADR 0090 encodes this rule: the Yarn selection-source
  restriction and its diagnostics and rejected fixtures are removed, and accepted coverage for
  `devEngines.packageManager`-sourced pnpm 10.x and Yarn pins is added.

### Confirmation

This decision is confirmed when:

- the JS/TS npm build-and-pack specification states the uniform selection rule and removes the
  "devEngines.packageManager alone is not a Yarn selection source" clause, with per-manager bounds
  applied identically from both manifest fields;
- the diagnostics table and the verification policy reflect Yarn acceptance from
  `devEngines.packageManager` and pnpm 10.x acceptance from both fields;
- the fixture corpus gains accepted coverage for `devEngines.packageManager`-sourced Yarn and pnpm
  10.x pins, and retains rejected coverage for ranges, shortened forms, out-of-line versions, and
  `yarn.lock`-only inference;
- a live verification runs a `devEngines.packageManager`-selected Yarn consumer and a pnpm 10.x
  consumer end to end with provenance recording the selection source.

## Pros and Cons of the Options

### Restrict selection sources to fields the selected package manager implements

- Good, because the selection source's meaning matches what the package manager binary itself
  recognizes.
- Bad, because its central premise — that the toolchain cannot read the field — is falsified for the
  entire Corepack window: Corepack validates and provisions from `devEngines.packageManager` for
  every supported manager, and errors on cross-field mismatch.
- Bad, because it creates a version-times-source trap: an identical manifest selects pnpm 11.x but
  is rejected when pinning pnpm 10.x, and Yarn remains a name-based special case.
- Bad, because consumers in mixed-tooling repositories must duplicate the pin into the top-level
  field, keeping two declarations of one fact.

### Accept `devEngines.packageManager`-sourced pins for every supported package manager (chosen)

- Good, because policy matches the verified behavior of the production toolchain instead of
  rejecting declarations the toolchain would honor.
- Good, because the rule reduces to one sentence — any supported manager from either manifest field
  — and the Yarn special case, its diagnostic surface, and its fixture class are deleted rather than
  paralleled for pnpm 10.x.
- Good, because build integrity is unaffected: the executed version is pinned and enforced by the
  builder under both provisioning mechanisms, and provenance records the selection source for strict
  verifier matching.
- Good, because the reversal of ADR 0063's exclusion happens while the project is pre-release, when
  no consumer contract breaks.
- Bad, because the package manager binary itself ignores the field for Yarn and pnpm 10.x, so local
  development outside Corepack gets no binary-level enforcement — though binary enforcement of even
  the top-level field is partial (pnpm 10 name-checks by default), and Corepack-based local
  development enforces both fields.
- Bad, because acceptance is a permanent posture: re-tightening after a stable release would be a
  breaking change, so this option forecloses the restrictive reading.

### Accept with a recorded annotation

- Good, because acceptance stays uniform while noting that the binary does not implement the field.
- Bad, because it opens an informational-diagnostic or metadata surface in a closed-registry,
  fail-closed system for a fact a verifier can already derive from the recorded selection source and
  version.
- Bad, because warnings in a fail-closed pipeline tend to become noise without changing any outcome.

### Keep the interim rejection without a principle

- Good, because no new decision record is written now.
- Bad, because the ADR 0090 specification sweep must encode some behavior for
  `devEngines.packageManager`-sourced pnpm 10.x pins regardless — deferral is a decision in
  disguise, taken without stated grounds.
- Bad, because the Yarn posture and the pnpm 10.x posture would remain two unexplained coincidences,
  and the verified Corepack facts would stay unrecorded.

## More Information

- Corepack README, `devEngines.packageManager` section:
  <https://github.com/nodejs/corepack#devenginespackagemanager> — provisioning fallback when the
  top-level field is missing, and `onFail`-governed validation.
- Live verification on 12026-10-08 (Corepack 0.36.0, Node.js 24.21.0): Yarn 4.18.1 and pnpm 10.34.6
  each executed from a `devEngines.packageManager`-only manifest; a disagreeing
  `packageManager`/`devEngines.packageManager` pair failed with a mismatch error. Invoked outside
  Corepack, the Yarn 4.18.1 binary installed past `packageManager` name and version mismatches — the
  top-level field is Corepack's interface for Yarn, not Yarn's.
- pnpm 11.x `package.json` documentation: <https://pnpm.io/11.x/package_json> —
  `devEngines.packageManager` is "Added in: v11.0.0"; pnpm 10.x predates it.
- ADR 0063 records the superseded Yarn clause: Yarn release builds never proceed from
  `devEngines.packageManager` alone, a version range, or a `yarn.lock` without top-level exact
  `packageManager` metadata. This ADR lifts the first item only.
