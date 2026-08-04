---
parent: Decisions
nav_order: 73
status: accepted
date: 12026-08-04
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0067
    scope:
      "the npm publish step binding proof: expected `dist.integrity` equality remains necessary but
      is no longer sufficient; same-run convergence additionally requires a published attestation
      whose verified run identity equals the current run_id"
  - type: see-also
    target: ADR-0024
  - type: see-also
    target: ADR-0068
  - type: see-also
    target: ADR-0072
---

# Require Published-Attestation Run Identity for npm Same-Run Convergence

## Context and Problem Statement

ADR 0067 defined the npm publish step's binding proof for same-`run_id` convergence as equality of
the expected `dist.integrity` (sha512 SRI) with the published packument, with a fallback that
downloads `dist.tarball` and hashes it locally. ADR 0072 then separated three guarantees for release
assets — production authorship, publication content integrity, and publication-event custody — and
declared custody non-attributable for unsigned GitHub Release assets, because no platform surface
binds such an upload to a run identity.

The npm surface is different. The npm profile always publishes with `npm publish --provenance-file`
(ADR 0024, ADR 0029, ADR 0064), so every version this workflow publishes carries a Sigstore
attestation whose Fulcio certificate embeds the run identity, including the Run Invocation URI
(`.../actions/runs/<run-id>/attempts/<n>`). The npm registry exposes published attestations through
a public read API. Publication-event custody is therefore provable for npm in a way it is not for
release assets.

Without using that proof, the npm step keeps a residual hole that ADR 0067's digest binding cannot
close: if run A's attempt dies around a publish call and, before A's retry, an out-of-band actor
publishes the same version with byte-identical content — possible because packed tarballs are
byte-reproducible for identical source, as the ADR 0067 spike verified — then A's retry observes
matching integrity and would silently adopt a commit it did not perform. The adoption is
content-correct but custody-false, and the published attestation, if any, would name a different run
or not exist at all.

The architecture specification currently makes the related provenance linkage check conditional
("when exposed by npmjs metadata or APIs used by the implementation"), so it cannot serve as a
convergence binding. ADR 0067's npm binding clause states only the integrity-equality requirement.

Should same-`run_id` convergence for the npm publish step require a published attestation whose
verified run identity equals the current `run_id`, in addition to digest equality — and what should
happen when that attestation is absent, foreign, or unreadable?

## Decision Drivers

- The npm profile always publishes with `--provenance-file`; for any version this workflow
  committed, an attestation must exist. Its absence is itself evidence.
- Use the strongest binding the platform actually offers per surface: custody is provable for npm,
  unlike unsigned release assets (ADR 0072), so the npm rule should be stricter, not harmonized
  downward.
- Never silently adopt remote state the run did not produce; for npm this clause can be enforced
  rather than narrowed.
- Fail closed on any doubt: an unreadable attestation surface is `indeterminate`, never a reason to
  proceed.
- Keep convergence recognition-only: the check must never trigger a re-publish or any other
  mutation.
- Registry reads require bounded polling; npm offers no read-after-write consistency guarantee, so
  attestation read-back joins the same documented polling discipline as the integrity read-back.

## Considered Options

- Published-attestation run-identity binding: require a registry-visible attestation for the
  published version whose verified run identity equals the current `run_id`, in addition to
  `dist.integrity` equality.
- Digest-only binding with an accepted-risk note: mirror ADR 0072's custody non-attribution for npm,
  documenting the identical-bytes adoption residual without using the attestation surface.
- Optional attestation check: use the attestation when the API responds, skip it when unavailable.
- Registry signature verification: rely on the packument `dist.signatures` ECDSA registry signatures
  instead of the Sigstore attestation.
- Strict fail-closed without adoption: any pre-existing version fails every execution, including
  same-run retries.

## Decision Outcome

Chosen option: "Published-attestation run-identity binding", because the npm profile's own publish
path guarantees that a genuine own commit always carries an attestation, the registry exposes that
attestation for read-back, and the check converts an unprovable-custody surface into a provable one
at the cost of one additional bounded read.

**Strengthened binding.** A retry attempt of the same `run_id` may classify the npm publish step as
`committed-as-expected` only when all of the following hold:

1. the published packument's `dist.integrity` for the exact version equals the expected sha512 SRI
   of the packed tarball, per the ADR 0067 npm binding, with the documented polling bound;
2. the registry attestation surface exposes at least one attestation for that exact version whose
   Sigstore bundle verifies under the full verification policy, including the ADR 0068 signer and
   source identity binding;
3. that attestation's verified Run Invocation URI carries a run-id equal to the current
   `github.run_id`; any earlier attempt number is acceptable and must not fail the comparison; and
4. the attestation's signed subject binds the npm Package URL subject name and the tarball digests
   per ADR 0064, consistent with the expected tarball.

**Failure classification.** The additional conditions classify as follows:

- no attestation is visible for the published version after the polling bound: `foreign-conflict`,
  because this profile always publishes with `--provenance-file`, so an attestation-free version was
  not committed by this workflow — including by its own earlier attempt, whose protocol violation
  must fail loudly rather than be adopted;
- an attestation is visible and verifies, but its run identity differs from the current
  `github.run_id`, or its subject binding mismatches: `foreign-conflict`;
- the attestation surface cannot be read, the bundle cannot be verified, or observations remain
  contradictory within the polling bound: `indeterminate`, with no further mutation.

**Attempt-component rule.** Run identity comparison for convergence uses only the run-id component
of the Run Invocation URI; the attempt component is ignored, because the committed bundle was signed
by an earlier attempt of the same run. This mirrors the comparison rule ADR 0072 established for
release asset sidecars, and both must be reflected in the verification policy specification.

**No new mutation surface.** The attestation read-back is recognition-only. It never triggers a
re-publish, an attestation regeneration, or any registry mutation, and it does not change the
publish call itself, the three-job graph, or the ADR 0066 mutation segment.

**Scope of amendment.** This ADR amends only the npm publish step binding in ADR 0067. The release
asset and release manifest step bindings, the four outcome states, the retry surface, the
starter-asset exception, and the reporting obligations are unchanged, except as ADR 0072 separately
provides for release assets. Consumer verification policy is unaffected: this binding governs the
workflow's own convergence classification, not third-party verification, which already binds builder
identity per ADR 0068.

### Consequences

- Good, because the npm step's custody residual is closed: an out-of-band identical-bytes publish is
  detected as `foreign-conflict` instead of being silently adopted, and ADR 0067's
  never-silently-adopt clause is enforced for npm rather than narrowed.
- Good, because a self-inflicted protocol violation — a successful publish that somehow lacks its
  attestation — fails loudly instead of being normalized by convergence.
- Good, because no new mutation, signing step, or infrastructure is introduced; the change is one
  additional bounded read against an existing public surface.
- Bad, because the attestation read-back adds a dependency on a registry API whose exact response
  schema and availability must be spike-verified and pinned in the specification; an outage of that
  surface now yields `indeterminate` and blocks convergence even when integrity matches.
- Bad, because the specification must grow the attestation endpoint contract, response schema,
  polling integration, and the run-id-component comparison rule, plus fixtures for each failure
  classification.
- Neutral, because the conditional provenance linkage clause in the npm provenance and publish
  specification becomes mandatory for the convergence path; its use for post-publish reporting is
  unchanged.

### Confirmation

This decision is confirmed when:

- a controlled spike verifies that the npm registry attestation surface exposes, for a
  `--provenance-file` publish, a Sigstore bundle whose Fulcio certificate carries the Run Invocation
  URI of the publishing run, and the endpoint, response schema, and error behavior are pinned in the
  architecture specification;
- the npm provenance and publish specification defines the strengthened binding, the three failure
  classifications, the polling integration, and the run-id-component comparison rule, tracing to
  this ADR, and the verification policy specification reflects the attempt-component rule;
- fixtures demonstrate: a same-`run_id` retry converging with a matching attestation; a published
  version without any attestation classifying `foreign-conflict`; an attestation with a different
  run identity classifying `foreign-conflict`; an attestation API or verification failure
  classifying `indeterminate`; and a prior-attempt bundle (attempt component mismatch, run-id match)
  converging;
- the conditional linkage language in the specification is replaced by the mandatory convergence
  check above;
- the outcome report records which evidence — integrity only, or integrity plus attestation run
  identity — supported each `committed-as-expected` classification.

## Pros and Cons of the Options

### Published-attestation run-identity binding

- Good, because it is the only option that fully closes the identical-bytes adoption hole on a
  surface where the proof actually exists, and it does so with a read-only check.
- Good, because it makes the npm rule strictly stronger than the release asset rule in proportion to
  what each platform can prove, instead of harmonizing both down to the weaker surface.
- Bad, because it adds a live registry API dependency to the convergence path, with schema-pinning
  and spike obligations, and turns attestation-surface outages into `indeterminate` outcomes.

### Digest-only binding with an accepted-risk note

- Good, because it needs no new reads, keeps the ADR 0067 binding untouched, and the residual
  requires both registry publish rights and byte-identical content.
- Bad, because it knowingly leaves an enforceable guarantee unenforced: unlike release assets, npm
  offers the custody proof, so waiving it is a weaker posture than the platform allows, and it is
  harder to justify to verifiers than the ADR 0072 narrowing, which had no alternative.

### Optional attestation check

- Good, because it degrades gracefully when the attestation API is unreachable.
- Bad, because "use it when convenient" evidence cannot ground a security classification: an
  attacker-influenced or flaky read path would silently demote the binding to digest-only exactly
  when the check matters, violating the fail-closed driver.

### Registry signature verification

- Good, because `dist.signatures` registry signatures authenticate the packument response itself and
  complement transport integrity.
- Bad, because registry signatures prove the registry served the data, not which workflow run
  published the version; they carry no run identity and cannot ground a same-run binding.

### Strict fail-closed without adoption

- Good, because it has no silent-adoption surface at all.
- Bad, because it resurrects the permanent version-loss failure mode that ADR 0067 was written to
  eliminate: any transient fault after a committed publish would make the trusted path unusable for
  that version.

## More Information

- Run-identity convergence and the original npm binding: ADR 0067; release asset ownership and the
  custody/non-custody separation: ADR 0072; verifier identity binding including the Run Invocation
  URI: ADR 0068; OIDC trusted publishing: ADR 0024; npm PURL subject and digest binding: ADR 0064.
- npm provenance and public attestation read-back:
  <https://docs.npmjs.com/generating-provenance-statements> and
  <https://docs.npmjs.com/cli/v11/commands/npm-publish/>; registry signatures (a complementary,
  non-run-bound surface): <https://docs.npmjs.com/about-registry-signatures>.
- Fulcio run identity certificate extensions:
  <https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md>; GitHub Actions OIDC `run_id` and
  `run_attempt` claims: <https://docs.github.com/en/actions/reference/security/oidc>.
