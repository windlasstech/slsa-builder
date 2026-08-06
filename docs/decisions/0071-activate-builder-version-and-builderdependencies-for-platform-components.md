---
parent: Decisions
nav_order: 71
status: accepted
date: 12026-08-03
decision-makers: Yunseo Kim
relations:
  - type: partially-superseded-by
    target: ADR-0077
    scope:
      "the signing-adapter descriptor for every producer profile identifies the governed sigstore-go
      module instead of actions/attest; field classification and closed-set policy remain"
  - type: see-also
    target: ADR-0028
  - type: see-also
    target: ADR-0029
  - type: see-also
    target: ADR-0035
  - type: see-also
    target: ADR-0055
  - type: see-also
    target: ADR-0070
---

# Activate builder.version and builderDependencies for Build Platform Components

## Context and Problem Statement

ADR 0029 delegates the content of `runDetails.builder.version`, `builderDependencies`,
`resolvedDependencies`, and invocation metadata to the architecture specifications. The common
provenance contract currently freezes `builder.version` to `null` and `builderDependencies` to an
empty array, both marked "reserved for future use", and no profile emits either field.

ADR 0070 records build-time dependency artifacts — the package-manager distribution and the runner
image — in `resolvedDependencies`. Two related pieces of build platform information remain
unrecorded. First, the versions of the platform components that shaped the build environment: the
Node.js runtime version and, when pnpm or Yarn is selected, the Corepack version that fetched and
activated the package manager. Second, the identity of the signing adapter, the SHA-pinned
`actions/attest` action that produces the signed provenance bundle. The signing adapter does not
affect the artifact bytes, but it directly affects provenance generation and its security
guarantees.

SLSA v1.2 defines `builder.version` as a "map of names of components of the build platform to their
version" and `builderDependencies` as "dependencies used by the orchestrator that are not run within
the workload and that do not affect the build, but might affect the provenance generation or
security guarantees". The v1 `buildType` contract is about to be frozen with ADR 0070's
`resolvedDependencies` expansion, so the contract must now decide whether to activate these two
`runDetails.builder` fields, with what closed shapes, and with what initial population, or to keep
them reserved.

## Decision Drivers

- Follow the SLSA v1.2 field definitions: platform component versions belong in the
  `builder.version` map; orchestrator-side artifacts that affect provenance generation or security
  guarantees, but not the build, belong in `builderDependencies`.
- Keep the common contract profile-neutral: this decision changes `runDetails.builder` for every
  current and future profile, so the shapes must not bake in npm-specific assumptions.
- Avoid duplicating values that other ADRs already record elsewhere: the npm CLI version stays in
  `externalParameters.package_manager` per ADR 0017, and the runner image identity stays in the
  `runner-image` descriptor per ADR 0070.
- Rely on the SHA-pinned `builder.id` from ADR 0028 as the transitive identity of the workflow
  definition; only record the platform component whose provenance impact is not visible from the
  workflow file alone at verification time.
- Keep both fields closed and enumerable so strict verification can reject unexpected content by
  exact shape, consistent with the rest of the provenance contract.
- Record only what is observable without changing the workflow topology or acquisition paths.

## Considered Options

- Activate both fields with closed shapes: a `builder.version` key allowlist populated with the
  Node.js and Corepack versions, and a `builderDependencies` set initially holding exactly the
  SHA-pinned signing-adapter descriptor.
- Activate only `builderDependencies` for the signing adapter and keep `builder.version` reserved as
  `null`.
- Activate only `builder.version` for platform component versions and keep `builderDependencies`
  reserved as an empty array.
- Record the signing adapter in `resolvedDependencies` alongside the ADR 0070 descriptors instead of
  activating `builderDependencies`.
- Keep both fields reserved and rely on `builder.id` and ADR 0070's descriptors alone.

## Decision Outcome

Chosen option: "Activate both fields with closed shapes", because each field has a concrete,
spec-defined population available today, the populations are orthogonal, and freezing v1 with both
fields defined avoids a post-freeze `buildType` versioning event for information the platform
already knows.

The common provenance contract activates `runDetails.builder` as follows:

- `builder.version`: a closed key allowlist. The initial keys are `nodejs` (required: the Node.js
  runtime version observed at build time) and `corepack` (required when the selected package manager
  is obtained through Corepack; absent otherwise). Keys are lowercase component names and values are
  exact version strings. Future profiles may add keys only through their own ADR and specification
  admission process. The npm CLI version is not a `builder.version` key because ADR 0017 records it
  in `externalParameters.package_manager`; the runner image version is not a key because ADR 0070
  records it in the `runner-image` descriptor annotations.
- `builderDependencies`: a closed descriptor set. The initial population is exactly one entry, the
  SHA-pinned signing adapter: a `ResourceDescriptor` whose `uri` is
  `git+https://github.com/actions/attest@<full commit SHA>`, whose `digest.gitCommit` is the same
  pinned commit, and whose annotations identify its role as the signing adapter selected by ADR 0035
  and used in custom Statement mode per ADR 0055. The entry binds the exact pinned action revision
  that produced the signature, which the SHA-pinned `builder.id` makes recoverable but not directly
  visible to a verifier inspecting only the provenance payload.

Build-job actions such as `actions/checkout` are not recorded in either field: they run inside the
SHA-pinned reusable workflow identified by `builder.id`, and enumerating them is deferred unless a
later ADR defines a workflow-dependency recording policy.

Strict producer-side verification accepts exactly these closed shapes and fails before signing on
unknown `builder.version` keys, missing required keys, unexpected `builderDependencies` entries, or
a signing-adapter descriptor that does not match the pinned revision used by the run. Verifier
policy and fixtures are updated to the same closed shapes, and the v1 `buildType` documentation
describes both fields.

### Consequences

- Good, because platform component versions that materially shape the build environment become
  verifier-visible without any acquisition or topology change.
- Good, because the signing adapter — the component that affects provenance generation and its
  security guarantees — is recorded in the field SLSA defines for exactly that role.
- Good, because the closed allowlists keep strict verification decidable and leave profile-neutral
  extension paths through the ADR/spec admission process.
- Good, because activating both fields now, together with ADR 0070, completes the v1
  `runDetails.builder` shape before the contract freeze.
- Neutral, because `builderDependencies` sees little ecosystem use, so some verifiers may ignore the
  field; none are known to reject it.
- Bad, because the `nodejs`/`corepack` version strings are observed values, not digest-bound
  artifacts, so they identify rather than pin the platform components.

### Confirmation

This decision is confirmed when:

- the common SLSA provenance specification replaces the "reserved for future use" rules for
  `builder.version` and `builderDependencies` with the closed shapes above, including valid and
  invalid examples;
- the `builder.version` rules pin the initial key allowlist (`nodejs`, conditional `corepack`),
  lowercase key form, exact version string values, and the non-duplication rules for the npm CLI
  version and runner image version;
- the `builderDependencies` rules pin the signing-adapter descriptor's URI form, `gitCommit` digest,
  and signing-adapter role annotation;
- strict verification rejects unknown keys, missing required keys, unexpected entries, and
  signing-adapter descriptor mismatches before signing, with verification policy and fixtures
  covering accepted and rejected cases;
- the v1 `buildType` documentation describes both activated fields;
- implementation review confirms no build-job action descriptors and no duplicated npm CLI or runner
  image values appear.

## Pros and Cons of the Options

### Activate both fields with closed shapes

Populate `builder.version` with the Node.js/Corepack allowlist and `builderDependencies` with the
signing-adapter descriptor.

- Good, because both SLSA-defined fields gain honest, closed populations before the v1 freeze.
- Good, because the two populations are orthogonal and independently extensible.
- Good, because ecosystem precedent exists for both (component version maps are the documented
  `builder.version` form; `builderDependencies` is used in the wild for builder-side artifacts).
- Bad, because two currently frozen fields change at once in the common contract, affecting every
  profile's provenance shape.

### Activate only builderDependencies

Record the signing adapter but keep `builder.version` as `null`.

- Good, because the highest-value entry — the signature-producing component — is recorded.
- Bad, because Node.js and Corepack versions remain unrecorded even though they are trivially
  observable and verifier-relevant.
- Bad, because a later activation of `builder.version` would become a separate contract change after
  the freeze.

### Activate only builder.version

Record platform component versions but keep `builderDependencies` empty.

- Good, because component versions are recorded with minimal conceptual surface.
- Bad, because the signing adapter's pinned revision stays invisible to payload-only verifiers even
  though it is the component with direct provenance security impact.

### Record the signing adapter in resolvedDependencies

Put the signing-adapter descriptor next to ADR 0070's build-time descriptors.

- Good, because it avoids activating a little-used field.
- Bad, because it misclassifies the artifact: the signing adapter is not run within the workload and
  does not affect the build, so SLSA's boundary rule places it in `builderDependencies`.
- Bad, because it invites future misclassification of other orchestrator-side components.

### Keep both fields reserved

Rely on `builder.id` and ADR 0070's descriptors.

- Good, because the common contract stays minimal.
- Bad, because verifiers cannot see platform component versions or the exact signing-adapter
  revision from the payload, weakening verification at the moment v1 is frozen.
- Bad, because activating the fields after a public v1 becomes a `buildType` versioning event.

## More Information

This decision follows ADR 0029's delegation of `runDetails.builder` content and applies the SLSA
v1.2 field definitions: `builder.version` as a component-version map and `builderDependencies` as
orchestrator-side dependencies that affect provenance generation or security guarantees without
affecting the build (<https://slsa.dev/spec/v1.2/build-provenance#builder>). The signing adapter
itself does not record its own identity in produced attestations, so the producer must record it
(<https://github.com/actions/attest>). ADR 0070 covers the sibling `resolvedDependencies` expansion;
ADR 0028's SHA-pinned `builder.id` remains the transitive identity of the workflow definition and is
unchanged.
