---
parent: Decisions
nav_order: 36
status: accepted
date: 12026-07-06
decision-makers: Yunseo Kim
---

# Use a Three-Job Digest-Verified Publish Graph

## Context and Problem Statement

ADR 0029 selected Windlass-generated SLSA provenance as the canonical npm package provenance, ADR
0035 selected `actions/attest` as the initial Sigstore signing adapter, and ADR 0024 selected OIDC
trusted publishing without npm publish secrets. Together, those decisions require a workflow graph
that builds a package, signs provenance for the exact packed artifact, and mutates the npm registry
only after the signed provenance and package artifact have been checked.

The open boundary is how the initial production JS/TS npm package SLSA3 reusable workflow should
split build, provenance-signing, and publish responsibilities across GitHub Actions jobs. The split
must preserve SLSA Build L3 signing-key isolation, keep caller-controlled package scripts away from
signing and publish permissions, and make the publish step verify the exact tarball and signed
provenance bundle it submits to npm.

Should the initial production profile use one job, two jobs, three jobs, four jobs, or leave publish
to the caller after a build-and-sign reusable workflow?

## Decision Drivers

- Keep caller-controlled install, build, pack, and package lifecycle scripts away from provenance
  signing permissions and npm registry mutation capability.
- Preserve SLSA v1.2 Build L3's requirement that provenance authentication material is not
  accessible to user-defined build steps.
- Make each cross-job artifact handoff digest-verified rather than name- or path-trusted.
- Require the publish job to verify both the packed artifact and signed provenance bundle before npm
  registry mutation.
- Keep Windlass-generated SLSA provenance, not unsigned workflow outputs, as the source of truth for
  verifier policy.
- Keep `actions/attest` signing permissions scoped to a signing job, as selected by ADR 0035.
- Keep npm trusted publishing authentication separate from provenance generation and signing.
- Follow the SLSA GitHub Generator precedent of reusable workflow isolation, job-level VM isolation,
  and hash-protected artifact handoff.
- Define a job graph that is precise enough for architecture specifications and implementation
  tests.

## Considered Options

- Use a three-job digest-verified graph: `build` → `provenance-sign` → `publish`.
- Use one job for build, provenance signing, and publish.
- Use two jobs: `build` → `publish-sign`.
- Use four jobs: `build` → `provenance-generate` → `sign` → `publish`.
- Build and sign inside the reusable workflow, then leave publish to the caller workflow.

## Decision Outcome

Chosen option: "Use a three-job digest-verified graph: `build` → `provenance-sign` → `publish`",
because it gives the initial production profile clear job isolation, minimal permission scopes, and
a publish-time verification gate for both the package tarball and signed provenance bundle before
npm registry mutation.

The initial production JS/TS npm package SLSA3 reusable workflow should define three primary jobs:

```text
build -> provenance-sign -> publish
```

The `build` job should run the caller-selected package build contract, produce the packed npm
tarball, validate package metadata from the packed artifact, calculate the package tarball `sha256`
and `sha512` digests, upload the tarball as a workflow artifact, and expose only the package
identity and digest handles needed by later jobs and public workflow outputs.

The `provenance-sign` job should download the package tarball artifact, recalculate its digest,
compare it with the `build` job's expected digest outputs, generate the Windlass SLSA provenance v1
statement for exactly that tarball, verify that the statement subject matches the tarball digest,
sign the statement through the SHA-pinned `actions/attest` adapter selected by ADR 0035, and upload
the resulting signed Sigstore bundle as a workflow artifact. The signing job should not have npm
publish credentials or npm registry mutation authority.

The `publish` job should download the package tarball and signed provenance bundle artifacts,
recalculate both artifact digests, verify that those digests match the handoff values produced by
the previous jobs, verify the signed provenance bundle against the expected Sigstore root and GitHub
Actions reusable workflow signer identity, extract and verify the SLSA statement expectations, and
only then run `npm publish <verified-tarball> --provenance-file <verified-bundle>` or the equivalent
npm external provenance-file submission. The publish job should fail before registry mutation if any
digest, signature, signer identity, package identity, subject, `predicateType`, `builder.id`,
`buildType`, `externalParameters`, source identity, package-directory, or runtime-policy check
fails.

The initial job permission boundary should be:

- `build`: `contents: read`; no `id-token: write`; no `attestations: write`; no npm publish
  authentication; no publish secrets.
- `provenance-sign`: `contents: read`, `id-token: write`, `attestations: write` when GitHub
  attestation storage is used, and optional `artifact-metadata: write` only when linked artifact
  metadata is intentionally created; no npm publish authentication; no registry mutation authority.
- `publish`: `contents: read` and only the OIDC or registry permissions required for npm trusted
  publishing; no `attestations: write` unless a later ADR adds post-publish attestation behavior.

Cross-job handoff should use immutable GitHub workflow artifacts and trusted job outputs as handles.
The architecture specification should define exact artifact names or IDs, digest encodings, digest
algorithms, download locations, and failure behavior. Jobs must not trust artifact names, paths,
workflow outputs, or logs as substitutes for recalculating digests and verifying the signed
provenance bundle. Unsigned public workflow outputs such as package identity and tarball digest
remain release handles only; they are not substitutes for signed provenance verification.

After publishing, the `publish` job should resolve npm registry metadata for the published package
version when possible and verify that the registry-visible package identity, tarball URL or name,
integrity or digest metadata, and provenance linkage correspond to the verified tarball and
submitted Windlass provenance bundle. The architecture specification may refine which npm registry
metadata is mandatory versus best-effort when npm registry behavior or tooling support changes.

### Consequences

- Good, because package build scripts cannot access provenance signing or npm publish authority.
- Good, because `actions/attest` permissions are scoped to the signing job rather than the build
  job.
- Good, because npm registry mutation is gated by digest checks and signed provenance verification.
- Good, because the job graph mirrors SLSA GitHub Generator's isolated-job and hash-protected
  handoff model while keeping Windlass-specific provenance semantics.
- Good, because architecture specifications can assign clear inputs, outputs, permissions, and
  failure behavior to each job.
- Good, because the future `sigstore-go` migration path can replace signing internals without
  collapsing the build/publish boundary.
- Neutral, because the publish job performs verifier-like checks inside the producer workflow; final
  consumers should still verify published artifacts independently.
- Bad, because the workflow has more jobs, handoff artifacts, and verification code than a one- or
  two-job design.
- Bad, because Windlass must maintain tooling to verify its own signed bundle before publish.
- Bad, because artifact handoff implementation must be tested carefully to avoid name/path
  confusion, stale artifacts, digest encoding drift, or accidental fallback behavior.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification,
workflow implementation, tests, and verifier guidance define:

- exactly three primary production jobs named or equivalent to `build`, `provenance-sign`, and
  `publish`;
- the job-level permissions and absence of signing or publish authority from caller-controlled build
  steps;
- package tarball artifact upload, download, digest calculation, and digest comparison behavior;
- signed provenance bundle artifact upload, download, digest calculation, and digest comparison
  behavior;
- Windlass SLSA statement generation and subject digest verification before signing;
- Sigstore bundle verification in the publish job before `npm publish`;
- publish-time checks for signer identity, `predicateType`, `builder.id`, `buildType`,
  `externalParameters`, source identity, package identity, package directory, runtime policy, and
  tarball digest;
- `npm publish --provenance-file` or equivalent external provenance-file submission using only the
  verified tarball and verified signed bundle paths;
- post-publish npm registry metadata checks and documented best-effort versus mandatory behavior;
- hard-failure behavior for missing artifacts, digest mismatch, signature mismatch, unexpected
  provenance fields, unsupported registry metadata, and any attempted fallback to npm automatic
  provenance, unsigned provenance, token-based publish, or caller-managed publish.

Implementation review should verify that no path can publish a package under the production SLSA3
builder identity unless the package tarball and signed Windlass provenance bundle have both passed
the publish job's verification gate.

## Pros and Cons of the Options

### Use a three-job digest-verified graph

The reusable workflow separates package creation, provenance signing, and npm registry mutation into
`build`, `provenance-sign`, and `publish` jobs. Each cross-job artifact handoff is checked by
digest, and the publish job verifies the signed provenance before publishing.

- Good, because it creates the clearest permission and trust boundary for the initial profile.
- Good, because build scripts cannot access signing credentials or publish authority.
- Good, because the publish job can behave like a producer-side verifier before registry mutation.
- Good, because it follows the SLSA GitHub Generator pattern of job isolation plus hash-protected
  artifact exchange.
- Good, because it keeps Windlass-generated provenance, not workflow outputs, as the verifier source
  of truth.
- Bad, because it requires more implementation and testing than simpler job graphs.
- Bad, because bundle verification logic must exist before publish, not only in downstream verifier
  tooling.

### Use one job for build, signing, and publish

One job checks out source, runs install/build/pack, generates and signs provenance, and publishes to
npm.

- Good, because it is simple and avoids artifact upload/download handoff.
- Good, because local paths are easy to reason about during an early prototype.
- Bad, because caller-controlled package scripts share a runner environment with signing and publish
  permissions.
- Bad, because it weakens the SLSA Build L3 signing-key isolation story.
- Bad, because it conflicts with the digest-verified isolated handoff direction.

### Use two jobs: `build` then `publish-sign`

The build job creates the tarball, and a second job verifies the tarball, signs provenance, and
publishes.

- Good, because build scripts are isolated from signing and publish permissions.
- Good, because it is simpler than a three-job graph.
- Good, because the publish-capable job can verify the tarball before signing and publishing.
- Bad, because provenance signing and registry mutation share one job boundary.
- Bad, because publish does not consume a previously signed bundle through an explicit verification
  handoff.
- Bad, because it is less precise for specifying publish-time verification of signed provenance.

### Use four jobs: `build`, `provenance-generate`, `sign`, and `publish`

The workflow separates unsigned provenance statement generation from Sigstore signing.

- Good, because the adapter boundary between statement generation and signing is explicit.
- Good, because replacing `actions/attest` with `sigstore-go` later may be mechanically simpler.
- Good, because unsigned statement validation can happen before signing.
- Bad, because it creates another cross-job artifact handoff and more digest state to verify.
- Bad, because unsigned provenance statement artifacts require their own integrity and
  path-confusion controls.
- Bad, because it is too much ceremony for the initial production profile.

### Build and sign in the reusable workflow, then leave publish to the caller

The trusted reusable workflow returns package and provenance artifacts; the caller workflow
downloads them and performs npm publish.

- Good, because it resembles SLSA GitHub Generator Node.js custom publishing examples.
- Good, because callers can customize publishing workflows.
- Bad, because the production profile would not control the registry mutation path.
- Bad, because callers could publish without verifying the signed provenance bundle or could submit
  a different provenance file.
- Bad, because it conflicts with ADR 0029's production profile requirement to submit Windlass
  canonical provenance through npm's external provenance-file path.
- Bad, because publish failures and downgrade behavior would become caller-specific rather than
  profile-specified.

## More Information

This decision follows ADR 0024, ADR 0025, ADR 0029, ADR 0034, and ADR 0035. It decides the initial
production workflow job graph and artifact handoff boundary only. It does not change package manager
selection, private dependency credential support, canonical provenance contents, signing adapter
choice, public workflow outputs, or signed release metadata.

Reference points considered:

- SLSA v1.2 Build Requirements require authentic provenance at Build L2 and unforgeable provenance
  at Build L3. Secret material used to authenticate provenance must not be accessible to
  user-defined build steps, and builds must run in isolated, ephemeral environments.
- SLSA v1.2 Build Provenance defines `subject`, `builder.id`, `buildType`, `externalParameters`,
  `internalParameters`, `resolvedDependencies`, and verifier-relevant provenance semantics. It also
  separates signer identity from builder identity and requires consumers to accept only expected
  signer-builder pairs.
- SLSA v1.2 Verifying Artifacts recommends checking the provenance envelope signature, subject
  digest, `predicateType`, trusted builder identity, `buildType`, and `externalParameters`;
  unrecognized external parameters should generally fail verification.
- The SLSA GitHub Generator technical design uses reusable workflows to prevent caller workflow
  interference, GitHub-hosted job isolation as the VM boundary, GitHub artifacts for larger job
  handoff data, and job outputs as a trusted channel for hashes and handles.
- The SLSA GitHub Generator Node.js builder produces package and provenance artifacts, exposes
  artifact names and SHA-256 values, and shows publish flows that securely download the package and
  provenance before running `npm publish --provenance-file`.
- The SLSA GitHub Generator `secure-package-download`, `secure-attestations-download`, and
  `nodejs/publish` actions use artifact names plus expected SHA-256 values as a package/provenance
  handoff model for publishing.
- GitHub `actions/attest` can create Sigstore bundle JSON, supports custom predicates, exposes a
  `bundle-path` output, and requires `id-token: write` plus attestation permissions only where the
  signing/storage action runs.
- GitHub `actions/upload-artifact` v4 and later make uploaded artifacts immutable, expose artifact
  IDs, URLs, and SHA-256 digests, and reject accidental same-name mutation patterns.
- GitHub `actions/download-artifact` supports downloading by name or artifact ID and has digest
  mismatch handling, but Windlass should still recalculate and compare package and bundle digests as
  part of its own trust boundary.
- npm provenance documentation supports publishing packages with provenance from cloud-hosted CI and
  third-party publish tooling that supplies an external provenance file, while npm trusted
  publishing can separately authenticate publish operations without long-lived npm tokens.
