# Composed Workflow Internal Handoff Contract

This document defines the internal same-run contract used when a producer profile and the GitHub
Release asset publisher are composed inside one workflow graph.

- Source ADRs: [0036](../decisions/0036-use-three-job-digest-verified-publish-graph.md),
  [0050](../decisions/0050-define-producer-to-publisher-handoff-contract.md),
  [0052](../decisions/0052-compose-npm-package-tarball-producer-with-release-asset-publisher.md)
- Related specs: [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md),
  [npm-to-release-asset composition](npm-to-release-asset-composition.md),
  [Core profile contract](core-profile-contract.md)

## Scope and non-goals

**In scope:**

- Same-workflow-run internal handoff from a producer to a composition mapping job.
- Producer-owned delivery of internal artifact names and provenance bundle SHA-256.
- Digest verification for the handoff manifest before publisher inputs are constructed.
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

## Producer-owned handoff manifest

After the producer has created the primary artifact and signed producer provenance bundle, it must
write a handoff manifest JSON file and upload it as a same-run GitHub Actions artifact.

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

## Manifest schema

The handoff manifest JSON object must use this closed schema shape. Unknown fields are invalid.

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
  "trusted_producer": {
    "builder_id": "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@0123456789abcdef0123456789abcdef01234567",
    "build_type": "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1",
    "source_repository": "https://github.com/example/project",
    "source_revision": "fedcba9876543210fedcba9876543210fedcba98"
  },
  "subject": {
    "name": "windlass-slsa-builder-1.2.3.tgz",
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

### Field rules

- `schema_version` must be `"1"`.
- `producer_profile` must identify the producer profile that owns the artifact and provenance. The
  initial value is `js-ts-npm-package`.
- `primary_artifact.artifact_name` must be a same-run GitHub Actions artifact uploaded by the
  producer job and containing exactly one payload file.
- `primary_artifact.payload_file_name` must be the basename of the pack-produced tarball. For the
  initial npm composition it must end in `.tgz`.
- `primary_artifact.sha256` must be the lowercase SHA-256 digest of the primary artifact bytes.
- `producer_provenance.artifact_name` must be a same-run GitHub Actions artifact uploaded by the
  producer signing job and containing exactly one signed producer provenance bundle file.
- `producer_provenance.payload_file_name` is the provenance bundle file basename. It is transport
  metadata only; the publisher sidecar name remains derived from the final asset name by the
  publisher contract.
- `producer_provenance.sha256` is the producer-owned SHA-256 digest of the signed bundle bytes. This
  value maps to the publisher handoff field `producer-provenance-sha256`.
- `trusted_producer.builder_id` and `trusted_producer.build_type` must match the producer provenance
  and the trusted release manifest or explicit policy.
- `trusted_producer.source_repository` and `trusted_producer.source_revision` must match the
  producer provenance `externalParameters.source` values required by the selected producer policy.
- `subject.name` must equal `primary_artifact.payload_file_name` and `release.final_asset_name`.
- `subject.sha256` must equal `primary_artifact.sha256`.
- `release.tag` must be the full Git tag ref, for example `refs/tags/v1.2.3`, that identifies the
  existing Git tag and target GitHub Release. A short tag name such as `v1.2.3` is invalid in the
  handoff manifest.
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
| `trusted_producer.builder_id`        | `trusted-builder-id`                |
| `trusted_producer.build_type`        | `trusted-build-type`                |
| `subject.name`                       | `expected-subject-name`             |
| `subject.sha256`                     | `expected-subject-sha256`           |
| `trusted_producer.source_repository` | `source-repository`                 |
| `trusted_producer.source_revision`   | `source-revision`                   |
| `native_provenance_locators`         | `native-provenance-locators`        |
| `linked_artifact_settings`           | `linked-artifact-settings`          |

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
  closed schema;
- any required artifact name, subject, digest, source identity, builder identity, build type,
  release tag, or final asset name is missing or malformed;
- `release.tag` is not a full `refs/tags/<tag-name>` ref;
- `producer_provenance.sha256` is not a 64-character lowercase hexadecimal SHA-256 digest;
- `native_provenance_locators` or `linked_artifact_settings` is present but violates the publisher
  contract schema for the mapped handoff field;
- `subject.name`, `primary_artifact.payload_file_name`, and `release.final_asset_name` are not
  identical; or
- the mapping job attempts to construct publisher inputs from caller-controlled values rather than
  the verified handoff manifest.

## TDD and fixtures

- Positive fixture: a valid npm producer handoff manifest maps to the exact publisher inputs.
- Rejected fixtures: missing internal handoff job outputs, manifest digest mismatch, malformed JSON,
  duplicate object member names, unknown fields, missing `producer_provenance.sha256`, subject/final
  asset name mismatch, malformed optional locator/settings fields, caller-supplied artifact name
  substitution, and deterministic-name-only substitution.
- A YAML review checklist proving that the composed graph keeps the handoff manifest internal to the
  same workflow run and does not expose internal artifact names as standalone producer public
  outputs.
