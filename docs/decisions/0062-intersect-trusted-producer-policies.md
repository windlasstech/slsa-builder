---
parent: Decisions
nav_order: 62
status: accepted
date: 12026-07-30
decision-makers: Yunseo Kim
---

# Intersect Trusted Producer Policies

## Context and Problem Statement

ADR 0031 selected a signed release manifest as the canonical machine-verifiable mapping from
human-readable Windlass release versions to immutable workflow SHAs, producer `builder.id` values,
and producer `buildType` values. ADR 0050 later required the GitHub Release asset publisher to
verify upstream producer provenance against a trusted producer policy before publication. The
verification policy specification also allows consumers to use explicit verifier policy when
checking published artifacts.

That leaves one unresolved trust-boundary question: when an authenticated signed release manifest
and an explicit verifier policy are both present and they disagree about trusted producer identity,
workflow SHA, `builder.id`, `buildType`, signer identity, source, release ref, subject, or
`externalParameters`, which policy wins?

SLSA v1.2 verification is expectation-driven: verifiers authenticate provenance and then compare the
provenance fields against configured roots of trust and expected values. `slsa-verifier`, GitHub
`gh attestation verify`, Sigstore `cosign`, and similar tools follow the same pattern by taking
verifier-supplied expectations for source identity, signer identity, workflow identity, builder
identity, predicate type, or annotations. A valid signature proves who made a statement and that the
statement was not changed; it does not by itself grant that statement authority to relax the local
verifier's trust policy.

Should the verifier prefer explicit verifier policy, prefer the signed release manifest, require
both policies to agree, accept either policy, or provide an override mode when these trusted
producer policy sources conflict?

## Decision Drivers

- Preserve verifier control over its configured roots of trust and expected producer policy.
- Preserve the signed release manifest as authenticated release-to-workflow metadata without letting
  it widen local trust by itself.
- Fail closed when multiple trust sources cannot be reconciled.
- Avoid last-writer-wins or precedence rules that silently convert policy drift into trust
  expansion.
- Align with SLSA's expectation-check model and Sigstore/GitHub tooling that compare signed claims
  against verifier-supplied constraints.
- Keep producer-side publisher gates and consumer-side verifier guidance consistent.
- Provide precise diagnostics for policy drift without publishing or accepting artifacts under an
  ambiguous trust basis.

## Considered Options

- Apply the intersection of explicit verifier policy and authenticated release manifest policy.
- Use explicit verifier policy and ignore release manifest producer policy.
- Let explicit verifier policy override release manifest producer policy.
- Let authenticated release manifest policy override explicit verifier policy.
- Accept producer provenance when either policy source allows it.
- Provide a break-glass override mode for policy conflicts.

## Decision Outcome

Chosen option: "Apply the intersection of explicit verifier policy and authenticated release
manifest policy", because neither policy source should be able to widen trusted producer authority
without the other independently allowing the same producer identity and parameters.

When both an explicit verifier policy and a signed release manifest policy are present, verifiers
and producer-side publisher gates must compute the effective trusted producer policy as the
intersection of the two policy sources. Producer provenance is trusted only when every
policy-relevant value is allowed by both sources. Policy-relevant values include signer identity,
source repository, source revision, release ref, workflow path, workflow SHA, `builder.id`,
`buildType`, SLSA `predicateType`, subject name, subject digest, and required or forbidden
`externalParameters`.

A signed release manifest policy is eligible for intersection only after the release manifest bundle
has been verified against an independently configured local trust root: Sigstore root, expected
release manifest signer identity, expected release manifest predicate type, supported schema
version, expected release tag, release commit SHA, and manifest generation invariants. The manifest
must not bootstrap trust in its own signer, trust root, predicate type, schema version, or policy
authority.

If a value is missing from either policy source, the verifier must not treat the omission as a
wildcard unless the policy schema explicitly marks that field as intentionally unconstrained and the
other policy source still allows the observed value. For the initial production policy, missing
producer identity, workflow SHA, `builder.id`, `buildType`, source identity, release ref, subject,
or strict `externalParameters` constraints make the effective policy unsatisfied.

If the two policy sources conflict or their intersection is empty, verification must fail closed.
The verifier must not choose one source by precedence, use last-writer-wins behavior, accept either
source independently, or silently downgrade to a looser policy.

The failure result should identify the conflicting policy source and field, such as an explicit
policy allowing one `builder.id` while the signed release manifest maps the selected release to a
different `builder.id`. Diagnostics may help operators update policy, but they must not change the
trust decision.

This decision applies to producer-side publication gates, such as the GitHub Release asset publisher
verifying upstream producer provenance before upload, and to consumer-side verification guidance for
artifacts and release assets. A lower-assurance or emergency override path, if ever needed, requires
a future ADR with a distinct mode name, audit requirements, and verifier-visible output that
prevents it from being confused with the default production policy.

### Consequences

- Good, because no single signed metadata document or local configuration file can expand producer
  trust alone.
- Good, because release manifest policy acts as an authenticated additional constraint while
  explicit verifier policy remains under verifier control.
- Good, because policy drift is detected before publication or artifact acceptance.
- Good, because the rule matches SLSA's expectation-check model and common Sigstore/GitHub verifier
  behavior.
- Good, because diagnostics can point to exact conflict fields instead of hiding drift behind
  precedence.
- Neutral, because operators must keep explicit policy and release manifest mappings synchronized.
- Bad, because some otherwise valid attestations will fail during policy migration until both policy
  sources are updated consistently.
- Bad, because verifier implementations must model policy source provenance and field-level conflict
  reporting rather than flattening all policy inputs into one map.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, fixtures, and
documentation define:

- release manifest policy verification before using manifest producer entries;
- explicit verifier policy and signed release manifest policy as independent policy sources;
- effective trusted producer policy as the intersection of both sources when both are present;
- rejection when either source does not allow the observed producer signer identity, source,
  workflow path, workflow SHA, `builder.id`, `buildType`, release ref, subject, or required
  `externalParameters`;
- rejection when policy intersections are empty or ambiguous;
- no implicit wildcard behavior for missing producer policy fields in the initial production policy;
- no precedence, last-writer-wins, or either-source-allowed acceptance in the default production
  policy;
- fixture cases for matching policies, manifest-only mismatch, explicit-policy-only mismatch,
  missing policy fields, and empty intersections;
- diagnostics that name the conflicting policy source and field.

Implementation review should verify that no producer-side publisher gate, consumer verification
guide, or reference command treats signed release manifest policy as permission to relax explicit
verifier policy, or treats explicit verifier policy as permission to bypass an authenticated release
manifest constraint in the default production path.

## Pros and Cons of the Options

### Intersect explicit verifier policy and authenticated manifest policy

The verifier accepts producer provenance only when both policy sources independently allow the
observed producer identity and parameters. Any conflict fails closed.

- Good, because it preserves monotonic security: adding a policy source can only narrow trust.
- Good, because it gives the signed release manifest real effect without letting it replace local
  verifier judgment.
- Good, because it detects release metadata drift, local configuration drift, and accidental
  producer mapping mismatches.
- Bad, because it is stricter than either-source acceptance and can create operational failures
  during migration.
- Bad, because it requires clearer policy schemas and conflict diagnostics.

### Use explicit verifier policy and ignore release manifest producer policy

The verifier trusts only the locally configured explicit policy after verifying provenance and does
not use signed release manifest producer mappings as a constraint.

- Good, because it is simple and keeps all trust decisions local.
- Good, because operators can update policy without waiting for release metadata changes.
- Bad, because it wastes the signed release manifest's authenticated version-to-workflow mapping.
- Bad, because consumers can accidentally trust a producer that the project release manifest did not
  authorize for that release.

### Let explicit verifier policy override manifest policy

When both policy sources are present, the explicit verifier policy replaces conflicting signed
release manifest producer constraints.

- Good, because operators can respond quickly to emergency policy changes.
- Good, because local policy remains the final authority.
- Bad, because it can relax signed release metadata constraints without a distinct emergency mode.
- Bad, because consumers may believe they verified the release manifest mapping when they actually
  bypassed it.

### Let manifest policy override explicit verifier policy

The signed release manifest decides trusted producer policy whenever it is present, even if explicit
verifier policy disagrees.

- Good, because release metadata and producer mappings are distributed atomically with the release.
- Good, because consumers need less local configuration after trusting the release manifest signer.
- Bad, because a compromised or mistaken release manifest signer can widen producer trust against
  local policy.
- Bad, because signature validity proves statement origin and integrity, not authority to rewrite a
  verifier's local roots of trust.
- Bad, because it conflicts with verifier-supplied expectation patterns in SLSA, Sigstore, GitHub
  artifact attestations, and `slsa-verifier`.

### Accept either policy source independently

The verifier accepts producer provenance when either explicit verifier policy or signed release
manifest policy allows it.

- Good, because it maximizes compatibility during policy transitions.
- Good, because either policy source can recover from the other being stale.
- Bad, because it is a trust union and therefore widens acceptance whenever a second source is
  added.
- Bad, because an attacker only needs to satisfy the weaker or stale policy source.
- Bad, because policy drift becomes silent instead of actionable.

### Provide a break-glass override mode

The default verifier fails on conflict, but an explicitly named emergency mode can bypass one policy
source with audit output.

- Good, because it can support incident response, metadata publication mistakes, or temporary
  migration failures.
- Good, because a separate mode can make risk visible instead of hiding it in normal precedence
  rules.
- Bad, because it adds product and documentation complexity before the project has a stable default
  verifier.
- Bad, because override paths are easy to normalize operationally unless guarded by strong audit and
  policy requirements.

## More Information

This decision follows ADR 0028, ADR 0031, ADR 0049, ADR 0050, ADR 0053, and ADR 0054. It decides
trusted producer policy conflict resolution only. It does not change the release manifest predicate
URI, the release manifest signing boundary, the producer-to-publisher handoff fields, or the SLSA
provenance schema.

Reference points considered:

- SLSA v1.2 verification recommends configuring roots of trust and comparing provenance against
  expected `builder.id`, `buildType`, and `externalParameters` values.
- SLSA v1.2 describes expectation formation as verifier, producer, ecosystem, or source controlled,
  but does not define a universal precedence algorithm for conflicting policy sources.
- SLSA Verification Summary Attestations identify the applied policy with `policy.uri` and
  preferably `policy.digest`, but consumers still decide whether to trust the verifier and policy.
- GitHub `gh attestation verify` validates actor identity against verifier-supplied repository,
  owner, signer workflow, certificate identity, source ref, source digest, and predicate type
  constraints.
- GitHub's attestation verification documentation distinguishes certificate and timestamp fields
  that workflows cannot manipulate from predicate contents that compromised workflows may falsify.
- `slsa-verifier` verifies signatures and then compares builder, source, tag, package, and workflow
  input values against verifier-supplied expectations.
- Sigstore and `cosign` verification require verifier-selected identity, issuer, key, or annotation
  constraints; signature verification alone is not equivalent to policy acceptance.
- Windlass organization security policy requires signed provenance, SHA-pinned workflow identities,
  OIDC-preferred authentication, least-privilege workflow permissions, and fail-closed release
  integrity gates.
