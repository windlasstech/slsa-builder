# Verification Policy, Fixtures, And Reference Commands

This document defines the verifier policy, the fixture taxonomy, and the reference commands that
downstream consumers can use to verify artifacts produced by `slsa-builder`.

- Source ADRs: [0028](../decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md),
  [0029](../decisions/0029-use-windlass-generated-slsa-provenance-for-npm-publish.md),
  [0030](../decisions/0030-accept-registry-url-while-guaranteeing-only-npmjs-semantics.md),
  [0036](../decisions/0036-use-three-job-digest-verified-publish-graph.md),
  [0037](../decisions/0037-define-initial-verification-deliverables.md),
  [0049](../decisions/0049-separate-artifact-production-from-github-release-asset-publication.md),
  [0050](../decisions/0050-define-producer-to-publisher-handoff-contract.md),
  [0051](../decisions/0051-distribute-producer-provenance-with-release-assets.md),
  [0052](../decisions/0052-compose-npm-package-tarball-producer-with-release-asset-publisher.md),
  [0053](../decisions/0053-use-three-job-release-manifest-signing-boundary.md),
  [0054](../decisions/0054-use-slsa-builder-dev-release-manifest-predicate-uri.md),
  [0055](../decisions/0055-use-actions-attest-custom-mode-for-statement-construction.md),
  [0056](../decisions/0056-treat-non-selected-lockfiles-as-stale-diagnostics.md),
  [0057](../decisions/0057-provide-composed-public-npm-release-asset-workflow.md),
  [0058](../decisions/0058-define-github-release-asset-publisher-authority-boundary.md),
  [0059](../decisions/0059-define-public-npm-release-composed-workflow-interface.md),
  [0060](../decisions/0060-unify-npm-profile-public-entrypoint-with-release-asset-mode.md),
  [0061](../decisions/0061-reject-duplicate-json-members-in-signed-slsa-statements.md),
  [0062](../decisions/0062-intersect-trusted-producer-policies.md)
- Related specs: [SLSA provenance v1](slsa-provenance-v1.md),
  [Identity and build types](identity-and-buildtypes.md), [Release manifest](release-manifest.md),
  [JS/TS npm build and pack](js-ts-npm-build-pack.md),
  [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md),
  [npm-to-release-asset composition](npm-to-release-asset-composition.md)

## Scope and non-goals

**In scope:**

- Verifier policy schema.
- Roots of trust.
- Producer-side vs. consumer-side verification.
- Fixture taxonomy.
- Reference commands.
- No standalone verifier CLI boundary.

**Out of scope:**

- A standalone consumer verifier CLI for the initial profile.
- Implementation of third-party tools.

## No standalone consumer verifier CLI in the initial profile

The initial profile delivers verifier policy, fixtures, and reference commands. It does not ship a
standalone verifier CLI. Downstream consumers may build their own verifiers using the policy and
fixtures in this document.

A future ADR may add a standalone verifier CLI when the project needs one.

## Roots of trust

A verifier must trust the following roots:

1. **Sigstore root of trust** for bundle signatures.
2. **Windlass release manifest** for the mapping of release versions to trusted workflow SHAs,
   `builder.id` values, and `buildType` URIs.
3. **GitHub** as the hosted runner and OIDC identity provider.

The verifier must obtain the release manifest from a trusted source, such as the signed release
manifest bundle on the GitHub Release page or a mirrored copy whose signature is verified.

## Trusted producer policy conflict resolution

When both an explicit verifier policy and a verified Windlass release manifest policy are present,
the effective trusted producer policy is the intersection of both sources. A verifier must reject
the artifact unless both policy sources allow the observed producer signer identity, source
repository, source revision, release ref, workflow path, workflow SHA, `builder.id`, `buildType`,
SLSA `predicateType`, subject name, subject digest, and required or forbidden `externalParameters`.

The signed release manifest policy is eligible for this intersection only after the manifest bundle
passes the release manifest verification policy below. The manifest must not define or relax its own
trust root, signer identity, predicate type, schema version, or authority to override explicit
verifier policy.

Missing trusted producer fields are not wildcards in the initial production policy. If either source
omits a required producer identity, source, release ref, workflow, subject, or strict
`externalParameters` constraint, the verifier must treat the effective policy as unsatisfied unless
a later ADR-backed schema explicitly marks that field as intentionally unconstrained.

When the two sources conflict or their intersection is empty, verification must fail closed. A
verifier must not use precedence, last-writer-wins behavior, or either-source-allowed behavior in
the default production policy. Diagnostics should name the conflicting source and field.

## npm package verification policy

For an npm package published by the JS/TS npm profile, a verifier must check:

1. The package tarball is the artifact being verified.
2. The provenance bundle is the Windlass-signed bundle for the tarball.
3. The bundle signature is valid and the signer identity is trusted.
4. The `predicateType` is `https://slsa.dev/provenance/v1`.
5. The `builder.id` is in the trusted release manifest and uses a full SHA.
6. The `buildType` is `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`.
7. The `subject[0].name` matches the expected tarball file name.
8. The `subject[0].digest.sha256` matches the tarball bytes.
9. The `externalParameters` include the expected source repository, ref, commit, package directory,
   package identity, package manager, runner, publish intent, release ref, and build-script result.
10. The signer identity is the Windlass reusable workflow identity, while the source repository in
    `externalParameters.source.repository` is the caller package repository.
11. No unexpected `externalParameters` are present under strict matching.
12. The selected package is not marked `private: true`.
13. For `https://registry.npmjs.org/`, the package version was not already present before publish
    and the selected package identity already existed before publish; first publication of a package
    identity is outside the initial npmjs trusted-publishing-only profile.
14. For non-npmjs registries, package identity and package version preflight fields are best-effort
    diagnostics unless a later ADR defines that registry class. Consumer-side verification must not
    report those diagnostics as Windlass-guaranteed registry support.
15. `externalParameters.package.package_url` equals the registry package-version URL reconstructed
    from `externalParameters.publish.resolved_registry_url`, `externalParameters.package.name`, and
    `externalParameters.package.version`. It must not be a Package URL (`pkg:npm/...`).
16. `externalParameters.source.ref`, `externalParameters.release.ref`, and the accepted runtime ref
    are the same full `refs/tags/<tag-name>` ref, and `externalParameters.release.version_tag`
    reconstructs that same ref.

npmjs.com trusted publisher configuration is registry-side publish authorization policy. It is
enforced by the producer-side publish gate and by npm during `npm publish`, but consumer-side
Windlass SLSA provenance verification does not reconstruct or require the caller repository/workflow
filename configured in npmjs.com trusted publisher settings.

When manifest metadata selected `externalParameters.package_manager.name` and that manager's
required lockfile is present, verifier policy treats that lockfile as the selected dependency
lockfile. Any non-selected lockfiles recorded in
`externalParameters.package_manager.ignored_lockfile_paths` are diagnostics only. A verifier must
not reject otherwise valid provenance merely because another supported package-manager lockfile was
present and recorded as ignored under that rule.

## GitHub Release asset verification policy

For a release asset uploaded by the publisher, a verifier must check:

1. The downloaded release asset is the primary artifact.
2. The provenance sidecar is present and is the unchanged producer bundle.
3. The producer bundle signature is valid and the signer identity is trusted.
4. The producer `predicateType` is `https://slsa.dev/provenance/v1`.
5. The producer `builder.id` and `buildType` are in the trusted policy.
6. The producer `subject[0].name` matches the release asset name.
7. The producer `subject[0].digest.sha256` matches the downloaded asset bytes.
8. The publisher did not generate or re-sign the provenance.

The publisher itself does not have a source-to-artifact `buildType` in the default path.

## Release manifest verification policy

For a release manifest, a verifier must check:

1. The signed bundle signature is valid.
2. The signer identity is the GitHub Actions OIDC identity for
   `.github/workflows/release-manifest.yml` in `windlasstech/slsa-builder` on the expected protected
   release tag.
3. The predicate type is `https://slsa-builder.dev/predicates/release-manifest/v1`.
4. The schema version is supported.
5. The release tag matches the expected tag.
6. The release commit SHA matches the tag.
7. Each producer profile entry maps to the expected workflow SHA, `builder.id`, and `buildType`.
8. Each publisher workflow entry maps to the expected workflow path, workflow SHA, and
   `verified-distributor` role, without `builder.id` or `buildType`.
9. The in-toto Statement predicate equals the plain release manifest JSON value, and the Statement
   subject digest equals the SHA-256 digest of the RFC 8785 JCS canonical JSON bytes for that value.
10. For schema version `1`, every producer and publisher workflow SHA equals `release_commit_sha`,
    `producer_profiles` is sorted by `profile`, and `publisher_workflows` is sorted by `publisher`.
11. Annotated release tags peel recursively to a terminal commit, and that terminal commit equals
    `release_commit_sha`.
12. `generated_at` uses the fixed UTC lexical form and is treated as diagnostic release metadata,
    not as a trust-mapping override.

## Producer-side vs. consumer-side verification

- **Producer-side verification** runs inside the profile workflow before publish or upload. It is
  the gate that prevents bad artifacts from reaching consumers.
- **Consumer-side verification** runs after publication. It checks the signed provenance bundle
  against the downloaded artifact and the trusted release manifest.

Producer-side verification does not remove the need for consumer-side verification. A verifier must
not trust the workflow outputs, logs, or release notes as substitutes for the signed bundle.

## Reference commands

The following commands are reference starting points. They are not complete Windlass policy
verifiers on their own.

### Verify a GitHub artifact attestation

```bash
gh attestation verify \
  <artifact-or-bundle> \
  --owner windlasstech \
  --predicate-type https://slsa.dev/provenance/v1
```

### Verify npm signatures

```bash
npm audit signatures
```

This command checks npm registry signatures but does not verify the Windlass SLSA provenance policy.

### Use slsa-verifier where compatible

```bash
slsa-verifier verify-artifact \
  <artifact> \
  --provenance-path <bundle>.intoto.jsonl \
  --source-uri github.com/<caller-owner>/<caller-repo> \
  --builder-id <windlass-builder-id>
```

`--source-uri` is the package source repository recorded in `externalParameters.source.repository`.
`--builder-id` is the SHA-pinned Windlass reusable workflow identity from `runDetails.builder.id`.
This command may verify the SLSA provenance structure but may not enforce all Windlass-specific
policy checks such as strict `externalParameters` and release manifest mapping. npm trusted
publisher caller identity is a producer-side registry authorization precondition rather than a
consumer-side SLSA provenance verification field.

## Fixture taxonomy

Every fixture must include:

| Field                       | Description                                               |
| --------------------------- | --------------------------------------------------------- |
| `name`                      | Unique fixture name.                                      |
| `type`                      | `accepted` or `rejected`.                                 |
| `surface`                   | `npm`, `publisher`, `composition`, or `release-manifest`. |
| `artifact`                  | Path to the artifact.                                     |
| `provenance`                | Path to the provenance bundle.                            |
| `release-manifest`          | Path to the release manifest, if applicable.              |
| `expected-result`           | `pass` or `fail`.                                         |
| `expected-failure-category` | Required for `rejected` fixtures.                         |
| `covered-requirement`       | Spec or ADR requirement covered by the fixture.           |

## Accepted fixtures

| Name                                      | Surface          | Description                                                       |
| ----------------------------------------- | ---------------- | ----------------------------------------------------------------- |
| `npm-valid-release`                       | npm              | Valid npm package tarball with matching Windlass provenance.      |
| `npm-valid-scoped-package-url`            | npm              | Valid scoped npm package with registry package-version URL.       |
| `npm-actions-attest-custom-bundle-valid`  | npm              | Custom-mode emitted bundle is accepted as npm provenance file.    |
| `npm-resolved-lockfile-valid`             | npm              | Selected lockfile descriptor matches path, digest, and manager.   |
| `npm-resolved-lockfile-stale-valid`       | npm              | Stale non-selected lockfiles are recorded as diagnostics only.    |
| `npm-release-asset-mode-valid`            | npm              | Public npm workflow release-asset mode uploads tarball + sidecar. |
| `npm-release-asset-linked-metadata-valid` | npm              | Release-asset mode creates linked artifact metadata when enabled. |
| `publisher-valid-upload`                  | publisher        | Valid producer handoff leading to release asset and sidecar.      |
| `composition-valid-npm-tarball`           | composition      | npm tarball successfully composes with publisher.                 |
| `release-manifest-valid`                  | release-manifest | Signed manifest with valid producer and publisher mappings.       |

## Rejected fixture categories

| Category                                   | Description                                                                                |
| ------------------------------------------ | ------------------------------------------------------------------------------------------ |
| `digest-mismatch`                          | Artifact digest does not match the provenance subject digest.                              |
| `signature-mismatch`                       | Bundle signature is invalid or missing.                                                    |
| `signer-mismatch`                          | Signer identity is not trusted.                                                            |
| `duplicate-json-member`                    | Signed Statement JSON contains duplicate object member names after unescaping.             |
| `actions-attest-adapter-contract`          | Adapter inputs, emitted bundle basename, or npm provenance-file compatibility is invalid.  |
| `wrong-producer-signer`                    | Producer signer repo, workflow path, ref, or issuer is not trusted.                        |
| `wrong-predicate-type`                     | `predicateType` is not SLSA provenance v1.                                                 |
| `wrong-manifest-predicate-type`            | Release manifest `predicateType` is not the ADR 0054 predicate URI.                        |
| `wrong-builder-id`                         | `builder.id` is not trusted or uses a non-SHA reference.                                   |
| `wrong-build-type`                         | `buildType` is not the canonical profile URI.                                              |
| `subject-cardinality-error`                | Provenance contains zero subjects or multiple subjects.                                    |
| `unexpected-external-parameters`           | `externalParameters` contains unexpected fields under strict matching.                     |
| `source-identity-mismatch`                 | Source repository or revision does not match policy.                                       |
| `release-ref-mismatch`                     | Source ref, release ref, runtime ref, or version tag do not identify the same tag.         |
| `source-repository-canonicalization-error` | Source repository URL is non-canonical, ambiguous, or malformed.                           |
| `trusted-publisher-mismatch`               | Producer-side npm trusted publishing caller identity or OIDC permission is wrong.          |
| `package-identity-mismatch`                | npm package name or version does not match.                                                |
| `package-url-mismatch`                     | npm registry package-version URL is malformed or does not match registry/name/version.     |
| `unsupported-initial-publication`          | Selected package identity does not already exist on npmjs.                                 |
| `package-version-mismatch`                 | Tag version does not match `package.json` version.                                         |
| `package-directory-mismatch`               | `externalParameters.package.directory` does not match expected.                            |
| `package-manager-selection-path-mismatch`  | Package-manager selection path is missing or wrong in provenance.                          |
| `private-package`                          | Selected package manifest has `private: true`.                                             |
| `publish-intent-conflict`                  | Workflow publish input conflicts with source `publishConfig`.                              |
| `invalid-publish-input`                    | Non-empty workflow publish input has an unsupported value or format.                       |
| `empty-publish-input-fallback`             | Empty workflow input failed to fall back to source `publishConfig`.                        |
| `already-published-version`                | Selected package name/version already exists before publish.                               |
| `workspace-resolution-mismatch`            | Workspace root, package manager root, or lockfile policy is wrong.                         |
| `workspace-pattern-base-mismatch`          | Workspace patterns were evaluated against the wrong base directory.                        |
| `workspace-command-mismatch`               | Workspace package targeting command can affect the wrong package.                          |
| `package-manager-manifest-shape-error`     | `devEngines.packageManager` uses an unsupported shape, member, or release version form.    |
| `resolved-dependencies-lockfile`           | Selected lockfile `resolvedDependencies` descriptor is missing, malformed, or mismatched.  |
| `release-asset-mode-schema-error`          | Public npm release-asset mode input or output schema is invalid.                           |
| `release-asset-mode-disabled-conflict`     | Release-asset-only inputs are supplied while release-asset mode is disabled.               |
| `release-asset-mode-permission-error`      | Caller or internal job permissions are missing or combine separated authorities.           |
| `release-asset-target-error`               | Effective release tag or target release is missing, malformed, or outside the caller repo. |
| `runtime-policy-mismatch`                  | Runner or Node.js version does not match policy.                                           |
| `excessive-publish-permission`             | npmjs publish job requests permissions outside the initial boundary.                       |
| `npm-version-too-old`                      | npm CLI version is below `11.5.1` for trusted publishing.                                  |
| `release-manifest-mismatch`                | Release manifest mapping does not match the provenance.                                    |
| `trusted-producer-policy-conflict`         | Explicit verifier policy and signed release manifest policy cannot both be satisfied.      |
| `manifest-predicate-mismatch`              | Signed Statement predicate differs from canonical manifest JSON.                           |
| `manifest-digest-mismatch`                 | Statement subject digest differs from canonical manifest JSON bytes.                       |
| `manifest-trigger-mismatch`                | Release manifest workflow did not run from the expected protected SemVer tag.              |
| `manifest-tag-peel-mismatch`               | Release tag cannot be peeled to the expected terminal commit.                              |
| `manifest-entrypoint-mismatch`             | Release manifest signer workflow path is not the fixed production entrypoint.              |
| `manifest-caller-override`                 | Caller-controlled input changed a signed manifest trust field.                             |
| `manifest-workflow-sha-mismatch`           | Schema v1 workflow SHA does not equal the release tag target commit.                       |
| `manifest-entry-order-mismatch`            | Release manifest producer or publisher arrays are not in canonical sorted order.           |
| `manifest-generated-at-invalid`            | `generated_at` is not a fixed-form UTC timestamp.                                          |
| `manifest-handoff-basename-mismatch`       | Manifest handoff artifact contains an unexpected payload basename.                         |
| `manifest-partial-json-uploaded`           | Plain manifest JSON uploaded but signed bundle upload failed.                              |
| `manifest-indeterminate-json-upload`       | Manifest upload state cannot be determined after an ambiguous upload attempt.              |
| `bundle-byte-format-mismatch`              | Signed bundle bytes were extracted, reserialized, wrapped, or otherwise changed.           |
| `missing-producer-provenance`              | Publisher receives an artifact without producer provenance.                                |
| `raw-artifact-bypass`                      | Raw caller artifact bypasses producer verification.                                        |
| `handoff-schema-mismatch`                  | Cross-job artifact handoff omits or changes required core fields.                          |
| `composition-handoff-substitution`         | Composition mapping trusts public outputs or deterministic names.                          |
| `publisher-handoff-field-error`            | Publisher handoff uses missing, stale, or malformed field names.                           |
| `publisher-workflow-schema-error`          | Publisher exposes or accepts unsupported public workflow inputs or secrets.                |
| `publisher-permission-boundary-violation`  | Publisher job permissions combine authorities that must stay separate.                     |
| `native-locator-malformed`                 | Native provenance locator is not valid diagnostic metadata.                                |
| `native-locator-digest-mismatch`           | Native provenance locator digest differs from the sidecar bundle digest.                   |
| `sidecar-mismatch`                         | Sidecar bundle does not match the primary asset's provenance.                              |
| `sidecar-digest-mismatch`                  | `sidecar-digest` does not equal the verified producer bundle SHA-256.                      |
| `sidecar-upload-partial-failure`           | Primary release asset uploaded but sidecar upload failed afterward.                        |
| `publisher-indeterminate-primary-upload`   | Primary release asset upload state cannot be determined after an ambiguous upload.         |
| `duplicate-release-asset`                  | Release asset name already exists.                                                         |
| `duplicate-sidecar-asset`                  | Deterministic sidecar asset name already exists before upload.                             |
| `registry-linkage-mismatch`                | Published package does not match the provenance registry metadata.                         |
| `custom-registry-preflight-diagnostic`     | Non-npmjs registry preflight metadata is best-effort and not guaranteed support.           |
| `custom-registry-token-required`           | Custom registry metadata or publish path requires token or weaker provenance behavior.     |
| `prepublish-registry-metadata-required`    | Workflow required post-publish registry metadata before publish.                           |
| `release-version-semver-mismatch`          | Release manifest version or tag is not valid SemVer 2.0.0.                                 |
| `trusted-core-boundary-violation`          | Trusted policy/provenance logic depends on profile ecosystem tooling.                      |

## Error categories

Each rejected fixture must produce a clear error category. Verifier implementations should surface
the category in a machine-readable way where possible.

## TDD usage

Fixtures drive implementation:

1. Write the failing fixture first.
2. Implement the behavior that makes the accepted fixture pass.
3. Ensure the rejected fixture continues to fail with the correct error category.
4. Add fixtures for any new normative behavior.

The publish-intent fixture set must distinguish empty workflow inputs from non-empty invalid or
conflicting inputs: empty `registry-url`, `dist-tag`, and `access` inputs are omitted and may fall
back to source `publishConfig`, while non-empty invalid values fail validation and non-empty values
that differ from source `publishConfig` fail with `publish-intent-conflict`.

The package URL fixture set must include unscoped and scoped npm package identities. Scoped package
fixtures must prove that `@scope/name` is emitted as an npm registry package-version URL such as
`https://registry.npmjs.org/%40scope%2Fname/<version>`, that `pkg:npm/...` PURLs are rejected for
the initial `package-url` output, and that the reconstructed URL matches
`externalParameters.publish.resolved_registry_url`, `externalParameters.package.name`, and
`externalParameters.package.version`.

The release-ref fixture set must prove that `externalParameters.source.ref`,
`externalParameters.release.ref`, and the runtime-accepted ref are identical full tag refs, and that
`externalParameters.release.version_tag` reconstructs that same ref. Fixtures with a short tag,
branch ref, pull request ref, mismatched source/release refs, or mismatched version tag must fail
with `release-ref-mismatch`.

The custom-registry fixture set must prove that non-npmjs package identity and package version
preflight checks are best-effort diagnostics: an unsupported custom registry may record
`publish.package_identity_preexisting` or `publish.package_version_preexisting` as `null` when a
tokenless metadata check cannot prove the state, but it must still fail if tokenless publish with
the external provenance bundle is unavailable.

The custom-registry fixture set must separate inconclusive diagnostics from hard failures. A fixture
where tokenless metadata is unavailable or inconclusive must pass only when the unproven preflight
fields are recorded as `null` and `publish.custom_registry_support` is
`unsupported-but-not-blocked`. A fixture where metadata discovery or publish requires `NPM_TOKEN`,
`NODE_AUTH_TOKEN`, OTP, unsigned provenance, npm automatic provenance fallback, or omission of the
external provenance bundle must fail with `custom-registry-token-required` or the narrower publish
failure category.

The workspace fixture set must include nested workspace roots and prove that workspace patterns are
evaluated relative to each candidate workspace root, not relative to the repository root. A fixture
whose pattern only matches under the wrong base path must fail with
`workspace-pattern-base-mismatch`.

The package-manager manifest fixture set must prove that top-level `packageManager` uses the
`name@version` descriptor form while `devEngines.packageManager` uses the closed object form
accepted by the JS/TS npm build and pack spec. Accepted fixtures must include exact pnpm and Yarn
versions in `devEngines.packageManager.version`. Rejected fixtures must cover string-form
`devEngines.packageManager`, array-form `devEngines.packageManager`, unknown object members, missing
pnpm/Yarn versions, range versions, tag versions, URL descriptors, hash-suffixed descriptors, and
`onFail: "ignore"` or `onFail: "warn"` attempts that would otherwise weaken release-build policy.
These failures use `package-manager-manifest-shape-error` unless a narrower package-manager
selection or lockfile category applies.

The `actions/attest` adapter fixture set must prove the stock custom-mode adapter contract selected
by ADR 0055. Accepted fixtures must include a custom-mode emitted bundle named
`<package-tarball-name>.intoto.jsonl` whose extracted Statement matches the Windlass-verified
subject inputs, `predicateType`, and SLSA predicate, and whose bytes are accepted by the npm
`--provenance-file` path. Rejected fixtures must cover raw Statement files used as provenance,
reserialized or wrapped bundles, GitHub artifact attestation storage locators substituted for bundle
bytes, wrong or missing `slsa-provenance-predicate.json` adapter input, wrong predicate type,
emitted Statement mismatch, missing or renamed emitted bundle file, unparseable Sigstore bundle
bytes, and npm CLI rejection of the external provenance file before registry mutation. These
failures use `actions-attest-adapter-contract` unless the narrower wrong-predicate,
bundle-byte-format, signer, or duplicate-member category applies.

The `resolvedDependencies` lockfile fixture set must prove that the initial JS/TS npm profile emits
exactly one selected lockfile `ResourceDescriptor` and no generated transitive dependency list.
Accepted fixtures must cover manifest-selected npm with `package-lock.json`, manifest-selected pnpm
with `pnpm-lock.yaml`, manifest-selected Yarn with `yarn.lock`, and lockfile-inferred npm with
`package-lock.json`. Accepted stale-lockfile fixtures must record stale non-selected lockfiles in
both `externalParameters.package_manager.ignored_lockfile_paths` and
`resolvedDependencies[0].annotations.stale_non_selected_lockfiles` while keeping the selected
lockfile descriptor as the only dependency graph input. Rejected fixtures must cover a missing
descriptor, extra descriptor entries, full dependency-list entries, digest mismatch, URI source or
revision mismatch, fragment path outside the repository, descriptor path that names a stale or
non-selected lockfile, missing stale-lockfile diagnostics, unknown annotation members, and treating
a stale non-selected lockfile as the selected dependency graph input. These failures use
`resolved-dependencies-lockfile` unless a narrower package-manager or workspace category applies.

The public npm release-asset mode fixture set must prove that
`.github/workflows/js-ts-npm-package-slsa3.yml` has one public npm entrypoint with two modes.
Accepted fixtures must cover npm-only mode, release-asset mode with mandatory provenance sidecar
upload, and release-asset mode with linked artifact metadata enabled. Rejected fixtures must cover
release-asset-only inputs while `release-asset-mode` is `false`, non-empty or malformed
`release-tag` values that do not equal the current package version tag, missing existing GitHub
Release targets, sidecar disable or rename attempts, linked metadata enabled without
`artifact-metadata: write`, release-asset mode without caller `contents: write`, and internal jobs
that combine release mutation, signing, package publishing, mapping, or metadata authorities. Schema
failures use `release-asset-mode-schema-error`; disabled-mode conflicts use
`release-asset-mode-disabled-conflict`; missing or excessive permissions use
`release-asset-mode-permission-error`; target selection failures use `release-asset-target-error`
unless a narrower publisher category applies.

The public npm release-asset mode fixture set must also prove that release-asset outputs are
publication result handles only. Accepted outputs include package identity, npm tarball digests,
release asset name, release asset URL, release asset SHA-256, provenance sidecar name, sidecar URL,
sidecar SHA-256, native provenance locators, upload result, and linked artifact result. Rejected
fixtures must prove that internal handoff manifest names, handoff manifest digests, internal
producer artifact names, publisher handoff input names, raw artifact paths, caller-supplied artifact
digests, upload URLs, target repository coordinates, custom tokens, overwrite flags, release
creation flags, or multi-asset controls are not public inputs or outputs.

The handoff fixture set must prove that every cross-job artifact handoff includes the core semantic
fields `transport`, `artifact_name`, `payload_file_name`, `payload_kind`, `digest.algorithm`, and
`digest.value`, whether those fields are public inputs or profile-owned fixed mappings. Missing or
malformed fields fail with `handoff-schema-mismatch`.

The signed bundle fixture set must prove that the bundle file bytes emitted by `actions/attest` are
preserved byte-for-byte through npm provenance submission, publisher sidecar redistribution, and
release manifest upload. Fixtures that extract only the Statement, reserialize the bundle, wrap it
in a new envelope, or substitute a native attestation locator for the bundle file must fail with
`bundle-byte-format-mismatch` or the narrower sidecar/provenance mismatch category.

The duplicate JSON member fixture set must prove that signed SLSA Statement payloads fail before
semantic policy validation when any JSON object contains duplicate member names after JSON string
unescaping. Rejected fixtures must cover a top-level Statement duplicate, a nested predicate
duplicate, a duplicate extension-field member, and escaped property spellings that decode to the
same member name. These fixtures fail with `duplicate-json-member`.

The npm publish permissions fixture set must prove that the initial npmjs production publish job has
`contents: read`, `id-token: write`, no `attestations: write`, and no `packages: write`. A workflow
that grants `packages: write` for the initial npmjs path fails with `excessive-publish-permission`.

The trusted-publisher fixture set is producer-side: it must prove that missing caller OIDC
permission or npmjs.com trusted publisher mismatch fails before registry mutation and never falls
back to token publish. These fixtures must not require adding caller trusted publisher settings to
the signed SLSA `externalParameters` schema.

The publisher fixture set must distinguish duplicate preflight failures from partial upload
failures: pre-existing primary or sidecar asset names fail before upload, while a sidecar API or
transport failure after successful primary upload fails with `sidecar-upload-partial-failure` and
reports `upload-result: partial-primary-uploaded`.

The publisher fixture set must also include an ambiguous primary upload result. If same-run release
lookup cannot prove that the primary asset was absent, or cannot prove that a same-name remote asset
has the expected digest, the fixture must fail with `publisher-indeterminate-primary-upload` and
report `upload-result: indeterminate-primary-upload`.

The publisher workflow schema fixture set must prove that the standalone publisher accepts only the
declared producer-neutral `workflow_call.inputs`, accepts no secrets, rejects target repository,
custom token, raw artifact, overwrite, cross-run artifact, or provenance-bypass inputs, and treats
empty optional JSON inputs as absent. Unsupported inputs or accepted secrets fail with
`publisher-workflow-schema-error`.

The publisher permission fixture set must prove that verification jobs are read-only, the release
upload job has `contents: write` without signing, package publication, or linked metadata authority,
and the optional metadata job has `artifact-metadata: write` without release mutation or signing
authority. Excessive or missing permissions fail with `publisher-permission-boundary-violation` or a
narrower permission category.

The sidecar digest fixture set must prove that `sidecar-digest` is the 64-character lowercase
SHA-256 digest of the exact verified producer bundle bytes and equals `producer-provenance-sha256`.
It must be set after bundle verification even when sidecar upload later fails, and unset when bundle
retrieval or digest verification fails. A mismatched value fails with `sidecar-digest-mismatch`.

The native locator fixture set must prove that native provenance locators are diagnostic metadata
only. A missing locator must not fail otherwise valid publisher verification. A locator with an
unsupported type, non-`github.com` URL, repository mismatch, userinfo, query, fragment, malformed
path, unknown field, or malformed digest must fail with `native-locator-malformed`. A locator digest
that does not equal the sidecar bundle SHA-256 must fail with `native-locator-digest-mismatch`.

The release manifest fixture set must distinguish duplicate preflight failures from partial upload
failures: pre-existing manifest JSON or bundle names fail before upload, while a signed bundle API
or transport failure after successful plain JSON upload fails with `manifest-partial-json-uploaded`
and reports `manifest-upload-result: partial-json-uploaded`.

The release manifest fixture set must include an ambiguous plain JSON upload result. If same-run
release lookup cannot prove that the plain manifest was absent, or cannot prove that a same-name
remote manifest has the expected digest, the fixture must fail with
`manifest-indeterminate-json-upload` and report `manifest-upload-result: indeterminate-json-upload`.

The release manifest generation fixture set must prove schema version `1` determinism: all producer
and publisher `workflow_sha` values equal `release_commit_sha`; producer `builder_id` values are
derived from `workflow_path` and `workflow_sha`; `producer_profiles` is sorted by `profile`; and
`publisher_workflows` is sorted by `publisher`. Mismatched workflow SHAs fail with
`manifest-workflow-sha-mismatch`; unsorted arrays fail with `manifest-entry-order-mismatch`.

The release manifest generation fixture set must also prove annotated tag peeling and timestamp
rules. Lightweight tags and annotated tags that peel to the expected commit pass. Missing tags,
cycles, non-commit terminal objects, and terminal commits that differ from `release_commit_sha` fail
with `manifest-tag-peel-mismatch`. A `generated_at` value with a timezone offset, leap second,
subsecond precision, non-UTC form, invalid calendar value, or caller-supplied override fails with
`manifest-generated-at-invalid` or `manifest-caller-override`.

The release manifest handoff fixture set must prove the fixed schema version `1` internal basenames:
`release-manifest-<version>.json`, `release-manifest-<version>.predicate.json`,
`release-manifest-<version>.signing-input.json`, and `release-manifest-<version>.intoto.jsonl`. A
handoff artifact with the expected digest but an unexpected payload basename must fail with
`manifest-handoff-basename-mismatch`.

The release manifest fixture set must also cover the production workflow public contract. Accepted
fixtures run from `.github/workflows/release-manifest.yml` on a protected `refs/tags/v<version>` ref
whose SemVer version equals `release_version` and whose target commit equals `release_commit_sha`.
Rejected fixtures must cover branch refs, pull request refs, short tag inputs, non-SemVer tags,
missing target releases, signer workflow path mismatches, signer workflow ref mismatches, and any
caller-controlled input that changes `release_version`, workflow SHAs, `builder.id`, `buildType`, or
predicate type.

## Future standalone verifier decision boundary

A standalone verifier CLI may be added when one or more of the following becomes true:

- Downstream consumers need a single maintained tool for Windlass verification.
- The reference commands no longer cover the common verification paths.
- The fixture taxonomy becomes too large to verify manually.
- A dedicated verifier would reduce duplication across consumer integrations.

## Verification checklist

Before declaring a release ready, verify that:

- Every normative "must" in the architecture specs has at least one fixture.
- Every rejected behavior has at least one rejected fixture.
- The release manifest is signed and maps every released profile.
- The signed release manifest bundle is uploaded to the GitHub Release.
- Reference commands are documented with their limitations.
- No standalone verifier CLI is promised for the initial profile.
