# Common SLSA Provenance v1 Contract

This document defines the shared SLSA provenance v1 contract used by producer profiles.
Profile-specific rules such as subject naming and `externalParameters` are layered on top of this
common contract.

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
  and
  [0077](../decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md)
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

Every SLSA provenance emitted by a producer profile must be wrapped in an in-toto Statement with the
following shape:

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
      "externalParameters": {},
      "internalParameters": {},
      "resolvedDependencies": [
        { "name": "lockfile" },
        {
          "name": "runner-image",
          "uri": "<platform-provided Included Software URL>",
          "annotations": {
            "image_os": "<ImageOS>",
            "image_version": "<ImageVersion>",
            "node_version": "<exact node --version output>"
          }
        }
      ]
    },
    "runDetails": {
      "builder": {
        "id": "<full GitHub reusable workflow URI with SHA>",
        "version": { "nodejs": "v24.0.0" },
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
        "finishedOn": "<RFC 3339 UTC timestamp at whole-second precision>"
      }
    }
  }
}
```

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

`externalParameters` must be complete and verifier-relevant. Every field that a verifier needs to
decide whether the build is trustworthy must be present and explicit.

Common field groups recorded by producer profiles include:

- `source`:
  - `repository`: source repository URI.
  - `ref`: source ref, for example `refs/tags/v1.2.3`.
  - `revision`: immutable source revision, such as a Git commit SHA.
- `workflow`:
  - `path`: reusable workflow file path.
  - `sha`: full commit SHA.
  - `builder_id`: the builder identity derived from the path and SHA.
- `inputs`: caller-provided inputs that affect the build output.
- `runtime`:
  - `runner`: `ubuntu-24.04`.
- `package_manager`: name, actual version, and selection source when the profile uses a package
  manager.

A verifier must reject unexpected `externalParameters` fields when the policy requires strict
matching. The profile spec must define the complete schema and the strict-matching policy.

For the initial JS/TS npm package profile, the complete normative schema is defined in
[JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md). The common contract is not a
fallback schema and must not be used to accept fields omitted by the profile schema.

## `internalParameters`

`internalParameters` must be exactly the empty object `{}`. It must not be absent, non-object, or
contain a member. A violation is rejected with
`windlass.verify.error.unexpected-internal-parameters`.

Valid:

```json
{ "internalParameters": {} }
```

Invalid, because it contains a member:

```json
{ "internalParameters": { "debug": true } }
```

Raw package metadata that is not trust input belongs only in `diagnostic_metadata.package_manifest`,
not in `internalParameters`. A producer that places it in `internalParameters` is rejected with
`windlass.verify.error.unexpected-internal-parameters`.

## `resolvedDependencies`

Each profile must define its complete, unordered, name-keyed `resolvedDependencies` shape and strict
matching policy. The common contract defines no fallback schema. A profile that emits an unknown or
non-enumerated dependency is rejected with
`windlass.verify.error.resolved-dependencies-unexpected-entry`.

The initial JS/TS npm profile enumerates `lockfile` and `runner-image`; pnpm and Yarn additionally
enumerate `package-manager-distribution`. Its descriptor shapes and conditional cardinalities are
defined by
[JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md#jsts-npm-resolveddependencies-schema).
That profile does not enumerate every installed package as a separate `ResourceDescriptor`. The
selected lockfile is the verifier-relevant dependency graph input, and package versions are
represented by its bytes and digest. Non-selected supported lockfiles may appear only as stale
diagnostics in profile-defined descriptor annotations and are not selected dependency graph inputs.

## `runDetails`

### `builder`

- `id`: full GitHub reusable workflow URI with SHA.
- `version`: a closed object containing required lowercase `nodejs` and conditional lowercase
  `corepack`. Values are exact observed version strings, not ranges, tags, or aliases. `corepack` is
  present only when Corepack supplied the selected package manager. `npm`, `runner-image`, and every
  other key are forbidden. Missing or extra keys, incorrect conditional `corepack` presence, or an
  observed-version mismatch is rejected with `windlass.verify.error.builder-version-mismatch`.
- `builderDependencies`: exactly one signing-adapter descriptor identifying the Go-native signer
  selected by ADR 0077. Its `uri` is `pkg:golang/github.com/sigstore/sigstore-go@v1.3.0`, its only
  digest member is the governed module checksum `h1: hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE=`,
  and its only annotation is `role: "signing-adapter"`. This is the default closed descriptor for
  every producer profile; profile admission must not substitute another signer. Missing, extra,
  malformed, wrong-role, or dependency-inconsistent descriptors, including build-job action
  descriptors, are rejected with
  `windlass.verify.error.builder-dependencies-signing-adapter-mismatch`.

The npm CLI version is already recorded by the npm profile in
`externalParameters.runtime.npm_version` and, for npm-selected runs, as
`externalParameters.package_manager.version`, so `builder.version` must not add an npm key. The
runner image version remains only in the named `runner-image` descriptor. A duplicated or forbidden
builder value is rejected with `windlass.verify.error.builder-version-mismatch`.

Valid direct-npm version shape:

```json
{ "version": { "nodejs": "v24.0.0" } }
```

Valid Corepack version shape:

```json
{ "version": { "nodejs": "v24.0.0", "corepack": "0.29.4" } }
```

Invalid, because npm is forbidden:

```json
{ "version": { "nodejs": "v24.0.0", "npm": "11.0.0" } }
```

Invalid, because `nodejs` is not an exact observed version string:

```json
{ "version": { "nodejs": "v24" } }
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

| Semantic field             | Required meaning                                                                                                       |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| OIDC issuer                | GitHub Actions, and not another identity provider.                                                                     |
| Signer workflow repository | Repository that owns the signing workflow, for example `windlasstech/slsa-builder`.                                    |
| Signer workflow path       | Workflow file path that performed signing, for example `.github/workflows/js-ts-npm-package-slsa3.yml`.                |
| Signer workflow ref or SHA | Immutable workflow identity selected by the profile: a full commit SHA for reusable producer workflows, or a protected |
|                            | release tag ref for the release manifest signer.                                                                       |
| Source repository          | Repository whose source was released, which may differ from the signer workflow repository for reusable workflows.     |
| Source ref                 | Release ref accepted by the producer or manifest runtime guards.                                                       |
| Source revision            | Immutable source revision recorded in the signed predicate and expected policy.                                        |
| Predicate type             | The predicate URI expected for the signed Statement.                                                                   |

For reusable producer profiles, the signer workflow repository and path identify the trusted
Windlass workflow, while the source repository, ref, and revision identify the caller package
repository and release. For the release manifest workflow, the signer workflow repository and source
repository are both `windlasstech/slsa-builder`, and the signer workflow ref is the protected
release tag recorded in the manifest.

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

When the pinned verification tool exposes multiple claim names for the same semantic field, the
implementation must apply this fallback order:

| Semantic field             | Preferred verified claim source                                         | Fallback verified claim source                                                  |
| -------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| OIDC issuer                | Sigstore certificate issuer extension.                                  | Tool-reported issuer value bound to the same certificate.                       |
| Signer workflow repository | Reusable workflow identity such as `job_workflow_ref` owner/repository. | Signing workflow `workflow_ref` owner/repository when no reusable claim exists. |
| Signer workflow path       | Reusable workflow identity such as `job_workflow_ref` workflow path.    | Signing workflow `workflow_ref` path when no reusable claim exists.             |
| Signer workflow SHA        | `job_workflow_sha` for reusable workflow signers.                       | `workflow_sha` or the SHA suffix of `runDetails.builder.id`.                    |
| Signer workflow ref        | Ref component of `job_workflow_ref` for reusable workflow signers.      | Ref component of `workflow_ref` when no reusable claim exists.                  |
| Source repository          | Source repository claim emitted for the caller/source repository.       | Signed predicate `externalParameters.source.repository` plus local policy.      |
| Source ref                 | Source ref claim emitted for the caller/source ref.                     | Signed predicate `externalParameters.source.ref` plus local policy.             |
| Source revision            | Source revision or commit claim emitted for the caller/source revision. | Signed predicate `externalParameters.source.revision` plus local policy.        |
| Predicate type             | Verified Statement `predicateType`.                                     | Tool-reported predicate type extracted from the same signed Statement.          |

If both a preferred and fallback source are present for the same semantic field, they must identify
the same value after the profile-defined canonicalization. A conflict is a signer identity failure,
not a reason to choose one spelling by precedence. If a required field remains unavailable after the
allowed fallback sources are checked, verification fails with a missing semantic identity field.

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
closed rather than accepting last-member-wins or first-member-wins behavior. Unknown fields remain
governed by the closed schema or profile strict-matching rules that apply after duplicate detection
succeeds.

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
| `buildType` is not in the canonical namespace                                                     | `windlass.verify.error.build-type-not-canonical`                           | verification | `1`       | `false`           |
| `externalParameters` is incomplete                                                                | `windlass.verify.error.external-parameters-incomplete`                     | verification | `1`       | `false`           |
| `externalParameters` contains unexpected fields                                                   | `windlass.verify.error.external-parameters-unexpected`                     | verification | `1`       | `false`           |
| `internalParameters` is absent, non-object, or nonempty                                           | `windlass.verify.error.unexpected-internal-parameters`                     | verification | `1`       | `false`           |
| A profile emits an unknown or non-enumerated dependency                                           | `windlass.verify.error.resolved-dependencies-unexpected-entry`             | verification | `1`       | `false`           |
| A package-manager-distribution descriptor is missing, duplicated, forbidden, or malformed         | `windlass.verify.error.resolved-dependencies-package-manager-distribution` | verification | `1`       | `false`           |
| A runner-image descriptor is missing, duplicated, malformed, digest-bearing, or mismatched        | `windlass.verify.error.resolved-dependencies-runner-image`                 | verification | `1`       | `false`           |
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

- Golden Statement fixture with valid `_type`, `predicateType`, `builder.id`, `buildType`, and
  `subject`.
- Rejected fixtures for each row in the rejection matrix, including zero-subject, multi-subject,
  top-level duplicate-member, nested predicate duplicate-member, duplicate extension-field, and
  escaped duplicate-member Statements, plus non-`Z`, fractional, Holocene-year, and excessive
  negative-skew timestamps.
- A fixture proving that the npm signing adapter's DSSE payload exactly matches the preassembled
  Windlass-verified Statement bytes.
- Accepted fixtures `npm-internal-parameters-empty-valid`, `npm-builder-version-direct-npm-valid`,
  `npm-builder-version-corepack-valid`, `npm-builder-signing-adapter-valid`, and
  `npm-invocation-id-certificate-uri-valid` prove exactly empty `internalParameters`, both
  `builder.version` shapes, the one signing-adapter dependency, and the run-attempt invocation URI
  equal to Fulcio OID `.21`.
- Rejected fixtures for nonempty or non-object `internalParameters`, missing or extra builder
  version keys, invalid package-manager-distribution or runner-image descriptors, an invalid
  signing-adapter descriptor, and malformed or certificate-unequal invocation IDs. They use
  `windlass.verify.error.unexpected-internal-parameters`,
  `windlass.verify.error.resolved-dependencies-package-manager-distribution`,
  `windlass.verify.error.resolved-dependencies-runner-image`,
  `windlass.verify.error.builder-version-mismatch`,
  `windlass.verify.error.builder-dependencies-signing-adapter-mismatch`, and
  `windlass.verify.error.run-invocation-uri-invalid`, respectively.
