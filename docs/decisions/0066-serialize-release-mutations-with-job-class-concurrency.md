---
parent: Decisions
nav_order: 66
status: accepted
date: 12026-08-02
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0026
  - type: see-also
    target: ADR-0058
  - type: see-also
    target: ADR-0060
  - type: see-also
    target: ADR-0067
---

# Serialize Release Mutations with Job-Class Concurrency

## Context and Problem Statement

ADR 0036 and ADR 0053 define the three-job graphs that place npm publication, release asset upload,
and release manifest signing in separate jobs, and ADR 0058 constrains which jobs may hold release
mutation authority. Those decisions fix the spatial authority topology: which job may mutate what.
They do not fix the temporal one: nothing prevents two concurrent runs of the same profile, in the
same caller repository, for the same release ref, from passing every pre-mutation check and then
racing the mutation jobs.

The race is realistic. The npm version-existence check and the release asset existence check are
both read-then-write: two runs can each observe "not yet published" before either commits. The
platform offers no transaction spanning npm publish, release asset upload, and manifest publication,
so a losing run can also strand a partial state such as "published to npm, not yet uploaded to the
release".

The documented platform mechanics are narrow. GitHub Actions concurrency groups are
repository-scoped and allow at most one running and one pending execution per group by default; a
third arrival cancels and replaces the pending one. `cancel-in-progress: true` additionally cancels
the running execution, while a `queue: max` variant raises the pending cap and rejects overflow.
Inside a called reusable workflow the `github` context is caller-scoped, so `github.workflow`
resolves to the caller's workflow name and must not be used in group keys. Cancelling a running job
delivers SIGINT, then SIGTERM after 7.5 seconds, then kills the process tree; GitHub documents no
rollback for external side effects, so a cancelled publish or upload may have partially committed.
Environments gate but do not serialize, and artifact names are immutable, not locks.

The registries themselves are idempotent at the final write: npm rejects a duplicate `name@version`
(`EPUBLISHCONFLICT`) and GitHub rejects a duplicate asset name (`422 already_exists`). Those checks
keep races from corrupting remote state, but they turn every race into a failure instead of an
ordering, and they say nothing about cancellation.

Verifiers reason about which run produced an artifact. SLSA expects a globally unique
`invocationId`, and Sigstore certificates bind `run_id` and `run_attempt`, so the run identity
primitives exist. What is missing is a policy that keeps two runs from holding release mutation
authority at the same time.

How should the profile guarantee a single writer for each release intent, what should happen to a
concurrent contender, and in what unit should "the same release intent" be defined — without relying
on caller configuration and without manufacturing ambiguous commits by design?

This ADR decides the exclusion policy only. Recovery semantics for externally interrupted or
repeated runs — manual cancellation, runner failure, rerun versus new release (which ADR 0026
delegated to specifications), and read-back determination of ambiguous commits — are the subject of
a follow-up decision. This decision bounds that one by guaranteeing the serialization mechanism
itself never cancels a mutation in flight.

## Decision Drivers

- Enforce a single writer per release intent regardless of caller configuration.
- Never create an ambiguous commit by policy: the mechanism must not cancel a mutation in flight.
- Keep the fail-closed posture: a contender that loses the race fails with a clear reason and never
  silently publishes.
- Avoid wasted compute where it is safe: stale pre-mutation work may be cancelled because it has no
  release, registry, or manifest side effects.
- Preserve verifier reasoning about which run committed: run identity stays bound to exactly one
  mutation segment execution.
- Generalize beyond npm: the publisher is producer-neutral, so the policy must hold for future
  producer profiles.
- Use only documented GitHub Actions mechanics and respect the caller-scoped context constraints of
  reusable workflows.
- Keep registry idempotency as a safety net beneath the policy, not as the policy.

## Considered Options

- Caller-owned serialization only; the reusable workflow declares nothing.
- Uniform callee-enforced serialization: the callee queues whole runs or mutation jobs with
  `cancel-in-progress: false`.
- Callee-enforced preemption: the newest contender cancels the running run with
  `cancel-in-progress: true`.
- No declared mutex; rely on npm and GitHub write idempotency plus pre-mutation duplicate checks.
- Job-class concurrency: cancel stale pre-mutation work, serialize the mutation segment.

## Decision Outcome

Chosen option: "Job-class concurrency: cancel stale pre-mutation work, serialize the mutation
segment", because it is the only option that both structurally prevents mutex-induced ambiguous
commits and avoids paying for stale computation, while remaining enforceable inside the reusable
workflow regardless of caller configuration.

The profile divides each run into two segments with different contention policies:

- The pre-mutation segment — checkout, dependency installation, build, pack, provenance generation,
  and signing — has no release, registry, or manifest side effects. Signing leaves a transparency
  log entry, but an attestation for bytes that are never published is inert: verification binds
  attestations to published artifacts, not the reverse. Pre-mutation jobs may therefore use
  job-level concurrency groups with `cancel-in-progress: true`, so that a newer run for the same
  release intent replaces stale in-flight work early.
- The mutation segment — the npm publish job, the release asset upload job, and the release manifest
  publication job — holds release mutation authority as bounded by ADR 0036, ADR 0053, and ADR 0058.
  These jobs must use job-level concurrency with `cancel-in-progress: false`, so a contender waits
  rather than interrupts a mutation in flight.

The unit of run ownership is one release intent: the caller repository plus the release source ref.
Production publication is constrained to tag refs by ADR 0026 and ADR 0032, so the source ref
identifies the release intent for both npm-only and release-asset runs. Group keys must be built
only from documented contexts that are meaningful inside a called workflow — `github.repository`,
`github.ref_name`, and declared `inputs` — and must never include `github.workflow`, which resolves
to the caller's workflow name and can make a caller cancel itself. Keys must also be namespaced per
job or per segment so that jobs within the same run never contend with one another; contention is
defined only across runs.

All mutation jobs for one release intent share one mutation group, so the first mutation job acts as
the segment gate. Because a queued contender's earlier checks are stale by the time it enters, each
mutation job must re-validate its duplicate and existence preconditions at job start, inside the
segment, before its first mutating call. A contender that finds the release intent already committed
must fail closed with a reason that names the conflicting remote state.

The project keeps the platform default pending policy: at most one running and one pending execution
per group, and a newer queued run replaces the older pending one. For release intents the
replacement semantics read as "the latest queued release intent for a ref supersedes the earlier
queued intent", which matches how releases are actually driven. The `queue: max` variant is rejected
for the initial contract: a first-in-first-out backlog of up to one hundred release runs is not a
meaningful release semantic, and its overflow rejection behavior is not relied upon until verified.

Because groups are repository-scoped, serialization never crosses caller repositories: two consumers
of the same profile never block each other, and the trusted builder's availability is not shared
state.

Caller-side serialization of the whole invocation remains a documented optional optimization that
saves duplicate compute; the callee must never rely on it. Registry idempotency — npm's
duplicate-version rejection and GitHub's duplicate-asset rejection — remains enabled at all times as
the safety net beneath the policy, not as a substitute for it.

This decision adds a temporal dimension to the existing authority boundaries without changing any
earlier clause: the ADR 0026 caller patterns, the ADR 0058 authority topology, and the ADR 0060
public entrypoint all remain in force.

Out of scope here, and deferred: exact group key strings, per-job re-validation details, and
contention fixtures belong to the architecture specifications. Recovery semantics for externally
interrupted or repeated runs — including read-back determination of ambiguous commits using the
release asset digest field — belong to a follow-up decision together with the rerun semantics ADR
0026 delegated to specifications.

### Consequences

- Good, because a single writer per release intent is enforced by the trusted workflow itself,
  regardless of caller configuration.
- Good, because the mutex never cancels a mutation in flight, so the policy cannot manufacture an
  ambiguous commit; the remaining ambiguous-commit sources are external interruptions, which the
  follow-up recovery decision addresses.
- Good, because stale pre-mutation work is cancelled early, bounding wasted compute when a caller
  re-triggers a release.
- Good, because a contender that loses the race fails closed inside the mutation segment with a
  clear, re-validated reason instead of corrupting or double-mutating remote state.
- Good, because the policy is producer-neutral: any future producer profile that feeds the publisher
  inherits the same ownership unit and segment rules.
- Good, because repository-scoped groups give cross-consumer isolation for free.
- Bad, because the workflow graph gains per-job concurrency declarations and key-construction rules
  that must be documented, reviewed, and fixture-tested.
- Bad, because a queued contender whose release intent is superseded observes a cancellation it did
  not request; documentation must explain the pending-replacement semantics.
- Neutral, because signing and provenance generation are classified as cancellation-safe on the
  strength of the rule that verification binds attestations to published artifacts; verification
  policy and fixtures must keep that rule true.

### Confirmation

This decision is confirmed when:

- architecture specifications define the two segments, the per-job concurrency group key
  composition, the `cancel-in-progress` values, and the in-segment re-validation requirements for
  the npm publish, release asset upload, and release manifest publication jobs;
- workflow implementations declare job-level concurrency on every mutation job with
  `cancel-in-progress: false` and on pre-mutation jobs with `cancel-in-progress: true`, using only
  documented contexts in keys and never `github.workflow`;
- review checklists verify that no mutation job may be cancelled by the concurrency mechanism and
  that group keys cannot collide across jobs within one run;
- fixtures demonstrate two concurrent runs for the same release intent: the first commits, the
  second fails closed inside the mutation segment naming the committed remote state, and a stale
  pre-mutation run is cancelled by a newer run for the same ref;
- caller documentation describes the pending-replacement semantics and the optional caller-side
  serialization of the whole invocation.

## Pros and Cons of the Options

### Caller-owned serialization only

The reusable workflow declares no concurrency; callers are told to serialize their own release jobs.

- Good, because there is nothing to implement in the trusted workflow, and whole-invocation
  serialization avoids duplicate compute.
- Bad, because it is unenforceable: a caller that omits the declaration races unprotected, the
  trusted builder cannot claim single-writer, and verifiers cannot rely on any serialization
  invariant.
- Bad, because it makes the unsafe configuration the default, contradicting the project's
  fail-closed posture.

### Uniform callee-enforced serialization with queued contenders

The callee declares `cancel-in-progress: false` on the whole workflow or uniformly on mutation jobs;
contenders wait.

- Good, because it is enforceable and never cancels a mutation in flight.
- Bad, because queuing whole runs makes a superseded release intent run to completion before the
  newest intent discovers the conflict, and head-of-line blocking wastes the most compute exactly
  when a caller is iterating on a broken release.
- Bad, because uniform job-level queuing without a pre-mutation cancellation class still pays for
  stale builds that a newer run for the same ref has already made irrelevant.

### Callee-enforced preemption of the running run

The newest contender cancels the running run with `cancel-in-progress: true`.

- Good, because the latest release intent always wins immediately and stale compute is cut early.
- Bad, because cancellation can land mid-publish or mid-upload with no rollback, so the policy
  itself manufactures ambiguous commits and pushes the entire recovery burden onto verifier-facing
  semantics.
- Bad, because provenance-producing runs can be cut between signing and publication, weakening
  verifier reasoning about which run committed.
- Bad, because key collisions with caller-declared groups can make a caller cancel itself; the
  caller-scoped `github` context makes this easy to get wrong.

### No declared mutex; platform idempotency only

Rely on npm duplicate-version rejection, GitHub duplicate-asset rejection, and pre-mutation
duplicate checks.

- Good, because there is no mechanism to design, document, or test, and the registry checks do in
  fact prevent double commits.
- Bad, because races surface as failures rather than ordering: even a legitimate identical retry
  fails with a duplicate error, and multi-step mutations leave partial states (published to npm, not
  yet uploaded) for concurrent runs to trip over.
- Bad, because cancellation semantics remain undefined and the builder's guarantee is reduced to
  optimistic scripting, which prior art shows is not enough: semantic-release documents
  concurrent-run failures in the absence of a cross-process lock.

### Job-class concurrency: cancel stale pre-mutation work, serialize the mutation segment

- Good, because the unsafe-to-cancel segment is structurally protected while the safe-to-cancel
  segment is aggressively preempted — the changesets "serialize releases, cancel stale CI" pattern
  applied at job level.
- Good, because the workflow structure itself expresses which work may be interrupted, which is a
  stronger contract than documentation.
- Bad, because it is the most complex option: per-job group keys, a segment classification rule, and
  in-segment re-validation all need specification, review checklists, and fixtures.
- Bad, because the cancellation-safe classification of signing depends on verification policy
  continuing to bind attestations to published artifacts rather than treating attestations as
  publication evidence.

## More Information

Platform mechanics: concurrency groups are repository-scoped with at most one running and one
pending execution by default
(<https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency>);
limits and the `queue: max` variant (<https://docs.github.com/en/actions/reference/limits>);
caller-scoped `github` context and reusable workflow constraints
(<https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations>);
cancellation signal sequence and the absence of rollback for external side effects
(<https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation>);
environments gate but do not serialize
(<https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/control-deployments>).

Registry idempotency: npm rejects duplicate versions with `EPUBLISHCONFLICT`
(<https://docs.npmjs.com/cli/v11/commands/npm-publish/>); GitHub rejects duplicate asset names with
`422 already_exists` and has exposed SHA-256 asset digests for read-back verification since
12025-06-03 (<https://docs.github.com/en/rest/releases/assets>,
<https://github.blog/changelog/2025-06-03-releases-now-expose-digests-for-release-assets/>).

Prior art: changesets serializes its publish workflow with `cancel-in-progress: false` and cancels
stale pull request flows (<https://github.com/changesets/action>); slsa-github-generator declares no
concurrency primitive and relies on platform idempotency
(<https://github.com/slsa-framework/slsa-github-generator>); semantic-release documents
concurrent-run failures in the absence of a cross-process lock
(<https://github.com/semantic-release/semantic-release/issues/1545>).

Verifier identity: SLSA v1 provenance expects a globally unique `invocationId`, and the GitHub
Actions build type recommends `run_id` plus `run_attempt`
(<https://slsa.dev/spec/v1.2/build-provenance>,
<https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/README.md>);
Fulcio certificates bind run identity and the called reusable workflow ref
(<https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md>).

This ADR resolves the serialization axis of the pre-implementation audit finding on release mutation
run ownership. The recovery axis — interrupted and repeated run outcome semantics, including the
rerun semantics ADR 0026 delegated to specifications and read-back determination via asset digests —
is expected to be decided as a separate follow-up ADR.
