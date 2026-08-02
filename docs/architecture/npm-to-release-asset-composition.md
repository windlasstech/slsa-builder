# npm Producer To GitHub Release Asset Publisher Composition

This document defines the first concrete producer-to-publisher composition: the JS/TS npm package
tarball producer feeding the GitHub Release asset publisher.

- Source ADRs:
  [0013](../decisions/0013-scope-initial-js-ts-profile-to-npm-packages.md)–[0019](../decisions/0019-validate-js-ts-package-metadata-through-packed-artifacts.md),
  [0022](../decisions/0022-use-js-ts-npm-package-slsa3-workflow-entrypoint.md)–[0037](../decisions/0037-define-initial-verification-deliverables.md),
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
  [0067](../decisions/0067-converge-repeated-runs-within-run-identity.md)
- Related specs: [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md),
  [Composed workflow internal handoff](composed-workflow-internal-handoff.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md),
  [SLSA provenance v1](slsa-provenance-v1.md),
  [Identity and build types](identity-and-buildtypes.md),
  [Verification policy and fixtures](verification-policy-and-fixtures.md)

## Scope and non-goals

**In scope:**

- How the npm producer outputs map to the generic publisher handoff.
- Binding between npm Package URL provenance subject, tarball filename, and release asset name.
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
- Making the tarball and provenance bundle available to a same-run composition mapping layer.
- Publishing to the npm registry using `npm publish --provenance-file` only after the composed
  release-state and policy preflight passes.

## Composition execution boundary

The initial npm-to-release-asset composition is a same-workflow-run composition, not a public
workflow-output-to-workflow-output API. The npm producer's public `workflow_call.outputs` remain the
package identity and tarball digest handles defined by the npm profile. Internal artifact names and
provenance bundle artifact names are not public outputs.

The recommended public caller surface for this composition is release-asset mode on
`.github/workflows/js-ts-npm-package-slsa3.yml`, as defined by the
[JS/TS npm package workflow contract](js-ts-npm-package-profile.md#workflow_call-contract). That
public mode exposes package, publish, release tag, sidecar policy, linked metadata, and publication
result handles. It does not expose this composition's internal artifact names, handoff manifest
name, handoff manifest digest, or publisher handoff inputs as public workflow inputs or outputs.

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

The observable composed ordering is:

```text
build/pack/sign/handoff
  -> read-only release-state and policy preflight
  -> npm publish or same-run npm convergence
  -> publisher upload or same-run read-only convergence
```

Before the release-state preflight, the mapping layer must verify the handoff and derive both
expected release asset names and their SHA-256 digests from it: the tarball `final-asset-name` and
`expected-sha256`, plus the deterministic sidecar name and `producer-provenance-sha256`. It must not
derive either expected asset or digest from public outputs, deterministic naming alone, logs, or
caller values.

The read-only preflight must select the publisher's closed producer-policy registry from the
verified `trusted_producer.build_type`, verify that the selected policy admits the handoff and
producer provenance, and read the existing target release's `draft` and `immutable` state and both
expected asset states. An absent or unknown selector fails before `npm publish` with
`windlass.verify.error.unregistered-producer-build-type`. Handoff and caller values are constraints
on the selected policy only. They cannot register, extend, replace, relax, union with, or override a
producer policy.

If the target is immutable and either expected asset is absent, foreign, or indeterminate, the
workflow must fail before `npm publish` with `windlass.verify.error.release-target-immutable`. An
immutable complete target may proceed only when the npm publish step and every release mutation step
can satisfy same-`run_id` read-only convergence. The preflight is fail-fast evidence only. It grants
no release mutation authority, and the publisher must re-read and revalidate target state when its
release mutation segment begins. It must never use the earlier preflight result as mutation
authorization.

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

The publisher retains all GitHub Release mutation authority. The build, signing, npm publish, and
mapping jobs may perform the read-only preflight but must not upload, replace, delete, or otherwise
mutate release assets. Before any publisher upload segment, the publisher must re-read the target
release state and revalidate both expected assets against the verified handoff and selected closed
policy. This mutation-entry revalidation is mandatory even when the preflight passed.

## Handoff field mapping

The publisher inputs are constructed from the digest-verified internal handoff manifest, not from
public npm producer outputs, deterministic names, logs, or caller-supplied values. Public npm
outputs such as `package-tarball-sha256` and `package-tarball-name` may be compared against the
manifest for diagnostics, but they are not trusted mapping sources for the publisher contract.

| Verified handoff manifest source     | Publisher handoff field             | Notes                                                              |
| ------------------------------------ | ----------------------------------- | ------------------------------------------------------------------ |
| `primary_artifact.artifact_name`     | `primary-artifact-name`             | Same-run workflow artifact containing the tarball.                 |
| `primary_artifact.sha256`            | `expected-sha256`                   | Canonical digest.                                                  |
| `release.final_asset_name`           | `final-asset-name`                  | Release asset name equals the pack-produced tarball name.          |
| `release.tag`                        | `release-tag`                       | Same full `refs/tags/<tag-name>` ref used by the npm release.      |
| `producer_provenance.artifact_name`  | `producer-provenance-artifact-name` | Same-run artifact containing the signed DSSE bundle.               |
| `producer_provenance.sha256`         | `producer-provenance-sha256`        | Canonical bundle digest.                                           |
| `trusted_producer.builder_id`        | `trusted-builder-id`                | Narrowing constraint on the selected registry policy.              |
| `trusted_producer.build_type`        | `trusted-build-type`                | Exact closed-registry selector, not a policy extension.            |
| `subject.name`                       | `expected-subject-name`             | npm Package URL subject from producer provenance.                  |
| `subject.sha256`                     | `expected-subject-sha256`           | Must equal uploaded tarball bytes.                                 |
| `trusted_producer.source_repository` | `source-repository`                 | Required canonical HTTPS source repository URL.                    |
| `trusted_producer.source_revision`   | `source-revision`                   | Required full source revision; GitHub Git sources use 40-char SHA. |
| `native_provenance_locators`         | `native-provenance-locators`        | Optional array of locator objects defined by the publisher spec.   |
| `linked_artifact_settings`           | `linked-artifact-settings`          | Optional settings object defined by the publisher spec.            |

The mapping job must verify that manifest-derived values agree with the npm producer provenance
before invoking the publisher. For example, `trusted_producer.source_repository` must equal
`externalParameters.source.repository`, `subject.name` must equal the npm Package URL provenance
`subject[0].name`, `subject.sha256` must equal the provenance `subject[0].digest.sha256`, and
`release.final_asset_name` must equal `primary_artifact.payload_file_name`. A mismatch is a
composition failure, not a reason to replace a manifest field with a public workflow output or
deterministic name.

For this composition, `trusted_producer.build_type` must exactly select the publisher's sole
registered policy, `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`. The mapping
layer must reject a missing or unknown value before the release-state preflight and before
`npm publish` with `windlass.verify.error.unregistered-producer-build-type`. It must pass the
selected value unchanged as `trusted-build-type`; it must not use handoff or caller values to add a
registry entry or override the registered signer, builder, subject, digest, source, release-ref,
tarball-name, or closed `externalParameters` baseline.

The mapping job must also verify the npm release binding before invoking the publisher. For this
composition, all of the following values must be the same full Git tag ref:

```text
release.tag == externalParameters.release.ref == externalParameters.source.ref
```

`externalParameters.release.version_tag` must be the short tag name whose `refs/tags/` expansion
equals the same ref. The target GitHub Release must be the caller repository's existing release for
that tag. A missing release field, short ref, branch ref, pull request ref, mismatched
source/release ref, mismatched version tag, or release target outside the caller repository is a
composition failure.

The mapping job must also verify that the npm provenance subject includes `subject[0].digest.sha512`
for the same tarball bytes. SHA-512 is not a generic publisher input, but it remains mandatory npm
producer policy for this composition.

## npm subject and release asset binding

ADR 0064 narrows ADR 0050 and ADR 0052 for the JS/TS npm producer path: the upstream npm producer
provenance `subject[0].name` is the npm Package URL, while the final GitHub Release asset name is
the pack-produced tarball filename. The composition must therefore verify package-version subject
identity and tarball filename binding as separate facts.

The npm package identity is available to verifiers through the Package URL subject and through
`externalParameters.package.name` and `externalParameters.package.version`. The tarball filename is
available through `externalParameters.package.tarball_name`, `primary_artifact.payload_file_name`,
and `release.final_asset_name`.

### Final composition rule

For the npm-to-release-asset composition:

- The primary `subject[0].name` is the npm Package URL, for example
  `pkg:npm/%40windlass/slsa-builder@1.2.3`.
- The npm package name and version are recorded in `externalParameters` under `package.name` and
  `package.version` and must match the Package URL subject.
- The final GitHub Release asset name equals the tarball file name.
- The composition release tag, producer `externalParameters.release.ref`, and producer
  `externalParameters.source.ref` are identical full tag refs.
- The mapping layer and publisher verify the tarball filename through producer policy and handoff
  fields before upload.
- The npm provenance subject digest includes both SHA-512 and SHA-256 for the same tarball bytes.

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

This publisher preflight occurs at release mutation-segment entry and is distinct from the earlier
composed preflight before `npm publish`. Before uploading the npm tarball, it must re-read the
target's `draft` and `immutable` state and classify both handoff-derived expected assets. A
pre-existing tarball asset name or deterministic sidecar name fails without release mutation when it
belongs to a new or different `run_id`, or when the required digest and same-run binding or
convergence proof cannot be established. When revalidation classifies both existing assets as a
complete same-`run_id` set satisfying read-only convergence, their presence is the expected
convergence state, not a duplicate failure, and the publisher must finish without mutation. An
immutable target with an incomplete asset set must fail with
`windlass.verify.error.release-target-immutable` without upload. A sidecar upload failure after
primary asset upload is a partial failure only when this qualified duplicate preflight permits the
primary upload and a later upload/API failure occurs.

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

`trusted-build-type` is the exact selector for the publisher's closed producer-policy registry. For
this composition, it selects only
`https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`; an absent or unknown value fails
before `npm publish` and before any release upload with
`windlass.verify.error.unregistered-producer-build-type`. The registry baseline, verified handoff,
authenticated release manifest, and caller constraints intersect. The mapping layer and publisher
must reject a conflict rather than allow a handoff or caller value to widen, register, or override a
policy.

The mapping layer must pass `release.tag` to the publisher as a full Git tag ref, such as
`refs/tags/v1.2.3`. It must not pass the short tag name from `release.version_tag` or derive a short
tag from the full ref for the publisher contract.

The mapping layer must not treat the signed release manifest as the source of npm caller release-ref
constraints. Manifest schema version `1` authenticates the trusted Windlass workflow mappings, but
the npm caller source ref and release ref come from the signed npm producer provenance,
digest-verified handoff, and public workflow runtime guards.

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
   - `subject[0].name`: `pkg:npm/%40windlass/slsa-builder@1.2.3`
   - `subject[0].digest.sha256`: `abc123...`
   - `subject[0].digest.sha512`: the SHA-512 of `windlass-slsa-builder-1.2.3.tgz`
   - `externalParameters.package.name`: `@windlass/slsa-builder`
   - `externalParameters.package.version`: `1.2.3`
   - `externalParameters.package.tarball_name`: `windlass-slsa-builder-1.2.3.tgz`
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
- A producer provenance whose npm Package URL subject differs from package name or version policy.
- A tarball whose handoff payload filename differs from the final asset name.
- A tarball whose digest differs from the provenance subject digest.
- A producer provenance missing the mandatory npm subject SHA-512 or SHA-256 digest.
- A producer provenance whose `externalParameters.source.ref`, `externalParameters.release.ref`, or
  reconstructed `release.version_tag` differs from the publisher `release-tag`.
- A composed ordering that performs `npm publish` before handoff-derived release-state and policy
  preflight.
- A missing or unknown producer `buildType`, including any value other than the sole registered npm
  policy selector (`unregistered-producer-build-type`).
- An immutable target with either expected handoff-derived asset absent, foreign, or indeterminate
  before `npm publish` (`release-target-immutable`).
- A stale preflight observation presented as authority for publisher mutation.
- Any handoff or caller attempt to register, widen, or override a producer policy.
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
- The npm Package URL subject, tarball filename binding, or digest does not align.
- The release target does not exist.
- The read-only preflight finds an immutable target whose required handoff-derived asset set cannot
  complete same-`run_id` convergence. It must fail before `npm publish` with
  `windlass.verify.error.release-target-immutable`.
- The verified `trusted_producer.build_type` is missing or unregistered. It must fail before
  `npm publish` with `windlass.verify.error.unregistered-producer-build-type`.
- Publisher mutation-entry revalidation fails or differs from the earlier preflight. The publisher
  must fail before release mutation and must not reuse the stale preflight as authorization.
- The primary asset name or deterministic sidecar name already exists on a mutable target without
  same-`run_id` convergence proof.
- The primary asset upload succeeds but the sidecar upload fails.

## TDD and fixtures

- Positive fixture: npm tarball successfully composes with the publisher.
- Rejected fixtures: raw tarball without provenance, npm Package URL subject mismatch, tarball
  filename binding mismatch, digest mismatch, npm-specific publisher coupling, pre-existing primary
  or sidecar release asset name, a composed ordering that reaches `npm publish` before preflight, an
  unknown `buildType` (`unregistered-producer-build-type`), an immutable target with a partial asset
  set before `npm publish` (`release-target-immutable`), and a stale preflight result reused at
  publisher mutation entry.
- A fixture proving that the publisher remains producer-neutral when the same handoff is constructed
  from a different producer profile (mock or stub).
- A review checklist proving that expected asset names and digests come from the verified handoff,
  policy selection uses the publisher's closed registry, immutable-incomplete targets fail before
  npm mutation, and publisher mutation-entry revalidation never trusts an early preflight result.
