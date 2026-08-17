---
parent: Decisions
nav_order: 85
status: accepted
date: 12026-08-17
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0027
    scope:
      "the toolchain resolution clause under which the exact Node.js patch version, and therefore
      the bundled npm CLI, floats within the Node.js 24 major line between runs of the same builder
      release; the Node.js 24 major selection, the runner and OS image constraints, the caller-input
      prohibitions, the build-stage bundled-npm coupling, the npm floor check, and the
      actual-version recording clauses remain in force"
  - type: see-also
    target: ADR-0016
  - type: see-also
    target: ADR-0017
  - type: see-also
    target: ADR-0067
  - type: see-also
    target: ADR-0082
  - type: see-also
    target: ADR-0083
  - type: see-also
    target: ADR-0084
---

# Pin the Node.js 24 Patch Version and Assert the Expected Bundled npm Pair

## Context and Problem Statement

The JS/TS npm package profile's build stage uses the npm CLI bundled with the Node.js 24 toolchain,
and the reusable workflow resolves that toolchain with a floating `node-version: "24"` in all three
jobs. Between two runs of the same builder release, a node-24 image update can silently change both
the Node.js patch version and the bundled npm CLI: the toolchain is recorded
(`externalParameters.runtime.npm_version`, floor `>= 11.5.1`) but not fixed. ADR 0027 recorded the
intent that "the exact Node.js patch version is selected by the builder implementation and its
pinned setup mechanism for a given builder release" — an intent the floating workflow never
implemented.

ADR 0082 pinned the publish-stage npm and deliberately left build-stage selection unchanged, while
recording that "any future parity with the manifest-first package-manager policy of ADR 0015–0017"
is explicitly out of its scope — leaving the build-stage question open for review on its own merits.
That review (12026-08-17) established the following facts:

- The publish-incident driver does not transfer: build-stage npm failures are detected before any
  registry mutation, so no version numbers are burned. The material build-stage risk is different:
  silent semantic drift — a run that succeeds while producing a different install tree or different
  packed tarball bytes, validly attested.
- The drift surface is real even within lockfileVersion 3 (stable across npm 9–12): npm 11 changed
  optional peer and platform resolution (npm/cli#9343), npm 11.6.2 broke `npm ci` at patch level
  (npm/cli#8669), npm 10.8.1 changed workspace `node_modules` cleanup (npm/cli#7692), `npm pack`
  glob behavior changed across npm 8→9→10 (npm/cli#6330, npm/cli#6936, npm/cli#7514), npm 10.5
  changed pack output behavior (npm/cli#7158, npm/cli#7561), and npm 12 blocks dependency lifecycle
  scripts by default and drops `npm-shrinkwrap.json` support.
- Node.js majors have historically bumped the bundled npm major mid-line (Node.js 18 shipped npm 8
  and later 9/10; Node.js 20 shipped npm 9 and later 10). "Node.js 24 bundles npm 11.x" is an
  observation about today's images, not a guarantee for the line's lifetime.
- ADR 0067's same-run convergence relies on byte-reproducible pack output for identical content; a
  different bundled npm between attempts of one run identity can defeat the recomputation fallback
  after an earlier mutation has committed.
- The specification requires a PURL-derivation compatibility fixture "for every supported npm CLI
  version"; while the build-stage npm floats, the supported set on the build path is not enumerable.
- Caller-declared npm versions (via Corepack or via the ADR 0084 tarball mechanism, with or without
  a builder allowlist) conflict with the builder-owned npm principle of ADR 0016 and ADR 0017,
  reaffirmed by ADR 0082; the npm CLI does not read the top-level `packageManager` field, so
  manifest parity would be a builder-enforced convention rather than npm behavior; and arbitrary
  caller versions would re-open the fixture enumerability problem. An independent builder-pinned
  build npm (the ADR 0084 mechanism) is viable but requires amending the closed v1
  `resolvedDependencies` schema (ADR 0070) and adds a run-time registry fetch to the build job —
  more change than the build-stage evidence justifies today.

The question this ADR answers: how the build stage's Node.js/npm toolchain is fixed per builder
release, at the smallest contract change that converts toolchain drift into a review event.

## Decision Drivers

- One builder release must mean one deterministic Node.js/npm toolchain pair; drift between runs of
  the same release becomes impossible without a reviewed bump.
- The supported npm set on the build path must be enumerable, so the per-version compatibility
  fixture obligation is satisfiable.
- ADR 0067 convergence must hold across attempts of one run identity: the same builder release must
  reproduce identical pack bytes for identical content.
- Minimal contract change: no new production tool, no run-time registry fetch, no Corepack, no new
  distribution descriptor, no caller-facing change.
- The builder-owned npm principle (ADR 0016, ADR 0017, reaffirmed by ADR 0082) and the publish/build
  separation (ADR 0082–0084) are preserved.
- Honest scope: this decision stabilizes the Node.js/npm pair; it does not claim full build
  reproducibility, because the runner image, native tooling, and network inputs still drift.

## Considered Options

1. Pin the Node.js 24 patch per builder release and assert the expected bundled npm pair before
   first npm use.
2. Keep the floating `node-version: "24"` resolution (status quo).
3. Honor caller manifest-declared npm versions (via Corepack or the digest-verified tarball, with or
   without a builder allowlist).
4. Provision a builder-pinned, independently selected build-stage npm via the ADR 0084
   digest-verified tarball mechanism (an independent build allowlist, or one shared with the publish
   pin).

## Decision Outcome

Chosen option: "Pin the Node.js 24 patch per builder release and assert the expected bundled npm
pair", because it converts toolchain drift into a review event and makes the supported set
enumerable with the smallest possible contract change, because it implements ADR 0027's recorded
pinned-setup intent rather than reversing any accepted decision, and because the heavier
alternatives are either inconsistent with builder-owned npm or buy version freedom the build stage
does not currently need.

### The pinned toolchain pair

The single shared definition of the build toolchain is one builder-owned machine-readable record
holding the pair: the exact Node.js 24 patch version and the npm version that exact patch bundles,
cross-checked against the Node.js release metadata (<https://nodejs.org/dist/index.json>). The
reusable workflow's Node.js setup uses the exact recorded patch, never a floating major, and the two
values change only together, in one commit, through the bump procedure below. A CI check fails on
drift between the record and any human-readable runtime table in the specifications.

### Pre-install pair assertion

In each job that uses the bundled npm, before the first npm invocation, the job observes
`node --version` and `npm --version` and requires exact equality with the recorded pair; a mismatch
fails the job before any install, build, or pack step. The observed npm version continues to be
recorded in `externalParameters.runtime.npm_version` exactly as today — the recording semantics of
ADR 0027, ADR 0070, and ADR 0071 are unchanged; the value is now validated against the pin rather
than merely observed.

The publish job is governed by ADR 0082 and ADR 0084 unchanged: it provisions the pinned,
digest-verified npm before any npm invocation, diagnostics included, and the bundled npm is never
invoked there. The pair assertion covers the publish job's Node.js runtime only.

### Bump procedure

A toolchain-pair bump pull request must:

1. update the Node.js patch version and the expected bundled npm version together, cross-checked
   against the Node.js release metadata;
2. review the npm changelog delta for build-affecting changes: install and Arborist resolution,
   workspace behavior, lifecycle-script policy, and pack, packlist, and pack output behavior;
3. add or refresh the compatibility fixtures for the new pair;
4. pass the full CI gate on the new pair.

Node.js security releases follow the same procedure on an expedited cadence; the project owns
adoption timing rather than inheriting it from image maintenance.

### Scope and non-goals

- The bundled npm is authenticated as part of the Node.js artifact, through the pinned setup
  mechanism's own checksum verification, not independently as an npm distribution. ADR 0070's
  conclusion that npm receives no package-manager-distribution descriptor remains valid, and the
  `resolvedDependencies` v1 schema is untouched.
- This ADR does not pin the runner image, does not introduce container jobs, and does not claim full
  build reproducibility.
- Caller manifests still must not select an npm version on any stage; ADR 0015, ADR 0016, and ADR
  0017 are untouched, and a manifest `packageManager: "npm@x.y.z"` still selects npm while its
  version component is ignored.
- The `>= 11.5.1` floor check on `runtime.npm_version` is retained as defense in depth; its
  rationale for the build stage is now independent of trusted publishing, which the separately
  pinned publish npm governs.
- The build toolchain pair and the publish npm pin are independent records with independent review
  cadences; neither selects the other. The PURL-derivation compatibility fixture's ownership moves
  to the publish allowlist in the deferred ADR 0083 specification amendment; this ADR does not
  change that fixture contract.

### Escalation trigger

An independently provisioned build-stage npm — the ADR 0084 mechanism with a separate build pin
record, never the publish pin's record — is revisited when any of the following occurs:

- a build-required npm version is unavailable from any acceptable Node.js 24 patch;
- Node.js and npm security or update cadences become irreconcilable;
- the Node.js 24 line adopts an npm major whose build-behavior changes the project can neither
  accept nor avoid within the line.

### Relationship to existing decisions

This ADR partially supersedes ADR 0027's floating toolchain resolution only — implementing its
recorded pinned-setup intent — and every other ADR 0027 clause remains in force: the Node.js 24
major selection, the Ubuntu 24.04 runner constraints, the caller-input prohibitions, the build-stage
bundled-npm coupling, the npm floor check, and the actual-version recording. ADR 0014 through ADR
0017 are untouched: npm stays builder-owned, and caller manifests do not select npm versions. ADR
0067 is unchanged, and its convergence assumption is strengthened. ADR 0070 and ADR 0071 are
untouched. ADR 0082, ADR 0083, and ADR 0084 are untouched: the publish npm pin, the initial pin
selection, the deferral scope, and the provisioning mechanism are all independent of this decision.
Implementation of this ADR is not blocked by ADR 0083's deferral — it does not depend on
npm/cli#9882 — and the workflow pin, the assertion step, the specification amendments, and the
fixtures may land on their own schedule.

## Pros and Cons of the Options

### Pin the Node.js 24 patch and assert the bundled npm pair

- Good, because one builder release deterministically means one Node.js/npm pair, and every
  toolchain change becomes a reviewed bump with fixtures.
- Good, because the supported npm set on the build path becomes enumerable, making the per-version
  compatibility fixture obligation satisfiable.
- Good, because ADR 0067's byte-reproducibility assumption holds across attempts of one run
  identity.
- Good, because it adds no production tool, no run-time fetch, no Corepack, no distribution
  descriptor, and no caller-facing change — the smallest contract change that closes the drift
  surface, implementing ADR 0027's recorded intent.
- Bad, because npm updates are coupled to Node.js patch releases: an npm security fix arrives only
  when an acceptable Node.js patch carries it, and Node.js security bumps force an npm behavior
  review on the project's cadence.
- Bad, because npm 12 is unreachable on the Node.js 24 line; accepted because the build stage has no
  npm 12 requirement today and the escalation trigger covers a future one.

### Keep the floating resolution

- Good, because it adds no machinery, and Node.js and npm security updates arrive automatically.
- Bad, because the same builder SHA can silently produce different install trees and different pack
  bytes after an image update — drift that succeeds rather than fails, validly attested.
- Bad, because the supported set stays unenumerable, the convergence fallback stays exposed to
  cross-attempt drift, and a future mid-line npm major (for example npm 12's lifecycle-script
  defaults) would arrive unannounced.

### Caller manifest-declared npm

- Good, because the caller's CI toolchain could match their local tooling, and the `packageManager`
  field is an ecosystem convention.
- Bad, because the npm CLI does not read the top-level `packageManager` field — the parity is
  illusory outside the builder — and the option conflicts with ADR 0016, ADR 0017, and ADR 0082's
  reaffirmation that callers must not select npm versions on any stage.
- Bad, because Corepack is not an npm-supported install path and is unbundled from Node.js 25+, and
  arbitrary caller versions re-open fixture enumerability and multiply the review surface.

### Builder-pinned independent build-stage npm

- Good, because it frees the npm version from the Node.js line, reuses the reviewed ADR 0084
  mechanism, and keeps the supported set enumerable.
- Bad, because an independently fetched npm is no longer bundled with the toolchain: ADR 0070's
  no-descriptor conclusion falls, the closed v1 `resolvedDependencies` schema must change, and the
  build job gains a run-time registry fetch and a second review cadence — more change than the
  build-stage evidence justifies today; reserved through the escalation trigger.

## More Information

- Motivating analysis: the 12026-08-17 build-stage npm version review, with the option space framed
  as selection authority × provisioning mechanism.
- Node.js release-to-npm mapping: <https://nodejs.org/dist/index.json>. Node.js mid-line npm major
  bumps: the Node.js 18 and 20 release lines.
- npm build-behavior drift evidence: npm/cli#9343 (optional platform resolution), npm/cli#8669
  (11.6.2 `npm ci` regression), npm/cli#7692 (workspace cleanup), npm/cli#6330, npm/cli#6936, and
  npm/cli#7514 (pack glob changes), npm/cli#7158 and npm/cli#7561 (pack output), and the npm 12
  changelog (dependency lifecycle scripts blocked by default; `npm-shrinkwrap.json` dropped).
- npm does not consume the top-level `packageManager` field:
  <https://github.com/npm/cli/issues/8148#issuecomment-2706687519>.
- The initial pair values are recorded in the builder repository when the implementing change lands
  and are reflected in the JS/TS npm build pack specification's runtime table.
