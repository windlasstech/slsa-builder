---
parent: Decisions
nav_order: 28
status: accepted
date: 12026-07-04
decision-makers: Yunseo Kim
---

# Use SHA-Pinned Reusable Workflow Builder Identity

## Context and Problem Statement

ADR 0020 selected reusable workflow path plus full Git tag ref as the production `builder.id` shape,
following the SLSA GitHub Generator convention. ADR 0022 selected
`.github/workflows/js-ts-npm-package-slsa3.yml` as the initial JS/TS npm package profile entrypoint
and showed caller examples using `@vX.Y.Z`. ADR 0026 reused that released-tag caller example when it
documented supported release caller patterns.

Further review of SLSA GitHub Generator and
[`slsa-verifier` issue #12](https://github.com/slsa-framework/slsa-verifier/issues/12) showed that
the upstream `@vX.Y.Z` requirement is not a best-practice preference over commit SHA pinning. It is
a verifier workaround: GitHub Actions OIDC tokens historically exposed the reusable workflow ref as
used by the caller, and hash-pinned refs were hard for `slsa-verifier` to map back to an accepted
version or branch during verification. The SLSA maintainers explicitly recognized full commit SHA
pinning as the stronger security posture, while choosing tag pinning because their verifier did not
yet have a manageable SHA-to-version trust mechanism.

This project is still defining its builder and verifier contract, so it does not need to inherit
that workaround. The next decision is whether production consumers should pin the Windlass reusable
workflow by full commit SHA, whether `builder.id` should identify that exact commit, and how the
human-readable release version should remain visible.

Should the production JS/TS npm package SLSA3 profile keep tag-based builder identity, use SHA-based
builder identity directly, or require SHA pinning while resolving the SHA back to a release tag at
verification time?

## Decision Drivers

- Follow GitHub Actions, OpenSSF Scorecard, and Windlass workflow hardening guidance that
  full-length commit SHA pins are the strongest `uses:` reference form.
- Avoid relying on mutable tag refs as the primary production builder identity.
- Preserve a human-readable release version for operators, release notes, and verifier diagnostics.
- Keep verifier trust rooted in the exact builder workflow code that executed.
- Avoid online verification dependence on current GitHub tag state when possible.
- Preserve ADR 0022's selected workflow filename and ADR 0026's release caller-pattern policy while
  replacing their tag-ref examples for production consumers.
- Keep future support for tag-resolving verifier behavior possible without making it the primary
  trust model.

## Considered Options

- Require full commit SHA pins, use SHA-based `builder.id`, and publish signed version-to-SHA
  metadata for each release.
- Require full commit SHA pins and make the verifier resolve each SHA back to a trusted release tag.
- Require full semantic version tag pins such as `@vX.Y.Z`.
- Allow either full commit SHA pins or full semantic version tag pins.
- Allow moving major/minor tags or branch refs.

## Decision Outcome

Chosen option: "Require full commit SHA pins, use SHA-based `builder.id`, and publish signed
version-to-SHA metadata for each release", because it keeps the production trust decision anchored
in immutable builder code while preserving version information through signed release metadata.

Production consumers of the initial JS/TS npm package SLSA3 profile should call the reusable
workflow with a full 40-character commit SHA:

```yaml
jobs:
  publish:
    uses: windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@0123456789abcdef0123456789abcdef01234567
```

Production consumer workflows should not call this profile with:

- branch refs such as `@main` or `@master`;
- moving major or minor refs such as `@v1` or `@v1.2`;
- full semantic version tags such as `@v1.2.3` as the executable production reference;
- short SHAs;
- caller-controlled or dynamically generated refs.

Production SLSA provenance emitted by profile-owned reusable workflows should use a `builder.id`
whose ref is the same full commit SHA used for the reusable workflow execution:

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/<profile-workflow>.yml@0123456789abcdef0123456789abcdef01234567
```

For the initial JS/TS npm package profile, the shape is:

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@0123456789abcdef0123456789abcdef01234567
```

This decision supersedes ADR 0020's tag-ref `builder.id` format and supersedes the tag-ref caller
examples in ADR 0022 and ADR 0026. It does not change ADR 0022's selected workflow filename or ADR
0026's supported caller trigger patterns.

Each released builder version should publish signed release metadata that maps the human-readable
release version to the exact workflow commit SHA. The metadata should at least identify:

- the release version, such as `vX.Y.Z`;
- the profile workflow path;
- the exact full commit SHA consumers should pin;
- the SHA-based `builder.id` value or values for that release;
- the metadata format/version and signature or attestation mechanism.

The verifier should directly validate the SHA-based `builder.id` against trusted SHA policy or
signed Windlass release metadata. It should not need to resolve the SHA to a tag through current
GitHub API state before making the primary trust decision. Verifier output may display the mapped
release version from trusted metadata so humans can see that a given SHA corresponds to `vX.Y.Z`,
but the machine trust decision remains SHA-based.

Release tags remain useful discovery and release-management handles. They should still be signed,
protected, non-moving, and non-deletable according to organization release-integrity policy, but
they are not the executable production reference consumers should place in `jobs.<job_id>.uses`, and
they are not the primary `builder.id` trust anchor.

### Consequences

- Good, because production consumers execute an immutable reusable workflow commit rather than a
  mutable tag or branch ref.
- Good, because `builder.id` identifies the exact trusted builder code that executed.
- Good, because the policy aligns with GitHub Actions hardening, OpenSSF Scorecard, and Windlass
  SHA-pinning guidance.
- Good, because signed release metadata preserves human-readable versions without making tag state
  the primary trust input.
- Good, because verification can avoid online dependence on current GitHub tag mappings for its
  primary trust decision.
- Neutral, because release tags remain important for discovery and signed release metadata, but no
  longer appear as the executable production ref.
- Bad, because this diverges from the current SLSA GitHub Generator reusable workflow convention.
- Bad, because consumers need a clear way to discover the full SHA for each release.
- Bad, because the project must define, sign, publish, and maintain release metadata and verifier
  support.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification,
release process, documentation, and verifier define:

- full 40-character commit SHA pins as the required production caller reference form;
- failure or documentation warnings for branch refs, moving tags, full semantic version tags, short
  SHAs, or dynamic refs in production examples and policy checks;
- SHA-based `builder.id` values for production provenance;
- signed release metadata mapping each `vX.Y.Z` release to the profile workflow path, full commit
  SHA, and SHA-based `builder.id`;
- verifier behavior that trusts the SHA-based `builder.id` directly through trusted SHA policy or
  signed Windlass release metadata;
- human-readable verifier output that can display the release version mapped to the trusted SHA;
- documentation that explains why this project does not inherit SLSA GitHub Generator's current
  tag-ref workaround.

Implementation review should verify that production provenance does not emit a tag-ref `builder.id`
and that production caller examples do not instruct consumers to execute the reusable workflow by
branch, moving tag, full semantic version tag, or short SHA.

## Pros and Cons of the Options

### Require SHA pins, SHA-based `builder.id`, and signed version metadata

Consumers execute the reusable workflow by full commit SHA. Provenance records the same SHA in
`builder.id`. Signed release metadata maps `vX.Y.Z` to the workflow commit SHA and lets verifiers
show human-readable version information.

- Good, because execution identity and trusted builder identity are the same immutable commit.
- Good, because release verification can be deterministic and independent of mutable tag state.
- Good, because signed metadata can preserve SemVer release UX without weakening the executable ref.
- Good, because this matches the desired end state discussed in
  [`slsa-verifier` issue #12](https://github.com/slsa-framework/slsa-verifier/issues/12) more
  closely than requiring tag execution.
- Bad, because this requires Windlass-specific release metadata and verifier support.
- Bad, because it is less familiar to users of SLSA GitHub Generator examples.

### Require SHA pins and resolve each SHA back to a trusted release tag

Consumers execute by full commit SHA, but the verifier queries or reads metadata to prove that the
SHA corresponds to an accepted release tag, then trusts the release tag.

- Good, because consumers still get the execution-time immutability of SHA pins.
- Good, because operators can express trust in release versions or tag patterns.
- Good, because this resembles the approach proposed in `slsa-verifier` pull request 882.
- Bad, because primary verification can depend on current GitHub API tag state unless a signed
  metadata source is added.
- Bad, because tags created, deleted, or moved after build time can make verification semantics
  subtle.
- Bad, because the machine trust decision still ultimately depends on tag governance rather than the
  exact trusted commit.

### Require full semantic version tag pins

Consumers execute the reusable workflow by a full tag such as `@v1.2.3`, and provenance uses a
tag-ref `builder.id`.

- Good, because this matches the current SLSA GitHub Generator convention and its verifier
  workaround.
- Good, because it is readable and easy for consumers to update with SemVer-aware tooling.
- Bad, because tags are mutable references and weaker than full commit SHA pins.
- Bad, because this project can design its verifier contract now rather than inheriting the upstream
  workaround.
- Bad, because it conflicts with GitHub Actions hardening, OpenSSF Scorecard, and Windlass's general
  SHA-pinning policy.

### Allow either full commit SHA pins or full semantic version tag pins

Consumers may choose either `@<sha>` or `@vX.Y.Z` for production calls.

- Good, because it maximizes adoption flexibility.
- Good, because consumers familiar with either security model can proceed.
- Bad, because the same production builder release may have multiple valid identity forms.
- Bad, because verifier policy, documentation, and support burden increase.
- Bad, because allowing tag execution preserves the weaker mutable-ref path this decision is trying
  to avoid.

### Allow moving major/minor tags or branch refs

Consumers may use refs such as `@v1`, `@v1.2`, or `@main`.

- Good, because consumers receive updates automatically.
- Good, because examples are shorter.
- Bad, because mutable refs can change what code executes without a caller workflow change.
- Bad, because moving refs are inappropriate for production registry mutation and SLSA3 builder
  trust.
- Bad, because branch refs can pick up unreleased or breaking builder changes.

## More Information

This decision supersedes ADR 0020. It also supersedes only the tag-ref executable examples and
tag-ref `builder.id` expectations in ADR 0022 and ADR 0026; their workflow filename and caller
trigger decisions remain in force.

Reference points considered:

- GitHub Actions reusable workflows can be referenced by SHA, tag, or branch, and GitHub documents
  full commit SHA as the safest form for stability and security.
- GitHub Actions security hardening guidance says full-length commit SHA is the only immutable
  action release reference and applies the same third-party workflow concerns to reusable workflows.
- OpenSSF Scorecard's pinned-dependencies check recommends hash-pinned GitHub workflow dependencies.
- Windlass workflow hardening policy requires full 40-character SHA pins for ordinary `uses:`
  references, with only narrow documented exceptions.
- SLSA GitHub Generator currently requires full semantic version tags for its reusable builders, but
  [`slsa-verifier` issue #12](https://github.com/slsa-framework/slsa-verifier/issues/12) documents
  this as a limitation caused by difficulty verifying hash-pinned reusable workflow refs from
  available OIDC token information.
- [`slsa-verifier` issue #12](https://github.com/slsa-framework/slsa-verifier/issues/12) and related
  discussion identify SHA pinning as stronger for stability and insider-risk resistance, and discuss
  allowlists, signed mechanisms, and SHA-to-tag resolution as possible ways to support hash-pinned
  builders.
