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
  [0054](../decisions/0054-use-slsa-builder-dev-release-manifest-predicate-uri.md)
- Related specs: [SLSA provenance v1](slsa-provenance-v1.md),
  [Identity and build types](identity-and-buildtypes.md), [Release manifest](release-manifest.md),
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
12. The selected package is not marked `private: true` and the package version was not already
    present in the selected registry before publish.
13. The selected package identity already existed in the selected registry before publish; first
    publication of a package identity is outside the initial trusted-publishing-only profile.

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
policy checks (strict `externalParameters`, release manifest mapping, npm trusted publisher caller
identity, etc.).

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

| Name                            | Surface          | Description                                                  |
| ------------------------------- | ---------------- | ------------------------------------------------------------ |
| `npm-valid-release`             | npm              | Valid npm package tarball with matching Windlass provenance. |
| `publisher-valid-upload`        | publisher        | Valid producer handoff leading to release asset and sidecar. |
| `composition-valid-npm-tarball` | composition      | npm tarball successfully composes with publisher.            |
| `release-manifest-valid`        | release-manifest | Signed manifest with valid producer and publisher mappings.  |

## Rejected fixture categories

| Category                                   | Description                                                            |
| ------------------------------------------ | ---------------------------------------------------------------------- |
| `digest-mismatch`                          | Artifact digest does not match the provenance subject digest.          |
| `signature-mismatch`                       | Bundle signature is invalid or missing.                                |
| `signer-mismatch`                          | Signer identity is not trusted.                                        |
| `wrong-producer-signer`                    | Producer signer repo, workflow path, ref, or issuer is not trusted.    |
| `wrong-predicate-type`                     | `predicateType` is not SLSA provenance v1.                             |
| `wrong-manifest-predicate-type`            | Release manifest `predicateType` is not the ADR 0054 predicate URI.    |
| `wrong-builder-id`                         | `builder.id` is not trusted or uses a non-SHA reference.               |
| `wrong-build-type`                         | `buildType` is not the canonical profile URI.                          |
| `subject-cardinality-error`                | Provenance contains zero subjects or multiple subjects.                |
| `unexpected-external-parameters`           | `externalParameters` contains unexpected fields under strict matching. |
| `source-identity-mismatch`                 | Source repository or revision does not match policy.                   |
| `source-repository-canonicalization-error` | Source repository URL is non-canonical, ambiguous, or malformed.       |
| `trusted-publisher-mismatch`               | npm trusted publishing caller identity or OIDC permission is wrong.    |
| `package-identity-mismatch`                | npm package name or version does not match.                            |
| `unsupported-initial-publication`          | Selected package identity does not already exist in the registry.      |
| `package-version-mismatch`                 | Tag version does not match `package.json` version.                     |
| `package-directory-mismatch`               | `externalParameters.package.directory` does not match expected.        |
| `package-manager-selection-path-mismatch`  | Package-manager selection path is missing or wrong in provenance.      |
| `private-package`                          | Selected package manifest has `private: true`.                         |
| `publish-intent-conflict`                  | Workflow publish input conflicts with source `publishConfig`.          |
| `already-published-version`                | Selected package name/version already exists before publish.           |
| `workspace-resolution-mismatch`            | Workspace root, package manager root, or lockfile policy is wrong.     |
| `workspace-command-mismatch`               | Workspace package targeting command can affect the wrong package.      |
| `runtime-policy-mismatch`                  | Runner or Node.js version does not match policy.                       |
| `npm-version-too-old`                      | npm CLI version is below `11.5.1` for trusted publishing.              |
| `release-manifest-mismatch`                | Release manifest mapping does not match the provenance.                |
| `manifest-predicate-mismatch`              | Signed Statement predicate differs from canonical manifest JSON.       |
| `manifest-digest-mismatch`                 | Statement subject digest differs from canonical manifest JSON bytes.   |
| `missing-producer-provenance`              | Publisher receives an artifact without producer provenance.            |
| `raw-artifact-bypass`                      | Raw caller artifact bypasses producer verification.                    |
| `composition-handoff-substitution`         | Composition mapping trusts public outputs or deterministic names.      |
| `publisher-handoff-field-error`            | Publisher handoff uses missing, stale, or malformed field names.       |
| `sidecar-mismatch`                         | Sidecar bundle does not match the primary asset's provenance.          |
| `duplicate-release-asset`                  | Release asset name already exists.                                     |
| `duplicate-sidecar-asset`                  | Deterministic sidecar asset name already exists before upload.         |
| `registry-linkage-mismatch`                | Published package does not match the provenance registry metadata.     |
| `prepublish-registry-metadata-required`    | Workflow required post-publish registry metadata before publish.       |
| `release-version-semver-mismatch`          | Release manifest version or tag is not valid SemVer 2.0.0.             |
| `trusted-core-boundary-violation`          | Trusted policy/provenance logic depends on profile ecosystem tooling.  |

## Error categories

Each rejected fixture must produce a clear error category. Verifier implementations should surface
the category in a machine-readable way where possible.

## TDD usage

Fixtures drive implementation:

1. Write the failing fixture first.
2. Implement the behavior that makes the accepted fixture pass.
3. Ensure the rejected fixture continues to fail with the correct error category.
4. Add fixtures for any new normative behavior.

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
