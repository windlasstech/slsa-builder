---
parent: Decisions
nav_order: 115
status: accepted
date: 12026-11-19
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0062
    scope:
      "the initial manifest represented-field limitation; authenticated revision-specific descriptor
      expectations for represented producer workflow SHA/profile entries are admitted by the
      governing schema; independent trust bootstrap, field-level intersection, and fail-closed
      conflict handling remain in force"
  - type: partially-supersedes
    target: ADR-0114
    scope:
      "the consumer expectation source for the publish-cli descriptor for producer revisions
      represented by an authenticated signed release manifest; actual-used provisioning agreement,
      descriptor shape and emission, annotation handling, and governed bump procedure remain in
      force"
  - type: see-also
    target: ADR-0031
  - type: see-also
    target: ADR-0077
  - type: see-also
    target: ADR-0113
---

# Record Revision-Specific Descriptor Expectations in Signed Release Manifests

## Context and Problem Statement

ADR 0077 requires the signing-adapter descriptor to agree with the module version and checksum used
by the signing binary. ADR 0113 later changed that descriptor's common shape without freezing its
values: ordinary governed module bumps change the URI version and digest without a new ADR (ADR
0113, `Incumbent migration`). ADR 0114 applies the same principle to the npm publish CLI: its
version-and-digest bumps follow the existing pin procedure, not a new architectural decision (ADR
0114, `npm profile descriptor`). These rules preserve an honest actual-used record while allowing
routine dependency maintenance.

Those decisions govern what producers record and how descriptor values evolve. They do not define
how a consumer obtains authoritative expectations for an authenticated producer revision, especially
after later governed bumps. This decision gap exists independently of current specification or
implementation behavior: every correct implementation needs a rule for selecting the values that
belong to the revision being verified.

Full-SHA builder identity remains necessary. ADR 0028 binds production execution and `builder.id` to
the exact workflow commit, and ADR 0068 binds the signing certificate's Build Signer Digest to that
commit through the `Signer workflow` binding in its `Decision Outcome`. That identity authenticates
which builder revision produced the Statement, not which descriptor values that revision should
carry. A verifier's own values are not evidence about that revision, while a global allowlist of
historically valid pairs cannot bind a pair to its producing revision. The provenance descriptor is
a claim to check, not a source of its own expectation.

The signed release manifest already supplies the adjacent release-to-revision mapping. ADR 0031's
`Decision Outcome` defines its minimum release, workflow SHA, `builder.id`, and `buildType` content.
ADR 0062's `Decision Outcome` closes schema version 1 to release identity, workflow, builder, build
type, and publisher mappings, excluding descriptor expectations. The selected generation path uses
the protected tagged source and the three-job signing boundary. The manifest surface is specified
but not yet implemented, so no existing signed manifest is evidence for this decision.

After authenticating the producing builder revision, what evidence should be authoritative for a
consumer's revision-specific descriptor expectations?

## Decision Drivers

- Bind every accepted descriptor expectation to the authenticated producer workflow SHA and profile,
  not to verifier-local state or a revision-agnostic allowlist.
- Preserve the producer's actual-used emission and pre-sign agreement checks, including the ADR 0077
  module agreement and the ADR 0114 embedded-pin provisioning checks.
- Reuse the release identity, tagged source inputs, per-release issuance cadence, and signing
  boundary already selected for the release manifest.
- Keep manifest trust bootstrap independent. A signature authenticates an authorized issuer and
  signed bytes; it does not establish that the issuer's assertion is truthful or sufficient for
  local policy.
- Preserve ADR 0062's field-level policy intersection and fail-closed conflict handling.
- Support reproducible offline verification from retained artifacts and governed trust material,
  without a required network lookup or a new freshness registry.
- Distinguish source-derived content evidence from a publisher's signed assertion without demanding
  historical builder execution for only one option.
- Avoid a second metadata authority and consumer-resolution path when the specified per-release
  manifest can carry the expectation with the same revision key.
- Recognize the project's pre-release state as a reason that prospective adoption has low migration
  cost, not as a reason to invent a blanket obligation to recover already issued experimental
  artifacts.

## Considered Options

- Distribute an authenticated, reviewed revision catalog with verification support.
- Derive expectations from authenticated original commit, tree, and blob data without executing
  historical builder code.
- Record revision-specific descriptor expectations in the signed per-release manifest.
- Publish a separate signed expectations document with its own signer and revision binding.

## Decision Outcome

Chosen option: "Record revision-specific descriptor expectations in the signed per-release
manifest", because the specified manifest already binds a release to the exact producer workflow SHA
and profile from the same governed tagged source. Generating descriptor expectations in that release
path adds no second trust channel and gives consumers one revision key and one authenticated
metadata artifact to resolve.

### Authoritative producer-issued evidence

For each authenticated producer workflow SHA and profile represented by a signed release manifest,
that manifest is the authoritative producer-issued evidence for the revision's applicable descriptor
expectations. The manifest records what that release expects; provenance records what a particular
run claims to have used. Consumer verification compares the two only after authenticating both and
binding them to the same full workflow SHA and profile.

The generator must derive expected values deterministically from governed tagged source inputs and
the descriptor rules applicable to that producer. Signer expectations use the selected module graph
and module-content checksum semantics; publish-cli expectations use the embedded provisioning pin
and deterministic digest conversion. Inconsistent or ambiguous inputs fail before signing. These
checks establish agreement with source and rules, not actual binary execution.

Producer authority remains unchanged: the signing binary must agree with its compiled signer
descriptor, the publish CLI must agree with its embedded provisioning pin before use, and exact
shape and pre-sign checks remain fail closed. The consumer change does not let a producer emit
arbitrary descriptors.

A consumer authenticates the manifest under independently configured roots of trust, authenticates
the producing workflow under ADR 0068, and selects the entry matching the full workflow SHA and
profile. It then applies ADR 0062's intersection to every policy source explicitly constraining the
same field. Neither the manifest nor local policy may widen the other; conflict or an empty
intersection fails closed.

Verifier-local values do not independently establish or substitute for manifest evidence. An
explicitly admitted verifier policy remains a separate constraint subject to intersection.

### Descriptor and profile scope

The authority applies only where the manifest represents a descriptor. Initial scope is the common
`signing-adapter` and npm `publish-cli`, whose shapes and actual-used rules remain governed by ADRs
0077, 0113, and 0114. Using the same mechanism for either needs no per-tool ADR solely for that use.

The mechanism neither admits a new profile, workflow, descriptor name, or descriptor shape nor
applies to every descriptor. In particular, it does not admit `node-runtime`; ADR 0099 places that
runtime in `resolvedDependencies`, and ADR 0110's representation distinctions remain unchanged. No
generic registry for arbitrary source-derived pins is mandated. Future use requires a descriptor
admitted by its governing accepted ADR, not a new evidence-authority decision solely for that use.

### Evidence failure, retention, and offline behavior

A descriptor that disagrees with authenticated expectations for its exact represented revision is an
established mismatch. Missing applicable evidence, an unauthenticated manifest, ambiguous revision
binding, or unavailable verification material instead means no authoritative expectation was
established. Specifications must preserve this distinction and define the corresponding diagnostics.

Neither class may fall back to verifier-local values. Offline verification performs no network
lookup and uses retained authenticated bundles and governed trust material under ADR 0069.

Release manifests, signed bundles, and necessary verification material must be retained under
existing release and trust-root policies. No separate freshness registry or revocation system is
added, and existing pinned-root freshness obligations remain unchanged.

### Schema and trust-boundary limits

Because the manifest schema is closed, specifications must choose and document the schema
revision/version treatment before new fields are emitted or accepted. This ADR selects no version
treatment, and pre-release status alone neither requires nor rules out a new major version. A
specification that would conflict with another accepted decision still requires the appropriate ADR.

ADR 0054's predicate URI and ADR 0053's three-job signing boundary and digest-verified handoffs
remain unchanged. No new predicate URI, signer identity, TUF deployment, registry service,
revocation mechanism, or standalone verifier CLI is adopted. ADR 0037's policy, fixtures, and
reference commands remain the consumer surface.

### Temporal and assurance scope

Adoption begins with the first release that implements and issues this contract, not with ADR
acceptance. This manifest mechanism applies prospectively to producer revisions represented by
manifests issued from that release forward; it does not authorize release-agnostic expectations for
arbitrary commits. The project's pre-release state creates no general obligation to reconstruct
evidence for already issued experimental artifacts. Other valid historical statements and
commitments remain unaffected.

The decision supplies revision-correct evidence, not revocation, safety disposition, source
availability, reproducible binary construction, or execution proof. Vulnerability policy remains
separate.

### Consequences

- Good, because exact SHA/profile binding accepts a truthful earlier-revision descriptor while
  rejecting a genuine descriptor copied from another release.
- Good, because release identity and expectations share tagged source inputs, per-release signing,
  and consumer resolution, while retained bundles support offline verification without another
  service.
- Good, because producer actual-used checks, independent manifest authentication, and ADR 0062
  policy intersection remain intact.
- Good, because one mechanism covers represented signer and publish-cli expectations without
  tool-specific authority decisions.
- Neutral, because the manifest is an authenticated release assertion, not independent
  source-to-binary or execution proof. Generation checks and review remain necessary.
- Bad, because the specified manifest is not yet implemented. Its schema, generation, retention,
  resolution, and fixtures must be completed together, and signing incorrect expectations would
  authenticate the error.
- Bad, because pre-adoption manifests do not acquire descriptor evidence. Any future backfill needs
  an explicit basis, although that historical cost is negligible for this pre-release choice.
- Bad, because strict policy intersection can reject an artifact during a legitimate policy
  migration when another admitted source explicitly constrains the same field to a conflicting
  value.

### Confirmation

Confirmation requires the following specification, implementation, and eventual release evidence:

- Specifications define the schema treatment, exact workflow SHA/profile binding, consumer
  resolution, retention, and distinction between established mismatch and unavailable or
  unauthenticated evidence, without widening descriptor or profile scope.
- Generation deterministically derives signer and publish-cli expectations from governed tagged
  inputs and applicable producer rules, failing on inconsistent or ambiguous inputs before signing.
- Existing actual-used producer checks, manifest signing authorities, and digest-verified handoffs
  remain intact.
- Consumers independently authenticate manifest and provenance and apply ADR 0062 to every admitted
  policy source; verifier-local values cannot substitute for manifest evidence.
- With two represented releases using different signer values, truthful provenance from the earlier
  release verifies against its own authenticated manifest after a later signer change. Wrong pairs
  and genuine signer or publish-cli descriptors copied from another release are rejected. Age or a
  vulnerability annotation alone is not a descriptor mismatch; vulnerability disposition is separate
  policy.
- Missing, unauthenticated, and ambiguous evidence remains distinct from mismatch and never falls
  back to verifier-local values. Offline cases use retained authenticated material without network
  access.
- Fixtures cover equivalent signer and publish-cli acceptance and rejection, policy conflict,
  missing and unauthenticated evidence, wrong-release pairs, and retained offline bundles under ADR
  0037's existing verification deliverables.
- Actual signed releases issued from the first implementing release forward demonstrate verification
  of an earlier represented release after a later signer change, with equivalent publish-cli
  evidence and no invented backfill.

## Pros and Cons of the Options

### Authenticated revision catalog distributed with verification support

A catalog maps workflow SHAs and profiles to expectations and is distributed with verification
support.

- Good, because one reviewed catalog can cover many releases and support explicit, evidence-based
  backfill.
- Good, because exact SHA/profile keys can bind expectations and reject pairs from other releases.
- Neutral, because it remains a publisher assertion whose correctness depends on review and
  generation.
- Bad, because it adds a second metadata lifecycle, distribution path, resolution rule, and
  retention obligation alongside the release manifest.
- Bad, because verifier/catalog freshness coupling and generic catalog authority need new controls
  before a requirement beyond the release cadence justifies them.

### Expectations derived from authenticated original source data

The consumer obtains authenticated commit, tree, and required blob objects for the workflow SHA,
then applies descriptor rules without executing historical builder code.

- Good, because consumers can independently recompute expectations and can backfill when source
  objects and applicable rules are available, without executing a historical binary.
- Neutral, because Git membership proves commit, path, and blob content, not which binary was built,
  what executed, or how it behaved.
- Bad, because a full SHA does not guarantee object availability; offline use needs authenticated
  source delivery and retention.
- Bad, because consumers must preserve historical derivation rules for module selection, content
  hashing, and descriptor evolution.
- Bad, because this is a larger consumer requirement than resolving the specified per-release
  bundle, though it remains useful defense in depth.

### Revision-specific expectations in the signed per-release manifest

The specified release manifest carries expectations generated from its tagged source and release
policy.

- Good, because release identity and expectations share generation, source, signing, distribution,
  and the exact consumer-resolution key.
- Good, because independent authentication and ADR 0062 intersection are already selected, and
  retained manifests support earlier represented revisions without current-source lookup.
- Good, because prospective adoption has negligible historical cost for this pre-release choice and
  no general pre-adoption backfill obligation.
- Neutral, because this mechanism remains an authenticated publisher assertion, not source or
  execution proof.
- Bad, because the unimplemented manifest path must gain schema and generation controls; signing an
  incorrect derivation would authenticate the error, and optional backfill would need explicit
  evidence and compatibility decisions.

### Separate signed expectations document with its own signer and revision binding

A dedicated signed document carries expectations with its own signer, schema, and revision binding.

- Good, because the contract can evolve independently, and exact SHA/profile binding with retained
  documents can support offline verification.
- Neutral, because it remains an authenticated publisher assertion requiring correct generation,
  with no inherent execution proof.
- Bad, because it duplicates the manifest's revision binding, authentication, distribution,
  retention, and conflict handling.
- Bad, because a second signer and document create bootstrap and applicability ambiguity without a
  demonstrated need for independent authority.

## More Information

Primary decision references:

- [Issue #156](https://github.com/windlasstech/slsa-builder/issues/156), which selected
  revision-specific descriptor expectations in signed release manifests.
- [ADR 0028](0028-use-sha-pinned-reusable-workflow-builder-identity.md) and
  [ADR 0068](0068-bind-verification-to-immutable-builder-and-source-identities): immutable full-SHA
  workflow identity and certificate binding.
- [ADR 0031](0031-use-sigstore-signed-in-toto-release-manifest.md),
  [ADR 0053](0053-use-three-job-release-manifest-signing-boundary.md),
  [ADR 0054](0054-use-slsa-builder-dev-release-manifest-predicate-uri.md), and the
  [release-manifest specification](../architecture/release-manifest.md): specified manifest
  authority, signing boundary, predicate, tagged generation, and closed schema.
- [ADR 0062](0062-intersect-trusted-producer-policies.md) and
  [ADR 0069](0069-require-rekor-transparency-and-govern-sigstore-trust-root.md): independent trust
  bootstrap, policy intersection, transparency, trust-root governance, and offline verification.
- [ADR 0037](0037-define-initial-verification-deliverables.md): producer-side verification, policy,
  fixtures, and reference commands without a standalone consumer CLI.
- [ADR 0077](0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md),
  [ADR 0113](0113-define-the-common-builderdependencies-descriptor-shape.md), and
  [ADR 0114](0114-record-the-publish-stage-npm-cli-in-builderdependencies.md): actual-used signer
  and publish-cli records, descriptor shapes, and ordinary value-bump rules.
- [ADR 0099](0099-record-the-digest-verified-nodejs-runtime-in-resolveddependencies.md) and
  [ADR 0110](0110-record-deterministic-media-types-for-non-runner-dependencies.md): runtime
  placement and representation distinctions left to later specification work.

External sources constrain the design but do not choose it:

- [SLSA v1.2 Build Provenance, Builder](https://slsa.dev/spec/v1.2/build-provenance#builder) defines
  `builderDependencies` and signer-builder pairing, not this project's expectation source.
- [SLSA v1.2 Verifying Artifacts, Forming Expectations](https://slsa.dev/spec/v1.2/verifying-artifacts#forming-expectations)
  permits producer-defined and source-defined models with authenticated communication; it does not
  mandate release manifests or select between the catalog, manifest, and separate-document
  alternatives.
- [in-toto's validation model](https://github.com/in-toto/attestation/blob/main/docs/validation.md)
  authenticates attesters and subjects before policy evaluation, not predicate truth.
- [Sigstore's bundle documentation](https://docs.sigstore.dev/about/bundle/) defines verification
  material and DSSE content; authenticating a bundle does not prove every assertion correct.
- [Git's object model](https://git-scm.com/book/en/v2/Git-Internals-Git-Objects) supports commit,
  path, and blob membership proofs, not actual binary behavior for any option.
- The [Go Modules Reference](https://go.dev/ref/mod#minimal-version-selection) distinguishes module
  requirements, selection, and the build list from selected module content.
- Go's
  [`dirhash` implementation](https://github.com/golang/go/blob/master/src/cmd/vendor/golang.org/x/mod/sumdb/dirhash/hash.go)
  defines deterministic `h1:` directory-content hashing, not binary execution or metadata authority.

None of these sources requires this release-manifest mechanism. The choice rests on this
repository's specified per-release identity issuance, signing boundary, offline policy, and
consumer-resolution costs.
