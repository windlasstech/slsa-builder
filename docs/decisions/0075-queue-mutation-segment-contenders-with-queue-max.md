---
parent: Decisions
nav_order: 75
status: accepted
date: 12026-08-04
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0066
    scope:
      "the platform-default pending policy clause and the initial rejection of queue: max; the
      concurrency group key composition, the cancel-in-progress values for both job classes, the
      job-class permission model, and the prohibition of github.workflow in group keys remain in
      force"
  - type: amends
    target: ADR-0074
    scope:
      "the acknowledged liveness limitation: pending mutation jobs can no longer be starved or
      silently evicted by later arrivals; the single-job segment topology and detection-based
      cross-run safety are unchanged"
  - type: see-also
    target: ADR-0067
  - type: see-also
    target: ADR-0072
---

# Queue Mutation Segment Contenders with queue: max

## Context and Problem Statement

ADR 0066 serialized release mutations with job-class concurrency and deliberately kept the platform
default pending policy: at most one running and one pending execution per concurrency group, with a
newer queued run replacing the older pending one. It rejected the `queue: max` variant "for the
initial contract" on two grounds: a first-in-first-out backlog of up to one hundred release runs was
not a meaningful release semantic, and the feature's behavior — in particular inside reusable
workflows — was not yet verified. ADR 0074 then consolidated each remote surface's mutations into a
single job and explicitly acknowledged the resulting liveness limitation: the default queue holds a
single pending job and replaces it on a third arrival, so contending runs can starve pending
mutation jobs. It deferred the queue policy to this decision, contingent on spike verification, and
recorded that the follow-up must land before high-contention operation is advisable.

The spikes are now executed (12026-08-04, `yunseo-kim/slsa-spike-tmp`@`329f386`), and they answer
every question ADR 0066 left unverified:

- `queue: max` on a job declared **inside a called reusable workflow** is accepted and enforced:
  with four rapid same-group dispatches, one running and three pending executions coexisted and all
  four completed; no pending execution was cancelled. A control group with default concurrency
  reproduced the documented pending replacement (the middle run was cancelled fifteen seconds after
  dispatch when the third arrived).
- The pending queue releases executions in **arrival order**: job start times exactly matched
  dispatch order, each job starting only after its predecessor completed. The platform documents
  ordering by when waiting started, not by dispatch time.
- Workflow-level `concurrency` declared in a reusable workflow is **honored**, and the `github`
  context used for group evaluation is **caller-scoped** (`github.run_id`, `github.workflow`, and
  `github.workflow_ref` inside the callee resolve to the caller's run and workflow).

The documented platform semantics are now precise: `concurrency.queue` accepts `single` (the
default: one running, one pending, pending replacement) or `max` (one running, up to one hundred
pending, arrivals beyond the cap rejected); `queue: max` combined with `cancel-in-progress: true` is
a workflow validation error.

Spike verification also exposed the load-bearing defect of the default policy for this system.
Within one concurrency group (`release-mutation-<repository>-<ref_name>`), every contender targets
the same release intent by construction — same caller repository, same source ref. Unlike a
cumulative deployment pipeline, where a newer run carries newer content and genuinely subsumes an
older one, a newer run here carries nothing the older run lacks; the contenders are duplicate
executions of one intent. Which duplicate the policy preserves therefore matters. Under ADR 0066 and
ADR 0067, exactly one `run_id` ever commits a release intent, and exactly one class of execution is
entitled to converge afterward: a re-run-failed-jobs attempt of that same `run_id` (ADR 0072's
pair-gate and ADR 0073's attestation binding both ignore the attempt component precisely to enable
this). The default replacement policy evicts whichever pending execution is oldest — potentially the
retry attempt that alone can classify as `committed-as-expected` — in favor of a fresh run whose
different `run_id` guarantees a `foreign-conflict` failure once it enters the segment. The policy
thus prefers the doomed contender over the entitled one, and the evicted contender disappears with a
bare platform cancellation: no classification event, no report substate, nothing the ADR 0067
machine ever observes. ADR 0066's own decision driver requires that a contender losing the race
"fail with a clear reason"; silent pending replacement does not meet that driver, and the ecosystem
has documented the operator harm (a queued release silently dropped behind a newer push).

How should the mutation concurrency groups queue contending executions, now that `queue: max` is
spike-verified on this system's exact topology — and should any additional serialization layer
accompany it?

## Decision Drivers

- Preserve ADR 0067 convergence liveness: the execution entitled to converge — the retry attempt of
  the committing `run_id` — must not be evicted from the queue by a contender that is guaranteed to
  fail closed.
- Every execution that reaches the mutation queue must receive a classified outcome: success,
  convergence, or a named failure — never a bare platform cancellation.
- Keep ADR 0066's compute-saving property intact: stale pre-mutation work must still be cancelled
  early; queue policy must not force doomed contenders to run.
- Use only spike-verified platform behavior; add no external machinery to the trusted computing
  base.
- Keep the ADR 0066 concurrency key, the `cancel-in-progress` values, and the repository-scoped
  cross-consumer isolation unchanged.
- Accept that github.com is the only target platform; do not pay design costs for platforms the
  system structurally cannot serve.
- Name every platform failure surface the policy introduces, even ones unreachable in practice.

## Considered Options

- Keep the platform default pending policy (status quo: one running, one pending, pending
  replacement).
- Queue mutation segment contenders with `queue: max` on the mutation jobs only.
- Add a workflow-level serialization layer on the composed entrypoint as an additional liveness
  layer.
- Layer queued mutation segments with workflow-level serialization (both of the above).
- Document a caller-side opt-in serialization pattern only; change nothing in the callee.

## Decision Outcome

Chosen option: "Queue mutation segment contenders with `queue: max` on the mutation jobs only",
because it is the only option that preserves the convergence entitlement of the committing `run_id`,
gives every queued contender a classified outcome, and does so without disturbing ADR 0066's
job-class model — and because every objection ADR 0066 recorded against it is now answered by spike
evidence or by this decision's scoping.

The mutation segment jobs — the npm publish job (ADR 0036), the single release asset upload job (ADR
0058, as consolidated by ADR 0074), and the release manifest publication job (ADR 0053) — MUST
declare job-level `concurrency` with the ADR 0066 group key, `cancel-in-progress: false`
(unchanged), and `queue: max`. Pre-mutation jobs are unchanged: `cancel-in-progress: true`, no
`queue` key, so stale pre-mutation compute is still cancelled early. The unit of run ownership
remains one release intent (caller repository plus release source ref); contenders for one intent
now wait in arrival order instead of replacing one another.

The convergence interaction is the deciding consequence. A re-run-failed-jobs attempt that reaches
the mutation queue keeps its position when fresh runs arrive, enters the segment in turn, and
converges as `committed-as-expected` through the ADR 0072 pair-gate or the ADR 0073 attestation
binding. Fresh runs that waited behind it then enter and fail closed with `foreign-conflict`, named
and reported. No execution disappears unclassified.

No workflow-level serialization layer is added in the callee. Caller-side serialization of the whole
invocation remains what ADR 0066 made it: a documented, optional, caller-owned optimization that
saves duplicate compute and that the callee must never rely on.

The platform rejects arrivals beyond one hundred pending executions in one group. That surface is
unreachable in release practice — one hundred queued executions of one release intent — but the
failure taxonomy MUST name it distinctly from `foreign-conflict` and `indeterminate`, because the
project does not leave platform behavior unclassified.

**Platform scope.** This decision targets github.com only, and that is a structural fact rather than
a new restriction. GitHub Enterprise Server cannot consume a reusable workflow hosted on github.com
— GitHub Connect bridges actions, not reusable workflows — so the trusted builder's only supported
consumption path does not exist there. The trust stack is equally github.com-bound: artifact
attestations are not supported on GHES, npm trusted publishing requires github.com GitHub-hosted
runners and the public OIDC issuer, the public Sigstore instance binds the github.com issuer, and
ADR 0068 verification binds github.com Run Invocation URIs. The only GHES route — mirroring the
workflows into a GHES instance — abandons the trusted builder model by construction and is out of
scope; a mirror that carries `queue: max` onto GHES today fails closed (the run materializes with
zero jobs and publishes nothing), which mirrors the project's posture. If a GHES deployment is ever
contemplated, queue policy is one item on a long incompatibility list and this decision should be
revisited then, not pre-paid now.

**Scope of partial supersession.** This ADR replaces ADR 0066's platform-default pending policy
clause and its initial rejection of `queue: max`. The concurrency group key composition, the
`cancel-in-progress` values for both job classes, the job-class permission model, and the
prohibition of `github.workflow` in group keys remain in force unchanged. ADR 0074's single-job
segment topology and detection-based cross-run safety are unaffected; the liveness limitation it
acknowledged is mitigated, not merely restated.

### Consequences

- Good, because the execution entitled to converge — the retry attempt of the committing `run_id` —
  keeps its queue position, so ADR 0067's automatic recovery path can no longer be silently
  dismantled by a later arrival that is guaranteed to fail.
- Good, because every contender receives a classified outcome: the queue no longer deletes pending
  runs without a classification event, satisfying ADR 0066's own decision driver.
- Good, because the mechanism is platform-native and spike-verified on this system's exact topology
  (job-level, inside a reusable workflow, FIFO release order); nothing enters the trusted computing
  base.
- Good, because ADR 0066's compute-saving design is fully preserved: pre-mutation jobs still cancel
  stale work early, and only mutation jobs queue.
- Good, because FIFO queueing of must-complete serialized work matches mainstream precedent —
  GitHub's merge queue, GitLab `oldest_first`, Jenkins lockable-resources default — while the
  rejected latest-wins pattern (Azure `runLatest`, GitLab `newest_first`) serves cumulative deploys
  whose semantics this system does not have.
- Good, because repository-scoped groups keep cross-consumer isolation: one consumer's queue never
  delays another's.
- Bad, because contenders guaranteed to fail closed still wait in the queue and burn pre-mutation
  compute before their classified failure; the waste is bounded by segment duration and is identical
  under the default policy up to the moment of eviction, but it is real.
- Bad, because the policy depends on a platform feature shipped 2026-05: `act` rejects the `queue`
  key, actionlint support only recently landed, and GHES fails silently with zero jobs — the last
  being moot under the structural exclusion above, but the tooling lag is not.
- Bad, because the specifications, conformance checks, fixtures, and caller documentation must all
  gain queue-semantics content: the failure taxonomy, the FIFO fixtures, and the operator-facing
  explanation of waiting behavior.
- Neutral, because queue depth in release practice will sit near zero; the policy exists for the
  contentious tail, not the common path.
- Neutral, because an unsupported GHES mirror fails closed and silently — acceptable for an
  unsupported topology, and documented here so the failure mode is at least not a surprise.

### Confirmation

This decision is confirmed when:

- architecture specifications declare `queue: max` with `cancel-in-progress: false` on the npm
  publish job, the release asset upload job, and the release manifest publication job, with the ADR
  0066 group key unchanged, and leave pre-mutation jobs at `cancel-in-progress: true` with no
  `queue` key;
- static conformance requires `queue: max` on every mutation segment job, rejects
  `cancel-in-progress: true` combined with `queue: max` (a platform validation error), and continues
  to reject any split of same-surface mutations across jobs (ADR 0074);
- the shared diagnostic taxonomy gains a distinct classification for platform queue-overflow
  rejection, and the specification pins the overflow surface (rejection versus cancellation) from
  platform documentation or a dedicated spike;
- fixtures demonstrate: three rapid same-intent dispatches all waiting with none cancelled and
  starting in arrival order; and a re-run-failed-jobs attempt queued behind a running segment
  keeping its position when a fresh run arrives, converging as `committed-as-expected`, after which
  the fresh run fails closed with `foreign-conflict`;
- caller documentation explains the FIFO waiting semantics — a queued run waits, then either
  converges (same `run_id`) or fails closed naming the committed remote state — and restates that
  caller-side whole-invocation serialization is optional and never load-bearing;
- the spike evidence (repository commit and run identifiers) is recorded in this decision's More
  Information section.

## Pros and Cons of the Options

### Keep the platform default pending policy (status quo)

- Good, because it introduces no dependency on a recently shipped platform feature; it behaves
  identically on GHES — though that buys nothing, since GHES is structurally excluded as a consumer
  of this builder regardless of queue policy.
- Good, because latest-wins pending replacement is a mainstream pattern for cumulative deployment
  pipelines (Azure DevOps `runLatest` default, GitLab `newest_first` to prevent outdated deploys).
- Good, because releases are low-frequency events, so contention is rare in the common path.
- Bad, because the cumulative-deploy justification does not transfer: within one group the
  contenders are duplicate executions of one intent, and the policy systematically preserves the
  newest — the one guaranteed to fail closed — while evicting the oldest, the only one entitled to
  converge. It inverts the survival priority the recovery model needs.
- Bad, because an evicted pending run disappears with a bare platform cancellation and no
  classification event, contradicting ADR 0066's own decision driver and the project's fail-closed
  reporting posture; the operator harm is documented in the ecosystem (a queued release silently
  dropped behind a newer push).
- Bad, because it leaves ADR 0074's acknowledged starvation limitation standing, and ADR 0074
  recorded that this follow-up must land before high-contention operation is advisable.

### Queue mutation segment contenders with queue: max (chosen)

- Good, because it is spike-verified on this system's exact topology: job-level, declared inside a
  called reusable workflow, holding multiple pending executions and releasing them in arrival order.
- Good, because FIFO preserves the queue position of the only execution entitled to converge, so the
  ADR 0067 recovery path survives contention intact.
- Good, because every contender eventually receives a classified outcome; nothing is silently
  dropped.
- Good, because it composes with ADR 0066 unchanged: group keys, permission classes, pre-mutation
  cancellation, and cross-consumer isolation all stand.
- Good, because it matches the mainstream fairness precedent for must-complete serialized work and
  has real adoption, including job-level release serialization and the fix for a reusable-workflow
  concurrency collision in the wild.
- Bad, because it depends on a 2026-05 platform feature with uneven tooling support: `act` rejects
  the key, actionlint only recently added it, and GHES does not support it at all. The GHES gap is
  moot — GHES cannot consume this builder through any supported path, and an unsupported mirror
  fails closed — but a team vendoring the workflows for non-trusted use must be told to strip the
  key.
- Bad, because doomed contenders wait in queue and burn pre-mutation compute before failing closed,
  where the default policy would have evicted them earlier and more cheaply.
- Bad, because the one-hundred-pending overflow surface must be named and pinned even though release
  practice will never approach it.

### Add a workflow-level serialization layer on the composed entrypoint

- Good, because the spikes verified the mechanics: workflow-level `concurrency` in a reusable
  workflow is honored and evaluated with caller-scoped context, and callee-declared workflow-level
  concurrency has production precedent (Metabase, Envoy, github/docs).
- Good, because it serializes before any compute starts, avoiding doomed contenders' pre-mutation
  spend.
- Bad, because it neutralizes ADR 0066's compute-saving design from the other side: serialized stale
  runs wait and then execute instead of being cancelled early, and whole-run queueing blocks
  non-mutation phases — the rationale for which ADR 0074 rejected workflow-level concurrency as a
  substitute, which applies to its cost as a layer as well.
- Bad, because ADR 0066 already considered and rejected callee-enforced whole-run queuing, and
  nothing in the spike results reverses that analysis.
- Bad, because it installs a second queue owner for the same release intent; the documented
  layered-locking anti-patterns (GitLab parent/child pipelines sharing one resource group, Jenkins
  chained throttling) warn against exactly this shape.
- Neutral on GHES: workflow-level `concurrency` is old enough to exist there, but the platform is
  structurally excluded anyway, so compatibility here purchases nothing.

### Layer queued mutation segments with workflow-level serialization

- Good, because it is maximal defense in depth for liveness: the workflow-level layer throttles the
  composed path and the job-level queue backstops every other entry path.
- Bad, because it violates the one-queue-owner-per-resource guidance and puts two queues in series
  for one intent: double waiting, opaque composite ordering, and the hardest fixture and
  documentation burden of any option.
- Bad, because it inherits every cost of both layers, including neutralized pre-mutation
  cancellation, for a contention profile releases do not have.
- Neutral on GHES: as above, structural exclusion makes the platform dimension irrelevant to the
  choice.

### Document a caller-side opt-in serialization pattern only

- Good, because it changes nothing in the callee and ADR 0066 already permits caller-side
  serialization as an optional optimization; it also runs on any platform — moot, given the
  structural GHES exclusion.
- Bad, because the callee can never rely on caller behavior (ADR 0066), so it cannot repair the
  callee-internal gap this decision exists to close: silent pending eviction and starvation happen
  inside the callee's own groups.
- Bad, because the ecosystem's reusable-workflow collision incident shows caller variance is the
  failure mode, not the safety layer.
- Bad, because it leaves the convergence-eviction scenario — the loss of the only execution entitled
  to converge — fully exposed.

## More Information

Spike verification (12026-08-04): repository `yunseo-kim/slsa-spike-tmp` at commit `329f386`; run
identifiers 30916657598, 30916667783, 30916678156, 30916689430 (`queue: max` set), 30916699473,
30916709194, 30916720258 (default control), 30916729353, 30916734263 (workflow-level pair). Syntax
reference: `concurrency.queue` with `single` (default) and `max`
(<https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency>);
the one-hundred-pending cap and overflow rejection
(<https://docs.github.com/en/actions/reference/limits>); the 12026-05-07 changelog
(<https://github.blog/changelog/2026-05-07-github-actions-concurrency-groups-now-allow-larger-queues/>).

Cross-platform queue-policy precedent: GitLab resource-group process modes, including `oldest_first`
and `newest_first` (<https://docs.gitlab.com/ci/resource_groups/>); Azure DevOps `runLatest` versus
`sequential`
(<https://learn.microsoft.com/en-us/azure/devops/pipelines/process/approvals?view=azure-devops>);
Jenkins lockable resources, FIFO by default with opt-in `inversePrecedence` and `priority`
(<https://www.jenkins.io/doc/pipeline/steps/lockable-resources/>); GitHub merge queue FIFO
(<https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue>).

`queue: max` adoption and incident evidence: job-level release serialization keeping superseded
pending runs queued
(<https://github.com/millionco/react-doctor/blob/25dbf6d92524f2495e6f81bdc68b710ce434bc69/.github/workflows/action-version-bump.yml#L98-L107>);
workflow-level release and deploy serialization
(<https://github.com/zwave-js/zwave-js-ui/blob/f3669bdaadde973f86f51f3af2a585a4f0b3d39b/.github/workflows/docker-release.yml#L1-L12>,
<https://github.com/NangoHQ/nango/blob/c17195e2e874fd6474a13afe1fa8399b610b86c6/.github/workflows/deploy.yaml#L31-L40>,
<https://github.com/ggml-org/llama.cpp/blob/1c3c9674de4d455f1e571bed808252af54932767/.github/workflows/build-cuda-windows.yml#L8-L14>);
silent pending-drop reports (<https://github.com/devantler-tech/homebrew-tap/issues/1283>,
<https://github.com/actions/runner/issues/3722>); the reusable-workflow group collision whose fix
combined key namespacing with `queue: max` (<https://github.com/github/gh-aw/issues/35161>,
<https://github.com/github/gh-aw/pull/35173>); tooling support status
(<https://github.com/rhysd/actionlint/pull/654>, <https://github.com/nektos/act/issues/6095>).

GitHub Enterprise Server structural exclusion: reusable workflows are instance-scoped even with
GitHub Connect; artifact attestations are not supported on GHES; npm trusted publishing requires
github.com GitHub-hosted runners and the public OIDC issuer
(<https://docs.npmjs.com/trusted-publishers>); the GHES silent zero-job failure with `queue: max`
(<https://github.com/github/gh-aw/pull/39722>). If a mirrored GHES deployment is ever contemplated,
re-check GHES feature parity for `concurrency.queue` at that time and revisit this decision.
