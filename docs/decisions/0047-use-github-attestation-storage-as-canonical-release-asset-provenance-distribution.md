---
parent: Decisions
nav_order: 47
status: superseded by ADR-0051
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Use GitHub Attestation Storage as Canonical Release Asset Provenance Distribution

## Context and Problem Statement

ADR 0038 selected Windlass-generated SLSA provenance for GitHub Release assets, with
`actions/attest` acting as the initial Sigstore signing and storage adapter. ADR 0039 scoped the
initial production profile to one GitHub Release asset per profile run. ADR 0044 selected a
three-job graph where `provenance-sign` signs the Windlass provenance and `release-upload` verifies
the signed bundle before uploading the release asset. ADR 0045 and ADR 0046 then fixed the release
asset subject name, digest, checksum, and SBOM semantics.

The remaining distribution question is how consumers and downstream automation should discover and
retrieve the signed SLSA provenance for the release asset. GitHub `actions/attest` can upload the
Sigstore bundle to GitHub's artifact attestation storage and also exposes a local `bundle-path` JSON
file. GitHub Releases can also publish that bundle as a downloadable release asset sidecar, but
doing so creates another release asset with its own naming, duplicate handling, and orchestration
concerns.

Should the initial GitHub Release asset profile distribute provenance primarily through GitHub
artifact attestation storage, through a GitHub Release asset sidecar, through both channels, or
through release-level attestation only?

## Decision Drivers

- Keep the production provenance discovery path aligned with ADR 0038's `actions/attest` adapter
  choice.
- Preserve ADR 0039's one-primary-release-asset profile boundary.
- Keep the SLSA provenance bound to the selected release asset digest rather than to the whole
  GitHub Release.
- Support GitHub-native verification with `gh attestation verify` and the artifact attestations API.
- Allow archival, air-gapped, or mirror-friendly bundle export when callers need it.
- Avoid making release asset sidecar upload mandatory for the low-level profile.
- Keep provenance sidecar naming, sidecar attestation, and whole-release UX in the orchestration
  layer unless explicitly enabled.
- Preserve the `release-upload` job's producer-side verification of the signed bundle before release
  mutation.

## Considered Options

- Use GitHub artifact attestation storage as the canonical distribution channel, with optional
  release asset bundle export.
- Require both GitHub artifact attestation storage and a GitHub Release asset bundle sidecar.
- Use only a GitHub Release asset bundle sidecar.
- Use only workflow artifacts or job outputs for provenance distribution.
- Rely on GitHub immutable release attestations or release verification instead of artifact-bound
  SLSA provenance distribution.

## Decision Outcome

Chosen option: "Use GitHub artifact attestation storage as the canonical distribution channel, with
optional release asset bundle export", because this keeps the initial profile on GitHub's native
attestation discovery and verification path while still allowing callers to publish a downloadable
Sigstore bundle when they need portability or archival behavior.

The production GitHub Release asset profile should treat GitHub artifact attestation storage as the
canonical distribution channel for the signed SLSA provenance bundle. The `provenance-sign` job
should invoke the SHA-pinned `actions/attest` custom attestation mode selected by ADR 0038 with
GitHub attestation storage enabled. The resulting attestation should be associated with the
repository and release asset subject digest through GitHub's attestations API.

The `provenance-sign` job should also capture the local JSON-serialized Sigstore bundle produced by
`actions/attest` through `bundle-path` and pass it to `release-upload` as a digest-checked workflow
artifact. That workflow artifact is an internal handoff artifact. It is required for ADR 0044's
producer-side verification before release mutation, but it is not the canonical public distribution
channel by itself.

The `release-upload` job should verify the signed bundle before uploading the selected release
asset. Verification should include the expected Sigstore root, GitHub Actions reusable workflow
signer identity, SLSA `predicateType`, `builder.id`, `buildType`, `externalParameters`, release tag,
subject name, and subject digest. The job should fail before release mutation if the GitHub
attestation storage upload did not produce the expected attestation identity or if the local bundle
cannot be verified.

Publishing the signed bundle as a GitHub Release asset sidecar should be optional. When enabled, it
is an export and compatibility channel, not the canonical provenance distribution channel. The
architecture specification should define the option name, default value, sidecar asset name, sidecar
digest outputs, and duplicate-name behavior. The production default should not require a sidecar
release asset.

A sidecar bundle release asset should not change the selected release asset's SLSA subject. It
should not be inserted into `subject[0].digest`, should not replace GitHub artifact attestation
storage, and should not be required for ordinary `gh attestation verify` based verification. If the
sidecar is itself treated as a published release asset requiring SLSA provenance, that attestation
is a separate one-asset profile invocation or an orchestration-layer responsibility under ADR 0039
and ADR 0046.

The profile should not rely on workflow artifacts, unsigned job outputs, logs, release notes, or
GitHub immutable release attestations as the public SLSA provenance distribution mechanism. Those
surfaces may provide handles, diagnostics, or complementary release integrity evidence, but they do
not replace artifact-bound signed SLSA provenance discoverable through GitHub attestation storage.

### Consequences

- Good, because the canonical path matches GitHub's official artifact attestation storage and
  `gh attestation verify` workflow.
- Good, because the release asset list does not need to contain provenance sidecars for the default
  production path.
- Good, because ADR 0044 can still verify the local signed bundle before release mutation.
- Good, because projects that need offline, mirrored, or archival verification can opt into a
  downloadable bundle sidecar.
- Good, because release-level attestations and immutable release verification remain complementary
  evidence rather than substitutes for artifact-bound build provenance.
- Neutral, because consumers that avoid GitHub's attestations API need the optional sidecar or a
  separate export workflow.
- Bad, because the canonical public discovery path depends on GitHub artifact attestation storage
  availability and support boundaries.
- Bad, because optional sidecar support still requires naming, duplicate handling, and verification
  rules when enabled.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification,
workflow implementation, tests, and verifier guidance define:

- GitHub artifact attestation storage as the canonical public distribution channel for the signed
  Windlass SLSA provenance bundle;
- `actions/attest` custom mode with GitHub attestation storage enabled in the `provenance-sign` job;
- capture of the local `bundle-path` JSON bundle as a digest-checked workflow artifact for the
  `release-upload` job;
- producer-side verification of the local signed bundle before GitHub Release asset upload;
- public outputs for the attestation ID, attestation URL, bundle digest, subject name, and subject
  digest as appropriate;
- ordinary consumer verification guidance based on GitHub artifact attestation storage and
  `gh attestation verify` or equivalent API-backed verification;
- an optional release asset sidecar export mode with explicit default, sidecar asset naming,
  duplicate-name failure behavior, and verifier guidance;
- documentation that workflow artifacts, logs, release notes, and release-level attestations are not
  the canonical SLSA provenance distribution mechanism.

Implementation review should verify that no production path silently skips GitHub attestation
storage upload, treats an unsigned or unverified bundle as distributable provenance, requires a
release asset sidecar for the default verification path, or uses GitHub release-level attestation as
a substitute for the Windlass artifact-bound SLSA provenance bundle.

## Pros and Cons of the Options

### Use GitHub artifact attestation storage as canonical distribution, with optional sidecar export

The signing job uploads the signed bundle to GitHub artifact attestation storage through
`actions/attest` and also passes the local bundle to the upload job for verification. Callers may
optionally publish the bundle as a release asset sidecar for portability.

- Good, because it uses the native storage and discovery path provided by the selected signing
  adapter.
- Good, because it supports `gh attestation verify` and digest-based attestation lookup.
- Good, because it keeps the low-level profile centered on one primary release asset.
- Good, because it leaves room for downloadable sidecars without making them mandatory.
- Neutral, because offline consumers need an explicit sidecar or prior bundle export.
- Bad, because GitHub artifact attestation storage is a provider-specific dependency.

### Require both GitHub artifact attestation storage and a release asset bundle sidecar

Every production profile run uploads the signed attestation to GitHub storage and publishes the same
bundle as a GitHub Release asset sidecar.

- Good, because consumers get both GitHub-native verification and a downloadable bundle file.
- Good, because provenance can be mirrored or archived with the release assets.
- Bad, because the low-level one-asset profile would always perform at least two release asset
  uploads.
- Bad, because sidecar upload failure creates ambiguity about whether the primary asset should be
  uploaded or rolled back.
- Bad, because sidecar naming, duplicate detection, and sidecar attestation policy become mandatory
  initial-profile complexity.

### Use only a GitHub Release asset bundle sidecar

The signed bundle is uploaded as a release asset next to the primary artifact, and GitHub artifact
attestation storage is not the canonical discovery channel.

- Good, because provenance is visible and downloadable from the GitHub Release page.
- Good, because consumers can archive the artifact and bundle together.
- Bad, because it avoids the native GitHub artifact attestation storage and lookup path selected by
  ADR 0038.
- Bad, because `gh attestation verify` defaults and GitHub attestation UI/API integration become
  less central.
- Bad, because every release asset provenance file becomes release upload lifecycle surface.

### Use only workflow artifacts or job outputs for provenance distribution

The signed bundle is retained only as a workflow artifact or output from the reusable workflow.

- Good, because it is simple for producer-side handoff between jobs.
- Good, because it avoids adding another release asset.
- Bad, because workflow artifacts are not the artifact's durable public provenance distribution
  surface.
- Bad, because job outputs and logs are not signed provenance and are unsuitable for long-term
  consumer verification.
- Bad, because consumers outside the workflow run would not have a stable discovery path.

### Rely on GitHub immutable release attestations or release verification instead

Consumers use GitHub release attestation and `gh release verify` or `gh release verify-asset` rather
than artifact-bound Windlass SLSA provenance distribution.

- Good, because release verification is a simple GitHub-native integrity check for immutable
  releases.
- Good, because it can prove that release assets match GitHub's signed release record.
- Bad, because release-level attestation does not carry the Windlass `buildType`, `builder.id`, or
  complete SLSA `externalParameters` contract.
- Bad, because it treats release integrity as a substitute for artifact-bound build provenance.
- Bad, because it conflicts with ADR 0038's decision to generate Windlass-owned SLSA provenance for
  release assets.

## More Information

This decision follows ADR 0038, ADR 0039, ADR 0043, ADR 0044, ADR 0045, and ADR 0046. It decides
only the public distribution channel for signed release asset SLSA provenance and the role of
optional bundle sidecar export. It does not decide the final sidecar filename convention, verifier
CLI UX, release manifest schema, SBOM attestation policy, or future whole-release orchestration
workflow.

Reference points considered:

- GitHub `actions/attest` creates Sigstore-signed in-toto attestations, uploads them to GitHub's
  attestations API, associates them with the repository, and exposes a local JSON-serialized
  Sigstore bundle through `bundle-path`.
- GitHub artifact attestation verification supports `gh attestation verify` against GitHub storage
  and local bundle verification through downloaded or supplied bundles.
- GitHub's artifact attestations API stores attestations by subject digest and repository, enabling
  digest-based discovery for artifacts.
- GitHub immutable releases and release attestations can verify release integrity and asset bytes
  but do not replace artifact-bound SLSA build provenance with Windlass-specific verifier policy.
- SLSA v1.2 recommends binding provenance to artifacts rather than releases and distributing
  provenance so consumers can map a downloaded artifact to its attestation.
- Windlass workflow hardening guidance requires SHA-pinned actions, minimal top-level permissions,
  job-level permission elevation, OIDC for signing, and explicit permissions for artifact
  attestations and release asset upload.
