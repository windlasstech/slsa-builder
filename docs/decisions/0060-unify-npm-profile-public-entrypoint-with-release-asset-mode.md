---
parent: Decisions
nav_order: 60
status: accepted
date: 12026-07-29
decision-makers: Yunseo Kim
---

# Unify the npm Profile Public Entrypoint with Release-Asset Mode

## Context and Problem Statement

ADR 0022 selected `.github/workflows/js-ts-npm-package-slsa3.yml` as the public JS/TS npm package
profile entrypoint. ADR 0028 then made SHA-pinned reusable workflow identity part of verifier policy
and release metadata. ADR 0057 and ADR 0059 now define GitHub Release asset publication as an
explicit mode on that existing public npm entrypoint, while ADR 0058 keeps release mutation
authority inside the same-repository GitHub Release publisher boundary.

The remaining architectural question is whether the public API and the internal authority topology
should be coupled. One option is to expose separate public workflows for npm-only and
npm-plus-release asset paths so their internal roles appear separate to callers. Another option is
to keep one public npm API while internally separating producer, mapper, publisher, signing, and
optional metadata roles.

Which public API shape should be the final direction for the initial production npm release path?

## Decision Drivers

- Preserve a single verifier-visible public npm workflow identity for the JS/TS npm profile.
- Avoid requiring npm trusted publishing configuration for multiple workflow filenames that
  represent the same package producer profile.
- Keep caller examples and compatibility policy simple for the common npm-only path.
- Preserve least-privilege internal job boundaries for npm publish, provenance signing, release
  asset mutation, and optional linked artifact metadata.
- Keep the GitHub Release asset publisher producer-neutral and reusable by future producer profiles.
- Avoid exposing internal graph topology as a public API compatibility commitment.
- Keep release manifest metadata and verifier policy small enough for strict matching.

## Considered Options

- Expose separate public npm-only and npm-plus-release-asset workflow entrypoints.
- Expose one public npm workflow entrypoint with explicit release-asset mode while keeping internal
  permission and role boundaries separated.
- Expose only low-level producer and publisher primitives and require caller composition.

## Decision Outcome

Chosen option: "Expose one public npm workflow entrypoint with explicit release-asset mode while
keeping internal permission and role boundaries separated", because callers and verifiers need one
stable public npm API, while the implementation still needs strict internal separation of duties.

The public JS/TS npm profile API is `.github/workflows/js-ts-npm-package-slsa3.yml`. It supports the
normal npm publication path and an explicit release-asset mode for the npm-plus-GitHub-Release path.
Callers should not choose between two public npm workflow filenames for the same package producer
profile.

The internal workflow graph remains role-separated. The npm producer role builds, packs, publishes
to npm, and produces source-to-artifact provenance. The handoff mapping role converts the
digest-verified producer handoff into publisher inputs. The GitHub Release asset publisher role
verifies the producer artifact and provenance before release mutation. Signing and optional linked
artifact metadata roles retain their own permission boundaries.

The public workflow API must not imply that these internal roles share authority. Caller-facing
documentation and examples may show one reusable workflow invocation, but architecture specs and
review checklists must still verify that each internal job receives only the permissions required
for its role. In particular, release mutation authority must stay out of build, pack, npm publish,
mapping, signing, and metadata-preparation jobs, and signing or metadata authority must stay out of
the release upload job unless a future ADR changes that boundary.

The project should not add a second public npm release-asset workflow in the initial production
contract. A future ADR may add a new public workflow only if it represents a distinct producer
profile, a distinct trust boundary, or a compatibility break that cannot be expressed as a mode on
the existing npm entrypoint.

### Consequences

- Good, because caller configuration, npm trusted publishing setup, release manifest metadata, and
  verifier policy keep one public npm workflow identity.
- Good, because release-asset publication can be added without superseding ADR 0022 or fragmenting
  ADR 0028 builder identity expectations.
- Good, because the implementation can still maintain least-privilege internal jobs and a
  producer-neutral GitHub Release asset publisher.
- Good, because future producer profiles can reuse the publisher without inheriting an npm-specific
  public API shape.
- Neutral, because the public workflow schema must make mode enablement and mode-specific failures
  obvious.
- Bad, because the single public npm workflow becomes more complex than an npm-only workflow.
- Bad, because documentation must repeatedly distinguish public API unity from internal authority
  separation.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, fixtures, and
documentation define:

- `.github/workflows/js-ts-npm-package-slsa3.yml` as the only public JS/TS npm profile workflow
  entrypoint for both npm-only and npm-plus-release-asset paths;
- an explicit release-asset mode instead of a second public npm workflow filename;
- mode-specific inputs, outputs, examples, and fail-closed behavior that do not expose internal
  handoff mechanics as caller-controlled trust inputs;
- internal jobs or reusable workflow calls that preserve separate producer, handoff mapping,
  publisher, signing, release mutation, and optional linked-metadata authorities;
- caller permission examples that grant release mutation and optional metadata permissions only when
  release-asset mode requires them;
- verifier fixtures that continue to match one npm workflow `builder.id` while separately validating
  release asset locator, sidecar, and publisher-boundary evidence;
- review checklists that reject a second initial public npm release-asset entrypoint unless a future
  ADR creates a distinct trust boundary.

Implementation review should verify that the public API remains a single npm profile entrypoint and
that this simplification does not collapse the internal least-privilege role boundaries selected by
ADR 0058.

## Pros and Cons of the Options

### Expose separate public npm workflow entrypoints

The project would expose one public workflow for npm-only publication and another public workflow
for npm publication plus GitHub Release asset publication.

- Good, because each public workflow name would describe one visible release path.
- Good, because mode-specific schemas could live in separate workflow files.
- Bad, because npm trusted publishing configuration and verifier policy would need to recognize two
  public npm workflow identities for the same producer profile.
- Bad, because release manifest metadata would need another public workflow path for artifacts that
  still come from the same npm producer profile.
- Bad, because users could choose the wrong npm entrypoint and create avoidable migration pressure.

### Expose one public npm workflow with internal role separation

The project exposes one public npm workflow and adds release-asset publication as an explicit mode.
Internal jobs still preserve separate permissions and responsibilities.

- Good, because the caller-facing API matches the profile identity: one npm profile, one public npm
  entrypoint.
- Good, because internal authority can stay least-privilege even when the public invocation is a
  single `uses:` job.
- Good, because the publisher remains producer-neutral and can be reused by future profiles.
- Good, because mode-specific validation can fail closed before release mutation.
- Bad, because the workflow schema needs careful documentation so release-asset mode does not feel
  like hidden magic.

### Expose only low-level primitives

The project would expose only the npm producer and GitHub Release asset publisher primitives and ask
callers to compose them.

- Good, because it maximizes advanced composition flexibility.
- Good, because it minimizes top-level public workflow mode logic.
- Bad, because ordinary callers would have to wire security-sensitive handoff state.
- Bad, because public-output and cross-run composition are not part of the accepted initial trust
  boundary.
- Bad, because it makes the safe default harder than the advanced path.

## More Information

This decision follows ADR 0022, ADR 0028, ADR 0057, ADR 0058, and ADR 0059. It records the final
public API direction for the initial JS/TS npm profile release-asset path: one public npm
entrypoint, explicit release-asset mode, and separated internal authority boundaries.
