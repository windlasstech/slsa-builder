---
parent: Decisions
nav_order: 53
status: accepted
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Use a Three-Job Release Manifest Signing Boundary

## Context and Problem Statement

ADR 0028 requires production consumers to execute profile-owned reusable workflows by full commit
SHA, and requires signed release metadata that maps human-readable Windlass release versions to the
exact workflow SHAs and SHA-based `builder.id` values. ADR 0031 then selected a Windlass-defined
release manifest carried in an in-toto Statement and signed as a Sigstore DSSE bundle as the
machine-verifiable release metadata mechanism.

The project now needs to decide the workflow trust boundary for generating, signing, and publishing
that release manifest. The manifest is security-critical because verifiers use it to decide whether
a profile provenance `builder.id`, workflow path, release version, and `buildType` mapping are
trusted. If the same job can freely generate metadata, sign it, and mutate the GitHub Release, then
a bug or compromise in that job can affect both the trust metadata and its distribution channel.

How should the release manifest signing workflow separate manifest generation, Sigstore signing, and
GitHub Release asset publication for the initial production release process?

## Decision Drivers

- Preserve the release manifest as the canonical machine-verifiable version-to-SHA trust metadata
  selected by ADR 0031.
- Keep release metadata generation, signing authority, and release asset mutation in distinct trust
  and permission boundaries where practical.
- Follow the project pattern of digest-verified handoff between isolated jobs selected for npm
  publishing and release asset publication.
- Use GitHub-hosted ephemeral runners, full-SHA-pinned actions, minimal top-level permissions, and
  job-level permission elevation.
- Avoid long-lived signing keys and use GitHub Actions OIDC with Sigstore as the initial signing
  mechanism.
- Keep the initial implementation smaller than a dedicated reusable workflow or TUF-style metadata
  system while leaving a clear migration path.
- Make future migration to a reusable signing workflow, direct Sigstore tooling, `sigstore-go`, or
  TUF-style metadata an explicit follow-up decision rather than an accidental implementation drift.

## Considered Options

- Use a three-job release manifest workflow: `manifest-generate` → `manifest-sign` →
  `manifest-upload`.
- Use a dedicated reusable workflow for release manifest signing from the beginning.
- Generate, sign, and upload the release manifest in one job.
- Use direct Sigstore tooling, such as `cosign` or future `sigstore-go`, instead of `actions/attest`
  for the initial signing path.
- Rely on GitHub immutable release attestations, signed tags, release notes, or checksum files
  rather than a Windlass-signed release manifest bundle.
- Adopt TUF-style release metadata from the beginning.

## Decision Outcome

Chosen option: "Use a three-job release manifest workflow: `manifest-generate` → `manifest-sign` →
`manifest-upload`", because it provides a clear least-privilege boundary for the initial release
metadata trust root while staying small enough for the first implementation.

The initial Windlass release process should produce release manifest metadata through three primary
jobs:

```text
manifest-generate -> manifest-sign -> manifest-upload
```

The `manifest-generate` job should create the unsigned Windlass release manifest JSON and the
in-toto Statement content required by ADR 0031. It should calculate the manifest digest, upload the
unsigned manifest material as workflow artifacts, and expose only the digest and artifact handles
needed by the signing job. This job should not have `id-token: write`, `attestations: write`,
`contents: write`, release mutation authority, long-lived signing credentials, or access to
publisher secrets.

The `manifest-sign` job should download the manifest artifacts, recalculate their digests, verify
that the contents match the `manifest-generate` handoff, and sign the release manifest Statement as
a Sigstore bundle. The initial signing adapter should be full-SHA-pinned `actions/attest` in custom
predicate mode or an equivalent `actions/attest` path that signs the Windlass release manifest
predicate selected by ADR 0031. The signing job should have only the permissions needed for signing
and optional GitHub attestation storage: `contents: read`, `id-token: write`, and
`attestations: write`. It should not have `contents: write` or GitHub Release mutation authority.

The `manifest-upload` job should download the unsigned manifest material and signed Sigstore bundle,
recalculate their digests, verify that those digests match the previous job handoff values, and
upload the release manifest artifacts to the selected existing GitHub Release. This job should have
only the release mutation permission it needs, normally `contents: write`, and should not have
`id-token: write`, `attestations: write`, signing credentials, or authority to create new signed
metadata. The upload job should fail rather than re-signing, regenerating, or mutating manifest
contents after signing.

The signed Sigstore bundle is the canonical machine-verifiable release metadata artifact. Plain JSON
manifests, checksums, release notes, GPG-signed annotated tags, GitHub immutable release evidence,
and GitHub release-level attestations may provide discovery, diagnostics, defense-in-depth, or
human-readable release integrity evidence, but they do not replace the signed release manifest
bundle as the version-to-SHA trust root selected by ADR 0031.

The release manifest architecture specification should define exact artifact names, bundle names,
digest encodings, predicate type, schema version, subject rules, signer identity, allowed workflow
path, release tag checks, release asset duplicate behavior, retry behavior, and verification
commands. Jobs must not trust workflow artifact names, logs, job outputs, release notes, or
unchecked files as substitutes for recalculating digests and verifying the signed bundle.

This decision does not permanently reject a dedicated reusable workflow, direct Sigstore tooling,
`sigstore-go`, or TUF-style metadata. Those options should be reconsidered through follow-up ADRs if
one or more of the following conditions becomes true:

- multiple release workflows or repositories need to reuse the same release manifest signing
  contract;
- verifier policy needs a distinct, stable release-manifest signer identity separate from the main
  release orchestration workflow;
- `actions/attest` cannot represent the required custom predicate, bundle, signer identity,
  verification, offline validation, or distribution behavior;
- a Go-native trusted core is ready to own Sigstore bundle creation and verification through
  `sigstore-go` or equivalent direct libraries;
- release metadata needs threshold signing, delegated roles, expiry, rollback/freeze attack defense,
  multiple release channels, mirrored metadata, or independently managed signing authorities;
- GitHub artifact attestation storage, immutable releases, or release verification features change
  enough to materially alter the release metadata trust model.

Any migration to a dedicated reusable workflow, direct `cosign`/`sigstore-go` signing, or TUF-style
metadata must preserve or explicitly replace the verifier-visible trust contract for release
manifest predicate type, schema version, signer identity, release version, workflow path, workflow
SHA, `builder.id`, and `buildType` mapping.

### Consequences

- Good, because manifest generation, signing, and release upload have separate job permissions.
- Good, because release mutation authority cannot create or modify signed metadata.
- Good, because signing authority cannot upload or replace GitHub Release assets.
- Good, because the design follows the repository's existing digest-verified handoff pattern.
- Good, because the initial signing path uses GitHub Actions OIDC and avoids long-lived signing
  keys.
- Good, because future migration paths are acknowledged without making the first implementation too
  large.
- Neutral, because the workflow still depends on GitHub-hosted runners and GitHub-native Sigstore
  integration.
- Neutral, because the signed manifest remains Windlass-specific and requires Windlass verifier
  policy.
- Bad, because three jobs require more handoff artifacts, digest checks, and failure handling than a
  single-job release process.
- Bad, because `actions/attest` remains a trusted action dependency until a future migration
  replaces it.

### Confirmation

This decision is confirmed when release process specifications, workflow implementations, tests, and
verifier guidance define:

- exactly three primary release manifest jobs named or equivalent to `manifest-generate`,
  `manifest-sign`, and `manifest-upload`;
- minimal top-level workflow permissions and explicit job-level permission elevation;
- no signing authority or release mutation authority in `manifest-generate`;
- `manifest-sign` permissions limited to `contents: read`, `id-token: write`, and
  `attestations: write` unless a later ADR changes the signing adapter;
- no `contents: write` or release mutation authority in `manifest-sign`;
- `manifest-upload` release mutation permission, normally `contents: write`, without signing
  authority;
- digest-verified handoff from manifest generation to signing and from signing to upload;
- full-SHA pinning for `actions/attest` and any other external action used in the release manifest
  signing path;
- the Windlass release manifest predicate type, schema version, subject rules, bundle artifact name,
  plain manifest artifact name, digest encoding, signer identity, and verifier policy;
- release upload behavior for missing releases, duplicate manifest assets, partial upload failure,
  retries, and immutable release publication flows;
- documentation that plain JSON manifests, checksums, release notes, signed tags, GitHub immutable
  release evidence, and release-level attestations are complementary evidence, not substitutes for
  the signed release manifest bundle;
- follow-up ADR criteria for migration to a dedicated reusable workflow, direct Sigstore or
  `sigstore-go` signing, or TUF-style metadata.

Implementation review should verify that no production path can publish or trust release manifest
metadata unless the manifest was generated, signed, digest-verified, and uploaded through the
selected boundary, and that no job combines signing authority with GitHub Release mutation authority
without a later ADR.

## Pros and Cons of the Options

### Use a three-job release manifest workflow

Release manifest generation, signing, and upload run in distinct jobs with digest-verified handoff
between jobs.

- Good, because each job has a small, auditable permission set.
- Good, because signing and release mutation authorities are separated.
- Good, because it aligns with the project's existing three-job profile patterns.
- Neutral, because it is GitHub Actions-specific.
- Bad, because it introduces cross-job artifact and digest-handling complexity.

### Use a dedicated reusable workflow from the beginning

Release manifest signing is moved into a standalone reusable workflow with its own public contract
and signer identity.

- Good, because multiple release workflows can reuse the same signing contract.
- Good, because verifier policy can trust a distinct release-manifest signer workflow.
- Bad, because it adds public API, builder identity, release metadata, and bootstrap complexity
  before reuse pressure exists.
- Bad, because the first implementation must version and document another reusable workflow surface.

### Generate, sign, and upload in one job

One release job creates the manifest, signs it, and uploads it to the GitHub Release.

- Good, because it is simplest to implement.
- Good, because there are no cross-job handoff artifacts.
- Bad, because one job needs both signing authority and release mutation authority.
- Bad, because a compromise or bug in that job affects both the trust metadata and its distribution
  channel.
- Bad, because it does not match the repository's least-privilege release profile pattern.

### Use direct Sigstore tooling instead of `actions/attest`

The release workflow invokes `cosign`, `sigstore-go`, or equivalent direct Sigstore tooling to
create the signed bundle.

- Good, because it can provide more direct control over bundle format, offline verification, and
  non-GitHub distribution.
- Good, because it aligns with the long-term Go trusted core direction if implemented through
  `sigstore-go`.
- Bad, because it requires Windlass to own more Sigstore integration, compatibility, and verifier
  behavior in the first release.
- Bad, because it bypasses the existing `actions/attest` adapter decision unless a follow-up ADR
  changes that boundary.

### Rely on GitHub release evidence or signed tags instead of a signed manifest bundle

The release process uses GitHub immutable releases, release attestations, GPG-signed annotated tags,
release notes, checksums, or a plain manifest rather than a Windlass-signed release manifest bundle.

- Good, because these mechanisms are familiar and useful as defense-in-depth evidence.
- Good, because signed tags and immutable release evidence remain valuable release integrity
  signals.
- Bad, because they do not provide the Windlass-specific machine-readable version-to-SHA,
  `builder.id`, workflow path, and `buildType` mapping selected by ADR 0031.
- Bad, because verifiers would have to infer trust from loosely related evidence instead of checking
  one signed predicate schema.

### Adopt TUF-style metadata from the beginning

Release metadata is modeled as TUF targets, roles, expiry, and signed metadata rather than a single
signed in-toto release manifest.

- Good, because TUF is strong for delegated roles, threshold signing, expiry, mirrors, rollback
  defense, and multi-channel release metadata.
- Bad, because it is operationally heavy for the initial single-repository, single-maintainer
  release process.
- Bad, because threshold and delegated-role benefits are limited until the organization has multiple
  trusted authorities or release channels.

## More Information

This decision follows ADR 0028 and ADR 0031. It complements ADR 0035's initial `actions/attest`
signing adapter decision and the three-job permission-boundary pattern selected for production npm
publishing and release asset publication.

This decision does not define the full release manifest JSON schema, exact artifact filenames,
consumer verifier CLI interface, TUF metadata repository, dedicated reusable workflow contract,
direct `sigstore-go` signing implementation, or long-term release-channel policy. Those belong in
architecture specifications or future ADRs.

Reference points considered:

- SLSA Build L3 requires provenance or equivalent metadata authenticity and strong resistance to
  tenant forgery, including keeping signing material away from user-controlled build steps.
- GitHub Actions artifact attestations use short-lived OIDC identity and require explicit
  `id-token: write` and `attestations: write` permissions.
- GitHub Actions hardening guidance recommends full commit SHA pinning, least-privilege
  `GITHUB_TOKEN` permissions, hosted ephemeral runners, and caution around privileged workflows.
- Windlass organization security policy requires SHA-pinned actions, least-privilege job
  permissions, OIDC instead of long-lived credentials, signed releases, protected release refs, and
  SLSA Build L3-oriented provenance wherever feasible.
