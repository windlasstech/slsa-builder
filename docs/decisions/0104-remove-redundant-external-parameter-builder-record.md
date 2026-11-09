---
parent: Decisions
nav_order: 104
status: accepted
date: 12026-11-05
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0028
  - type: see-also
    target: ADR-0029
  - type: see-also
    target: ADR-0068
  - type: see-also
    target: ADR-0103
  - type: see-also
    target: ADR-0105
  - type: see-also
    target: ADR-0106
  - type: see-also
    target: ADR-0109
---

# Remove Redundant External-Parameter Builder Record

## Context and Problem Statement

ADR 0103 redefined `buildDefinition.externalParameters.workflow` as the common caller invocation
identity `{ref, repository, path}`. Before that decision, the same group carried the reusable
builder's `path`, full commit `sha`, and canonical `builder_id`. ADR 0103 retained ADR 0028's
SHA-pinned `runDetails.builder.id` as canonical builder identity and expressly reserved relocation
or removal of the old external-parameter representation for this follow-up decision.

The architecture specifications and npm implementation still describe the pre-ADR-0103 shape. The
producer constructs the builder path, SHA, and `builder_id` in `externalParameters.workflow`, copies
the same builder URI into `runDetails.builder.id`, and checks equality between those
representations. The canonical builder URI also contains the workflow path and full commit SHA.
Verification additionally uses the signing certificate's Build Signer URI and Build Signer Digest
and the trusted policy or release metadata that authorizes the builder. Byte-exact fixtures pin this
current serialization.

This leaves two problems. First, the old builder path cannot retain its former meaning in
`workflow.path`, because ADR 0103 assigns that member to the caller workflow. Second, the Statement
currently carries the builder identity in both the external-parameter group and
`runDetails.builder.id`.

The decision affects more than JSON organization. Producer construction, post-sign verification,
consumer verification, and fixture conformance currently use these fields. Any new representation or
removal must preserve the exact trusted profile and builder SHA checks, the binding to the signer
identity in the same verified certificate, and independent authorization by trusted policy or
authenticated release metadata. It must also preserve the distinction between caller invocation,
built source, and reusable builder identity established by the earlier ADRs.

The current implementation distinguishes generic builder-URI validity from npm profile validity. The
common `ValidateBuilderID` implementation accepts multiple Windlass workflow filenames directly
under `.github/workflows`, with either `.yml` or `.yaml`. The npm profile's exact path restriction
currently lives in validation of the old `externalParameters.workflow.path`. A migration that keeps
only the generic URI check would therefore accept a different set of npm builders unless the profile
restriction is enforced elsewhere.

This decision applies to the common producer-provenance representation. The npm profile supplies the
current concrete profile path and buildType invariants, but it does not acquire a separate builder
identity model. Invocation-source `resolvedDependencies`, deduplication with
`source.invocation_ref`/`source.invocation_revision`, and retention of those fields remain outside
this decision, as reserved for ADR 0105 or a separate focused decision by ADR 0103.

Should the former builder path, SHA, and canonical URI record be relocated or removed, and how must
verification change so that every existing builder-identity guarantee is preserved?

## Decision Drivers

- Remove duplicate representation without removing the evidence needed to prove which trusted
  builder executed and signed the build.
- Keep ADR 0028's canonical full-SHA reusable-workflow identity, ADR 0068's authenticated signer and
  immutable policy bindings, and ADR 0029's Windlass-generated provenance model intact.
- Preserve ADR 0103's `externalParameters.workflow` exactly as caller invocation identity
  `{ref, repository, path}`. The reusable builder path must not reclaim or alias the caller `path`.
- Reject non-canonical, unauthorized, or merely self-consistent builder identities fail-closed.
  Expectations must come from trusted policy and authenticated release metadata, never from the
  candidate predicate itself.
- Preserve the custom Windlass buildType, built-source/invocation split, exact-byte signing, subject
  digest checks, and digest-bound handoffs. Compatibility work must not flatten these distinct
  security roles into one record.
- Minimize the signed schema and the number of equality rules whose drift could create conflicting
  identities, following SLSA v1.2's guidance to minimize external-parameter complexity.
- Use the `workflow/v1` README and example as field-placement precedent only. Its example uses a tag
  ref for the reusable provenance generator, while the Windlass production model requires a full
  commit SHA.
- Keep this decision atomic. New dependency inventories, invocation-source deduplication, buildType
  versioning, and old-bundle compatibility require separate decisions where applicable.

## Considered Options

- Remove the redundant external-parameter builder record while retaining canonical builder identity,
  authenticated signer identity, and trusted-policy verification.
- Add a reusable-workflow `ResourceDescriptor` to `resolvedDependencies`.
- Add a vendor-namespaced `buildDefinition` extension for the reusable-builder record.
- Relocate the record to another `externalParameters` extension.
- Relocate the record to `internalParameters`.
- Relocate the path and SHA to `builder.version`.
- Relocate the reusable workflow to `builderDependencies`.
- Move the record to unsigned report-only output.
- Retain the old duplicate external-parameter representation.

## Decision Outcome

Chosen option: "Remove the redundant external-parameter builder record while retaining canonical
builder identity, authenticated signer identity, and trusted-policy verification", because the old
record adds no independent evidence, ADR 0028 already provides the canonical signed identity, and
ADR 0068 already requires the signer and trusted-policy bindings that make that identity meaningful.

### Removed representation and retained invocation identity

`externalParameters.workflow` remains exactly the ADR 0103 caller invocation group:

- `ref`: the platform-authenticated invocation ref;
- `repository`: the canonical caller repository URI;
- `path`: the platform-authenticated caller workflow path.

The former builder `path`, `sha`, and `builder_id` members are removed from this group and are not
recreated under another external-parameter name. The reusable builder path must never populate or
masquerade as caller `workflow.path`. Conversely, caller invocation values do not replace builder
identity. The two records answer different questions and remain independently verified against their
respective platform claims.

No replacement builder record is required by this ADR. Neither "Add a reusable-workflow
`ResourceDescriptor` to `resolvedDependencies`" nor "Add a vendor-namespaced `buildDefinition`
extension for the reusable-builder record" is selected. This ADR defines neither alternative's
schema.

### Canonical builder identity and strict profile authorization

For every producer profile, the canonical signed builder identity is `runDetails.builder.id`. Its
value follows ADR 0028's full reusable-workflow URI form:

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/<profile-workflow>.yml@<full-sha>
```

The `<full-sha>` suffix is exactly 40 lowercase hexadecimal characters identifying the full
reusable-workflow commit SHA. For the JS/TS npm producer, builder identity uses this exact workflow
path:

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@<full-sha>
```

The expected npm buildType remains
`https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`.

Canonical validation compares the complete `runDetails.builder.id` byte-for-byte with the URI
constructed from the authorized profile workflow path and full SHA. It rejects repository or path
aliases, filename-extension aliases outside the selected profile path, percent-encoded separators
such as encoded `/` or `@`, extra `@` characters, query or fragment components, whitespace, branch
or tag suffixes, short SHAs, uppercase or otherwise malformed SHAs, and every other alternate
spelling. URL normalization must not turn an alternate spelling into acceptance.

A well-formed `runDetails.builder.id` is not thereby trusted. The exact profile path, SHA, complete
builder URI, and buildType must be authorized by trusted verifier policy or authenticated Windlass
release metadata. When both policy sources constrain the tuple, the existing intersection rules
apply. Verifier expectations must not be derived from the candidate predicate, its `builder.id`
suffix, or any other candidate-controlled duplicate. A candidate that supplies mutually consistent
but unauthorized values fails closed.

### Authenticated signer binding

For the current SHA-pinned production profile, the same verified signing certificate must satisfy
both of these comparisons:

- Build Signer URI equals `runDetails.builder.id` byte-for-byte.
- Build Signer Digest equals the full 40-character SHA suffix of `runDetails.builder.id`.

Existing URI SAN, issuer, source, run-invocation, runner-environment, transparency, and signing-time
checks remain in force. Missing or malformed signer evidence fails verification. The SHA suffix in
the predicate's `builder.id` cannot substitute for an absent Build Signer Digest, just as the
predicate path cannot substitute for an absent or mismatched Build Signer URI. All comparisons bind
to the same certificate and verified bundle; evidence from different certificates must not be
combined into an apparently valid identity.

Production builder identity derives from GitHub's `job_workflow_ref` and `job_workflow_sha` claims,
which identify the called reusable workflow. It never derives from caller `github.workflow_sha` or
the caller `workflow_sha` claim. The `job_workflow_ref` suffix must itself be the authorized full
SHA and must agree with `job_workflow_sha`. These authenticated production values, rather than the
removed external-parameter fields, support construction and verification of `runDetails.builder.id`.

### Migration guard for profile path restrictions

The current implementation facts described in Context and Problem Statement are mandatory migration
evidence. Canonical common validation can establish the Windlass repository, workflow-path grammar,
and full-SHA form, but that is not equivalent to npm's exact workflow-path restriction. Before the
old external-parameter path check disappears, conformance evidence must show equivalent enforcement
of the exact npm profile path through canonical `builder.id` validation, authenticated signer
checks, and exact trusted policy or release metadata.

No old builder check may disappear without this replacement coverage. In particular, removal must
not reduce the old path/SHA/`builder_id` equality checks to a mere well-formedness check on
`runDetails.builder.id`. Profile selection, canonical identity, authenticated signer, and
independent authorization must still agree on the same exact builder.

### Preserved provenance, signing, and release contracts

- The custom Windlass buildType remains unchanged. This ADR does not adopt the GitHub Actions
  `workflow/v1` buildType, rename its fields into a claimed conformance profile, or relax the
  Windlass full-SHA rule to match the upstream example's tag ref.
- Built source and invocation remain separate. `source.*` and `release.*` continue to identify the
  built release content, while ADR 0103's `workflow.*` identifies invocation. Tag-push runs and ADR
  0079 dispatch retries retain their existing semantics.
- Subject identity and digest verification, exact pre-sign Statement construction, exact-byte DSSE
  signing, exact post-sign payload comparison, and digest-rebound producer-to-publisher handoffs are
  unchanged. Removing fields changes the current pre-release Statement contract, not the
  cryptographic boundary around it.
- The existing `builder.version` component map and the closed signer descriptor in
  `builderDependencies` retain their ADR 0071 and ADR 0077 meanings. Neither becomes a new home for
  the reusable-builder identity.
- Consistent with ADR 0103's pre-release policy, there is no buildType version migration and no
  old-bundle compatibility path. Historical dogfood bundles remain historical evidence, while
  conformance fixtures follow the revised contract.
- ADR 0105's invocation-source `resolvedDependencies` and deduplication questions, including whether
  to retain `source.invocation_ref` and `source.invocation_revision`, are explicitly out of scope.
  This ADR neither adds nor removes an invocation-source descriptor and does not define the schema
  for "Add a reusable-workflow `ResourceDescriptor` to `resolvedDependencies`".

### Consequences

- Good, because one canonical signed builder identity replaces three redundant external-parameter
  fields. Producers and verifiers no longer need to maintain equality among representations that the
  same signer already covers.
- Good, because builder trust remains explicit and conjunctive: canonical `builder.id`,
  same-certificate signer identity, and independently authorized policy must all match. Removing the
  duplicate does not turn a predicate assertion into a trust root.
- Good, because ADR 0103's caller invocation record stays small and truthful. The builder path and
  caller workflow path cannot be conflated within one field group.
- Good, because the result follows SLSA v1.2's field semantics and the `workflow/v1` example's
  single-generator-identity precedent without inheriting that example's tag-based identity or
  top-level-build semantics.
- Neutral, because current pre-release fixtures and exact Statement goldens must change together.
  Historical bundles are not supported under a parallel compatibility schema.
- Bad, because migration touches canonical validation, signer binding, trusted policy, fixtures, and
  live evidence as one security-sensitive unit. The npm fixed-path rule currently coupled to the old
  representation must be proven at its replacement boundaries before deletion.
- Bad, because an explicit future requirement for an artifact-level reusable-workflow inventory or a
  consumer-specific duplicate record would need another focused decision. This ADR deliberately does
  not pre-commit to the schemas for "Add a reusable-workflow `ResourceDescriptor` to
  `resolvedDependencies`" or "Add a vendor-namespaced `buildDefinition` extension for the
  reusable-builder record".
- Bad, because a controlled npm dogfood attempt remains necessary. The `workflow/v1` precedent and
  the absence of a particular registry complaint do not prove acceptance of the custom Windlass
  buildType or canonical builder identity.

## Confirmation

This decision is confirmed when specifications, test-first conformance cases, exact-byte fixtures,
migration review, and controlled live evidence demonstrate all of the following:

- the common provenance and producer profile contracts remove the old builder
  `path`/`sha`/`builder_id` representation while preserving `externalParameters.workflow` exactly as
  `{ref, repository, path}` and retaining canonical `runDetails.builder.id`;
- table-driven canonical and profile validation accepts the exact authorized `runDetails.builder.id`
  and rejects every tested alias, percent-encoded separator, extra `@`, query, fragment, whitespace
  variant, branch, tag, short SHA, uppercase or malformed SHA, wrong repository, and wrong profile
  path. The npm cases require exactly `.github/workflows/js-ts-npm-package-slsa3.yml` as the builder
  workflow path, together with the expected Windlass npm buildType;
- negative fixtures distinguish a well-formed but unauthorized full SHA from a malformed identity.
  Both fail, and the former proves that canonical syntax is not trusted-policy authorization;
- signer tests reject missing or mismatched Build Signer URI and Build Signer Digest, including when
  the predicate's `builder.id` contains the expected path and SHA suffix. Tests also prove that all
  signer comparisons use the same verified certificate, retain the existing SAN and other identity
  bindings, derive production identity from `job_workflow_ref`/`job_workflow_sha`, and never use
  caller `github.workflow_sha` as builder identity;
- policy tests reject expectations derived from the candidate predicate. A jointly consistent forged
  `builder.id`, matching predicate suffix, and any old duplicate values cannot rescue an untrusted
  builder, an unauthorized well-formed SHA, a wrong buildType, or missing signer evidence;
- migration review maps every removed old builder check to equivalent canonical `builder.id` plus
  signer plus trusted-policy checks. In particular, a test with a different Windlass workflow
  filename proves that deleting old external-parameter validation has not removed the npm fixed-path
  restriction, and no old builder check disappears without replacement coverage;
- accepted tag-push and ADR 0079 dispatch-retry examples remain valid. On retry, `workflow.ref`
  remains the invocation ref, built `source.ref`/`release.ref` remain the release tag, and canonical
  `runDetails.builder.id` remains the reusable builder identity. A negative fixture proves that the
  builder path cannot masquerade as caller `workflow.path` when the authenticated caller path
  differs;
- exact pre-sign Statement bytes remain the DSSE payload, post-sign verification compares the exact
  signed bytes, and updated byte-exact fixtures contain no old builder record. Negative fixtures
  with the removed fields are rejected under the closed schema, while subject digest and
  digest-rebound handoff tests continue to pass unchanged;
- scope review confirms that no invocation-source `resolvedDependencies` descriptor, dependency
  deduplication rule, or `source.invocation_ref`/`source.invocation_revision` retention decision has
  been smuggled into this migration, and that no old-bundle compatibility path or buildType version
  migration has been introduced;
- a subsequent controlled dogfood attempt exercises publish, registry attestation read-back, and
  consumer verification against the revised bundle and records the evidence in `docs/dogfood/`.
  Registry absence of complaints about the removed fields, `builder.id`, or buildType is not treated
  as complete acceptance proof. Any remaining rejection family stays fail-closed and is recorded
  rather than being inferred away from the `workflow/v1` example.

## Pros and Cons of the Options

### Remove the redundant external-parameter builder record while retaining canonical builder identity, authenticated signer identity, and trusted-policy verification

Remove the former external-parameter builder fields without relocating them, while strengthening the
explicit invariant that canonical identity, authenticated signer, and trusted policy must agree.

- Good, because it deletes representation duplication rather than evidence. The canonical ADR 0028
  URI already carries the workflow path and full SHA, and the certificate and policy provide their
  independent authentication and authorization.
- Good, because signed self-duplication by one signer is not independent authentication, and an
  atomic JSON group provides no additional cryptographic atomicity. Removing the fields eliminates a
  misleading appearance of defense in depth.
- Good, because fewer signed fields and equality rules reduce drift, parser differentials, fixture
  size, and the chance that one representation is checked while another is trusted.
- Good, because it preserves the custom buildType, full-SHA builder model, source/invocation split,
  exact-byte signing, and all existing fail-closed artifact and handoff checks.
- Bad, because safe removal requires coordinated proof that profile-specific restrictions formerly
  attached to the old fields remain enforced. The common validator's wider workflow filename set
  makes this a real migration risk, not a theoretical cleanup detail.
- Bad, because pre-release fixtures and historical examples require regeneration, with no supported
  old-bundle compatibility mode.

### Add a reusable-workflow `ResourceDescriptor` to `resolvedDependencies`

Record the reusable workflow as a URI-and-digest dependency in addition to `runDetails.builder.id`.

- Good, because SLSA's standard build-input model can fit this representation when the buildType
  treats the reusable workflow as an artifact needed to initialize the build. `resolvedDependencies`
  records artifacts fetched during initialization or execution with their resolved digests.
- Good, because an explicit future artifact-inventory requirement could make the descriptor useful
  for recursive supply-chain analysis independently of canonical builder identity.
- Bad, because no such inventory requirement exists now. The descriptor would duplicate the path and
  SHA already carried by `builder.id`, adding another identity equality surface without improving
  signer authentication or trusted-policy authorization.
- Bad, because the npm dependency schema is closed and name-keyed. It currently enumerates the
  selected lockfile, runtime, runner image, and conditional package-manager distribution, not a
  reusable-builder entry. Adding one changes that contract and its cardinalities rather than filling
  an unused standard slot.
- Bad, because dogfood attempt 9 observed npm checking `resolvedDependencies[0].uri` against the
  caller source URI at the invocation ref and `resolvedDependencies[0].digest.gitCommit` against the
  invocation commit. Inserting or reordering a builder descriptor could disturb that separately
  observed registry behavior. SLSA calls the collection unordered and example order is not
  normative, but npm's index-0 observations remain concrete registry evidence that must not be
  dismissed.
- Neutral, because this remains a fallback only if an explicit artifact-inventory requirement is
  later established. It is not part of ADR 0105, and this ADR decides neither its descriptor name,
  URI, annotations, cardinality, nor placement.

### Add a vendor-namespaced `buildDefinition` extension for the reusable-builder record

Carry a duplicate reusable-builder record in an extension such as a vendor-prefixed member of
`buildDefinition`.

- Good, because SLSA v1.2 permits vendor-namespaced extension fields for information not captured by
  standard fields, provided they do not alter standard-field meanings and ignoring or removing them
  cannot turn a denial into an allowance.
- Neutral, because a concrete future consumer requirement could justify a separately specified,
  additive annotation whose absence changes no verification result.
- Bad, because consumers may ignore extensions. The current typed `DecodeStatement` intentionally
  ignores unrecognized fields, and canonical re-encoding from that typed model drops them, so the
  extension would not reliably survive the existing decode/re-encode path as a canonical builder
  record.
- Bad, because no concrete consumer requirement justifies that new compatibility surface today. A
  duplicate signed assertion would still add no independent authentication. A policy that requires
  the extension must fail closed when it is absent, while a consumer that ignores it cannot claim
  equivalent verification. Its presence must never override a canonical builder, signer, or policy
  failure.
- Bad, because preserving the extension through exact-byte and strict-validation boundaries would
  require new consumer and schema evidence. That conditional future work is not a reason to retain
  the redundant record now.

### Relocate the record to another `externalParameters` extension

Move the builder triplet into a differently named member of `externalParameters`.

- Good, because the values would remain signed and visible to strict consumers that know the custom
  schema.
- Bad, because the reusable builder's own path, SHA, and URI are not externally controlled caller
  inputs. Moving them within `externalParameters` does not correct that semantic mismatch.
- Bad, because SLSA recommends minimizing external parameters and rejecting unexpected fields. A
  second closed extension would preserve all duplicate validation and drift costs while adding no
  new evidence.

### Relocate the record to `internalParameters`

Move the builder record into the builder-controlled parameter object.

- Good, because `internalParameters` can contain platform-controlled debugging or incident-response
  values, so a redundant diagnostic copy would not be categorically impossible.
- Bad, because SLSA does not require downstream verification of these values once the platform is
  trusted. That makes `internalParameters` an inappropriate trust authority for canonical builder
  identity or a required signer/policy binding.
- Bad, because the rejection does not depend only on the current empty-object rule. ADR 0103 already
  anticipates `internalParameters.github`, so allowing some internal fields would not make this
  relocation valid. The semantic authority mismatch is decisive.

### Relocate the path and SHA to `builder.version`

Encode the reusable workflow path and commit in the builder's component-version map.

- Good, because the values would sit near canonical `builder.id` in `runDetails.builder`.
- Bad, because SLSA defines `builder.version` as a map of build-platform component names to
  versions, not as a second builder URI or a replacement for the immutable `builder.id` suffix.
- Bad, because the current closed map records observed components such as Node.js and conditionally
  Corepack. Adding path and SHA duplicates identity, weakens the component-version semantics, and
  still supplies no independent signer or policy evidence.

### Relocate the reusable workflow to `builderDependencies`

Add a descriptor for the reusable builder alongside the signing dependency.

- Good, because a descriptor can identify an orchestrator-side component that affects provenance
  generation or security guarantees.
- Bad, because SLSA restricts `builderDependencies` to orchestrator dependencies that are not run
  within the workload and do not affect the build. The reusable builder is the trusted platform that
  defines and executes the build, not an orchestrator-only dependency with no workload effect.
- Bad, because `builder.id` is the standard field for that platform identity. The existing closed
  signer descriptor must retain its own ADR 0077 role rather than being joined by a misclassified
  builder descriptor.

### Move the record to unsigned report-only output

Keep the path and SHA in logs or diagnostic reports but remove them from signed provenance.

- Good, because unsigned output can help operators debug a run without changing the Statement
  schema.
- Bad, because report-only data is not signed provenance evidence and cannot authenticate or
  authorize the builder. It cannot replace `builder.id`, certificate signer evidence, or trusted
  policy.
- Neutral, because diagnostic logging may continue independently, but it is not a relocation option
  for the removed verifier-relevant representation.

### Retain the old duplicate external-parameter representation

Keep builder `path`, `sha`, and `builder_id` alongside ADR 0028's `runDetails.builder.id`.

- Good, because existing old-shape fixtures would require fewer edits, and consumers already reading
  the duplicate fields would not need to migrate.
- Bad, because ADR 0103 assigns `workflow.path` to the caller workflow, so retaining the old builder
  meaning in the same group would create a direct semantic conflict rather than harmless redundancy.
- Bad, because the duplicate values add verification branches and possible disagreement without new
  cryptographic evidence. The same signer covers both copies, and JSON grouping adds no independent
  atomicity or trust.
- Bad, because preserving a representation solely to avoid pre-release fixture work perpetuates the
  schema complexity that SLSA recommends minimizing and leaves the npm fixed-path restriction in a
  misleading location.

## More Information

Local decisions, contracts, and evidence:

- [ADR 0028](0028-use-sha-pinned-reusable-workflow-builder-identity.md): canonical SHA-pinned
  reusable-workflow `builder.id` and signed release metadata.
- [ADR 0029](0029-use-windlass-generated-slsa-provenance-for-npm-publish.md): Windlass-generated
  canonical provenance and verifier-relevant custom buildType.
- [ADR 0068](0068-bind-verification-to-immutable-builder-and-source-identities.md): maximal
  immutable identity binding, including Build Signer URI/Digest and the prohibition on caller
  `github.workflow_sha` as builder identity.
- [ADR 0103](0103-define-external-workflow-parameters-as-common-invocation-identity.md): caller
  invocation `{ref, repository, path}`, pre-release compatibility policy, and the reserved ADR 0104
  and ADR 0105 scope boundaries.
- [ADR 0071](0071-activate-builder-version-and-builderdependencies-for-platform-components.md) and
  [ADR 0077](0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md):
  component-version versus orchestrator-dependency semantics and exact-byte signing.
- [ADR 0079](0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md)
  and
  [ADR 0080](0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md):
  built-source and invocation separation.
- [Builder identity and build types](../architecture/identity-and-buildtypes.md),
  [SLSA provenance v1](../architecture/slsa-provenance-v1.md),
  [JS/TS npm provenance and publish](../architecture/js-ts-npm-provenance-publish.md), and
  [verification policy and fixtures](../architecture/verification-policy-and-fixtures.md): current
  canonical identity, closed schema, signer, policy, and exact-byte contracts to migrate after
  acceptance.
- [Dogfood attempt 9](../dogfood/npm-m1.md) and
  [npm registry provenance requirements](../dogfood/npm-registry-provenance-requirements.md):
  observed `resolvedDependencies[0]` source URI/gitCommit checks, certificate-claim mapping, and the
  limits of registry evidence.
- Current implementation migration evidence:
  [common builder identity validation](../../internal/identity/identity.go),
  [npm external-parameter validation](../../internal/npmprofile/provenance_validate.go), and
  [typed Statement decoding and canonical re-encoding](../../internal/provenance/model.go). These
  references explain the fixed-path and unknown-extension risks; they do not prescribe the future
  implementation.

Primary external sources:

- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance), especially
  [BuildDefinition](https://slsa.dev/spec/v1.2/build-provenance#builddefinition),
  [Builder](https://slsa.dev/spec/v1.2/build-provenance#builder),
  [parsing rules](https://slsa.dev/spec/v1.2/build-provenance#parsing-rules), and
  [extension fields](https://slsa.dev/spec/v1.2/build-provenance#extension-fields): trusted builder
  identity, signer-builder pairing, minimal external parameters, dependency and builder-dependency
  semantics, and the requirement that consumers ignore unrecognized extensions.
- [GitHub Actions `workflow/v1` README](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/README.md)
  and
  [example.json](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/example.json):
  caller invocation, source dependency, runner environment, and reusable provenance-generator
  precedent.
- [Fulcio OID directory](https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md): Build
  Signer URI and Digest derive from `job_workflow_ref` and `job_workflow_sha`, while Build Config
  URI derives from caller `workflow_ref`.

### What the `workflow/v1` precedent does and does not establish

The upstream example puts the caller's top-level invocation in `externalParameters.workflow`, puts
the caller source at the invocation ref and commit in `resolvedDependencies[0]`, and puts the runner
environment in `resolvedDependencies[1]`. Its reusable provenance generator appears only in
`runDetails.builder.id`. There is no reusable-builder descriptor, no duplicate builder
path/SHA/`builder_id` triplet, and no `builder.version` or `builderDependencies` member.

That example order is not a normative ordering rule: SLSA defines `resolvedDependencies` as an
unordered collection. npm's observed index-0 checks in dogfood attempt 9 are separate registry
evidence, not a rule inferred from the example's array order.

The README excludes a reusable workflow as the build described by `workflow/v1`, yet explicitly
allows a reusable workflow that generates provenance to be named by `runDetails.builder.id`. These
statements are not contradictory: the described build is the caller's top-level workflow, while the
provenance generator is a different entity recorded in run details. Windlass's actual build and
signing boundary is the SHA-pinned reusable workflow, so it must not claim `workflow/v1` conformance
or adopt that buildType. The example's generator identity uses `@refs/tags/v0.0.1`; Windlass instead
requires `@<40 lowercase hex>` under ADR 0028.

The precedent therefore supports only the narrow representation conclusion selected here: canonical
builder identity belongs in `runDetails.builder.id`, while caller invocation and dependencies retain
their own semantics. It establishes neither npm acceptance nor a requirement for "Add a
reusable-workflow `ResourceDescriptor` to `resolvedDependencies`" or "Add a vendor-namespaced
`buildDefinition` extension for the reusable-builder record".

### Deferred scope

This proposal deliberately does not decide ADR 0105's invocation-source dependency or
`source.invocation_*` deduplication, an artifact-inventory descriptor under "Add a reusable-workflow
`ResourceDescriptor` to `resolvedDependencies`", a consumer-driven extension under "Add a
vendor-namespaced `buildDefinition` extension for the reusable-builder record", buildType version
migration, or old-bundle compatibility. Indexes, traceability tables, specifications,
implementation, and fixtures follow only after the operator's explicit acceptance.
