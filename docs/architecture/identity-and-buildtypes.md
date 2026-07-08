# Builder Identity And BuildType URI Contract

This document defines the machine-verifiable identity of `slsa-builder` workflows and the canonical
`buildType` URI scheme used by producer profiles.

- Source ADRs: [0028](../decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md),
  [0031](../decisions/0031-use-sigstore-signed-in-toto-release-manifest.md),
  [0042](../decisions/0042-use-acquired-domains-for-buildtype-uris.md),
  [0049](../decisions/0049-separate-artifact-production-from-github-release-asset-publication.md),
  [0053](../decisions/0053-use-three-job-release-manifest-signing-boundary.md)
- Related specs: [Core profile contract](core-profile-contract.md),
  [SLSA provenance v1](slsa-provenance-v1.md), [Release manifest](release-manifest.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md)

## Scope and non-goals

**In scope:**

- Production workflow reference policy.
- `builder.id` format.
- `buildType` URI format and namespace.
- Release manifest linkage to `builder.id` and `buildType`.
- Verifier trust policy expectations.

**Out of scope:**

- Ecosystem-specific subject naming (profile specs).
- Provenance predicate fields other than identity (provenance spec).
- Release manifest JSON schema (release manifest spec).
- Publisher verification policy (publisher and verification specs).

## Production workflow reference policy

A production caller must invoke a profile-owned reusable workflow by a **full commit SHA** of the
`slsa-builder` repository. Any other reference is not a valid production invocation for SLSA
verification purposes.

**Allowed references:**

- Full commit SHA, for example
  `uses: windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@e40a91e0a0c1...`.

**Rejected references:**

- Branch names, for example `@main` or `@docs/some-branch`.
- Moving tags, for example `@v1` or `@v1.2`.
- SemVer tags as executable references, for example `@v1.0.0`.
- Short SHAs.
- Dynamic refs computed at runtime.

## `builder.id` format

The `builder.id` emitted in SLSA provenance must be the full GitHub URI of the reusable workflow
file, including the full commit SHA.

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/<workflow-file>.yml@<full-sha>
```

### Example

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5
```

### Rejected examples

- `https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@main`
- `https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@v1`
- `https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@e40a91e`

## `buildType` URI format

Every producer profile must define a canonical `buildType` URI. The URI is an identifier, not a
network endpoint.

### Identifier namespace

`buildtype.dev` is the identifier namespace for all Windlass `buildType` values.

### Canonical URI template

```text
https://buildtype.dev/windlass/slsa-builder/<profile-name>/v<major-version>
```

### Example

```text
https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1
```

### Rejected examples

- `https://windlasstech.github.io/slsa-builder/js-ts-npm-package/v1` (superseded by ADR 0042).
- `https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@e40a91e...`
  (this is a `builder.id`, not a `buildType`).
- `https://buildtype.dev/windlass/slsa-builder/github-release-asset/v1` (the default production
  publisher path does not have a source-to-artifact `buildType` after ADR 0049).
- `https://slsa-builder.dev/predicates/release-manifest/v1` (this is the release manifest
  `predicateType` selected by ADR 0054, not a producer `buildType`).

## Documentation host

`slsa-builder.dev` is the documentation host for human-readable `buildType` and `builder.id`
documentation. The documentation page for a `buildType` URI should explain the `externalParameters`
schema, subject naming rules, digest semantics, and verification expectations for that profile.

The documentation host is complementary to the identifier namespace. Production builds, provenance
generation, and verifier policy evaluation must not dereference the `buildType` URI at build time or
verification time to decide whether provenance is valid. They compare the exact identifier value in
the provenance and the trusted policy.

The public identifier URL is still expected to resolve for human-readable specification discovery.
Requests to `https://buildtype.dev/windlass/slsa-builder/<profile-name>/v<major-version>` should use
a permanent redirect, preferably HTTP 308 or otherwise HTTP 301, to the corresponding
`https://slsa-builder.dev/buildtypes/<profile-name>/v<major-version>` documentation page. Temporary
redirects, aliases, or redirect targets are not equivalent `buildType` identifiers for verifier
matching.

Failure to resolve the documentation URL is not a profile runtime failure and must not change
emitted provenance. It is a release documentation readiness failure: release review must not mark a
new `buildType` URI ready for public use until the `buildtype.dev` redirect and `slsa-builder.dev`
documentation page exist or the release notes explicitly identify the missing documentation as a
known pre-release limitation.

## Release manifest linkage

The signed release manifest maps a human-readable Windlass release version to the exact producer
workflow SHAs, SHA-based producer `builder.id` values, producer `buildType` URIs, and publisher
workflow SHAs that verifiers should trust. See the [release manifest spec](release-manifest.md) for
the full schema.

For each producer profile, the release manifest must record:

- The profile name.
- The workflow file path.
- The full workflow commit SHA.
- The resulting `builder.id`.
- The canonical `buildType` URI.

For each publisher workflow, the release manifest must record:

- The publisher name.
- The workflow file path.
- The full workflow commit SHA.
- The publisher role.

Publisher workflow entries must not record `builder_id` or `build_type` in the default production
path because the publisher is not a source-to-artifact builder.

### Example mapping

```json
{
  "profile": "js-ts-npm-package",
  "workflow_path": ".github/workflows/js-ts-npm-package-slsa3.yml",
  "workflow_sha": "e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
  "builder_id": "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5",
  "build_type": "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1"
}
```

## GitHub Release asset publisher identity

The default production GitHub Release asset publisher path does **not** define a source-to-artifact
`buildType` for the uploaded release asset. The publisher is a verified distributor, not a builder.
Consumers must verify the producer profile `builder.id` and `buildType` carried in the producer
provenance bundle, not a publisher-specific `buildType`.

The signed release manifest may still record the publisher workflow path and SHA so users can pin
the publisher workflow release. That entry is release metadata for a verified-distributor workflow,
not a SLSA source-to-artifact builder identity.

If a future workflow needs a publisher-owned publication attestation or a source-to-artifact
`buildType` for release assets, that change requires a new ADR and a new spec.

## Verifier trust policy

A verifier must accept a provenance only if all of the following hold:

1. The `builder.id` is a full GitHub reusable workflow URI with a full commit SHA.
2. The workflow path and SHA match a mapping in the signed release manifest or in an explicit
   verifier policy.
3. The `buildType` is a canonical `https://buildtype.dev/windlass/slsa-builder/...` URI for the
   matching profile.
4. The `buildType` is not the removed GitHub Release asset production `buildType` in the default
   publisher path.
5. The workflow reference used by the caller equals the SHA in the `builder.id`.

## Failure behavior

A verifier must reject provenance when:

- The `builder.id` uses a branch, tag, short SHA, or dynamic reference.
- The `buildType` is not in the canonical namespace.
- The workflow SHA in the `builder.id` does not match the release manifest or verifier policy.
- The default production publisher path is presented with a source-to-artifact `buildType` for
  GitHub Release assets.

## TDD and fixtures

- Golden fixtures for `builder.id` and `buildType` for each accepted profile.
- Rejected fixtures for branch refs, moving tags, short SHAs, and removed GitHub Release asset
  production `buildType`.
- A fixture that proves the release manifest correctly maps a release version to the expected
  `builder.id` and `buildType`.
