---
parent: Decisions
nav_order: 46
status: accepted
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Keep Checksums and SBOMs Out of the Release Asset Subject Digest

## Context and Problem Statement

ADR 0038 selected Windlass-generated SLSA provenance for GitHub Release assets. ADR 0039 scoped the
initial production profile to exactly one GitHub Release asset per profile run and explicitly kept
checksum files, SBOM files, provenance sidecars, and detached signatures out of the low-level
verification unit for another asset. ADR 0044 selected a three-job graph that recalculates asset and
bundle digests across handoffs. ADR 0045 selected the final GitHub Release asset name as
`subject[0].name` and lowercase hexadecimal SHA-256 of the uploaded asset bytes as
`subject[0].digest.sha256`.

The remaining digest relationship question is how the selected release asset subject digest relates
to checksum files and SBOMs. GitHub Releases commonly publish checksum manifests and SBOM files next
to binaries. Those files can be useful verification aids and may themselves be release assets, but
they are not the same bytes as the primary release asset.

Should checksum files and SBOMs extend, replace, or otherwise participate in the initial GitHub
Release asset profile's SLSA subject digest?

## Decision Drivers

- Keep the one-asset profile's SLSA subject digest bound to exactly the selected release asset
  bytes.
- Preserve ADR 0039's separation between the low-level one-asset profile and multi-asset release
  orchestration.
- Avoid treating checksum manifests or SBOM documents as substitutes for SLSA build provenance over
  the artifact they describe.
- Allow checksum files, SBOMs, provenance sidecars, and detached signatures to be independently
  published and attested when they are release assets.
- Keep consumer verification simple: the downloaded asset bytes must match the SLSA subject digest.
- Keep cross-asset completeness, checksum manifest generation, SBOM set publication, and
  whole-release policy in the orchestration layer.
- Align with SLSA v1.2's artifact-bound provenance guidance and GitHub `actions/attest` SBOM mode,
  where the artifact remains the subject and the SBOM is the predicate.

## Considered Options

- Keep checksum files and SBOMs out of the release asset subject digest.
- Add checksum file and SBOM digests to the primary release asset subject digest map.
- Include the release asset, checksum file, and SBOM as multiple subjects in one provenance
  statement.
- Use the checksum manifest as the primary SLSA subject.
- Use the SBOM file as the primary SLSA subject.

## Decision Outcome

Chosen option: "Keep checksum files and SBOMs out of the release asset subject digest", because the
initial GitHub Release asset profile's SLSA subject should describe exactly the selected asset
bytes, while checksum files and SBOMs are either independent release assets or supporting metadata.

The production GitHub Release asset profile should keep the SLSA subject digest limited to the one
selected release asset:

```json
"subject": [
  {
    "name": "<github-release-asset-name>",
    "digest": {
      "sha256": "<lowercase-hex-sha256-of-release-asset-bytes>"
    }
  }
]
```

Checksum files, SBOM files, provenance sidecars, detached signatures, and other supporting files do
not extend or replace `subject[0].digest`. Their digests should not be inserted into the selected
release asset subject digest map under custom keys. The subject digest map should contain only
cryptographic digests of the selected release asset bytes, keyed by digest algorithm.

When a checksum file, SBOM file, provenance sidecar, detached signature, or similar supporting file
is published as a GitHub Release asset, it should be handled by its own one-asset profile
invocation. For example, a `checksums.txt` release asset can have
`subject[0].name == "checksums.txt"` and a subject digest over the `checksums.txt` bytes. An SBOM
release asset can similarly be attested as the asset whose bytes are uploaded.

When checksum files or SBOMs are generated only to support verification, diagnostics, release notes,
or orchestration policy, they may be recorded as byproducts, referenced by orchestration-layer
metadata, linked through a future release manifest, or attested with a separate predicate. They are
not part of the low-level profile's primary SLSA subject for another release asset.

The architecture specification may require optional consistency checks when the caller supplies a
checksum file or SBOM for the selected asset. For example, a checksum file entry for the selected
asset may be required to match `subject[0].digest.sha256`, or an SBOM package/component hash for the
selected asset may be required to match the same bytes. Such checks are consistency gates; they do
not make the checksum file or SBOM bytes part of the selected asset subject digest.

Cross-asset completeness checks remain orchestration-layer responsibilities. A higher-level release
workflow may verify that every asset listed in a checksum manifest was uploaded and attested, that
every binary has an SBOM, or that all SBOMs and checksum files are present before publishing an
immutable release. That orchestration must compose one-asset profile invocations rather than
changing the low-level profile's subject digest semantics.

### Consequences

- Good, because the SLSA subject digest has one clear meaning: the digest of the selected release
  asset bytes.
- Good, because checksum manifests and SBOMs cannot accidentally become substitutes for
  artifact-bound build provenance.
- Good, because checksum files, SBOMs, provenance sidecars, and detached signatures can still be
  independently published and attested as release assets.
- Good, because consumer verification remains a direct comparison between downloaded asset bytes and
  `subject[0].digest.sha256`.
- Good, because whole-release completeness stays in the orchestration layer selected by ADR 0039 and
  ADR 0043.
- Neutral, because optional checksum or SBOM consistency checks may still be specified later without
  changing the primary subject digest.
- Bad, because the low-level profile alone cannot prove that a complete release has a correct
  checksum manifest and SBOM set.
- Bad, because callers that want whole-release guarantees must use additional orchestration or
  release manifest policy.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification,
workflow implementation, tests, and verifier guidance define:

- `subject[0].digest.sha256` as the digest of the selected release asset bytes only;
- no checksum file, SBOM file, provenance sidecar, detached signature, release manifest, or other
  supporting-file digest in the selected release asset subject digest map;
- separate one-asset profile invocations when checksum files, SBOMs, provenance sidecars, detached
  signatures, or similar files are themselves published GitHub Release assets;
- optional consistency checks, if supported, that compare checksum-file entries or SBOM component
  hashes with the selected asset subject digest without changing subject semantics;
- byproduct or orchestration metadata rules for non-primary checksum files and SBOMs;
- documentation that whole-release checksum/SBOM completeness belongs to an orchestration layer or
  future release manifest, not the low-level one-asset profile.

Implementation review should verify that no production path can sign or upload provenance where the
selected release asset subject digest describes a checksum file, SBOM, sidecar, signature, manifest,
or any bytes other than the uploaded release asset.

## Pros and Cons of the Options

### Keep checksum files and SBOMs out of the release asset subject digest

The selected release asset is the only SLSA subject. Checksum files and SBOMs are either independent
release assets, byproducts, consistency inputs, or orchestration metadata.

- Good, because it preserves one subject and one primary digest per profile run.
- Good, because it matches SLSA's artifact-bound provenance model.
- Good, because it composes cleanly with releases that publish many binaries, checksum files, SBOMs,
  provenance sidecars, and signatures.
- Good, because SBOM attestation can still describe the selected asset using an SBOM predicate
  rather than changing the build provenance subject.
- Neutral, because the architecture still needs to define optional consistency checks and byproduct
  fields.
- Bad, because whole-release completeness requires another layer.

### Add checksum file and SBOM digests to the primary subject digest map

The selected release asset subject contains additional digest-map keys for related checksum files or
SBOM files.

- Good, because one subject entry appears to carry all related digest handles.
- Good, because producer-side checks could compare related files in one place.
- Bad, because a SLSA subject digest map is naturally an algorithm-keyed map for the same subject
  bytes, not a collection of digests for different files.
- Bad, because custom digest keys for other files can confuse generic in-toto and SLSA tooling.
- Bad, because it blurs whether the subject is the release asset, the checksum file, the SBOM, or a
  bundle of all three.

### Include the release asset, checksum file, and SBOM as multiple subjects

One provenance statement contains separate subject entries for the primary release asset, checksums,
SBOMs, and related files.

- Good, because in-toto and `actions/attest` can represent multiple named subjects.
- Good, because each listed file has its own digest without overloading one digest map.
- Bad, because ADR 0039 selected exactly one production release asset subject for the initial
  profile.
- Bad, because partial upload, duplicate names, retries, and verification failures become
  multi-asset problems.
- Bad, because checksum files and SBOMs may describe assets outside the current profile run.

### Use the checksum manifest as the primary SLSA subject

The SLSA subject is the checksum file, and the checksum file lists the release asset digest.

- Good, because checksum manifests are familiar release verification artifacts.
- Good, because one manifest can reference many release assets.
- Bad, because the SLSA provenance describes the manifest bytes rather than the selected release
  asset bytes.
- Bad, because ADR 0039 rejected checksum-manifest-centered verification for the low-level profile.
- Bad, because manifest completeness and synchronization with uploaded assets are orchestration
  concerns.

### Use the SBOM file as the primary SLSA subject

The SLSA subject is the SBOM file, and the SBOM package or component hashes describe the release
asset.

- Good, because SPDX and CycloneDX can record package or component checksums.
- Good, because SBOM tooling can consume dependency and component metadata directly.
- Bad, because the SLSA build provenance subject becomes metadata rather than the built artifact.
- Bad, because GitHub `actions/attest` SBOM mode keeps the artifact as the subject and the SBOM as
  the predicate, not the other way around.
- Bad, because SBOM format choices and optional availability would affect the primary build
  provenance verification path.

## More Information

This decision follows ADR 0038, ADR 0039, ADR 0043, ADR 0044, and ADR 0045. It decides only the
relationship between the selected release asset subject digest and checksum or SBOM files. It does
not decide the checksum manifest schema, SBOM format, SBOM predicate policy, provenance sidecar
naming, detached signature policy, release manifest schema, or future whole-release orchestration
workflow.

Reference points considered:

- SLSA v1.2 Distributing Provenance says attestations should be bound to artifacts rather than
  releases and that provenance should accompany the artifact at publish time.
- SLSA v1.2 Build Provenance represents build outputs through the top-level in-toto Statement
  `subject` list and uses `byproducts` for additional artifacts generated during the build that are
  not considered build outputs.
- SLSA v1.2 Verifying Artifacts recommends verifying that the Statement subject matches the digest
  of the artifact being verified before checking `predicateType`, trusted builder identity,
  `buildType`, and `externalParameters`.
- in-toto Statement v1 requires subject elements to have digest values and says subject artifacts
  are matched purely by digest.
- in-toto ResourceDescriptor says a digest-bearing descriptor is assumed to refer to an immutable
  resource or artifact, and the predicate type should document which field consumers match.
- GitHub `actions/attest` SBOM mode takes both `subject-path` for the artifact and `sbom-path` for
  the SBOM predicate. Its checksum-file subject mode enumerates subjects from `shasum`-style lines,
  but those lines identify attestation subjects rather than modifying one subject's digest map.
- The SLSA GitHub Generator generic workflow accepts `sha256sum`-style subject inputs, and common
  release tooling such as GoReleaser and JReleaser can generate checksum files for release
  artifacts.
- SPDX and CycloneDX SBOM formats can carry package or component hashes, but those hashes are SBOM
  metadata about described artifacts rather than replacements for SLSA build provenance subject
  digests.
