---
parent: Decisions
nav_order: 70
status: accepted
date: 12026-08-03
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0017
  - type: see-also
    target: ADR-0028
  - type: see-also
    target: ADR-0029
  - type: see-also
    target: ADR-0071
---

# Record Package Manager Distributions and Runner Image in resolvedDependencies

## Context and Problem Statement

ADR 0017 requires the profile to record the selected package manager, its exact version, and the
selection source in verifier-relevant provenance parameters, and ADR 0029 delegates the complete
`resolvedDependencies`, `builder.version`, and `builderDependencies` content to the profile
architecture specification. The current specification emits exactly one `resolvedDependencies`
entry, the selected lockfile descriptor, and freezes `builder.version` and `builderDependencies` as
reserved.

Two build-time artifact classes remain unrecorded. First, when pnpm or Yarn is selected, Corepack
fetches a package-manager distribution during build initialization and that distribution executes
install, build, and pack. Second, the Node.js runtime executes the same build scripts. SLSA v1.2
states that "all artifacts fetched during initialization or execution of the build process are
considered dependencies" and defines `resolvedDependencies` as the "unordered collection of
artifacts needed at build time", so both classes are build-time dependencies that the current schema
does not capture.

Recording them is constrained by unequal digest authorities. pnpm distributions come from the npm
registry, whose metadata carries `dist.integrity` that Corepack verifies. Yarn Berry's default
distribution path (`repo.yarnpkg.com`) publishes no registry-style integrity metadata; Corepack only
computes a hash while downloading. The runner-provided Node.js runtime has no published distribution
digest at all; the honest observable identity is the runner image version and its software report,
exposed at runtime through `ImageOS` and `ImageVersion`.

The v1 `buildType` contract is about to be frozen, and the closed-schema strict verification rejects
unexpected `resolvedDependencies` entries. The profile therefore needs to decide, before that
freeze, which of these artifacts to record in `resolvedDependencies`, with what descriptor shapes
and digest authorities, so the v1 schema can enumerate them.

## Decision Drivers

- Follow the SLSA v1.2 field boundary: artifacts fetched during build initialization or execution
  that run in the workload and affect the build belong in `resolvedDependencies`.
- Record the actually used artifact honestly; use an external integrity authority where one exists
  instead of inventing a uniform one.
- Do not change artifact acquisition paths for v1: Corepack default distribution paths and the
  runner-provided Node.js runtime selected by ADR 0027 stay as they are.
- Keep `resolvedDependencies` a closed, enumerable schema so strict verification can reject
  unexpected entries by exact shape.
- Preserve ADR 0017's exact-version enforcement and its existing `externalParameters` recording;
  this decision adds artifact descriptors, not parameter duplicates.
- Avoid breaking public verifiers; additions must stay within SLSA's best-effort
  `resolvedDependencies` semantics.
- Keep the decision scoped to the npm profile; common-contract `runDetails.builder` changes belong
  to ADR 0071.

## Considered Options

- Record both artifact classes with source-native digest authorities: npm registry `dist.integrity`
  for pnpm, build-time download hash for Yarn, and a runner-image identity descriptor without a
  distribution digest for the Node.js runtime.
- Route Yarn through the npm registry (`@yarnpkg/cli-dist` via `COREPACK_NPM_REGISTRY`) so both
  package managers share one npm integrity authority, and record the runner image as in the first
  option.
- Require a `packageManager` hash pin (for example `yarn@4.x.y+sha224.<hash>`) in the target
  repository manifest so distribution integrity is source-declared, and record the runner image as
  in the first option.
- Switch Node.js acquisition to a controlled `actions/setup-node` path with `nodejs.org`
  `SHASUMS256.txt` verification and record the Node distribution digest directly.
- Do not record these artifacts and rely on `builder.id`, the runner environment, and the
  already-recorded package-manager name and version.

## Decision Outcome

Chosen option: "Record both artifact classes with source-native digest authorities", because it
follows the SLSA field boundary without changing any acquisition path, uses the strongest digest
authority each source actually offers, and records the Yarn and Node.js cases honestly at the
identification level those ecosystems can support today.

The JS/TS npm package profile extends its closed `resolvedDependencies` schema from the single
lockfile descriptor to an enumerated set:

- `lockfile` (existing, unchanged): the selected lockfile descriptor.
- `package-manager-distribution`: exactly one entry, present only when pnpm or Yarn is selected. The
  `uri` is the actual distribution URL used by Corepack; for pnpm that is the npm registry tarball
  URL, and for Yarn that is the `repo.yarnpkg.com` bundle URL. The `digest` uses the source's native
  authority: the npm registry `dist.integrity` SHA-512 for pnpm, and the SHA-512 computed over the
  downloaded bytes at build time for Yarn, with an annotation distinguishing `registry-integrity`
  from `download-hash` authority. Annotations record the package manager, exact version, and
  acquisition source (`corepack`).
- `runner-image`: exactly one entry identifying the GitHub-hosted runner image version that provided
  the Node.js runtime. The `uri` identifies the runner image release or its software report;
  annotations record `image_os`, `image_version`, and `node_version`. No distribution digest is
  recorded, because no Node.js distribution digest is published for the runner image; the descriptor
  relies on its URI, which the in-toto `ResourceDescriptor` contract permits.

The npm CLI receives no separate distribution descriptor: ADR 0016 binds it to the selected Node.js
toolchain, and ADR 0017 already records its actual version in `externalParameters.package_manager`.
A Node.js distribution descriptor is likewise not recorded; the runner-image descriptor plus the
`node_version` annotation is the honest identity available without changing the acquisition path.

Strict producer-side verification accepts exactly this closed descriptor set and fails before
signing on missing, extra, or malformed entries; verifier policy and fixtures are updated to the
same closed shape. These descriptors are part of the v1 `buildType` contract freeze.

Yarn `packageManager` hash pinning and a controlled `actions/setup-node` acquisition path with
`SHASUMS256.txt` verification are rejected for v1 as acquisition-path and input-requirement changes;
either may return as a later ADR on the reproducibility roadmap. The activation of `builder.version`
and `builderDependencies`, including the signing-adapter descriptor, is out of scope here and
decided by ADR 0071.

### Consequences

- Good, because the two artifact classes that SLSA counts as build-time dependencies are now
  recorded with the strongest authority each source offers.
- Good, because no acquisition path, workflow topology, or caller input requirement changes.
- Good, because Yarn's `download-hash` annotation keeps the weaker authority explicit instead of
  implying a registry guarantee that does not exist.
- Good, because the enumerated closed schema keeps strict verification decidable and v1-frozen.
- Good, because public verifiers are unaffected: the npm verifier does not inspect
  `resolvedDependencies`, `gh attestation verify` focuses on signer identity, and SLSA treats
  recursive dependency verification as best effort.
- Neutral, because Yarn and Node.js recording identifies artifacts at URL-plus-hash or image-version
  level rather than against an independent external authority.
- Bad, because Yarn distribution verification is limited to cross-run consistency of the recorded
  hash until the Yarn ecosystem publishes signed or integrity-listed distributions.
- Bad, because the runner-image descriptor depends on GitHub continuing to publish image versions
  and software reports for the recorded `image_version`.

### Confirmation

This decision is confirmed when:

- the JS/TS npm provenance and publish specification defines the closed `resolvedDependencies`
  descriptor set (`lockfile`, conditional `package-manager-distribution`, `runner-image`) with valid
  and invalid examples for each descriptor;
- the descriptor rules pin URI forms, digest algorithms, digest authority annotations, and the
  exact-version and acquisition-source annotations for `package-manager-distribution`, and the
  `image_os`/`image_version`/`node_version` annotations for `runner-image`;
- the build and pack specification defines how the producer captures each descriptor value,
  including the build-time SHA-512 of the Yarn distribution bytes and the runtime observation of
  `ImageOS`/`ImageVersion`;
- strict verification rejects missing, extra, unknown-annotation, or shape-violating entries before
  signing, and the verification policy and fixtures specification covers accepted and rejected cases
  for every descriptor;
- the v1 `buildType` documentation describes the complete descriptor set;
- implementation review confirms that npm-selected runs emit no `package-manager-distribution` entry
  and that no Node.js distribution descriptor or `packageManager` hash-pin requirement appears.

## Pros and Cons of the Options

### Record both classes with source-native digest authorities

Record pnpm from npm registry integrity, Yarn from the build-time download hash, and the Node.js
runtime through a runner-image identity descriptor.

- Good, because it matches the SLSA boundary rule and ecosystem precedent (the GitHub Actions
  buildtype example records the runner virtual-environment image; Tekton and kpack record
  build-defining images in `resolvedDependencies`).
- Good, because pnpm gains an externally checkable integrity authority while Yarn and Node.js are
  recorded at the strongest honestly available level.
- Good, because acquisition paths, workflow topology, and caller inputs are untouched.
- Bad, because the Yarn hash is self-asserted by the build that used it, and the runner-image
  descriptor carries no digest.
- Bad, because three descriptor shapes with per-source digest rules are more specification and
  fixture surface than a single uniform rule.

### Route Yarn through the npm registry for a uniform authority

Use `COREPACK_NPM_REGISTRY` so Yarn resolves through `@yarnpkg/cli-dist` on the npm registry and
both package managers record npm `dist.integrity`.

- Good, because one integrity authority and one digest rule cover both package managers.
- Bad, because it changes the Yarn acquisition path, which is a behavior change beyond recording.
- Bad, because it depends on the Yarn project continuing to publish `@yarnpkg/cli-dist` and on
  Corepack version-specific registry-override behavior.

### Require `packageManager` hash pinning in target manifests

Require the target repository to declare `packageManager` with an embedded hash so distribution
integrity is source-declared and Corepack verifies it at download.

- Good, because integrity becomes source-declared and Corepack enforces it natively, matching the
  Corepack project's own pinning guidance.
- Bad, because it tightens caller input requirements beyond ADR 0017's exact-version rule and blocks
  every Yarn repository that has not added a hash pin.
- Bad, because it still does not address the Node.js runtime.

### Switch Node.js to a controlled setup-node acquisition with digest recording

Acquire Node.js through `actions/setup-node`, verify the download against `nodejs.org`
`SHASUMS256.txt`, and record the distribution digest.

- Good, because the Node.js runtime would gain a real distribution digest from an external
  authority.
- Bad, because it replaces the runner-provided runtime that ADR 0027 selected and adds a
  verification step that `actions/setup-node` does not perform itself.
- Bad, because it belongs to a reproducibility-strengthening decision rather than a recording
  decision, and its cost is not justified for the v1 freeze.

### Do not record these artifacts

Keep the single lockfile descriptor and rely on `builder.id`, runner environment context, and the
recorded package-manager name and version.

- Good, because the schema stays minimal and nothing new needs verification policy or fixtures.
- Bad, because two SLSA-defined build-time dependency classes remain invisible to verifiers,
  weakening the profile's traceability story at the moment the v1 contract is frozen.
- Bad, because adding the descriptors after a public v1 would become a `buildType` versioning event
  instead of part of the initial freeze.

## More Information

This decision follows ADR 0017's recording requirement and ADR 0029's delegation of provenance field
content, and it scopes itself to the npm profile's `resolvedDependencies`; ADR 0071 decides the
common-contract activation of `builder.version` and `builderDependencies`, including the
signing-adapter descriptor.

Key evidence:

- SLSA v1.2 build provenance: "all artifacts fetched during initialization or execution of the build
  process are considered dependencies" and `resolvedDependencies` is the "unordered collection of
  artifacts needed at build time" (<https://slsa.dev/spec/v1.2/build-provenance>).
- The GitHub Actions `workflow/v1` buildtype records the resolved workflow commit and a runner
  virtual-environment image in `resolvedDependencies`
  (<https://github.com/slsa-framework/github-actions-buildtypes>).
- Corepack verifies pnpm through npm registry `dist.integrity` and signatures and caches the derived
  hash, while the default Yarn Berry path downloads and hashes without registry metadata
  (<https://github.com/nodejs/corepack>, `sources/npmRegistryUtils.ts`, `sources/corepackUtils.ts`).
- GitHub-hosted runner images expose `ImageOS`/`ImageVersion` and publish per-image software
  reports, but no Node.js distribution digest (<https://github.com/actions/runner-images>).
- Public verifiers do not reject additional `resolvedDependencies` entries: npm's provenance
  verifier does not inspect the field, `gh attestation verify` checks signer identity, and SLSA
  verification guidance treats recursive dependency verification as best effort
  (<https://slsa.dev/spec/v1.2/verification_summary>).
