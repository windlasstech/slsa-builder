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
  [0062](../decisions/0062-intersect-trusted-producer-policies.md),
  [0063](../decisions/0063-limit-yarn-support-to-berry-v4-with-corepack-package-manager.md),
  [0064](../decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md),
  [0066](../decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [0067](../decisions/0067-converge-repeated-runs-within-run-identity.md),
  [0068](../decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md),
  [0069](../decisions/0069-require-rekor-transparency-and-govern-sigstore-trust-root.md)
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

This section implements ADRs 0037 and 0069. A verifier must use only the following roots; using an
unlisted or ungoverned root fails with `windlass.verify.error.ungoverned-trust-root`:

1. **Sigstore public good root of trust** for the Fulcio CA, Fulcio certificate-transparency log,
   and Rekor log. GitHub's private Sigstore instance and custom Sigstore deployments are outside the
   production policy.
2. **Windlass release manifest** for the mapping of release versions to trusted workflow SHAs,
   `builder.id` values, `buildType` URIs, and the immutable identity expectations represented by the
   verifier-facing manifest expectation schema below.
3. **GitHub** as the hosted runner and OIDC identity provider.

The verifier must obtain the release manifest from a trusted source, such as the signed release
manifest bundle on the GitHub Release page or a mirrored copy whose signature is verified; an
unsigned, self-trusting, or unverifiable manifest fails with
`windlass.verify.error.release-manifest-mismatch`.

### Sigstore trust-root acquisition and freshness

The default trust-root path is the Sigstore public good instance's TUF-distributed trusted root. The
verification tool refreshes and validates TUF metadata before an online verification session and
then performs bundle verification locally. Failure to authenticate current TUF metadata fails with
`windlass.verify.error.ungoverned-trust-root`; the verifier must not fall back to cached component
keys or an environment override.

A pinned `trusted_root.json` is allowed only for offline or reproducible verification under all of
these ADR 0069 freshness rules:

- the verifier policy identifies the pinned file by SHA-256 and records the TUF repository against
  which it was last revalidated; a digest or repository mismatch fails with
  `windlass.verify.error.ungoverned-trust-root`;
- whenever the verification environment is online, the pinned root is revalidated against the TUF
  repository before it is used; failure or refusal to revalidate fails with
  `windlass.verify.error.stale-pinned-trust-root` rather than silently using the pin; and
- a long-lived verification environment documents a refresh schedule and records the next refresh
  deadline. Use after that deadline fails with `windlass.verify.error.stale-pinned-trust-root`.

ADR 0069 deliberately does not select one universal duration for the documented schedule. Therefore
the verifier policy supplies the deadline rather than this specification inventing a global age. A
pin is stale when its policy deadline has passed or when an online run has not revalidated it,
regardless of whether the bundle would otherwise verify.

The documented verification path forbids `SIGSTORE_ROOT_FILE`, `SIGSTORE_REKOR_PUBLIC_KEY`,
`SIGSTORE_CT_LOG_PUBLIC_KEY_FILE`, and semantically equivalent per-component environment overrides.
If any such override can affect root selection, verification fails with
`windlass.verify.error.legacy-trust-root-override`.

## Verifier policy and manifest expectation schemas

This section implements ADRs 0037, 0062, and 0068. These are verifier-input schemas, not a new
standalone CLI interface. Both schemas are closed: an unknown member, a missing required member, a
numeric identifier that does not match `^[1-9][0-9]*$`, or a SHA that is not 40 lowercase
hexadecimal characters fails with `windlass.verify.error.policy-schema-invalid`. Identifiers are
strings because GitHub identifiers are opaque decimal identifiers, not values on which a verifier
performs arithmetic.

The explicit verifier policy has this minimum identity and root shape:

```json
{
  "schema_version": "1",
  "source": {
    "repository_uri": "https://github.com/example/acme-widget",
    "repository_id": "123456789",
    "repository_owner_id": "9876543",
    "digest": "0123456789abcdef0123456789abcdef01234567",
    "ref": "refs/tags/v1.2.3"
  },
  "producer": {
    "workflow_path": ".github/workflows/js-ts-npm-package-slsa3.yml",
    "workflow_sha": "89abcdef0123456789abcdef0123456789abcdef",
    "runner_environment": "github-hosted"
  },
  "trust_root": {
    "mode": "tuf",
    "instance": "sigstore-public-good"
  }
}
```

The explicit policy requires every member shown in the example. Its identity members have these
closed constraints; a violation fails with `windlass.verify.error.policy-schema-invalid`:

| Field                         | Required value form                                                                                      |
| ----------------------------- | -------------------------------------------------------------------------------------------------------- |
| `schema_version`              | Exactly `"1"`.                                                                                           |
| `source.repository_uri`       | Canonical `https://github.com/<owner>/<repository>` URI without userinfo, port, query, or fragment.      |
| `source.repository_id`        | Positive decimal string matching `^[1-9][0-9]*$`; authoritative over the repository name.                |
| `source.repository_owner_id`  | Positive decimal string matching `^[1-9][0-9]*$`; authoritative over the owner name.                     |
| `source.digest`               | Full 40-character lowercase hexadecimal Git commit SHA.                                                  |
| `source.ref`                  | Full `refs/tags/<tag-name>` ref.                                                                         |
| `producer.workflow_path`      | Exact trusted `.github/workflows/<file>.yml` or `.yaml` path, with no traversal or extra path separator. |
| `producer.workflow_sha`       | Full 40-character lowercase hexadecimal called-workflow SHA.                                             |
| `producer.runner_environment` | Exactly `"github-hosted"`.                                                                               |
| `trust_root`                  | Exactly one of the closed TUF or pinned-root shapes specified here.                                      |

For pinned-root operation, `trust_root` instead has this shape. The timestamps are technical JSON
timestamps and therefore use the standard four-digit Gregorian year:

```json
{
  "mode": "pinned",
  "instance": "sigstore-public-good",
  "path": "trusted_root.json",
  "sha256": "7c222fb2927d828af22f592134e8932480637c0d3f9c2072e82716801567e69f",
  "tuf_repository": "https://tuf-repo-cdn.sigstore.dev",
  "revalidated_at": "2026-08-01T00:00:00Z",
  "refresh_before": "2026-08-08T00:00:00Z"
}
```

For `mode: "tuf"`, `instance` must be `"sigstore-public-good"` and no other root member is allowed;
violation fails with `windlass.verify.error.ungoverned-trust-root`. For `mode: "pinned"`, all
pinned-root members shown above are required, `instance` has that same fixed value, `sha256` is 64
lowercase hexadecimal characters, both timestamps use `YYYY-MM-DDTHH:mm:ssZ`, and `refresh_before`
is later than `revalidated_at`. Malformed values fail with
`windlass.verify.error.policy-schema-invalid`; an authenticated but expired or
not-online-revalidated pin fails with `windlass.verify.error.stale-pinned-trust-root`.

The verifier-facing release-manifest expectation schema carries the immutable source identity for
the release manifest signer as well as the selected producer mapping:

```json
{
  "schema_version": "1",
  "release_manifest": {
    "source_repository_uri": "https://github.com/windlasstech/slsa-builder",
    "source_repository_id": "102030405",
    "source_repository_owner_id": "5060708",
    "workflow_path": ".github/workflows/release-manifest.yml",
    "workflow_sha": "89abcdef0123456789abcdef0123456789abcdef"
  },
  "producer_profile": {
    "profile": "js-ts-npm-package",
    "workflow_path": ".github/workflows/js-ts-npm-package-slsa3.yml",
    "workflow_sha": "89abcdef0123456789abcdef0123456789abcdef"
  }
}
```

The release-manifest expectation requires every member shown above. Both numeric-ID fields match
`^[1-9][0-9]*$`, both workflow paths are exact trusted paths, both workflow SHAs are full lowercase
40-hex values, and `producer_profile.profile` is a non-empty policy-registered profile name. A
missing numeric ID, a name in place of an ID, an unknown member, an unregistered profile, or a
malformed path/SHA fails with `windlass.verify.error.policy-schema-invalid` before the manifest can
contribute constraints.

The signed release-manifest payload remains the closed schema defined by
[Release manifest](release-manifest.md); the expectation object above does not add caller-specific
fields to that signed payload. It supplies the numeric identity against which the manifest bundle's
own certificate is checked. Caller/source numeric IDs for a producer bundle come from the explicit
verifier policy or producer-side expected values. This preserves ADR 0062's schema-version-`1` field
scope while satisfying ADR 0068's requirement that every bundle be checked against immutable numeric
identity. ADR 0068 assigns those caller-specific expectations to the explicit verifier policy rather
than the signed release manifest; placing them in the signed manifest v1 instead fails with
`windlass.verify.error.release-manifest-mismatch`.

Invalid examples include the following; each fails before bundle policy evaluation with
`windlass.verify.error.policy-schema-invalid`:

```json
{
  "source": {
    "repository_uri": "https://github.com/example/acme-widget",
    "repository_id": "example/acme-widget",
    "repository_owner_id": -42,
    "digest": "01234567",
    "ref": "v1.2.3"
  }
}
```

```json
{
  "release_manifest": {
    "source_repository_uri": "https://github.com/windlasstech/slsa-builder",
    "source_repository_id": "",
    "source_repository_owner_id": "windlasstech",
    "workflow_path": ".github/workflows/release-manifest.yml",
    "workflow_sha": "main"
  }
}
```

Repository and owner names in URIs are display and routing values. The decimal
`repository_id`/`source_repository_id` and `repository_owner_id`/`source_repository_owner_id` values
are authoritative. A name match cannot compensate for a numeric-ID mismatch; that mismatch fails
with `windlass.verify.error.source-numeric-id-mismatch`.

## Fulcio claims and maximal identity binding

This section implements the identity depth selected by ADR 0068 and the fail-closed deliverable from
ADR 0037. The verifier extracts the signing certificate from the verified bundle and decodes these
Fulcio extensions. Values are UTF-8 strings after X.509 extension decoding; a duplicate, malformed,
or undecodable required extension fails with `windlass.verify.error.signer-identity-claim-missing`.

| Fulcio extension OID     | Fulcio semantic claim                      | GitHub OIDC origin         | Required comparison                                               |
| ------------------------ | ------------------------------------------ | -------------------------- | ----------------------------------------------------------------- |
| `1.3.6.1.4.1.57264.1.1`  | Issuer (deprecated legacy raw-string form) | `iss`                      | Decode old bundles only; never use as authoritative issuer.       |
| `1.3.6.1.4.1.57264.1.8`  | Issuer V2                                  | `iss`                      | DER-decoded issuer exactly matches the trusted GitHub issuer.     |
| `1.3.6.1.4.1.57264.1.9`  | Build Signer URI                           | `job_workflow_ref`         | Exact trusted policy-selected signer workflow URI and path.       |
| `1.3.6.1.4.1.57264.1.10` | Build Signer Digest                        | `job_workflow_sha`         | Full SHA equals the selected manifest/policy `workflow_sha`.      |
| `1.3.6.1.4.1.57264.1.11` | Runner Environment                         | `runner_environment`       | Exactly `github-hosted`.                                          |
| `1.3.6.1.4.1.57264.1.12` | Source Repository URI                      | `repository`               | Exact expected canonical `https://github.com/<owner>/<repo>` URI. |
| `1.3.6.1.4.1.57264.1.13` | Source Repository Digest                   | `sha`                      | Full expected source commit SHA.                                  |
| `1.3.6.1.4.1.57264.1.14` | Source Repository Ref                      | `ref`                      | Exact full expected `refs/tags/...` ref.                          |
| `1.3.6.1.4.1.57264.1.15` | Source Repository Identifier               | `repository_id`            | Exact expected decimal repository ID.                             |
| `1.3.6.1.4.1.57264.1.17` | Source Repository Owner Identifier         | `repository_owner_id`      | Exact expected decimal owner ID.                                  |
| `1.3.6.1.4.1.57264.1.21` | Run Invocation URI                         | `run_id` and `run_attempt` | Well-formed URI for the expected source repository.               |

Issuer V2 OID `.8` is the current authoritative issuer source. A verifier may decode deprecated OID
`.1` for old-bundle diagnostics, but using `.1` instead of `.8` as the authoritative issuer source
fails with `windlass.verify.error.issuer-mismatch`.

The certificate URI SAN carries the called workflow ref. The Build Signer URI and SAN are separate
surfaces and both are checked; equality on one does not excuse absence or mismatch on the other. The
verifier enforces all six bindings below for npm producer, release asset producer, and release
manifest bundles. Every binding is mandatory, and no mismatch is downgraded to a warning.

| Binding | Required check                                                                                                                                                                                                                                                                                                         | Failure diagnostic                                                                                            |
| ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| 1       | Certificate OIDC issuer is exactly `https://token.actions.githubusercontent.com`; URI normalization, redirects, prefixes, and alternate GitHub issuers are not accepted.                                                                                                                                               | `windlass.verify.error.issuer-mismatch`                                                                       |
| 2       | URI SAN and Build Signer URI identify the exact policy-selected signer workflow path, and Build Signer Digest (`job_workflow_sha`) exactly equals the manifest/policy `workflow_sha`. `github.workflow_sha` is the caller SHA and is forbidden as builder identity.                                                    | `windlass.verify.error.signer-workflow-path-mismatch` or `windlass.verify.error.signer-workflow-sha-mismatch` |
| 3       | Source Repository URI equals the expected source URI, and OIDs `.15` and `.17` equal the expected decimal repository and owner IDs. Numeric IDs decide; names are display-only.                                                                                                                                        | `windlass.verify.error.source-identity-mismatch` or `windlass.verify.error.source-numeric-id-mismatch`        |
| 4       | Source Repository Digest equals the expected full source commit SHA and Source Repository Ref equals the expected full release ref.                                                                                                                                                                                    | `windlass.verify.error.source-digest-mismatch` or `windlass.verify.error.source-ref-mismatch`                 |
| 5       | Run Invocation URI is present and has exactly `https://github.com/<owner>/<repo>/actions/runs/<run-id>/attempts/<attempt-number>`, where owner/repo equal the expected source URI and both final components are positive base-10 integers without signs, whitespace, query, fragment, userinfo, or a non-default port. | `windlass.verify.error.run-invocation-uri-invalid`                                                            |
| 6       | Runner Environment OID `.11` is present and its platform-signed value is exactly `github-hosted`. A self-hosted, missing, unknown, or caller-asserted runner value is not accepted.                                                                                                                                    | `windlass.verify.error.self-hosted-runner`                                                                    |

ADR 0062 policy intersection operates over these immutable keys: workflow path plus `workflow_sha`,
source `repository_id`, source `repository_owner_id`, source digest, and source ref. When the
explicit policy and an authenticated release-manifest expectation both constrain one of those keys,
both values apply. An empty intersection fails with
`windlass.verify.error.trusted-producer-policy-conflict`; names never break a tie between different
numeric IDs.

## Bundle, transparency, and signing-time requirements

This section implements ADRs 0037 and 0069. Every Windlass bundle class—npm provenance, release
asset producer provenance, and release manifest—must contain all evidence below. Missing or invalid
evidence rejects the bundle with the listed diagnostic before predicate policy is trusted.

| Evidence                        | Required verification                                                                                                                                                                            | Failure diagnostic                               |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------ |
| Fulcio chain                    | Validate the leaf and chain against the governed Sigstore public good trusted root.                                                                                                              | `windlass.verify.error.signature-mismatch`       |
| Fulcio SCT                      | Verify the certificate's embedded signed certificate timestamp against the governed Fulcio CT log key and certificate.                                                                           | `windlass.verify.error.missing-sct`              |
| Rekor inclusion proof in bundle | Verify the bundle-contained inclusion proof, log entry body, log index/tree material, and Rekor signature against the governed Rekor key; bind the entry to the exact signature and certificate. | `windlass.verify.error.missing-rekor-entry`      |
| SET-covered integrated time     | Verify the Rekor signed entry timestamp and use its covered `integratedTime` as signing time; prove that time lies within the Fulcio leaf certificate validity interval.                         | `windlass.verify.error.signature-time-violation` |
| Optional RFC 3161 TSA timestamp | If present, verify it against the governed root and require consistency with the signature and certificate.                                                                                      | `windlass.verify.error.signature-time-violation` |

The SCT, Rekor inclusion proof, SET, certificate, and signature must be mutually consistent;
inconsistency fails with the narrowest diagnostic above or
`windlass.verify.error.signature-mismatch` when no narrower check applies. A TSA timestamp is
additive evidence only. It never substitutes for the SCT, bundle-contained Rekor proof, SET, or SET
integrated time.

The required verification path must not call Rekor, Fulcio, a CT log, or another log operator.
Attempting or requiring such a call fails conformance with
`windlass.verify.error.verification-network-call`. Fetching TUF metadata when the environment is
online and obtaining artifacts before verification are trust-root acquisition and input-retrieval
steps, not log queries. Once the bundle and governed root are inputs, all cryptographic and policy
checks run offline. Optional online monitoring is outside the pass/fail path and cannot repair a
bundle that lacks required evidence.

## Trusted producer policy conflict resolution

When both an explicit verifier policy and a verified Windlass release manifest policy are present,
the effective trusted producer policy is the intersection of the fields each source explicitly
constrains. A verifier must reject the artifact when any policy source that constrains an observed
field does not allow that observed value.

The signed release manifest policy is eligible for this intersection only after the manifest bundle
passes the release manifest verification policy below. The manifest must not define or relax its own
trust root, signer identity, predicate type, schema version, or authority to override explicit
verifier policy.

For schema version `1`, the signed release manifest constrains only the fields it represents:

| Policy field class                 | Manifest v1 constraint source                                                               |
| ---------------------------------- | ------------------------------------------------------------------------------------------- |
| Windlass release identity          | `release_version`, `release_tag`, `release_commit_sha`, and release manifest signer checks. |
| Producer workflow path and SHA     | `producer_profiles[].workflow_path` and `producer_profiles[].workflow_sha`.                 |
| Producer `builder.id`              | `producer_profiles[].builder_id`.                                                           |
| Producer `buildType`               | `producer_profiles[].build_type`.                                                           |
| Publisher workflow path, SHA, role | `publisher_workflows[].workflow_path`, `workflow_sha`, and `role`.                          |

The following fields remain mandatory for the relevant npm, publisher, or consumer verification
surface, but they are not constrained by the schema version `1` release manifest and must be
supplied by explicit verifier policy, producer-side expected values, the digest-verified handoff, or
another ADR-backed policy source:

| Required verification field class                                       | Required non-manifest source                                                                                               |
| ----------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Producer signer identity beyond manifest mappings                       | Explicit verifier policy or profile-owned producer policy.                                                                 |
| Caller/source repository URI and immutable numeric repository/owner IDs | Explicit verifier policy or producer-side expected data; certificate OIDs `.12`, `.15`, and `.17` provide observed values. |
| Caller/source ref and revision                                          | Explicit verifier policy, signed producer `externalParameters`, and producer-side expected data.                           |
| Producer release ref and version tag                                    | Explicit verifier policy, signed producer `externalParameters`, and digest-verified handoff.                               |
| Subject name and subject digest                                         | Explicit verifier policy, profile subject rules, and downloaded artifact bytes.                                            |
| Strict `externalParameters` requirements                                | Profile verifier policy and signed producer Statement schema.                                                              |
| Release asset filename to producer artifact binding                     | Producer policy and digest-verified publisher handoff fields.                                                              |

If a field is absent because a policy source's schema does not represent it, that field is
unconstrained by that source. The absence is not affirmative permission and does not remove a check
required by another spec. If a field required by a policy source's own schema is missing from that
source, the policy source is invalid and verification must fail closed.

When two sources explicitly constrain the same field and their constraints conflict or their
intersection is empty, verification must fail closed. A verifier must not use precedence,
last-writer-wins behavior, or either-source-allowed behavior in the default production policy. The
error diagnostic must name the conflicting source and field; omission fails the diagnostics contract
with `windlass.verify.error.diagnostics-contract-invalid`.

## npm package verification policy

For an npm package published by the JS/TS npm profile, the bundle must first pass the maximal
identity, offline transparency, signing-time, and trust-root sections above; failure rejects the
package with the narrow diagnostic from those sections. The verifier then must check:

1. The package tarball is the artifact being verified.
2. The provenance bundle is the Windlass-signed bundle for the tarball.
3. The bundle signature is valid and the signer identity is trusted.
4. The `predicateType` is `https://slsa.dev/provenance/v1`.
5. The `builder.id` is in the trusted release manifest and uses a full SHA.
6. The `buildType` is `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`.
7. The `subject[0].name` matches the expected npm Package URL for the package version.
8. The `subject[0].digest.sha512` matches the tarball bytes.
9. The `subject[0].digest.sha256` matches the tarball bytes.
10. The `externalParameters` include the expected source repository, ref, commit, package directory,
    package identity, package manager, runner, publish intent, release ref, and build-script result.
11. The signer identity is the Windlass reusable workflow identity, while the source repository in
    `externalParameters.source.repository` is the caller package repository.
12. No unexpected `externalParameters` are present under strict matching.
13. The selected package is not marked `private: true`.
14. For `https://registry.npmjs.org/`, the package version was not already present before publish
    and the selected package identity already existed before publish; first publication of a package
    identity is outside the initial npmjs trusted-publishing-only profile.
15. For non-npmjs registries, package identity and package version preflight fields are best-effort
    diagnostics unless a later ADR defines that registry class. Consumer-side verification must not
    report those diagnostics as Windlass-guaranteed registry support.
16. `externalParameters.package.package_url` equals the registry package-version URL reconstructed
    from `externalParameters.publish.resolved_registry_url`, `externalParameters.package.name`, and
    `externalParameters.package.version`. It must not be a Package URL (`pkg:npm/...`).
17. `externalParameters.source.ref`, `externalParameters.release.ref`, and the accepted runtime ref
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

When `externalParameters.package_manager.name` is `yarn`, verifier policy must additionally require:

- `externalParameters.package_manager.selection_source` is `packageManager`;
- `externalParameters.package_manager.version` is an exact SemVer version greater than or equal to
  `4.0.0`;
- `externalParameters.package_manager.yarn_install_mode` is `immutable`;
- `resolvedDependencies[0].annotations.selection_lockfile_path` identifies the selected `yarn.lock`;
- no policy accepts Yarn Classic 1.x, Yarn Berry v2, Yarn Berry v3, `devEngines.packageManager`-only
  Yarn selection, lockfile-only Yarn inference, ambient global Yarn, or Corepack Known Good Release
  fallback.

## GitHub Release asset verification policy

For a release asset uploaded by the publisher, the unchanged producer bundle must first pass the
maximal identity, offline transparency, signing-time, and trust-root sections above; failure rejects
the asset with the narrow diagnostic from those sections. The verifier then must check:

1. The downloaded release asset is the primary artifact.
2. The provenance sidecar is present and is the unchanged producer bundle.
3. The producer bundle signature is valid and the signer identity is trusted.
4. The producer `predicateType` is `https://slsa.dev/provenance/v1`.
5. The producer `builder.id` and `buildType` are in the trusted policy.
6. The producer `subject[0].name` matches the expected producer subject under the selected producer
   policy. For the initial npm composition, this is the npm Package URL, not the release asset name.
7. The selected producer policy binds the release asset name to the verified producer artifact. For
   the initial npm composition, the release asset name must match the pack-produced tarball filename
   recorded in producer provenance and handoff fields.
8. The producer `subject[0].digest.sha256` matches the downloaded asset bytes.
9. The publisher did not generate or re-sign the provenance.

The publisher itself does not have a source-to-artifact `buildType` in the default path.

## Release manifest verification policy

For a release manifest, its bundle must first pass the maximal identity, offline transparency,
signing-time, and trust-root sections above using the release-manifest expectation object; failure
rejects the manifest with the narrow diagnostic from those sections. The verifier then must check:

1. The signed bundle signature is valid.
2. The signer identity is the GitHub Actions OIDC identity for
   `.github/workflows/release-manifest.yml` in `windlasstech/slsa-builder` on the expected full
   release tag ref.
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
13. The certificate Source Repository Identifier and Source Repository Owner Identifier equal the
    numeric IDs in the release-manifest expectation object; matching `windlasstech/slsa-builder`
    names cannot override an ID mismatch.
14. The Run Invocation URI is present and well-formed, and the platform-signed runner environment is
    GitHub-hosted.

Consumer-side release manifest verification does not require offline proof that GitHub tag
protection was enabled at signing time. The required consumer check is that the verified signer
workflow ref, manifest `release_tag`, and verifier-expected tag are the same full `refs/tags/...`
ref and that the tag peels to `release_commit_sha`. GitHub tag protection, branch protection, and
repository ruleset evidence are release-process controls and complementary evidence unless a later
ADR-backed verifier policy defines an online policy-evidence source, observation time, freshness
window, and cache semantics.

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

### Complete verification procedure

This procedure implements ADRs 0037, 0062, 0068, and 0069. There is currently no off-the-shelf
command that checks Fulcio's numeric repository-ID extensions. In particular, a successful
`gh attestation verify` command is necessary but insufficient. The current procedure is GitHub CLI
verification plus machine-readable JSON and X.509 post-processing; the future trusted Go
implementation path is `sigstore-go`, which can verify the bundle and expose the certificate claims
without changing this policy.

Run the steps in this order:

1. Parse the explicit verifier policy, manifest expectation, signed release manifest, and bundle
   with duplicate-member rejection. Validate their closed schemas. A parse or schema failure stops
   evaluation with `windlass.verify.error.duplicate-json-member` or
   `windlass.verify.error.policy-schema-invalid`; signed fields are not evaluated after ambiguous
   parsing.
2. Resolve the trust root. Use TUF by default, or authenticate the pinned `trusted_root.json` and
   enforce its freshness record. Root failure stops evaluation with the applicable trust-root
   diagnostic; no signature or predicate claim is trusted after that failure.
3. Verify the local artifact and local bundle with `gh`, requesting JSON. Supply all constraints the
   command supports rather than relying on tool defaults. For example:

   ```bash
   gh attestation verify "$ARTIFACT" \
     --bundle "$BUNDLE" \
     --repo "$SOURCE_REPOSITORY" \
     --signer-workflow "$SIGNER_WORKFLOW" \
     --cert-oidc-issuer https://token.actions.githubusercontent.com \
     --predicate-type "$PREDICATE_TYPE" \
     --format json >gh-verified.json
   ```

   For pinned-root verification, add `--custom-trusted-root trusted_root.json` only after the
   freshness checks above. The command consumes the supplied bundle; it must not discover or repair
   Rekor material online. A nonzero result fails with the corresponding signature, transparency,
   identity, or input diagnostic rather than continuing to predicate checks.

4. Parse `gh-verified.json` as untrusted machine-readable data and require a successful result for
   the exact artifact digest and bundle supplied in step 3. Extract the leaf certificate from the
   verified bundle, decode the URI SAN and the OIDs in the Fulcio table above, and emit those values
   into the post-processor's typed JSON model. Missing, duplicate, or malformed certificate values
   fail with `windlass.verify.error.signer-identity-claim-missing`.
5. In the JSON post-processor, compare the exact issuer; SAN and Build Signer URI; Build Signer
   Digest; source URI, numeric repository IDs, digest, and ref; Run Invocation URI; and runner
   environment against the policy inputs. A mismatch fails with the diagnostic assigned to that
   binding. Do not read `github.workflow_sha`, workflow logs, unsigned outputs, or artifact names to
   fill a missing certificate claim.
6. Verify the bundle-contained SCT, Rekor inclusion proof, SET, and SET-covered integrated time
   against the governed root. This step uses only local bundle and root bytes and fails with the
   applicable transparency or signing-time diagnostic. An RFC 3161 timestamp, when present, is
   verified as additional evidence and cannot make missing Rekor or SCT evidence pass.
7. Authenticate the release manifest bundle by repeating steps 3–6 with the release-manifest
   predicate type and manifest identity expectations. Only after it passes may its represented
   fields enter the ADR 0062 policy intersection. An unauthenticated manifest fails with
   `windlass.verify.error.release-manifest-mismatch` and contributes no policy values.
8. Compute the field-by-field intersection of authenticated manifest expectations and explicit
   policy, then validate Statement structure, `predicateType`, `builder.id`, `buildType`, strict
   `externalParameters`, subject identity, and artifact digests. A conflict fails with
   `windlass.verify.error.trusted-producer-policy-conflict`; semantic or digest failures use the
   narrower registered diagnostic.
9. Serialize the diagnostic report defined below. Acceptance occurs only when every required check
   passed and the report has no `error` diagnostic. Warnings remain visible but do not convert a
   failure to a pass.

The JSON post-processor may be a small consumer-owned program during the initial profile, but its
comparisons and output must conform to this document or its result is not Windlass policy
verification. `jq` may validate ordinary JSON values, but it is not by itself an X.509 extension
decoder. The post-processor must use an X.509 parser for certificate extensions; treating a rendered
certificate dump or log text as authoritative fails with
`windlass.verify.error.signer-identity-claim-missing`.

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

## Diagnostics contract

This section specifies the conformance diagnostics required by ADRs 0037 and 0062. It does not
stabilize a standalone verifier CLI, which remains a future ADR decision. Producer gates, fixture
harnesses, and consumer implementations claiming conformance must apply this contract; an output
that cannot represent it fails the fixture harness with
`windlass.verify.error.diagnostics-contract-invalid`.

### Stable diagnostic IDs

Every diagnostic ID has the closed form `windlass.verify.<severity>.<category>`, where `<severity>`
is `error` or `warning` and `<category>` is a lowercase kebab-case stable check name.
Rejected-fixture categories in the registry below map to
`windlass.verify.error.<expected-failure-category>`. The ID, not the human message, is the stable
machine key. Implementations may translate `message`, but they must not rename, reuse, or
dynamically construct a different ID for the same registered check; doing so fails with
`windlass.verify.error.diagnostics-contract-invalid`.

The non-fatal warning IDs initially registered by this specification are:

| Diagnostic ID                                                    | Meaning                                                                                |
| ---------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| `windlass.verify.warning.stale-non-selected-lockfile`            | A supported but non-selected lockfile was recorded; verification remains valid.        |
| `windlass.verify.warning.custom-registry-preflight-inconclusive` | Non-npmjs metadata preflight was inconclusive under the documented best-effort policy. |
| `windlass.verify.warning.native-provenance-locator-missing`      | Optional native provenance locator is absent while the required sidecar verifies.      |

No identity, root, signature, transparency, signing-time, policy-intersection, Statement, or digest
failure has a warning form. Emitting one of those failures as a warning fails the diagnostics
contract and the verification result remains rejected.

### Machine-readable serialization

The report is one UTF-8 JSON object. It is parsed with duplicate-member rejection and has this
closed shape:

| Member           | Type                        | Contract                                                                                  |
| ---------------- | --------------------------- | ----------------------------------------------------------------------------------------- |
| `schema_version` | string                      | Exactly `"1"`.                                                                            |
| `result`         | string                      | `pass` or `fail`; `fail` exactly when at least one `error` exists.                        |
| `exit_code`      | integer                     | Equals the exit-code table below.                                                         |
| `primary_id`     | string or `null`            | First ordered error ID, or `null` when no error exists.                                   |
| `run_invocation` | string or `null`            | Verified Run Invocation URI, or `null` only when verification could not authenticate one. |
| `diagnostics`    | array of diagnostic objects | Deterministically ordered as specified below.                                             |

Each diagnostic object has required `id`, `severity`, `category`, `check`, and `message` string
members. It may also have `field`, `expected`, `actual`, `policy_sources`, and `evidence` members.
`expected` and `actual` are typed JSON values, not interpolated message fragments. `policy_sources`
is an array containing only `explicit-policy`, `release-manifest`, `producer-expected-value`, or
`digest-verified-handoff`. `evidence` contains only non-secret local identifiers such as an OID,
bundle path, artifact digest, or certificate fingerprint. Unknown members, secret/token values, or
an `actual` value copied from a secret fail with
`windlass.verify.error.diagnostics-contract-invalid`.

Valid warning-only example:

```json
{
  "schema_version": "1",
  "result": "pass",
  "exit_code": 0,
  "primary_id": null,
  "run_invocation": "https://github.com/example/acme-widget/actions/runs/123456789/attempts/2",
  "diagnostics": [
    {
      "id": "windlass.verify.warning.stale-non-selected-lockfile",
      "severity": "warning",
      "category": "stale-non-selected-lockfile",
      "check": "externalParameters.package_manager.ignored_lockfile_paths",
      "message": "A non-selected pnpm lockfile was recorded.",
      "field": "externalParameters.package_manager.ignored_lockfile_paths",
      "actual": ["pnpm-lock.yaml"],
      "evidence": { "bundle": "package.tgz.intoto.jsonl" }
    }
  ]
}
```

Valid failed-verification example:

```json
{
  "schema_version": "1",
  "result": "fail",
  "exit_code": 1,
  "primary_id": "windlass.verify.error.source-numeric-id-mismatch",
  "run_invocation": "https://github.com/example/acme-widget/actions/runs/123456789/attempts/2",
  "diagnostics": [
    {
      "id": "windlass.verify.error.source-numeric-id-mismatch",
      "severity": "error",
      "category": "source-numeric-id-mismatch",
      "check": "fulcio.1.3.6.1.4.1.57264.1.15",
      "message": "The source repository identifier does not match policy.",
      "field": "source.repository_id",
      "expected": "123456789",
      "actual": "222222222",
      "policy_sources": ["explicit-policy"],
      "evidence": {
        "oid": "1.3.6.1.4.1.57264.1.15",
        "certificate_sha256": "7c222fb2927d828af22f592134e8932480637c0d3f9c2072e82716801567e69f"
      }
    }
  ]
}
```

Invalid examples include `result: "pass"` with an error, exit code `0` with `result: "fail"`, a
warning ID whose `severity` is `error`, diagnostics in nondeterministic order, a missing
`primary_id` for a failed result, duplicate JSON members, or a token in `evidence`. Each is a
`windlass.verify.error.diagnostics-contract-invalid` fixture-harness failure.

### Diagnostic precedence and ordering

Checks and diagnostics use this precedence, from highest to lowest:

1. input availability, byte digest, strict JSON parsing, and schema;
2. trust-root governance and freshness;
3. certificate chain, SCT, Rekor inclusion, SET, and signing time;
4. issuer, workflow, source numeric identity, source content, run invocation, and runner identity;
5. authenticated-policy intersection;
6. Statement, predicate, `builder.id`, `buildType`, and `externalParameters` semantics;
7. package, manifest, handoff, publisher, and registry surface checks; and
8. warnings.

Within a precedence level, diagnostics sort by `id`, then `field` (missing `field` sorts as the
empty string), then canonical JSON serialization of `actual`. `primary_id` is the first ordered
error. Implementations report every independently provable diagnostic that is safe to evaluate, but
they must not derive secondary diagnostics from unauthenticated or ambiguously parsed content. A
duplicate-member parse error therefore prevents semantic diagnostics for that JSON value; a failed
manifest authentication prevents manifest-intersection diagnostics; and a missing certificate
prevents extension mismatch diagnostics. Ordering or suppression contrary to these rules fails the
fixture harness with `windlass.verify.error.diagnostics-contract-invalid`.

### Exit codes and warning behavior

| Exit code | Class              | Required result and behavior                                                                                                                       |
| --------- | ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `0`       | Verified           | `result` is `pass`; zero or more registered `warning` diagnostics may be present.                                                                  |
| `1`       | Verification error | `result` is `fail`; at least one registered policy, cryptographic, identity, schema, artifact, or fixture-check error exists.                      |
| `2`       | Invocation failure | `result` is `fail`; required local input is unreadable, an option/policy document is unusable, or the verifier cannot execute the requested check. |

Exit code `2` does not mean the artifact failed a completed cryptographic check; it means no valid
acceptance decision was produced and the report contains `windlass.verify.error.input-unavailable`
or `windlass.verify.error.verifier-execution-failure` as its primary diagnostic. Producer-side
publication gates treat both `1` and `2` as fatal and stop before mutation. Consumer automation
treats both as non-acceptance. Warnings are non-fatal only with exit code `0`, remain structurally
distinguishable through `severity: "warning"`, and must not hide, replace, or lower the exit status
of any error. Violating this mapping fails the fixture harness with
`windlass.verify.error.diagnostics-contract-invalid`.

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
| `npm-maximal-identity-valid`              | npm              | All six ADR 0068 identity bindings match immutable policy keys.   |
| `npm-offline-transparency-valid`          | npm              | SCT, Rekor proof, SET, and signing time verify without log calls. |
| `release-manifest-pinned-root-valid`      | release-manifest | Authenticated fresh pin verifies the manifest bundle offline.     |

## Rejected fixture categories

| Category                                   | Description                                                                                 |
| ------------------------------------------ | ------------------------------------------------------------------------------------------- |
| `digest-mismatch`                          | Artifact digest does not match the provenance subject digest.                               |
| `signature-mismatch`                       | Bundle signature is invalid or missing.                                                     |
| `signer-mismatch`                          | Signer identity is not trusted.                                                             |
| `issuer-mismatch`                          | Certificate OIDC issuer is not the exact GitHub Actions issuer.                             |
| `signer-workflow-path-mismatch`            | SAN or Build Signer URI does not identify the exact expected workflow path.                 |
| `signer-workflow-sha-mismatch`             | Build Signer Digest does not equal the manifest/policy workflow SHA.                        |
| `signer-identity-claim-missing`            | Required semantic signer or source identity cannot be proven from verified bundle data.     |
| `source-numeric-id-mismatch`               | Source repository or owner numeric ID is missing, malformed, or mismatched.                 |
| `source-digest-mismatch`                   | Certificate Source Repository Digest differs from the expected source commit.               |
| `source-ref-mismatch`                      | Certificate Source Repository Ref differs from the expected full ref.                       |
| `run-invocation-uri-invalid`               | Run Invocation URI is missing, malformed, or identifies another repository.                 |
| `self-hosted-runner`                       | Runner identity is missing, unknown, caller-asserted, or not GitHub-hosted.                 |
| `missing-rekor-entry`                      | Bundle lacks a valid bundle-contained Rekor inclusion proof or SET binding.                 |
| `missing-sct`                              | Fulcio certificate lacks a valid embedded SCT.                                              |
| `signature-time-violation`                 | SET-covered integrated time is invalid or outside certificate validity.                     |
| `ungoverned-trust-root`                    | Trust root is not the authenticated Sigstore public good TUF root or allowed pin.           |
| `stale-pinned-trust-root`                  | Pinned trusted root missed online revalidation or its documented refresh deadline.          |
| `legacy-trust-root-override`               | A forbidden per-component Sigstore environment override can affect verification.            |
| `verification-network-call`                | Required verification attempts to query Rekor, Fulcio, or a log operator.                   |
| `policy-schema-invalid`                    | Explicit policy or manifest expectation is missing, malformed, or contains unknown fields.  |
| `diagnostics-contract-invalid`             | Machine-readable diagnostics violate the ID, shape, order, severity, or exit-code contract. |
| `input-unavailable`                        | A required local artifact, bundle, policy, manifest, or trusted-root input is unreadable.   |
| `verifier-execution-failure`               | The verifier cannot execute a requested check and therefore produces no acceptance result.  |
| `duplicate-json-member`                    | Signed Statement, bundle, or DSSE JSON contains duplicate object member names.              |
| `actions-attest-adapter-contract`          | Adapter inputs, emitted bundle basename, or npm provenance-file compatibility is invalid.   |
| `wrong-producer-signer`                    | Producer signer repo, workflow path, ref, or issuer is not trusted.                         |
| `wrong-predicate-type`                     | `predicateType` is not SLSA provenance v1.                                                  |
| `wrong-manifest-predicate-type`            | Release manifest `predicateType` is not the ADR 0054 predicate URI.                         |
| `wrong-builder-id`                         | `builder.id` is not trusted or uses a non-SHA reference.                                    |
| `wrong-build-type`                         | `buildType` is not the canonical profile URI.                                               |
| `subject-cardinality-error`                | Provenance contains zero subjects or multiple subjects.                                     |
| `npm-purl-subject-mismatch`                | npm provenance subject is missing, malformed, or not the expected Package URL.              |
| `tarball-filename-subject-rejected`        | npm provenance uses the tarball filename as the Statement subject.                          |
| `missing-subject-sha512`                   | npm provenance subject omits the required tarball SHA-512 digest.                           |
| `missing-subject-sha256`                   | Provenance subject omits the required tarball SHA-256 digest.                               |
| `unexpected-external-parameters`           | `externalParameters` contains unexpected fields under strict matching.                      |
| `source-identity-mismatch`                 | Source repository or revision does not match policy.                                        |
| `release-ref-mismatch`                     | Source ref, release ref, runtime ref, or version tag do not identify the same tag.          |
| `source-repository-canonicalization-error` | Source repository URL is non-canonical, ambiguous, or malformed.                            |
| `trusted-publisher-mismatch`               | Producer-side npm trusted publishing caller identity or OIDC permission is wrong.           |
| `package-identity-mismatch`                | npm package name or version does not match.                                                 |
| `package-url-mismatch`                     | npm registry package-version URL is malformed or does not match registry/name/version.      |
| `unsupported-initial-publication`          | Selected package identity does not already exist on npmjs.                                  |
| `package-version-mismatch`                 | Tag version does not match `package.json` version.                                          |
| `package-directory-mismatch`               | `externalParameters.package.directory` does not match expected.                             |
| `package-manager-selection-path-mismatch`  | Package-manager selection path is missing or wrong in provenance.                           |
| `private-package`                          | Selected package manifest has `private: true`.                                              |
| `publish-intent-conflict`                  | Workflow publish input conflicts with source `publishConfig`.                               |
| `invalid-publish-input`                    | Non-empty workflow publish input has an unsupported value or format.                        |
| `empty-publish-input-fallback`             | Empty workflow input failed to fall back to source `publishConfig`.                         |
| `already-published-version`                | Selected package name/version already exists before publish.                                |
| `workspace-resolution-mismatch`            | Workspace root, package manager root, or lockfile policy is wrong.                          |
| `workspace-pattern-base-mismatch`          | Workspace patterns were evaluated against the wrong base directory.                         |
| `workspace-command-mismatch`               | Workspace package targeting command can affect the wrong package.                           |
| `package-manager-manifest-shape-error`     | `devEngines.packageManager` uses an unsupported shape, member, or release version form.     |
| `unsupported-yarn-version`                 | Yarn is Classic 1.x, Berry v2, Berry v3, non-exact, or selected from an unsupported source. |
| `resolved-dependencies-lockfile`           | Selected lockfile `resolvedDependencies` descriptor is missing, malformed, or mismatched.   |
| `release-asset-mode-schema-error`          | Public npm release-asset mode input or output schema is invalid.                            |
| `release-asset-mode-disabled-conflict`     | Release-asset-only inputs are supplied while release-asset mode is disabled.                |
| `release-asset-mode-permission-error`      | Caller or internal job permissions are missing or combine separated authorities.            |
| `release-asset-target-error`               | Effective release tag or target release is missing, malformed, or outside the caller repo.  |
| `runtime-policy-mismatch`                  | Runner or Node.js version does not match policy.                                            |
| `excessive-publish-permission`             | npmjs publish job requests permissions outside the initial boundary.                        |
| `npm-version-too-old`                      | npm CLI version is below `11.5.1` for trusted publishing.                                   |
| `release-manifest-mismatch`                | Release manifest mapping does not match the provenance.                                     |
| `trusted-producer-policy-conflict`         | Explicit verifier policy and signed release manifest policy cannot both be satisfied.       |
| `manifest-predicate-mismatch`              | Signed Statement predicate differs from canonical manifest JSON.                            |
| `manifest-digest-mismatch`                 | Statement subject digest differs from canonical manifest JSON bytes.                        |
| `manifest-trigger-mismatch`                | Release manifest workflow did not run from the expected protected SemVer tag.               |
| `manifest-tag-peel-mismatch`               | Release tag cannot be peeled to the expected terminal commit.                               |
| `manifest-entrypoint-mismatch`             | Release manifest signer workflow path is not the fixed production entrypoint.               |
| `manifest-caller-override`                 | Caller-controlled input changed a signed manifest trust field.                              |
| `manifest-workflow-sha-mismatch`           | Schema v1 workflow SHA does not equal the release tag target commit.                        |
| `manifest-entry-order-mismatch`            | Release manifest producer or publisher arrays are not in canonical sorted order.            |
| `manifest-generated-at-invalid`            | `generated_at` is not a fixed-form UTC timestamp.                                           |
| `manifest-handoff-basename-mismatch`       | Manifest handoff artifact contains an unexpected payload basename.                          |
| `manifest-signing-input-mismatch`          | Manifest signing input metadata is malformed or does not bind verified signing inputs.      |
| `manifest-partial-json-uploaded`           | Plain manifest JSON uploaded but signed bundle upload failed.                               |
| `manifest-indeterminate-json-upload`       | Manifest upload state cannot be determined after an ambiguous upload attempt.               |
| `manifest-remote-digest-unproven`          | Same-name remote manifest asset exists but SHA-256 equality cannot be proven.               |
| `bundle-byte-format-mismatch`              | Signed bundle bytes were extracted, reserialized, wrapped, or otherwise changed.            |
| `missing-producer-provenance`              | Publisher receives an artifact without producer provenance.                                 |
| `raw-artifact-bypass`                      | Raw caller artifact bypasses producer verification.                                         |
| `handoff-schema-mismatch`                  | Cross-job artifact handoff omits or changes required core fields.                           |
| `composition-handoff-substitution`         | Composition mapping trusts public outputs or deterministic names.                           |
| `publisher-handoff-field-error`            | Publisher handoff uses missing, stale, or malformed field names.                            |
| `release-asset-binding-mismatch`           | Producer policy cannot bind release asset name to verified producer artifact bytes.         |
| `linked-artifact-settings-mismatch`        | Linked artifact settings do not match the target repository, release tag, or download URL.  |
| `linked-artifact-locator-mismatch`         | Linked artifact locator outputs are missing, set in the wrong state, or malformed.          |
| `publisher-workflow-schema-error`          | Publisher exposes or accepts unsupported public workflow inputs or secrets.                 |
| `publisher-permission-boundary-violation`  | Publisher job permissions combine authorities that must stay separate.                      |
| `native-locator-malformed`                 | Native provenance locator is not valid diagnostic metadata.                                 |
| `native-locator-digest-mismatch`           | Native provenance locator digest differs from the sidecar bundle digest.                    |
| `sidecar-mismatch`                         | Sidecar bundle does not match the primary asset's provenance.                               |
| `sidecar-digest-mismatch`                  | `sidecar-digest` does not equal the verified producer bundle SHA-256.                       |
| `sidecar-upload-partial-failure`           | Primary release asset uploaded but sidecar upload failed afterward.                         |
| `publisher-indeterminate-primary-upload`   | Primary release asset upload state cannot be determined after an ambiguous upload.          |
| `publisher-remote-digest-unproven`         | Same-name remote release asset exists but SHA-256 equality cannot be proven.                |
| `duplicate-release-asset`                  | Release asset name already exists.                                                          |
| `duplicate-sidecar-asset`                  | Deterministic sidecar asset name already exists before upload.                              |
| `registry-linkage-mismatch`                | Published package does not match the provenance registry metadata.                          |
| `custom-registry-preflight-diagnostic`     | Non-npmjs registry preflight metadata is best-effort and not guaranteed support.            |
| `custom-registry-token-required`           | Custom registry metadata or publish path requires token or weaker provenance behavior.      |
| `custom-registry-provenance-weakened`      | Custom registry publish omits, rewrites, re-signs, or substitutes the Windlass bundle.      |
| `prepublish-registry-metadata-required`    | Workflow required post-publish registry metadata before publish.                            |
| `release-version-semver-mismatch`          | Release manifest version or tag is not valid SemVer 2.0.0.                                  |
| `trusted-core-boundary-violation`          | Trusted policy/provenance logic depends on profile ecosystem tooling.                       |

## Error categories

Each rejected fixture must produce the stable `windlass.verify.error.<expected-failure-category>` ID
and machine-readable report defined by the diagnostics contract. A missing, warning-only,
differently named, or text-only result fails the fixture harness with
`windlass.verify.error.diagnostics-contract-invalid`.

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
the public `package-url` output and `externalParameters.package.package_url`, and that the
reconstructed URL matches `externalParameters.publish.resolved_registry_url`,
`externalParameters.package.name`, and `externalParameters.package.version`.

The npm provenance subject fixture set must prove ADR 0064's Package URL subject contract. Accepted
fixtures must include unscoped and scoped package subjects such as `pkg:npm/left-pad@1.3.0` and
`pkg:npm/%40windlass/slsa-builder@1.2.3`, with `subject[0].digest.sha512` and
`subject[0].digest.sha256` both matching the same tarball bytes. Rejected fixtures must cover a
tarball-filename subject, malformed PURL, package name or version mismatch, missing `sha512`,
missing `sha256`, SHA-512 mismatch, SHA-256 mismatch, zero subjects, multiple subjects, and a PURL
incorrectly placed in the public `package-url` output or `externalParameters.package.package_url`.
These failures use `npm-purl-subject-mismatch`, `tarball-filename-subject-rejected`,
`missing-subject-sha512`, `missing-subject-sha256`, `digest-mismatch`, `package-url-mismatch`, or
`subject-cardinality-error` as applicable.

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

The custom-registry fixture set must also prove the minimum no-secret external-provenance contract.
Accepted fixtures may use a non-npmjs HTTPS registry only when publish can run without
publish-capable secrets and submits the exact verified Windlass bundle unchanged. Rejected fixtures
must cover a custom registry that requires npm automatic provenance, drops the
`--provenance-file`/equivalent external provenance input, rewrites or re-signs the bundle, omits the
bundle, requires token credentials, or silently drops a non-empty caller `access` value in order to
continue. Bundle weakening failures use `custom-registry-provenance-weakened`; credential or token
fallback failures use `custom-registry-token-required`.

The workspace fixture set must include nested workspace roots and prove that workspace patterns are
evaluated relative to each candidate workspace root, not relative to the repository root. A fixture
whose pattern only matches under the wrong base path must fail with
`workspace-pattern-base-mismatch`.

The workspace fixture set must prove the initial limited glob semantics. Accepted fixtures must
cover `*` matching exactly one path segment, `**` matching one or more nested segments, `**`
matching zero segments only when the selected package directory is the candidate root and contains
`package.json`, nested workspace roots where the nearest claiming ancestor wins, and multiple
patterns that resolve to the same selected package directory. Rejected fixtures must cover
`packages/*` incorrectly matching `packages/a/b`, a pattern matching a descendant or ancestor rather
than the exact selected package directory, a selected package claimed by workspace metadata but
missing its own `package.json`, patterns that resolve to different package directories for one
input, malformed `pnpm-workspace.yaml`, unsupported `workspaces` shapes, negation, brace expansion,
extended glob syntax, absolute paths, empty path segments, traversal segments, and backslash
separators. Pattern base failures use `workspace-pattern-base-mismatch`; malformed metadata or
non-exact package selection failures use `workspace-resolution-mismatch` unless a narrower category
applies.

The package-manager manifest fixture set must prove that top-level `packageManager` uses the
`name@version` descriptor form while `devEngines.packageManager` uses the closed object form
accepted by the JS/TS npm build and pack spec. Accepted fixtures must include exact pnpm versions in
`devEngines.packageManager.version` and exact Yarn Berry v4+ versions in top-level `packageManager`.
Rejected fixtures must cover string-form `devEngines.packageManager`, array-form
`devEngines.packageManager`, unknown object members, missing pnpm versions, range versions, tag
versions, URL descriptors, hash-suffixed descriptors, and `onFail: "ignore"` or `onFail: "warn"`
attempts that would otherwise weaken release-build policy. These failures use
`package-manager-manifest-shape-error` unless a narrower package-manager selection, Yarn support, or
lockfile category applies.

The Yarn support fixture set must prove ADR 0063's stable boundary. Accepted fixtures must cover a
root package and workspace package selected by top-level exact `packageManager` values such as
`yarn@4.0.0` or newer, with `yarn.lock`, Corepack exact-version execution, and
`package_manager.yarn_install_mode: "immutable"` in provenance. Rejected fixtures must cover
`yarn@1.x`, `yarn@2.x`, `yarn@3.x`, Yarn version ranges, Yarn tags, Yarn URL descriptors,
hash-suffixed Yarn descriptors, missing `packageManager` with only `yarn.lock`, Yarn selected only
by `devEngines.packageManager`, Corepack Known Good Release fallback, and ambient global Yarn
execution. Unsupported Yarn generation, descriptor, or selection-source failures use
`unsupported-yarn-version` unless the failure is more specifically a malformed manifest shape,
lockfile mismatch, or Corepack enforcement error.

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

The signer identity fixture set must prove semantic GitHub Actions identity binding rather than
artifact-name or log-based inference. Accepted npm producer fixtures must show a bundle whose signer
workflow repository, workflow path, workflow SHA, source repository, source ref, source revision,
numeric repository and owner IDs, Run Invocation URI, GitHub-hosted runner identity, and predicate
type all match the signed Statement and trusted policy. Rejected npm fixtures must cover a wrong
reusable workflow path, a signer workflow SHA that differs from both `runDetails.builder.id` and the
manifest `workflow_sha`, a correct Windlass signer with a mismatched caller source repository, a
name-matching source repository whose repository or owner numeric ID differs, a correct caller
source identity with an untrusted Windlass signer, a missing or malformed Run Invocation URI, a
self-hosted runner, and a tool output that omits a required semantic identity field. Accepted
release manifest fixtures must show signer repository, signer workflow path, protected tag ref,
peeled release commit SHA, source repository, numeric repository and owner IDs, source ref, source
revision, Run Invocation URI, GitHub-hosted runner identity, and predicate type matching the
manifest expectation. Rejected release manifest fixtures must cover branch-ref signing, signer
workflow path mismatch, release tag/ref mismatch, release commit mismatch, numeric-ID mismatch, and
missing semantic identity fields. Wrong values fail with the narrow ADR 0068 diagnostic; absent
unverifiable identity fields fail with `signer-identity-claim-missing`.

The mandatory ADR 0068 rejection fixtures are:

| Fixture name                             | Mutation                                                                                           | Expected diagnostic             |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------- | ------------------------------- |
| `identity-issuer-mismatch`               | Issuer differs from `https://token.actions.githubusercontent.com` by host, path, or normalization. | `issuer-mismatch`               |
| `identity-workflow-path-mismatch`        | URI SAN or Build Signer URI names another workflow while all other claims match.                   | `signer-workflow-path-mismatch` |
| `identity-workflow-sha-mismatch`         | Build Signer Digest differs from manifest/policy `workflow_sha`.                                   | `signer-workflow-sha-mismatch`  |
| `identity-source-repository-id-mismatch` | Source repository name/URI matches but OID `.15` differs.                                          | `source-numeric-id-mismatch`    |
| `identity-source-owner-id-mismatch`      | Source owner name/URI matches but OID `.17` differs.                                               | `source-numeric-id-mismatch`    |
| `identity-run-invocation-missing`        | Run Invocation URI extension is absent.                                                            | `run-invocation-uri-invalid`    |
| `identity-run-invocation-malformed`      | Run Invocation URI has another repository, zero/non-decimal IDs, query, fragment, or userinfo.     | `run-invocation-uri-invalid`    |
| `identity-self-hosted-runner`            | Platform-signed runner environment identifies a self-hosted runner.                                | `self-hosted-runner`            |

Each fixture changes only the named condition from `npm-maximal-identity-valid`. Every fixture is a
hard failure; no fixture may pass because a repository name, `builder.id`, workflow log, or unsigned
context value agrees with policy.

The ADR 0069 transparency and trust-root fixtures are:

| Fixture name                               | Mutation                                                                                        | Expected diagnostic          |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------- | ---------------------------- |
| `transparency-rekor-entry-missing`         | Bundle omits the Rekor inclusion proof or SET/log-entry binding.                                | `missing-rekor-entry`        |
| `transparency-rekor-entry-mismatch`        | Included entry binds another signature or certificate.                                          | `missing-rekor-entry`        |
| `transparency-sct-missing`                 | Fulcio leaf certificate omits its embedded SCT.                                                 | `missing-sct`                |
| `transparency-signature-time-before-cert`  | SET-covered `integratedTime` is before the certificate validity interval.                       | `signature-time-violation`   |
| `transparency-signature-time-after-cert`   | SET-covered `integratedTime` is after the certificate validity interval.                        | `signature-time-violation`   |
| `transparency-tsa-only`                    | A valid RFC 3161 timestamp is present, but Rekor evidence is absent.                            | `missing-rekor-entry`        |
| `trust-root-pinned-stale-deadline`         | Verification time is after the policy's `refresh_before` timestamp.                             | `stale-pinned-trust-root`    |
| `trust-root-pinned-online-not-revalidated` | An online run uses the pin without first revalidating it against the configured TUF repository. | `stale-pinned-trust-root`    |
| `trust-root-legacy-override`               | A prohibited component-key environment override can change root selection.                      | `legacy-trust-root-override` |
| `transparency-required-online-query`       | The pass/fail path attempts a Rekor, Fulcio, or CT-log network call.                            | `verification-network-call`  |

The valid offline fixture denies network access to log operators and still passes from local
artifact, bundle, and governed root bytes. The stale-root fixtures use standard Gregorian technical
timestamps in JSON. Each rejected fixture changes only the named condition from an accepted bundle
or trust-root fixture and fails before signed predicate contents can grant trust.

The `resolvedDependencies` lockfile fixture set must prove that the initial JS/TS npm profile emits
exactly one selected lockfile `ResourceDescriptor` and no generated transitive dependency list.
Accepted fixtures must cover manifest-selected npm with `package-lock.json`, manifest-selected pnpm
with `pnpm-lock.yaml`, manifest-selected Yarn Berry v4+ from top-level `packageManager` with
`yarn.lock`, and lockfile-inferred npm with `package-lock.json`. Accepted stale-lockfile fixtures
must record stale non-selected lockfiles in both
`externalParameters.package_manager.ignored_lockfile_paths` and
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
sidecar SHA-256, native provenance locators, upload result, linked artifact result, and linked
artifact metadata locators when metadata creation succeeds. Rejected fixtures must prove that linked
artifact locator outputs are unset when metadata is disabled or failed after upload, and that
internal handoff manifest names, handoff manifest digests, internal producer artifact names,
publisher handoff input names, raw artifact paths, caller-supplied artifact digests, upload URLs,
target repository coordinates, custom tokens, overwrite flags, release creation flags, or
multi-asset controls are not public inputs or outputs.

The handoff fixture set must prove that every cross-job artifact handoff includes the core semantic
fields `transport`, `artifact_name`, `payload_file_name`, `payload_kind`, `digest.algorithm`, and
`digest.value`, whether those fields are public inputs or profile-owned fixed mappings. Missing or
malformed fields fail with `handoff-schema-mismatch`.

The signed bundle fixture set must prove that the bundle file bytes emitted by `actions/attest` are
preserved byte-for-byte through npm provenance submission, publisher sidecar redistribution, and
release manifest upload. Fixtures that extract only the Statement, reserialize the bundle, wrap it
in a new envelope, or substitute a native attestation locator for the bundle file must fail with
`bundle-byte-format-mismatch` or the narrower sidecar/provenance mismatch category.

The duplicate JSON member fixture set must prove that signed SLSA Statement payloads and
security-relevant Sigstore bundle or DSSE JSON values fail before semantic policy validation when
any JSON object contains duplicate member names after JSON string unescaping. Rejected fixtures must
cover a top-level Statement duplicate, a nested predicate duplicate, a duplicate extension-field
member, duplicate DSSE payload-carrying fields, duplicate Sigstore bundle fields used for signature
or certificate verification, and escaped property spellings that decode to the same member name.
These fixtures fail with `duplicate-json-member`.

The npm publish permissions fixture set must prove that the initial npmjs production publish job has
`contents: read`, `id-token: write`, no `attestations: write`, and no `packages: write`. A workflow
that grants `packages: write` for the initial npmjs path fails with `excessive-publish-permission`.

The trusted-publisher fixture set is producer-side: it must prove that missing caller OIDC
permission or npmjs.com trusted publisher mismatch fails before registry mutation and never falls
back to token publish. These fixtures must not require adding caller trusted publisher settings to
the signed SLSA `externalParameters` schema.

The publisher convergence fixture set must classify the primary asset and sidecar independently,
before mutation and after an ambiguous mutating call, into exactly `committed-as-expected`,
`absent`, `foreign-conflict`, or `indeterminate`. A same-`run_id` retry must treat
`committed-as-expected` as satisfied without upload, upload only an `absent` asset, and fail without
mutation on `foreign-conflict` or `indeterminate`, reporting the exact state and remote evidence. A
new-`run_id` fixture with any pre-existing same-name asset must report `foreign-conflict` and fail
without mutation even when the asset digest matches. Any fixture that cannot produce exactly one of
the four states fails as `indeterminate` with `publisher-indeterminate-primary-upload` or the
narrower step diagnostic.

Partial and ambiguous publisher fixtures must express each step through those same four states. For
example, a primary upload followed by a sidecar API or transport failure reports the primary as
`committed-as-expected` after authoritative read-back and the sidecar as `absent`,
`foreign-conflict`, or `indeterminate` according to its observed remote state; a sidecar state other
than `committed-as-expected` fails with `sidecar-upload-partial-failure` or the narrower state
diagnostic. An ambiguous primary upload reports `committed-as-expected` only after authoritative
digest equality, `absent` after bounded authoritative absence, `foreign-conflict` after
authoritative digest inequality, or `indeterminate` when presence or digest equality cannot be
proved within the polling bound.

The publisher remote digest fixture set must prove that the GitHub Release asset `digest` field is
the sole authoritative release-asset binding. Accepted fixtures must cover an exact
`sha256:<64 lowercase hexadecimal characters>` match with `expected-sha256`. Rejected fixtures must
cover a missing or unreadable `digest`, an unsupported or malformed algorithm/value, API or
transport failure through the polling bound, contradictory observations, and a digest unequal to
`expected-sha256`. An unequal authoritative digest reports `foreign-conflict`; inability to read a
usable authoritative digest reports `indeterminate`. Downloaded bytes and a locally computed hash
may appear only as diagnostic evidence and must not change either result to `committed-as-expected`;
doing so fails the fixture with `publisher-remote-digest-unproven` or the narrower state diagnostic.

The publisher workflow schema fixture set must prove that the standalone publisher accepts only the
declared producer-neutral `workflow_call.inputs`, accepts no secrets, rejects target repository,
custom token, raw artifact, overwrite, cross-run artifact, or provenance-bypass inputs, and treats
empty optional JSON inputs as absent. Unsupported inputs or accepted secrets fail with
`publisher-workflow-schema-error`.

The linked artifact settings fixture set must prove that standalone publisher settings cannot widen
the release target. Accepted fixtures must derive `version` from `release-tag`, use the target
caller repository as `repository`, and use exactly the GitHub Release download URL prefix for that
same repository and tag. Rejected fixtures must cover a repository mismatch, owner or repository
case/canonicalization mismatch that changes identity, a version not derived from `release-tag`, a
registry URL for another repository or tag, non-HTTPS registry URLs, embedded credentials, queries,
fragments, non-default ports, and setting linked artifact locator outputs when the result is not
`created`. Settings failures use `linked-artifact-settings-mismatch`; output locator failures use
`linked-artifact-locator-mismatch`.

The publisher permission fixture set must prove that verification jobs are read-only, the release
upload job has `contents: write` without signing, package publication, or linked metadata authority,
and the optional metadata job has `artifact-metadata: write` without release mutation or signing
authority. Excessive or missing permissions fail with `publisher-permission-boundary-violation` or a
narrower permission category.

Permission verification has two distinct halves. Static YAML conformance runs at lint time and
checks declared workflow/job `permissions`, authority separation, and the absence of unintended
write grants. A static mismatch fails its fixture with `publisher-permission-boundary-violation` or
the narrower `release-asset-mode-permission-error`; it must not be reported as proof of the
permissions GitHub actually granted at runtime.

Runtime permission verification checks actual GitHub API behavior because GitHub does not expose the
caller's complete effective permission map in any workflow context. Read-only verification jobs
probe the non-mutating API operations they require, while mutation jobs observe the required API
operation at the guarded mutation boundary after all non-permission preconditions pass. An HTTP
`403` is a permission-failure signal and fails closed with `publisher-permission-boundary-violation`
or the narrower permission category before any later dependent operation. The implementation must
not infer runtime authority only from YAML, context strings, or token presence; such inference fails
the runtime-permission fixture with the same category. Static conformance and successful runtime API
behavior are both required, and success in one half cannot waive failure in the other.

The sidecar digest fixture set must prove that `sidecar-digest` is the 64-character lowercase
SHA-256 digest of the exact verified producer bundle bytes and equals `producer-provenance-sha256`.
It must be set after bundle verification even when sidecar upload later fails, and unset when bundle
retrieval or digest verification fails. A mismatched value fails with `sidecar-digest-mismatch`.

The native locator fixture set must prove that native provenance locators are diagnostic metadata
only. A missing locator must not fail otherwise valid publisher verification. A locator with an
unsupported type, non-`github.com` URL, repository mismatch, userinfo, query, fragment, malformed
path, unknown field, or malformed digest must fail with `native-locator-malformed`. A locator digest
that does not equal the sidecar bundle SHA-256 must fail with `native-locator-digest-mismatch`.

The release manifest convergence fixture set must classify the plain JSON asset and signed bundle
asset independently, before mutation and after an ambiguous mutating call, into exactly
`committed-as-expected`, `absent`, `foreign-conflict`, or `indeterminate`. A same-`run_id` retry
must treat `committed-as-expected` as satisfied without upload, upload only an `absent` asset, and
fail without mutation on `foreign-conflict` or `indeterminate`, reporting the exact state and remote
evidence. A new-`run_id` fixture with either pre-existing same-name manifest asset must report
`foreign-conflict` and fail without mutation even when its digest matches. Failure to classify an
asset into exactly one state reports `indeterminate` and fails with
`manifest-indeterminate-json-upload` or the narrower step diagnostic.

Partial and ambiguous manifest fixtures must use the four-state results rather than separate partial
or indeterminate upload result names. A signed-bundle API or transport failure after a successful
plain JSON upload reports the JSON asset as `committed-as-expected` after authoritative read-back
and the bundle as `absent`, `foreign-conflict`, or `indeterminate` according to remote state; a
bundle state other than `committed-as-expected` fails with `manifest-partial-json-uploaded` or the
narrower state diagnostic. An ambiguous plain JSON upload reports `committed-as-expected` only after
authoritative digest equality, `absent` after bounded authoritative absence, `foreign-conflict`
after authoritative digest inequality, or `indeterminate` when presence or digest equality cannot be
proved within the polling bound.

The release manifest remote digest fixture set must prove that the GitHub Release asset `digest`
field is the sole authoritative binding for both manifest assets. Accepted fixtures must cover an
exact `sha256:<64 lowercase hexadecimal characters>` match with the expected handoff digest.
Rejected fixtures must cover a missing or unreadable `digest`, an unsupported or malformed
algorithm/value, API or transport failure through the polling bound, contradictory observations, and
a digest unequal to the expected handoff digest. An unequal authoritative digest reports
`foreign-conflict`; inability to read a usable authoritative digest reports `indeterminate`.
Downloaded bytes and a locally computed hash may appear only as diagnostic evidence and must not
change either result to `committed-as-expected`; doing so fails with
`manifest-remote-digest-unproven` or the narrower state diagnostic.

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

The release manifest signing input fixture set must prove the closed signing input metadata schema.
Accepted fixtures must bind the same subject name, canonical manifest SHA-256, predicate type,
predicate artifact digest and content, release identity, and same-run artifact handles verified from
the manifest handoff. Rejected fixtures must cover unknown fields, duplicate fields, missing subject
or predicate fields, subject digest differing from the canonical manifest digest, predicate artifact
content differing from the manifest JSON value, release identity differing from the manifest JSON,
and artifact handles or basenames that do not match the handoff. These failures use
`manifest-signing-input-mismatch` unless the narrower basename, digest, duplicate-member, or
predicate mismatch category applies.

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
