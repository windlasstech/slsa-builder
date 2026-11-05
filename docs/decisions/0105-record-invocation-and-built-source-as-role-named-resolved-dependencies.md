---
parent: Decisions
nav_order: 105
status: accepted
date: 12026-11-06
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0079
  - type: see-also
    target: ADR-0080
  - type: see-also
    target: ADR-0103
  - type: see-also
    target: ADR-0104
---

# Record Invocation and Built Source as Role-Named Resolved Dependencies

## Context and Problem Statement

ADR 0079 separated the built release source from the invocation origin so that a failed release can
be retried from a fixed pipeline while the signed tag content is built. ADR 0080 completed that
model's binding rules: the signed `externalParameters.source.*` and `release.*` fields carry the
built release identity that verifier policy binds to, while the signing certificate's Source
Repository Ref and Source Repository Digest claims authenticate the invocation context against a
signed invocation record. ADR 0103 then redefined `externalParameters.workflow` as the common caller
invocation identity `{ref, repository, path}` and expressly reserved two questions for this
decision: the invocation-source `resolvedDependencies` descriptor, and the relationship between that
descriptor and the conditional `source.invocation_ref`/`source.invocation_revision` pair.

The npm profile's `resolvedDependencies` is a closed, name-keyed set. It currently enumerates the
`lockfile` descriptor, whose URI embeds the built ref's resolved commit; the `node-runtime` and
`runner-image` environment descriptors; and the conditional `package-manager-distribution`
descriptor. No entry identifies the invocation source: the repository content at the ref on which
the run was dispatched, which supplies the workflow logic the platform executes. The invocation
commit appears only in the conditional `source.invocation_revision` field, and the invocation ref
only in `externalParameters.workflow.ref` and, conditionally, `source.invocation_ref`.

Dogfood attempt 9 against the live npm registry exposed the concrete cost of that gap. The
registry's E422 rejection message compared `buildDefinition.resolvedDependencies[0].uri` against
`git+<SourceRepositoryURI>@<SourceRepositoryRef>` and
`buildDefinition.resolvedDependencies[0].digest.gitCommit` against the certificate's Source
Repository Digest — the invocation head commit, not the built tag's peeled commit. The
Windlass-signed statement contained no descriptor that could satisfy either comparison. This
evidence is observational: it proves those two first-entry comparisons, not the registry's full
acceptance predicate, and SLSA v1.2 defines `resolvedDependencies` as an unordered collection, so
the registry's positional check is a compatibility behavior to accommodate rather than a normative
rule to adopt.

SLSA v1.2 and the in-toto attestation specification supply the semantic frame. External parameters
are the top-level inputs actually passed to the build platform, while metadata about the artifacts
those parameters reference — particularly their digests — belongs in `resolvedDependencies`. The
in-toto `ResourceDescriptor` identifies a resource by `uri`, `digest`, `name`, and optional
annotations, and its DigestSet guidance expressly supports `gitCommit` as the conventional digest
for Git objects. The community-maintained GitHub Actions `workflow/v1` build type, which describes
top-level workflows rather than reusable-workflow execution, records the workflow repository at its
invocation ref as a `resolvedDependencies` entry with a `gitCommit` digest. Under that semantic
reading, both source roles are resolved Git resources the build depends on: the invocation source
supplies the executed workflow logic, and the built source supplies the checked-out, packed, and
attested content. Neither role currently has a descriptor of its own, and the built commit's only
dependency-side presence is embedded in the lockfile URI.

How should the JS/TS npm producer profile represent the invocation source and the built source in
`resolvedDependencies`, so that the observed npm registry comparisons, SLSA v1.2 and in-toto
semantics, and every existing Windlass identity and verification binding all hold?

## Decision Drivers

- Satisfy the observed registry compatibility checks: the first serialized descriptor must identify
  the caller repository at the invocation ref, with `digest.gitCommit` equal to the
  certificate-authenticated invocation commit. Treat this as evidence about npm's validator, not as
  a normative SLSA ordering rule.
- Follow SLSA v1.2 and in-toto semantics: resolved source commits are digest metadata of source
  artifacts and belong in `resolvedDependencies`; descriptor semantics follow the in-toto
  `ResourceDescriptor` contract, with `gitCommit` as the Git digest convention.
- Keep the three source-related roles distinct and honestly labeled: the invocation source (the
  workflow logic origin), the built source (the attested release content), and the selected lockfile
  (the install constraint). No descriptor may borrow another role's identity.
- Preserve every existing binding during coexistence: ADR 0080's release-policy binding to signed
  `externalParameters.source.*`/`release.*` fields, the certificate-to-invocation-record
  comparisons, the ADR 0103 `workflow.*` invocation identity, the lockfile descriptor contract, and
  the digest-rebound handoffs all remain in force until a separate consolidation decision.
- Keep cardinality and names stable. Whether invocation and built identities coincide must not
  change which descriptors exist; conditional presence would make the closed set depend on whether
  two semantic roles happen to resolve to the same commit.
- Keep verification semantics unordered and name-keyed while making the producer's serialization
  order deterministic and fixed before signing. RFC 8785 canonicalization does not order arrays, so
  array order is part of the signed bytes.
- Keep this decision atomic. Relocation or removal of existing `externalParameters.source` members,
  invocation event metadata placement, and migration of ADR 0080's policy-binding field locations
  are follow-up decisions; this ADR adds descriptors and equality invariants without deleting any
  existing signed record.

## Considered Options

- Add an invocation-source descriptor only.
- Add role-named invocation-source and built-source descriptors.
- Add an invocation-source descriptor and a conditional built-source descriptor.
- Emit one source descriptor per distinct resolved source resource.
- Carry the invocation identity on the existing lockfile descriptor.
- Replace the lockfile descriptor with a built-source descriptor.

## Decision Outcome

Chosen option: "Add role-named invocation-source and built-source descriptors", because the
invocation source and the built source are distinct roles that can legitimately resolve to different
commits, both are resolved Git resources the build depends on under SLSA v1.2's model, and naming
both roles with stable cardinality satisfies the observed registry checks while keeping every
existing signed record and verification binding intact during coexistence.

### The two role-named descriptors

The npm producer's `resolvedDependencies` gains exactly two entries with these closed shapes:

```json
{
  "name": "invocation-source",
  "uri": "git+https://github.com/example/project@refs/heads/main",
  "digest": { "gitCommit": "89abcdef0123456789abcdef0123456789abcdef" }
}
```

```json
{
  "name": "built-source",
  "uri": "git+https://github.com/example/project@refs/tags/v1.2.3",
  "digest": { "gitCommit": "0123456789abcdef0123456789abcdef01234567" }
}
```

- `invocation-source` identifies the caller repository content at the invocation ref. Its `uri` is
  exactly `git+<externalParameters.workflow.repository>@<externalParameters.workflow.ref>`, with no
  path fragment. Its `digest.gitCommit` is the invocation head commit: the platform-authenticated
  value the signing certificate records as Source Repository Digest.
- `built-source` identifies the built release content. Its `uri` is exactly
  `git+<externalParameters.source.repository>@<externalParameters.source.ref>`, with no path
  fragment. Its `digest.gitCommit` is the built ref's resolved commit, equal to
  `externalParameters.source.revision`.
- Both descriptors carry exactly the three members `name`, `uri`, and `digest`, and `digest` carries
  exactly the one member `gitCommit`, a full lowercase hexadecimal Git commit ID. Under the current
  contract the producer emits no annotations and consumers reject them under the closed-schema rule.
  That is not a judgment that annotations are semantically unsuitable for these descriptors: an
  annotation key may be admitted when a later decision deliberately extends this closed shape, at
  which point the specification defines the permitted keys and values. The anticipated relocation of
  the `ref_type` classifier to a descriptor annotation is one such candidate, reserved for the
  source-field consolidation follow-up.
- Each descriptor derives its identity from its own role's acquisition source: `invocation-source`
  from the authenticated invocation context, and `built-source` from the verified built-source
  resolution. Neither descriptor substitutes the other role's values when the two roles differ.
  Their `uri` and `digest` values are identical when both identities coincide.

### Cardinality, names, and the closed set

The npm profile's closed `resolvedDependencies` set becomes:

- npm selections: exactly five descriptors — `invocation-source`, `built-source`, `lockfile`,
  `node-runtime`, and `runner-image`;
- pnpm or Yarn selections: exactly six descriptors — those five plus `package-manager-distribution`.

Both source descriptors are required on every run, including tag-push runs where invocation and
built identities coincide. On such a run the two descriptors carry identical `uri` and `digest`
values under their distinct role names; neither is omitted, because descriptor presence encodes role
coverage, not value novelty.

Consumers continue to treat `resolvedDependencies` as an unordered, name-keyed set and select
descriptors by `name`, never by array position. Duplicate descriptor names are rejected before any
name-keyed lookup, so no consumer can observe a first-match/last-match disagreement. Unknown names
remain rejected under the existing closed-set rule.

### Producer serialization order

The npm producer serializes `resolvedDependencies` in this deterministic order: `invocation-source`,
`built-source`, `lockfile`, `node-runtime`, the conditional `package-manager-distribution`, then
`runner-image`.

Placing `invocation-source` first is an npm emission compatibility rule grounded in the attempt-9
registry observation; it is not a semantic property of the set. The serialization order is fixed
before signing, because RFC 8785 canonicalization does not reorder arrays and any post-hoc
reordering would change the signed bytes. Semantic verification remains name-keyed and unordered: a
permutation of a valid set is semantically valid, and only the npm emission path enforces the
index-0 rule.

### Equality invariants during coexistence

Until the follow-up consolidation decision, the existing signed records are retained, and the new
descriptors join them under these invariants:

| Left side                                  | Right side                                                                              |
| ------------------------------------------ | --------------------------------------------------------------------------------------- |
| `invocation-source` URI                    | `git+<workflow.repository>@<workflow.ref>` byte-for-byte                                |
| `invocation-source` `digest.gitCommit`     | the signing certificate's Source Repository Digest                                      |
| `source.invocation_ref`, when present      | `workflow.ref`, and therefore the invocation-source URI's ref component                 |
| `source.invocation_revision`, when present | `invocation-source` `digest.gitCommit`                                                  |
| `built-source` URI                         | `git+<source.repository>@<source.ref>` byte-for-byte                                    |
| `built-source` `digest.gitCommit`          | `source.revision`, and therefore the commit embedded in the `lockfile` URI              |
| `workflow.repository`                      | `source.repository`, consistent with ADR 0080's context-independent repository identity |

When `source.input_ref` is absent, the invocation ref is the built ref and the invocation commit is
the built commit, so the two source descriptors carry identical `uri` and `digest` values; the
invariants reduce accordingly. The retained conditional-presence rule for
`source.invocation_ref`/`source.invocation_revision` is unchanged by this ADR: that pair is present
exactly when `source.input_ref` is present, and a present `input_ref` never justifies omitting the
pair, either descriptor, or any equality — even when the caller-supplied tag and the invocation ref
resolve to the same commit.

These invariants cross-check representations of the same role. They do not authorize substituting
one role's evidence for another's: the certificate comparison proves the invocation binding, the
signed `source.*` fields remain the release-policy binding surface, and no verifier may treat the
first serialized descriptor or "the Git descriptor" as the built-source policy anchor.

### Preserved contracts and boundaries

- The `lockfile`, `node-runtime`, `package-manager-distribution`, and `runner-image` descriptors are
  unchanged in shape and meaning. In particular, the `lockfile` URI continues to embed the built
  revision and the percent-encoded lockfile path; the new `built-source` descriptor is an additional
  role record, not a replacement for any existing entry.
- ADR 0080's binding model is unchanged: release policy binds to the signed
  `externalParameters.source.*` and `release.*` fields, and certificate source claims authenticate
  the invocation context. Migrating those policy bindings onto the descriptors is a follow-up
  decision, not part of this one.
- ADR 0103's `externalParameters.workflow` group and ADR 0104's canonical `runDetails.builder.id`
  model are unchanged. This ADR adds no builder descriptor and reopens neither decision.
- Producer construction follows each role's own acquisition source, consistent with ADR 0103's
  acquisition doctrine. `invocation-source` is constructed from platform-authenticated invocation
  claims, and its repository, ref, and commit are compared fail-closed with the corresponding
  certificate claims before any registry or release mutation. `built-source` is constructed from the
  trusted builder's validated source selection and verified resolution, with equality enforced
  against the retained `source.*` fields and the lockfile descriptor's embedded commit; per ADR
  0080, the built ref and revision are builder-attested rather than platform-claim-bound, so they
  must not be required to match the certificate's invocation claims when `source.input_ref` is
  present. Repository identity remains certificate-bound in both roles. Unvalidated caller-provided
  identity assertions are never a source for either descriptor; the caller's `source-ref` selector
  is used only after the existing validation and resolution guards accept it.
- The custom Windlass buildType, exact-byte signing, subject digest rules, and digest-rebound
  handoffs are unchanged. Consistent with ADR 0103's pre-release policy, there is no buildType
  version migration and no old-bundle compatibility path; conformance fixtures follow the revised
  contract.
- The index-0 serialization rule is npm-specific compatibility behavior. Future profiles may adopt
  the role-named source descriptors where semantically appropriate, but they do not inherit a
  registry-specific ordering requirement.

### Consequences

- Good, because the observed npm registry comparisons become satisfiable with a descriptor whose
  semantics are honest: the invocation source is recorded as the invocation source, not as a
  disguise layered onto an unrelated entry.
- Good, because both source roles gain explicit, digest-anchored records that follow SLSA v1.2's
  division of labor: selectors and interface values in `externalParameters`, resolved artifact
  identities in `resolvedDependencies`.
- Good, because the built commit's dependency-side presence is no longer implicit in a lockfile URI
  fragment convention; the `built-source` descriptor states it directly while the lockfile contract
  is untouched.
- Good, because coexistence with equality invariants lets this decision land without migrating
  verification policy, fixtures' source-field expectations, or ADR 0080's binding model in the same
  step.
- Neutral, because the signed statement grows by two descriptors, and the npm and pnpm/Yarn
  cardinalities, fixtures, and closed-set validation tables all change together.
- Bad, because source identity is temporarily represented in more than one signed location, and
  every pairing must carry a defined equality until the consolidation follow-up retires one side.
- Bad, because strict verifiers built against the pre-change closed set will reject the new
  descriptors; the pre-release contract makes this acceptable, but historical dogfood bundles are
  not conforming inputs afterward.
- Bad, because a controlled dogfood attempt remains the only definitive test of registry acceptance.
  The attempt-9 evidence proves two first-entry comparisons, not the full acceptance predicate, and
  a conforming statement could still surface a further rejection family.

## Confirmation

This decision is confirmed when specifications, test-first conformance cases, exact-byte fixtures,
and controlled live evidence demonstrate all of the following:

- the common provenance contract and the npm provenance and publish specification define both
  descriptors with their exact closed shapes, names, digest member, and manager-dependent
  cardinalities (five for npm, six for pnpm or Yarn), and the closed-set validation rejects unknown
  and duplicate descriptor names before any name-keyed lookup;
- producer construction constructs `invocation-source` from platform-authenticated invocation claims
  and compares its repository, ref, and commit fail-closed with the corresponding certificate claims
  before any registry or release mutation, while `built-source` is constructed from the validated
  source selection and verified resolution with equality enforced against the retained `source.*`
  fields and the lockfile descriptor's embedded commit, and is not required to match the
  certificate's invocation ref or digest when `source.input_ref` is present;
- accepted fixtures cover a tag-push run, where the two source descriptors carry identical `uri` and
  `digest` values under their distinct names, and an ADR 0079 dispatch retry, where
  `invocation-source` records the dispatch ref and head commit while `built-source` records the
  release tag and its resolved commit; both cases retain the `source.invocation_*` pair exactly when
  `source.input_ref` is present, including the edge cases where a present `input_ref` resolves to
  the invocation commit under a different ref and where all identities fully coincide;
- negative fixtures reject an invocation-source URI or digest that disagrees with
  `externalParameters.workflow` or the certificate Source Repository Digest, a built-source URI or
  digest that disagrees with `externalParameters.source.*`, and any disagreement between present
  `source.invocation_*` members and the invocation-source descriptor;
- serialization fixtures prove that `invocation-source` occupies index 0 in the signed bytes and
  that the order is fixed before signing, while semantic validation of a descriptor set selected by
  name accepts any permutation; reordering an already signed payload remains a byte mismatch and
  fails;
- verification evidence proves that release policy continues to bind to the signed
  `externalParameters.source.*` and `release.*` fields, and that no verification path treats
  descriptor position or a certificate invocation claim as built-source policy evidence;
- the `lockfile` descriptor's existing shape and built-revision binding pass unchanged, and its
  URI's embedded commit equals `built-source` `digest.gitCommit` in every accepted fixture;
- scope review confirms that no `externalParameters.source` member was removed or relocated, that no
  invocation event metadata placement was decided, and that ADR 0080's policy-binding field
  locations are unchanged by this migration;
- a subsequent controlled dogfood attempt submits a conforming bundle and records the registry's
  response in `docs/dogfood/`. Registry silence about the new descriptors is not treated as proof of
  full acceptance; any remaining rejection family stays fail-closed and is recorded.

## Pros and Cons of the Options

### Add an invocation-source descriptor only

Record the invocation source as a single new descriptor and leave the built commit's dependency-side
presence embedded in the lockfile URI.

- Good, because it is the smallest closed-set expansion that satisfies the observed registry checks,
  and it leaves the existing built-source policy binding and lockfile contract untouched.
- Good, because the built release identity is already signed in `externalParameters.source.*`, so no
  additional record is required for verification to keep working exactly as ADR 0080 defined.
- Bad, because the built commit's only presence in `resolvedDependencies` remains embedded in the
  lockfile descriptor's URI, a convention a reader must know rather than a role a consumer can
  select by name.
- Bad, because under SLSA v1.2's semantic reading the built source is itself a resolved dependency
  of the build — the content that was checked out, built, and packed — and omitting its descriptor
  leaves the dependency inventory less honest than the parameter list.

### Add role-named invocation-source and built-source descriptors

Record both source roles as mandatory, name-keyed descriptors with stable cardinality, as selected
above.

- Good, because both resolved Git resources the build depends on are identified explicitly, with
  digests, under names that state their roles.
- Good, because stable cardinality keeps the closed set independent of whether the two identities
  coincide, avoiding conditional-presence rules that change shape between tag-push and
  dispatch-retry runs.
- Good, because the attempt-9 registry checks are satisfied by the descriptor's own semantics, and
  the built-source descriptor gives the ADR 0080 built identity its natural dependency-side home
  without disturbing the lockfile contract.
- Bad, because tag-push runs record the same commit twice under two names, an intentional redundancy
  that prefers role stability over value deduplication.
- Bad, because the statement grows and every equality pairing must be specified and tested until the
  consolidation follow-up retires the duplicate representations.

### Add an invocation-source descriptor and a conditional built-source descriptor

Always emit the invocation-source descriptor, and add the built-source descriptor only when the
invocation and built identities differ.

- Good, because it avoids recording the same commit twice on tag-push runs.
- Bad, because descriptor absence gains meaning, so every consumer must implement the conditional
  rule and the "different in what sense?" comparison before it can interpret the set.
- Bad, because conditional cardinality makes the closed set depend on runtime values, complicating
  fixtures, validation tables, and the equality invariants for no additional assurance.

### Emit one source descriptor per distinct resolved source resource

Coalesce the two roles into one descriptor when the identities coincide and emit two when they
differ, with a role convention distinguishing the cases.

- Good, because it mirrors how a deduplicating dependency inventory would model identical resources.
- Bad, because it requires a new role-naming or role-annotation convention inside the name-keyed
  model, turning identity equivalence into policy and multiplying the validation matrix.
- Bad, because the complexity buys nothing here: the two roles answer different verification
  questions even when they name the same commit.

### Carry the invocation identity on the existing lockfile descriptor

Keep the descriptor count unchanged by pointing the lockfile entry's URI at the invocation ref and
commit, preserving the actual lockfile location through another field or relationship.

- Good, because it satisfies the observed positional check without expanding the closed set.
- Bad, because the descriptor's primary locator would no longer identify the lockfile's actual
  built-revision location, misrepresenting what the entry authenticates.
- Bad, because relocating the lockfile's own binding into a side channel is a semantic redesign of
  the lockfile contract, far beyond what the registry evidence requires.

### Replace the lockfile descriptor with a built-source descriptor

Add the invocation-source descriptor and replace the lockfile entry with a built-source descriptor,
keeping the net descriptor count one lower than the chosen option.

- Good, because the built source gains an explicit role descriptor with only one net new entry.
- Bad, because it abolishes the dedicated lockfile dependency record — the selected lockfile's path,
  byte digest, and selection provenance — which the new descriptor does not reproduce, and every
  consumer of that contract must migrate.
- Bad, because the change is far broader than the demonstrated compatibility need and discards a
  verified binding this decision has no reason to disturb.

## More Information

Local decisions, contracts, and evidence:

- [ADR 0079](0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md):
  the built-source/invocation split and the caller-specified `source-ref` retry model.
- [ADR 0080](0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md):
  release-policy binding to signed built-source fields and certificate claims as invocation-context
  evidence.
- [ADR 0103](0103-define-external-workflow-parameters-as-common-invocation-identity.md): the common
  `externalParameters.workflow` invocation identity, the acquisition doctrine, and the reservation
  of this decision's scope.
- [ADR 0104](0104-remove-redundant-external-parameter-builder-record.md): the canonical
  `runDetails.builder.id` model this decision does not touch.
- [ADR 0070](0070-record-package-manager-distributions-and-runner-image-in-resolved-dependencies.md)
  and [ADR 0099](0099-record-the-digest-verified-nodejs-runtime-in-resolveddependencies.md): the
  existing descriptor conventions this decision extends.
- [SLSA provenance v1](../architecture/slsa-provenance-v1.md),
  [JS/TS npm provenance and publish](../architecture/js-ts-npm-provenance-publish.md), and
  [verification policy and fixtures](../architecture/verification-policy-and-fixtures.md): the
  closed-set schema, equality rules, and fixture contracts to migrate after acceptance.
- [Dogfood attempt 9](../dogfood/npm-m1.md) and
  [npm registry provenance requirements](../dogfood/npm-registry-provenance-requirements.md): the
  observed `resolvedDependencies[0]` URI and `gitCommit` comparisons and the limits of that
  evidence.
- Current implementation migration evidence:
  [npm dependency construction](../../internal/npmprofile/workflow_metadata.go) and
  [npm statement validation](../../internal/npmprofile/provenance_validate.go). These references
  explain the current closed set; they do not prescribe the future implementation.

Primary external sources:

- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance), especially
  [BuildDefinition](https://slsa.dev/spec/v1.2/build-provenance#builddefinition): the unordered
  `resolvedDependencies` collection, the division between interface parameters and resolved artifact
  metadata, and external versus internal parameter classification.
- [in-toto ResourceDescriptor](https://github.com/in-toto/attestation/blob/main/spec/v1/resource_descriptor.md)
  and [DigestSet](https://github.com/in-toto/attestation/blob/main/spec/v1/digest_set.md):
  descriptor field semantics and the `gitCommit` convention for Git objects.
- [GitHub Actions `workflow/v1` README](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/README.md)
  and
  [example.json](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/example.json):
  the community build type's workflow-repository descriptor with a resolved `gitCommit`.

### What the attempt-9 evidence does and does not establish

The registry's rejection message proves two comparisons against the first serialized descriptor: its
`uri` against the caller repository at the invocation ref, and its `digest.gitCommit` against the
certificate's invocation commit. It does not establish the registry's full acceptance predicate, a
normative ordering rule, a restriction on descriptor names, or a limit on the number of Git
descriptors. SLSA v1.2 defines the collection as unordered, and the `workflow/v1` example's array
order is illustrative, not normative. The index-0 serialization rule adopted here is a compatibility
accommodation to observed registry behavior, scoped to npm emission; semantic verification remains
name-keyed and unordered.

### Deferred scope

This ADR deliberately does not decide the relocation or removal of any existing
`externalParameters.source` member — including `revision`, `ref_type`, `invocation_ref`,
`invocation_revision`, `repository`, or `ref` — nor the disposition of `source.event_name`, nor any
migration of ADR 0080's policy bindings onto the descriptors. (The `ref_type` relocation includes
its candidate re-expression as a descriptor annotation, per the extension path above.) Those
consolidations are follow-up decisions; until they land, the existing fields remain in force under
the equality invariants above. This ADR also does not reopen ADR 0103's standing pre-release policy:
no buildType version migration and no old-bundle compatibility path are introduced here. Indexes,
traceability tables, specifications, implementation, and fixtures follow only after the operator's
explicit acceptance.
