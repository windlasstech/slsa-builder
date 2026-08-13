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
  [0080](../decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md)
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

- Downloads the tarball and signed bundle.
- Recomputes both digests and verifies them against the handoff.
- Verifies the provenance signature, signer identity, `builder.id`, `buildType`, subject digest,
  subject name, and `externalParameters`.
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
release-mutation-${{ github.repository }}-${{ github.ref_name }}
```

The mutation concurrency group represents one release intent and is composed only from the literal
namespace plus `github.repository` and `github.ref_name`. PRE-mutation groups retain job-specific
namespaces. Any other context or input in the mutation key is a conformance failure and must cause
workflow validation to reject the release configuration. `github.workflow` must not appear in this
or any called-workflow concurrency key: in a reusable workflow it resolves to the caller's workflow
name and creates a self-cancellation trap. Detection of `github.workflow` in a key must reject the
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
  "primary_artifact": {
    "artifact_name": "js-ts-npm-package-tarball-123456789-1",
    "payload_file_name": "windlass-slsa-builder-1.2.3.tgz",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "sha512": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "external_parameters": {},
  "resolved_dependencies": []
}
```

All four top-level members are required. `schema_version` is the string `"1"`.
`primary_artifact.artifact_name` is the deterministic tarball artifact name, and
`primary_artifact.payload_file_name` is the tarball basename. `primary_artifact.sha256` and
`primary_artifact.sha512` are respectively 64 and 128 lowercase hexadecimal digests of that one
tarball's bytes. `external_parameters` is the complete closed object specified in
[JS/TS npm `externalParameters` schema](#jsts-npm-externalparameters-schema), with every required
member present and only that schema's permitted optional members. `resolved_dependencies` is the
complete array of closed ResourceDescriptor values specified in
[JS/TS npm `resolvedDependencies` schema](#jsts-npm-resolveddependencies-schema).

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

The npm package identity is also recorded in `externalParameters.package.name` and
`externalParameters.package.version`. The tarball filename remains verifier-relevant through
`externalParameters.package.tarball_name`, the tarball artifact handoff, and any GitHub Release
asset handoff. It is not the npm provenance Statement subject name.

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
`externalParameters.runtime.npm_version`. A compatibility fixture is required for every supported
npm CLI version and must cover both scoped and unscoped names. Any CLI-version variance from this
derivation is a production conformance failure until this specification and its compatibility
fixtures are updated; an implementation must not silently delegate the subject value to the CLI.

## JS/TS npm `externalParameters` schema

The JS/TS npm package profile must emit exactly the following `externalParameters` object. All
listed fields are required unless marked optional. Unknown top-level fields and unknown nested
fields must be rejected by producer-side verification and by strict consumer policy.

```json
{
  "source": {
    "repository": "https://github.com/example/project",
    "ref": "refs/tags/v1.2.3",
    "revision": "0123456789abcdef0123456789abcdef01234567",
    "event_name": "push",
    "ref_type": "tag"
  },
  "workflow": {
    "path": ".github/workflows/js-ts-npm-package-slsa3.yml",
    "sha": "0123456789abcdef0123456789abcdef01234567",
    "builder_id": "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@0123456789abcdef0123456789abcdef01234567"
  },
  "runtime": {
    "runner": "ubuntu-24.04",
    "node_version": "24.0.0",
    "npm_version": "11.5.1"
  },
  "package": {
    "directory": ".",
    "workspace_root": null,
    "source_manifest": "package.json",
    "name": "@windlass/slsa-builder",
    "version": "1.2.3",
    "private": false,
    "repository": "https://github.com/example/project",
    "tarball_name": "windlass-slsa-builder-1.2.3.tgz",
    "package_url": "https://registry.npmjs.org/%40windlass%2Fslsa-builder/1.2.3",
    "packed_name": "@windlass/slsa-builder",
    "packed_version": "1.2.3"
  },
  "package_manager": {
    "name": "pnpm",
    "version": "10.0.0",
    "selection_source": "packageManager",
    "selection_manifest": "package.json",
    "selection_manifest_path": "package.json",
    "selection_lockfile_path": null,
    "root": "."
  },
  "publish": {
    "input_registry_url": null,
    "input_dist_tag": null,
    "input_access": null,
    "publish_config": null,
    "resolved_registry_url": "https://registry.npmjs.org/",
    "resolved_dist_tag": "latest",
    "publish_access_option": null,
    "effective_access": "existing-package-access",
    "trusted_publishing": true,
    "provenance_file": true,
    "package_identity_preexisting": true,
    "package_version_preexisting": false
  },
  "release": {
    "ref": "refs/tags/v1.2.3",
    "version_tag": "v1.2.3"
  },
  "distribution": {
    "release_asset_mode": true,
    "release_tag_supplied": true,
    "provenance_sidecar": "required",
    "linked_artifact_metadata": true
  },
  "caller": {
    "workflow_filename": "release.yml"
  },
  "build": {
    "script_present": true,
    "script_result": "executed"
  }
}
```

### Closed schema rules

The `externalParameters` value is a closed JSON object. The required top-level members are exactly
`source`, `workflow`, `runtime`, `package`, `package_manager`, `publish`, `release`, `distribution`,
`caller`, and `build`. No other top-level members are allowed. JSON objects in this schema must not
contain duplicate member names; duplicate member names are rejected before semantic validation.

Required nested members are exactly the fields shown in the example above, with the optional fields
listed below as the only allowed additions:

| Object            | Required members                                                                                                                                                                                                                                                            | Optional members                                         |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| `source`          | `repository`, `ref`, `revision`, `event_name`, `ref_type`                                                                                                                                                                                                                   | `input_ref`, `invocation_ref`, `invocation_revision`     |
| `workflow`        | `path`, `sha`, `builder_id`                                                                                                                                                                                                                                                 | none                                                     |
| `runtime`         | `runner`, `node_version`, `npm_version`                                                                                                                                                                                                                                     | none                                                     |
| `package`         | `directory`, `workspace_root`, `source_manifest`, `name`, `version`, `private`, `repository`, `tarball_name`, `package_url`, `packed_name`, `packed_version`                                                                                                                | `publish_config_raw`, `packed_files`, `consumer_surface` |
| `package_manager` | `name`, `version`, `selection_source`, `selection_manifest`, `selection_manifest_path`, `selection_lockfile_path`, `root`                                                                                                                                                   | `ignored_lockfile_paths`, `yarn_install_mode`            |
| `publish`         | `input_registry_url`, `input_dist_tag`, `input_access`, `publish_config`, `resolved_registry_url`, `resolved_dist_tag`, `publish_access_option`, `effective_access`, `trusted_publishing`, `provenance_file`, `package_identity_preexisting`, `package_version_preexisting` | `custom_registry_support`                                |
| `release`         | `ref`, `version_tag`                                                                                                                                                                                                                                                        | none                                                     |
| `distribution`    | `release_asset_mode`, `release_tag_supplied`, `provenance_sidecar`, `linked_artifact_metadata`                                                                                                                                                                              | none                                                     |
| `caller`          | `workflow_filename`                                                                                                                                                                                                                                                         | none                                                     |
| `build`           | `script_present`, `script_result`                                                                                                                                                                                                                                           | none                                                     |

Type and nullability rules:

- All digest, path, URL, package identity, version, event, tag, and enum fields are JSON strings
  unless explicitly defined as boolean, object, array, or `null` below.
- `package.private`, `publish.trusted_publishing`, `publish.provenance_file`, and
  `build.script_present` are JSON booleans.
- `distribution.release_asset_mode`, `distribution.release_tag_supplied`, and
  `distribution.linked_artifact_metadata` are JSON booleans.
- `distribution.provenance_sidecar` is either `null` in npm-only mode or exactly `"required"` after
  release-mode normalization.
- `publish.package_identity_preexisting` and `publish.package_version_preexisting` are JSON booleans
  for `https://registry.npmjs.org/`. For non-npmjs registries, either field may be `null` when the
  unsupported-but-not-blocked registry does not expose a tokenless metadata check that can prove the
  state before publish.

> [!IMPORTANT]  
> Custom (third-party) npm registry support is an explicit non-goal of the first milestone. The
> behavior in this section remains the eventual contract but is deferred: first-milestone
> conformance does not require it. Consistent with ADR 0030's "unsupported but not blocked" stance,
> this deferral does not prohibit attempts; their results are outside first-milestone conformance
> scope. Promoting custom registries to supported, or blocking them, requires a new ADR.

- `package.workspace_root` is either `null` or a repository-root-relative directory string.
- `package_manager.selection_manifest`, `package_manager.selection_manifest_path`, and
  `package_manager.selection_lockfile_path` are either `null` or repository-root-relative file path
  strings, according to the selection-source rules below.
- `package_manager.ignored_lockfile_paths`, when present, is an array of repository-root-relative
  file path strings.
- `package_manager.yarn_install_mode`, when present, is a string. It is required when
  `package_manager.name` is `yarn` and omitted otherwise.
- `publish.input_registry_url`, `publish.input_dist_tag`, `publish.input_access`,
  `publish.publish_access_option`, and `publish.publish_config` are either `null` or the normalized
  value type defined below.
- Optional members are omitted when their value is unknown or not verifier-relevant. Optional
  members must not be emitted as `null` unless the field rule explicitly allows `null`.
- Arrays preserve order and contain only strings or objects allowed by the field rule. Unknown
  object members are rejected at every nesting level.

### Field rules

- `source.repository` must be the canonical HTTPS source repository URL.
- `source.ref` must be the built Git ref: the ref whose content the profile checked out, built,
  packed, and attests. It is the `source-ref` input value when that input is supplied and the
  invocation ref otherwise, and it is the release ref accepted by the runtime guards.
- `source.revision` and `workflow.sha` must be full 40-character lowercase Git commit SHAs.
  `source.revision` is the commit the built ref resolved to before checkout.
- `source.event_name` records the invocation event and must match a supported caller event, such as
  `push` or constrained `workflow_dispatch`.
- `source.ref_type` describes the built ref and must be `tag` for the production release path.
- `source.input_ref` records the caller-supplied `source-ref` input. It must be present exactly when
  the caller supplied a non-empty `source-ref`, must be a full `refs/tags/<tag-name>` ref, and must
  be byte-for-byte equal to `source.ref` — the runtime guards reject any other outcome before
  signing. It must be absent when `source-ref` is omitted. A present-but-invalid, unequal, or
  unexpectedly absent `input_ref` fails with `windlass.verify.error.source-ref-invalid`.
- `source.invocation_ref` and `source.invocation_revision` are the signed invocation record defined
  by ADR 0080: the ref the run was dispatched on (`github.ref`) and its commit SHA (`github.sha`).
  They must be present exactly when `source.input_ref` is present and absent otherwise.
  `source.invocation_ref` must be a full Git ref and `source.invocation_revision` must be a full
  40-character lowercase Git commit SHA. When `source-ref` is omitted, the invocation ref is the
  built ref, so these members would duplicate `source.ref` and `source.revision`; the schema forbids
  that duplication by requiring their absence.

The following focused valid `source` group records a dispatch retry: the run was dispatched on
`refs/heads/main` (invocation), while the built and attested content is the release tag
`refs/tags/v1.2.3` at its resolved commit:

```json
{
  "source": {
    "repository": "https://github.com/example/project",
    "ref": "refs/tags/v1.2.3",
    "revision": "0123456789abcdef0123456789abcdef01234567",
    "event_name": "workflow_dispatch",
    "ref_type": "tag",
    "input_ref": "refs/tags/v1.2.3",
    "invocation_ref": "refs/heads/main",
    "invocation_revision": "89abcdef0123456789abcdef0123456789abcdef01"
  }
}
```

The following focused invalid `source` group supplies the invocation record while `input_ref` is
absent, violating the conditional-presence rule; it fails with
`windlass.verify.error.unexpected-external-parameters`:

```json
{
  "source": {
    "repository": "https://github.com/example/project",
    "ref": "refs/tags/v1.2.3",
    "revision": "0123456789abcdef0123456789abcdef01234567",
    "event_name": "push",
    "ref_type": "tag",
    "invocation_ref": "refs/tags/v1.2.3",
    "invocation_revision": "0123456789abcdef0123456789abcdef01234567"
  }
}
```

- `workflow.path` must be `.github/workflows/js-ts-npm-package-slsa3.yml`.
- `workflow.builder_id` must equal the SHA-based builder identity for `workflow.path` and
  `workflow.sha`.
- `runtime.runner` must be `ubuntu-24.04`.
- `runtime.node_version` must have major version `24`.
- `runtime.npm_version` must be greater than or equal to `11.5.1`, the minimum npm CLI version for
  the initial trusted publishing contract. The comparison is a SemVer numeric comparison over major,
  minor, and patch components; pre-release versions are not accepted for the production profile.
- `package.directory` must equal the resolved `package-directory` input.
- `package.workspace_root` must be `null` for standalone packages or the repository-root-relative
  workspace root directory for workspace packages.
- `package.name`, `package.version`, and `package.private` must come from the validated source
  manifest.
- `package.private` must be `false`.
- `package.repository` must be the normalized canonical repository identity produced from the source
  manifest `repository` value by the accepted forms and normalization rules in the
  [build and pack specification](js-ts-npm-build-pack.md#repository-identity-validation). Its value
  must be exactly `https://github.com/<lowercase-owner>/<lowercase-repository>`.
- `package.repository`, `source.repository`, and the observed caller repository identity must be
  byte-for-byte equal after the required canonical normalization. A missing, malformed, or unequal
  repository identity fails before signing with
  `windlass.verify.error.package-repository-identity-mismatch`.
- The closed `package` object must not include the raw source-manifest `repository` spelling or any
  other raw package metadata. Raw, non-trust package metadata belongs only in the
  `diagnostic_metadata.package_manifest` report surface defined by the
  [verification policy and fixtures](verification-policy-and-fixtures.md#producer-diagnostic-metadata-extension).
- `package.tarball_name` must equal the basename of the pack-produced tarball and must not be
  treated as the npm provenance subject name.
- `package.package_url` must be the registry package-version URL reconstructed from
  `publish.resolved_registry_url`, `package.name`, and `package.version` according to the
  `package-url` rules in the public profile spec. It must not be a Package URL (`pkg:npm/...`).
- `package.packed_name` and `package.packed_version` must match the source package name and version.
- `package_manager.name` must be `npm`, `pnpm`, or `yarn`.
- `package_manager.version` must be the actual package-manager version used.
- When `package_manager.name` is `yarn`, `package_manager.version` must be an exact SemVer version
  greater than or equal to `4.0.0`.
- `package_manager.selection_source` must be one of `packageManager`, `devEngines.packageManager`,
  or `lockfile`.
- When `package_manager.name` is `yarn`, `package_manager.selection_source` must be
  `packageManager`; Yarn releases selected from `devEngines.packageManager` or lockfile inference
  are invalid for the stable initial profile.
- `package_manager.selection_manifest` must identify the manifest whose metadata selected the
  package manager by basename, or be `null` when `selection_source` is `lockfile`.
- `package_manager.selection_manifest_path` must identify the repository-root-relative manifest path
  whose metadata selected the package manager, or be `null` when `selection_source` is `lockfile`.
  For workspace packages, this path distinguishes selected package metadata from workspace root
  metadata even when both files are named `package.json`.
- `package_manager.selection_lockfile_path` must identify the repository-root-relative lockfile path
  when `selection_source` is `lockfile`, and must be `null` otherwise.
- `package_manager.root` must be the repository-root-relative package manager root used for install
  and frozen-lockfile checks.
- `package_manager.ignored_lockfile_paths` is permitted only when the package manager was selected
  from manifest metadata, the selected manager's required lockfile is present in
  `package_manager.root`, and supported lockfiles for non-selected managers are also present in that
  same root. The array records those non-selected lockfiles as stale diagnostics. It must be omitted
  when no lockfile was ignored, and verifiers must not treat the recorded paths as selected
  lockfiles or dependency graph inputs.
- `package_manager.yarn_install_mode` is required and must be `immutable` when
  `package_manager.name` is `yarn`. It records that the release install used the supported Yarn
  Berry v4+ immutable install mode. It must be omitted for npm and pnpm.
- `publish.input_registry_url`, `publish.input_dist_tag`, and `publish.input_access` record
  caller-supplied non-empty workflow inputs when supplied and are `null` when omitted or supplied as
  an empty string after trimming ASCII whitespace. GitHub Actions `workflow_call` defaults must not
  populate these fields.
- `publish.publish_config` records source `publishConfig` fields that affect publish intent. It is
  `null` when source `publishConfig` is absent. When present, it must contain only `registry`,
  `access`, `tag`, and `provenance`; it must not contain `directory` because
  `publishConfig.directory` is rejected before pack. `registry`, `access`, and `tag` are strings
  when present; `provenance` is boolean when present. Unknown `publish_config` members are rejected.
  This normalized object is intentionally narrower than the source manifest's `publishConfig`:
  source members outside `registry`, `access`, `tag`, `provenance`, and `directory` are ignored for
  verifier-relevant publish intent, are not passed through as normalized policy, and must not cause
  rejection solely because they exist.
- `publish.resolved_registry_url` is the normalized effective registry URL: absolute `https:`,
  lowercase scheme and host, no userinfo, no query, no fragment, no default port `443`, path `/`,
  and exactly one trailing slash.
- `publish.resolved_dist_tag` is the resolved npm dist-tag after caller input, `publishConfig`, and
  default resolution.
- `publish.publish_access_option` is the exact value passed to `npm publish --access`; it is
  `public`, `restricted`, or `null` when the option is omitted.
- `publish.effective_access` is the Windlass publish intent used for diagnostics and verification.
  It is `public` or `restricted` when `publish_access_option` is supplied. It is
  `existing-package-access` when `publish_access_option` is `null`, meaning the release must publish
  a new version of an existing package identity without creating or changing package access. npm's
  first-publication default for scoped packages without `--access public` is restricted, but first
  publication is outside the initial production profile.
- Empty workflow inputs for `registry-url`, `dist-tag`, and `access` are omitted before publish
  intent resolution. For example, an empty `publish.input_access` is recorded as `null`; if
  `publish.publish_config.access` is `public`, the resolved `publish.publish_access_option` is
  `public`. If both `publish.input_access` and `publish.publish_config.access` are non-empty and
  normalize to different values, producer-side verification must reject the bundle before publish.
- `publish.trusted_publishing` and `publish.provenance_file` must both be `true`.
- `publish.package_identity_preexisting` must be `true` for `https://registry.npmjs.org/`; the
  initial npmjs production path does not support first publication of a package identity. For
  non-npmjs registries, it is `true` or `false` when the best-effort metadata diagnostic can prove
  that state, and `null` when the state is not verifier-proven for that unsupported registry.
- `publish.package_version_preexisting` must be `false` for `https://registry.npmjs.org/`; the
  selected package version must not exist before publish. For non-npmjs registries, it is `true` or
  `false` when the best-effort metadata diagnostic can prove that state, and `null` when the state
  is not verifier-proven for that unsupported registry.

> [!IMPORTANT]  
> Custom (third-party) npm registry support is an explicit non-goal of the first milestone. The
> behavior in this section remains the eventual contract but is deferred: first-milestone
> conformance does not require it. Consistent with ADR 0030's "unsupported but not blocked" stance,
> this deferral does not prohibit attempts; their results are outside first-milestone conformance
> scope. Promoting custom registries to supported, or blocking them, requires a new ADR.

- `release.ref` must equal the release ref accepted by runtime guards and must be byte-for-byte
  equal to `source.ref` after both values are represented as full Git refs.
- `release.version_tag` must be the tag name without `refs/tags/`.
- `distribution.release_asset_mode` records the accepted `release-asset-mode` boolean.
- `distribution.release_tag_supplied` records whether the caller supplied a non-empty `release-tag`;
  it does not duplicate that raw input. The effective release identity remains in `release.ref` and
  `release.version_tag`.
- `distribution.provenance_sidecar` must be `null` in npm-only mode and `"required"` in release
  mode. Omitted and explicitly `required` public inputs normalize to the same signed value.
- `distribution.linked_artifact_metadata` records the accepted `linked-artifact-metadata` boolean.
- `caller.workflow_filename` must be the normalized caller workflow filename observed for the run
  and used by npm trusted-publisher authorization. It is not a ninth public workflow input. If the
  producer cannot observe it before candidate predicate construction, the run fails with
  `windlass.verify.error.trusted-publisher-mismatch` before signing; a signed value that differs
  from the trusted-publisher identity fails producer-side and consumer-side verification with the
  same diagnostic.
- `build.script_present` records whether `scripts.build` existed in the source manifest.
- `build.script_result` must be `executed` when `scripts.build` ran and `skipped-absent` when the
  build step was an explicit no-op.

### Nine-input completeness mapping

The public workflow's nine inputs have exactly these signed locations:

| Public input               | Signed location                                                                                                                 |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `package-directory`        | `package.directory`                                                                                                             |
| `registry-url`             | `publish.input_registry_url`                                                                                                    |
| `dist-tag`                 | `publish.input_dist_tag`                                                                                                        |
| `access`                   | `publish.input_access`                                                                                                          |
| `source-ref`               | `source.input_ref`; the invocation context it decouples is recorded in `source.invocation_ref` and `source.invocation_revision` |
| `release-asset-mode`       | `distribution.release_asset_mode`                                                                                               |
| `release-tag`              | suppliedness in `distribution.release_tag_supplied`; effective identity remains in `release.ref` and `release.version_tag`      |
| `provenance-sidecar`       | `distribution.provenance_sidecar`                                                                                               |
| `linked-artifact-metadata` | `distribution.linked_artifact_metadata`                                                                                         |

The completeness mapping intentionally excludes four duplications. The profile must not duplicate
the raw `release-tag` input, duplicate caller repository identity already held in
`source.repository`, sign the remote npm trusted-publisher configuration object, or preserve the
omitted-versus-`required` `provenance-sidecar` spelling after both forms normalize to the same
policy. The observed caller filename relevant to the run is nevertheless signed at
`caller.workflow_filename`. A missing or unknown `distribution` or `caller` member, or another
duplicate representation fails with `windlass.verify.error.unexpected-external-parameters`; a raw
release tag copied into `distribution`, or disagreement between normalized distribution values and
the accepted public mode inputs, fails with `windlass.verify.error.release-asset-mode-schema-error`.

The following focused invalid `distribution` object omits `linked_artifact_metadata` and fails with
`windlass.verify.error.unexpected-external-parameters`:

```json
{
  "distribution": {
    "release_asset_mode": false,
    "release_tag_supplied": false,
    "provenance_sidecar": null
  }
}
```

The following focused invalid `caller` object has an unknown member and fails with
`windlass.verify.error.unexpected-external-parameters`:

```json
{
  "caller": {
    "workflow_filename": "release.yml",
    "workflow_path": ".github/workflows/release.yml"
  }
}
```

The following focused invalid `distribution` object copies the raw release tag and fails with
`windlass.verify.error.release-asset-mode-schema-error`:

```json
{
  "distribution": {
    "release_asset_mode": true,
    "release_tag_supplied": true,
    "provenance_sidecar": "required",
    "linked_artifact_metadata": false,
    "release_tag": "v1.2.3"
  }
}
```

### Canonical source repository URL

For GitHub-hosted source repositories, `source.repository` is canonicalized from the repository
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

Producer-side verification must reject the bundle before publish when `source.repository` is
missing, cannot be canonicalized by these rules, or differs from the observed caller repository
identity after case-insensitive GitHub owner/repository comparison.

The normalized `package.repository` value must use this same canonical form and must be exactly
equal to `source.repository` and the observed caller repository identity. The profile must reject a
missing, malformed, or mismatched member before signing with
`windlass.verify.error.package-repository-identity-mismatch`.

Valid complete-schema repository identity example:

```json
{
  "source": {
    "repository": "https://github.com/example/project",
    "ref": "refs/tags/v1.2.3",
    "revision": "0123456789abcdef0123456789abcdef01234567",
    "event_name": "push",
    "ref_type": "tag"
  },
  "workflow": {
    "path": ".github/workflows/js-ts-npm-package-slsa3.yml",
    "sha": "0123456789abcdef0123456789abcdef01234567",
    "builder_id": "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@0123456789abcdef0123456789abcdef01234567"
  },
  "runtime": { "runner": "ubuntu-24.04", "node_version": "24.0.0", "npm_version": "11.5.1" },
  "package": {
    "directory": ".",
    "workspace_root": null,
    "source_manifest": "package.json",
    "name": "@windlass/slsa-builder",
    "version": "1.2.3",
    "private": false,
    "repository": "https://github.com/example/project",
    "tarball_name": "windlass-slsa-builder-1.2.3.tgz",
    "package_url": "https://registry.npmjs.org/%40windlass%2Fslsa-builder/1.2.3",
    "packed_name": "@windlass/slsa-builder",
    "packed_version": "1.2.3"
  },
  "package_manager": {
    "name": "pnpm",
    "version": "10.0.0",
    "selection_source": "packageManager",
    "selection_manifest": "package.json",
    "selection_manifest_path": "package.json",
    "selection_lockfile_path": null,
    "root": "."
  },
  "publish": {
    "input_registry_url": null,
    "input_dist_tag": null,
    "input_access": null,
    "publish_config": null,
    "resolved_registry_url": "https://registry.npmjs.org/",
    "resolved_dist_tag": "latest",
    "publish_access_option": null,
    "effective_access": "existing-package-access",
    "trusted_publishing": true,
    "provenance_file": true,
    "package_identity_preexisting": true,
    "package_version_preexisting": false
  },
  "release": { "ref": "refs/tags/v1.2.3", "version_tag": "v1.2.3" },
  "distribution": {
    "release_asset_mode": false,
    "release_tag_supplied": false,
    "provenance_sidecar": null,
    "linked_artifact_metadata": false
  },
  "caller": { "workflow_filename": "release.yml" },
  "build": { "script_present": true, "script_result": "executed" }
}
```

Invalid complete-schema repository identity example, which fails before signing with
`windlass.verify.error.package-repository-identity-mismatch` because `package.repository` differs
from the normalized source and observed caller identity:

```json
{
  "source": {
    "repository": "https://github.com/example/project",
    "ref": "refs/tags/v1.2.3",
    "revision": "0123456789abcdef0123456789abcdef01234567",
    "event_name": "push",
    "ref_type": "tag"
  },
  "workflow": {
    "path": ".github/workflows/js-ts-npm-package-slsa3.yml",
    "sha": "0123456789abcdef0123456789abcdef01234567",
    "builder_id": "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@0123456789abcdef0123456789abcdef01234567"
  },
  "runtime": { "runner": "ubuntu-24.04", "node_version": "24.0.0", "npm_version": "11.5.1" },
  "package": {
    "directory": ".",
    "workspace_root": null,
    "source_manifest": "package.json",
    "name": "@windlass/slsa-builder",
    "version": "1.2.3",
    "private": false,
    "repository": "https://github.com/example/other-project",
    "tarball_name": "windlass-slsa-builder-1.2.3.tgz",
    "package_url": "https://registry.npmjs.org/%40windlass%2Fslsa-builder/1.2.3",
    "packed_name": "@windlass/slsa-builder",
    "packed_version": "1.2.3"
  },
  "package_manager": {
    "name": "pnpm",
    "version": "10.0.0",
    "selection_source": "packageManager",
    "selection_manifest": "package.json",
    "selection_manifest_path": "package.json",
    "selection_lockfile_path": null,
    "root": "."
  },
  "publish": {
    "input_registry_url": null,
    "input_dist_tag": null,
    "input_access": null,
    "publish_config": null,
    "resolved_registry_url": "https://registry.npmjs.org/",
    "resolved_dist_tag": "latest",
    "publish_access_option": null,
    "effective_access": "existing-package-access",
    "trusted_publishing": true,
    "provenance_file": true,
    "package_identity_preexisting": true,
    "package_version_preexisting": false
  },
  "release": { "ref": "refs/tags/v1.2.3", "version_tag": "v1.2.3" },
  "distribution": {
    "release_asset_mode": false,
    "release_tag_supplied": false,
    "provenance_sidecar": null,
    "linked_artifact_metadata": false
  },
  "caller": { "workflow_filename": "release.yml" },
  "build": { "script_present": true, "script_result": "executed" }
}
```

### Release ref equality

For the production release path, `externalParameters.source.ref`, `externalParameters.release.ref`,
and the built Git ref accepted by the workflow guards must all be the same full Git tag ref in the
form `refs/tags/<tag-name>`. When `source-ref` is supplied, that built ref is the input's tag, not
the invocation ref. The short tag name recorded in `externalParameters.release.version_tag` must be
exactly the suffix of that ref after removing `refs/tags/`.

Producer-side verification must reject the bundle before publish when any of these refs differ after
canonical full-ref representation, when either field is a short tag such as `v1.2.3`, when either
field is a branch or pull request ref, or when `release.version_tag` does not reconstruct the same
full ref. Consumer-side verification must apply the same equality rule before accepting provenance.

The schema permits these optional fields only when the value is known and verifier-relevant:

- `package.publish_config_raw`: diagnostic copy of the source manifest `publishConfig` when needed
  for fixture debugging. It must be a JSON object, must not contain secrets, and is not used to
  relax the normalized `publish.publish_config` schema. It may contain source `publishConfig`
  members that were ignored as non-verifier-relevant publish intent, but verifiers must not derive
  policy from those ignored members.
- `package.packed_files`: array of packed file paths as strings in package archive order.
- `package.consumer_surface`: object containing only packed `exports`, `main`, `type`, `bin`,
  `types`, `typings`, `typesVersions`, and `files` fields when present. Values are copied from the
  packed `package/package.json` JSON value without semantic normalization beyond secret rejection
  and duplicate-member rejection.
- `publish.custom_registry_support`: `unsupported-but-not-blocked` when `resolved_registry_url` is
  not `https://registry.npmjs.org/`.

Producer-side verification must reject the bundle before publish when any required field is missing,
has the wrong type, has an unexpected value, when `runtime.npm_version` is below `11.5.1`, or when
an unknown field is present.

## JS/TS npm `resolvedDependencies` schema

The JS/TS npm package profile emits an unordered, name-keyed set of closed SLSA v1
`ResourceDescriptor` values. A verifier selects descriptors by `name`, never by array position. The
complete cardinality is:

- npm: exactly one `lockfile` and exactly one `runner-image`;
- pnpm or Yarn: exactly one `lockfile`, exactly one `package-manager-distribution`, and exactly one
  `runner-image`.

An unknown descriptor name, generated transitive-package entry, or other non-enumerated extra fails
with `windlass.verify.error.resolved-dependencies-unexpected-entry`. A known descriptor's missing,
duplicate, malformed, or mismatched form fails with that descriptor's narrower diagnostic below.

### Lockfile descriptor

The selected lockfile that constrained the release install has this unchanged closed shape:

```json
{
  "name": "lockfile",
  "uri": "git+https://github.com/example/project@0123456789abcdef0123456789abcdef01234567#pnpm-lock.yaml",
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
  `git+<externalParameters.source.repository>@<externalParameters.source.revision>#<selection_lockfile_path>`.
  The `@<externalParameters.source.revision>` component records the built ref's resolved commit, so
  the descriptor carries the immutable built-source resolution required by ADR 0079 without a
  separate source descriptor.
- The URI fragment must be the repository-root-relative selected lockfile path. It must not be
  empty, absolute, contain path traversal, contain backslashes, point outside the repository, or
  name a non-selected lockfile.
- `digest.sha256` must be the SHA-256 digest of the selected lockfile bytes as 64 lowercase
  hexadecimal characters.
- `annotations.package_manager` must equal `externalParameters.package_manager.name`.
- `annotations.package_manager_root` must equal `externalParameters.package_manager.root`.
- `annotations.selection_source` must equal `externalParameters.package_manager.selection_source`.
- `annotations.selection_manifest_path` must equal
  `externalParameters.package_manager.selection_manifest_path` when manifest metadata selected the
  package manager, and must be `null` when `selection_source` is `lockfile`.
- `annotations.selection_lockfile_path` must be the repository-root-relative selected lockfile path
  for the package manager root, regardless of whether manifest metadata or lockfile inference
  selected the package manager.
- `annotations.stale_non_selected_lockfiles` must be an array. It contains the same paths as
  `externalParameters.package_manager.ignored_lockfile_paths` when that optional field is present,
  and is empty otherwise.

For manifest-selected npm, pnpm, and Yarn releases, `externalParameters.package_manager` records the
manifest source that selected the package manager while the descriptor named `lockfile` records the
selected lockfile path and digest. For lockfile-inferred npm releases, both
`externalParameters.package_manager.selection_lockfile_path` and the named lockfile descriptor's
`annotations.selection_lockfile_path` identify `package-lock.json`.

The profile must fail before signing when the selected lockfile descriptor is missing, has the wrong
digest, points to a non-selected or stale lockfile, contains extra entries, contains unknown
annotation members, omits stale lockfile diagnostics that were recorded in `externalParameters`, or
treats stale non-selected lockfiles as selected dependency graph inputs. The primary diagnostic
category is `resolved-dependencies-lockfile`, severity `error`, exit code `1`.

This focused invalid lockfile example fails with that diagnostic because the selected digest is not
64 lowercase hexadecimal characters:

```json
{
  "name": "lockfile",
  "uri": "git+https://github.com/example/project@0123456789abcdef0123456789abcdef01234567#pnpm-lock.yaml",
  "digest": { "sha256": "not-a-sha256" },
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

### Package-manager distribution descriptor

This descriptor is required exactly once when pnpm or Yarn is selected and is forbidden when npm is
selected. It has exactly `name`, `uri`, `digest`, and `annotations`; its only digest member is
`sha512`, exactly 128 lowercase hexadecimal characters. Its annotations contain exactly
`digest_authority`, `package_manager`, `package_manager_version`, and `acquisition_source`.
`package_manager` and `package_manager_version` must equal `externalParameters.package_manager.name`
and `externalParameters.package_manager.version`, and `acquisition_source` is exactly `"corepack"`.

For pnpm, `uri` is the actual distribution URL used by Corepack and `digest_authority` is exactly
`"registry-integrity"`. The producer authenticates registry `dist.integrity`, requires its
`sha512-<base64>` SRI form, decodes the base64 digest to bytes, and hex-encodes those bytes as
`digest.sha512`; it must not store the SRI wrapper. A valid pnpm descriptor is:

```json
{
  "name": "package-manager-distribution",
  "uri": "https://registry.npmjs.org/pnpm/-/pnpm-10.0.0.tgz",
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
authority, npm presence, pnpm/Yarn absence, duplication, malformed SHA-512, unknown members, or
disagreement with the selected manager or version fails with
`windlass.verify.error.resolved-dependencies-package-manager-distribution`, severity `error`, exit
code `1`.

Before candidate predicate construction, the producer must have the actual Corepack distribution URL
and the applicable authenticated digest evidence. Unavailable evidence fails with
`windlass.verify.error.input-unavailable`, severity `error`, exit code `2`; the producer must not
use Known Good Release fallback, an ambient manager, a hash pin, or a guessed distribution URL.

### Runner-image descriptor

Every manager selection requires exactly one `runner-image` descriptor with exactly `name`, `uri`,
and `annotations`. A `digest` member is prohibited. `uri` is the non-empty `Included Software`
software-report URL read verbatim from the `Runner Image` entry in
`/imagegeneration/imagedata.json`; it must never be derived, reconstructed, or rewritten from image
labels or versions. The annotations contain exactly `image_os`, `image_version`, and `node_version`:
the observed `$ImageOS`, the observed `$ImageVersion`, and the exact observed `node --version`
output. A valid descriptor is:

```json
{
  "name": "runner-image",
  "uri": "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md",
  "annotations": {
    "image_os": "ubuntu24",
    "image_version": "20260801.1.0",
    "node_version": "v24.0.0"
  }
}
```

Before candidate predicate construction, the producer must prove that the file exists and parses as
the documented JSON array, that its `Runner Image` detail contains `Image`, `Version`, and a
non-empty `Included Software` URL, that `Version` equals `$ImageVersion`, and that `Image` is
consistent with `$ImageOS`. Unavailable capture evidence fails before signing with
`windlass.verify.error.input-unavailable`, severity `error`, exit code `2`; the producer must not
guess a URI or validate it over the network.

This invalid descriptor carries a prohibited digest:

```json
{
  "name": "runner-image",
  "uri": "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md",
  "digest": {
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "annotations": {
    "image_os": "ubuntu24",
    "image_version": "20260801.1.0",
    "node_version": "v24.0.0"
  }
}
```

A missing, duplicate, malformed, digest-bearing, unknown-member, or observed-value-mismatched runner
descriptor fails with `windlass.verify.error.resolved-dependencies-runner-image`, severity `error`,
exit code `1`.

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

For this profile, `runDetails.builder.version` is the closed common-spec shape. Lowercase `nodejs`
is required and equals the exact observed `node --version` output. Lowercase `corepack` is required
and equals the exact observed Corepack version when Corepack supplied pnpm or Yarn, and it is absent
when npm is selected directly. npm, runner-image identity, ranges, tags, aliases, and unknown keys
are forbidden. These are valid direct-npm and Corepack shapes:

```json
{
  "version": {
    "nodejs": "v24.0.0"
  }
}
```

```json
{
  "version": {
    "nodejs": "v24.0.0",
    "corepack": "0.34.0"
  }
}
```

Missing `nodejs`, conditional `corepack` absence or presence, an unknown key, or disagreement with
the observed versions fails candidate-predicate validation with
`windlass.verify.error.builder-version-mismatch`, severity `error`, exit code `1`, before signing.
For example, the following npm-selected shape is invalid because `corepack` must be absent:

```json
{
  "selected_package_manager": "npm",
  "version": {
    "nodejs": "v24.0.0",
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
`error`, exit code `1`, before signing. The npm CLI version is already recorded by the npm profile
in `externalParameters.runtime.npm_version` and, for npm-selected runs, as
`externalParameters.package_manager.version`, so `builder.version` must not add an npm key.
Runner-image identity remains in the descriptor named `runner-image`.

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
or runner-image capture evidence fails with `windlass.verify.error.input-unavailable` and exit code
`2`; an unobservable caller workflow filename fails with
`windlass.verify.error.trusted-publisher-mismatch` as specified in the field rules; a constructed
candidate that violates a closed shape or expected value fails with the field's central diagnostic
and exit code `1`. Each result stops before signing.

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
- built source ref: the release tag recorded in the signed `externalParameters.source.ref`, normally
  `refs/tags/v<version>`;
- OIDC issuer: GitHub Actions;
- predicate type: `https://slsa.dev/provenance/v1`.

The signer workflow repository is the trusted builder repository, not the package source repository.
The package source repository remains caller-specific and is recorded separately in
`externalParameters.source.repository`. Verification must check both identities: the signer identity
must match the Windlass reusable workflow identity, and the built source identity recorded in the
signed fields must match the expected caller repository and release ref.

Under ADR 0080, the certificate's platform-issued source claims describe the invocation context, not
the built content, and the two differ on a `source-ref` dispatch retry. Verification therefore binds
each identity to the evidence that can prove it:

- The **built** source identity (repository, release tag ref, resolved commit SHA) is verified
  against the signed `externalParameters.source.*` and `externalParameters.release.*` fields and the
  trusted producer policy.
- The **invocation** context is verified by comparing the certificate's Source Repository Ref and
  Source Repository Digest claims against the signed invocation record:
  `externalParameters.source.invocation_ref` and `externalParameters.source.invocation_revision`
  when present, or `externalParameters.source.ref` and `externalParameters.source.revision` when the
  invocation record is absent (`source-ref` omitted). A mismatch fails with
  `windlass.verify.error.source-ref-mismatch` or `windlass.verify.error.source-digest-mismatch`.
- The Source Repository URI and the numeric repository and owner identifiers are
  context-independent: they identify the same caller repository in both contexts and continue to
  bind certificate claims against the expected values exactly as ADR 0068 requires.

The npm producer verifier must bind the semantic identity fields from the common SLSA provenance
contract to the npm profile values as follows:

| Semantic field             | Required npm producer value                                                                                                                                                |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| OIDC issuer                | GitHub Actions.                                                                                                                                                            |
| Signer workflow repository | `windlasstech/slsa-builder`.                                                                                                                                               |
| Signer workflow path       | `.github/workflows/js-ts-npm-package-slsa3.yml`.                                                                                                                           |
| Signer workflow SHA        | Full commit SHA from `externalParameters.workflow.sha` and the SHA suffix of `runDetails.builder.id`.                                                                      |
| Signer workflow ref        | Must not be a branch, tag, pull request ref, or short SHA when the tool exposes it separately from the full workflow SHA.                                                  |
| Source repository          | Canonical GitHub source repository URL exactly equal to `externalParameters.package.repository`, `externalParameters.source.repository`, and the observed caller identity. |
| Source ref                 | Full release tag ref from the signed `externalParameters.source.ref` and `externalParameters.release.ref`; policy expectations bind to these signed fields.                |
| Source revision            | Full 40-character lowercase commit SHA from the signed `externalParameters.source.revision` and the trusted producer policy.                                               |
| Invocation ref             | Certificate Source Repository Ref, byte-equal to `source.invocation_ref` when present, otherwise to `source.ref`.                                                          |
| Invocation revision        | Certificate Source Repository Digest, byte-equal to `source.invocation_revision` when present, otherwise to `source.revision`.                                             |
| Predicate type             | `https://slsa.dev/provenance/v1`.                                                                                                                                          |

When a verification tool exposes both reusable-workflow identity claims and caller-workflow/source
claims, the reusable-workflow claims must satisfy the signer workflow rows above, and the caller
source claims must satisfy the source repository, invocation ref, and invocation revision rows
above. The producer verifier must reject a bundle when the trusted Windlass workflow identity is
correct but the signed source repository, built source ref, or built source revision differs from
the policy expectation, and it must also reject a bundle when the caller source identity is correct
but the signer workflow path or SHA is not the trusted Windlass reusable workflow identity. On a
tag-triggered run without `source-ref`, the invocation rows compare the certificate claims against
`source.ref` and `source.revision`, which reduces to the single-identity contract.

The producer verifier must also reject the bundle before publish with
`windlass.verify.error.package-repository-identity-mismatch` when normalized
`externalParameters.package.repository`, `externalParameters.source.repository`, and the observed
caller repository identity are not exactly equal.

The npm verifier must use the common signer identity fallback and conflict rules from
[SLSA provenance v1](slsa-provenance-v1.md#signer-identity-verification-inputs). All signer,
workflow, source, and predicate-type fields for one npm producer verification decision must come
from the same verified bundle and signing certificate, or from verification output bound to that
same certificate. If `job_workflow_ref`/`job_workflow_sha` and `workflow_ref`/`workflow_sha` are
both present, the reusable workflow identity must identify the Windlass signer above, while the
source or caller identity must agree with the signed `externalParameters.source` fields and trusted
producer policy. Conflicting claim spellings or missing required semantic fields fail closed before
publish.

A bundle signed by another repository, another workflow path, a branch ref, a pull request ref, a
short SHA ref, a signer identity that does not match `runDetails.builder.id`, a source identity that
does not match `externalParameters.source`, or a non-GitHub OIDC issuer must be rejected before
publish.

## `npm publish --provenance-file`

- The `publish` job must invoke exactly this argv, in this order, with the optional final argument
  present only when `publish.publish_access_option` is non-null:

  ```text
  npm publish <tarball-path> --provenance-file=<bundle-path> --registry=<resolved-registry-url> --tag=<resolved-dist-tag> [--access=<publish-access-option>]
  ```

  `<tarball-path>` must identify the downloaded tarball whose SHA-256 and SHA-512 match the verified
  tarball handoff. `<resolved-registry-url>`, `<resolved-dist-tag>`, and `<publish-access-option>`
  must equal the corresponding signed `externalParameters.publish` values. No other `npm publish`
  argument is permitted, including `--provenance`, `--dry-run`, a package directory, or a registry,
  tag, access, or provenance override supplied through npm configuration or the environment.

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
  version state records `true` or `false` in `publish.package_identity_preexisting` and
  `publish.package_version_preexisting`. An unavailable or inconclusive tokenless check records
  `null` for the unproven field and may continue to the tokenless publish attempt; this is a
  diagnostic limitation, not a Windlass-guaranteed pre-publish gate. A check that requires
  `NPM_TOKEN`, `NODE_AUTH_TOKEN`, OTP, publish credentials, unsigned provenance, npm automatic
  provenance fallback, or any other weakening of the production contract must fail before registry
  mutation with `windlass.verify.error.custom-registry-token-required` when the condition is a token
  or OTP requirement, or with `windlass.verify.error.custom-registry-provenance-weakened` when it
  weakens exact external bundle submission. The custom registry still must complete tokenless
  publish with the external provenance bundle; otherwise tokenless authentication rejection fails at
  the authentication or publish boundary with
  `windlass.verify.error.custom-registry-tokenless-auth-failed`.

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

For non-npmjs registries, `publish.package_identity_preexisting: null` or
`publish.package_version_preexisting: null` is allowed only when a tokenless metadata check is
absent or inconclusive. Those `null` values are diagnostics and must be paired with
`publish.custom_registry_support: "unsupported-but-not-blocked"`. They must not be interpreted by
producer-side or consumer-side verification as Windlass-guaranteed registry support, and they must
not relax the requirement that the publish attempt submit the exact external provenance bundle.

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
      }
    }
  ]
}
```

The `publish` job must select attestation elements by `predicateType`, never by array position. npm
automatic GitHub Actions provenance uses `https://slsa.dev/provenance/v1`; the registry's own
publish attestation uses `https://github.com/npm/attestation/tree/main/specs/publish/v0.1`. The
custom `--provenance-file` bundle surfaces in this same collection because npm uploads it as a
`*.sigstore` `_attachments` member. The job must select the `https://slsa.dev/provenance/v1` element
or elements, verify each candidate under the full verification policy, including ADR 0068 signer and
source identity binding, and accept only a candidate that meets the required run-identity and
subject binding. It must not treat the registry publish-attestation predicate as the submitted
Windlass provenance.

The accepted candidate's verified Run Invocation URI must contain a run-id equal to the current
`github.run_id`; its attempt component is ignored. Its signed subject must bind the ADR 0064 npm
Package URL and the expected SHA-512 and SHA-256 tarball digests. A matching `dist.integrity` alone
is never sufficient for same-run convergence.

npm offers no read-after-write SLA, and observed metadata delays can be long. The explicit read-back
budget is pinned conservatively pending first-publish measurement: one immediate request followed by
one request every 15 seconds until 15 minutes have elapsed from the first request. Each polling
cycle must pair the packument version-existence check with the attestation endpoint check. The
attestation endpoint returns `404 {"error":"Not found"}` both when a version has no provenance and
when the version does not exist, so its 404 response alone cannot establish either attestation
absence or version absence. A transient not-found, missing `dist.integrity`, missing attestation,
transport failure, or rate-limit response remains pending while budget remains. If the packument
confirms the version exists but no required attestation is visible when the budget expires, the
result is `foreign-conflict`. If the version remains unconfirmed, or required metadata cannot be
read or verified, the result is `indeterminate` and the job must fail while reporting that
publication may already have committed. Before the run's first publish call, an authoritative
packument version-not-found response classifies as `absent`; after a publish call or ambiguous
publish result, the same response remains pending because it may reflect replication lag. As the
documented fallback within the same total budget, the job may download `dist.tarball` and compute
its sha512 SRI locally; only exact equality with the expected SRI plus the verified attestation
binding can establish `committed-as-expected`, and an unavailable or inconclusive fallback still
ends as `indeterminate`.

On `absent`, the same run may invoke `npm publish` once. A successful call is not final until
read-back reaches `committed-as-expected`; `foreign-conflict` or `indeterminate` after the call must
fail as a possible partial publication and preserve the evidence. `EPUBLISHCONFLICT` is not an
automatic failure for a retry attempt within the same `github.run_id`: it is a candidate for
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

## npm trusted publishing authentication

- The `publish` job uses OIDC trusted publishing.
- The caller job invoking the reusable workflow must grant `contents: read` and `id-token: write` so
  the called workflow can obtain the OIDC token required by npm trusted publishing.
- npm trusted publisher configuration must identify the caller repository and caller workflow
  filename, not `windlasstech/slsa-builder` or `.github/workflows/js-ts-npm-package-slsa3.yml`.
- The remote npm trusted publisher configuration object remains registry-side authorization policy
  and is not signed. The observed normalized caller workflow filename relevant to the run is signed
  as `externalParameters.caller.workflow_filename`; producer-side and consumer-side verification
  must compare it with the trusted-publisher identity.
- No npm token, OTP, or other long-lived publish secret is used.
- The registry URL must support OIDC trusted publishing.
- A missing caller OIDC permission, npm trusted publisher mismatch, or unavailable caller workflow
  identity must fail before `npm publish` with `windlass.verify.error.trusted-publisher-mismatch`;
  the profile must not fall back to publish credentials or npm automatic provenance.

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

For a non-npmjs `publish.resolved_registry_url`, the profile records
`publish.custom_registry_support: "unsupported-but-not-blocked"`. Pre-publish package existence
checks and post-publish registry metadata linkage checks are registry-specific diagnostics for that
target unless a later ADR defines a supported custom registry class. Inconclusive tokenless
preflight metadata checks are recorded as `null` values in `externalParameters`; they are not
reported as Windlass-guaranteed registry support and are not by themselves workflow failures.
Metadata or linkage checks that require publish credentials, token fallback, unsigned provenance,
npm automatic provenance, or omission of the external provenance bundle are hard failures. A custom
registry still must accept tokenless publish with the exact external provenance bundle; otherwise
the authentication or publish-boundary failure is
`windlass.verify.error.custom-registry-tokenless-auth-failed` or
`windlass.verify.error.custom-registry-provenance-submission-rejected`, as applicable.

After a non-npmjs publish attempt, missing required linkage metadata fails post-publication with
`windlass.verify.error.custom-registry-linkage-metadata-absent`. Absent, malformed, incompatible, or
mismatched digest semantics fail post-publication with
`windlass.verify.error.custom-registry-digest-semantics-mismatch`. Each diagnostic report must state
that publication may already have committed. These are fail-clearly observations, not successful
custom-registry conformance.

Consumer verifiers must preserve this distinction. A non-npmjs provenance bundle with
`publish.custom_registry_support: "unsupported-but-not-blocked"` and `null` preflight fields can be
valid Windlass provenance for the tarball, but it is not evidence that Windlass verified npmjs-style
registry state for that custom registry. A consumer verifier must reject the bundle if the signed
`externalParameters` imply token fallback, npm automatic provenance fallback, omitted external
provenance, or any other weakening of the production contract.

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
9. The `externalParameters` match the expected schema and values, including `package.tarball_name`
   and exact equality among normalized `package.repository`, `source.repository`, and the observed
   caller repository identity; the closed `distribution` and `caller` groups match the accepted
   inputs and observed caller workflow filename.
10. The name-keyed `resolvedDependencies` set has the exact manager-dependent cardinality and every
    lockfile, package-manager distribution, and runner-image descriptor satisfies its closed shape.
11. `builder.version` has the exact observed `nodejs` and conditional `corepack` shape.
12. `builderDependencies` contains only the exact `sigstore-go` module descriptor whose URI, `h1`
    checksum, role, and signing-binary dependency agree.
13. The extracted DSSE payload is byte-for-byte equal to the complete candidate Statement that
    Windlass validated before invoking the signer.

If any check fails, the job must fail before registry mutation.

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
- Missing or unknown closed `distribution` or `caller` members. The primary diagnostic is
  `windlass.verify.error.unexpected-external-parameters`, severity `error`, exit code `1`.
- Signed normalized distribution values disagree with accepted public mode inputs. The primary
  diagnostic is `windlass.verify.error.release-asset-mode-schema-error`, severity `error`, exit code
  `1`.
- The observed caller workflow filename is unavailable or mismatches trusted-publisher identity. The
  primary diagnostic is `windlass.verify.error.trusted-publisher-mismatch`, severity `error`, exit
  code `1`.
- An unknown or non-enumerated dependency descriptor is present. The primary diagnostic is
  `windlass.verify.error.resolved-dependencies-unexpected-entry`, severity `error`, exit code `1`.
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
- Required package-manager distribution or runner-image capture evidence is unavailable before
  candidate predicate construction. The primary diagnostic is
  `windlass.verify.error.input-unavailable`, severity `error`, exit code `2`; no predicate is
  signed.
- Source identity mismatch.
- `package.repository` is missing, malformed, or differs from `source.repository` or the observed
  caller repository identity. The job emits
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

The profile must not fall back to:

- npm automatic provenance.
- Token-based publish.
- Unsigned provenance.
- Local-only provenance.
- GitHub-only attestations without the signed bundle.

## TDD and fixtures

- Positive fixture: accepted signed bundle leading to successful npmjs `npm publish`.
- `npm-external-parameters-distribution-caller-valid` proves the closed `distribution` and `caller`
  groups, all nine public-input representations, normalized sidecar policy, and observed caller
  filename.
- `npm-source-ref-dispatch-retry-valid` proves the dispatch-retry `source` group: `input_ref` equals
  the built release tag, the invocation record carries the dispatch ref and its commit SHA, and the
  certificate source claims match the invocation record.
- `npm-source-ref-omitted-valid` proves the single-identity shape: `input_ref`, `invocation_ref`,
  and `invocation_revision` are absent, and the certificate source claims match `source.ref` and
  `source.revision` exactly as the pre-`source-ref` contract required.
- `npm-resolved-dependencies-npm-valid` proves npm emits the name-keyed lockfile and runner-image
  set and no package-manager distribution. `npm-resolved-dependencies-pnpm-valid` proves one
  registry-integrity pnpm distribution. `npm-resolved-dependencies-yarn-valid` proves one
  download-hash Yarn distribution.
- `npm-builder-version-direct-npm-valid` proves the `nodejs`-only builder version;
  `npm-builder-version-corepack-valid` proves the `nodejs` plus `corepack` shape; and
  `npm-builder-signing-adapter-valid` proves the sole governed `sigstore-go` descriptor.
- Existing `npm-resolved-lockfile-valid` and `npm-resolved-lockfile-stale-valid` fixtures continue
  to prove the unchanged lockfile descriptor within the manager-dependent set.
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
- Focused rejected mutations must cover npm emitting `package-manager-distribution`; pnpm or Yarn
  omitting or duplicating it; pnpm using `download-hash`; Yarn using `registry-integrity`; manager
  or version disagreement; malformed non-hex SHA-512; runner absence, duplication, unknown member,
  or prohibited digest; unknown dependency names and generated transitive-package entries; missing
  or unknown `distribution`/`caller` members; raw release-tag duplication; normalized distribution
  disagreement; unavailable or mismatched caller filename; `input_ref` present but unequal to
  `source.ref` (`source-ref-invalid`); and `invocation_ref` or `invocation_revision` present without
  `input_ref`, or absent when `input_ref` is present (`unexpected-external-parameters`). Their
  expected primary IDs are the corresponding central diagnostics listed in Failure behavior.
- Builder rejected mutations must cover missing `nodejs`, absent conditional `corepack`, `corepack`
  present for npm, an unknown version key, missing or extra signing adapters, URI/module/checksum
  disagreement, and a wrong role. Their expected primary IDs are `builder-version-mismatch` or
  `builder-dependencies-signing-adapter-mismatch` as applicable.
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
- Concurrency fixtures: the npm publish job uses the release-intent mutation group with
  `cancel-in-progress: false` and `queue: max`; pre-mutation jobs use `cancel-in-progress: true`
  with no `queue` key; a queued contender revalidates at segment entry; and keys containing
  `github.workflow`, a mutation job without `queue: max`, a pre-mutation job with `queue`, or
  `queue: max` combined with `cancel-in-progress: true` are rejected statically.
- A fixture proving that the `publish` job cannot publish without the signed bundle.
- Successful registry-conformance fixtures use `https://registry.npmjs.org/` only. Synthetic custom
  registry fixtures cover warning and fail-clearly behavior only.
