---
parent: Decisions
nav_order: 45
status: accepted
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Use the GitHub Release Asset Name as the SLSA Subject Name

## Context and Problem Statement

ADR 0038 selected Windlass-generated SLSA provenance for GitHub Release assets, with
`actions/attest` acting only as the initial Sigstore signing and storage adapter. ADR 0039 scoped
the initial production profile to exactly one GitHub Release asset per profile run. ADR 0043
selected an upload-only lifecycle for one explicitly named asset on an existing GitHub Release, and
ADR 0044 selected a three-job graph where `release-upload` verifies the signed provenance bundle
before mutating the release.

The next subject and digest semantics decision is the `subject[0].name` value in the in-toto
Statement. The profile needs a stable name that identifies the one asset the user expects to upload,
without confusing local build paths, GitHub Release URLs, GitHub API IDs, or whole-release identity
with the artifact-bound SLSA subject.

What should the initial GitHub Release asset profile use as the SLSA subject name for the one
release asset artifact?

## Decision Drivers

- Keep the profile's SLSA subject artifact-bound and aligned with ADR 0039's one-asset scope.
- Match the GitHub Release asset identity that callers and consumers see on the release page.
- Avoid making workspace-local paths part of the verifier contract when the asset may be renamed for
  upload.
- Avoid making mutable or provider-specific URLs the primary subject identifier.
- Avoid using GitHub API asset IDs that do not exist until after the provenance is signed.
- Keep digest verification as the cryptographic identity check, with `name` as the policy-relevant
  asset identifier.
- Align with `actions/attest`, in-toto, and SLSA GitHub Generator conventions for named artifact
  subjects with digest maps.

## Considered Options

- Use the final GitHub Release asset name as `subject[0].name`.
- Use the repository-relative local artifact path as `subject[0].name`.
- Use the GitHub Release browser or download URL as `subject[0].name`.
- Use the GitHub Release asset API ID or GraphQL node ID as `subject[0].name`.
- Use a package-url-like GitHub Release asset identifier as `subject[0].name`.

## Decision Outcome

Chosen option: "Use the final GitHub Release asset name as `subject[0].name`", because it matches
the one explicitly named upload target selected by ADR 0039 and ADR 0043 while keeping digest
matching as the cryptographic artifact identity.

The initial GitHub Release asset profile should emit exactly one production SLSA subject for the
release asset:

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

The subject name is the final GitHub Release asset name requested by the caller and accepted by the
profile for upload. It is not the local workspace path, repository-relative build output path,
workflow artifact name, GitHub Release URL, GitHub API asset ID, provenance sidecar name, or package
URL.

The subject name should be a single release asset name, not a path. The architecture specification
should reject subject asset names that contain path separators, empty path segments, URL syntax,
query strings, fragments, control characters, or values that normalize differently across supported
platforms. The same normalized asset name should be used for duplicate-asset checks, upload, public
workflow output handles, and subject-name verification.

The subject digest should be the SHA-256 digest of the exact bytes uploaded as the GitHub Release
asset. It should be encoded as lowercase hexadecimal in `subject[0].digest.sha256`, without a
`sha256:` prefix. The implementation may expose or use prefixed digest strings at tool boundaries
where required, such as `actions/attest` `subject-digest`, but the in-toto Statement digest map
should use the algorithm key `sha256` and the raw lowercase hex value.

Release repository, release tag, release URL, release asset API ID, draft or prerelease state,
download URL, upload metadata, and GitHub attestation storage identifiers should not be folded into
`subject[0].name`. Those values should be represented in the profile's `externalParameters`,
`resolvedDependencies`, byproducts, upload verification policy, workflow outputs, or release
metadata fields as defined by later architecture specifications.

Producer-side verification in the `release-upload` job should require the signed provenance subject
name to equal the expected release asset name and the recalculated release asset SHA-256 digest to
equal `subject[0].digest.sha256` before upload. Consumer-side verification should similarly check
the downloaded asset bytes against the subject digest and check the subject name against the
expected release asset name when policy cares about the release asset slot.

### Consequences

- Good, because the SLSA subject name matches the GitHub Release asset identity that users upload
  and consumers download.
- Good, because local workspace layout does not become part of the long-term verifier contract.
- Good, because the duplicate asset-name check selected by ADR 0043 and the subject-name check use
  the same asset slot identifier.
- Good, because the digest remains the cryptographic artifact identity while the name remains the
  stable policy label.
- Good, because the shape aligns with `sha256sum`-style SLSA GitHub Generator subjects and
  `actions/attest` named artifact subjects.
- Neutral, because release repository, tag, URL, and API ID still need explicit representation in
  other provenance fields or verification policy.
- Bad, because the same asset name can exist in different repositories, releases, or tags, so
  subject name alone is not globally unique.
- Bad, because callers must choose final release asset names before provenance signing.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification,
workflow implementation, tests, and verifier guidance define:

- exactly one SLSA subject for the production one-asset release profile path;
- `subject[0].name` as the final GitHub Release asset name requested for upload;
- rejection of local paths, repository-relative paths, URLs, GitHub API IDs, package URLs, and
  provenance sidecar names as subject names;
- normalization and validation rules for release asset names;
- `subject[0].digest.sha256` as the lowercase hexadecimal SHA-256 digest of the exact uploaded asset
  bytes, without a `sha256:` prefix;
- tool-boundary conversion rules for any APIs that require prefixed digest strings;
- producer-side verification that the signed subject name and digest match the expected asset before
  release upload;
- consumer-side verifier guidance that checks both subject digest and expected release asset name;
- separate provenance or verification-policy fields for repository, release tag, release state,
  release URLs, asset API IDs, and attestation storage identifiers.

Implementation review should verify that production provenance cannot be signed or uploaded when the
subject name describes a different release asset slot than the upload target, or when the subject
digest describes bytes different from the uploaded asset.

## Pros and Cons of the Options

### Use the final GitHub Release asset name

The subject name is the basename-style GitHub Release asset name that the profile will upload, such
as `slsa-builder_0.1.0_linux_amd64.tar.gz`.

- Good, because it matches the user-visible release asset slot.
- Good, because it aligns subject verification with duplicate asset-name checks.
- Good, because it stays stable when local workspace paths or workflow artifact names change.
- Good, because it follows the filename-style subject convention used by checksum files,
  `actions/attest` subject checksums, and the SLSA GitHub Generator generic workflow.
- Neutral, because globally unique release identity must still come from repository and tag fields.
- Bad, because the asset name must be known before signing and cannot be inferred only from the
  local file path.

### Use the repository-relative local artifact path

The subject name is a path such as `dist/slsa-builder_0.1.0_linux_amd64.tar.gz`.

- Good, because it can directly identify the file generated by the build job.
- Good, because it resembles `actions/attest` `subject-path` usage for local binaries.
- Bad, because it binds verifier policy to workspace layout rather than release asset identity.
- Bad, because upload-time renaming would make the subject name differ from the release asset name.
- Bad, because path normalization differs across platforms and shell tools.

### Use the GitHub Release browser or download URL

The subject name is a URL such as
`https://github.com/windlasstech/slsa-builder/releases/download/v0.1.0/foo.tar.gz`.

- Good, because it can be globally recognizable to humans and scripts.
- Good, because it points at the consumer download surface after release publication.
- Bad, because URL shape, redirects, repository renames, and CDN behavior are provider-specific and
  mutable.
- Bad, because draft-release upload flows may not have a final consumer-facing URL at signing time.
- Bad, because in-toto ResourceDescriptor has `uri` and `downloadLocation` fields for URI semantics;
  overloading `name` with a URL weakens the distinction.

### Use the GitHub Release asset API ID or node ID

The subject name is a GitHub REST asset ID, GraphQL node ID, or API URL for the uploaded release
asset.

- Good, because GitHub can identify the uploaded asset object precisely after upload.
- Good, because it avoids ambiguity among same-named files in different releases.
- Bad, because the asset ID does not exist until after upload, while ADR 0044 requires provenance to
  be signed and verified before release mutation.
- Bad, because it would make the subject name GitHub-API-specific and less meaningful to consumers.
- Bad, because it conflicts with the failure-closed no-overwrite upload path: the expected asset
  slot must be known before creating the asset object.

### Use a package-url-like GitHub Release asset identifier

The subject name is a custom package URL or URI-like identifier containing repository, tag, and
asset name.

- Good, because it could make the subject name globally scoped without relying on GitHub download
  URL stability.
- Good, because it can be useful for future package-ecosystem-style verifier indexing.
- Bad, because GitHub Release asset package URL conventions are not as established as npm package or
  OCI image naming conventions.
- Bad, because it duplicates repository and tag fields that should be verified through the profile's
  `externalParameters` and source or release metadata.
- Bad, because it is less compatible with `actions/attest` and SLSA GitHub Generator filename-style
  subject conventions.

## More Information

This decision follows ADR 0038, ADR 0039, ADR 0043, and ADR 0044. It decides only the initial
production GitHub Release asset subject name and SHA-256 subject digest encoding. It does not decide
the complete release asset `externalParameters` schema, release URL outputs, GitHub asset API ID
outputs, `downloadLocation` usage, provenance sidecar naming, checksum file policy, SBOM policy, or
future multi-asset orchestration model.

Reference points considered:

- in-toto Statement v1 requires each subject to have a digest, treats subjects as immutable, and
  says subject artifacts are matched purely by digest. The `name` field can distinguish artifacts
  within the subject and may be used by policy when meaningful.
- in-toto ResourceDescriptor says `name` should be a stable machine-readable identifier such as a
  filename, while `uri` and `downloadLocation` carry URI semantics.
- SLSA v1.2 Build Provenance represents build outputs as the top-level in-toto Statement `subject`
  list and uses digest maps keyed by algorithm names such as `sha256`.
- SLSA v1.2 Verifying Artifacts recommends verifying that the Statement subject matches the digest
  of the artifact being verified before checking `predicateType`, trusted builder identity,
  `buildType`, and `externalParameters`.
- GitHub `actions/attest` binds a named artifact and digest to a predicate. Its `subject-digest`
  input requires the form `sha256:HEX_DIGEST` and a `subject-name`, while checksum-file subjects use
  `shasum`-style lines containing a hex digest and filename.
- The SLSA GitHub Generator generic workflow accepts subjects formatted like `sha256sum` output,
  where the decoded input is a SHA-256 hash followed by an artifact name, and its examples use
  artifact filenames as subject names for release artifacts.
