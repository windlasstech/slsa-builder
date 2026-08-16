---
parent: Decisions
nav_order: 82
status: accepted
date: 12026-08-16
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0027
    scope:
      "the npm provisioning clause for the publish stage (the publish job uses the npm CLI provided
      by the selected Node.js 24 toolchain, floating with the node-24 image); the Node.js 24 runtime
      selection, the runner and OS image constraints, the caller-input prohibitions, the build-stage
      npm clauses, and the floor check and actual-version recording clauses remain in force"
  - type: see-also
    target: ADR-0016
  - type: see-also
    target: ADR-0017
  - type: see-also
    target: ADR-0029
  - type: see-also
    target: ADR-0081
---

# Pin the Publish-Stage npm CLI Version with Integrity-Verified Provisioning and a Reviewed Allowlist

## Context and Problem Statement

The JS/TS npm package profile's npm version selection is currently floating, on every stage. All
three jobs of the reusable workflow (build, provenance-sign, publish) provision Node.js with
`node-version: "24"` and no npm installation step, so the npm CLI is whatever the resolved node-24
image bundles at run time (the fourth M1 dogfood resolved node 24.19.0, npm 11.17.0). The contract
around this is a floor plus an equality check: the signed `externalParameters.runtime.npm_version`
must satisfy SemVer `>= 11.5.1` (the trusted-publishing minimum), and the publish job re-runs
`npm --version` and requires exact string equality with the build-time record. There is no upper
bound, no allowlist, and no review gate: any npm that a future node-24 image happens to bundle
silently becomes the production publish toolchain.

ADR 0016 and ADR 0017 already established that npm is builder-owned: caller manifests must not
select the npm version (ADR 0017 explicitly rejected requiring a caller-side npm pin), because the
provenance-aware `npm publish` path runs on the toolchain's npm regardless of the package's
build-time package manager. ADR 0027 further recorded the intent that "the exact Node.js patch
version is selected by the builder implementation and its pinned setup mechanism" — an intent the
floating `node-version: "24"` workflow never implemented for the bundled npm.

The fourth M1 dogfood
([vers-js run 31840088262](https://github.com/windlasstech/vers-js/actions/runs/31840088262),
tracked as [issue #97](https://github.com/windlasstech/slsa-builder/issues/97)) exposed why this
matters specifically on the **publish** stage. npm trusted publishing auto-enables provenance and
silently discards a caller-supplied `--provenance-file` bundle; the run published
`@windlass/vers-js@0.1.2` with an npm-generated statement instead of the Windlass-signed Statement.
The fail-closed read-back rejected the foreign evidence exactly as designed — but the registry
mutation had already committed, and npm policy makes an unpublished version number permanently
unreusable, so `0.1.2` remains publicly visible with the wrong provenance. The remediation for #97
(suppressing npm's CI/OIDC detection for the publish subprocess, injecting the publish-time
exchanged token as the registry credential, and a closed `publishConfig` allowlist) depends on npm
**internal** behavior on the publish path: `oidc()` call ordering and early-return conditions,
`@npmcli/config` exclusive-enforcement semantics, `publishConfig` flatten timing, and
`libnpmpublish`'s `verifyProvenance` with its bundled sigstore-js.

Source review across the currently allowed range shows those internals already vary:

| npm version       | `oidc.js` auto-enable structure         | Exclusive env carve-out  | sigstore-js        |
| ----------------- | --------------------------------------- | ------------------------ | ------------------ |
| 11.5.1 (floor)    | `isDefaultProvenance \|\| intent` shape | absent (throws)          | `^3.0.0` (→ 3.1.0) |
| 11.17.0 (dogfood) | intermediate shape                      | present (silently skips) | intermediate       |
| 12.0.2 (latest)   | adds CircleCI branch                    | present (silently skips) | `^5.0.0`           |

A publish mechanism verified against one npm can therefore be silently invalidated by the next
node-24 image update, and the failure mode is not hypothetical: it is a publish-then-reject incident
that burns a version number in every affected caller at once, detected only after mutation.

Additionally, the JS/TS npm provenance and publish specification requires a PURL-derivation
compatibility fixture "for every supported npm CLI version", but no document enumerates the
supported set, so that obligation is currently unsatisfiable. And while the upstream defect is now
filed and fix-pending ([npm/cli#9879](https://github.com/npm/cli/issues/9879),
[npm/cli#9882](https://github.com/npm/cli/pull/9882)), every npm release in the allowed range will
retain the defect permanently; adopting the upstream fix must become an explicit, reviewed event,
not toolchain drift.

This ADR scopes its decision to the publish stage, where the incident occurred and where the
mechanism's npm-internal dependence lives. Build-stage npm version selection is unchanged by this
ADR: the build stage continues to use the npm bundled with the Node.js 24 toolchain per ADR 0016,
ADR 0017, and ADR 0027.

## Decision Drivers

- The #97 remediation mechanism is source-dependent on the publish path; a source-dependent
  mechanism under a floating toolchain leaves the verification surface open and every guarantee
  past-tense.
- Fail-closed read-back protects acceptance, not registry state: rejection happens after mutation,
  and burned versions cannot be republished (0.1.2 precedent).
- A node-24 image update is a correlated-failure vector: one image roll can break every caller's
  publish simultaneously, at release time.
- Builder-owned npm is already the design principle for the publish path (ADR 0016, ADR 0017);
  builder-side pinning is the recorded intent (ADR 0027's "pinned setup mechanism"), so determinism
  completes existing decisions rather than changing direction.
- The repository's supply-chain stance pins everything else in the trust boundary (action SHAs,
  `mise.lock`, the Go toolchain); npm is the sole floating component whose internals the publish
  path depends on.
- The specification's per-version compatibility-fixture obligation requires an enumerable supported
  set on the publish path.
- Adopting the upstream npm fix (npm/cli#9882) should be a deliberate pinned bump with regression
  evidence, not an accident of image maintenance.

## Considered Options

1. Pin the publish-stage npm: provision a reviewed npm version explicitly in the publish job with
   integrity verification, hold the supported set in a reviewed allowlist, and require a diff-review
   checklist plus fixtures for every bump.
2. Keep the `>= 11.5.1` floor and verify the publish mechanism across the full range: matrix CI over
   every supported npm minor with per-version compatibility fixtures.
3. Keep the current gating unchanged and trust the fail-closed read-back.

## Decision Outcome

Chosen option: "Pin the publish-stage npm", because it is the only option that closes the publish
path's verification surface, because it completes the builder-owned principle and the recorded
pinning intent rather than contradicting them, and because its costs (update ownership, a
registry-fetch provisioning path) are bounded and already priced in by the repository's existing
pinning stance.

### Publish-job provisioning with integrity verification

The publish job of the reusable workflow provisions npm explicitly before any npm invocation, from
one shared definition of version plus distribution SHA-256. The install step verifies the
distribution digest before first use. Node.js 24 remains the runtime per ADR 0027; only the
publish-stage npm provisioning clause is superseded. The build and provenance-sign jobs are out of
scope: they continue to use the Node.js 24 toolchain's bundled npm as today.

### Publish-side version contract

The publish job's npm version must equal a version in a reviewed allowlist recorded in the JS/TS npm
provenance and publish specification (initially a single version, pinned when the #97 fix lands and
re-verified there). This replaces the current publish-side validation, which required exact equality
with the build-time `runtime.npm_version` record: since the build-stage npm remains
toolchain-bundled and floating while the publish npm is pinned, equality between the two is no
longer the invariant. The build stage continues to record the actual npm version it used in
`externalParameters.runtime.npm_version` unchanged; the implementing specification amendment
redefines the publish-side check as pin validation and records the publish npm version in the
outcome evidence.

### Bump procedure

An allowlist bump pull request must:

1. update the pinned version and distribution digest together;
2. diff-review the npm-internals checklist for the new version: `lib/utils/oidc.js` early-return and
   auto-enable structure, `libnpmpublish` `buildMetadata()` branch structure, `@npmcli/config`
   exclusive enforcement (including the env-layer carve-out), `publishConfig` flatten timing, and
   the bundled sigstore-js major version;
3. add or refresh the specification-required compatibility fixtures for the new pin;
4. pass the full CI gate on the new pin.

### Relationship to existing decisions and safety nets

This ADR partially supersedes ADR 0027's npm provisioning clause for the publish stage only; every
other ADR 0027 clause remains in force, including the build-stage use of the toolchain npm. ADR 0016
and ADR 0017 are untouched: npm stays builder-owned, and caller manifests still must not select an
npm version on any stage. The fail-closed publish read-back is unchanged and remains the permanent
safety net for everything outside the pinned surface; this ADR removes the need to rely on it for
publish npm-version drift. When the upstream fix (npm/cli#9882) ships in an npm release, adopting it
is a pinned bump under the procedure above; retiring the #97 workaround on fixed npm versions is a
separate later decision.

## Pros and Cons of the Options

### Pin the publish-stage npm

- Good, because the publish mechanism's behavior becomes deterministic and its verification surface
  is closed and enumerable.
- Good, because publish npm changes become review events with an internals checklist, matching the
  action SHA-pinning and `mise.lock` stance already applied to the rest of the trust boundary.
- Good, because the specification's per-version compatibility-fixture obligation becomes satisfiable
  on the publish path.
- Good, because correlated failure from node-24 image drift is eliminated for the publish path, and
  upstream-fix adoption becomes a deliberate, evidence-backed bump.
- Good, because the build stage is untouched: no provenance-schema split between build-time and
  publish-time npm is introduced beyond redefining the publish-side check, and no new caller-facing
  constraint appears.
- Bad, because publish npm security and feature updates no longer arrive automatically with node-24
  image updates; the project owns the publish npm update cadence and the review checklist.
- Bad, because the workflow fetches the npm distribution from the registry at run time instead of
  using the node-bundled copy; this is mitigated by pinning the distribution digest, and is no worse
  than the implicit, unverified trust currently placed in whatever the image bundles.

### Keep the floor and verify the full range

- Good, because callers keep toolchain flexibility and the specification's per-version fixtures get
  built across the range.
- Bad, because the range is open-ended: a new npm ships in a node-24 image and runs in production
  before any fixture or matrix entry can exist for it. The gate cannot test what does not yet exist,
  so the riskiest moment is precisely the uncovered one.
- Bad, because the sigstore-js v3–v5 span across the range multiplies fixture and review maintenance
  with npm's release cadence.

### Keep current gating and trust fail-closed

- Good, because it adds no machinery and the read-back rejection is proven live (dogfood 4).
- Bad, because fail-closed protects acceptance, not registry state: a drifted npm can still
  publish-then-reject, burning version numbers and leaving mislabeled provenance publicly visible in
  every affected caller at once.
- Bad, because incident cost is externalized to callers at release time, the worst possible moment.

## More Information

- Motivating incident and RCA: [issue #97](https://github.com/windlasstech/slsa-builder/issues/97)
  and the fourth-attempt evidence in
  [issue #30](https://github.com/windlasstech/slsa-builder/issues/30).
- Upstream: [npm/cli#9879](https://github.com/npm/cli/issues/9879) (report),
  [npm/cli#9882](https://github.com/npm/cli/pull/9882) (fix pending).
- The initial pinned version is selected at #97 fix implementation time and recorded in the JS/TS
  npm provenance and publish specification's allowlist, with the bump procedure above governing all
  later changes.
- Build-stage npm version selection (including any future parity with the manifest-first
  package-manager policy of ADR 0015–ADR 0017) is explicitly out of scope for this ADR and remains
  governed by the existing decisions unchanged.
