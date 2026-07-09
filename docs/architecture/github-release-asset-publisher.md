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
  [0052](../decisions/0052-compose-npm-package-tarball-producer-with-release-asset-publisher.md)
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
| `expected-subject-name`             | yes      | Expected upstream subject name, must match `final-asset-name`.          |
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
satisfy the release asset name rules below and must equal `expected-subject-name`.
`source-repository` must be a canonical HTTPS repository URL, and `source-revision` must be the full
immutable source revision expected by the producer policy; for GitHub-hosted Git sources this is a
40-character lowercase Git commit SHA. For the initial npm composition, `source-repository` must use
the canonical GitHub repository URL rules defined by the
[JS/TS npm provenance and publish spec](js-ts-npm-provenance-publish.md). The publisher must reject
a `source-repository` value that cannot be canonicalized by those rules or that does not exactly
match the canonical `externalParameters.source.repository` value in the verified producer
provenance.

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

`final-asset-name` is the basename that will be uploaded to the target GitHub Release and the value
that must appear in the upstream producer provenance subject. The publisher must validate it before
retrieving or uploading release assets.

The value is invalid when any of the following are true:

- It is empty or becomes empty after trimming ASCII whitespace.
- It has leading or trailing ASCII whitespace.
- It contains `/`, `\\`, NUL, or any ASCII control character.
- It is exactly `.` or `..`.
- It contains a path traversal segment when interpreted as slash- or backslash-separated text.
- It differs from `expected-subject-name`.
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
7. The `subject[0].name` exactly matches the final release asset name.
8. Source repository, source revision, and other producer `externalParameters` match the trusted
   producer policy.
9. No unexpected `externalParameters` are present when the policy requires strict matching.

## Final release asset subject name

The final release asset name must be identical to the upstream producer provenance
`subject[0].name`. If they differ, the publisher must fail before upload. This avoids the need for a
separate signed name-mapping predicate.

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
- `repository` must identify the GitHub repository that owns the release asset storage surface.
- `registry_url` must be the GitHub Release download URL prefix for the target release.

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
the workflow must fail clearly, set `linked-artifact-result` to `failed-after-upload`, and must not
delete, replace, or clobber the uploaded release assets. When disabled, `linked-artifact-result` is
`disabled`. When enabled and successful, it is `created`.

## Partial failure behavior

If the primary asset upload succeeds but the sidecar upload fails, the workflow must fail clearly
without deleting, replacing, or clobbering the primary asset. The failure output must make the
partial state explicit so operators can retry sidecar publication or remove the incomplete release
asset according to repository policy.

The observable upload result is reported through `upload-result`:

- `completed`: primary asset and sidecar upload both succeeded.
- `failed-before-upload`: validation, verification, duplicate preflight, or target lookup failed
  before the primary asset was uploaded.
- `partial-primary-uploaded`: the primary asset upload succeeded, but sidecar upload failed.

When `upload-result` is `partial-primary-uploaded`, the workflow fails, primary asset outputs such
as `asset-name`, `asset-url`, and `asset-sha256` must be set when GitHub returned them,
`sidecar-name` must be the deterministic sidecar name, and `sidecar-url` must be unset. A duplicate
primary asset or duplicate deterministic sidecar detected during preflight is
`failed-before-upload`, not a partial upload state.

## Outputs

| Output                       | Description                                                         |
| ---------------------------- | ------------------------------------------------------------------- |
| `asset-name`                 | Final release asset name.                                           |
| `asset-url`                  | Browser URL of the uploaded asset.                                  |
| `asset-api-id`               | GitHub API asset ID if available.                                   |
| `asset-sha256`               | SHA-256 of the uploaded bytes.                                      |
| `sidecar-name`               | Provenance sidecar asset name.                                      |
| `sidecar-url`                | Browser URL of the sidecar.                                         |
| `sidecar-digest`             | Digest of the sidecar bundle.                                       |
| `native-provenance-locators` | Native producer locators.                                           |
| `upload-result`              | `completed`, `failed-before-upload`, or `partial-primary-uploaded`. |
| `linked-artifact-result`     | `disabled`, `created`, or `failed-after-upload`.                    |

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
- The upstream subject name differs from the final asset name.
- The producer policy does not allow the upstream `builder.id`, `buildType`, source, release ref, or
  `externalParameters`.
- A locator is provided without a retrievable producer provenance bundle artifact.
- A native provenance locator is malformed or unsupported.
- Linked artifact storage is enabled but the metadata job permission boundary is invalid: it lacks
  `artifact-metadata: write` or has prohibited signing or release mutation authority.

The publisher must not expose any option to bypass upstream provenance verification in the
production path.

## TDD and fixtures

- Positive fixture: a valid producer handoff results in a release asset and a sidecar.
- Rejected fixtures: missing provenance, wrong subject name, digest mismatch, stale
  `artifact-artifact-name` handoff field, duplicate asset name, non-existent release, raw artifact
  bypass, pre-existing deterministic sidecar name, sidecar upload failure after primary upload,
  malformed JSON input for complex handoff fields, and attempted re-signing of producer provenance.
- A YAML review checklist proving the publisher does not combine signing and release mutation
  authorities.
