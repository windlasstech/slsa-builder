---
parent: Decisions
nav_order: 38
status: superseded by ADR-0049
date: 12026-07-06
decision-makers: Yunseo Kim
---

# Use Windlass-Generated Provenance with actions/attest for GitHub Release Assets

## Context and Problem Statement

ADR 0013 scoped the initial JS/TS profile to npm package releases and deferred GitHub Release assets
to a future composable profile. The initial release scope now needs to include a GitHub Release
asset profile as a sibling profile rather than waiting for a later release. The first architectural
choice for that profile is whether it should be a thin reusable workflow wrapper around GitHub
`actions/attest` default provenance behavior, or whether Windlass should directly implement the
profile's provenance and signing behavior in Go or another trusted runtime.

GitHub Release assets differ from npm packages. They are distributed by the source repository
hosting provider rather than by npm, may be protected by GitHub immutable releases, and can be
verified with GitHub release and artifact-attestation tooling. However, SLSA v1.2 still treats
provenance as artifact-bound build metadata that should identify the output artifact by digest,
describe how it was produced, and be generated or verified by the trusted build platform for Build
L3. GitHub's artifact attestation documentation also distinguishes direct artifact attestations,
which provide SLSA Build Level 2 by themselves, from reusable-workflow-based builds that can provide
the additional isolation needed for Build Level 3.

Should the initial GitHub Release asset profile rely on `actions/attest` default provenance as a
wrapper, generate Windlass-owned SLSA provenance in the Go trusted core and use `actions/attest`
only as a signing and storage adapter, implement Sigstore signing directly in Go from the start, or
rely on GitHub immutable release attestations instead of a separate build provenance profile?

## Decision Drivers

- Preserve ADR 0002 and ADR 0003's trusted reusable workflow and profile-owned workflow boundary.
- Keep the GitHub Release asset profile aligned with SLSA v1.2 Build L3 expectations for
  unforgeable, artifact-bound provenance.
- Reuse ADR 0004's Go trusted core direction for profile-specific provenance, subject, digest, and
  policy logic.
- Use a Windlass-owned `builder.id`, `buildType`, and complete `externalParameters` schema rather
  than inheriting a generic GitHub Actions workflow build type as the production verifier contract.
- Avoid giving caller-controlled build steps the ability to inject or alter provenance contents
  other than allowed artifact subject information.
- Avoid implementing Fulcio, Rekor, timestamp, Sigstore bundle, and GitHub attestations API behavior
  directly before the trusted core has enough specification and fixture coverage.
- Keep GitHub immutable releases and release attestations available as release-integrity evidence
  without treating them as a substitute for artifact-bound build provenance.
- Preserve a future migration path toward a Go-native `sigstore-go` signing adapter if the project
  later needs to own signing internals directly.

## Considered Options

- Use `actions/attest` default provenance as a thin reusable workflow wrapper.
- Generate Windlass SLSA provenance in Go and use `actions/attest` as the signing and storage
  adapter.
- Implement provenance generation, Sigstore signing, and GitHub attestation persistence directly in
  Go for the initial profile.
- Rely on GitHub immutable release attestations and release verification instead of a separate
  artifact-bound SLSA provenance profile.

## Decision Outcome

Chosen option: "Generate Windlass SLSA provenance in Go and use `actions/attest` as the signing and
storage adapter", because it gives the GitHub Release asset profile a Windlass-owned SLSA verifier
contract while still using GitHub's maintained Sigstore and artifact-attestation integration for the
initial signing path.

The initial GitHub Release asset profile should generate canonical SLSA build provenance v1 inside
Windlass trusted logic. The trusted implementation, expected to be Go by default under ADR 0004,
should own at least:

- release asset subject naming and digest normalization;
- the profile-specific `buildType` URI and schema;
- complete verifier-relevant `buildDefinition.externalParameters`;
- `runDetails.builder.id` using the SHA-pinned reusable workflow identity model from ADR 0028;
- source repository, source ref, resolved source commit, release tag, asset path or name, runtime
  policy, and release event context fields;
- validation that the provenance subject identifies exactly the release asset artifact being signed
  and later uploaded or verified.

The profile should invoke a full-SHA-pinned `actions/attest` step in custom attestation mode to sign
the Windlass-generated SLSA predicate and, when configured, upload the resulting Sigstore bundle to
GitHub's attestation storage. `actions/attest` should not be used in its default provenance mode as
the canonical production provenance generator for this profile. In this profile, `actions/attest` is
an adapter for Sigstore bundle generation and GitHub attestation storage, not the owner of the
Windlass release asset build type semantics.

GitHub immutable releases, release attestations, `gh release verify`, and `gh release verify-asset`
should be treated as complementary release-integrity and asset-immutability mechanisms. They may be
required or recommended by the profile's release process, but they do not replace artifact-bound
SLSA build provenance for the release asset.

The initial profile should not implement Sigstore signing directly in Go. A later ADR may replace
the `actions/attest` adapter with `sigstore-go` or another Go-native signing path after the project
has specified bundle compatibility, signer identity, GitHub attestation storage behavior, release
asset verification policy, trust roots, and migration rules. If that migration materially changes
security properties, verifier expectations, or `builder.id` meaning, it should use a distinct
builder identity or explicitly document compatibility.

### Consequences

- Good, because the release asset profile can define a Windlass-owned `buildType` and
  `externalParameters` schema instead of inheriting a generic GitHub Actions provenance contract.
- Good, because SLSA predicate generation, subject rules, digest handling, and release asset policy
  stay in the Go trusted core direction selected by ADR 0004.
- Good, because `actions/attest` still provides a maintained GitHub-native Sigstore signing and
  attestation-storage path for the initial release.
- Good, because reusable workflow signer checks through GitHub artifact attestation tooling remain
  available.
- Good, because GitHub immutable release verification can complement, rather than replace,
  artifact-bound SLSA provenance.
- Neutral, because the profile will need Windlass-specific verifier policy in addition to generic
  `gh attestation verify` or `gh release verify` success.
- Bad, because the release asset profile must implement and test its own SLSA predicate generation,
  release asset subject model, and provenance policy before shipping.
- Bad, because `actions/attest` remains a trusted, SHA-pinned GitHub-specific dependency in the
  initial profile.
- Bad, because a future Go-native signing migration will require compatibility and verifier-policy
  work.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification,
workflow implementation, tests, and verifier guidance define:

- Windlass-generated SLSA build provenance v1 as the canonical provenance for GitHub Release assets;
- a profile-specific GitHub Release asset `buildType` URI and schema;
- complete `externalParameters`, `internalParameters`, `resolvedDependencies`, `builder.id`,
  `builder.version`, `builderDependencies`, invocation metadata, and byproducts rules for the
  release asset profile;
- release asset subject naming, digest algorithms, and digest encodings;
- full-SHA pinning for the `actions/attest` action reference;
- use of `actions/attest` custom attestation mode with
  `predicate-type: https://slsa.dev/provenance/v1` or an equivalent path that signs the
  Windlass-generated SLSA predicate;
- job-level permissions that keep Sigstore signing and GitHub attestation storage permissions away
  from caller-controlled build steps;
- release asset upload, immutable release, release attestation, and `gh release verify` behavior as
  complementary release-integrity checks rather than substitutes for SLSA provenance;
- verifier guidance for Sigstore signature, reusable workflow signer identity, SHA-based
  `builder.id`, Windlass `buildType`, `externalParameters`, source identity, release tag, release
  asset name, release asset digest, and release manifest linkage;
- a documented future path for replacing `actions/attest` with a Go-native Sigstore adapter without
  requiring that implementation in the initial release asset profile.

Implementation review should verify that production GitHub Release asset provenance cannot silently
fall back to `actions/attest` default provenance, unsigned release assets, release-level attestation
only, mutable release assets, long-lived signing keys, or a different signer identity under the same
production builder identity.

## Pros and Cons of the Options

### Use `actions/attest` default provenance as a thin reusable workflow wrapper

The release asset profile builds or receives an artifact in a reusable workflow and invokes
`actions/attest` with only `subject-path`, `subject-digest`, or `subject-checksums`, allowing the
action to generate its default SLSA provenance predicate.

- Good, because it is the smallest implementation and uses GitHub's official artifact attestation
  action directly.
- Good, because it naturally integrates with GitHub attestations storage and
  `gh attestation verify`.
- Good, because GitHub documents reusable workflows plus artifact attestations as a path toward SLSA
  Build Level 3.
- Bad, because the default predicate is not the Windlass release asset build type contract.
- Bad, because Windlass would have less control over `buildType`, `externalParameters`, and
  profile-specific release asset verification semantics.
- Bad, because it risks creating a release asset profile whose public verifier contract differs from
  the Windlass-generated provenance model selected for the npm package profile.

### Generate Windlass SLSA provenance in Go and use `actions/attest` as the adapter

The trusted core creates the SLSA build provenance predicate for the release asset profile, then a
SHA-pinned `actions/attest` step signs and stores that predicate in custom attestation mode.

- Good, because Windlass owns the release asset subject, build type, parameter, and verification
  schema.
- Good, because it keeps the security-critical JSON, digest, subject, and policy logic in the Go
  trusted-core direction.
- Good, because it avoids implementing Sigstore signing and GitHub attestation persistence directly
  in the first release asset milestone.
- Good, because it is consistent with ADR 0035's adapter model while adapting it to a non-npm
  profile.
- Neutral, because generic GitHub verification tools remain useful but incomplete without
  Windlass-specific policy checks.
- Bad, because it requires a real provenance generator and conformance fixture set before the
  profile can ship.
- Bad, because `actions/attest` behavior, versioning, and GitHub support boundaries must be
  monitored as part of the trusted release path.

### Implement signing and attestation persistence directly in Go

The Go trusted core generates provenance, obtains or uses GitHub Actions OIDC identity, creates the
Sigstore bundle, and uploads or distributes the attestation without invoking `actions/attest`.

- Good, because it gives Windlass maximum control over signing, bundle generation, error handling,
  tests, and compatibility.
- Good, because it may eventually reduce the number of trusted workflow action dependencies.
- Bad, because the initial profile would immediately own Fulcio, Rekor, timestamp, bundle,
  trust-root, GitHub attestations API, and private/public Sigstore behavior.
- Bad, because the larger initial trusted implementation surface increases security and maintenance
  risk.
- Bad, because a direct implementation is unnecessary to decide the release asset profile's build
  semantics.

### Rely on GitHub immutable release attestations instead

The profile depends on GitHub immutable release protections and release attestations to prove the
release tag, commit, and assets, without generating separate artifact-bound SLSA build provenance.

- Good, because immutable releases protect release tags and assets from modification or deletion
  after publication.
- Good, because GitHub release verification commands provide a simple consumer-facing integrity
  check.
- Good, because release attestations can record the release tag, commit SHA, and release assets.
- Bad, because SLSA v1.2 recommends binding build attestations to artifacts, not only to releases.
- Bad, because release-level attestation does not define the Windlass build process, `buildType`, or
  complete `externalParameters` needed for the profile's SLSA verifier contract.
- Bad, because this would make GitHub release integrity a substitute for build provenance rather
  than defense-in-depth evidence.

## More Information

This decision extends ADR 0002, ADR 0003, ADR 0004, ADR 0028, ADR 0031, ADR 0035, and ADR 0037 to
the initial GitHub Release asset profile. It does not decide the full release asset profile scope,
workflow filename, public input contract, job graph, release creation policy, immutable release
requirement, asset upload behavior, or standalone verifier CLI surface. Those details belong in
follow-up ADRs or the release asset architecture specification.

Reference points considered:

- SLSA v1.2 Build Requirements require authentic provenance at Build L2 and unforgeable provenance
  at Build L3. At Build L3, every provenance field must be generated or verified by the trusted
  build platform, except permitted tenant-controlled subject information, and `externalParameters`
  must be fully enumerated.
- SLSA v1.2 Build Provenance defines `predicateType: https://slsa.dev/provenance/v1`, separates
  `builder.id` from signer identity, and expects build type specifications to define the schema for
  `externalParameters` and `internalParameters`.
- SLSA v1.2 Distributing Provenance says attestations should be bound to artifacts rather than
  releases, and recommends publishing provenance alongside source repository releases when that is
  the artifact distribution surface.
- GitHub artifact attestations documentation states that artifact attestations by themselves provide
  SLSA Build Level 2, while reusable workflows can provide the known build instructions and
  isolation needed for Build Level 3.
- GitHub `actions/attest` supports provenance, SBOM, and custom attestation modes; custom mode
  accepts `predicate-type` with `predicate` or `predicate-path`, emits Sigstore bundle JSON, and can
  upload attestations to GitHub's attestations API.
- GitHub immutable releases protect tags and release assets from modification or deletion after
  publication and create release attestations, but they do not replace artifact-bound SLSA build
  provenance for a Windlass-owned builder profile.
