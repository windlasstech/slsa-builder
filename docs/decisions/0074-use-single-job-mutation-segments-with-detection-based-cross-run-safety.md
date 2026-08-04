---
parent: Decisions
nav_order: 74
status: accepted
date: 12026-08-04
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0066
    scope:
      "the multi-job mutation segment definition and the strict single-writer prevention wording;
      the concurrency group key, cancel-in-progress: false, and the job-class permission model
      remain in force"
  - type: amends
    target: ADR-0058
    scope:
      "the publisher mutation topology: all release asset uploads for one run live in exactly one
      upload job per remote surface"
  - type: see-also
    target: ADR-0067
  - type: see-also
    target: ADR-0072
  - type: amended-by
    target: ADR-0075
    scope:
      "the acknowledged liveness limitation: pending mutation jobs can no longer be starved or
      silently evicted by later arrivals; the single-job segment topology and detection-based
      cross-run safety are unchanged"
---

# Use Single-Job Mutation Segments with Detection-Based Cross-Run Safety

## Context and Problem Statement

ADR 0066 serialized release mutations with job-class concurrency: every job holding release mutation
authority declares a job-level concurrency group (`release-mutation-<repository>-<ref_name>`,
`cancel-in-progress: false`), and the release-asset mutation segment was defined as beginning when
the first such job enters the group and ending only after the primary and sidecar upload calls
complete — a segment that can span multiple jobs.

Platform semantics make that definition unsound. Per GitHub's concurrency documentation, a
concurrency group holds at most one running job and one pending job; a third arrival replaces the
pending one, and `cancel-in-progress: false` protects only the running job. Jobs in a group are
processed in FIFO order by the time each started waiting, with no same-run stickiness: nothing
prevents another run's queued job from acquiring the group between two mutation jobs of the same
run. GitHub environments do not help — the documentation states that concurrency and environment are
not connected. A multi-job mutation segment is therefore not atomic, no matter how the concurrency
keys are arranged.

The strict-prevention alternatives all fail on other axes. Consolidating every mutation into one
mega-job would hold the group for the whole segment, but it would concentrate `id-token: write`
(signing identity), `contents: write` (release mutation), `artifact-metadata: write`, and npm
publish authority in a single job — collapsing the least-privilege boundaries that ADR 0036, ADR
0053, and ADR 0058 established, and creating exactly the lateral-movement target that GitHub and
OpenSSF hardening guidance warns about: one compromised step would inherit signing and release
authority simultaneously. External mutex or lease actions (polling waiters, git-branch mutexes,
object-store leases) would preserve the permission split, but the lock itself becomes a trusted
component gating the mutation segment, adds stale-lock and lease-expiry failure modes, and the
ecosystem has largely retired such tools since native concurrency shipped.

The decisive structural observation is about where mutations actually land. Every mutation against
the same remote surface in this system shares a single permission class: the primary asset and the
provenance sidecar both upload to GitHub Release assets under `contents: write`; the release
manifest JSON and its bundle upload to the same surface under the same class. The remaining
cross-job boundaries — npm publish (`id-token: write`, npm registry), the signing jobs (append-only
Sigstore entries), the linked artifact metadata step (`artifact-metadata: write`, registry metadata
API) — each mutate a different remote surface, so no same-surface race exists across jobs. The
platforms supply last-line defenses as well: npm rejects re-publication of an existing version, and
GitHub rejects an upload whose asset name already exists. And ADR 0067 already requires every
mutation job to classify remote state at segment entry and fail closed on `foreign-conflict` or
`indeterminate`.

If strict prevention across a multi-job segment is unattainable and the alternatives are worse, how
should the mutation segment and ADR 0066's single-writer guarantee be redefined so that what the
workflow claims is exactly what the platform can enforce?

## Decision Drivers

- Job-level least privilege is non-negotiable: signing authority (`id-token: write`) and release
  mutation authority (`contents: write`) must never share a job; the ADR 0036/0053/0058 boundaries
  stand.
- Guarantees must match platform capability: an invariant the runner model cannot enforce is an
  overclaim that misleads verifiers and operators.
- Prefer topology over machinery: no new trusted components in the mutation path; external locks
  would enter the trusted computing base.
- Preserve ADR 0067 recoverability and the ADR 0072 sidecar-first ordering unchanged.
- Correctness is fail-closed; liveness degradation under contention is acceptable because releases
  are low-frequency events.
- Keep the ADR 0066 concurrency key and `cancel-in-progress: false`; they remain the mechanism that
  serializes same-surface writers.

## Considered Options

- Single-job mutation segments with detection-based cross-run safety: consolidate each remote
  surface's mutations into exactly one job per run, and redefine cross-run safety as detection via
  the ADR 0067 classification machine.
- Mega-job consolidation: place every mutation — npm publish, signing, release upload, linked
  metadata — in one job that holds the concurrency group for the entire segment.
- External lease or mutex: gate the multi-job segment with a third-party lock action or an
  object-store lease.
- Workflow-level concurrency as the sole segment lock: serialize entire runs at the public
  entrypoint and leave the multi-job segment unchanged.
- Draft staging or merge-queue gating: converge mutations through a staging object or admit releases
  only through a merge queue.

## Decision Outcome

Chosen option: "Single-job mutation segments with detection-based cross-run safety", because it
makes the atomic unit coincide with the unit the platform can actually lock — a job — at zero
permission cost, while the already-accepted classification machine covers the boundaries that
topology cannot.

**The segment is a single job.** All mutations against one remote surface in one profile run must
live in exactly one job. A mutation segment therefore begins when that job enters its concurrency
group and ends when the job completes — the platform's lock now covers the whole segment by
construction. A workflow graph that splits same-surface mutations across jobs fails static
conformance.

**Topology rulings per surface.**

- Release assets (publisher): the primary asset and provenance sidecar uploads consolidate into the
  one release-upload job, which already holds `contents: write` and no signing authority. The ADR
  0072 sidecar-first ordering applies within that job.
- Release manifest: the manifest JSON and its Sigstore bundle already upload from the single publish
  job of the ADR 0053 three-job boundary; this ADR ratifies that arrangement as an instance of the
  principle.
- npm registry: the publish job remains the only job that mutates the registry; the npm three-job
  graph (ADR 0036) is unchanged because its jobs differ in permission class and remote surface.
- Linked artifact metadata: the registry metadata API is a distinct surface with its own job class
  (`artifact-metadata: write`); its ordering relative to release upload is governed by ADR 0067
  classification, not by segment membership.

**Cross-run safety is detection, not prevention, at the run boundary.** Within one remote surface
and one shared concurrency key, at most one job runs at a time — that much the platform enforces.
Across surfaces, and against out-of-band actors, safety comes from the ADR 0067 classification
machine: every mutation step classifies remote state before acting and after any ambiguous call, and
fails closed on `foreign-conflict` or `indeterminate`, with the platforms' own rejections (duplicate
asset name, immutable npm version) as the last line. ADR 0066's single-writer invariant is narrowed
accordingly: it prevents concurrent same-surface writers within the shared key; it does not claim
that a whole run is atomic against other runs. Verification and operator documentation must state
the narrowed guarantee and must not describe a run as an atomic release transaction.

**Acknowledged liveness limitation.** The default concurrency queue holds a single pending job and
replaces it on a third arrival, so contending runs can starve pending mutation jobs. Queue policy
for the mutation groups — including adoption of larger queued-waiting support and possible
workflow-level serialization of the composed entrypoint — is deferred to a separate decision that
depends on spike verification of that newer platform behavior in reusable workflows.

**Scope of partial supersession.** This ADR replaces ADR 0066's multi-job mutation segment
definition and its strict single-writer prevention wording. The concurrency group key composition,
`cancel-in-progress: false`, the job-class permission model, and the prohibition of
`github.workflow` in the key remain in force unchanged. ADR 0067's outcome states, bindings, retry
surface, and reporting obligations are unaffected. ADR 0072's sidecar-first ordering is unaffected
and becomes enforceable within one job.

### Consequences

- Good, because the atomic unit and the lockable unit coincide: the segment's integrity no longer
  depends on undocumented scheduling behavior or same-run stickiness that the platform never
  promised.
- Good, because consolidation is free: every same-surface mutation pair already shares a permission
  class, so no job gains any new authority.
- Good, because the guarantee becomes honest: what the documentation claims — per-surface
  serialization plus fail-closed detection — is exactly what the platform enforces.
- Good, because no new trusted component enters the mutation path, preserving the thin-core posture.
- Bad, because the cross-run story weakens from "one writer at a time, period" to "one writer per
  surface, everything else detected": interleaving runs waste work and fail noisily rather than
  waiting, until the deferred queue-policy decision improves liveness.
- Bad, because the publisher and composition specifications must be rewritten around the single-job
  segment — job graph, segment definition, preflight revalidation placement, and fixtures all
  change.
- Neutral, because the linked artifact metadata step stays outside the segment and gains an explicit
  classification dependency.
- Neutral, because the queue-policy follow-up (planned as a separate ADR) must land before
  high-contention operation is advisable.

### Confirmation

This decision is confirmed when:

- the GitHub Release asset publisher specification defines exactly one release-upload job containing
  both the sidecar-first and primary uploads, and redefines the mutation segment as that job's
  occupancy of the concurrency group, tracing to this ADR;
- the composition and release manifest specifications state the single-job-per-surface principle and
  route every cross-surface ordering rule through ADR 0067 classification;
- a static conformance check rejects any workflow graph that splits same-surface mutations across
  jobs;
- verification documentation states the narrowed guarantee — per-surface serialization plus
  fail-closed detection — and contains no remaining claim of whole-run atomicity;
- fixtures demonstrate: two contending runs where the loser's next mutation step detects
  `foreign-conflict` and fails without mutation; a duplicate-asset-name platform rejection mapped to
  the correct classification; and the consolidated upload job executing the ADR 0072 sidecar-first
  order;
- the follow-up queue-policy decision is recorded as pending and linked from the affected
  specifications.

## Pros and Cons of the Options

### Single-job mutation segments with detection-based cross-run safety

- Good, because it achieves atomicity where it matters — within one remote surface — using only the
  platform's native lock, and covers the rest with machinery that already exists and is already
  accepted.
- Good, because it costs no permission consolidation: same-surface mutations already share a
  permission class, so no job's authority grows.
- Bad, because it formally narrows ADR 0066's guarantee and forces the specification and fixture
  churn described above.

### Mega-job consolidation

- Good, because one job holding the group for the entire release is genuinely atomic across all
  surfaces.
- Bad, because the job must hold signing, release-mutation, package-publish, and metadata authority
  simultaneously, abolishing the least-privilege boundaries of ADR 0036, ADR 0053, and ADR 0058 and
  creating a single step-compromise target with full release authority — the exact lateral-movement
  configuration that GitHub and OpenSSF hardening guidance exists to prevent.

### External lease or mutex

- Good, because it can approximate true mutual exclusion across jobs and runs without merging
  permissions.
- Bad, because the lock implementation becomes part of the trusted base that gates every release
  mutation, stale locks and lease expiry introduce new failure modes requiring their own recovery
  semantics, and the third-party ecosystem around such tools has stagnated since native concurrency
  shipped — an unmaintained gatekeeper is a liability, not a control.

### Workflow-level concurrency as the sole segment lock

- Good, because serializing whole runs would make cross-run interleaving impossible without touching
  the job graph.
- Bad, because under the default queue the pending run is replaced by newer arrivals, so starvation,
  not safety, is the actual outcome; and because whole-run serialization also serializes
  non-mutation phases, paying head-of-line blocking for builds.
- Bad, because it serializes only workflow runs: out-of-band actors — direct API uploads,
  out-of-band npm publishes — bypass run-level serialization entirely, so the classification machine
  remains load-bearing and any strict-prevention wording would still overclaim.
- Neutral, because whether to add whole-run serialization as an additional liveness layer on top of
  single-job segments is a separate queue-policy question, contingent on spike verification in
  reusable workflows; adopting it there would not displace this ADR's topology decision.

### Draft staging or merge-queue gating

- Good, because staging objects and merge queues are proven serialization patterns elsewhere in the
  ecosystem.
- Bad, because they do not fit this system: the publisher uploads to an existing release by design
  (ADR 0043), and releases here are tag- and dispatch-driven, not merge-driven — the patterns solve
  a different project's problem.

## More Information

- Concurrency group semantics — one running plus one pending by default, pending replacement,
  `cancel-in-progress` scope, FIFO by wait-start time:
  <https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency>;
  larger queued waiting (`queue: max`, 12026-05):
  <https://github.blog/changelog/2026-05-07-github-actions-concurrency-groups-now-allow-larger-queues/>.
- Caller-scoped `github` context in reusable workflows:
  <https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations>.
- Environments are not mutexes ("concurrency and environment are not connected"):
  <https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/control-deployments>.
- Job isolation rationale in the reference SLSA generator:
  <https://github.com/slsa-framework/slsa-github-generator/blob/main/SPECIFICATIONS.md>; npm trusted
  publishing's dedicated `id-token: write` publish job:
  <https://docs.npmjs.com/trusted-publishers/>; `actions/attest` permission requirements:
  <https://github.com/actions/attest>.
- Least-privilege and lateral-movement guidance:
  <https://docs.github.com/en/actions/reference/security/secure-use>,
  <https://openssf.org/blog/2024/08/12/mitigating-attack-vectors-in-github-workflows/>.
- Ecosystem serialization tools surveyed and set aside: softprops/turnstyle (polling waiter),
  git-branch mutex actions, object-store lease actions; release-please and changesets staging
  patterns; merge queue documentation:
  <https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue>.
- Related decisions: ADR 0066 (job-class concurrency), ADR 0067 (run-identity convergence and the
  classification machine), ADR 0072 (sidecar-first pair binding).
