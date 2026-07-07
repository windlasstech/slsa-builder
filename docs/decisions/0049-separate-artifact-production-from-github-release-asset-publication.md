---
parent: Decisions
nav_order: 49
status: accepted
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Separate Artifact Production from GitHub Release Asset Publication

## Context and Problem Statement

ADR 0038 through ADR 0048 defined an initial GitHub Release asset profile as a sibling production
profile that would generate Windlass-owned SLSA provenance, sign it with `actions/attest`, and
upload one release asset to an existing GitHub Release. Those decisions intentionally deferred the
last unresolved boundary: how the release asset bytes are produced or ingested before signing and
upload.

Further review of SLSA v1.2, `slsa-github-generator`, GitHub reusable workflows, and
`actions/attest` shows that a generic GitHub Release asset build profile would either need to run a
broad repository-defined build recipe or accept prebuilt artifacts from caller workflows. The first
path risks becoming a lowest-common-denominator generic build runner. The second path more closely
resembles a provenance generator or promotion workflow, because the release asset bytes are produced
outside the GitHub Release asset profile's trusted build boundary.

Should Windlass keep a generic GitHub Release asset build profile, redefine the GitHub Release asset
workflow as a publisher that creates its own publication attestation, or redefine it as a verified
distributor that republishes producer-generated provenance unchanged?

## Decision Drivers

- Preserve the profile-owned architecture from ADR 0002 and ADR 0003 without turning the GitHub
  Release asset surface into a generic arbitrary build runner.
- Keep source-to-artifact build semantics in ecosystem-specific profiles that can define precise
  toolchain, dependency, subject, and verifier contracts.
- Keep GitHub Release upload policy separate from artifact production policy.
- Avoid claiming that a publisher workflow built an artifact from source when it only received,
  verified, and uploaded already-produced bytes.
- Preserve SLSA v1.2 Build L3 expectations that `externalParameters` are complete and that signing
  authority is isolated from user-controlled build steps.
- Keep room for Go, Rust, JS/TS, container, archive, and other producer profiles to publish through
  the same GitHub Release asset publication surface.
- Require any production publication path to fail closed when upstream producer provenance is
  missing or does not match the artifact being uploaded.
- Avoid the cost and semantic ambiguity of a publisher-owned SLSA provenance or custom publication
  predicate in the default production path.

## Considered Options

- Redefine GitHub Release asset handling as a verified distributor for ecosystem-produced artifacts
  and producer-generated provenance.
- Redefine GitHub Release asset handling as a publisher that signs its own publication or binding
  attestation.
- Keep a generic GitHub Release asset build profile that runs a source-controlled build recipe.
- Accept arbitrary caller-provided build commands in the GitHub Release asset profile.
- Accept caller-produced workflow artifacts without mandatory upstream provenance verification.
- Skip a GitHub Release asset profile and leave publication entirely to ecosystem-specific profiles.

## Decision Outcome

Chosen option: "Redefine GitHub Release asset handling as a verified distributor for
ecosystem-produced artifacts and producer-generated provenance", because GitHub Release upload is a
distribution concern while the artifact's source-to-bytes SLSA provenance belongs to the producer
profile that knows the ecosystem.

Windlass will not ship a generic production GitHub Release asset build profile. Ecosystem-specific
producer profiles should produce final artifact bytes and source-to-artifact SLSA provenance. A
GitHub Release asset publisher profile should then verify the produced artifact and its upstream
producer provenance, upload the exact verified bytes to an existing GitHub Release, and publish or
expose the producer-generated provenance without modifying or re-signing it.

The publisher profile's public workflow entrypoint should be renamed away from
`.github/workflows/github-release-asset-slsa3.yml`. The production entrypoint should use publication
language such as `.github/workflows/github-release-asset-publish.yml` or an equivalent name selected
by the architecture specification. The old `slsa3` filename is misleading for a workflow that does
not itself build the asset from source.

The publisher profile should not define a production SLSA `buildType` for GitHub Release asset
publication in the default path, because it does not produce the artifact from source and does not
generate a new SLSA provenance statement. The concrete GitHub Release asset `buildType` value
implied by ADR 0042 is removed for this surface. The producer profile's `buildType` remains the
source-to-artifact build type that consumers must verify.

The semantic boundary is fixed by this ADR: the GitHub Release asset publisher profile verifies and
publishes. It does not claim to compile, package, otherwise produce arbitrary release asset bytes
from source, or issue an independent SLSA provenance statement for the publication operation.

The publisher profile must not accept raw unsigned artifacts as a production SLSA-equivalent path.
If a produced artifact lacks acceptable upstream producer provenance, the default production
publisher path must fail before mutating the GitHub Release. A separate lower assurance ingestion
workflow or experimental mode may be added only through a future ADR with a distinct workflow name,
predicate or evidence model, and verifier guidance.

This ADR supersedes the source-to-release-asset build-profile interpretation in:

- ADR 0038, which selected Windlass-generated SLSA build provenance for GitHub Release assets as if
  the release asset profile owned artifact production;
- ADR 0040, which selected `.github/workflows/github-release-asset-slsa3.yml` as the production
  workflow entrypoint;
- ADR 0044, which selected a `build` → `provenance-sign` → `release-upload` graph where the release
  asset profile runs caller-controlled build or packaging steps;
- the concrete GitHub Release asset `buildType` value implied by ADR 0042, while preserving ADR
  0042's domain and URI namespace decision.

ADR 0039's one-asset-per-run boundary, ADR 0043's existing-release upload policy, ADR 0045's final
release asset subject name, ADR 0046's selected-asset digest semantics, and ADR 0048's
linked-artifact opt-in policy remain valid where they are interpreted as publication-profile rules
rather than artifact-production rules.

### Consequences

- Good, because each producer profile can define precise ecosystem build semantics instead of
  forcing all release assets through a generic build recipe.
- Good, because the GitHub Release asset workflow becomes smaller and focused on verification,
  upload policy, provenance distribution, and release asset binding.
- Good, because the architecture avoids overclaiming that a publisher workflow produced bytes it
  only received.
- Good, because the default path avoids defining a Windlass custom publication predicate before the
  project has a concrete need for one.
- Good, because future ecosystem profiles can compose with the same publication surface while
  keeping distinct `buildType` and `builder.id` values.
- Neutral, because users need an ecosystem producer profile before they can use the production
  publisher path.
- Neutral, because consumers verify producer provenance directly and rely on GitHub Release state
  for publication discovery rather than a publisher-signed attestation.
- Bad, because the previous GitHub Release asset ADR sequence needs supersession and terminology
  cleanup before stable architecture specifications are written.
- Bad, because the project must define a producer-to-publisher handoff contract before implementing
  the publisher workflow.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, tests, and
verifier guidance define:

- no generic production GitHub Release asset build profile;
- ecosystem-specific producer profiles as the source-to-artifact build surfaces;
- a renamed GitHub Release asset publisher workflow entrypoint that does not use the old
  `github-release-asset-slsa3.yml` name;
- no publisher-owned SLSA provenance or custom publication attestation in the default production
  path;
- producer-generated SLSA provenance as the only source-to-artifact provenance for the uploaded
  release asset;
- mandatory upstream producer provenance verification before production publication;
- failure before GitHub Release mutation when upstream producer provenance is missing, untrusted, or
  mismatched with the artifact digest;
- verifier guidance that treats producer provenance as the provenance consumers must verify for the
  uploaded release asset;
- separate builder identities for producer profiles and the publisher profile.

Implementation review should verify that no production path silently accepts raw caller-produced
artifacts, generates publisher-owned SLSA provenance for the release asset, labels publication-only
evidence as source-to-artifact build provenance, or reuses the old generic release asset builder
identity after this boundary change.

## Pros and Cons of the Options

### Redefine GitHub Release asset handling as a verified distributor

Ecosystem-specific producer profiles build final artifacts and generate source-to-artifact
provenance. The GitHub Release asset publisher profile verifies the artifact and upstream producer
provenance, uploads the verified bytes to an existing GitHub Release, and publishes or exposes the
producer-generated provenance unchanged.

- Good, because artifact production semantics stay with the profile that understands the ecosystem.
- Good, because GitHub Release upload is treated as a distribution and publication concern.
- Good, because the publisher profile can be reused across producer profiles without becoming a
  generic build runner.
- Good, because the trust statement is honest: producer profiles build and attest, the publisher
  verifies and distributes.
- Good, because the default path does not require a new Windlass publication predicate.
- Neutral, because the publisher's verification action is not itself represented by a new signed
  attestation.
- Bad, because implementation depends on a precise producer-to-publisher handoff contract.

### Sign publisher-owned publication or binding evidence

The GitHub Release asset publisher verifies producer provenance, then creates a new SLSA provenance
statement or custom publication predicate for the publication operation.

- Good, because the publisher's verification result and release-slot binding are signed explicitly.
- Good, because downstream automation can verify a producer attestation and a publisher attestation
  as a chain.
- Bad, because SLSA provenance is awkward for a workflow that does not build the artifact from
  source.
- Bad, because a custom publication predicate adds schema design, versioning, documentation, and
  verifier implementation cost.
- Bad, because consumers may mistake publisher evidence for source-to-artifact provenance.

### Keep a generic release asset build profile with source-controlled recipe execution

The GitHub Release asset profile checks out source, runs a repository-declared build script or
recipe, signs provenance for the resulting asset, and uploads it.

- Good, because one workflow can build, sign, and upload one release asset.
- Good, because the signing and upload authorities can be isolated from the build job.
- Neutral, because SLSA v1.2 does not prohibit user-defined build steps when provenance and signing
  controls are correct.
- Bad, because the profile becomes a broad generic build runner with weak ecosystem semantics.
- Bad, because verifier expectations must account for arbitrary repository recipes, output paths,
  toolchains, environment behavior, and network access.
- Bad, because the profile can drift toward arbitrary command execution over time.

### Accept arbitrary caller-provided build commands

The caller passes a command string such as `make release` and the release asset profile executes it
in the build job.

- Good, because existing workflows are easy to adapt.
- Bad, because command strings become verifier-relevant external parameters that are difficult to
  review and constrain.
- Bad, because shell behavior, quoting, environment expansion, and network access create a broad and
  fragile build interface.
- Bad, because this undermines the goal of keeping build definitions in source-controlled ecosystem
  profiles.

### Accept caller-produced artifacts without mandatory upstream provenance verification

The caller builds an artifact in another workflow job and passes the bytes, workflow artifact name,
or artifact ID to the GitHub Release asset workflow for signing and upload.

- Good, because it composes with existing release workflows and complex matrix builds.
- Good, because it resembles the `slsa-github-generator` generic generator pattern.
- Bad, because the publisher workflow did not produce the bytes and should not claim
  source-to-artifact build provenance for them.
- Bad, because workflow artifact transport is not a substitute for trusted producer provenance.
- Bad, because missing upstream provenance would let arbitrary bytes enter the production publisher
  path.

### Leave publication entirely to ecosystem-specific profiles

Each ecosystem-specific profile builds, signs, and uploads its own GitHub Release assets directly.

- Good, because every ecosystem profile can fully own its build and release lifecycle.
- Good, because no separate producer-to-publisher handoff is needed.
- Bad, because release upload policy, existing-release checks, duplicate-asset behavior, optional
  sidecars, and linked-artifact metadata would be duplicated across ecosystem profiles.
- Bad, because whole-project GitHub Release publication behavior would become inconsistent across
  profiles.

## More Information

This decision follows ADR 0002, ADR 0003, ADR 0013, ADR 0038, ADR 0039, ADR 0040, ADR 0042, ADR
0043, ADR 0044, ADR 0045, ADR 0046, ADR 0047, and ADR 0048. It decides the architecture boundary
between artifact production and GitHub Release asset publication. It does not decide the exact
handoff fields, upstream provenance policy format, producer provenance distribution channel, or
final workflow implementation details; those are decided by ADR 0050, ADR 0051, and the architecture
specifications.

Reference points considered:

- SLSA v1.2 Build L3 requires unforgeable provenance, isolated builds, and complete
  `externalParameters`, but it does not require one generic build interface for all artifact types.
- SLSA v1.2 Build Provenance separates `buildType`, `externalParameters`, `resolvedDependencies`,
  `builder.id`, and subject digest so different build or publication operations can use distinct
  schemas and trust boundaries.
- `slsa-github-generator` distinguishes ecosystem-specific builders, which build artifacts and
  generate provenance, from generic generators, which attest subjects produced by an existing
  workflow.
- GitHub reusable workflows accept typed inputs, secrets, and outputs; callers cannot inject steps
  into a reusable workflow job.
- GitHub `actions/attest` can sign a subject name and digest with a custom predicate and publish the
  resulting bundle to GitHub artifact attestation storage.
