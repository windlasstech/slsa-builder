---
parent: Decisions
nav_order: 61
status: accepted
date: 12026-07-30
decision-makers: Yunseo Kim
---

# Reject Duplicate JSON Members in Signed SLSA Statements

## Context and Problem Statement

The common SLSA provenance contract, npm provenance profile, publisher handoff, and verification
specifications already require malformed JSON and duplicate object member names to fail closed in
some places. The decision basis and exact scope were still incomplete: SLSA v1.2 and in-toto define
the Statement and predicate model, but they do not define how every verifier must handle duplicate
JSON object member names in a signed payload.

Duplicate JSON members are dangerous for signed supply-chain metadata because different parsers may
interpret the same signed bytes differently. RFC 8259 says object member names should be unique and
warns that behavior is unpredictable when they are not. RFC 8785 JSON Canonicalization Scheme (JCS),
which this project already uses for release manifest predicates, requires I-JSON input and forbids
duplicate property names. DSSE signs serialized payload bytes, so a valid signature proves byte
integrity but does not prove that the JSON payload has one unambiguous semantic interpretation.

Should the project reject duplicate JSON object member names throughout signed SLSA Statements, only
reject duplicates in selected top-level or known fields, rely on default ecosystem parser behavior,
or require all signed Statements to use RFC 8785 JCS canonical JSON?

## Decision Drivers

- Keep verifier policy deterministic across Go, JavaScript, policy engines, and downstream
  consumers.
- Prevent parser-differential attacks where one component validates one value while another
  component consumes a different duplicate value.
- Preserve SLSA and in-toto compatibility without requiring every signed Statement payload to be JCS
  canonical JSON.
- Preserve the DSSE model that verifies serialized payload bytes before parsing them according to
  the payload type.
- Keep existing release manifest JCS canonicalization rules intact.
- Fail closed before semantic SLSA validation when the signed payload is structurally ambiguous.
- Make fixture expectations precise enough to test nested objects inside the Statement and
  predicate.

## Considered Options

- Reject duplicate JSON object member names at every depth of the signed SLSA Statement payload.
- Reject duplicate JSON object member names only in top-level Statement fields.
- Reject duplicate JSON object member names only for known schema fields.
- Accept default JSON parser behavior for duplicate member names.
- Require RFC 8785 JCS canonical JSON for every signed SLSA Statement payload.

## Decision Outcome

Chosen option: "Reject duplicate JSON object member names at every depth of the signed SLSA
Statement payload", because it removes ambiguity from signed provenance without imposing full JCS
canonicalization on SLSA and in-toto payloads.

Verifiers and profile-internal validation steps must reject duplicate JSON object member names at
every depth of the decoded signed in-toto Statement payload before ordinary semantic SLSA
validation. The check applies to all JSON objects contained in the Statement, including `_type`,
`subject`, `predicateType`, `predicate`, the SLSA provenance predicate, nested objects under
`buildDefinition`, `externalParameters`, `internalParameters`, `resolvedDependencies`, `runDetails`,
`builder`, and `metadata`, and any extension fields present in those objects. Arrays do not have
member names, but any JSON object contained in an array is checked by the same rule.

Duplicate detection must be performed over the decoded signed payload bytes using a parser or
scanner that preserves object member occurrence. Implementations must not first decode into a lossy
map or struct representation and then attempt to infer whether duplicates were present. Duplicate
member names are compared after JSON string unescaping, so two syntactically different property
spellings that decode to the same string are duplicates.

The duplicate-member check is a structural validity check, not a policy match. A duplicate member
makes the Statement malformed. Verification must fail closed before checking SLSA `predicateType`,
subject digests, `builder.id`, `buildType`, `externalParameters`, release-asset publisher inputs, or
other semantic policy conditions.

This decision applies to common signed SLSA provenance Statements consumed by the project, including
producer provenance used by npm publish verification and GitHub Release asset publisher
verification. It does not require every SLSA Statement payload to be serialized in RFC 8785 JCS
canonical form.

Release manifest predicate canonicalization remains governed by the RFC 8785 JCS rules selected for
release manifests. When a release manifest predicate is wrapped in an in-toto Statement, the
manifest predicate's trust digest and canonical JSON comparison continue to use the release manifest
spec's JCS rules, while the Statement wrapper and signed payload still fail closed if duplicate JSON
object member names are present.

### Consequences

- Good, because verifiers and downstream policy engines receive one unambiguous JSON object graph.
- Good, because signed payload verification cannot be combined with last-value-wins parsing to hide
  policy-relevant fields.
- Good, because the decision follows RFC 8259 interoperability guidance and aligns with JCS/I-JSON's
  stricter treatment of cryptographic JSON input.
- Good, because release manifest JCS canonicalization stays isolated to the manifest predicate
  digest contract rather than becoming a global SLSA Statement serialization requirement.
- Neutral, because generated provenance from maintained tooling should not contain duplicate object
  members under ordinary operation.
- Bad, because implementations need duplicate-aware JSON scanning rather than only default map-based
  JSON decoding.
- Bad, because ambiguous third-party attestations that some permissive JSON parsers accept will be
  rejected by this project's verifier policy.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, fixtures, and
documentation define:

- duplicate JSON object member names as malformed signed SLSA Statement payloads;
- duplicate detection before semantic validation of `predicateType`, subject digests, `builder.id`,
  `buildType`, `externalParameters`, and publisher policy;
- recursive duplicate detection for every JSON object depth, including objects inside arrays;
- duplicate-name comparison after JSON string unescaping;
- verifier and profile-internal validation paths that inspect signed payload bytes before lossy JSON
  map or struct decoding;
- failure taxonomy and fixture cases for top-level Statement duplicates, nested predicate
  duplicates, duplicate extension fields, and escaped property names that decode to the same member
  name;
- release manifest documentation that keeps RFC 8785 JCS canonicalization as the manifest predicate
  digest rule while still rejecting duplicate members in the signed Statement wrapper.

Implementation review should verify that no production verifier path relies on host-language default
JSON duplicate handling for signed SLSA Statement payloads.

## Pros and Cons of the Options

### Reject duplicate members at every depth

Every JSON object in the signed Statement payload is scanned before semantic validation. Any
duplicate member name makes the Statement malformed.

- Good, because it prevents parser-differential and field-smuggling ambiguity throughout the
  complete policy-relevant payload.
- Good, because it gives verifiers, fixtures, and downstream consumers one deterministic rule.
- Good, because it aligns with RFC 8259's interoperability warning and RFC 8785's stricter
  cryptographic JSON input model.
- Neutral, because conforming generators that produce objects programmatically should not emit
  duplicate members.
- Bad, because duplicate-aware JSON scanning is additional implementation work.

### Reject duplicates only in top-level Statement fields

Only `_type`, `subject`, `predicateType`, `predicate`, and other top-level Statement members are
checked for duplicates. Nested predicate objects use ordinary parser behavior.

- Good, because it protects the fields that route the Statement to the right schema.
- Good, because it is simpler than scanning the complete payload.
- Bad, because the most important SLSA policy fields are nested under `predicate`.
- Bad, because an attacker could still create ambiguity in `builder.id`, `buildType`,
  `externalParameters`, or release-specific nested fields.

### Reject duplicates only for known schema fields

The verifier checks duplicate occurrences of recognized Statement and SLSA predicate fields but does
not reject duplicates in unknown extension objects.

- Good, because it focuses on fields currently used by verifier policy.
- Good, because it appears to preserve more room for in-toto extension fields.
- Bad, because extension fields may become policy-relevant for downstream consumers.
- Bad, because the implementation must distinguish known and unknown object scopes while still
  preserving duplicate occurrence information.
- Bad, because it leaves signed payloads with ambiguous semantics.

### Accept default JSON parser behavior

The verifier accepts whatever the implementation's standard JSON parser accepts, such as
last-value-wins behavior in common Go or JavaScript parsing paths.

- Good, because implementation is simple and maximally permissive for externally produced JSON.
- Good, because it matches many ecosystem tools that do not document duplicate-member rejection.
- Bad, because parser behavior differs across implementations and policy engines.
- Bad, because signed supply-chain metadata can be validated under one interpretation and consumed
  under another.
- Bad, because tightening this later would be a compatibility break for ambiguous attestations.

### Require JCS canonical JSON for every signed Statement

Every signed SLSA Statement payload must be serialized as RFC 8785 JCS canonical JSON. Duplicate
members are rejected because JCS input forbids them.

- Good, because it provides the strongest deterministic byte representation.
- Good, because it aligns all signed JSON payloads with the release manifest predicate's
  canonicalization model.
- Bad, because SLSA, in-toto, DSSE, `actions/attest`, and `slsa-github-generator` do not require all
  Statement payloads to be JCS canonical JSON.
- Bad, because it would reject otherwise valid SLSA ecosystem attestations based on serialization
  details that are not part of the SLSA provenance schema.
- Bad, because it conflates release manifest predicate digest canonicalization with general SLSA
  Statement acceptance.

## More Information

This decision follows ADR 0029, ADR 0035, ADR 0037, ADR 0050, ADR 0052, ADR 0054, and ADR 0055. It
records the duplicate JSON member policy for common signed SLSA Statement payloads before
implementation begins.

Reference points considered:

- SLSA v1.2 provenance uses in-toto Statements and the `https://slsa.dev/provenance/v1` predicate,
  but does not define duplicate JSON member handling for every verifier.
- in-toto attestation parsing rules require consumers to ignore unrecognized fields for compatible
  minor-version evolution, but do not make duplicate object member names safe.
- DSSE signs the serialized payload bytes and then requires verifiers to parse those bytes according
  to `payloadType`; a valid DSSE signature does not remove JSON semantic ambiguity.
- RFC 8259 says JSON object member names should be unique and warns that behavior is unpredictable
  when they are not unique.
- RFC 8785 JCS constrains canonical JSON input to I-JSON and requires JSON objects not to exhibit
  duplicate property names.
- GitHub `actions/attest` and SLSA ecosystem tools generate and verify signed in-toto/SLSA
  attestations, but their public documentation does not establish a portable duplicate-member
  acceptance policy for this project's verifier contracts.
