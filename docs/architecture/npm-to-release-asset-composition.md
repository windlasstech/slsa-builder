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
  [0067](../decisions/0067-converge-repeated-runs-within-run-identity.md),
  [0072](../decisions/0072-use-sidecar-first-pair-binding-for-release-asset-run-ownership.md),
  [0074](../decisions/0074-use-single-job-mutation-segments-with-detection-based-cross-run-safety.md),
  [0075](../decisions/0075-queue-mutation-segment-contenders-with-queue-max.md),
  [0076](../decisions/0076-use-observation-preflights-and-first-mutation-classification.md),
  [0077](../decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md),
  [0103](../decisions/0103-define-external-workflow-parameters-as-common-invocation-identity.md),
  [0105](../decisions/0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md),
  [0106](../decisions/0106-consolidate-external-source-parameters-onto-workflow-and-descriptor-bindings.md),
  [0108](../decisions/0108-remove-redundant-npm-release-asset-inputs.md),
  [0109](../decisions/0109-minimize-external-parameters-to-actual-interface-inputs.md)
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
- Signing the exact preassembled Statement bytes with the Go-native `sigstore-go` DSSE adapter.
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
public mode exposes package selection, source selection, publish intent, release-asset mode, linked
metadata, and publication result handles. The release target derives from the effective built tag,
and provenance sidecar publication is mandatory in release-asset mode. Neither has an independent
public input. The public mode does not expose this composition's internal artifact names, handoff
manifest name, handoff manifest digest, or publisher handoff inputs as public workflow inputs or
outputs.

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

### Caller-facing tiered failure guarantee

For this composed path, callers receive the tiered guarantee defined by ADR 0076. The npm profile
and publisher specifications own the detailed operation rules and their fixtures.

**Tier 1, fails before any registry or release mutation.** Static YAML conformance failures, absent
OIDC capability (`ACTIONS_ID_TOKEN_REQUEST_TOKEN` is absent, producing
`windlass.verify.error.oidc-capability-unavailable`), and npm trusted-publisher configuration
failures caught by the early OIDC exchange stop the composition before mutation. An exchange 401 or
404 produces `windlass.verify.error.trusted-publisher-mismatch`; a 5xx response or malformed
exchange surface produces `windlass.verify.error.npm-oidc-exchange-indeterminate` and fails as
`indeterminate`. The caller workflow filename observation also fails before mutation: a conflict
between the OIDC `workflow_ref` claim and caller-scoped `github.workflow_ref` produces
`windlass.verify.error.trusted-publisher-mismatch`. The existing handoff-derived target-state gate
also remains Tier 1: an immutable target produces `windlass.verify.error.release-target-immutable`,
and a `foreign-conflict` or `indeterminate` release-asset classification blocks `npm publish`.

**Tier 2, classified at the first mutation.** GitHub `contents: write` runtime authority has no
side-effect-free probe. A definitive 403 or 401 from the first mutating call produces
`windlass.verify.error.mutation-permission-denied` and makes no read-back request. An ambiguous
mutating result follows ADR 0067 read-back and fails as `indeterminate` unless it proves another
outcome. Residual npm publish-time authorization failures (`E404` or `ENEEDAUTH`) produce
`windlass.verify.error.trusted-publisher-mismatch`, not raw npm diagnostics.

Each remote surface's mutations must live in exactly one job per run. The npm registry, GitHub
Release assets, and release-manifest surface are distinct surfaces. The composition must not split
mutations for any one surface across jobs, and it must not claim whole-run atomicity. Every ordering
rule that crosses a surface boundary, including the npm publish before release-asset upload rule,
must rely on the ADR 0067 classification machine. Per-surface concurrency prevents concurrent
writers within its shared group. Cross-surface and out-of-band interference is detected by
classification and fails closed; it is not prevented by a whole-run transaction.

Mutation-segment jobs use `queue: max` with `cancel-in-progress: false` on their ADR 0066
concurrency groups. Contenders wait in first-in, first-out order by the time waiting started. When a
queued run enters its segment, a retry of the same `run_id` either converges through its applicable
binding proof or a fresh `run_id` fails closed, naming the committed remote state. Caller-side
whole-invocation serialization remains an optional, caller-owned compute optimization. The callee
must never rely on it for ordering, convergence, or safety.

Before the release-state preflight, the mapping layer must verify the handoff and derive both
expected release asset names and their SHA-256 digests from it: the tarball `final-asset-name` and
`expected-sha256`, plus the deterministic sidecar name and `producer-provenance-sha256`. It must not
derive either expected asset or digest from public outputs, deterministic naming alone, logs, or
caller values.

The read-only preflight must select the publisher's closed producer-policy registry from the
verified `trusted_producer.build_type`, verify that the selected policy admits the handoff and
producer provenance, and read the existing target release's `draft` and `immutable` state and both
expected asset states using the sidecar-first pair classification below. An absent or unknown
selector fails before `npm publish` with `windlass.verify.error.unregistered-producer-build-type`.
Handoff and caller values are constraints on the selected policy only. They cannot register, extend,
replace, relax, union with, or override a producer policy.

If either release-asset side classification is `foreign-conflict` or `indeterminate`, the workflow
must fail before `npm publish`, including for a mutable target. It must name the classification and
the conflicting remote evidence. This is the fail-closed consequence of ADR 0067, and ADR 0072 does
not attribute a release-asset upload to the current run without the required pair evidence. If the
target is immutable and either expected asset is absent, `foreign-conflict`, or `indeterminate`, the
workflow must fail before `npm publish` with `windlass.verify.error.release-target-immutable`. An
immutable complete target may proceed only when the npm publish step and every release mutation step
can satisfy same-`run_id` read-only convergence. The preflight is fail-fast evidence only. It grants
no release mutation authority, and the publisher must re-read and revalidate target state when its
release mutation segment begins. It must never use the earlier preflight result as mutation
authorization.

The release-asset side classification is sidecar-first. It must classify the pair in this order:

1. If neither asset exists, the release-asset side is `absent`.
2. If the primary exists and the sidecar is absent, the release-asset side is `foreign-conflict`.
   The protocol uploads the verified sidecar first, so it can never create this state.
3. If the sidecar exists, it must first pass the sidecar conditions: its authoritative digest equals
   the expected bundle digest, it verifies under the full policy, its verified Run Invocation URI
   has the current `github.run_id` with any attempt component accepted, and its signed subject and
   producer fields bind the expected primary name and digest. Failure to establish any condition is
   `foreign-conflict` when the evidence is readable and disproves it, or `indeterminate` when the
   required state cannot be established within the polling bound.
4. A same-`run_id` verified sidecar with an absent primary permits the primary upload. A primary is
   `committed-as-expected` only if that verified sidecar also binds the remote primary's
   authoritative digest. This is the ADR 0072 pair gate. Any pre-existing asset observed by a new
   `run_id` is `foreign-conflict`, even when its bytes match.

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

| Verified handoff manifest source     | Publisher handoff field             | Notes                                                                      |
| ------------------------------------ | ----------------------------------- | -------------------------------------------------------------------------- |
| `primary_artifact.artifact_name`     | `primary-artifact-name`             | Same-run workflow artifact containing the tarball.                         |
| `primary_artifact.sha256`            | `expected-sha256`                   | Canonical digest.                                                          |
| `release.final_asset_name`           | `final-asset-name`                  | Release asset name equals the pack-produced tarball name.                  |
| `release.tag`                        | `release-tag`                       | Full effective built tag ref carried by the internal composition contract. |
| `producer_provenance.artifact_name`  | `producer-provenance-artifact-name` | Same-run artifact containing the signed DSSE bundle.                       |
| `producer_provenance.sha256`         | `producer-provenance-sha256`        | Canonical bundle digest.                                                   |
| `trusted_producer.builder_id`        | `trusted-builder-id`                | Narrowing constraint on the selected registry policy.                      |
| `trusted_producer.build_type`        | `trusted-build-type`                | Exact closed-registry selector, not a policy extension.                    |
| `subject.name`                       | `expected-subject-name`             | npm Package URL subject from producer provenance.                          |
| `subject.sha256`                     | `expected-subject-sha256`           | Must equal uploaded tarball bytes.                                         |
| `trusted_producer.source_repository` | `source-repository`                 | Canonical HTTPS URL from `externalParameters.workflow.repository`.         |
| `trusted_producer.source_revision`   | `source-revision`                   | Full Git commit from `built-source.digest.gitCommit`.                      |
| `native_provenance_locators`         | `native-provenance-locators`        | Optional array of locator objects defined by the publisher spec.           |
| `linked_artifact_settings`           | `linked-artifact-settings`          | Optional settings object defined by the publisher spec.                    |

The mapping job must verify that manifest-derived values agree with the npm producer provenance
before invoking the publisher. In particular, `trusted_producer.source_repository` must equal
`externalParameters.workflow.repository`, `trusted_producer.source_revision` must equal the
`built-source` descriptor's `digest.gitCommit`, `subject.name` must equal the npm Package URL
provenance `subject[0].name`, `subject.sha256` must equal the provenance `subject[0].digest.sha256`,
and `release.final_asset_name` must equal `primary_artifact.payload_file_name`. A mismatch is a
composition failure, not a reason to replace a manifest field with a public workflow output or
deterministic name.

For this composition, `trusted_producer.build_type` must exactly select the publisher's sole
registered policy, `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`. The mapping
layer must reject a missing or unknown value before the release-state preflight and before
`npm publish` with `windlass.verify.error.unregistered-producer-build-type`. It must pass the
selected value unchanged as `trusted-build-type`; it must not use handoff or caller values to add a
registry entry or override the registered signer, builder, subject, digest, source, release-ref,
tarball-name, or closed `externalParameters` baseline.

The mapping job must also verify the npm release binding before invoking the publisher. The
effective built ref and the required release binding are:

```text
effective built ref: externalParameters.source.ref ?? externalParameters.workflow.ref
release.tag == effective built ref == built-source URI ref component
```

The effective built ref must be the packed package's version tag in full `refs/tags/<tag-name>`
form, and `built-source.digest.gitCommit` must be its verified resolved commit. The target GitHub
Release must be the caller repository's existing release for that tag. A missing release tag or
`built-source` descriptor, a short ref, branch ref, pull request ref, disagreement among these
values, a built-source commit mismatch, or a release target outside the caller repository is a
composition failure.

The mapping job must also verify that the npm provenance subject includes `subject[0].digest.sha512`
for the same tarball bytes. SHA-512 is not a generic publisher input, but it remains mandatory npm
producer policy for this composition.

## npm subject and release asset binding

ADR 0064 narrows ADR 0050 and ADR 0052 for the JS/TS npm producer path: the upstream npm producer
provenance `subject[0].name` is the npm Package URL, while the final GitHub Release asset name is
the pack-produced tarball filename. The composition must therefore verify package-version subject
identity and tarball filename binding as separate facts.

The npm package identity is verified from the Package URL subject and the original selected
source-manifest bytes. The `package-manifest` descriptor binds those bytes by digest and source
identity, and the offline manifest-evidence contract supplies them to verifiers. The manifest's
`name` and `version` must match the Package URL subject. Missing, mismatched, or malformed evidence
is a composition failure.

The tarball filename is bound by the verified pack output and the digest-verified handoff.
`primary_artifact.payload_file_name` and `release.final_asset_name` must both equal the
pack-produced tarball filename. The signed provenance schema carries no tarball-name field.

### Final composition rule

For the npm-to-release-asset composition:

- The primary `subject[0].name` is the npm Package URL, for example
  `pkg:npm/%40windlass/slsa-builder@1.2.3`.
- The npm package name and version in the authenticated source-manifest bytes must match the Package
  URL subject. The manifest evidence is bound to the `package-manifest` descriptor.
- The final GitHub Release asset name equals the verified pack-produced tarball file name.
- The composition `release.tag`, the effective built ref (`source.ref ?? workflow.ref`), and the
  `built-source` URI's ref component are identical full tag refs.
- The mapping layer and publisher verify the tarball filename through the verified pack output and
  handoff fields before upload.
- The npm provenance subject digest includes both SHA-512 and SHA-256 for the same tarball bytes.

## Tarball digest alignment

The publisher computes the SHA-256 of the tarball bytes and compares it to:

- The `expected-sha256` from the handoff.
- The `subject[0].digest.sha256` from the producer provenance.

All three must match.

## Producer provenance sidecar publication

Sidecar publication is mandatory whenever release-asset mode is enabled. That behavior follows from
release-asset mode itself and requires no separate public sidecar input or sidecar-required
provenance parameter.

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
`windlass.verify.error.release-target-immutable` without upload. A sidecar upload failure is a
partial failure before any primary upload occurs. The release-upload job must upload and
digest-verify the sidecar before starting the primary upload. After a verified sidecar, an
interrupted or ambiguous primary upload is resolved only through the sidecar-first pair gate; the
success report may state that expected provenance-covered bytes are present, but must not claim that
the current run performed the upload without a matching receipt.

## npm-specific evidence as producer metadata

The publisher treats the following npm-specific evidence as producer metadata or policy inputs, not
as part of the generic publisher contract:

- The npm Package URL subject.
- Package `name`, `version`, `publishConfig`, and normalized `repository` values from authenticated
  package-manifest bytes.
- `publish.input_registry_url`, `publish.input_dist_tag`, and `publish.input_access`, plus the
  registry, dist-tag, and access resolutions recomputed from those raw inputs, the authenticated
  manifest's `publishConfig`, and the documented defaults.
- Tarball SHA-512 diagnostics from workflow outputs or registry metadata.
- Package-manager selection evidence in the `lockfile` descriptor's annotations and, for pnpm or
  Yarn, the observed distribution version in the `package-manager-distribution` descriptor's
  annotations.

These values are verified from producer provenance, authenticated manifest bytes, or producer-side
diagnostics. They are not generic publisher handoff fields and must not be used to bypass the
producer-neutral handoff contract.

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
extensions. The mapping layer must pass them from `externalParameters.workflow.repository` and the
`built-source` descriptor's `digest.gitCommit`, respectively, and the publisher must verify exact
equality with those signed provenance targets before upload. A missing source repository or
`built-source` descriptor, a branch or tag name in place of a revision, a short SHA, or a value that
differs from npm producer provenance is a composition failure.

`trusted-build-type` is the exact selector for the publisher's closed producer-policy registry. For
this composition, it selects only
`https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`; an absent or unknown value fails
before `npm publish` and before any release upload with
`windlass.verify.error.unregistered-producer-build-type`. The registry baseline, verified handoff,
authenticated release manifest, and caller constraints intersect. The mapping layer and publisher
must reject a conflict rather than allow a handoff or caller value to widen, register, or override a
policy.

The mapping layer must pass the internal handoff's `release.tag` to the publisher as a full Git tag
ref, such as `refs/tags/v1.2.3`. It must equal the effective built ref and the `built-source` URI's
ref component. The mapping layer must not pass a short tag name, derive a short tag for the
publisher contract, or accept an independent caller-supplied release target.

The mapping layer must not treat the signed release manifest as the source of npm caller release-ref
constraints. Manifest schema version `1` authenticates the trusted Windlass workflow mappings. The
effective built ref is recomputed from the signed source and workflow parameters and bound to the
`built-source` descriptor, while the digest-verified handoff carries the resulting `release.tag`.
Public workflow runtime guards remain in force.

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
   - `package-manifest` evidence: the original selected source-manifest bytes containing the package
     name `@windlass/slsa-builder` and version `1.2.3`, authenticated by the descriptor's digest and
     source identity
   - `built-source`: the effective built tag `refs/tags/v1.2.3` and its verified `digest.gitCommit`
   - Verified pack output and handoff filename binding: `windlass-slsa-builder-1.2.3.tgz`
3. The npm profile publishes to npm and makes the internal handoff values available to the same-run
   mapping layer.
4. The publisher receives the handoff, verifies the tarball digest and provenance, and uploads:
   - `windlass-slsa-builder-1.2.3.tgz` (primary asset)
   - `windlass-slsa-builder-1.2.3.tgz.intoto.jsonl` (sidecar)
5. A consumer downloads the tarball and the sidecar, verifies the producer provenance against the
   tarball, and verifies the npm package identity from the Package URL subject and authenticated
   source-manifest bytes.

## Rejected composition cases

The following must be rejected by the publisher:

- A raw tarball without acceptable producer provenance.
- A producer provenance whose npm Package URL subject differs from the authenticated source-manifest
  name or version, or whose package-manifest evidence is missing, malformed, or digest-mismatched.
- A tarball whose handoff payload filename differs from the final asset name.
- A tarball whose digest differs from the provenance subject digest.
- A producer provenance missing the mandatory npm subject SHA-512 or SHA-256 digest.
- A composition whose publisher `release-tag`, effective built ref (`source.ref ?? workflow.ref`),
  or `built-source` URI ref component disagree, or whose handoff source revision differs from
  `built-source.digest.gitCommit`.
- A composed ordering that performs `npm publish` before handoff-derived release-state and policy
  preflight.
- A missing or unknown producer `buildType`, including any value other than the sole registered npm
  policy selector (`unregistered-producer-build-type`).
- An immutable target with either expected handoff-derived asset absent, foreign, or indeterminate
  before `npm publish` (`release-target-immutable`).
- A mutable target whose release-asset side is `foreign-conflict` or `indeterminate` before
  `npm publish`; the workflow must fail closed with the classification and remote evidence.
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
- The npm Package URL subject does not match the authenticated source-manifest identity, the tarball
  filename binding or digest does not align, or required package-manifest evidence is missing or
  mismatched.
- The handoff source repository or revision differs from `externalParameters.workflow.repository` or
  `built-source.digest.gitCommit`, or `release.tag` differs from the effective built ref or the
  `built-source` URI's ref component.
- The release target does not exist.
- The read-only preflight finds an immutable target whose required handoff-derived asset set cannot
  complete same-`run_id` convergence. It must fail before `npm publish` with
  `windlass.verify.error.release-target-immutable`.
- The read-only preflight finds a mutable target with a release-asset-side `foreign-conflict` or
  `indeterminate` classification. It must fail before `npm publish`, naming the classification and
  the remote evidence.
- The verified `trusted_producer.build_type` is missing or unregistered. It must fail before
  `npm publish` with `windlass.verify.error.unregistered-producer-build-type`.
- Publisher mutation-entry revalidation fails or differs from the earlier preflight. The publisher
  must fail before release mutation and must not reuse the stale preflight as authorization.
- The primary asset name or deterministic sidecar name already exists on a mutable target without
  same-`run_id` convergence proof.
- The sidecar upload fails before the primary upload begins.

## TDD and fixtures

- Positive fixture: an npm tarball successfully composes with the publisher, with package identity
  verified from the Package URL subject and authenticated package-manifest bytes, source repository
  and revision mapped from `externalParameters.workflow.repository` and
  `built-source.digest.gitCommit`, and the tarball filename bound by the verified pack output and
  handoff.
- Rejected fixtures: raw tarball without provenance, npm Package URL subject or manifest-evidence
  mismatch, missing package-manifest evidence, tarball filename binding mismatch, digest mismatch, a
  release tag that disagrees with the effective built ref or `built-source` URI, a source revision
  that disagrees with `built-source.digest.gitCommit`, npm-specific publisher coupling, pre-existing
  primary or sidecar release asset name classified through the sidecar-first pair rules, a lone
  primary classified as `foreign-conflict`, a composed ordering that reaches `npm publish` before
  preflight, a mutable-target `foreign-conflict` or `indeterminate` result that blocks
  `npm publish`, an unknown `buildType` (`unregistered-producer-build-type`), an immutable target
  with a partial asset set before `npm publish` (`release-target-immutable`), and a stale preflight
  result reused at publisher mutation entry.
- A fixture proving that the publisher remains producer-neutral when the same handoff is constructed
  from a different producer profile (mock or stub).
- A fixture proving that release-asset mode publishes the unchanged sidecar without any separate
  sidecar input, while the internal handoff retains the full `release.tag` binding.
- A review checklist proving that expected asset names and digests come from the verified handoff,
  policy selection uses the publisher's closed registry, immutable-incomplete targets fail before
  npm mutation, and publisher mutation-entry revalidation never trusts an early preflight result.
