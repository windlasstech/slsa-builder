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

## Canonical artifacts

Every release produces two manifest artifacts:

1. **Plain JSON manifest** — human-readable and machine-parseable, but not a trust root by itself.
2. **Signed Sigstore bundle** — the canonical trust root. The bundle payload is an in-toto Statement
   whose `predicate` is byte-for-byte equivalent to the plain JSON manifest after the canonical JSON
   serialization rules below are applied.

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
      "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$"
    },
    "source_repository": { "const": "https://github.com/windlasstech/slsa-builder" },
    "release_tag": {
      "type": "string",
      "pattern": "^refs/tags/v[0-9]+\\.[0-9]+\\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$"
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

The verifier must additionally check cross-field constraints that JSON Schema cannot express here:

- `release_tag` must equal `refs/tags/v<release_version>`.
- Each `producer_profiles[].builder_id` must equal
  `https://github.com/windlasstech/slsa-builder/<workflow_path>@<workflow_sha>`.
- `producer_profiles[]` must not contain duplicate `profile` values.
- `publisher_workflows[]` must not contain duplicate `publisher` values.
- A publisher workflow entry must not contain `builder_id`, `build_type`, or any other field that
  claims source-to-artifact provenance for the publisher.

Invalid examples include a manifest with a short workflow SHA, a release tag that does not match
`release_version`, a producer entry whose `builder_id` SHA differs from `workflow_sha`, a publisher
entry containing `build_type`, or any unknown top-level field such as `notes`.

### Canonical JSON serialization

The release manifest digest and the signed Statement predicate are computed from the canonical JSON
serialization of the manifest object. The canonical form is JSON Canonicalization Scheme (JCS, RFC
8785): object members are sorted lexicographically by code point, insignificant whitespace is
omitted, strings use JSON escaping as defined by JCS, and numbers are serialized using JCS rules.

The initial schema uses only strings, arrays, and objects, so release manifest producers must not
add numeric, boolean, or null fields unless a later schema version defines their semantics. Arrays
retain their declared order; duplicate `producer_profiles[].profile` or
`publisher_workflows[].publisher` entries are invalid before canonicalization.

The plain JSON file published for humans may be pretty-printed, but it must parse to the same JSON
value as the canonical manifest object. The trust digest is always over the canonical JSON bytes,
not over the pretty-printed file bytes. A verifier must parse the plain JSON, canonicalize it, and
compare that canonical digest with the Statement subject digest and Statement predicate value.

### Field semantics

- `schema_version`: manifest schema version. Initial value is `"1"`.
- `release_version`: SemVer release version.
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
- Each `producer_profiles[]` `build_type` must be a canonical producer `buildType` URI.
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

The signing adapter signs the in-toto Statement bytes, not the plain JSON file directly. The
Statement's `predicate` must be exactly the canonical manifest JSON value represented as a JSON
object. The Statement subject digest must be the SHA-256 digest of the canonical manifest JSON
bytes.

The signed bundle is invalid when:

- the DSSE payload is not an in-toto Statement;
- the Statement `predicateType` is not `https://slsa-builder.dev/predicates/release-manifest/v1`;
- the Statement `predicate` does not equal the canonical manifest JSON value;
- the Statement `subject[0].name` is not `release-manifest-<version>.json`;
- the Statement `subject[0].digest.sha256` does not equal the canonical manifest JSON SHA-256; or
- the plain JSON manifest published alongside the bundle canonicalizes to different bytes.

## Three-job signing boundary

The release manifest is produced through three primary jobs:

```text
manifest-generate -> manifest-sign -> manifest-upload
```

### `manifest-generate`

- Creates the unsigned release manifest JSON and the in-toto Statement payload.
- Computes the manifest digest.
- Uploads the unsigned manifest and Statement material as workflow artifacts.
- Exposes the expected digest and artifact handles to the signing job.
- Must **not** have:
  - `id-token: write`
  - `attestations: write`
  - `contents: write`
  - release mutation authority
  - long-lived signing credentials

### `manifest-sign`

- Downloads the manifest artifacts.
- Recomputes their digests and verifies them against the handoff from `manifest-generate`.
- Signs the in-toto Statement as a Sigstore DSSE bundle using full-SHA-pinned `actions/attest` in
  custom predicate mode.
- Uploads the signed bundle as a workflow artifact.
- Permissions:
  - `contents: read`
  - `id-token: write`
  - `attestations: write`
- Must **not** have `contents: write` or release mutation authority.

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

## Handoff rules

- Every handoff between jobs must include an expected digest.
- The receiving job must recompute the digest and fail closed on mismatch.
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
- The Statement predicate does not equal the canonical manifest JSON value.
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
  manifest digest, Statement predicate mismatch, and duplicate asset upload.
- A fixture proving that `manifest-upload` cannot re-sign or mutate the manifest.
