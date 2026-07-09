# npm Producer To GitHub Release Asset Publisher Composition

This document defines the first concrete producer-to-publisher composition: the JS/TS npm package
tarball producer feeding the GitHub Release asset publisher.

- Source ADRs:
  [0013](../decisions/0013-scope-initial-js-ts-profile-to-npm-packages.md)–[0037](../decisions/0037-define-initial-verification-deliverables.md),
  [0049](../decisions/0049-separate-artifact-production-from-github-release-asset-publication.md),
  [0050](../decisions/0050-define-producer-to-publisher-handoff-contract.md),
  [0051](../decisions/0051-distribute-producer-provenance-with-release-assets.md),
  [0052](../decisions/0052-compose-npm-package-tarball-producer-with-release-asset-publisher.md)
- Related specs: [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md),
  [Composed workflow internal handoff](composed-workflow-internal-handoff.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md),
  [SLSA provenance v1](slsa-provenance-v1.md),
  [Identity and build types](identity-and-buildtypes.md),
  [Verification policy and fixtures](verification-policy-and-fixtures.md)

## Scope and non-goals

**In scope:**

- How the npm producer outputs map to the generic publisher handoff.
- Subject name alignment between npm provenance and release asset name.
- Digest alignment.
- Sidecar publication.
- Producer-neutral publisher constraints.
- Future producer compatibility.

**Out of scope:**

- Generic raw file upload without producer provenance.
- Other ecosystem producers such as Go binary or container producers.
- Publisher-owned publication predicates.

## Composition overview

The JS/TS npm package profile produces the package tarball and its SLSA provenance. The GitHub
Release asset publisher receives the tarball through the producer-to-publisher handoff, verifies the
producer provenance, and uploads the tarball and the provenance sidecar to an existing GitHub
Release.

```text
JS/TS npm package profile
         │
         │ produces
         │
         ▼
  npm package tarball
  + Windlass SLSA provenance
         │
         │ producer-to-publisher handoff
         │
         ▼
GitHub Release asset publisher
         │
         │ verifies and distributes
         │
         ▼
  GitHub Release page
  ├── tarball (primary asset)
  └── tarball.intoto.jsonl (sidecar)
```

## npm producer responsibilities

The npm producer is responsible for:

- Selecting the package directory and package manager.
- Installing dependencies, running the build script, and packing the tarball.
- Generating the Windlass SLSA provenance v1 Statement for the tarball.
- Signing the provenance with `actions/attest`.
- Publishing to the npm registry using `npm publish --provenance-file`.
- Making the tarball and provenance bundle available to a same-run composition mapping layer.

## Composition execution boundary

The initial npm-to-release-asset composition is a same-workflow-run composition, not a public
workflow-output-to-workflow-output API. The npm producer's public `workflow_call.outputs` remain the
package identity and tarball digest handles defined by the npm profile. Internal artifact names and
provenance bundle artifact names are not public outputs.

A composed release workflow or mapping layer that runs the npm producer and publisher in the same
workflow run must receive the producer-owned internal handoff manifest defined by the
[composed workflow internal handoff spec](composed-workflow-internal-handoff.md). The mapping layer
must not derive trust from logs, release notes, public workflow outputs, deterministic naming alone,
or caller-supplied artifact names as substitutes for the producer-owned same-run handoff manifest
and its digest-verified contents.

The deterministic initial internal artifact names are:

```text
js-ts-npm-package-tarball-<github.run_id>-<github.run_attempt>
js-ts-npm-provenance-bundle-<github.run_id>-<github.run_attempt>
js-ts-npm-composition-handoff-<github.run_id>-<github.run_attempt>
```

The deterministic names are transport locators only. The composed workflow must pass the composition
handoff manifest artifact name and manifest SHA-256 through internal same-run job outputs owned by
the producer job. Those internal outputs are not public `workflow_call.outputs`; they exist only for
the composed workflow graph that connects the producer to the publisher in the same run.

If a future release process needs to connect separately invoked reusable workflows through public
outputs, that is a new composition contract and must be specified separately.

## Publisher responsibilities

The publisher is responsible for:

- Receiving the tarball via the generic handoff contract.
- Verifying the tarball digest against the expected digest.
- Verifying the producer provenance.
- Uploading the tarball as the primary GitHub Release asset.
- Uploading the unchanged producer provenance bundle as the sidecar.
- Exposing native provenance locators if available.

## Handoff field mapping

The publisher inputs are constructed from the digest-verified internal handoff manifest, not from
public npm producer outputs, deterministic names, logs, or caller-supplied values. Public npm
outputs such as `package-tarball-sha256` and `package-tarball-name` may be compared against the
manifest for diagnostics, but they are not trusted mapping sources for the publisher contract.

| Verified handoff manifest source     | Publisher handoff field             | Notes                                                               |
| ------------------------------------ | ----------------------------------- | ------------------------------------------------------------------- |
| `primary_artifact.artifact_name`     | `primary-artifact-name`             | Same-run workflow artifact containing the tarball.                  |
| `primary_artifact.sha256`            | `expected-sha256`                   | Canonical digest.                                                   |
| `release.final_asset_name`           | `final-asset-name`                  | Release asset name equals tarball name.                             |
| `release.tag`                        | `release-tag`                       | Same full `refs/tags/<tag-name>` ref used by the npm release.       |
| `producer_provenance.artifact_name`  | `producer-provenance-artifact-name` | Same-run artifact containing the signed DSSE bundle.                |
| `producer_provenance.sha256`         | `producer-provenance-sha256`        | Canonical bundle digest.                                            |
| `trusted_producer.builder_id`        | `trusted-builder-id`                | From the release manifest or explicit policy.                       |
| `trusted_producer.build_type`        | `trusted-build-type`                | `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`. |
| `subject.name`                       | `expected-subject-name`             | Must equal `final-asset-name`.                                      |
| `subject.sha256`                     | `expected-subject-sha256`           | Must equal uploaded bytes.                                          |
| `trusted_producer.source_repository` | `source-repository`                 | Required canonical HTTPS source repository URL.                     |
| `trusted_producer.source_revision`   | `source-revision`                   | Required full source revision; GitHub Git sources use 40-char SHA.  |
| `native_provenance_locators`         | `native-provenance-locators`        | Optional array of locator objects defined by the publisher spec.    |
| `linked_artifact_settings`           | `linked-artifact-settings`          | Optional settings object defined by the publisher spec.             |

The mapping job must verify that manifest-derived values agree with the npm producer provenance
before invoking the publisher. For example, `trusted_producer.source_repository` must equal
`externalParameters.source.repository`, `subject.name` must equal the provenance `subject[0].name`,
and `subject.sha256` must equal the provenance `subject[0].digest.sha256`. A mismatch is a
composition failure, not a reason to replace a manifest field with a public workflow output or
deterministic name.

## Subject name alignment

ADR 0050 requires the upstream producer provenance `subject[0].name` to exactly match the final
GitHub Release asset name. For this composition, the npm producer must name the tarball file as the
provenance subject.

The npm package identity remains available to verifiers through `externalParameters` so that
consumers can correlate the release asset with the published npm package.

### Final composition rule

For the npm-to-release-asset composition:

- The primary `subject[0].name` is the tarball file name, for example
  `windlass-slsa-builder-1.2.3.tgz`.
- The npm package name and version are recorded in `externalParameters` under `package.name` and
  `package.version`.
- The final GitHub Release asset name equals the tarball file name.
- The publisher verifies this equality before upload.

## Tarball digest alignment

The publisher computes the SHA-256 of the tarball bytes and compares it to:

- The `expected-sha256` from the handoff.
- The `subject[0].digest.sha256` from the producer provenance.

All three must match.

## Producer provenance sidecar publication

The publisher uploads the unchanged npm producer provenance bundle as:

```text
<tarball-name>.intoto.jsonl
```

For npm pack-produced tarballs, the sidecar therefore normally uses a `.tgz.intoto.jsonl` suffix,
for example `windlass-slsa-builder-1.2.3.tgz.intoto.jsonl`.

The sidecar must contain the same signed bundle bytes that the npm producer generated and that the
publisher verified, byte-for-byte. The publisher must not extract only the Statement, reserialize
the bundle, or substitute a native provenance locator for the sidecar file.

Before uploading the npm tarball, the publisher must preflight-check that neither the tarball asset
name nor the deterministic sidecar name already exists on the target GitHub Release. If either name
exists, the composition fails without mutating the release. A sidecar upload failure after primary
asset upload is a partial failure only when the duplicate preflight passed and a later upload/API
failure occurred.

## npm-specific fields as producer metadata

The publisher treats the following npm-specific fields as producer metadata or policy inputs, not as
part of the generic publisher contract:

- `package.name`
- `package.version`
- `publish.resolved_registry_url`
- `publish.resolved_dist_tag`
- `package.package_url`
- tarball SHA-512 diagnostics from workflow outputs or registry metadata
- `package_manager.name`
- `package_manager.version`
- `package_manager.selection_source`

These fields are verified from producer provenance or producer-side diagnostics. They are not
generic publisher handoff fields and must not be used to bypass the producer-neutral handoff
contract.

## Producer-neutral publisher constraints

The publisher's implementation must not hardcode npm-specific logic. The publisher must rely on the
generic handoff fields:

- `primary-artifact-name`
- `expected-sha256`
- `final-asset-name`
- `release-tag`
- `producer-provenance-artifact-name`
- `producer-provenance-sha256`
- `trusted-builder-id`
- `trusted-build-type`
- `expected-subject-name`
- `expected-subject-sha256`
- `source-repository`
- `source-revision`

For this initial npm composition, `source-repository` and `source-revision` are not optional policy
extensions. The mapping layer must pass them from npm provenance `externalParameters.source`, and
the publisher must verify exact equality before upload. A missing source repository, a branch or tag
name in place of a revision, a short SHA, or a value that differs from npm producer provenance is a
composition failure.

The mapping layer must pass `release.tag` to the publisher as a full Git tag ref, such as
`refs/tags/v1.2.3`. It must not pass the short tag name from `release.version_tag` or derive a short
tag from the full ref for the publisher contract.

`native-provenance-locators`, when present, must use the plural field name and the locator object
schema from the publisher contract. A singular `native-provenance-locator` field is invalid and must
be rejected by strict handoff validation.

Native provenance locators in this composition are diagnostic discovery metadata only. The mapping
layer and publisher must not use them as substitutes for the same-run producer provenance bundle
artifact, the `producer-provenance-sha256` digest, or producer bundle verification. When a locator
includes a digest, that digest must equal the signed producer bundle bytes that will be uploaded as
the release sidecar.

Any npm-specific behavior must be isolated in the handoff mapping layer or in the trusted producer
policy.

## End-to-end example

A project releases `@windlass/slsa-builder` version `1.2.3` to npm and also wants a GitHub Release
copy of the tarball.

1. The npm profile runs and produces `windlass-slsa-builder-1.2.3.tgz` with SHA-256 `abc123...`.
2. The npm profile generates provenance with:
   - `subject[0].name`: `windlass-slsa-builder-1.2.3.tgz`
   - `subject[0].digest.sha256`: `abc123...`
   - `externalParameters.package.name`: `@windlass/slsa-builder`
   - `externalParameters.package.version`: `1.2.3`
3. The npm profile publishes to npm and makes the internal handoff values available to the same-run
   mapping layer.
4. The publisher receives the handoff, verifies the tarball digest and provenance, and uploads:
   - `windlass-slsa-builder-1.2.3.tgz` (primary asset)
   - `windlass-slsa-builder-1.2.3.tgz.intoto.jsonl` (sidecar)
5. A consumer downloads the tarball and the sidecar, verifies the producer provenance against the
   tarball, and trusts the npm package identity recorded in `externalParameters`.

## Rejected composition cases

The following must be rejected by the publisher:

- A raw tarball without acceptable producer provenance.
- A tarball whose subject name differs from the final asset name.
- A tarball whose digest differs from the provenance subject digest.
- A producer provenance with a non-npm `buildType` unless the policy explicitly allows it.
- Any attempt to use npm-specific fields to bypass the generic handoff contract.

## Future producer profile compatibility

The publisher handoff contract must be designed so that future producer profiles can compose with
the same publisher without changing the publisher's trust boundary. Future producers must provide:

- Same-run artifact name for the primary asset.
- Expected SHA-256.
- Final asset name.
- Release tag.
- Same-run artifact name for the producer provenance bundle.
- Producer provenance bundle SHA-256.
- Trusted producer `builder.id` and `buildType`.
- Expected subject name and digest.
- Source identity required by policy.

## Failure behavior

The composition must fail when:

- The npm producer does not produce a valid tarball and provenance.
- The handoff fields are missing or inconsistent.
- The publisher cannot verify the producer provenance.
- The subject name or digest does not align.
- The release target does not exist.
- The primary asset name or deterministic sidecar name already exists on the target release.
- The primary asset upload succeeds but the sidecar upload fails.

## TDD and fixtures

- Positive fixture: npm tarball successfully composes with the publisher.
- Rejected fixtures: raw tarball without provenance, renamed subject, digest mismatch, npm-specific
  publisher coupling, pre-existing primary or sidecar release asset name, and unsupported producer
  `buildType`.
- A fixture proving that the publisher remains producer-neutral when the same handoff is constructed
  from a different producer profile (mock or stub).
