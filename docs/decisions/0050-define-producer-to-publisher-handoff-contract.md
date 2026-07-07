---
parent: Decisions
nav_order: 50
status: accepted
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Define the Producer-to-Publisher Handoff Contract

## Context and Problem Statement

ADR 0049 separated artifact production from GitHub Release asset publication. Ecosystem-specific
producer profiles now own source-to-artifact builds and provenance, while the GitHub Release asset
publisher profile verifies an already-produced artifact and publishes it to an existing GitHub
Release.

That separation creates a new contract surface. The publisher profile must receive artifact bytes,
the release tag, and producer-generated SLSA provenance for the exact final release asset. The
workflow artifact, download path, job output, or other transport used to hand the bytes to the
publisher is not itself a trust root. The publisher must verify digest and producer provenance
before mutating the GitHub Release, then distribute the producer provenance unchanged according to
ADR 0051.

What information must a producer profile hand to the GitHub Release asset publisher profile, and
what must the publisher verify before upload?

## Decision Drivers

- Make the publisher profile safe to compose with multiple ecosystem-specific producer profiles.
- Treat byte transport between producer and publisher jobs as untrusted until digest and provenance
  verification succeeds.
- Require a complete, verifier-friendly interface for the publication profile's
  `externalParameters`.
- Preserve ADR 0039's one-asset publication unit and ADR 0045's final release asset subject name.
- Keep upstream source-to-artifact provenance mandatory for the production publisher path.
- Require producer provenance to describe the final GitHub Release asset name and exact bytes, so
  the publisher does not need a separate signed name-mapping predicate.
- Fail closed before uploading to GitHub Release when any handoff input is missing, ambiguous, or
  mismatched.
- Keep future lower-assurance ingestion or mirror workflows separate from the production publisher
  identity.

## Considered Options

- Require a digest- and provenance-verified producer handoff contract.
- Pass only artifact bytes and recompute the digest in the publisher.
- Pass artifact bytes plus an expected digest, but do not require upstream producer provenance.
- Use a producer-generated manifest as the only handoff input.
- Let each ecosystem producer profile define its own publisher input shape.

## Decision Outcome

Chosen option: "Require a digest- and provenance-verified producer handoff contract", because the
publisher profile must not treat arbitrary transported bytes as production release assets unless it
can bind those bytes to trusted upstream producer provenance and the requested GitHub Release asset
slot.

The production GitHub Release asset publisher profile should accept a fixed, profile-owned handoff
contract from producer profiles. The exact input names belong in the architecture specification, but
the contract must include at least these semantic fields:

- an artifact handle that lets the publisher retrieve the produced bytes within the same workflow
  run or approved workflow boundary;
- the expected artifact digest as lowercase SHA-256, plus any tool-boundary representation such as
  `sha256:<hex>` where required;
- the final GitHub Release asset name to upload;
- the release tag identifying the existing Git tag and existing GitHub Release target;
- the producer-generated SLSA provenance bundle or a locator from which the publisher can retrieve
  and verify the bundle;
- the expected upstream producer `builder.id` or trusted builder policy;
- the expected upstream producer `buildType` or trusted build type policy;
- the expected upstream subject digest and subject name, both of which must match the final GitHub
  Release asset;
- the source repository and resolved source revision when they are needed for release policy or
  verifier expectations;
- optional sidecar and linked-artifact publication settings selected by earlier ADRs.

The artifact handle is a transport handle only. It may identify a workflow artifact, file path,
artifact ID, job output, or future internal handoff format, but it must not be treated as proof that
the artifact is trusted. The publisher must retrieve the bytes, compute their digest, and compare
the result with the expected digest before upload.

The upstream producer provenance must be verified before publication. Verification should include at
least:

- the upstream attestation signature and trusted signer identity;
- the upstream `predicateType` expected for source-to-artifact SLSA provenance;
- the upstream `builder.id` and `buildType` allowed by the caller's or profile's trusted producer
  policy;
- the upstream subject digest matching the expected artifact digest and the bytes to upload;
- the upstream subject name exactly matching the final GitHub Release asset name;
- source repository, source revision, release tag, package or artifact identity, and other producer
  `externalParameters` required by the selected producer policy;
- absence of unrecognized or unexpected producer `externalParameters` where the policy requires
  strict matching.

The publisher profile should not sign independent publication evidence in the default production
path. Instead, it should publish or expose the producer-generated provenance bundle unchanged after
successful verification. If a future workflow needs signed publication evidence or a subject-name
mapping predicate, that workflow requires a separate ADR, predicate model, and verifier guidance.

The publisher must fail before GitHub Release mutation when:

- the artifact bytes cannot be retrieved;
- the computed digest differs from the expected digest;
- the release tag or target GitHub Release does not exist;
- the final release asset name is invalid or already present under the target release;
- upstream producer provenance is missing, unsigned, unverifiable, untrusted, or mismatched;
- the upstream subject digest does not match the bytes to upload;
- the upstream subject name differs from the final release asset name;
- the producer policy does not allow the upstream `builder.id`, `buildType`, source, release ref, or
  external parameters.

The production publisher profile must not expose an option to bypass upstream provenance
verification. A future lower-assurance upload or mirror workflow may accept raw bytes, rename
artifacts, or sign publication evidence, but it must use a distinct workflow name, evidence model,
and verifier guidance.

### Consequences

- Good, because the publisher profile can safely compose with many ecosystem producer profiles
  without trusting their transport mechanism.
- Good, because release publication is bound to both artifact digest and upstream source-to-artifact
  provenance.
- Good, because consumers verify the same producer provenance that the publisher verified before
  upload.
- Good, because requiring the producer subject name to match the final release asset name avoids a
  separate signed rename or mapping predicate.
- Neutral, because the publisher profile needs a policy format for trusted producer builders and
  build types.
- Neutral, because publisher-side verification is an operational gate rather than independent signed
  evidence.
- Bad, because producer profiles must expose stable handoff outputs and upstream provenance
  locators.
- Bad, because producer profiles must produce final release asset names before provenance signing.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, tests, and
documentation define:

- the public producer-to-publisher input and output names;
- artifact handle retrieval rules and digest calculation behavior;
- mandatory expected SHA-256 digest comparison before upload;
- upstream producer provenance locator or bundle requirements;
- trusted producer policy fields for `builder.id`, `buildType`, signer identity, source, release
  ref, subject, and external parameters;
- rejection of producer subject names that differ from the final GitHub Release asset name;
- producer provenance distribution rules and outputs, as selected by ADR 0051;
- failure behavior for missing artifact bytes, digest mismatch, missing or invalid upstream
  provenance, invalid release target, duplicate asset name, and release upload failure;
- verifier guidance for checking producer provenance against the downloaded GitHub Release asset;
- tests that prove raw caller-produced artifacts cannot reach the production upload path without
  successful upstream provenance verification.

Implementation review should verify that no workflow artifact name, job output, file path, GitHub
artifact ID, or caller-provided digest can bypass upstream provenance verification, and that the
exact bytes uploaded to the GitHub Release match the expected digest, the verified upstream subject
digest, and the verified upstream subject name.

## Pros and Cons of the Options

### Require a digest- and provenance-verified producer handoff contract

Producer profiles expose artifact bytes, digest, upstream provenance, and policy-relevant identity
metadata. The publisher verifies all of them before upload and republishes the producer provenance
unchanged.

- Good, because the contract matches ADR 0049's separation of production and publication.
- Good, because the publisher can reject arbitrary bytes even when they arrive through a valid
  workflow artifact handle.
- Good, because producer and publisher responsibilities are explicit and independently verifiable.
- Good, because the handoff is stable enough for multiple ecosystem producer profiles.
- Good, because the publisher does not need to define or sign a new attestation format.
- Neutral, because policy configuration is required to describe which producers are trusted.
- Bad, because this is the most implementation-heavy initial handoff option.
- Bad, because producers must align their SLSA subject name with the final GitHub Release asset
  name.

### Pass only artifact bytes and recompute the digest

The publisher receives or downloads bytes, computes their digest, and uploads them without an
expected digest or upstream producer provenance.

- Good, because the publisher implementation is small.
- Good, because it can publish any file-like artifact.
- Bad, because the publisher has no trusted source-to-artifact build evidence for the bytes.
- Bad, because a compromised or incorrect caller can substitute arbitrary bytes before publication.
- Bad, because this collapses the production publisher into a raw upload helper rather than a
  trusted release asset publisher.

### Pass artifact bytes plus expected digest, without producer provenance

The publisher verifies the transported bytes against an expected digest but does not verify how
those bytes were produced.

- Good, because it prevents transport corruption or accidental artifact mixups.
- Good, because it is compatible with many existing workflows.
- Bad, because the expected digest itself can be caller-controlled without a trusted provenance
  root.
- Bad, because SLSA source-to-artifact claims remain absent.
- Bad, because consumers could mistake release upload success for build provenance.

### Use a producer-generated manifest as the only handoff input

The producer emits a manifest that names the artifact, digest, release tag, upstream provenance, and
publication metadata. The publisher accepts only the manifest and retrieves everything from it.

- Good, because one manifest can make the handoff easy to pass through workflow boundaries.
- Good, because the manifest can be reused by release orchestration.
- Neutral, because this may become useful as the stable API after producer profiles mature.
- Bad, because the manifest itself must be authenticated or linked to producer provenance to avoid
  becoming a caller-controlled bundle of claims.
- Bad, because it adds a manifest schema before the project has implemented any
  producer-to-publisher handoff.

### Let each ecosystem producer define its own publisher inputs

Every producer profile exposes whatever outputs it wants, and the publisher adapts to each
ecosystem.

- Good, because each producer can optimize its own ergonomics.
- Bad, because the publisher becomes a matrix of ecosystem-specific adapters.
- Bad, because verifier guidance and tests fragment across producer profiles.
- Bad, because adding a new ecosystem profile would require changing the publisher's public
  contract.

## More Information

This decision follows ADR 0049 and replaces the unresolved build-or-ingest portion of the earlier
GitHub Release asset ADR sequence. It depends on ADR 0039's one-asset unit, ADR 0043's existing
release requirement, ADR 0045's final release asset subject name, ADR 0046's selected-asset digest
semantics, ADR 0048's linked-artifact opt-in policy, and ADR 0051's producer provenance distribution
model as publication-profile rules.

This decision does not define the first ecosystem-specific producer profile, the exact JSON schema
for a future handoff manifest, a future publisher predicate type, or a lower-assurance raw upload
workflow. Those require architecture specifications or future ADRs.

Reference points considered:

- SLSA v1.2 Build Provenance treats `externalParameters` as the complete external interface to the
  build or operation and expects verifiers to reject unexpected fields.
- SLSA v1.2 Build L3 requires provenance fields to be generated or verified by the trusted build
  platform, except permitted tenant-controlled subject information.
- `slsa-github-generator` generic workflows accept caller-provided subject digests, while ecosystem
  builders own artifact production semantics.
- GitHub reusable workflows exchange typed inputs, secrets, outputs, and artifacts; those transport
  mechanisms are not by themselves provenance or trust roots.
- GitHub `actions/attest` can bind a subject name and digest to a custom predicate and expose a
  local Sigstore bundle plus GitHub attestation storage handles for downstream verification.
