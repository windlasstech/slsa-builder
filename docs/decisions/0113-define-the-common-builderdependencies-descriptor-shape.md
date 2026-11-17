---
parent: Decisions
nav_order: 113
status: accepted
date: 12026-11-15
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0077
    scope:
      'the signing-adapter descriptor''s member set: the descriptor gains `name` with the value
      "signing-adapter", drops the `annotations` member whose identity function moves to `name`, and
      migrates the digest member from the Go-native h1 base64 form to the DigestSet dirHash1
      lowercase-hex form over the same digest bytes; the signer selection, the descriptor''s URI
      form, the underlying digest bytes, the single-descriptor cardinality, and the validation
      obligations remain in force'
  - type: see-also
    target: ADR-0071
  - type: see-also
    target: ADR-0105
  - type: see-also
    target: ADR-0109
  - type: see-also
    target: ADR-0110
---

# Define the Common `builderDependencies` Descriptor Shape with `name` as the Identity Key

## Context and Problem Statement

ADR 0071 activated `runDetails.builder.version` and `builderDependencies` for platform components,
and ADR 0077 fixed the array's sole descriptor — the Go-native Sigstore signing adapter — with the
member set `{uri, digest, annotations{role}}`. That shape exists only as the incumbent descriptor's
instance: the common contract never defined which members a `builderDependencies` descriptor must
carry or which member identifies a descriptor within the array.

That shape was never reviewed at the field level: ADR 0071 set it as a convention for its
`actions/attest` descriptor, and ADR 0077 replaced the signer and updated the identity values while
inheriting the member set unchanged. The need to review the required common member set of a
`builderDependencies` descriptor and decide it explicitly has now emerged. A descriptor addition is
under consideration as a separate decision, and this decision does not depend on it: the common
shape must be an explicit contract regardless of whether any further descriptor is ever added.
Without codifying it, the producer has no member-set rule to emit against, the verifier no closed
shape to enforce fail-closed, and future descriptors no stable identity convention. in-toto's
ResourceDescriptor makes every field optional and explicitly allows the using context to require
fields, so the shape contract is this project's to define.

What is the required common member set of a `builderDependencies` descriptor, and which member is
its identity key?

This decision governs the common shape only. It applies to the existing signing-adapter descriptor
and to every descriptor added later. Whether additional members such as `mediaType` or `annotations`
may appear on a particular descriptor is decided by that descriptor's governing ADR and is outside
this decision's scope. The array's cardinality and the admission of any new descriptor are likewise
outside this decision's scope: the ADR 0071/0077 single-descriptor rule is unchanged here, and any
descriptor addition is a separate decision whose descriptor must conform to this shape.

## Decision Drivers

- Follow in-toto's field semantics: `name` is defined as the machine-readable identifier for
  distinguishing between descriptors, while `annotations` is the producer/consumer-agreed extension
  vocabulary.
- Keep the array's convention aligned with the repository's `resolvedDependencies` descriptors,
  which are name-keyed (ADR 0105, ADR 0109).
- Preserve a fail-closed, byte-predictable closed shape consistent with the repository's closed
  contracts (diagnostic registry, external parameters, version map).
- Make the incumbent descriptor conform with a single, explicitly scoped migration rather than
  leaving it on a divergent convention.
- Keep value conventions — the `uri` form and the digest algorithm and encoding — per-descriptor,
  pinned by each descriptor's governing ADR.

## Considered Options

- `name`-keyed three-member shape: `{name, uri, digest}`, with `name` as the identity key.
- `role`-keyed three-member shape: `{uri, digest, annotations{role}}`, with `annotations.role` as
  the identity key (the incumbent's current form, generalized).
- Dual-key four-member shape: `{name, uri, digest, annotations{role}}`, both members required.
- No common shape rule: each descriptor's governing ADR defines its own member set freely.
- Full five-member shape: `{name, uri, digest, mediaType, annotations}` for every descriptor.

## Decision Outcome

Chosen option: "`name`-keyed three-member shape", because `name` is the in-toto field whose defined
purpose is distinguishing descriptors, it aligns the array with the repository's name-keyed
`resolvedDependencies` convention, and its cost is a one-time, scoped migration of the incumbent
descriptor.

### Common required members

Every `builderDependencies` descriptor carries exactly these required common members:

- `name`: the descriptor's identity key. The value is a lowercase kebab-case token declared by the
  descriptor's governing ADR, unique within the array, and stable across releases. A descriptor
  whose `name` is not declared by an accepted ADR fails closed, as does a duplicated `name`.
- `uri`: the globally identifying URI of the resource. The concrete form (for example a package URL)
  is pinned per descriptor by its governing ADR.
- `digest`: the cryptographic digest set of the resource. The algorithm follows the ecosystem-native
  integrity authority of the resource and is pinned per descriptor by its governing ADR. The
  encoding follows in-toto's DigestSet rules: algorithms named by DigestSet (for example `sha256`,
  `sha512`, `gitCommit`, `dirHash1`) use lowercase hexadecimal values, and a non-predefined
  algorithm is used only when DigestSet defines no named form for the resource, with its encoding
  documented by the governing ADR.

### Additional members

This decision defines no additional common members. A descriptor's governing ADR may define
additional members (for example `mediaType` or `annotations`) for that descriptor; such definitions
are outside this decision's scope, which sets no admission condition on them beyond their
declaration by that governing ADR. The closed-shape rule then applies to the union: the producer
emits exactly the common members plus the declared additional members, and producer-side emission of
any undeclared member fails closed. For the incumbent signing-adapter descriptor, consumer
verification ignores `annotations` as descriptive metadata even though the producer emits no such
member. This exception admits no other member and gives annotations no identity or integrity role.

### Incumbent migration

The signing-adapter descriptor becomes:

```json
{
  "name": "signing-adapter",
  "uri": "pkg:golang/github.com/sigstore/sigstore-go@<governed module version>",
  "digest": { "dirHash1": "<lowercase hex dirHash1 of that module version>" }
}
```

This template fixes the descriptor's shape, not its values. The `uri` version and the digest value
at any time are derived from the governed Go module dependency graph (`go.mod` and `go.sum`) for the
module the signing binary actually uses; at the time of drafting these are sigstore-go v1.4.0 and
its recorded module checksum. A governed sigstore-go version bump changes only those values: it is
not a change to this decision, and it does not require a new ADR. Only a change to the shape — the
required common members or the identity-key rule — is a new decision requiring its own ADR.

The digest member migrates from the Go-native `h1` base64 form to the DigestSet-standardized
`dirHash1` form: the digest bytes are unchanged, and the recorded value is the lowercase-hex
rendering of the same directory hash that `go.sum` records as `h1:<base64>`. The agreement check
between the descriptor and the governed module graph compares them through this deterministic
conversion.

The `annotations.role` member is removed: its identity function moves to `name`, and retaining both
would create a redundant second identity axis requiring an authoritative-mismatch rule. The signer
selection, the URI form, and the cardinality are unchanged.

### Failure behavior

These rules are stricter than the in-toto baseline parsing rule that consumers must ignore
unrecognized fields; they take effect because this predicate's specification notes them explicitly,
following the precedent by which SLSA verification rejects unrecognized `externalParameters`. Member
presence is evaluated under the in-toto equivalence of unset, null, and empty values.

Shape failures are rejected fail-closed: a descriptor missing a required common member, carrying an
undeclared `name`, duplicating another descriptor's `name`, or carrying any member beyond its
declared shape, except that consumer verification ignores the signing-adapter's `annotations` member
as descriptive metadata. The producer still emits no annotations on that descriptor, and no other
additional member is admitted by this exception. Duplicate JSON members are already rejected at
parse time under ADR 0061 before these semantic checks run. Producer-side emission of such a
descriptor aborts before signing; consumer-side verification of every other shape violation fails
the Statement at the expectations stage, after envelope signature verification. Value-level
agreement of `uri` and `digest` members with their governed expectations is defined per descriptor
by its governing ADR and is outside this decision.

The closed diagnostic registry gains the corresponding `windlass.verify.error.*` entries, with the
specification tables updated to match.

### Consequences

- Good, because the array's identity convention now uses the in-toto field defined for that purpose
  and matches the `resolvedDependencies` name-keyed convention.
- Good, because the common shape is a verifiable contract rather than an emergent pattern: the
  verifier enforces declared names and declared member sets fail-closed.
- Good, because per-descriptor additional members remain possible without reopening this decision.
- Good, because the incumbent migration is a single additive member and a single removed redundant
  member, scoped by the relation to ADR 0077.
- Neutral, because `annotations` is no longer the identity channel; descriptors needing annotations
  define them through their own governing ADRs.
- Bad, because the migration touches the ADR 0077 descriptor content, its validator constants, and
  every byte-exact fixture and dogfood expectation that renders the signing-adapter descriptor.

### Confirmation

This decision is confirmed when:

- the common SLSA provenance v1 specification defines the common required members, the `name`
  identity-key rule, the additional-member governance, and the failure behavior above, each with a
  valid example and rejection cases;
- the incumbent signing-adapter descriptor in the producer specification, validator constants, and
  canonical emitted fixture carries `name: "signing-adapter"` and no `annotations` member, while
  consumer verification ignores annotation-varied descriptors as descriptive metadata;
- the diagnostic registry and specification tables contain the new shape-failure entries, enforced
  by the registry parity test;
- `fixture-check` passes with the updated corpus.

## Pros and Cons of the Options

### `name`-keyed three-member shape

- Good, because it uses the in-toto field whose defined purpose is distinguishing descriptors.
- Good, because the only external usage example found — the AdaCore e3-core provenance example,
  annotated as taken from the SLSA v1.0 spec schema — identifies its `builderDependencies`
  descriptor by `name` and uses `annotations` for descriptive metadata, so this shape matches the
  sole external reference point rather than diverging from it.
- Good, because it aligns with the name-keyed `resolvedDependencies` descriptors of ADR 0105 and ADR
  0109, so both descriptor arrays share one convention.
- Good, because additional members remain available per descriptor without changing this rule.
- Bad, because the incumbent descriptor must gain `name` and drop `annotations.role`, a governed
  content change to ADR 0077 with fixture and validator churn.

### `role`-keyed three-member shape

- Good, because the incumbent already conforms, so no migration is needed.
- Good, because the admission vocabulary lives in `annotations`, whose in-toto definition is the
  producer/consumer-agreed extension point.
- Bad, because it leaves the in-toto `name` field's defined purpose unused and keeps the array on a
  convention divergent from the repository's other descriptor array.
- Bad, because occupying `annotations` for identity leaves no separated channel for future
  descriptive metadata on the same descriptors.

### Dual-key four-member shape

- Good, because it separates a human-facing handle from a machine admission key.
- Bad, because with one descriptor per identity the two keys always carry the same value, a
  structural redundancy that violates the repository's single-home and minimization direction and
  requires a new authoritative-mismatch rule.
- Bad, because it carries the same incumbent migration cost as the `name`-keyed option.

### No common shape rule

- Good, because each descriptor kind can take its optimal member set, and the incumbent is
  untouched.
- Bad, because the verifier loses the shared shape invariant: per-descriptor validation logic
  diverges, byte-predictability weakens, and the array drifts from the repository's closed-shape
  doctrine.

### Full five-member shape

- Good, because it mirrors the richest existing descriptor definition.
- Bad, because it is internally inconsistent for the incumbent: the signing-adapter digest covers a
  Go module tree (a directory hash), not a single serialized artifact representation, so ADR 0110's
  own admission test admits no `mediaType` for it; a common shape that an existing member cannot
  satisfy is self-contradictory.

## More Information

- in-toto ResourceDescriptor field definitions:
  <https://github.com/in-toto/attestation/blob/df02077bf97218a8860a5c534eff1f1381f56984/spec/v1/resource_descriptor.md>
  (identical at tag v1.2.0).
- in-toto DigestSet field type specification (lowercase-hex encoding for named algorithms and the
  `dirHash1` form the incumbent digest migrates to):
  <https://github.com/in-toto/attestation/blob/v1.2.0/spec/v1/digest_set.md>.
- in-toto attestation parsing rules (unrecognized-field, unset/null/empty equivalence, and
  monotonic-principle rules the failure behavior builds on):
  <https://github.com/in-toto/attestation/blob/main/spec/v1/README.md#parsing-rules>.
- SLSA v1.2 build provenance, `Builder` field definitions:
  <https://slsa.dev/spec/v1.2/build-provenance#builder>.
- SLSA v1.2 verifying artifacts (expectations stage and the unrecognized-`externalParameters`
  rejection precedent cited by the failure behavior):
  <https://slsa.dev/spec/v1.2/verifying-artifacts>.
- Incumbent descriptor and its migration source:
  [ADR 0077](0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md).
- Parse-time duplicate-member rejection referenced by the failure behavior:
  [ADR 0061](0061-reject-duplicate-json-members-in-signed-slsa-statements.md).
- Name-keyed descriptor precedent:
  [ADR 0105](0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md),
  [ADR 0109](0109-minimize-external-parameters-to-actual-interface-inputs.md).
- `mediaType` admission test referenced by the rejected five-member option:
  [ADR 0110](0110-record-deterministic-media-types-for-non-runner-dependencies.md).
- External convention survey, 12026-11-15: a GitHub-wide code search for `builderDependencies`
  returned only schema definitions and no instance carrying a `role` annotation; the major public
  producers (slsa-github-generator, GitHub `actions/attest`) omit the field entirely. The
  incumbent's `annotations.role` shape was therefore a project-internal convention inherited from
  ADR 0071, not an adopted external convention. The only member-carrying example found is the
  AdaCore e3-core
  [provenance example](https://github.com/AdaCore/e3-core/blob/master/tests/tests_e3/slsa/provenance-example.json),
  which identifies the descriptor by `name` and uses `annotations` for descriptive metadata. Its own
  comment claims origin in the SLSA v1.0 spec schema, but that example does not appear in the
  published v1.0 or v1.0-RC2 spec text, so it is cited as an external usage example, not as spec
  text.
- A descriptor addition under consideration as a separate decision must define its descriptor to
  conform to this shape; this decision neither requires nor presupposes that addition.
