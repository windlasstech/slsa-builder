---
parent: Decisions
nav_order: 107
status: accepted
date: 12026-11-06
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0026
  - type: see-also
    target: ADR-0079
  - type: see-also
    target: ADR-0103
  - type: see-also
    target: ADR-0106
  - type: see-also
    target: ADR-0109
---

# Record Invocation Event Metadata in internalParameters.github Only

## Context and Problem Statement

ADR 0106 consolidated `externalParameters.source` down to the caller's explicit optional `ref`
selector while deliberately leaving `event_name` untouched, reserving two questions for this
decision: the classification and placement of invocation event metadata, and the source group's
residual shape once `event_name` is removed.

The invocation event — `push` on a release tag, or a constrained `workflow_dispatch` — is currently
recorded as the required `source.event_name` member. That placement predates the registry evidence.
Dogfood attempt 9 established that the npm registry requires `internalParameters.github.event_name`
and compares it against the signing certificate's Build Trigger claim, alongside
`internalParameters.github.repository_id` and `repository_owner_id`. ADR 0103 already carries the
standing premise that this three-field closed `internalParameters.github` object is constructed from
GitHub-issued OIDC claims before signing and compared fail-closed with the issued certificate before
any registry or release mutation, with actor fields excluded unless later evidence justifies them.
The registry-required location is therefore fixed; what remains open is whether the
externalParameters copy should survive alongside it.

SLSA v1.2 supplies the classification frame. `externalParameters` are the external interface to the
build — the values actually passed in through the build platform's interface — while
`internalParameters` are set internally by the platform and need not be verified downstream. The
specification also states that the same field name should not be used in both parameter objects. The
invocation event name is not a value the caller passes through the workflow's input interface: it is
the platform's record of how the run was initiated, authenticated by the certificate's Build Trigger
claim. The reference implementations agree. The community GitHub Actions `workflow/v1` build type
records `event_name` only in `internalParameters.github`, and slsa-github-generator documented the
event name in builder-controlled `invocation.environment` data in its v0.2 format; SLSA's own
v0.2-to-v1 migration guidance maps that environment record to `internalParameters`. Neither
precedent duplicates the event name into the external interface.

Once `source.event_name` is removed, the `source` group contains only the optional caller-selector
`ref`. Whether that group should always be emitted, possibly as an empty object, or only when an
explicit selector exists, determines whether group presence itself carries meaning.

Where should invocation event metadata live in the signed statement, and what is the residual shape
of `externalParameters.source` once that metadata has a single canonical home?

## Decision Drivers

- Satisfy the observed npm registry requirement: `internalParameters.github.event_name` must exist
  and match the certificate's Build Trigger claim. Any option that omits it is not viable for npm
  publication.
- Follow SLSA v1.2 semantics: record values actually passed through the build interface in
  `externalParameters`, platform-recorded invocation facts in `internalParameters`, and avoid using
  the same field name in both parameter objects.
- Preserve verification strength through the ADR 0103 premise: the event value is constructed from
  GitHub-issued OIDC claims and compared fail-closed with the issued certificate before any registry
  or release mutation, so the internal placement does not make the value unverified in this
  project's model.
- Preserve the supported-event admission policy itself. Tag-push and constrained dispatch releases
  remain the only supported caller patterns; this decision changes where the event is recorded and
  checked, not which events are admitted.
- Keep the signed schema minimal and each recorded value single-homed, avoiding permanent equality
  invariants between duplicate representations.
- Make the residual `source` group's presence meaningful: it should record the caller's explicit
  source selection, not exist as an empty container.

## Considered Options

- Move event metadata to `internalParameters.github` as the sole record, and emit the `source` group
  only when an explicit selector is present.
- Move event metadata to `internalParameters.github` as the sole record, but always emit the
  `source` group, allowing an empty object.
- Keep event metadata in both `externalParameters.source` and `internalParameters.github` with an
  equality invariant.
- Keep event metadata only in `externalParameters.source`.
- Move event metadata into `externalParameters.workflow`.

## Decision Outcome

Chosen option: "Move event metadata to `internalParameters.github` as the sole record, and emit the
`source` group only when an explicit selector is present", because the event name is a
platform-recorded invocation fact rather than an interface input, the registry requires the internal
location, duplicating the field would violate SLSA's own guidance without adding assurance, and a
group emitted only for an explicit selector makes the caller's choice visible by its presence.

### Event metadata classification and single record

`internalParameters.github.event_name` is the sole signed statement field recording the invocation
event. The `source.event_name` member is removed and is not recreated under any other
`externalParameters` name. (The signing certificate independently records Build Trigger; this
decision concerns the statement's fields.)

- The value continues to be constructed from GitHub-issued OIDC claims before signing and compared
  fail-closed with the issued certificate's Build Trigger claim before any registry or release
  mutation, per ADR 0103's standing premise. SLSA's statement that internal parameters need not be
  verified downstream does not weaken this project's comparison, which remains mandatory.
- The supported-event admission policy is unchanged in substance: production releases remain limited
  to a tag `push` or a constrained `workflow_dispatch`. The statement-side checks and fixtures that
  previously evaluated `source.event_name` now evaluate `internalParameters.github.event_name`, in
  producer-side validation and downstream verification alike. A certificate-authenticated event can
  still be an unsupported event, so the certificate comparison and the admission check remain
  separate, both required checks.
- The closed three-field `internalParameters.github` object (`event_name`, `repository_id`,
  `repository_owner_id`) is ratified as this project's placement decision rather than a borrowed
  premise. Actor fields such as `actor_id` and `triggering_actor_id` remain excluded under ADR
  0103's standing rule unless later registry or verifier evidence justifies a separate decision.
- No duplication with an `externalParameters` member of the same or an aliased name is introduced,
  honoring SLSA v1.2's guidance that the same field name should not appear in both parameter
  objects.

### Residual source group shape

After the removal of `source.event_name`, the `source` group is optional and, when present, contains
exactly one required member, `ref`, with the ADR 0106 caller-selector semantics.

- The `source` group is emitted exactly when the caller supplied a non-empty `source-ref` input —
  that is, exactly when `ref` is present. When the caller did not supply the input, the group is
  omitted entirely; an empty `source: {}` object is not emitted.
- Group presence therefore records that the caller made an explicit source selection, and its
  absence records defaulting. ADR 0106's effective built ref derivation rule is unchanged:
  `source.ref` when present, otherwise `workflow.ref`. The rule reads identically whether the group
  is omitted or empty, so omission removes no information.
- The registry's observed comparisons do not inspect the `source` group, so conditional emission
  introduces no registry absence risk of the kind ADR 0103 addressed for the required `workflow`
  fields. This is grounded in positive evidence, not inference from silence: npm's own
  auto-generated provenance carries no `source` group at all, and the registry accepted exactly such
  a statement in dogfood attempt 4.

### Preserved contracts

- ADR 0106's consolidated source selector, effective built ref derivation, verification re-binding
  to workflow fields and descriptors, and the `built-source` `annotations.ref_type` member are
  unchanged.
- ADR 0105's descriptor shapes, cardinalities, and index-0 npm emission order are unchanged, as are
  ADR 0028's canonical builder identity and ADR 0104's builder-record removal.
- The certificate's invocation bindings — Source Repository Ref and Digest against `workflow.ref`
  and the `invocation-source` descriptor, and the numeric repository and owner identifiers against
  policy — are unchanged. Build Trigger now binds solely to `internalParameters.github.event_name`.
- Exact-byte signing, subject digest rules, and digest-rebound handoffs are unchanged. Consistent
  with ADR 0103's pre-release policy, there is no buildType version migration and no old-bundle
  compatibility path; statements carrying `source.event_name` are non-conforming under the revised
  closed schema.

### Consequences

- Good, because the invocation event has exactly one signed home, in the location the registry
  checks, and the external parameter surface no longer carries a value that was never an interface
  input.
- Good, because the `source` group's presence now carries meaning by itself: a present group means
  the caller explicitly selected a source ref, and an absent group means the effective built ref
  defaulted to the invocation ref.
- Good, because no new equality invariant is needed between duplicate event records, avoiding the
  drift surface that dual placement would have created.
- Neutral, because the supported-event admission checks, certificate Build Trigger comparison, and
  their fixtures must be re-pointed to `internalParameters.github.event_name`, with the policy's
  substance unchanged.
- Bad, because statements carrying `source.event_name` become non-conforming under the closed
  schema, so the pre-release fixtures and goldens must be regenerated with no old-bundle
  compatibility path.
- Bad, because a controlled dogfood attempt remains necessary to confirm that the revised statement
  still satisfies the registry's observed comparisons and exposes no new rejection family.

## Confirmation

This decision is confirmed when specifications, test-first conformance cases, exact-byte fixtures,
and controlled live evidence demonstrate all of the following:

- the common provenance contract and the npm provenance and publish specification record the
  invocation event only as `internalParameters.github.event_name`, reject `source.event_name` under
  the closed schema, and keep the `internalParameters.github` object closed to its three fields with
  no actor members;
- the `source` group is emitted exactly when the caller supplied a non-empty `source-ref` input, and
  fixtures prove both directions: a group present with the exact full-ref selector when the input is
  supplied, and a group omitted entirely — never an empty object — when it is not;
- ADR 0106's derivation rule passes unchanged with the group omitted, and coincident-identity
  behavior is preserved: when no selector is supplied, the effective built ref equals the invocation
  ref and the two source descriptors carry identical `uri` and `digest` values;
- supported-event fixtures accept a tag `push` and a constrained `workflow_dispatch` and reject an
  unsupported event, all evaluated against `internalParameters.github.event_name`;
- verification tests prove that the certificate's Build Trigger claim is compared fail-closed with
  `internalParameters.github.event_name` before any registry or release mutation, while Source
  Repository Ref and Digest continue to bind to `workflow.ref` and the `invocation-source`
  descriptor;
- scope review confirms that no actor field was added, that no `externalParameters` member
  duplicates or aliases the event name, that ADR 0105's descriptors and ADR 0106's re-bindings are
  unchanged, and that no old-bundle compatibility path or buildType version migration was
  introduced;
- a subsequent controlled dogfood attempt submits a revised bundle and records the registry's
  response in `docs/dogfood/`. A tag-push run without a selector naturally exercises the omitted
  `source` group; any remaining rejection family stays fail-closed and is recorded.

## Pros and Cons of the Options

### Move event metadata to `internalParameters.github` as the sole record, and emit the `source` group only when an explicit selector is present

- Good, because the event name is classified where SLSA's model and both reference implementations
  put platform-recorded invocation facts, in the exact location the registry requires.
- Good, because group presence itself encodes explicit selection versus defaulting, and no empty
  container or duplicate event record is ever signed.
- Good, because verification strength is preserved through the certificate Build Trigger comparison
  rather than through redundant external duplication.
- Bad, because the closed top-level parameter shape becomes conditional, so the specification must
  define the emission rule precisely and consumers must handle group absence.

### Move event metadata to `internalParameters.github` as the sole record, but always emit the `source` group, allowing an empty object

- Good, because the event name still gains a single canonical home in the registry-required
  location, and the top-level parameter shape stays constant.
- Bad, because a required-but-possibly-empty group means group presence alone carries no
  information: the explicit-versus-defaulted distinction would survive only as member-level
  inspection, and an always-present container adds nothing to it.
- Bad, because emitting `source: {}` is signed noise that every consumer must tolerate forever, with
  no compensating assurance.

### Keep event metadata in both `externalParameters.source` and `internalParameters.github` with an equality invariant

- Good, because the event name would remain in the parameter object that SLSA says must be verified
  downstream, at the cost of a duplicate record.
- Bad, because SLSA v1.2 explicitly advises against using the same field name in both parameter
  objects, and neither reference implementation duplicates it.
- Bad, because the duplicate adds only a drift surface: the internal copy is already compared
  fail-closed with the certificate's Build Trigger claim, so the external copy adds no assurance.
- Bad, because the event name is not an interface input, so retaining it in `externalParameters`
  permanently misclassifies it.

### Keep event metadata only in `externalParameters.source`

- Good, because it would avoid introducing `internalParameters.github` at all.
- Bad, because it fails the registry's observed requirement for
  `internalParameters.github.event_name`, making npm publication impossible, and contradicts ADR
  0103's standing premise.

### Move event metadata into `externalParameters.workflow`

- Good, because the event would sit beside the invocation ref and repository in the invocation
  group.
- Bad, because it reopens ADR 0103's closed `{ref, repository, path}` group, and still fails the
  registry's required `internalParameters.github.event_name` location.

## More Information

Local decisions, contracts, and evidence:

- [ADR 0026](0026-document-supported-release-caller-patterns-and-runtime-guards.md): the supported
  caller patterns and event admission policy whose recording location moves but whose substance is
  unchanged.
- [ADR 0079](0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md):
  the caller-specified `source-ref` input whose presence now controls the `source` group's emission.
- [ADR 0103](0103-define-external-workflow-parameters-as-common-invocation-identity.md): the
  `internalParameters.github` acquisition-and-verification premise this decision ratifies, and the
  actor-field exclusion.
- [ADR 0106](0106-consolidate-external-source-parameters-onto-workflow-and-descriptor-bindings.md):
  the consolidated source selector and derivation rule this decision leaves intact.
- [SLSA provenance v1](../architecture/slsa-provenance-v1.md),
  [JS/TS npm provenance and publish](../architecture/js-ts-npm-provenance-publish.md), and
  [verification policy and fixtures](../architecture/verification-policy-and-fixtures.md): the
  source-group schema, event admission checks, certificate comparison table, and fixture contracts
  to migrate after acceptance.
- [Dogfood attempt 9](../dogfood/npm-m1.md) and
  [npm registry provenance requirements](../dogfood/npm-registry-provenance-requirements.md): the
  observed `internalParameters.github` requirements and the limits of that evidence.

Primary external sources:

- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance), especially
  [BuildDefinition](https://slsa.dev/spec/v1.2/build-provenance#builddefinition): external versus
  internal parameter classification and the guidance against using the same field name in both.
- [GitHub Actions `workflow/v1` README](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/README.md)
  and
  [example.json](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/example.json):
  `event_name` recorded only in `internalParameters.github`, and the supported-event set.
- [slsa-github-generator PROVENANCE_FORMAT.md](https://github.com/slsa-framework/slsa-github-generator/blob/main/PROVENANCE_FORMAT.md):
  the event name recorded in builder-controlled `invocation.environment` data in the v0.2 format;
  SLSA's v0.2-to-v1 migration guidance maps that environment record to `internalParameters`.

### Deferred scope

This ADR deliberately does not admit actor fields into `internalParameters.github`, change the
supported-event set, or alter any descriptor or verification binding beyond moving the event
record's location. It does not reopen ADR 0103's standing pre-release policy: no buildType version
migration and no old-bundle compatibility path are introduced here. Indexes, traceability tables,
specifications, implementation, and fixtures follow only after the operator's explicit acceptance.
