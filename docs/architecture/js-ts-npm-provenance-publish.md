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
  [0052](../decisions/0052-compose-npm-package-tarball-producer-with-release-asset-publisher.md)
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
- Constructs the in-toto Statement and SLSA provenance v1 predicate.
- Signs the Statement using full-SHA-pinned `actions/attest`.
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

## Job permissions summary

| Job               | `contents` | `id-token` | `attestations` | `packages` (if needed) |
| ----------------- | ---------- | ---------- | -------------- | ---------------------- |
| `build`           | read       | none       | none           | none                   |
| `provenance-sign` | read       | write      | write          | none                   |
| `publish`         | read       | write      | none           | write                  |

## Tarball artifact handoff

The `build` job uploads the tarball as a workflow artifact with this deterministic name:

```text
js-ts-npm-package-tarball-<github.run_id>-<github.run_attempt>
```

The artifact must contain exactly one file: the pack-produced `.tgz` package tarball. The
`provenance-sign` job downloads the artifact and recomputes its digest.

The handoff must include:

- Tarball artifact name.
- Tarball SHA-256.
- Tarball SHA-512 as lowercase hexadecimal.

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

The artifact must contain exactly one file: the signed Sigstore bundle. The `publish` job downloads
the bundle and recomputes its digest.

The handoff must include:

- Bundle artifact name.
- Bundle digest (SHA-256).

## Digest verification between jobs

Every receiving job must:

1. Download the artifact.
2. Recompute the digest using the algorithm specified in the handoff.
3. Compare the computed digest with the expected digest.
4. Fail closed on mismatch.

## npm package subject naming

The `subject[0].name` in the provenance Statement is the packed tarball file name, not the npm
package identity. The subject name must exactly match the tarball file name that the profile
publishes to npm and may later hand off to the GitHub Release asset publisher.

For example:

```text
windlass-slsa-builder-1.2.3.tgz
```

The npm package identity is recorded in `externalParameters.package.name` and
`externalParameters.package.version`.

The profile must fail before signing when the tarball name is empty, contains a path separator, is
not the basename of the pack-produced tarball path, or does not end in `.tgz`.

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
    "tarball_name": "windlass-slsa-builder-1.2.3.tgz",
    "package_url": "pkg:npm/%40windlass/slsa-builder@1.2.3",
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
- `package.tarball_name` must equal `subject[0].name`.
- `package.package_url` must be a package URL (`pkg:npm/...`) for the package name and version.
- `package.packed_name` and `package.packed_version` must match the source package name and version.
- `package_manager.name` must be `npm`, `pnpm`, or `yarn`.
- `package_manager.version` must be the actual package-manager version used.
- `package_manager.selection_source` must be one of `packageManager`, `devEngines.packageManager`,
  or `lockfile`.
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
- `publish.input_registry_url`, `publish.input_dist_tag`, and `publish.input_access` record
  caller-supplied workflow inputs when supplied and are `null` when omitted. GitHub Actions
  `workflow_call` defaults must not populate these fields.
- `publish.publish_config` records source `publishConfig` fields that affect publish intent. It is
  `null` when source `publishConfig` is absent. When present, it must contain only `registry`,
  `access`, `tag`, and `provenance`; it must not contain `directory` because
  `publishConfig.directory` is rejected before pack.
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
- `publish.trusted_publishing` and `publish.provenance_file` must both be `true`.
- `publish.package_identity_preexisting` must be `true`; the initial production profile does not
  support first publication of a package identity.
- `publish.package_version_preexisting` must be `false`; the selected package version must not exist
  before publish.
- `release.ref` must equal the release ref accepted by runtime guards.
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

The schema permits these optional fields only when the value is known and verifier-relevant:

- `package.publish_config_raw`: diagnostic copy of the source manifest `publishConfig` when needed
  for fixture debugging; it must not contain secrets and is not used to relax the normalized
  `publish.publish_config` schema.
- `package.packed_files`: array of packed file paths.
- `package.consumer_surface`: object containing packed `exports`, `main`, `type`, `bin`, `types`,
  `typings`, `typesVersions`, and `files` fields when present.
- `publish.custom_registry_support`: `unsupported-but-not-blocked` when `resolved_registry_url` is
  not `https://registry.npmjs.org/`.

Producer-side verification must reject the bundle before publish when any required field is missing,
has the wrong type, has an unexpected value, when `runtime.npm_version` is below `11.5.1`, or when
an unknown field is present.

## Digest semantics

- The provenance `subject[0].digest` must include `sha256`.
- The provenance may also include `sha512` as lowercase hexadecimal for npm integrity compatibility.
- The `sha256` digest is the canonical digest for cross-job handoff and verifier comparison.
- Public workflow output `package-tarball-sha512` is the tarball SHA-512 digest as 128 lowercase
  hexadecimal characters.
- The npm registry SRI integrity string is not stored in `subject[0].digest` and is not a public
  workflow output in the initial profile. It may be recorded only in registry metadata checks or
  verifier diagnostics.

## Windlass-generated SLSA provenance

The profile generates its own SLSA provenance v1 Statement and predicate. It does not rely on npm's
automatic provenance feature. The profile uses `actions/attest` only as a signing adapter to sign
the Windlass-generated Statement.

## `actions/attest` signing adapter

- The adapter is invoked with the full Statement payload.
- The adapter signs the payload as a Sigstore-backed DSSE bundle.
- The adapter may upload the bundle to GitHub artifact attestation storage.
- The adapter must not alter the Statement contents.

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

A bundle signed by another repository, another workflow path, a branch ref, a pull request ref, a
short SHA ref, a signer identity that does not match `runDetails.builder.id`, a source identity that
does not match `externalParameters.source`, or a non-GitHub OIDC issuer must be rejected before
publish.

## `npm publish --provenance-file`

- The `publish` job must use `npm publish --provenance-file=<bundle-path>` to publish the tarball
  with the Windlass-generated provenance.
- The profile must not use npm's automatic provenance generation.
- The profile must not fall back to token-based publish.
- Before running `npm publish`, the profile must check whether the selected registry already has the
  package identity and package version. If the package identity does not already exist, the workflow
  must fail clearly before attempting registry mutation because first publication is outside the
  initial trusted-publishing-only profile. If the package version already exists, the workflow must
  fail clearly before attempting registry mutation and must report that verification or inspection,
  not republish, is the correct operation.

## npm trusted publishing authentication

- The `publish` job uses OIDC trusted publishing.
- The caller job invoking the reusable workflow must grant `contents: read` and `id-token: write` so
  the called workflow can obtain the OIDC token required by npm trusted publishing.
- npm trusted publisher configuration must identify the caller repository and caller workflow
  filename, not `windlasstech/slsa-builder` or `.github/workflows/js-ts-npm-package-slsa3.yml`.
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
`publish.custom_registry_support: "unsupported-but-not-blocked"`. Registry metadata linkage checks
are registry-specific diagnostics for that target unless a later ADR defines a supported custom
registry class. A custom registry still must accept tokenless publish with the external provenance
bundle; otherwise `npm publish` fails and the workflow must fail.

## Workflow outputs

| Output                   | Description                                    |
| ------------------------ | ---------------------------------------------- |
| `package-name`           | Normalized npm package name.                   |
| `package-version`        | Package version from `package.json`.           |
| `package-registry-url`   | Normalized effective registry URL.             |
| `package-url`            | Package URL (`pkg:npm/...`).                   |
| `package-tarball-name`   | Tarball file name.                             |
| `package-tarball-sha256` | Tarball SHA-256, 64 lowercase hex characters.  |
| `package-tarball-sha512` | Tarball SHA-512, 128 lowercase hex characters. |

Outputs are release handles. They are not substitutes for signed provenance.

Workflow artifact names for the tarball and provenance bundle are internal same-run handoff handles,
not public `workflow_call.outputs`. npm SRI integrity values are registry diagnostics, not public
workflow outputs in the initial profile.

## Producer-side verification gate

Before `npm publish`, the `publish` job must verify:

1. The bundle signature is valid.
2. The signer identity is trusted.
3. The `predicateType` is `https://slsa.dev/provenance/v1`.
4. The `builder.id` matches the trusted policy.
5. The `buildType` matches the canonical JS/TS npm package `buildType`.
6. The `subject[0].digest.sha256` matches the tarball bytes.
7. The `subject[0].name` matches the expected tarball file name.
8. The `externalParameters` match the expected schema and values.

If any check fails, the job must fail before registry mutation.

## Failure behavior

The `publish` job must fail before `npm publish` when:

- Tarball digest mismatch.
- Bundle digest mismatch.
- Invalid signature.
- Unexpected signer.
- Wrong `predicateType`.
- Wrong `builder.id` or `buildType`.
- Unexpected or mismatched `externalParameters`.
- Source identity mismatch.
- Package identity mismatch.
- Package identity does not already exist in the selected registry.
- Package version already exists in the selected registry.
- Runtime policy mismatch.

The `publish` job must fail after `npm publish` when npmjs post-publish registry metadata
verification fails. This is a partial-publication failure: the package version may already exist in
the registry, and the workflow must report that state instead of retrying with weaker publication or
provenance behavior.

The profile must not fall back to:

- npm automatic provenance.
- Token-based publish.
- Unsigned provenance.
- Local-only provenance.
- GitHub-only attestations without the signed bundle.

## TDD and fixtures

- Positive fixture: accepted signed bundle leading to successful `npm publish`.
- Rejected fixtures: digest mismatch, signature mismatch, signer mismatch, wrong `predicateType`,
  wrong `builder.id`, wrong `buildType`, unexpected `externalParameters`, package identity mismatch,
  unsupported initial package publication, package-manager selection path mismatch, runtime policy
  mismatch, npm CLI below `11.5.1`, missing caller OIDC permission, npm trusted publisher caller
  identity mismatch, npmjs post-publish metadata mismatch, and npm automatic provenance fallback
  attempt.
- A fixture proving that the `publish` job cannot publish without the signed bundle.
