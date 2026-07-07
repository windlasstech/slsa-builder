---
parent: Decisions
nav_order: 41
status: superseded by ADR-0042
date: 12026-07-06
decision-makers: Yunseo Kim
---

# Use `github-release-asset` as the GitHub Release Asset `buildType` Profile URI

## Context and Problem Statement

ADR 0021 selected Windlass-owned, profile-specific, major-versioned repository-path `buildType` URIs
for profile-owned reusable workflows. ADR 0038 added the GitHub Release asset profile and requires a
profile-specific GitHub Release asset `buildType` URI and schema. ADR 0039 scoped that profile to
one explicitly named GitHub Release asset per run. ADR 0040 selected
`.github/workflows/github-release-asset-slsa3.yml` as the profile's reusable workflow entrypoint.

The release asset build type specification will define how verifiers interpret the profile's asset
inputs, source ref, resolved source commit, artifact digest, GitHub Release linkage, and
verification rules. Because SLSA provenance uses `buildDefinition.buildType` as the schema identity
for `externalParameters`, `internalParameters`, and `resolvedDependencies`, the URI must identify
the GitHub Release asset profile semantics without confusing them with the reusable workflow path,
signing adapter, release orchestration layer, or a future generic artifact profile.

What `buildType` URI should the initial GitHub Release asset profile emit?

## Decision Drivers

- Follow ADR 0021's Windlass-owned repository-path `buildType` URI convention.
- Preserve SLSA provenance v1's separation between `builder.id` as the trusted builder identity and
  `buildType` as the parameterized build template and schema identity.
- Give verifiers a stable schema URI for interpreting release asset `externalParameters` and
  rejecting unexpected fields.
- Make GitHub Release asset semantics first-class: release tag, asset name, local asset path, upload
  target, uploaded asset identity, immutable release behavior, and GitHub attestation storage.
- Keep ADR 0039's one-asset-per-run scope visible in the profile segment.
- Avoid a generic artifact URI that hides GitHub Release-specific linkage and verification rules.
- Avoid embedding the SLSA level or workflow filename into the build type schema identity.
- Preserve a future path for a stable Windlass documentation-domain URI if documentation hosting
  changes later.

## Considered Options

- Use `https://github.com/windlasstech/slsa-builder/buildtypes/github-release-asset/v1`.
- Use `https://github.com/windlasstech/slsa-builder/buildtypes/release-asset/v1`.
- Use `https://github.com/windlasstech/slsa-builder/buildtypes/github-release-assets/v1`.
- Use `https://github.com/windlasstech/slsa-builder/buildtypes/generic-artifact/v1`.
- Use `https://github.com/windlasstech/slsa-builder/buildtypes/github-release-asset-slsa3/v1`.
- Use `https://slsa-builder.windlass.dev/buildtypes/github-release-asset/v1`.

## Decision Outcome

Chosen option: "Use
`https://github.com/windlasstech/slsa-builder/buildtypes/github-release-asset/v1`", because it
follows the existing Windlass build type URI convention while naming the selected GitHub Release
asset profile precisely and preserving one-asset-per-run semantics.

The initial GitHub Release asset profile should emit this SLSA provenance build type:

```text
https://github.com/windlasstech/slsa-builder/buildtypes/github-release-asset/v1
```

This URI identifies the release asset profile's schema and verification contract, not the workflow
filename, signing adapter, or SLSA Build level. The production reusable workflow identity selected
in ADR 0040 remains the `builder.id`, while this build type remains stable across
backward-compatible builder releases:

```text
builder.id: https://github.com/windlasstech/slsa-builder/.github/workflows/github-release-asset-slsa3.yml@0123456789abcdef0123456789abcdef01234567
buildType:  https://github.com/windlasstech/slsa-builder/buildtypes/github-release-asset/v1
```

The build type specification for this URI should define at least:

- the profile and artifact scope: exactly one explicitly named GitHub Release asset per production
  profile run;
- supported and unsupported invocation modes;
- `externalParameters` for source repository, source ref, release tag, local asset path, release
  asset name, upload intent, release policy, runtime policy, and other verifier-relevant
  caller-controlled inputs;
- `internalParameters`, if any, for trusted workflow-controlled values useful for reproduction,
  debugging, or incident response;
- `resolvedDependencies` rules, including source ref to resolved commit mapping;
- subject naming and digest rules for the single GitHub Release asset;
- byproduct rules for upload result, asset URL or API identifier, signed attestation bundle, release
  metadata, or related diagnostic artifacts;
- GitHub Release linkage rules connecting source identity, release tag, asset name, uploaded asset
  identity, and provenance subject digest;
- verifier expectations for Sigstore signature, signer identity, SHA-based `builder.id`, this
  `buildType`, complete `externalParameters`, source commit, release tag, asset name, subject
  digest, and GitHub Release linkage;
- rejection behavior for unknown `externalParameters` unless the specification explicitly allows
  them;
- exclusion of full-release, multi-asset, checksum-manifest-centered, SBOM-bundled,
  provenance-sidecar-bundled, detached-signature-bundled, and GitHub-generated source archive
  semantics from this low-level build type;
- a complete example SLSA provenance predicate and version history.

Breaking changes to required fields, field meanings, verification expectations, supported release
asset scope, or subject semantics should use a new major build type URI such as `/v2`. Backward-
compatible clarifications and optional fields may remain under `/v1` if verifiers can continue to
reject unexpected `externalParameters` according to the specification.

### Consequences

- Good, because the build type URI matches ADR 0021's existing convention.
- Good, because `github-release-asset` directly names the GitHub Release asset profile selected by
  ADR 0038 and scoped by ADR 0039.
- Good, because singular `asset` keeps the one-primary-subject verification unit visible.
- Good, because the URI lets verifiers distinguish this profile from generic artifact,
  whole-release, and multi-asset orchestration workflows.
- Good, because the URI stays independent of workflow release SHAs, signing adapter implementation,
  and SLSA Build level claims.
- Neutral, because the URI is repository-hosted rather than hosted on a dedicated documentation
  domain.
- Bad, because Windlass must maintain a concrete build type specification and fixtures for another
  profile-specific schema.
- Bad, because generic GitHub or SLSA tooling will not understand this custom build type without
  Windlass-specific policy or documentation.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification,
workflow implementation, tests, and verifier guidance define:

- `https://github.com/windlasstech/slsa-builder/buildtypes/github-release-asset/v1` as the
  production release asset profile `buildType`;
- a repository-local specification document that explains that build type;
- complete schemas for `externalParameters`, `internalParameters`, `resolvedDependencies`, subjects,
  byproducts, and release linkage;
- examples showing the distinction between SHA-based `builder.id` and this stable `buildType`;
- verifier behavior that rejects unexpected `externalParameters` unless explicitly allowed by the
  build type specification;
- accepted and rejected fixture cases for source mismatch, release tag mismatch, asset name
  mismatch, digest mismatch, upload mismatch, wrong `builder.id`, wrong `buildType`, unknown
  parameters, and unsupported full-release or multi-asset semantics;
- versioning rules for backward-compatible clarifications and breaking schema changes;
- a future-decision boundary for moving to a stable Windlass documentation-domain URI.

Implementation review should verify that production GitHub Release asset provenance does not reuse a
generic GitHub Actions workflow build type, SLSA GitHub Generator generic build type, generic
artifact build type, released workflow identity, or `actions/attest` default provenance build type
under this production builder identity.

## Pros and Cons of the Options

### Use `https://github.com/windlasstech/slsa-builder/buildtypes/github-release-asset/v1`

Use the existing Windlass repository-path convention with a singular GitHub Release asset profile
segment.

- Good, because it is the most direct match for the selected profile and one-asset scope.
- Good, because it keeps GitHub Release-specific parameters first-class.
- Good, because it decouples the schema from builder release SHAs and workflow filenames.
- Good, because a later `/v2` can represent breaking release asset schema changes cleanly.
- Neutral, because URI-to-file mapping must be documented until a dedicated documentation site
  exists.
- Bad, because it is intentionally GitHub Release-specific and not reusable for arbitrary artifacts.

### Use `https://github.com/windlasstech/slsa-builder/buildtypes/release-asset/v1`

Use a shorter profile segment that omits GitHub.

- Good, because it is concise and still names release asset semantics.
- Good, because it might appear reusable for non-GitHub release surfaces.
- Bad, because the initial profile is tightly coupled to GitHub Release tags, assets, upload
  behavior, immutable release behavior, and GitHub attestation storage.
- Bad, because future release surfaces could make the URI ambiguous.
- Bad, because GitHub-specific verifier expectations are less visible.

### Use `https://github.com/windlasstech/slsa-builder/buildtypes/github-release-assets/v1`

Use a plural profile segment for GitHub Release assets.

- Good, because GitHub Releases commonly contain multiple downloadable assets.
- Good, because it may feel natural to users thinking about complete release publication.
- Bad, because plural `assets` conflicts with ADR 0039's one-asset-per-run verification unit.
- Bad, because verifiers and users may infer multi-subject or whole-release semantics.
- Bad, because documentation would need to explain that the build type emits exactly one primary
  release asset subject.

### Use `https://github.com/windlasstech/slsa-builder/buildtypes/generic-artifact/v1`

Use a broad artifact-bound build type and treat GitHub Release upload as one specialization.

- Good, because SLSA build provenance is artifact-bound and generic file subjects are easy to model.
- Good, because it may be reusable across future file artifact profiles.
- Bad, because it broadens the profile beyond the GitHub Release asset contract selected in
  ADR 0038.
- Bad, because it hides release tag, asset name, upload target, immutable release, and GitHub
  attestation storage semantics.
- Bad, because it would conflict with a future truly generic artifact profile.

### Use `https://github.com/windlasstech/slsa-builder/buildtypes/github-release-asset-slsa3/v1`

Include the SLSA3 trust mode in the build type profile segment.

- Good, because it mirrors the production workflow filename closely.
- Good, because it makes the SLSA Build L3-oriented claim visible in the build type URI.
- Bad, because SLSA level and trust mode are better represented by `builder.id`, signer identity,
  and verifier roots of trust than by the build template schema name.
- Bad, because the same release asset schema may remain valid across future assurance modes, while
  different assurance modes should use distinct builder identities.
- Bad, because it couples schema identity to the production workflow naming convention.

### Use `https://slsa-builder.windlass.dev/buildtypes/github-release-asset/v1`

Use a stable Windlass documentation-domain URI.

- Good, because it provides the cleanest long-term human-readable documentation URI.
- Good, because it decouples build type identity from GitHub repository layout.
- Good, because it remains a good future direction if Windlass publishes stable versioned docs.
- Bad, because ADR 0021 already selected repository-path URIs for the current stage.
- Bad, because the project does not yet have the dedicated documentation domain, redirect policy, or
  publication workflow needed to make this operational.
- Bad, because changing URI families now would require broader compatibility and transition rules.

## More Information

This decision applies ADR 0021's build type URI convention to the GitHub Release asset profile. It
decides only the `buildDefinition.buildType` URI and high-level scope of its specification. It does
not decide the exact public workflow input names, output names, job graph, release creation policy,
immutable release requirement, upload implementation, checksum manifest schema, SBOM format,
provenance sidecar naming, orchestration workflow interface, or standalone verifier CLI surface.

Reference points considered:

- SLSA v1.2 Build Provenance defines `buildDefinition.buildType` as the URI identifying how to
  perform the build and interpret `externalParameters`, `internalParameters`, and
  `resolvedDependencies`.
- SLSA v1.2 Build Provenance says `buildType` URIs should resolve to a human-readable specification
  that includes parameter schemas, unambiguous initiation instructions, and a complete example.
- SLSA v1.2 Verifying Artifacts recommends verifying `buildType` and `externalParameters`, with
  unrecognized `externalParameters` causing verification failure unless allowed by policy.
- SLSA v1.2 Distributing Provenance says attestations should be bound to artifacts rather than
  releases and that source repository releases are a valid provenance distribution surface when they
  publish artifacts.
- The community GitHub Actions workflow build type describes top-level workflow execution and is not
  intended to describe reusable workflows, actions, or jobs.
- SLSA GitHub Generator uses project-owned build type URIs such as
  `https://github.com/slsa-framework/slsa-github-generator/generic@v1`, but its generic profile does
  not define Windlass GitHub Release asset semantics.
- GitHub `actions/attest` can sign custom predicates and identify subjects by path, digest, or
  checksum input, but ADR 0038 treats it as a signing and storage adapter rather than the owner of
  the Windlass release asset build type semantics.
