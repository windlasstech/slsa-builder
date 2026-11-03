---
parent: Decisions
nav_order: 101
status: accepted
date: 12026-11-03
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0024
  - type: see-also
    target: ADR-0076
  - type: see-also
    target: ADR-0083
---

# Relay External CLI stderr to Logs Under Hygiene Bounds, with Template Classification as Enrichment

## Context and Problem Statement

The seventh and eighth live M1 dogfood attempts (12026-11-02, recorded in
[issue #30](https://github.com/windlasstech/slsa-builder/issues/30) and
[npm-m1.md](../dogfood/npm-m1.md)) reached the npm publish stage and failed closed on a registry
`E422`. The operational diagnostics introduced by PR #141 retained only the npm error code and HTTP
status: the boundary captured the npm CLI's stderr but dropped its content, so the registry's
rejection message — which npm CLI rendered into that stderr — never reached the log. Issue
[#142](https://github.com/windlasstech/slsa-builder/issues/142) tracks recovering that rejection
detail.

A research pass (recorded in
[npm-registry-provenance-requirements.md](../dogfood/npm-registry-provenance-requirements.md))
established the facts that bear on the boundary question:

- **The trusted core never possesses the registry response body.** The publish is executed by the
  npm CLI subprocess; the HTTP response is visible only inside that process. What the trusted core
  can obtain is the npm CLI's rendered stderr. "Log the raw registry body" is not an available
  option without intercepting the HTTP layer, which is rejected as unjustified complexity.
- **The registry's rejection message class is log-safe by observation.** All five confirmed
  rejection families (predicate field mismatch, repository metadata mismatch, private source
  repository, self-hosted runner, missing certificate claim) contain only public values: repository
  URLs, refs, workflow paths, event names, public numeric repository and owner IDs, runner
  environment, and visibility. The trusted-publishing token travels in the request, and no report
  shows it echoed in a response. npm CLI itself renders this message class to stderr on every
  failure, in public CI logs worldwide.
- **The registry derives expected values from the signing certificate.** Its own error text names
  Fulcio extension labels (`SourceRepositoryRef`, `BuildConfigURI`, and so on) as the expected-value
  source, so the message content is largely a reflection of public certificate claims.
- **npm CLI is already inside the trusted computing base.** The profile delegates the authenticated
  publish mutation itself to npm CLI (ADR 0024, ADR 0076). Trusting the same process to perform the
  mutation while treating its stderr as unloggable is an inconsistent asymmetry: the output channel
  is lower-stakes than the mutation channel already granted.

The motivating secrecy question — whether the registry's rejection messages might carry non-log-safe
content — was investigated and answered in the negative for the observed message class. What remains
as legitimate rationale for any filtering is not secrecy but boundary control: the scope of what an
external tool's current and future output behavior can place in project logs, the determinism of the
canonical diagnostic contract, and the exclusion of non-message content.

Two further considerations shape the decision. First, the project's responsibility is the SLSA Build
L3 build-and-publish contract, not curated re-presentation of each ecosystem registry's error text.
Second, the project intends to add per-language profiles; an approach that requires researching and
re-implementing every registry's message grammar before its failures become diagnosable does not
scale.

How should the rendered stderr of external ecosystem CLIs cross the log boundary?

## Decision Drivers

- The dogfood learning loop must not be gated on instrumentation: a novel registry rejection must
  reach the log on the attempt that produces it, not after a recognizer update and a retry cycle.
- Scope of responsibility: the project builds and publishes under SLSA Build L3; faithfully relaying
  the toolchain's own error output is sufficient, and curating per-registry message presentations is
  not a project responsibility.
- Profile scalability: a new profile should inherit diagnosable logs without per-registry message
  research.
- The canonical diagnostic contract (fixed-vocabulary IDs, the outcome report, the classification
  state machine of ADR 0076) must remain deterministic, hermetic, and free of external text.
- The boundary applies a hygiene filter to relayed output. The filter's concrete conditions are
  owned by the specification, not by this decision, so their strength and mechanics can be adjusted
  by specification amendment as operational evidence accumulates, without a new ADR.

## Considered Options

- Relay external CLI stderr through a hygiene filter (pass-through)
- Extract the registry message and re-render it under project formatting (extraction)
- Fixed-vocabulary recognizer only, no text crossing
- Pass through only the failing command's error block
- Sealed or encrypted capture of raw output

## Decision Outcome

Chosen option: "Relay external CLI stderr through a hygiene filter (pass-through)", because it is
the only option under which every novel registry rejection is diagnosable on its first occurrence,
without adding per-registry message curation to the project's permanent responsibilities, and
without weakening the canonical diagnostic contract.

Concretely, the external-process operational diagnostics clause of the core profile contract is
amended so that:

- The stderr of allowlisted external processes (currently `node`, `npm`, `npx`, `corepack`, `pnpm`,
  `yarn`) is relayed to the workflow log through a hygiene filter. This decision fixes only the
  filter's existence and purpose; the concrete conditions — such as control-character stripping,
  size and line bounding, or exact-value redaction of secrets the trusted core injected into the
  subprocess environment — are defined by the specification, may differ per condition class, and may
  be strengthened or relaxed by specification amendment without a new ADR. The initial specification
  is expected to carry all three named conditions, but that content is a spec-level choice, not part
  of this decision.
- The existing fixed-vocabulary lifecycle diagnostics (`windlass operational:` lines) and the
  canonical outcome report are unchanged: no external text enters canonical diagnostics, the closed
  diagnostic ID registry, or any signed or verifier-consumed artifact.
- The registry-rejection template recognizer is retained but demoted from information gatekeeper to
  enrichment: when npm CLI output matches a known rejection family, the family identifier is added
  to the operational diagnostics; a non-match loses nothing, because the text itself is already in
  the log. The five confirmed message families recorded in the research note become its initial
  fixtures.
- The npm CLI's own stdout/stderr relay applies at the tool's default log level; the profile does
  not enable verbose or silly logging, which carry materially broader and less characterized output.
- This establishes the default pattern for future profiles: a new profile inherits hygienic
  pass-through for its ecosystem tools and may add template classification incrementally as
  enrichment. Each profile confirms the pattern at its own admission.

The amendment to the specification and the TDD implementation follow the established path: the core
profile contract's diagnostics clause is updated (the blanket prohibition on child output is
replaced by the hygiene-filter rule for allowlisted processes), and the specification defines the
filter's initial concrete conditions with their conformance fixtures. Later adjustments to those
conditions are specification amendments and do not require revisiting this ADR.

### Consequences

- Good, because the next novel npm rejection is diagnosable from the failing run's own log, with no
  instrumentation cycle; attempt 8's missing-message failure mode cannot recur.
- Good, because future profiles inherit diagnosable logging for free; per-registry message research
  becomes optional enrichment rather than a diagnosability prerequisite.
- Good, because the trust model becomes consistent: the process trusted with the authenticated
  publish is also trusted to have its rendered output relayed, while the canonical contract stays
  sealed.
- Good, because the hygiene filter's strength is a spec-level dial: operational evidence (for
  example a newly observed output class) can tighten or relax concrete conditions through a
  specification amendment alone, while the boundary principle decided here remains stable.
- Neutral, because log volume and content now vary with npm CLI's rendering; the specification's
  bounds cap the impact, and the operational channel is explicitly non-authoritative and
  non-protocol.
- Bad, because npm CLI output wording changes can add noise or shift what the recognizer matches;
  the canonical diagnostics are unaffected, and a recognizer miss is now a missed enrichment rather
  than a loss of evidence.
- Bad, because content from a broader class than the five observed families (for example deprecation
  notices or network retry messages) now reaches the log; these classes are public by construction
  at the default log level, and the specification's hygiene conditions apply to them uniformly.

## Confirmation

This decision is confirmed when:

- the core profile contract's external-process diagnostics clause replaces the child-output
  prohibition with the hygiene-filter rule for allowlisted processes, records the default log level
  constraint, and states that the filter's concrete conditions are spec-owned and adjustable by
  specification amendment;
- the specification defines the filter's initial concrete conditions, and the trusted core relays
  allowlisted subprocess stderr through a filter implementing exactly them, with conformance tests
  for each condition the specification defines;
- the recognizer operates as enrichment: known families yield family identifiers in operational
  diagnostics, unknown output flows through the hygiene filter, and no external text reaches the
  canonical outcome report;
- a subsequent dogfood attempt captures the exact registry rejection family from the log without any
  instrumentation change;
- issue #143 proceeds on the research note's projection mapping.

## Pros and Cons of the Options

### Relay external CLI stderr through a hygiene filter (pass-through)

The stderr of allowlisted external processes reaches the log through a hygiene filter whose concrete
conditions are defined and adjusted at the specification level; classification rides along as
enrichment.

- Good, because information recovery on novel rejections is immediate and total within the
  specification's hygiene bounds: the registry's own message, rendered by the registry's own client,
  is the most faithful diagnostic obtainable.
- Good, because the per-profile cost is zero: any ecosystem toolchain's failures become diagnosable
  the moment the profile runs, without message-grammar research.
- Good, because it is consistent with the trust model: npm CLI already executes the authenticated
  publish (ADR 0024); relaying its default-level output is a strictly smaller delegation.
- Good, because the failure mode it fixes is the one actually observed: in attempt 8 the rejection
  message existed at the boundary and was dropped by policy, not by any technical limitation.
- Neutral, because the log channel carries external text whose vocabulary evolves with the pinned
  tool; the channel is documented as non-authoritative and is not consumed by verifiers or the
  canonical report.
- Bad, because the hygiene guarantee is whatever the specification defines rather than a property
  frozen by this decision: an unforeseen npm CLI output path carrying unexpected content would cross
  until the specification is amended, mitigated by the default log level constraint and by the fact
  that the content originates from two parties already inside the trust boundary (the registry and
  the pinned CLI). The initial specification's expected conditions (control-character stripping,
  size bounding, exact-value redaction of injected secrets) address the classes known at decision
  time.
- Bad, because secret exclusion is a spec-level commitment rather than a decision-level invariant;
  the research record shows no such content in the observed families, and this residual is accepted
  consciously rather than through ignorance.

### Extract the registry message and re-render it under project formatting (extraction)

Parse the npm CLI error block, extract the message, classify it, and re-emit a bounded, redacted
excerpt in project format.

- Good, because the emitted text is limited to the message class npm CLI renders for users, with
  project-controlled formatting, and classification is integrated.
- Good, because it provides a stable, greppable presentation independent of npm CLI wording drift.
- Bad, because the extraction layer is a per-profile, per-registry permanent responsibility: every
  new ecosystem requires its message grammar to be researched and implemented before its failures
  are diagnosable, which contradicts the profile-scaling goal and the project's responsibility
  scope.
- Bad, because it optimizes presentation, which is not the bottleneck: the bottleneck was evidence
  recovery, and pass-through solves that with no grammar work.
- Neutral, because extraction remains available later as an optional enrichment (for example a
  bounded rejection excerpt field in the outcome report) without changing this decision.

### Fixed-vocabulary recognizer only, no text crossing

Only template matches cross the boundary as family identifiers; unmatched output yields
`unclassified` with size and hash metadata.

- Good, because the boundary admits no external text at all, giving the strongest conceivable
  confidentiality posture for the log channel.
- Bad, because every novel rejection is invisible: the registry's message is dropped, and each new
  family costs a dogfood cycle plus a recognizer update before diagnosis can begin. The #2499
  history shows the registry's validation behavior changes over time, so this cost recurs.
- Bad, because its secrecy justification is not supported by the gathered evidence: the observed
  message class contains only public values, so the option pays a recurring diagnosability cost for
  a confidentiality benefit the evidence does not corroborate.

### Pass through only the failing command's error block

Relay only the npm CLI-rendered error block of a failed publish, not the whole stream.

- Good, because log noise is lower than full pass-through while the rejection message is preserved.
- Bad, because "the error block" is itself a parsing commitment to npm CLI's output framing, which
  is exactly the per-tool grammar dependency the chosen option avoids; a framing change silently
  drops the block, recreating the blindness the decision exists to remove.
- Neutral, because the hygiene filter's size bound already caps total volume, so the noise reduction
  is marginal.

### Sealed or encrypted capture of raw output

Retain raw output in an encrypted or never-uploaded store, with classified identifiers in the log.

- Good, in theory, because full fidelity would be preserved for forensics while logs stay clean.
- Bad, and disqualifying, because publish runs on ephemeral hosted runners: anything not uploaded is
  lost when the runner ends, and anything uploaded has the same exposure class as the log itself.
  Key management for an encrypted channel costs more than the residual risk it addresses.

## More Information

- Research record:
  [npm-registry-provenance-requirements.md](../dogfood/npm-registry-provenance-requirements.md)
  (documented requirements, the slsa-github-generator#2499 precedent, the verbatim rejection-message
  catalog, the npm CLI code-observed behavior, and the accepted-versus-rejected bundle comparison).
- Dogfood evidence: [npm-m1.md](../dogfood/npm-m1.md), attempts 7 and 8; issue thread
  [#30](https://github.com/windlasstech/slsa-builder/issues/30).
- Tracking issues: [#142](https://github.com/windlasstech/slsa-builder/issues/142) (publish
  rejection diagnostics), [#143](https://github.com/windlasstech/slsa-builder/issues/143) (the
  provenance compatibility projection, a separate decision axis).
- Data-flow note: the trusted core never holds the registry's raw response body; the npm CLI parses
  it and renders the error to stderr, and every option in this decision operates on that rendered
  stream. "Log the raw registry response" was eliminated as not implementable without HTTP-layer
  interception.
- Correction recorded by this ADR: the secrecy concern behind a text-suppressing boundary was a
  valid fail-closed default before the research; the research showed the observed rejection-message
  class contains only public values and is one the npm CLI prints to public CI logs by default. The
  residual rationale for filtering is boundary control (output scope, canonical-contract
  determinism, hygiene invariants), which this decision satisfies without suppressing the message.
- The recognised rejection families' verbatim forms, including the multiline predicate-mismatch form
  whose expected-value labels name Fulcio certificate extensions, are catalogued in the research
  note and serve as the enrichment recognizer's fixtures.
