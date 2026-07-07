---
parent: Decisions
nav_order: 39
status: accepted
date: 12026-07-06
decision-makers: Yunseo Kim
---

# Scope the GitHub Release Asset Profile to One Asset per Run

## Context and Problem Statement

ADR 0038 added the initial GitHub Release asset profile direction: Windlass trusted logic generates
canonical SLSA provenance for release assets, while `actions/attest` acts as the initial Sigstore
signing and storage adapter. The next decision is the release asset profile's artifact scope and
verification unit.

GitHub Releases commonly contain multiple downloadable files: OS-specific binaries, architecture
variants, checksum files, SBOMs, provenance sidecars, signatures, archives, and sometimes generated
source archives. SLSA v1.2 recommends binding attestations to artifacts rather than releases, while
GitHub `actions/attest` models attestation subjects as named artifacts with digests and supports
both one subject and multiple subjects per attestation. The project needs an initial scope that
keeps the production SLSA3 profile easy to verify, while still allowing full releases to contain
multiple assets through composition.

Should the initial GitHub Release asset profile treat one profile run as one release asset, an
explicit set of release assets, a full GitHub Release, a generated archive from a directory, or a
checksum-manifest-centered asset group?

## Decision Drivers

- Keep the profile's SLSA subject and verifier policy artifact-bound, not release-bound.
- Make one profile run's verification unit as simple and auditable as the npm profile's
  one-package-per-run decision in ADR 0018.
- Avoid partial-success and partial-retry ambiguity for multi-platform or multi-file releases.
- Keep release asset upload, duplicate-name handling, immutable release behavior, and provenance
  verification straightforward for the initial profile.
- Preserve a path for releases that publish multiple OS or architecture assets, checksum files,
  SBOMs, provenance sidecars, and signatures.
- Avoid treating GitHub-generated source archives as ordinary release assets in the initial build
  artifact profile.
- Avoid making checksum manifests the primary SLSA subject when the real downloadable artifacts are
  the files referenced by the checksum manifest.
- Keep future multi-asset orchestration separate from the low-level trusted profile contract.

## Considered Options

- Publish and verify one explicitly named GitHub Release asset file per profile run, with an
  orchestration layer for multi-asset releases.
- Publish and verify an explicit set of GitHub Release asset files per profile run.
- Treat the full GitHub Release as the profile run's verification unit.
- Treat one generated archive from a selected directory as the profile run's release asset.
- Treat a checksum manifest plus referenced files as the profile run's verification unit.
- Include GitHub-generated source archives in the release asset profile.

## Decision Outcome

Chosen option: "Publish and verify one explicitly named GitHub Release asset file per profile run,
with an orchestration layer for multi-asset releases", because it keeps the trusted profile's SLSA
subject model small and artifact-bound while still allowing a higher-level release workflow to
compose complete multi-asset GitHub Releases.

The initial GitHub Release asset profile should define one profile run as exactly one GitHub Release
asset file. That asset is the profile's primary output artifact and primary verification unit. The
profile's SLSA provenance should have exactly one release asset subject for the production path
unless a later ADR defines a multi-subject profile or mode. The release asset subject should
identify the uploaded asset by a profile-defined asset name and digest, with release tag, source
identity, and upload metadata represented in the profile's provenance parameters, dependencies,
byproducts, or verification policy as specified by the architecture documentation.

The initial profile should not treat these as the same low-level verification unit:

- a full GitHub Release;
- a set of OS or architecture assets;
- checksum files plus all referenced artifacts;
- SBOM files plus the artifact they describe;
- provenance sidecars plus the artifact they describe;
- detached signatures plus the artifact they sign;
- GitHub-generated source archives.

Repositories that need to publish multiple release assets should use a higher-level orchestration
layer. That orchestration layer may call the one-asset profile once per OS or architecture artifact,
once per SBOM, once per checksum file, or once per provenance sidecar when those files are intended
to be published release assets. The orchestration layer should own cross-asset concerns such as
release creation order, draft or immutable release workflow, grouping by tag, aggregate release
notes, checksum manifest generation, SBOM set publication, provenance sidecar naming, and
whole-release user experience. It should not weaken the one-asset profile's artifact-bound SLSA
subject contract.

Checksum files, SBOMs, provenance sidecars, and detached signatures may be published as release
assets through the same one-asset profile when the project wants those files to be independently
downloadable and independently attested. They may also be generated or referenced as byproducts of
an orchestration workflow where appropriate. The architecture specification should define when such
files are release assets, byproducts, or verification inputs; this ADR only decides that they are
not implicitly bundled into the low-level profile's verification unit for another asset.

GitHub-generated source archives are excluded from the initial GitHub Release asset profile because
they are not uploaded release assets produced by the trusted profile. Source integrity and generated
source archive verification should be handled by source-track documentation, GitHub release
verification, or a later dedicated decision if needed.

### Consequences

- Good, because the profile's SLSA subject is simple: one release asset file and its digest.
- Good, because release asset upload and verification can fail closed without partial multi-asset
  state handling.
- Good, because the model matches SLSA's artifact-bound provenance guidance more closely than a
  full-release verification unit.
- Good, because the profile remains easy to compose for OS and architecture matrices by calling it
  once per asset.
- Good, because checksum files, SBOMs, provenance sidecars, and detached signatures can still be
  published and attested as first-class release assets when desired.
- Good, because a future orchestration workflow can provide a friendly whole-release abstraction
  without changing the trusted asset-level profile.
- Neutral, because complete release publication will require orchestration for projects with more
  than one asset.
- Bad, because users do not get a one-call multi-platform release asset workflow from the low-level
  profile alone.
- Bad, because cross-asset consistency, such as "all assets in the checksum file were published and
  attested", must be specified in the orchestration layer rather than in the one-asset profile.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification,
workflow implementation, tests, and documentation define:

- one explicitly named GitHub Release asset file as the unit of one profile run;
- exactly one SLSA subject for the production release asset profile path;
- release asset name, local path, digest, upload target, and GitHub Release tag handling for one
  asset;
- failure behavior for missing asset, unexpected directory input, duplicate asset names, upload
  mismatch, digest mismatch, and unsupported source archive inputs;
- outputs for the uploaded asset identity, asset URL or API identifier, and digest values;
- exclusion of full-release, multi-asset, checksum-manifest-centered, SBOM-bundled,
  provenance-sidecar-bundled, detached-signature-bundled, and GitHub-generated source archive
  verification units from the low-level profile;
- an orchestration-layer boundary for multi-OS, multi-architecture, checksum, SBOM, provenance
  sidecar, detached signature, and whole-release workflows;
- documentation explaining that the orchestration layer composes multiple one-asset profile runs
  rather than changing the one-asset SLSA subject contract.

Implementation review should verify that the low-level production release asset profile cannot
silently publish, sign, or verify multiple primary release assets in one run, and that orchestration
logic does not bypass the one-asset profile's provenance, digest, signing, and upload checks for any
published release asset.

## Pros and Cons of the Options

### Publish and verify one explicitly named GitHub Release asset file per profile run

Each invocation produces, signs, uploads, and verifies one named release asset file. Multi-file
releases are composed by a separate orchestration layer that invokes the profile once per asset.

- Good, because it provides the smallest clear artifact-bound verification unit.
- Good, because one subject, one digest, one upload result, and one signed provenance bundle are
  easy to explain and test.
- Good, because failed upload, duplicate asset name, or digest mismatch handling is localized to one
  asset.
- Good, because OS and architecture matrices can map naturally to repeated profile invocations.
- Neutral, because multi-asset releases still require an orchestration layer.
- Bad, because it can feel lower-level than users expect from a release-publishing workflow.

### Publish and verify an explicit set of GitHub Release asset files per profile run

One invocation accepts a manifest, glob, or list of release asset files and creates one provenance
attestation with multiple subjects.

- Good, because it matches `actions/attest` support for multiple subjects in a single attestation.
- Good, because a release with several platform artifacts can be published by one workflow call.
- Good, because shared release metadata appears once in the provenance predicate.
- Bad, because partial upload, partial attestation, already-existing asset, and retry behavior
  become more complex.
- Bad, because all assets in the set must share one build definition and verifier expectation, which
  may not be true for OS or architecture variants built differently.
- Bad, because adding or rebuilding one platform asset after the fact is harder to model cleanly.

### Treat the full GitHub Release as the verification unit

One invocation treats the release tag and all attached assets as the artifact being verified.

- Good, because it matches a user's intuitive concept of "verify this release".
- Good, because it aligns with GitHub immutable release and release attestation concepts.
- Bad, because SLSA recommends artifact-bound build attestations rather than release-bound
  attestations.
- Bad, because releases can contain assets from different builds, environments, and times.
- Bad, because this conflates build provenance with release integrity and orchestration policy.

### Treat one generated archive from a selected directory as the release asset

The profile accepts a directory and creates a tarball or zip file that becomes the single release
asset.

- Good, because the final verification unit is still one release asset file.
- Good, because profile-owned archive generation can make file ordering, permissions, and timestamps
  explicit.
- Bad, because archive format, deterministic packing, symlink behavior, permissions, timestamps, and
  path normalization become part of the trusted profile contract.
- Bad, because accepting arbitrary directories can blur the boundary between build output and
  release packaging.
- Bad, because this is a packaging feature, not the minimal release asset scope decision.

### Treat a checksum manifest plus referenced files as the verification unit

One invocation centers verification on a checksum file that names all release assets and their
digests.

- Good, because checksum manifests are common in GitHub Releases.
- Good, because they are convenient for human and script-based download verification.
- Bad, because the checksum file is not the same artifact as the files it references.
- Bad, because the SLSA subject can become ambiguous: the manifest, the referenced files, or both.
- Bad, because synchronization between the manifest, uploaded assets, and signed provenance requires
  orchestration-level policy.

### Include GitHub-generated source archives

The profile treats GitHub-generated source zip or tarball downloads as release assets.

- Good, because those archives appear on GitHub Release pages and are commonly downloaded.
- Neutral, because GitHub release verification can cover release integrity around immutable
  releases.
- Bad, because GitHub-generated source archives are not uploaded artifacts produced by this trusted
  profile.
- Bad, because they are source snapshots or generated source distributions rather than build outputs
  owned by the release asset profile.
- Bad, because mixing source archive verification into this profile would blur SLSA Build Track and
  Source Track responsibilities.

## More Information

This decision follows ADR 0038 and decides only the initial GitHub Release asset profile's artifact
scope and verification unit. It does not decide the profile workflow filename, exact public input
names, job graph, build or pack command policy, immutable release requirement, release creation
policy, checksum manifest schema, SBOM format, sidecar naming convention, or orchestration workflow
interface.

Reference points considered:

- SLSA v1.2 Distributing Provenance says attestations should be bound to artifacts rather than
  releases, and package ecosystems should support multiple individual attestations per release.
- SLSA v1.2 Build Provenance represents build outputs through the top-level `subject` list, with
  each subject identified by digest.
- GitHub `actions/attest` models subjects as named artifacts with digest maps. It supports one
  subject, multiple subjects by glob or explicit list, and subject enumeration through checksum
  files, but multiple subjects are still artifact subjects rather than a full-release subject.
- GitHub immutable releases protect release tags and release assets after publication and can
  produce release attestations, but that release-level mechanism remains complementary to
  artifact-bound build provenance.
