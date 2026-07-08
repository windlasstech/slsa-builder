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
  [0034](../decisions/0034-do-not-support-private-dependency-credentials-in-initial-profile.md)
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
- GitHub Release asset distribution (publisher and composition specs).
- Private dependency credentials, JSR, or non-npm registry semantics.

## Supported artifact

This profile produces exactly one npm package release per run. The package may be a root package or
a workspace package, but it is always one package identity.

The initial production profile supports publishing a new version of an existing npm package identity
through OIDC trusted publishing. It does not support first publication of a package identity because
the profile does not accept npm tokens, OTP credentials, or other fallback credentials that npm may
require to create or configure a package for the first time. A package name that does not already
exist in the selected registry must fail before registry mutation with an unsupported initial
publication error.

## Unsupported artifact classes

The following are explicitly out of scope for the initial profile:

- Standalone tarballs without package identity.
- GitHub Release assets (use the publisher profile instead).
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

| Input          | Type   | Default | Description                                        |
| -------------- | ------ | ------- | -------------------------------------------------- |
| `registry-url` | string | unset   | Registry URL. Only npmjs semantics are guaranteed. |
| `dist-tag`     | string | unset   | npm dist-tag for the publish step.                 |
| `access`       | string | unset   | `public`, `restricted`, or empty.                  |

The workflow must not define GitHub Actions `workflow_call` defaults for these optional inputs. An
omitted input is represented as unset until the profile's publish intent resolution step. This keeps
caller-supplied values distinguishable from source `publishConfig` values and Windlass/npm defaults.

For the initial GitHub Actions reusable workflow contract, an optional string input whose value is
an empty string after trimming ASCII whitespace is normalized as omitted before publish intent
resolution. Empty `registry-url`, `dist-tag`, and `access` inputs are therefore not caller-supplied
publish intent values. A caller-supplied value exists only when the normalized input is non-empty.

#### Optional input rules

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
- `publish_access_option` is the exact value passed to `npm publish --access`; it is `public`,
  `restricted`, or `null` when the option is omitted.
- `effective_access` records the Windlass publish intent used for diagnostics and verification. When
  `publish_access_option` is `null`, `effective_access` is `existing-package-access`, meaning the
  publish operation must not create or change package access and relies on the existing registry
  package access state. npm's first-publication default for scoped packages without
  `--access public` is restricted, but first publication is outside the initial Windlass production
  profile.
- When `registry-url` is not `https://registry.npmjs.org/`, a caller-supplied non-empty `access` is
  passed only if the registry accepts it during the same tokenless publish flow. If the registry
  rejects the access option or requires token/OTP fallback, the workflow fails; it must not silently
  drop the option and continue.

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

### Outputs

| Output                   | Type   | Description                                             |
| ------------------------ | ------ | ------------------------------------------------------- |
| `package-name`           | string | Normalized npm package name.                            |
| `package-version`        | string | Package version from `package.json`.                    |
| `package-registry-url`   | string | Normalized effective registry URL.                      |
| `package-url`            | string | Registry package-version URL for the published package. |
| `package-tarball-name`   | string | Name of the tarball produced for npm publish.           |
| `package-tarball-sha256` | string | SHA-256 of the tarball as 64 lowercase hex characters.  |
| `package-tarball-sha512` | string | SHA-512 of the tarball as 128 lowercase hex characters. |

Outputs are release handles for downstream workflows and human operators. They are not substitutes
for signed provenance.

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

If a future profile needs a canonical Package URL, it must define a separate field such as
`package-purl` or `externalParameters.package.package_purl`; it must not overload `package-url`.

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

- The profile accepts a `registry-url` input.
- The profile guarantees only npmjs publish semantics.
- A non-npmjs registry may be accepted as a transport target, but unsupported registry behavior is
  at the caller's risk and must be recorded in provenance as
  `publish.custom_registry_support: "unsupported-but-not-blocked"`.
- The profile must fail if the registry requires authentication mechanisms that violate the
  no-publish-secrets policy.
- The profile must fail if `npm publish --provenance-file` or the equivalent external provenance
  submission path is unavailable for the selected registry.
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
  - If the metadata check requires `NPM_TOKEN`, `NODE_AUTH_TOKEN`, OTP, publish credentials, private
    dependency credentials, unsigned provenance, npm automatic provenance fallback, or any other
    weakening of the production contract, the workflow must fail before registry mutation.
- For npmjs, post-publish registry metadata checks are required by the provenance and publish spec.
  For custom registries, registry linkage verification is registry-specific and must not be reported
  as Windlass-guaranteed unless a later ADR defines that registry class.

## Private dependency credentials

The initial profile does not support private dependency credentials. A package that requires private
registry authentication or dependency-fetch secrets must use a different workflow or a future
profile.

The initial profile also rejects packages whose selected source manifest has `private: true`. It
does not provide a pack-only, provenance-only, or no-publish mode for private npm packages.

## Rejected caller inputs

The profile must reject or ignore any attempt to supply:

- Arbitrary build, install, or pack commands.
- Package manager override inputs.
- Runtime environment overrides.
- npm token or OTP secrets.
- Inherited broad secrets.
- Multi-package selection inputs.
- Branch or pull-request based release triggers.

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
- The selected registry cannot complete tokenless trusted publishing with the supplied external
  provenance bundle.
- The selected package identity does not already exist on npmjs when publishing to
  `https://registry.npmjs.org/`.

## TDD and fixtures

- Positive fixture: valid tag push with root package and workspace package.
- Rejected fixtures: wrong trigger, mismatched tag/version, missing `package.json`, arbitrary
  command input, npm token secret, private package, private dependency requirement, `publishConfig`
  conflict, unsupported `publishConfig.directory`, disabled provenance metadata, producer-side
  missing caller OIDC permission, producer-side npm trusted publisher caller identity mismatch, and
  unsupported registry behavior.
- A YAML review checklist that a human can apply to the workflow file.
