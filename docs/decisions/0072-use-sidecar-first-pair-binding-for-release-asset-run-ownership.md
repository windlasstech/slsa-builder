---
parent: Decisions
nav_order: 72
status: accepted
date: 12026-08-04
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0067
    scope:
      "the release-asset digest-only binding proof and the never-silently-adopt and
      singular-committer clauses as applied to unsigned release asset publication events; all other
      ADR 0067 semantics remain in force"
  - type: amends
    target: ADR-0066
    scope:
      "intra-segment mutation ordering: within the release mutation segment, the provenance sidecar
      upload must precede the primary asset upload"
  - type: see-also
    target: ADR-0049
  - type: see-also
    target: ADR-0051
  - type: see-also
    target: ADR-0073
  - type: see-also
    target: ADR-0074
  - type: see-also
    target: ADR-0075
---

# Use Sidecar-First Pair Binding for Release Asset Run Ownership

## Context and Problem Statement

ADR 0067 adopted run-identity-scoped convergence: retry attempts of the same `run_id` may recognize
their own commits through binding proofs, while a new `run_id` fails closed on any pre-existing
remote state. For GitHub Release assets, the binding proof is digest equality — the expected
SHA-256, recovered from prior-attempt carry-over or recomputed from byte-reproducible source,
compared against the authoritative release asset `digest` field.

Digest equality proves content identity, not authorship. The GitHub release asset API exposes `id`,
`node_id`, `digest`, `uploader`, `created_at`, `updated_at`, `state`, `size`, `content_type`, and
URLs; none of these fields binds an upload to a workflow run identity. Run identity exists only
inside signed artifacts, through the Fulcio certificate's Run Invocation URI
(`.../actions/runs/<run-id>/attempts/<n>`). Two distinct histories therefore produce the same
remotely observable state:

1. run A uploads the primary asset, GitHub commits it, and the response is lost before the attempt
   dies; or
2. after run A's signed sidecar appears, another writer using the same repository automation
   principal uploads a byte-identical primary asset — possible because packed artifact digests are
   byte-reproducible for identical source, as the ADR 0067 spike verified.

A retry cannot distinguish these histories. Client-side evidence generated before the upload proves
intent only; evidence generated after the upload leaves an interruption gap. No client-side
combination of digests, bundles, or receipts can simultaneously prove uploader custody and recover
every interruption.

The current primary-first upload order additionally admits a primary-present/sidecar-absent partial
state in which no signed artifact covers the primary at all, so even the weaker "valid pair"
statement cannot be made about it.

ADR 0067's clauses "never silently adopt remote state the run did not produce" and "which run
committed stays singular" consequently overclaim for unsigned release asset publication events.
Three guarantees must be separated: production authorship (provable; the producer provenance is
signed), publication content integrity (provable; digest binding), and publication-event custody
(not provable with current platform surfaces). SLSA provenance is artifact- and digest-centric, and
under ADR 0049 and ADR 0051 the publisher is a verified distributor: consumers verify the producer
run, not the release-upload job.

How should same-`run_id` convergence for release assets be proven — given that publication-event
custody cannot be proven — while preserving transient-fault recoverability, the
no-overwrite/no-delete posture, and honest reporting?

## Decision Drivers

- Never claim what the evidence cannot prove: a success report must not assert uploader custody
  without a matching server-assigned receipt.
- Preserve ADR 0067 recoverability: a transient fault must not become permanent loss of the trusted
  redistribution path; requiring evidence that can be destroyed by the fault itself would recreate
  that failure mode.
- Keep the no-overwrite/no-delete posture; the same-run starter-asset deletion exception is
  unchanged.
- Prefer protocol structure (mutation ordering) over new evidence machinery (additional signing
  steps, receipt storage, external services).
- Keep the producer-neutral publisher model: the rule must hold for future producer profiles.
- Fail closed on any doubt: `indeterminate` is always acceptable, a wrong success is not.
- Undocumented platform behavior (prior-attempt carry-over) remains an optimization, never a
  load-bearing assumption.

## Considered Options

- Sidecar-first pair binding with explicit custody non-attribution.
- Load-bearing server-assigned receipt binding: convergence requires the prior attempt's recorded
  upload-response asset ID to equal the remote asset ID.
- Signed custody chain: a publisher-issued publication attestation per mutation step, binding
  `run_id`, step, server-assigned asset ID, digest, and timestamp.
- Content poisoning: embed the run identity in the primary asset bytes so digest equality becomes
  run-attributive.
- Staging asset with ID-preserving rename: upload under a run-scoped temporary name, then rename the
  same object to the final name.
- Organizational hardening only: immutable releases, draft sealing, environment and tag protection.
- Strict fail-closed: never adopt a pre-existing primary asset, even under a valid signed pair.
- External run-bound publication gateway: an OIDC-authenticated service performs conditional
  creation and persists the verified `run_id` with the object in one server-side transaction.

## Decision Outcome

Chosen option: "Sidecar-first pair binding with explicit custody non-attribution", because it
converts the unattributable partial state into a structurally impossible one, keeps every
interruption recoverable without new machinery, and replaces an unprovable guarantee with an honest
one that matches what SLSA consumers actually verify.

**Ordering.** Within the release mutation segment defined by ADR 0066, the provenance sidecar upload
must complete — committed and digest-verified — before the primary asset upload begins. The workflow
must never create a primary-present/sidecar-absent state through its own protocol. This amends the
mutation segment with a normative intra-segment ordering; the segment's serialization semantics are
unchanged.

**Pair-gated convergence.** A retry attempt of the same `run_id` may classify a pre-existing primary
asset as `committed-as-expected` only when all of the following hold:

1. the remote sidecar's authoritative GitHub `digest` equals the expected bundle digest;
2. the sidecar verifies under the full verification policy, including signer and source identity
   binding per ADR 0068;
3. the sidecar's verified Fulcio Run Invocation URI carries a run-id equal to the current
   `github.run_id`; any earlier attempt number is acceptable and must not fail the comparison;
4. the sidecar's signed subject and producer fields bind the expected primary asset name and digest;
   and
5. the remote primary asset's authoritative GitHub `digest` equals the bound digest.

A sidecar-present/primary-absent state with a same-`run_id` verified sidecar converges by uploading
the primary. A primary-present/sidecar-absent state is definitionally foreign under this protocol
and classifies as `foreign-conflict`. A new `run_id` continues to fail closed on either pre-existing
asset, even when its bytes match, exactly as ADR 0067 requires.

**Custody non-attribution.** Convergence success means "the release slot contains the expected
provenance-covered bytes", not "this run performed the upload". Production authorship remains
singular and verifier-visible through the signed producer provenance; publication-event custody is
explicitly not attributed by the workflow, the outcome report, or the verification documentation.

**Receipts as diagnostic evidence.** When a prior attempt's upload response or carry-over survives,
the workflow must retain the server-assigned asset ID, uploader identity, timestamps, and state as
report evidence. A surviving receipt that contradicts the remote asset ID proves replacement and
classifies `foreign-conflict`. A missing receipt never by itself blocks convergence, because the
receipt is destroyed by exactly the ambiguous-response faults convergence exists to recover from;
the same-run starter-asset deletion exception in the publisher specification is the sole place where
a receipt remains load-bearing, and it is unchanged.

**Reporting.** The always-run outcome report must distinguish, as evidence substates beneath the ADR
0067 outcome grammar: uploaded with a confirmed response and asset ID; converged from a prior
confirmed receipt; and converged as a valid provenance-covered pair with uploader custody unproven.
A report that asserts custody without a matching asset-ID receipt is a conformance defect.

**Scope of partial supersession.** For unsigned release asset publication events, this ADR replaces
ADR 0067's digest-only asset binding with the pair binding above, and narrows ADR 0067's
never-silently-adopt and singular-committer clauses to production authorship and signed artifacts.
Every other ADR 0067 semantic — the four outcome states, the re-run-failed-jobs retry surface, the
npm and release manifest step bindings, polling bounds, the starter-asset exception, and the
always-run reporting obligation — remains in force. The npm publish binding is unaffected by this
ADR; strengthening it with a published-attestation run-identity check is deferred to a separate
decision.

### Consequences

- Good, because the workflow's own protocol can never leave a primary asset without signed coverage:
  any lone primary is definitionally foreign, and every own partial state carries a verifiable
  same-`run_id` sidecar.
- Good, because every interruption point remains recoverable: after sidecar commit, during the
  primary upload, after primary commit with a lost response, and after a confirmed response.
- Good, because no new signing steps, receipt storage, or external infrastructure are introduced,
  and the producer-neutral publisher model is preserved.
- Good, because success reports become truthful: they distinguish "this run uploaded it" from "the
  expected provenance-covered bytes are present", which is the statement the evidence supports.
- Bad, because an authorized actor can still publish an identical, validly proven copy between
  protocol steps; the workflow records the observation but cannot attribute the upload event. This
  residual is accepted: it requires repository write access, it cannot inject different content
  without defeating SHA-256, and it cannot forge the signed producer provenance.
- Bad, because the publisher specification's upload order, aggregate partial-failure conditions, and
  convergence fixtures assume primary-first ordering and must be rewritten, with new fixtures for
  the pair binding.
- Neutral, because receipt and uploader metadata remain audit and incident-investigation evidence
  rather than being discarded.
- Neutral, because immutable release sealing remains available as defense-in-depth for final-state
  integrity; it is not a custody proof and its adoption would be a separate decision about release
  lifecycle authority.

### Confirmation

This decision is confirmed when:

- architecture specifications define the sidecar-first ordering, the five pair-gate conditions, the
  custody non-attribution rule, the receipt diagnostics rule, and the report substates, tracing to
  this ADR;
- a conformance check proves the workflow can never create a primary-present/sidecar-absent state;
- crash-matrix fixtures demonstrate correct recovery from interruption after sidecar commit, during
  the primary upload, after primary commit with a lost response, and after a confirmed response;
- pair-validation fixtures demonstrate that a wrong digest, a wrong run identity, a malformed
  bundle, a wrong signer or source identity, or a wrong subject/name binding fails without mutation;
- a cross-run fixture demonstrates that a new `run_id` fails closed on byte-identical pre-existing
  assets;
- an adversarial spike interleaves a second workflow uploading identical bytes after sidecar
  publication and documents exactly which API fields do and do not distinguish the event;
- a reporting fixture demonstrates that no success report asserts uploader custody without a
  matching asset-ID receipt.

## Pros and Cons of the Options

### Sidecar-first pair binding with explicit custody non-attribution

- Good, because it closes the unattributable partial state by construction rather than by evidence
  collection, and it is the only option that both preserves recoverability and requires no new
  trusted machinery.
- Good, because the guarantee it declares — expected provenance-covered bytes in the release slot —
  is exactly the guarantee SLSA consumers verify.
- Bad, because it formally gives up publication-event custody attribution, which ADR 0067's wording
  had claimed, and therefore requires the partial supersession recorded above.
- Bad, because it reorders the mutation segment and forces specification and fixture churn in the
  publisher and composition specs.

### Load-bearing server-assigned receipt binding

- Good, because a surviving asset-ID receipt is strong, server-assigned ownership evidence, already
  precedented by the starter-asset deletion exception.
- Bad, because the receipt is lost in precisely the ambiguous-response window that recovery must
  handle: the upload commits, the response vanishes, and no ID was ever recorded. Requiring the
  receipt then converts that transient fault into permanent loss, repealing ADR 0067's core
  achievement; tolerating its absence reopens the indistinguishable state.

### Signed custody chain

- Good, because it produces durable audit evidence that a trusted publisher observed and approved a
  specific asset ID.
- Bad, because it is not atomic with the upload: the upload-to-receipt interruption window remains,
  receipt storage adds a new failure mode, and it reintroduces publisher-owned publication evidence
  that ADR 0049 and ADR 0051 deliberately excluded.

### Content poisoning

- Bad, because it destroys the spike-verified byte-reproducibility of packed artifacts, breaks the
  npm PURL and digest subject binding, and pollutes the artifact — and it still does not prove
  custody, because a foreign actor can copy and upload the same run-marked bytes. It attributes
  production, which is already signed, not the publication event.

### Staging asset with ID-preserving rename

- Good, because a run-scoped temporary name lets a retry locate the exact object the prior attempt
  created, improving remote recovery precision.
- Bad, because it adds nonce persistence, foreign staging-name handling, rename-response loss, and
  extra mutations — more machinery than the pair binding, still without a cryptographic custody
  proof.

### Organizational hardening only

- Good, because immutable releases seal the final asset set, and environment or tag protection
  shrinks the window in which out-of-band writers can act.
- Bad, because none of these controls binds an asset upload to a run identity, environment rules
  constrain workflows rather than direct API uploads by other principals, and draft-to-publish
  sealing would widen the publisher's authority beyond its current boundary.

### Strict fail-closed on any pre-existing primary

- Good, because it is the simplest rule and has no silent-adoption surface at all.
- Bad, because an interruption between the primary and sidecar uploads — or a lost upload response —
  becomes unrecoverable through the trusted path, since deletion is forbidden; this resurrects the
  exact permanent-loss failure ADR 0067 was written to eliminate.

### External run-bound publication gateway

- Good, because a server-side conditional create that persists the verified OIDC `run_id` with the
  object in one transaction is the only complete custody proof identified.
- Bad, because it introduces external infrastructure, a new trust and availability dependency, and
  operational cost that is disproportionate to a residual risk that requires repository write access
  and cannot alter content.

## More Information

- Run-identity convergence, outcome states, and the 12026-08-02 spike evidence: ADR 0066 and
  [ADR 0067](0067-converge-repeated-runs-within-run-identity.md).
- Verified-distributor publisher model: ADR 0049; producer provenance distribution: ADR 0051;
  verifier identity binding including the Run Invocation URI:
  [ADR 0068](0068-bind-verification-to-immutable-builder-and-source-identities.md).
- GitHub release asset API fields (`id`, `digest`, `uploader`, `state`, timestamps), none of which
  carries workflow run identity: <https://docs.github.com/en/rest/releases/assets>; asset digest
  availability:
  <https://github.blog/changelog/2025-06-03-releases-now-expose-digests-for-release-assets/>.
- Fulcio run identity certificate extensions:
  <https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md>; GitHub Actions OIDC `run_id` and
  `run_attempt` claims: <https://docs.github.com/en/actions/reference/security/oidc>.
- Artifact/digest-centric provenance model and `invocationId` uniqueness:
  <https://slsa.dev/spec/v1.2/build-provenance>.
- Immutable releases as final-state sealing (not per-asset custody evidence):
  <https://docs.github.com/en/code-security/supply-chain-security/understanding-your-software-supply-chain/immutable-releases>.
