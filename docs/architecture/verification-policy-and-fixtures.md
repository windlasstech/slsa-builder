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
  [0069](../decisions/0069-require-rekor-transparency-and-govern-sigstore-trust-root.md),
  [0070](../decisions/0070-record-package-manager-distributions-and-runner-image-in-resolved-dependencies.md),
  [0071](../decisions/0071-activate-builder-version-and-builderdependencies-for-platform-components.md),
  [0076](../decisions/0076-use-observation-preflights-and-first-mutation-classification.md),
  [0077](../decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md),
  [0078](../decisions/0078-treat-settings-only-pnpm-workspace-yaml-as-standalone-root-package-mode.md),
  [0079](../decisions/0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md),
  and
  [0080](../decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md)
- Related specs: [SLSA provenance v1](slsa-provenance-v1.md),
  [Identity and build types](identity-and-buildtypes.md), [Release manifest](release-manifest.md),
  [JS/TS npm build and pack](js-ts-npm-build-pack.md),
  [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md),
  [GitHub Release asset publisher](github-release-asset-publisher.md),
  [npm-to-release-asset composition](npm-to-release-asset-composition.md), and
  [Composed workflow internal handoff](composed-workflow-internal-handoff.md)

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

Verification mode is an invocation-level input outside the signed artifact. Exactly one mode,
`online` or `offline`, must be selected after resolving CLI, configuration, and environment inputs.
Absent, multiply selected, or disagreeing mode inputs, or a mode that does not match the selected
trust-root shape, fail before trust acquisition with
`windlass.verify.error.verification-mode-invalid` and exit code `2`.

`online` mode pairs only with the TUF trust-root shape. The verifier authenticates current Sigstore
public-good TUF metadata before bundle verification. TUF authentication failure, or any attempted
pinned-root fallback after that failure, fails with `windlass.verify.error.ungoverned-trust-root`;
an online mode must not use a pin, cached component key, or environment override as a fallback.

`offline` mode pairs only with the pinned-root shape and prohibits every network attempt, including
TUF acquisition and log-operator queries. A network attempt fails with
`windlass.verify.error.verification-network-call`. The pinned root must match its declared SHA-256
and TUF repository identity, or verification fails with
`windlass.verify.error.ungoverned-trust-root`. It must be used no later than `refresh_before`; a
stale pin fails with `windlass.verify.error.stale-pinned-trust-root`. ADR 0069 deliberately does not
select one universal duration for that deadline.

Valid invocation and root-shape examples are:

```json
{
  "verification_mode": "online",
  "trust_root": { "mode": "tuf", "instance": "sigstore-public-good" }
}
```

```json
{
  "verification_mode": "offline",
  "trust_root": {
    "mode": "pinned",
    "instance": "sigstore-public-good",
    "path": "trusted_root.json",
    "sha256": "7c222fb2927d828af22f592134e8932480637c0d3f9c2072e82716801567e69f",
    "tuf_repository": "https://tuf-repo-cdn.sigstore.dev",
    "revalidated_at": "2026-08-01T00:00:00Z",
    "refresh_before": "2026-08-08T00:00:00Z"
  }
}
```

The following invalid invocation fails before trust acquisition with
`windlass.verify.error.verification-mode-invalid` and exit code `2` because offline mode cannot use
a TUF root:

```json
{
  "verification_mode": "offline",
  "trust_root": { "mode": "tuf", "instance": "sigstore-public-good" }
}
```

The documented verification path forbids `SIGSTORE_ROOT_FILE`, `SIGSTORE_REKOR_PUBLIC_KEY`,
`SIGSTORE_CT_LOG_PUBLIC_KEY_FILE`, and semantically equivalent per-component environment overrides.
If any such override can affect root selection, verification fails with
`windlass.verify.error.legacy-trust-root-override`.

## Verifier policy and manifest expectation schemas

This section implements ADRs 0037, 0062, and 0068. These are verifier-input schemas, not a new
standalone CLI interface. Both schemas are closed: an unknown member, a missing required member, a
numeric identifier that does not match `^[1-9][0-9]*$`, or a SHA that is not 40 lowercase
hexadecimal characters fails with `windlass.verify.error.policy-schema-invalid` and exit code `1`.
Identifiers are strings because GitHub identifiers are opaque decimal identifiers, not values on
which a verifier performs arithmetic.

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

| Field                         | Required value form                                                                                                     |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `schema_version`              | Exactly `"1"`.                                                                                                          |
| `source.repository_uri`       | Canonical `https://github.com/<lowercase-owner>/<lowercase-repository>` URI without userinfo, port, query, or fragment. |
| `source.repository_id`        | Positive decimal string matching `^[1-9][0-9]*$`; authoritative over the repository name.                               |
| `source.repository_owner_id`  | Positive decimal string matching `^[1-9][0-9]*$`; authoritative over the owner name.                                    |
| `source.digest`               | Full 40-character lowercase hexadecimal Git commit SHA.                                                                 |
| `source.ref`                  | Full `refs/tags/<tag-name>` ref.                                                                                        |
| `producer.workflow_path`      | Exact trusted `.github/workflows/<file>.yml` or `.yaml` path, with no traversal or extra path separator.                |
| `producer.workflow_sha`       | Full 40-character lowercase hexadecimal called-workflow SHA.                                                            |
| `producer.runner_environment` | Exactly `"github-hosted"`.                                                                                              |
| `trust_root`                  | Exactly one of the closed TUF or pinned-root shapes specified here.                                                     |

Under ADR 0080, the policy's `source.digest` and `source.ref` express expectations about the
**built** source identity: they are compared against the signed `externalParameters.source.revision`
and `externalParameters.source.ref` fields, not against certificate extensions. The certificate's
platform-fixed source claims describe the invocation context and are bound separately, as defined in
the identity binding section below.

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
`windlass.verify.error.policy-schema-invalid`; a pin used after `refresh_before` fails with
`windlass.verify.error.stale-pinned-trust-root`. The invocation mode rules above, rather than this
signed policy object, decide whether either root shape is eligible.

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

The release-manifest expectation requires every member shown above. Both repository URIs use the
same lowercase canonical GitHub URI form as `source.repository_uri`; both numeric-ID fields match
`^[1-9][0-9]*$`, both workflow paths are exact trusted paths, both workflow SHAs are full lowercase
40-hex values, and `producer_profile.profile` is a non-empty policy-registered profile name. A
missing numeric ID, a name in place of an ID, an unknown member, an unregistered profile, or a
malformed URI, path, or SHA fails with `windlass.verify.error.policy-schema-invalid` before the
manifest can contribute constraints.

This lowercase GitHub URI rule is the canonical repository-URI casing rule for every policy,
manifest expectation, signed source-repository field, and identity comparison in this specification.
First, each URI is parsed and rejected unless it has the exact canonical lowercase form. Then,
values that represent the same field are compared byte-for-byte. Numeric repository and owner IDs
remain authoritative identity checks, but they do not permit a non-canonical URI spelling. The npm
package verification comparison in item 11 applies this parse-then-byte-compare rule to its three
source repository values.

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

| Fulcio extension OID     | Fulcio semantic claim                      | GitHub OIDC origin         | Required comparison                                                                                                                                                |
| ------------------------ | ------------------------------------------ | -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `1.3.6.1.4.1.57264.1.1`  | Issuer (deprecated legacy raw-string form) | `iss`                      | Decode old bundles only; never use as authoritative issuer.                                                                                                        |
| `1.3.6.1.4.1.57264.1.8`  | Issuer V2                                  | `iss`                      | DER-decoded issuer exactly matches the trusted GitHub issuer.                                                                                                      |
| `1.3.6.1.4.1.57264.1.9`  | Build Signer URI                           | `job_workflow_ref`         | Exact trusted policy-selected signer workflow URI and path.                                                                                                        |
| `1.3.6.1.4.1.57264.1.10` | Build Signer Digest                        | `job_workflow_sha`         | Full SHA equals the selected manifest/policy `workflow_sha`.                                                                                                       |
| `1.3.6.1.4.1.57264.1.11` | Runner Environment                         | `runner_environment`       | Exactly `github-hosted`.                                                                                                                                           |
| `1.3.6.1.4.1.57264.1.12` | Source Repository URI                      | `repository`               | Exact expected canonical `https://github.com/<owner>/<repo>` URI.                                                                                                  |
| `1.3.6.1.4.1.57264.1.13` | Source Repository Digest                   | `sha`                      | Byte-equal to the signed invocation record revision: `externalParameters.source.invocation_revision` when present, otherwise `externalParameters.source.revision`. |
| `1.3.6.1.4.1.57264.1.14` | Source Repository Ref                      | `ref`                      | Byte-equal to the signed invocation record ref: `externalParameters.source.invocation_ref` when present, otherwise `externalParameters.source.ref`.                |
| `1.3.6.1.4.1.57264.1.15` | Source Repository Identifier               | `repository_id`            | Exact expected decimal repository ID.                                                                                                                              |
| `1.3.6.1.4.1.57264.1.17` | Source Repository Owner Identifier         | `repository_owner_id`      | Exact expected decimal owner ID.                                                                                                                                   |
| `1.3.6.1.4.1.57264.1.21` | Run Invocation URI                         | `run_id` and `run_attempt` | Byte-for-byte equal to `metadata.invocationId` and well-formed for the expected source repository.                                                                 |

Issuer V2 OID `.8` is the current authoritative issuer source. A verifier may decode deprecated OID
`.1` for old-bundle diagnostics, but using `.1` instead of `.8` as the authoritative issuer source
fails with `windlass.verify.error.issuer-mismatch`.

The certificate URI SAN carries the called workflow ref. The Build Signer URI and SAN are separate
surfaces and both are checked; equality on one does not excuse absence or mismatch on the other. The
verifier enforces all six bindings below for npm producer, release asset producer, and release
manifest bundles. Every binding is mandatory, and no mismatch is downgraded to a warning.

| Binding | Required check                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Failure diagnostic                                                                                            |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| 1       | Certificate OIDC issuer is exactly `https://token.actions.githubusercontent.com`; URI normalization, redirects, prefixes, and alternate GitHub issuers are not accepted.                                                                                                                                                                                                                                                                                                                                                                                                      | `windlass.verify.error.issuer-mismatch`                                                                       |
| 2       | URI SAN and Build Signer URI identify the exact policy-selected signer workflow path, and Build Signer Digest (`job_workflow_sha`) exactly equals the manifest/policy `workflow_sha`. `github.workflow_sha` is the caller SHA and is forbidden as builder identity.                                                                                                                                                                                                                                                                                                           | `windlass.verify.error.signer-workflow-path-mismatch` or `windlass.verify.error.signer-workflow-sha-mismatch` |
| 3       | Source Repository URI equals the expected source URI, and OIDs `.15` and `.17` equal the expected decimal repository and owner IDs. Numeric IDs decide; names are display-only.                                                                                                                                                                                                                                                                                                                                                                                               | `windlass.verify.error.source-identity-mismatch` or `windlass.verify.error.source-numeric-id-mismatch`        |
| 4       | Source Repository Digest and Source Repository Ref describe the invocation context (ADR 0080): each must be byte-for-byte equal to the signed invocation record — `externalParameters.source.invocation_revision` and `externalParameters.source.invocation_ref` when those members are present, otherwise `externalParameters.source.revision` and `externalParameters.source.ref`. The expected release commit SHA and release tag ref from policy instead bind to the signed built-source fields `externalParameters.source.revision` and `externalParameters.source.ref`. | `windlass.verify.error.source-digest-mismatch` or `windlass.verify.error.source-ref-mismatch`                 |
| 5       | Run Invocation URI is present, byte-for-byte equals `metadata.invocationId`, and has exactly `https://github.com/<owner>/<repo>/actions/runs/<run-id>/attempts/<attempt-number>`, where owner/repo equal the expected source URI and both final components are positive base-10 integers without signs, whitespace, query, fragment, userinfo, or a non-default port.                                                                                                                                                                                                         | `windlass.verify.error.run-invocation-uri-invalid`                                                            |
| 6       | Runner Environment OID `.11` is present and its platform-signed value is exactly `github-hosted`. A self-hosted, missing, unknown, or caller-asserted runner value is not accepted.                                                                                                                                                                                                                                                                                                                                                                                           | `windlass.verify.error.self-hosted-runner`                                                                    |

ADR 0062 policy intersection operates over these immutable keys: workflow path plus `workflow_sha`,
source `repository_id`, source `repository_owner_id`, source digest, and source ref — the latter two
binding the signed built-source fields per ADR 0080. When the explicit policy and an authenticated
release-manifest expectation both constrain one of those keys, both values apply. An empty
intersection fails with `windlass.verify.error.trusted-producer-policy-conflict`; names never break
a tie between different numeric IDs.

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

For publisher and composed verification, the closed producer-policy registry is the baseline and the
caller constraints, digest-verified handoff, and authenticated release-manifest constraints only
narrow that baseline. Their effective policy is their intersection. An unknown `buildType` fails
before publication acceptance with `windlass.verify.error.unregistered-producer-build-type`; an
empty intersection or any attempted caller, handoff, or manifest override fails with
`windlass.verify.error.trusted-producer-policy-conflict` or a narrower field diagnostic. No source
may add, union with, replace, relax, or override a registry policy.

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
    normalized package repository, package identity, package manager, runner, publish intent,
    release ref, and build-script result.
11. `externalParameters.package.repository`, `externalParameters.source.repository`, and the
    observed caller repository identity are byte-for-byte equal canonical
    `https://github.com/<lowercase-owner>/<lowercase-repository>` values. A missing, malformed, or
    unequal value rejects the package with
    `windlass.verify.error.package-repository-identity-mismatch`.
12. The signer identity is the Windlass reusable workflow identity, while the source repository in
    `externalParameters.source.repository` is the caller package repository.
13. No unexpected `externalParameters` are present under strict matching.
14. The selected package is not marked `private: true`.
15. For `https://registry.npmjs.org/`, the package version was not already present before publish
    and the selected package identity already existed before publish; first publication of a package
    identity is outside the initial npmjs trusted-publishing-only profile.
16. For non-npmjs registries, package identity and package version preflight fields are best-effort
    diagnostics unless a later ADR defines that registry class. A custom-registry report, including
    `custom-registry-preflight-inconclusive`, never establishes successful registry conformance.
17. `externalParameters.package.package_url` equals the registry package-version URL reconstructed
    from `externalParameters.publish.resolved_registry_url`, `externalParameters.package.name`, and
    `externalParameters.package.version`. It must not be a Package URL (`pkg:npm/...`).
18. `externalParameters.source.ref`, `externalParameters.release.ref`, and the accepted built ref
    are the same full `refs/tags/<tag-name>` ref, and `externalParameters.release.version_tag`
    reconstructs that same ref. When `source.input_ref` is present it must equal that same ref, and
    `source.invocation_ref` and `source.invocation_revision` record the invocation context per
    ADR 0080.

npmjs.com trusted publisher configuration is registry-side publish authorization policy. It is
enforced by the producer-side publish gate and by npm during `npm publish`. Consumer-side policy
does not reconstruct the remote registry configuration, but it requires signed
`externalParameters.caller.workflow_filename` to equal the observed caller filename relevant to that
trusted-publisher authorization; absence or mismatch fails with
`windlass.verify.error.trusted-publisher-mismatch`.

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
- the name-keyed `lockfile` descriptor identifies the selected `yarn.lock`;
- no policy accepts Yarn Classic 1.x, Yarn Berry v2, Yarn Berry v3, `devEngines.packageManager`-only
  Yarn selection, lockfile-only Yarn inference, ambient global Yarn, or Corepack Known Good Release
  fallback.

For the v1 npm profile, verifier policy additionally requires the closed `distribution` and `caller`
groups defined by the [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md) contract.
The signed locations for the nine public inputs are `package.directory`,
`publish.input_registry_url`, `publish.input_dist_tag`, `publish.input_access`, `source.input_ref`,
`distribution.release_asset_mode`, `distribution.release_tag_supplied`,
`distribution.provenance_sidecar`, and `distribution.linked_artifact_metadata`; the observed trusted
publisher filename is `caller.workflow_filename`. Missing or unknown `distribution` or `caller`
members fail with `windlass.verify.error.unexpected-external-parameters`. A signed normalized
distribution value that disagrees with accepted public mode inputs fails with
`windlass.verify.error.release-asset-mode-schema-error`; an unavailable or mismatched observed
caller workflow filename fails with `windlass.verify.error.trusted-publisher-mismatch`.

The closed `distribution` object has exactly `release_asset_mode`, `release_tag_supplied`,
`provenance_sidecar`, and `linked_artifact_metadata`. The first, second, and fourth members are
booleans; `provenance_sidecar` is `null` in npm-only mode and exactly `"required"` in release-asset
mode. `release_tag_supplied` records suppliedness only; the effective tag remains in `release.ref`
and `release.version_tag`. The closed `caller` object has exactly `workflow_filename`, the
normalized observed caller workflow filename. A raw release tag in `distribution`, a sidecar value
preserving omitted-versus-`required` spelling, or another distribution normalization disagreement
fails with `windlass.verify.error.release-asset-mode-schema-error`; a missing or unknown group
member fails with `windlass.verify.error.unexpected-external-parameters`.

`internalParameters` must be exactly the empty object `{}`. A non-object or any member fails with
`windlass.verify.error.unexpected-internal-parameters`.

`resolvedDependencies` is unordered and selected by its `name`, never by array position. npm
requires exactly `lockfile` and `runner-image`; pnpm and Yarn additionally require exactly one
`package-manager-distribution`. An unknown descriptor name or non-enumerated extra entry fails with
`windlass.verify.error.resolved-dependencies-unexpected-entry`. A known manager-distribution
descriptor that is missing, duplicated, forbidden for npm, malformed, uses the wrong authority, or
disagrees with `externalParameters.package_manager` fails with
`windlass.verify.error.resolved-dependencies-package-manager-distribution`. A missing, duplicated,
malformed, digest-bearing, or mismatched `runner-image` fails with
`windlass.verify.error.resolved-dependencies-runner-image`.

The `runner-image` descriptor has no `digest`; its `uri` is the verbatim `Included Software` report
URL read from `/imagegeneration/imagedata.json`, and its closed annotations are `image_os`,
`image_version`, and `node_version`. Capture must confirm the documented file shape, the `ImageOS`
and `ImageVersion` correspondence, and the exact observed `node --version` before predicate
construction. Capture evidence unavailable at that stage fails with
`windlass.verify.error.input-unavailable` and exit code `2`; a constructed descriptor that violates
this shape fails with the narrow runner-image diagnostic.

`builder.version` is closed: it contains required lowercase `nodejs` and contains lowercase
`corepack` only when Corepack supplied the selected manager. Missing or extra keys, wrong
conditional `corepack` presence, or an observed-version mismatch fails with
`windlass.verify.error.builder-version-mismatch`. `builderDependencies` contains exactly one signing
adapter descriptor: `pkg:golang/github.com/sigstore/sigstore-go@v1.3.0`, with exactly
`digest.h1: hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE=` and `annotations.role: "signing-adapter"`.
A missing, extra, malformed, wrong-role, or module/checksum-inconsistent descriptor fails with
`windlass.verify.error.builder-dependencies-signing-adapter-mismatch`.

The following valid npm-profile fragments show the closed v1 shapes. The runner-image URI is the
verbatim platform-provided `Included Software` URL, not a reconstructed URL.

```json
{
  "internalParameters": {},
  "externalParameters": {
    "distribution": {
      "release_asset_mode": false,
      "release_tag_supplied": false,
      "provenance_sidecar": null,
      "linked_artifact_metadata": false
    },
    "caller": { "workflow_filename": "release.yml" }
  },
  "resolvedDependencies": [
    { "name": "lockfile" },
    {
      "name": "runner-image",
      "uri": "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md",
      "annotations": {
        "image_os": "ubuntu24",
        "image_version": "20260801.1.0",
        "node_version": "v24.0.0"
      }
    }
  ],
  "runDetails": {
    "builder": {
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
      "invocationId": "https://github.com/example/acme-widget/actions/runs/123456789/attempts/2"
    }
  }
}
```

The following invalid fragment fails with `windlass.verify.error.resolved-dependencies-runner-image`
because the runner image carries a digest:

```json
{
  "resolvedDependencies": [
    {
      "name": "runner-image",
      "uri": "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md",
      "digest": { "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" },
      "annotations": {
        "image_os": "ubuntu24",
        "image_version": "20260801.1.0",
        "node_version": "v24.0.0"
      }
    }
  ]
}
```

## GitHub Release asset verification policy

For a release asset uploaded by the publisher, the unchanged producer bundle must first pass the
maximal identity, offline transparency, signing-time, and trust-root sections above; failure rejects
the asset with the narrow diagnostic from those sections. The verifier then must check:

1. The downloaded release asset is the primary artifact.
2. The provenance sidecar is present and is the unchanged producer bundle.
3. The producer bundle signature is valid and the signer identity is trusted.
4. The producer `predicateType` is `https://slsa.dev/provenance/v1`.
5. The producer `buildType` exactly selects the sole entry in the
   [publisher's closed producer-policy registry](github-release-asset-publisher.md#closed-producer-policy-registry),
   `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`. An absent or unregistered
   `buildType` rejects the asset before publication acceptance with
   `windlass.verify.error.unregistered-producer-build-type`.
6. The producer `builder.id`, signer repository and workflow, npm Package URL subject, SHA-512 and
   SHA-256 subject digests, tarball-name binding, canonical source and release-ref rules, and closed
   JS/TS npm `externalParameters` schema satisfy the selected registry baseline and every applicable
   narrowing caller, handoff, and authenticated manifest constraint. A conflict rejects the asset
   with `windlass.verify.error.trusted-producer-policy-conflict` or a narrower field diagnostic.
7. The producer `subject[0].name` matches the expected producer subject under the selected producer
   policy. For the initial npm composition, this is the npm Package URL, not the release asset name.
8. The selected producer policy binds the release asset name to the verified producer artifact. For
   the initial npm composition, the release asset name must match the pack-produced tarball filename
   recorded in producer provenance and handoff fields.
9. The producer `subject[0].digest.sha256` matches the downloaded asset bytes.
10. The publisher did not generate or re-sign the provenance.

For publication-state or same-run convergence checks, the GitHub Release asset `digest` is the sole
authoritative remote binding. Downloaded bytes and locally computed hashes are diagnostic evidence
in that decision and cannot establish `committed-as-expected`; a missing or unusable GitHub `digest`
fails as `indeterminate` with `windlass.verify.error.publisher-remote-digest-unproven` or a narrower
diagnostic. This does not replace the consumer artifact-byte verification in item 9.

For same-`run_id` convergence identity comparison (ADRs 0072 and 0073), the Run Invocation URI
run-id component must equal the current run's `github.run_id` byte-for-byte, and the attempt
component is ignored: an earlier attempt of the same run is the same run identity for convergence.
This is a named exception to the byte-for-byte Run Invocation URI comparison in rule 5, scoped to
same-run convergence verification only. Consumer artifact verification compares the full Run
Invocation URI, including the attempt component, byte-for-byte.

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

When the verifier evaluates release-manifest publication convergence, it must use the one pair-level
outcome and per-asset evidence substates specified by
[Release manifest](release-manifest.md#outcome-classification-and-same-run-convergence-adr-0067).
Both assets require valid matching GitHub `digest` evidence and successful semantic binding before
the pair is `committed-as-expected`; downloaded bytes and local hashes are diagnostic-only. A
partial committed/absent pair is `foreign-conflict` with
`windlass.verify.error.manifest-partial-json-uploaded`; unresolved required evidence is
`indeterminate` with `windlass.verify.error.manifest-indeterminate-json-upload`. An immutable target
that cannot complete verified same-`run_id` read-only convergence fails before mutation with
`windlass.verify.error.release-target-immutable`.

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

1. Resolve the invocation-level verification mode before trust acquisition. Exactly one `online` or
   `offline` selection must agree with the selected trust-root shape; failure stops with
   `windlass.verify.error.verification-mode-invalid` and exit code `2`.
2. Parse the explicit verifier policy, manifest expectation, signed release manifest, and bundle
   with duplicate-member rejection. Validate their closed schemas. A parse or schema failure stops
   evaluation with `windlass.verify.error.duplicate-json-member` or
   `windlass.verify.error.policy-schema-invalid`; signed fields are not evaluated after ambiguous
   parsing.
3. Acquire the selected trust root. Online mode authenticates current TUF metadata and never falls
   back to a pin; offline mode uses only the fresh pinned root and makes no network call. A failure
   stops with the applicable mode, trust-root, or network diagnostic; no signature or predicate
   claim is trusted after that failure.
4. Verify the local artifact and local bundle with `gh`, requesting JSON. Supply all constraints the
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

5. Parse `gh-verified.json` as untrusted machine-readable data and require a successful result for
   the exact artifact digest and bundle supplied in step 4. Extract the leaf certificate from the
   verified bundle, decode the URI SAN and the OIDs in the Fulcio table above, and emit those values
   into the post-processor's typed JSON model. Missing, duplicate, or malformed certificate values
   fail with `windlass.verify.error.signer-identity-claim-missing`.
6. In the JSON post-processor, compare the exact issuer; SAN and Build Signer URI; Build Signer
   Digest; source URI, numeric repository IDs, digest, and ref; Run Invocation URI;
   `metadata.invocationId`; and runner environment against the policy inputs. A mismatch fails with
   the diagnostic assigned to that binding. Do not read `github.workflow_sha`, workflow logs,
   unsigned outputs, or artifact names to fill a missing certificate claim.
7. Verify the bundle-contained SCT, Rekor inclusion proof, SET, and SET-covered integrated time
   against the governed root. This step uses only local bundle and root bytes and fails with the
   applicable transparency or signing-time diagnostic. An RFC 3161 timestamp, when present, is
   verified as additional evidence and cannot make missing Rekor or SCT evidence pass.
8. Authenticate the release manifest bundle by repeating steps 4–7 with the release-manifest
   predicate type and manifest identity expectations. Only after it passes may its represented
   fields enter the ADR 0062 policy intersection. An unauthenticated manifest fails with
   `windlass.verify.error.release-manifest-mismatch` and contributes no policy values.
9. Compute the field-by-field intersection of authenticated manifest expectations and explicit
   policy, then validate Statement structure, `predicateType`, `builder.id`, `buildType`, strict
   `externalParameters`, subject identity, and artifact digests. A conflict fails with
   `windlass.verify.error.trusted-producer-policy-conflict`; semantic or digest failures use the
   narrower registered diagnostic.
10. Serialize the diagnostic report defined below. Acceptance occurs only when every required check
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
policy checks such as strict `externalParameters` and release manifest mapping. The remote npm
trusted-publisher configuration object is a producer-side registry authorization precondition, but
the signed `caller.workflow_filename` is a consumer-verified provenance field enforced under the npm
package verification policy.

## Fixture taxonomy

Every fixture manifest is one UTF-8 JSON object parsed with duplicate-member rejection. It has this
closed schema; unknown members and values that fail this schema fail the fixture harness with
`windlass.verify.error.diagnostics-contract-invalid`:

```json
{
  "type": "object",
  "additionalProperties": false,
  "required": [
    "name",
    "type",
    "surface",
    "artifact",
    "provenance",
    "release-manifest",
    "expected-result",
    "expected-failure-category",
    "expected-primary-id",
    "expected-secondary-ids",
    "covered-requirement"
  ],
  "properties": {
    "name": { "type": "string", "minLength": 1 },
    "type": { "enum": ["accepted", "rejected"] },
    "surface": { "enum": ["npm", "publisher", "composition", "release-manifest"] },
    "artifact": { "type": "string", "minLength": 1 },
    "provenance": { "type": "string", "minLength": 1 },
    "release-manifest": { "type": ["string", "null"] },
    "expected-result": { "enum": ["pass", "fail"] },
    "expected-failure-category": { "type": ["string", "null"] },
    "expected-primary-id": { "type": ["string", "null"] },
    "expected-secondary-ids": { "type": "array", "items": { "type": "string" } },
    "covered-requirement": { "type": "string", "minLength": 1 }
  }
}
```

An `accepted` fixture has `expected-result: "pass"`, `expected-failure-category: null`, and
`expected-primary-id: null`. A `rejected` fixture has `expected-result: "fail"`, a registered
non-null failure category, and its corresponding non-null canonical primary ID. Every fixture must
include:

| Field                       | Description                                                    |
| --------------------------- | -------------------------------------------------------------- |
| `name`                      | Unique fixture name.                                           |
| `type`                      | `accepted` or `rejected`.                                      |
| `surface`                   | `npm`, `publisher`, `composition`, or `release-manifest`.      |
| `artifact`                  | Path to the artifact.                                          |
| `provenance`                | Path to the provenance bundle.                                 |
| `release-manifest`          | Path to the release manifest, if applicable.                   |
| `expected-result`           | `pass` or `fail`.                                              |
| `expected-failure-category` | Required for `rejected` fixtures.                              |
| `expected-primary-id`       | Required stable primary diagnostic ID for a rejected fixture.  |
| `expected-secondary-ids`    | Ordered independent secondary diagnostic IDs, when applicable. |
| `covered-requirement`       | Spec or ADR requirement covered by the fixture.                |

## Diagnostics contract

This section specifies the conformance diagnostics required by ADRs 0037 and 0062. It does not
stabilize a standalone verifier CLI, which remains a future ADR decision. Producer gates, fixture
harnesses, and consumer implementations claiming conformance must apply this contract; an output
that cannot represent it fails the fixture harness with
`windlass.verify.error.diagnostics-contract-invalid`.

### Shared diagnostic taxonomy

This is the shared diagnostic taxonomy for all architecture specifications. New output IDs must use
the canonical `windlass.verify.<severity>.<category>` shape. Existing bare kebab-case names remain
stable fixture aliases for the corresponding canonical error ID: for example,
`release-target-immutable` means `windlass.verify.error.release-target-immutable`. A new
implementation must emit the canonical ID, while a fixture may use its registered bare alias in
`expected-failure-category` only. Existing prefixed IDs and bare aliases must not be renamed.

`phase` identifies the latest processing phase that must complete before the diagnostic can be
emitted: `invocation` accepts local invocation inputs, `policy` parses and validates policy inputs,
`verification` evaluates authenticated artifact evidence, `pre-mutation` checks a remote target
before a write, and `mutation` performs or read-backs a remote write. `mutation_possible` is `true`
when the failing step could already have mutated remote state, including an ambiguous mutating
request that requires ADR 0067 read-back. It is not a claim that a mutation happened. A `false`
value means the diagnostic is emitted before any remote mutation by that step. Implementations must
treat `true` as requiring the applicable ADR 0067 state classification before the final report.

Each comma-separated value in `diagnostic_id` is an independently registered diagnostic with the
row's same phase, exit code, and mutation flag. The rejected-fixture category registry below
registers every remaining canonical error alias not expanded here, with `verification`, exit code
`1`, and `mutation_possible: false`, unless a more specific row below applies.

| diagnostic_id                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | phase                           | exit_code            | mutation_possible    |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------- | -------------------- | -------------------- |
| `windlass.verify.error.verification-mode-invalid`, `windlass.verify.error.input-unavailable`, `windlass.verify.error.verifier-execution-failure`                                                                                                                                                                                                                                                                                                                                       | invocation                      | `2`                  | `false`              |
| `windlass.verify.error.policy-schema-invalid`, `windlass.verify.error.duplicate-json-member`, `windlass.verify.error.legacy-trust-root-override`                                                                                                                                                                                                                                                                                                                                       | policy                          | `1`                  | `false`              |
| `windlass.verify.error.ungoverned-trust-root`, `windlass.verify.error.stale-pinned-trust-root`, `windlass.verify.error.verification-network-call`                                                                                                                                                                                                                                                                                                                                      | verification                    | `1`                  | `false`              |
| `windlass.verify.error.run-invocation-uri-invalid`, `windlass.verify.error.diagnostics-contract-invalid`, `windlass.verify.error.release-manifest-mismatch`, `windlass.verify.error.trusted-producer-policy-conflict`                                                                                                                                                                                                                                                                  | verification                    | `1`                  | `false`              |
| `windlass.verify.error.release-target-immutable`, `windlass.verify.error.handoff-schema-mismatch`, `windlass.verify.error.publisher-remote-digest-unproven`, `windlass.verify.error.npm-oidc-exchange-indeterminate`, `windlass.verify.error.oidc-capability-unavailable`, `windlass.verify.error.source-ref-invalid`                                                                                                                                                                  | pre-mutation                    | `1`                  | `false`              |
| `windlass.verify.error.mutation-permission-denied`                                                                                                                                                                                                                                                                                                                                                                                                                                     | mutation                        | `1`                  | `false`              |
| `windlass.verify.error.publisher-indeterminate-primary-upload`, `windlass.verify.error.mutation-queue-overflow`, `windlass.verify.error.manifest-partial-json-uploaded`, `windlass.verify.error.manifest-indeterminate-json-upload`, `windlass.verify.error.manifest-remote-digest-unproven`                                                                                                                                                                                           | mutation                        | `1`                  | `true`               |
| `windlass.verify.error.sidecar-upload-partial-failure`, `windlass.verify.error.duplicate-release-asset`, `windlass.verify.error.duplicate-sidecar-asset`, `windlass.verify.error.registry-linkage-mismatch`, `windlass.verify.error.custom-registry-tokenless-auth-failed`, `windlass.verify.error.custom-registry-provenance-submission-rejected`, `windlass.verify.error.custom-registry-linkage-metadata-absent`, `windlass.verify.error.custom-registry-digest-semantics-mismatch` | mutation                        | `1`                  | `true`               |
| `release-target-immutable`, `handoff-schema-mismatch`, `policy-schema-invalid`, `publisher-indeterminate-primary-upload`, `mutation-queue-overflow`, `run-invocation-uri-invalid`                                                                                                                                                                                                                                                                                                      | Alias of the canonical ID above | Same as canonical ID | Same as canonical ID |

`policy-schema-invalid` is the sole diagnostic for a malformed or otherwise unusable explicit policy
or manifest expectation. It is a completed policy-validation failure, not an invocation failure, and
therefore always exits `1`. `verification failed` means a completed policy, cryptographic, identity,
or artifact verification check rejected the supplied evidence and also exits `1`; it is distinct
from unusable local invocation inputs, which use the exit-code-`2` invocation diagnostics above.
`mutation-permission-denied` is the first `mutation`-phase row with `mutation_possible: false`: a
definitive HTTP `403` or `401` from the first mutating call proves that submission was rejected,
unlike an ambiguous submission that requires ADR 0067 read-back.

### Canonical JSON serialization

Where this specification requires canonical JSON bytes or an ordering key, it means RFC 8785 JSON
Canonicalization Scheme (JCS) serialization of the parsed JSON value. Implementations must reject
duplicate object members before parsing the value for JCS. Structural equality means recursive JSON
value equality after that duplicate-member rejection and is not byte equality. A requirement for
original byte equality applies only when it explicitly names the original serialized artifact or
bundle bytes. In particular, implementations must use JCS, not the contradictory phrase
"byte-for-byte JSON-value equivalent", for diagnostic ordering and for a signed JSON value's
canonical digest contract.

### Stable diagnostic IDs

Every diagnostic ID has the closed form `windlass.verify.<severity>.<category>`, where `<severity>`
is `error` or `warning` and `<category>` is a lowercase kebab-case stable check name.
Rejected-fixture categories in the registry below map to
`windlass.verify.error.<expected-failure-category>`. The ID, not the human message, is the stable
machine key. Implementations may translate `message`, but they must not rename, reuse, or
dynamically construct a different ID for the same registered check; doing so fails with
`windlass.verify.error.diagnostics-contract-invalid`.

The non-fatal warning IDs initially registered by this specification are:

| Diagnostic ID                                                    | Meaning                                                                                                    |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `windlass.verify.warning.stale-non-selected-lockfile`            | A supported but non-selected lockfile was recorded; verification remains valid.                            |
| `windlass.verify.warning.custom-registry-preflight-inconclusive` | Non-npmjs metadata preflight was inconclusive under the documented best-effort policy.                     |
| `windlass.verify.warning.native-provenance-locator-missing`      | Optional native provenance locator is absent while the required sidecar verifies.                          |
| `windlass.verify.warning.timestamp-clock-skew`                   | `finishedOn` precedes `startedOn` by one to five whole seconds; verification continues with zero duration. |

No identity, root, signature, transparency, signing-time, policy-intersection, Statement, or digest
failure has a warning form. Emitting one of those failures as a warning fails the diagnostics
contract and the verification result remains rejected.

The following stable policy-error IDs are registered for the failing-fixture contract. Each has
severity `error` and exit code `1`, except `verification-mode-invalid`, which is an invocation error
and exits `2`; a fixture that proves its condition but emits another primary ID fails with
`windlass.verify.error.diagnostics-contract-invalid`.

| Diagnostic ID                                                              | Required failure condition                                                                                                                                                                                                                                                                  |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `windlass.verify.error.manifest-partial-json-uploaded`                     | A manifest pair has a partial committed/absent result.                                                                                                                                                                                                                                      |
| `windlass.verify.error.manifest-indeterminate-json-upload`                 | Required manifest-pair evidence remains unresolved.                                                                                                                                                                                                                                         |
| `windlass.verify.error.manifest-remote-digest-unproven`                    | A manifest asset lacks usable authoritative GitHub `digest` evidence.                                                                                                                                                                                                                       |
| `windlass.verify.error.custom-registry-token-required`                     | A custom registry requires a token or OTP before mutation.                                                                                                                                                                                                                                  |
| `windlass.verify.error.custom-registry-provenance-weakened`                | Custom-registry publication weakens the exact external provenance bundle.                                                                                                                                                                                                                   |
| `windlass.verify.error.custom-registry-tokenless-auth-failed`              | Tokenless authentication fails at the authentication or publish boundary.                                                                                                                                                                                                                   |
| `windlass.verify.error.custom-registry-access-option-rejected`             | A custom registry rejects the caller-supplied `access` option without proving a token or OTP requirement.                                                                                                                                                                                   |
| `windlass.verify.error.custom-registry-provenance-submission-rejected`     | The registry rejects the exact external provenance file.                                                                                                                                                                                                                                    |
| `windlass.verify.error.custom-registry-linkage-metadata-absent`            | Required registry linkage metadata is absent after publication.                                                                                                                                                                                                                             |
| `windlass.verify.error.custom-registry-digest-semantics-mismatch`          | Registry digest evidence is absent, malformed, incompatible, or mismatched.                                                                                                                                                                                                                 |
| `windlass.verify.error.release-target-immutable`                           | An immutable target cannot satisfy permitted read-only convergence conditions.                                                                                                                                                                                                              |
| `windlass.verify.error.package-repository-identity-mismatch`               | Raw repository metadata is missing, malformed, or normalizes to another source.                                                                                                                                                                                                             |
| `windlass.verify.error.unregistered-producer-build-type`                   | A producer `buildType` is absent from the closed publisher policy registry.                                                                                                                                                                                                                 |
| `windlass.verify.error.verification-mode-invalid`                          | Invocation mode is absent, multiple, conflicting, or incompatible with trust-root shape.                                                                                                                                                                                                    |
| `windlass.verify.error.unexpected-internal-parameters`                     | `internalParameters` is not an object or is nonempty.                                                                                                                                                                                                                                       |
| `windlass.verify.error.resolved-dependencies-unexpected-entry`             | An unknown descriptor name or non-enumerated dependency entry is present.                                                                                                                                                                                                                   |
| `windlass.verify.error.resolved-dependencies-package-manager-distribution` | The known conditional manager-distribution descriptor is missing, duplicated, forbidden, or malformed.                                                                                                                                                                                      |
| `windlass.verify.error.resolved-dependencies-runner-image`                 | The runner-image descriptor is missing, duplicated, malformed, digest-bearing, or mismatched.                                                                                                                                                                                               |
| `windlass.verify.error.builder-version-mismatch`                           | Closed `builder.version` keys or observed versions are invalid.                                                                                                                                                                                                                             |
| `windlass.verify.error.builder-dependencies-signing-adapter-mismatch`      | The sole signing-adapter builder dependency is missing, extra, malformed, or dependency-inconsistent.                                                                                                                                                                                       |
| `windlass.verify.error.mutation-queue-overflow`                            | GitHub rejects an arrival beyond 100 pending executions in one mutation concurrency group.                                                                                                                                                                                                  |
| `windlass.verify.error.npm-oidc-exchange-indeterminate`                    | The npm OIDC token exchange surface is unreadable or erroring, including HTTP `5xx` or a malformed response, so trusted-publisher configuration cannot be classified; the run fails as `indeterminate` even though the exchange mints only a short-lived publish token and mutates nothing. |
| `windlass.verify.error.oidc-capability-unavailable`                        | `ACTIONS_ID_TOKEN_REQUEST_TOKEN` is absent or the id-token request fails, proving that the caller job cannot provide OIDC credentials because `id-token: write` is missing.                                                                                                                 |
| `windlass.verify.error.mutation-permission-denied`                         | The first mutating call receives a definitive HTTP `403` or `401`; the rejection proves no mutation occurred, so no ADR 0067 read-back is required.                                                                                                                                         |

Expanded ownership for existing IDs is fixed as follows: `run-invocation-uri-invalid` owns a
malformed or missing URI and inequality between `metadata.invocationId` and Fulcio OID `.21`;
`unexpected-external-parameters` owns missing or unknown closed `distribution` and `caller` members;
`release-asset-mode-schema-error` owns a signed normalized distribution value that disagrees with an
accepted public mode input; `trusted-publisher-mismatch` owns unavailable or mismatched observed
caller workflow filename, trusted-publisher configuration or authentication rejection at the early
OIDC exchange preflight (HTTP `401` or `404`), and residual publish-time authorization rejections
(npm `E404` or `ENEEDAUTH`) mapped into this taxonomy under ADR 0076; `verification-network-call`
owns any offline network attempt as well as its existing log-query prohibition;
`ungoverned-trust-root` owns online TUF authentication failure, online pin fallback, and pinned-root
digest or repository mismatch; `stale-pinned-trust-root` owns a pin used after `refresh_before`; and
`input-unavailable` owns producer capture evidence unavailable before candidate predicate
construction. These ownership rules do not change their existing severity or exit mappings.

### Machine-readable serialization

The report is one UTF-8 JSON object. It is parsed with duplicate-member rejection and has this
closed shape:

| Member                | Type                        | Contract                                                                                  |
| --------------------- | --------------------------- | ----------------------------------------------------------------------------------------- |
| `schema_version`      | string                      | Exactly `"1"`.                                                                            |
| `result`              | string                      | `pass` or `fail`; `fail` exactly when at least one `error` exists.                        |
| `exit_code`           | integer                     | Equals the exit-code table below.                                                         |
| `primary_id`          | string or `null`            | First ordered error ID, or `null` when no error exists.                                   |
| `run_invocation`      | string or `null`            | Verified Run Invocation URI, or `null` only when verification could not authenticate one. |
| `diagnostics`         | array of diagnostic objects | Deterministically ordered as specified below.                                             |
| `diagnostic_metadata` | object, optional            | Closed non-trust metadata shape defined below.                                            |

Each diagnostic object is a closed object with this normative schema. `expected` and `actual` are
typed JSON values, not interpolated message fragments. `evidence` contains only non-secret local
identifiers such as an OID, bundle path, artifact digest, or certificate fingerprint.

```json
{
  "type": "object",
  "additionalProperties": false,
  "required": ["id", "severity", "category", "check", "message"],
  "properties": {
    "id": {
      "type": "string",
      "pattern": "^windlass\\.verify\\.(error|warning)\\.[a-z0-9]+(?:-[a-z0-9]+)*$"
    },
    "severity": { "enum": ["error", "warning"] },
    "category": { "type": "string", "pattern": "^[a-z0-9]+(?:-[a-z0-9]+)*$" },
    "check": { "type": "string", "minLength": 1 },
    "message": { "type": "string", "minLength": 1 },
    "field": { "type": "string", "minLength": 1 },
    "expected": {},
    "actual": {},
    "policy_sources": {
      "type": "array",
      "items": {
        "enum": [
          "explicit-policy",
          "release-manifest",
          "producer-expected-value",
          "digest-verified-handoff"
        ]
      }
    },
    "evidence": {
      "type": "object",
      "additionalProperties": { "type": ["string", "number", "boolean", "null"] }
    }
  }
}
```

`id` must equal `windlass.verify.<severity>.<category>`. Unknown members, secret/token values, an
`actual` value copied from a secret, or a schema violation fail with
`windlass.verify.error.diagnostics-contract-invalid`.

#### Producer diagnostic metadata extension

`diagnostic_metadata`, when present, is a closed object containing only the optional closed
`package_manifest` object. It records raw, non-trust package metadata for diagnostic reports. It
does not change verifier policy, publication policy, or provenance. In particular, no raw package
metadata belongs in SLSA `internalParameters` or `externalParameters`; the separately normalized
`externalParameters.package.repository` value is defined by the npm producer specification.

`package_manifest` may contain only the following members. Absent source fields are omitted. `null`,
credentials, tokens, unknown members, and duplicate JSON members fail with
`windlass.verify.error.diagnostics-contract-invalid`.

| Member        | Closed source-preserving form                                                             |
| ------------- | ----------------------------------------------------------------------------------------- |
| `repository`  | string, or object with required string `type` and `url`, plus optional string `directory` |
| `license`     | string, or object with required string `type` and optional string `url`                   |
| `description` | string                                                                                    |
| `keywords`    | array of strings                                                                          |
| `author`      | string, or object with required string `name` and optional string `email` and `url`       |
| `homepage`    | string                                                                                    |

The object alternatives in this table are closed, and the report preserves the selected JSON form
rather than normalizing an object to a string or a string to an object. A field with another JSON
type, an unknown object member, an empty credential-bearing URL, a token-like member or value, or a
duplicate member is invalid and fails with `windlass.verify.error.diagnostics-contract-invalid`.

Valid diagnostic metadata example:

```json
{
  "schema_version": "1",
  "result": "pass",
  "exit_code": 0,
  "primary_id": null,
  "run_invocation": "https://github.com/example/acme-widget/actions/runs/123456789/attempts/2",
  "diagnostics": [],
  "diagnostic_metadata": {
    "package_manifest": {
      "repository": { "type": "git", "url": "https://github.com/example/acme-widget.git" },
      "license": "Apache-2.0",
      "description": "Example package.",
      "keywords": ["example", "widget"],
      "author": { "name": "Example Maintainer", "email": "maintainer@example.com" },
      "homepage": "https://example.com/acme-widget"
    }
  }
}
```

Invalid diagnostic metadata example, which fails with
`windlass.verify.error.diagnostics-contract-invalid`:

```json
{
  "diagnostic_metadata": {
    "package_manifest": {
      "repository": { "type": "git", "url": "https://token@example.com/acme-widget.git" },
      "private_note": "not safe"
    }
  }
}
```

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
empty string), then the RFC 8785 JCS canonical JSON serialization of `actual`. `primary_id` is the
first ordered error. Implementations report every independently provable diagnostic that is safe to
evaluate, but they must not derive secondary diagnostics from unauthenticated or ambiguously parsed
content. A duplicate-member parse error therefore prevents semantic diagnostics for that JSON value;
a failed manifest authentication prevents manifest-intersection diagnostics; and a missing
certificate prevents extension mismatch diagnostics. Ordering or suppression contrary to these rules
fails the fixture harness with `windlass.verify.error.diagnostics-contract-invalid`.

### Exit codes and warning behavior

| Exit code | Class              | Required result and behavior                                                                                                             |
| --------- | ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `0`       | Verified           | `result` is `pass`; zero or more registered `warning` diagnostics may be present.                                                        |
| `1`       | Verification error | `result` is `fail`; at least one registered policy, cryptographic, identity, schema, artifact, or fixture-check error exists.            |
| `2`       | Invocation failure | `result` is `fail`; required local input is unreadable, invocation mode is unusable, or the verifier cannot execute the requested check. |

Exit code `2` does not mean the artifact failed a completed cryptographic check; it means no valid
acceptance decision was produced and the report contains `windlass.verify.error.input-unavailable`,
`windlass.verify.error.verifier-execution-failure`, or
`windlass.verify.error.verification-mode-invalid` as its primary diagnostic. Producer-side
publication gates treat both `1` and `2` as fatal and stop before mutation. Consumer automation
treats both as non-acceptance. Warnings are non-fatal only with exit code `0`, remain structurally
distinguishable through `severity: "warning"`, and must not hide, replace, or lower the exit status
of any error. Violating this mapping fails the fixture harness with
`windlass.verify.error.diagnostics-contract-invalid`.

## Accepted fixtures

Every accepted fixture has `expected-primary-id: null`; every rejected mutation contract in TDD
usage declares its non-null `expected-primary-id` explicitly.

| Name                                                | Surface          | Description                                                              |
| --------------------------------------------------- | ---------------- | ------------------------------------------------------------------------ |
| `npm-valid-release`                                 | npm              | Valid npm package tarball with matching Windlass provenance.             |
| `npm-valid-scoped-package-url`                      | npm              | Valid scoped npm package with registry package-version URL.              |
| `npm-actions-attest-custom-bundle-valid`            | npm              | Custom-mode emitted bundle is accepted as npm provenance file.           |
| `npm-resolved-lockfile-valid`                       | npm              | Selected lockfile descriptor matches path, digest, and manager.          |
| `npm-resolved-lockfile-stale-valid`                 | npm              | Stale non-selected lockfiles are recorded as diagnostics only.           |
| `npm-release-asset-mode-valid`                      | npm              | Public npm workflow release-asset mode uploads tarball + sidecar.        |
| `npm-release-asset-linked-metadata-valid`           | npm              | Release-asset mode creates linked artifact metadata when enabled.        |
| `package-repository-forms-valid`                    | npm              | Every accepted raw form normalizes to the caller source identity.        |
| `diagnostic-package-manifest-valid`                 | npm              | Safe-listed raw metadata is reported through `diagnostic_metadata`.      |
| `publisher-valid-upload`                            | publisher        | Valid producer handoff leading to release asset and sidecar.             |
| `publisher-mutable-target-valid`                    | publisher        | A mutable or draft target accepts the required asset set.                |
| `publisher-immutable-same-run-valid`                | publisher        | Complete digest-proven same-`run_id` assets converge read-only.          |
| `composition-valid-npm-tarball`                     | composition      | npm tarball successfully composes with publisher.                        |
| `composition-mutable-target-valid`                  | composition      | A mutable or draft target permits the composed publication path.         |
| `composition-immutable-same-run-valid`              | composition      | A complete verified same-`run_id` target converges read-only.            |
| `release-manifest-valid`                            | release-manifest | Signed manifest with valid producer and publisher mappings.              |
| `release-manifest-mutable-target-valid`             | release-manifest | A mutable or draft target accepts the manifest pair.                     |
| `release-manifest-immutable-same-run-valid`         | release-manifest | A complete verified same-`run_id` manifest pair converges read-only.     |
| `registered-npm-build-type-valid`                   | publisher        | The registered npm producer `buildType` selects its fixed policy.        |
| `npm-maximal-identity-valid`                        | npm              | All six ADR 0068 identity bindings match immutable policy keys.          |
| `npm-offline-transparency-valid`                    | npm              | SCT, Rekor proof, SET, and signing time verify without log calls.        |
| `release-manifest-pinned-root-valid`                | release-manifest | Authenticated fresh pin verifies the manifest bundle offline.            |
| `npm-resolved-dependencies-npm-valid`               | npm              | Name-keyed `lockfile` and runner image only; no manager distribution.    |
| `npm-resolved-dependencies-pnpm-valid`              | npm              | pnpm has one registry-integrity manager distribution.                    |
| `npm-resolved-dependencies-yarn-valid`              | npm              | Yarn has one download-hash manager distribution.                         |
| `npm-builder-version-direct-npm-valid`              | npm              | Closed builder version contains `nodejs` only.                           |
| `npm-builder-version-corepack-valid`                | npm              | Closed builder version contains `nodejs` and conditional `corepack`.     |
| `npm-builder-signing-adapter-valid`                 | npm              | Exactly one governed `sigstore-go` signing-adapter dependency.           |
| `npm-go-signer-bundle-valid`                        | npm              | Go signer payload exactly matches the preassembled Statement.            |
| `npm-external-parameters-distribution-caller-valid` | npm              | Closed distribution and caller groups complete the nine-input mapping.   |
| `npm-source-ref-dispatch-retry-valid`               | npm              | Built tag identity with a signed invocation record for the dispatch ref. |
| `npm-source-ref-omitted-valid`                      | npm              | Omitted `source-ref`: no invocation record; single-identity contract.    |
| `npm-internal-parameters-empty-valid`               | npm              | `internalParameters` is exactly `{}`.                                    |
| `npm-invocation-id-certificate-uri-valid`           | npm              | `metadata.invocationId` byte-equals Fulcio OID `.21`.                    |
| `trust-root-online-tuf-valid`                       | npm              | Online mode authenticates current TUF metadata without a pin.            |
| `trust-root-offline-pinned-valid`                   | npm              | Offline mode uses a fresh pinned root without network access.            |

## Rejected fixture categories

| Category                                             | Description                                                                                                                    |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `digest-mismatch`                                    | Artifact digest does not match the provenance subject digest.                                                                  |
| `signature-mismatch`                                 | Bundle signature is invalid or missing.                                                                                        |
| `signer-mismatch`                                    | Signer identity is not trusted.                                                                                                |
| `issuer-mismatch`                                    | Certificate OIDC issuer is not the exact GitHub Actions issuer.                                                                |
| `signer-workflow-path-mismatch`                      | SAN or Build Signer URI does not identify the exact expected workflow path.                                                    |
| `signer-workflow-sha-mismatch`                       | Build Signer Digest does not equal the manifest/policy workflow SHA.                                                           |
| `signer-identity-claim-missing`                      | Required semantic signer or source identity cannot be proven from verified bundle data.                                        |
| `source-numeric-id-mismatch`                         | Source repository or owner numeric ID is missing, malformed, or mismatched.                                                    |
| `source-digest-mismatch`                             | Certificate Source Repository Digest differs from the signed invocation record revision.                                       |
| `source-ref-mismatch`                                | Certificate Source Repository Ref differs from the signed invocation record ref.                                               |
| `source-ref-invalid`                                 | A supplied `source-ref` is not a full tag ref, does not resolve, mismatches the version, or conflicts with the invocation tag. |
| `run-invocation-uri-invalid`                         | Run Invocation URI is missing, malformed, or identifies another repository.                                                    |
| `self-hosted-runner`                                 | Runner identity is missing, unknown, caller-asserted, or not GitHub-hosted.                                                    |
| `missing-rekor-entry`                                | Bundle lacks a valid bundle-contained Rekor inclusion proof or SET binding.                                                    |
| `missing-sct`                                        | Fulcio certificate lacks a valid embedded SCT.                                                                                 |
| `signature-time-violation`                           | SET-covered integrated time is invalid or outside certificate validity.                                                        |
| `ungoverned-trust-root`                              | Trust root is not the authenticated Sigstore public good TUF root or allowed pin.                                              |
| `stale-pinned-trust-root`                            | Offline pinned trusted root is used after its documented `refresh_before` deadline.                                            |
| `legacy-trust-root-override`                         | A forbidden per-component Sigstore environment override can affect verification.                                               |
| `verification-network-call`                          | Required verification attempts to query Rekor, Fulcio, or a log operator.                                                      |
| `policy-schema-invalid`                              | Explicit policy or manifest expectation is missing, malformed, or contains unknown fields.                                     |
| `diagnostics-contract-invalid`                       | Machine-readable diagnostics violate the ID, shape, order, severity, or exit-code contract.                                    |
| `input-unavailable`                                  | A required local artifact, bundle, policy, manifest, or trusted-root input is unreadable.                                      |
| `verifier-execution-failure`                         | The verifier cannot execute a requested check and therefore produces no acceptance result.                                     |
| `duplicate-json-member`                              | Signed Statement, bundle, or DSSE JSON contains duplicate object member names.                                                 |
| `actions-attest-adapter-contract`                    | Exact DSSE payload, emitted bundle, or npm provenance-file compatibility is invalid.                                           |
| `wrong-producer-signer`                              | Producer signer repo, workflow path, ref, or issuer is not trusted.                                                            |
| `wrong-predicate-type`                               | `predicateType` is not SLSA provenance v1.                                                                                     |
| `wrong-manifest-predicate-type`                      | Release manifest `predicateType` is not the ADR 0054 predicate URI.                                                            |
| `wrong-builder-id`                                   | `builder.id` is not trusted or uses a non-SHA reference.                                                                       |
| `wrong-build-type`                                   | `buildType` is not the canonical profile URI.                                                                                  |
| `subject-cardinality-error`                          | Provenance contains zero subjects or multiple subjects.                                                                        |
| `npm-purl-subject-mismatch`                          | npm provenance subject is missing, malformed, or not the expected Package URL.                                                 |
| `tarball-filename-subject-rejected`                  | npm provenance uses the tarball filename as the Statement subject.                                                             |
| `missing-subject-sha512`                             | npm provenance subject omits the required tarball SHA-512 digest.                                                              |
| `missing-subject-sha256`                             | Provenance subject omits the required tarball SHA-256 digest.                                                                  |
| `unexpected-external-parameters`                     | `externalParameters` contains unexpected fields under strict matching.                                                         |
| `source-identity-mismatch`                           | Source repository or revision does not match policy.                                                                           |
| `release-ref-mismatch`                               | Source ref, release ref, built ref, or version tag do not identify the same tag.                                               |
| `source-repository-canonicalization-error`           | Source repository URL is non-canonical, ambiguous, or malformed.                                                               |
| `trusted-publisher-mismatch`                         | Producer-side npm trusted publishing caller identity or OIDC permission is wrong.                                              |
| `package-identity-mismatch`                          | npm package name or version does not match.                                                                                    |
| `package-url-mismatch`                               | npm registry package-version URL is malformed or does not match registry/name/version.                                         |
| `unsupported-initial-publication`                    | Selected package identity does not already exist on npmjs.                                                                     |
| `package-version-mismatch`                           | Tag version does not match `package.json` version.                                                                             |
| `package-directory-mismatch`                         | `externalParameters.package.directory` does not match expected.                                                                |
| `package-manager-selection-path-mismatch`            | Package-manager selection path is missing or wrong in provenance.                                                              |
| `private-package`                                    | Selected package manifest has `private: true`.                                                                                 |
| `publish-intent-conflict`                            | Workflow publish input conflicts with source `publishConfig`.                                                                  |
| `invalid-publish-input`                              | Non-empty workflow publish input has an unsupported value or format.                                                           |
| `empty-publish-input-fallback`                       | Empty workflow input failed to fall back to source `publishConfig`.                                                            |
| `already-published-version`                          | Selected package name/version already exists before publish.                                                                   |
| `workspace-resolution-mismatch`                      | Workspace root, package manager root, or lockfile policy is wrong.                                                             |
| `workspace-pattern-base-mismatch`                    | Workspace patterns were evaluated against the wrong base directory.                                                            |
| `workspace-command-mismatch`                         | Workspace package targeting command can affect the wrong package.                                                              |
| `package-manager-manifest-shape-error`               | `devEngines.packageManager` uses an unsupported shape, member, or release version form.                                        |
| `unsupported-yarn-version`                           | Yarn is Classic 1.x, Berry v2, Berry v3, non-exact, or selected from an unsupported source.                                    |
| `resolved-dependencies-lockfile`                     | Selected lockfile `resolvedDependencies` descriptor is missing, malformed, or mismatched.                                      |
| `release-asset-mode-schema-error`                    | Public npm release-asset mode input or output schema is invalid.                                                               |
| `release-asset-mode-disabled-conflict`               | Release-asset-only inputs are supplied while release-asset mode is disabled.                                                   |
| `release-asset-mode-permission-error`                | Caller or internal job permissions are missing or combine separated authorities.                                               |
| `release-asset-target-error`                         | Effective release tag or target release is missing, malformed, or outside the caller repo.                                     |
| `runtime-policy-mismatch`                            | Runner or Node.js version does not match policy.                                                                               |
| `excessive-publish-permission`                       | npmjs publish job requests permissions outside the initial boundary.                                                           |
| `npm-version-too-old`                                | npm CLI version is below `11.5.1` for trusted publishing.                                                                      |
| `release-manifest-mismatch`                          | Release manifest mapping does not match the provenance.                                                                        |
| `trusted-producer-policy-conflict`                   | Explicit verifier policy and signed release manifest policy cannot both be satisfied.                                          |
| `manifest-predicate-mismatch`                        | Signed Statement predicate differs from canonical manifest JSON.                                                               |
| `manifest-digest-mismatch`                           | Statement subject digest differs from canonical manifest JSON bytes.                                                           |
| `manifest-trigger-mismatch`                          | Release manifest workflow did not run from the expected protected SemVer tag.                                                  |
| `manifest-tag-peel-mismatch`                         | Release tag cannot be peeled to the expected terminal commit.                                                                  |
| `manifest-entrypoint-mismatch`                       | Release manifest signer workflow path is not the fixed production entrypoint.                                                  |
| `manifest-caller-override`                           | Caller-controlled input changed a signed manifest trust field.                                                                 |
| `manifest-workflow-sha-mismatch`                     | Schema v1 workflow SHA does not equal the release tag target commit.                                                           |
| `manifest-entry-order-mismatch`                      | Release manifest producer or publisher arrays are not in canonical sorted order.                                               |
| `manifest-generated-at-invalid`                      | `generated_at` is not a fixed-form UTC timestamp.                                                                              |
| `manifest-handoff-basename-mismatch`                 | Manifest handoff artifact contains an unexpected payload basename.                                                             |
| `manifest-signing-input-mismatch`                    | Manifest signing input metadata is malformed or does not bind verified signing inputs.                                         |
| `manifest-partial-json-uploaded`                     | Plain manifest JSON uploaded but signed bundle upload failed.                                                                  |
| `manifest-indeterminate-json-upload`                 | Manifest upload state cannot be determined after an ambiguous upload attempt.                                                  |
| `manifest-remote-digest-unproven`                    | Same-name remote manifest asset exists but SHA-256 equality cannot be proven.                                                  |
| `release-target-immutable`                           | An immutable target lacks a complete, verified same-`run_id` required asset set.                                               |
| `bundle-byte-format-mismatch`                        | Signed bundle bytes were extracted, reserialized, wrapped, or otherwise changed.                                               |
| `missing-producer-provenance`                        | Publisher receives an artifact without producer provenance.                                                                    |
| `raw-artifact-bypass`                                | Raw caller artifact bypasses producer verification.                                                                            |
| `handoff-schema-mismatch`                            | Cross-job artifact handoff omits or changes required core fields.                                                              |
| `composition-handoff-substitution`                   | Composition mapping trusts public outputs or deterministic names.                                                              |
| `publisher-handoff-field-error`                      | Publisher handoff uses missing, stale, or malformed field names.                                                               |
| `release-asset-binding-mismatch`                     | Producer policy cannot bind release asset name to verified producer artifact bytes.                                            |
| `linked-artifact-settings-mismatch`                  | Linked artifact settings do not match the target repository, release tag, or download URL.                                     |
| `linked-artifact-locator-mismatch`                   | Linked artifact locator outputs are missing, set in the wrong state, or malformed.                                             |
| `publisher-workflow-schema-error`                    | Publisher exposes or accepts unsupported public workflow inputs or secrets.                                                    |
| `publisher-permission-boundary-violation`            | Publisher job permissions combine authorities that must stay separate.                                                         |
| `native-locator-malformed`                           | Native provenance locator is not valid diagnostic metadata.                                                                    |
| `native-locator-digest-mismatch`                     | Native provenance locator digest differs from the sidecar bundle digest.                                                       |
| `sidecar-mismatch`                                   | Sidecar bundle does not match the primary asset's provenance.                                                                  |
| `sidecar-digest-mismatch`                            | `sidecar-digest` does not equal the verified producer bundle SHA-256.                                                          |
| `sidecar-upload-partial-failure`                     | Primary release asset uploaded but sidecar upload failed afterward.                                                            |
| `publisher-indeterminate-primary-upload`             | Primary release asset upload state cannot be determined after an ambiguous upload.                                             |
| `publisher-remote-digest-unproven`                   | Same-name remote release asset exists but SHA-256 equality cannot be proven.                                                   |
| `duplicate-release-asset`                            | Release asset name already exists.                                                                                             |
| `duplicate-sidecar-asset`                            | Deterministic sidecar asset name already exists before upload.                                                                 |
| `registry-linkage-mismatch`                          | Published package does not match the provenance registry metadata.                                                             |
| `custom-registry-token-required`                     | Custom registry metadata or publish response proves a token or OTP is required.                                                |
| `custom-registry-provenance-weakened`                | Custom registry publish omits, rewrites, re-signs, substitutes, or auto-generates provenance.                                  |
| `custom-registry-tokenless-auth-failed`              | Tokenless authentication fails at the custom-registry authentication or publish boundary.                                      |
| `custom-registry-access-option-rejected`             | Custom registry rejects caller `access` during tokenless publish without proving token/OTP need.                               |
| `custom-registry-provenance-submission-rejected`     | Registry rejects the exact external provenance file.                                                                           |
| `custom-registry-linkage-metadata-absent`            | Required linkage metadata is absent after custom-registry publication.                                                         |
| `custom-registry-digest-semantics-mismatch`          | Registry digest evidence is absent, malformed, incompatible, or mismatched.                                                    |
| `package-repository-identity-mismatch`               | Raw package repository metadata is missing, malformed, or normalizes to another repository.                                    |
| `unregistered-producer-build-type`                   | Producer `buildType` is unknown to the closed publisher policy registry.                                                       |
| `verification-mode-invalid`                          | Invocation mode is absent, multiple, conflicting, or incompatible with trust-root shape.                                       |
| `unexpected-internal-parameters`                     | `internalParameters` is non-object or contains one or more members.                                                            |
| `resolved-dependencies-unexpected-entry`             | A dependency descriptor has an unknown name or is a non-enumerated extra entry.                                                |
| `resolved-dependencies-package-manager-distribution` | The known conditional manager-distribution descriptor is missing, duplicated, forbidden, or malformed.                         |
| `resolved-dependencies-runner-image`                 | The runner-image descriptor is missing, duplicated, malformed, digest-bearing, or mismatched.                                  |
| `builder-version-mismatch`                           | Closed `builder.version` keys or observed versions are invalid.                                                                |
| `builder-dependencies-signing-adapter-mismatch`      | The sole signing-adapter builder dependency is missing, extra, malformed, or dependency-inconsistent.                          |
| `mutation-queue-overflow`                            | GitHub rejects an arrival beyond 100 pending executions in one mutation concurrency group.                                     |
| `npm-oidc-exchange-indeterminate`                    | npm OIDC token exchange is unreadable or errors with HTTP `5xx` or a malformed response before registry mutation.              |
| `oidc-capability-unavailable`                        | Caller lacks OIDC capability because the id-token request token is absent or the id-token request fails.                       |
| `mutation-permission-denied`                         | First mutating call receives definitive HTTP `403` or `401`, proving no mutation occurred.                                     |
| `prepublish-registry-metadata-required`              | Workflow required post-publish registry metadata before publish.                                                               |
| `release-version-semver-mismatch`                    | Release manifest version or tag is not valid SemVer 2.0.0.                                                                     |
| `trusted-core-boundary-violation`                    | Trusted policy/provenance logic depends on profile ecosystem tooling.                                                          |

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
preflight checks are best-effort diagnostics. A tokenless metadata check that cannot prove state
records `publish.package_identity_preexisting` or `publish.package_version_preexisting` as `null`,
sets `publish.custom_registry_support` to `unsupported-but-not-blocked`, emits
`custom-registry-preflight-inconclusive`, and exits `0`. Synthetic custom-registry fixtures test
only this warning behavior and fail-clearly behavior. They never establish successful registry
conformance.

Rejected custom-registry fixtures must cover token or OTP requirement detected before mutation,
tokenless OIDC or equivalent authentication failure at the authentication or publish boundary, exact
external provenance-file rejection, caller `access` rejection without token or OTP proof, caller
`access` rejection with an ambiguous cause, bundle omission, rewriting, re-signing, or
automatic-provenance fallback, absent linkage metadata after publication, and absent, malformed,
incompatible, or mismatched digest semantics. Their primary diagnostics are respectively
`custom-registry-token-required`, `custom-registry-tokenless-auth-failed`,
`custom-registry-provenance-submission-rejected`, `custom-registry-access-option-rejected`,
`custom-registry-access-option-rejected`, `custom-registry-provenance-weakened`,
`custom-registry-linkage-metadata-absent`, and `custom-registry-digest-semantics-mismatch`. These
fixtures exit `1`. The `custom-registry-tokenless-auth-failed` diagnostic is reserved exclusively
for a proved tokenless authentication failure; access-option rejection must not use it. A required
token, OTP, automatic provenance fallback, or caller-policy widening attempt must fail rather than
continue with a fallback.

The v1 provenance contract rejected fixture set must cover these isolated mutations. Every rejected
fixture declares the listed value as `expected-primary-id`; independently provable secondary errors
follow the diagnostics ordering rules.

| Mutation                                                                        | Expected primary ID                                  |
| ------------------------------------------------------------------------------- | ---------------------------------------------------- |
| npm emits `package-manager-distribution`                                        | `resolved-dependencies-package-manager-distribution` |
| pnpm or Yarn omits `package-manager-distribution`                               | `resolved-dependencies-package-manager-distribution` |
| pnpm uses `download-hash` or Yarn uses `registry-integrity`                     | `resolved-dependencies-package-manager-distribution` |
| Descriptor manager or version differs from `externalParameters.package_manager` | `resolved-dependencies-package-manager-distribution` |
| Runner image is absent or contains `digest`                                     | `resolved-dependencies-runner-image`                 |
| Unknown descriptor or generated transitive package entry                        | `resolved-dependencies-unexpected-entry`             |
| `builder.version.nodejs` is missing                                             | `builder-version-mismatch`                           |
| `corepack` is missing when used or present for npm                              | `builder-version-mismatch`                           |
| `builder.version` has an unknown key                                            | `builder-version-mismatch`                           |
| Signing-adapter descriptor is missing or extra                                  | `builder-dependencies-signing-adapter-mismatch`      |
| Adapter URI, module version, or checksum disagrees                              | `builder-dependencies-signing-adapter-mismatch`      |
| Adapter `role` annotation is wrong                                              | `builder-dependencies-signing-adapter-mismatch`      |
| `distribution` or `caller` member is missing or unknown                         | `unexpected-external-parameters`                     |
| Raw release tag is duplicated into `distribution`                               | `release-asset-mode-schema-error`                    |
| Observed caller filename is unavailable or mismatched                           | `trusted-publisher-mismatch`                         |
| `internalParameters` is nonempty                                                | `unexpected-internal-parameters`                     |
| Invocation URI is malformed or differs from OID `.21`                           | `run-invocation-uri-invalid`                         |
| Verification mode is absent, conflicting, or root-shape-inconsistent            | `verification-mode-invalid`                          |
| Online TUF failure attempts pin fallback                                        | `ungoverned-trust-root`                              |
| Offline mode attempts TUF or another network call                               | `verification-network-call`                          |
| Offline pin is stale                                                            | `stale-pinned-trust-root`                            |

The package repository fixture set must cover accepted shorthand `owner/repository` and
`github:owner/repository`, HTTPS, `git+https`, `git://`, SCP-like SSH, `ssh://`, and object-form
repository values. It must also cover malformed forms, a missing repository, and a normalized value
that differs from the observed source repository. Each rejection fails before pack with
`package-repository-identity-mismatch`. Fixtures for the diagnostics report must preserve each
safe-listed source JSON form, omit absent fields, and reject `null`, credentials, tokens, unknown
members, and duplicate members with `diagnostics-contract-invalid`.

The producer-policy registry fixture set must accept the registered npm `buildType`, reject an
unknown `buildType` with `unregistered-producer-build-type`, and reject every caller override or
union attempt with `trusted-producer-policy-conflict` or the narrower field diagnostic. No fixture
may admit a producer by caller input alone.

The workspace fixture set must include nested workspace roots and prove that workspace patterns are
evaluated relative to each candidate workspace root, not relative to the repository root. A fixture
whose pattern only matches under the wrong base path must fail with
`workspace-pattern-base-mismatch`. Under ADR 0078, the accepted workspace fixtures must also include
a settings-only `pnpm-workspace.yaml` with no `packages` member that resolves exactly the root
`package.json` as a standalone root package and does not infer any subpackage member. A rejected
fixture must select a subdirectory beneath that root and fail with `package-resolution-invalid`.

The workspace fixture set must prove the initial limited glob semantics. Accepted fixtures must
cover `*` matching exactly one path segment, `**` matching one or more nested segments, `**`
matching zero segments only when the selected package directory is the candidate root and contains
`package.json`, nested workspace roots where the nearest claiming ancestor wins, and multiple
patterns that resolve to the same selected package directory. Rejected fixtures must cover
`packages/*` incorrectly matching `packages/a/b`, a pattern matching a descendant or ancestor rather
than the exact selected package directory, a selected package claimed by workspace metadata but
missing its own `package.json`, patterns that resolve to different package directories for one
input, a non-object `pnpm-workspace.yaml`, a present pnpm `packages` member with a non-array value
or non-string member, unsupported `workspaces` shapes, negation, brace expansion, extended glob
syntax, absolute paths, empty path segments, traversal segments, and backslash separators. Pattern
base failures use `workspace-pattern-base-mismatch`; malformed metadata or non-exact package
selection failures use `package-resolution-invalid` unless a narrower category applies. An absent
pnpm `packages` member must not appear in the rejected corpus.

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

The npm Go-signer fixture set must prove the ADR 0077 contract. The accepted production fixture must
be a bundle named `<package-tarball-name>.intoto.jsonl` with GitHub Actions OIDC identity, a Fulcio
certificate and embedded SCT, bundle-contained Rekor evidence, and an extracted DSSE payload exactly
equal to the preassembled one-subject, dual-digest Statement bytes. It must verify offline and be
accepted by the npm `--provenance-file` path. Rejected fixtures must cover a reconstructed or
reserialized Statement payload, wrong payload type, raw Statement files used as provenance,
reserialized or wrapped bundles, GitHub artifact attestation storage locators substituted for bundle
bytes, wrong predicate type, missing or renamed emitted bundle file, unparseable Sigstore bundle
bytes, and npm CLI rejection of the external provenance file before registry mutation. These
failures retain the registered `actions-attest-adapter-contract` ID unless the narrower
wrong-predicate, bundle-byte-format, signer, transparency, or duplicate-member category applies. The
ID name is historical: the C03 diagnostic registry is a closed machine contract, so renaming it or
adding a signer-specific replacement is deferred to P02, which must update the registry and this
taxonomy atomically.

The existing F03 `actions/attest` two-subject bundle remains unchanged under
`testdata/platform/contracts/` as historical platform evidence. It is not an accepted production
signer fixture and must continue to demonstrate that one checksum line per algorithm creates
multiple subjects. ADR 0077 implementation adds the separate `npm-go-signer-bundle-valid` fixture;
it must not modify F03 evidence to make the stock action appear conformant.

Before production enablement, controlled registry conformance must also prove a real npm publish,
registry attestation read-back of the same bundle, and pacote consumer verification of the published
package and provenance. Dry-run acceptance alone does not satisfy this production-signer fixture
gate.

This npm-shaped evidence is the first confirmation of the shared signer contract. Release-manifest
fixtures must prove that M01/M02 use the same Go-native adapter and preserve the exact preassembled
manifest Statement bytes; each future profile must add equivalent exact-payload and bundle fixtures
when its contract is admitted. If the controlled publish, P06, or pacote verification surfaces a
signer defect, a follow-up ADR must evaluate remediation or rollback.

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

| Fixture name                              | Mutation                                                                          | Expected diagnostic          |
| ----------------------------------------- | --------------------------------------------------------------------------------- | ---------------------------- |
| `transparency-rekor-entry-missing`        | Bundle omits the Rekor inclusion proof or SET/log-entry binding.                  | `missing-rekor-entry`        |
| `transparency-rekor-entry-mismatch`       | Included entry binds another signature or certificate.                            | `missing-rekor-entry`        |
| `transparency-sct-missing`                | Fulcio leaf certificate omits its embedded SCT.                                   | `missing-sct`                |
| `transparency-signature-time-before-cert` | SET-covered `integratedTime` is before the certificate validity interval.         | `signature-time-violation`   |
| `transparency-signature-time-after-cert`  | SET-covered `integratedTime` is after the certificate validity interval.          | `signature-time-violation`   |
| `transparency-tsa-only`                   | A valid RFC 3161 timestamp is present, but Rekor evidence is absent.              | `missing-rekor-entry`        |
| `trust-root-pinned-stale-deadline`        | Verification time is after the policy's `refresh_before` timestamp.               | `stale-pinned-trust-root`    |
| `trust-root-mode-absent`                  | No invocation-level verification mode is selected.                                | `verification-mode-invalid`  |
| `trust-root-mode-conflict`                | CLI, configuration, or environment sources select disagreeing modes.              | `verification-mode-invalid`  |
| `trust-root-online-tuf-fallback`          | Online TUF authentication fails and the verifier attempts a pinned-root fallback. | `ungoverned-trust-root`      |
| `trust-root-offline-network-attempt`      | Offline verification attempts TUF acquisition or another network call.            | `verification-network-call`  |
| `trust-root-offline-pinned-stale`         | Offline pinned root is used after `refresh_before`.                               | `stale-pinned-trust-root`    |
| `trust-root-legacy-override`              | A prohibited component-key environment override can change root selection.        | `legacy-trust-root-override` |
| `transparency-required-online-query`      | The pass/fail path attempts a Rekor, Fulcio, or CT-log network call.              | `verification-network-call`  |

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
`externalParameters.package_manager.ignored_lockfile_paths` and the `lockfile` descriptor's
`annotations.stale_non_selected_lockfiles` while keeping the selected lockfile descriptor as the
only dependency graph input. These existing lockfile fixtures run within the enumerated name-keyed
set: npm has `lockfile` plus `runner-image`; pnpm and Yarn also have one
`package-manager-distribution`. Rejected fixtures must cover a missing descriptor, extra descriptor
entries, full dependency-list entries, digest mismatch, URI source or revision mismatch, fragment
path outside the repository, descriptor path that names a stale or non-selected lockfile, missing
stale-lockfile diagnostics, unknown annotation members, and treating a stale non-selected lockfile
as the selected dependency graph input. These failures use `resolved-dependencies-lockfile`; unknown
extra entries use `resolved-dependencies-unexpected-entry`; known package-manager distribution and
runner-image defects use their respective narrow diagnostics.

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

The composed npm fixture set must accept a mutable or draft target. It must reject an immutable
target with any required asset absent using `release-target-immutable`, accept a complete,
digest-proven same-`run_id` asset set only as read-only convergence, and reject cross-run or
indeterminate immutable evidence with `release-target-immutable`. Each immutable rejection exits `1`
before any composed publication mutation.

The handoff fixture set must prove that every cross-job artifact handoff includes the core semantic
fields `transport`, `artifact_name`, `payload_file_name`, `payload_kind`, `digest.algorithm`, and
`digest.value`, whether those fields are public inputs or profile-owned fixed mappings. Missing or
malformed fields fail with `handoff-schema-mismatch`.

The signed bundle fixture set must prove that npm Go-signer bundle bytes are preserved byte-for-byte
through npm provenance submission and publisher sidecar redistribution, while release-manifest
fixtures preserve the bundle bytes emitted by that path's selected adapter. Fixtures that extract
only the Statement, reserialize the bundle, wrap it in a new envelope, or substitute a native
attestation locator for the bundle file must fail with `bundle-byte-format-mismatch` or the narrower
sidecar/provenance mismatch category.

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
The npm signing permissions fixture must likewise prove `contents: read`, `id-token: write`, and no
`attestations: write`, publish permission, caller-controlled step, or long-lived credential.

The trusted-publisher fixture set is producer-side: it must prove that missing caller OIDC
permission or npmjs.com trusted publisher mismatch fails before registry mutation and never falls
back to token publish. These fixtures must not require adding caller trusted publisher settings to
the signed SLSA `externalParameters` schema.

The ADR 0076 trusted-publisher preflight fixture set must include an exchange-preflight failure
matrix: HTTP `401` and `404` each produce `trusted-publisher-mismatch`; HTTP `5xx` and a malformed
exchange response each produce `npm-oidc-exchange-indeterminate`. Every matrix case fails before any
registry mutation because the exchange only mints a short-lived publish token. The set must also
include an id-token env-absence capability probe in which a job without `id-token: write` observes
an absent `ACTIONS_ID_TOKEN_REQUEST_TOKEN` and fails with `oidc-capability-unavailable` before any
mutation. A filename cross-check mismatch fixture must prove that the final path segment of the OIDC
`workflow_ref` claim disagrees with the caller-scoped `github.workflow_ref` context and fails with
`trusted-publisher-mismatch` before signing and registry mutation.

The publisher convergence fixture set must classify the primary asset and sidecar independently,
before mutation and after an ambiguous mutating call, into exactly `committed-as-expected`,
`absent`, `foreign-conflict`, or `indeterminate`. A same-`run_id` retry must treat
`committed-as-expected` as satisfied without upload, upload only an `absent` asset, and fail without
mutation on `foreign-conflict` or `indeterminate`, reporting the exact state and remote evidence. A
new-`run_id` fixture with any pre-existing same-name asset must report `foreign-conflict` and fail
without mutation even when the asset digest matches. Any fixture that cannot produce exactly one of
the four states fails as `indeterminate` with `publisher-indeterminate-primary-upload` or the
narrower step diagnostic.

The ADR 0076 publisher first-mutation fixture set must prove that a definitive HTTP `403` on the
first mutating call fails with `mutation-permission-denied` and performs no ADR 0067 read-back,
because the rejection proves no mutation occurred. A separate ambiguous upload-result fixture must
perform ADR 0067 read-back and fail with `publisher-indeterminate-primary-upload` when the state
remains unresolved.

The ADR 0072 and ADR 0073 convergence fixture set must include the following fixtures. Each fixture
uses a re-run-failed-jobs attempt unless it explicitly names a fresh run. The Run Invocation URI
comparison fixtures hold every component constant except the named run-id or attempt component.

| Fixture name                                   | Surface   | Required observation and result                                                                                                                                                                                                                                                                                                                               |
| ---------------------------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `publisher-sidecar-first-crash-matrix`         | publisher | Exercise interruption after sidecar commit, during primary upload, after primary commit with a lost response, and after a confirmed response. The retry must read back the sidecar-first pair, recover the primary only when it is absent, and otherwise use the pair gate without overwrite or deletion.                                                     |
| `publisher-sidecar-first-ordering`             | publisher | The consolidated release-upload job commits and digest-verifies the sidecar before it starts the primary upload. No workflow-produced trace may contain a primary-present, sidecar-absent state.                                                                                                                                                              |
| `publisher-pair-gate-failures`                 | publisher | Independently fail each pair-gate condition: remote sidecar digest mismatch, full-policy sidecar verification failure, different Run Invocation URI run-id, signed primary name or digest binding mismatch, and remote primary digest mismatch. Each case is `foreign-conflict` or `indeterminate` as evidence requires, and performs no mutation.            |
| `publisher-cross-run-identical-bytes-conflict` | publisher | A fresh `run_id` observes byte-identical pre-existing primary and sidecar assets. It must fail closed as `foreign-conflict`, not converge.                                                                                                                                                                                                                    |
| `publisher-oob-identical-bytes-observation`    | publisher | Record that own-workflow and another-workflow byte-identical uploads have indistinguishable release-asset `digest`, size, name, state, content type, and bot uploader values. Record that no release-asset API field carries run identity, and that server IDs and timestamps do not bind an upload to a run. The fixture must not claim custody attribution. |
| `publisher-convergence-report-honesty`         | publisher | Confirm that reports distinguish `uploaded-with-confirmed-receipt`, `converged-from-prior-receipt`, and `converged-as-verified-pair-custody-unproven`. A successful verified-pair report without a matching asset-ID receipt must not assert that the current run uploaded the asset.                                                                         |
| `same-run-id-prior-attempt-convergence`        | npm       | A bundle signed by an earlier attempt with the same run-id converges. Its attempt component differs, its run-id component matches byte-for-byte, and its exact-version integrity, selected published attestation, npm Package URL, and tarball digest bindings all verify.                                                                                    |
| `npm-attestation-404-version-existence`        | npm       | A `404 {"error":"Not found"}` from the attestation endpoint is paired with the packument exact-version existence check. An absent version is not treated as attestation absence; an existing version with no selected attestation after polling is `foreign-conflict`.                                                                                        |
| `npm-attestation-selection-not-array-order`    | npm       | The desired custom `--provenance-file` or auto-provenance bundle is selected by `predicateType` from a reordered collection. A different predicate at the first array position must not affect the result.                                                                                                                                                    |

The publisher topology and cross-run-safety fixture set must also include the ADR 0074 fixtures:

| Fixture name                                            | Surface   | Required observation and result                                                                                                                                                                                                                                               |
| ------------------------------------------------------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `publisher-contending-runs-foreign-conflict`            | publisher | Two runs contend for one release surface. After the first committed state becomes visible, the other run's next mutation step detects `foreign-conflict`, reports its remote evidence, and performs no mutation.                                                              |
| `publisher-duplicate-asset-name-platform-rejection`     | publisher | A duplicate release-asset upload is rejected by the platform with the observed message `asset under the same name already exists`. The workflow maps the rejection through authoritative read-back to `foreign-conflict` or `indeterminate`, never to a successful overwrite. |
| `publisher-consolidated-job-sidecar-first-upload-order` | publisher | The one release-upload job, not separate jobs, contains the sidecar upload and then the primary upload. Static conformance rejects any graph that splits these same-surface mutations across jobs.                                                                            |

The mutation queue fixture set must include the ADR 0075 fixtures:

| Fixture name                                          | Surface   | Required observation and result                                                                                                                                                                                                                                          |
| ----------------------------------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `mutation-queue-three-rapid-same-intent-fifo`         | publisher | Three rapid same-intent dispatches enter a `queue: max` mutation group. All wait, none is cancelled, and their mutation jobs start in arrival order.                                                                                                                     |
| `mutation-queue-rerun-preserves-convergence-position` | publisher | A re-run-failed-jobs attempt waits behind a running segment. A fresh run arrives afterward. The retry keeps its queue position, converges as `committed-as-expected`, and the fresh run then enters and fails closed as `foreign-conflict`.                              |
| `mutation-queue-overflow-classification`              | publisher | More than 100 pending contenders cause the platform queue-overflow surface. The fixture records whether the platform exposes rejection or cancellation only after that behavior is pinned from documentation or a dedicated spike, then emits `mutation-queue-overflow`. |

The standalone publisher immutable-target fixture set must accept a mutable or draft target. It must
reject an immutable target with incomplete required assets using `release-target-immutable`, accept
a complete, verified same-`run_id` asset set only as read-only convergence, and reject cross-run or
indeterminate immutable evidence with `release-target-immutable`. Each rejection exits `1` before
upload.

Partial and ambiguous publisher fixtures must express each step through those same four states. For
example, a sidecar commit followed by a primary API or transport failure reports the sidecar as
`committed-as-expected` after authoritative read-back and the primary as `absent`,
`foreign-conflict`, or `indeterminate` according to its observed remote state. A primary state other
than `committed-as-expected` fails with `sidecar-upload-partial-failure` or the narrower state
diagnostic. An ambiguous primary upload reports `committed-as-expected` only after authoritative
digest equality, `absent` after bounded authoritative absence, `foreign-conflict` after
authoritative digest inequality, or `indeterminate` when presence or digest equality cannot be
proved within the publisher's
[release-asset digest polling contract](github-release-asset-publisher.md#release-asset-digest-binding-and-polling):
one immediate request, then every 5 seconds, with at most 24 observations and a 120-second cap from
the first request.

The publisher remote digest fixture set must prove that the GitHub Release asset `digest` field is
the sole authoritative release-asset binding. Accepted fixtures must cover an exact
`sha256:<64 lowercase hexadecimal characters>` match with `expected-sha256`. Rejected fixtures must
cover a missing or unreadable `digest`, an unsupported or malformed algorithm/value, API or
transport failure through the publisher's
[release-asset digest polling contract](github-release-asset-publisher.md#release-asset-digest-binding-and-polling),
contradictory observations, and a digest unequal to `expected-sha256`. An unequal authoritative
digest reports `foreign-conflict`; inability to read a usable authoritative digest reports
`indeterminate`. Downloaded bytes and a locally computed hash may appear only as diagnostic evidence
and must not change either result to `committed-as-expected`; doing so fails the fixture with
`publisher-remote-digest-unproven` or the narrower state diagnostic.

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

The release manifest convergence fixture set reports one pair-level outcome for the plain JSON asset
and signed bundle. Each asset retains an evidence substate, but no asset substate is an independent
manifest result. Pair reduction is deterministic: both assets absent reduce to `absent`; both
digest-proven assets whose semantic binding succeeds reduce to `committed-as-expected`; any proved
mismatch, cross-run ownership, or incomplete committed/absent pair reduces to `foreign-conflict`;
otherwise, any unresolved required evidence reduces to `indeterminate`.

A same-`run_id` retry treats the `committed-as-expected` pair as satisfied without upload and may
upload only an `absent` pair. It fails without mutation on a `foreign-conflict` or `indeterminate`
pair, reporting the pair outcome and both evidence substates. A new `run_id` with either existing
same-name manifest asset reduces to `foreign-conflict` and fails without mutation, even when a
digest matches. Failure to obtain required evidence reduces to `indeterminate` and fails with
`manifest-indeterminate-json-upload` or a narrower step diagnostic.

Partial and ambiguous manifest fixtures use this one pair result. A successful plain JSON upload
followed by signed-bundle absence is `foreign-conflict`; it reports `manifest-partial-json-uploaded`
as the aggregate failure diagnostic and does not create a fifth outcome. A signed-bundle API or
transport failure with unresolved evidence is `indeterminate`. Authoritative read-back can establish
an asset evidence substate only as provided by the remote digest rule below.

The release manifest immutable-target fixture set must accept a mutable or draft target. It must
reject an immutable target with incomplete required assets using `release-target-immutable`, accept
a complete, verified same-`run_id` pair only as read-only convergence, and reject cross-run or
indeterminate immutable evidence with `release-target-immutable`. Each rejection exits `1` before
manifest upload.

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

### v1 provenance completion traceability matrix

Each row names one primary diagnostic. The normative sources define the shape; this verifier policy
defines the accepted and rejected fixture contracts.

| Requirement                    | Normative section                                                                                                  | Accepted fixture                                    | Rejected fixture                                                    | Primary diagnostic                                   |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------- | ------------------------------------------------------------------- | ---------------------------------------------------- |
| npm dependency set             | [npm dependency schema](js-ts-npm-provenance-publish.md#jsts-npm-resolveddependencies-schema)                      | `npm-resolved-dependencies-npm-valid`               | npm emits a manager distribution                                    | `resolved-dependencies-package-manager-distribution` |
| pnpm distribution              | [Package-manager distribution descriptor](js-ts-npm-provenance-publish.md#package-manager-distribution-descriptor) | `npm-resolved-dependencies-pnpm-valid`              | pnpm distribution missing or uses `download-hash`                   | `resolved-dependencies-package-manager-distribution` |
| Yarn distribution              | [Package-manager distribution descriptor](js-ts-npm-provenance-publish.md#package-manager-distribution-descriptor) | `npm-resolved-dependencies-yarn-valid`              | Yarn distribution missing or uses `registry-integrity`              | `resolved-dependencies-package-manager-distribution` |
| Unknown dependency             | [npm dependency schema](js-ts-npm-provenance-publish.md#jsts-npm-resolveddependencies-schema)                      | `npm-resolved-dependencies-npm-valid`               | generated transitive or unknown entry                               | `resolved-dependencies-unexpected-entry`             |
| Runner image                   | [Runner-image descriptor](js-ts-npm-provenance-publish.md#runner-image-descriptor)                                 | `npm-resolved-dependencies-npm-valid`               | runner absent, digest-bearing, or mismatched                        | `resolved-dependencies-runner-image`                 |
| Capture availability           | [Runner image capture](js-ts-npm-build-pack.md#runner-image-capture)                                               | `npm-resolved-dependencies-npm-valid`               | runner or manager capture unavailable before predicate construction | `input-unavailable`                                  |
| Distribution and caller groups | [Closed schema rules](js-ts-npm-provenance-publish.md#closed-schema-rules)                                         | `npm-external-parameters-distribution-caller-valid` | missing or unknown group member                                     | `unexpected-external-parameters`                     |
| Mode normalization             | [Optional input rules](js-ts-npm-package-profile.md#optional-input-rules)                                          | `npm-external-parameters-distribution-caller-valid` | normalized distribution disagrees with public mode input            | `release-asset-mode-schema-error`                    |
| Caller filename                | [Caller trusted publishing requirements](js-ts-npm-package-profile.md#caller-trusted-publishing-requirements)      | `npm-external-parameters-distribution-caller-valid` | observed filename unavailable or mismatched                         | `trusted-publisher-mismatch`                         |
| Empty internal parameters      | [internalParameters](slsa-provenance-v1.md#internalparameters)                                                     | `npm-internal-parameters-empty-valid`               | non-object or nonempty value                                        | `unexpected-internal-parameters`                     |
| Builder direct-npm version     | [builder](slsa-provenance-v1.md#builder)                                                                           | `npm-builder-version-direct-npm-valid`              | missing `nodejs` or npm-only `corepack`                             | `builder-version-mismatch`                           |
| Builder Corepack version       | [builder](slsa-provenance-v1.md#builder)                                                                           | `npm-builder-version-corepack-valid`                | missing conditional `corepack` or observed mismatch                 | `builder-version-mismatch`                           |
| Signing adapter                | [builder](slsa-provenance-v1.md#builder)                                                                           | `npm-builder-signing-adapter-valid`                 | missing, extra, wrong-role, or module/checksum-inconsistent adapter | `builder-dependencies-signing-adapter-mismatch`      |
| Invocation identity            | [metadata](slsa-provenance-v1.md#metadata)                                                                         | `npm-invocation-id-certificate-uri-valid`           | malformed URI or OID `.21` inequality                               | `run-invocation-uri-invalid`                         |
| Online trust root              | [Sigstore trust-root acquisition](#sigstore-trust-root-acquisition-and-freshness)                                  | `trust-root-online-tuf-valid`                       | TUF failure followed by pin fallback                                | `ungoverned-trust-root`                              |
| Offline network isolation      | [Sigstore trust-root acquisition](#sigstore-trust-root-acquisition-and-freshness)                                  | `trust-root-offline-pinned-valid`                   | offline network attempt                                             | `verification-network-call`                          |
| Offline pin freshness          | [Sigstore trust-root acquisition](#sigstore-trust-root-acquisition-and-freshness)                                  | `trust-root-offline-pinned-valid`                   | offline stale pin                                                   | `stale-pinned-trust-root`                            |
| Invocation mode                | [Sigstore trust-root acquisition](#sigstore-trust-root-acquisition-and-freshness)                                  | `trust-root-online-tuf-valid`                       | mode absent, conflicting, or root-shape-inconsistent                | `verification-mode-invalid`                          |

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
- Exactly one invocation mode is selected, and its trust-root behavior satisfies the online or
  offline mode contract.
- npm provenance has the closed `distribution` and `caller` groups, exactly empty
  `internalParameters`, the manager-dependent name-keyed dependency set, closed `builder.version`,
  one signing-adapter dependency, and `metadata.invocationId` equal to Fulcio OID `.21`.
