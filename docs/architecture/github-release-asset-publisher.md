# GitHub Release Asset Publisher Contract

This document defines the GitHub Release asset publisher profile as a verified distributor of
ecosystem-produced artifacts, not a source-to-artifact builder.

- Source ADRs: [0039](../decisions/0039-scope-release-asset-profile-to-one-asset-per-run.md),
  [0043](../decisions/0043-upload-release-assets-to-existing-github-releases.md),
  [0045](../decisions/0045-use-release-asset-name-as-slsa-subject-name.md),
  [0046](../decisions/0046-keep-checksums-and-sboms-out-of-release-asset-subject-digest.md),
  [0048](../decisions/0048-make-linked-artifacts-storage-records-explicit-opt-in-for-release-assets.md),
  [0049](../decisions/0049-separate-artifact-production-from-github-release-asset-publication.md),
  [0050](../decisions/0050-define-producer-to-publisher-handoff-contract.md),
  [0051](../decisions/0051-distribute-producer-provenance-with-release-assets.md),
  [0052](../decisions/0052-compose-npm-package-tarball-producer-with-release-asset-publisher.md),
  [0057](../decisions/0057-provide-composed-public-npm-release-asset-workflow.md),
  [0058](../decisions/0058-define-github-release-asset-publisher-authority-boundary.md),
  [0059](../decisions/0059-define-public-npm-release-composed-workflow-interface.md),
  [0060](../decisions/0060-unify-npm-profile-public-entrypoint-with-release-asset-mode.md),
  [0062](../decisions/0062-intersect-trusted-producer-policies.md),
  [0064](../decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md)
- Related specs: [Core profile contract](core-profile-contract.md),
  [Identity and build types](identity-and-buildtypes.md),
  [SLSA provenance v1](slsa-provenance-v1.md),
  [npm-to-release-asset composition](npm-to-release-asset-composition.md),
  [Verification policy and fixtures](verification-policy-and-fixtures.md)

## Scope and non-goals

**In scope:**

- Publisher role as verified distributor.
- Producer-to-publisher handoff contract.
- Mandatory producer provenance verification.
- Release asset upload behavior.
- Provenance sidecar distribution.
- Native producer provenance locators.
- Linked artifact storage opt-in.

**Out of scope:**

- Source-to-artifact build semantics (producer profile specs).
- Publisher-owned SLSA provenance or custom publication predicates in the default path.
- Generic raw file upload without producer provenance.

## Publisher is not a builder

The GitHub Release asset publisher does not build artifacts from source and does not generate
source-to-artifact SLSA provenance. Its responsibilities are:

1. Receive an artifact from a producer profile through a trusted handoff.
2. Verify the artifact digest against the expected digest.
3. Verify the upstream producer provenance.
4. Upload the exact verified bytes to an existing GitHub Release.
5. Redistribute the unchanged producer provenance bundle as a release asset sidecar.
6. Expose native producer provenance locators when available.

## No publisher-owned source-to-artifact `buildType`

The default production publisher path does not define a `buildType` URI for the upload operation.
Consumers must verify the producer profile's `buildType` carried in the producer provenance.

## Workflow entrypoint

The production workflow entrypoint is:

```text
.github/workflows/github-release-asset-publish.yml
```

The release manifest must record the exact workflow path and SHA for this entrypoint.

### `workflow_call` contract

The standalone publisher exposes the producer-neutral handoff fields as its public
`workflow_call.inputs`. The public contract is intentionally low-level: it is an advanced
composition primitive, not the recommended npm caller surface selected by the public npm profile.

| Input                               | Type   | Required | Default | Validation summary                                            |
| ----------------------------------- | ------ | -------- | ------- | ------------------------------------------------------------- |
| `primary-artifact-name`             | string | yes      | none    | Same-run artifact containing exactly one primary payload.     |
| `expected-sha256`                   | string | yes      | none    | 64 lowercase hexadecimal SHA-256 of the primary payload.      |
| `final-asset-name`                  | string | yes      | none    | Safe basename that will be uploaded as the release asset.     |
| `release-tag`                       | string | yes      | none    | Full `refs/tags/<tag-name>` ref for an existing release.      |
| `producer-provenance-artifact-name` | string | yes      | none    | Same-run artifact containing exactly one signed bundle.       |
| `producer-provenance-sha256`        | string | yes      | none    | 64 lowercase hexadecimal SHA-256 of the signed bundle.        |
| `trusted-builder-id`                | string | yes      | none    | Expected producer `builder.id`.                               |
| `trusted-build-type`                | string | yes      | none    | Expected producer `buildType`.                                |
| `expected-subject-name`             | string | yes      | none    | Expected producer `subject[0].name`.                          |
| `expected-subject-sha256`           | string | yes      | none    | Expected producer `subject[0].digest.sha256`.                 |
| `source-repository`                 | string | yes      | none    | Canonical producer source repository URL.                     |
| `source-revision`                   | string | yes      | none    | Immutable source revision; GitHub Git sources use SHA-1.      |
| `native-provenance-locators`        | string | no       | empty   | UTF-8 JSON array; empty string means absent.                  |
| `linked-artifact-settings`          | string | no       | empty   | UTF-8 JSON object; empty string means `{ "enabled": false }`. |

The workflow must not declare inputs for target repository owner, target repository name, upload
URL, release URL, custom GitHub token, release creation, overwrite mode, arbitrary local file paths,
cross-run artifact IDs, or bypassing producer provenance verification. If a caller supplies such a
value through an unsupported input name, workflow dispatch must fail schema validation or the
publisher must reject the configuration before release mutation.

The standalone publisher accepts no secrets in the initial production contract. Release mutation
uses the caller-scoped `GITHUB_TOKEN` permissions granted to the calling job and reduced by the
called workflow. A future custom-token or cross-repository publication mode requires a new ADR
because it changes the mutation authority boundary.

### Permissions matrix

The publisher workflow must keep the default top-level permission set read-only and elevate only the
jobs that need mutation authority.

| Job class                                  | Required permissions                         | Forbidden permissions                                                                             |
| ------------------------------------------ | -------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| Handoff download, digest, and verification | `contents: read`                             | `contents: write`, `id-token: write`, `attestations: write`, `artifact-metadata: write`           |
| Release upload                             | `contents: write`                            | `id-token: write`, `attestations: write`, `artifact-metadata: write`, package publish permissions |
| Linked artifact metadata, when enabled     | `artifact-metadata: write`, `contents: read` | `contents: write`, `id-token: write`, `attestations: write`, release upload authority             |

The caller job must grant only the permissions required by the selected path. A caller that enables
linked artifact metadata must grant `artifact-metadata: write`; a caller that disables it must not
cause the metadata job to run or request that permission. Missing required permissions and excessive
job permissions are distinct pre-upload failures.

## One-primary-asset unit

Each publisher run uploads exactly one primary release asset. If a project needs multiple release
assets, it must invoke the publisher once per asset.

## Existing release requirement

The publisher uploads to an existing GitHub Release identified by an existing Git tag. The publisher
does not create the release or the tag. If the tag or release does not exist, the publisher fails
before any upload.

## Draft and prerelease behavior

The publisher may upload to a draft or prerelease target only if the release already exists. The
publisher does not change the draft or prerelease status.

## Duplicate asset behavior

Before uploading anything, the publisher must check the target release for both the primary release
asset name and the deterministic sidecar name `<asset-name>.intoto.jsonl`. If either name already
exists under the target release, the publisher must fail without uploading the primary asset or the
sidecar. The publisher must not overwrite, replace, delete, or clobber an existing asset.

If the preflight duplicate check passes but a later GitHub API race or upload failure prevents the
sidecar from being uploaded after the primary asset succeeds, the run enters the partial failure
state described below. That partial state is for post-preflight API or transport failures, not for
known duplicate names.

## Producer-to-publisher handoff contract

The publisher accepts a fixed, profile-owned handoff contract. The contract must include the
following semantic fields:

| Field                               | Required | Description                                                             |
| ----------------------------------- | -------- | ----------------------------------------------------------------------- |
| `primary-artifact-name`             | yes      | Same-run GitHub Actions artifact that contains the primary asset bytes. |
| `expected-sha256`                   | yes      | Expected SHA-256 of the artifact bytes, lowercase hex.                  |
| `final-asset-name`                  | yes      | Final GitHub Release asset name.                                        |
| `release-tag`                       | yes      | Existing full Git tag ref and target release.                           |
| `producer-provenance-artifact-name` | yes      | Same-run artifact that contains the producer SLSA provenance bundle.    |
| `producer-provenance-sha256`        | yes      | Expected SHA-256 of the provenance bundle bytes, lowercase hex.         |
| `trusted-builder-id`                | yes      | Expected upstream producer `builder.id`.                                |
| `trusted-build-type`                | yes      | Expected upstream producer `buildType`.                                 |
| `expected-subject-name`             | yes      | Expected upstream subject name under the selected producer policy.      |
| `expected-subject-sha256`           | yes      | Expected upstream subject SHA-256, lowercase hex.                       |
| `source-repository`                 | yes      | Source repository for producer policy.                                  |
| `source-revision`                   | yes      | Source revision for producer policy.                                    |
| `native-provenance-locators`        | no       | Producer-native provenance locators.                                    |
| `linked-artifact-settings`          | no       | Linked artifact storage opt-in settings.                                |

The field names above are the public semantic handoff names for the publisher contract. The
publisher constructs two core same-run artifact handoff objects from them:

| Core handoff field  | Primary asset handoff value | Producer provenance handoff value                  |
| ------------------- | --------------------------- | -------------------------------------------------- |
| `transport`         | `github-actions-artifact`   | `github-actions-artifact`                          |
| `artifact_name`     | `primary-artifact-name`     | `producer-provenance-artifact-name`                |
| `payload_file_name` | `final-asset-name`          | Basename of the single provenance bundle artifact. |
| `payload_kind`      | `primary-artifact`          | `provenance-bundle`                                |
| `digest.algorithm`  | `sha256`                    | `sha256`                                           |
| `digest.value`      | `expected-sha256`           | `producer-provenance-sha256`                       |

The provenance bundle `payload_file_name` is transport metadata only; the public release sidecar
name is still derived from `final-asset-name` as `<final-asset-name>.intoto.jsonl`.

All required handoff string fields must be non-empty after trimming ASCII whitespace. SHA-256 fields
must be 64-character lowercase hexadecimal strings. `release-tag` must be a full Git tag ref in the
form `refs/tags/<tag-name>` and must identify the target existing GitHub Release. A short tag name
such as `v1.2.3`, a branch ref, a pull request ref, an empty tag name, or a ref containing path
traversal or ASCII control characters must be rejected before upload. `final-asset-name` must
satisfy the release asset name rules below. `expected-subject-name` must satisfy the selected
producer policy; for the initial npm composition it is an npm Package URL and not the final release
asset name. `source-repository` must be a canonical HTTPS repository URL, and `source-revision` must
be the full immutable source revision expected by the producer policy; for GitHub-hosted Git sources
this is a 40-character lowercase Git commit SHA. For the initial npm composition,
`source-repository` must use the canonical GitHub repository URL rules defined by the
[JS/TS npm provenance and publish spec](js-ts-npm-provenance-publish.md). The publisher must reject
a `source-repository` value that cannot be canonicalized by those rules or that does not exactly
match the canonical `externalParameters.source.repository` value in the verified producer
provenance.

`release-tag` is also the expected producer release ref unless an ADR-backed producer policy defines
a different release-binding source for a future producer profile. For the initial npm composition,
the publisher must verify that `release-tag`, producer `externalParameters.release.ref`, and
producer `externalParameters.source.ref` are the same full `refs/tags/<tag-name>` ref and that
producer `externalParameters.release.version_tag` reconstructs that same ref. A missing producer
release ref, short tag, branch ref, pull request ref, or mismatch with `release-tag` fails before
upload. Future producer profiles whose signed provenance does not use those npm `externalParameters`
fields must define, before composition, which signed field or digest-verified handoff field binds
the verified producer artifact to the target release ref.

Complex handoff fields in the public workflow contract are passed as UTF-8 JSON strings. The
publisher must parse `native-provenance-locators` as a JSON array and `linked-artifact-settings` as
a JSON object before validation. Empty or omitted optional inputs are treated as absent. Invalid
JSON, duplicate object member names, top-level JSON values of the wrong kind, or fields not allowed
by the closed schemas below must be rejected before upload. The parsed JSON values are validation
inputs; their original string formatting is not trusted and is not used for digest calculation.

For the initial npm composition, `source-repository` and `source-revision` are always required
because the publisher verifies them against the npm producer provenance
`externalParameters.source.repository` and `externalParameters.source.revision`. A missing, short,
branch-like, tag-like, or mismatched source identity must fail before upload.

### Final release asset name rules

`final-asset-name` is the basename that will be uploaded to the target GitHub Release. The publisher
must validate it before retrieving or uploading release assets and must verify that the selected
producer policy binds it to the producer artifact bytes.

The value is invalid when any of the following are true:

- It is empty or becomes empty after trimming ASCII whitespace.
- It has leading or trailing ASCII whitespace.
- It contains `/`, `\\`, NUL, or any ASCII control character.
- It is exactly `.` or `..`.
- It contains a path traversal segment when interpreted as slash- or backslash-separated text.
- It differs from the basename of the single payload file inside the `primary-artifact-name` handoff
  artifact.
- The derived sidecar name `<final-asset-name>.intoto.jsonl` would violate the same character and
  whitespace rules.

The publisher must not rely on GitHub's API acceptance rules as the primary validation policy. A
name that passes GitHub API validation but violates the Windlass rules above must be rejected before
upload. The duplicate preflight check must evaluate both `final-asset-name` and the derived sidecar
name as distinct target release asset names.

### Artifact transport

The initial production publisher accepts only same-run GitHub Actions artifact names for primary
asset transport. `primary-artifact-name` must name an artifact produced earlier in the same workflow
run, and that artifact must contain exactly one file whose basename equals `final-asset-name`.

The publisher must reject file paths, artifact IDs, URLs, job outputs containing raw bytes, and
artifacts from another workflow run in the initial production contract. Future transports require a
new specification section or ADR when they change the trust boundary.

The artifact name is a transport handle only. The publisher must not treat the artifact name as
proof of trust. It must retrieve the bytes, compute SHA-256, and compare the result with
`expected-sha256` before upload.

### Producer provenance bundle transport

The initial production publisher accepts only a same-run GitHub Actions artifact containing the
exact producer provenance bundle bytes. `producer-provenance-artifact-name` must name an artifact
produced earlier in the same workflow run, and that artifact must contain exactly one signed
Sigstore bundle file.

`producer-provenance-sha256` is mandatory. The publisher must retrieve the bundle bytes, compute
SHA-256, compare it with `producer-provenance-sha256`, verify the bundle, and then upload those
exact bytes as the sidecar if verification succeeds.

Native provenance locators are optional metadata only. A locator-only provenance input must be
rejected because the publisher cannot redistribute an unchanged sidecar without bundle bytes.

## Producer provenance verification

Before publication, the publisher must verify the upstream producer provenance:

1. The attestation signature is valid.
2. The signer identity matches the producer signer identity defined by the producer profile; for the
   initial npm composition, this is the JS/TS npm producer signer identity.
3. The `predicateType` is `https://slsa.dev/provenance/v1`.
4. The `builder.id` matches the trusted producer policy.
5. The `buildType` matches the trusted producer policy.
6. The `subject[0].digest.sha256` matches `expected-subject-sha256`, `expected-sha256`, and the
   bytes to upload.
7. The `subject[0].name` exactly matches `expected-subject-name` under the selected producer policy.
8. The final release asset name is authorized by producer policy and handoff fields. For the initial
   npm composition, this means the asset name matches the pack-produced tarball filename recorded in
   producer provenance and the same-run handoff, while `subject[0].name` remains the npm Package
   URL.
9. Source repository, source revision, release ref, and other producer `externalParameters` match
   the trusted producer policy. For the initial npm composition, `externalParameters.source.ref`,
   `externalParameters.release.ref`, and `release-tag` must be identical full tag refs.
10. No unexpected `externalParameters` are present when the policy requires strict matching.

When the publisher has both an explicit producer policy from the handoff or profile configuration
and a verified signed release manifest policy, the effective producer policy is the intersection of
the fields each source explicitly constrains. For schema version `1`, the release manifest
constrains Windlass release identity, producer workflow path/SHA, producer `builder.id`, producer
`buildType`, and publisher workflow path/SHA/role; it does not constrain caller-specific source,
producer release ref, subject, digest, final asset name, or strict `externalParameters` fields.
Those fields must still be constrained by explicit producer policy, profile policy, signed producer
provenance, or digest-verified handoff fields. The publisher must fail before upload when any source
that constrains an observed field does not allow it, and it must not let a signed release manifest
relax explicit producer policy, let explicit producer policy bypass an authenticated manifest
constraint, or use either-source-allowed or last-writer-wins behavior.

## Final release asset and producer subject binding

The final release asset name must be bound to the upstream producer provenance by the selected
producer policy. Producer profiles whose subject name is their release asset basename can satisfy
this by requiring `final-asset-name` to equal `subject[0].name`. The initial npm composition cannot:
ADR 0064 requires the npm producer subject to be the package Package URL. For npm producer
provenance, the publisher must therefore verify that `subject[0].name` matches the expected npm
Package URL and separately verify that `final-asset-name` matches the pack-produced tarball filename
recorded by the producer policy and handoff fields.

If the selected producer policy cannot bind the final release asset name to the verified producer
artifact bytes and provenance subject, the publisher must fail before upload.

## Digest semantics

- The primary release asset digest is SHA-256, lowercase hex.
- The publisher may also record a profile-specific digest, such as `sha512`, for handoff or sidecar
  purposes.
- The primary asset's SLSA subject digest must not include checksum files, SBOMs, or the provenance
  sidecar.

## Provenance sidecar

The publisher redistributes the exact verified producer provenance bundle as a GitHub Release asset
sidecar. The sidecar must not be altered, re-signed, wrapped, or regenerated.

The sidecar bytes are the exact signed bundle bytes received through the verified producer
provenance handoff. The `.intoto.jsonl` suffix is only the release asset naming convention; the
publisher must not extract the Statement, reserialize the bundle, rewrite DSSE or Sigstore metadata,
or substitute a native attestation locator for the bundle file. See the
[SLSA provenance v1 signed bundle file format](slsa-provenance-v1.md#signed-bundle-file-format).

### Sidecar name

The sidecar name is deterministic and derived from the primary asset name by appending
`.intoto.jsonl`:

```text
<asset-name>.intoto.jsonl
```

For example, `my-package-1.2.3.tar.gz.intoto.jsonl`. This convention is fixed and must be recorded
in the profile and verification docs.

## Native producer provenance locators

When the producer profile provides native provenance locators, such as GitHub artifact attestation
URLs, the publisher must expose them in its outputs. Native locators are useful for online
verification but do not replace the sidecar as the release-page distribution copy.

Native provenance locators are diagnostic and discovery metadata only. They are not trusted handoff
inputs, they do not prove artifact integrity, and their presence or absence must not change whether
the publisher trusts the primary asset. The publisher trust decision depends on the retrieved
producer bundle bytes, producer bundle signature, producer signer identity, producer provenance
contents, and primary asset digest checks. A valid native locator may help users find
producer-native attestation storage after those checks succeed, but it must not bypass them.

`native-provenance-locators` must be an array of objects with this shape:

```json
[
  {
    "type": "github-artifact-attestation",
    "url": "https://github.com/example/project/attestations/123",
    "digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }
]
```

Field rules:

- `type` must be `github-artifact-attestation` for the initial production contract. Future locator
  types require profile documentation before use.
- `url` must be an absolute `https:` URL whose host is exactly `github.com` after lowercase
  normalization. It must not contain embedded credentials, a query, a fragment, a non-default port,
  backslashes, percent-encoded path separators, empty path segments, `.` or `..` segments, or ASCII
  control characters.
- For the initial npm composition, the URL path must be under the canonical source repository path
  recorded in producer provenance, followed by an attestation locator path selected by GitHub. A URL
  under a different repository is malformed for this locator type.
- `digest` is optional. When present, it must be `sha256:<64 lowercase hex characters>` and must
  equal the SHA-256 digest of the exact producer signed bundle bytes that the publisher
  redistributes as the sidecar. It is not a digest of a separately fetched native service payload.
- Unknown fields must be rejected by strict publisher policy.

The publisher must reject a locator object with an unsupported `type`, non-HTTPS URL, embedded
credentials, malformed repository path, malformed digest, digest that differs from the sidecar
bundle digest, or unknown fields. A valid locator still remains metadata only; the publisher must
also receive and verify `producer-provenance-artifact-name` and `producer-provenance-sha256`. A
missing locator is not a provenance verification failure.

When supplied as a public workflow input, `native-provenance-locators` must be encoded as a UTF-8
JSON string whose parsed value is the array shape above. For example:

```json
[
  {
    "type": "github-artifact-attestation",
    "url": "https://github.com/example/project/attestations/123"
  }
]
```

## Linked artifact storage opt-in

Linked artifact storage records are explicit opt-in. When enabled, the publisher records metadata
about the primary asset and its related artifacts. The linked artifact metadata must not be inserted
into the primary asset's SLSA subject digest.

`linked-artifact-settings` is optional. When absent, it is equivalent to:

```json
{
  "enabled": false
}
```

When present, it must have this shape:

```json
{
  "enabled": true,
  "version": "1.2.3",
  "repository": "windlasstech/example",
  "registry_url": "https://github.com/windlasstech/example/releases/download/v1.2.3/"
}
```

Field rules:

- `enabled` is required and must be boolean.
- When `enabled` is `false`, no other fields are allowed, the workflow must not request
  `artifact-metadata: write`, and the publisher must not call the artifact metadata REST API.
- When `enabled` is `true`, the metadata job must have `artifact-metadata: write` and must not have
  `id-token: write`, `attestations: write`, signing credentials, or GitHub Release mutation
  authority.
- `version` must be the release version derived from the `release-tag` full ref by removing
  `refs/tags/` and then removing the leading `v` from the tag name.
- `repository` must identify the same GitHub repository that owns the target release asset storage
  surface. The publisher must compare it with the caller/target repository after canonicalizing both
  values to `owner/repo` with lowercase host semantics and no URL syntax, owner or repository path
  traversal, empty components, `.`, `..`, or extra path segments.
- `registry_url` must be the GitHub Release download URL prefix for the target release in the same
  repository and tag. It must exactly equal
  `https://github.com/<owner>/<repo>/releases/download/<tag-name>/` after deriving `<owner>/<repo>`
  from the target release and `<tag-name>` from `release-tag`. The publisher must reject a settings
  object whose `registry_url` points at another repository, another tag, a short or rewritten tag, a
  non-`https` URL, embedded credentials, a query, a fragment, a non-default port, or any alternate
  release-download surface.

When supplied as a public workflow input, `linked-artifact-settings` must be encoded as a UTF-8 JSON
string whose parsed value is the object shape above. For example:

```json
{ "enabled": false }
```

When enabled, the linked artifact storage record maps fields as follows:

| Storage metadata field | Value                                                                  |
| ---------------------- | ---------------------------------------------------------------------- |
| `name`                 | `final-asset-name`                                                     |
| `digest`               | `sha256:<expected-sha256>`                                             |
| `version`              | `linked-artifact-settings.version`                                     |
| `artifact_url`         | Uploaded primary asset browser URL                                     |
| `registry_url`         | `linked-artifact-settings.registry_url`                                |
| `repository`           | `linked-artifact-settings.repository`                                  |
| `github_repository`    | Target repository when required by the artifact metadata API contract. |

If the primary asset and sidecar upload succeed but linked artifact storage record creation fails,
the workflow must fail clearly, set `linked-artifact-result` to `failed-after-upload`, leave linked
artifact locator outputs unset, and must not delete, replace, or clobber the uploaded release
assets. When disabled, `linked-artifact-result` is `disabled` and linked artifact locator outputs
are unset. When enabled and successful, `linked-artifact-result` is `created` and the publisher must
expose at least one stable linked artifact metadata locator through `linked-artifact-url` or
`linked-artifact-id`; when the metadata API returns both, both outputs must be set.

## Partial failure behavior

If the primary asset upload succeeds but the sidecar upload fails, the workflow must fail clearly
without deleting, replacing, or clobbering the primary asset. The failure output must make the
partial state explicit so operators can retry sidecar publication or remove the incomplete release
asset according to repository policy.

If the GitHub API request or network transport returns an ambiguous result after the publisher
starts uploading the primary asset, the workflow must not assume either success or failure. It must
perform a same-run target release lookup for the primary asset name and computed digest. When that
lookup can prove that no primary asset exists, the result remains `failed-before-upload`. When it
proves that the primary asset exists with the expected digest but the sidecar was not uploaded, the
result is `partial-primary-uploaded`. When the lookup cannot determine whether the primary asset was
committed or finds an asset with the expected name but unknown or mismatched digest, the result is
`indeterminate-primary-upload` and the workflow fails without uploading the sidecar or linked
artifact metadata.

The same-run target release lookup must prove remote asset digest equality before it treats a
same-name asset as the asset uploaded by this run. The publisher may use a GitHub API digest or
checksum field only when the field explicitly identifies the bytes of the release asset and the
algorithm is SHA-256. If such a field is unavailable, absent, uses another algorithm, or is not
documented as an asset-byte digest, the publisher must download the candidate release asset bytes
through the same authenticated GitHub release asset surface and recompute SHA-256 locally. Asset
IDs, browser URLs, filenames, sizes, content types, release notes, logs, or native provenance
locators are not digest proof. If the candidate asset cannot be downloaded with the caller-scoped
token, if the downloaded bytes cannot be hashed, if the candidate digest is unavailable, or if the
digest differs from `expected-sha256`, the lookup cannot prove success and the result is
`indeterminate-primary-upload`.

When the lookup proves that the same-name primary asset exists with the expected SHA-256, the
publisher may classify the upload state from the sidecar state: `partial-primary-uploaded` when the
sidecar is absent after sidecar upload failed, and `completed` only when both primary and sidecar
assets are present with their expected digests. When the lookup proves that no same-name primary
asset exists after an upload attempt that failed before any remote commit was possible, the result
is `failed-before-upload`. A same-name asset with an unknown digest, a mismatched digest, or an
unreadable digest is never treated as a successful upload by this run.

Reruns are not allowed to overwrite or repair release assets silently. A rerun that observes the
primary asset or sidecar already present during duplicate preflight must fail before upload with the
same duplicate category as a fresh run. Operators must reconcile partial or indeterminate release
state outside the publisher before rerunning the production path.

The observable upload result is reported through `upload-result`:

- `completed`: primary asset and sidecar upload both succeeded.
- `failed-before-upload`: validation, verification, duplicate preflight, or target lookup failed
  before the primary asset was uploaded.
- `partial-primary-uploaded`: the primary asset upload succeeded, but sidecar upload failed.
- `indeterminate-primary-upload`: the publisher cannot prove whether the primary asset upload
  committed, or cannot prove that the remote asset with the target name has the expected digest.

When `upload-result` is `partial-primary-uploaded`, the workflow fails, primary asset outputs such
as `asset-name`, `asset-url`, and `asset-sha256` must be set when GitHub returned them,
`sidecar-name` must be the deterministic sidecar name, `sidecar-digest` must be set to the verified
producer bundle SHA-256 when the sidecar bytes were already verified, and `sidecar-url` must be
unset. When `upload-result` is `indeterminate-primary-upload`, release asset locator outputs must be
unset unless GitHub returned a locator for an asset whose digest was verified in the same run. A
duplicate primary asset or duplicate deterministic sidecar detected during preflight is
`failed-before-upload`, not a partial or indeterminate upload state.

## Outputs

| Output                       | Description                                                                                         |
| ---------------------------- | --------------------------------------------------------------------------------------------------- |
| `asset-name`                 | Final release asset name.                                                                           |
| `asset-url`                  | Browser URL of the uploaded asset.                                                                  |
| `asset-api-id`               | GitHub API asset ID if available.                                                                   |
| `asset-sha256`               | SHA-256 of the uploaded bytes.                                                                      |
| `sidecar-name`               | Provenance sidecar asset name.                                                                      |
| `sidecar-url`                | Browser URL of the sidecar.                                                                         |
| `sidecar-digest`             | SHA-256 of the sidecar bundle as 64 lowercase hex characters.                                       |
| `native-provenance-locators` | Native producer locators.                                                                           |
| `upload-result`              | `completed`, `failed-before-upload`, `partial-primary-uploaded`, or `indeterminate-primary-upload`. |
| `linked-artifact-result`     | `disabled`, `created`, or `failed-after-upload`.                                                    |
| `linked-artifact-url`        | Stable browser or API URL for the created linked artifact metadata record, or unset.                |
| `linked-artifact-id`         | Stable API identifier for the created linked artifact metadata record, or unset.                    |

`sidecar-digest` must equal `producer-provenance-sha256` and the SHA-256 digest of the exact bundle
bytes redistributed as the sidecar. It is set when the bundle bytes were retrieved and verified,
including `partial-primary-uploaded` cases where the sidecar upload failed after bundle
verification. It must be unset when producer provenance retrieval or digest verification failed.

`linked-artifact-url` and `linked-artifact-id` are set only when `linked-artifact-result` is
`created` and the metadata API returned the corresponding locator. Both outputs must be unset when
linked metadata is disabled, when metadata creation fails after upload, when the primary or sidecar
upload did not complete, or when metadata creation was not attempted. They are publication locator
outputs only and must not be used as substitutes for release asset digest verification or signed
producer provenance.

## Failure behavior

The publisher must fail before primary asset or sidecar upload when:

- The artifact bytes cannot be retrieved.
- The handoff uses the stale `artifact-artifact-name` field or omits `primary-artifact-name`.
- The computed digest differs from the expected digest.
- The provenance bundle bytes cannot be retrieved.
- The computed provenance bundle digest differs from `producer-provenance-sha256`.
- The release tag or target GitHub Release does not exist.
- The release tag is not a full `refs/tags/<tag-name>` ref.
- The final asset name is invalid, the primary asset name is already present, or the deterministic
  sidecar asset name is already present.
- Upstream producer provenance is missing, unsigned, unverifiable, or untrusted.
- The upstream subject digest does not match the bytes to upload.
- The upstream subject name differs from `expected-subject-name`.
- The selected producer policy cannot bind the final asset name to the verified producer artifact.
- The producer policy does not allow the upstream `builder.id`, `buildType`, source, release ref, or
  `externalParameters`.
- Explicit producer policy and verified signed release manifest policy conflict or have an empty
  intersection.
- A locator is provided without a retrievable producer provenance bundle artifact.
- A native provenance locator is malformed or unsupported.
- Linked artifact storage is enabled but the metadata job permission boundary is invalid: it lacks
  `artifact-metadata: write` or has prohibited signing or release mutation authority.
- The caller grants release mutation, signing, package publication, or linked metadata permissions
  to a job that must not hold that authority.
- The publisher cannot determine whether a started primary upload committed remotely.

The publisher must not expose any option to bypass upstream provenance verification in the
production path.

## TDD and fixtures

- Positive fixture: a valid producer handoff results in a release asset and a sidecar.
- Rejected fixtures: missing provenance, wrong subject name, missing final-asset binding, digest
  mismatch, stale `artifact-artifact-name` handoff field, duplicate asset name, non-existent
  release, raw artifact bypass, pre-existing deterministic sidecar name, sidecar upload failure
  after primary upload, indeterminate primary upload, malformed JSON input for complex handoff
  fields, excessive job permissions, and attempted re-signing of producer provenance.
- A YAML review checklist proving the publisher `workflow_call` schema does not expose unsupported
  target repository, custom token, raw artifact, overwrite, or bypass inputs.
- A YAML review checklist proving the publisher does not combine signing, release mutation, package
  publication, or linked artifact metadata authorities.
