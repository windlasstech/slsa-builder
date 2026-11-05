---
parent: Decisions
nav_order: 103
status: accepted
date: 12026-11-05
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0028
  - type: see-also
    target: ADR-0029
  - type: see-also
    target: ADR-0079
  - type: partially-supersedes
    target: ADR-0080
    scope:
      "the omitted-source-ref byte-compatibility promise (the 'Change nothing when the capability is
      unused' decision driver and the 'Backward compatibility is structural' outcome clause):
      pre-release contract revisions may add required invocation-identity fields, so byte-for-byte
      compatibility with pre-change bundles is not guaranteed; the built-source policy binding,
      certificate source claims as invocation-context evidence, repository identity binding, and Run
      Invocation URI binding remain in force"
  - type: see-also
    target: ADR-0104
  - type: see-also
    target: ADR-0105
---

# Define External Workflow Parameters as Common Invocation Identity

## Context and Problem Statement

The ninth live M1 dogfood attempt on 12026-11-03 (run
[41666974638](https://github.com/windlasstech/vers-js/actions/runs/41666974638), recorded in
[npm-m1.md](../dogfood/npm-m1.md) and
[issue #30](https://github.com/windlasstech/slsa-builder/issues/30)) captured npm's complete
registry rejection through the ADR 0101 stderr relay. The registry rejected the Windlass-signed SLSA
v1 predicate with `E422`, including these three external-parameter comparisons:

```text
buildDefinition.externalParameters.workflow.ref: <empty> does not match expected value for SourceRepositoryRef: refs/heads/main,
buildDefinition.externalParameters.workflow.repository: <empty> does not match expected value for SourceRepositoryURI: https://github.com/windlasstech/vers-js,
buildDefinition.externalParameters.workflow.path: .github/workflows/js-ts-npm-package-slsa3.yml does not match expected value for BuildConfigURI: .github/workflows/publish.yml,
```

The rejection establishes three facts about the registry's GitHub Actions compatibility checks.
`workflow.ref` and `workflow.repository` must be present, `workflow.path` must identify the caller
workflow rather than the reusable builder workflow, and all three values must agree with the
corresponding platform-issued certificate claims. The registry-accepted, npm-generated statement for
`@windlass/vers-js@0.1.2` independently exhibits the same shape: its workflow ref is
`refs/heads/main` even though the published content was built from the `v0.1.2` tag, and its
workflow path is the caller's `.github/workflows/publish.yml`.

The current Windlass external-parameter contract instead uses `externalParameters.workflow` as part
of the builder identity record: its `path` is fixed to the reusable builder workflow path, and its
`sha` and `builder_id` describe that same builder. The group therefore combines a path, commit, and
canonical URI for one trusted builder. It does not represent the caller-selected invocation
interface that npm checks. [Issue #143](https://github.com/windlasstech/slsa-builder/issues/143)
tracks the compatibility work; this ADR decides only the placement and meaning of the external
workflow fields.

The decision applies to every profile that emits SLSA producer provenance. The common specification
must define the invocation identity field set and its meaning once, and each profile specification
must reference that definition rather than introduce a registry-specific projection. This is not a
requirement for whole-predicate uniformity: ecosystem-specific package, publish, and dependency
records remain profile-owned, publishers preserve producer provenance without becoming builders, and
the release manifest retains its distinct predicate and trust records.

ADR 0079 and ADR 0080 already distinguish the built release source from the invocation context. The
built tag and its resolved commit remain recorded in signed `source.*` and `release.*` fields, while
certificate source claims authenticate the invocation. The new external workflow group must fit that
model without substituting the invocation ref for the built tag or replacing the SHA-pinned builder
identity selected by ADR 0028.

Which fields and meanings should `externalParameters.workflow` carry to represent caller invocation
identity consistently across all producer profiles without conflating invocation, built source, and
builder identity?

## Decision Drivers

- One signed statement must serve both Windlass verification and npm registry admission. Separate
  canonical and registry-facing serializations would create a second authority and allow them to
  drift.
- Invocation identity must have the same field set and meaning across all producer profiles, with
  one normative definition in the common provenance specification.
- The group must describe the caller-selected invocation interface: repository, invocation ref, and
  caller workflow path. It must not assert that these values identify the content that was built.
- Built-source and invocation identities must remain distinct on an ADR 0079 dispatch retry, and
  must both remain present when they coincide on a tag-push run.
- Windlass's custom `buildType` and ADR 0028's SHA-pinned `runDetails.builder.id` remain unchanged.
  Compatibility requires borrowing field vocabulary, not misdescribing a reusable-workflow build as
  a top-level workflow build.
- The change should minimize verification-policy and fixture churn and should leave independent
  decisions, including builder-record relocation and dependency-descriptor changes, to focused
  follow-up ADRs.
- Adding invocation evidence must not rescue an invalid built-source identity, untrusted builder, or
  mismatched artifact digest. Existing fail-closed decisions remain conjunctive.

## Considered Options

- Redefine `externalParameters.workflow` as the common invocation-interface identity group
  `{ref, repository, path}`.
- Define neutral common invocation fields and add an npm-only registry projection.
- Emit both neutral invocation fields and npm-compatible aliases in every producer profile.

Adopting the complete upstream GitHub Actions `workflow/v1` build type was excluded before option
comparison. It is a build-type conversion, not a field-placement option, and its definition
explicitly excludes reusable-workflow execution. The detailed basis for that exclusion is recorded
under More Information.

## Decision Outcome

Chosen option: "Redefine `externalParameters.workflow` as the common invocation-interface identity
group `{ref, repository, path}`", because it satisfies the observed registry comparisons with one
canonical serialization, gives every producer profile the same externally controlled invocation
record, and preserves the existing built-source and builder identity models without introducing a
second projection contract.

### The common invocation-interface group

Every SLSA producer profile emits `externalParameters.workflow` with exactly these members:

```json
{
  "workflow": {
    "ref": "refs/heads/main",
    "repository": "https://github.com/example/project",
    "path": ".github/workflows/publish.yml"
  }
}
```

The members have these common meanings:

- `ref` is the invocation ref: the Git ref on which the run was invoked, as authenticated by the
  platform's Source Repository Ref certificate claim. On an ADR 0079 dispatch retry this may be
  `refs/heads/main`; it is not replaced by the built release tag.
- `repository` is the canonical HTTPS repository URI for that invocation, consistent with the
  platform's Source Repository URI claim and the existing canonical repository identity rules.
- `path` is the repository-relative path of the caller workflow YAML file, derived from the
  platform-authenticated caller workflow reference and consistent with the certificate's Build
  Config URI claim. It is not the reusable builder workflow's own path.

All three members are required whenever producer provenance is emitted, including when invocation
and built-source identities coincide. Conditional omission would recreate the registry-observed
absence rejection and would make the common contract depend on whether two semantic roles happen to
identify the same commit.

This group records the external interface to the build. The caller selects the repository,
invocation ref, and workflow; GitHub authenticates the resulting invocation through OIDC and the
signing certificate. Platform authentication does not turn those selections into builder identity or
into evidence about which source content was built.

### Boundaries with existing identity records

- `externalParameters.source.*` and `externalParameters.release.*` remain the built-release policy
  surface defined by ADR 0079 and ADR 0080. In particular, `source.ref` continues to identify the
  built release tag even when `workflow.ref` identifies a different invocation ref.
- `runDetails.builder.id` continues to identify the SHA-pinned reusable builder workflow selected by
  ADR 0028. The registry's silence about the custom Windlass `buildType` and `builder.id` in attempt
  9 is not treated as proof of end-to-end acceptance, but neither is converted into a reason to
  replace them.
- The redefined `workflow` group no longer carries the builder's path, SHA, or canonical builder URI
  as one coherent record. Those fields are additional builder evidence, not the statement's sole
  builder identity: `runDetails.builder.id` remains the canonical SHA-pinned identity required by
  ADR 0028. Relocation or deliberate removal of the redundant external-parameter representations is
  a separate follow-up decision (planned as ADR 0104), not an implicit part of this one. Until that
  decision lands, no specification or implementation migration may simply delete the existing
  builder path, SHA, or URI checks. Every verification gate must retain equivalent builder-identity
  verification against `runDetails.builder.id` and the authenticated signer identity.
- Retention, removal, or deduplication of the conditional
  `source.invocation_ref`/`source.invocation_revision` pair is outside this decision. That
  relationship belongs with the invocation-source dependency decision (planned as ADR 0105) or a
  separate focused ADR. This decision does not rename or delete those existing fields.
- ADR 0080's omitted-`source-ref` byte-for-byte compatibility promise is partially superseded. Its
  built-source policy binding, certificate source claims as invocation-context evidence, repository
  identity binding, and Run Invocation URI binding remain in force. Because the project is
  pre-release and has no supported external consumers of the old statement shape, this ADR does not
  introduce a build-type version migration or an old-bundle compatibility path. Historical dogfood
  bundles remain evidence for the attempts that produced them, not supported inputs under the
  revised contract; conformance fixtures follow the current contract.
- The exact equality graph, certificate extension OIDs, canonicalization and parsing rules,
  diagnostics, and fixtures belong to the common provenance and verification specifications.

The required `internalParameters.github` triple of string fields (`event_name`, `repository_id`, and
`repository_owner_id`) and the acquisition-and-verification doctrine are premises of the overall
compatibility work rather than options decided by this ADR. The default remains the three-field
closed GitHub object, without `actor_id` or `triggering_actor_id`; values are derived from
GitHub-issued OIDC claims before signing and compared fail-closed with the issued certificate before
any registry or release mutation. Their observable rules belong in the specifications.

### Consequences

- Good, because the bytes verified by Windlass are also the bytes evaluated by npm. There is no
  second registry-facing serialization whose fields could diverge from the canonical statement.
- Good, because every producer profile inherits one invocation-interface definition. A future
  profile does not need to invent its own registry compatibility field names or reinterpret an
  npm-only projection.
- Good, because ADR 0079's retry model and ADR 0080's built-source/invocation binding model remain
  coherent: a statement can say that the workflow logic was invoked from one ref while different
  source content was built and attested, without conflating those claims.
- Good, because ADR 0028's immutable builder identity and Windlass's custom `buildType` survive.
  Compatibility comes from accurate invocation metadata, not from weakening or disguising the
  trusted build model.
- Good, because the three fields fit SLSA v1.2's `externalParameters` model: they describe the
  externally selected build interface and are covered by the signed, closed build-type contract.
- Neutral, because this reuses selected GitHub Actions `workflow/v1` build type vocabulary without
  adopting that build type. The common specification must explain the invocation-interface meaning
  explicitly rather than claiming upstream build-type conformance.
- Bad, because the redundant external-parameter builder record no longer has a decided home until
  the follow-up builder relocation decision lands. The canonical identity remains at
  `runDetails.builder.id`, but specifications and implementation must be updated as a coordinated
  set: existing builder path, SHA, and URI checks must be retained or replaced with equivalent
  checks, never dropped without replacement.
- Bad, because invocation identity can be represented in more than one signed location during the
  transition. Until the separate deduplication decision, the specifications must define every
  required equality and must not let one representation override another.
- Bad, because a conforming statement still has no proven end-to-end npm acceptance guarantee. The
  registry validator is closed-source, its published design document is archived, and a later
  dogfood attempt is the only definitive test for any remaining rejection family.

## Confirmation

This decision is confirmed when:

- the common SLSA provenance specification defines `externalParameters.workflow` exactly once as the
  required invocation-interface group `{ref, repository, path}`, and every producer profile
  specification references that definition without redefining the fields or adding an npm-only
  projection;
- accepted examples cover both a tag-push run, where invocation and built-source identities
  coincide, and an ADR 0079 dispatch retry, where `workflow.ref` is the invocation ref while
  `source.ref` and `release.ref` remain the built release tag;
- producer-side validation and post-sign verification reject absent fields and any disagreement
  between these signed values and the corresponding authenticated invocation or certificate claims
  before any registry or release mutation;
- verification rejects an invalid built-source identity, untrusted builder, or subject-digest
  mismatch even when all three invocation fields are internally consistent;
- migration review confirms that no existing builder path, SHA, or URI check is deleted without an
  equivalent check against `runDetails.builder.id` and the authenticated signer identity before the
  follow-up placement decision;
- TDD fixtures demonstrate that the builder workflow path cannot appear as `workflow.path`, that the
  built tag cannot replace an invocation ref on a dispatch retry, and that required fields remain
  present on an equal-identity tag-push run;
- a subsequent controlled dogfood attempt establishes whether npm stops reporting these three fields
  as absent or mismatched. Any remaining rejection family is recorded in `docs/dogfood/npm-m1.md`;
  absence of a complaint about these fields alone is not represented as proof of complete registry
  acceptance.

## Pros and Cons of the Options

### Redefine `externalParameters.workflow` as common invocation identity

Use the registry-observed field names in the common contract and define the whole group as the
caller-selected invocation interface.

- Good, because it directly satisfies the three registry comparisons without translating between
  Windlass field names and npm field names.
- Good, because the common contract remains the sole authority for every producer profile, meeting
  the profile-consistency requirement without a profile-specific exception.
- Good, because `externalParameters` remains a truthful description of the build's external
  interface: the caller chose the invocation repository, ref, and workflow, while the platform
  authenticated those facts.
- Good, because `source.*`, `release.*`, and `runDetails.builder.id` retain their established
  meanings. The invocation record supplements rather than rewrites built-source and builder
  identity.
- Good, because strict consumer validation has one closed shape to enforce, and certificate
  comparisons can target the same fields that npm reads.
- Neutral, because `workflow` is a shorter and less explicit group name than `invocation`; its
  common definition must remove that ambiguity.
- Bad, because the former `workflow.path` meaning is replaced, so historical dogfood bundles and
  fixtures no longer conform without regeneration. This is a pre-release revision cost rather than a
  public compatibility commitment.
- Bad, because the builder triplet must move or be deliberately eliminated by the follow-up
  builder-record decision, adding coordinated specification and fixture work.

### Neutral common invocation fields plus an npm-only projection

Define names such as `invocation.ref`, `invocation.repository`, and
`invocation.caller_workflow_path` in the common contract, then add the npm-compatible `workflow.*`
fields only in the npm profile.

- Good, because the neutral common names do not depend on npm's current registry vocabulary.
- Good, because an npm-specific registry policy change could be contained in the npm profile.
- Bad, because it violates the decision driver that invocation identity fields and meanings are
  common across producer profiles: the npm profile acquires fields other profiles do not share.
- Bad, because two serialization contracts describe the same invocation. Every producer and consumer
  must define which is authoritative and enforce their equality indefinitely.
- Bad, because any future profile facing the same registry family must duplicate or reintroduce the
  projection, multiplying contract and fixture surface rather than containing it.

### Neutral invocation fields plus common npm-compatible aliases everywhere

Emit both the neutral invocation record and the `workflow.*` alias in every producer profile.

- Good, because every profile has the same complete field set and the npm-compatible names are
  always available.
- Good, because neutral names remain available to consumers that prefer platform-independent
  terminology.
- Bad, because it expands the npm-only projection's duplication to every profile instead of
  eliminating it. It adds the largest schema, equality graph, and fixture burden of the three
  options.
- Bad, because no existing consumer requirement justifies the neutral aliases. The duplicate record
  buys terminology flexibility at the price of a permanent second authority to validate.
- Bad, because two required representations create more ways for a statement to be internally
  inconsistent and do not improve npm admission over the chosen option.

## More Information

Evidence and normative references:

- Dogfood attempt 9 and the verbatim registry rejection: [npm-m1.md](../dogfood/npm-m1.md) and run
  [41666974638](https://github.com/windlasstech/vers-js/actions/runs/41666974638).
- Compatibility-projection tracking:
  [issue #143](https://github.com/windlasstech/slsa-builder/issues/143).
- Source reliability tiers, certificate-claim mapping, accepted-versus-rejected statement
  comparison, and registry-behavior caveats:
  [npm-registry-provenance-requirements.md](../dogfood/npm-registry-provenance-requirements.md).
- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance): `externalParameters`
  describes the external build interface, each build type defines parameter interpretation, and
  verifiers may reject unexpected fields under a closed contract.
- [GitHub Actions `workflow/v1` build type](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/README.md):
  source of the borrowed `workflow.{ref,repository,path}` vocabulary.

### Why the complete GitHub Actions `workflow/v1` build type was excluded

The excluded alternative would adopt the complete upstream build type rather than borrow its field
vocabulary. Its README states:

> This build type is **not** meant to describe execution of a subset of a top-level workflow, such
> as an action, job, or reusable workflow.

It separately explains that reusable workflows have sufficient isolation but are excluded to avoid
an error-prone dual schema. Windlass's trusted build and signing boundary is precisely a SHA-pinned
reusable workflow, so changing `buildType` to `workflow/v1` would misdescribe the operation being
attested. The upstream `release` object also records non-default release-event payload parameters,
whereas Windlass's `release` records the built release tag identity. Complete adoption is therefore
neither a field-placement alternative nor evidence required by attempt 9, whose rejection did not
name the custom Windlass `buildType` or `builder.id`.

### Scope boundaries for follow-up decisions

This ADR deliberately does not settle:

- relocation or removal of the former builder path/SHA/URI record (planned as ADR 0104);
- the invocation-source `resolvedDependencies` descriptor and the relationship between that
  descriptor and `source.invocation_ref`/`source.invocation_revision` (planned as ADR 0105 or a
  separate deduplication ADR);
- optional GitHub actor fields. The three-field `internalParameters.github` object is the default;
  `actor_id` and `triggering_actor_id` are excluded unless later registry and verifier evidence
  justifies a separate decision;
- acquisition mechanics beyond the standing premise that OIDC-derived construction values are
  checked fail-closed against the issued certificate before mutation. Exact acquisition, extraction,
  comparison, and diagnostic rules are specification work, not competing architectural options here.
