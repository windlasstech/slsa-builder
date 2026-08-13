---
parent: Decisions
nav_order: 79
status: accepted
date: 12026-08-13
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0032
    scope:
      "the invocation-ref requirement for manual dispatch (workflow_dispatch must run on a tag ref
      and must fail when github.ref_type is not tag); the version-tag matching, no-tag-creation,
      rerun-as-same-release, and already-published-version clauses remain in force"
  - type: see-also
    target: ADR-0026
  - type: see-also
    target: ADR-0068
  - type: see-also
    target: ADR-0080
---

# Support a Tags-Only Caller-Specified Build Source Ref for Release Retries Across Profiles

## Context and Problem Statement

ADR 0026 selected tag-push caller workflows plus constrained `workflow_dispatch` as the supported
production release caller patterns, and ADR 0032 decided the manual-dispatch constraints: a dispatch
release must run on a tag ref whose name matches `v${package.json version}`, and the called workflow
must fail before packing, provenance generation, or publish when `github.ref_type` is not `tag`. The
profile's provenance and verification contracts bind source identity to that same invocation ref:
the built content is whatever the run's `github.ref`/`github.sha` resolve to, and the signed
`source.ref`/`source.revision` fields record those invocation values.

This binding blocks the standard "retry a failed release with a fixed pipeline" scenario, reported
as [issue #81](https://github.com/windlasstech/slsa-builder/issues/81) after the first live npm
dogfood of `vers-js`. That run,
[run 31622651874](https://github.com/windlasstech/vers-js/actions/runs/31622651874), failed on the
settings-only pnpm workspace classification defect that ADR 0078 and its implementation corrected.
The correction reaches the caller as a re-pinned reusable workflow reference on the caller's `main`
branch, but the existing release tag still contains the old caller workflow file with the old pin.
Dispatching from the tag re-executes the broken logic. Dispatching from `main` uses the fixed
pipeline but builds `main` HEAD, and the runtime guards reject the run because the invocation ref is
not the version tag. The only in-contract recovery is deleting and recreating the signed tag on the
fixed commit, which requires temporarily relaxing tag-protection rulesets and conflicts with the
organization's tag-immutability posture.

Platform facts constrain the design space. GitHub `workflow_dispatch` executes the workflow file at
the selected ref, and the OIDC token's `ref`/`sha` claims — which the Fulcio signing certificate
embeds as the Source Repository Ref (`.14`) and Source Repository Digest (`.13`) extensions — are
platform-issued invocation-context values that the signer cannot override. `actions/checkout`
supports building a caller-named ref independent of the run's ref. The prior hand-rolled `vers-js`
pipeline used exactly this separation in production for the v0.1.1 publish: per issue #81, the
workflow file came from the dispatch ref while every job checked out the release tag, so the built
tarball derived from the signed tag commit. slsa-github-generator never offered an equivalent
capability; its provenance source fields always tracked the invocation ref
([issue #1947](https://github.com/slsa-framework/slsa-github-generator/issues/1947),
[issue #3321](https://github.com/slsa-framework/slsa-github-generator/issues/3321) record the same
unsolved pain).

Under the SLSA v1 provenance model, a caller-selected source ref is an externally controlled build
input and therefore belongs in `externalParameters`, whose completeness SLSA Build L3 requires; the
immutable commit it resolves to belongs in `resolvedDependencies`. The specification model
accommodates "build logic from one origin, built content from another" as long as both are recorded
honestly and verifiably.

Should slsa-builder support building a caller-specified source ref so a failed release can be
retried with a fixed pipeline — as a default policy for every profile that builds and attests source
content, starting with the initial JS/TS npm package profile — and if so, under what constraint?

## Decision Drivers

- Solve the problem once at the foundation level: the retry dead end is structural, not
  npm-specific. Every future producer profile built on the extensible foundation (ADR 0002,
  ADR 0003) would otherwise inherit the same dead end or re-solve it with a divergent contract.
- Preserve the release trust anchor: published artifacts must provably derive from the signed,
  protected release tag, not from a movable branch head.
- Keep npm package releases tied to immutable Git tags and package versions, per ADR 0032's core
  concern that manual dispatch must not become an arbitrary release button.
- Provide an operational recovery path for builder-side or caller-pipeline defects without weakening
  tag protection or re-signing tags.
- Record externally controlled inputs completely, per SLSA Build L3 external-parameter completeness,
  and record the resolved immutable revision so verification binds to commit SHAs rather than
  movable refs (ADR 0068).
- Respect platform facts: the caller workflow file always comes from the dispatch ref, and the
  signing certificate's source claims always describe the invocation context.
- Stay within the npm registry's accepted external-provenance surface for the first adopter: the
  registry and npm CLI validate the subject PURL, the tarball SHA-512, the signature, and the
  package repository identity against the certificate's Source Repository URI; none of these change
  when a different ref of the same repository is built.
- Keep the supersession of ADR 0032 as narrow as possible: only the invocation-ref requirement
  moves; the version-tag matching, no-tag-mutation, rerun, and already-published clauses stand.

## Considered Options

- Keep invocation-bound source identity and treat retagging as the canonical recovery path.
- Support a caller-specified build source ref restricted to the version tag.
- Support a caller-specified build source ref without a ref-class constraint.

## Decision Outcome

Chosen option: "Support a caller-specified build source ref restricted to the version tag", because
it preserves the signed-tag release anchor and ADR 0032's anti-arbitrary-release boundary while
unblocking the fixed-pipeline retry scenario that the current contract makes impossible.

This is a default policy for slsa-builder as a whole, not an npm-specific exception. Every producer
profile — every profile that builds and attests source content — must offer the same capability
under the same constraint, and the initial JS/TS npm package profile is the first adopter. Future
producer profiles must include the input in their initial public contract rather than retrofitting
it. Profiles that do not build source content, such as the GitHub Release asset publisher, which
distributes verified producer bytes under ADR 0049's distributor model, are out of scope: they have
no built source ref to select.

Every producer profile's public `workflow_call` contract gains one optional input, `source-ref`,
with these decision-level semantics (the JS/TS npm package profile implements them first):

- When `source-ref` is omitted, behavior is exactly as before: the invocation ref is the built ref,
  and the profile's existing tag-push and constrained tag-dispatch caller patterns apply unchanged
  (for the npm profile, ADR 0026 and ADR 0032).
- When `source-ref` is supplied, it names the Git ref whose content the profile builds, packs,
  attests, and publishes. The input must be a full `refs/tags/<tag-name>` ref; short tag names,
  branch refs, pull-request refs, arbitrary commit SHAs, and other ref classes are rejected.
- The supplied tag must satisfy the profile's release tag convention: for the npm profile, the tag
  name must equal `v${package.json version}`, checked against the packed artifact metadata as ADR
  0032 already requires; future profiles define their own tag convention and apply the same rule.
  The tag must already exist in the caller repository and must resolve to a commit; resolution
  failure is a fail-closed error before install, pack, signing, or publish.
- With `source-ref` supplied on a `workflow_dispatch` run, the invocation ref may be any ref the
  caller selects — in particular the caller's default branch carrying a fixed pipeline. The release
  identity remains the built tag. The runtime guards therefore move from the invocation ref to the
  built ref: the built ref must be the version tag, while the invocation ref is recorded as
  invocation context rather than rejected.
- A `source-ref` that disagrees with the invocation ref on a tag-triggered run (for example a tag
  push for `v1.2.3` carrying `source-ref: refs/tags/v1.2.4`) is a conflict and fails before build.
- No profile ever creates, moves, signs, or deletes tags; tag creation, signing, and protection
  remain caller-side operations outside every profile, exactly as ADR 0032 decided for npm.
- Per-profile rerun and already-published semantics are unchanged: for npm, repeated dispatches for
  the same release tag are distinct invocations of the same release, and an already-published
  package version fails clearly, as ADR 0032 decided.

This ADR decides only the scenario-support axis: whether producer profiles support a
caller-specified build source ref and under which constraint. It deliberately does not decide how
the signed provenance and the verification policy represent the separation between the built source
identity and the invocation context, including how the Fulcio certificate's invocation-bound source
claims relate to the signed source fields. Those binding-model questions are the subject of a
follow-up ADR, which must land before any profile implements this capability. Until then, the
current verification contract — which requires the certificate source claims to equal the signed
source fields — continues to reject dispatch-from-a-non-tag runs, so this ADR alone changes no
observable verification behavior.

### Consequences

- Good, because the capability is decided once as a foundation-level default: every current and
  future producer profile shares one documented retry pattern instead of diverging per ecosystem.
- Good, because a failed release can be retried with a fixed caller pipeline while the published
  artifact still provably derives from the signed release tag; tag protection never needs relaxation
  for pipeline fixes.
- Good, because the constraint keeps the release source identity exactly where ADR 0032 anchored it:
  the built ref must be the version tag, so the provenance claim "this package derives from the
  signed tag's commit" is preserved rather than weakened.
- Good, because the input is an ordinary externally controlled parameter in the SLSA v1 model: it
  will be recorded in signed `externalParameters` and its resolution in `resolvedDependencies`,
  keeping Build L3 completeness intact.
- Good, because the npm publish path is unaffected: registry- and CLI-side validation of the subject
  PURL, tarball SHA-512, signature, and repository identity does not depend on which ref of the
  caller repository was built.
- Good, because the supersession of ADR 0032 is minimal and explicit: only the invocation-ref
  requirement for manual dispatch moves to the built ref.
- Neutral, because the npm profile's public contract grows from eight inputs to nine as the first
  adopter, and future producer profiles carry the additional input from their initial contract; the
  supported-trigger, runtime-guard, and manual-dispatch sections of each adopting profile's
  specification must be written or rewritten when the follow-up binding-model ADR lands.
- Neutral, because GitHub environment deployment rules match the invocation ref (`GITHUB_REF`), so
  callers that gate releases with tag-pattern environment rules must rely on the profile's built-ref
  guard instead; this is documented caller guidance, not a new mechanism.
- Bad, because callers gain a second way to express release intent, and documentation must be
  precise about when `source-ref` is appropriate (pipeline-fix retries) versus unnecessary (ordinary
  tag-push releases).
- Bad, because the npm registry's server-side provenance validation beyond the documented checks
  (subject, digest, signature, repository identity) is not fully observable from primary sources; a
  registry-side cross-check of provenance source fields against OIDC invocation claims, if one
  exists, would surface only at publish time. The first dogfood retry must confirm acceptance.

### Confirmation

This decision is confirmed when, after the follow-up binding-model ADR, the JS/TS npm package
profile — as the first adopter — satisfies the criteria below in its specification, implementation,
documentation, and verifier guidance, and when every future producer profile's initial contract
satisfies the same criteria from its first version:

- the optional `source-ref` public input in the `workflow_call` contract, including its signed
  representation in `externalParameters` and the resolved commit in `resolvedDependencies`;
- rejection, before install, pack, signing, or publish, of: a non-`refs/tags/` value, a tag name
  that does not satisfy the profile's release tag convention (`v${package.json version}` for npm,
  proven by the packed artifact metadata), a tag that does not exist or does not resolve to a
  commit, and a `source-ref` that conflicts with the invocation tag on a tag-triggered run;
- runtime guards that evaluate the built ref rather than the invocation ref for release-ref
  acceptance, while the profile's supported caller event set (for npm, `push` and constrained
  `workflow_dispatch` from ADR 0026) remains the only accepted trigger classes;
- unchanged behavior when `source-ref` is omitted, proven by regression fixtures covering the
  tag-push and tag-dispatch flows;
- caller documentation showing the fixed-pipeline retry pattern — dispatch from the default branch
  with `source-ref: refs/tags/vX.Y.Z` — and stating that the profile never creates, moves, signs, or
  deletes tags;
- dogfood evidence from `vers-js`: a dispatch from the caller's default branch with
  `source-ref: refs/tags/v0.1.2` that publishes provenance whose signed source revision is the
  release tag's commit, accepted by the npm registry;
- the follow-up binding-model ADR referenced by number once accepted, with the verification policy
  and fixtures updated to match.

Implementation review should verify that no code path allows a non-tag `source-ref`, an unresolved
ref, or a tag-convention mismatch to reach pack, signing, or publish, and that the omitted-input
flows are byte-for-byte compatible with the pre-existing contract.

## Pros and Cons of the Options

### Keep invocation-bound source identity and treat retagging as the canonical recovery path

Keep ADR 0032 unchanged: every production release runs on its version tag, and a pipeline defect
that strands a tag is recovered by deleting and recreating the signed tag on the fixed commit.

- Good, because the contract, the verification model, and the implementation stay exactly as they
  are; no specification, provenance, or verifier change is needed.
- Good, because the invocation ref and the built ref can never diverge, which keeps the
  certificate-to-provenance source binding trivially satisfiable.
- Bad, because deleting and recreating a signed tag requires temporarily relaxing tag-protection
  rulesets, which conflicts with the organization's tag-immutability posture and opens a governance
  gap precisely when a release is already in a failure state.
- Bad, because the failure mode is not hypothetical: the first live dogfood hit it immediately, and
  every future builder-side defect affecting an already-created tag lands in the same dead end.
- Bad, because slsa-github-generator shipped the same invocation-bound model and left the retry pain
  unsolved for its whole lifetime; inheriting that limitation is a known cost, not a theoretical
  one.

### Support a caller-specified build source ref restricted to the version tag

Add an optional input that names the built ref, constrained to the full version-tag ref, with
fail-closed resolution and conflict handling.

- Good, because one foundation-level pattern serves every producer profile: future ecosystems adopt
  the same input, constraint, and guard semantics instead of inventing per-profile variants.
- Good, because the fixed-pipeline retry works without touching tags, rulesets, or signing
  infrastructure.
- Good, because the release trust anchor is unchanged: the built content must still be the version
  tag, so provenance continues to bind the artifact to the signed tag's commit.
- Good, because the constraint composes with caller-side guards and with SLSA Build L3
  external-parameter completeness: the input is small, enumerable, and recorded.
- Good, because it matches the production-proven pattern from the caller's previous hand-rolled
  pipeline (logic from the dispatch ref, content from the signed tag).
- Neutral, because the verification binding between certificate invocation claims and signed source
  fields must be redesigned by a follow-up ADR before implementation.
- Bad, because the public contract, the runtime guards, the provenance specification, and the
  verifier guidance all gain a second release-intent path that must be specified, tested, and
  documented.

### Support a caller-specified build source ref without a ref-class constraint

Add the same input but accept any ref or commit SHA, relying on honest provenance and verifier
policy to distinguish releases from other builds.

- Good, because it maximizes caller flexibility for future prerelease or experimental channels.
- Good, because the guard logic is simplest: any resolvable ref is buildable.
- Bad, because it reintroduces the arbitrary release button that ADR 0032 explicitly rejected: any
  dispatcher could publish a registry mutation from a movable branch head, and tag signing and
  protection policies would no longer gate what may be released.
- Bad, because it shifts the release-integrity burden to every downstream verifier's policy, which
  must then distinguish legitimate release refs from incidental ones — complexity the ecosystem
  precedent (tag-anchored releases in slsa-verifier policy flags and GitHub deployment rules) does
  not support for production package publishing.
- Bad, because npm package versions are immutable: a branch-head release that accidentally publishes
  a version is unrecoverable, so flexibility here buys little and risks much.

## More Information

This decision follows ADR 0026 and ADR 0032 and partially supersedes the latter's invocation-ref
requirement for manual dispatch. It preserves ADR 0032's version-tag convention, tag-mutation
prohibition, rerun semantics, and already-published failure behavior. It applies the
profile-extensible foundation of ADR 0002 and ADR 0003 consistently: the capability is decided once
as a default policy and adopted by every producer profile, with the JS/TS npm package profile as the
first adopter. It is consistent with ADR 0028 (the reusable workflow's own identity is SHA-pinned
independently of the caller's ref choices) and ADR 0068 (verification ultimately binds to the
immutable source repository and commit identities). The representation of the
built-versus-invocation separation in signed provenance and the verification policy is deferred to
the follow-up binding-model ADR named in the confirmation criteria.

Reference points considered:

- [Issue #81](https://github.com/windlasstech/slsa-builder/issues/81): the retry-scenario report,
  including the failed `vers-js` v0.1.2 run and the prior hand-rolled pipeline's production use of
  dispatch-ref logic with tag-content builds.
- [GitHub: Manually running a workflow](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow)
  and
  [OIDC with reusable workflows](https://docs.github.com/en/actions/how-tos/secure-your-work/security-harden-deployments/oidc-with-reusable-workflows):
  the dispatched run executes the selected ref's workflow file, and the OIDC `ref`/`sha` claims
  describe the caller invocation context.
- [Fulcio OID information](https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md): the
  certificate Source Repository Digest (`.13`) and Source Repository Ref (`.14`) extensions map to
  the platform-issued `sha` and `ref` claims.
- [actions/checkout](https://github.com/actions/checkout): the `ref` input builds a caller-named ref
  independent of the run's ref.
- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance) and
  [Verifying Artifacts](https://slsa.dev/spec/v1.2/verifying-artifacts): externally controlled
  inputs belong in `externalParameters` (complete at Build L3), resolved immutable revisions in
  `resolvedDependencies`, and verifiers bind policy to `buildType` and `externalParameters`.
- slsa-github-generator
  [issue #1947](https://github.com/slsa-framework/slsa-github-generator/issues/1947) and
  [issue #3321](https://github.com/slsa-framework/slsa-github-generator/issues/3321): the same
  dispatch-versus-source-ref pain, never resolved in that ecosystem.
- npm external provenance validation surface:
  [`libnpmpublish` provenance checks](https://github.com/npm/cli/blob/51c2bf81fa2c31547d0fec44fff2aaac3d9a9862/workspaces/libnpmpublish/lib/provenance.js)
  validate the single subject, its PURL name, and its SHA-512 digest plus the sigstore signature;
  the documented registry checks add the certificate issuer, runner environment, public repository
  visibility, and `repository.url` equality with the certificate Source Repository URI (enforced in
  practice, [npm/cli#8036](https://github.com/npm/cli/issues/8036)). A registry-side equality check
  between provenance source ref or digest fields and the OIDC invocation claims was not found in
  primary sources and remains unverified; the first dogfood retry is the empirical bound.
