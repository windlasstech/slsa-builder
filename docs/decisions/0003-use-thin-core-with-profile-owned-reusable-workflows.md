---
parent: Decisions
nav_order: 3
status: accepted
date: 12026-06-23
decision-makers: Yunseo Kim
---

# Use a Thin Core with Profile-Owned Reusable Workflows

## Context and Problem Statement

ADR 0002 chose an extensible trusted reusable workflow foundation with JS/TS as the first ecosystem
profile. The next decision is where to draw the boundary between shared implementation and language
or ecosystem-specific behavior. The boundary must preserve the SLSA Build L3-oriented trust model
while keeping the initial JS/TS implementation practical and leaving room for future profiles.

Should `slsa-builder` use a thin shared core with profile-owned reusable workflows, a BYOB-style
callback framework, or a generic artifact core with ecosystem metadata overlays?

## Decision Drivers

- Keep the trusted computing base small enough to audit and explain.
- Preserve the reusable workflow isolation boundary chosen in ADR 0002.
- Make JS/TS package releases first-class without making JS/TS assumptions part of the shared core.
- Keep SLSA provenance v1 generation centered on documented `builder.id`, `buildType`, complete
  `externalParameters`, and subject digests.
- Avoid recreating the full `slsa-framework/slsa-github-generator` Build Your Own Builder framework
  before this project needs that level of plugin abstraction.
- Make each ecosystem profile independently reviewable, testable, and documentable.
- Reduce the risk that generic artifact handling erases package ecosystem semantics such as package
  URLs, registry verification rules, or digest algorithm expectations.

## Considered Options

- Use a thin core with profile-owned reusable workflows.
- Build a BYOB-style profile callback framework.
- Use a generic artifact core with ecosystem metadata overlays.
- Use direct `actions/attest` wrappers as the primary profile boundary.

## Decision Outcome

Chosen option: "Use a thin core with profile-owned reusable workflows", because it gives
`slsa-builder` a small shared trust and provenance foundation while allowing each ecosystem profile
to own the workflow, subject, digest, publish, and verification semantics that make that ecosystem
correct.

The shared core should own:

- trusted workflow invariants, including hosted runner requirements, job isolation, and no
  caller-injected arbitrary steps, services, defaults, or environment;
- digest-verified artifact handoff between isolated jobs;
- common SLSA provenance v1 statement and predicate construction;
- `builder.id` documentation conventions and shared `buildType` URI conventions;
- validation that profile `externalParameters` are complete, explicit, and verifier-relevant;
- common subject and digest plumbing where it does not erase ecosystem semantics;
- signing, attestation storage, and optional `actions/attest` integration as implementation
  adapters;
- shared verification documentation conventions.

Each profile-owned reusable workflow should own:

- the profile workflow entrypoint and supported triggers;
- ecosystem toolchain setup and build or pack steps;
- package or artifact subject naming;
- digest algorithm semantics, including ecosystem-specific expectations such as npm tarball `sha512`
  handling;
- profile-specific `buildType` and `externalParameters` schema;
- registry publish and verification behavior;
- profile fixtures, examples, and downstream verification commands.

The initial JS/TS profile should be implemented as the first profile-owned reusable workflow using
shared core modules or actions for provenance, digest verification, signing, and documentation
conventions.

### Consequences

- Good, because the core remains focused on trust, provenance, signing, and handoff rather than
  ecosystem-specific package behavior.
- Good, because JS/TS can be first-class without freezing npm, pnpm, or Node.js assumptions into the
  whole project.
- Good, because future profiles can be added by defining a new reusable workflow and profile
  contract rather than redesigning the core trust boundary.
- Good, because profile workflows remain directly inspectable as GitHub Actions documents, which
  improves auditability.
- Good, because `actions/attest` can still be used as a signing or storage adapter without becoming
  the architectural boundary.
- Neutral, because some YAML structure and profile documentation will be duplicated across profiles.
- Bad, because profile workflows can drift unless shared invariants are tested, linted, or reviewed
  consistently.
- Bad, because this structure requires a clear profile contract even before multiple profiles exist.

### Confirmation

This decision is confirmed when the initial architecture specification and JS/TS profile define:

- which modules, actions, or workflow fragments are shared core;
- which workflow file is the JS/TS profile-owned reusable workflow;
- the profile contract for future profiles, including required inputs, outputs, `buildType`,
  `externalParameters`, subjects, digest semantics, publish rules, and verification docs;
- checks or review criteria that prevent profile workflows from bypassing hosted runners, job
  isolation, digest-verified handoff, minimal permissions, and trusted provenance generation;
- documentation explaining that direct `actions/attest` use is a baseline or adapter path, not the
  primary trusted-builder profile boundary.

Implementation review should verify that adding a new profile does not require changing the core
trust model unless a new ADR explicitly changes that model.

## Pros and Cons of the Options

### Use a thin core with profile-owned reusable workflows

Keep shared code and shared workflow conventions focused on the trusted builder boundary, provenance
generation, signing, attestation distribution, artifact handoff, and verification documentation.
Represent each ecosystem as its own reusable workflow plus profile-specific schema, subject rules,
digest rules, publishing behavior, fixtures, and verification examples.

- Good, because it matches ADR 0002's extensible reusable workflow foundation while making the
  profile boundary concrete.
- Good, because profile-specific workflow files are easier to audit than a generic plugin engine.
- Good, because package ecosystems can use their own subject names and digest algorithms without
  compromising the shared core.
- Good, because initial JS/TS implementation can be direct and small.
- Neutral, because shared modules or actions still need stable interfaces for profiles to call.
- Bad, because duplicated workflow structure may grow as more profiles are added.
- Bad, because invariant enforcement must be deliberate to avoid profile drift.

### Build a BYOB-style profile callback framework

Build a core reusable workflow that acts as an orchestrator and invokes profile callback actions.
Each callback would build or package artifacts and emit metadata that the core uses to generate
provenance and attestations.

- Good, because it provides a highly standardized extension mechanism for many profiles.
- Good, because it resembles the proven separation in `slsa-framework/slsa-github-generator` Build
  Your Own Builder.
- Good, because the core can strongly control orchestration, handoff, provenance, and signing.
- Neutral, because this option may become useful if `slsa-builder` grows many mature profiles with
  similar lifecycle needs.
- Bad, because it creates more framework surface than the first JS/TS profile needs.
- Bad, because the callback interface becomes another trusted API that must be secured, versioned,
  tested, and documented.
- Bad, because it risks recreating the broad framework that ADR 0001 deliberately avoided inheriting
  wholesale.

### Use a generic artifact core with ecosystem metadata overlays

Treat every profile output as a generic artifact name and digest, then add package ecosystem
information as metadata or annotations around that generic subject model.

- Good, because it creates the smallest apparent common model.
- Good, because it works naturally for generic release files and checksum-style artifacts.
- Good, because it resembles the subject input model supported by generic attestation tools.
- Bad, because package ecosystem identity becomes secondary rather than part of the profile's
  primary contract.
- Bad, because npm package URLs, tarball integrity digests, registry verification, and future Maven
  or OCI semantics still require profile-specific interpretation.
- Bad, because a lowest-common-denominator subject model can make downstream verification weaker or
  more confusing.

### Use direct `actions/attest` wrappers as the primary profile boundary

Create thin wrappers around GitHub `actions/attest` for each artifact or package type and let caller
workflows perform the build.

- Good, because it is simple and uses GitHub's maintained attestation action, Sigstore-backed
  signing, GitHub attestations API upload, and linked artifact metadata support.
- Good, because it can serve repositories that are not ready to adopt a trusted reusable builder.
- Neutral, because `actions/attest` remains useful inside the chosen architecture as a signing or
  storage adapter.
- Bad, because post-build attestation in caller-controlled workflows does not by itself provide the
  trusted reusable workflow build boundary targeted by ADR 0002.
- Bad, because it cannot reliably enforce profile-specific build, package-manager, subject, or
  publish rules.
- Bad, because it is better suited to a Build L2-oriented baseline than the primary SLSA Build
  L3-oriented profile architecture.

## More Information

This decision refines ADR 0002 by treating "profile-owned reusable workflows" as the implementation
style for the selected extensible trusted reusable workflow foundation.

Relevant reference points:

- SLSA v1.2 Build requirements define Build L3 around hosted isolated builds, unforgeable
  provenance, and complete external parameters generated or verified by the trusted build platform.
- SLSA provenance v1 separates `builder.id`, which represents the trusted build platform, from
  `buildType`, which identifies how to interpret the build definition and parameters.
- `slsa-framework/slsa-github-generator` demonstrates reusable workflow isolation, digest-verified
  job handoff, and BYOB callback patterns, but its broad framework surface is larger than this
  project's first profile needs.
- `actions/attest` provides a useful attestation and signing substrate, but direct use after
  caller-controlled builds should remain a baseline or adapter path rather than the main profile
  boundary.
