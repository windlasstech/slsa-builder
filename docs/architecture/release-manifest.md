# Signed Release Manifest And Three-Job Signing Boundary

This document defines the Windlass release manifest: the machine-verifiable mapping from
human-readable release versions to the exact workflow SHAs, producer `builder.id` values, producer
`buildType` URIs, and publisher workflow SHAs that verifiers should trust. It also defines the
three-job signing boundary that produces and publishes the manifest.

- Source ADRs: [0028](../decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md),
  [0031](../decisions/0031-use-sigstore-signed-in-toto-release-manifest.md),
  [0035](../decisions/0035-use-actions-attest-as-initial-sigstore-signing-adapter.md),
  [0042](../decisions/0042-use-acquired-domains-for-buildtype-uris.md),
  [0053](../decisions/0053-use-three-job-release-manifest-signing-boundary.md),
  [0054](../decisions/0054-use-slsa-builder-dev-release-manifest-predicate-uri.md)
- Related specs: [Identity and build types](identity-and-buildtypes.md),
  [SLSA provenance v1](slsa-provenance-v1.md), [Core profile contract](core-profile-contract.md)

## Scope and non-goals

**In scope:**

- Release manifest JSON schema.
- in-toto Statement and predicate type for the manifest.
- Three-job signing/upload boundary.
- Job permissions and handoff rules.
- Verifier trust roots and verification commands.
- Migration criteria for future signing models.

**Out of scope:**

- Consumer verifier implementation (verification policy spec).
- Profile-specific `externalParameters` (profile specs).
- Exact predicate or bundle format details already covered by `actions/attest` documentation
  (referenced but not duplicated).

## Release manifest purpose

The release manifest is the canonical trust root that maps a Windlass release version to the
concrete reusable workflow SHAs that consumers may trust. It is signed as a Sigstore-backed DSSE
bundle so that verifiers can authenticate it without relying on GitHub UI state, release notes, or
unsigned JSON files.

## Workflow entrypoint and public contract

The production release manifest workflow entrypoint is:

```text
.github/workflows/release-manifest.yml
```

The initial production contract supports release runs from protected SemVer version tags only. A
production invocation must run on a Git tag ref whose short tag is `v<release_version>`, where
`release_version` is the SemVer 2.0.0 version without the leading `v` recorded in the manifest.

The workflow must not expose public inputs that let callers override the release version, release
tag, release commit SHA, producer workflow SHA, publisher workflow SHA, `builder.id`, `buildType`,
predicate type, signer workflow path, or manifest artifact names. Those values are derived from the
checked-out release tag, repository contents, release manifest generation policy, and the trusted
workflow files in the tagged `slsa-builder` release. A future release process that needs
caller-supplied profile mappings or cross-repository release metadata requires a later ADR or schema
version.

The workflow may expose only non-trust-changing operational inputs, such as a dry-run flag or an
explicit target release repository, if a later section defines their validation and proves they do
not change the signed manifest value. Until such inputs are specified, the production workflow
contract has no caller-controlled inputs.

## Release manifest generation policy

The initial schema version `1` release manifest is generated only from the checked-out
`windlasstech/slsa-builder` repository at the protected release tag. It must not discover trusted
profiles by globbing arbitrary workflow files, reading caller-supplied allowlists, consuming release
notes, or accepting workflow SHA mappings from workflow inputs.

For schema version `1`, every trusted workflow entry records the same immutable Git commit as the
release tag target:

- `release_commit_sha` is the full 40-character lowercase commit SHA resolved from `release_tag`.
- Every `producer_profiles[].workflow_sha` must equal `release_commit_sha`.
- Every `publisher_workflows[].workflow_sha` must equal `release_commit_sha`.
- Every `producer_profiles[].builder_id` must be derived from that entry's `workflow_path` and
  `workflow_sha`.

The schema version `1` manifest includes only the production profiles and publishers explicitly
specified by the architecture docs for the release. The initial required entries are:

| Array                 | Name                             | Workflow path                                        |
| --------------------- | -------------------------------- | ---------------------------------------------------- |
| `producer_profiles`   | `js-ts-npm-package`              | `.github/workflows/js-ts-npm-package-slsa3.yml`      |
| `publisher_workflows` | `github-release-asset-publisher` | `.github/workflows/github-release-asset-publish.yml` |

The release manifest generator must fail before signing when an expected workflow file is missing
from the tagged checkout, when any generated workflow SHA differs from `release_commit_sha`, when a
`builder_id` does not match the generated path and SHA, or when the generator attempts to include an
unknown producer or publisher entry without a later schema version or ADR-backed spec that admits
it.

### Manifest entry ordering

Because the signed manifest digest is computed over the manifest JSON value and JSON arrays preserve
order, array ordering is part of the signed contract.

- `producer_profiles` must be sorted by `profile` in lexicographic ascending order.
- `publisher_workflows` must be sorted by `publisher` in lexicographic ascending order.
- Duplicate `producer_profiles[].profile` or `publisher_workflows[].publisher` values are invalid.
- Producers must emit the arrays in sorted order before canonicalization.
- Verifiers must reject a manifest whose arrays are not sorted, even if sorting them locally would
  produce an otherwise trusted mapping.

### Supported trigger and runtime guards

The release manifest workflow must fail before manifest signing or GitHub Release mutation when any
of the following is true:

- `github.ref_type` is not `tag`.
- `github.ref` is not `refs/tags/v<release_version>` for the generated manifest's `release_version`.
- The tag name is not a valid `v`-prefixed SemVer 2.0.0 version.
- The tag does not exist in the `windlasstech/slsa-builder` repository.
- The target GitHub Release for the tag does not already exist.
- The resolved tag target commit does not equal `release_commit_sha`.
- The workflow path observed in the signer identity is not `.github/workflows/release-manifest.yml`.
- The signer workflow ref is not the same protected release tag recorded in `release_tag`.
- Any generated producer or publisher workflow path/SHA mapping is caller-supplied rather than
  derived from the tagged repository contents and release manifest generation policy.

Branch runs, pull request runs, untagged manual dispatch runs, short tag names passed as inputs, and
caller-supplied workflow SHA allowlists are not production release manifest invocations. They may be
used only for tests or dry runs that do not upload a signed bundle claiming the production predicate
type.

## Canonical artifacts

Every release produces two manifest artifacts:

1. **Plain JSON manifest** — human-readable and machine-parseable, but not a trust root by itself.
2. **Signed Sigstore bundle** — the canonical trust root. The bundle payload is an in-toto Statement
   whose `predicate` is the same JSON value as the plain JSON manifest. The trust digest is computed
   from that value using RFC 8785 JSON Canonicalization Scheme (JCS), as defined below.

### Artifact names

The release manifest artifacts have the following fixed filenames, derived from the
`release_version` value:

```text
release-manifest-<version>.json
release-manifest-<version>.intoto.jsonl
```

For a release version `1.2.3`, the filenames are:

```text
release-manifest-1.2.3.json
release-manifest-1.2.3.intoto.jsonl
```

These names are fixed for the initial release manifest schema version.

## Release manifest JSON schema

The following JSON document is an example manifest payload for schema version `1`:

```json
{
  "schema_version": "1",
  "release_version": "1.2.3",
  "source_repository": "https://github.com/windlasstech/slsa-builder",
  "release_tag": "refs/tags/v1.2.3",
  "release_commit_sha": "e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
  "generated_at": "2026-07-07T12:00:00Z",
  "producer_profiles": [
    {
      "profile": "js-ts-npm-package",
      "workflow_path": ".github/workflows/js-ts-npm-package-slsa3.yml",
      "workflow_sha": "e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
      "builder_id": "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
      "build_type": "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1"
    }
  ],
  "publisher_workflows": [
    {
      "publisher": "github-release-asset-publisher",
      "workflow_path": ".github/workflows/github-release-asset-publish.yml",
      "workflow_sha": "e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
      "role": "verified-distributor"
    }
  ]
}
```

### Normative JSON schema

The release manifest payload must conform to this schema. The schema is intentionally closed:
unknown top-level fields and unknown entry fields are invalid.

```json
{
  "type": "object",
  "required": [
    "schema_version",
    "release_version",
    "source_repository",
    "release_tag",
    "release_commit_sha",
    "generated_at",
    "producer_profiles",
    "publisher_workflows"
  ],
  "additionalProperties": false,
  "properties": {
    "schema_version": { "const": "1" },
    "release_version": {
      "type": "string",
      "pattern": "^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\\+[0-9A-Za-z-]+(?:\\.[0-9A-Za-z-]+)*)?$"
    },
    "source_repository": { "const": "https://github.com/windlasstech/slsa-builder" },
    "release_tag": {
      "type": "string",
      "pattern": "^refs/tags/v(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\\+[0-9A-Za-z-]+(?:\\.[0-9A-Za-z-]+)*)?$"
    },
    "release_commit_sha": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
    "generated_at": {
      "type": "string",
      "pattern": "^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"
    },
    "producer_profiles": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["profile", "workflow_path", "workflow_sha", "builder_id", "build_type"],
        "additionalProperties": false,
        "properties": {
          "profile": { "type": "string", "minLength": 1 },
          "workflow_path": { "type": "string", "pattern": "^\\.github/workflows/[^/]+\\.ya?ml$" },
          "workflow_sha": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
          "builder_id": {
            "type": "string",
            "pattern": "^https://github.com/windlasstech/slsa-builder/\\.github/workflows/[^@]+@[0-9a-f]{40}$"
          },
          "build_type": {
            "type": "string",
            "pattern": "^https://buildtype.dev/windlass/slsa-builder/.+/v[0-9]+$"
          }
        }
      }
    },
    "publisher_workflows": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["publisher", "workflow_path", "workflow_sha", "role"],
        "additionalProperties": false,
        "properties": {
          "publisher": { "type": "string", "minLength": 1 },
          "workflow_path": { "type": "string", "pattern": "^\\.github/workflows/[^/]+\\.ya?ml$" },
          "workflow_sha": { "type": "string", "pattern": "^[0-9a-f]{40}$" },
          "role": { "const": "verified-distributor" }
        }
      }
    }
  }
}
```

The JSON Schema above is a structural validation gate, not a complete trust policy. A verifier must
not accept a release manifest only because it passes JSON Schema validation. After schema
validation, the verifier must additionally check cross-field and policy constraints that JSON Schema
cannot express here:

- `release_tag` must equal `refs/tags/v<release_version>`.
- Each `producer_profiles[].builder_id` must equal
  `https://github.com/windlasstech/slsa-builder/<workflow_path>@<workflow_sha>`.
- Each `producer_profiles[].build_type` must equal the canonical producer `buildType` declared by
  the architecture spec for that `producer_profiles[].profile` value or by an explicit verifier
  policy that admits the producer profile.
- A producer profile that is unknown to the verifier policy must be rejected even when its
  `build_type` matches the general `buildtype.dev` URI pattern.
- `producer_profiles[]` must not contain duplicate `profile` values.
- `publisher_workflows[]` must not contain duplicate `publisher` values.
- A publisher workflow entry must not contain `builder_id`, `build_type`, or any other field that
  claims source-to-artifact provenance for the publisher.

Invalid examples include a manifest with a short workflow SHA, a release tag that does not match
`release_version`, a producer entry whose `builder_id` SHA differs from `workflow_sha`, a producer
entry whose `build_type` is not the canonical build type for its `profile`, a producer profile that
the verifier policy does not recognize, a publisher entry containing `build_type`, or any unknown
top-level field such as `notes`.

### RFC 8785 JCS canonical JSON serialization

The release manifest digest and the signed Statement predicate are computed from the canonical JSON
serialization of the manifest object. The canonical form is JSON Canonicalization Scheme (JCS, RFC
8785): object members are sorted lexicographically by code point, insignificant whitespace is
omitted, strings use JSON escaping as defined by JCS, and numbers are serialized using JCS rules.

The initial schema uses only strings, arrays, and objects, so release manifest producers must not
add numeric, boolean, or null fields unless a later schema version defines their semantics. Arrays
retain their declared order; duplicate `producer_profiles[].profile` or
`publisher_workflows[].publisher` entries are invalid before canonicalization.

The plain JSON file published for humans may be pretty-printed, but it must parse to the same JSON
value as the Statement `predicate`. The trust digest is always over the RFC 8785 JCS canonical JSON
bytes for that manifest value, not over the pretty-printed file bytes or the raw Statement payload
bytes. A verifier must parse the plain JSON manifest and the Statement `predicate` as JSON values,
compare those values for equality, canonicalize the agreed value with RFC 8785 JCS, and compare the
resulting SHA-256 digest with the Statement subject digest.

### Field semantics

- `schema_version`: manifest schema version. Initial value is `"1"`.
- `release_version`: SemVer 2.0.0 release version without a leading `v`; prerelease and build
  metadata are allowed.
- `source_repository`: canonical source repository URI.
- `release_tag`: exact Git tag ref, for example `refs/tags/v1.2.3`.
- `release_commit_sha`: commit SHA that the tag points to.
- `generated_at`: ISO 8601 UTC timestamp in the fixed lexical form `YYYY-MM-DDTHH:mm:ssZ`, for
  example `2026-07-07T12:00:00Z`.
- `producer_profiles`: array of source-to-artifact producer entries. Each entry maps a profile name
  to the trusted workflow SHA, `builder.id`, and `buildType`.
- `publisher_workflows`: array of publisher workflow entries. Each entry maps a publisher name to a
  trusted workflow path and SHA. Publisher entries must not include `builder_id` or `build_type` in
  the default production path.

### Normative schema rules

- `schema_version` must be the string `"1"`.
- `release_version` must be a SemVer version string without the leading `v`.
- `source_repository` must be `https://github.com/windlasstech/slsa-builder`.
- `release_tag` must be `refs/tags/v<release_version>`.
- `release_commit_sha` and all workflow SHA fields must be full 40-character lowercase Git commit
  SHAs.
- `generated_at` must match `^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$` exactly.
  Verifiers may parse it with standard ISO 8601 timestamp parsers after this lexical validation.
- `producer_profiles` must contain at least the `js-ts-npm-package` entry for a release that ships
  the npm producer workflow.
- Each `producer_profiles[]` entry must include `profile`, `workflow_path`, `workflow_sha`,
  `builder_id`, and `build_type`.
- Each `producer_profiles[]` `builder_id` must be derived from its `workflow_path` and
  `workflow_sha`.
- Each `producer_profiles[]` `build_type` must be a canonical producer `buildType` URI for the same
  producer profile. The schema pattern only checks the URI family; the verifier policy must check
  the profile-to-buildType mapping.
- Each `publisher_workflows[]` entry must include `publisher`, `workflow_path`, `workflow_sha`, and
  `role`.
- The initial publisher `role` value is `verified-distributor`.
- `publisher_workflows[]` entries must not contain `builder_id`, `build_type`, or any field that
  claims source-to-artifact provenance for the publisher.
- Unknown top-level fields and unknown entry fields must be rejected by strict release manifest
  verification.

## in-toto Statement for the manifest

The signed manifest is wrapped in an in-toto Statement with the following shape:

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "release-manifest-1.2.3.json",
      "digest": {
        "sha256": "<lowercase hex digest of the canonical manifest JSON bytes>"
      }
    }
  ],
  "predicateType": "https://slsa-builder.dev/predicates/release-manifest/v1",
  "predicate": {
    "schema_version": "1",
    "release_version": "1.2.3",
    "source_repository": "https://github.com/windlasstech/slsa-builder",
    "release_tag": "refs/tags/v1.2.3",
    "release_commit_sha": "e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
    "generated_at": "2026-07-07T12:00:00Z",
    "producer_profiles": [
      {
        "profile": "js-ts-npm-package",
        "workflow_path": ".github/workflows/js-ts-npm-package-slsa3.yml",
        "workflow_sha": "e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
        "builder_id": "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
        "build_type": "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1"
      }
    ],
    "publisher_workflows": [
      {
        "publisher": "github-release-asset-publisher",
        "workflow_path": ".github/workflows/github-release-asset-publish.yml",
        "workflow_sha": "e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
        "role": "verified-distributor"
      }
    ]
  }
}
```

## Predicate type

The predicate type for the release manifest is:

```text
https://slsa-builder.dev/predicates/release-manifest/v1
```

The `buildtype.dev` namespace is reserved for producer `buildType` identifiers. The superseded draft
URI `https://buildtype.dev/windlass/slsa-builder/release-manifest/v1` is not an equivalent predicate
identifier and must be rejected when a release manifest predicate is expected.

## Signed payload rules

The initial signing adapter is stock `actions/attest` in custom attestation mode. The manifest
signing job must pass the verified subject name, subject digest, predicate type, and manifest
predicate JSON to the adapter; it must not pass or document a complete in-toto Statement payload as
an adapter input. The adapter constructs the in-toto Statement and signs that Statement as a
Sigstore-backed bundle.

The Statement's `predicate` must be the manifest JSON value, represented as a JSON object. The
Statement subject digest must be the SHA-256 digest of the RFC 8785 JCS canonical JSON bytes for
that value. After signing, the manifest signing or upload path must extract the emitted Statement
payload from the signed bundle and compare it against the Statement implied by the verified subject
inputs, predicate type, and manifest predicate JSON.

The signed bundle is invalid when:

- the DSSE payload is not an in-toto Statement;
- the Statement `predicateType` is not `https://slsa-builder.dev/predicates/release-manifest/v1`;
- the Statement `predicate` JSON value does not equal the plain manifest JSON value;
- the Statement `subject[0].name` is not `release-manifest-<version>.json`;
- the Statement `subject[0].digest.sha256` does not equal the RFC 8785 JCS canonical manifest JSON
  SHA-256; or
- the plain JSON manifest published alongside the bundle canonicalizes with RFC 8785 JCS to a
  different digest.

## Three-job signing boundary

The release manifest is produced through three primary jobs:

```text
manifest-generate -> manifest-sign -> manifest-upload
```

### `manifest-generate`

- Creates the unsigned release manifest JSON, manifest predicate JSON, and signing input metadata.
- Computes the manifest digest.
- Uploads the unsigned manifest, predicate JSON, and signing input material as workflow artifacts.
- Exposes the expected digest and artifact handles to the signing job.
- Must **not** have:
  - `id-token: write`
  - `attestations: write`
  - `contents: write`
  - release mutation authority
  - long-lived signing credentials

The initial handoff from `manifest-generate` to `manifest-sign` contains these same-run artifact
handles and digests. Each row maps to the core same-run artifact handoff schema with
`transport: github-actions-artifact` and `digest.algorithm: sha256`:

| Handoff payload         | `artifact_name` output             | `payload_file_name`                          | `payload_kind`               | `digest.value` output           |
| ----------------------- | ---------------------------------- | -------------------------------------------- | ---------------------------- | ------------------------------- |
| Plain manifest JSON     | `manifest-json-artifact-name`      | `release-manifest-<version>.json`            | `release-manifest`           | `manifest-json-sha256`          |
| Manifest predicate JSON | `manifest-predicate-artifact-name` | Profile-defined manifest predicate basename. | `release-manifest-predicate` | `manifest-predicate-sha256`     |
| Signing input metadata  | `manifest-signing-input-name`      | Profile-defined signing input JSON basename. | `signing-input-metadata`     | `manifest-signing-input-sha256` |

The manifest predicate JSON must parse to the same JSON value as the plain manifest JSON. The
signing input metadata is transport metadata only; the manifest JSON value and the digest above
remain the trust inputs.

### `manifest-sign`

- Downloads the manifest artifacts.
- Recomputes their digests and verifies them against the handoff from `manifest-generate`.
- Invokes full-SHA-pinned `actions/attest` in custom attestation mode with the verified subject
  name, subject digest, predicate type, and manifest predicate JSON.
- Extracts the emitted in-toto Statement from the signed bundle and verifies that it matches the
  Statement implied by the verified signing inputs.
- Uploads the signed bundle as a workflow artifact.
- Re-exports the verified plain manifest artifact handle and canonical manifest digest to
  `manifest-upload` without modifying the manifest JSON bytes or canonical JSON value.
- Permissions:
  - `contents: read`
  - `id-token: write`
  - `attestations: write`
- Must **not** have `contents: write` or release mutation authority.

The handoff from `manifest-sign` to `manifest-upload` contains only values verified or produced by
`manifest-sign`. Each row maps to the core same-run artifact handoff schema with
`transport: github-actions-artifact` and `digest.algorithm: sha256`:

| Handoff payload     | `artifact_name` output          | `payload_file_name`                       | `payload_kind`      | `digest.value` output    |
| ------------------- | ------------------------------- | ----------------------------------------- | ------------------- | ------------------------ |
| Plain manifest JSON | `manifest-json-artifact-name`   | `release-manifest-<version>.json`         | `release-manifest`  | `manifest-json-sha256`   |
| Signed bundle       | `manifest-bundle-artifact-name` | `release-manifest-<version>.intoto.jsonl` | `provenance-bundle` | `manifest-bundle-sha256` |

`manifest-sign` must fail before producing the upload handoff if the plain manifest, predicate JSON,
signing input metadata, emitted Statement, or signed bundle does not match the verified signing
inputs.

### `manifest-upload`

- Downloads the unsigned manifest and signed bundle.
- Recomputes their digests and verifies them against the handoff from `manifest-sign`.
- Uploads both artifacts to the selected existing GitHub Release.
- Permissions:
  - `contents: write` (only on this job)
- Must **not** have:
  - `id-token: write`
  - `attestations: write`
  - signing credentials
  - authority to create or modify signed metadata

`manifest-upload` must use `manifest-sign` as its only trusted handoff source. It must not consume a
direct `manifest-generate` handoff, reconstruct artifact names from release version strings, or use
release notes, logs, local files, or caller inputs as substitutes for the `manifest-sign` outputs.
It may download the same GitHub Actions artifact originally uploaded by `manifest-generate` only
through the artifact handle re-exported by `manifest-sign`.

## Handoff rules

- Every handoff between jobs must include an expected digest.
- The receiving job must recompute the digest and fail closed on mismatch.
- Every handoff row above must satisfy the core handoff schema, including `transport`,
  `payload_file_name`, `payload_kind`, `digest.algorithm`, and `digest.value`.
- Artifact handles, job outputs, logs, and release notes must not be treated as substitutes for
  digest verification.
- The upload job must fail rather than re-signing, regenerating, or mutating manifest contents after
  signing.
- The upload job must have `contents: write` because it mutates an existing GitHub Release, and it
  must not have signing authority. Missing release upload authority and excessive signing authority
  are distinct failures.

## Release upload behavior

- Upload targets an existing GitHub Release identified by the release tag.
- If the release or tag does not exist, the upload job must fail.
- If a manifest artifact with the same name already exists, the upload job must fail rather than
  overwrite.
- If the primary manifest upload succeeds but the bundle upload fails, the job must fail clearly
  without deleting or clobbering the primary manifest. The failure output must make the partial
  state explicit.

The observable manifest upload result is reported through `manifest-upload-result`:

- `completed`: the plain JSON manifest and signed bundle were both uploaded.
- `failed-before-upload`: validation, digest verification, duplicate preflight, or target lookup
  failed before the plain JSON manifest was uploaded.
- `partial-json-uploaded`: the plain JSON manifest upload succeeded, but signed bundle upload
  failed.

When `manifest-upload-result` is `partial-json-uploaded`, the workflow fails, the plain JSON
manifest may already exist on the target release, and the signed release manifest bundle is absent.
Because the signed bundle is the canonical trust root, this state must not be reported as a verified
release manifest publication. The upload job must not delete, replace, clobber, re-sign, or
regenerate the uploaded JSON manifest. A duplicate plain JSON manifest or duplicate signed bundle
detected during preflight is `failed-before-upload`, not a partial upload state.

## Complementary evidence

Plain JSON manifests, checksum files, release notes, GPG-signed annotated tags, GitHub immutable
release evidence, and release-level attestations are valuable complementary evidence, but they do
not replace the signed release manifest bundle as the canonical version-to-SHA trust root.

## Verifier trust roots

### Release manifest signer identity

The initial release manifest signer identity is the GitHub Actions OIDC identity for the Windlass
release workflow in this repository. Verifiers must check all of the following signer constraints:

- repository: `windlasstech/slsa-builder`;
- signer workflow path: `.github/workflows/release-manifest.yml`;
- signer workflow ref: `refs/tags/v<release_version>`;
- source repository URI: `https://github.com/windlasstech/slsa-builder`;
- release tag: the protected tag recorded in `release_tag`;
- predicate type: `https://slsa-builder.dev/predicates/release-manifest/v1`.

The certificate identity must be the GitHub Actions workflow identity for the signer workflow path
and tag ref above. A bundle signed by another repository, another workflow path, a branch ref, a
pull request ref, a short SHA ref, or a non-GitHub OIDC issuer must be rejected.

The signer workflow path is fixed for the initial release manifest contract. If the project later
renames the release workflow or moves release manifest signing into a reusable workflow, direct
Sigstore tool, or TUF metadata process, that change must preserve or explicitly replace this signer
identity contract through a later ADR or schema version.

A release manifest verifier must check:

1. The bundle signature is valid.
2. The signer identity matches the release manifest signer identity above.
3. The predicate type is `https://slsa-builder.dev/predicates/release-manifest/v1`.
4. The schema version is supported.
5. The `release_tag` matches the expected tag.
6. The `release_commit_sha` matches the tag.
7. Each producer profile entry maps to the expected `workflow_sha`, `builder_id`, and `build_type`.
8. Each publisher workflow entry maps to the expected `workflow_path`, `workflow_sha`, and `role`,
   and does not claim a `builder_id` or `build_type`.

## Reference verification command

A verifier may use the following command as a starting point. It is not a complete Windlass policy
verifier.

```bash
gh attestation verify release-manifest-1.2.3.intoto.jsonl \
  --owner windlasstech \
  --predicate-type https://slsa-builder.dev/predicates/release-manifest/v1
```

The full Windlass verifier must also check the manifest schema version, release commit SHA, producer
profile mappings, publisher workflow mappings, and producer `builder.id`/`buildType` values against
a trusted release manifest or explicit policy.

## Migration criteria

A future ADR may replace the three-job inline workflow with one of the following. Any migration must
preserve the verifier-visible trust contract for predicate type, schema version, signer identity,
release version, workflow path, workflow SHA, `builder.id`, and `buildType`.

- A dedicated reusable workflow for release manifest signing.
- Direct Sigstore tooling such as `cosign` or `sigstore-go`.
- TUF-style metadata with delegated roles, thresholds, expiry, or mirrors.

## Failure behavior

The release manifest workflow must fail before any mutation when:

- The manifest artifacts cannot be retrieved.
- A computed digest does not match the handoff digest.
- The Statement predicate JSON value does not equal the plain manifest JSON value.
- The release tag or target release does not exist.
- A manifest artifact with the same name already exists.
- The signing adapter cannot produce a valid bundle.
- The upload job lacks the required `contents: write` permission for release asset upload.
- The upload job has prohibited signing authority, including `id-token: write`,
  `attestations: write`, signing credentials, or permission to regenerate signed manifest contents.

## TDD and fixtures

- Golden signed manifest fixture with valid predicate, subject, producer profile mappings, and
  publisher workflow mappings.
- Rejected fixtures for wrong signer, wrong predicate type, the superseded
  `buildtype.dev/windlass/slsa-builder/release-manifest/v1` predicate URI, wrong schema version,
  wrong workflow SHA, mismatched builder/buildType, publisher entry with buildType, non-canonical
  RFC 8785 JCS manifest digest, Statement predicate JSON value mismatch, and duplicate asset upload.
- A fixture proving that `manifest-upload` cannot re-sign or mutate the manifest.
