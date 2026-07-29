---
parent: Decisions
nav_order: 57
status: accepted
date: 12026-07-29
decision-makers: Yunseo Kim
---

# Provide a Public npm Release-Asset Mode

## Context and Problem Statement

ADR 0052 selected the JS/TS npm package tarball as the first producer artifact to feed the GitHub
Release asset publisher. The architecture specifications now define the same-run internal handoff
that lets the npm producer pass the tarball, producer provenance, release target, trusted producer
identity, and digest-verified metadata to the publisher.

That internal composition still leaves a public API question: how should callers invoke the combined
release path when they want to publish an npm package and attach the same verified package tarball
plus unchanged producer-provenance sidecar to a GitHub Release?

The public surface must preserve the current trust boundary. The internal handoff manifest is
producer-owned, digest verified, and scoped to one workflow run. Promoting its artifact names and
digests into ordinary public `workflow_call.outputs` would create a new composition contract between
separately invoked reusable workflows. That contract may be valuable later, but it is a different
trust boundary from the same-run composition already specified.

At the same time, ADR 0022 already selected `.github/workflows/js-ts-npm-package-slsa3.yml` as the
public JS/TS npm package profile entrypoint. Adding a second public npm entrypoint only for GitHub
Release asset publication would split caller documentation, npm trusted publishing configuration,
verifier-visible builder identity expectations, release manifest metadata, and compatibility policy.

Should the project expose no combined public path, expose a separate combined public workflow, add a
release-asset mode to the existing public npm workflow while retaining low-level profile surfaces,
or provide only copyable workflow templates?

## Decision Drivers

- Keep the same-run producer-to-publisher handoff as the initial production trust boundary.
- Give normal callers one recommended reusable workflow entrypoint for npm publication, with an
  explicit opt-in path for GitHub Release asset publication.
- Preserve the existing ADR 0022 public npm workflow name and ADR 0028 builder identity model.
- Avoid requiring callers to wire internal artifact names, provenance bundle names, or handoff
  manifest digests by hand.
- Preserve the standalone npm producer and GitHub Release publisher as profile-owned reusable
  workflows that can be used by advanced composition authors and future producer profiles.
- Do not support cross-run or separately invoked workflow composition through public outputs until a
  future ADR specifies that trust boundary.
- Keep the publisher producer-neutral rather than turning it into an npm-specific workflow.
- Align with SLSA's recommendation that provenance accompany the artifact and be bound to the
  artifact, not merely to the release as a whole.
- Account for GitHub Actions reusable workflow constraints, including explicit `workflow_call`
  schemas and the fact that nested reusable workflows can only maintain or reduce caller-granted
  `GITHUB_TOKEN` permissions.

## Considered Options

- Expose no combined public path; require callers to connect the npm producer and publisher
  themselves.
- Expose a separate combined public npm-to-release-asset workflow.
- Add a GitHub Release asset publication mode to the existing public npm workflow while retaining
  standalone low-level producer and publisher workflows as advanced composition primitives.
- Provide only workflow templates or examples that callers copy into their repositories.

## Decision Outcome

Chosen option: "Add a GitHub Release asset publication mode to the existing public npm workflow
while retaining standalone low-level producer and publisher workflows as advanced composition
primitives", because it gives ordinary callers a safe one-job `uses:` surface without creating a
second public npm profile entrypoint.

The project should provide the complete npm-to-GitHub Release asset composition through the existing
public JS/TS npm profile workflow, `.github/workflows/js-ts-npm-package-slsa3.yml`. Release asset
publication is an explicit opt-in mode of that workflow. A caller that enables the mode can use one
release run to:

1. build and pack one JS/TS npm package;
2. generate and sign Windlass SLSA provenance for the package tarball;
3. publish the package to npm using the producer profile's trusted publishing path;
4. map the producer-owned same-run handoff manifest to publisher inputs; and
5. upload the same tarball plus the unchanged producer provenance sidecar to an existing GitHub
   Release.

The public npm workflow should own the same-run composition graph when release-asset mode is
enabled. Callers interact with it through its declared `workflow_call` inputs, secrets, permissions,
and outputs. The internal handoff manifest, internal artifact names, provenance bundle transport
names, and mapping job outputs remain internal to that workflow graph. They must not become stable
public `workflow_call.outputs` for connecting separately invoked reusable workflows in the initial
production contract.

The standalone JS/TS npm package producer and GitHub Release asset publisher workflows remain
available as lower-level profile surfaces. They are advanced composition primitives, not the
recommended caller path for the npm-to-release-asset release. Their direct use is supported only
when the caller or a future composed workflow preserves the profile's documented trust boundary,
digest-verified same-run artifact handoff, producer provenance verification, and least-privilege job
separation.

This advanced low-level model is analogous in spirit to the SLSA GitHub Generator's BYOB model:
expert users can assemble trusted workflow primitives into a custom release graph, but the safe
default remains the public npm profile workflow whose release-asset mode hides security-sensitive
internal handoff details. It is not itself a Build-Your-Own-Builder framework; for this project, it
is a Build-Your-Own-Composition pattern over already-defined producer and publisher profiles.

The initial production contract does not support a separate public npm-to-release-asset workflow,
public-output composition between separately invoked producer and publisher workflows, cross-run
artifact handoff, caller-supplied artifact names as trust inputs, raw file uploads without accepted
producer provenance, or multi-asset publication in one publisher invocation. Any of those
capabilities requires a future ADR because each changes the public trust boundary and verifier
expectations.

The release-asset mode should expose user-intent inputs and outputs rather than internal handoff
mechanics. Specification updates should define the exact `workflow_call` schema, but the public
surface should be shaped around package selection, build options, npm publish options, release tag,
same-repository target policy, optional linked artifact metadata settings, publication results,
release asset locators, provenance sidecar locators, and verifier-relevant digests.

### Consequences

- Good, because normal callers keep one recommended reusable workflow entrypoint for npm package
  publication and optional GitHub Release asset distribution.
- Good, because the producer-owned same-run handoff manifest remains internal and digest verified
  instead of becoming a loosely connected public output API.
- Good, because the standalone producer and publisher remain reusable for future composed workflows
  and expert integrations.
- Good, because the public surface maps to release intent, while the implementation keeps the
  security-sensitive handoff details inside the trusted workflow graph.
- Good, because SLSA provenance distribution is handled alongside the release artifact as an
  artifact-bound sidecar.
- Good, because the project avoids an additional public npm workflow filename, builder identity, and
  npm trusted publishing configuration surface.
- Neutral, because the project must document two levels of use: the public npm workflow's normal and
  release-asset modes, plus lower-level advanced profile primitives.
- Neutral, because caller workflows still need to grant sufficient permissions; nested reusable
  workflows cannot elevate permissions that the caller withholds.
- Bad, because the existing public npm workflow becomes broader and needs clear mode-specific
  validation, examples, and fixtures.
- Bad, because advanced users who need cross-run orchestration must wait for a later public
  composition contract.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, fixtures, and
documentation define:

- `.github/workflows/js-ts-npm-package-slsa3.yml` as the single public npm profile entrypoint for
  both ordinary npm publication and explicit GitHub Release asset publication mode;
- the release-asset mode's complete `workflow_call` inputs, secrets, permissions, outputs, runtime
  guards, and failure behavior;
- the same-run internal graph that connects npm producer jobs, the handoff mapping job, and the
  GitHub Release asset publisher when release-asset mode is enabled;
- confirmation that internal artifact names, handoff manifest names, and handoff manifest digests
  are not standalone public `workflow_call.outputs` for separately invoked reusable workflows;
- the allowed advanced use of standalone producer and publisher workflows as low-level composition
  primitives;
- rejection or non-support statements for a separate public npm-to-release-asset workflow, cross-run
  artifact handoff, public-output composition, caller-supplied artifact-name trust inputs, raw files
  without producer provenance, and multi-asset publisher invocations;
- caller permission examples proving that the public npm workflow can receive only the permissions
  it needs and that insufficient permissions fail before release mutation;
- verification fixtures that prove the publisher receives values from the digest-verified internal
  handoff manifest rather than caller-controlled public outputs.

Implementation review should verify that the public npm workflow does not leak its internal handoff
API as a stable caller contract, does not require callers to reconstruct publisher inputs by hand,
and does not weaken the standalone publisher's producer-neutral trust boundary.

## Pros and Cons of the Options

### Expose no combined public path

Callers would invoke the npm producer reusable workflow and then directly connect its outputs or
artifacts to the GitHub Release asset publisher.

- Good, because it minimizes the public npm workflow surface.
- Good, because it resembles the SLSA GitHub Generator pattern of a builder reusable workflow plus a
  separate publish action or custom publish job.
- Bad, because callers would need to handle security-sensitive composition details that the current
  architecture treats as internal same-run handoff state.
- Bad, because public-output composition between separately invoked workflows is a new trust
  boundary and is not specified yet.
- Bad, because ordinary users are more likely to omit sidecar publication, mismatch digests, or
  trust artifact names without digest verification.

### Expose a separate combined public npm-to-release-asset workflow

The project would add a second public npm profile entrypoint dedicated to npm publication plus
GitHub Release asset distribution.

- Good, because it keeps the release-asset schema visually separate from the npm-only schema.
- Good, because examples for release publishing could point at a purpose-named workflow.
- Bad, because it splits public npm API compatibility across two workflow filenames.
- Bad, because it splits verifier-visible builder identity expectations and release manifest
  metadata for artifacts that are still produced by the same npm profile.
- Bad, because npm trusted publishing configuration is tied to workflow identity in caller
  repositories, so a second public filename increases caller setup and migration risk.

### Add release-asset mode to the existing public npm workflow

The existing public npm profile entrypoint remains the recommended caller surface. Release asset
publication is an explicit mode that internally composes the npm producer with the GitHub Release
asset publisher.

- Good, because it balances safe defaults with profile extensibility.
- Good, because it matches the already-specified same-run handoff model.
- Good, because caller documentation, npm trusted publishing configuration, verifier policy, and
  release manifest metadata continue to reference one npm workflow identity.
- Good, because future producers can compose with the same publisher without changing the default
  npm workflow.
- Good, because low-level direct composition can be documented as a Build-Your-Own-Composition
  pattern rather than a casual public-output API.
- Bad, because the single public workflow needs precise mode validation so the npm-only path and the
  release-asset path remain easy to reason about.

### Provide only workflow templates or examples

The project would publish copyable workflow YAML instead of a trusted reusable workflow surface.

- Good, because templates are flexible and easy for callers to customize.
- Bad, because copied YAML drifts from the trusted reusable workflow identity that verifiers need to
  recognize.
- Bad, because templates do not provide the same centralized trusted workflow boundary as a reusable
  workflow.
- Bad, because this conflicts with the project's profile-owned reusable workflow foundation.

## More Information

This decision follows ADR 0002, ADR 0003, ADR 0022, ADR 0028, ADR 0049, ADR 0050, ADR 0051, and
ADR 0052. It resolves the public orchestration surface left open after the npm producer to GitHub
Release publisher composition was selected.

Reference points considered:

- SLSA v1.2 requires producers to distribute provenance and recommends artifact-bound provenance
  that accompanies the artifact, including source repository release sidecars such as
  `.intoto.jsonl`.
- SLSA v1.2 verification guidance expects verifiers to check provenance signatures, `builder.id`,
  `buildType`, `externalParameters`, and artifact subject digests against expectations.
- GitHub Actions reusable workflows require explicit `workflow_call` inputs, secrets, and outputs;
  nested reusable workflows can only maintain or reduce the caller-granted `GITHUB_TOKEN`
  permissions.
- GitHub's secure-use guidance recommends full-SHA pinning for third-party actions and workflows;
  this project additionally records trusted workflow identities in release metadata and verifier
  policy.
- npm trusted publishing ties package publishing authorization to GitHub repository and workflow
  identity, so retaining one public npm workflow identity reduces caller setup and verification
  drift.
- The SLSA GitHub Generator Node.js builder exposes a reusable builder workflow and separate publish
  actions or custom publish steps. That pattern validates the value of low-level primitives, while
  this project chooses a safer public npm workflow mode because its publisher trust boundary depends
  on a same-run internal handoff manifest.
