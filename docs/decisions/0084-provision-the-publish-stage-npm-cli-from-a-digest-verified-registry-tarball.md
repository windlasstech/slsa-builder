---
parent: Decisions
nav_order: 84
status: accepted
date: 12026-08-17
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0082
    scope:
      "the designation of SHA-256 as the publish-stage npm distribution digest algorithm in the
      provisioning clause ('one shared definition of version plus distribution SHA-256'): the pinned
      digest is SHA-512 in SRI form, byte-identical to the registry-native dist.integrity field. The
      pre-use digest-verification obligation, the single shared definition, the allowlist, and the
      bump procedure are unaffected and remain in force"
  - type: amends
    target: ADR-0082
    scope:
      "the selection of the concrete publish-stage npm provisioning mechanism, which ADR-0082 left
      unspecified: ADR-0082 mandates explicit provisioning before any npm invocation with the
      distribution digest verified before first use but does not name the mechanism; ADR-0084
      narrows the open choice to the digest-verified npm registry tarball mechanism"
  - type: see-also
    target: ADR-0083
  - type: see-also
    target: ADR-0027
  - type: see-also
    target: ADR-0085
---

# Provision the Publish-Stage npm CLI from a Digest-Verified Registry Tarball

## Context and Problem Statement

ADR 0082 decided that the publish job of the JS/TS npm reusable workflow provisions npm explicitly
before any npm invocation, from one shared definition of version plus distribution SHA-256, with the
distribution digest verified before first use, and that the supported set lives in a reviewed
allowlist. It deliberately did not name the provisioning mechanism: "the install step verifies the
distribution digest" describes an obligation, not a mechanism, and the workflow change cannot be
implemented until a concrete one is selected.

ADR 0083 then fixed which npm version opens the allowlist: the first reviewed npm release containing
the npm/cli#9882 fix, preferring an npm 11-line backport and otherwise the first 12.x release
carrying the fix. The provisioning mechanism must therefore be able to reach **both** pin lines on
the Node.js 24 runtime, which remains in force per ADR 0027.

An investigation of the candidate mechanisms established the following facts:

- `actions/setup-node` has no npm-version input and never modifies npm after installation; the npm
  CLI is whatever the resolved Node artifact bundles. The Node-release-to-npm mapping is fixed per
  exact release ([nodejs.org/dist/index.json](https://nodejs.org/dist/index.json)), but the entire
  Node.js 24 line bundles npm 11.x only (24.19.0 bundles 11.17.0), so npm 12.x is unreachable
  through any Node.js 24 artifact.
- The npm package tarball on the public registry is fully self-contained: npm declares all
  production dependencies in `bundleDependencies`, and after plain extraction
  `node package/bin/npm-cli.js` runs directly (verified against npm 12.0.2).
- The registry packument provides, per version, `dist.tarball` (URL), `dist.shasum` (SHA-1),
  `dist.integrity` (SRI SHA-512), and `dist.signatures` (npm registry ECDSA signatures). SHA-512 in
  SRI form is the registry's native strong digest; the packument provides **no SHA-256 field**.
  Sampled npm CLI releases (9.5.0 through 12.0.2) carry the registry ECDSA signature but no
  npm/sigstore provenance attestation and no GitHub artifact attestation for the tarball.
- `npm install -g npm@<version>` verifies integrity only against registry metadata fetched at
  install time (trust on first use); the npm CLI exposes no option to supply an expected digest, so
  an independently committed digest cannot be enforced on that path.
- Corepack recognizes npm and supports exact pins with hashes (`npm@<version>+sha512.<hex>`), but it
  installs no npm shims by default, the npm team does not treat Corepack as a supported npm install
  path, the January 12025 npm registry signing-key rotation broke older bundled Corepack versions,
  and Node.js 25+ no longer distributes Corepack.
- The publish boundary in this repository invokes npm through a Go command that takes an absolute
  executable path whose basename must be `npm` (`--npm-executable`), and the publish job currently
  invokes the ambient npm in its diagnostics step before publishing.

ADR 0082 names SHA-256 as the distribution digest algorithm. Because the registry's native strong
digest is SHA-512 in SRI form, pinning SHA-512 makes the committed value byte-identical to the
packument's `dist.integrity` field: the review-time cross-check becomes a direct equality check
instead of a cross-algorithm agreement argument, and the pin format aligns with npm ecosystem
conventions (package-lock `integrity`, Corepack hashes) and with this profile's existing SHA-512
tarball digest recording (ADR 0064). The security properties of the two algorithms are equivalent
for this use; the choice is about verifiability and ecosystem alignment. This ADR therefore settles
the digest algorithm together with the mechanism.

The question this ADR answers: which mechanism provisions the pinned npm in the publish job, and on
which digest algorithm the pin is recorded, while satisfying ADR 0082's clauses and ADR 0083's
pin-line reachability.

## Decision Drivers

- Conformance to ADR 0082's three hard clauses: provisioning before **any** npm invocation
  (diagnostics included), verification of the committed distribution digest before first use, and a
  single shared definition of version plus digest.
- Reachability of both candidate pin lines (an npm 11.x backport and npm 12.x) on the Node.js 24
  runtime, per ADR 0083 and ADR 0027.
- No bootstrap trust in a floating installer: the verification path must not depend on the
  node-bundled npm, whose own version is unpinned.
- Fit with the existing publish boundary: the Go publish command selects npm via an absolute
  executable path with basename `npm`; npm-native trusted publishing and the pinned publish argv
  remain unchanged per ADR 0024, ADR 0029, and ADR 0083.
- Consistency with the repository's existing pinning stance: full-SHA action pins and `mise.lock`'s
  recorded URL plus SHA-256 per tool artifact.
- Every provisioning failure must occur before any registry mutation.

## Considered Options

1. Digest-verified registry tarball: download `https://registry.npmjs.org/npm/-/npm-<version>.tgz`,
   verify the committed SHA-512 digest over the compressed bytes before extraction, extract, and
   execute npm through a launcher named `npm`.
2. `npm install -g npm@<version>` from the node-bundled npm.
3. Corepack `packageManager: "npm@<version>+sha512.<hex>"` pin.
4. setup-node exact Node patch coupling: pin `node-version: 24.x.y` for the npm its artifact
   bundles.
5. Self-hosted vendored tarball: re-host the npm tarball as a project-controlled artifact.
6. Digest-pinned container job: run the publish job in a container image pinned by digest.

## Decision Outcome

Chosen option: "Digest-verified registry tarball", because it is the only option that satisfies all
three ADR 0082 hard clauses without bootstrap trust in a floating installer, because it reaches both
pin lines ADR 0083 allows, and because it matches the repository's recorded
URL-plus-committed-digest pinning stance. The pinned digest algorithm is SHA-512 in SRI form — the
registry's native strong digest — so the committed value is byte-identical to the packument's
`dist.integrity` field.

### Canonical pin data

The single shared definition required by ADR 0082 is one machine-readable file recording the pinned
npm version, the exact registry tarball URL, and the SHA-512 of the compressed tarball in SRI form
(`sha512-<base64>`), byte-identical to the registry packument's `dist.integrity` field. The
provisioning step and the publish-side pin validation both read this file, and the JS/TS npm
provenance and publish specification references it normatively as the allowlist's pin-data source. A
CI check fails on drift between the file and any human-readable allowlist table in the
specification.

### Provisioning flow

The publish job provisions npm before any npm invocation, including the existing diagnostics step:

1. Download the exact recorded URL over HTTPS; no URL, version, or integrity value is derived from
   registry metadata at run time.
2. Hash the compressed bytes and require equality with the recorded SHA-512 digest; a mismatch fails
   the job before extraction.
3. Extract into a fresh private directory with archive-safety checks (a single `package/` root, no
   absolute or parent-traversing entries).
4. Create a launcher named `npm` that execs the setup-node-provided `node` at its absolute path
   against the extracted `bin/npm-cli.js`; the launcher's absolute path is passed to the publish
   command's `--npm-executable`. No global installation and no `PATH` mutation is required.

Digest mismatch, download failure, unsafe archive layout, a missing CLI entry point, or a
post-extraction version mismatch all fail the job before any npm execution and before any registry
mutation. Per ADR 0004, the fetch-verify-extract-emit logic lives in the trusted Go boundary; the
workflow step only invokes it.

### Validation and evidence

The publish-side version contract remains as ADR 0082 redefined it: the provisioned npm's reported
version must equal the pinned version recorded in the allowlist. The build stage continues to record
its toolchain-bundled npm in `externalParameters.runtime.npm_version` unchanged; the publish npm
version is recorded in the outcome evidence and is not merged into the build-time record.

### No caching at adoption time

No download cache is introduced. If one is added later, only the original compressed tarball may be
cached, and the recorded SHA-512 digest must be re-verified after every restore: a cache-key match
does not authenticate cached bytes.

### Bump-review deep defense

The ADR 0082 bump procedure gains review-time (not run-time) cross-checks for the recorded pin data:
exact equality of the packument `dist.integrity` with the committed SRI SHA-512 value, consistency
of the legacy `dist.shasum` (SHA-1) over the same bytes, presence of the npm registry ECDSA
signature, and equality of the packument `gitHead` with the npm/cli release tag. These signals all
originate from the registry or the publisher and are not independent provenance; the substance of
the bump review remains the internals diff-review of the extracted distribution per the ADR 0082
checklist.

### Implementation timing

This ADR selects the mechanism only. Implementation timing remains governed by ADR 0083's deferral
scope: the specification amendment, the workflow provisioning step, the Go changes, and the
compatibility fixtures land when the fixed npm release exists and the initial pin is selected.

### Relationship to existing decisions

This ADR partially supersedes ADR 0082's designation of SHA-256 as the distribution digest algorithm
— the pinned digest is SHA-512 in SRI form — and amends ADR 0082 by narrowing the open
provisioning-mechanism choice to the digest-verified registry tarball. Every other ADR 0082 clause
remains in force, including the pre-use digest verification, the single shared definition, the
allowlist, the bump procedure, and the build-stage scope exclusion. ADR 0083 is untouched: the
initial pin is still selected when the fixed release ships, and this mechanism serves either pin
line. ADR 0027 is untouched: Node.js 24 remains the runtime and continues to float within the major
line, and the build stage's bundled npm is unaffected. ADR 0024 and ADR 0029 are untouched:
npm-native trusted publishing authenticates the registry mutation, and the pinned publish argv is
unchanged.

## Pros and Cons of the Options

### Digest-verified registry tarball

- Good, because it satisfies all three ADR 0082 hard clauses exactly: provisioning precedes every
  npm invocation, the committed digest is verified before first use, and one file is the single
  shared definition.
- Good, because the verification path uses runner-provided tools (HTTPS download plus SHA-512) and
  never trusts the floating node-bundled npm.
- Good, because npm 11.x and 12.x are equally reachable on Node.js 24, leaving ADR 0083's initial
  pin selection unconstrained.
- Good, because the self-contained tarball needs no install step; direct execution from the
  extracted tree is verified upstream packaging behavior.
- Good, because it mirrors the `mise.lock` recorded-URL-plus-committed-digest pattern already
  applied to the rest of the trust boundary.
- Good, because the pinned digest is byte-identical to the registry-native `dist.integrity`, so the
  bump review's registry cross-check is a direct equality check, and the pin format matches npm
  ecosystem conventions (package-lock `integrity`, Corepack hashes) and the profile's existing
  SHA-512 tarball digest recording (ADR 0064).
- Bad, because it adds a run-time fetch from registry.npmjs.org; the dependency is incremental
  rather than new (the publish itself requires the registry), but it is one more pre-mutation
  failure point.
- Bad, because the digest algorithm now differs from the `mise.lock` SHA-256 convention used for
  development tools, so reviewers handle two digest algorithms across the trust boundary; mitigated
  because the npm profile already records SHA-512 tarball digests and the development-tooling and
  publish-trust axes are separable.

### npm install -g npm@\<version\>

- Good, because it is the simplest possible step and has widespread community precedent.
- Bad, because integrity is verified only against registry metadata fetched at install time (trust
  on first use) and no expected-digest input exists, so the committed-digest clause cannot be met.
- Bad, because provisioning begins with an npm invocation by the floating node-bundled npm,
  conflicting with the before-any-invocation clause and trusting the unpinned installer.

### Corepack packageManager pin

- Good, because hash pinning (`+sha512.<hex>`) and caching are built in.
- Bad, because npm shims are not installed by default and the npm team does not treat Corepack as a
  supported npm install path; invocation must go through `corepack npm`, which does not fit the
  absolute-executable publish boundary.
- Bad, because bundled Corepack lags standalone releases, the 12025-01 registry key rotation broke
  older Corepack, and Node.js 25+ no longer distributes Corepack: a shrinking, additional trust
  surface.

### setup-node exact Node patch coupling

- Good, because it adds no machinery: the bundled npm arrives with the pinned Node artifact.
- Bad, because Node.js 24 artifacts carry npm 11.x only, so the npm 12.x pin line that ADR 0083 may
  require is structurally unreachable.
- Bad, because nothing verifies the npm distribution independently (the Node artifact's integrity is
  not the npm distribution digest), and npm bumps become coupled to Node patch bumps.

### Self-hosted vendored tarball

- Good, because it removes the run-time registry fetch and gives the project full custody of the
  bytes.
- Bad, because re-hosting adds storage, licensing, and update-custody burden for a marginal gain:
  the same review and the same committed digest are still required, and the fetch merely moves to
  the hosting location.

### Digest-pinned container job

- Good, because it freezes the whole toolchain under one immutable digest, with the SLSA
  container-based builder as precedent.
- Bad, because npm 12.x on Node.js 24 requires a custom image build-and-maintenance pipeline the
  repository does not have, the image must also carry the Go toolchain, and ADR 0082's in-job
  distribution-digest verification is still required — reducing the option to the tarball mechanism
  inside a much heavier environment.

## More Information

- Motivating incident and RCA: [issue #97](https://github.com/windlasstech/slsa-builder/issues/97);
  upstream [npm/cli#9879](https://github.com/npm/cli/issues/9879) (report) and
  [npm/cli#9882](https://github.com/npm/cli/pull/9882) (fix).
- Mechanism investigation sources:
  - setup-node inputs (no npm selection):
    <https://github.com/actions/setup-node/blob/main/action.yml>
  - Node-to-npm mapping: <https://nodejs.org/dist/index.json>
  - npm self-contained tarball (`bundleDependencies`):
    <https://github.com/npm/cli/blob/main/package.json>
  - Registry metadata fields:
    <https://github.com/npm/registry/blob/main/docs/responses/package-metadata.md>
  - npm registry ECDSA signatures: <https://docs.npmjs.com/about-registry-signatures/>
  - Corepack npm status: <https://github.com/nodejs/corepack/pull/418>; key-rotation incident:
    <https://github.com/nodejs/corepack/issues/612>; Node.js 25 unbundling:
    <https://nodejs.org/en/blog/release/v25.0.0>
  - Guidance: SLSA "pin by digest" (<https://slsa.dev/spec/v1.0/threats#dependency-threats>); npm
    trusted publishing requirements (<https://docs.npmjs.com/trusted-publishers/>)
- The initial pin data (version, URL, SHA-512) is recorded in the specification's allowlist when ADR
  0083's fixed release is selected, under the ADR 0082 bump procedure as augmented by the
  review-time cross-checks above.
