# JS/TS npm Provenance, Publish, And Three-Job Graph

This document defines how the JS/TS npm profile generates SLSA provenance, signs it, and publishes
the package to an npm registry through a three-job digest-verified graph.

- Source ADRs: [0024](../decisions/0024-use-oidc-trusted-publishing-without-publish-secrets.md),
  [0025](../decisions/0025-return-package-identity-and-tarball-digest-outputs.md),
  [0028](../decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md),
  [0029](../decisions/0029-use-windlass-generated-slsa-provenance-for-npm-publish.md),
  [0030](../decisions/0030-accept-registry-url-while-guaranteeing-only-npmjs-semantics.md),
  [0036](../decisions/0036-use-three-job-digest-verified-publish-graph.md),
  [0037](../decisions/0037-define-initial-verification-deliverables.md),
  [0052](../decisions/0052-compose-npm-package-tarball-producer-with-release-asset-publisher.md),
  [0056](../decisions/0056-treat-non-selected-lockfiles-as-stale-diagnostics.md),
  [0057](../decisions/0057-provide-composed-public-npm-release-asset-workflow.md),
  [0059](../decisions/0059-define-public-npm-release-composed-workflow-interface.md),
  [0060](../decisions/0060-unify-npm-profile-public-entrypoint-with-release-asset-mode.md),
  [0061](../decisions/0061-reject-duplicate-json-members-in-signed-slsa-statements.md),
  [0063](../decisions/0063-limit-yarn-support-to-berry-v4-with-corepack-package-manager.md),
  [0064](../decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md),
  [0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [0067](../decisions/0067-converge-repeated-runs-within-run-identity.md),
  [0068](../decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md),
  [0070](../decisions/0070-record-package-manager-distributions-and-runner-image-in-resolved-dependencies.md),
  and
  [0071](../decisions/0071-activate-builder-version-and-builderdependencies-for-platform-components.md),
  [0073](../decisions/0073-require-published-attestation-run-identity-for-npm-same-run-convergence.md),
  [0075](../decisions/0075-queue-mutation-segment-contenders-with-queue-max.md),
  [0077](../decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md),
  [0079](../decisions/0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md),
  and
  [0080](../decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md),
  [0081](../decisions/0081-pin-npm-oidc-exchange-response-contract-and-correct-token-lifetime-assumption.md),
  [0082](../decisions/0082-pin-publish-stage-npm-cli-version-with-integrity-verified-provisioning-and-reviewed-allowlist.md),
  [0083](../decisions/0083-defer-npm-m1-publish-remediation-to-the-upstream-provenance-file-fix.md),
  [0084](../decisions/0084-provision-the-publish-stage-npm-cli-from-a-digest-verified-registry-tarball.md),
  [0085](../decisions/0085-pin-the-node-24-patch-version-and-assert-the-expected-bundled-npm-pair.md),
  [0099](../decisions/0099-record-the-digest-verified-nodejs-runtime-in-resolveddependencies.md),
  [0101](../decisions/0101-relay-external-cli-stderr-to-logs-under-hygiene-bounds-with-classification-as-enrichment.md),
  and
  [0102](../decisions/0102-classify-npm-e422-publish-rejection-as-determinate-under-reduced-readback-budget.md),
  [0103](../decisions/0103-define-external-workflow-parameters-as-common-invocation-identity.md),
  [0104](../decisions/0104-remove-redundant-external-parameter-builder-record.md),
  [0105](../decisions/0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md),
  [0106](../decisions/0106-consolidate-external-source-parameters-onto-workflow-and-descriptor-bindings.md),
  [0107](../decisions/0107-record-invocation-event-metadata-in-internalparameters-github.md),
  [0108](../decisions/0108-remove-redundant-npm-release-asset-inputs.md),
  [0109](../decisions/0109-minimize-external-parameters-to-actual-interface-inputs.md),
  [0110](../decisions/0110-record-deterministic-media-types-for-non-runner-dependencies.md),
  [0111](../decisions/0111-record-runner-image-reported-identity-with-included-software-uri-and-full-image-version.md),
  and [0112](../decisions/0112-record-runner-image-operating-system-from-on-disk-metadata.md)
- Related specs: [Core profile contract](core-profile-contract.md),
  [Identity and build types](identity-and-buildtypes.md),
  [SLSA provenance v1](slsa-provenance-v1.md),
  [JS/TS npm package profile](js-ts-npm-package-profile.md),
  [JS/TS npm build and pack](js-ts-npm-build-pack.md),
  [npm-to-release-asset composition](npm-to-release-asset-composition.md)

## Scope and non-goals

**In scope:**

- Three-job publish graph.
- Artifact and provenance handoff between jobs.
- npm package subject naming and digest semantics.
- Windlass-generated SLSA provenance.
- `npm publish --provenance-file` behavior.
- Producer-side verification gate.
- Job-class concurrency and mutation-segment entry checks.
- Same-run npm publish convergence.
- Workflow outputs.

**Out of scope:**

- Package manager selection and build commands (build and pack spec).
- GitHub Release asset upload (publisher spec).
- Consumer verifier implementation (verification policy spec).

## Job graph

In npm-only mode, the profile uses exactly these three jobs:

```text
build -> provenance-sign -> publish
```

In release-asset mode, the complete same-run graph is:

```text
build -> provenance-sign -> composition-map -> publish -> release-upload
```

`composition-map` is the read-only composition mapping and release-state preflight job defined by
the [composed workflow internal handoff](composed-workflow-internal-handoff.md) and
[npm-to-release-asset composition](npm-to-release-asset-composition.md). `release-upload` is the
single GitHub Release asset mutation-segment job. It uploads or converges on the provenance sidecar
and tarball only after `publish` succeeds or reaches same-run read-only convergence. No mode may add
an edge that permits `publish` before `provenance-sign`, `composition-map` before `provenance-sign`,
or `release-upload` before `publish`.

### `build`

- Runs install, optional build, and pack.
- Produces the npm package tarball and metadata.
- Uploads the tarball as a workflow artifact.
- Exposes the tarball name, SHA-256, and SHA-512 to downstream jobs.
- Uploads the build metadata artifact and exposes its artifact name and SHA-256 to `provenance-sign`
  through internal same-run job outputs.
- Must not have signing or publish permissions.

### `provenance-sign`

- Downloads the tarball artifact.
- Recomputes the tarball digest and verifies it against the handoff.
- Constructs and verifies the complete SLSA provenance v1 Statement.
- Signs those exact Statement bytes through the Go-native `sigstore-go` DSSE adapter.
- Uploads the signed bundle as a workflow artifact.
- Permissions:
  - `contents: read`
  - `id-token: write`
- Must not have `contents: write` or npm publish authority.

### `publish`

- Downloads the tarball, signed bundle, and the same digest-bound build metadata artifact.
- Recomputes both digests and verifies them against the handoff.
- Verifies the provenance signature, signer identity, `builder.id`, `buildType`, subject digest,
  subject name, and `externalParameters`.
- Verifies the build metadata handoff and its manifest evidence before both local bundle
  verification and registry read-back verification; unavailable or mismatched evidence stops
  verification.
- Provisions the pinned publish-stage npm CLI before any npm invocation in the job, including
  diagnostics, and validates the provisioned npm against the publish allowlist pin.
- Publishes the tarball using `npm publish --provenance-file`.
- Verifies registry metadata after publish.
- Permissions:
  - `contents: read` for source checkout if needed.
  - `id-token: write` for OIDC trusted publishing.
- Must not have `attestations: write` or re-signing authority.

## Job-class concurrency and mutation-segment entry

This section specifies the npm mutation boundary required by
[ADR 0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md) and the
same-run recovery qualification required by
[ADR 0067](../decisions/0067-converge-repeated-runs-within-run-identity.md).

The `publish` job is mutation-class. It must declare job-level concurrency with
`cancel-in-progress: false` and `queue: max`; omission or any other value is a conformance failure
that must be detected before the workflow is accepted for release use. The `build` and
`provenance-sign` jobs are PRE-mutation jobs and must use separate job-level groups with
`cancel-in-progress: true` and no `queue` key because they hold no registry mutation authority.
Omission, a `false` value, a `queue` key, or placement in the mutation group must cause static
workflow conformance to reject the workflow before release use. `queue: max` combined with
`cancel-in-progress: true` is a platform validation error and must be rejected statically.

The exact mutation concurrency group is:

```text
release-mutation-${{ github.repository }}-${{ inputs.source-ref || github.ref }}
```

The mutation concurrency group represents one release intent and is composed only from the literal
namespace plus `github.repository` and the canonical full built ref
`${{ inputs.source-ref || github.ref }}`. PRE-mutation groups retain job-specific namespaces. Any
other context or input in the mutation key is a conformance failure and must cause workflow
validation to reject the release configuration. `github.workflow` must not appear in this or any
called-workflow concurrency key: in a reusable workflow it resolves to the caller's workflow name
and creates a self-cancellation trap. Detection of `github.workflow` in a key must reject the
workflow before release execution.

All mutation jobs participating in one public profile invocation use the same mutation group, with
`cancel-in-progress: false` and `queue: max`, so a contender waits rather than interrupts registry
or release mutation. The platform queues mutation-segment contenders in arrival order rather than
replacing pending executions. Pre-mutation jobs retain `cancel-in-progress: true` with no `queue`
key, so stale compute remains eligible for early cancellation.

At entry to the serialized mutation segment, the `publish` job must revalidate the npmjs package
identity precondition, classify the package-version state under the four-state convergence contract
below, and rerun every producer-side verification gate that could have become stale while queued.
The job must make no registry mutation until those checks complete. A missing package identity, a
failed producer-side gate, `foreign-conflict`, or `indeterminate` must fail closed before mutation;
`absent` permits the publish call, while `committed-as-expected` permits continuation only for a
retry attempt within the same `github.run_id`.

## Job permissions summary

| Job               | `contents` | `id-token` | `attestations` |
| ----------------- | ---------- | ---------- | -------------- |
| `build`           | read       | none       | none           |
| `provenance-sign` | read       | write      | none           |
| `publish`         | read       | write      | none           |

The initial npmjs production path must not request `packages: write`. A custom registry that needs
GitHub Packages or another package-write permission is outside the initial guaranteed production
surface unless a later ADR and profile spec define that registry class and its permission boundary.

## Tarball artifact handoff

The `build` job uploads the tarball as a workflow artifact with this deterministic name:

```text
js-ts-npm-package-tarball-<github.run_id>-<github.run_attempt>
```

The artifact must contain exactly one file: the pack-produced `.tgz` package tarball. The
`provenance-sign` job downloads the artifact and recomputes its digest.

The tarball handoff must satisfy the core same-run artifact handoff schema:

| Core handoff field  | Value                                                                |
| ------------------- | -------------------------------------------------------------------- |
| `transport`         | `github-actions-artifact`                                            |
| `artifact_name`     | `js-ts-npm-package-tarball-<github.run_id>-<github.run_attempt>`     |
| `payload_file_name` | Basename of the single pack-produced `.tgz` file in the artifact.    |
| `payload_kind`      | `primary-artifact`                                                   |
| `digest.algorithm`  | `sha256`                                                             |
| `digest.value`      | SHA-256 of the tarball bytes as 64 lowercase hexadecimal characters. |

The handoff also carries the tarball SHA-512 as lowercase hexadecimal for npm diagnostics and the
public `package-tarball-sha512` output, but SHA-512 is not the cross-job handoff digest algorithm.

The tarball handoff contains values computed from the local packed tarball bytes before publish. It
must not require npm registry metadata, because registry metadata exists only after registry
mutation. If an implementation derives an SRI string from the local tarball bytes for diagnostics,
it must treat that value as a local diagnostic equivalent of the SHA-512 digest, not as registry
evidence.

## Build metadata handoff

The tarball artifact remains exactly one `.tgz` file. Build metadata must therefore travel in a
separate same-run GitHub Actions artifact, not in the tarball artifact or as a public
`workflow_call` output. The metadata artifact pattern is required because the composed-workflow
internal handoff contract reserves internal job outputs for producer-owned artifact names and
digests, then requires consumers to verify the named one-file artifact before reading its contents.
Job outputs alone are not an acceptable metadata transport.

The build job must upload this deterministic artifact:

```text
js-ts-npm-build-metadata-<github.run_id>-<github.run_attempt>
```

It must contain exactly one file named `build-metadata.json`. The build job must compute the SHA-256
of those file bytes and expose both the artifact name and digest through internal same-run job
outputs. `provenance-sign` must retrieve the named artifact from the same workflow run, require the
one-file shape and filename, recompute the SHA-256, and fail before signing if either output is
missing, malformed, or differs from the computed value. These outputs must not be exposed as public
workflow outputs.

`build-metadata.json` is a closed JSON object with this required shape:

```json
{
  "schema_version": "1",
  "started_on": "2026-08-14T11:59:58Z",
  "finished_on": "2026-08-14T12:00:03Z",
  "primary_artifact": {
    "artifact_name": "js-ts-npm-package-tarball-123456789-1",
    "payload_file_name": "windlass-slsa-builder-1.2.3.tgz",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "sha512": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "external_parameters": {},
  "resolved_dependencies": [],
  "manifest_evidence": {
    "path": "package.json",
    "content_base64": "eyJuYW1lIjoiZXhhbXBsZSIsInZlcnNpb24iOiIxLjIuMyJ9"
  },
  "build_script_disposition": "skipped-absent"
}
```

All eight top-level members are required. The example is a transport-shape illustration, not a
complete conforming artifact: replace the empty `external_parameters` and `resolved_dependencies`
with the complete objects below and bind their identities and digests to the actual bytes.
`schema_version` is the string `"1"`. `started_on` and `finished_on` are the observed build start
and completion times, recorded by the build job in whole-second UTC RFC 3339 form
(`YYYY-MM-DDTHH:MM:SSZ`). The signing job carries these values verbatim into
`predicate.runDetails.metadata.startedOn` and `finishedOn`, respectively, rather than recording
signing time. Missing timestamp members fail closed-schema decoding; timestamp format and ordering
follow the [common metadata contract](slsa-provenance-v1.md#metadata).
`primary_artifact.artifact_name` is the deterministic tarball artifact name, and
`primary_artifact.payload_file_name` is the tarball basename. `primary_artifact.sha256` and
`primary_artifact.sha512` are respectively 64 and 128 lowercase hexadecimal digests of that one
tarball's bytes. `external_parameters` is the complete closed object specified in
[JS/TS npm `externalParameters` schema](#jsts-npm-externalparameters-schema), with every required
member present and only that schema's permitted optional members. `resolved_dependencies` is the
complete array of closed ResourceDescriptor values specified in
[JS/TS npm `resolvedDependencies` schema](#jsts-npm-resolveddependencies-schema).

`build_script_disposition` is exactly `executed` or `skipped-absent`, observed by the build job and
carried verbatim into `runDetails.metadata.windlass_build_script_disposition`. Missing or
manifest-inconsistent disposition fails with
`windlass.verify.error.build-script-disposition-mismatch`.

### Offline manifest evidence

This section owns the mandatory manifest-evidence delivery contract for every manifest-dependent
producer or consumer check. `manifest_evidence` is exactly one closed object with the two required
string members `path` and `content_base64`; an array, multiple objects, duplicate members, or an
extra member is invalid. The build job captures the original selected `package.json` bytes before
any install, build, or pack script executes. Neither a packed manifest nor a later source-file read
can replace that snapshot.

- `path` is the repository-root-relative selected manifest path: `package.json` when
  `package.directory` is `.`, otherwise `<package.directory>/package.json`. Absolute paths,
  traversal, backslashes, empty paths, and paths outside the selected directory are rejected.
- `content_base64` is canonical standard RFC 4648 base64, with required padding when applicable and
  no whitespace. URL-safe alphabet, noncanonical padding or pad bits, and malformed encodings are
  rejected. Decoding and re-encoding with the standard padded encoding must reproduce the exact
  string; otherwise verification fails.
- Decoded bytes are bounded to 16 MiB (16,777,216 bytes), with the bound enforced before allocating
  an unbounded decode. Larger evidence is rejected. Parsing uses the existing strict, bounded
  manifest rules, including duplicate-member rejection; valid base64 is not manifest validation.
- SHA-256 of the decoded bytes must equal `package-manifest.digest.sha256`. The evidence path must
  equal both the selected-directory derivation above and the descriptor's canonically encoded URI
  fragment under the lockfile path-encoding rule. The descriptor repository must equal
  `workflow.repository`, and its commit must equal `built-source.digest.gitCommit`. Any disagreement
  rejects the evidence before its contents can supply an identity or configuration value.

`provenance-sign` and `publish` retrieve and digest-verify the same build-metadata handoff before
using this object. The publish stage supplies that verified evidence to both local bundle
verification and each registry read-back candidate's verification, including same-run convergence.
Downstream offline consumers supply exactly this evidence object alongside the bundle and tarball;
its digest and source bindings are verified against the signed descriptor even when the consumer
does not have access to the same-run artifact transport. No consumer fetches the manifest over the
network or silently skips a manifest-dependent check.

The object in the transport example is valid evidence encoding for the exact UTF-8 bytes
`{"name":"example","version":"1.2.3"}` without a trailing newline, provided its path, digest, and
source bindings match. It illustrates encoding only; the full manifest must also pass repository and
package validation. For a complete valid handoff, use the accepted external-parameter example, the
complete selected-manager descriptor set, and evidence for the validated source manifest whose
SHA-256 is recorded in that set. The empty object/array placeholders above are rejected if
submitted.

This focused evidence object is invalid because the base64 contains whitespace:

```json
{ "path": "package.json", "content_base64": "e30=\n" }
```

Missing, malformed, oversized, path-mismatched, commit-mismatched, or digest-mismatched evidence
fails with `windlass.verify.error.package-manifest-evidence`, exit code `1`. Before signing or
mutation it stops that operation; during post-publish read-back it fails verification under the
standing indeterminate/partial-publication rules without republishing. Handoff byte or
artifact-shape failures also remain fatal under the same-run handoff contract. Manifest evidence is
not embedded as descriptor `content`, and no `downloadLocation` or network fallback is introduced.

`provenance-sign` must construct its candidate predicate only from the verified metadata object, the
verified downloaded tarball, and signing-job observations required by this specification. It must
reject duplicate JSON members, unknown members at every closed-schema level, malformed types, or
disagreement among the metadata artifact, tarball handoff, downloaded tarball, and signing-job
observations before signing. The metadata artifact must not contain a provenance bundle, signed
Statement, registry state, release-state preflight result, credential, token, or remote publication
result.

## Provenance bundle handoff

The `provenance-sign` job uploads the signed bundle as a workflow artifact with this deterministic
name:

```text
js-ts-npm-provenance-bundle-<github.run_id>-<github.run_attempt>
```

The artifact must contain exactly one file with this deterministic basename:

```text
<package-tarball-name>.intoto.jsonl
```

The file contents are the signed Sigstore bundle emitted by the Go-native DSSE signer for the
package tarball. The `publish` job downloads the bundle and recomputes its digest.

The signed bundle file is the exact byte sequence emitted by the Go-native DSSE signer. The profile
must hand off and submit those bytes unchanged; it must not replace the file with an extracted
Statement, reserialized bundle, or GitHub-attestation-storage locator. See the
[SLSA provenance v1 signed bundle file format](slsa-provenance-v1.md#signed-bundle-file-format).

The provenance bundle handoff must satisfy the core same-run artifact handoff schema:

| Core handoff field  | Value                                                                      |
| ------------------- | -------------------------------------------------------------------------- |
| `transport`         | `github-actions-artifact`                                                  |
| `artifact_name`     | `js-ts-npm-provenance-bundle-<github.run_id>-<github.run_attempt>`         |
| `payload_file_name` | `<package-tarball-name>.intoto.jsonl`                                      |
| `payload_kind`      | `provenance-bundle`                                                        |
| `digest.algorithm`  | `sha256`                                                                   |
| `digest.value`      | SHA-256 of the signed bundle bytes as 64 lowercase hexadecimal characters. |

The `.intoto.jsonl` basename is a distribution name for the emitted Sigstore bundle bytes. It must
not be interpreted as permission to upload a raw in-toto Statement, extracted predicate, GitHub
attestation storage locator, or any normalized representation in place of the bundle file.

## Digest verification between jobs

Every receiving job must:

1. Download the artifact.
2. Recompute the digest using the algorithm specified in the handoff.
3. Compare the computed digest with the expected digest.
4. Fail closed on mismatch.

## npm package subject naming

The `subject[0].name` in the provenance Statement is the npm Package URL for the package version
being published, not the packed tarball file name. The subject name must exactly match the Package
URL that npm CLI derives for `npm publish --provenance-file`.

For example:

```text
pkg:npm/%40windlass/slsa-builder@1.2.3
```

Package name and version come from the verified original manifest evidence and must agree with the
subject PURL and the digest-verified tarball's `package/package.json`. Any disagreement fails before
signing or publication. The tarball filename remains verifier-relevant through the actual pack
output, the tarball artifact handoff, and any GitHub Release asset handoff. It is not derived from
the PURL and is not the npm provenance Statement subject name.

The profile must fail before signing when the tarball name is empty, contains a path separator, is
not the basename of the pack-produced tarball path, or does not end in `.tgz`.

The profile must fail before signing when it cannot derive an npm Package URL subject from the
validated package name and version, when the derived subject is empty or malformed, or when the
subject differs from the npm Package URL that npm CLI will validate for the package being published.

The normative derivation is `pkg:npm/` followed by the package name and version in Package URL form.
For an unscoped package, it is `pkg:npm/<encoded-name>@<encoded-version>`. For a scoped package
`@<scope>/<name>`, it is `pkg:npm/%40<encoded-scope>/<encoded-name>@<encoded-version>`. Each
`encoded-*` value percent-encodes every UTF-8 byte outside RFC 3986 unreserved characters `A-Z`,
`a-z`, `0-9`, `-`, `.`, `_`, and `~`, using uppercase hexadecimal. The slash between a scoped
package's scope and name is structural and is not encoded. The literal scope marker `@` is encoded
as `%40`.

For example, validated `@windlass/slsa-builder` version `1.2.3` derives
`pkg:npm/%40windlass/slsa-builder@1.2.3`. The producer must derive this value before signing and
must verify that it exactly equals the Package URL accepted by the npm CLI version recorded in
`runDetails.builder.version.npm`. The supported npm CLI version set for the compatibility fixture
obligation is enumerable: every entry of the
[publish-stage npm CLI allowlist](#publish-stage-npm-cli-allowlist) plus the build-stage npm bundled
with the pinned Node.js 24 toolchain pair
([ADR 0085](../decisions/0085-pin-the-node-24-patch-version-and-assert-the-expected-bundled-npm-pair.md)).
A compatibility fixture is required for every version in that set and must cover both scoped and
unscoped names. Any CLI-version variance from this derivation is a production conformance failure
until this specification and its compatibility fixtures are updated; an implementation must not
silently delegate the subject value to the CLI.

## JS/TS npm `externalParameters` schema

The complete npm interface is closed to the object below. `source` is the only optional group. The
common [invocation-interface contract](slsa-provenance-v1.md#externalparameters) owns the meaning of
`workflow`; this profile does not redefine it or introduce a registry-specific projection. Missing
required fields, wrong types, and unexpected members fail before signing or mutation and are
rejected by consumers.

```json
{
  "source": {
    "ref": "refs/tags/v1.2.3"
  },
  "workflow": {
    "ref": "refs/heads/main",
    "repository": "https://github.com/example/project",
    "path": ".github/workflows/publish.yml"
  },
  "package": {
    "directory": "."
  },
  "publish": {
    "input_registry_url": null,
    "input_dist_tag": null,
    "input_access": null
  },
  "distribution": {
    "release_asset_mode": false,
    "linked_artifact_metadata": false
  }
}
```

### Closed schema rules

Required groups are exactly `workflow`, `package`, `publish`, and `distribution`. Optional `source`,
when present, contains exactly the required string `ref`; it is never `null` or `{}`. The other
groups contain exactly the members shown above. `workflow` has three required strings, `package` has
one required string, `publish` has three required nullable strings, and `distribution` has two
required booleans. No additional members are permitted at any level. Duplicate JSON members are
rejected before semantic validation.

The following removed representations are explicitly rejected, not relocated into aliases:

- `source.repository`, `revision`, `ref_type`, `input_ref`, `invocation_ref`, `invocation_revision`,
  and `event_name`; the former required built-ref meaning of `source.ref` is also retired. Its only
  meaning is the optional caller selector below.
- `workflow.sha` and `workflow.builder_id`, or the reusable builder path used as `workflow.path`.
- The entire `runtime`, `package_manager`, `release`, `caller`, and `build` groups.
- Every `package` member other than `directory`, including name, version, private status,
  repository, workspace root, source manifest, tarball name, package URL, packed identity, packed
  files, consumer surface, and raw publish configuration.
- Every `publish` member other than its three raw inputs, including `publish_config`,
  `resolved_registry_url`, `resolved_dist_tag`, `publish_access_option`, `effective_access`,
  `trusted_publishing`, `provenance_file`, `custom_registry_support`, and pre-existing-state flags.
- `distribution.release_tag_supplied`, `distribution.provenance_sidecar`, or any raw release tag.

Missing required fields use `windlass.verify.error.external-parameters-incomplete`; unexpected
members use `windlass.verify.error.external-parameters-unexpected`. Type or value violations fail
the applicable field rule. The common contract never supplies a fallback group.

### Field rules

- `package.directory` equals the validated repository-root-relative `package-directory` selector. A
  different or unsafe directory fails package selection before signing and consumer verification.
- The three `publish.input_*` members record the caller's raw non-empty strings, not resolved
  values. Omission or an empty string after trimming ASCII whitespace records `null`; workflow
  defaults must not manufacture suppliedness. Normalization for resolution does not overwrite a
  retained raw value.
- The two distribution booleans equal the accepted caller options. Disagreement or incompatible
  modes fail with `windlass.verify.error.release-asset-mode-schema-error`. The current npm-only
  implementation continues to reject enabled but unimplemented distribution modes; this schema does
  not claim composition is implemented.
- The caller workflow filename for trusted-publisher authorization is the basename of the
  certificate-bound `workflow.path`. Missing or mismatched caller identity fails with
  `windlass.verify.error.trusted-publisher-mismatch`; no duplicate filename field is signed.

### Effective built ref derivation

Producers apply all of these rules before construction; consumers independently recompute and check
the effective built identity:

1. `source` and its sole `ref` member are present if and only if the caller supplied a non-empty
   `source-ref`. Only the exact empty input string counts as omission. Producers compare
   suppliedness with the actual workflow input. An empty object, null, or a selector invented on
   omission fails.
2. A present `source.ref` preserves exactly the full canonical `refs/tags/<tag-name>` selector.
   Whitespace, short tags, branches, pull request refs, and malformed refs fail with
   `windlass.verify.error.source-ref-invalid`; no trimming or short-ref repair is allowed.
3. If present, `source.ref` is the effective built ref; otherwise `workflow.ref` is. If
   `workflow.ref` starts with `refs/tags/`, including dispatch on a tag, a present different
   `source.ref` is rejected. An equal explicit selector remains present and is accepted.
4. `built-source.uri` equals `git+<workflow.repository>@<effective built ref>` byte-for-byte,
   without a fragment. Its `digest.gitCommit` is the verified resolved commit from the
   isolated-fetch and peel-to-commit procedure. A different URI or commit fails the built-source
   descriptor check.
5. ADR 0079's version-tag matching remains mandatory: the effective built ref is the packed
   package's version tag under the existing release guards. A non-tag or version mismatch fails
   before signing and is rejected by consumers.
6. When `source` is absent, both source descriptors have identical URI and digest values. This is an
   enforced equality, not an inference from matching ref strings; disagreement fails verification.

The complete example above is valid for a dispatch retry from `main` building `v1.2.3`. A valid
tag-push variant changes `workflow.ref` to `refs/tags/v1.2.3` and omits `source` entirely. An
explicit equal tag selector on that run is also valid, including on dispatch. A retry may resolve
different refs to the same commit without losing either source role.

Focused invalid selector examples (each rejects) are:

```json
{ "source": {} }
```

```json
{ "source": { "ref": " v1.2.3 " } }
```

### Manifest-derived checks and publish resolution

Every manifest-dependent check first verifies
[offline manifest evidence](#offline-manifest-evidence). The authenticated source manifest supplies
name, version, private status, repository, `publishConfig`, and whether `scripts.build` exists.
Private packages fail the existing package validation gate. The digest-verified tarball supplies
packed name/version, packed files, and consumer surface; its identity must match the source manifest
and the subject PURL, or verification fails. The filename comes only from pack output and the
verified handoff, never a name/version derivation.

Consumers recompute publish settings from the three signed raw inputs and authenticated
`publishConfig` using the public profile's validation and these resolution rules:

1. Treat empty raw inputs as omitted as above. Validate and normalize each non-empty input and its
   corresponding manifest setting. If both are present and normalize differently, reject the
   conflict before signing or publication rather than override the manifest.
2. Select the normalized caller value when present, otherwise the manifest value, otherwise the
   documented default: `https://registry.npmjs.org/` for registry, `latest` for dist-tag, and no
   `--access` option for access. Access is `public`, `restricted`, or omitted. Omission means
   `existing-package-access`, not permission to create an identity or change its access.
3. The effective registry is absolute HTTPS with lowercase scheme/host, no userinfo, query, or
   fragment, no default port `443`, root path `/`, and exactly one trailing slash. Invalid registry,
   dist-tag, or access values fail the existing publish-intent validation; non-empty access must
   never be silently discarded.
4. For verifier-relevant manifest publish intent, evaluate `registry`, `tag`, `access`, and
   `provenance` with their existing string/boolean types. Reject `publishConfig.directory` before
   pack. Other source members remain non-policy metadata and do not cause rejection solely by
   existing. No manifest setting may enable automatic provenance or override exact bundle
   submission.
5. Derive the registry package-version URL from the effective registry, verified name, and version
   under the public profile's `package-url` rule; it is distinct from the subject PURL.

For example, all three null inputs with no `publishConfig` resolve to npmjs, `latest`, and omitted
access. Null access with manifest `access: "public"` resolves to `--access=public`. A caller
`input_access: "restricted"` against that manifest is rejected. Resolutions are not signed duplicate
fields. Trusted publishing, external provenance-file submission, and the npmjs-only support
guarantee remain buildType behavior, not assertions that a candidate can toggle.

The required signed `runDetails.metadata.windlass_build_script_disposition` is `executed` when the
declared build script ran and `skipped-absent` when none was declared; declared means a non-empty
`scripts.build` command, so an explicit empty string counts as not declared. Missing, unsupported,
or manifest-inconsistent disposition fails with
`windlass.verify.error.build-script-disposition-mismatch`. For example, a verified manifest without
`scripts.build` and `skipped-absent` is valid; the same manifest with `executed` is invalid. Build
failures never produce successful provenance.

Package identity/version pre-existence observations belong only to unsigned `diagnostic_metadata`.
Their consumer-side signed expectations are retired; the accepted builder revision's mandatory
preflights and mutation-entry convergence gates remain in force. Immutable Yarn installation is
fixed buildType behavior, with no signed constant or replacement consumer field check.

### Seven-input completeness mapping

All paths below are relative to `externalParameters`. The seven retained public inputs map as
follows:

| Public input               | Signed location                                                 |
| -------------------------- | --------------------------------------------------------------- |
| `package-directory`        | `package.directory`                                             |
| `source-ref`               | Optional `source.ref`, present exactly for a non-empty selector |
| `registry-url`             | `publish.input_registry_url`                                    |
| `dist-tag`                 | `publish.input_dist_tag`                                        |
| `access`                   | `publish.input_access`                                          |
| `release-asset-mode`       | `distribution.release_asset_mode`                               |
| `linked-artifact-metadata` | `distribution.linked_artifact_metadata`                         |

The removed public `release-tag` and `provenance-sidecar` inputs are rejected and have no signed
projection. No suppliedness or sidecar-required alias replaces them. Release-asset mode mandates the
unchanged verified bundle sidecar and derives the Release target from the effective built tag.
Sidecar bytes, deterministic naming, ordering, partial-failure behavior, and locator outputs remain
mandatory; registry-native provenance and attestation locators are not substitutes.

A valid distribution object appears in the complete example above. This focused object is invalid
because a required member is missing:

```json
{ "distribution": { "release_asset_mode": false } }
```

This object is invalid because the removed member remains forbidden even when its value restates
mandatory behavior:

```json
{
  "distribution": {
    "release_asset_mode": true,
    "linked_artifact_metadata": false,
    "provenance_sidecar": "required"
  }
}
```

A `caller` group or `workflow.sha` is likewise invalid under the closed schema. The complete
external-parameter example with `package: {"directory":".","name":"example"}` is rejected, even when
that name equals the manifest's name.

### Canonical source repository URL

For GitHub-hosted source repositories, `workflow.repository` is canonicalized from the repository
owner and name observed in the release workflow context. Implementations must derive exactly this
form:

```text
https://github.com/<owner>/<repo>
```

Canonicalization rules:

- The scheme must be `https` and the host must be exactly `github.com` after lowercase
  normalization.
- The path must contain exactly two non-empty segments: `<owner>` and `<repo>`. Both segments must
  be lowercased in the emitted canonical URL.
- The output must not have a trailing slash, `.git` suffix, query, fragment, userinfo, port, or
  extra path segment.
- Backslashes, percent-encoded path separators, empty path segments, `.` or `..` segments, and ASCII
  control characters are invalid.
- Comparisons for GitHub repository identity must treat owner and repository names
  case-insensitively before emitting the lowercase canonical URL.

Examples that canonicalize to `https://github.com/windlasstech/example`:

- `https://github.com/WindlassTech/Example`
- `https://github.com/WindlassTech/Example.git`
- `HTTPS://github.com/WindlassTech/Example/`

Rejected examples include `git@github.com:WindlassTech/Example.git`,
`https://github.com/WindlassTech/Example/releases`,
`https://github.com/WindlassTech/Example?tab=readme`,
`https://github.com/WindlassTech/Example.git/extra`, and
`https://github.com/WindlassTech/%2E%2E/Example`.

Producer-side verification must reject the bundle before publish when `workflow.repository` is
missing, cannot be canonicalized by these rules, or differs from the observed caller repository
identity after case-insensitive GitHub owner/repository comparison.

The authenticated manifest's repository is normalized using the accepted manifest forms in
[repository identity validation](js-ts-npm-build-pack.md#repository-identity-validation). Its result
must equal `workflow.repository`, the certificate Source Repository URI, and both source
descriptors' URI repository components. Missing, malformed, or unequal manifest repository identity
fails with `windlass.verify.error.package-repository-identity-mismatch`. Certificate and descriptor
mismatches also fail their respective identity checks. Numeric repository and owner identifier
bindings remain mandatory and context-independent.

For example, a manifest repository accepted and normalized to `https://github.com/example/project`,
the complete external-parameter example above, a certificate naming that repository, and both source
URIs beginning `git+https://github.com/example/project@` satisfy this chain. The same manifest with
repository `https://github.com/example/other-project` fails even if the subject name and version
match.

### Release ref equality

`release.ref` and `release.version_tag` are removed and rejected. Release identity is the recomputed
effective built ref plus `built-source.digest.gitCommit`. Policy release-tag expectations compare
with that ref and the `built-source` URI ref component; commit expectations compare with its digest.
The short Release tag is the suffix after removing the exact `refs/tags/` prefix. The internal
handoff still carries that derived tag, and the standalone publisher compares its release-tag input
and existing caller-repository Release with the same verified identity. No independent target or
caller-supplied identity can override it.

For example, effective ref `refs/tags/v1.2.3`, a matching built-source descriptor, and handoff tag
`v1.2.3` agree. Handoff tag `v1.2.4`, a branch ref, a short selector, or a differing resolved commit
fails before registry or release mutation. The same comparisons apply to consumers. The mandatory
sidecar remains the exact verified producer bundle, with its existing digest and filename binding.

## JS/TS npm `resolvedDependencies` schema

The JS/TS npm package profile emits a closed set of SLSA v1 `ResourceDescriptor` values. The
producer serializes that set in deterministic order: `invocation-source`, `built-source`,
`package-manifest`, `lockfile`, `node-runtime`, the conditional `package-manager-distribution`, then
`runner-image`. This file owns that emission order. `invocation-source` occupies index 0 as an npm
emission compatibility rule, not a semantic set rule. Order is fixed before signing; changing it
after signing fails exact-payload comparison. Consumers treat `resolvedDependencies` as an
unordered, name-keyed set and select descriptors by `name`, never by array position. The complete
cardinality is:

- npm: exactly six descriptors, one of each unconditional role listed above;
- pnpm or Yarn: exactly seven descriptors, adding one `package-manager-distribution`.

Duplicate names are rejected before name-keyed lookup. A permutation of a valid set is semantically
valid for consumers; an npm emitter that violates the specified order is non-conforming. No entry
carries `content` or `downloadLocation`. The common
[deterministic media-type matrix](slsa-provenance-v1.md#deterministic-media-types) governs concrete
representations, with no generic fallback, remote Content-Type lookup, or network repair. Missing,
incorrect, or forbidden media types fail the owning descriptor check.

An unknown descriptor name, including `npm-cli`, a generated transitive-package entry, or any other
non-enumerated extra fails with `windlass.verify.error.resolved-dependencies-unexpected-entry`.
`package-manager-distribution` under npm remains a known-but-forbidden descriptor and fails with
`windlass.verify.error.resolved-dependencies-package-manager-distribution`. A known descriptor's
missing, duplicate, malformed, or mismatched form fails with that descriptor's narrower diagnostic
below.

### Source descriptors

The exact source shapes and their common certificate bindings are defined in
[Git source descriptors](slsa-provenance-v1.md#git-source-descriptors). For npm, `invocation-source`
has exactly `name`, `uri`, and `digest`; `built-source` additionally requires `annotations` with
exactly `ref_type: "tag"`. Both digest objects contain only `gitCommit`, a full 40-character
lowercase hexadecimal commit. Neither source accepts `mediaType` or a path fragment;
invocation-source also forbids annotations.

These valid retry descriptors correspond to the external-parameter example:

```json
[
  {
    "name": "invocation-source",
    "uri": "git+https://github.com/example/project@refs/heads/main",
    "digest": { "gitCommit": "89abcdef0123456789abcdef0123456789abcdef" }
  },
  {
    "name": "built-source",
    "uri": "git+https://github.com/example/project@refs/tags/v1.2.3",
    "digest": { "gitCommit": "0123456789abcdef0123456789abcdef01234567" },
    "annotations": { "ref_type": "tag" }
  }
]
```

They are a focused subset, not a complete dependency array. Omitting either role when identities
coincide is invalid. Adding `"mediaType":"application/gzip"` to either object is invalid; removing
the built-source annotation or changing it to `"branch"` is invalid. A wrong invocation URI or
certificate commit fails `windlass.verify.error.resolved-dependencies-invocation-source`; a wrong
effective built URI, commit, or classifier fails
`windlass.verify.error.resolved-dependencies-built-source`. No position-based lookup can substitute
the invocation commit for the release-policy commit.

### Package-manifest descriptor

Exactly one descriptor has the closed shape `name`, `uri`, `digest`, `mediaType`, and `annotations`:

```json
{
  "name": "package-manifest",
  "uri": "git+https://github.com/example/project@0123456789abcdef0123456789abcdef01234567#package.json",
  "digest": {
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "mediaType": "application/json",
  "annotations": { "workspace_root": null }
}
```

The example's digest stands for the actual SHA-256 of the selected original source bytes. `digest`
contains only that 64-character lowercase hexadecimal `sha256`. The URI uses `workflow.repository`,
the built-source commit, and the selected manifest path under the lockfile percent-encoding rule
below. `annotations` contains exactly `workspace_root`: null for standalone packages or the
validated repository-root-relative workspace directory (including `.` for a root workspace).
Workspace membership follows build-and-pack selection, not a caller assertion.

For example, `package.directory: "packages/widget"` requires fragment
`#packages/widget/package.json` and evidence path `packages/widget/package.json`; using
`#package.json` instead is invalid. Wrong media type, digest shape, commit, path, workspace
classification, unknown annotation, `content`, or `downloadLocation` fails
`windlass.verify.error.resolved-dependencies-package-manifest`. A well-shaped descriptor with absent
or mismatched decoded bytes fails `windlass.verify.error.package-manifest-evidence`. Both stop
manifest-dependent verification; neither permits retrieval or substitution of a packed manifest.

### Lockfile descriptor

The selected lockfile has exactly `name`, `uri`, `digest`, `mediaType`, and `annotations`:

```json
{
  "name": "lockfile",
  "uri": "git+https://github.com/example/project@0123456789abcdef0123456789abcdef01234567#pnpm-lock.yaml",
  "mediaType": "application/yaml",
  "digest": {
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "annotations": {
    "package_manager": "pnpm",
    "package_manager_root": ".",
    "selection_source": "packageManager",
    "selection_manifest_path": "package.json",
    "selection_lockfile_path": "pnpm-lock.yaml",
    "stale_non_selected_lockfiles": []
  }
}
```

- `uri` must be
  `git+<workflow.repository>@<built-source.digest.gitCommit>#<selection_lockfile_path>`. A
  mismatched built commit fails even if the invocation commit matches.
- The URI fragment must be the RFC 3986 percent-encoded form of the repository-root-relative
  selected lockfile path. Producers must encode each `/`-separated segment and rejoin the encoded
  segments with `/`. Consumers must independently reconstruct the expected URI with that escaped
  fragment and compare the raw `uri` value byte-for-byte against it. Percent-decoding the fragment
  is an optional part of an equivalent consumer implementation, not a requirement; the canonical raw
  comparison still applies. The selected lockfile path must not be empty, absolute, contain path
  traversal, contain backslashes, point outside the repository, or name a non-selected lockfile.
- `digest.sha256` must be the SHA-256 digest of the selected lockfile bytes as 64 lowercase
  hexadecimal characters.
- Each path segment encodes UTF-8 bytes outside RFC 3986 unreserved characters using uppercase hex;
  structural `/` separators remain literal. The same rule applies to the package-manifest URI.
- `mediaType` is `application/json` for selected `package-lock.json`, or `application/yaml` for
  selected `pnpm-lock.yaml` and supported Berry v4/v5 `yarn.lock`. Shrinkwrap and Yarn Classic are
  not admitted; stale non-selected files receive no descriptors or media-type assignments.
- `annotations` contains exactly the six members shown. `package_manager` is the selected `npm`,
  `pnpm`, or `yarn`; `package_manager_root` is the validated repository-root-relative install root.
- `selection_source` is `packageManager`, `devEngines.packageManager`, or `lockfile`, under the
  build-and-pack selection rules. Yarn lockfile inference is forbidden.
- `selection_manifest_path` is the repository-root-relative manifest that selected the manager,
  distinguishing workspace-root selection from selected-package selection, or null for lockfile
  inference. Selection observations come from the validated build selection, not removed parameters.
- `annotations.selection_lockfile_path` must be the repository-root-relative selected lockfile path
  for the package manager root, regardless of whether manifest metadata or lockfile inference
  selected the package manager.
- `annotations.stale_non_selected_lockfiles` is an array of repository-root-relative strings for
  supported non-selected lockfiles observed in the same manager root after manifest selection, or
  empty when none were ignored. These are diagnostics, not dependency graph inputs.

For lockfile-inferred npm, `selection_manifest_path` is null and `selection_lockfile_path`
identifies the selected `package-lock.json`. All other cases preserve the build-and-pack selection
provenance.

The profile must fail before signing when the selected lockfile descriptor is missing, has the wrong
digest, points to a non-selected or stale lockfile, contains extra entries, contains unknown
annotation members, omits captured stale lockfile diagnostics, has a missing or wrong media type, or
treats stale non-selected lockfiles as selected dependency graph inputs. The primary diagnostic
category is `resolved-dependencies-lockfile`, severity `error`, exit code `1`.

This focused invalid lockfile example fails with that diagnostic because the selected digest is not
64 lowercase hexadecimal characters:

```json
{
  "name": "lockfile",
  "uri": "git+https://github.com/example/project@0123456789abcdef0123456789abcdef01234567#pnpm-lock.yaml",
  "digest": { "sha256": "not-a-sha256" },
  "mediaType": "application/yaml",
  "annotations": {
    "package_manager": "pnpm",
    "package_manager_root": ".",
    "selection_source": "packageManager",
    "selection_manifest_path": "package.json",
    "selection_lockfile_path": "pnpm-lock.yaml",
    "stale_non_selected_lockfiles": []
  }
}
```

### Node.js runtime descriptor

Every manager selection requires exactly one `node-runtime` descriptor for the actual build
platform. The descriptor has exactly `name`, `uri`, `digest`, `mediaType`, and `annotations`. For
the currently supported `linux-x64` platform, its closed shape and exact values are:

```json
{
  "name": "node-runtime",
  "uri": "https://nodejs.org/download/release/v24.22.1/node-v24.22.1-linux-x64.tar.gz",
  "mediaType": "application/gzip",
  "digest": {
    "sha256": "eb6728e77c576a35162c122c45ccadf224d89ff00abc8dae46cc2897c6002d7c"
  },
  "annotations": {
    "digest_authority": "builder-pinned-record",
    "platform": "linux-x64"
  }
}
```

- `uri` is exactly the `linux-x64` artifact URL embedded in
  `internal/toolchainpins/node-runtime.json` for the applicable builder revision.
- `digest` contains exactly one member, `sha256`, whose 64 lowercase hexadecimal characters equal
  the embedded `linux-x64` SHA-256. The digest covers the compressed tarball bytes, not the
  extracted file tree and not a SHASUMS file.
- `annotations` contains exactly the two JSON strings shown above. `role`, `node_version`,
  `npm_version`, `acquisition_source`, and every other additional annotation are forbidden.
- `"builder-pinned-record"` identifies the repository-committed
  `internal/toolchainpins/node-runtime.json` record as the digest trust anchor. It does not mean
  registry integrity and does not claim runtime signature verification.
- `mediaType` is exactly `application/gzip` for the compressed `.tar.gz`, never an extracted tree.

The expected values belong to the pin embedded at the applicable builder revision. This contract
does not require retrieval of historical pin records across builder releases. Consumer verification
must compare the received descriptor against that embedded pin before checking equality with
accompanying expected metadata, so a descriptor and metadata corrupted to the same wrong values
still fail.

This descriptor records the build-stage Node.js runtime and transitively covers the build-stage npm
CLI bytes bundled in that runtime tarball. It does not introduce a separate `npm-cli` descriptor and
does not record the separately provisioned publish-stage npm CLI. Future platform expansion still
emits exactly one descriptor for the actual build platform, not one descriptor per supported
platform; it does not broaden the currently supported platform set.

The following descriptor is invalid because its URI differs from the embedded pin. Its digest
intentionally remains the expected pin value, so the URI is the only field that differs from that
pin:

```json
{
  "name": "node-runtime",
  "uri": "https://nodejs.org/dist/v24.0.0/node-v24.0.0-linux-x64.tar.gz",
  "mediaType": "application/gzip",
  "digest": {
    "sha256": "eb6728e77c576a35162c122c45ccadf224d89ff00abc8dae46cc2897c6002d7c"
  },
  "annotations": {
    "digest_authority": "builder-pinned-record",
    "platform": "linux-x64"
  }
}
```

After successful strict JSON and structural decoding, an unknown top-level member on this
descriptor, such as `downloadLocation` or a case-variant of a recognized member, is rejected with
`windlass.verify.error.resolved-dependencies-node-runtime`. The same diagnostic covers a missing or
duplicate runtime descriptor, a wrong URI, a wrong, malformed, or extra digest member, a missing,
unknown, wrong-type, or wrong-value annotation, missing or wrong media type, or exact metadata
mismatch. This is a verification diagnostic with severity `error`, exit code `1`, no mutation, and
default precedence. Existing strict JSON and structural decode failures retain their existing
diagnostics and precedence; not every JSON decode error maps to the runtime diagnostic. This
descriptor-local rejection does not change the common in-toto Statement envelope's tolerant handling
of unrecognized fields or the build metadata artifact's strict closed-schema decoding. Failure to
load the embedded pin is trusted input unavailable, reported as
`windlass.verify.error.input-unavailable` with exit code `2`, not as a malformed runtime descriptor.

### Package-manager distribution descriptor

This descriptor is required exactly once when pnpm or Yarn is selected and is forbidden when npm is
selected. It has exactly `name`, `uri`, `digest`, `mediaType`, and `annotations`; its only digest
member is `sha512`, exactly 128 lowercase hexadecimal characters. Its annotations contain exactly
`digest_authority`, `package_manager`, `package_manager_version`, and `acquisition_source`.
`package_manager` agrees with the lockfile selection. `package_manager_version` records the observed
selected distribution version under the build-and-pack selection contract, including a declared
`+<algorithm>.<hex>` suffix when applicable; the declaration does not replace the observed digest or
its authority. The current acquisition contract uses exactly `"corepack"` as `acquisition_source`.
Supported version ranges and digest-declaration validation remain those of the build-and-pack spec.
`mediaType` describes the actual digest-covered representation: `application/gzip` for registry
`.tgz` bytes and `text/javascript` only for the acquired single `yarn.js` bytes. A registry Yarn
tarball is not JavaScript media merely because it contains JavaScript. ADR 0110 does not change
acquisition or introduce a new dual-artifact schema; every represented registry tarball, including
any pnpm dual-artifact tarball admitted by its acquisition contract, uses `application/gzip`.

For pnpm, `uri` is the actual distribution URL used by Corepack and `digest_authority` is exactly
`"registry-integrity"`. The producer authenticates registry `dist.integrity`, requires its
`sha512-<base64>` SRI form, decodes the base64 digest to bytes, and hex-encodes those bytes as
`digest.sha512`; it must not store the SRI wrapper. A valid pnpm descriptor is:

```json
{
  "name": "package-manager-distribution",
  "uri": "https://registry.npmjs.org/pnpm/-/pnpm-10.0.0.tgz",
  "mediaType": "application/gzip",
  "digest": {
    "sha512": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "annotations": {
    "digest_authority": "registry-integrity",
    "package_manager": "pnpm",
    "package_manager_version": "10.0.0",
    "acquisition_source": "corepack"
  }
}
```

For Yarn, `uri` is the actual distribution URL used by Corepack and `digest_authority` is exactly
`"download-hash"`. The producer computes SHA-512 over the downloaded distribution bytes and
hex-encodes that digest as `digest.sha512`. A valid Yarn descriptor is:

```json
{
  "name": "package-manager-distribution",
  "uri": "https://repo.yarnpkg.com/4.5.0/packages/yarnpkg-cli/bin/yarn.js",
  "mediaType": "text/javascript",
  "digest": {
    "sha512": "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
  },
  "annotations": {
    "digest_authority": "download-hash",
    "package_manager": "yarn",
    "package_manager_version": "4.5.0",
    "acquisition_source": "corepack"
  }
}
```

This focused wrong-authority example is invalid because pnpm cannot use `download-hash`:

```json
{
  "name": "package-manager-distribution",
  "uri": "https://registry.npmjs.org/pnpm/-/pnpm-10.0.0.tgz",
  "mediaType": "application/gzip",
  "digest": {
    "sha512": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "annotations": {
    "digest_authority": "download-hash",
    "package_manager": "pnpm",
    "package_manager_version": "10.0.0",
    "acquisition_source": "corepack"
  }
}
```

This focused npm-selected fragment is invalid because npm must not emit a package-manager
distribution:

```json
{
  "selected_package_manager": "npm",
  "resolved_dependency": {
    "name": "package-manager-distribution",
    "uri": "https://registry.npmjs.org/pnpm/-/pnpm-10.0.0.tgz",
    "mediaType": "application/gzip",
    "digest": {
      "sha512": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    },
    "annotations": {
      "digest_authority": "registry-integrity",
      "package_manager": "pnpm",
      "package_manager_version": "10.0.0",
      "acquisition_source": "corepack"
    }
  }
}
```

This is a rejected fixture fragment, not permission to define an npm distribution shape. Wrong
authority, npm presence, pnpm/Yarn absence, duplication, malformed SHA-512, wrong media type,
unknown members, or disagreement with the selected manager or version fails with
`windlass.verify.error.resolved-dependencies-package-manager-distribution`, severity `error`, exit
code `1`.

Before candidate predicate construction, the producer must have the actual Corepack distribution URL
and the applicable authenticated digest evidence. Unavailable evidence fails with
`windlass.verify.error.input-unavailable`, severity `error`, exit code `2`; the producer must not
use Known Good Release fallback, an ambient manager, a hash pin, or a guessed distribution URL.

### Runner-image descriptor

Every selection requires exactly one `runner-image` with exactly `name`, `uri`, and `annotations`.
The [common runner contract](slsa-provenance-v1.md#runner-image) governs its reported identity and
capture. Its four required annotations are exactly `deployment_version`, `operating_system`,
`image_version`, and `runner_label`. A valid descriptor is:

```json
{
  "name": "runner-image",
  "uri": "https://github.com/actions/runner-images/blob/ubuntu24/20261025.353/images/ubuntu/Ubuntu2404-Readme.md",
  "annotations": {
    "deployment_version": "20261026.354.1",
    "operating_system": "Ubuntu\n24.04.5\nLTS",
    "image_version": "20261025.353.1",
    "runner_label": "ubuntu-24.04"
  }
}
```

The producer takes one bounded snapshot of `/imagegeneration/imagedata.json` before
caller-controlled install, build, or pack scripts. The verbatim Included Software URI, exact full
Runner Image `Version:` string, and exact decoded Operating System `detail` come from that same
snapshot. Missing, malformed, or duplicate required groups/details fail capture before construction.
The OS detail must be a non-empty, non-whitespace string. It is not trimmed, compacted,
Unicode-normalized, line-ending-converted, parsed into components, or mapped to a label. Equivalent
JSON escapes such as `\n` and `\u000a` produce the same decoded string; no raw metadata is retained
or compared.

The URI and full `image_version` alone form reported identity, not immutable image-byte identity,
report integrity, or a public-availability claim. Producer and baseline consumer validation project
the first two numeric version components to the tag component in the supported Included Software URI
offline: `20261025.353.1` projects to `20261025.353`. Malformed full versions, unsupported URI
grammar, or projection mismatches fail rather than being repaired. Neither captured value is
rewritten or truncated, and the URI alone does not imply a unique full version.

`deployment_version` is the observed control-plane `$ImageVersion` string or null when absent. It is
non-authoritative, non-identifying, and may differ from `image_version`; equality is never required.
`runner_label` is the independent observed Actions runner label, not derived from any other field.
`operating_system` is context only. `$ImageOS` is optional diagnostic-only context, never required,
compared, signed elsewhere, or used as fallback. Missing local evidence cannot be rescued by any of
these observations or by the network.

Baseline consumers require no raw snapshot and do not independently recheck OS extraction. They
check only the signed closed shape, annotation types/values, and offline URI/version projection.
Producers also reject any descriptor/snapshot disagreement. No DNS, HTTP, tag, release, or report
lookup gates construction, signing, or verification. HTTP 404 neither warns nor rejects valid
evidence, omits the descriptor, or substitutes an available deployment report. No public response
proves integrity.

Unavailable capture evidence fails with `windlass.verify.error.input-unavailable`, exit code `2`.
Malformed local groups/details or inconsistent constructed descriptors fail closed before signing;
descriptor violations use `windlass.verify.error.resolved-dependencies-runner-image`, exit code `1`.
Diagnostics do not require `$ImageOS`. No raw metadata transport, metadata digest, evidence
annotation, consumer decoded-content check, or extra descriptor is introduced.

This invalid descriptor carries a prohibited digest:

```json
{
  "name": "runner-image",
  "uri": "https://github.com/actions/runner-images/blob/ubuntu24/20261025.353/images/ubuntu/Ubuntu2404-Readme.md",
  "digest": {
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "annotations": {
    "deployment_version": null,
    "operating_system": "Ubuntu\n24.04.5\nLTS",
    "image_version": "20261025.353.1",
    "runner_label": "ubuntu-24.04"
  }
}
```

A missing, duplicate, malformed, digest-bearing, unknown-member, or observed-value-mismatched runner
descriptor fails with `windlass.verify.error.resolved-dependencies-runner-image`, severity `error`,
exit code `1`. This includes `content`, `mediaType`, `downloadLocation`, `image_os`, `node_version`,
any fifth annotation, a missing required annotation, or a projection mismatch. A separately governed
future evidence annotation would require its own decision; none is permitted by this contract.

The initial profile does not emit one dependency descriptor per installed transitive package. The
enumerated name-keyed set above is complete; producer-side and consumer-side verification must apply
the same manager-dependent cardinality.

## Digest semantics

- The provenance `subject[0].digest` must include `sha512` and `sha256` for the same packed tarball
  bytes.
- The `sha512` digest is required for npm CLI and registry-facing `--provenance-file` compatibility.
- The `sha256` digest is the canonical digest for cross-job handoff and Windlass verifier
  comparison.
- Public workflow output `package-tarball-sha512` is the tarball SHA-512 digest as 128 lowercase
  hexadecimal characters.
- The npm registry SRI integrity string is not stored in `subject[0].digest` and is not a public
  workflow output in the initial profile. It may be recorded only in registry metadata checks or
  verifier diagnostics.

## Windlass-generated SLSA provenance

The profile generates its own complete SLSA provenance v1 Statement. It does not rely on npm's
automatic provenance feature or any `actions/attest` provenance mode. Windlass owns the
verifier-relevant contents of the emitted Statement: subject, `predicateType`, `buildType`,
`externalParameters`, `builder.id`, and profile-defined predicate fields.

For this profile, `runDetails.builder.version` is the closed common-spec shape with required
`nodejs` and `npm`, plus conditional `corepack`. `nodejs` equals the exact observed `node --version`
output; `npm` records the build-stage bundled npm, never publish-stage npm. `corepack` equals its
observed version exactly when Corepack supplied pnpm or Yarn and is absent for direct npm. Runner
identity, ranges, tags, aliases, and unknown keys are forbidden. The exact pinned Node.js/bundled
npm pair is asserted before first use; the Node.js 24 major and npm `>= 11.5.1` floors remain
defense in depth. Production version comparison uses numeric SemVer major/minor/patch and rejects
prereleases. These are valid direct-npm and Corepack shapes:

```json
{
  "version": {
    "nodejs": "v24.22.1",
    "npm": "11.19.1"
  }
}
```

```json
{
  "version": {
    "nodejs": "v24.22.1",
    "npm": "11.19.1",
    "corepack": "0.34.0"
  }
}
```

Missing `nodejs` or `npm`, conditional `corepack` absence or presence, an unknown key, or
disagreement with the observed versions fails candidate-predicate validation with
`windlass.verify.error.builder-version-mismatch`, severity `error`, exit code `1`, before signing.
For example, the following npm-selected shape is invalid because `corepack` must be absent:

```json
{
  "selected_package_manager": "npm",
  "version": {
    "nodejs": "v24.22.1",
    "npm": "11.19.1",
    "corepack": "0.34.0"
  }
}
```

`runDetails.builder.builderDependencies` contains exactly one closed signing-adapter descriptor. It
identifies the governed `sigstore-go` module version and checksum used by the signing binary:

```json
[
  {
    "uri": "pkg:golang/github.com/sigstore/sigstore-go@v1.3.0",
    "digest": {
      "h1": "hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE="
    },
    "annotations": {
      "role": "signing-adapter"
    }
  }
]
```

The profile must not add build-job actions or any other builder dependency. A missing or additional
descriptor, malformed URI or checksum, URI/module-version disagreement, checksum disagreement with
the signing binary, unknown member, or role other than `"signing-adapter"` fails candidate-predicate
validation with `windlass.verify.error.builder-dependencies-signing-adapter-mismatch`, severity
`error`, exit code `1`, before signing. Node.js and bundled npm versions are single-homed in
`builder.version`; selected pnpm/Yarn versions remain in the distribution descriptor. Runner-image
identity remains in the descriptor named `runner-image`.

This focused invalid descriptor fails with that diagnostic because the URI and digest revisions do
not agree:

```json
[
  {
    "uri": "pkg:golang/github.com/sigstore/sigstore-go@v1.3.0",
    "digest": {
      "h1": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
    },
    "annotations": {
      "role": "signing-adapter"
    }
  }
]
```

## Go-native `sigstore-go` signing adapter

The `provenance-sign` job uses `sigstore-go` v1.3.0 to sign the exact complete Statement bytes that
Windlass assembled and validated:

| Signer input | Required value                                                                    |
| ------------ | --------------------------------------------------------------------------------- |
| DSSE data    | Exact preassembled in-toto Statement bytes; no reconstruction or reserialization. |
| Payload type | `application/vnd.in-toto+json`.                                                   |

Before invoking the adapter, the producer must validate the complete candidate Statement against the
common SLSA and npm-profile closed schemas and expected captured values. This pre-sign gate includes
the closed external-parameter groups, manager-dependent enumerated dependency set,
`builder.version`, and sole signing-adapter builder dependency. Missing package-manager-distribution
or runner-image capture evidence, or an embedded node-runtime pin load failure, fails with
`windlass.verify.error.input-unavailable` and exit code `2`; an unobservable caller workflow
filename fails with `windlass.verify.error.trusted-publisher-mismatch` as specified in the field
rules; a constructed candidate that violates a closed shape or expected value fails with the field's
central diagnostic and exit code `1`. Each result stops before signing.

The adapter uses GitHub Actions OIDC with an ephemeral key, obtains a Fulcio certificate and
embedded SCT, signs the exact bytes as `sign.DSSEData`, and emits a Sigstore bundle containing the
Rekor evidence required by ADR 0069. It emits the bundle file named
`<package-tarball-name>.intoto.jsonl`. The signing boundary must not accept a caller-provided
signing key, arbitrary OIDC token, npm token, publish credential, or caller-controlled step.

After signing, the producer-side verification gate must verify the bundle offline, extract the DSSE
payload, and reject the bundle before publish unless those payload bytes are byte-for-byte equal to
the pre-sign validated Statement. It must also reject adapter drift when the bundle file is missing,
the emitted bundle basename differs from `<package-tarball-name>.intoto.jsonl`, the bundle cannot be
parsed as a Sigstore bundle, the bundle contains a raw Statement instead of the expected signed
bundle structure, or the extracted Statement uses unexpected `_type`, subject, `predicateType`,
predicate, `builder.id`, `buildType`, or `externalParameters` values.

GitHub artifact attestation storage is disabled for this npm path while its SLSA semantic validation
rejects the Windlass custom `buildType`. If that restriction changes, storage may be reconsidered,
but it must remain an optional locator and must not substitute for the signed bundle bytes required
by `npm publish --provenance-file`, same-run handoff, GitHub Release sidecar publication, or
consumer verification.

## Producer signer identity

The signed npm producer bundle must be signed by the GitHub Actions OIDC identity for the SHA-pinned
JS/TS npm reusable workflow execution. Producer-side and consumer-side verification must check all
of the following signer constraints:

- signer workflow repository: `windlasstech/slsa-builder`;
- signer workflow path: `.github/workflows/js-ts-npm-package-slsa3.yml`;
- signer workflow ref: the same full commit SHA recorded in `runDetails.builder.id`;
- built source ref: the effective built ref recorded in `built-source`, normally
  `refs/tags/v<version>`;
- OIDC issuer: GitHub Actions;
- predicate type: `https://slsa.dev/provenance/v1`.

The signer workflow repository is the trusted builder repository, not the package source repository.
The package source repository remains caller-specific and is recorded separately in
`externalParameters.workflow.repository`. Verification must check both identities: the signer
identity must match the Windlass reusable workflow identity, and the built source identity recorded
in the signed fields must match the expected caller repository and release ref.

Under ADR 0080, the certificate's platform-issued source claims describe the invocation context, not
the built content, and the two differ on a `source-ref` dispatch retry. Verification therefore binds
each identity to the evidence that can prove it:

- The **built** source identity binds policy repository expectations to `workflow.repository`, ref
  expectations to the recomputed effective built ref and built-source URI, and commit expectations
  to `built-source.digest.gitCommit`.
- The **invocation** context binds certificate Source Repository Ref to `workflow.ref` and Source
  Repository Digest to `invocation-source.digest.gitCommit`. There is no conditional fallback to
  built-source values or removed members. A mismatch fails verification even if the built identity
  is independently valid.
- The Source Repository URI and the numeric repository and owner identifiers are
  context-independent: they identify the same caller repository in both contexts and continue to
  bind certificate claims against the expected values exactly as ADR 0068 requires.

The [common signer comparison table](slsa-provenance-v1.md#signer-identity-verification-inputs)
defines the certificate bindings once. The npm-specific authorization requirements are:

| Authorization element     | npm requirement                                                                                                                            |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Canonical builder         | `runDetails.builder.id` is the exact Windlass npm workflow URI with an authorized 40-character lowercase SHA.                              |
| Profile path              | Exactly `.github/workflows/js-ts-npm-package-slsa3.yml`, not another Windlass workflow or `.yaml` alias.                                   |
| Build type                | Exactly `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`.                                                                |
| Same-certificate signer   | Build Signer URI equals `runDetails.builder.id` byte-for-byte; Build Signer Digest equals its full SHA suffix.                             |
| Independent authorization | Trusted policy or authenticated release metadata authorizes the exact path/SHA/URI/buildType tuple; intersect constraints when both apply. |

Production identity is constructed from authenticated `job_workflow_ref` and `job_workflow_sha`. The
ref suffix must itself be the authorized full SHA and equal the latter claim. Caller
`github.workflow_sha` and caller `workflow_sha` are never builder identity. Missing signer evidence
cannot be replaced by a predicate suffix. Expectations never derive from the candidate predicate;
jointly consistent but unauthorized values still fail closed.

The exact
[canonical builder rules and migration guard](identity-and-buildtypes.md#npm-profile-path-migration-guard)
apply: generic well-formedness does not preserve npm's fixed-path restriction. Every former builder
path/SHA/URI check requires equivalent profile-aware canonical validation, same-certificate signer
binding, and independent authorization. Wrong paths, aliases, encoded separators, extra `@`, query,
fragment, whitespace, non-SHA refs, short/uppercase/malformed SHAs, and unauthorized well-formed
SHAs are rejected before mutation.

Build Config URI is checked against caller `workflow.path`, never the reusable builder path. A
missing or mismatched binding fails with `windlass.verify.error.build-config-uri-mismatch`.
`internalParameters` follows the required common closed shape: exactly `github`, whose members are
exactly the strings `event_name`, `repository_id`, and `repository_owner_id`. Construction uses
GitHub-issued OIDC claims. Shape failures, including actor fields, fail with
`windlass.verify.error.unexpected-internal-parameters`. Build Trigger must match `event_name` or
fail with `windlass.verify.error.build-trigger-mismatch`. Numeric IDs bind both certificate and
policy. These comparisons occur before any registry or release mutation. The supported-event check
remains separate: only tag push or constrained dispatch is admitted, even if another event is
authenticated.

Valid internal parameters for a dispatch retry are:

```json
{
  "github": {
    "event_name": "workflow_dispatch",
    "repository_id": "123456789",
    "repository_owner_id": "987654321"
  }
}
```

The same object with numeric `repository_id: 123456789`, a missing owner ID, or `actor_id` added is
invalid. The valid shape with `event_name: "push"` against a dispatch certificate is also rejected.
Caller path `.github/workflows/publish.yml` matching Build Config URI is valid; substituting the
reusable builder path is invalid. Correct invocation fields never rescue an untrusted builder, wrong
built ref/commit, repository-chain mismatch, or subject digest mismatch.

The npm verifier must use the common signer identity fallback and conflict rules from
[SLSA provenance v1](slsa-provenance-v1.md#signer-identity-verification-inputs). All signer,
workflow, source, and predicate-type fields for one npm producer verification decision must come
from the same verified bundle and signing certificate, or from verification output bound to that
same certificate. If `job_workflow_ref`/`job_workflow_sha` and `workflow_ref`/`workflow_sha` are
both present, the reusable workflow identity must identify the Windlass signer above, while the
source or caller identity must agree with `workflow` and `invocation-source`, with built-source
expectations independently checked against trusted producer policy. Conflicting claim spellings or
missing required semantic fields fail closed before publish.

A bundle signed by another repository, another workflow path, a branch ref, a pull request ref, a
short SHA ref, a signer identity that does not match `runDetails.builder.id`, a source identity that
does not match its role-specific signed record, or a non-GitHub OIDC issuer must be rejected before
publish.

## Publish-stage npm CLI allowlist

This section is normative under
[ADR 0082](../decisions/0082-pin-publish-stage-npm-cli-version-with-integrity-verified-provisioning-and-reviewed-allowlist.md),
[ADR 0083](../decisions/0083-defer-npm-m1-publish-remediation-to-the-upstream-provenance-file-fix.md),
and
[ADR 0084](../decisions/0084-provision-the-publish-stage-npm-cli-from-a-digest-verified-registry-tarball.md).

The publish-stage npm CLI is pinned. The single shared pin-data source is the machine-readable file
`internal/toolchainpins/publish-npm.json`, which records, per allowlist entry, the pinned npm
version, the exact npm registry tarball URL, and the SHA-512 digest of the compressed tarball in SRI
form (`sha512-<base64>`), byte-identical to the registry packument's `dist.integrity` field. The
provisioning step and the publish-side pin validation both read this file. The human-readable
allowlist table below mirrors it, and a CI check fails on any drift between the file and the table.

| Version   | Tarball URL                                        | SHA-512 (SRI)                                                                                     |
| --------- | -------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `11.21.1` | `https://registry.npmjs.org/npm/-/npm-11.21.1.tgz` | `sha512-YTzL6QBEJXyHcTu0JiUc7RKMp+mzlJtl7rGLmERaXUQeFHIMc9atdr6G9M9C8vDWbv40epoTQAzNL2/PtZVDhQ==` |

The current entry is npm `11.21.1`. Review note for this entry (12026-10-30): the downloaded
tarball's SHA-512 matches the SRI above. The publish-path internals review confirmed compatibility
of the `oidc.js` early-return and auto-enable guards, libnpmpublish's `buildMetadata()` branches and
supplied-provenance verification path, `@npmcli/config` exclusive enforcement including its env/CLI
carve-out, and `publishConfig` flatten timing. The bundled sigstore-js remains `4.1.1`. The upstream
`--provenance-file` fix remains present. New scoped and unscoped fixtures target npm `11.21.1`.
Their live PURL CLI compatibility verification passed on 12026-10-29 UTC with Go 1.27.3 and Node.js
24.22.1 on darwin/arm64. `TestNPMPURLCLICompatibilityLive` compared the npm CLI's bundled
`npm-package-arg`, `NPMPackagePURL`, and the fixture subjects byte-for-byte. No publish or OIDC
exchange was performed; neither this review nor the CLI compatibility result is live-publish or
dogfood evidence.

The initial entry was npm `11.21.0`: at selection time, the latest npm 11-line release containing
the upstream npm/cli#9882 fix (shipped in npm 12.1.0 and backported to the 11 line as npm/cli#9947),
selected per ADR 0083's 11-line preference.

Review note for the initial entry (the ADR 0082 internals checklist plus the ADR 0084 cross-checks,
verified 12026-10-13 against the real tarball and npm/cli):

1. `lib/utils/oidc.js:225` carries the auto-enable guard
   `isDefaultProvenance && !ciInfo.CIRCLE && !opts.provenanceFile`: the npm/cli#9882/#9947 fix is
   present.
2. The bundled libnpmpublish is 11.2.1, with `EPROVENANCECONFLICT` on
   `provenance === true && provenanceFile` and the `verifyProvenance(subject, provenanceFile)` path.
3. `@npmcli/config` 10.14.0 keeps exclusive enforcement with the CLI-data carve-out, matching the
   previously reviewed structure.
4. `publishConfig` flatten timing is irrelevant on the profile's path because `--provenance-file`
   arrives via pinned argv (cliData).
5. The bundled sigstore-js is 4.1.1 (the v4 major, the same major as the libnpmpublish 11.2.0 line
   exercised by the dogfood on npm 11.17.0; the sigstore-js v5 compatibility criterion in ADR 0083
   applies only to the npm 12 line).
6. Registry cross-checks: `dist.integrity` is byte-equal to the pinned SRI (verified over the
   downloaded bytes), `dist.shasum` `7184c24ba6c6d8c1419ed1c4a187377bc05b254c` is consistent, and
   two registry ECDSA signatures are present.
7. Deviation, stated plainly: the packument `gitHead` `5fd1e17e468d58e7f14dc6cbb5390029ff49a41d` is
   the `chore: release 11.21.0` commit in npm/cli (verified via the commits API), but npm/cli
   published no git tag and no GitHub Release for v11.21.0, so the ADR 0084 `gitHead` cross-check
   resolves at the release-commit level for this pin.

Every allowlist bump follows the ADR 0082 bump procedure: the pinned version and distribution digest
are updated together; the npm-internals checklist (`lib/utils/oidc.js` early-return and auto-enable
structure, libnpmpublish `buildMetadata()` branch structure, `@npmcli/config` exclusive enforcement
including the env-layer carve-out, `publishConfig` flatten timing, and the bundled sigstore-js major
version) is diff-reviewed for the new version; the ADR 0084 review-time registry cross-checks are
refreshed; the specification-required compatibility fixtures are added or refreshed for the new pin;
and the full CI gate passes on the new pin.

The supported publish-stage npm CLI set is exactly this allowlist. The complete supported npm CLI
version set for the PURL-derivation compatibility fixture obligation is this allowlist plus the
build-stage npm bundled with the pinned Node.js 24 toolchain pair (ADRs 0085 and 0098); the
build-side half of that keying is specified with the build-side runtime table.

## Publish-stage npm provisioning

The `publish` job first provisions the pinned Node.js runtime per the
[build and pack specification's digest-verified runtime provisioning](js-ts-npm-build-pack.md#digest-verified-runtime-provisioning)
([ADR 0098](../decisions/0098-provision-the-build-stage-nodejs-runtime-from-a-digest-verified-release-tarball.md));
the npm provisioning below runs against that provisioned runtime. The `publish` job must provision
the pinned npm CLI before any npm invocation in the job, including diagnostic invocations. In every
npm-invocation requirement of this specification, an npm invocation is an invocation of the npm CLI
by a step of the job's workflow (for example, a diagnostics, install, build, pack, or publish
command). The runtime provisioning step's internal, read-only verification and reporting is part of
the authenticated toolchain setup, touches no package operation, and is not an npm invocation for
the purposes of this specification. Per
[ADR 0004](../decisions/0004-use-go-as-primary-implementation-language.md), the
fetch-verify-extract-emit logic lives in the trusted Go boundary; the workflow step only invokes it.
Provisioning performs exactly these steps, in this order:

1. Download the exact tarball URL recorded in `internal/toolchainpins/publish-npm.json` over HTTPS.
   No URL, version, or integrity value is derived from registry metadata at run time. A download
   failure fails the job with `windlass.verify.error.npm-provision-download` (check
   `npm.publish.provision`).
2. Hash the compressed bytes and require equality with the recorded SHA-512 digest. A mismatch fails
   the job before extraction with `windlass.verify.error.npm-provision-digest-mismatch` (check
   `npm.publish.provision`).
3. Extract into a fresh private directory with archive-safety checks: a single `package/` root, and
   no absolute or parent-traversing entries. A violation fails the job with
   `windlass.verify.error.npm-provision-archive-unsafe` (check `npm.publish.provision`).
4. Create a launcher named `npm` that execs the provisioned runtime's `node` at its absolute path
   against the extracted `bin/npm-cli.js`. The launcher's absolute path is passed to the publish
   command's `--npm-executable`. The version recorded in the extracted tree must equal the pinned
   version; a post-extraction version mismatch fails before any npm execution with
   `windlass.verify.error.npm-provision-version-mismatch` (check `npm.publish.provision`).

Every provisioning failure occurs before any npm execution and before any registry mutation. No
global installation is performed, no `PATH` mutation is required, and no download cache is
introduced at adoption time. If a cache is added later, only the original compressed tarball may be
cached, and the recorded SHA-512 digest must be re-verified after every restore: a cache-key match
does not authenticate cached bytes.

The workflow conformance gate
([build and pack specification](js-ts-npm-build-pack.md#workflow-conformance)) statically enforces
this provisioning's placement and consumption: exactly one step runs
`go run ./cmd/slsa-builder-internal npm-profile-provision-npm` from the trusted checkout
(`working-directory: .slsa-builder`) with
`--output-directory "${{ runner.temp }}/windlass-npm-cli"`, `--github-output "$GITHUB_OUTPUT"`, and
`--report-path "$REPORT_PATH"`, carries an `id`, and sets `REPORT_PATH` to the outcome report path;
it executes after the node-only pair assertion and before any npm consumer. Every npm executable
reference in the job resolves to exactly `${{ steps.<npm-provision-id>.outputs.npm-executable }}`,
including the `NPM_EXECUTABLE` environment binding and the publish subcommand's single
`--npm-executable` argument; an ambient npm invocation is a conformance failure.

## Publish-stage npm pin validation

The publish job's npm version must equal the allowlist pin recorded in
`internal/toolchainpins/publish-npm.json`. This pin validation replaces the prior publish-side
exact-equality check against the build-time npm record: the build-stage npm remains
toolchain-bundled (pinned separately per ADR 0085, provisioned per ADR 0098) while the publish npm
is pinned independently, so equality between the two is no longer the invariant. The build stage
continues to record the npm it observed in `runDetails.builder.version.npm`, and the floor that
value must satisfy (`>= 11.5.1`) is retained as defense in depth (ADR 0085).

The publish npm version is recorded in the publish job's outcome report as the
`diagnostic_metadata.publish_npm_version` member defined by the
[verification policy and fixtures specification](verification-policy-and-fixtures.md), on success
and on failure whenever the version was observed; a pin-mismatch failure additionally carries the
observed version in the failing diagnostic's `evidence`. It is not merged into the build-time
record. A mismatch between the provisioned npm's version and the allowlist pin fails closed with
`windlass.verify.error.publish-npm-pin-mismatch` (check `npm.publish.toolchain`) before any registry
mutation.

## `npm publish --provenance-file`

- The `publish` job must invoke exactly this argv, in this order, with the optional final argument
  present only when recomputed publish access is non-null:

  ```text
  npm publish <tarball-path> --provenance-file=<bundle-path> --registry=<resolved-registry-url> --tag=<resolved-dist-tag> [--access=<publish-access-option>]
  ```

  `<tarball-path>` must identify the downloaded tarball whose SHA-256 and SHA-512 match the verified
  tarball handoff. `<resolved-registry-url>`, `<resolved-dist-tag>`, and `<publish-access-option>`
  must equal the resolutions recomputed from signed raw publish inputs and verified manifest
  evidence. A disagreement fails before mutation. No other `npm publish` argument is permitted,
  including `--provenance`, `--dry-run`, a package directory, or a registry, tag, access, or
  provenance override supplied through npm configuration or the environment.

- The `<bundle-path>` must point to the downloaded `<package-tarball-name>.intoto.jsonl` file whose
  SHA-256 digest matched the provenance-bundle handoff. The profile must submit that file unchanged.
- The profile must not use npm's automatic provenance generation.
- The profile must not fall back to token-based publish.
- Before invoking `npm publish`, the profile must prove that the provenance file exists, is the
  exact byte sequence emitted by the Go-native DSSE signer, parses as the expected Sigstore bundle,
  and contains an extracted Statement matching the Windlass-verified signing inputs. Missing files,
  raw Statement files, reserialized bundle files, GitHub attestation storage locator files, digest
  mismatches, and Statement mismatches must fail closed before registry mutation. For a non-npmjs
  registry, rejection of the exact external provenance file fails at the publish boundary with
  `windlass.verify.error.custom-registry-provenance-submission-rejected`; the diagnostic report must
  state whether publication could have committed.
- Before running `npm publish` to `https://registry.npmjs.org/`, the profile must check whether
  npmjs already has the package identity and package version. If the package identity does not
  already exist, the workflow must fail clearly before attempting registry mutation because first
  publication is outside the initial trusted-publishing-only profile. If the package version already
  exists for a new `github.run_id`, the workflow must fail clearly before attempting registry
  mutation and must report that verification or inspection, not republish, is the correct operation.
  A retry attempt within the same `github.run_id` instead applies the convergence classification
  below; it may continue without republishing only on `committed-as-expected`.

> [!IMPORTANT]  
> Custom (third-party) npm registry support is an explicit non-goal of the first milestone. The
> behavior in this section remains the eventual contract but is deferred: first-milestone
> conformance does not require it. Consistent with ADR 0030's "unsupported but not blocked" stance,
> this deferral does not prohibit attempts; their results are outside first-milestone conformance
> scope. Promoting custom registries to supported, or blocking them, requires a new ADR.

- For non-npmjs registries, package-identity and package-version existence checks are best-effort
  diagnostics unless a later ADR defines that registry class. The workflow should attempt an
  equivalent metadata check when the selected registry exposes one without requiring publish secrets
  or weakening provenance behavior. A tokenless check that proves package identity or package
  version state records `true` or `false` only in unsigned `diagnostic_metadata`. An unavailable or
  inconclusive tokenless check records `null` for the unproven field and may continue to the
  tokenless publish attempt; this is a diagnostic limitation, not a Windlass-guaranteed pre-publish
  gate. A check that requires `NPM_TOKEN`, `NODE_AUTH_TOKEN`, OTP, publish credentials, unsigned
  provenance, npm automatic provenance fallback, or any other weakening of the production contract
  must fail before registry mutation with `windlass.verify.error.custom-registry-token-required`
  when the condition is a token or OTP requirement, or with
  `windlass.verify.error.custom-registry-provenance-weakened` when it weakens exact external bundle
  submission. The custom registry still must complete tokenless publish with the external provenance
  bundle; otherwise tokenless authentication rejection fails at the authentication or publish
  boundary with `windlass.verify.error.custom-registry-tokenless-auth-failed`.

The minimum non-npmjs publish contract is the same external provenance contract as npmjs, minus
Windlass-guaranteed registry metadata semantics. The profile may proceed only when `npm publish` can
run without publish-capable secrets and can submit the exact verified
`<package-tarball-name>.intoto.jsonl` bundle unchanged through `--provenance-file=<bundle-path>` or
a documented equivalent no-secret external provenance-file submission path. The profile must fail
before registry mutation if the registry or npm CLI path requires token credentials, OTP,
registry-specific secret material, unsigned provenance, npm automatic provenance, omission of the
Windlass bundle, rewriting or re-signing of the bundle, or silently dropping a non-empty
caller-supplied `access` value in order to publish.

If a custom registry rejects a caller-supplied non-empty `access` option during the tokenless
publish flow without proving that a token or OTP is required, the publish must fail at the publish
boundary with `windlass.verify.error.custom-registry-access-option-rejected`. The accepted publish
flow has not committed; the diagnostic report must state whether any registry mutation could
nevertheless have committed.

> [!IMPORTANT]  
> Custom (third-party) npm registry support is an explicit non-goal of the first milestone. The
> behavior in this section remains the eventual contract but is deferred: first-milestone
> conformance does not require it. Consistent with ADR 0030's "unsupported but not blocked" stance,
> this deferral does not prohibit attempts; their results are outside first-milestone conformance
> scope. Promoting custom registries to supported, or blocking them, requires a new ADR.

For non-npmjs registries, null package-identity or version-preexistence diagnostics are allowed only
when a tokenless metadata check is absent or inconclusive. These unsigned observations do not enter
the Statement. The unsupported-but-not-blocked boundary is buildType behavior, not a signed flag.
These diagnostics must not be interpreted by producer-side or consumer-side verification as
Windlass-guaranteed registry support, and they must not relax the requirement that the publish
attempt submit the exact external provenance bundle.

## npm publish same-run convergence

This section is normative under
[ADR 0067](../decisions/0067-converge-repeated-runs-within-run-identity.md) and
[ADR 0073](../decisions/0073-require-published-attestation-run-identity-for-npm-same-run-convergence.md),
and preserves the serialized mutation segment from
[ADR 0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md).
`github.run_id` is the idempotency key. Only a retry attempt within the same `github.run_id` may
recognize an earlier attempt's commit, and only when it passes both the `run_attempt` gate and the
integrity-plus-attestation binding below. A new `github.run_id` is a new release intent and must
fail closed on any pre-existing package version, even when its bytes match; accepting such state is
a conformance failure and must terminate before registry mutation.

The npm mutation step classifies registry state using exactly these four outcome-state names:

- `committed-as-expected`: a retry attempt within the same `github.run_id` observes version metadata
  whose authoritative integrity binding equals the expected packed tarball integrity and a verified
  published attestation that satisfies the required run-identity and subject binding;
- `absent`: authoritative version metadata reports that the package version does not exist;
- `foreign-conflict`: the version exists but its `dist.integrity` differs from the expected packed
  tarball integrity; no required attestation is visible after the polling bound; or a verified
  attestation has a different run identity or a mismatched subject binding;
- `indeterminate`: the registry state cannot be established within the polling bound, or the
  attestation surface is unreadable, unverifiable, or contradictory within that bound.

The expected binding is the packed tarball's SHA-512 SRI value plus the verified published
attestation. The `publish` job must poll the registry version metadata's `dist.integrity` and
compare the complete normalized sha512 SRI value, not a prefix, tarball URL, filename, version
string, or existence alone. A malformed or non-sha512 `dist.integrity` is `indeterminate`; a
well-formed unequal value is `foreign-conflict`. Either state must fail closed, naming the observed
metadata and expected integrity without retrying publication.

For the npmjs production path, the attestation read-back endpoint is exactly:

```text
GET https://registry.npmjs.org/-/npm/v1/attestations/{package-name}@{version}
```

Its successful response has this shape:

```json
{
  "attestations": [
    {
      "predicateType": "https://slsa.dev/provenance/v1",
      "bundle": {
        "dsseEnvelope": {},
        "mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json",
        "verificationMaterial": {}
      },
      "signedAccessSignatureUrl": "https://registry.npmjs.org/-/npm/v1/attestations/example/signature"
    }
  ]
}
```

npm may include a `signedAccessSignatureUrl` member in each attestation element; consumers must
ignore its value, and any other unknown element member remains rejected. The top-level response
object carries only the `attestations` member.

The `publish` job must select attestation elements by `predicateType`, never by array position. npm
automatic GitHub Actions provenance uses `https://slsa.dev/provenance/v1`; the registry's own
publish attestation uses `https://github.com/npm/attestation/tree/main/specs/publish/v0.1`. The
custom `--provenance-file` bundle surfaces in this same collection because npm uploads it as a
`*.sigstore` `_attachments` member. The job must select the `https://slsa.dev/provenance/v1` element
or elements, verify each candidate with the same verified build-metadata manifest evidence under the
full verification policy, including ADR 0068 signer and source identity binding, and accept only a
candidate that meets the required run-identity and subject binding. It must not treat the registry
publish-attestation predicate as the submitted Windlass provenance.

The accepted candidate's verified Run Invocation URI must contain a run-id equal to the current
`github.run_id`; its attempt component is ignored. Its signed subject must bind the ADR 0064 npm
Package URL and the expected SHA-512 and SHA-256 tarball digests. A matching `dist.integrity` alone
is never sufficient for same-run convergence.

npm offers no read-after-write SLA, and observed metadata delays can be long. The explicit read-back
budget is one immediate request followed by one request every 15 seconds until 5 minutes have
elapsed from the first request. Each polling cycle must pair the packument version-existence check
with the attestation endpoint check. The attestation endpoint returns `404 {"error":"Not found"}`
both when a version has no provenance and when the version does not exist, so its 404 response alone
cannot establish either attestation absence or version absence. A transient not-found, missing
`dist.integrity`, missing attestation, transport failure, or rate-limit response remains pending
while budget remains. If the packument confirms the version exists but no required attestation is
visible when the budget expires, the result is `foreign-conflict`. If the version remains
unconfirmed, or required metadata cannot be read or verified, the result is `indeterminate` and the
job must fail while reporting that publication may already have committed. Before the run's first
publish call, an authoritative packument version-not-found response classifies as `absent`; after a
publish call or ambiguous publish result, the same response remains pending because it may reflect
replication lag, except for the determinate E422 absence contract below. As the documented fallback
within the same total budget, the job may download `dist.tarball` and compute its sha512 SRI
locally; only exact equality with the expected SRI plus the verified attestation binding can
establish `committed-as-expected`, and an unavailable or inconclusive fallback still ends as
`indeterminate`.

Under
[ADR 0102](../decisions/0102-classify-npm-e422-publish-rejection-as-determinate-under-reduced-readback-budget.md),
a failed publish mutation whose npm error code is exactly `E422` uses a reduced read-back budget of
one immediate request followed by one request every 15 seconds until 60 seconds have elapsed from
the first read-back request. An observation taken exactly 60 seconds after the first read-back
request is inside the reduced window; window membership is decided by when the observation request
is issued, not by when its response arrives. The reduced and full budgets share that same start
time; reduced expiry never starts a fresh five-minute budget. Definitive `E401`, `E403`, `E404`, and
`ENEEDAUTH` mappings and every non-E422 failure retain their standing behavior.

If the reduced window contains no clean packument observation of the target version but does contain
at least one clean packument observation with the target version absent, reduced expiry produces a
terminal `absent` result with `windlass.verify.error.publish-rejected-unprocessable`. An attestation
error never invalidates an otherwise clean packument absence for this determination. If no clean
packument absence was observed, reduced expiry instead returns to the standing rules through the
original full deadline. A clean absence first observed after the reduced window never becomes
determinate absence, and determinate absence is never re-evaluated after the transition to the full
budget.

Any clean packument observation of the target version at any post-mutation poll, including one after
reduced expiry or one paired with an attestation error, adds
`windlass.verify.warning.publish-committed-after-rejection` with the fixed message "npm reported
E422 but registry read-back observed a committed publication". The full budget measured from the
first read-back request still governs custody. The standing committed-as-expected, foreign-conflict,
and indeterminate rules determine the final outcome, and the warning coexists with any final error.
The rejection-family recognizer never influences this classification.

On pre-mutation `absent`, the same run may invoke `npm publish` once. E422 determinate absence is a
terminal failure and grants no same-run retry. A successful call is not final until read-back
reaches `committed-as-expected`; `foreign-conflict` or `indeterminate` after the call must fail as a
possible partial publication and preserve the evidence. `EPUBLISHCONFLICT` is not an automatic
failure for a retry attempt within the same `github.run_id`: it is a candidate for
`committed-as-expected` pending the same integrity-plus-attestation binding check. An integrity
mismatch, missing attestation after the polling bound, or a verified foreign run identity or subject
mismatch after `EPUBLISHCONFLICT` must become `foreign-conflict`; an unreadable or unverifiable
attestation surface, contradictory observations, or an unconfirmed version at polling exhaustion
must become `indeterminate`. For a new `github.run_id`, `EPUBLISHCONFLICT` remains a hard
`foreign-conflict` failure and must never be adopted.

Re-run failed jobs is the supported convergence surface. Re-run all jobs receives no broader
adoption right and must reapply these rules to every mutation step; inability to recover or
recompute the expected tarball SRI must produce `indeterminate` and fail closed. The final npm step
classification and its integrity evidence must be included in the run's machine-readable
`if: always()` outcome report; a missing report is a conformance failure and must leave the run
failed rather than silently claim convergence.

## Best-effort operational diagnostics

The profile follows the
[external-process operational diagnostics contract](core-profile-contract.md#external-process-operational-diagnostics).
In addition to process lifecycle notices, it may emit best-effort, human-readable stderr diagnostics
for a failed npm publish process and for registry read-back. These diagnostics remain transport
metadata, not trust proof or a stable consumer interface.

A failed-publish summary may contain only a recognized npm error code, an independently observed
HTTP status, allowlisted server-reported predicate field paths, and an optional rejection family
identifier. The code and status are reported only when independently observed; neither is inferred
from the other. Field paths are limited to dotted or indexed paths beneath
`predicate.buildDefinition.buildType`, `predicate.buildDefinition.externalParameters`,
`predicate.buildDefinition.resolvedDependencies`, `predicate.runDetails.builder`, and
`predicate.runDetails.metadata`. Only the field path is reported, never its value or an
expected/actual comparison. A request phase may be reported only when reliably known; the current
implementation omits request-phase information.

The rejection family is a closed vocabulary of five identifiers, each bound to an anchored match on
the complete catalogued template of a documented npm registry rejection message family:
`predicate-field-mismatch`, `repository-metadata-mismatch`, `private-source-repository`,
`unsupported-runner-environment`, and `missing-certificate-claim`. The family is reported only when
the publish output matches a family template, including its routed and multiline forms; incomplete
or altered templates carry no family field. Family matching never gates behavior: it enriches the
operational channel only. Independently of any family match, the publish process's stderr is relayed
under the core contract's hygiene filter, so a novel rejection reaches the log in the registry's own
rendered words.

Read-back diagnostics emit one summary for each existing classification run, never one line per
poll. The summary contains the first observation, the latest observation changed from the first when
one exists, the final observation, observation and change counts, whether read-back follows a
mutation, and the final decision. It uses only observations already made for classification and adds
no registry or verifier operations.

Operational diagnostics must not change canonical reports, classification, mutation, retry, exit
behavior, or existing classification inputs. They must not create a durable artifact, a
`$GITHUB_OUTPUT` entry, a JSON or JSONL schema, or a diagnostic ID. The trusted core never possesses
the raw registry response body: registry message text reaches the log only as the filtered stderr
relay defined by the core contract, and categorical summary lines contain no server message text.
When a process writes to both stdout and stderr, the combined captured stream that failure
classification reads preserves each stream's internal order but not the relative order between
streams; the npm CLI emits its failure diagnostics on stderr, so this relaxation does not change
observed classifications. Logging and stderr write failures are discarded and cannot change any of
these behaviors or outcomes.

Valid illustrative lines are:

```text
windlass operational: npm_failure family=predicate-field-mismatch code=E422 http_status=422 predicate_paths=predicate.buildDefinition.buildType,predicate.runDetails.builder
windlass operational: npm_failure code=E422 http_status=422 predicate_paths=predicate.buildDefinition.buildType,predicate.runDetails.builder
windlass operational: operation=npm stderr: npm error 422 Unprocessable Content - POST https://registry.npmjs.org/-/package/example/dist-tags
windlass operational: readback after_mutation=true observations=2 changes=1 first="package=unknown version=unknown integrity=unknown attestations=unknown errors=both" latest_changed="package=present version=present integrity=match attestations=present errors=none" final="package=present version=present integrity=match attestations=present errors=none" decision=committed-as-expected
```

The following lines are invalid because they claim an unknown request phase, place server message
text in a categorical summary instead of the filtered relay, or disclose prohibited raw values:

```text
windlass operational: npm_failure request_phase=PUT url="https://registry.npmjs.org/@example%2fpkg" message="expected X, actual Y"
windlass operational: npm_failure family=registry-said-no code=E422
windlass operational: readback poll=17 response_body="{\"error\":\"server message\"}"
```

## npm trusted publishing authentication

- The `publish` job uses OIDC trusted publishing.
- npm-native trusted publishing performs the OIDC exchange inside `npm publish`, and the pinned
  `--provenance-file` argv above is unchanged
  ([ADR 0083](../decisions/0083-defer-npm-m1-publish-remediation-to-the-upstream-provenance-file-fix.md)).
  No OIDC-environment suppression, no token injection into an isolated npmrc, and no `publishConfig`
  credential allowlist is specified or implemented.
- The [ADR 0076](../decisions/0076-use-observation-preflights-and-first-mutation-classification.md)
  exchange preflight is retained as a non-mutating observation preflight for early detection of
  trusted-publishing misconfiguration. The minted token is discarded and is never consumed as a
  publish credential.
- The caller job invoking the reusable workflow must grant `contents: read` and `id-token: write` so
  the called workflow can obtain the OIDC token required by npm trusted publishing.
- npm trusted publisher configuration must identify the caller repository and caller workflow
  filename, not `windlasstech/slsa-builder` or `.github/workflows/js-ts-npm-package-slsa3.yml`.
- The remote npm trusted publisher configuration object remains registry-side authorization policy
  and is not signed. The caller filename is derived from the certificate-bound
  `externalParameters.workflow.path`; producer-side and consumer-side verification compare that
  derived filename with trusted-publisher identity and reject disagreement.
- No npm token, OTP, or other long-lived publish secret is used.
- The registry URL must support OIDC trusted publishing.
- A missing caller OIDC permission, npm trusted publisher mismatch, or unavailable caller workflow
  identity must fail before `npm publish` with `windlass.verify.error.trusted-publisher-mismatch`;
  the profile must not fall back to publish credentials or npm automatic provenance.

### npm OIDC exchange response contract

The exchange request itself is defined in
[JS/TS npm package profile](js-ts-npm-package-profile.md#caller-trusted-publishing-requirements):
`POST /-/npm/v1/oidc/token/exchange/package/{pkg}` with a Bearer OIDC `id_token` minted with
audience `npm:registry.npmjs.org`, performed early, before signing and before any publish attempt.
This section pins the success-status and response-body contract the exchange client must enforce
(ADR 0081).

- HTTP `201` is the only success status. HTTP `401` and `404` must fail before registry mutation
  with `windlass.verify.error.trusted-publisher-mismatch`. Every other status, an unreadable or
  malformed body, or a success body violating any rule in this section must fail before registry
  mutation as `indeterminate` with `windlass.verify.error.npm-oidc-exchange-indeterminate`.
- The success body must contain exactly the four members `token_type`, `token`, `created`, and
  `expires`. An additional or missing member is a contract violation and fails as above.
- `token_type` must be exactly `"oidc"`, and `token` must be a non-empty string free of CR, LF, and
  NUL characters. Any other value fails as above.
- `created` and `expires` must each be either a JSON number interpreted as epoch seconds (integral
  and positive) or an RFC 3339 timestamp string; both representations are accepted and normalized
  internally to instants. Any other type or value fails as above.
- `expires` must be later than `created`, and `expires` must be later than the exchange time. An
  inverted or already-expired lifetime fails as above.
- The exchange-token lifetime observed from the live registry is 15 minutes. The Windlass-side
  exchange exists only as the early, non-mutating observation preflight: a token minted by that
  preflight validates configuration only and is discarded. It must never be assumed usable at
  publish time and must never be consumed as a publish credential. The publish-time exchange is
  performed by npm itself inside `npm publish` under trusted publishing, per the finalized
  authentication path above.
- Token material is secret: token values must never appear in logs, diagnostics, reports, or
  artifacts. Failure diagnostics may name the violated rule (for example, the timestamp
  representation) but must never include token material.

A valid observed-shape success body (epoch-second numeric timestamps):

```json
{
  "created": 1786705013,
  "expires": 1786705913,
  "token": "<token>",
  "token_type": "oidc"
}
```

A valid documented-shape success body (RFC 3339 string timestamps):

```json
{
  "token_type": "oidc",
  "token": "<token>",
  "created": "2025-07-18T10:30:00.000Z",
  "expires": "2025-07-18T11:30:00.000Z"
}
```

An invalid success body (the additional member `scope` is a contract violation, rejected before any
registry mutation):

```json
{
  "created": 1786705013,
  "expires": 1786705913,
  "token": "<token>",
  "token_type": "oidc",
  "scope": "publish"
}
```

The fixture set proving this contract is defined in
[Verification policy and fixtures](verification-policy-and-fixtures.md) as part of the
exchange-preflight fixture matrix. No diagnostic ID, classification mapping, or fixture identifier
changes under this contract.

## Registry metadata checks

After publish to `https://registry.npmjs.org/`, the profile must verify that:

- The published package version exists for the expected package identity.
- The registry package version metadata resolves to the same tarball name and package version.
- The registry tarball integrity matches the expected SHA-512 or equivalent npm SRI value derived
  from the same tarball bytes.
- The npmjs attestation endpoint for the exact package version exposes a verified
  `https://slsa.dev/provenance/v1` attestation whose Run Invocation URI run-id equals the current
  `github.run_id`, ignoring its attempt component, and whose signed subject binds the ADR 0064 npm
  Package URL and expected SHA-512 and SHA-256 tarball digests.

These checks run only after successful registry mutation and therefore are post-publish verification
failures, not pre-publish gate failures. The attestations check must use the endpoint contract and
paired packument existence check in
[npm publish same-run convergence](#npm-publish-same-run-convergence). No visible required
attestation after the polling bound, a verified foreign run identity, or a subject mismatch is
`foreign-conflict`; an unreadable or unverifiable attestation surface is `indeterminate`. Each
outcome is read-only and must not trigger a re-publish, attestation regeneration, or any other
registry mutation. If an npmjs metadata check fails, the workflow must fail clearly, report that
publication may have partially succeeded, and must not retry with token credentials, npm automatic
provenance, or unsigned provenance.

For a recomputed non-npmjs registry URL, the unsupported-but-not-blocked boundary remains implicit
in the buildType. Pre-publish package existence checks and post-publish registry metadata linkage
checks are registry-specific diagnostics for that target unless a later ADR defines a supported
custom registry class. Inconclusive tokenless preflight metadata checks are recorded as null
unsigned diagnostics, never in `externalParameters`; they are not reported as Windlass-guaranteed
registry support and are not by themselves workflow failures. Metadata or linkage checks that
require publish credentials, token fallback, unsigned provenance, npm automatic provenance, or
omission of the external provenance bundle are hard failures. A custom registry still must accept
tokenless publish with the exact external provenance bundle; otherwise the authentication or
publish-boundary failure is `windlass.verify.error.custom-registry-tokenless-auth-failed` or
`windlass.verify.error.custom-registry-provenance-submission-rejected`, as applicable.

After a non-npmjs publish attempt, missing required linkage metadata fails post-publication with
`windlass.verify.error.custom-registry-linkage-metadata-absent`. Absent, malformed, incompatible, or
mismatched digest semantics fail post-publication with
`windlass.verify.error.custom-registry-digest-semantics-mismatch`. Each diagnostic report must state
that publication may already have committed. These are fail-clearly observations, not successful
custom-registry conformance.

Consumer verifiers must preserve this distinction. A conforming bundle whose recomputed registry is
non-npmjs can be valid provenance for the tarball without proving npmjs-style state for that
registry. Unsigned preflight diagnostics cannot confer trust. Added external parameters purporting
to permit token fallback, automatic provenance, omitted bundle submission, or other weakened
behavior are rejected under the closed schema; no candidate flag can relax buildType requirements.

## npm producer outputs

| Output                   | Description                                    |
| ------------------------ | ---------------------------------------------- |
| `package-name`           | Normalized npm package name.                   |
| `package-version`        | Package version from `package.json`.           |
| `package-registry-url`   | Normalized effective registry URL.             |
| `package-url`            | Registry package-version URL.                  |
| `package-tarball-name`   | Tarball file name.                             |
| `package-tarball-sha256` | Tarball SHA-256, 64 lowercase hex characters.  |
| `package-tarball-sha512` | Tarball SHA-512, 128 lowercase hex characters. |

These outputs are npm producer release handles. They are always present when the npm publish path
succeeds, including when the public npm workflow later continues into release-asset mode. They are
not substitutes for signed provenance.

Workflow artifact names for the tarball and provenance bundle are internal same-run handoff handles,
not public `workflow_call.outputs`. npm SRI integrity values are registry diagnostics, not public
workflow outputs in the initial profile.

The public `.github/workflows/js-ts-npm-package-slsa3.yml` workflow adds mode-specific GitHub
Release asset outputs when `release-asset-mode` is enabled, as defined by the
[JS/TS npm package workflow contract](js-ts-npm-package-profile.md#outputs). Those release-asset
outputs are publication result handles only. They must not expose the internal composition handoff
manifest name, handoff manifest digest, producer artifact names, publisher handoff field names, or
other values that would let separately invoked workflows treat public outputs as a trusted
composition API.

## Producer-side verification gate

Before invoking the Go-native signer, the producer must validate the candidate Statement as
specified in the signing-adapter section. It must not sign a candidate that fails capture,
closed-schema, cardinality, cross-field, or expected-value validation.

After signing and before `npm publish`, the `publish` job must verify:

1. The bundle signature is valid.
2. The signer identity is trusted.
3. The `predicateType` is `https://slsa.dev/provenance/v1`.
4. The `builder.id` matches the trusted policy.
5. The `buildType` matches the canonical JS/TS npm package `buildType`.
6. The `subject[0].digest.sha512` matches the tarball bytes.
7. The `subject[0].digest.sha256` matches the tarball bytes.
8. The `subject[0].name` matches the expected npm Package URL.
9. The minimized `externalParameters` match the accepted inputs and closed schema. The same
   downloaded build-metadata artifact passes its digest handoff, and its mandatory manifest evidence
   passes decoding, size, digest, path, and source-commit binding. Manifest identity matches the
   tarball and PURL; pack filename matches the handoff, not a signed external field.
10. The name-keyed `resolvedDependencies` set has the exact manager-dependent cardinality and every
    source, manifest, lockfile, runtime, conditional distribution, and runner-image descriptor
    satisfies its closed shape, media-type rule, and expected values. Repository identity follows
    the full manifest/workflow/certificate/source-descriptor chain. Built release expectations bind
    to the effective built ref and built-source commit, not invocation claims.
11. `builder.version` has the exact observed `nodejs`, `npm`, and conditional `corepack` shape.
12. `builderDependencies` contains only the exact `sigstore-go` module descriptor whose URI, `h1`
    checksum, role, and signing-binary dependency agree.
13. The extracted DSSE payload is byte-for-byte equal to the complete candidate Statement that
    Windlass validated before invoking the signer.
14. The same verified certificate binds Build Signer URI/Digest to canonical `builder.id`, Source
    Repository Ref/Digest to `workflow.ref` and invocation-source, Build Config URI to caller
    `workflow.path`, and Build Trigger to `internalParameters.github.event_name`. The closed
    three-string GitHub object and policy-bound numeric IDs are verified without fallback.
15. Manifest-derived publish resolutions, caller filename, and required signed build-script
    disposition pass their checks. Release handoff equality uses the effective built tag and
    built-source descriptor. Mandatory sidecar bytes and output behavior remain unchanged.

If any check fails, the job must fail before registry or release mutation. Registry read-back uses
the same manifest evidence and the same semantic policy, with the existing same-run attempt
handling; it does not compare an earlier attempt's payload to a newly constructed attempt's
Statement bytes. Read-back failures retain their post-publication classification and never authorize
a weaker retry.

## Failure behavior

The `publish` job must fail before `npm publish` when:

- Tarball digest mismatch.
- Bundle digest mismatch.
- Invalid signature.
- Unexpected signer.
- Wrong `predicateType`.
- Wrong `builder.id` or `buildType`.
- Missing `subject[0].digest.sha512` or `subject[0].digest.sha256`.
- npm Package URL subject mismatch.
- Tarball-filename subject used for npm provenance.
- Extracted DSSE payload mismatch with the preassembled Statement.
- Unexpected or mismatched `externalParameters`.
- Missing required external fields or any removed/unknown member, including a `caller` group. The
  primary diagnostics are `windlass.verify.error.external-parameters-incomplete` and
  `windlass.verify.error.external-parameters-unexpected`, respectively, severity `error`, exit code
  `1`.
- Missing, malformed, oversized, or mismatched manifest evidence, including path, digest, or commit
  disagreement: `windlass.verify.error.package-manifest-evidence`, severity `error`, exit code `1`.
- A malformed internal GitHub triple: `windlass.verify.error.unexpected-internal-parameters`. Build
  Config URI disagreement uses `windlass.verify.error.build-config-uri-mismatch`; Build Trigger
  disagreement uses `windlass.verify.error.build-trigger-mismatch`. Each fails with exit code `1`
  before any registry or release mutation, including when all other certificate claims match.
- A missing or invalid source or package-manifest descriptor fails its named descriptor diagnostic
  from the common contract. Invalid selector suppliedness, spelling, or tag conflict fails the
  source selection gate. Missing or inconsistent script disposition fails
  `windlass.verify.error.build-script-disposition-mismatch`.
- Signed normalized distribution values disagree with accepted public mode inputs. The primary
  diagnostic is `windlass.verify.error.release-asset-mode-schema-error`, severity `error`, exit code
  `1`.
- The observed caller workflow filename is unavailable or mismatches trusted-publisher identity. The
  primary diagnostic is `windlass.verify.error.trusted-publisher-mismatch`, severity `error`, exit
  code `1`.
- An unknown or non-enumerated dependency descriptor is present, including one named `npm-cli`. The
  primary diagnostic is `windlass.verify.error.resolved-dependencies-unexpected-entry`, severity
  `error`, exit code `1`.
- The node-runtime descriptor is missing, duplicated, has a missing or wrong `mediaType`, has an
  unknown top-level member such as `downloadLocation` or a case-variant, has a wrong URI, has a
  wrong, malformed, or extra digest member, has a missing, unknown, wrong-type, or wrong-value
  annotation, or mismatches exact expected metadata after structural decoding. The primary
  diagnostic is `windlass.verify.error.resolved-dependencies-node-runtime`, severity `error`, exit
  code `1`, with no mutation and default precedence.
- The package-manager distribution is missing, duplicated, forbidden for npm, malformed, or
  mismatched. The primary diagnostic is
  `windlass.verify.error.resolved-dependencies-package-manager-distribution`, severity `error`, exit
  code `1`.
- The runner-image descriptor is missing, duplicated, malformed, digest-bearing, or mismatched. The
  primary diagnostic is `windlass.verify.error.resolved-dependencies-runner-image`, severity
  `error`, exit code `1`.
- `builder.version` is missing, has an extra or conditionally wrong key, or differs from observed
  versions. The primary diagnostic is `windlass.verify.error.builder-version-mismatch`, severity
  `error`, exit code `1`.
- The sole signing-adapter builder dependency is missing, extra, malformed, or inconsistent with the
  governed signer module version and checksum. The primary diagnostic is
  `windlass.verify.error.builder-dependencies-signing-adapter-mismatch`, severity `error`, exit code
  `1`.
- Required package-manager distribution or runner-image capture evidence is unavailable, or the
  embedded node-runtime pin cannot be loaded, before candidate predicate construction. The primary
  diagnostic is `windlass.verify.error.input-unavailable`, severity `error`, exit code `2`; no
  predicate is signed.
- Source identity mismatch.
- The verified manifest repository is missing, malformed, or differs from `workflow.repository`,
  certificate Source Repository URI, or either source descriptor repository component. The job emits
  `windlass.verify.error.package-repository-identity-mismatch`.
- Package identity mismatch.
- Package identity does not already exist on npmjs when publishing to `https://registry.npmjs.org/`.
- Package version already exists on npmjs for a new `github.run_id` when publishing to
  `https://registry.npmjs.org/`.
- Same-`github.run_id` registry state classifies as `foreign-conflict` or `indeterminate`.
- The required attestation is absent after the polling bound, has a foreign run identity or subject
  mismatch, or its surface is unreadable or unverifiable.
- Mutation concurrency is missing, permits in-progress cancellation, lacks `queue: max` on the
  mutation job, uses an invalid key component, includes `github.workflow`, places a `queue` key on a
  pre-mutation job, or combines `queue: max` with `cancel-in-progress: true`.
- Runtime policy mismatch.
- Publish-stage npm provisioning fails (download failure, distribution digest mismatch, unsafe
  archive layout, or post-extraction version mismatch), or the provisioned publish npm differs from
  the allowlist pin, as defined by the publish-stage npm sections above.

The `publish` job must fail after `npm publish` when npmjs post-publish registry metadata
verification reaches `foreign-conflict` or `indeterminate`. This is a partial-publication failure:
the package version may already exist in the registry, and the workflow must report the final state
and integrity evidence instead of retrying with weaker publication or provenance behavior.

For a non-npmjs registry, the job must fail at the authentication or publish boundary when tokenless
authentication fails, the registry rejects the exact external provenance file, or the registry
rejects a caller-supplied non-empty `access` option without proving a token or OTP requirement. The
access-option rejection uses `windlass.verify.error.custom-registry-access-option-rejected`; the
report must state that the accepted publish flow has not committed and whether any mutation could
have committed. The job must fail post-publication when linkage metadata is absent or digest
semantics are absent, malformed, incompatible, or mismatched. Each post-publication failure must
report that publication may already have committed and must not retry with a token, automatic
provenance, unsigned provenance, or an altered bundle.

The publish-stage npm toolchain failures defined by the allowlist, provisioning, and pin-validation
sections emit the following diagnostics. Each is emitted before any registry mutation, so
`mutation_possible` is always `false`:

| Condition                                                                                               | Diagnostic ID                                          | Phase        | Exit code | Mutation possible |
| ------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ | ------------ | --------- | ----------------- |
| The pinned publish-stage npm tarball cannot be downloaded.                                              | `windlass.verify.error.npm-provision-download`         | pre-mutation | `1`       | `false`           |
| The downloaded publish-stage npm tarball digest differs from the pinned SHA-512.                        | `windlass.verify.error.npm-provision-digest-mismatch`  | pre-mutation | `1`       | `false`           |
| The publish-stage npm tarball has an unsafe archive layout (no single `package/` root, unsafe entries). | `windlass.verify.error.npm-provision-archive-unsafe`   | pre-mutation | `1`       | `false`           |
| The extracted publish-stage npm tree reports a version different from the pin.                          | `windlass.verify.error.npm-provision-version-mismatch` | pre-mutation | `1`       | `false`           |
| The publish job's npm version differs from the allowlist pin.                                           | `windlass.verify.error.publish-npm-pin-mismatch`       | pre-mutation | `1`       | `false`           |

The profile must not fall back to:

- npm automatic provenance.
- Token-based publish.
- Unsigned provenance.
- Local-only provenance.
- GitHub-only attestations without the signed bundle.

## TDD and fixtures

- Positive fixture: accepted signed bundle leading to successful npmjs `npm publish`.
- Minimized external-parameter fixtures prove all seven retained input locations, exact nullable raw
  publish input recording, caller workflow identity, and the two distribution booleans. A mutation
  for every removed member/group in Closed schema rules proves rejection rather than silent
  ignoring. Removed public inputs and their direct projections have no accepted suppliedness cases.
- `npm-source-ref-dispatch-retry-valid` proves a caller selector in `source.ref`, invocation on
  `refs/heads/main`, and a built release tag with its independently resolved commit. Certificate
  source claims bind to workflow/invocation-source, while policy binds to built-source.
- `npm-source-ref-omitted-valid` proves a tag-push with the entire source group omitted and both
  role-named descriptors retained with identical URI/digest values. Fixtures reject an empty source
  object, fabricated selector on omission, or missing selector on supply. Accepted cases include
  different refs resolving to the same commit and an explicit selector equal to a tag invocation,
  including dispatch on a tag. Rejections cover conflicting tags, whitespace-only or padded input,
  short tags, branches, malformed refs, mismatched source URI/digest, wrong version tag, and missing
  or extra built-source annotations. No certificate fallback to built identity is accepted.
- `npm-resolved-dependencies-npm-valid` proves the six-entry set with no distribution descriptor.
  pnpm and Yarn variants prove seven entries with their respective authenticated distribution.
  Exact-byte emission fixtures pin invocation-source at index 0 and the full order in this spec,
  including package-manifest between built-source and lockfile. Reordered valid sets pass name-keyed
  semantic verification; reordered signed payloads fail byte comparison. Duplicate names fail before
  lookup, and missing, unknown, or extra descriptors fail their owning rules.
- `npm-builder-version-direct-npm-valid` proves required `nodejs` and `npm`;
  `npm-builder-version-corepack-valid` adds conditional `corepack`; and
  `npm-builder-signing-adapter-valid` proves the sole governed `sigstore-go` descriptor.
- Existing `npm-resolved-lockfile-valid` and `npm-resolved-lockfile-stale-valid` fixtures continue
  to prove selection provenance and stale diagnostics, with the required media type and built-source
  commit binding. No stale file becomes a dependency.
- Media-type fixtures accept every common matrix row: JSON manifest/npm lockfile, YAML pnpm/Berry
  v4/v5 lockfiles, gzip Node.js and represented registry distributions, and JavaScript only for the
  single yarn.js representation. Reject missing/wrong types, source media types, unknown/generic
  fallback types, extracted-tree gzip claims, manager-name/suffix/header-only selection, shrinkwrap,
  Yarn Classic expansion, and any `content` or `downloadLocation`. No network request repairs a
  type.
- Manifest fixtures cover standalone and workspace paths/annotations, original pre-script bytes,
  valid canonical base64, exact 16 MiB decoded bound, and all digest/path/commit/repository
  bindings. Reject absent/multiple/non-object evidence, extra or duplicate members,
  malformed/URL-safe/ whitespace/noncanonical base64, over-bound bytes, duplicate manifest keys,
  wrong hashes or paths, wrong built commit, packed/build-mutated replacement bytes, and wrong
  manifest media type or workspace annotation. Network-disabled verification proves no fetch and no
  silent skipped check. Both local and registry-candidate verification require the same
  digest-verified metadata handoff.
- Manifest recompute fixtures accept matching packed identity/PURL, normalized repository chains,
  publish defaults and equal input/manifest values. Reject identity/repository conflicts, publish
  intent conflicts, and missing/unsupported/manifest-inconsistent build-script disposition. Both
  `executed` and `skipped-absent` have accepted cases. Pre-existence observations are unsigned only,
  with no consumer signed-state expectation, while mandatory producer preflight tests remain.
- Runner fixtures preserve the 353 URI plus full `20261025.353.1` version and exact multiline
  `Ubuntu\n24.04.5\nLTS` OS value, independent runner label, and null or divergent deployment
  version. Equivalent newline escapes produce equal decoded OS values; meaningful whitespace and
  line endings survive. Capture without ImageOS succeeds. Offline consumer verification requires no
  raw snapshot; simulated 404 never warns, omits, rejects, or substitutes valid local evidence.
- Runner rejections cover missing/duplicate/malformed groups/details,
  non-string/empty/whitespace-only OS detail, normalization or compaction, mixed snapshots or late
  reads, environment substitution, ImageOS requirements/comparison/fallback, deployment equality
  requirements, URI/version projection mismatch, unknown URI grammar, truncated version, `image_os`,
  `node_version`, forbidden top-level members, extra annotations/descriptors, and raw metadata
  requirements. Consumer checks do not claim independent OS extraction proof, report integrity, or
  image-byte identity.
- Identity fixtures accept exact caller workflow and closed three-string GitHub metadata for
  supported push/dispatch. Reject missing/extra/non-string/actor fields, unsupported authenticated
  events, Build Config URI and Build Trigger mismatches, numeric ID mismatches, caller/builder path
  confusion, and invocation/source-role substitution before any registry or release mutation.
- Builder migration fixtures reject every canonical alias and malformed SHA, another otherwise valid
  Windlass workflow filename, unauthorized well-formed SHAs, candidate-derived expectations, missing
  or mismatched Build Signer URI/Digest, cross-certificate evidence, and caller workflow SHA used as
  builder identity. Matching authorized job_workflow_ref/job_workflow_sha succeeds. Review maps
  every removed builder check to canonical builder.id, same-certificate evidence, and independent
  policy.
- Release-asset contract fixtures derive the existing Release target from the effective built tag,
  preserve standalone publisher/internal handoff tag checks, and reject mismatches. Mandatory
  sidecar bytes, naming, upload order, partial failure, and output locators remain covered; no
  native attestation locator substitutes for the sidecar. Reserved-mode rejection remains until
  implemented.
- Positive convergence fixture: a retry attempt within the same `github.run_id` observes an existing
  version, pairs packument and attestation polling, proves an exact sha512 SRI match, verifies a
  `https://slsa.dev/provenance/v1` attestation with the same Run Invocation URI run-id, ignoring its
  attempt component, and proves its ADR 0064 PURL and SHA-512 and SHA-256 subject binding. It then
  classifies `committed-as-expected` and continues without another publish call. The same result is
  valid when the retry first receives `EPUBLISHCONFLICT` and then proves the full binding.
- Rejected fixtures: digest mismatch, signature mismatch, signer mismatch, wrong `predicateType`,
  wrong `builder.id`, wrong `buildType`, unexpected `externalParameters`, package identity mismatch,
  unsupported initial package publication, package-manager selection path mismatch, runtime policy
  mismatch, npm CLI below `11.5.1`, producer-side missing caller OIDC permission, producer-side npm
  trusted publisher caller identity mismatch, emitted Statement mismatch, npmjs post-publish
  metadata mismatch, tarball-filename npm subject, missing `sha512`, missing `sha256`, `sha512` or
  `sha256` digest mismatch, multiple subjects, raw Statement used as the provenance file, and npm
  automatic provenance fallback attempt, package repository identity missing, malformed, or
  mismatched after normalization (`package-repository-identity-mismatch`), custom registry token or
  OTP requirement before mutation (`custom-registry-token-required`), weakened external provenance
  before mutation (`custom-registry-provenance-weakened`), tokenless authentication failure at the
  authentication or publish boundary (`custom-registry-tokenless-auth-failed`), exact external
  provenance-file rejection at publish with commit status reported
  (`custom-registry-provenance-submission-rejected`), missing linkage metadata after publication
  (`custom-registry-linkage-metadata-absent`), and absent, malformed, incompatible, or mismatched
  digest semantics after publication with possible committed publication reported
  (`custom-registry-digest-semantics-mismatch`), and custom-registry rejection of a caller-supplied
  non-empty `access` option without token or OTP proof, with ambiguous cause and possible mutation
  status reported (`custom-registry-access-option-rejected`) as defined by the central fixture
  contract.
- Focused rejected runtime fixtures are `npm-resolved-dependencies-node-runtime-missing`,
  `npm-resolved-dependencies-node-runtime-duplicate`,
  `npm-resolved-dependencies-node-runtime-wrong-uri`,
  `npm-resolved-dependencies-node-runtime-wrong-digest`,
  `npm-resolved-dependencies-node-runtime-malformed-digest`,
  `npm-resolved-dependencies-node-runtime-annotations`, and
  `npm-resolved-dependencies-node-runtime-unknown-member`, all expecting
  `windlass.verify.error.resolved-dependencies-node-runtime`.
  `npm-resolved-dependencies-unknown-entry` expects
  `windlass.verify.error.resolved-dependencies-unexpected-entry`, and
  `npm-resolved-dependencies-npm-manager-distribution` expects
  `windlass.verify.error.resolved-dependencies-package-manager-distribution`. Semantic Go tests
  execute the accepted and rejected fixtures; `fixture-check` alone proves fixture governance, not
  runtime-descriptor behavior.
- Additional focused rejected mutations must cover pnpm or Yarn omitting or duplicating the
  package-manager distribution; pnpm using `download-hash`; Yarn using `registry-integrity`; manager
  or version disagreement; malformed non-hex SHA-512; runner absence, duplication, unknown member,
  or prohibited digest; generated transitive-package entries; missing or unknown distribution
  members; removed caller group; raw release-tag duplication; distribution disagreement; unavailable
  or mismatched derived caller filename; and invalid selector suppliedness or spelling. Their
  expected primary IDs are the corresponding central diagnostics listed in Failure behavior.
- Builder rejected mutations must cover missing `nodejs` or `npm`, absent conditional `corepack`,
  `corepack` present for npm, an unknown version key, missing or extra signing adapters,
  URI/module/checksum disagreement, and a wrong role. Their expected primary IDs are
  `builder-version-mismatch` or `builder-dependencies-signing-adapter-mismatch` as applicable.
- Candidate-predicate fixtures must prove capture unavailability exits `2` with `input-unavailable`,
  malformed constructed candidates exit `1` with the narrow field diagnostic, no failing candidate
  reaches signing, and post-sign comparison rejects any adapter-altered predicate before publish.
- Rejected convergence fixtures: a same-`github.run_id` retry observes a well-formed but unequal
  `dist.integrity`; an existing packument version has no required attestation after the polling
  bound; and a verified required attestation has a different run-id or a mismatched subject. Each
  classifies `foreign-conflict` and fails without adopting or republishing the version. An
  unreadable, unverifiable, or contradictory attestation surface classifies `indeterminate`. A 404
  response from the attestation endpoint alone cannot classify version or attestation absence and
  must be paired with the packument result.
- ADR 0102 E422 fixtures must prove determinate absence with continuous clean absence and with clean
  absence followed by errors, each using exactly 6 packument requests, 6 attestation requests, and 4
  sleeps including the entry observation. Immediate committed and foreign-conflict outcomes each use
  exactly 2 requests per endpoint and 0 sleeps. Attestation-only indeterminacy uses exactly 22
  requests per endpoint and 20 sleeps without granting a fresh full budget after reduced expiry. A
  version first observed at 75 seconds uses exactly 7 requests per endpoint and 5 sleeps and carries
  the committed-after-rejection warning. Each mutation case invokes `npm --version` once and
  `npm publish` once. Foreign-conflict and indeterminate outcomes prove error-before-warning
  ordering and error primacy; the non-E422 ambiguous-read-back control retains exactly 22 requests
  per endpoint and 20 sleeps and never carries the E422 warning.
- Concurrency fixtures: the npm publish job uses the release-intent mutation group with
  `cancel-in-progress: false` and `queue: max`; pre-mutation jobs use `cancel-in-progress: true`
  with no `queue` key; a queued contender revalidates at segment entry; and keys containing
  `github.workflow`, a mutation job without `queue: max`, a pre-mutation job with `queue`, or
  `queue: max` combined with `cancel-in-progress: true` are rejected statically.
- A fixture proving that the `publish` job cannot publish without the signed bundle.
- Rejected publish-stage npm fixtures: tarball download failure (`npm-provision-download`),
  distribution digest mismatch (`npm-provision-digest-mismatch`), unsafe archive layout
  (`npm-provision-archive-unsafe`), post-extraction version mismatch
  (`npm-provision-version-mismatch`), and publish npm pin mismatch (`publish-npm-pin-mismatch`).
  Each fails before any npm execution and before any registry mutation, and the pin file and the
  human-readable allowlist table are proven drift-free.
- Successful registry-conformance fixtures use `https://registry.npmjs.org/` only. Synthetic custom
  registry fixtures cover warning and fail-clearly behavior only.
- Operational-diagnostics TDD and fixtures cover late-`E422` recognition with output truncation and
  a secret canary, the process lifecycle (start failure, exit, signal, timeout, and cancellation),
  first/latest-changed/final read-back transitions without per-poll lines, failing-writer
  non-interference, and real-CLI stream separation between canonical stdout and human-readable
  stderr.
