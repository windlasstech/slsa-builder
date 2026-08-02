# JS/TS npm Package Workflow Contract

This document defines the public reusable workflow contract for the initial JS/TS npm package
profile.

- Source ADRs: [0013](../decisions/0013-scope-initial-js-ts-profile-to-npm-packages.md),
  [0018](../decisions/0018-publish-one-js-ts-package-per-profile-run.md),
  [0022](../decisions/0022-use-js-ts-npm-package-slsa3-workflow-entrypoint.md),
  [0023](../decisions/0023-use-package-directory-as-required-js-ts-npm-package-selector.md),
  [0024](../decisions/0024-use-oidc-trusted-publishing-without-publish-secrets.md),
  [0026](../decisions/0026-document-supported-release-caller-patterns-and-runtime-guards.md),
  [0027](../decisions/0027-use-github-hosted-ubuntu-2404-and-node-24-runtime.md),
  [0030](../decisions/0030-accept-registry-url-while-guaranteeing-only-npmjs-semantics.md),
  [0032](../decisions/0032-constrain-manual-dispatch-releases-to-version-tags.md),
  [0034](../decisions/0034-do-not-support-private-dependency-credentials-in-initial-profile.md),
  [0057](../decisions/0057-provide-composed-public-npm-release-asset-workflow.md),
  [0058](../decisions/0058-define-github-release-asset-publisher-authority-boundary.md),
  [0059](../decisions/0059-define-public-npm-release-composed-workflow-interface.md),
  [0060](../decisions/0060-unify-npm-profile-public-entrypoint-with-release-asset-mode.md),
  [0064](../decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md),
  [0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [0067](../decisions/0067-converge-repeated-runs-within-run-identity.md)
- Related specs: [Core profile contract](core-profile-contract.md),
  [Identity and build types](identity-and-buildtypes.md),
  [SLSA provenance v1](slsa-provenance-v1.md), [JS/TS npm build and pack](js-ts-npm-build-pack.md),
  [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md)

## Scope and non-goals

**In scope:**

- Supported artifact type: a single npm package release.
- Workflow entrypoint and public `workflow_call` contract.
- Supported caller triggers and runtime guards.
- Required and optional inputs.
- Secrets and permissions.
- Unsupported modes and rejection criteria.

**Out of scope:**

- Exact package manager selection, install, build, or pack commands (build and pack spec).
- Provenance generation and publish graph internals (provenance and publish spec).
- Standalone GitHub Release asset publisher internals (publisher and composition specs).
- Private dependency credentials, JSR, or non-npm registry semantics.

## Supported artifact

This profile produces exactly one npm package release per run. The package may be a root package or
a workspace package, but it is always one package identity. When release-asset mode is enabled, the
same workflow run also attaches the pack-produced tarball and unchanged producer provenance sidecar
to an existing GitHub Release in the caller repository.

The initial production profile supports publishing a new version of an existing npm package identity
through OIDC trusted publishing. It does not support first publication of a package identity because
the profile does not accept npm tokens, OTP credentials, or other fallback credentials that npm may
require to create or configure a package for the first time. A package name that does not already
exist in the selected registry must fail before registry mutation with an unsupported initial
publication error.

## Unsupported artifact classes

The following are explicitly out of scope for the initial profile:

- Standalone tarballs without package identity.
- GitHub Release assets that are not the same verified npm package tarball produced by this run.
- Generic files or archives.
- Container images.
- JSR or other non-npm registries.
- Multiple packages in one run.

## Workflow entrypoint

The public reusable workflow entrypoint is:

```text
.github/workflows/js-ts-npm-package-slsa3.yml
```

This path is stable for the initial profile. The release manifest records the exact workflow SHA for
production use.

## `workflow_call` contract

### Required inputs

| Input               | Type   | Description                                                                         |
| ------------------- | ------ | ----------------------------------------------------------------------------------- |
| `package-directory` | string | Directory containing the package's `package.json`. Use `.` for the repository root. |

### Optional inputs

| Input                      | Type    | Default | Description                                                                |
| -------------------------- | ------- | ------- | -------------------------------------------------------------------------- |
| `registry-url`             | string  | unset   | Registry URL. Only npmjs semantics are guaranteed.                         |
| `dist-tag`                 | string  | unset   | npm dist-tag for the publish step.                                         |
| `access`                   | string  | unset   | `public`, `restricted`, or empty.                                          |
| `release-asset-mode`       | boolean | `false` | Enables GitHub Release asset publication for the verified package tarball. |
| `release-tag`              | string  | unset   | Existing GitHub Release tag name for release-asset mode.                   |
| `provenance-sidecar`       | string  | unset   | Sidecar policy; omitted or `required` for production release-asset mode.   |
| `linked-artifact-metadata` | boolean | `false` | Enables linked artifact storage metadata after release asset upload.       |

The workflow must not define GitHub Actions `workflow_call` defaults for `registry-url`, `dist-tag`,
`access`, `release-tag`, or `provenance-sidecar`. An omitted string input is represented as unset
until the profile's intent resolution step. This keeps caller-supplied values distinguishable from
source `publishConfig` values, Windlass/npm defaults, and release-asset mode defaults. Boolean
inputs may use GitHub Actions defaults because `false` is the explicit disabled state.

For the initial GitHub Actions reusable workflow contract, an optional string input whose value is
an empty string after trimming ASCII whitespace is normalized as omitted before intent resolution.
Empty `registry-url`, `dist-tag`, `access`, `release-tag`, and `provenance-sidecar` inputs are
therefore not caller-supplied intent values. A caller-supplied value exists only when the normalized
input is non-empty.

#### Optional input rules

> [!IMPORTANT] Custom (third-party) npm registry support is an explicit non-goal of the first
> milestone. The behavior in this section remains the eventual contract but is deferred:
> first-milestone conformance does not require it. Consistent with ADR 0030's "unsupported but not
> blocked" stance, this deferral does not prohibit attempts; their results are outside
> first-milestone conformance scope. Promoting custom registries to supported, or blocking them,
> requires a new ADR.

Custom registry diagnostics use the stable IDs defined by the
[verification policy and fixtures](verification-policy-and-fixtures.md#stable-diagnostic-ids). A
non-npmjs registry URL is not itself a failed preflight condition. The profile may emit
`windlass.verify.warning.custom-registry-preflight-inconclusive` when its best-effort tokenless
metadata preflight cannot establish the requested state, then it must continue to the tokenless
publish attempt unless a separately proved failure condition below applies.

- `registry-url` must be an absolute `https:` URL. The profile must normalize scheme and host to
  lowercase, remove default port `443`, and ensure exactly one trailing `/` in the effective
  `package-registry-url` output. A non-HTTPS registry URL, URL with userinfo, fragment, or query, or
  URL whose path is not `/` must be rejected before install, pack, publish, or signing. An empty
  `registry-url` input is normalized as omitted before URL validation.
- `https://registry.npmjs.org/` is the only registry URL with Windlass-guaranteed production
  semantics. Other HTTPS registry URLs are unsupported-but-not-blocked only if they complete the
  same tokenless publish and external provenance submission flow.
- `dist-tag` must be a non-empty npm dist-tag string that does not contain whitespace, `/`, `\\`,
  NUL, or path traversal segments. An empty `dist-tag` input is normalized as omitted before
  dist-tag validation.
- `access` must be one of `public`, `restricted`, or an empty string. An empty `access` value means
  omitted for publish intent resolution; it does not override source `publishConfig.access`.
- `release-asset-mode` must be `false` for npm-only publication and `true` for npm publication plus
  GitHub Release asset distribution. The workflow must not infer release-asset mode from the
  presence of release-related inputs.
- `release-tag` is used only when `release-asset-mode` is `true`. It is a Git tag name, not a full
  ref. When omitted, the effective release tag is the current release tag accepted by the runtime
  guards. When supplied, it must equal the current tag name, must reconstruct the same full
  `refs/tags/<tag-name>` ref as `github.ref`, and must equal `v${package.json version}`. A branch
  name, pull request ref, full ref, empty tag name, tag name with path traversal or ASCII control
  characters, or a tag that does not already have a GitHub Release in the caller repository must be
  rejected before release mutation.
- `provenance-sidecar` is used only when `release-asset-mode` is `true`. Omitted and `required` both
  mean the unchanged producer provenance bundle is uploaded as the deterministic sidecar
  `<package-tarball-name>.intoto.jsonl`. Any value that disables, renames, rewrites, re-signs, or
  replaces the sidecar is rejected because ADR 0051 makes sidecar distribution mandatory for the
  production release-asset path.
- `linked-artifact-metadata` is used only when `release-asset-mode` is `true`. When `false`, the
  workflow must not request `artifact-metadata: write` or call the linked artifact metadata API.
  When `true`, the workflow derives the publisher `linked-artifact-settings` object from the caller
  repository, effective release tag, release download URL prefix, and package version; callers do
  not supply the publisher's internal JSON settings directly.
- `publish_access_option` is the exact value passed to `npm publish --access`; it is `public`,
  `restricted`, or `null` when the option is omitted.
- `effective_access` records the Windlass publish intent used for diagnostics and verification. When
  `publish_access_option` is `null`, `effective_access` is `existing-package-access`, meaning the
  publish operation must not create or change package access and relies on the existing registry
  package access state. npm's first-publication default for scoped packages without
  `--access public` is restricted, but first publication is outside the initial Windlass production
  profile.
- When `registry-url` is not `https://registry.npmjs.org/`, a caller-supplied non-empty `access` is
  passed only if the registry accepts it during the same tokenless publish flow. A registry response
  that proves token or OTP authentication is required fails before publish with
  `windlass.verify.error.custom-registry-token-required`. An access-option rejection without proof
  of a token or OTP requirement fails before publish at the publish boundary with
  `windlass.verify.error.custom-registry-access-option-rejected`. The workflow must not silently
  drop the option and continue.

#### Mode validation

The public npm workflow has two modes:

| Mode          | `release-asset-mode` | Observable behavior                                                               |
| ------------- | -------------------- | --------------------------------------------------------------------------------- |
| npm-only      | `false`              | Build, sign, publish to npm, and emit npm package outputs only.                   |
| release-asset | `true`               | Run npm-only behavior, then internally compose with the GitHub Release publisher. |

Release-asset mode is explicit. The workflow must reject release-asset-only inputs when
`release-asset-mode` is `false`, including non-empty `release-tag`, non-empty `provenance-sidecar`,
or `linked-artifact-metadata: true`. This prevents callers from believing that release assets were
published when the mode was not enabled.

When `release-asset-mode` is `true`, the workflow must construct the same-run internal handoff
described by the [composition spec](npm-to-release-asset-composition.md). The caller cannot supply
publisher handoff fields, artifact names, artifact digests, release upload URLs, target repository
coordinates, custom GitHub tokens, overwrite behavior, release creation behavior, raw file paths, or
multi-asset configuration through this public npm workflow. Unsupported public inputs must fail
GitHub Actions schema validation or be rejected before npm publish, signing, release mutation, or
linked metadata publication.

The public release target is always the caller repository's existing GitHub Release for the
effective release tag. The workflow must not create the release or tag, change draft or prerelease
status, change the latest marker, delete an existing asset, overwrite an asset, or upload to another
repository.

After the pack-produced tarball and the unchanged signed producer provenance sidecar have known
names and digests, and before `npm publish`, release-asset mode must read the existing release's
`draft` and `immutable` state and evaluate both expected release assets. This target-state gate is
fail-fast evidence only, not authority to mutate the release. If the target is immutable and either
required asset is absent, the workflow must fail before npm mutation with
`windlass.verify.error.release-target-immutable`. If the target is immutable and both required
assets exist, the workflow may proceed only when both can satisfy complete same-`run_id` read-only
convergence. The publisher must re-read the target state at mutation-segment entry; it must not
reuse this earlier observation as mutation authorization.

#### Publish intent resolution

The effective publish intent is resolved from caller-supplied workflow inputs, source
`publishConfig`, and Windlass/npm defaults in that order. Defaults are applied only during this
resolution step, not by GitHub Actions `workflow_call` defaults. Caller-supplied workflow inputs
must not silently override conflicting source metadata.

Resolution rules:

- `registry-url` resolves from the caller-supplied non-empty workflow input when supplied, otherwise
  from `publishConfig.registry` when present, otherwise `https://registry.npmjs.org/`.
- `dist-tag` resolves from the caller-supplied non-empty workflow input when supplied, otherwise
  from `publishConfig.tag` when present, otherwise `latest`.
- `access` resolves from the caller-supplied non-empty workflow input when supplied, otherwise from
  `publishConfig.access` when present, otherwise the documented npm default represented by an empty
  publish option. A caller input of `access: ""` is omitted and therefore allows
  `publishConfig.access` to supply the value.
- `publishConfig.provenance`, when present, must not be `false`; the profile always uses
  Windlass-generated external provenance and must reject metadata that attempts to disable
  provenance submission.
- `publishConfig.directory` is unsupported in the initial profile and must be rejected because it
  can redirect the packed package root away from the selected `package-directory`.
- Other source `publishConfig` members that do not affect registry selection, dist-tag selection,
  access selection, provenance enablement, or package-root selection are not verifier-relevant
  publish intent. Their presence in the source manifest must not by itself fail the production
  profile. Implementations must ignore them for publish intent resolution and must not pass them to
  `npm publish` unless another rule in this spec explicitly allows that behavior. When useful for
  fixture debugging, they may appear only in the diagnostic `package.publish_config_raw` field
  defined by the provenance spec; they must not appear in the normalized `publish.publish_config`
  object.

Conflict rules:

- If a caller-supplied workflow input and the corresponding `publishConfig` field are both present
  and normalize to different effective values, the workflow must fail before install, pack, publish,
  or signing.
- If they normalize to the same effective value, the workflow may proceed and must record both the
  supplied input and source metadata in provenance when verifier-relevant.
- A Windlass/npm default must not create a conflict with `publishConfig`. For example, if `dist-tag`
  is omitted and `publishConfig.tag` is `next`, the resolved dist tag is `next`, not a conflict with
  the default `latest`.
- The workflow must not silently prefer workflow inputs over `publishConfig`, silently prefer
  `publishConfig` over workflow inputs, or drop a conflicting field to continue.

Examples:

- Omitted `access` plus `publishConfig.access: "public"` resolves to `public` and passes
  `npm publish --access public`.
- `access: ""` plus `publishConfig.access: "public"` also resolves to `public`; the empty input is
  omitted and does not create a conflict.
- `access: ""` with no `publishConfig.access` omits the `npm publish --access` option and records
  `effective_access` as `existing-package-access`.
- `access: "restricted"` plus `publishConfig.access: "public"` fails with a publish intent conflict.
- `dist-tag: ""` plus `publishConfig.tag: "next"` resolves to `next`.

### Secrets

The profile must not require or expose long-lived publish secrets, npm tokens, OTP secrets, or
dependency-fetch credentials. OIDC trusted publishing is the only supported production
authentication mechanism.

Release-asset mode must not require or expose a custom GitHub token, personal access token, GitHub
App token, release upload URL, or cross-repository release credential. GitHub Release mutation uses
only the caller-scoped `GITHUB_TOKEN` permissions granted to the reusable workflow invocation and
reduced by internal jobs.

### Caller trusted publishing requirements

The caller workflow is part of the npm trusted publishing public contract. A production caller job
that invokes this reusable workflow must grant at least:

```yaml
permissions:
  contents: read
  id-token: write
```

npm trusted publisher configuration for the package must identify the **caller** repository and the
caller workflow filename that invokes this reusable workflow. It must not identify
`windlasstech/slsa-builder` or `.github/workflows/js-ts-npm-package-slsa3.yml` as the package's
trusted publisher workflow. npm validates the calling workflow identity for reusable workflow
publishing, while Windlass provenance separately records and verifies the SHA-pinned reusable
workflow builder identity.

This npmjs.com trusted publisher configuration is registry-side publish authorization policy, not a
SLSA `externalParameters` field. The profile must document it as caller setup and enforce it through
the producer-side publish gate, but consumer-side SLSA provenance verification is not required to
reconstruct or re-verify the npmjs.com trusted publisher settings from the signed provenance bundle.

The production profile must fail before registry mutation when the caller job cannot provide an OIDC
token to the called workflow, when npm trusted publishing is not configured for the caller
repository and caller workflow filename, or when the caller repository/workflow identity observed by
npm does not match the package's trusted publisher policy. The workflow must not recover from these
failures by accepting `NPM_TOKEN`, `NODE_AUTH_TOKEN`, an OTP secret, or any other publish-capable
credential.

### Caller permissions by mode

The caller job that invokes `.github/workflows/js-ts-npm-package-slsa3.yml` must grant permissions
for the selected mode. The called workflow must still reduce permissions at each internal job so
build, publish, signing, release upload, and optional metadata authorities remain separated.

| Mode                               | Required caller permissions                                                             | Notes                                                                     |
| ---------------------------------- | --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| npm-only                           | `contents: read`, `id-token: write`, `attestations: write`                              | Enables checkout, npm trusted publishing, and producer bundle signing.    |
| release-asset                      | `contents: write`, `id-token: write`, `attestations: write`                             | Adds release upload authority for the existing caller-repository release. |
| release-asset with linked metadata | `contents: write`, `id-token: write`, `attestations: write`, `artifact-metadata: write` | Adds linked artifact metadata only when explicitly enabled.               |

`contents: write` is required at the caller job level only when release-asset mode is enabled. The
called workflow must grant it only to the internal release upload job. Signing jobs must not have
release mutation authority, release upload jobs must not have signing or package publishing
authority, and linked artifact metadata jobs must not have signing or release upload authority.

If a caller enables release-asset mode without `contents: write`, the workflow must fail before
release mutation. If a caller enables linked artifact metadata without `artifact-metadata: write`,
the workflow must fail before metadata publication. If the workflow's internal jobs combine
authorities that must remain separate, implementation review and YAML fixtures must reject the
workflow even when GitHub would allow the permission set.

### Job-class concurrency (ADR 0066)

The build, pack, and producer signing jobs in this profile are PRE-mutation jobs. Each of these jobs
must declare job-level concurrency with `cancel-in-progress: true`; a workflow that omits that
declaration, sets it to `false`, or places one of these jobs in a mutation concurrency class must be
rejected by the YAML review gate before release use. A newer run for the same release intent
therefore cancels stale build, pack, or signing work early. This cancellation is safe because none
of these jobs has performed registry or GitHub Release mutation. Signing may leave an inert
transparency log entry, but verification binds attestations to published artifact bytes rather than
treating the entry itself as publication.

For each PRE-mutation job, the concurrency group key must consist only of a job-specific namespace,
`github.repository`, `github.ref_name`, and, when needed to distinguish documented release intent,
declared workflow inputs. A key that uses any other context or omits the repository, release source
ref, or job-specific namespace must fail the YAML review gate because it can collide across release
intents or make jobs within one run contend with one another. The key must not include
`github.workflow`; inside a called reusable workflow that value resolves to the caller's workflow
name and creates a self-cancellation trap. Any key containing `github.workflow` must fail the YAML
review gate.

The PRE-mutation/mutation boundary lies after the signed producer bundle has been generated and
verified and before the npm publish job begins. npm publication is the first registry mutation, and
GitHub Release asset upload in release-asset mode is a later release mutation. PRE-mutation jobs
must not hold registry or release mutation authority; a permission or call path that permits such a
side effect must fail authority-boundary review before release use. Mutation-job serialization and
precondition re-validation are defined by ADR 0066 and the
[provenance and publish spec](js-ts-npm-provenance-publish.md); they must not be replaced by the
PRE-mutation `cancel-in-progress: true` policy, and a workflow that applies that policy to a
mutation job must be rejected because cancellation could interrupt an external side effect.

The npm publish job, the release-asset upload jobs, and the manifest publish job use this exact
shared mutation concurrency key:

```text
release-mutation-${{ github.repository }}-${{ github.ref_name }}
```

The mutation key must not include `github.workflow` or any other component; a workflow that uses a
different mutation key must fail the YAML review gate. PRE-mutation groups retain their job-specific
namespaces so jobs within one run do not contend with one another.

### Outputs

| Output                       | Type   | Description                                                                                                                         |
| ---------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------- |
| `package-name`               | string | Normalized npm package name.                                                                                                        |
| `package-version`            | string | Package version from `package.json`.                                                                                                |
| `package-registry-url`       | string | Normalized effective registry URL.                                                                                                  |
| `package-url`                | string | Registry package-version URL for the published package.                                                                             |
| `package-tarball-name`       | string | Name of the tarball produced for npm publish.                                                                                       |
| `package-tarball-sha256`     | string | SHA-256 of the tarball as 64 lowercase hex characters.                                                                              |
| `package-tarball-sha512`     | string | SHA-512 of the tarball as 128 lowercase hex characters.                                                                             |
| `release-asset-name`         | string | Uploaded primary GitHub Release asset name, or unset in npm-only mode.                                                              |
| `release-asset-url`          | string | Browser URL of the uploaded primary asset, or unset in npm-only mode.                                                               |
| `release-asset-sha256`       | string | SHA-256 of the uploaded primary asset, or unset in npm-only mode.                                                                   |
| `provenance-sidecar-name`    | string | Deterministic provenance sidecar name, or unset in npm-only mode.                                                                   |
| `provenance-sidecar-url`     | string | Browser URL of the sidecar asset, or unset when unavailable.                                                                        |
| `provenance-sidecar-sha256`  | string | SHA-256 of the exact producer bundle sidecar, or unset before bundle verification.                                                  |
| `native-provenance-locators` | string | UTF-8 JSON array of diagnostic producer-native provenance locators, or unset.                                                       |
| `release-upload-result`      | string | `disabled`, `completed`, `failed-before-upload`, `partial-primary-uploaded`, `foreign-conflict`, or `indeterminate-primary-upload`. |
| `linked-artifact-result`     | string | `disabled`, `created`, or `failed-after-upload`.                                                                                    |
| `linked-artifact-url`        | string | Stable browser or API URL for linked artifact metadata, or unset.                                                                   |
| `linked-artifact-id`         | string | Stable API identifier for linked artifact metadata, or unset.                                                                       |

Outputs are release handles for downstream workflows and human operators. They are not substitutes
for signed provenance.

Release-asset outputs mirror the publisher result categories but remain public result handles, not
public composition inputs. Internal handoff artifact names, handoff manifest names, handoff manifest
digests, publisher `primary-artifact-name`, publisher `producer-provenance-artifact-name`, and
publisher policy JSON are not public outputs of this workflow.

In npm-only mode, all release-asset outputs must be unset except `release-upload-result`, which is
`disabled`, and `linked-artifact-result`, which is `disabled`. In release-asset mode, successful
primary and sidecar upload sets `release-upload-result` to `completed`, sets the primary asset and
sidecar locator outputs, and sets `linked-artifact-result` according to the linked metadata setting.
Partial and indeterminate release upload states follow the publisher output rules and must not be
reported as successful publication.

`linked-artifact-url` and `linked-artifact-id` mirror the standalone publisher locator outputs. They
must be unset when `linked-artifact-result` is `disabled` or `failed-after-upload`, when release
asset upload did not complete, or when the metadata API did not return the corresponding locator.
They are set only when linked metadata creation succeeds and are not substitutes for signed
provenance, release asset digests, or sidecar outputs.

### `package-url` output

`package-url` is the registry package-version metadata URL for the published npm package.

> [!WARNING]  
> It is not a Package URL (PURL), and the initial profile must not emit `pkg:npm/...` in this output
> or in `externalParameters.package.package_url`.

For `https://registry.npmjs.org/`, the initial profile emits the npm registry package-version
metadata URL:

```text
https://registry.npmjs.org/<registry-escaped-package-name>/<version>
```

The registry-escaped package name is the validated npm package name with URL percent-encoding for
path-unsafe bytes. For scoped package names, the slash between scope and name is encoded as `%2F`.
The version path segment is the validated package version.

Canonical npmjs examples:

| npm package identity     | Version | `package-url`                                                 |
| ------------------------ | ------- | ------------------------------------------------------------- |
| `left-pad`               | `1.3.0` | `https://registry.npmjs.org/left-pad/1.3.0`                   |
| `@windlass/slsa-builder` | `1.2.3` | `https://registry.npmjs.org/%40windlass%2Fslsa-builder/1.2.3` |

For non-npmjs registries, `package-url` is best-effort diagnostic metadata derived from the
normalized `package-registry-url`, the validated package name, and the validated package version
using the same path construction rule. Windlass does not guarantee that the resulting URL is a
stable human page or metadata endpoint unless a later ADR defines that registry class.

The profile must fail before signing or publishing when it cannot construct a URL from the
normalized registry URL, validated package name, and validated package version. Producer-side
verification must reject a provenance bundle before publish when
`externalParameters.package.package_url` is not byte-for-byte equal to the expected registry
package-version URL reconstructed from `externalParameters.publish.resolved_registry_url`,
`externalParameters.package.name`, and `externalParameters.package.version`.

Rejected `package-url` examples for the initial profile:

- `https://www.npmjs.com/package/@windlass/slsa-builder` because it omits the package version.
- `https://www.npmjs.org/@windlass/slsa-builder/v/1.2.3` because npm web UI package pages are not
  registry metadata URLs for this output.
- `https://registry.npmjs.org/%40windlass%2Fslsa-builder` because it omits the package version path
  segment.
- `https://registry.npmjs.org/%40windlass%2Fslsa-builder/1.2.4` when the validated package version
  is `1.2.3`.
- `https://registry.npmjs.org/left-pad/latest` because dist-tags are not version identifiers for
  this output.

The signed npm provenance Statement subject is a Package URL as defined by the
[provenance and publish spec](js-ts-npm-provenance-publish.md#npm-package-subject-naming). That
subject identity must not be exposed by overloading the public `package-url` output or
`externalParameters.package.package_url`, both of which remain registry package-version URLs.

## Supported caller triggers

The profile supports the following production caller patterns:

1. **Push of a SemVer tag** matching `v${package.json version}`.
2. **Constrained manual dispatch** from a tag ref that matches the package version.

Any other trigger, including untagged pushes, branch-based pushes, pull requests, and arbitrary
manual dispatch refs, is rejected.

### Runtime guards

- `github.ref_type` must be `tag` for production release runs.
- The tag must match `v${package.json version}` exactly.
- `github.event_name` must be one of the supported triggers.
- The workflow must run on `ubuntu-24.04` GitHub-hosted runners.
- The Node.js runtime must be version 24.

## Manual dispatch constraints

A manual dispatch release must satisfy all of the following:

- The workflow is invoked from a tag ref.
- The tag matches `v${package.json version}`.
- The tag already exists in the repository.
- The caller does not supply arbitrary runtime overrides.

## Registry URL support

> [!IMPORTANT] Custom (third-party) npm registry support is an explicit non-goal of the first
> milestone. The behavior in this section remains the eventual contract but is deferred:
> first-milestone conformance does not require it. Consistent with ADR 0030's "unsupported but not
> blocked" stance, this deferral does not prohibit attempts; their results are outside
> first-milestone conformance scope. Promoting custom registries to supported, or blocking them,
> requires a new ADR.

- The profile accepts a `registry-url` input.
- The profile guarantees only npmjs publish semantics.
- A non-npmjs registry may be accepted as a transport target, but unsupported registry behavior is
  at the caller's risk and must be recorded in provenance as
  `publish.custom_registry_support: "unsupported-but-not-blocked"`.
- `unsupported-but-not-blocked` means only that the workflow may attempt the same no-secret external
  provenance publish flow against the selected HTTPS registry. It is not a Windlass guarantee that
  the registry implements npmjs metadata semantics, provenance discovery, package access behavior,
  dist-tag behavior, or registry-side verification APIs.
- A non-npmjs registry attempt is allowed only when all minimum production invariants remain true:
  the normalized registry URL passes the profile URL rules, the publish path can run without
  `NPM_TOKEN`, `NODE_AUTH_TOKEN`, OTP, private dependency credentials, or publish-capable secrets,
  and `npm publish` can submit the exact Windlass signed bundle through
  `--provenance-file=<bundle-path>` or an equivalent no-secret external provenance-file mechanism.
- A non-npmjs registry that requires a token or OTP before mutation fails before publish with
  `windlass.verify.error.custom-registry-token-required`. The profile must not add a token, OTP, or
  secret fallback.
- A detected requirement for npm automatic provenance, unsigned provenance, omission of the exact
  Windlass bundle, bundle rewriting, or dropping caller-supplied `--access` intent fails before
  publish with `windlass.verify.error.custom-registry-provenance-weakened`.
- A tokenless authentication rejection fails at the authentication or publish boundary with
  `windlass.verify.error.custom-registry-tokenless-auth-failed`.
- Rejection of the exact external provenance file during publish fails at the publish boundary with
  `windlass.verify.error.custom-registry-provenance-submission-rejected`. The diagnostic report must
  state whether the package mutation could have committed.
- Missing required registry linkage metadata after publication fails post-publish with
  `windlass.verify.error.custom-registry-linkage-metadata-absent`. Absent, malformed, incompatible,
  or mismatched registry digest semantics fail post-publish with
  `windlass.verify.error.custom-registry-digest-semantics-mismatch`. Each report must state that a
  partial publication may have occurred.
- The profile must fail before registry mutation when publishing to `https://registry.npmjs.org/`
  and the selected package identity does not already exist. The initial npmjs production path
  publishes new versions of existing packages only; it does not create the first version of a
  package identity. For non-npmjs registries, package identity and package version preflight checks
  are best-effort diagnostics unless a later ADR defines that registry class. The custom registry
  still must complete tokenless trusted publishing with the supplied external provenance bundle.
- For non-npmjs registries, preflight metadata diagnostics have three observable outcomes:
  - If a tokenless metadata check proves the package identity or version state, the workflow records
    the proven boolean values in provenance and may continue.
  - If no tokenless metadata check is available, or the check is inconclusive without weakening the
    no-secret and provenance-file contract, the workflow records `null` for the unproven state and
    may continue to the tokenless publish attempt.
  - If the metadata check proves that a token or OTP is required, the workflow must fail before
    publish with `windlass.verify.error.custom-registry-token-required`. If it proves a provenance
    weakening, it must fail before publish with
    `windlass.verify.error.custom-registry-provenance-weakened`. The profile must never substitute a
    token, OTP, secret, unsigned provenance, or npm automatic-provenance fallback.
- A custom registry run that records `null` preflight fields must still submit the exact Windlass
  signed bundle unchanged during publish. `null` means only that package identity or version state
  was not verifier-proven before publish; it does not permit weaker authentication, weaker
  provenance, or reporting the registry as Windlass-guaranteed.
- For npmjs, post-publish registry metadata checks are required by the provenance and publish spec.
  For custom registries, linkage and digest checks are required fail-clearly observations, not
  successful-registry conformance. They must use the registered post-publish diagnostics above and
  must not be reported as Windlass-guaranteed unless a later ADR defines that registry class.

## Private dependency credentials

The initial profile does not support private dependency credentials. A package that requires private
registry authentication or dependency-fetch secrets must use a different workflow or a future
profile.

The initial profile also rejects packages whose selected source manifest has `private: true`. It
does not provide a pack-only, provenance-only, or no-publish mode for private npm packages.

## Rejected caller inputs

The profile must reject any attempt to supply:

- Arbitrary build, install, or pack commands.
- Package manager override inputs.
- Runtime environment overrides.
- npm token or OTP secrets.
- Inherited broad secrets.
- Multi-package selection inputs.
- Branch or pull-request based release triggers.
- Release target repository owner, release target repository name, release upload URL, release API
  URL, release creation, tag creation, draft mutation, prerelease mutation, latest-marker mutation,
  asset overwrite, asset delete, or asset replacement inputs.
- Custom GitHub token, personal access token, GitHub App token, raw artifact path, arbitrary local
  file upload, caller-supplied release asset digest, internal artifact name, handoff manifest name,
  handoff manifest digest, or publisher handoff field inputs.

Unsupported release-asset inputs must be rejected, not silently ignored, when their presence could
make a caller believe that a release asset, sidecar, metadata record, target repository, or
overwrite policy was applied. Unknown inputs normally fail GitHub Actions reusable workflow schema
validation before the workflow starts.

## Failure behavior

The workflow must fail before any registry mutation when:

- The trigger is not supported.
- The tag does not match the package version.
- The package directory does not contain a valid `package.json`.
- The selected package manifest has `private: true`.
- The package manager selection is ambiguous or unsupported.
- The runtime environment is not `ubuntu-24.04` with Node.js 24.
- Private dependency credentials are required.
- The caller job cannot provide OIDC credentials to the called reusable workflow.
- npm trusted publisher configuration does not match the caller repository and caller workflow
  filename.
- Explicit workflow inputs conflict with source `publishConfig` fields.
- `publishConfig.provenance` disables provenance or `publishConfig.directory` redirects the package
  root.
- Any optional input fails validation.
- Release-asset-only inputs are supplied while `release-asset-mode` is `false`.
- The selected registry cannot complete tokenless trusted publishing with the supplied external
  provenance bundle. Custom registry failures use the fixed timing and diagnostics in Registry URL
  support: `custom-registry-token-required` and `custom-registry-provenance-weakened` fail before
  publish, `custom-registry-access-option-rejected`, `custom-registry-tokenless-auth-failed`, and
  `custom-registry-provenance-submission-rejected` fail at the authentication or publish boundary,
  and `custom-registry-linkage-metadata-absent` and `custom-registry-digest-semantics-mismatch` fail
  post-publish with possible partial publication reported.
- The selected package identity does not already exist on npmjs when publishing to
  `https://registry.npmjs.org/`.

When `release-asset-mode` is `true`, the workflow must additionally fail before release mutation or
linked metadata publication when:

- The caller job does not grant `contents: write` to the reusable workflow invocation.
- `linked-artifact-metadata` is `true` and the caller job does not grant `artifact-metadata: write`.
- The effective release tag cannot be reconstructed as the same full ref as the runtime release ref.
- The producer provenance `externalParameters.source.ref`, `externalParameters.release.ref`, or
  `externalParameters.release.version_tag` does not bind to that same full release ref.
- The effective release tag does not already have a GitHub Release in the caller repository.
- The effective release target resolves outside the caller repository.
- The pack-produced tarball, producer provenance bundle, or internal handoff manifest is missing,
  malformed, unverifiable, or digest-mismatched.
- The primary release asset name or deterministic sidecar name already exists on the target release
  for a new `run_id`, a different run identity, or without the required binding and convergence
  proof. Existing names are not a failure when the target satisfies complete same-`run_id` read-only
  convergence.
- The producer provenance sidecar is disabled, renamed, rewritten, re-signed, replaced by a native
  locator, or otherwise not the exact signed bundle bytes produced and verified by the npm profile.
- The internal mapping job attempts to derive publisher inputs from public workflow outputs, logs,
  deterministic names alone, or caller-controlled values instead of the digest-verified handoff
  manifest.
- Internal jobs combine authorities that ADR 0060 requires to stay separate, including release
  mutation in build, signing, publish, mapping, or metadata jobs; signing authority in release
  upload jobs; or release mutation authority in linked metadata jobs.
- The pre-publication target-state gate observes an immutable release with either expected release
  asset absent. This must fail before `npm publish` with
  `windlass.verify.error.release-target-immutable`.
- The pre-publication target-state gate observes a complete immutable asset pair that cannot satisfy
  same-`run_id` read-only convergence. This must fail before `npm publish` with
  `windlass.verify.error.release-target-immutable`.

If npm publish succeeds but release asset upload later fails, the workflow must report the mode as a
partial release failure and must not retry by overwriting assets, deleting assets, changing the
release target, weakening provenance verification, or using a custom token.

## TDD and fixtures

- Positive fixture: valid tag push with root package and workspace package.
- Positive fixture: valid release-asset mode run that publishes the npm package, uploads the same
  tarball as the GitHub Release primary asset, uploads the unchanged producer provenance sidecar,
  and emits release asset locator outputs.
- Positive fixture: valid release-asset mode run with linked artifact metadata enabled and separated
  metadata permissions.
- Rejected fixtures: wrong trigger, mismatched tag/version, missing `package.json`, arbitrary
  command input, npm token secret, private package, private dependency requirement, `publishConfig`
  conflict, unsupported `publishConfig.directory`, disabled provenance metadata, producer-side
  missing caller OIDC permission, producer-side npm trusted publisher caller identity mismatch, and
  unsupported registry behavior, a custom registry that requires token or OTP before mutation
  (`custom-registry-token-required`), a detected weakened provenance path before mutation
  (`custom-registry-provenance-weakened`), access-option rejection without token or OTP proof at the
  publish boundary (`custom-registry-access-option-rejected`), tokenless authentication rejection at
  the authentication boundary (`custom-registry-tokenless-auth-failed`), external provenance-file
  rejection at publish with mutation-commit status reported
  (`custom-registry-provenance-submission-rejected`), absent linkage metadata after publish
  (`custom-registry-linkage-metadata-absent`), and incompatible digest semantics after publish with
  possible partial publication reported (`custom-registry-digest-semantics-mismatch`).
- Rejected fixtures: release-asset-only inputs while mode is disabled, missing caller
  `contents: write` for release-asset mode, missing caller `artifact-metadata: write` when linked
  metadata is enabled, supplied full-ref `release-tag`, release tag mismatch, missing target
  release, cross-repository target attempt, custom GitHub token attempt, raw artifact upload
  attempt, caller-supplied artifact digest attempt, overwrite attempt, release creation attempt,
  sidecar disable or rename attempt, duplicate primary asset, duplicate sidecar asset, internal
  handoff substitution, internal job permission-boundary violation, immutable target with either
  expected asset absent before `npm publish` (`release-target-immutable`), and a complete immutable
  target that cannot perform same-`run_id` read-only convergence (`release-target-immutable`).
- A YAML review checklist that a human can apply to the workflow file.
- A YAML review checklist proving that `.github/workflows/js-ts-npm-package-slsa3.yml` is the only
  public npm entrypoint and that release-asset mode does not expose internal handoff mechanics as
  public inputs or outputs.
- A YAML review checklist proving that build, pack, and producer signing jobs use PRE-mutation
  concurrency with `cancel-in-progress: true`; group keys use only the ADR 0066 composition and
  never `github.workflow`; and no mutation job uses the PRE-mutation cancellation policy.
- A YAML review checklist proving that no registry allowlist or registry-identity preflight rejects
  a non-npmjs URL; inconclusive custom-registry preflight emits only
  `custom-registry-preflight-inconclusive` and permits the tokenless attempt; and every custom
  registry failure surface emits its registered diagnostic at the required stage.
- A YAML review checklist proving that release-asset mode reads `draft` and `immutable` after both
  expected asset names and digests are known but before `npm publish`, rejects incomplete immutable
  targets with `release-target-immutable`, permits only same-`run_id` read-only convergence for a
  complete immutable target, and re-reads target state at publisher mutation-segment entry.
