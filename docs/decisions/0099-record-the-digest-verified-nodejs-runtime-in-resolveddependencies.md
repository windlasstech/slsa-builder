---
parent: Decisions
nav_order: 99
status: accepted
date: 12026-10-15
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0070
    scope:
      "the clause recording no Node.js distribution descriptor ('A Node.js distribution descriptor
      is likewise not recorded') together with its stated rationale that the runner-provided Node.js
      runtime had no published distribution digest: under ADR-0098's digest-verified provisioning an
      exact release URL and a committed SHA-256 exist, so the runtime is recorded. The separate
      clause giving the npm CLI no descriptor of its own remains in force and is now transitively
      covered by the runtime descriptor, as do the closed-descriptor-set discipline, the lockfile,
      package-manager-distribution, and runner-image descriptors, and the source-native
      digest-authority discipline"
  - type: partially-supersedes
    target: ADR-0085
    scope:
      "only the Scope and non-goals clause stating that the `resolvedDependencies` v1 schema is
      untouched; the separate conclusion that the npm CLI receives no package-manager-distribution
      descriptor remains in force, with the bundled npm CLI covered transitively by the node-runtime
      descriptor"
  - type: see-also
    target: ADR-0088
  - type: see-also
    target: ADR-0098
---

# Record the Digest-Verified Node.js Runtime in resolvedDependencies

## Context and Problem Statement

ADR 0098 provisions the build-stage Node.js runtime from a digest-verified official release tarball:
the trusted core downloads the exact pinned URL, requires the compressed bytes to match the SHA-256
digest committed in `internal/toolchainpins/node-runtime.json`, and only then extracts the archive
and executes `node`, `npm`, `npx`, and Corepack from it. The runtime is therefore a fetched artifact
that is authenticated against a repository-committed trust anchor and that executes the package
manager, the dependency install, the build scripts, and the pack.

SLSA v1.2 defines `resolvedDependencies` as the unordered collection of artifacts needed at build
time, recommends recording artifacts whose compromise could impact the build, and treats
completeness as best effort through Build L3. The npm profile currently records the selected
lockfile, the conditional pnpm or Yarn package-manager distribution, and the runner image (ADR
0070). The Node.js runtime is absent. ADR 0070 recorded no Node.js distribution descriptor because
the runner-provided runtime had no published distribution digest to record, and the ADR
0098-normative provisioning specification states that the provisioning contract introduces no
`resolvedDependencies` schema change.

ADR 0098 retired ADR 0070's rationale: the runtime is no longer runner-provided, and an exact URL
plus a committed SHA-256 now exist for every supported platform. A 12026-10-15 conformance review of
the npm profile against SLSA v1.2 and in-toto Statement v1 classified the omission as a dependency
completeness gap rather than a format violation, precisely because the evidence is already produced
and verified at build time and the omission is a policy choice.

Should the npm profile's signed provenance record the digest-verified Node.js runtime tarball as a
named `resolvedDependencies` descriptor?

## Decision Drivers

- The runtime executes the package manager and every lifecycle and build script; its compromise
  directly compromises the build, which is exactly the artifact class SLSA v1.2 recommends recording
  in `resolvedDependencies`.
- The URL and SHA-256 are already pinned, fetched, and verified for the build; recording them adds
  no acquisition path, trust anchor, or run-time work.
- Symmetry of the record: pnpm and Yarn executions are covered by the package-manager-distribution
  descriptor, and the bundled npm bytes are authenticated with the runtime artifact (ADR 0098), yet
  neither the runtime nor the npm it carries appears in the signed record today.
- `resolvedDependencies` is a closed, enumerable schema (ADR 0070). Adding a descriptor changes the
  cardinality that producers, verifiers, fixtures, and consumer policies enumerate, so the addition
  must be a deliberate decision rather than a specification edit.
- Redundancy counterweight: `builder.id`, `builder.version.nodejs`, and the SHA-pinned workflow
  already let a consumer resolve the pin record indirectly. The descriptor's marginal value is a
  signed, in-band binding, not new evidence.
- The profile is pre-release, so a cardinality change is cheaper now than after a stable release.

## Considered Options

- Record the runtime tarball as one `resolvedDependencies` descriptor for every manager selection.
- Keep the status quo: no runtime descriptor; runtime identity stays implicit through
  `builder.version` and the committed pin record.
- Record the runtime in `builderDependencies` instead of `resolvedDependencies`.
- Fold the runtime digest into the existing runner-image descriptor's annotations.

## Decision Outcome

Chosen option: "Record the runtime tarball as one `resolvedDependencies` descriptor for every
manager selection", because the runtime is a fetched, authenticated, executed artifact whose
evidence already exists at signing time, and because recording it closes the asymmetry in which the
profile executes code from an artifact its provenance does not name. The descriptor's concrete shape
(its `name`, annotation keys, and authority wording) is deferred to the specification phase;
candidate shapes investigated during this decision are recorded under More Information and are not
part of the decision.

### Nature of this decision

This decision changes the closed cardinality of the npm profile's signed `resolvedDependencies` set
and nothing else. It does not change acquisition, verification, or provisioning behavior: the
recorded URL and digest are the same values the ADR 0098 provisioning step already pins and
verifies. The specification phase owns the exact descriptor contract, including its interaction with
the existing per-descriptor diagnostics, and the specification and implementation may not widen this
decision into a general open-ended dependency recording.

### Consequences

- Good, because the signed provenance names every authenticated artifact the build executes,
  including the runtime and, transitively, the bundled npm bytes authenticated with it.
- Good, because consumers can bind the executed runtime to a committed digest from the provenance
  alone, without resolving the builder's pin record out of band.
- Bad, because every producer rule, verifier policy, fixture, and golden that enumerates the closed
  descriptor set must grow one entry, and consumer policies written against the current enumeration
  must be updated.
- Bad, because future platform expansion of the pin record must keep the descriptor contract in
  step, adding a standing coupling between the pin record and the profile schema.
- Neutral, because the npm CLI still receives no separate descriptor: it is covered through the
  runtime descriptor that binds the bytes it ships in.
- Neutral, because the runner-image descriptor, `builder.version`, and `builderDependencies` are
  unchanged.

### Confirmation

- The provenance-and-publish specification's `resolvedDependencies` cardinality section and the
  build-and-pack specification's sentence that provisioning introduces no `resolvedDependencies`
  schema change are revised in the same change.
- The producer emits the descriptor for npm, pnpm, and Yarn selections, and producer-side and
  consumer-side verification agree on its cardinality and shape, proven by accepted and rejected
  fixtures registered in the governed corpus.
- Byte-exact provenance goldens are regenerated through the documented update gates, and the
  verification policy fixture taxonomy gains the descriptor's accepted and rejected cases.
- The next live dogfood publish emits provenance containing the descriptor, and the read-back
  verification procedure passes on it.

## Pros and Cons of the Options

### Record the runtime tarball descriptor for every manager selection

- Good, because the signed record covers an artifact that both participates in the build and can
  execute arbitrary build logic. The runtime runs the package manager, dependency lifecycle scripts,
  the build script, and the pack operation. Recording only the lockfile and package-manager
  distribution leaves a material execution input outside the provenance even though its bytes are
  already authenticated.
- Good, because it follows SLSA v1.2's guidance at the appropriate strength. The standard recommends
  recording known fetched artifacts whose compromise could impact the build; it does not require
  complete dependency enumeration at Build L3. This option implements that recommendation for a
  high-value artifact without claiming that the resulting dependency list is complete.
- Good, because a consumer can verify the exact runtime artifact from the provenance itself. The
  descriptor supplies the release URL and the digest of the bytes that were fetched and verified,
  rather than merely a version string. A consumer that obtains the tarball can therefore compare its
  digest without reading the builder repository or interpreting the workflow that supplied it.
- Good, because the provenance becomes more useful for incident response and recursive supply-chain
  analysis. A runtime vulnerability, a malicious or withdrawn release artifact, or a suspect digest
  can be correlated with published packages from the signed records alone. The record also binds the
  bundled npm bytes transitively, since the runtime tarball is the artifact that supplied them.
- Good, because it uses existing evidence rather than creating a new trust or acquisition path. The
  provisioning process already pins the URL, downloads the tarball, and verifies its SHA-256 before
  extraction. Recording those same observations does not add network access, weaken the trust
  boundary, or introduce a second verification mechanism.
- Good, because it makes the profile internally consistent. pnpm and Yarn already have their
  executed distributions recorded, and the lockfile records the dependency-resolution input. The
  runtime descriptor completes the account of the toolchain that interpreted those inputs instead of
  treating the executable environment as an unexplained exception.
- Good, because making the change before a stable release limits migration cost. The descriptor set,
  fixtures, and consumer expectations can be revised together before consumers need a long-term
  compatibility path for two profile schema generations.
- Bad, because the addition changes a closed, verifier-visible cardinality. Producers, the
  producer-side gate, consumer verification, fixture expectations, byte-exact goldens, and consumer
  policies must all be updated atomically. Existing strict consumers that accept only the current
  enumerated entries will reject otherwise valid provenance containing the new descriptor.
- Bad, because it duplicates information that is already reachable through the SHA-pinned builder
  and its committed pin record. The specification must make the pin record the single source from
  which the descriptor is derived; otherwise an independently maintained descriptor could drift and
  turn a recording enhancement into contradictory evidence.
- Bad, because it creates recurring maintenance work beyond the initial migration. Toolchain bumps
  change the recorded URL and digest, and any future expansion beyond `linux-x64` must preserve the
  descriptor's cardinality and platform semantics. The profile schema and the provisioning pin
  record consequently remain coupled.
- Bad, because the benefit is smaller for consumers that already fetch and trust the exact builder
  commit. Such consumers can resolve the pin record out of band, so the descriptor mainly improves
  self-containment and generic dependency analysis rather than adding an independent security
  guarantee.
- Bad, because a custom descriptor name and annotations are meaningful only under the profile's
  `buildType` documentation. Their presence must not lead consumers to assume a standardized SLSA
  runtime descriptor or a complete inventory of every transitive build dependency.

### Keep the status quo

- Good, because it preserves compatibility without any migration. The closed descriptor set,
  existing consumer policies, fixture corpus, and goldens remain valid, and strict verifiers do not
  encounter a new entry that they must learn to evaluate.
- Good, because the provisioning security improvement from ADR 0098 does not depend on recording the
  runtime. The tarball is still digest-verified before use even if no descriptor is emitted, so this
  option does not reopen the unauthenticated acquisition problem that ADR 0098 solved.
- Good, because it avoids duplicating the pin record in the signed output. The exact workflow commit
  selected by `builder.id` identifies the builder-owned pin record, and `builder.version.nodejs`
  records the observed version. A consumer that trusts and reads that exact builder revision can
  already determine the expected tarball URL and SHA-256 without maintaining a second representation
  in provenance.
- Good, because SLSA treats `resolvedDependencies` completeness as best effort through Build L3.
  Omitting this descriptor is therefore not, by itself, a schema violation or proof that the build
  fails a required level. This option is a legitimate choice to accept a narrower record when the
  migration cost is judged to outweigh its analytical value.
- Good, because it keeps the profile's verification contract smaller. Consumers need no custom
  runtime annotation or descriptor semantics, and future platform expansion does not require a
  matching change to the provenance schema.
- Bad, because the provenance is not self-contained with respect to the runtime bytes. A consumer
  must retrieve and understand the correct builder revision and its pin record to reconstruct the
  URL and digest. Verification from a registry-published bundle alone, or in an otherwise offline
  setting without the builder source, cannot recover that binding from the record itself.
- Bad, because it leaves an executed and security-relevant artifact outside recursive dependency
  analysis. Tooling that consumes `resolvedDependencies` sees package-manager distributions and the
  lockfile but not the runtime that executed them, making vulnerability impact analysis and incident
  correlation less direct.
- Bad, because it preserves an asymmetry in the evidence model. pnpm and Yarn distributions are
  named while the runtime and its bundled npm are only implicit, even though the runtime now has
  stronger digest evidence than when ADR 0070 originally justified the omission.
- Bad, because it deliberately forgoes available evidence rather than lacking evidence. The
  completeness gap remains a policy choice that must be revisited if consumers later require a
  complete toolchain account or if SLSA guidance becomes stricter.
- Bad, because indirect resolution depends on the continued availability and interpretation of the
  builder repository and workflow identity. It is weaker as a portable, long-lived audit record than
  a digest carried by the signed provenance itself.

### Record the runtime in builderDependencies

- Good, because it would acknowledge the runtime as a builder-controlled platform component and
  would place it alongside other builder-owned execution dependencies. The field is already intended
  for dependencies used by the orchestrator, so it offers an existing conceptual location for
  platform tooling rather than requiring a new top-level construct.
- Good, because it could centralize inventory of the components that the builder provisions. A
  consumer interested specifically in the trusted platform could inspect builder-side dependencies
  without mixing them with source and package-manager inputs to the workload.
- Good, because it would avoid adding another entry to `resolvedDependencies` and could appear to
  preserve that field's current cardinality. The URL and SHA-256 are already available, so the
  evidence needed to populate either field is the same.
- Bad, because it contradicts the SLSA field semantics for the current build model.
  `builderDependencies` is for orchestrator dependencies that are not run within the workload and do
  not affect the build. The Node.js runtime executes the package manager and build scripts inside
  the workload and directly affects their results, so placing it there would misclassify a build
  input as an orchestrator-only detail.
- Bad, because it does not actually avoid schema churn. `builderDependencies` is also closed and
  exactly verified under the profile contract. Adding a runtime entry would change its cardinality,
  fixtures, and consumer policies while additionally teaching consumers the wrong meaning of the
  field.
- Bad, because consumers could reasonably skip an orchestrator-only dependency when reconstructing
  the build's input supply chain. The option would therefore hide the runtime from the very analysis
  that motivates recording it, while creating a superficial impression that it had been covered.
- Bad, because it blurs the boundary established by ADRs 0070 and 0071 between workload inputs and
  builder platform components. It would also duplicate the runtime's presence in
  `builder.version.nodejs` without expressing the relationship between the runtime, its bundled npm,
  and the workload's dependency installation.
- Bad, because it optimizes field placement for the producer's provisioning mechanism rather than
  for the consumer's question of which artifacts affected the build. Rejected in favor of the field
  whose standard semantics match the runtime's actual role.

### Fold the runtime digest into the runner-image descriptor's annotations

- Good, because it would keep the number of `resolvedDependencies` entries unchanged. A producer
  could attach runtime information to an existing environment descriptor instead of defining and
  documenting a new named descriptor.
- Good, because annotations are the intended extension point for profile-specific metadata. If the
  runtime were merely an attribute of the runner image, recording its version or acquisition details
  there would be a compact way to enrich the existing environment record.
- Good, because the initial producer-side edit could be small and the existing `uri` and `digest`
  fields of the runner-image descriptor could remain untouched. It might therefore look attractive
  as a narrowly scoped diagnostic addition rather than a new dependency contract.
- Bad, because the premise no longer holds after ADR 0098. The runner image and the Node.js runtime
  tarball are different artifacts with different URIs, bytes, release processes, and trust anchors.
  Attaching the runtime's digest to the runner-image descriptor would imply a containment or
  identity relationship that does not exist.
- Bad, because a digest nested in annotations is not a standard ResourceDescriptor `digest` for the
  runtime. Generic consumers cannot use normal DigestSet algorithm handling to match the runtime
  artifact, and the runner-image descriptor's own URI does not resolve to the runtime tarball.
- Bad, because it makes the evidence harder for consumers to discover and evaluate. Consumers select
  the closed dependency set by descriptor `name`; burying a second artifact inside another
  descriptor's annotations requires profile-specific parsing and prevents generic recursive
  dependency tooling from treating the runtime as a first-class input.
- Bad, because it avoids only the appearance of schema change, not the change itself. The
  runner-image annotation set is also closed and verified, so producers, verifiers, fixtures, and
  policies would still need coordinated updates while receiving less clear semantics in return.
- Bad, because it creates ambiguous lifecycle and comparison rules. A runner-image update and a
  runtime-pin update can occur independently, and consumers would need custom documentation to know
  which annotations belong to which artifact and which values are authoritative.
- Bad, because it conflicts with the ResourceDescriptor guidance that a digest should match the
  resource identified by the descriptor. Rejected in favor of a separate descriptor that identifies
  the runtime artifact directly and leaves the runner-image record unchanged.

## More Information

- Candidate descriptor shape (investigated but undecided; the specification phase fixes it):
  `name: "node-runtime"`; `uri` set to the exact pinned release tarball URL; `digest` carrying the
  committed SHA-256 as lowercase hexadecimal, the same value the provisioning step verifies; and
  annotation candidates such as `platform` (initially `linux-x64`), a role marker such as
  `build-runtime`, and the observed Node.js and bundled npm versions. A digest-authority annotation
  naming the builder-pinned record as the authority would follow ADR 0070's honest-authority
  discipline.
- Platform cardinality: the ADR 0098 pin record supports exactly `linux-x64`, so the descriptor set
  holds one entry per build; wider platform coverage keeps one descriptor whose platform annotation
  disambiguates.
- SLSA v1.2 reference: <https://slsa.dev/spec/v1.2/build-provenance>, the `BuildDefinition` and
  `resolvedDependencies` guidance.
- Related decisions: ADR 0070 (closed descriptor set and source-native authorities), ADR 0088
  (package-manager distribution re-specification), ADR 0098 (runtime provisioning and the pin
  record), ADR 0085 (runtime pair pin), and ADR 0071 (the `builder.version` boundary).
- Origin: the 12026-10-15 SLSA v1.2 and in-toto v1 conformance review of the npm profile, which
  classified the missing runtime descriptor as a SHOULD-level completeness gap and traced the
  exclusion to ADR 0070's retired rationale.
