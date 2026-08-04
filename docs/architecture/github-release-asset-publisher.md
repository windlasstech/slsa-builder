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
  [0064](../decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md),
  [0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [0067](../decisions/0067-converge-repeated-runs-within-run-identity.md),
  [0072](../decisions/0072-use-sidecar-first-pair-binding-for-release-asset-run-ownership.md),
  [0074](../decisions/0074-use-single-job-mutation-segments-with-detection-based-cross-run-safety.md),
  [0075](../decisions/0075-queue-mutation-segment-contenders-with-queue-max.md),
  [0076](../decisions/0076-use-observation-preflights-and-first-mutation-classification.md)
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
| `trusted-builder-id`                | string | yes      | none    | Narrowing constraint on the selected policy's `builder.id`.   |
| `trusted-build-type`                | string | yes      | none    | Exact closed-registry selector and caller constraint.         |
| `expected-subject-name`             | string | yes      | none    | Narrowing constraint on the registered producer subject.      |
| `expected-subject-sha256`           | string | yes      | none    | Narrowing SHA-256 constraint on the registered subject.       |
| `source-repository`                 | string | yes      | none    | Narrowing canonical producer source repository constraint.    |
| `source-revision`                   | string | yes      | none    | Narrowing immutable source revision constraint.               |
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

`trusted-build-type` selects an entry from the
[closed producer-policy registry](#closed-producer-policy-registry). It does not register a producer
or extend the selected policy. All other `trusted-*`, subject, and source inputs are caller
constraints. They must intersect the selected policy and may only narrow what that policy allows. A
caller value that conflicts with a registered baseline fails before upload with
`windlass.verify.error.trusted-producer-policy-conflict`, or the narrower registered field
diagnostic when one applies.

### Permissions matrix

Per ADRs 0058 and 0066, the publisher workflow must keep the default top-level permission set
read-only and elevate only the jobs that need mutation authority; a workflow that grants broader
authority fails static conformance and must not be released.

| Job class                                  | Required permissions                         | Forbidden permissions                                                                             |
| ------------------------------------------ | -------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| Handoff download, digest, and verification | `contents: read`                             | `contents: write`, `id-token: write`, `attestations: write`, `artifact-metadata: write`           |
| Release upload                             | `contents: write`                            | `id-token: write`, `attestations: write`, `artifact-metadata: write`, package publish permissions |
| Linked artifact metadata, when enabled     | `artifact-metadata: write`, `contents: read` | `contents: write`, `id-token: write`, `attestations: write`, release upload authority             |

The caller job must grant only the permissions required by the selected path. A caller that enables
linked artifact metadata must grant `artifact-metadata: write`; a caller that disables it must not
cause the metadata job to run or request that permission. Missing required permissions and excessive
job permissions are distinct pre-upload failures.

Permission validation has two distinct layers:

1. A static YAML conformance check runs at lint time against the caller workflow file. It must
   verify that the calling job declares the selected path's required permissions and no forbidden
   permissions; a nonconforming caller file fails lint and is not an eligible release caller. This
   is the only pre-mutation permission check.
2. Runtime authority uses ADR 0076 tier-2 first-mutation classification. No side-effect-free
   write-capability probe exists on GitHub: spike-verified, the repository `permissions` API field
   returns all-false under both `contents: write` and `contents: read` job permissions, and no API
   exposes job-effective `GITHUB_TOKEN` permissions (spike repository `yunseo-kim/slsa-spike-tmp` at
   commit `50b7cbe`, run 30935100751). Therefore, the first mutating call is the runtime authority
   check. A definitive HTTP `403` or `401` response must fail the run with
   `windlass.verify.error.mutation-permission-denied`, without read-back or further mutation,
   because the definitive rejection proves no mutation occurred. Other API or transport failures
   must retain their own categories and must not be mislabeled as permission failures; a failure to
   do so fails conformance. If a mutating request may already have been submitted when its result is
   ambiguous, the publisher must perform ADR 0067 read-back and fail as `indeterminate` unless that
   classification proves another outcome. The primary-upload case uses
   `windlass.verify.error.publisher-indeterminate-primary-upload` when the classification remains
   unresolved.

### Mutation-class concurrency

The handoff download, digest, and verification jobs are PRE-mutation jobs. Each must declare
job-level concurrency with `cancel-in-progress: true`; a workflow that omits that declaration, sets
it to `false`, or places one of these jobs in the mutation concurrency group must fail static
workflow conformance before release use.

Per ADRs 0074 and 0075, exactly one `release-upload` mutation-class job uploads both release assets:
first the provenance sidecar, then the primary asset. It must declare job-level concurrency with
`cancel-in-progress: false` and `queue: max`; a missing declaration, a `true` value, a queue value
other than `max`, or a second release-asset upload job fails static workflow conformance.
Pre-mutation jobs must declare `cancel-in-progress: true` with no `queue` key.

The exact mutation concurrency group is:

```text
release-mutation-${{ github.repository }}-${{ github.ref_name }}
```

The key is composed only from the literal namespace plus `github.repository` and `github.ref_name`.
It must fail static workflow conformance if it uses any other context. In particular, a key must not
contain `github.workflow`: inside a called reusable workflow that value resolves to the caller's
workflow name, creating a self-cancellation trap in which the caller and callee can collide. The npm
publish job, the one release-upload job, and the manifest publish job for one caller repository and
release source ref use this one shared mutation key.

The release-asset mutation segment is the one release-upload job's occupancy of this concurrency
group. It begins when that job enters the group and ends when the job completes. At segment entry,
before the sidecar upload, the release-upload job must re-read and revalidate the target repository
and release ref, release existence, expected primary and sidecar digests, and the absence or
same-run convergence classification of both target asset names. A failed or indeterminate
revalidation fails before mutation with the corresponding evidence. Checks completed before queueing
are never sufficient after the job enters the segment.

On github.com, `queue: max` keeps up to one hundred pending mutation segment contenders in arrival
order behind the running segment. A queued contender must wait, then either converge when it is a
retry of the committing `run_id` or fail closed with its classified remote state. The platform
rejects arrivals beyond that pending limit. Caller-side whole-invocation serialization is an
optional compute-saving optimization, never a substitute for the publisher's mutation-class
declaration.

## One-primary-asset unit

Each publisher run uploads exactly one primary release asset. If a project needs multiple release
assets, it must invoke the publisher once per asset.

## Existing release requirement

The publisher uploads to an existing GitHub Release identified by an existing Git tag. The publisher
does not create the release or the tag. Before any upload, it must read the target's `draft` and
`immutable` state, as well as both required asset names. If the tag or release does not exist, the
publisher fails before any upload.

## Draft and prerelease behavior

The publisher may upload to a draft or prerelease target only if the release already exists. The
publisher does not change the draft or prerelease status. Draft state does not weaken the immutable
target rule: when `immutable` is true and either required asset is absent, the publisher fails
before mutation with `windlass.verify.error.release-target-immutable`. When `immutable` is true and
both assets are complete and satisfy the same-run pair gate in
[Same-run convergence](#same-run-convergence), the publisher may perform only read-only same-run
convergence. It must not upload, delete, replace, or create linked metadata.

## Duplicate asset behavior

ADR 0067 amends the original strict duplicate rule only for retries within the same `run_id`. For a
new `run_id`, before uploading anything, the publisher must check the target release for both the
primary release asset name and the deterministic sidecar name `<asset-name>.intoto.jsonl`. If either
name already exists under the target release, the publisher must fail as `foreign-conflict` without
uploading the primary asset or the sidecar. The publisher must not overwrite, replace, delete, or
clobber an existing asset; the sole deletion exception is the same-run `starter` asset defined in
[Same-run convergence](#same-run-convergence), and any other attempted deletion fails the run before
the delete call.

A retry with the same `run_id` may converge on a pre-existing primary asset only through the
five-condition pair gate in [Same-run convergence](#same-run-convergence). A sidecar-present,
primary-absent state may continue by uploading the primary after that gate proves the sidecar is
this run's expected provenance. A primary-present, sidecar-absent state is definitionally foreign
and must fail as `foreign-conflict`. Existence alone is not proof. A same-run retry that cannot
prove the required binding fails as `foreign-conflict` or `indeterminate` as specified below and
must not upload, overwrite, or adopt the existing asset; violating that prohibition fails the run
and leaves all existing assets unchanged.

If the preflight duplicate check passes but a later GitHub API race or upload failure prevents the
primary asset from being uploaded after the sidecar succeeds, the run enters the aggregate partial
failure condition described below. That condition is for post-preflight API or transport failures,
not for known duplicate names.

## Producer-to-publisher handoff contract

### Closed producer-policy registry

This registry is the publisher's complete initial producer-policy admission set. It is keyed by the
exact observed producer `buildType`, has no wildcard, alias, default, union, or caller-extension
mechanism, and is evaluated before any release upload. The sole registered entry is:

| Exact `buildType`                                                  | Required baseline                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1` | The signer repository is `windlasstech/slsa-builder`; its signer workflow is `.github/workflows/js-ts-npm-package-slsa3.yml`; `builder.id` is SHA-based; `subject[0].name` is the npm Package URL; `subject[0].digest` contains matching SHA-512 and SHA-256 digests; `final-asset-name` equals `externalParameters.package.tarball_name`; canonical source repository, immutable source revision, and identical full source, release, and caller release-tag refs apply; and `externalParameters` uses the closed JS/TS npm schema. |

For this entry, the publisher validates the signer repository and workflow, SHA-based `builder.id`,
npm Package URL subject, both required subject digests, tarball filename binding, canonical source
and release-ref rules, and the complete closed npm `externalParameters` schema. The profile
baseline, caller constraints, digest-verified handoff, and authenticated release-manifest
constraints are intersected. Every source that constrains an observed field must allow that value.

An absent or unknown observed `buildType` fails before upload with
`windlass.verify.error.unregistered-producer-build-type`. A caller may select only the exact entry
above using `trusted-build-type`; it may not add, replace, relax, union, or override any baseline
constraint. A conflicting caller constraint fails with
`windlass.verify.error.trusted-producer-policy-conflict`, unless a narrower registered field
diagnostic applies. A future producer policy requires profile admission through its own accepted
ADR, specification process, and accepted and rejected fixtures before registry insertion.

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
| `trusted-builder-id`                | yes      | Narrowing constraint on the selected policy's `builder.id`.             |
| `trusted-build-type`                | yes      | Exact registry selector and caller constraint, not an extension.        |
| `expected-subject-name`             | yes      | Narrowing constraint on the selected policy's subject name.             |
| `expected-subject-sha256`           | yes      | Narrowing constraint on the selected policy's subject SHA-256.          |
| `source-repository`                 | yes      | Narrowing source repository constraint for the selected policy.         |
| `source-revision`                   | yes      | Narrowing source revision constraint for the selected policy.           |
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

All required handoff string fields must be non-empty after trimming ASCII whitespace. The
`trusted-build-type` value must exactly select a registry entry before artifact retrieval or upload;
an unknown value fails with `windlass.verify.error.unregistered-producer-build-type`. SHA-256 fields
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

Before publication, the publisher must select the exact `buildType` registry entry and verify the
upstream producer provenance against the intersection of that immutable baseline, caller
constraints, the digest-verified handoff, and authenticated release-manifest constraints:

1. The attestation signature is valid.
2. The signer repository is `windlasstech/slsa-builder` and the signer workflow is
   `.github/workflows/js-ts-npm-package-slsa3.yml`.
3. The `predicateType` is `https://slsa.dev/provenance/v1`.
4. The SHA-based `builder.id` matches the registry baseline and `trusted-builder-id` constraint.
5. The `buildType` exactly selects and matches the registered policy.
6. The `subject[0].digest.sha512` and `subject[0].digest.sha256` both match the primary bytes; the
   SHA-256 also matches `expected-subject-sha256`, `expected-sha256`, and the bytes to upload.
7. The `subject[0].name` is the registered npm Package URL and exactly matches
   `expected-subject-name`.
8. The final release asset name is authorized by producer policy and handoff fields. For the initial
   npm composition, this means the asset name matches the pack-produced tarball filename recorded in
   producer provenance and the same-run handoff, while `subject[0].name` remains the npm Package
   URL.
9. Source repository and revision match every applicable narrowing constraint. The canonical source
   repository and `externalParameters.source.repository` match; `externalParameters.source.ref`,
   `externalParameters.release.ref`, and `release-tag` are identical full tag refs; and
   `externalParameters.release.version_tag` reconstructs that ref.
10. `externalParameters.package.tarball_name` equals `final-asset-name`, and the closed JS/TS npm
    `externalParameters` schema has no missing or unexpected member.

For schema version `1`, the release manifest constrains Windlass release identity, producer workflow
path/SHA, producer `builder.id`, producer `buildType`, and publisher workflow path/SHA/role; it does
not constrain caller-specific source, producer release ref, subject, digest, final asset name, or
strict `externalParameters` fields. Those fields remain constrained by the registry baseline, caller
constraints, signed producer provenance, and digest-verified handoff. The publisher must fail before
upload when any source that constrains an observed field does not allow it. It must not let the
manifest, a caller value, or a handoff value relax the registry baseline, use either-source-allowed,
or use last-writer-wins behavior.

## Final release asset and producer subject binding

The final release asset name must be bound to the upstream producer provenance by the selected
producer policy. Producer profiles whose subject name is their release asset basename can satisfy
this by requiring `final-asset-name` to equal `subject[0].name`. The initial npm composition cannot:
ADR 0064 requires the npm producer subject to be the package Package URL. For npm producer
provenance, the publisher must therefore verify that `subject[0].name` matches the expected npm
Package URL and separately verify that `final-asset-name` exactly matches
`externalParameters.package.tarball_name` and the digest-verified handoff.

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

### Linked artifact metadata convergence

Linked artifact metadata is a distinct remote surface under ADR 0074. It is not part of the
`release-upload` job's release-asset mutation segment and must not claim that the sidecar-first pair
gate, release-asset concurrency group, or GitHub Release asset `digest` field serializes or proves
the metadata record. The metadata job may start only after the primary and sidecar are complete, but
its record-creation step is independently governed by ADRs 0066 and 0067.

**Idempotency-key choice:** the metadata record's idempotency key is `github.run_id`. This applies
the ADR 0067 run-identity rule uniformly to this distinct mutation step. The API has no specified
record field that can carry or prove that run identity, so the binding proof is the retry-attempt
gate plus equality of the complete expected record identity below. The publisher must not infer
same-run ownership from a record ID, URL, timestamp, actor, or existence alone.

The metadata record identity is the exact tuple of `github_repository`, `name`, `digest`, `version`,
`artifact_url`, `registry_url`, and `repository` from the mapping above, after the required input
canonicalization. Before a create call and after an ambiguous create response, the publisher must
look up every metadata record in the target `github_repository` with the expected `name` and
`version`, then compare each candidate with that complete tuple. It must not treat a match on only
name, digest, version, URL, ID, or any proper subset as a duplicate or binding proof.

For enabled metadata, the metadata job must classify the record into exactly one ADR 0067 outcome
state before its first create call and after every ambiguous create call:

- `committed-as-expected`: only a retry attempt of the same `run_id` may use this state, and exactly
  one readable remote record matches the complete expected record identity with no other candidate
  record. The job treats the step as satisfied without another create call and sets
  `linked-artifact-result` to `converged-as-expected`.
- `absent`: no remote record matches the complete expected record identity, and no conflicting
  candidate record is found, after the applicable lookup or bounded polling. The job may create the
  record only in this state.
- `foreign-conflict`: a record candidate exists but differs in any member of the expected record
  identity, more than one matching record exists, or any metadata record exists for a new `run_id`.
  The job must fail without creating, updating, deleting, or adopting a record.
- `indeterminate`: the publisher cannot determine the candidate set or compare every required
  identity member within the polling bound. The job must fail without another metadata mutation.

After a create call, including one with an ambiguous response, the publisher must poll the metadata
lookup once immediately and then every 5 seconds, stopping after 24 total observations or 120
seconds from the first request, whichever occurs first. It may finish early only when it can
classify `committed-as-expected` or `foreign-conflict`. At the bound, a stable empty candidate set
is `absent`; an unreadable record, incomplete identity data, repeated API or transport failure, or
contradictory observations is `indeterminate`. A definitive HTTP `403` or `401` response to the
first metadata create call must fail with `windlass.verify.error.mutation-permission-denied`, with
no read-back or further metadata mutation, because the rejection proves no metadata mutation
occurred. Other API or transport failures retain their own categories and must not be reported as
permission failures.

The always-run status report must include `linked-artifact-metadata` when enabled, with its final
ADR 0067 outcome, the expected record identity, and any returned record locator. `created` reports
`committed-as-expected` only after read-back proves exactly one matching record. A disabled metadata
path reports `disabled` outside the four-state machine.

## Same-run convergence

Per ADR 0067, `run_id` is the publisher idempotency key. Re-run failed jobs is the supported
recovery surface: an incremented `run_attempt` under the same `run_id` may converge, including after
cancellation. Re-run all jobs is not a recovery surface and re-executes every job, although each
mutation step remains subject to the same-`run_id` rules below. A new `run_id` remains fail-closed
for any pre-existing primary asset or sidecar, even when its bytes match, and fails as
`foreign-conflict` without mutation.

At the start of the one release-upload job, inside its mutation segment, the publisher must re-read
the target's `draft` and `immutable` state. If `immutable` is true and either required asset is
`absent`, `foreign-conflict`, or `indeterminate`, it must fail before mutation with
`windlass.verify.error.release-target-immutable`. If `immutable` is true and both assets satisfy the
same-run pair gate below, the publisher may finish only as read-only convergence. The primary
asset's remote binding proves content integrity, not a `run_id`; GitHub Release asset fields cannot
prove publication-event custody. No upload, deletion, retry, or linked artifact metadata mutation is
permitted. A mutable or draft target continues through the normal convergence rules below.

Before a release-asset mutation and after any mutating call with an ambiguous response, the
publisher must classify each of the primary asset and sidecar independently into exactly one ADR
0067 outcome state; inability to produce one of these states fails the run as `indeterminate`:

- `committed-as-expected`: the remote asset exists and its authoritative SHA-256 digest equals the
  local expected value. This classification is available only after the `run_attempt` gate permits
  same-`run_id` convergence, with that value recovered from prior-attempt carry-over or recomputed
  from the exact verified handoff bytes.
- `absent`: the remote asset does not exist after the applicable lookup or bounded post-call
  polling.
- `foreign-conflict`: the remote asset exists but has a different digest, or any asset with that
  name pre-exists a new `run_id`.
- `indeterminate`: the publisher cannot determine presence or authoritative digest equality within
  the polling bounds.

For a same-`run_id` retry, the publisher must recover each local expected value from prior-attempt
outputs or artifacts when available, or recompute it from the exact verified handoff bytes. Failure
of both paths classifies the step as `indeterminate` and fails the run without adoption. It must
then apply the following pair gate before treating a pre-existing primary as
`committed-as-expected`:

1. The remote sidecar's authoritative GitHub `digest` equals the expected bundle digest.
2. The sidecar verifies under the full verification policy, including signer and source identity
   binding required by ADR 0068.
3. The sidecar's verified Fulcio Run Invocation URI has a run-id equal to `github.run_id`; the
   attempt component is ignored, so an earlier `run_attempt` is acceptable.
4. The sidecar's signed subject and producer fields bind the expected primary asset name and digest.
5. The remote primary asset's authoritative GitHub `digest` equals that bound digest.

Only when all five conditions hold may the retry classify the pre-existing primary as
`committed-as-expected`, reporting that the release slot contains expected provenance-covered bytes
without attributing the upload to this run. A sidecar-present, primary-absent state that satisfies
conditions 1 through 4 may continue by uploading the primary. A primary-present, sidecar-absent
state is `foreign-conflict`. The workflow uploads only an `absent` asset and fails closed without
mutation on `foreign-conflict` or `indeterminate`, naming the state and remote evidence.

### Release asset digest binding and polling

The authoritative binding is the GitHub Release asset `digest` field, which exposes the asset-byte
SHA-256 and has been generally available since 12025-06-03. The publisher must compare its
`sha256:<64 lowercase hexadecimal characters>` value with the locally computed expected SHA-256; a
missing algorithm prefix, another algorithm, malformed value, or unequal digest cannot prove
`committed-as-expected` and fails as `foreign-conflict` when it proves different content or as
`indeterminate` when no authoritative digest can be read.

For read-back after an upload or an ambiguous mutating call, the publisher must poll the release
asset endpoint once immediately and then every 5 seconds, stopping after 24 total observations or
120 seconds from the first request, whichever occurs first. It may finish earlier only when it can
classify `committed-as-expected` or `foreign-conflict`. At the bound, repeated authoritative absence
is `absent`; an existing asset with a missing or unreadable `digest`, repeated API or transport
failure, or any contradictory observations are `indeterminate` and fail the run without another
mutation. An HTTP `403` additionally identifies the runtime permission failure required by the
[permissions matrix](#permissions-matrix); the final mutation state is `indeterminate`, and the run
fails with both the permission signal and observed API evidence.

The publisher must not substitute asset IDs, names, URLs, sizes, content types, logs, native
provenance locators, or an unbounded download loop for the `digest` binding; doing so fails
conformance, and runtime inability to obtain the authoritative field reaches `indeterminate` at the
bound. The existing authenticated-download-and-local-hash check remains optional diagnostic evidence
but cannot by itself change an `indeterminate` ADR 0067 outcome into `committed-as-expected`.

### Sole deletion exception

The publisher must never delete a release asset except when all of the following are proven: the
asset has GitHub state `starter`; it was left by this same run's own failed upload; its API asset ID
was returned to this `run_id` by that failed upload or recovered from this run's prior-attempt
outputs; and its target release and asset name match the current mutation step. This is the sole
deletion exception. When all conditions hold, the run may delete only that own-run `starter` asset
and retry the upload. If any condition is absent or indeterminate, deletion is forbidden and the run
fails without deleting, overwriting, or replacing the asset.

### Always-run status report

Per ADR 0067, the publisher workflow must include a status-report job with `if: always()` that
depends on every mutation-class job and records each step's final outcome and evidence on success,
failure, and cancellation. The report is diagnostic and must never be accepted as trusted input by
another run; a missing, malformed, or unproducible report fails the workflow and no convergence
success may be claimed.

A valid machine-readable report includes the stable `run_id`, current `run_attempt`, and the exact
outcome-state names:

```json
{
  "run_id": "30744787367",
  "run_attempt": 2,
  "steps": {
    "primary-asset": {
      "outcome": "committed-as-expected",
      "publication_evidence": "converged-as-verified-pair-custody-unproven",
      "digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      "asset_url": "https://github.com/example/project/releases/download/v1.2.3/package.tgz"
    },
    "provenance-sidecar": {
      "outcome": "committed-as-expected",
      "publication_evidence": "uploaded-with-confirmed-receipt",
      "digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      "asset_url": "https://github.com/example/project/releases/download/v1.2.3/package.tgz.intoto.jsonl"
    }
  }
}
```

A successful `committed-as-expected` step must include exactly one `publication_evidence` substate:
`uploaded-with-confirmed-receipt`, `converged-from-prior-receipt`, or
`converged-as-verified-pair-custody-unproven`. The first two require the matching server-assigned
asset-ID receipt, either from this attempt or a prior attempt. The third requires the five-condition
pair gate and must not assert that this run performed the upload. A report with another outcome or
substate spelling, a changed `run_id`, no final classification for either mutation step, evidence
that contradicts its classification, or a custody assertion without a matching asset-ID receipt is
invalid; the reporting job must reject it, fail the workflow, and leave the report unavailable as a
successful diagnostic.

## Partial failure behavior

If the sidecar upload succeeds but the primary asset upload fails, the workflow must fail clearly
without deleting, replacing, or clobbering the sidecar. The failure output must make the aggregate
partial condition explicit so operators can use same-`run_id` convergence or reconcile the
incomplete release asset according to repository policy.

An immutable target never enters this partial-failure path. Its target state is read before upload,
and any incomplete required asset set fails before mutation with
`windlass.verify.error.release-target-immutable`. Only a complete, digest-proven same-`run_id` set
may converge, read-only.

If the GitHub API request or network transport returns an ambiguous result after the publisher
starts uploading the sidecar, the workflow must not assume either success or failure. It must
perform a same-run target release lookup for the sidecar name and computed digest. When that lookup
can prove that no sidecar exists, the result remains `failed-before-upload`. When it proves that the
sidecar exists with the expected digest and the primary was not uploaded, the result is
`partial-sidecar-uploaded`. When the lookup cannot determine whether the sidecar was committed or
finds a sidecar with an unknown digest, the result is `indeterminate-sidecar-upload`, the mutation
outcome is `indeterminate`, and the workflow fails without uploading the primary or linked artifact
metadata. A mismatched authoritative digest is `foreign-conflict`, fails the workflow without
further mutation, and sets aggregate `upload-result` to `foreign-conflict`.

ADR 0067 amends the earlier same-run target lookup rule that permitted an authenticated download and
local hash as the binding fallback. The lookup must now prove remote asset digest equality through
the bounded GitHub Release asset `digest` polling contract above; failure to obtain that proof is
`indeterminate`, fails the workflow, and maps to aggregate `upload-result`
`indeterminate-sidecar-upload`. Asset IDs, browser URLs, filenames, sizes, content types, release
notes, logs, native provenance locators, downloaded bytes, and local hashes are not authoritative
remote binding proof.

When the lookup proves that the same-name sidecar exists with the expected SHA-256, the publisher
may classify the aggregate upload result from the primary result: `partial-sidecar-uploaded` when
the primary is absent after primary upload failed, and `completed` only when both primary and
sidecar assets are present with their expected digests. When the lookup proves that no same-name
sidecar exists after an upload attempt that failed before any remote commit was possible, the result
is `failed-before-upload`. A same-name asset with an unknown, mismatched, or unreadable digest is
never treated as a successful upload by this run.

ADR 0067 amends, rather than removes, the original blanket rerun failure rule. Reruns are not
allowed to overwrite or repair release assets silently: a new `run_id` that observes the primary
asset or sidecar already present during duplicate preflight must fail before upload as
`foreign-conflict`. Only a retry attempt within the same `run_id` may converge under the digest
binding, polling, and sole-deletion rules above; `foreign-conflict` or `indeterminate` still fails
without mutation. Operators must reconcile cross-run partial or indeterminate release conditions
outside the publisher before starting another run.

The observable aggregate upload result is reported through `upload-result`. These compatibility
values summarize the pair of mutation-step results and are not ADR 0067 outcome-state names:

- `completed`: sidecar and primary asset upload both succeeded.
- `failed-before-upload`: validation, verification, duplicate preflight, or target lookup failed
  before the sidecar was uploaded.
- `partial-sidecar-uploaded`: the sidecar upload succeeded, but primary asset upload failed.
- `foreign-conflict`: authoritative remote content or cross-run ownership conflicts with this run.
- `indeterminate-sidecar-upload`: the publisher cannot prove whether the sidecar upload committed,
  or cannot prove that the remote asset with the target name has the expected digest.

When `upload-result` is `partial-sidecar-uploaded`, the workflow fails, `sidecar-name`,
`sidecar-digest`, and `sidecar-url` must be set when GitHub returned them, `asset-name` must be the
final asset name, and primary asset locator outputs must be unset. When `upload-result` is
`foreign-conflict` or `indeterminate-sidecar-upload`, release asset locator outputs must be unset
unless GitHub returned a locator for an asset whose digest was verified in the same run. A duplicate
primary asset or duplicate deterministic sidecar detected for a new `run_id` during preflight has
outcome `foreign-conflict` and aggregate result `failed-before-upload`; it is not a partial or
indeterminate aggregate upload result.

## Outputs

| Output                       | Description                                                                                                             |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `asset-name`                 | Final release asset name.                                                                                               |
| `asset-url`                  | Browser URL of the uploaded asset.                                                                                      |
| `asset-api-id`               | GitHub API asset ID if available.                                                                                       |
| `asset-sha256`               | SHA-256 of the uploaded bytes.                                                                                          |
| `sidecar-name`               | Provenance sidecar asset name.                                                                                          |
| `sidecar-url`                | Browser URL of the sidecar.                                                                                             |
| `sidecar-digest`             | SHA-256 of the sidecar bundle as 64 lowercase hex characters.                                                           |
| `native-provenance-locators` | Native producer locators.                                                                                               |
| `upload-result`              | `completed`, `failed-before-upload`, `partial-sidecar-uploaded`, `foreign-conflict`, or `indeterminate-sidecar-upload`. |
| `linked-artifact-result`     | `disabled`, `created`, `converged-as-expected`, or `failed-after-upload`.                                               |
| `linked-artifact-url`        | Stable browser or API URL for the created linked artifact metadata record, or unset.                                    |
| `linked-artifact-id`         | Stable API identifier for the created linked artifact metadata record, or unset.                                        |

`sidecar-digest` must equal `producer-provenance-sha256` and the SHA-256 digest of the exact bundle
bytes redistributed as the sidecar. It is set when the bundle bytes were retrieved and verified,
including `partial-sidecar-uploaded` cases where the sidecar upload completed before primary upload
failed. It must be unset when producer provenance retrieval or digest verification failed.

`linked-artifact-url` and `linked-artifact-id` are set only when `linked-artifact-result` is
`created` or `converged-as-expected` and the metadata API returned the corresponding locator. Both
outputs must be unset when linked metadata is disabled, when metadata creation fails after upload,
when the primary or sidecar upload did not complete, or when metadata creation was not attempted.
They are publication locator outputs only and must not be used as substitutes for release asset
digest verification or signed producer provenance.

## Failure behavior

The publisher must fail before sidecar or primary asset upload when:

- The artifact bytes cannot be retrieved.
- The handoff uses the stale `artifact-artifact-name` field or omits `primary-artifact-name`.
- The computed digest differs from the expected digest.
- The provenance bundle bytes cannot be retrieved.
- The computed provenance bundle digest differs from `producer-provenance-sha256`.
- The release tag or target GitHub Release does not exist.
- The target is immutable and either required asset is absent, foreign, or indeterminate; this fails
  with `windlass.verify.error.release-target-immutable` before any upload.
- The release tag is not a full `refs/tags/<tag-name>` ref.
- The final asset name is invalid, or a primary or deterministic sidecar asset is `foreign-conflict`
  under the ADR 0067 new-run or same-run rules.
- Upstream producer provenance is missing, unsigned, unverifiable, or untrusted.
- The observed producer `buildType` is absent from the closed registry; this fails with
  `windlass.verify.error.unregistered-producer-build-type` before upload.
- The upstream subject digest does not match the bytes to upload.
- The upstream subject name differs from `expected-subject-name`.
- The selected producer policy cannot bind the final asset name to the verified producer artifact.
- The producer policy does not allow the upstream `builder.id`, `buildType`, source, release ref, or
  `externalParameters`.
- Explicit producer policy and verified signed release manifest policy conflict or have an empty
  intersection.
- A caller constraint attempts to widen, replace, union with, or override a registered producer
  baseline; this fails with `windlass.verify.error.trusted-producer-policy-conflict` or the narrower
  registered field diagnostic.
- A locator is provided without a retrievable producer provenance bundle artifact.
- A native provenance locator is malformed or unsupported.
- Linked artifact storage is enabled but the metadata job permission boundary is invalid: it lacks
  `artifact-metadata: write` or has prohibited signing or release mutation authority.
- The caller grants release mutation, signing, package publication, or linked metadata permissions
  to a job that must not hold that authority.
- The publisher cannot determine whether a started sidecar or primary upload committed remotely.

The publisher must not expose any option to bypass upstream provenance verification in the
production path.

Here and in fixture names, stale `artifact-artifact-name` refers to the field's pre-rename name; the
current field is `primary-artifact-name`.

## TDD and fixtures

- Positive fixture: a valid registered npm producer handoff results in a release asset and a
  sidecar.
- Rejected fixtures: missing provenance, wrong subject name, missing final-asset binding, digest
  mismatch, stale `artifact-artifact-name` handoff field, duplicate asset name, non-existent
  release, raw artifact bypass, pre-existing deterministic sidecar name, primary upload failure
  after sidecar upload, indeterminate sidecar upload, ambiguous primary upload that remains
  unresolved after ADR 0067 read-back and fails with
  `windlass.verify.error.publisher-indeterminate-primary-upload`, malformed JSON input for complex
  handoff fields, excessive job permissions, and attempted re-signing of producer provenance.
- Producer-policy fixtures accept only
  `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1` with its signer, SHA-based
  `builder.id`, npm PURL, SHA-512 plus SHA-256, tarball-name, canonical source/ref, and closed npm
  `externalParameters` requirements. They reject an unknown build type before upload with
  `unregistered-producer-build-type` and reject every caller baseline override or union attempt with
  `trusted-producer-policy-conflict` or the narrower field diagnostic.
- Immutable-target fixtures accept a mutable or draft existing release, reject an immutable target
  with either required asset absent using `release-target-immutable` before mutation, and accept a
  complete digest-proven same-`run_id` pair only as read-only convergence. They reject cross-run or
  indeterminate immutable evidence with `release-target-immutable`.
- A YAML review checklist proving the publisher `workflow_call` schema does not expose unsupported
  target repository, custom token, raw artifact, overwrite, or bypass inputs.
- A YAML review checklist proving the publisher does not combine signing, release mutation, package
  publication, or linked artifact metadata authorities.
- ADR 0074 static conformance rejects a workflow graph that splits sidecar and primary mutations on
  the same Release asset surface across jobs. A topology fixture proves the one release-upload job
  executes sidecar-first, then primary, with revalidation at the start of that job's segment.
- ADR 0075 static conformance requires `queue: max` with `cancel-in-progress: false` on the
  release-upload job and rejects `queue: max` with `cancel-in-progress: true` as a platform
  validation error. It also rejects a `queue` key on a pre-mutation job. A three-arrival fixture
  proves all mutation contenders wait in arrival order without cancellation; a pre-mutation fixture
  proves a newer run cancels stale cancellation-safe work with `cancel-in-progress: true`.
- ADR 0072 convergence fixtures prove each five-condition pair-gate requirement, including a Run
  Invocation URI comparison that ignores the attempt component. They accept a same-`run_id`
  sidecar-present, primary-absent retry that uploads the primary, reject a primary-present,
  sidecar-absent state as `foreign-conflict`, and reject wrong digest, signer, source identity,
  run-id, subject name, or primary binding without mutation. A valid re-run-failed-jobs attempt with
  the same `run_id` converges as `committed-as-expected` without another primary upload and reports
  custody unproven unless it has a matching asset-ID receipt. Additional rejection fixtures cover an
  unreadable digest reaching `indeterminate`, a new `run_id` encountering matching pre-existing
  content, and deletion of a `starter` asset not proven to belong to the same run.
- Permission fixtures distinguish static caller-YAML lint failures, the only pre-mutation permission
  check, from first-mutation classification: missing or excessive declared permissions fail lint; a
  definitive `403` on the first mutating call fails with
  `windlass.verify.error.mutation-permission-denied`, without read-back or further mutation; and an
  ambiguous upload result requires ADR 0067 read-back and fails with
  `windlass.verify.error.publisher-indeterminate-primary-upload` when classification remains
  unresolved.
- Linked artifact metadata fixtures prove that the distinct metadata surface is outside the
  release-upload mutation segment, uses `run_id` as its idempotency key, and requires an exact
  complete-record-identity match for same-run convergence. They accept one same-`run_id` matching
  record after the canonical 5-second, 24-observation, 120-second polling bound; reject a new
  `run_id`, a partial tuple match, a differing tuple member, or duplicate matching records as
  `foreign-conflict`; and reject unreadable, incomplete, contradictory, or polling-bound-exhausted
  metadata observations as `indeterminate` without another metadata mutation.
