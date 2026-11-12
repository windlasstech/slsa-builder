# Composed Workflow Internal Handoff Contract

This document defines the internal same-run contract used when a producer profile and the GitHub
Release asset publisher are composed inside one workflow graph.

- Source ADRs: [0036](../decisions/0036-use-three-job-digest-verified-publish-graph.md),
  [0050](../decisions/0050-define-producer-to-publisher-handoff-contract.md),
  [0052](../decisions/0052-compose-npm-package-tarball-producer-with-release-asset-publisher.md),
  [0057](../decisions/0057-provide-composed-public-npm-release-asset-workflow.md),
  [0058](../decisions/0058-define-github-release-asset-publisher-authority-boundary.md),
  [0059](../decisions/0059-define-public-npm-release-composed-workflow-interface.md),
  [0060](../decisions/0060-unify-npm-profile-public-entrypoint-with-release-asset-mode.md),
  [0062](../decisions/0062-intersect-trusted-producer-policies.md),
  [0064](../decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md),
  [0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [0067](../decisions/0067-converge-repeated-runs-within-run-identity.md),
  [0105](../decisions/0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md),
  [0106](../decisions/0106-consolidate-external-source-parameters-onto-workflow-and-descriptor-bindings.md),
  [0108](../decisions/0108-remove-redundant-npm-release-asset-inputs.md), and
  [0109](../decisions/0109-minimize-external-parameters-to-actual-interface-inputs.md)
- Related specs: [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md),
  [npm-to-release-asset composition](npm-to-release-asset-composition.md),
  [Core profile contract](core-profile-contract.md)

## Scope and non-goals

**In scope:**

- Same-workflow-run internal handoff from a producer to a composition mapping job.
- Producer-owned delivery of internal artifact names and provenance bundle SHA-256.
- Digest verification for the handoff manifest before publisher inputs are constructed.
- Offline package-manifest evidence transport and binding for producer-policy checks.
- Rejection behavior when the composed graph cannot prove producer ownership of handoff values.

**Out of scope:**

- Public `workflow_call.outputs` for standalone producer workflows.
- Cross-run or cross-workflow composition.
- Caller-provided artifact names, file paths, URLs, or raw-byte handoffs.
- Generic raw release asset upload without accepted producer provenance.

## Composition boundary

The initial composed workflow contract is internal to one GitHub Actions workflow run. The producer
and publisher must be connected through same-run jobs and same-run GitHub Actions artifacts. The
contract is not a stable public API between separately invoked reusable workflows.

The standalone npm producer workflow's public `workflow_call.outputs` remain limited to package
identity and tarball digest handles. A composed workflow that needs publisher inputs must not infer
publisher handoff values from those public outputs alone. It must consume the producer-owned handoff
manifest defined below.

The verified handoff supplies the expected primary and sidecar names and digests, producer build
type, and source identity needed by the composed release-state and policy preflight. Remote release
state is time-sensitive and is not handoff evidence: the composed workflow must read it directly
before composed npm mutation, and the publisher must read it again at publisher mutation-segment
entry. The later publisher read is required even when the earlier preflight passed.

The handoff also carries package-manifest evidence for offline producer-policy checks.

## Producer-owned handoff manifest

After the producer has created the primary artifact and signed producer provenance bundle, it must
write a handoff manifest JSON file and upload it as a same-run GitHub Actions artifact.

The manifest carries producer expectations only. Its closed schema does not carry `draft`,
`immutable`, observed release asset state, or a release-state preflight result. Those remote values
can become stale between jobs, so they must be obtained through the direct reads at the two points
defined in [Composition boundary](#composition-boundary), not accepted from the handoff.

The handoff manifest artifact name is deterministic for the initial npm composition:

```text
js-ts-npm-composition-handoff-<github.run_id>-<github.run_attempt>
```

The artifact must contain exactly one file:

```text
composition-handoff.json
```

The producer job must compute the SHA-256 digest of the `composition-handoff.json` bytes and pass
the handoff manifest artifact name and digest to the composition mapping job through internal
same-run job outputs. These internal job outputs are producer-owned delivery channels for the
composed graph; they must not be exposed as standalone producer `workflow_call.outputs` unless a
later public composition API is specified.

The composition handoff manifest artifact itself must satisfy the core same-run artifact handoff
schema with `transport: github-actions-artifact`, `payload_file_name: composition-handoff.json`,
`payload_kind: composition-handoff`, `digest.algorithm: sha256`, and `digest.value` equal to the
SHA-256 of the `composition-handoff.json` bytes.

## Manifest schema

The handoff manifest JSON object must use this closed schema shape. Unknown fields, including
injected release-state or preflight-result fields, are invalid.

```json
{
  "schema_version": "1",
  "producer_profile": "js-ts-npm-package",
  "primary_artifact": {
    "artifact_name": "js-ts-npm-package-tarball-123456789-1",
    "payload_file_name": "windlass-slsa-builder-1.2.3.tgz",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "producer_provenance": {
    "artifact_name": "js-ts-npm-provenance-bundle-123456789-1",
    "payload_file_name": "windlass-slsa-builder-1.2.3.tgz.intoto.jsonl",
    "sha256": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
  },
  "manifest_evidence": {
    "path": "package.json",
    "content_base64": "eyJuYW1lIjoiZXhhbXBsZSIsInZlcnNpb24iOiIxLjIuMyJ9"
  },
  "trusted_producer": {
    "builder_id": "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@0123456789abcdef0123456789abcdef01234567",
    "build_type": "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1",
    "source_repository": "https://github.com/example/project",
    "source_revision": "fedcba9876543210fedcba9876543210fedcba98"
  },
  "subject": {
    "name": "pkg:npm/%40windlass/slsa-builder@1.2.3",
    "sha512": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "release": {
    "tag": "refs/tags/v1.2.3",
    "final_asset_name": "windlass-slsa-builder-1.2.3.tgz"
  },
  "native_provenance_locators": [
    {
      "type": "github-artifact-attestation",
      "url": "https://github.com/example/project/attestations/123"
    }
  ],
  "linked_artifact_settings": {
    "enabled": false
  }
}
```

The example illustrates the closed transport shape, not a complete conformance fixture. A conforming
instance must satisfy every cross-check below against the actual producer provenance, selected
manifest, and artifact bytes.

### Closed member enumeration

The manifest and every object it contains are closed. A conforming manifest must contain exactly the
required members and, where applicable, only the optional members enumerated below. Missing required
members, `null` in place of a member value, and any unlisted member are invalid.

| Object                                               | Required members                                                                                                                               | Optional members                                         |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Manifest root                                        | `schema_version`, `producer_profile`, `primary_artifact`, `producer_provenance`, `manifest_evidence`, `trusted_producer`, `subject`, `release` | `native_provenance_locators`, `linked_artifact_settings` |
| `primary_artifact`                                   | `artifact_name`, `payload_file_name`, `sha256`                                                                                                 | None                                                     |
| `producer_provenance`                                | `artifact_name`, `payload_file_name`, `sha256`                                                                                                 | None                                                     |
| `manifest_evidence`                                  | `path`, `content_base64`                                                                                                                       | None                                                     |
| `trusted_producer`                                   | `builder_id`, `build_type`, `source_repository`, `source_revision`                                                                             | None                                                     |
| `subject`                                            | `name`, `sha512`, `sha256`                                                                                                                     | None                                                     |
| `release`                                            | `tag`, `final_asset_name`                                                                                                                      | None                                                     |
| Each `native_provenance_locators` item               | `type`, `url`                                                                                                                                  | `digest`                                                 |
| `linked_artifact_settings` when `enabled` is `false` | `enabled`                                                                                                                                      | None                                                     |
| `linked_artifact_settings` when `enabled` is `true`  | `enabled`, `version`, `repository`, `registry_url`                                                                                             | None                                                     |

`native_provenance_locators` must be an array, and `linked_artifact_settings` must be an object.
Their values and the conditional `linked_artifact_settings` member rules must conform to the
[publisher contract](github-release-asset-publisher.md#native-producer-provenance-locators) and its
[linked artifact storage opt-in](github-release-asset-publisher.md#linked-artifact-storage-opt-in).

### Field rules

- `schema_version` must be `"1"`.
- `producer_profile` must identify the producer profile that owns the artifact and provenance. The
  initial value is `js-ts-npm-package`.
- `primary_artifact.artifact_name` must be a same-run GitHub Actions artifact uploaded by the
  producer job and containing exactly one payload file.
- `primary_artifact.payload_file_name` must be the basename of the pack-produced tarball. For the
  initial npm composition it must end in `.tgz`.
- `primary_artifact.sha256` must be the lowercase SHA-256 digest of the primary artifact bytes.
  Together with `primary_artifact.artifact_name` and `primary_artifact.payload_file_name`, it maps
  to a core handoff object with `transport: github-actions-artifact`,
  `payload_kind: primary-artifact`, `digest.algorithm: sha256`, and
  `digest.value: primary_artifact.sha256`.
- `producer_provenance.artifact_name` must be a same-run GitHub Actions artifact uploaded by the
  producer signing job and containing exactly one signed producer provenance bundle file.
- `producer_provenance.payload_file_name` is the provenance bundle file basename. It is transport
  metadata only; the publisher sidecar name remains derived from the final asset name by the
  publisher contract.
- The sidecar keeps its deterministic filename and exact verified producer bundle bytes.
- No `provenance-sidecar` input or handoff member exists.
- Release-asset mode itself makes sidecar publication mandatory.
- `producer_provenance.sha256` is the producer-owned SHA-256 digest of the signed bundle bytes. This
  value maps to the publisher handoff field `producer-provenance-sha256`. Together with
  `producer_provenance.artifact_name` and `producer_provenance.payload_file_name`, it maps to a core
  handoff object with `transport: github-actions-artifact`, `payload_kind: provenance-bundle`,
  `digest.algorithm: sha256`, and `digest.value: producer_provenance.sha256`.
- `trusted_producer.builder_id` must equal the verified provenance's `runDetails.builder.id`.
- The removed `externalParameters.builder` triplet (`path`, `sha`, `builder_id`) is not its source.
- `trusted_producer.build_type` must equal the verified provenance's `buildDefinition.buildType`.
- It must also match the trusted release manifest or explicit policy.
- `trusted_producer.build_type` must be an exact selector registered by the
  [publisher's closed producer-policy registry](github-release-asset-publisher.md#closed-producer-policy-registry).
  It is a selector and narrowing constraint, not a mechanism to register, extend, or override
  producer policy. In composed mode, an absent or unknown selector must fail before `npm publish`
  with `windlass.verify.error.unregistered-producer-build-type`.
- `trusted_producer.source_repository` is sourced from verified producer provenance.
- It must equal `externalParameters.workflow.repository`.
- Both Git source descriptor URIs must have that same repository component.
- Removed `externalParameters.source.repository` is not a handoff source.
- `trusted_producer.source_revision` is sourced from the `built-source` descriptor.
- It must equal that descriptor's `digest.gitCommit` in verified producer provenance.
- Removed `externalParameters.source.revision` is not a handoff source.
- `manifest_evidence` is required and contains exactly `path` and `content_base64` strings.
  - It must equal the object in the producer's digest-verified `build-metadata.json`.
  - Evidence is the original selected `package.json`, captured before install, build, or pack.
  - Re-read, build-mutated, or packed manifest bytes cannot replace that snapshot.
  - Arrays, duplicate members, missing members, and additional members are invalid.
  - `path` is the repository-root-relative selected manifest path.
  - Use `package.json` for directory `.` and `<package-directory>/package.json` otherwise.
  - Reject absolute, traversing, backslashed, empty, or out-of-directory paths.
  - `content_base64` is canonical standard RFC 4648 base64 with required padding and no whitespace.
  - Reject URL-safe alphabet, noncanonical padding or pad bits, and malformed encodings.
  - Standard padded decoding and re-encoding must reproduce the exact string.
  - Decoded evidence is limited to 16 MiB (16,777,216 bytes).
  - Decoded JSON uses strict bounded manifest parsing and duplicate-member rejection.
  - The publisher uses decoded bytes offline for all manifest-dependent producer-policy checks.
  - This covers package identity, publish configuration, and normalized repository checks.
  - Decoded SHA-256 must equal `package-manifest.digest.sha256`.
  - Encoding `path` under the lockfile rule must reproduce the descriptor URI path fragment.
  - The descriptor's built commit must equal `built-source.digest.gitCommit`.
  - That commit must also equal `trusted_producer.source_revision`.
  - The descriptor repository must equal `workflow.repository`.
  - It must also equal `trusted_producer.source_repository`.
  - Missing, malformed, oversized, or mismatched evidence is rejected.
  - The diagnostic is `windlass.verify.error.package-manifest-evidence`.
  - Network fallback and silent skipping of manifest-dependent checks are forbidden.
- For the initial npm composition, `subject.name` must equal the npm Package URL subject in the
  producer provenance, not `primary_artifact.payload_file_name` or `release.final_asset_name`.
- `subject.sha512` must equal the producer provenance `subject[0].digest.sha512` and the SHA-512 of
  the primary artifact bytes.
- `subject.sha256` must equal `primary_artifact.sha256` and the producer provenance
  `subject[0].digest.sha256`.
- `primary_artifact.payload_file_name` must equal `release.final_asset_name` for the initial npm
  composition.
- `release.tag` must be the full Git tag ref, for example `refs/tags/v1.2.3`, that identifies the
  existing Git tag and target GitHub Release. A short tag name such as `v1.2.3` is invalid in the
  handoff manifest.
- It must equal the effective built ref: `source.ref` when present, otherwise `workflow.ref`.
- It must also equal the `built-source` URI's ref component.
- No `externalParameters.release.*` member supplies or replaces this check.
- The internal handoff tag remains; only the public npm `release-tag` input is removed.
- The standalone publisher's required `release-tag` input and equality checks remain.
- `release.final_asset_name` must be the GitHub Release asset name that the publisher will upload.
- `native_provenance_locators` is optional. When present, it must be an array of locator objects
  that conform to the `native-provenance-locators` schema in the
  [publisher contract](github-release-asset-publisher.md#native-producer-provenance-locators). It
  maps to the publisher handoff field `native-provenance-locators`.
- `linked_artifact_settings` is optional. When present, it must be an object that conforms to the
  `linked-artifact-settings` schema in the
  [publisher contract](github-release-asset-publisher.md#linked-artifact-storage-opt-in). It maps to
  the publisher handoff field `linked-artifact-settings`.
- Optional fields are absent when the producer does not request them. They must not be represented
  as `null`, empty strings, or differently named fields.

## Mapping to publisher inputs

The composition mapping job must verify the handoff manifest digest before reading its fields. It
then maps the manifest to publisher handoff inputs as follows:

| Handoff manifest field               | Publisher handoff field             |
| ------------------------------------ | ----------------------------------- |
| `primary_artifact.artifact_name`     | `primary-artifact-name`             |
| `primary_artifact.sha256`            | `expected-sha256`                   |
| `release.final_asset_name`           | `final-asset-name`                  |
| `release.tag`                        | `release-tag`                       |
| `producer_provenance.artifact_name`  | `producer-provenance-artifact-name` |
| `producer_provenance.sha256`         | `producer-provenance-sha256`        |
| `manifest_evidence`                  | `manifest-evidence`                 |
| `trusted_producer.builder_id`        | `trusted-builder-id`                |
| `trusted_producer.build_type`        | `trusted-build-type`                |
| `subject.name`                       | `expected-subject-name`             |
| `subject.sha256`                     | `expected-subject-sha256`           |
| `trusted_producer.source_repository` | `source-repository`                 |
| `trusted_producer.source_revision`   | `source-revision`                   |
| `native_provenance_locators`         | `native-provenance-locators`        |
| `linked_artifact_settings`           | `linked-artifact-settings`          |

- `source-repository` binds to verified `externalParameters.workflow.repository`.
- `source-revision` binds to verified `built-source.digest.gitCommit`.
- Removed `externalParameters.source.repository` and `source.revision` are not comparison targets.
- The `release.tag` to `release-tag` mapping remains the preserved internal handoff contract.
- Only the public npm workflow's redundant `release-tag` input is removed.

### Complex-field `workflow_call` serialization

When the composition mapping job passes a manifest array or object value through a `workflow_call`
boundary, it must serialize that value as a UTF-8 JSON string. The receiving workflow must parse
that string as JSON before applying the destination field schema. The mapping job must pass
`native_provenance_locators` as the UTF-8 JSON string for the `native-provenance-locators` input and
`linked_artifact_settings` as the UTF-8 JSON string for the `linked-artifact-settings` input. It
must not pass a YAML sequence or mapping, a language-specific object representation, or a coerced
scalar.

`manifest_evidence` is serialized the same way for the required `manifest-evidence` input.

This transport serialization preserves the JSON value across the `workflow_call` boundary. It does
not by itself require canonical JSON bytes. When a downstream rule requires canonical JSON bytes, a
canonical digest, or an ordering key for the transported value, the mapping and receiving workflows
must reject duplicate object member names and use RFC 8785 JCS serialization as defined in the
[canonical JSON serialization rule](verification-policy-and-fixtures.md#canonical-json-serialization).
They must not treat arbitrary transport-string bytes as canonical bytes.

Malformed JSON, duplicate object member names, a value with the wrong JSON type, or a value that
violates the destination closed schema must fail before publisher invocation with
`windlass.verify.error.handoff-schema-mismatch`.

`manifest_evidence` failures instead use `windlass.verify.error.package-manifest-evidence`.

Before composed npm mutation, the mapping job uses the verified expected release asset names and
digests, `trusted_producer.build_type`, and source identity from this mapping for the release-state
and policy preflight. It must read the target release state directly at that point and must not read
or derive it from the handoff. The preflight result is not publisher mutation authorization: at
publisher mutation-segment entry, the publisher must directly re-read and revalidate the target
release state and both expected asset names and digests.

Manifest-dependent policy checks use the authenticated handoff evidence offline.

The mapping job must not use caller-supplied artifact names, deterministic naming alone, public npm
producer outputs, logs, release notes, or raw file paths as substitutes for this manifest. It may
use deterministic names only to retrieve artifacts whose names are also present in the
digest-verified handoff manifest.

## Failure behavior

The composed workflow must fail before invoking the publisher when:

- the handoff manifest artifact name or digest is missing from the producer-owned internal job
  outputs;
- the handoff manifest artifact cannot be retrieved from the same workflow run;
- the manifest artifact contains zero files, more than one file, or a file not named
  `composition-handoff.json`;
- the computed SHA-256 of `composition-handoff.json` differs from the producer-owned internal job
  output digest;
- the JSON is malformed, contains duplicate object member names, has unknown fields, or violates the
  closed schema, including an injected `draft`, `immutable`, observed release asset state, or
  preflight-result field;
- any required artifact name, subject, digest, source identity, builder identity, build type,
  manifest evidence, release tag, or final asset name is missing or malformed;
- `trusted_producer.builder_id` differs from verified `runDetails.builder.id`;
- `trusted_producer.build_type` differs from verified `buildDefinition.buildType`;
- `trusted_producer.source_repository` differs from `externalParameters.workflow.repository`;
- `trusted_producer.source_revision` differs from `built-source.digest.gitCommit`;
- producer provenance carries removed `externalParameters.source.repository` or `source.revision`;
- producer provenance carries removed `source.ref_type` or `source.input_ref`;
- producer provenance carries removed `source.invocation_ref` or `source.invocation_revision`;
- producer provenance carries `externalParameters.release.*` or the removed builder triplet;
- `release.tag` is not a full `refs/tags/<tag-name>` ref;
- `release.tag` differs from the effective built ref or `built-source` URI ref component;
- `manifest_evidence` is missing, malformed, oversized, noncanonical, or mismatched;
- these evidence failures use `windlass.verify.error.package-manifest-evidence`;
- `producer_provenance.sha256` is not a 64-character lowercase hexadecimal SHA-256 digest;
- `trusted_producer.build_type` is absent or does not exactly select the publisher's registered
  producer policy. In composed mode this must fail before `npm publish` with
  `windlass.verify.error.unregistered-producer-build-type`;
- the initial npm composition omits `subject.sha512` or records a malformed subject digest;
- `native_provenance_locators` or `linked_artifact_settings` is present but violates the publisher
  contract schema for the mapped handoff field;
- the npm Package URL subject, provenance subject, or package identity policy do not match;
- `primary_artifact.payload_file_name` and `release.final_asset_name` are not identical; or
- the mapping job attempts to construct publisher inputs from caller-controlled values rather than
  the verified handoff manifest.

Before mutation, the publisher repeats evidence binding and all manifest-dependent checks offline.

Missing, malformed, or mismatched evidence is rejected.

The diagnostic is `windlass.verify.error.package-manifest-evidence`.

No network fetch or silent check skipping is permitted.

## TDD and fixtures

- Positive fixture: a valid npm producer handoff manifest maps to the exact publisher inputs.
- Positive fixture: `manifest-evidence` is a UTF-8 JSON string identical to the metadata object.
- Positive fixture: evidence binds to the producer provenance's `package-manifest` descriptor.
- Positive fixture: standalone and workspace selected-manifest paths are accepted.
- Positive fixture: canonical base64 with applicable padding and valid original bytes is accepted.
- Positive fixture: without `source.ref`, `release.tag` uses `workflow.ref` and `built-source`.
- Positive fixture: a dispatch retry uses `source.ref` and the built, not invocation, commit.
- Both source cases use `workflow.repository` and `built-source.digest.gitCommit`.
- Both source cases bind manifest evidence to that built commit.
- Rejected evidence: missing object, missing members, additional members, or duplicate members.
- Rejected evidence: `null`, wrong JSON types, arrays, or multiple objects.
- Rejected evidence: whitespace or URL-safe base64 alphabet.
- Rejected evidence: noncanonical padding or pad bits and malformed encodings.
- Rejected evidence: decoded content exceeds 16 MiB.
- Rejected evidence: malformed or duplicate-member manifest JSON.
- Rejected evidence: absolute, traversing, backslashed, empty, or out-of-directory paths.
- Rejected evidence: path or descriptor URI path-fragment mismatch.
- Rejected evidence: decoded SHA-256 or descriptor built-commit mismatch.
- Rejected evidence: descriptor repository mismatch.
- Rejected evidence: re-read, build-mutated, or packed bytes.
- Each evidence rejection uses `windlass.verify.error.package-manifest-evidence`.
- Offline test: missing evidence causes no network fetch and no skipped manifest check.
- Rejected source: handoff repository differs from `workflow.repository`.
- Rejected source: handoff revision differs from `built-source.digest.gitCommit`.
- Rejected source: a dispatch retry incorrectly uses `invocation-source.digest.gitCommit`.
- Rejected release: tag differs from the effective built ref or `built-source` URI ref.
- Rejected builder: `trusted_producer.builder_id` differs from `runDetails.builder.id`.
- Rejected provenance: removed `source.repository`, `source.revision`, or `source.ref_type`.
- Rejected provenance: removed `source.input_ref` or `source.invocation_ref`.
- Rejected provenance: removed `source.invocation_revision`.
- Rejected provenance: any `externalParameters.release.*` member or removed builder triplet.
- Rejected fixtures: missing internal handoff job outputs, manifest digest mismatch, malformed JSON,
  duplicate object member names, unknown fields, injected release-state fields, stale
  preflight-result fields, missing `producer_provenance.sha256`, unknown producer build type before
  `npm publish` (`unregistered-producer-build-type`), missing `subject.sha512`, npm Package URL
  subject mismatch, tarball payload/final asset name mismatch, malformed optional locator/settings
  fields, caller-supplied artifact name substitution, and deterministic-name-only substitution.
- A YAML review checklist proving that the composed graph keeps the handoff manifest internal to the
  same workflow run and does not expose internal artifact names as standalone producer public
  outputs.
- YAML review: public npm has no `release-tag` or `provenance-sidecar` input.
- Schema review: the internal `release.tag` to `release-tag` mapping remains.
- Sidecar review: deterministic filename and exact verified bundle bytes remain unchanged.
- A YAML review checklist proving that the handoff schema has no release-state or preflight-result
  member, composed mode reads target release state directly before `npm publish`, and the publisher
  re-reads target release state at mutation-segment entry.
