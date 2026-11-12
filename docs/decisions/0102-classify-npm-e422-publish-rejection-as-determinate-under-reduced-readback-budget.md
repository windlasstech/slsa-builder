---
parent: Decisions
nav_order: 102
status: accepted
date: 12026-11-04
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0076
    scope:
      "the tier-2 first-mutation classification set for the npm publish invocation: an E422
      rejection is read back under a reduced polling budget and fails as a determinate pre-commit
      failure when no publication appears within that budget and at least one clean absence is
      observed; the 401/403 and E404/ENEEDAUTH mappings and the full-budget read-back doctrine for
      all other outcomes remain in force"
  - type: see-also
    target: ADR-0067
  - type: see-also
    target: ADR-0073
  - type: see-also
    target: ADR-0101
---

# Classify npm Publish E422 Rejections as Determinate Pre-Commit Failures Under a Reduced Read-Back Budget

## Context and Problem Statement

ADR 0076's tier-2 doctrine makes the first mutating call the runtime authority check and pins its
definitive classification set: HTTP 403/401 are permission-failure signals, npm's publish-time
E404/ENEEDAUTH are mapped to `windlass.verify.error.trusted-publisher-mismatch`, and every other
ambiguous outcome goes through ADR 0067 read-back polling, failing as `indeterminate` unless the
classification proves another outcome. That doctrine was written before the project had any live
evidence about npm's provenance-rejection behavior.

The eighth and ninth live M1 dogfood attempts (12026-11-02 and 12026-11-03, recorded in
[npm-m1.md](../dogfood/npm-m1.md) and
[issue #30](https://github.com/windlasstech/slsa-builder/issues/30)) both reached the publish PUT
and failed closed on a registry `E422` provenance-predicate rejection. Attempt 8 polled the
read-back surface for fifteen minutes and observed nothing; attempt 9, under the five-minute budget,
polled for five minutes and observed nothing. In both runs the outcome was `indeterminate` with
`windlass.verify.error.registry-linkage-mismatch`, which per the verification runbook stops the
operator for a manual registry investigation — thirty-five-plus minutes of cumulative polling and
two manual investigations for a rejection class the registry had already described in full.

Attempt 9 also confirmed, through the ADR 0101 stderr relay, the exact rejection semantics: the
registry's message enumerates the provenance predicate fields it rejected, accurately and
completely. The rejection is a validation refusal of the publish request, not a transient or partial
failure.

Three structural facts bound the design space:

- **The npm publish invocation issues exactly one mutating request.** In npm/cli v11.21.0 (the ADR
  0082 pinned line), `libnpmpublish`'s `buildMetadata()` writes the resolved dist-tag into the
  publish packument body and `publish()` sends the version, the tarball attachment, the Sigstore
  bundle attachment, and the dist-tag as a single `PUT /<escaped package name>`; no second mutating
  request (such as a separate dist-tags POST) follows. An E422 from this invocation can therefore
  only be the rejection of that one PUT.
- **Server-side verification precedes acceptance.** The npm provenance documentation states the
  registry performs its Sigstore verifications "before accepting the publish". Combined with the two
  attempts' negative read-backs, the E422 provenance rejection is empirically pre-commit.
- **HTTP semantics and convention.** RFC 9110 defines 422 as the server's inability to process the
  instructions of a syntactically correct request and attaches no retry rule — the RFC's only
  repeat-related normative text is 403's credential-scoped one — and the WebDAV origin RFC 4918 is
  likewise silent. Ecosystem convention reads 422 more strongly: MDN documents that a client
  receiving 422 should expect an unmodified repeat to fail the same way, and provider APIs use 422
  for validation rejections the caller should fix rather than retry. That convention governs
  retryability, not commit semantics: it does not say the server made no state change before
  refusing, and state-dependent validation can succeed later. The non-retryability of this rejection
  class therefore rests on the npm-specific evidence above — the single-PUT structure and pre-commit
  server-side verification — so polling cannot change the outcome class for the observed
  provenance-rejection form; it can only detect the unexpected case where something committed
  anyway. Other E422 variants carry no such assurance, which the accepted risks below keep explicit.

Under the current doctrine the trusted core pays the full polling budget and emits an
`indeterminate` outcome for a rejection that is, on the evidence, neither retryable nor
commit-ambiguous. The full budget exists to absorb attestation propagation after a committed
publish; a rejection the evidence says is pre-commit has nothing to propagate, so the wait and the
manual-investigation trigger are operational cost without corresponding information.

How should the trusted core classify an npm publish E422, and how much read-back evidence must it
collect before reaching that classification?

## Decision Drivers

- The canonical outcome report must never assert a registry state the trusted core did not verify.
- A rejection that evidence shows to be pre-commit and non-retryable should fail fast with a
  determinate outcome, rather than polling a surface at a budget sized for a different failure
  class.
- Any relaxation of the read-back doctrine must be scoped to the evidence that justifies it and must
  name the residual risks it accepts.
- Rejection depth differs across status codes. Authentication, authorization, and routing refusals
  (401/403/404) fire at request entry, before the payload is processed, so their rejection point is
  structurally far from the mutation boundary. A 422 validation refusal fires only after the server
  has parsed and evaluated the request body — adjacent to the mutation boundary, which is exactly
  where a "parsed, partially applied, then rejected" implementation defect would live. Semantic
  parity with the entry-gate codes therefore cannot carry the determination by itself: the residual
  risk concentrates in npm's implementation, and an implementation fact is closable only by
  observation.
- Version visibility has no pinned speed guarantee. The single committed sample (attempt 4) showed
  version and attestation visible in the same first read-back cycle, but n=1 establishes no
  asymmetry between surfaces and no upper bound on lag; an observation window must tolerate a
  version record that appears late.
- A wrong determinate assertion must be self-limiting: the system's own fail-closed checks must
  surface the error on a subsequent run.
- ADR 0101's constraint that classification enrichment never alters failure semantics stands; this
  decision must not gate outcomes on recognizer output.

## Considered Options

- Classify every publish-time E422 as a determinate pre-commit failure with no read-back at all.
- Keep read-back polling for E422 outcomes but under a reduced budget, failing determinately when no
  publication appears within that budget and at least one clean absence is observed, with the full
  standard budget retained when the version is present (chosen).
- Gate the determinate classification on the rejection-family recognizer identifying the
  provenance-verification family.
- Keep read-back polling for E422 outcomes but under a reduced budget, failing `indeterminate` when
  no publication appears within that budget, with the full standard budget retained when the version
  is present.
- Keep the ADR 0076 behavior unchanged.

## Decision Outcome

Chosen option: "Keep read-back polling for E422 outcomes but under a reduced budget, failing
determinately when no publication appears within that budget and at least one clean absence is
observed, with the full standard budget retained when the version is present", because it converts
an evidenced non-retryable rejection into a fast determinate failure while the reduced window — not
a single sighting — covers the version-visibility lag that no pinned evidence bounds. The 422
refusal fires after the request body has been parsed and evaluated — structurally closer to the
mutation boundary than the entry-gate rejections the existing definitive classes cover — so the
determination rests on an npm implementation fact, and an implementation fact deserves observation,
but observation sized to the risk it covers rather than to the attestation-propagation budget of a
different failure class.

**The rule.** The standing doctrine treats every post-publish registry read as replication-lagged:
npm offers no read-after-write consistency guarantee (ADR 0067), attestation read-back joined the
same polling discipline as the integrity read-back (ADR 0073), and the current specification
classifies a version-not-found response after a publish call as pending rather than absent. This ADR
carves an explicit, evidence-scoped exception for the E422 class only. When the single npm publish
invocation fails and its captured output carries the npm error code `E422`, the trusted core runs
the standard read-back classification under a reduced polling budget:

- The reduced budget is spec-owned and sized to the version-visibility risk — on the order of a few
  polling intervals (tens of seconds), not the attestation-propagation scale of the full budget. It
  begins, as the standard read-back does, with one immediate observation.
- If the version never appears within the reduced budget and at least one clean packument
  observation establishes target-version absence, the run fails determinately: outcome state
  `absent`, mutation attempted, with the new registered diagnostic
  `windlass.verify.error.publish-rejected-unprocessable` ("npm rejected the publication as
  unprocessable and no committed publication was observed"). If every observation in the window
  errored and no clean absence exists, reduced expiry instead continues under the standing rules
  through the original full deadline, measured from the first read-back request — no fresh budget
  starts — because an unreadable registry cannot support a determinate claim. This is the exception:
  for every other failure class, an unconfirmed version at budget exhaustion remains
  `indeterminate`.
- If the version appears at any observation, the E422 is contradicted — npm reported a validation
  refusal while the registry committed — and the trusted core must surface it rather than classify
  silently: the outcome report carries the registered warning
  `windlass.verify.warning.publish-committed-after-rejection`, whose message states the
  contradiction in fixed project vocabulary and carries no external text. The custody question that
  remains — whether the committed attestation is this run's own — is a pending observation under the
  no-read-after-write doctrine, so this anomalous branch continues under the full standard budget,
  and the final classification follows the existing rules: `committed-as-expected` (carrying the
  warning), `foreign-conflict`, or `indeterminate` on budget exhaustion. The warning is also the
  field signal for this ADR's confirmation watch: a live run emitting it means the pre-commit
  evidence no longer describes registry behavior.

All other npm failure outcomes — any other error code, transport failure, or an unreadable result —
keep the ADR 0076 doctrine unchanged, including the full read-back polling budget. The trigger is
the npm error code alone; the ADR 0101 rejection-family recognizer plays no role in outcome
semantics.

**Accepted risks, pinned explicitly.** Two residual risks are accepted as the price of the
determinate answer:

1. **Unobserved E422 classes could accompany a partial registry commit.** No authoritative npm
   source guarantees that every E422 variant is commit-free. If the registry ever commits state and
   then returns E422, and the version becomes visible within the reduced window, the anomaly branch
   handles it correctly; if it commits without any version record ever becoming visible, the report
   asserts `absent` while a version is live. The error is bounded but not immediate: the next
   publish run for that version re-classifies at entry before any mutation, yet while the committed
   version remains invisible that entry still reads `absent` and permits one more `npm publish` call
   — and so can every later run, until the version becomes visible. Each such call is a write issued
   on an incorrect absence classification; it produces no unintended state change because npm never
   republishes an existing version, so the registry rejects it, and once the version becomes visible
   the entry classification surfaces it as `committed-as-expected` or `foreign-conflict`. The false
   assertion is therefore bounded only by eventual visibility: if the committed version never
   becomes visible, the residual is a registry-side inconsistency that no client-side classification
   can observe, and each run pays one futile, registry-rejected publish call.
2. **Visibility lag beyond the reduced window.** A version-not-found throughout the reduced budget
   may still reflect replication lag rather than genuine absence — the standing specification
   deliberately classifies that response as pending after any publish call, and this ADR overrides
   that caution for the E422 branch after a shortened window rather than the full one. The same
   next-run bound applies. The risk is accepted on the attempt 8-9 evidence that the
   provenance-rejection class leaves nothing to become visible; and the custody question in a
   version-present branch is never settled from the reduced window, because an absent attestation
   remains pending under the general doctrine and keeps the full budget.

Both risks mirror what the existing definitive classes already accept: the 401/403 and
E404/ENEEDAUTH mappings assert pre-commit rejection with no read-back observation at all, the
E404/ENEEDAUTH mapping on top of npm diagnostics npm itself documents as misleading. This decision
keeps an observation window those classes do not have.

### Consequences

- Good, because an evidenced non-retryable rejection now fails in tens of seconds with a determinate
  outcome instead of consuming the full polling budget and emitting `indeterminate`.
- Good, because the determinate assertion is backed by a bounded observation window, which the
  existing definitive classes do not perform, and the window covers the unbounded-in-theory version
  lag as far as any finite window can.
- Good, because the accepted-risk modes are bounded by the next run's fail-closed entry
  classification; a false `absent` cannot persist or enable mutation.
- Good, because ADR 0101's enrichment boundary is untouched: outcomes key on the npm error code,
  never on recognizer output.
- Good, because a committed publication discovered after an E422 surfaces as an explicit warning
  (`publish-committed-after-rejection`) instead of passing as an unremarkable success, so a registry
  behavior change cannot hide inside a green run — and its custody is still settled by the standard
  full-budget classification, not by the shortened window.
- Bad, because the doctrine's simplicity ("only authentication and enrollment failures are
  definitive") is replaced by an evidence-scoped exception that must be maintained if npm's
  rejection behavior changes.
- Bad, because the accepted risks above are real: the canonical report can be transiently false in
  the named failure modes, and operators must be able to discover that from this record.
- Bad, because the rule narrows the specification's uniform post-publish caution ("version-not-found
  remains pending") with a per-class exception and a second budget value, so the specification must
  state the E422 branch explicitly and the two budgets must not drift apart.
- Neutral, because ADR 0073's post-success propagation bound is unaffected: it governs
  attestation-endpoint propagation after a committed publish, and this decision's reduced budget
  fires only on the version-existence surface after an E422.

## Confirmation

- TDD conformance fixtures cover: E422 with the version never appearing and at least one clean
  absence observed failing determinately with
  `windlass.verify.error.publish-rejected-unprocessable`, outcome `absent`, with observation and
  sleep counts bounded by the reduced budget; E422 with the version appearing within the reduced
  window emitting `windlass.verify.warning.publish-committed-after-rejection` and then engaging the
  full-budget classification, ending `committed-as-expected` with the warning, or
  `foreign-conflict`, or `indeterminate` on budget exhaustion; and non-E422 failures retaining the
  full polling budget unchanged.
- The diagnostic registry parity tests carry the new error and warning IDs.
- No dedicated live run is required: the decision table is fully exercisable by the hermetic
  fixtures above, and the E422 trigger surface was already field-verified by attempts 8 and 9, so
  acceptance rests on the TDD fixtures and review alone. Field confirmation is opportunistic: if a
  future live run hits an E422 under this behavior, the observation is recorded in
  `docs/dogfood/npm-m1.md`, and a live run emitting
  `windlass.verify.warning.publish-committed-after-rejection` contradicts the pre-commit evidence
  and requires this ADR to be revisited.

## Pros and Cons of the Options

### Classify every publish-time E422 as determinate with no read-back at all

- Good, because it is the simplest rule and the fastest failure.
- Bad, because it asserts the unpinned "E422 means no commit" invariant with zero verification; a
  contradictory registry behavior would be misreported with no chance of same-run detection. The
  entry-gate analogy fails here: unlike 401/403/404, a 422 fires after the request body has been
  parsed and evaluated, adjacent to the mutation boundary, so the question is an npm implementation
  fact rather than a protocol semantic.
- Bad, because without any observation the anomaly has no in-run signal: a registry that committed
  despite the E422 would surface only at the next run's entry classification as an unexplained
  `foreign-conflict`, and the `publish-committed-after-rejection` warning could not exist.

### Keep read-back polling for E422 outcomes but under a reduced budget, failing determinately when no publication appears within that budget and at least one clean absence is observed, with the full standard budget retained when the version is present

- Good, because the wait shrinks to a window sized for the actual risk (version-visibility lag)
  while the determinate outcome eliminates the manual-investigation trigger.
- Good, because the reduced window catches a late-appearing version record that a single observation
  would miss; the single-observation design is its degenerate one-cycle case, subsumed.
- Good, because the determinate label fires only on a clean observed absence: an all-errored window
  degrades to the standing full-budget path rather than asserting absence on no evidence.
- Good, because a version-present anomaly still receives the full budget for the custody question,
  so no absence-type observation beyond the E422 version check is promoted to a fact early.
- Bad, because it accepts the two pinned residual risks above, including a shortened but still
  finite horizon for the visibility race.
- Bad, because a second budget value adds a specification surface that must be maintained and kept
  coherent with the full budget's rationale.

### Gate the determinate classification on the rejection-family recognizer

- Good, because it couples the shortcut to the rejection cause confirmed in the message text.
- Bad, because it contradicts ADR 0101's enrichment-only constraint — classification would alter
  failure semantics — and couples outcome semantics to npm's English message prose, which a silent
  upstream rewording could degrade.

### Keep read-back polling for E422 outcomes but under a reduced budget, failing `indeterminate` when no publication appears within that budget, with the full standard budget retained when the version is present

- Good, because it never asserts anything unverified and still eliminates the long wait — the
  reduced budget and the version-present fallback are identical to the chosen option's.
- Bad, because it keeps the `indeterminate` label and its manual-investigation trigger for a
  rejection class the evidence says is determinate — the operator still pays the investigation cost
  this decision exists to remove. The window is the same; only the outcome label differs, and the
  label is the costlier half of the status quo.

### Keep the ADR 0076 behavior unchanged

- Good, because the doctrine stays uniform and no risk is newly accepted.
- Bad, because every provenance rejection costs the full polling budget plus a manual registry
  investigation, as attempts 8 and 9 demonstrated, with no possible information gain.

## More Information

- Attempt 8 and 9 evidence: [npm-m1.md](../dogfood/npm-m1.md) and
  [issue #30](https://github.com/windlasstech/slsa-builder/issues/30) (attempt-9 comment).
- Attempt 4 (run 31840088262) is the single committed sample: version and attestation were both
  visible in the first read-back cycle within the same minute as the publish call; n=1 establishes
  no surface asymmetry and no lag bound, and the run's fine-grained logs have expired.
- Rejection-message catalog and npm CLI behavior notes:
  [npm-registry-provenance-requirements.md](../dogfood/npm-registry-provenance-requirements.md).
- Single-PUT structure: npm/cli v11.21.0 `workspaces/libnpmpublish/lib/publish.js`
  (`buildMetadata()` writes `dist-tags` into the publish packument; `publish()` issues one
  `PUT /<escaped package name>` and no subsequent mutating request), confirmed by direct source read
  at tag v11.21.0.
- Single-PUT history: the same topology appears in every sampled earlier release — v6.14.18,
  v7.24.2, v8.19.4, v9.9.4, v10.9.4, and v11.0.0 (tag-pointed commits inspected in `libnpmpublish`'s
  publish implementation) — so the shape was present by v6.14.18; the original introduction point
  before that was not pinned. No sampled publish flow issues a separate dist-tag mutation after the
  package PUT. The standalone `npm dist-tag add` command does use the separate
  `/-/package/<name>/dist-tags/<tag>` endpoint, but it is not part of the publish flow. Older
  versions carried an E409 retry that re-read metadata and repeated the same package PUT on a
  concurrent-version conflict; the pinned v11.21.0 publish path contains no such retry loop.
- Server-side verification ordering: the npm provenance documentation's "before accepting the
  publish" verification statement —
  [npm/provenance README, "Server-side provenance verification and proof"](https://github.com/npm/provenance#server-side-provenance-verification-and-proof).
- HTTP 422 semantics:
  [RFC 9110 section 15.5.21](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.5.21)
  (definition; no retry rule — the RFC's only repeat-related normative text is 403's
  credential-scoped one; the WebDAV origin RFC 4918 is likewise silent) and
  [MDN's 422 reference](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status/422) (the
  conventional expectation that an unmodified repeat fails the same way; MDN editorial guidance
  citing the RFC definition, governing retryability rather than commit semantics).
- Status-code scoping survey (12026-11-04): the npm registry publishes no status-code contract for
  the publish PUT, dist-tag, or OIDC exchange endpoints (the archived registry API reference covers
  read endpoints only), and the npm CLI maps every status ≥ 400 to a generic `E<status>` label with
  no processing-stage taxonomy (only 401 is specialized, to EOTP/EAUTHIP/EAUTHUNKNOWN). Among
  observed publish-path statuses, E422 is the only one with direct evidence of a post-parse
  validation refusal naming the rejected content; E400/E413/E415 lack stage evidence, E409 is
  conflict semantics with observed stale-state cases (not interchangeable with a definitive
  rejection), E429 is edge-generated and client-retried (a transient class), and E451 has no
  publish-path observation. No other status currently belongs to the class this ADR scopes.
- Follow-up issue [#142](https://github.com/windlasstech/slsa-builder/issues/142) tracks the
  recognizer template widening, which is orthogonal to this decision.
