# JS/TS npm Provenance, Publish, And Three-Job Graph

This document defines how the JS/TS npm profile generates SLSA provenance, signs it, and publishes
the package to an npm registry through a three-job digest-verified graph.

- Source ADRs: [0024](../decisions/0024-use-oidc-trusted-publishing-without-publish-secrets.md),
  [0025](../decisions/0025-return-package-identity-and-tarball-digest-outputs.md),
  [0028](../decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md),
  [0029](../decisions/0029-use-windlass-generated-slsa-provenance-for-npm-publish.md),
  [0030](../decisions/0030-accept-registry-url-while-guaranteeing-only-npmjs-semantics.md),
  [0035](../decisions/0035-use-actions-attest-as-initial-sigstore-signing-adapter.md),
  [0036](../decisions/0036-use-three-job-digest-verified-publish-graph.md),
  [0037](../decisions/0037-define-initial-verification-deliverables.md),
  [0052](../decisions/0052-compose-npm-package-tarball-producer-with-release-asset-publisher.md),
  [0055](../decisions/0055-use-actions-attest-custom-mode-for-statement-construction.md),
  [0056](../decisions/0056-treat-non-selected-lockfiles-as-stale-diagnostics.md),
  [0057](../decisions/0057-provide-composed-public-npm-release-asset-workflow.md),
  [0059](../decisions/0059-define-public-npm-release-composed-workflow-interface.md),
  [0060](../decisions/0060-unify-npm-profile-public-entrypoint-with-release-asset-mode.md),
  [0061](../decisions/0061-reject-duplicate-json-members-in-signed-slsa-statements.md),
  [0063](../decisions/0063-limit-yarn-support-to-berry-v4-with-corepack-package-manager.md),
  [0064](../decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md),
  [0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [0067](../decisions/0067-converge-repeated-runs-within-run-identity.md),
  [0068](../decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md)
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

## Three-job publish graph

The npm profile uses three jobs:

```text
build -> provenance-sign -> publish
```

### `build`

- Runs install, optional build, and pack.
- Produces the npm package tarball and metadata.
- Uploads the tarball as a workflow artifact.
- Exposes the tarball name, SHA-256, and SHA-512 to downstream jobs.
- Must not have signing or publish permissions.

### `provenance-sign`

- Downloads the tarball artifact.
- Recomputes the tarball digest and verifies it against the handoff.
- Constructs and verifies the SLSA provenance v1 predicate and subject inputs.
- Invokes full-SHA-pinned `actions/attest` custom mode to construct and sign the in-toto Statement.
- Uploads the signed bundle as a workflow artifact.
- Permissions:
  - `contents: read`
  - `id-token: write`
  - `attestations: write`
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
`cancel-in-progress: false`; omission or any other value is a conformance failure that must be
detected before the workflow is accepted for release use. The `build` and `provenance-sign` jobs are
PRE-mutation jobs and must use separate job-level groups with `cancel-in-progress: true` because
they hold no registry mutation authority. Omission, a `false` value, or placement in the mutation
group must cause static workflow conformance to reject the workflow before release use.

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
`cancel-in-progress: false`, so a contender waits rather than interrupts registry or release
mutation. The platform default permits one running and one pending execution; a newer pending
execution replaces the older pending execution. The replaced pending execution is cancelled and must
report that it never entered the mutation segment.

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
| `provenance-sign` | read       | write      | write          |
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

The file contents are the signed Sigstore bundle emitted by the `actions/attest` custom-mode
invocation for the package tarball. The `publish` job downloads the bundle and recomputes its
digest.

The signed bundle file is the exact byte sequence emitted by the `actions/attest` custom-mode
invocation. The profile must hand off and submit those bytes unchanged; it must not replace the file
with an extracted Statement, reserialized bundle, or GitHub-attestation-storage locator. See the
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
  "build": {
    "script_present": true,
    "script_result": "executed"
  }
}
```

### Closed schema rules

The `externalParameters` value is a closed JSON object. The required top-level members are exactly
`source`, `workflow`, `runtime`, `package`, `package_manager`, `publish`, `release`, and `build`. No
other top-level members are allowed. JSON objects in this schema must not contain duplicate member
names; duplicate member names are rejected before semantic validation.

Required nested members are exactly the fields shown in the example above, with the optional fields
listed below as the only allowed additions:

| Object            | Required members                                                                                                                                                                                                                                                            | Optional members                                         |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| `source`          | `repository`, `ref`, `revision`, `event_name`, `ref_type`                                                                                                                                                                                                                   | none                                                     |
| `workflow`        | `path`, `sha`, `builder_id`                                                                                                                                                                                                                                                 | none                                                     |
| `runtime`         | `runner`, `node_version`, `npm_version`                                                                                                                                                                                                                                     | none                                                     |
| `package`         | `directory`, `workspace_root`, `source_manifest`, `name`, `version`, `private`, `repository`, `tarball_name`, `package_url`, `packed_name`, `packed_version`                                                                                                                | `publish_config_raw`, `packed_files`, `consumer_surface` |
| `package_manager` | `name`, `version`, `selection_source`, `selection_manifest`, `selection_manifest_path`, `selection_lockfile_path`, `root`                                                                                                                                                   | `ignored_lockfile_paths`, `yarn_install_mode`            |
| `publish`         | `input_registry_url`, `input_dist_tag`, `input_access`, `publish_config`, `resolved_registry_url`, `resolved_dist_tag`, `publish_access_option`, `effective_access`, `trusted_publishing`, `provenance_file`, `package_identity_preexisting`, `package_version_preexisting` | `custom_registry_support`                                |
| `release`         | `ref`, `version_tag`                                                                                                                                                                                                                                                        | none                                                     |
| `build`           | `script_present`, `script_result`                                                                                                                                                                                                                                           | none                                                     |

Type and nullability rules:

- All digest, path, URL, package identity, version, event, tag, and enum fields are JSON strings
  unless explicitly defined as boolean, object, array, or `null` below.
- `package.private`, `publish.trusted_publishing`, `publish.provenance_file`, and
  `build.script_present` are JSON booleans.
- `publish.package_identity_preexisting` and `publish.package_version_preexisting` are JSON booleans
  for `https://registry.npmjs.org/`. For non-npmjs registries, either field may be `null` when the
  unsupported-but-not-blocked registry does not expose a tokenless metadata check that can prove the
  state before publish.

> [!IMPORTANT] Custom (third-party) npm registry support is an explicit non-goal of the first
> milestone. The behavior in this section remains the eventual contract but is deferred:
> first-milestone conformance does not require it. Consistent with ADR 0030's "unsupported but not
> blocked" stance, this deferral does not prohibit attempts; their results are outside
> first-milestone conformance scope. Promoting custom registries to supported, or blocking them,
> requires a new ADR.

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
- `source.ref` must be the Git ref used for release intent.
- `source.revision` and `workflow.sha` must be full 40-character lowercase Git commit SHAs.
- `source.event_name` must match a supported caller event, such as `push` or constrained
  `workflow_dispatch`.
- `source.ref_type` must be `tag` for the production release path.
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

> [!IMPORTANT] Custom (third-party) npm registry support is an explicit non-goal of the first
> milestone. The behavior in this section remains the eventual contract but is deferred:
> first-milestone conformance does not require it. Consistent with ADR 0030's "unsupported but not
> blocked" stance, this deferral does not prohibit attempts; their results are outside
> first-milestone conformance scope. Promoting custom registries to supported, or blocking them,
> requires a new ADR.

- `release.ref` must equal the release ref accepted by runtime guards and must be byte-for-byte
  equal to `source.ref` after both values are represented as full Git refs.
- `release.version_tag` must be the tag name without `refs/tags/`.
- `build.script_present` records whether `scripts.build` existed in the source manifest.
- `build.script_result` must be `executed` when `scripts.build` ran and `skipped-absent` when the
  build step was an explicit no-op.

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
  "build": { "script_present": true, "script_result": "executed" }
}
```

### Release ref equality

For the production release path, `externalParameters.source.ref`, `externalParameters.release.ref`,
and the runtime Git ref accepted by the workflow guards must all be the same full Git tag ref in the
form `refs/tags/<tag-name>`. The short tag name recorded in `externalParameters.release.version_tag`
must be exactly the suffix of that ref after removing `refs/tags/`.

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

The JS/TS npm package profile must emit exactly one `resolvedDependencies` entry for the selected
lockfile that constrained the release install. The entry is a SLSA v1 `ResourceDescriptor` with this
closed shape:

```json
[
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
]
```

The initial profile does not emit one `resolvedDependencies` entry per installed transitive package.
The selected lockfile descriptor is the verifier-relevant dependency graph input. Consumer-side
verifiers must not require a generated dependency list for the initial profile, and producer-side
verification must reject unexpected `resolvedDependencies` entries or unknown annotation members
under strict policy.

Required descriptor rules:

- `resolvedDependencies` must contain exactly one entry named `lockfile`.
- `uri` must be
  `git+<externalParameters.source.repository>@<externalParameters.source.revision>#<selection_lockfile_path>`.
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
manifest source that selected the package manager while `resolvedDependencies[0]` records the
selected lockfile path and digest. For lockfile-inferred npm releases, both
`externalParameters.package_manager.selection_lockfile_path` and
`resolvedDependencies[0].annotations.selection_lockfile_path` identify `package-lock.json`.

The profile must fail before signing when the selected lockfile descriptor is missing, has the wrong
digest, points to a non-selected or stale lockfile, contains extra entries, contains unknown
annotation members, omits stale lockfile diagnostics that were recorded in `externalParameters`, or
treats stale non-selected lockfiles as selected dependency graph inputs.

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

The profile generates its own SLSA provenance v1 predicate and subject inputs. It does not rely on
npm's automatic provenance feature and does not use `actions/attest` default provenance mode.
Windlass owns the verifier-relevant contents of the emitted Statement: subject, `predicateType`,
`buildType`, `externalParameters`, `builder.id`, and profile-defined predicate fields.

## `actions/attest` signing adapter

The `provenance-sign` job invokes stock, full-SHA-pinned `actions/attest` in custom attestation
mode. Windlass supplies only the adapter inputs supported by that mode:

| Adapter input    | Required value                                                               |
| ---------------- | ---------------------------------------------------------------------------- |
| Subject name     | The verified npm Package URL, equal to `subject[0].name`.                    |
| Subject digest   | The verified tarball digest map, including lowercase `sha512` and `sha256`.  |
| `predicate-type` | `https://slsa.dev/provenance/v1`.                                            |
| Predicate input  | The Windlass-generated SLSA provenance predicate JSON, not a full Statement. |

The deterministic predicate input file basename is:

```text
slsa-provenance-predicate.json
```

This file is an adapter input only. It must not be uploaded, published, or redistributed as the
provenance bundle.

The adapter constructs the in-toto Statement, signs it as a Sigstore-backed bundle, emits the bundle
file named `<package-tarball-name>.intoto.jsonl`, and may also upload the attestation to GitHub
artifact attestation storage. The adapter must not be invoked in default provenance mode for the
production npm profile and must not be documented as accepting a complete in-toto Statement input.

The producer-side verification gate must extract the emitted Statement from the signed bundle and
reject the bundle before publish if the Statement does not match the verified signing inputs. It
must also reject adapter drift when the bundle file is missing, the emitted bundle basename differs
from `<package-tarball-name>.intoto.jsonl`, the bundle cannot be parsed as a Sigstore bundle, the
bundle contains a raw Statement instead of the expected signed bundle structure, or the extracted
Statement uses unexpected `_type`, subject, `predicateType`, predicate, `builder.id`, `buildType`,
or `externalParameters` values.

GitHub artifact attestation storage, when used, is an additional native locator. It is not a
substitute for the signed bundle bytes required by `npm publish --provenance-file`, same-run
handoff, GitHub Release sidecar publication, or consumer verification.

## Producer signer identity

The signed npm producer bundle must be signed by the GitHub Actions OIDC identity for the SHA-pinned
JS/TS npm reusable workflow execution. Producer-side and consumer-side verification must check all
of the following signer constraints:

- signer workflow repository: `windlasstech/slsa-builder`;
- signer workflow path: `.github/workflows/js-ts-npm-package-slsa3.yml`;
- signer workflow ref: the same full commit SHA recorded in `runDetails.builder.id`;
- source ref: the release ref accepted by runtime guards, normally `refs/tags/v<version>`;
- OIDC issuer: GitHub Actions;
- predicate type: `https://slsa.dev/provenance/v1`.

The signer workflow repository is the trusted builder repository, not the package source repository.
The package source repository remains caller-specific and is recorded separately in
`externalParameters.source.repository`. Verification must check both identities: the signer identity
must match the Windlass reusable workflow identity, and the source identity must match the expected
caller repository and release ref.

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
| Source ref                 | Full release tag ref from `externalParameters.source.ref` and `externalParameters.release.ref`.                                                                            |
| Source revision            | Full 40-character lowercase commit SHA from `externalParameters.source.revision` and the trusted producer policy.                                                          |
| Predicate type             | `https://slsa.dev/provenance/v1`.                                                                                                                                          |

When a verification tool exposes both reusable-workflow identity claims and caller-workflow/source
claims, the reusable-workflow claims must satisfy the signer workflow rows above and the caller
source claims must satisfy the source rows above. The producer verifier must reject a bundle when
the trusted Windlass workflow identity is correct but the caller source repository, source ref, or
source revision differs from the signed `externalParameters`, and it must also reject a bundle when
the caller source identity is correct but the signer workflow path or SHA is not the trusted
Windlass reusable workflow identity.

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

- The `publish` job must use `npm publish --provenance-file=<bundle-path>` to publish the tarball
  with the Windlass-generated provenance.
- The `<bundle-path>` must point to the downloaded `<package-tarball-name>.intoto.jsonl` file whose
  SHA-256 digest matched the provenance-bundle handoff. The profile must submit that file unchanged.
- The profile must not use npm's automatic provenance generation.
- The profile must not fall back to token-based publish.
- Before invoking `npm publish`, the profile must prove that the provenance file exists, is the
  exact byte sequence emitted by the `actions/attest` custom-mode invocation, parses as the expected
  Sigstore bundle, and contains an extracted Statement matching the Windlass-verified signing
  inputs. Missing files, raw Statement files, reserialized bundle files, GitHub attestation storage
  locator files, digest mismatches, and Statement mismatches must fail closed before registry
  mutation. For a non-npmjs registry, rejection of the exact external provenance file fails at the
  publish boundary with `windlass.verify.error.custom-registry-provenance-submission-rejected`; the
  diagnostic report must state whether publication could have committed.
- Before running `npm publish` to `https://registry.npmjs.org/`, the profile must check whether
  npmjs already has the package identity and package version. If the package identity does not
  already exist, the workflow must fail clearly before attempting registry mutation because first
  publication is outside the initial trusted-publishing-only profile. If the package version already
  exists for a new `github.run_id`, the workflow must fail clearly before attempting registry
  mutation and must report that verification or inspection, not republish, is the correct operation.
  A retry attempt within the same `github.run_id` instead applies the convergence classification
  below; it may continue without republishing only on `committed-as-expected`.

> [!IMPORTANT] Custom (third-party) npm registry support is an explicit non-goal of the first
> milestone. The behavior in this section remains the eventual contract but is deferred:
> first-milestone conformance does not require it. Consistent with ADR 0030's "unsupported but not
> blocked" stance, this deferral does not prohibit attempts; their results are outside
> first-milestone conformance scope. Promoting custom registries to supported, or blocking them,
> requires a new ADR.

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

> [!IMPORTANT] Custom (third-party) npm registry support is an explicit non-goal of the first
> milestone. The behavior in this section remains the eventual contract but is deferred:
> first-milestone conformance does not require it. Consistent with ADR 0030's "unsupported but not
> blocked" stance, this deferral does not prohibit attempts; their results are outside
> first-milestone conformance scope. Promoting custom registries to supported, or blocking them,
> requires a new ADR.

For non-npmjs registries, `publish.package_identity_preexisting: null` or
`publish.package_version_preexisting: null` is allowed only when a tokenless metadata check is
absent or inconclusive. Those `null` values are diagnostics and must be paired with
`publish.custom_registry_support: "unsupported-but-not-blocked"`. They must not be interpreted by
producer-side or consumer-side verification as Windlass-guaranteed registry support, and they must
not relax the requirement that the publish attempt submit the exact external provenance bundle.

## npm publish same-run convergence

This section is normative under
[ADR 0067](../decisions/0067-converge-repeated-runs-within-run-identity.md) and preserves the
serialized mutation segment from
[ADR 0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md).
`github.run_id` is the idempotency key. Retry attempts within the same `github.run_id` may recognize
an earlier attempt's commit only through the integrity binding below. A new `github.run_id` remains
a new release intent and must fail closed on any pre-existing package version, even when its bytes
match; accepting such state is a conformance failure and must terminate before registry mutation.

The npm mutation step classifies registry state using exactly these four outcome-state names:

- `committed-as-expected`: version metadata exists and its authoritative integrity binding equals
  the expected packed tarball integrity for a retry within the same `github.run_id`;
- `absent`: authoritative version metadata reports that the package version does not exist;
- `foreign-conflict`: the version exists but its `dist.integrity` differs from the expected packed
  tarball integrity, or it belongs to a different `github.run_id` release intent;
- `indeterminate`: the registry state or binding cannot be established within the polling budget.

The expected binding is the packed tarball's SHA-512 SRI value. The `publish` job must poll the
registry version metadata's `dist.integrity` and compare the complete normalized sha512 SRI value,
not a prefix, tarball URL, filename, version string, or existence alone. A malformed or non-sha512
`dist.integrity` is `indeterminate`; a well-formed unequal value is `foreign-conflict`. Either state
must fail closed, naming the observed metadata and expected integrity without retrying publication.

npm offers no read-after-write SLA, and observed metadata delays can be long. The explicit read-back
budget is one immediate request followed by one request every 15 seconds until 15 minutes have
elapsed from the first request. A transient not-found, missing `dist.integrity`, transport failure,
or rate-limit response remains pending while budget remains. If no authoritative equality,
inequality, or absence result is available when the 15-minute budget expires, the result degrades to
`indeterminate` and the job must fail while reporting that publication may already have committed.
Before the run's first publish call, an authoritative version-not-found response classifies as
`absent`; after a publish call or ambiguous publish result, the same response remains pending
because it may reflect replication lag. As the documented fallback within the same total budget, the
job may download `dist.tarball` and compute its sha512 SRI locally; only exact equality with the
expected SRI can establish `committed-as-expected`, and an unavailable or inconclusive fallback
still ends as `indeterminate`.

On `absent`, the same run may invoke `npm publish` once. A successful call is not final until
read-back reaches `committed-as-expected`; `foreign-conflict` or `indeterminate` after the call must
fail as a possible partial publication and preserve the evidence. `EPUBLISHCONFLICT` is not an
automatic failure for a retry attempt within the same `github.run_id`: it is a candidate for
`committed-as-expected` pending the same `dist.integrity` equality check. An integrity mismatch
after `EPUBLISHCONFLICT` must become `foreign-conflict`; exhaustion of the polling budget must
become `indeterminate`. For a new `github.run_id`, `EPUBLISHCONFLICT` remains a hard
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
- The npm trusted publisher configuration is registry-side authorization policy. It is a required
  producer-side publish precondition, but it is not part of the closed SLSA `externalParameters`
  schema and is not a consumer-side Windlass provenance verification field.
- No npm token, OTP, or other long-lived publish secret is used.
- The registry URL must support OIDC trusted publishing.
- A missing caller OIDC permission, npm trusted publisher mismatch, or unavailable caller workflow
  identity must fail before `npm publish`; the profile must not fall back to publish credentials or
  npm automatic provenance.

## Registry metadata checks

After publish to `https://registry.npmjs.org/`, the profile must verify that:

- The published package version exists for the expected package identity.
- The registry package version metadata resolves to the same tarball name and package version.
- The registry tarball integrity matches the expected SHA-512 or equivalent npm SRI value derived
  from the same tarball bytes.
- The registry provenance linkage, when exposed by npmjs metadata or APIs used by the
  implementation, refers to the submitted Windlass-generated provenance bundle.

These checks run only after successful registry mutation and therefore are post-publish verification
failures, not pre-publish gate failures. If an npmjs metadata check fails, the workflow must fail
clearly, report that publication may have partially succeeded, and must not retry with token
credentials, npm automatic provenance, or unsigned provenance.

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

Before `npm publish`, the `publish` job must verify:

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
   caller repository identity.
10. The emitted Statement matches the subject inputs, predicate type, and predicate that Windlass
    verified before invoking `actions/attest`.

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
- Emitted Statement mismatch after `actions/attest` construction.
- Unexpected or mismatched `externalParameters`.
- Source identity mismatch.
- `package.repository` is missing, malformed, or differs from `source.repository` or the observed
  caller repository identity. The job emits
  `windlass.verify.error.package-repository-identity-mismatch`.
- Package identity mismatch.
- Package identity does not already exist on npmjs when publishing to `https://registry.npmjs.org/`.
- Package version already exists on npmjs for a new `github.run_id` when publishing to
  `https://registry.npmjs.org/`.
- Same-`github.run_id` registry state classifies as `foreign-conflict` or `indeterminate`.
- Mutation concurrency is missing, permits in-progress cancellation, uses an invalid key component,
  or includes `github.workflow`.
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
- Positive convergence fixture: a retry attempt within the same `github.run_id` observes an existing
  version, polls `dist.integrity`, proves an exact sha512 SRI match, classifies
  `committed-as-expected`, and continues without another publish call. The same result is valid when
  the retry first receives `EPUBLISHCONFLICT` and then proves the integrity match.
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
- Rejected convergence fixture: a same-`github.run_id` retry observes a well-formed but unequal
  `dist.integrity`, classifies `foreign-conflict`, and fails without adopting or republishing the
  version.
- Concurrency fixtures: the npm publish job uses the release-intent mutation group with
  `cancel-in-progress: false`; a queued contender revalidates at segment entry; and a key containing
  `github.workflow` is rejected as a self-cancellation hazard.
- A fixture proving that the `publish` job cannot publish without the signed bundle.
- Successful registry-conformance fixtures use `https://registry.npmjs.org/` only. Synthetic custom
  registry fixtures cover warning and fail-clearly behavior only.
