---
parent: Decisions
nav_order: 83
status: accepted
date: 12026-08-17
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0082
    scope:
      "the identification of the initial pinned publish-stage npm version: ADR-0082 records the
      allowlist as initially a single version pinned when the #97 fix lands; ADR-0083 defines the
      #97 remediation as adopting the upstream npm/cli#9882 fix and selects the initial pin as the
      first reviewed npm release containing that fix. ADR-0082's provisioning,
      integrity-verification, allowlist, and bump-procedure clauses are unchanged and remain in
      force"
  - type: see-also
    target: ADR-0024
  - type: see-also
    target: ADR-0029
  - type: see-also
    target: ADR-0076
  - type: see-also
    target: ADR-0081
---

# Defer the npm M1 Publish Remediation to the Upstream provenance-file Fix and Adopt the Fixed Release as the Initial Publish npm Pin

## Context and Problem Statement

The fourth M1 dogfood
([vers-js run 31840088262](https://github.com/windlasstech/vers-js/actions/runs/31840088262),
tracked as [issue #97](https://github.com/windlasstech/slsa-builder/issues/97)) pinned the npm
defect that blocks the JS/TS npm profile's publish path: npm trusted publishing auto-enables
provenance and silently discards a caller-supplied `--provenance-file` bundle, publishing an
npm-generated statement instead of the Windlass-signed Statement. The run published
`@windlass/vers-js@0.1.2` with the foreign evidence; the fail-closed read-back rejected it exactly
as designed, but only after the registry mutation had committed, and npm policy makes the burned
version number permanently unreusable. The RCA identified three interacting layers: `oidc.js`
auto-enables provenance whenever the `provenance` config is at its default, the inner
`provenance === true` branch in libnpmpublish's `buildMetadata()` then never reaches the
`verifyProvenance()` path that reads the supplied file, and every documented way to disable
automatic provenance is unusable in combination with `--provenance-file` (config-layer mutual
exclusivity, the env-layer carve-out, and publishConfig flatten timing).

Two candidate remediations were on the table:

1. A local workaround: suppress npm's CI/OIDC detection for the publish subprocess, inject the
   self-exchanged trusted-publishing token into an isolated npmrc as the registry credential, and
   enforce a closed `publishConfig` allowlist. This mechanism is source-dependent on npm internals —
   exactly the verification surface ADR 0082 exists to close.
2. The upstream fix. The project filed [npm/cli#9879](https://github.com/npm/cli/issues/9879)
   (report) and [npm/cli#9882](https://github.com/npm/cli/pull/9882) (fix PR, OPEN at decision
   time).

ADR 0082 pinned the publish-stage npm provisioning mechanics and recorded the allowlist as
"initially a single version, pinned when the #97 fix lands and re-verified there" — deliberately
leaving open what the #97 fix is, and therefore which npm version opens the allowlist.

A diff review of npm/cli#9882 establishes what the fixed npm does on the profile's exact path:

- `lib/utils/oidc.js` skips auto-enabling provenance when `opts.provenanceFile` is configured; the
  option already carries the value from every config source (CLI, env, npmrc, publishConfig) by the
  time the OIDC flow runs, so all entry paths are covered. The OIDC exchange itself still runs —
  trusted publishing continues to authenticate the publish.
- libnpmpublish's `buildMetadata()` throws `EPROVENANCECONFLICT` when both `provenance: true` and
  `provenanceFile` are set programmatically. The profile's pinned argv never sets `provenance=true`,
  so this branch is unreachable on the profile's path.
- The documentation updates state the precedence rule, and the PR's new CLI regression test asserts
  the profile's exact scenario: OIDC trusted publishing with a `provenance-file` publishes the
  packument with a sigstore attachment deep-equal to the supplied bundle, with provenance generation
  never invoked.

The residual review surface (the error-code naming for the conflict branch, docs wording) does not
intersect the profile's path. The only outcome that would break the profile's path is a wholesale
rejection of the precedence direction, which would contradict npm's own published documentation of
`--provenance-file`.

Two further facts frame the choice. First, ADR 0029 already decided that npm trusted publishing
authenticates the registry mutation while the Windlass-signed bundle is the canonical provenance
(building on ADR 0024's trusted-publishing authentication decision). The workaround would replace
npm's own exchange with a self-performed exchange plus an injected credential — a defect-forced
deviation from that accepted design, whose reason to exist disappears when the fix ships. Second,
the wait is expected to cost little or no schedule time: npm 12.0.1 shipped two days after 12.0.0
and 12.0.2 seventeen days later, while the workaround path requires a specification amendment, a TDD
implementation across the publish boundary, and a dogfood run before it could land.

## Decision Drivers

- A mechanism that is never written is strictly cheaper than one that is written, verified, and
  later retired; ADR 0082 already earmarks workaround retirement on fixed npm versions as a separate
  later decision.
- The workaround's dependence on npm internals (`oidc()` ordering, config exclusivity, publishConfig
  flatten timing) is precisely the verification surface ADR 0082 closes; adopting the workaround
  even temporarily would expand the surface that must be reviewed per bump.
- The fix's semantics for the profile's argv are diff-verified today; the uncertain residue does not
  intersect the profile's path.
- Waiting restores the ADR 0024/ADR 0029 authentication design instead of entrenching a deviation
  from it.
- The ADR 0076 exchange preflight retains its value — early, mutation-free detection of
  trusted-publishing misconfiguration — regardless of which party consumes the minted token, so ADR
  0081's pinned exchange contract stays in force untouched.
- Dependence on an external release schedule is the principal risk of waiting and must be bounded by
  an explicit revisit trigger rather than left open-ended.

## Considered Options

1. Defer the #97 remediation to the first reviewed npm release containing npm/cli#9882; adopt that
   release as the initial publish npm pin; authenticate the publish with npm-native trusted
   publishing.
2. Implement the #97 workaround now on a pinned npm 11.17.0; adopt the upstream fix later through
   the ADR 0082 bump procedure.
3. Hybrid: implement the version-agnostic ADR 0082 machinery now (the workflow provisioning step,
   pin-data plumbing, and Go-side pin validation) and defer only the version selection and the
   authentication-path finalization.

## Decision Outcome

Chosen option: "Defer the #97 remediation to the first reviewed npm release containing
npm/cli#9882", because the fix's semantics on the profile's path are already diff-verified, because
waiting restores the accepted authentication design rather than entrenching a deviation, and because
waiting is expected to add little or no schedule cost compared with specifying, implementing, and
dogfooding the workaround first.

### Initial pin selection at adoption time

When an npm release containing the #9882 fix ships, the initial allowlist entry is selected under
the ADR 0082 bump procedure, preferring:

1. an npm 11-line backport, if one is released — minimal internals delta from the dogfood-exercised
   11.17.0 and alignment with the Node 24 LTS line; otherwise
2. the first 12.x release containing the fix, with bundle-verification compatibility evidence
   against its bundled sigstore-js major (v5) as an added acceptance criterion, because npm's
   `verifyProvenance()` path has never been exercised live against a Windlass-signed bundle.

The pin data (version, distribution URL, distribution SHA-256) is recorded in the JS/TS npm
provenance and publish specification's allowlist per ADR 0082.

### Publish authentication path

With the fix in place, the publish job uses npm-native trusted publishing exactly as ADR 0024 and
ADR 0029 intended: npm performs its own OIDC exchange inside `npm publish`, and the pinned argv is
unchanged. No OIDC-environment suppression, no token injection into an isolated npmrc, and no
`publishConfig` credential allowlist is specified or implemented. The ADR 0076 exchange preflight is
retained as a non-mutating observation preflight for early detection of trusted-publishing
misconfiguration; the minted token is discarded, not consumed. The fail-closed post-publish
read-back remains the permanent safety net, unchanged.

### Deferral scope and revisit trigger

All #97 remediation implementation is deferred until the fixed release exists and is selected: the
specification amendment (authentication path, publish-side pin validation replacing the
build/publish npm-version equality check, the allowlist initial entry), the workflow provisioning
step, the Go changes, and the compatibility fixtures.

This decision is revisited — with option 2 (the workaround on a pinned npm 11.17.0) as the default
fallback — when any of the following occurs:

- npm/cli#9882 is closed unmerged;
- npm/cli#9882 is merged with precedence semantics that differ materially for the profile's argv
  (for example, automatic provenance taking precedence over a supplied `--provenance-file`);
- npm/cli#9882 has not merged by 12026-10-01.

### Relationship to existing decisions

This ADR amends ADR 0082's identification of the initial pinned version only; ADR 0082's
provisioning, integrity-verification, allowlist, and bump-procedure clauses are unchanged and remain
in force. ADR 0024 and ADR 0029 are not modified — this ADR removes a planned deviation from them.
ADR 0076 and ADR 0081 remain fully in force: the exchange preflight keeps its observation role and
its pinned response contract.

## Pros and Cons of the Options

### Defer to the fixed release and adopt it as the initial pin

- Good, because no workaround code, specification text, or fixtures are written up front for a
  mechanism that becomes unnecessary if the fix ships before the initial pin is selected.
- Good, because publish authentication returns to the ADR 0024/ADR 0029 design: npm trusted
  publishing authenticates the registry mutation; the Windlass bundle is the canonical provenance.
- Good, because the verification surface ADR 0082 must close stays minimal — one pinned npm whose
  behavior includes first-class support for the profile's exact publish mode, covered by npm's own
  regression test.
- Good, because the decision is evidence-based today: the fix's behavior on the profile's path is
  pinned by the reviewed diff, not by a guess about maintainer intent.
- Bad, because npm M1 completion becomes dependent on the npm maintainers' review and release
  schedule, which the project does not control; the revisit trigger bounds but does not eliminate
  this dependence.
- Bad, because the fixed release's line is unknown: if the fix ships only in npm 12.x, the initial
  pin carries the sigstore-js v5 bundle-verification compatibility question and a larger internals
  diff-review than an 11.x pin would require.

### Implement the workaround now on pinned 11.17.0

- Good, because npm M1 unblocks immediately on the project's own schedule, and 11.17.0 is the
  dogfood-exercised version with live evidence.
- Bad, because the workaround is specified, implemented, and dogfooded at a calendar cost comparable
  to the expected wait, and it becomes dead weight if the fix ships before the initial pin is
  selected.
- Bad, because the workaround is source-dependent on npm internals across the allowed range and
  entrenches a deviation from ADR 0029's authentication design, with its retirement already known to
  require a separate later decision.

### Hybrid: build version-agnostic machinery now

- Good, because part of the wait becomes productive: the provisioning step and pin plumbing do not
  depend on the selected version.
- Bad, because the specification's authentication path and the pin-validation contract cannot be
  finalized before the fixed release's behavior is confirmed, so a meaningful fraction of the work
  risks rework, and a partial-implementation state is harder to reason about than a clean deferral.

## More Information

- Motivating incident and RCA: [issue #97](https://github.com/windlasstech/slsa-builder/issues/97)
  and the fourth-attempt evidence in
  [issue #30](https://github.com/windlasstech/slsa-builder/issues/30).
- Upstream: [npm/cli#9879](https://github.com/npm/cli/issues/9879) (report),
  [npm/cli#9882](https://github.com/npm/cli/pull/9882) (fix PR; OPEN and diff-verified as described
  above at decision time).
- Issue #97's fix-requirement section described the workaround; it is superseded by this ADR and
  will be rewritten when the remediation implementation begins.
- The burned `@windlass/vers-js@0.1.2` publication remains the foreign-conflict test target for the
  publish read-back, and the preserved dogfood-4 bundle remains the reference bundle the fixed npm's
  `verifyProvenance()` path must accept.
- If this decision is revisited under the trigger above, the fallback selects npm 11.17.0 as the
  initial pin per the analysis recorded in ADR 0082's context.
