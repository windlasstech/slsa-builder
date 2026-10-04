---
parent: Decisions
nav_order: 88
status: accepted
date: 12026-10-04
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0016
    scope:
      "the Corepack provisioning mechanism clauses for pnpm and Yarn: the requirement to use
      Corepack in strict mode for pnpm and Yarn install, build, and pack stages, and the prohibition
      on disabling Corepack project-spec enforcement. The npm-from-toolchain clause, the
      fail-on-version-mismatch clause, the package-manager selection deference to ADR 0015, and the
      provenance recording clause remain in force"
  - type: partially-supersedes
    target: ADR-0063
    scope:
      "condition 3 of the Yarn support boundary (that Corepack can prepare and dispatch the exact
      Yarn version without Known Good Release or global fallback): Yarn is provisioned from the
      digest-verified npm registry tarball instead. The Berry v4+ version boundary, the top-level
      packageManager metadata requirement, and the immutable install requirement remain in force"
  - type: partially-supersedes
    target: ADR-0070
    scope:
      "the acquisition-path clauses: Corepack as the acquiring agent (the clause recording the
      actual distribution URL used by Corepack), the COREPACK_NPM_REGISTRY routing of Yarn through
      @yarnpkg/cli-dist, the acquisition source annotation value 'corepack', and the v1 prohibition
      on changing acquisition paths. Under this ADR the trusted core fetches the digest-verified npm
      registry tarballs directly, records the registry tarball URL and its SRI, and the acquisition
      source annotation takes a new value fixed in the specification phase. The recording
      obligation, the closed-descriptor-set discipline with strict producer-side verification, the
      lockfile and runner-image descriptors, the absence of npm CLI and Node.js distribution
      descriptors, and the source-native digest-authority discipline remain in force; the concrete
      descriptor shape — the pnpm 12 dual-artifact record and the Yarn authority value — is
      re-specified in the specification phase under that discipline"
  - type: see-also
    target: ADR-0017
  - type: see-also
    target: ADR-0071
  - type: see-also
    target: ADR-0084
  - type: see-also
    target: ADR-0085
  - type: see-also
    target: ADR-0086
---

# Provision Build-Stage pnpm and Yarn from Digest-Verified npm Registry Tarballs

## Context and Problem Statement

ADR 0016 selected Corepack strict mode to provision pnpm and Yarn in the JS/TS npm package profile's
build stages. ADRs 0086 and 0087 re-decided the development-tooling side of the same ecosystem shift
(pnpm provisioning via mise, pnpm 12 adoption); this ADR is the build-stage counterpart,
commissioned after a re-review of the build and deployment line on 12026-10-04.

The ecosystem facts verified for that re-review (all as of 2026-10-04):

**Corepack's trajectory.** The Node.js TSC voted on 2025-03-19 to stop distributing Corepack from
Node.js 25 onward; Node.js 24 LTS is the last bundled line and keeps it, as an experimental feature,
through its EOL on 2028-04-30 ([nodejs/node#61207](https://github.com/nodejs/node/pull/61207),
[nodejs/node#59835](https://github.com/nodejs/node/pull/59835)). The standalone project is alive —
v0.36.0 (2026-08-28), active development — and every feature this design uses (exact
`name@version+sha512` pins, `COREPACK_NPM_REGISTRY`, strict and project-spec switches) is intact.
The npm team does not consume the `packageManager` field, hardcodes npm as the package manager in
its `devEngines` implementation, and rejected package-manager auto-download by design
([npm/cli#8148](https://github.com/npm/cli/issues/8148#issuecomment-2706687519),
[npm/cli#8003](https://github.com/npm/cli/issues/8003#issuecomment-2565859327)).

**pnpm.** The pnpm project removed Corepack from its recommended install and update paths
(documentation section deleted 2026-08-09; CLI update notifications repointed in
[pnpm/pnpm#14115](https://github.com/pnpm/pnpm/pull/14115), merged 2026-08-24). pnpm 12's move to
native-executable packaging initially broke Corepack dispatch
([pnpm/pnpm#13018](https://github.com/pnpm/pnpm/issues/13018)); pnpm restored compatibility on its
own side in 12.0.0-rc.6 via `bin/pnpm.mjs` shims that download the pinned
`@pnpm/exe.<platform>-<arch>` native binary on first use
([pnpm/pnpm#13922](https://github.com/pnpm/pnpm/pull/13922), merged 2026-08-15), while the
Corepack-side fix ([nodejs/corepack#887](https://github.com/nodejs/corepack/pull/887)) was closed
unmerged and the pnpm maintainer does not recommend Corepack. The structural consequence for this
builder: under Corepack with pnpm 12, the executed bits — the native binary plus the `dist/` payload
— arrive through the shim's first-use download, outside the builder's distribution capture, so the
`package-manager-distribution` record would not cover the artifact that actually executed. pnpm 11
remains maintained as the JS-bundle line (latest-11 11.28.2, 2026-09-28).

**Yarn.** Corepack remains the only documented install path for Yarn Berry 4.x. `yarn set version`
performs no checksum or signature verification, and checked-in `.yarn/releases` bundles are
unattested repository content. `repo.yarnpkg.com` publishes no integrity metadata and no signatures
(the signed-releases PR [yarnpkg/berry#6490](https://github.com/yarnpkg/berry/pull/6490) has been
open since 2025-03-04). `@yarnpkg/cli-dist` on the npm registry is an official Yarn publication
(yarnbot, published with `--provenance`), carries `dist.integrity`, npm signatures, and SLSA
attestations, and its `bin/yarn.js` is byte-identical to the `repo.yarnpkg.com` bundle — the npm
registry is Yarn's only integrity-bearing distribution channel. Yarn 6 (the Rust rewrite) plans a
new version manager (Yarn Switch) whose downloads are currently unsigned.

**Existing design.** ADR 0084 already provisions the publish-stage npm CLI from a digest-verified
npm registry tarball, having rejected Corepack for that stage on the same unbundling and
support-path grounds. The build stage still provisions pnpm and Yarn through Corepack, records the
distribution through Corepack's acquisition evidence (ADR 0070), and records a conditional
`corepack` key in `builder.version` (ADR 0071).

The question: what mechanism provisions pnpm and Yarn in the build stage going forward?

## Decision Drivers

- Executed-bits fidelity: the distribution record must cover the artifact that actually executes,
  for every supported package manager and major line.
- Fail-closed enforcement inside the Go trusted core, per the repository rule that trusted and
  runtime logic is not implemented in Node.js or pnpm.
- A single acquisition trust root and a uniform verification vocabulary for downstream verifiers,
  continuing ADR 0070's closed-descriptor discipline.
- Consumer-pin authority preserved: ADRs 0015 and 0017 keep the consumer manifest as the sole
  package-manager version source, and the trusted core keeps enforcing it.
- Consistency with ADR 0084's publish-stage provisioning model.
- Engineering and maintenance cost, weighed honestly against the integrity properties above.
- Rollout must not be forced: the mechanism change ships when its implementation is ready, not when
  this ADR is accepted.

## Considered Options

- Provision pnpm and Yarn from digest-verified npm registry tarballs, implemented in the trusted
  core
- Keep Corepack strict mode for pnpm and Yarn (status quo)
- Provision pnpm through the first-party `pnpm/setup` GitHub Action; Yarn remains on Corepack for
  now
- Consumer checked-in package-manager bundles (the `yarn set version` pattern)

## Decision Outcome

Chosen option: "Provision pnpm and Yarn from digest-verified npm registry tarballs, implemented in
the trusted core", because it comes out best (see below): it is the only option in which every
executed package-manager bit is fetched from the channel whose integrity the builder itself
verifies, recorded with the same digest-rebound discipline the rest of the provenance contract uses,
and enforced by the Go trusted core rather than delegated to an external tool.

### Provisioning flow

Exact observable behavior is deferred to the architecture specifications; the mechanism-level
contract is:

- The trusted core resolves the consumer-pinned exact package-manager version through the unchanged
  ADR 0015/0017 selection semantics, fetches the version metadata from the npm registry, verifies
  the npm registry ECDSA signature over that metadata against the pinned npm signing key (the same
  key pnpm's own installer toolchain pins), records `dist.tarball` and `dist.integrity` into the
  package-manager distribution descriptor, downloads the tarball, verifies the SRI over the
  compressed bytes before extraction, and only then executes.
- Post-provisioning version assertion and every failure diagnostic remain in the trusted core.
- `builder.version`: the conditional `corepack` key (ADR 0071) is absent for pnpm and Yarn builds,
  which ADR 0071 already permits for package managers not obtained through Corepack.

### Package-manager-specific acquisition shapes

- pnpm 11.x: the single JS-bundle tarball is executed through a `node` launcher — the same shape as
  ADR 0084's publish-stage npm.
- pnpm 12 and newer: a dual artifact — the `@pnpm/exe.<platform>-<arch>` native-binary tarball and
  the `pnpm` package's `dist/` payload tarball, each SRI-verified, assembled following the model
  pnpm's own installer (`get-pnpm` / `install.sh`) implements. Platforms outside a reviewed matrix
  fail closed with a diagnostic until added.
- Yarn Berry 4+: the `@yarnpkg/cli-dist` tarball is SRI-verified and its `bin/yarn.js` is executed
  through a `node` launcher. Because acquisition itself becomes SRI-verified, Yarn's recorded digest
  authority can upgrade from ADR 0070's `download-hash` to `registry-integrity`; the final
  descriptor shape is a specification-phase decision.

### Difference from ADR 0084

The publish-stage npm digest is committed in a reviewed allowlist because the builder pins that
version. Build-stage pnpm and Yarn versions are consumer-pinned (ADR 0017) and cannot be
pre-committed, so the integrity anchor at resolution time is the registry signature, and the
verified digest is recorded into signed provenance. The registry-signature anchor plus the
provenance recording substitute for the pre-committed allowlist.

### Rollout

The builder release that first ships this mechanism is a release-management decision and may be
adjusted freely without a new ADR. Initial builder release(s) may continue to ship the ADR 0016
Corepack mechanism as the production path; ADR 0016's mechanism remains valid production behavior
until the release that carries this ADR's replacement. This ADR fixes the target design, not the
calendar.

**pnpm 12 support boundary during the Corepack window.** For as long as the ADR 0016 Corepack
mechanism is the production path, pnpm 12 and newer are explicitly outside the supported consumer
set: a consumer manifest pinning pnpm 12 or newer must fail closed with a diagnostic rather than
build under Corepack. The reason is the record/execution gap recorded above — under Corepack, a pnpm
12 consumer's executed native binary arrives through the shim's first-use download outside the
builder's capture, so the build would emit a `package-manager-distribution` record that does not
cover the executed artifact. Exclusion is the honest posture: the project does not issue provenance
it cannot stand behind. pnpm 12 support begins with the release that completes implementation and
rollout of this ADR's mechanism, whose dual-artifact capture records both executed pnpm 12
artifacts. pnpm 11 and Yarn Berry consumers are unaffected by this boundary.

**Yarn 6 and newer: pre-emptive exclusion.** The same boundary applies to Yarn 6 and newer, and it
is recorded ahead of need: Yarn 6 is an unreleased Rust rewrite whose intended version manager (Yarn
Switch) distributes unsigned binaries today, so its acquisition and integrity semantics under any
provisioning mechanism are unknown. A mechanism whose semantics are unknown must not silently enter
a fail-closed pipeline, and a consumer manifest pinning Yarn 6 or newer must fail closed with a
diagnostic for as long as the ADR 0016 Corepack mechanism is the production path. Unlike pnpm 12,
however, Yarn 6 support does not automatically begin with the release carrying this ADR's mechanism:
whether Yarn 6 ships through an integrity-bearing channel this design can consume (an
`@yarnpkg/cli-dist`-equivalent registry publication, signed Yarn Switch releases, or otherwise) is
evaluated when Yarn 6 actually releases, and support requires a positive finding at that time. Yarn
Berry v4 — and any future Berry v5, per ADR 0063's existing boundary — is unaffected.

### Consequences

- Good, because the distribution record binds the executed bits for every supported line, closing
  the pnpm 12 record/execution gap that the Corepack status quo cannot close without reimplementing
  this mechanism's capture inside a deprecated-path tool.
- Good, because the build stage and the publish stage share one acquisition trust root (npm registry
  integrity plus registry signature) and one verification vocabulary, so downstream verifiers
  implement a single acquisition model.
- Good, because exact-version resolution, download verification, and failure diagnostics live in the
  Go trusted core, covered by the diagnostic registry, fuzz policy, and fixture corpus, instead of
  being delegated to Corepack's strictness flags or to action code.
- Good, because Yarn's only integrity-bearing channel is used directly, with a stronger recorded
  authority than today's `download-hash`.
- Good, because the Corepack expiry (Node.js 24 EOL, 2028-04-30) and both upstreams'
  non-recommendation cease to be design risks.
- Bad, because this is the most expensive option: the Corepack acquisition chain in the trusted core
  is rewritten, a platform matrix and dual-artifact assembly are owned by this project, the
  provenance vocabulary evolves, byte-exact fixtures are regenerated, and the workflow static check
  is replaced.
- Bad, because upstream packaging drift (pnpm's native packaging, Yarn's eventual Switch
  distribution) becomes this project's maintenance watch rather than an ecosystem tool's.
- Bad, because pnpm 12 consumers — and, pre-emptively, Yarn 6 and newer consumers — are explicitly
  unsupported during the Corepack window: a deliberate support-scope cost paid to avoid issuing
  provenance whose distribution record does not cover the executed package manager.
- Neutral, because consumer-pinned versions require runtime registry resolution with signature
  verification instead of ADR 0084's committed digest; the registry-signature anchor and the
  provenance recording substitute for the pre-committed allowlist.

### Confirmation

This decision is confirmed when:

- the JS/TS npm build-and-pack specification defines the registry-tarball acquisition, signature and
  SRI verification, launcher, and platform-matrix behavior for pnpm and Yarn;
- the provenance specifications define the updated package-manager distribution descriptor(s),
  including the pnpm 12 dual-artifact record and the Yarn authority upgrade;
- the verification policy and fixture corpus are regenerated to the new closed shape;
- the workflow static check enforces the new provisioning steps instead of `corepack enable`;
- the Corepack-regime implementation rejects consumer manifests pinning pnpm 12 or newer, or Yarn 6
  or newer, with a fail-closed diagnostic for the duration of the window;
- a live dogfood run on the first builder release carrying this mechanism demonstrates install,
  build, and pack for npm, pnpm 11, pnpm 12, and Yarn Berry consumers with records that bind the
  executed bits.

Until that release, ADR 0016's Corepack mechanism remains the production behavior, and this ADR's
confirmation criteria apply to the transition work only.

## Pros and Cons of the Options

### Provision pnpm and Yarn from digest-verified npm registry tarballs (trusted core)

The chosen option. The trusted core resolves the consumer pin, verifies the npm registry signature
over the version metadata against a pinned key, verifies the tarball SRI before extraction, records
the distribution, and executes through a launcher it controls.

- Good, because every executed bit — including pnpm 12's `@pnpm/exe.<platform>-<arch>` native binary
  and `dist/` payload — is fetched from the channel whose integrity the builder itself verifies, so
  the distribution record covers the executed artifact by construction.
- Good, because it shares ADR 0084's trust root (npm registry `dist.integrity` plus the registry
  ECDSA signature) and verification vocabulary, keeping the downstream verifier story uniform.
- Good, because enforcement and failure semantics live in the Go trusted core with diagnostic
  registry, fuzz, and fixture coverage, consistent with the repository rule that trusted logic is
  not implemented in Node.js.
- Good, because Yarn's `@yarnpkg/cli-dist` is an official, byte-identical, SLSA-attested
  publication, so Yarn gains a stronger recorded authority (`registry-integrity`) than ADR 0070's
  `download-hash`.
- Bad, because it carries the largest engineering cost: trusted-core acquisition rewrite, platform
  matrix ownership, dual-artifact assembly, vocabulary evolution, fixture regeneration, and static
  workflow-check replacement.
- Bad, because upstream packaging drift becomes this project's maintenance watch.
- Neutral, because consumer-pinned versions rule out a committed digest allowlist; the registry
  signature at resolution time plus the signed provenance record take its place.

### Keep Corepack strict mode for pnpm and Yarn

The status quo: ADR 0016's mechanism unchanged, with revisit triggers.

- Good, because it is zero-change and functional on Node.js 24 through 2028-04-30.
- Good, because pnpm 12 works again through the `bin/pnpm.mjs` shim, and every Corepack feature this
  design uses (exact hash pins, `COREPACK_NPM_REGISTRY`, strict and project-spec switches) remains
  intact in Corepack 0.36.0.
- Bad, because for pnpm 12 consumers the executed native binary arrives through the shim's first-use
  download outside the builder's capture, so the provenance record would not cover the executed
  artifact — and closing that gap inside Corepack means reimplementing the chosen option's capture
  on top of a mechanism with a known bundling expiry and an upstream that does not recommend it.
- Bad, because Node.js 26 and later runners require a manual Corepack installation step.
- Bad, because both the pnpm project and the npm team have removed or rejected the paths this option
  depends on.

### Provision pnpm through the `pnpm/setup` action; Yarn remains on Corepack

The first-party action provisions pnpm; Yarn keeps the ADR 0016 mechanism. Its verification model
was verified in the action's source (v2): pnpm is downloaded from the npm registry through
`get-pnpm`, npm's ECDSA signature over the checksum is verified against a pinned key
(`SHA256:DhQ8wR5APBvFHLF/+Tc+AYvPOdTpcIDqOhxsBHRwC7U`), the download is verified against
`dist.integrity`, failures are fail-closed, and the deprecated `token` input documents that no
GitHub API request is made; the action's own documentation declines GitHub's asset digest because it
"catches corruption rather than tampering."

- Good, because its trust root equals the chosen option's — npm registry signature plus SRI — and
  action tampering is blocked by this repository's SHA-pinning policy.
- Good, because first-party maintenance absorbs the platform matrix and future packaging drift, and
  the change surface is the smallest of the replacing options.
- Good, because the consumer's `packageManager` field is honored natively, and an explicit `version`
  input exists, so pin interpretation can remain on the builder's side with a post-provisioning
  version assertion in the trusted core.
- Bad, because the action's outputs expose only install locations and runtime metadata — no resolved
  version, tarball URL, or SRI — so the acquirer-evidence cross-check the current implementation
  performs (registry integrity re-fetched and compared against the acquirer's recorded evidence)
  cannot be replicated, and the record would bind to the asserted version string rather than to the
  executed bits.
- Bad, because artifact verification executes in Node.js action code, outside the Go trusted core,
  against the repository rule that trusted logic is not implemented in Node.js.
- Bad, because the `runtime` input auto-reads the consumer's `devEngines.runtime` with no documented
  opt-out — an integration hazard with ADR 0085's pinned Node.js assertion — and `install` defaults
  to true and must be overridden.
- Neutral, because Yarn's Corepack retention under this option is deferrable rather than decisive:
  Yarn's eventual transition is the cheapest part of either replacing option — but retaining
  Corepack for Yarn means this option neither exits Corepack nor unifies the verification
  vocabulary.

### Consumer checked-in package-manager bundles

The `yarn set version` pattern generalized: consumers commit the package-manager bundle into their
repository.

- Bad, because `yarn set version` performs no checksum or signature verification and checked-in
  bundles are unattested repository content, violating the fail-closed principle and ADR 0017's
  explicit-version discipline.

## More Information

- Verified ecosystem sources (retrieved 2026-10-04): Node.js distribution policy
  ([nodejs/node#61207](https://github.com/nodejs/node/pull/61207),
  [#59835](https://github.com/nodejs/node/pull/59835)); Corepack feature reference
  ([nodejs/corepack README](https://github.com/nodejs/corepack/blob/main/README.md), v0.36.0); pnpm
  packaging and shim repair ([pnpm/pnpm#13018](https://github.com/pnpm/pnpm/issues/13018),
  [#13922](https://github.com/pnpm/pnpm/pull/13922),
  [nodejs/corepack#887](https://github.com/nodejs/corepack/pull/887)); pnpm provisioning references
  ([pnpm/setup](https://github.com/pnpm/setup) v2 source, `src/install-pnpm/download.ts`; `get-pnpm`
  pinned-key verification; `get.pnpm.io/install.sh`); Yarn channels
  ([yarnpkg/berry](https://github.com/yarnpkg/berry) `scripts/release/03-release-npm.sh`,
  `packages/plugin-essentials/sources/commands/set/version.ts`,
  [#6490](https://github.com/yarnpkg/berry/pull/6490);
  [@yarnpkg/cli-dist](https://www.npmjs.com/package/@yarnpkg/cli-dist) registry metadata).
- Watch items: Yarn Switch signing status; Node.js 24 EOL (2028-04-30) as the Corepack sunset for
  any retention window; npm signing-key rotation (the pinned key set must be reviewable); pnpm
  native-packaging drift; registry revisions
  ([issue #105](https://github.com/windlasstech/slsa-builder/issues/105)).
- The development-tooling precedent for leaving Corepack is ADR 0086; the publish-stage npm
  provisioning pattern this ADR extends is ADR 0084; the build-stage Node.js toolchain pin is
  ADR 0085.
