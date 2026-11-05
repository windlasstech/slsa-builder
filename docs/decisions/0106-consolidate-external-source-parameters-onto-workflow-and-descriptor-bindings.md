---
parent: Decisions
nav_order: 106
status: accepted
date: 12026-11-06
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0079
  - type: partially-supersedes
    target: ADR-0080
    scope:
      "the signed field locations of the built-source policy binding
      (externalParameters.source.repository/ref/revision) and the certificate-to-invocation-record
      comparison fields (source.invocation_ref/invocation_revision and the
      source.ref/source.revision fallback); the binding meanings are preserved, with repository
      authorization re-pointed to externalParameters.workflow.repository, release ref/commit
      expectations to the built-source descriptor, and certificate invocation comparisons to
      workflow.ref and the invocation-source descriptor"
  - type: see-also
    target: ADR-0103
  - type: see-also
    target: ADR-0104
  - type: partially-supersedes
    target: ADR-0105
    scope:
      "the coexistence equality invariants that reference the externalParameters.source members
      removed here (built-source URI/digest equalities with source.repository/ref/revision and the
      conditional source.invocation_* rows); the invocation-source descriptor shape, descriptor
      cardinality, serialization order, and invocation-side invariants remain in force, while the
      built-source descriptor's closed shape is extended here with a required annotations.ref_type
      member"
---

# Consolidate External Source Parameters onto Workflow and Descriptor Bindings

## Context and Problem Statement

ADR 0105 added the mandatory role-named `invocation-source` and `built-source` descriptors to the
npm profile's closed `resolvedDependencies` set. It deliberately left every existing
`externalParameters.source` member in place and cross-checked the new descriptors against them with
equality invariants, reserving the consolidation of those members for this follow-up decision.

The retained `source` group currently carries `repository`, `ref`, `revision`, `event_name`, and
`ref_type` as required members, plus the optional `input_ref`, `invocation_ref`, and
`invocation_revision`. Measured against SLSA v1.2's parameter model — parameters are the top-level
inputs actually passed through the build platform's interface, while metadata about the artifacts
those inputs reference, particularly their resolved digests, belongs in `resolvedDependencies` —
most of these members are not independent inputs:

- `source.repository` is always identical to `externalParameters.workflow.repository`, because the
  built repository and the invocation repository are the same caller repository under ADR 0080's
  context-independent repository identity rule.
- `source.ref` is an effective selector computed from the caller's explicit `source-ref` input when
  supplied and the invocation ref otherwise; it is derived, not independently passed.
- `source.revision` is the resolution output of the built ref and is now recorded as `built-source`
  `digest.gitCommit`.
- `source.ref_type` is a classifier derivable from the built ref's prefix.
- `source.invocation_ref` duplicates `workflow.ref`, and `source.invocation_revision` duplicates the
  `invocation-source` descriptor's digest.
- `source.input_ref` is the genuine exception: it records the caller-supplied `source-ref` value, a
  full `refs/tags/<tag-name>` ref exactly as passed through the workflow interface.

The redundancy has real verification consequences, not just schema aesthetics. ADR 0080 binds the
verifier's release policy — canonical repository, release tag ref, and commit SHA — to the signed
`source.repository`/`source.ref`/`source.revision` fields, and binds the certificate's Source
Repository Ref and Digest claims to `source.invocation_ref`/`source.invocation_revision` when
present, falling back to `source.ref`/`source.revision` otherwise. Removing or relocating these
fields therefore requires re-pointing every one of those comparisons, without weakening any of them.
The policy schema's own expectation field names (`source.repository_uri`, `source.digest`,
`source.ref`) are policy-side vocabulary and do not change; what moves is the statement-side
comparison target.

The coexistence state also cannot be the resting point. ADR 0105's equality invariants reference
members whose values are fully determined by other signed records, so each such invariant is a
permanent drift surface rather than an independent check. Meanwhile `source.input_ref`'s name only
made sense in contrast to the old required `source.ref`; once that field is gone, the `input_`
prefix is redundant with `externalParameters` itself, and the field's value — the full ref as
actually passed by the caller — should keep its exact interface spelling.

How should the npm profile consolidate `externalParameters.source` so that every remaining member is
a genuine external input, every existing verification guarantee is preserved through re-pointed
bindings, and the caller's explicit source selection remains recorded with its interface-faithful
value?

## Decision Drivers

- Follow SLSA v1.2's parameter semantics: retain only top-level external inputs in
  `externalParameters`; keep resolved artifact identities in `resolvedDependencies`; do not store
  values that are fully derived from other recorded inputs.
- Preserve every verification guarantee, changing comparison targets rather than expectations:
  repository authorization, release ref and commit checks, certificate-to-invocation binding, the
  lockfile's built-revision binding, and the numeric repository/owner identifier bindings must all
  remain enforced fail-closed.
- Keep the caller's explicit source selection as a first-class signed record with its
  interface-faithful value: the full `refs/tags/<tag-name>` form exactly as passed, with the
  existing presence, spelling, and conflict rules.
- Make the derivation of the effective built ref a specified rule that consumers can independently
  recompute and compare, not an informal defaulting expression.
- Admit descriptor annotations only through deliberate closed-shape extension, as ADR 0105
  anticipated; the first such admission is the built ref's classifier.
- Keep this decision atomic. Invocation event metadata placement is a separate follow-up decision,
  and this ADR does not reopen ADR 0103's standing pre-release policy: no buildType version
  migration and no old-bundle compatibility path.

## Considered Options

- Consolidate the source group to the caller's explicit selector and re-point verification to
  workflow fields and descriptors.
- Remove resolved metadata only, retaining effective source selectors in externalParameters.
- Retain the complete source group with permanent equality invariants.
- Remove the source group entirely, including the caller selector.

## Decision Outcome

Chosen option: "Consolidate the source group to the caller's explicit selector and re-point
verification to workflow fields and descriptors", because the other members are derived values or
resolution outputs rather than independent inputs, every guarantee they supported has a more
authoritative surviving home, and the caller's explicit selector is the only member whose presence
carries information no other signed record contains.

### The consolidated source group

After this decision, `externalParameters.source` contains:

- `ref` (optional): the caller's explicit source selection, recorded exactly as passed through the
  workflow's `source-ref` input — a full `refs/tags/<tag-name>` ref. This is a new definition, not
  the retention of the former required built-ref member: the former `source.ref` and the former
  `source.input_ref` are both removed, and this optional member replaces them with the
  caller-selector meaning.
- `event_name`: unchanged by this ADR. Its classification and any relocation are the separate
  event-metadata follow-up decision, which also owns the group's residual shape afterward.

Every other former member — `repository`, the former required `ref`, `revision`, `ref_type`,
`input_ref`, `invocation_ref`, and `invocation_revision` — is removed from the group, and none is
recreated under another `externalParameters` name.

### Effective built ref derivation

The effective built ref is defined by this rule, which producers apply before constructing the
statement and consumers independently recompute during verification:

1. If `source.ref` is present, the effective built ref is its value; otherwise it is
   `externalParameters.workflow.ref`.
2. `source.ref` is present exactly when the caller supplied a non-empty `source-ref` input. It must
   be a full `refs/tags/<tag-name>` ref in its exact canonical spelling without surrounding
   whitespace; a whitespace-padded, shortened, or otherwise malformed value is rejected, and only
   the exact empty string counts as omission at the input layer.
3. A tag-triggered run means `workflow.ref` begins with `refs/tags/`, including a dispatch on a tag.
   On such a run, a present `source.ref` that differs from `workflow.ref` is rejected, per ADR
   0079's conflict rule; an equal selector is accepted.
4. `built-source.uri` is exactly `git+<workflow.repository>@<effective built ref>`, and
   `built-source.digest.gitCommit` is the effective built ref's verified resolved commit, proven by
   the existing isolated-fetch and peel-to-commit resolution procedure. A consumer that recomputes a
   different effective built ref, or a different URI from it, rejects the statement.
5. ADR 0079's version-tag matching is preserved: the effective built ref must be the packed
   package's version tag, exactly as the existing release guards require.

The effective built ref itself is not stored as a separate field: it is reconstructible from
`source.ref` and `workflow.ref` by this rule, and the `built-source` descriptor is its signed
recording.

### Verification re-binding

Every comparison that previously targeted the removed fields is re-pointed as follows. The policy
schema and certificate claims are unchanged; only the statement-side targets move.

| Verification purpose                                            | Previous statement-side target                                            | New statement-side target                                                                                                                |
| --------------------------------------------------------------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| Repository authorization against policy `source.repository_uri` | `source.repository`                                                       | `externalParameters.workflow.repository`                                                                                                 |
| Repository identity chain                                       | `package.repository` == `source.repository` == observed caller repository | `package.repository` == `workflow.repository` == certificate Source Repository URI == both source descriptors' URI repository components |
| Release tag expectation (policy `source.ref`)                   | `source.ref`, equal to `release.ref`                                      | effective built ref, equal to the `built-source` URI's ref component and to `release.ref`                                                |
| Release commit expectation (policy `source.digest`)             | `source.revision`                                                         | `built-source` `digest.gitCommit`                                                                                                        |
| Certificate Source Repository Ref                               | `source.invocation_ref` when present, else `source.ref`                   | `externalParameters.workflow.ref`, and therefore the `invocation-source` URI's ref component                                             |
| Certificate Source Repository Digest                            | `source.invocation_revision` when present, else `source.revision`         | `invocation-source` `digest.gitCommit`                                                                                                   |

The certificate fallback disappears because the `invocation-source` descriptor is always present
under ADR 0105. The certificate's numeric repository and owner identifier bindings are untouched.
`externalParameters.release.*` retains its existing meaning and continues to identify the release;
the built-source descriptor does not replace it.

### The built ref classifier as a descriptor annotation

The `built-source` descriptor gains its first annotation under ADR 0105's extension path:

```json
{
  "name": "built-source",
  "uri": "git+https://github.com/example/project@refs/tags/v1.2.3",
  "digest": { "gitCommit": "0123456789abcdef0123456789abcdef01234567" },
  "annotations": { "ref_type": "tag" }
}
```

`built-source` now has a closed shape of exactly `name`, `uri`, `digest`, and `annotations`, and
`annotations` is required and contains exactly one key, `ref_type`, whose value is exactly `"tag"`
on the production release path. A missing `annotations` member, a missing `ref_type` key, or any
additional annotation member is rejected, and no other annotation is defined for either source
descriptor by this ADR. Consumers cross-check the annotation against the URI's ref prefix, so the
classifier and the ref cannot disagree. The `invocation-source` descriptor retains its existing
three-member, annotation-free shape; its invocation ref carries no release-class claim.

### Equality invariants after consolidation

The surviving invariants are:

| Left side                              | Right side                                                                |
| -------------------------------------- | ------------------------------------------------------------------------- |
| `invocation-source` URI                | `git+<workflow.repository>@<workflow.ref>` byte-for-byte                  |
| `invocation-source` `digest.gitCommit` | the signing certificate's Source Repository Digest                        |
| `built-source` URI                     | `git+<workflow.repository>@<effective built ref>` per the derivation rule |
| `built-source` `digest.gitCommit`      | the verified resolved commit of the effective built ref                   |
| `built-source` `digest.gitCommit`      | the commit embedded in the `lockfile` descriptor's URI                    |
| `built-source` `annotations.ref_type`  | the built ref's class, consistent with the URI's ref prefix               |
| `release.ref`                          | the effective built ref                                                   |

ADR 0105's coexistence rows that referenced `source.repository`, the former `source.ref`,
`source.revision`, and the conditional `source.invocation_*` members are retired, because those
members no longer exist. When `source.ref` is absent, the effective built ref equals the invocation
ref, so the two source descriptors carry identical `uri` and `digest` values under their distinct
role names, exactly as ADR 0105 specified for coincident identities. That equality is an enforced
check, not an inference: a coincident-identity statement whose descriptors disagree is rejected,
even when the ref strings alone would match.

### Preserved contracts and acquisition doctrine

- Producer acquisition is unchanged in kind: `invocation-source` continues to derive from
  platform-authenticated invocation claims and is compared fail-closed with the certificate before
  any registry or release mutation; `built-source` continues to derive from the validated source
  selection and verified resolution. The caller's `source-ref` selector is used only after the
  existing validation and resolution guards accept it; unvalidated caller assertions remain
  inadmissible as identity evidence.
- ADR 0080's binding meanings are preserved: release policy expresses expectations about the built
  release, and certificate source claims authenticate the invocation context. What changes is where
  each expectation is compared, not what is authorized.
- The lockfile, runtime, runner-image, and conditional distribution descriptors are unchanged, as
  are ADR 0105's `invocation-source` descriptor shape, descriptor cardinalities, duplicate-name
  rejection, and index-0 npm emission order. The only descriptor-shape change is the `built-source`
  annotation extension defined above.
- ADR 0028's canonical builder identity, ADR 0103's `workflow` group, and ADR 0104's builder-record
  removal are untouched. No builder descriptor is added.
- Exact-byte signing, subject digest rules, and digest-rebound handoffs are unchanged. Consistent
  with ADR 0103's pre-release policy, there is no buildType version migration and no old-bundle
  compatibility path; statements carrying the removed members are non-conforming under the revised
  closed schema.

### Consequences

- Good, because every surviving `externalParameters.source` member other than the temporarily
  retained `event_name` — whose classification the event-metadata follow-up owns — is a genuine
  external input, and every resolved or derived value has exactly one signed home: the workflow
  group, the role-named descriptors, or the derivation rule, rather than two or three.
- Good, because verification keeps its full strength while its targets become more explicit: release
  expectations bind to a digest-anchored descriptor, and certificate invocation claims bind to the
  invocation-source descriptor with no conditional fallback.
- Good, because the caller's explicit selection remains visible even when it coincides with the
  invocation ref, preserving the distinction between "the caller chose this ref" and "the run
  happened to be invoked on it".
- Good, because the retired equality invariants eliminate permanent drift surfaces that ADR 0105 had
  to carry only during coexistence.
- Neutral, because the policy schema's expectation names are unchanged while their comparison
  targets move, so the verification policy and fixture specifications must state the new targets
  precisely and the conformance corpus must be regenerated.
- Bad, because this is a breaking statement-schema change for the strict verifier: statements with
  the removed members, and pre-change fixtures, are rejected under the closed schema, with no
  old-bundle compatibility path.
- Bad, because the derivation rule becomes security-relevant specification text: an ambiguous or
  incorrectly implemented defaulting rule would change which content is attested, so it requires
  explicit fixtures for every presence and conflict case.
- Bad, because a controlled dogfood attempt remains necessary to confirm that the consolidated
  statement still satisfies the registry's observed comparisons and exposes no new rejection family.

## Confirmation

This decision is confirmed when specifications, test-first conformance cases, exact-byte fixtures,
and controlled live evidence demonstrate all of the following:

- the common provenance contract and the npm provenance and publish specification define the
  consolidated `source` group — optional `ref` as the caller's full-ref selector, `event_name`
  untouched pending the event-metadata decision — and reject every genuinely removed member under
  the closed schema: `repository`, `revision`, `ref_type`, `input_ref`, and the `invocation_*` pair.
  The `ref` key itself is not forbidden: the former required-field contract is retired, and `ref` is
  validated under its new optional-selector semantics, with producer-side suppliedness enforced
  against the actual workflow input;
- the effective built ref derivation rule is specified with its presence, exact-spelling,
  tag-conflict, and resolution requirements, and fixtures cover: a tag-push run without
  `source.ref`; a dispatch retry with `source.ref`; a present `source.ref` that resolves to the
  invocation commit under a different ref; a tag-triggered run (including a dispatch on a tag) whose
  `source.ref` equals `workflow.ref`, which is accepted; a tag-triggered run whose `source.ref`
  conflicts with `workflow.ref`, which is rejected; and a whitespace-padded or shortened selector,
  which is rejected;
- verification tests prove that policy repository expectations bind to `workflow.repository`,
  release tag expectations bind to the recomputed effective built ref and the `built-source` URI's
  ref component, and release commit expectations bind to `built-source` `digest.gitCommit`, while
  certificate Source Repository Ref and Digest bind to `workflow.ref` and the `invocation-source`
  descriptor with no fallback;
- repository identity tests prove the full chain: `package.repository`, `workflow.repository`, the
  certificate's Source Repository URI, and both descriptors' URI repository components must agree,
  with numeric repository and owner identifier bindings unchanged;
- the `built-source` descriptor's `annotations.ref_type` is required and accepted exactly as `"tag"`
  with its closed key set, and fixtures reject a missing `annotations` member, a missing `ref_type`
  key, unknown annotation members, an annotation that disagrees with the URI's ref prefix, and any
  annotation on `invocation-source`;
- accepted fixtures retain ADR 0105's coincident-identity behavior: when `source.ref` is absent, the
  two source descriptors carry identical `uri` and `digest` values, and the lockfile URI's embedded
  commit equals `built-source` `digest.gitCommit` in every accepted fixture;
- scope review confirms that no event-metadata placement was decided, that ADR 0105's
  `invocation-source` shape, descriptor cardinalities, and index-0 serialization are unchanged apart
  from the `built-source` annotation extension defined here, that no builder record was added, and
  that no old-bundle compatibility path or buildType version migration was introduced;
- a subsequent controlled dogfood attempt submits a consolidated bundle and records the registry's
  response in `docs/dogfood/`, with any remaining rejection family staying fail-closed and recorded
  rather than inferred away.

## Pros and Cons of the Options

### Consolidate the source group to the caller's explicit selector and re-point verification to workflow fields and descriptors

Remove every derived or resolution-output member, keep only the caller's explicit selector and the
untouched event field, and re-point all verification bindings to the workflow group and the
role-named descriptors.

- Good, because it realizes SLSA v1.2's parameter model exactly: inputs in `externalParameters`,
  resolved identities in `resolvedDependencies`, and derived values nowhere.
- Good, because every verification guarantee survives with a more authoritative target, and the
  certificate's invocation comparisons lose their conditional fallback, simplifying the verifier.
- Good, because the derivation rule is explicit and consumer-recomputable, so dropping the effective
  selector field does not drop any check.
- Bad, because it is the largest statement-schema change among the options and forces coordinated
  specification, verifier, fixture, and producer migration with no old-bundle compatibility.
- Bad, because it partially supersedes both ADR 0080's field locations and ADR 0105's coexistence
  invariants, which must be recorded precisely to keep the decision graph truthful.

### Remove resolved metadata only, retaining effective source selectors in externalParameters

Drop `revision`, `ref_type`, and the `invocation_*` pair, but keep `repository` and the effective
`ref` as normalized selector records alongside the caller's selector.

- Good, because the migration is smaller: policy repository and release-tag comparisons could keep
  their existing statement-side targets.
- Bad, because the retained fields are still derived values that duplicate `workflow.repository` and
  the `built-source` URI, so SLSA's top-level-input semantics remain violated.
- Bad, because it preserves the drift surface this decision exists to remove: an effective selector
  stored next to its own inputs must be kept equal forever, with no new assurance in return.

### Retain the complete source group with permanent equality invariants

Make ADR 0105's coexistence state permanent and change nothing in `externalParameters`.

- Good, because no statement-schema migration is needed beyond ADR 0105, and existing fixtures and
  comparisons keep working unchanged.
- Bad, because derived and resolved values remain signed in two or three places, and every equality
  invariant becomes permanent maintenance surface whose only failure mode is drift between redundant
  records.
- Bad, because the schema continues to misclassify inputs and resolved metadata, the semantic defect
  this consolidation is meant to fix.

### Remove the source group entirely, including the caller selector

Eliminate the whole group and rely on `workflow.ref` and the descriptors for all source identity.

- Good, because it is the smallest possible parameter surface.
- Bad, because it destroys the distinction between an explicit caller selection and a defaulted one:
  when a caller supplies `source-ref`, that fact is itself verifier-relevant input that no other
  record preserves, and ADR 0079's retry semantics depend on it.
- Bad, because the derivation rule would lose its explicit-selector branch, so consumers could no
  longer prove that a retry built the tag the caller actually named.

## More Information

Local decisions, contracts, and evidence:

- [ADR 0079](0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md):
  the caller-specified `source-ref` input, its tags-only constraint, and the tag-conflict rule that
  the derivation rule preserves.
- [ADR 0080](0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md):
  the built-source policy binding and certificate invocation-evidence model whose comparison targets
  this decision re-points.
- [ADR 0103](0103-define-external-workflow-parameters-as-common-invocation-identity.md): the common
  `workflow.{ref, repository, path}` invocation identity and the acquisition doctrine.
- [ADR 0104](0104-remove-redundant-external-parameter-builder-record.md): the canonical builder
  identity this decision does not touch.
- [ADR 0105](0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md): the
  role-named descriptors, their coexistence invariants, and the annotation extension path this
  decision uses for `ref_type`.
- [SLSA provenance v1](../architecture/slsa-provenance-v1.md),
  [JS/TS npm provenance and publish](../architecture/js-ts-npm-provenance-publish.md), and
  [verification policy and fixtures](../architecture/verification-policy-and-fixtures.md): the
  closed source-group schema, certificate comparison table, policy binding rows, and fixture
  contracts to migrate after acceptance.
- Current implementation migration evidence:
  [source-ref validation and resolution](../../internal/npmprofile/source_ref.go),
  [npm dependency construction](../../internal/npmprofile/workflow_metadata.go), and
  [npm statement validation](../../internal/npmprofile/provenance_validate.go). These references
  explain the current presence and comparison rules; they do not prescribe the future
  implementation.

Primary external sources:

- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance), especially
  [BuildDefinition](https://slsa.dev/spec/v1.2/build-provenance#builddefinition): top-level external
  inputs, the guidance that resolved artifact digests belong in `resolvedDependencies`, and the
  absence of any requirement that source identity live at particular `externalParameters.source`
  paths.
- [SLSA verifying artifacts](https://slsa.dev/spec/v1.2/verifying-artifacts#step-2-check-expectations):
  the canonical-source-repository expectation that this decision preserves through
  `workflow.repository` and the descriptor bindings.
- [in-toto ResourceDescriptor](https://github.com/in-toto/attestation/blob/main/spec/v1/resource_descriptor.md)
  and [DigestSet](https://github.com/in-toto/attestation/blob/main/spec/v1/digest_set.md):
  descriptor annotation and `gitCommit` semantics used by the consolidated representation.

### Deferred scope

This ADR deliberately does not decide the classification or relocation of `source.event_name` or the
source group's residual shape after that decision; both belong to the event-metadata follow-up. It
defines no annotation key other than `built-source` `annotations.ref_type`, and any further
annotation admission requires its own deliberate closed-shape extension. It does not change the
workflow `source-ref` input's full-ref interface form, and it does not reopen ADR 0103's standing
pre-release policy: no buildType version migration and no old-bundle compatibility path are
introduced here. Indexes, traceability tables, specifications, implementation, and fixtures follow
only after the operator's explicit acceptance.
