---
parent: Decisions
nav_order: 44
status: superseded by ADR-0049
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Use a Three-Job Permission Boundary for GitHub Release Assets

## Context and Problem Statement

ADR 0038 selected Windlass-generated SLSA provenance with `actions/attest` as the signing and
storage adapter for GitHub Release assets. ADR 0039 scoped the profile to one explicitly named
release asset per profile run. ADR 0043 then selected an upload-only lifecycle policy: the profile
uploads one asset to an existing GitHub Release selected by an existing tag, fails when the tag or
release is missing, and never overwrites an existing asset.

The remaining boundary is how the GitHub Actions jobs and `GITHUB_TOKEN` permissions should be
split. The profile needs to run caller-controlled build or packaging steps, sign Windlass-generated
SLSA provenance, and mutate a GitHub Release by uploading one asset. Combining those
responsibilities in a single job would put provenance signing authority and release mutation
authority in the same runner environment as user-defined build steps.

Should the initial GitHub Release asset profile use one job, two jobs, three jobs, or leave release
upload outside the reusable profile?

## Decision Drivers

- Keep caller-controlled build and packaging steps away from provenance signing permissions.
- Keep caller-controlled build and packaging steps away from GitHub Release mutation authority.
- Preserve SLSA v1.2 Build L3's requirement that provenance authentication material is not
  accessible to user-defined build steps.
- Keep `actions/attest` OIDC and attestation-storage permissions scoped to the job that signs
  provenance.
- Keep `contents: write` release mutation authority scoped to the job that uploads the verified
  release asset.
- Require release upload to verify both the asset and signed provenance bundle before mutating the
  existing GitHub Release.
- Match ADR 0036's npm package precedent: build, signing, and publish or upload mutation are
  separate digest-verified jobs.
- Follow Windlass workflow hardening guidance: minimal top-level permissions and job-level elevation
  only where required.

## Considered Options

- Use a three-job digest-verified graph: `build` → `provenance-sign` → `release-upload`.
- Use one job for build, provenance signing, and release upload.
- Use two jobs: `build` → `sign-and-upload`.
- Use two jobs: `build-and-sign` → `release-upload`.
- Build and sign inside the reusable workflow, then leave release upload to the caller or
  orchestration workflow.

## Decision Outcome

Chosen option: "Use a three-job digest-verified graph: `build` → `provenance-sign` →
`release-upload`", because it gives the initial GitHub Release asset profile clear job isolation,
minimal permission scopes, and an upload-time verification gate before GitHub Release mutation.

The initial production GitHub Release asset SLSA3 reusable workflow should define three primary
jobs:

```text
build -> provenance-sign -> release-upload
```

The `build` job should run the profile's caller-controlled build or packaging contract, produce the
single release asset file, validate that the input resolves to exactly one file, calculate the asset
digest, upload the asset as a workflow artifact, and expose only the asset identity and digest
handles needed by later jobs and public workflow outputs.

The `provenance-sign` job should download the asset artifact, recalculate its digest, compare it
with the `build` job's expected digest outputs, generate the Windlass SLSA provenance v1 statement
for exactly that asset, verify that the statement subject matches the asset digest, sign the
statement through the SHA-pinned `actions/attest` adapter selected by ADR 0038, and upload the
resulting signed Sigstore bundle as a workflow artifact. The signing job should not have GitHub
Release mutation authority.

The `release-upload` job should download the asset and signed provenance bundle artifacts,
recalculate both artifact digests, verify that those digests match the handoff values produced by
the previous jobs, verify the signed provenance bundle against the expected Sigstore root and GitHub
Actions reusable workflow signer identity, extract and verify the SLSA statement expectations, and
only then upload the verified asset to the existing GitHub Release selected by the requested tag.
The upload job should fail before release mutation if any digest, signature, signer identity, asset
identity, subject, `predicateType`, `builder.id`, `buildType`, `externalParameters`, source
identity, release tag, release state, or duplicate-asset check fails.

The initial job permission boundary should be:

- `build`: `contents: read`; no `id-token: write`; no `attestations: write`; no `contents: write`;
  no release mutation authority.
- `provenance-sign`: `contents: read`, `id-token: write`, `attestations: write` when GitHub
  attestation storage is used, and optional `artifact-metadata: write` only when linked artifact
  metadata is intentionally created; no `contents: write`; no release mutation authority.
- `release-upload`: `contents: write`; no `id-token: write`; no `attestations: write`; no signing
  authority. The job should use its write permission only for the ADR 0043-selected operation of
  uploading one non-duplicate asset to an existing GitHub Release.

The reusable workflow should set minimal top-level permissions, preferably `permissions: {}` or the
smallest read-only default that GitHub Actions permits for the implementation, and should grant the
above permissions at the job level. Caller documentation should make the required caller workflow
permissions explicit so that the called workflow can receive the needed job-level permissions
without granting broad default write access to every job.

Cross-job handoff should use immutable GitHub workflow artifacts and trusted job outputs as handles.
The architecture specification should define exact artifact names or IDs, digest encodings, digest
algorithms, download locations, and failure behavior. Jobs must not trust artifact names, paths,
workflow outputs, or logs as substitutes for recalculating digests and verifying the signed
provenance bundle. Unsigned public workflow outputs such as uploaded asset name, asset URL, or
digest remain release handles only; they are not substitutes for signed provenance verification.

The `release-upload` job should not create tags, create releases, publish draft releases, edit
release notes, change prerelease state, change latest-release state, delete assets, overwrite
assets, or invoke clobber behavior. Those lifecycle mutations remain outside the low-level profile
under ADR 0043.

### Consequences

- Good, because caller-controlled build steps cannot access provenance signing authority.
- Good, because caller-controlled build steps cannot access GitHub Release mutation authority.
- Good, because `actions/attest` OIDC and attestation permissions are scoped to the signing job.
- Good, because `contents: write` release mutation authority is scoped to the upload job.
- Good, because release upload is gated by digest checks and signed provenance verification.
- Good, because the job graph mirrors the npm package profile's ADR 0036 split while adapting the
  mutation boundary from npm publish to GitHub Release asset upload.
- Good, because a future `sigstore-go` migration can replace signing internals without collapsing
  the build/sign/upload boundary.
- Neutral, because the upload job performs producer-side verification; final consumers should still
  verify downloaded assets independently.
- Bad, because the workflow has more jobs, handoff artifacts, and verification logic than one- or
  two-job designs.
- Bad, because Windlass must maintain tooling to verify its own signed bundle before release upload.
- Bad, because artifact handoff implementation must be tested carefully to avoid name/path
  confusion, stale artifacts, digest encoding drift, or accidental fallback behavior.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification,
workflow implementation, tests, and verifier guidance define:

- exactly three primary production jobs named or equivalent to `build`, `provenance-sign`, and
  `release-upload`;
- top-level workflow permissions that default to no access or minimal read-only access;
- job-level permission requirements for `build`, `provenance-sign`, and `release-upload`;
- absence of signing and release mutation authority from caller-controlled build steps;
- absence of release mutation authority from the signing job;
- absence of signing authority from the release upload job;
- release asset artifact upload, download, digest calculation, and digest comparison behavior;
- signed provenance bundle artifact upload, download, digest calculation, and digest comparison
  behavior;
- Windlass SLSA statement generation and subject digest verification before signing;
- Sigstore bundle verification in the `release-upload` job before GitHub Release mutation;
- upload-time checks for signer identity, `predicateType`, `builder.id`, `buildType`,
  `externalParameters`, source identity, release tag, release state, asset name, and asset digest;
- hard-failure behavior for missing artifacts, digest mismatch, signature mismatch, unexpected
  provenance fields, missing tag, missing release, duplicate asset name, and any attempted fallback
  to unsigned provenance, `actions/attest` default provenance, release creation, draft publication,
  or asset clobbering.

Implementation review should verify that no path can upload a GitHub Release asset under the
production SLSA3 builder identity unless the asset and signed Windlass provenance bundle have both
passed the `release-upload` job's verification gate.

## Pros and Cons of the Options

### Use a three-job digest-verified graph

The reusable workflow separates asset creation, provenance signing, and GitHub Release mutation into
`build`, `provenance-sign`, and `release-upload` jobs. Each cross-job artifact handoff is checked by
digest, and the upload job verifies the signed provenance before uploading the asset.

- Good, because it creates the clearest permission and trust boundary for the initial GitHub Release
  asset profile.
- Good, because build steps cannot access signing credentials or release mutation authority.
- Good, because signing and release upload authorities are never present in the same job.
- Good, because the upload job can behave like a producer-side verifier before mutating the release.
- Good, because it follows the SLSA GitHub Generator and ADR 0036 pattern of job isolation plus
  hash-protected artifact exchange.
- Good, because it keeps Windlass-generated provenance, not workflow outputs, as the verifier source
  of truth.
- Bad, because it requires more implementation and testing than simpler job graphs.
- Bad, because bundle verification logic must exist before upload, not only in downstream verifier
  tooling.

### Use one job for build, signing, and release upload

One job checks out source, runs build or packaging steps, generates and signs provenance, and
uploads the release asset.

- Good, because it is simple and avoids artifact upload/download handoff.
- Good, because local paths are easy to reason about during an early prototype.
- Bad, because caller-controlled build steps share a runner environment with signing and release
  mutation permissions.
- Bad, because it weakens the SLSA Build L3 signing-key isolation story.
- Bad, because a compromised build step could both influence provenance inputs and attempt release
  mutation.

### Use two jobs: `build` then `sign-and-upload`

The build job creates the asset, and a second job verifies the asset, signs provenance, and uploads
the release asset.

- Good, because build steps are isolated from signing and release mutation permissions.
- Good, because it is simpler than a three-job graph.
- Good, because the privileged job can verify the asset before signing and uploading.
- Bad, because provenance signing and GitHub Release mutation share one job boundary.
- Bad, because upload does not consume a previously signed bundle through an explicit verification
  handoff.
- Bad, because a signing-adapter failure or fallback path is closer to the release mutation
  boundary.

### Use two jobs: `build-and-sign` then `release-upload`

The first job creates the asset and signs provenance. The second job verifies the signed bundle and
uploads the asset.

- Good, because release mutation authority is isolated from signing authority.
- Good, because the upload job can verify the signed provenance before mutation.
- Bad, because caller-controlled build steps share a runner environment with provenance signing
  permissions.
- Bad, because this is a weaker fit for SLSA Build L3 signing-key isolation than the selected
  three-job split.
- Bad, because it conflicts with ADR 0038's requirement to keep signing and GitHub attestation
  storage permissions away from caller-controlled build steps.

### Build and sign in the reusable workflow, then leave upload to the caller

The trusted reusable workflow returns the release asset and signed provenance bundle; the caller or
a separate orchestration workflow downloads them and performs GitHub Release upload.

- Good, because the low-level profile would not need `contents: write` release mutation authority.
- Good, because callers could integrate custom release upload or orchestration logic.
- Bad, because the production release asset profile would not control the release mutation path
  selected by ADR 0043.
- Bad, because callers could upload without verifying the signed provenance bundle or could upload a
  different asset from the one that was signed.
- Bad, because upload failures, duplicate asset behavior, and downgrade behavior would become
  caller-specific rather than profile-specified.

## More Information

This decision follows ADR 0038, ADR 0039, ADR 0040, and ADR 0043. It decides the initial production
GitHub Release asset profile's job graph and GitHub token permission boundary only. It does not
change the release asset lifecycle policy, release creation policy, immutable release requirement,
canonical provenance contents, signing adapter choice, public workflow outputs, or future
orchestration workflow interface.

Reference points considered:

- SLSA v1.2 Build Requirements require authentic provenance at Build L2 and unforgeable provenance
  at Build L3. Secret material used to authenticate provenance must not be accessible to
  user-defined build steps, and builds must run in isolated, ephemeral environments.
- SLSA v1.2 Verifying Artifacts recommends checking the provenance envelope signature, subject
  digest, `predicateType`, trusted builder identity, `buildType`, and `externalParameters`;
  unrecognized external parameters should generally fail verification.
- GitHub Actions supports `permissions` at workflow and job level, and when any explicit permission
  set is specified, unspecified permissions are set to `none`.
- Windlass workflow hardening guidance requires explicit minimal top-level permissions and job-level
  elevation only when required.
- GitHub `actions/attest` generates Sigstore-signed in-toto attestations, supports custom predicate
  mode, exposes a `bundle-path` output, and requires `id-token: write` plus `attestations: write`
  only where signing and attestation storage occur.
- GitHub Release asset upload uses GitHub Release mutation authority, represented in GitHub Actions
  by `contents: write`; ADR 0043 narrows that mutation to one non-duplicate asset upload on an
  existing release.
- The SLSA GitHub Generator generic workflow examples use a separate provenance job with
  `id-token: write` and `contents: write` when uploading provenance to releases, while the Node.js
  builder examples use expected SHA-256 values and secure download actions before package publish.
- ADR 0036 selected the stricter Windlass npm pattern of separate build, signing, and mutation jobs
  with digest-verified handoff, which this decision adapts to GitHub Release assets.
