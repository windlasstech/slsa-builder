---
parent: Decisions
nav_order: 1
status: "accepted"
date: 12026-06-23
decision-makers: Yunseo Kim
---

# Start slsa-builder as a Clean Repository

## Context and Problem Statement

Windlass plans to create `windlasstech/slsa-builder` to provide SLSA Build L3-oriented trusted
reusable builders for organization package build and release workflows. The original plan considered
using `slsa-framework/slsa-github-generator`, but that project carries a broad legacy surface, shows
insufficient maintenance for Windlass's needs, and has not fully incorporated current SLSA v1
specification expectations.

Specific ecosystem issues, such as the generic generator's `sha256`-oriented subject handling when
npm package provenance expects the package tarball's `sha512` digest, are examples of the risk. They
are not the primary reason for this decision. The primary concern is whether Windlass should base
new security-critical release infrastructure on a project whose modernization and ecosystem support
are not moving fast enough for Windlass to rely on it directly.

Should `slsa-builder` be created by forking `slsa-framework/slsa-github-generator`, or should it
start as a new clean repository that selectively references or ports proven ideas and implementation
details?

## Decision Drivers

- Avoid relying on an upstream project whose maintenance cadence and modernization progress are
  insufficient for Windlass's security-critical release infrastructure.
- Generate provenance compatible with current SLSA expectations, with SLSA v1.x and the latest
  stable SLSA specification as the target baseline.
- Keep the trusted computing base and long-term maintenance surface small enough for Windlass to
  audit and evolve.
- Avoid inheriting outdated or unfinished behavior from a large upstream monorepo unless it is
  deliberately selected.
- Support Windlass JS/TS package release workflows without assuming npm-only workflows or npm as the
  only package manager used during development and publishing.
- Preserve the option to contribute improvements back to the open source SLSA ecosystem.
- Make future support for additional artifact types and ecosystems possible without committing to
  all upstream builders.

## Considered Options

- Fork `slsa-framework/slsa-github-generator` and modify it in place.
- Start `slsa-builder` as a clean repository and selectively reference or port upstream design and
  implementation.
- Use upstream `slsa-github-generator` directly without maintaining a Windlass builder.

## Decision Outcome

Chosen option: "Start `slsa-builder` as a clean repository and selectively reference or port
upstream design and implementation", because it avoids inheriting insufficiently maintained and
outdated upstream surfaces while keeping the security-critical codebase intentionally small and
aligned with current SLSA expectations.

### Consequences

- Good, because `slsa-builder` can target current SLSA v1 expectations from the start instead of
  modernizing legacy assumptions after the fact.
- Good, because Windlass can define JS/TS package workflows in a way that accounts for npm, pnpm,
  and other package-manager choices rather than inheriting npm-only assumptions.
- Good, because the repository can be designed around current SLSA Build Track terminology and
  provenance formats instead of carrying SLSA v0.2-era assumptions.
- Good, because Windlass can keep a smaller trusted builder surface for review, testing, and
  incident response.
- Good, because selected upstream concepts such as reusable workflow isolation, GitHub OIDC
  identity, Sigstore signing, and BYOB-style separation can still be reused where they remain sound.
- Bad, because Windlass must recreate initial repository structure, workflows, tests, release
  automation, and verification fixtures instead of inheriting them wholesale.
- Bad, because a new project has lower immediate ecosystem recognition than a fork of an established
  OpenSSF-adjacent repository.
- Bad, because any code ported from upstream must be reviewed for license compliance, attribution,
  compatibility, and current security assumptions.

### Confirmation

This decision is confirmed when the `slsa-builder` repository is initialized as an original
repository under `windlasstech`, rather than as a GitHub fork of
`slsa-framework/slsa-github-generator`. Repository review should verify that imported or adapted
upstream code is explicit, minimal, attributed under the Apache-2.0 license where applicable, and
covered by Windlass-maintained tests and documentation.

## Pros and Cons of the Options

### Fork `slsa-framework/slsa-github-generator` and modify it in place

Create `windlasstech/slsa-github-generator` or a similarly named fork from the existing upstream
repository, then modernize and patch it for Windlass needs.

- Good, because the fork would inherit existing builders, generators, reusable workflows, signing
  logic, tests, examples, and project history.
- Good, because upstream comparison and patch contribution workflows would be straightforward.
- Good, because the project is Apache-2.0 licensed and already known in the SLSA ecosystem.
- Neutral, because the upstream repository is not archived and still has activity, including a
  v2.1.0 release in 2025 and later pushes.
- Bad, because the repository contains a wide legacy surface across generic, container, Go, Node.js,
  Maven, Gradle, Bazel, BYOB, actions, and release automation.
- Bad, because upstream modernization to current SLSA v1 expectations is incomplete and visible in
  open tracking work.
- Bad, because existing ecosystem builders and generators include assumptions that do not match all
  Windlass workflows, including package-manager choices beyond npm.
- Bad, because existing generic provenance generation is centered on `sha256sum`-style subjects,
  which creates issues for ecosystems such as npm that validate `sha512` package tarball digests.
- Bad, because forking can create an implicit expectation of broad upstream compatibility even when
  Windlass only needs a focused builder.

### Start `slsa-builder` as a clean repository and selectively reference or port upstream design and implementation

Create `windlasstech/slsa-builder` as an original repository. Use
`slsa-framework/slsa-github-generator` as prior art and selectively reuse ideas or code only after
explicit review.

- Good, because the project can optimize for current SLSA v1 semantics and Windlass's actual release
  workflows without waiting for upstream modernization.
- Good, because the project can support JS/TS packages while explicitly considering package managers
  beyond npm, including pnpm-based development and publish workflows.
- Good, because the repository can use current SLSA terminology and target current specification
  behavior without preserving old public APIs.
- Good, because a smaller scope reduces audit, test, and maintenance burden for security-sensitive
  release infrastructure.
- Good, because selected upstream design ideas can still be incorporated deliberately, including
  reusable workflow isolation and keyless signing.
- Good, because later support for generic files, containers, or other ecosystems can be added as
  explicit product decisions rather than inherited obligations.
- Neutral, because upstream remains useful as an implementation reference and possible contribution
  target, even without using GitHub's fork relationship.
- Bad, because Windlass must build initial project scaffolding, documentation, validation workflows,
  and compatibility tests.
- Bad, because a clean repository must earn trust through transparent design, tests, releases, and
  adoption rather than through upstream continuity.

### Use upstream `slsa-github-generator` directly without maintaining a Windlass builder

Continue using the upstream reusable workflows and builders wherever possible, and work around
limitations in downstream release workflows.

- Good, because this option has the lowest initial implementation cost.
- Good, because Windlass would avoid owning a security-sensitive builder project.
- Neutral, because upstream may still be sufficient for some older, narrower, or `sha256`-based
  artifact provenance use cases.
- Bad, because Windlass would still be exposed to upstream's incomplete SLSA v1 modernization.
- Bad, because it does not resolve ecosystem coverage gaps, including JS/TS workflows that use
  package managers beyond npm.
- Bad, because it does not resolve the npm `sha512` subject digest mismatch for the generic
  generator path, although that issue is only one example of the broader concern.
- Bad, because Windlass would depend on upstream response time for ecosystem-specific blockers.
- Bad, because current upstream documentation and open issues show incomplete modernization to
  current SLSA provenance expectations.
- Bad, because it limits Windlass's ability to respond quickly to organization-specific package
  publishing requirements.

## More Information

The decision is based on review of the upstream repository structure, README, provenance format
documentation, generic generator implementation, Node.js builder documentation, npm provenance
documentation, and SLSA v1.2 specification pages as of 2026-06-23.

Relevant findings:

- `slsa-framework/slsa-github-generator` provides multiple builders and generators, including
  generic file, container, Go, Node.js, Maven, Gradle, Bazel, and container-based builders.
- The upstream README describes SLSA Build Level 3-oriented reusable workflows, but the provenance
  format documentation still describes SLSA provenance v0.2.
- Upstream issues track unfinished SLSA v1.0 support for the generic generator and Node.js builder,
  while the current stable SLSA specification is v1.2.
- The upstream Node.js builder is more aligned with npm provenance than the generic generator, but
  it remains beta and has documented ecosystem limitations.
- The upstream Node.js builder's documented support is centered on npm workflows and does not
  sufficiently address Windlass's need to account for pnpm and other package managers used as
  development or publishing engines.
- The upstream generic generator parses `sha256sum`-style subject inputs and records subjects with a
  `sha256` digest.
- npm provenance requires registry verification of the published package name, version, package URL,
  and tarball `sha512` digest. This is an example of an ecosystem-specific gap, not the central
  reason for choosing a clean repository.

Initial implementation should focus on a JS/TS package-oriented trusted builder that can support
Windlass's actual package-manager choices, including npm and pnpm workflows. Generic file and
container support should be considered later through separate ADRs once concrete Windlass use cases
exist.
