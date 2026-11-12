# Builder Identity And BuildType URI Contract

This document defines the machine-verifiable identity of `slsa-builder` workflows and the canonical
`buildType` URI scheme used by producer profiles.

- Source ADRs: [0028](../decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md),
  [0031](../decisions/0031-use-sigstore-signed-in-toto-release-manifest.md),
  [0042](../decisions/0042-use-acquired-domains-for-buildtype-uris.md),
  [0049](../decisions/0049-separate-artifact-production-from-github-release-asset-publication.md),
  [0053](../decisions/0053-use-three-job-release-manifest-signing-boundary.md),
  [0068](../decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md),
  [0080](../decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md),
  [0103](../decisions/0103-define-external-workflow-parameters-as-common-invocation-identity.md),
  [0104](../decisions/0104-remove-redundant-external-parameter-builder-record.md),
  [0105](../decisions/0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md)
- Related specs: [Core profile contract](core-profile-contract.md),
  [SLSA provenance v1](slsa-provenance-v1.md), [Release manifest](release-manifest.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md)

## Scope and non-goals

**In scope:**

- Production workflow reference policy.
- Canonical `runDetails.builder.id` format and its separation from caller invocation identity.
- Authenticated signer binding and profile-specific builder authorization.
- `buildType` URI format and namespace.
- Release manifest linkage to `builder.id` and `buildType`.
- Verifier trust policy expectations.

**Out of scope:**

- Ecosystem-specific subject naming (profile specs).
- Invocation field and source-descriptor schemas, and provenance predicate fields other than
  identity (the [common provenance spec](slsa-provenance-v1.md)).
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

The canonical signed builder identity is `runDetails.builder.id`. It must be the full GitHub URI of
the reusable workflow file, including the full commit SHA, in exactly this form:

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/<profile-workflow>.yml@<40 lowercase hex sha>
```

For the JS/TS npm producer, the authorized workflow path is exactly
`.github/workflows/js-ts-npm-package-slsa3.yml`, and the expected `buildType` is exactly
`https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`.

### Example

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5
```

### Canonical validation

Validation must compare the complete `runDetails.builder.id` byte-for-byte with the URI constructed
from the authorized profile workflow path and full SHA. It must reject:

- Repository or path aliases, and filename-extension aliases outside the selected profile path.
- Percent-encoded `/` or `@`, extra `@` characters, query or fragment components, and whitespace.
- Branch or tag suffixes, short SHAs, uppercase SHAs, and every other malformed or alternate
  spelling.

URL parsing or normalization must not turn an alternate spelling into acceptance. A well-formed
`builder.id` is not thereby trusted. The exact profile path, SHA, complete builder URI, and
`buildType` must also be authorized as described in [Verifier trust policy](#verifier-trust-policy).

### Rejected examples

- `https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@main`
- `https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@v1`
- `https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@e40a91e`
- `https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yaml@e40a91e0a0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5`
- `https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@E40A91E0A0C1D2E3F4A5B6C7D8E9F0A1B2C3D4E5`

### Caller invocation identity boundary

Under ADRs 0103 and 0104, `buildDefinition.externalParameters.workflow` is exactly the caller
invocation identity group `{ref, repository, path}` defined by the
[common provenance spec](slsa-provenance-v1.md). Its `path` is the repository-relative path of the
caller workflow, never the reusable builder workflow path. The group must not contain the builder
`sha` or `builder_id`, reuse `path` for the builder, or recreate that builder record under another
external-parameter name. Canonical builder identity remains only at `runDetails.builder.id`.

Caller invocation values and builder identity answer different questions and must be verified
independently against their respective platform claims. This separation does not change the distinct
release-manifest mapping in [Release manifest linkage](#release-manifest-linkage).

### Runtime identity acquisition and pin validation

This section specifies the runtime claim sources required by ADRs
[0028](../decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md),
[0068](../decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md),
[0103](../decisions/0103-define-external-workflow-parameters-as-common-invocation-identity.md),
[0104](../decisions/0104-remove-redundant-external-parameter-builder-record.md), and
[0105](../decisions/0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md).

The called reusable workflow must obtain its own full 40-hex commit SHA from the GitHub-issued OIDC
token's `job_workflow_sha` claim. GitHub resolves this value and signs it into the token, so it is
authoritative and caller-unforgeable even when the caller used a tag. If `job_workflow_sha` is
missing, malformed, or not exactly 40 lowercase hexadecimal characters, the workflow must terminate
before constructing or signing provenance. Production `runDetails.builder.id` derives from the
authenticated `job_workflow_ref` and `job_workflow_sha` claims, never from the removed
external-parameter builder fields.

The job that reads the OIDC token must have `id-token: write`. Without that permission, token
acquisition fails, and the job must terminate before constructing or signing provenance rather than
falling back to any workflow context or caller input.

The called workflow must not use caller `github.workflow_sha` or the caller `workflow_sha` claim for
builder identity. In a called reusable workflow, those values contain the **caller's** workflow SHA,
not the callee's SHA. Selecting either as the builder SHA is a fatal identity error, and the
workflow must terminate before constructing or signing provenance.

Production pin-form enforcement is separate from resolved identity acquisition. The called workflow
must validate that the OIDC token's `job_workflow_ref` claim ends with `@` followed by exactly 40
lowercase hexadecimal characters (`@[0-9a-f]{40}$`) and that this suffix equals `job_workflow_sha`.
The
[verifier policy's canonical SHA-casing rule](verification-policy-and-fixtures.md#verifier-policy-and-manifest-expectation-schemas)
governs this validation. A missing claim, a branch, tag, short SHA, malformed suffix, or mismatch
must reject the production invocation before provenance is constructed or signed.

### Authenticated signer binding

For the current SHA-pinned production profile, one verified signing certificate must satisfy both
comparisons:

- Build Signer URI equals `runDetails.builder.id` byte-for-byte.
- Build Signer Digest equals the full 40-character SHA suffix of `runDetails.builder.id`.

The binding has these failure and preservation rules:

- Missing or malformed signer evidence fails verification.
- Any Build Signer URI or Build Signer Digest mismatch fails verification.
- Evidence from different certificates cannot be combined.
- The predicate's path or SHA suffix cannot substitute for certificate evidence.
- Existing URI SAN, issuer, source, run-invocation, and runner-environment checks remain in force.
- Transparency and signing-time checks also remain in force.

The same token carries the source repository's numeric `repository_id` and `repository_owner_id`
claims. ADR 0068 verification uses these platform-signed values as the authoritative immutable
source identifiers; repository and owner names are display-only. A missing, malformed, or mismatched
numeric identifier is an identity verification failure.

The invocation identity bindings are:

- Platform `ref` and certificate Source Repository Ref identify the invocation ref.
- That ref must match `externalParameters.workflow.ref` and the `invocation-source` URI ref.
- Platform `sha` and certificate Source Repository Digest identify the invocation head commit.
- That commit must match the `invocation-source` descriptor's `digest.gitCommit`.
- Source Repository URI must match `externalParameters.workflow.repository`.
- It must also match the `invocation-source` URI's repository component.
- `built-source` separately records the built release tag and resolved commit.
- Invocation claims never replace built-source identity when the roles differ.

The [common provenance spec](slsa-provenance-v1.md) defines these equality rules. Numeric repository
and owner identifiers remain context-independent.

> [!NOTE]  
> The 12026-08-02 live spike confirmed this behavior. Runs `30745570800` (SHA pin) and `30745572730`
> (tag pin) both returned the resolved callee SHA through `job_workflow_sha`; observed callee
> commits included `1f4ebdc3…` and `b30c14d8…`. The spike also confirmed that `github.workflow_sha`
> exposed the caller SHA.

## `buildType` URI format

Every producer profile must define a canonical `buildType` URI. The URI is an identifier, not a
network endpoint.

### Identifier namespace

`buildtype.dev` is the identifier namespace for all Windlass `buildType` values.

### Canonical URI template

```text
https://buildtype.dev/windlass/slsa-builder/<profile-name>/v<major-version>
```

`<profile-name>` is exactly one non-empty URI-safe segment. It must match `[A-Za-z0-9._~-]+`; `/`,
percent-encoded sequences, and characters outside that set are invalid. This is the canonical
`profile-name` definition for `buildType` URIs.

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
schema, subject naming rules, digest semantics, and verification expectations for that profile. For
producer provenance, the parameter documentation must follow ADRs
[0103](../decisions/0103-define-external-workflow-parameters-as-common-invocation-identity.md) and
[0104](../decisions/0104-remove-redundant-external-parameter-builder-record.md) and link to the
[common provenance spec](slsa-provenance-v1.md): `externalParameters.workflow` is caller invocation
identity, not a duplicate builder record.

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

The authorized builder tuple comprises the exact profile path, full SHA, complete builder URI, and
`buildType`.

A verifier must accept a provenance only if all of the following hold:

1. `runDetails.builder.id` matches the authorized profile path and full SHA byte-for-byte.
2. Trusted policy or authenticated Windlass release metadata authorizes the exact builder tuple.
3. When both policy sources constrain that tuple, their existing intersection rules apply.
4. The `buildType` exactly matches the selected profile's canonical URI.
5. The same verified certificate passes both comparisons in
   [Authenticated signer binding](#authenticated-signer-binding).
6. The `job_workflow_ref` suffix and `job_workflow_sha` equal the authorized full SHA.
7. Caller `github.workflow_sha` and caller `workflow_sha` are not builder identity sources.
8. `externalParameters.workflow.path` is the caller path, never the reusable builder path.
9. The default publisher path does not present the removed GitHub Release asset `buildType`.

Acceptance also follows these rules:

- A well-formed `builder.id` is not necessarily trusted.
- Verifier expectations never derive from the candidate predicate or candidate-controlled values.
- Mutually consistent but unauthorized path, SHA, builder URI, or `buildType` values fail closed.

### npm profile-path migration guard

The common `ValidateBuilderID` contract has a wider grammar than the npm profile:

- It accepts multiple Windlass workflow filenames directly under `.github/workflows`.
- It permits either `.yml` or `.yaml`.
- That generic acceptance does not enforce the npm profile's exact path.

For npm, the required builder path is `.github/workflows/js-ts-npm-package-slsa3.yml`. Removing the
former external-parameter path check is equivalent only when all three replacement checks remain:

1. Profile-aware canonical `runDetails.builder.id` validation.
2. Same-certificate Build Signer URI and Build Signer Digest verification.
3. Exact policy or release-metadata authorization, including the expected npm `buildType`.

The migration guard requires all of the following:

- Any other Windlass workflow filename fails npm verification.
- Well-formed syntax and candidate-side consistency cannot change that result.
- No former builder path, SHA, or `builder_id` check disappears without replacement coverage.
- Profile selection, canonical identity, signer evidence, and authorization name the same builder.

## Failure behavior

A verifier must reject provenance when:

- The `builder.id` uses a repository or path alias or a non-selected filename extension.
- The `builder.id` uses an encoded separator, extra `@`, query, fragment, or whitespace.
- The `builder.id` uses a branch, tag, short SHA, uppercase SHA, or malformed SHA.
- The exact builder tuple is unauthorized after applying policy or release-metadata intersection.
- Build Signer URI or Build Signer Digest is missing, malformed, or mismatched.
- Signer comparisons do not use the same verified certificate.
- Production identity uses caller `github.workflow_sha` or caller `workflow_sha`.
- The authenticated `job_workflow_ref` or `job_workflow_sha` disagrees with canonical `builder.id`.
- `externalParameters.workflow` carries a builder `sha` or `builder_id`.
- `externalParameters.workflow.path` substitutes the reusable builder path for the caller path.
- The default publisher path presents a source-to-artifact `buildType` for release assets.

## TDD and fixtures

- Golden fixtures cover canonical `runDetails.builder.id` and `buildType` for each accepted profile.
- Rejected fixtures cover repository and path aliases and non-selected `.yml`/`.yaml` aliases.
- Rejected fixtures cover percent-encoded `/` or `@`, extra `@`, query, fragment, and whitespace.
- Rejected fixtures cover branch and tag refs and short, uppercase, or malformed SHAs.
- A rejected fixture covers the removed GitHub Release asset production `buildType`.
- A well-formed but unauthorized builder proves that syntax is not trusted-policy authorization.
- Candidate-side consistency cannot replace trusted policy or authenticated release metadata.
- Signer fixtures accept the exact same-certificate Build Signer URI/Digest pair.
- Signer fixtures reject missing or mismatched evidence even when the predicate path and SHA match.
- An npm migration fixture uses a different Windlass workflow filename.
- That fixture proves generic `ValidateBuilderID` acceptance cannot bypass the exact npm path.
- Invocation fixtures require the exact caller `{ref, repository, path}` group.
- They reject builder `sha`/`builder_id` and any builder path used as caller `workflow.path`.
- Runtime fixtures accept matching 40-hex `job_workflow_sha` and `job_workflow_ref` claims.
- Runtime fixtures reject missing OIDC permission and caller `github.workflow_sha` substitution.
- Runtime fixtures reject caller `workflow_sha` substitution.
- Runtime fixtures reject non-SHA `job_workflow_ref` suffixes.
- Runtime fixtures reject claim mismatches and missing or malformed numeric repository IDs.
- Invocation fixtures bind platform `ref` to `workflow.ref` and the `invocation-source` URI ref.
- Invocation fixtures bind platform `sha` to `invocation-source` `digest.gitCommit`.
- Invocation fixtures never substitute invocation claims for built-source identity.
- A fixture proves the release manifest maps each version to its authorized builder tuple.
