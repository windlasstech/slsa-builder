# Common SLSA Provenance v1 Contract

This document defines the shared SLSA provenance v1 contract used by producer profiles.
Profile-specific rules such as subject naming and `externalParameters` are layered on top of this
common contract.

- Source ADRs: [0002](../decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md),
  [0003](../decisions/0003-use-thin-core-with-profile-owned-reusable-workflows.md),
  [0028](../decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md),
  [0029](../decisions/0029-use-windlass-generated-slsa-provenance-for-npm-publish.md),
  [0035](../decisions/0035-use-actions-attest-as-initial-sigstore-signing-adapter.md),
  [0037](../decisions/0037-define-initial-verification-deliverables.md),
  [0042](../decisions/0042-use-acquired-domains-for-buildtype-uris.md),
  [0055](../decisions/0055-use-actions-attest-custom-mode-for-statement-construction.md)
- Related specs: [Core profile contract](core-profile-contract.md),
  [Identity and build types](identity-and-buildtypes.md),
  [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md),
  [Release manifest](release-manifest.md)

## Scope and non-goals

**In scope:**

- Common in-toto Statement shape.
- Common SLSA provenance v1 predicate shape.
- Shared subject and digest rules.
- Shared fields such as `builder.id`, `buildType`, and `externalParameters`.
- The role of the signing adapter.
- Common verifier rejection criteria.

**Out of scope:**

- Profile-specific `externalParameters` schemas (profile specs).
- Profile-specific subject naming (profile specs).
- Publisher verification policy (publisher and verification specs).
- Release manifest JSON schema (release manifest spec).

## in-toto Statement shape

Every SLSA provenance emitted by a producer profile must be wrapped in an in-toto Statement with the
following shape:

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "<profile-specific subject name>",
      "digest": {
        "sha256": "<lowercase hex>",
        "sha512": "<optional lowercase hex>"
      }
    }
  ],
  "predicateType": "https://slsa.dev/provenance/v1",
  "predicate": {
    "buildDefinition": {
      "buildType": "<canonical buildType URI>",
      "externalParameters": {},
      "internalParameters": {},
      "resolvedDependencies": []
    },
    "runDetails": {
      "builder": {
        "id": "<full GitHub reusable workflow URI with SHA>",
        "version": null,
        "builderDependencies": []
      },
      "metadata": {
        "invocationId": "<github.run_id>.<github.run_attempt>",
        "startedOn": "<ISO 8601 timestamp>",
        "finishedOn": "<ISO 8601 timestamp>"
      }
    }
  }
}
```

### `_type`

Must be exactly `https://in-toto.io/Statement/v1`. Any other value is rejected.

### `predicateType`

Must be exactly `https://slsa.dev/provenance/v1`. Any other value is rejected.

## Subject rules

- The initial production producer profiles must emit exactly one `subject` entry.
- That single entry is the primary artifact and is addressed as `subject[0]` by verifier policy.
- `subject[0].name` must be the canonical name defined by the profile.
- `subject[0].digest` must include at least `sha256`.
- Additional digest algorithms may be present if the profile requires them (for example, npm tarball
  `sha512`).
- The digest value must be lowercase hexadecimal without a prefix.
- Checksum files, SBOMs, and provenance sidecars must not appear in `subject[0].digest`.
- Checksum files, SBOMs, provenance sidecars, and secondary artifacts must not be added as
  additional `subject` entries unless a later ADR and profile spec define their semantics. Verifiers
  for the initial production profiles must reject Statements with zero subjects or more than one
  subject.

## Digest encoding

- All digests in provenance use lowercase hex unless a profile explicitly defines another encoding.
- A profile may define an additional tool-boundary representation, such as `sha256:<hex>`, for
  handoff between jobs or external tools, but the provenance itself must use lowercase hex.
- `sha512` may be recorded in the digest map when the profile requires it.

## `builder.id`

See the [identity and build types spec](identity-and-buildtypes.md). Must be the full GitHub
reusable workflow URI with the full commit SHA.

## `buildType`

See the [identity and build types spec](identity-and-buildtypes.md). Must be a canonical
`https://buildtype.dev/windlass/slsa-builder/...` URI.

## `externalParameters`

`externalParameters` must be complete and verifier-relevant. Every field that a verifier needs to
decide whether the build is trustworthy must be present and explicit.

Common field groups recorded by producer profiles include:

- `source`:
  - `repository`: source repository URI.
  - `ref`: source ref, for example `refs/tags/v1.2.3`.
  - `revision`: immutable source revision, such as a Git commit SHA.
- `workflow`:
  - `path`: reusable workflow file path.
  - `sha`: full commit SHA.
  - `builder_id`: the builder identity derived from the path and SHA.
- `inputs`: caller-provided inputs that affect the build output.
- `runtime`:
  - `runner`: `ubuntu-24.04`.
- `package_manager`: name, actual version, and selection source when the profile uses a package
  manager.

A verifier must reject unexpected `externalParameters` fields when the policy requires strict
matching. The profile spec must define the complete schema and the strict-matching policy.

For the initial JS/TS npm package profile, the complete normative schema is defined in
[JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md). The common contract is not a
fallback schema and must not be used to accept fields omitted by the profile schema.

## `internalParameters`

`internalParameters` may contain non-verifier-relevant metadata that does not affect trust
decisions. If a field affects trust, it must be in `externalParameters`.

## `resolvedDependencies`

The profile may record resolved dependencies when they are available and verifier-relevant. For the
JS/TS npm profile, this includes the installed package versions and lockfile information.

## `runDetails`

### `builder`

- `id`: full GitHub reusable workflow URI with SHA.
- `version`: reserved for future use; must be `null` in the initial profile.
- `builderDependencies`: reserved for future use; must be an empty array in the initial profile.

### `metadata`

- `invocationId`: `<github.run_id>.<github.run_attempt>`.
- `startedOn`: ISO 8601 timestamp.
- `finishedOn`: ISO 8601 timestamp.

## Signing adapter role

The initial signing adapter is stock `actions/attest` invoked in custom attestation mode. Windlass
generates and validates the subject inputs, `predicateType`, and SLSA provenance predicate before
invoking the adapter. `actions/attest` constructs the in-toto Statement from those inputs and
produces a Sigstore-backed bundle.

The adapter must not define the Statement's meaning. The producer-side verification gate must
extract the signed bundle payload before publication and prove that the emitted Statement has:

- `_type: https://in-toto.io/Statement/v1`.
- The Windlass-verified subject entries.
- `predicateType: https://slsa.dev/provenance/v1`.
- The Windlass-generated SLSA provenance predicate.
- The expected `builder.id`, `buildType`, and `externalParameters` values.

The initial stock `actions/attest` adapter must not be documented or invoked as if it accepted a
complete in-toto Statement payload. A future adapter that signs complete Statement bytes directly
requires a later ADR if it changes verifier-visible behavior or trust boundaries.

## Signed bundle file format

The initial production signed provenance artifact is the exact Sigstore bundle file emitted by the
full-SHA-pinned `actions/attest` invocation. The bundle bytes are verifier inputs and must be
preserved byte-for-byte across job handoff, npm `--provenance-file` submission, and GitHub Release
sidecar redistribution.

The `.intoto.jsonl` filename suffix used by npm, release sidecars, and release manifest assets is a
distribution naming convention. It does not mean that Windlass may replace the signed bundle with a
raw in-toto Statement, a reserialized DSSE envelope, extracted predicate JSON, or any other
normalized representation.

Implementations must not alter signed bundle bytes after `actions/attest` emits them. In particular,
they must not:

- extract the in-toto Statement and store only that Statement as the provenance file;
- reserialize, pretty-print, compact, wrap, unwrap, or reorder the bundle JSON;
- regenerate DSSE envelopes or Sigstore bundle metadata;
- re-sign the bundle in publisher or upload jobs; or
- treat GitHub artifact attestation storage metadata as a substitute for the bundle bytes when a
  bundle file is required.

Producer-side and consumer-side verifiers must parse the preserved bundle bytes, extract the signed
in-toto Statement payload, verify the bundle signature and signer identity, and compare the
extracted Statement fields against the expected subject inputs, `predicateType`, and predicate.
Verification must fail when the preserved bundle bytes cannot be parsed, the signature is invalid,
the extracted Statement does not match the expected contract, or a sidecar/downloaded bundle is not
byte-for-byte the same bundle that the producer emitted and the publisher verified.

## Common verifier rejection matrix

A verifier must reject provenance if any of the following are true:

| Condition                                               | Rejection reason                  |
| ------------------------------------------------------- | --------------------------------- |
| `_type` is not `https://in-toto.io/Statement/v1`        | Wrong statement type              |
| `predicateType` is not `https://slsa.dev/provenance/v1` | Wrong predicate type              |
| Signature is missing or invalid                         | Signature mismatch                |
| Signer identity is not trusted                          | Signer mismatch                   |
| `builder.id` uses a branch, tag, or short SHA           | Builder identity policy violation |
| `buildType` is not in the canonical namespace           | Build type policy violation       |
| `externalParameters` is incomplete                      | Incomplete parameters             |
| `externalParameters` contains unexpected fields         | Strict matching violation         |
| `subject` contains zero or multiple entries             | Subject cardinality error         |
| `subject[0].digest.sha256` is missing                   | Missing required digest           |
| `subject[0].name` does not match the profile rule       | Subject name mismatch             |
| Digest encoding is not lowercase hex                    | Digest encoding error             |
| Sidecar, SBOM, or checksum is in `subject[0].digest`    | Subject digest scope error        |
| Emitted Statement differs from validated signing inputs | Statement assembly mismatch       |

## Failure behavior

If the trusted core or a profile produces signing inputs that would not result in a Statement
matching this contract, the producer-side verification gate must reject the bundle before
publication.

If a verifier receives a bundle that violates this contract, the verification must fail with a clear
error category from the rejection matrix above.

## TDD and fixtures

- Golden Statement fixture with valid `_type`, `predicateType`, `builder.id`, `buildType`, and
  `subject`.
- Rejected fixtures for each row in the rejection matrix, including zero-subject and multi-subject
  Statements.
- A fixture proving that the signing adapter payload matches the Statement implied by the
  Windlass-verified subject inputs, predicate type, and predicate.
