# Common SLSA Provenance v1 Contract

This document defines the shared SLSA provenance v1 contract used by producer profiles.
Profile-specific rules such as subject naming and the remaining `externalParameters` surface are
layered on top of this common contract.

- Source ADRs: [0002](../decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md),
  [0003](../decisions/0003-use-thin-core-with-profile-owned-reusable-workflows.md),
  [0028](../decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md),
  [0029](../decisions/0029-use-windlass-generated-slsa-provenance-for-npm-publish.md),
  [0037](../decisions/0037-define-initial-verification-deliverables.md),
  [0042](../decisions/0042-use-acquired-domains-for-buildtype-uris.md),
  [0061](../decisions/0061-reject-duplicate-json-members-in-signed-slsa-statements.md),
  [0064](../decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md),
  [0069](../decisions/0069-require-rekor-transparency-and-govern-sigstore-trust-root.md),
  [0070](../decisions/0070-record-package-manager-distributions-and-runner-image-in-resolved-dependencies.md),
  [0071](../decisions/0071-activate-builder-version-and-builderdependencies-for-platform-components.md),
  [0077](../decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md),
  [0079](../decisions/0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md),
  [0080](../decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md),
  [0099](../decisions/0099-record-the-digest-verified-nodejs-runtime-in-resolveddependencies.md),
  and
  [0100](../decisions/0100-record-runner-image-identity-from-on-disk-metadata-with-the-control-plane-deployment-version-as-separate-evidence.md)
- Related specs: [Core profile contract](core-profile-contract.md),
  [Identity and build types](identity-and-buildtypes.md),
  [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md),
  [Release manifest](release-manifest.md)

## Scope and non-goals

**In scope:**

- Common in-toto Statement shape.
- Common SLSA provenance v1 predicate shape.
- Shared subject and digest rules.
- Shared fields such as `builder.id`, `buildType`, and `externalParameters`.
- The role of the signing adapter.
- Common verifier rejection criteria.

**Out of scope:**

- Profile-specific `externalParameters` schemas (profile specs).
- Profile-specific subject naming (profile specs).
- Publisher verification policy (publisher and verification specs).
- Release manifest JSON schema (release manifest spec).

## in-toto Statement shape

Consumers must ignore unrecognized fields in the common in-toto Statement envelope, as required by
the in-toto parsing rules. This does not relax the project-owned closed schemas and strict-matching
rules for `externalParameters`, `resolvedDependencies` entries, `builder.version`, or
`builderDependencies`. Duplicate-member rejection still applies before semantic validation.

Every SLSA provenance emitted by a producer profile must be wrapped in an in-toto Statement with the
following common shape. The example omits profile-owned external interface groups whose enumeration
belongs to the profile specification:

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "<profile-specific subject name>",
      "digest": {
        "sha256": "<lowercase hex>",
        "sha512": "<optional lowercase hex>"
      }
    }
  ],
  "predicateType": "https://slsa.dev/provenance/v1",
  "predicate": {
    "buildDefinition": {
      "buildType": "<canonical buildType URI>",
      "externalParameters": {
        "workflow": {
          "ref": "refs/heads/main",
          "repository": "https://github.com/example/project",
          "path": ".github/workflows/publish.yml"
        },
        "source": { "ref": "refs/tags/v1.2.3" }
      },
      "internalParameters": {
        "github": {
          "event_name": "workflow_dispatch",
          "repository_id": "123456789",
          "repository_owner_id": "987654321"
        }
      },
      "resolvedDependencies": [
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
        },
        {
          "name": "package-manifest",
          "uri": "git+https://github.com/example/project@0123456789abcdef0123456789abcdef01234567#package.json",
          "digest": {
            "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
          },
          "mediaType": "application/json",
          "annotations": { "workspace_root": null }
        },
        {
          "name": "lockfile",
          "uri": "git+https://github.com/example/project@0123456789abcdef0123456789abcdef01234567#package-lock.json",
          "digest": {
            "sha256": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
          },
          "mediaType": "application/json"
        },
        {
          "name": "node-runtime",
          "uri": "https://nodejs.org/download/release/v24.22.1/node-v24.22.1-linux-x64.tar.gz",
          "digest": {
            "sha256": "eb6728e77c576a35162c122c45ccadf224d89ff00abc8dae46cc2897c6002d7c"
          },
          "mediaType": "application/gzip",
          "annotations": {
            "digest_authority": "builder-pinned-record",
            "platform": "linux-x64"
          }
        },
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
      ]
    },
    "runDetails": {
      "builder": {
        "id": "<full GitHub reusable workflow URI with SHA>",
        "version": { "nodejs": "v24.0.0", "npm": "11.0.0" },
        "builderDependencies": [
          {
            "uri": "pkg:golang/github.com/sigstore/sigstore-go@v1.3.0",
            "digest": { "h1": "hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE=" },
            "annotations": { "role": "signing-adapter" }
          }
        ]
      },
      "metadata": {
        "invocationId": "https://github.com/<owner>/<repo>/actions/runs/<run-id>/attempts/<attempt-number>",
        "startedOn": "<RFC 3339 UTC timestamp at whole-second precision>",
        "finishedOn": "<RFC 3339 UTC timestamp at whole-second precision>",
        "windlass_build_script_disposition": "executed"
      }
    }
  }
}
```

The example depicts an ADR 0079 dispatch retry: the workflow and `invocation-source` record the
dispatch invocation on `refs/heads/main` and its head commit, while `source.ref`, `built-source`,
and the source-artifact descriptors (`package-manifest`, `lockfile`) record the caller-selected
release tag and its resolved commit. On a tag-push run with no selector, `source` is omitted and the
two source descriptors carry identical URI and digest values under their distinct names.

### `_type`

Must be exactly `https://in-toto.io/Statement/v1`. Any other value is rejected.

### `predicateType`

Must be exactly `https://slsa.dev/provenance/v1`. Any other value is rejected.

## Subject rules

- The initial production producer profiles must emit exactly one `subject` entry.
- That single entry is the primary artifact and is addressed as `subject[0]` by verifier policy.
- `subject[0].name` must be the canonical name defined by the profile.
- `subject[0].digest` must include at least `sha256`.
- Additional digest algorithms may be present if the profile requires them. The JS/TS npm profile
  requires tarball `sha512` alongside `sha256` for its Package URL subject.
- The digest value must be lowercase hexadecimal without a prefix.
- Checksum files, SBOMs, and provenance sidecars must not appear in `subject[0].digest`.
- Checksum files, SBOMs, provenance sidecars, and secondary artifacts must not be added as
  additional `subject` entries unless a later ADR and profile spec define their semantics. Verifiers
  for the initial production profiles must reject Statements with zero subjects or more than one
  subject.

## Digest encoding

- All digests in provenance use lowercase hex without a prefix.
- A profile may define an additional tool-boundary representation, such as `sha256:<hex>`, for
  handoff between jobs or external tools, but the provenance itself must use lowercase hex.
- `sha512` may be recorded in the digest map when the profile requires it.

## `builder.id`

See the [identity and build types spec](identity-and-buildtypes.md). Must be the full GitHub
reusable workflow URI with the full commit SHA.

## `buildType`

See the [identity and build types spec](identity-and-buildtypes.md). Must be a canonical
`https://buildtype.dev/windlass/slsa-builder/...` URI.

## `externalParameters`

`externalParameters` records the actual values passed through the build interface. Derived values,
resolved artifact metadata, environment observations, manifest content, and behavior fixed by the
buildType do not belong in this object. Verifier-relevant evidence remains available through the
common invocation record, resolved dependencies, `builder.version`, signed run metadata, input
artifacts, or a profile-specified recompute rule.

Every producer profile must emit a required, closed `workflow` object with exactly `ref`,
`repository`, and `path`:

```json
{
  "workflow": {
    "ref": "refs/heads/main",
    "repository": "https://github.com/example/project",
    "path": ".github/workflows/publish.yml"
  }
}
```

- `ref` is the invocation ref authenticated by the signing certificate's Source Repository Ref
  claim. It is not replaced by the built release ref on a dispatch retry.
- `repository` is the canonical HTTPS repository URI authenticated by the certificate's Source
  Repository URI claim.
- `path` is the repository-relative caller workflow YAML path authenticated by the certificate's
  Build Config URI claim. It is never the reusable builder workflow's own path.

All three members are required whenever producer provenance is emitted, including when invocation
and built-source identities coincide. A missing or additional member is rejected under the closed
schema. A Source Repository Ref or Source Repository URI disagreement is a signer identity failure;
a Build Config URI disagreement is rejected with `windlass.verify.error.build-config-uri-mismatch`.
The former builder `path`, `sha`, and `builder_id` members are forbidden and are not recreated under
another external-parameter name. Canonical builder identity remains only in `runDetails.builder.id`.

The remaining external-parameter surface is profile-specific and minimal. Each profile spec defines
its complete closed schema and strict-matching policy. The common contract provides no fallback
`inputs`, `runtime`, `package_manager`, or other group that can accept a field omitted by that
schema. Unexpected fields are rejected with `windlass.verify.error.external-parameters-unexpected`.

A profile that exposes a caller-selected source ref defines an optional `source` group containing
exactly one required member, `ref`. The group is emitted exactly when the caller supplied the
profile's non-empty source-selector input, and `ref` preserves that selector's exact interface
spelling. The group is omitted entirely when no selector was supplied; an empty `source: {}` object
is non-conforming. The profile spec defines selector grammar, suppliedness, and conflict rejection.

The effective built ref is `source.ref` when that member is present and `workflow.ref` otherwise.
Producers apply this rule before constructing the statement, and consumers independently recompute
it. Built-release policy binds to the effective built ref and to the `built-source` descriptor's URI
ref component and `digest.gitCommit`, not to a duplicate external-parameter record. Resolved source
revisions, repository duplicates, ref classifiers, invocation duplicates, and release identity are
recorded through descriptors or descriptor annotations rather than additional `source` members. In
particular, `source.repository`, `source.revision`, `source.ref_type`, `source.input_ref`,
`source.invocation_ref`, `source.invocation_revision`, `source.event_name`, and
`externalParameters.release.*` are forbidden under the revised closed contract. There is no
`source.invocation_*` or built-field fallback for certificate invocation comparisons.

For the initial JS/TS npm package profile, the complete normative profile-owned schema is defined in
[JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md). The common contract must not be
used to accept fields omitted by that profile schema.

## `internalParameters`

For GitHub Actions producers, `internalParameters` is a required closed object containing exactly
one member, `github`. The `github` object contains exactly three string fields: `event_name`,
`repository_id`, and `repository_owner_id`.

```json
{
  "internalParameters": {
    "github": {
      "event_name": "workflow_dispatch",
      "repository_id": "123456789",
      "repository_owner_id": "987654321"
    }
  }
}
```

The values are constructed from GitHub-issued OIDC claims before signing. All three are compared
fail-closed with the corresponding claims in the issued signing certificate before any registry or
release mutation. In particular, `event_name` must equal the certificate's Build Trigger claim; a
disagreement is rejected with `windlass.verify.error.build-trigger-mismatch`. The numeric repository
and owner identifiers must also agree with the certificate and trusted policy. Certificate
authentication of an event does not replace the profile's separate supported-event admission check.

Shape failures use `windlass.verify.error.unexpected-internal-parameters`:

- An absent, `null`, empty, non-object, or otherwise malformed `internalParameters` value.
- A missing or additional `github` member.
- A missing, additional, or non-string field in the required triple.

Actor fields including `actor_id` and `triggering_actor_id` are forbidden.

No `externalParameters` member may duplicate or alias the invocation event.

Invalid, because `debug` is not the closed `github` object:

```json
{ "internalParameters": { "debug": true } }
```

Invalid, because the required `repository_owner_id` string is absent:

```json
{
  "internalParameters": {
    "github": { "event_name": "push", "repository_id": "123456789" }
  }
}
```

Raw package metadata that is not trust input belongs only in `diagnostic_metadata.package_manifest`,
not in `internalParameters`. A producer that places it there is rejected with
`windlass.verify.error.unexpected-internal-parameters`.

## `resolvedDependencies`

Each profile must define its complete, unordered, name-keyed `resolvedDependencies` shape and strict
matching policy. The common contract defines no fallback schema. Consumers select descriptors by
`name`, never by array position. Duplicate names are rejected before any name-keyed lookup, and an
unknown or non-enumerated name is rejected with
`windlass.verify.error.resolved-dependencies-unexpected-entry`.

Producer serialization order is deterministic and fixed before signing. Semantic verification
accepts any permutation of a valid set, but reordering an already signed payload changes its bytes
and fails exact-payload comparison. For the initial JS/TS npm profile, the closed set contains
`invocation-source`, `built-source`, `package-manifest`, `lockfile`, `node-runtime`, and
`runner-image`, exactly six descriptors for npm selections. pnpm and Yarn add exactly one
`package-manager-distribution`, for seven descriptors. The npm profile's deterministic emission
order, including the index-0 placement of `invocation-source`, is an npm emission compatibility rule
owned by
[JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md#jsts-npm-resolveddependencies-schema);
it does not change name-keyed consumer semantics.

The npm profile does not enumerate every installed package as a separate `ResourceDescriptor`. The
selected lockfile is the verifier-relevant dependency graph input, and package versions are
represented by its bytes and digest. Non-selected supported lockfiles remain stale diagnostics; they
do not become selected dependencies or acquire descriptor media types. Exact profile-owned
descriptor rules are defined by
[JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md#jsts-npm-resolveddependencies-schema),
subject to the common source, media-type, and runner-image rules below.

### Git source descriptors

The role-named Git source descriptors have these closed shapes:

```json
{
  "name": "invocation-source",
  "uri": "git+https://github.com/example/project@refs/heads/main",
  "digest": { "gitCommit": "89abcdef0123456789abcdef0123456789abcdef" }
}
```

```json
{
  "name": "built-source",
  "uri": "git+https://github.com/example/project@refs/tags/v1.2.3",
  "digest": { "gitCommit": "0123456789abcdef0123456789abcdef01234567" },
  "annotations": { "ref_type": "tag" }
}
```

- `invocation-source` identifies the caller repository content supplying the invocation workflow.
  Its URI is exactly `git+<workflow.repository>@<workflow.ref>` with no path fragment, and its
  `digest.gitCommit` equals the certificate's Source Repository Digest.
- The descriptor has exactly `name`, `uri`, and `digest`; annotations are forbidden. A missing,
  duplicate, malformed, or mismatched descriptor is rejected with
  `windlass.verify.error.resolved-dependencies-invocation-source`.
- `built-source` identifies the content that was checked out, built, and attested. Its URI equals
  `git+<workflow.repository>@<effective built ref>`, and its `digest.gitCommit` is that ref's
  verified resolved commit. Its required `annotations` object contains exactly `ref_type: "tag"`,
  which must agree with the URI's `refs/tags/` prefix. A missing, duplicate, malformed,
  annotation-inconsistent, or identity-mismatched descriptor is rejected with
  `windlass.verify.error.resolved-dependencies-built-source`.
- Each `digest` object contains exactly `gitCommit`, a full 40-character lowercase hexadecimal Git
  commit ID. `mediaType` is forbidden on both descriptors because a Git commit or tree identity is
  not one serialized artifact representation. `content` and `downloadLocation` are also forbidden.
- Both descriptors are required whenever a profile enumerates these roles, including when the
  identities coincide. With no explicit `source.ref`, the effective built ref equals `workflow.ref`,
  so the descriptors carry identical URI and digest values under their distinct names. Neither role
  may be omitted or substituted for the other.
- Invocation certificate claims bind only to `workflow.ref`, `workflow.repository`, and
  `invocation-source`. Built-release ref and commit expectations bind to the recomputed effective
  built ref and `built-source`. The commit embedded in source-artifact descriptor URIs such as
  `package-manifest` and `lockfile` must equal `built-source.digest.gitCommit`.

### `package-manifest` and offline evidence

A profile that enumerates `package-manifest` records the original selected source manifest with
exactly `name`, `uri`, `digest`, `mediaType`, and `annotations`:

- `uri` is `git+<workflow.repository>@<built revision>#<repository-root-relative manifest path>`.
  The built revision equals `built-source.digest.gitCommit`, and the path uses the profile's
  percent-encoding rule.
- `digest` contains exactly `sha256` over the original selected source-manifest bytes, not a
  build-mutated or packed manifest.
- `mediaType` is exactly `application/json`.
- `annotations` contains exactly `workspace_root`, which is `null` for a standalone package or the
  repository-root-relative workspace root for a workspace package.
- `content` and `downloadLocation` are forbidden.

A missing, duplicate, malformed, wrongly typed, or identity-mismatched descriptor is rejected with
`windlass.verify.error.resolved-dependencies-package-manifest`. Manifest-dependent verification uses
the original source bytes supplied through the profile's explicit offline evidence contract, bound
to this descriptor's digest and source identity. Missing, mismatched, or malformed evidence is
rejected with `windlass.verify.error.package-manifest-evidence`. There is no network fallback and no
silent skipping of manifest-dependent checks. In particular, package identity, publish
configuration, and normalized manifest repository checks use those authenticated bytes. The
normalized manifest repository must equal `workflow.repository`, the certificate's Source Repository
URI, and both Git source descriptors' URI repository components.

### Deterministic media types

Concrete non-runner artifact descriptors carry the following closed `mediaType` matrix. The value
describes the actual digest-covered representation, not merely a filename suffix, manager name, or
remote response header.

| Descriptor                     | Actual digest-covered representation                                    | `mediaType`        | Closed rule                                                                                           |
| ------------------------------ | ----------------------------------------------------------------------- | ------------------ | ----------------------------------------------------------------------------------------------------- |
| `package-manifest`             | Selected `package.json` source bytes                                    | `application/json` | Required; retains ADR 0109's value and no-content rule                                                |
| `lockfile`                     | Selected `package-lock.json` bytes                                      | `application/json` | Required for the selected npm lockfile only                                                           |
| `lockfile`                     | Selected `pnpm-lock.yaml` bytes                                         | `application/yaml` | Required for the selected pnpm lockfile only                                                          |
| `lockfile`                     | Selected supported Yarn Berry v4/v5 `yarn.lock` bytes                   | `application/yaml` | Required for the selected Berry lockfile only                                                         |
| `node-runtime`                 | Exact compressed Node.js `.tar.gz` bytes covered by the existing digest | `application/gzip` | Required; never describes the extracted runtime tree                                                  |
| `package-manager-distribution` | Exact npm registry `.tgz` bytes                                         | `application/gzip` | Required for every represented registry tarball, including any represented pnpm dual-artifact tarball |
| `package-manager-distribution` | Exact current single `yarn.js` bytes                                    | `text/javascript`  | Required only when this is the actual acquired and digest-covered representation                      |
| `invocation-source`            | Git commit or tree identity, not one serialized artifact                | none               | `mediaType` is forbidden                                                                              |
| `built-source`                 | Git commit or tree identity, not one serialized artifact                | none               | `mediaType` is forbidden                                                                              |

Every represented registry tarball, including a pnpm dual-artifact tarball, uses `application/gzip`.
`text/javascript` applies only when the descriptor's digest covers the single `yarn.js` bytes rather
than a containing registry tarball. `application/gzip` never describes an extracted runtime or
distribution tree. There is no generic or producer-selected fallback such as
`application/octet-stream` or `text/plain`; an unenumerated representation is rejected until the
closed matrix is deliberately extended.

A missing, incorrect, or forbidden `mediaType` is rejected under the owning descriptor's validation
rule. `mediaType` is descriptive only: it does not authenticate bytes, authorize a parser, replace
digest or source verification, or permit execution. Producers and consumers must not fetch or trust
a remote Content-Type value to select, repair, or override it. No `resolvedDependencies` descriptor
carries `downloadLocation`.

### `runner-image`

Exactly one `runner-image` descriptor remains mandatory in the npm closed set. Its top-level members
are exactly `name`, `uri`, and `annotations`. The required annotation set is exactly
`deployment_version`, `operating_system`, `image_version`, and `runner_label`:

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

- `uri` is the verbatim, non-empty, platform-supplied Included Software URI from the local Runner
  Image metadata. It is not reconstructed, normalized, replaced, or rewritten.
- `image_version` is the exact full version string from the same on-disk metadata's `Version:`
  detail, including every component. The URI and full `image_version` together are the sole reported
  identity pair. They do not assert immutable image bytes, report integrity, unique tag-to-version
  mapping, or public report availability.
- Producer and consumer conformance validate the supported URI/version projection offline. The full
  version's first two numeric components must agree with the release-tag component in the captured
  URI. Neither captured value is rewritten to force a match. No DNS, HTTP, tag, release, or report
  lookup is required, and HTTP 404 neither rejects valid local evidence nor selects descriptor
  omission, substitution, or a warning.
- `operating_system` is the exact decoded `Operating System` `detail` string from the same bounded
  local metadata snapshot, captured before caller-controlled install, build, or pack scripts. The
  snapshot must contain exactly one well-formed Operating System group with exactly one non-empty,
  non-whitespace string `detail`. The producer does not trim, normalize, convert line endings,
  compact, parse components, or map the value to another label. `image_os` is forbidden. Runtime
  `$ImageOS` is optional diagnostic-only context and is never required, compared, moved into another
  signed field, or used as a fallback.
- `deployment_version` is the control-plane `ImageVersion` value when supplied and `null` otherwise.
  It is non-authoritative, non-identifying rollout evidence, may differ from `image_version`, and is
  never an identity or evidence fallback. `runner_label` is the independently observed Actions
  runner label and is not derived from the other annotations.
- `digest`, `content`, `mediaType`, `downloadLocation`, unknown top-level members, `image_os`,
  `node_version`, and arbitrary additional annotations are forbidden. No raw metadata, metadata
  digest, or additional metadata-evidence descriptor is emitted.

Baseline consumers validate the signed closed shape, annotation schema and values, and offline
URI/version projection without requiring the raw local snapshot or independently rechecking OS
extraction. Missing or malformed local evidence fails closed at the producer. A missing, duplicate,
malformed, forbidden-member-bearing, projection-mismatched, or snapshot-inconsistent runner
descriptor is rejected with `windlass.verify.error.resolved-dependencies-runner-image`.

## `runDetails`

### `builder`

- `id`: the canonical full GitHub reusable workflow URI with a full commit SHA.
- The SHA suffix is exactly 40 lowercase hexadecimal characters (ADR 0028 and the identity and build
  types spec).
- No external-parameter builder record duplicates or replaces this identity.
- Build Signer URI and Digest come from the same verified certificate.
- The URI equals `runDetails.builder.id` byte-for-byte; the Digest equals its full SHA suffix.
- Trusted policy or authenticated release metadata authorizes the builder.
- Authorization covers the exact profile path, full SHA, builder URI, and buildType.
- Missing, malformed, unauthorized, or signer-inconsistent identity evidence fails closed.
- `version`: a closed object with required lowercase `nodejs` and `npm` keys plus conditional
  lowercase `corepack`.
- Values are exact observed version strings, not ranges, tags, or aliases.
- `nodejs` is the single home for the observed Node.js version.
- `npm` records only the build-stage bundled npm from the pinned toolchain pair, not the separately
  provisioned publish-stage npm.
- `corepack` is present only when Corepack supplied the selected package manager.
- `runner-image` and every other key are forbidden.

Version failures use `windlass.verify.error.builder-version-mismatch`:

- Missing or extra keys.
- Incorrect conditional `corepack` presence.
- An observed-version mismatch.
- `builderDependencies`: exactly one signing-adapter descriptor identifying the Go-native signer
  selected by ADR 0077. Its `uri` is `pkg:golang/github.com/sigstore/sigstore-go@v1.3.0`, its only
  digest member is the governed module checksum `h1: hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE=`,
  and its only annotation is `role: "signing-adapter"`. This is the default closed descriptor for
  every producer profile; profile admission must not substitute another signer. Missing, extra,
  malformed, wrong-role, or dependency-inconsistent descriptors, including build-job action
  descriptors, are rejected with
  `windlass.verify.error.builder-dependencies-signing-adapter-mismatch`.

The former `externalParameters.runtime.node_version` and `runtime.npm_version` members and the
runner-image `node_version` annotation are forbidden rather than duplicated here. For pnpm and Yarn,
the selected manager's observed version remains in the `package-manager-distribution` descriptor's
annotations. The runner image version remains only in the named `runner-image` descriptor. A
duplicated or forbidden builder value is rejected with
`windlass.verify.error.builder-version-mismatch`.

Valid direct-npm version shape:

```json
{ "version": { "nodejs": "v24.0.0", "npm": "11.0.0" } }
```

Valid Corepack version shape:

```json
{ "version": { "nodejs": "v24.0.0", "npm": "11.0.0", "corepack": "0.29.4" } }
```

Invalid, because the required bundled `npm` observation is absent:

```json
{ "version": { "nodejs": "v24.0.0" } }
```

Invalid, because `nodejs` is not an exact observed version string:

```json
{ "version": { "nodejs": "v24", "npm": "11.0.0" } }
```

Valid signing-adapter dependency:

```json
{
  "builderDependencies": [
    {
      "uri": "pkg:golang/github.com/sigstore/sigstore-go@v1.3.0",
      "digest": { "h1": "hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE=" },
      "annotations": { "role": "signing-adapter" }
    }
  ]
}
```

Invalid, because the checksum does not identify the module version in the URI:

```json
{
  "builderDependencies": [
    {
      "uri": "pkg:golang/github.com/sigstore/sigstore-go@v1.3.0",
      "digest": { "h1": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" },
      "annotations": { "role": "signing-adapter" }
    }
  ]
}
```

### `metadata`

Profiles may require additional vendor-prefixed run-outcome fields in this object. Their closed
value grammars and verification rules belong to the profile contract. The npm profile requires
`windlass_build_script_disposition` with exactly one of `executed` or `skipped-absent`. The field
records what the builder did about the declared build script, including a skip reason, rather than a
success/failure result. A missing, unsupported, or authenticated-manifest-inconsistent value is
rejected with `windlass.verify.error.build-script-disposition-mismatch`. The common cross-profile
schema does not require this field for profiles that do not enumerate it.

- `invocationId`: exactly
  `https://github.com/<owner>/<repo>/actions/runs/<run-id>/attempts/<attempt-number>`, where the
  owner and repository identify the source repository and the final components are positive base-10
  integers. It must be byte-for-byte equal to authenticated Fulcio OID `1.3.6.1.4.1.57264.1.21`. A
  malformed, absent, or unequal value is rejected with
  `windlass.verify.error.run-invocation-uri-invalid`.
- `startedOn`: build start timestamp.
- `finishedOn`: build completion timestamp.

Both timestamps must use the RFC 3339 profile of ISO 8601, be normalized to UTC with the literal `Z`
suffix, and use exactly whole-second precision in the form `YYYY-MM-DDTHH:MM:SSZ`. The producer-side
verification gate must reject provenance before publication when either timestamp has a numeric UTC
offset, fractional seconds, no timezone, or any other representation.

Whole-second precision is required because SLSA v1 provenance uses these values as event times, and
seconds preserve interoperable ordering without claiming subsecond accuracy that the build platform
does not guarantee. When `finishedOn` precedes `startedOn` by one to five whole seconds, consumers
must emit `windlass.verify.warning.timestamp-clock-skew`, treat the observed duration as zero, and
otherwise continue verification. This warning is emitted only for that accepted negative interval;
it has phase `verification`, exit code `0`, and `mutation_possible: false`. Consumer verification
must reject provenance with a negative interval greater than five seconds as a timestamp-ordering
error.

These are technical protocol fields, so they must retain standard Gregorian RFC 3339 dates rather
than the project's Holocene Era convention. A Holocene-formatted value, including a five-digit year
such as `12026`, must fail timestamp-format validation.

Valid example: `startedOn: 2026-08-02T12:00:00Z` and `finishedOn: 2026-08-02T12:00:03Z`.

Invalid examples include `2026-08-02T12:00:00.123Z` (fractional precision),
`2026-08-02T12:00:00+00:00` (not the canonical `Z` form), and `12026-08-02T12:00:00Z` (Holocene
year). A pair with `startedOn: 2026-08-02T12:00:10Z` and `finishedOn: 2026-08-02T12:00:04Z` is also
invalid because its negative interval exceeds the five-second clock-skew tolerance.

Valid invocation ID:

```json
{
  "invocationId": "https://github.com/example/acme-widget/actions/runs/123456789/attempts/2"
}
```

Invalid, because the attempt is not represented as the run-attempt URI:

```json
{ "invocationId": "123456789.2" }
```

## Signing adapter role

The Go-native `sigstore-go` v1.3.0 adapter selected by ADR 0077 is the signing adapter for all
Windlass provenance. Windlass assembles and validates the complete in-toto Statement under the
profile's closed contract, then the adapter signs those exact bytes as `sign.DSSEData` with payload
type `application/vnd.in-toto+json`. It must not reconstruct, normalize, or reserialize the
Statement. The npm profile adopts this contract first; future profiles apply it as their Statement
and verification contracts are admitted.

The adapter must not define the Statement's meaning. The producer-side verification gate must
extract the signed bundle payload before publication and prove that the emitted Statement has:

- `_type: https://in-toto.io/Statement/v1`.
- The Windlass-verified subject entries.
- `predicateType: https://slsa.dev/provenance/v1`.
- The Windlass-generated SLSA provenance predicate.
- The expected `builder.id`, `buildType`, and `externalParameters` values.

Every other profile must use the same exact-byte input boundary, preserve its verifier-visible
Statement contract, and identify the governed signer dependency in `builderDependencies`.

### Go-native signing adapter contract

The signing adapter boundary is the Go-native `sigstore-go` v1.3.0 DSSE interface. Windlass
supplies:

- the exact preassembled Statement bytes containing the profile-defined single subject and complete
  digest map; and
- payload type `application/vnd.in-toto+json`.

For npm, that subject is the one npm Package URL subject with its SHA-512 and SHA-256 digest map.

The adapter signs with a GitHub Actions OIDC-backed ephemeral key, obtains the Fulcio certificate
and embedded SCT, and emits a Sigstore bundle with the Rekor evidence required by ADR 0069. Windlass
must extract the DSSE payload and compare it byte-for-byte with the preassembled Statement before
publication. GitHub artifact attestation storage is disabled for the npm path while its custom
`buildType` restriction applies.

The adapter contract has exactly one file output that can become provenance evidence for registry
publication, cross-job handoff, release sidecar redistribution, and consumer verification: the
emitted Sigstore bundle bytes. GitHub artifact attestation storage metadata, URLs, IDs, or lookup
results are native diagnostic locators only. They must not replace the bundle file when a workflow,
registry, release asset, handoff, or verifier requires provenance bytes.

### Signer identity verification inputs

Verifier policy is expressed in terms of semantic GitHub Actions identity fields rather than a
single tool-specific claim spelling. Implementations may read those fields from Sigstore certificate
extensions, GitHub artifact attestation metadata, DSSE bundle verification output, or another
verified representation of the same signing certificate, but they must prove the semantic fields
below before accepting a bundle.

The required semantic fields are the GitHub Actions OIDC issuer; signer workflow repository, path,
and immutable ref or SHA; caller workflow path; source repository; invocation ref and revision;
built source ref and revision; Build Trigger; numeric repository and owner identifiers; and
predicate type. Reusable producer workflows require a full signer commit SHA, while the release
manifest signer uses its protected release tag ref. The signer repository and path identify the
trusted Windlass workflow rather than the caller workflow.

For reusable producers, the signer workflow repository and path identify the Windlass workflow.

- The `workflow` group and `invocation-source` record the authenticated caller invocation.
- The effective built ref and `built-source` record the release content.
- Certificate source claims authenticate only the invocation side.
- On an ADR 0079 dispatch retry, those claims are not compared with the built ref or commit.
- Removed `source.invocation_*` and `source.revision` members are not fallbacks.
- The optional `source.ref` selector is not invocation evidence.
- Release manifest signing uses `windlasstech/slsa-builder` as both signer and source repository.
- Its signer workflow ref is the protected release tag recorded in the manifest.

Common GitHub/Sigstore verification outputs expose these concepts with claim names such as `issuer`,
`repository`, `workflow_ref`, `workflow_sha`, `job_workflow_ref`, `job_workflow_sha`, `ref`, and
source repository/ref fields. Those names are reference mappings, not alternate policy. If a tool
omits one of the required semantic fields, the implementation must recover it from another verified
certificate or bundle field; otherwise verification fails closed. The implementation must not infer
signer identity from artifact names, workflow outputs, logs, release notes, or unsigned metadata.

All semantic signer identity fields used for one verification decision must be derived from the same
verified bundle and the same signing certificate or from verification output that is
cryptographically bound to that certificate. An implementation must not combine a signer workflow
repository from one bundle, a source repository from another bundle, or unsigned GitHub API metadata
with signed certificate claims to satisfy one policy.

The verified claim sources and their required statement-side comparisons are:

| Authenticated input (verified) | Required signed comparison                                               |
| ------------------------------ | ------------------------------------------------------------------------ |
| Source Repository Ref          | `workflow.ref` and the `invocation-source` URI ref component             |
| Source Repository Digest       | `invocation-source.digest.gitCommit`                                     |
| Source Repository URI          | `workflow.repository` and both source descriptors' repository components |
| Build Trigger                  | `internalParameters.github.event_name`                                   |
| Build Config URI               | `workflow.path`, never the reusable builder path                         |
| Numeric repository/owner IDs   | `internalParameters.github.repository_id` and `repository_owner_id`      |
| Build Signer URI               | Byte-for-byte equal to `runDetails.builder.id`                           |
| Build Signer Digest            | Full SHA suffix of `runDetails.builder.id`                               |
| Built-release policy           | `source.ref ?? workflow.ref`; `built-source` URI and digest              |
| Predicate type                 | Expected predicate URI                                                   |

- For reusable producers, Build Signer URI derives from `job_workflow_ref`.
- Build Signer Digest derives from `job_workflow_sha`.
- The `job_workflow_ref` suffix must be the authorized full SHA and agree with `job_workflow_sha`.
- Caller `workflow_ref` and `workflow_sha` never identify the reusable builder.
- `github.workflow_sha` never identifies it either.
- Other signer profiles use their specified verified claims.
- Caller claims cannot substitute for reusable producer signer evidence.
- The Build Trigger comparison occurs before any registry or release mutation.
- Numeric identifier comparisons also bind to trusted policy.

If two verified representations of one semantic field are present, they must identify the same value
after the profile-defined canonicalization. A conflict is a signer identity failure, not a reason to
choose one spelling by precedence. A candidate predicate value cannot repair a missing certificate
claim. In particular, the SHA suffix in `runDetails.builder.id` cannot substitute for Build Signer
Digest, and built-source fields cannot substitute for invocation certificate claims. This equality
rule does not compare the built source ref or revision against the certificate source claims on an
ADR 0079 dispatch retry. If a required verified field remains unavailable after the allowed sources
are checked, verification fails with a missing semantic identity field.

Before the npm production implementation is accepted, a controlled publish must prove that the
Go-signer bundle is accepted by `npm publish --provenance-file`, appears in registry read-back, and
passes pacote consumer verification, and that the same bytes can be preserved as the GitHub Release
provenance sidecar.

## Signed bundle file format

The initial production signed provenance artifact is the exact Sigstore bundle file emitted by the
Go-native signing adapter selected by ADR 0077. The bundle bytes are verifier inputs and must be
preserved byte-for-byte across job handoff, registry submission such as npm `--provenance-file`, and
GitHub Release sidecar redistribution.

The `.intoto.jsonl` filename suffix used by npm, release sidecars, and release manifest assets is a
distribution naming convention. It does not mean that Windlass may replace the signed bundle with a
raw in-toto Statement, a reserialized DSSE envelope, extracted predicate JSON, or any other
normalized representation.

Implementations must not alter signed bundle bytes after the signing adapter emits them. In
particular, they must not:

- extract the in-toto Statement and store only that Statement as the provenance file;
- reserialize, pretty-print, compact, wrap, unwrap, or reorder the bundle JSON;
- regenerate DSSE envelopes or Sigstore bundle metadata;
- re-sign the bundle in publisher or upload jobs; or
- treat GitHub artifact attestation storage metadata as a substitute for the bundle bytes when a
  bundle file is required.

Producer-side and consumer-side verifiers must parse the preserved bundle bytes, extract the signed
in-toto Statement payload, verify the bundle signature and signer identity, and compare the
extracted Statement fields against the expected subject inputs, `predicateType`, and predicate.
Verification must fail when the preserved bundle bytes cannot be parsed, the signature is invalid,
the extracted Statement does not match the expected contract, or a sidecar/downloaded bundle is not
byte-for-byte the same bundle that the producer emitted and the publisher verified.

Duplicate JSON member rejection applies to every JSON object that the verifier parses before making
a trust decision from the bundle. This includes the signed in-toto Statement payload, the SLSA
predicate, the DSSE envelope fields that identify or carry the payload, and Sigstore bundle JSON
fields used to verify the signature, certificate, transparency log inclusion, or payload binding. A
verifier may rely on a trusted Sigstore library for cryptographic validation, but any JSON value
from which Windlass extracts policy fields must be parsed with duplicate-member detection before
semantic policy checks. If the library exposes only normalized JSON for a security-relevant field
and cannot prove duplicate-member rejection for that parsed value, Windlass verification must fail
closed rather than accepting last-member-wins or first-member-wins behavior. After duplicate
detection succeeds, the common in-toto Statement envelope decoder ignores unrecognized fields per
the in-toto parsing rules. Project-owned closed schemas and profile strict-matching rules still
apply to `externalParameters`, `resolvedDependencies` entries, `builder.version`, and
`builderDependencies`.

## Common verifier rejection matrix

A verifier must reject provenance if any of the following are true:

| Condition                                                                                         | Diagnostic ID                                                              | Phase        | Exit code | Mutation possible |
| ------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------- | ------------ | --------- | ----------------- |
| `_type` is not `https://in-toto.io/Statement/v1`                                                  | `windlass.verify.error.statement-type-invalid`                             | verification | `1`       | `false`           |
| `predicateType` is not `https://slsa.dev/provenance/v1`                                           | `windlass.verify.error.predicate-type-invalid`                             | verification | `1`       | `false`           |
| Signature is missing or invalid                                                                   | `windlass.verify.error.signature-invalid`                                  | verification | `1`       | `false`           |
| Signer identity is not trusted                                                                    | `windlass.verify.error.signer-identity-untrusted`                          | verification | `1`       | `false`           |
| Any JSON object in the signed Statement payload contains duplicate member names after unescaping  | `windlass.verify.error.duplicate-json-member`                              | policy       | `1`       | `false`           |
| Any security-relevant bundle or DSSE JSON object contains duplicate member names after unescaping | `windlass.verify.error.duplicate-json-member`                              | policy       | `1`       | `false`           |
| `builder.id` uses a branch, tag, or short SHA                                                     | `windlass.verify.error.builder-id-not-immutable`                           | verification | `1`       | `false`           |
| `builder.id` is unauthorized or disagrees with Build Signer URI or Digest                         | `windlass.verify.error.signer-identity-untrusted`                          | verification | `1`       | `false`           |
| `buildType` is not in the canonical namespace                                                     | `windlass.verify.error.build-type-not-canonical`                           | verification | `1`       | `false`           |
| `externalParameters` is incomplete                                                                | `windlass.verify.error.external-parameters-incomplete`                     | verification | `1`       | `false`           |
| `externalParameters` contains unexpected fields                                                   | `windlass.verify.error.external-parameters-unexpected`                     | verification | `1`       | `false`           |
| Source Repository Ref or URI disagrees with `workflow.ref` or `workflow.repository`               | `windlass.verify.error.signer-identity-untrusted`                          | verification | `1`       | `false`           |
| `workflow.path` differs from Build Config URI or uses the reusable builder path                   | `windlass.verify.error.build-config-uri-mismatch`                          | verification | `1`       | `false`           |
| `internalParameters` violates the closed `github` three-string-field shape                        | `windlass.verify.error.unexpected-internal-parameters`                     | verification | `1`       | `false`           |
| `internalParameters.github.event_name` differs from Build Trigger                                 | `windlass.verify.error.build-trigger-mismatch`                             | verification | `1`       | `false`           |
| Numeric repository or owner identifiers disagree with `internalParameters.github`                 | `windlass.verify.error.signer-identity-untrusted`                          | verification | `1`       | `false`           |
| A dependency name is unknown, non-enumerated, or duplicated before lookup                         | `windlass.verify.error.resolved-dependencies-unexpected-entry`             | verification | `1`       | `false`           |
| An invocation-source descriptor is missing, duplicated, malformed, wrongly typed, or mismatched   | `windlass.verify.error.resolved-dependencies-invocation-source`            | verification | `1`       | `false`           |
| A built-source descriptor is missing, duplicated, malformed, or identity/annotation mismatched    | `windlass.verify.error.resolved-dependencies-built-source`                 | verification | `1`       | `false`           |
| A package-manifest descriptor is missing, duplicated, malformed, wrongly typed, or mismatched     | `windlass.verify.error.resolved-dependencies-package-manifest`             | verification | `1`       | `false`           |
| Required offline package-manifest evidence is missing, malformed, or mismatched                   | `windlass.verify.error.package-manifest-evidence`                          | verification | `1`       | `false`           |
| A node-runtime descriptor is missing, duplicated, malformed, wrongly typed, or mismatched         | `windlass.verify.error.resolved-dependencies-node-runtime`                 | verification | `1`       | `false`           |
| A package-manager-distribution descriptor is missing, duplicated, forbidden, or malformed         | `windlass.verify.error.resolved-dependencies-package-manager-distribution` | verification | `1`       | `false`           |
| A runner-image descriptor is missing, duplicated, malformed, digest-bearing, or mismatched        | `windlass.verify.error.resolved-dependencies-runner-image`                 | verification | `1`       | `false`           |
| A required build-script disposition is missing, unsupported, or manifest-inconsistent             | `windlass.verify.error.build-script-disposition-mismatch`                  | verification | `1`       | `false`           |
| `builder.version` has an invalid key, conditional shape, or observed version                      | `windlass.verify.error.builder-version-mismatch`                           | verification | `1`       | `false`           |
| `builderDependencies` lacks exactly one valid signing adapter                                     | `windlass.verify.error.builder-dependencies-signing-adapter-mismatch`      | verification | `1`       | `false`           |
| `metadata.invocationId` is malformed or differs from Fulcio OID `.21`                             | `windlass.verify.error.run-invocation-uri-invalid`                         | verification | `1`       | `false`           |
| `subject` contains zero or multiple entries                                                       | `windlass.verify.error.subject-cardinality-invalid`                        | verification | `1`       | `false`           |
| `subject[0].digest.sha256` is missing                                                             | `windlass.verify.error.subject-sha256-missing`                             | verification | `1`       | `false`           |
| `subject[0].name` does not match the profile rule                                                 | `windlass.verify.error.subject-name-mismatch`                              | verification | `1`       | `false`           |
| Digest encoding is not lowercase hex                                                              | `windlass.verify.error.digest-encoding-invalid`                            | verification | `1`       | `false`           |
| Sidecar, SBOM, or checksum is in `subject[0].digest`                                              | `windlass.verify.error.subject-digest-scope-invalid`                       | verification | `1`       | `false`           |
| `startedOn` or `finishedOn` is not canonical whole-second UTC RFC 3339                            | `windlass.verify.error.timestamp-format-invalid`                           | verification | `1`       | `false`           |
| `finishedOn` precedes `startedOn` by more than five seconds                                       | `windlass.verify.error.timestamp-ordering-invalid`                         | verification | `1`       | `false`           |
| Emitted Statement differs from validated signing inputs                                           | `windlass.verify.error.statement-assembly-mismatch`                        | verification | `1`       | `false`           |

## Failure behavior

If the trusted core or a profile produces signing inputs that would not result in a Statement
matching this contract, the producer-side verification gate must reject the bundle before
publication.

If a verifier receives a bundle that violates this contract, the verification must fail with a clear
error category from the rejection matrix above.

## TDD and fixtures

- Golden Statement fixtures have valid `_type` and `predicateType`.
- They have valid `builder.id`, `buildType`, and `subject`.
- Golden fixtures include the required `workflow` invocation identity.
- They include the closed `internalParameters.github` invocation metadata.
- They cover role-named source descriptors and deterministic media types.
- They cover current runner-image annotations and required `nodejs`/`npm` builder versions.
- Rejected fixtures cover every rejection-matrix row and zero or multiple subjects.
- Duplicate cases cover top-level, nested predicate, extension-field, and escaped members.
- Timestamp cases cover non-`Z`, fractional, Holocene-year, and excessive negative skew.
- An npm signing fixture proves the DSSE payload equals the preassembled Statement bytes.
- `npm-internal-parameters-github-valid` proves the three-string GitHub object.
- `npm-builder-version-direct-npm-valid` proves direct-npm `builder.version`.
- `npm-builder-version-corepack-valid` proves Corepack `builder.version`.
- Both version fixtures require `nodejs` and `npm`; Corepack adds conditional `corepack`.
- `npm-builder-signing-adapter-valid` proves the one signing-adapter dependency.
- `npm-invocation-id-certificate-uri-valid` proves a run-attempt URI equal to Fulcio OID `.21`.
- A tag-push fixture omits `source` when no selector is supplied and never emits `source: {}`.
- A dispatch-retry fixture records the exact caller selector in `source.ref`.
- Effective built refs follow `source.ref ?? workflow.ref` in both cases.
- Coincident runs keep distinct source descriptors with identical URI and digest values.
- Retry runs bind certificate claims to `workflow.ref` and `invocation-source`.
- Release policy binds to the effective built ref and `built-source`.
- No certificate comparison uses `source.invocation_*` or a built-field fallback.
- Accepted descriptor fixtures cover every media-type matrix row and six/seven cardinalities.
- Producer order is fixed before signing, with `invocation-source` at npm index 0.
- Semantic verification accepts descriptor permutations selected by name.
- Runner fixtures preserve multiline `operating_system`, full `image_version`, and the verbatim URI.
- They cover nullable divergent `deployment_version` and independent `runner_label`.
- Valid captures without `$ImageOS` and offline report unavailability are accepted.
- Acceptance tests ignore unrecognized Statement envelope members.
- Recognized fields stay unchanged and project-owned closed schemas remain strict.
- Rejected fixtures cover invalid `internalParameters` presence and shape.
- Cases include absent, `null`, empty, non-object, and wrong-shaped values.
- They cover missing, extra, non-string, actor, and certificate-mismatched GitHub fields.
- They cover Build Trigger and Build Config URI mismatches.
- They cover caller/builder path confusion and numeric repository/owner mismatches.
- Removed builder fields and derived source or release fields are rejected.
- Removed runtime, package-manager, caller, and build records are rejected.
- Descriptor fixtures reject duplicate names before lookup and unknown names.
- They reject source URI or `gitCommit` mismatches and source `mediaType`.
- They reject missing or extra source annotations and URI-inconsistent built `ref_type`.
- They reject missing, incorrect, or representation-inconsistent media types.
- They reject `downloadLocation` and incorrect six/seven cardinalities.
- They reject invalid package-manager-distribution, node-runtime, and runner-image descriptors.
- Package-manifest cases reject wrong identity, path, digest, type, or `workspace_root`.
- Missing or mismatched offline manifest evidence fails without a network fallback.
- Runner fixtures reject `image_os`, `node_version`, and missing or additional annotations.
- They reject forbidden top-level members and malformed or duplicate OS groups or details.
- They reject whitespace-only, trimmed, or compacted OS values and mixed metadata snapshots.
- They reject `$ImageOS` comparison or fallback and URI/version projection mismatches.
- They reject deployment/image equality requirements.
- Producer evidence failures remain fail closed without network substitution.
- Builder fixtures reject missing or extra version keys, absent `npm`, and wrong `corepack`.
- They reject unauthorized or signer-inconsistent builder identity and invalid signer descriptors.
- They reject missing or manifest-inconsistent `windlass_build_script_disposition`.
- They reject malformed or certificate-unequal invocation IDs.
- Descriptor and evidence failures use their corresponding registered diagnostics.
- Version failures use `windlass.verify.error.builder-version-mismatch`.
- Signer dependency failures use the signing-adapter mismatch diagnostic from the matrix.
- Disposition failures use `windlass.verify.error.build-script-disposition-mismatch`.
- Invocation ID failures use `windlass.verify.error.run-invocation-uri-invalid`.
