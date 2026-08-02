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
  [0054](../decisions/0054-use-slsa-builder-dev-release-manifest-predicate-uri.md),
  [0062](../decisions/0062-intersect-trusted-producer-policies.md),
  [0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [0067](../decisions/0067-converge-repeated-runs-within-run-identity.md),
  [0068](../decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md),
  [0069](../decisions/0069-require-rekor-transparency-and-govern-sigstore-trust-root.md)
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

The initial production contract supports release runs from SemVer version tags only. Repository tag
protection and rulesets are required release-process controls, but consumer-side manifest
verification checks the signed bundle identity and expected full tag ref rather than attempting to
prove historical GitHub tag protection state offline. A production invocation must run on a Git tag
ref whose short tag is `v<release_version>`, where `release_version` is the SemVer 2.0.0 version
without the leading `v` recorded in the manifest.

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

The release tag target is resolved by recursively peeling annotated tags to a terminal Git object.
The terminal object must be a commit. The generator and verifier must reject a missing tag, a tag
object cycle, an annotated tag chain whose terminal object is not a commit, or any resolved commit
that differs from `release_commit_sha`. Lightweight tags are already terminal commit refs and are
accepted only when all other protected tag and release checks pass.

### Manifest entry ordering

Because the signed manifest digest is computed over the manifest JSON value and JSON arrays preserve
order, array ordering is part of the signed contract. This comparator detail specifies the ordering
contract already established by ADR 0031 and does not introduce locale-dependent behavior.

- `producer_profiles` must be sorted by the exact `profile` field value in ascending Unicode code
  point order; producer generation fails before canonicalization or signing, and verification
  rejects the manifest, when the array is not sorted by this comparator.
- `publisher_workflows` must be sorted by the exact `publisher` field value using the same
  comparator; producer generation fails before canonicalization or signing, and verification rejects
  the manifest, when the array is not sorted by this comparator.
- The comparator compares each string from its first differing Unicode code point, with the lower
  code point sorting first; if one string is an exact prefix of the other, the shorter string sorts
  first. For valid Unicode strings this is equivalent to comparing their UTF-8 byte sequences as
  unsigned bytes.
- Comparison is not locale-aware and applies no case folding or Unicode normalization. In
  particular, neither NFC nor NFD normalization is applied: the byte-exact field value is compared
  as supplied.
- Duplicate `producer_profiles[].profile` or `publisher_workflows[].publisher` values are invalid;
  producer generation fails before signing and verification rejects the manifest on a duplicate.
- Producers must emit the arrays in sorted order before canonicalization; a producer that cannot
  apply the comparator fails before canonicalization and signing rather than emitting a manifest.
- Verifiers must reject a manifest whose arrays are not sorted, even if sorting them locally would
  produce an otherwise trusted mapping; verification does not repair producer output.

The following comparator-only fixture is valid for either ordering field. Uppercase `Z` (U+005A)
sorts before lowercase `a` (U+0061), which distinguishes this contract from locale-sensitive orders:

```json
["Alpha", "Zulu", "alpha", "éclair"]
```

The following fixture is invalid because `alpha` appears before `Zulu`; producers fail before
signing and verifiers reject the containing manifest:

```json
["Alpha", "alpha", "Zulu", "éclair"]
```

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

Every production release publishes exactly two release manifest assets to the GitHub Release:

1. **Plain JSON manifest** — human-readable and machine-parseable, but not a trust root by itself.
2. **Signed Sigstore bundle** — the canonical trust root. The bundle payload is an in-toto Statement
   whose `predicate` is the same JSON value as the plain JSON manifest. The trust digest is computed
   from that value using RFC 8785 JSON Canonicalization Scheme (JCS), as defined below.

These are the only release assets produced by the release manifest workflow. The workflow also uses
additional same-run GitHub Actions artifacts as internal handoff material between jobs; those
internal artifacts are not release assets and are not public distribution artifacts.

### Release asset names

The release manifest release assets have the following fixed filenames, derived from the
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

These release asset names are fixed for the initial release manifest schema version.

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
  example `2026-07-07T12:00:00Z`. It is the manifest generation job's UTC wall-clock time captured
  once after all release identity inputs have been validated and before canonicalization. It is
  diagnostic release metadata, not a reproducibility input; rerunning the same release after cleanup
  may produce a different timestamp while every trust mapping remains identical.
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
  Verifiers must reject leap seconds, timezone offsets, subsecond precision, non-UTC timestamps, and
  timestamps that are not exactly representable in that lexical form.
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

The signed release manifest bundle file is the exact byte sequence emitted by `actions/attest`. The
workflow must not replace it with the extracted Statement, reserialize it, wrap it in another DSSE
or Sigstore representation, or re-sign it in the upload job. The `.intoto.jsonl` filename is the
release manifest bundle naming convention, not permission to change the bundle byte format. See the
[SLSA provenance v1 signed bundle file format](slsa-provenance-v1.md#signed-bundle-file-format).

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

`manifest-generate` and `manifest-sign` are PRE-mutation jobs. Each must declare job-level
concurrency with `cancel-in-progress: true`; a workflow that omits that declaration, sets it to
`false`, or places either job in the mutation concurrency group must fail static workflow
conformance before release use.

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

The initial handoff from `manifest-generate` to `manifest-sign` contains these same-run GitHub
Actions artifact handles and digests. These are internal workflow artifacts used only for job
handoff. They are distinct from the two release manifest assets published to the GitHub Release.
Each row maps to the core same-run artifact handoff schema with `transport: github-actions-artifact`
and `digest.algorithm: sha256`:

| Handoff payload         | `artifact_name` output             | `payload_file_name`                             | `payload_kind`               | `digest.value` output           |
| ----------------------- | ---------------------------------- | ----------------------------------------------- | ---------------------------- | ------------------------------- |
| Plain manifest JSON     | `manifest-json-artifact-name`      | `release-manifest-<version>.json`               | `release-manifest`           | `manifest-json-sha256`          |
| Manifest predicate JSON | `manifest-predicate-artifact-name` | `release-manifest-<version>.predicate.json`     | `release-manifest-predicate` | `manifest-predicate-sha256`     |
| Signing input metadata  | `manifest-signing-input-name`      | `release-manifest-<version>.signing-input.json` | `signing-input-metadata`     | `manifest-signing-input-sha256` |

The manifest predicate JSON must parse to the same JSON value as the plain manifest JSON. The
signing input metadata is transport metadata only; the manifest JSON value and the digest above
remain the trust inputs.

The signing input metadata payload must be a closed JSON object with this shape:

```json
{
  "schema_version": "1",
  "release_identity": {
    "source_repository": "https://github.com/windlasstech/slsa-builder",
    "release_version": "1.2.3",
    "release_tag": "refs/tags/v1.2.3",
    "release_commit_sha": "e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5"
  },
  "subject": {
    "name": "release-manifest-1.2.3.json",
    "digest": {
      "sha256": "<lowercase hex digest of the canonical manifest JSON bytes>"
    }
  },
  "predicate_type": "https://slsa-builder.dev/predicates/release-manifest/v1",
  "predicate_artifact": {
    "artifact_name": "release-manifest-1.2.3-predicate",
    "payload_file_name": "release-manifest-1.2.3.predicate.json",
    "sha256": "<lowercase hex digest of the predicate artifact bytes>",
    "canonical_manifest_sha256": "<same value as subject.digest.sha256>"
  },
  "manifest_artifact": {
    "artifact_name": "release-manifest-1.2.3-json",
    "payload_file_name": "release-manifest-1.2.3.json",
    "sha256": "<lowercase hex digest of the plain manifest file bytes>",
    "canonical_manifest_sha256": "<same value as subject.digest.sha256>"
  },
  "signing_input_artifact": {
    "artifact_name": "release-manifest-1.2.3-signing-input",
    "payload_file_name": "release-manifest-1.2.3.signing-input.json"
  }
}
```

Normative signing input metadata rules:

- The schema is closed. Unknown top-level fields, unknown nested fields, duplicate object member
  names, or missing required fields are invalid.
- `subject.name` must be `release-manifest-<version>.json` and must match the plain manifest
  artifact payload filename.
- `subject.digest.sha256`, `predicate_artifact.canonical_manifest_sha256`, and
  `manifest_artifact.canonical_manifest_sha256` must all equal the SHA-256 digest of the RFC 8785
  JCS canonical manifest JSON bytes.
- `predicate_type` must be `https://slsa-builder.dev/predicates/release-manifest/v1`.
- The predicate artifact bytes must parse to the same JSON value as the plain manifest JSON. Its
  `sha256` field is the digest of those artifact bytes, while `canonical_manifest_sha256` binds the
  predicate content to the canonical manifest value that becomes the Statement subject digest.
- `release_identity` must exactly match the corresponding fields inside the manifest JSON value.
- Every artifact handle must name the same-run artifact that carried the corresponding payload to
  `manifest-sign`; handles, filenames, and byte digests are checked before invoking the signing
  adapter.
- `manifest-sign` must reject the handoff before signing when the signing input metadata does not
  bind the same subject name, canonical manifest digest, predicate type, predicate content, release
  identity, and artifact handles that it verified from the downloaded handoff artifacts.

The internal handoff basenames above are fixed for schema version `1`. A receiving job must reject
an artifact whose sole file has a different basename, even when the file contents have the expected
digest, because the basename is part of the closed same-run handoff contract and protects later jobs
from accidentally consuming stale or misrouted artifacts.

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

### Manifest publication concurrency (ADR 0066)

`manifest-upload` is the manifest-publish job and belongs to the mutation segment defined by
ADR 0066. It must declare job-level concurrency with `cancel-in-progress: false`; a workflow lacking
that declaration fails conformance review and must not be used for production manifest publication.
The concurrency mechanism must therefore queue a contender instead of cancelling a manifest
publication in flight; a configuration that can cancel the running mutation job fails conformance
review and is not a valid production workflow.

The concurrency group represents one release intent and is shared by mutation jobs for that intent.
The exact mutation concurrency group is:

```text
release-mutation-${{ github.repository }}-${{ github.ref_name }}
```

It consists only of the common literal namespace, caller-scoped `github.repository`, and
`github.ref_name`. The namespace distinguishes the mutation segment from PRE-mutation job groups but
is identical across release mutation jobs so jobs within the same release intent use the same gate.
Any other component fails conformance review and must not publish.

The group key must never include `github.workflow`; a workflow that includes it fails conformance
review and must not publish. In a called reusable workflow that value resolves to the caller's
workflow name, so using it can collide with caller-level concurrency and trigger the
self-cancellation trap that ADR 0066 forbids.

After acquiring the mutation group and before its first release lookup or upload that can mutate
remote state, `manifest-upload` must revalidate the release tag and target release, the handoff
digests and fixed asset names, and the duplicate/convergence state specified below. Failure to
revalidate, or any failed or indeterminate precondition, fails the job before its first mutating
call. Checks performed before the job waited for the group are not substitutes for this
mutation-segment entry revalidation.

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
- Under ADR 0067, a manifest artifact with the same name may be adopted only by a retry attempt of
  the same `run_id` after the semantic and signer binding below succeeds. A new `run_id` or a failed
  binding must fail before upload rather than overwrite, delete, repair, or adopt the artifact.
- If the primary manifest upload succeeds but the bundle upload fails, the job must fail clearly
  without deleting or clobbering the primary manifest. The failure output must make the partial
  state explicit.

### Outcome classification and same-run convergence (ADR 0067)

Before acting, and again after a mutating call whose result is ambiguous, the manifest publication
step must classify the remote pair into exactly one of these states; inability to produce exactly
one state fails the step as `indeterminate` without another mutating call:

- `committed-as-expected`: both same-name assets exist, and their content, signed binding, and run
  identity satisfy the comparison procedure below;
- `absent`: neither same-name asset exists;
- `foreign-conflict`: one or both same-name assets exist, but the pair is incomplete, belongs to a
  different `run_id`, or fails the required content or signed binding;
- `indeterminate`: the workflow cannot determine existence or complete the required download,
  parsing, digest, signature, identity, or comparison checks through the authenticated release asset
  surface.

The state machine must upload both candidate assets only from `absent`; encountering any other state
before a first-run or new-`run_id` upload fails before mutation. After an ambiguous mutating call,
or within a retry attempt of the same `run_id`, `committed-as-expected` satisfies the step without
another upload. `foreign-conflict` and `indeterminate` always fail closed, naming the state and the
available remote evidence, without uploading, deleting, replacing, regenerating, or re-signing an
asset.

Run identity is the idempotency key. A retry with the same `github.run_id` may converge, including
when `github.run_attempt` has increased; a new `github.run_id` remains a new release intent and must
fail closed on either pre-existing manifest asset. Failure to prove from the verified existing
bundle identity that its signed run identity equals the current `github.run_id` classifies the pair
as `foreign-conflict` when the mismatch is proved, or `indeterminate` when the identity cannot be
read or verified. Re-run failed jobs is the supported recovery surface. Re-run all jobs does not
relax any binding or state rule; if it encounters existing assets, it can converge only under the
same same-`run_id` procedure.

For same-`run_id` convergence, the publication step must perform this procedure in order; failure at
any step produces the state specified here and prevents mutation:

1. Download both existing release assets through the authenticated GitHub release asset surface. If
   only one asset exists, classify `foreign-conflict`; if existence or bytes cannot be determined,
   classify `indeterminate`.
2. Strictly parse the existing plain manifest and the current candidate manifest, rejecting
   duplicate JSON member names, unknown fields, schema violations, and invalid ordering. A proved
   validation failure classifies `foreign-conflict`; an unreadable value classifies `indeterminate`.
3. Create comparison copies of both parsed manifest values and remove the top-level `generated_at`
   member from each copy. Remove no other field, nested member, or array element; do not normalize
   or otherwise transform string values. Failure to isolate exactly that one field classifies
   `indeterminate`.
4. Serialize both comparison copies with RFC 8785 JCS and compare the resulting byte sequences for
   exact equality. Unequal bytes classify `foreign-conflict`; serialization failure classifies
   `indeterminate`.
5. Verify the existing bundle under the signed payload rules in this spec, including that its
   Statement predicate equals the complete existing plain manifest value with the existing
   `generated_at`, its subject digest binds that complete value, and its verified run identity
   equals the current `github.run_id`. A proved mismatch classifies `foreign-conflict`; inability to
   complete verification classifies `indeterminate`.

When all five steps succeed, the result is `committed-as-expected`. The publication step must adopt
the first commit's existing plain manifest and bundle as the committed artifacts, including the
existing manifest's `generated_at`, complete JSON value, asset bytes, and digests; failure to use
those existing values fails the step before reporting success. The retry's newly generated manifest
and bundle are discarded as candidates and must not be uploaded or substituted. Thus `generated_at`
is the only field excluded from semantic comparison, but the adopted signed artifact continues to
bind the first commit's original `generated_at`.

If the GitHub API request or network transport becomes ambiguous after upload starts, the upload job
must repeat the same classification procedure before reporting failure; if it cannot do so, it
reports `indeterminate` and performs no further mutation. The lookup may use a GitHub API digest
field only when it explicitly identifies release asset bytes with SHA-256. Otherwise it must
download the candidate bytes through the same authenticated surface and recompute SHA-256 locally;
failure to download or hash produces `indeterminate`. Asset IDs, browser URLs, filenames, sizes,
content types, release notes, logs, and workflow artifact names are not digest proof and using one
as proof fails the step as `indeterminate`.

The observable result is reported through `manifest-upload-result` using exactly the four state
names above. An auxiliary `manifest-report` job must run with `if: always()` and emit a
machine-readable report containing the final state and available evidence even when an earlier job
fails or is cancelled; failure to emit the report is a workflow failure and the missing report is
never trusted as evidence of `absent` or success. The report is diagnostic and must not be accepted
as trusted input by another run; a consumer that attempts to use it as a convergence binding must
fail closed.

A partial pair, including a plain manifest whose bundle upload failed, is `foreign-conflict`, not a
verified publication. The upload job must not delete, replace, clobber, re-sign, regenerate, or
repair the uploaded JSON manifest; violating this rule fails the workflow and invalidates the
publication result. Operators must resolve partial or indeterminate release state outside the
production manifest workflow.

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
- release tag: the full tag ref recorded in `release_tag`;
- predicate type: `https://slsa-builder.dev/predicates/release-manifest/v1`.

The certificate identity must be the GitHub Actions workflow identity for the signer workflow path
and tag ref above. A bundle signed by another repository, another workflow path, a branch ref, a
pull request ref, a short SHA ref, or a non-GitHub OIDC issuer must be rejected.

The release manifest verifier must bind the semantic identity fields from the common SLSA provenance
contract to the release manifest values as follows:

| Semantic field             | Required release manifest value                                                                                |
| -------------------------- | -------------------------------------------------------------------------------------------------------------- |
| OIDC issuer                | GitHub Actions.                                                                                                |
| Signer workflow repository | `windlasstech/slsa-builder`.                                                                                   |
| Signer workflow path       | `.github/workflows/release-manifest.yml`.                                                                      |
| Signer workflow ref        | Full tag ref `refs/tags/v<release_version>`, equal to the manifest `release_tag`.                              |
| Signer workflow SHA        | When exposed, the full commit SHA reached by recursively peeling `release_tag`, equal to `release_commit_sha`. |
| Source repository          | `https://github.com/windlasstech/slsa-builder`.                                                                |
| Source ref                 | Full release tag ref equal to `release_tag`.                                                                   |
| Source revision            | Full 40-character lowercase commit SHA equal to `release_commit_sha`.                                          |
| Predicate type             | `https://slsa-builder.dev/predicates/release-manifest/v1`.                                                     |

Common GitHub/Sigstore verification outputs may expose signer workflow identity through reusable
workflow or job-workflow claim names and source identity through repository/ref claim names. The
claim spelling is tool-specific; the semantic values above are the policy. If the verifier cannot
prove every required semantic identity field from verified certificate or bundle data, it must
reject the release manifest. It must not infer manifest signer identity from release asset names,
tag names alone, release notes, workflow outputs, or unsigned JSON files.

Consumer verifiers do not need to prove from the signed bundle that GitHub tag protection was
enabled at the time of signing. They must verify the full tag-ref equality and tag peel rules above.
Online checks of GitHub repository rulesets, tag protection, or immutable release evidence are
complementary release-process evidence unless a later verifier policy defines them as required
policy inputs.

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
6. The `release_commit_sha` matches the tag after recursively peeling annotated tags to a terminal
   commit.
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

When both a trusted release manifest and explicit verifier policy are present, the manifest producer
profile mappings are additional constraints, not overrides. The effective trusted producer policy is
the intersection of the verified manifest mappings and explicit verifier policy. A conflict or empty
intersection must fail verification rather than selecting one source by precedence.

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
- The release tag cannot be peeled to a terminal commit or peels to a commit that differs from
  `release_commit_sha`.
- A manifest artifact with the same name already exists and the current attempt is a new `run_id`,
  or a same-`run_id` retry cannot classify the complete existing pair as `committed-as-expected`.
- A handoff artifact contains a file whose basename differs from the fixed schema version `1`
  basename for that payload kind.
- Signing input metadata is missing, has unknown or duplicate fields, or does not bind the same
  subject name, canonical manifest digest, predicate type, predicate content, release identity, and
  artifact handles verified from the handoff.
- The upload job cannot determine whether a started plain manifest upload committed remotely.
- The signing adapter cannot produce a valid bundle.
- The upload job lacks the required `contents: write` permission for release asset upload.
- The upload job has prohibited signing authority, including `id-token: write`,
  `attestations: write`, signing credentials, or permission to regenerate signed manifest contents.
- The manifest-publish job lacks mutation-class concurrency with `cancel-in-progress: false`, uses a
  prohibited group component such as `github.workflow`, or does not revalidate preconditions after
  entering the mutation segment.

## TDD and fixtures

- Golden signed manifest fixture with valid predicate, subject, producer profile mappings, and
  publisher workflow mappings.
- Rejected fixtures for wrong signer, wrong predicate type, the superseded
  `buildtype.dev/windlass/slsa-builder/release-manifest/v1` predicate URI, wrong schema version,
  wrong workflow SHA, mismatched builder/buildType, publisher entry with buildType, non-canonical
  RFC 8785 JCS manifest digest, malformed `generated_at`, tag peel failure, wrong internal handoff
  basename, malformed or mismatched signing input metadata, Statement predicate JSON value mismatch,
  a new-`run_id` duplicate asset upload, and an `indeterminate` manifest publication.
- Comparator fixtures containing the valid order `["Alpha", "Zulu", "alpha", "éclair"]` and the
  rejected order `["Alpha", "alpha", "Zulu", "éclair"]`, proving code-point order without locale,
  case-folding, or Unicode normalization.
- An ADR 0066 concurrency fixture with two runs for the same repository and ref: the first
  manifest-publish job remains running without cancellation, the second waits on the shared mutation
  group, and the second revalidates at segment entry and fails closed on the first run's committed
  state.
- A valid ADR 0067 convergence fixture in which a later attempt of the same `run_id` generates a
  candidate differing only in `generated_at`, obtains equal RFC 8785 JCS bytes after removing
  exactly that field, verifies the existing bundle's same-run binding, and adopts the first commit's
  complete manifest and bundle as `committed-as-expected` without upload or re-signing.
- An invalid ADR 0067 convergence fixture in which a same-`run_id` candidate differs semantically in
  `workflow_sha`; unequal RFC 8785 JCS comparison bytes produce `foreign-conflict`, and no asset is
  uploaded, replaced, deleted, or adopted.
- A fixture proving that `manifest-upload` cannot re-sign or mutate the manifest.
