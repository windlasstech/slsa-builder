---
parent: Decisions
nav_order: 37
status: accepted
date: 12026-07-06
decision-makers: Yunseo Kim
---

# Define Initial Verification Deliverables

## Context and Problem Statement

ADR 0028 selected a SHA-pinned reusable workflow `builder.id`, ADR 0029 selected Windlass-generated
SLSA provenance as the canonical npm package provenance, ADR 0031 selected a Sigstore-signed release
manifest, ADR 0035 selected `actions/attest` as the initial signing adapter, and ADR 0036 selected a
three-job graph with publish-time verification before npm registry mutation.

Those decisions require verifier behavior, but they do not yet define what verifier-facing artifacts
Windlass must ship in the initial production JS/TS npm package SLSA3 profile. The open question is
whether the first release must include a standalone consumer verifier CLI, only document verifier
policy, rely on ecosystem tools, or provide a smaller set of policy and conformance deliverables
that can support a later standalone verifier without committing to one too early.

What verification deliverables should the initial production profile ship?

## Decision Drivers

- Preserve SLSA v1.2's expectation that provenance is useful only when checked against trusted roots
  and package-specific expectations.
- Ensure npm registry mutation is blocked unless the package tarball and signed Windlass provenance
  bundle pass producer-side verification.
- Avoid making a premature public CLI/API commitment before the Go trusted core owns Sigstore bundle
  verification and Windlass policy evaluation.
- Keep consumer guidance explicit enough to verify Windlass-produced artifacts using existing tools
  where possible.
- Make full-SHA builder identity, release manifest, package identity, source identity, and runtime
  policy expectations testable.
- Avoid delegating Windlass-specific SLSA policy entirely to generic ecosystem tools that do not
  know the profile's required `builder.id`, `buildType`, `externalParameters`, and package-directory
  semantics.
- Preserve a clear future path for a standalone verifier CLI or upstream verifier integration.

## Considered Options

- Ship a standalone verifier consumer CLI in the initial release.
- Ship only documented verifier policy.
- Implement only the publish job's producer-side verification gate.
- Ship producer-side publish verification, documented verifier policy, fixtures, and reference
  commands, but no standalone consumer verifier CLI.
- Ship a thin wrapper around `gh attestation verify`.
- Make `slsa-verifier` compatibility the official initial verifier deliverable.

## Decision Outcome

Chosen option: "Ship producer-side publish verification, documented verifier policy, fixtures, and
reference commands, but no standalone consumer verifier CLI", because this gives the first
production profile concrete verification behavior and conformance evidence without locking Windlass
into a consumer verifier interface before the trusted Go implementation and Sigstore policy surface
are ready.

The initial production JS/TS npm package SLSA3 profile should include these verification
deliverables:

- a producer-side verification gate in the `publish` job selected by ADR 0036;
- an architecture specification section or document that defines Windlass verifier policy;
- conformance fixtures for accepted and rejected package/provenance/bundle combinations;
- reference commands and guidance for existing tools such as `gh attestation verify`,
  `npm audit signatures`, and, where compatible, `slsa-verifier`;
- explicit non-goal language that the initial profile does not ship a standalone consumer verifier
  CLI.

The `publish` job remains in scope as production logic, not merely documentation. It should verify
the package tarball digest, signed bundle digest, Sigstore signer identity, SLSA statement subject,
`predicateType`, `builder.id`, `buildType`, `externalParameters`, source identity, package identity,
package version, package directory, runtime policy, and release manifest expectations before running
`npm publish --provenance-file` or an equivalent external provenance-file submission. It should fail
closed before registry mutation if any required check fails.

The documented verifier policy should explain which expectations are enforced by Windlass before
publish, which expectations consumers should independently verify after publication, and which
checks generic ecosystem tools do and do not cover. It should describe expected roots of trust and
identity bindings in terms of the Sigstore root, GitHub Actions reusable workflow signer identity,
SHA-pinned Windlass `builder.id`, SLSA `predicateType`, SLSA `buildType`, external parameters,
package identity, source identity, and release manifest linkage.

The conformance fixture set should include at least one valid package tarball and signed provenance
bundle shape, plus invalid cases for digest mismatch, signature or signer mismatch, wrong
`predicateType`, wrong `builder.id`, wrong `buildType`, unrecognized or mismatched
`externalParameters`, source identity mismatch, package identity or version mismatch,
package-directory mismatch, runtime-policy mismatch, and release manifest mismatch. Fixtures may be
synthetic until the implementation can generate real signed bundles, but their expected pass/fail
semantics must match the architecture specification.

The reference guidance may show existing tools as partial verifier building blocks.
`gh attestation verify` can verify GitHub/Sigstore attestation signature material, predicate type,
signer workflow, source ref, and source digest, and can emit JSON for additional policy checks.
`npm audit signatures` can report registry signatures and npm-recognized attestations for installed
packages, but it is not a complete Windlass policy verifier. `slsa-verifier` may be documented as an
ecosystem verifier or future compatibility target only where it can express the profile's required
expectations.

Windlass should add a standalone verifier CLI only after a later ADR decides the command scope,
input model, offline/online Sigstore behavior, release manifest policy, error taxonomy, and
compatibility responsibilities. Upstream `slsa-verifier` support may also be pursued later, but the
initial profile should not make its release criteria depend on external verifier support for
Windlass-specific policy.

### Consequences

- Good, because initial production publishing cannot proceed without producer-side verification.
- Good, because consumers receive explicit policy guidance instead of implicit trust in unsigned
  workflow outputs.
- Good, because fixtures make verifier semantics testable before a public verifier CLI exists.
- Good, because the future standalone verifier can reuse the same policy and fixture corpus.
- Good, because `gh attestation verify`, `npm audit signatures`, and `slsa-verifier` are positioned
  accurately as useful tools with specific limits.
- Good, because Windlass avoids prematurely owning a consumer CLI surface for Sigstore verification,
  npm registry lookup, release manifest policy, and error compatibility.
- Neutral, because the publish job performs verifier-like checks inside the producer workflow while
  consumers still need independent verification for defense in depth.
- Bad, because users do not get a single verify command in the initial release.
- Bad, because reference commands cannot fully enforce Windlass-specific policy without additional
  manual or scripted checks.
- Bad, because fixture design must be kept aligned with the future implementation to avoid stale
  policy examples.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification,
workflow implementation, tests, and verifier guidance define:

- the `publish` job's producer-side verification gate before npm registry mutation;
- exact pass/fail checks for tarball digest, signed bundle digest, Sigstore signature, signer
  identity, `subject`, `predicateType`, `builder.id`, `buildType`, `externalParameters`, source
  identity, package identity, package version, package directory, runtime policy, and release
  manifest linkage;
- documented roots of trust and identity bindings for the initial Windlass production builder;
- documented consumer-side verification expectations and the limits of npm registry verification;
- reference commands for `gh attestation verify`, `npm audit signatures`, and any supported
  `slsa-verifier` compatibility path;
- conformance fixtures that include both accepted and rejected verification cases;
- hard-failure behavior for every invalid fixture category;
- an explicit statement that a standalone consumer verifier CLI is not part of the initial profile;
- a future-decision note for standalone verifier CLI or upstream verifier integration.

Implementation review should verify that no documentation or workflow output implies that generic
tool success alone is equivalent to complete Windlass policy verification unless all
Windlass-specific expectations have also been checked.

## Pros and Cons of the Options

### Ship a standalone verifier consumer CLI

Windlass would include a first-party command such as `windlass verify npm-package` that consumes a
package tarball, signed provenance bundle, release manifest, and expected policy inputs.

- Good, because it gives users the clearest consumer-side verification UX.
- Good, because Windlass-specific policy can be enforced by one tool.
- Good, because full-SHA `builder.id`, release manifest, and profile-specific expectations can be
  checked without generic-tool gaps.
- Bad, because it expands the initial trusted implementation surface significantly.
- Bad, because Windlass would immediately own Sigstore root handling, online/offline bundle
  verification, registry lookup, error taxonomy, and CLI compatibility.
- Bad, because a premature verifier interface could become difficult to change after consumers
  automate it.

### Ship only documented verifier policy

Windlass would define what consumers should verify, but would not implement publish-time
verification or provide fixtures and reference commands.

- Good, because it is the smallest documentation-only scope.
- Good, because SLSA expectation fields can be described before implementation.
- Bad, because it would not satisfy ADR 0036's publish-time verification gate.
- Bad, because users would have no concrete examples or conformance evidence.
- Bad, because policy could drift from implementation without fixtures.

### Implement only the publish job's producer-side verification gate

Windlass would verify package and provenance artifacts before publish but would not document
consumer policy or provide reusable fixtures.

- Good, because it protects npm registry mutation from bad package/provenance combinations.
- Good, because the checks live in the workflow path that must be correct for release.
- Bad, because it does not explain what downstream consumers should verify.
- Bad, because internal verification behavior would be harder to audit without documented policy and
  fixtures.
- Bad, because producer-side verification does not cover registry compromise or download-time
  threats.

### Ship publish verification, documented policy, fixtures, and reference commands

Windlass includes the producer-side verification gate, policy documentation, conformance fixtures,
and examples using existing tools, but does not ship a standalone consumer verifier CLI.

- Good, because it balances initial security value with implementation scope.
- Good, because it makes verifier semantics testable before public CLI stabilization.
- Good, because it supports defense-in-depth guidance without overclaiming generic verifier
  coverage.
- Good, because it creates a stable base for a later verify command.
- Bad, because users must combine reference tools and policy guidance rather than run one Windlass
  verifier command.
- Bad, because Windlass must clearly communicate which tool results are partial.

### Ship a thin wrapper around `gh attestation verify`

Windlass would provide a small command or action that shells out to, or directly mirrors,
`gh attestation verify`.

- Good, because GitHub CLI already verifies GitHub/Sigstore attestation identity and predicate type.
- Good, because it aligns with the initial `actions/attest` signing adapter.
- Bad, because it would still need extra Windlass policy checks for `builder.id`, `buildType`,
  `externalParameters`, release manifest, and npm package semantics.
- Bad, because a wrapper could look complete while only enforcing part of the policy.
- Bad, because it would inherit GitHub CLI versioning and output-shape compatibility concerns.

### Make `slsa-verifier` compatibility the official initial verifier deliverable

Windlass would treat existing `slsa-verifier` support as the initial verification surface, or block
release on upstream compatibility.

- Good, because `slsa-verifier` is the main SLSA ecosystem verifier precedent.
- Good, because it already supports source, builder, package, and provenance checks for several
  builders, including experimental npm package verification.
- Bad, because npm package support is still experimental.
- Bad, because Windlass-specific profile policy may not be expressible without upstream changes.
- Bad, because initial Windlass release criteria would depend on external support for the profile's
  exact builder identity and policy semantics.

## More Information

This decision follows ADR 0028, ADR 0029, ADR 0031, ADR 0035, and ADR 0036. It decides initial
verification deliverables only. It does not change the canonical provenance format, signing adapter,
job graph, release manifest decision, or future option to provide a standalone verifier.

Reference points considered:

- SLSA v1.2 Verifying Artifacts recommends checking the provenance envelope signature, artifact
  subject digest, `predicateType`, trusted builder identity, `buildType`, and `externalParameters`;
  unrecognized external parameters should generally fail verification.
- SLSA v1.2 describes package ecosystem, consumer, and monitor verification as valid verification
  locations, with package upload-time verification useful but not a replacement for consumer-side
  verification against registry or download-time compromise.
- SLSA v1.2 Build Requirements require authentic provenance at Build L2 and unforgeable provenance
  at Build L3, including protection of provenance authentication material from user-defined build
  steps.
- The SLSA GitHub Generator technical design treats the provenance verifier as part of the trusted
  computing base and describes verification of Fulcio/Sigstore identity, subject digest, builder
  identity, source identity, and policy fields.
- The SLSA GitHub Generator Node.js builder documents `npm audit signatures` and `slsa-verifier` as
  verification paths while also using secure package/provenance download and hash checks during
  publish.
- `slsa-verifier` provides established consumer-side SLSA verification UX and policy flags, but npm
  package verification is documented as experimental and cannot be assumed to cover all
  Windlass-specific policy without compatibility work.
- GitHub `gh attestation verify` can verify artifact attestations, predicate type, signer workflow,
  source ref, and source digest, and can emit JSON for further policy checks; GitHub warns that
  predicate contents need trusted-builder context and additional policy care.
- GitHub `actions/attest` emits Sigstore bundle JSON and is verifiable with GitHub attestation
  tooling, but generic attestation success does not by itself prove Windlass package, release
  manifest, or profile policy expectations.
- npm provenance documentation exposes `npm audit signatures` for installed package registry
  signatures and attestations, but npm provenance does not guarantee absence of malicious code and
  is not a complete Windlass policy verifier.
