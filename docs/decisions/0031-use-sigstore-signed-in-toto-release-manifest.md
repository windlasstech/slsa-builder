---
parent: Decisions
nav_order: 31
status: accepted
date: 12026-07-05
decision-makers: Yunseo Kim
---

# Use Sigstore-Signed in-toto Release Manifest

## Context and Problem Statement

ADR 0028 selected full commit SHA pins as the production reusable workflow reference form and
required signed release metadata that maps each human-readable release version to the exact workflow
commit SHA and SHA-based `builder.id` values. It intentionally left the metadata format and
signature or attestation mechanism open.

SLSA v1.2 verification expects verifiers to compare provenance against preconfigured roots of trust,
including recognized signing identities, trusted `builder.id` values, and the maximum SLSA Build
level assigned to those builders. For this project, the release metadata is not the package build
provenance itself. Instead, it is producer-defined expectation metadata that tells a verifier which
Windlass release version corresponds to which immutable reusable workflow commit and SHA-based
builder identities.

The project needs a release metadata format that is machine-readable, signed, compatible with the
Sigstore and in-toto ecosystem used elsewhere in the profile, easy to distribute with GitHub
Releases, and precise enough for verifier trust decisions. It should also leave room to adopt a more
complete update-metadata system later if release metadata distribution grows beyond a simple signed
manifest model.

Should Windlass release metadata use a custom signed manifest, SLSA provenance, Git tag signatures,
repository-committed metadata, TUF-style metadata, verifier-embedded allowlists, or another trust
root mechanism?

## Decision Drivers

- Satisfy ADR 0028's requirement for signed version-to-SHA metadata without reintroducing mutable
  tag refs as the primary machine trust input.
- Align with SLSA v1.2's verifier root-of-trust model: trusted signer identity, trusted
  `builder.id`, expected `buildType`, and expected external parameters.
- Use a signing mechanism that supports transparency logs and avoids long-lived signing keys in
  release workflows.
- Keep release metadata separate from SLSA build provenance while using a familiar in-toto
  attestation envelope.
- Let verifiers make deterministic SHA-based trust decisions without querying current GitHub tag
  state as the primary check.
- Keep the initial metadata system small enough for a 1-person organization and early profile
  implementation.
- Preserve a path to stronger TUF-style metadata if rollback, freeze, threshold-signing, delegation,
  or metadata-mirroring needs become material.

## Considered Options

- Publish a Windlass-defined JSON release manifest wrapped as an in-toto Statement and signed as a
  Sigstore DSSE bundle.
- Treat the release metadata file as a build artifact and publish SLSA provenance for that artifact.
- Store the version-to-SHA mapping only in GPG-signed annotated Git tag messages.
- Commit release metadata files into the repository and trust signed commits plus protected
  branches.
- Use TUF-style role-separated metadata with root, targets, snapshot, and timestamp roles.
- Embed the version-to-SHA allowlist directly in verifier releases or verifier configuration.
- Publish unsigned or checksum-only release metadata.

## Decision Outcome

Chosen option: "Publish a Windlass-defined JSON release manifest wrapped as an in-toto Statement and
signed as a Sigstore DSSE bundle", because it gives the verifier authenticated, machine-readable
version-to-SHA expectations while staying aligned with the Sigstore/in-toto trust model already used
for package provenance.

Each Windlass release that publishes reusable profile workflows should publish a release metadata
manifest as a GitHub Release asset. The manifest should use a Windlass-defined JSON schema carried
in an in-toto Statement. The Statement should use a Windlass-owned release metadata predicate type,
for example:

```text
https://github.com/windlasstech/slsa-builder/predicate/release-manifest/v1
```

The Statement should be signed as a Sigstore DSSE bundle by the Windlass release workflow. The
bundle is the canonical machine-verifiable release metadata artifact. Plain JSON copies, checksums,
release notes, and Git tag messages may help humans discover the mapping, but they are not
substitutes for the signed DSSE bundle.

The release manifest predicate should identify at least:

- the manifest schema version;
- the Windlass release version, such as `vX.Y.Z`;
- the source repository URL;
- the release tag ref;
- the release commit SHA;
- each released profile name;
- each profile workflow path;
- the exact full workflow commit SHA production consumers should pin;
- each SHA-based `builder.id` value trusted for the release;
- each profile `buildType` URI associated with that builder identity;
- the metadata generation time and release workflow identity fields needed by verifier diagnostics.

The in-toto Statement `subject` should identify the canonical release manifest artifact by digest,
or an equivalent stable release metadata artifact whose digest lets verifiers detect tampering. The
predicate itself should carry the version-to-SHA and builder mapping. The profile architecture
specification may refine exact filenames, digest algorithms, JSON field names, and canonicalization
rules.

Verifier trust roots for this metadata should include:

- the configured Sigstore trust root, such as the Sigstore public-good Fulcio/Rekor root unless a
  later ADR selects a different root;
- the expected GitHub Actions OIDC signer identity for the Windlass release workflow;
- the Windlass release metadata predicate type and schema version;
- the `github.com/windlasstech/slsa-builder` repository identity;
- the signed and protected release tag policy as release-integrity evidence;
- the allowed profile workflow paths, SHA-based `builder.id` shapes, and `buildType` URIs.

The verifier should trust a SHA-based `builder.id` through this metadata only when the DSSE
signature verifies against the configured Sigstore root, the signing certificate identity matches
the expected Windlass release workflow identity, the Statement and predicate use the expected types
and schema version, and the metadata maps the requested release version to the exact workflow SHA
and `builder.id` observed in package provenance.

Git signed annotated tags remain required release-integrity evidence and human discovery handles,
but they are not the canonical machine-readable version-to-SHA trust root. Verifiers should not need
to resolve current GitHub tag state before making the primary machine trust decision, although they
may optionally check tag signatures and tag-to-commit consistency as defense in depth.

TUF-style metadata is not adopted for the initial release metadata mechanism. The project should
reconsider TUF-style role-separated metadata if at least one of the following becomes true:

- Windlass maintains multiple release channels, repositories, profile families, or delegated
  metadata authorities that need independent signing roles;
- there are multiple trusted maintainers or automated signers that can provide meaningful threshold
  signing for root or targets metadata;
- verifier clients need automatic metadata refresh with strong rollback and freeze attack defenses;
- release metadata is mirrored outside GitHub Releases or served through a registry-like metadata
  repository;
- key rotation, metadata expiry, timestamping, or delegated ownership becomes difficult to manage
  with a single signed manifest per release;
- consumers require a long-lived update framework rather than per-release metadata verification.

### Consequences

- Good, because the release metadata is machine-readable and directly supports ADR 0028's
  version-to-SHA trust model.
- Good, because Sigstore keyless signing avoids introducing long-lived release metadata signing keys
  into the workflow.
- Good, because DSSE and in-toto are consistent with the project's provenance signing direction.
- Good, because verifiers can validate release metadata without trusting mutable Git tag state as
  the primary machine input.
- Good, because a Windlass-owned predicate can represent release-to-builder expectations without
  overloading SLSA build provenance semantics.
- Good, because Git signed tags remain useful release integrity evidence without becoming the
  primary executable reference or metadata format.
- Neutral, because the manifest is Windlass-specific and requires a dedicated verifier policy.
- Bad, because Windlass must define and version a JSON schema, predicate type, file naming policy,
  and verifier support.
- Bad, because this does not provide TUF's built-in rollback, freeze, delegation, threshold-signing,
  or metadata-expiry model.

### Confirmation

This decision is confirmed when the release process, profile architecture specification,
documentation, and verifier define:

- the Windlass release manifest predicate type and schema version;
- the exact release manifest JSON fields and canonical field semantics;
- the in-toto Statement subject rules and digest algorithms;
- Sigstore DSSE bundle generation by the Windlass release workflow;
- GitHub Release asset names and distribution expectations;
- verifier trust root configuration for Sigstore root, signer identity, repository, predicate type,
  schema version, workflow path, `builder.id`, and `buildType` values;
- verifier behavior for mapping a human release version to a full workflow SHA and SHA-based
  `builder.id`;
- optional defense-in-depth checks for signed tag consistency;
- documentation of the criteria that would trigger reconsideration of TUF-style metadata.

Implementation review should verify that package provenance trust does not depend on unsigned
release notes, mutable GitHub tag lookup, repository-committed metadata alone, checksum-only files,
or verifier-embedded allowlists when a signed Windlass release manifest is expected.

## Pros and Cons of the Options

### Publish a Sigstore-signed in-toto release manifest

Windlass defines a JSON release manifest predicate, wraps it in an in-toto Statement, signs the
Statement as a Sigstore DSSE bundle, and publishes the bundle with the GitHub Release.

- Good, because it authenticates the release version to workflow SHA and `builder.id` mapping needed
  by ADR 0028.
- Good, because it fits SLSA's expectation-based verification model without pretending the mapping
  is package build provenance.
- Good, because it uses keyless signing, transparency logging, and an attestation envelope already
  familiar to SLSA tooling.
- Good, because distribution through GitHub Releases is simple for humans and automation.
- Bad, because Windlass must maintain a custom predicate and verifier policy.
- Bad, because a single per-release manifest is weaker than TUF for update metadata lifecycle
  management.

### Publish SLSA provenance for the release metadata artifact

The release metadata file is treated as an artifact and receives SLSA build provenance from the
release workflow.

- Good, because it reuses the standard SLSA provenance predicate and existing provenance tooling.
- Good, because it can prove how the metadata file was produced.
- Bad, because the important payload is not how the metadata was built; it is the producer-defined
  expectation mapping inside the metadata.
- Bad, because verifiers still need a Windlass-specific schema to interpret the version-to-SHA
  mapping.
- Bad, because this can confuse release expectation metadata with package build provenance.

### Store the mapping in GPG-signed annotated tags

The release tag message contains the workflow SHA and builder IDs, and verifiers trust the signed
tag.

- Good, because release tags are already required to be signed and protected by organization policy.
- Good, because humans can inspect tag messages with standard Git tooling.
- Bad, because tag messages are a poor machine-readable schema and schema evolution surface.
- Bad, because verifiers need Git tag object and signature verification support.
- Bad, because it risks making tag state feel like the primary trust root again, contrary to ADR
  0028's SHA-based machine trust decision.

### Commit release metadata into the repository

Release metadata files live in the repository and are protected by signed commits, protected
branches, and signed release tags.

- Good, because changes are visible in normal source review and history.
- Good, because repository files are easy to browse and diff.
- Bad, because the metadata can be changed after a release unless additional immutability rules are
  enforced and verified.
- Bad, because verification can become dependent on current repository or GitHub API state.
- Bad, because repository-committed metadata does not by itself create a per-release signed artifact
  distributed with the release.

### Use TUF-style metadata

Windlass adopts role-separated metadata such as root, targets, snapshot, and timestamp metadata with
versioning, expiry, threshold signatures, and delegation.

- Good, because TUF directly addresses rollback, freeze, key rotation, delegation, and threshold
  signing for update metadata.
- Good, because it scales well to many release channels, signers, and mirrored metadata stores.
- Bad, because it is operationally heavy for the initial project and a 1-person organization.
- Bad, because threshold-signing benefits are limited if one maintainer controls all signing roles.
- Bad, because verifier and release automation complexity would grow before the project has a clear
  need for TUF's stronger lifecycle guarantees.

### Embed the allowlist in verifier releases or configuration

The verifier binary or its local configuration contains trusted version-to-SHA mappings.

- Good, because offline verification is simple once the verifier is updated.
- Good, because no separate metadata download path is needed.
- Bad, because verifier releases become coupled to builder releases.
- Bad, because third-party verifiers and consumers need a separate trust distribution channel.
- Bad, because metadata fixes or new builder releases require verifier updates or config changes.

### Publish unsigned or checksum-only metadata

Windlass publishes JSON metadata plus checksums without an authenticated attestation envelope.

- Good, because it is easy to implement.
- Good, because checksums catch accidental corruption when delivered over a trusted channel.
- Bad, because checksums alone do not identify an authorized signer or trusted release workflow.
- Bad, because this does not satisfy ADR 0028's signed metadata requirement.
- Bad, because verifiers would need to place excessive trust in the release asset distribution
  channel.

## More Information

This decision follows ADR 0028 and decides only the release metadata format and trust root for
mapping Windlass release versions to SHA-based reusable workflow builder identities. It does not
decide package provenance format, npm provenance submission, registry scope, release-note format,
changelog format, or final verifier command-line UX.

Reference points considered:

- SLSA v1.2 verification recommends verifying provenance envelope signatures, artifact subjects,
  `predicateType`, trusted `builder.id`, `buildType`, and expected `externalParameters` against
  configured roots of trust.
- SLSA v1.2 describes verifier roots of trust as mappings from recognized signing identities and
  `builder.id` values to trusted SLSA Build levels.
- SLSA v1.2 distinguishes package build provenance from producer-defined expectations used by
  verifiers.
- SLSA v1.2 Build L2 and L3 require signed provenance and stronger protection of provenance
  generation, but they do not prescribe this project's release version metadata format.
- Windlass security policy requires signed release tags, signed commits on protected branches,
  OIDC-preferred workflows, SHA-pinned workflow references, and signed release provenance wherever
  feasible.
- ADR 0028 already made the primary machine trust decision SHA-based and required signed metadata to
  preserve human-readable release versions.
