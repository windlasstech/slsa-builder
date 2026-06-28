---
parent: Decisions
nav_order: 2
status: accepted
date: 12026-06-23
decision-makers: Yunseo Kim
---

# Use an Extensible Trusted Reusable Workflow Foundation as the Initial SLSA Builder Architecture

## Context and Problem Statement

ADR 0001 chose to start `slsa-builder` as a clean repository rather than forking
`slsa-framework/slsa-github-generator`. The next decision is how to structure the first concrete
builder foundation so that it stays small, auditable, aligned with SLSA v1.2 Build Track
expectations, and flexible enough to support multiple ecosystems and package formats over time.
Windlass JS/TS package release workflows are the first priority, but they should be implemented as
the first ecosystem profile rather than as the permanent shape of the whole project.

Should the initial implementation be an extensible trusted reusable workflow foundation with JS/TS
as the first profile, an `actions/attest`-centered baseline workflow, or a broader custom builder
platform?

## Decision Drivers

- Keep the trusted computing base small enough for Windlass to audit and maintain.
- Target SLSA Build L3-oriented properties from the start: hosted GitHub runners, reusable workflow
  isolation, keyless signing, and provenance generation outside caller-controlled workflow steps.
- Support JS/TS package releases first, including npm and pnpm-oriented workflows where
  package-manager choice may differ between development and publishing.
- Keep the core builder architecture ecosystem-extensible so future package profiles can be added
  without redesigning the trust boundary.
- Generate SLSA v1 provenance with explicit `builder.id`, `buildType`, `externalParameters`, and
  subject handling.
- Represent package subjects through ecosystem-specific profiles so JS package URLs and tarball
  digest expectations are first-class without becoming the only supported model.
- Avoid inheriting the broad legacy surface, SLSA v0.2-era assumptions, and npm-only constraints of
  existing upstream builders.
- Preserve a straightforward path to future SBOM, generic artifact, or container support without
  committing to those surfaces initially.

## Considered Options

- Build an extensible trusted reusable workflow foundation with JS/TS as the first profile.
- Use `actions/attest` directly as the primary architecture.
- Build a broader standalone builder platform or CLI.

## Decision Outcome

Chosen option: "Build an extensible trusted reusable workflow foundation with JS/TS as the first
profile", because it gives Windlass the SLSA Build L3-oriented trust boundary needed for release
builds while keeping the first implementation intentionally small and preserving a clear path to
additional ecosystems.

The initial architecture should consist of:

- a public GitHub Actions reusable workflow foundation for trusted builds and provenance generation;
- an initial JS/TS package profile for npm and pnpm-oriented release workflows;
- small implementation modules for package subject normalization and SLSA provenance v1 predicate
  generation;
- digest-verified artifact handoff between isolated jobs;
- explicit job-level permission boundaries for build, provenance signing, and publishing;
- architecture documentation for `builder.id`, `buildType`, `externalParameters`, subject digest
  semantics, distribution, verification, and the contract for adding future ecosystem profiles.

### Consequences

- Good, because caller workflows interact with the trusted builder only through declared reusable
  workflow inputs and secrets.
- Good, because trusted build and signing logic can run on GitHub-hosted runners in isolated jobs
  rather than in caller-defined steps.
- Good, because OIDC and keyless Sigstore/GitHub artifact attestation patterns can be reused without
  storing long-lived signing keys.
- Good, because package subject handling can be profile-specific while sharing a common trusted
  workflow and provenance foundation.
- Good, because JS package ecosystem needs, including package URLs and tarball digests, can be
  handled first without making them the only architectural concern.
- Good, because SLSA provenance v1 can be the baseline format rather than carrying forward older
  predicate assumptions.
- Good, because the first trusted surface remains smaller than a fork or full Build Your Own Builder
  clone.
- Neutral, because `actions/attest` remains useful as an implementation reference and possible
  signing/storage layer.
- Bad, because Windlass must implement and maintain its own provenance schema mapping, workflow
  contract, fixtures, and verification documentation.
- Bad, because the profile boundary must be carefully designed so the foundation is extensible
  without becoming a premature multi-ecosystem platform.

### Confirmation

This decision is confirmed when the initial architecture specification defines:

- the reusable workflow entrypoint and supported triggers;
- the use of GitHub-hosted runners for trusted build and signing jobs;
- job-level permissions with signing permissions limited to the provenance or attestation job;
- a SLSA provenance v1 predicate shape using `predicateType: https://slsa.dev/provenance/v1`;
- the `builder.id` and `buildType` values and the documentation each resolves to;
- the complete `externalParameters` schema and which values are verifier-relevant;
- the profile contract for ecosystem-specific subject naming, package URL usage, digest semantics,
  and verifier expectations;
- the initial JS/TS profile's package subject naming, npm package URL usage, and digest semantics,
  including when `sha512` and `sha256` are recorded;
- digest verification for artifacts passed between isolated jobs;
- publication and verification commands for downstream users.

Implementation review should verify that release builds do not depend on self-hosted runners,
caller-provided arbitrary shell commands, caller workflow environment injection, or broad default
workflow permissions.

## Pros and Cons of the Options

### Build an extensible trusted reusable workflow foundation with JS/TS as the first profile

Create a public reusable workflow foundation in this repository and small implementation modules for
package subject handling and SLSA provenance v1 generation. The foundation defines the trusted
build, artifact handoff, provenance, signing, and verification contracts. The first concrete profile
supports JS/TS package releases, while future profiles can add other ecosystems or artifact types
without redefining the core trust boundary.

- Good, because GitHub reusable workflows provide a narrow interface: callers can pass declared
  inputs and secrets but cannot inject extra steps, services, defaults, or environment into the
  trusted workflow.
- Good, because separate GitHub-hosted jobs provide an isolation boundary between build, provenance
  generation, and publishing concerns.
- Good, because the design can use low-permission defaults and elevate only the jobs that require
  OIDC, attestation, package publishing, or release upload permissions.
- Good, because the core architecture can support multiple package subject models without collapsing
  them into a lowest-common-denominator generic `sha256sum`-style artifact model.
- Good, because the JS/TS package subject model can be first-class in the initial profile while
  remaining one profile among several possible profiles.
- Good, because the workflow can document its trust boundary and expected verification policy
  through stable `builder.id` and `buildType` documentation.
- Neutral, because it still depends on GitHub Actions, GitHub-hosted runner isolation, OIDC, and
  Sigstore or GitHub artifact attestation infrastructure as part of the trusted platform.
- Bad, because Windlass must write and maintain the initial reusable workflow foundation, profile
  boundary, subject normalization, provenance predicate generation, fixtures, and docs.

### Use `actions/attest` directly as the primary architecture

Build artifacts in each caller workflow and add `actions/attest` as a post-build step to generate
GitHub artifact attestations.

- Good, because it uses GitHub's official action for signed attestations and GitHub attestations API
  integration.
- Good, because it provides a simple baseline path for repositories that cannot yet adopt a trusted
  reusable builder.
- Good, because it already supports provenance, SBOM, and custom attestation modes.
- Neutral, because it may remain useful inside or alongside the trusted reusable workflow foundation
  for signing, storage, or GitHub API integration.
- Bad, because caller-controlled workflow steps produce the artifact before attestation, weakening
  the SLSA Build L3 trust-boundary story.
- Bad, because it does not by itself enforce a Windlass-defined trusted build contract, ecosystem
  profile boundary, subject naming policy, or package-manager policy.
- Bad, because direct use is better treated as a Build L2-oriented baseline or fallback than as the
  project's first trusted builder architecture.

### Build a broader standalone builder platform or CLI

Create a dedicated builder CLI or service that checks out source, builds artifacts, signs
provenance, publishes artifacts, and potentially supports multiple CI systems or artifact
ecosystems.

- Good, because it could eventually support non-GitHub environments, generic files, containers, and
  additional package ecosystems.
- Good, because a custom platform could define its own trust boundary without being limited to
  GitHub Actions reusable workflow semantics.
- Bad, because it substantially increases the initial trusted computing base and operational burden.
- Bad, because Windlass would need to define and secure runner isolation, signing key management,
  artifact storage, logging, distribution, and verification infrastructure beyond the current need.
- Bad, because it conflicts with the clean repository decision's intent to start small and
  selectively reuse proven ideas rather than building a broad platform immediately.

## More Information

This decision follows ADR 0001 and adopts the reusable workflow foundation option identified during
review of SLSA v1.2 Build Track requirements, `slsa-framework/slsa-github-generator`, its Build Your
Own Builder design, and GitHub `actions/attest`.

Relevant reference points:

- SLSA v1.2 Build requirements define Build L3 around hosted, isolated build environments and
  unforgeable provenance generated by the trusted build platform.
- SLSA provenance v1 requires `predicateType: https://slsa.dev/provenance/v1`, a documented
  `builder.id`, a `buildType`, complete verifier-relevant `externalParameters`, and artifact
  subjects identified by digest.
- `slsa-framework/slsa-github-generator` demonstrates reusable workflow isolation, keyless signing,
  low-permission builder variants, and digest-verified job handoff, but also carries a broad
  multi-ecosystem surface and older provenance assumptions that this project should not inherit
  wholesale.
- `actions/attest` demonstrates a focused TypeScript action for subject parsing, attestation mode
  selection, OIDC-backed Sigstore signing, GitHub attestations API upload, SBOM support, and linked
  artifact storage, but direct post-build use should be treated as a baseline rather than the
  primary SLSA Build L3-oriented builder boundary.

Initial implementation should prioritize the JS/TS package profile, but the foundation should define
extension points for additional ecosystem profiles. Generic artifact and container builders should
still wait until concrete Windlass release use cases justify separate ADRs and architecture
specifications.
