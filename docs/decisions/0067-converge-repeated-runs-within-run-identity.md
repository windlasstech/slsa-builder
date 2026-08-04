---
parent: Decisions
nav_order: 67
status: accepted
date: 12026-08-02
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0026
    scope:
      "the rerun-versus-new-release semantics that ADR 0026 delegated to the architecture
      specification; ADR 0067 decides them at ADR level"
  - type: see-also
    target: ADR-0053
  - type: see-also
    target: ADR-0058
  - type: see-also
    target: ADR-0066
  - type: partially-superseded-by
    target: ADR-0072
    scope:
      "the release-asset digest-only binding proof and the never-silently-adopt and
      singular-committer clauses as applied to unsigned release asset publication events; all other
      ADR 0067 semantics remain in force"
  - type: amended-by
    target: ADR-0073
    scope:
      "the npm publish step binding proof: expected `dist.integrity` equality remains necessary but
      is no longer sufficient; same-run convergence additionally requires a published attestation
      whose verified run identity equals the current run_id"
  - type: see-also
    target: ADR-0074
  - type: see-also
    target: ADR-0075
  - type: see-also
    target: ADR-0076
---

# Converge Repeated Runs Within Run Identity

## Context and Problem Statement

ADR 0066 serialized release mutations and deliberately excluded recovery semantics: it guaranteed
that the concurrency mechanism never cancels a mutation in flight, and left the outcome of
externally interrupted or repeated runs to this decision. Interruption sources that remain are
manual cancellation, runner or infrastructure failure, and timeouts. A run can therefore stop
between mutation steps — for example after a successful npm publish but before the release asset
upload — leaving a partial release state behind.

The cost of getting this wrong is asymmetric. npm versions are immutable, so "publish the same
version again as a new release" is not physically possible; the only meanings a repeated execution
can have are "continue the same release intent" or "a new release intent that must be rejected". A
policy that fails every repeated execution turns a transient infrastructure fault into permanent
loss of the trusted redistribution path for that version. A policy that blindly retries risks double
mutation or, worse, silently adopting remote state the run did not produce — the failure mode a
trusted builder must never have.

The architecture specifications currently impose strict duplicate-fail behavior on reruns
(`release-manifest.md`, `github-release-asset-publisher.md`). Those clauses were written before any
ADR decided rerun semantics, and they treat every repeated execution identically regardless of run
identity. ADR 0026 also deferred the question "how a rerun differs from a new release" to the
architecture specification. This ADR now decides both at ADR level.

Platform behavior relevant to the decision was verified by a controlled spike on 12026-08-02
(private repository `yunseo-kim/slsa-spike-tmp`, deleted after the experiment):

- A workflow run cancelled mid-job accepted `gh run rerun --failed`; the new attempt started with
  `run_attempt` incremented (run `30744787367`, attempts 1–2). Cancelled runs are rerunnable.
- In a re-run-failed-jobs attempt, the re-executed job read the previous attempt's job outputs
  (`produce-output=[attempt-1-payload]`) and downloaded the previous attempt's artifact
  (`artifact-content=[artifact-from-attempt-1]`) (run `30744714726`, attempts 1–2). Prior attempt
  state carries over in this mode, although this remains undocumented platform behavior.
- A re-run-all-jobs attempt regenerated and replaced same-named artifacts
  (`artifact-content=[artifact-from-attempt-3]`, attempt 3). Re-run all is a state-replacing
  surface, not a recovery surface.
- `npm pack`, `pnpm pack`, and `yarn pack` on identical source produced byte-identical tarballs
  (sha512 `fc2a97fa…`, `e4efbaca…`, `c49cb0e7…` respectively) even with file mtimes set to different
  future dates and from different directories. Packed artifact digests are recomputable across
  attempts for identical content.

Read-back surfaces are documented elsewhere: GitHub Release assets expose an immutable SHA-256
`digest`, a failed upload may leave an empty `starter` asset that GitHub documents as safe to
delete, and npm packuments expose `dist.integrity`, with registry reads requiring polling because
npm offers no read-after-write consistency guarantee.

How should a repeated execution converge partial release state safely — who may adopt existing
remote state, under what binding proof, and through which retry surface — while a new run identity
keeps failing closed?

## Decision Drivers

- Never silently adopt remote state: adoption requires a binding proof, never existence alone.
- Keep "which run committed" unambiguous for verifiers: at most one run ever performs a given
  mutation; convergence is recognition, not mutation.
- Prefer loud failures over silent wrong successes; convergence logic must fail closed on any doubt.
- Make transient infrastructure faults recoverable through the trusted path instead of permanent
  version loss.
- Use the run identity the platform already signs (OIDC `run_id`/`run_attempt`, Fulcio certificate
  extensions, SLSA `invocationId`) as the idempotency key.
- Rely on undocumented platform behavior only as an optimization, never as a load-bearing
  assumption; every convergence path needs a documented fallback.
- Keep the producer-neutral publisher model: convergence rules must hold for future producer
  profiles.
- Preserve ADR 0066: the mutation segment is still serialized, and convergence never runs
  concurrently with another writer.

## Considered Options

- Strict fail-closed everywhere: every repeated execution fails on any pre-existing remote state;
  recovery is manual and unspecified.
- Full convergence: any run may adopt remote state when content matches, regardless of run identity.
- Run-identity-scoped convergence: retry attempts of the same `run_id` may converge through binding
  proof; a new `run_id` fails closed on pre-existing state.
- Step-differentiated convergence: byte-reproducible steps converge for any run, the manifest stays
  strict, regardless of run identity.
- Strict failure plus defined state reporting and a manual recovery contract, without any adoption.

## Decision Outcome

Chosen option: "Run-identity-scoped convergence", because the run identity is the idempotency key
the platform already signs, it makes transient faults recoverable without weakening the
single-writer invariant, and the spike verified that its load-bearing mechanics (cancelled-run
reruns, prior-attempt state in re-run-failed-jobs, recomputable pack digests) work in practice.

**Run identity is the idempotency key.** A workflow run (`run_id`) names one release intent. Retry
attempts of the same run (`run_attempt` increments) continue that intent and may converge. A new
`run_id` is a new release intent and must fail closed on any pre-existing remote state for the same
version or release assets — this ratifies the existing duplicate-fail clauses in the architecture
specifications for the cross-run case, and it answers the question ADR 0026 delegated: a rerun (same
`run_id`) is a continuation of the same release; a new run is a new release intent that may not
reuse version or asset state.

**Re-run failed jobs is the only supported retry surface.** It re-executes failed jobs and their
dependents while prior attempt state remains readable. Re-run all jobs is not a recovery path: it
replaces prior attempt state, and a workflow that encounters re-run-all must treat the attempt as a
fresh execution of every job, still bound by the same convergence rules per mutation step.
Cancellation does not end recoverability; cancelled runs accept re-run failed jobs.

**Outcome states.** Each mutation step classifies remote state before acting, and again after any
ambiguous mutating call, into exactly one of:

- `committed-as-expected` — remote state exists and matches this run's expected content by binding
  proof;
- `absent` — remote state does not exist;
- `foreign-conflict` — remote state exists and does not match the expected content;
- `indeterminate` — the state cannot be determined after documented polling bounds.

**Convergence rule.** Within a retry attempt of the same `run_id`, each mutation step must: perform
the mutation on `absent`; treat the step as satisfied on `committed-as-expected` and continue; fail
closed on `foreign-conflict` or `indeterminate`, naming the state and the conflicting remote
evidence. Existence alone is never sufficient: adoption requires equality of the expected digest or
integrity value with the authoritative remote value.

**Binding proof data flow.** A retry attempt determines expected values in this order:

1. prior-attempt job outputs and artifacts carried over by re-run failed jobs (spike-verified, but
   undocumented — used as the fast path only);
2. recomputation from source (packed artifact digests are byte-reproducible for identical content,
   spike-verified for npm, pnpm, and Yarn) combined with remote read-back;
3. if neither yields a binding value, fail closed as `indeterminate`.

**Step-specific bindings.**

- npm publish: expected `dist.integrity` (sha512 SRI) of the packed tarball against the published
  packument, with polling to absorb registry replication lag; the fallback is downloading
  `dist.tarball` and hashing locally.
- GitHub Release assets (primary asset and provenance sidecar): the expected SHA-256 digest against
  the asset `digest` field, polling until present; an empty `starter` asset left by this run's own
  failed upload may be deleted and the upload retried — this is the only permitted deletion,
  justified by GitHub's documentation that a starter asset is safe to delete. Anything that is not a
  `starter` asset produced by this run's own upload attempt must never be deleted, overwritten, or
  replaced.
- Release manifest and its bundle: manifest bytes are not reproducible across attempts because
  `generated_at` may change. Within the same `run_id`, if the manifest assets already exist, the
  step must compare them semantically — every field equal except `generated_at` — and adopt the
  existing assets on match, or fail closed as `foreign-conflict` on mismatch. No re-signing and no
  re-upload of same-named manifest assets ever occurs.

**Ambiguous mutating calls.** If a mutating call's outcome is unknown (for example a network failure
after the request was sent), the step must use the same read-back classification to resolve the
outcome before reporting failure. This generalizes the existing manifest upload states
(`failed-before-upload`, `partial-json-uploaded`, `indeterminate-json-upload`, `completed`) to every
mutation step.

**State reporting.** Every run must end — including on cancellation — with a machine-readable
outcome report produced by an `if: always()` reporting job, listing each mutation step's final
classification and evidence (digests, URLs, asset states). The report is diagnostic, not trusted
input for other runs.

**Orphaned attestations** from interrupted pre-mutation work remain inert: verification binds
attestations to published artifacts, never the reverse. No cleanup is required or performed.

**Deferred.** Cross-run recovery mediated by the registry (re-fetching a published tarball and
provenance to complete a release-asset-only retry) changes the same-run handoff trust boundary and
requires a future ADR. Specification updates must carve the same-`run_id` convergence rules into the
existing strict duplicate-fail clauses of `release-manifest.md` and
`github-release-asset-publisher.md`, define polling bounds, and add convergence fixtures.

### Consequences

- Good, because transient infrastructure faults between mutation steps become recoverable through
  the trusted path: a re-run converges instead of permanently losing the release-asset
  redistribution for an already-published version.
- Good, because the idempotency key is not a convention but a signed platform identity already
  present in OIDC claims, Fulcio certificate extensions, and SLSA `invocationId`.
- Good, because the single-writer invariant is preserved: convergence recognizes a commit; it never
  performs one, so "which run committed" stays singular and verifier-visible.
- Good, because the only permitted deletion is the narrow, platform-documented starter-asset case,
  keeping the no-overwrite/no-delete posture intact for all real content.
- Good, because existing strict duplicate-fail clauses are ratified for cross-run attempts rather
  than replaced, limiting specification churn to the same-run carve-out.
- Bad, because every mutation step gains a state-classification machine, polling bounds, and two
  binding data paths that must be specified, reviewed, and fixture-tested.
- Bad, because the fast binding path relies on undocumented carry-over of prior-attempt state; the
  design mitigates this with the recompute fallback, but the fast path could regress without notice.
- Neutral, because re-run all jobs is declared a non-recovery surface; callers must be told to use
  re-run failed jobs for recovery, which is a documentation and review-checklist obligation.

### Confirmation

This decision is confirmed when:

- architecture specifications define the four outcome states, per-step binding proofs, polling
  bounds, and the same-`run_id` carve-out in the duplicate-fail clauses, tracing to this ADR;
- workflow implementations classify remote state before and after every mutating call and fail
  closed on `foreign-conflict` and `indeterminate` with named evidence;
- starter-asset deletion is the only deletion implemented, guarded to assets created by the same
  run's own upload attempt in `starter` state;
- the manifest step implements semantic comparison ignoring `generated_at` within the same `run_id`,
  and strict duplicate-fail across runs;
- an `if: always()` reporting job emits the machine-readable outcome report on success, failure, and
  cancellation;
- fixtures demonstrate: publish-then-interrupt followed by a converging re-run failed jobs attempt;
  a new `run_id` failing on pre-existing state; a foreign digest producing `foreign-conflict`; an
  unreadable remote producing `indeterminate`; and a re-run-all attempt treated as full re-execution
  under the same rules;
- caller documentation states that re-run failed jobs is the recovery path and that a new run for an
  already-released version fails by design.

## Pros and Cons of the Options

### Strict fail-closed everywhere

- Good, because there is no adoption logic at all, so there is no silent-adoption failure mode, and
  the failure modes are all loud.
- Bad, because a single transient fault after npm publish permanently removes the trusted
  release-asset path for that version, forcing either a version bump or untrusted manual repair.
- Bad, because it ignores the idempotency key the platform already signs, paying the worst
  operational cost for the simplest rule.

### Full convergence for any run

- Good, because operationally any re-trigger heals any partial state when content matches.
- Bad, because it blurs "this run produced it" with "the intent is satisfied": the published
  provenance names the committing run while a later unrelated run reports success, weakening
  verifier reasoning about authority.
- Bad, because adoption by identity-less runs widens the silent-adoption surface for no security
  benefit.

### Run-identity-scoped convergence

- Good, because the idempotency key is the signed run identity itself: only the continuation of the
  same logical operation may recognize its own commits, and every other run fails loudly.
- Good, because spike evidence shows the required mechanics work: cancelled runs rerunnable,
  prior-attempt state readable in re-run-failed-jobs, pack digests recomputable.
- Bad, because it is the most machinery: outcome states, per-step bindings, polling, and a reporting
  job, all requiring fixtures.
- Bad, because the fast binding path uses undocumented platform behavior and must always degrade
  gracefully to recomputation.

### Step-differentiated convergence regardless of run identity

- Good, because it matches each step's reproducibility precisely and avoids trusting run identity.
- Bad, because it creates asymmetric rules that callers and verifiers must learn per step, and still
  allows identity-less adoption for the byte-reproducible steps, keeping part of the
  full-convergence weakness.

### Strict failure with defined reporting and a manual recovery contract

- Good, because it keeps failures loud while replacing improvised recovery with a documented one,
  and it needs the least new machinery.
- Bad, because it still cannot heal the bounded but real version-loss scenario through the trusted
  path, leaving the most painful failure class to humans.
- Bad, because a manual recovery contract on a trusted pipeline is an operational admission that the
  pipeline cannot recover itself, which this project can avoid.

## More Information

Spike evidence (12026-08-02, private repository `yunseo-kim/slsa-spike-tmp`, deleted after the
experiment): cancelled run `30744787367` accepted `gh run rerun --failed` and started attempt 2; run
`30744714726` attempt 2 (re-run failed jobs) logged `produce-output=[attempt-1-payload]` and
`artifact-content=[artifact-from-attempt-1]`; attempt 3 (re-run all jobs) logged
`artifact-content=[artifact-from-attempt-3]`, demonstrating state replacement; pack reproducibility
sha512 digests `fc2a97fa…` (npm), `e4efbaca…` (pnpm), `c49cb0e7…` (Yarn Berry) were identical across
file mtime and directory changes.

Platform documentation: re-run semantics and attempt behavior
(<https://docs.github.com/en/actions/how-tos/manage-workflow-runs/re-run-workflows-and-jobs>); OIDC
`run_id`/`run_attempt` claims (<https://docs.github.com/en/actions/reference/security/oidc>);
release asset `digest` field and starter asset deletion
(<https://docs.github.com/en/rest/releases/assets>,
<https://github.blog/changelog/2025-06-03-releases-now-expose-digests-for-release-assets/>); npm
`dist.integrity` and registry signatures (<https://docs.npmjs.com/cli/v11/commands/npm-publish/>,
<https://docs.npmjs.com/about-registry-signatures>); workflow cancellation guarantees the absence of
rollback for external side effects
(<https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation>).

Verifier identity: SLSA v1 `invocationId` uniqueness and the GitHub Actions build type's `run_id`
plus `run_attempt` recommendation (<https://slsa.dev/spec/v1.2/build-provenance>,
<https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/README.md>);
Fulcio run identity certificate extensions
(<https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md>).

This ADR is the recovery-semantics follow-up announced by ADR 0066. Together they define run
ownership in full: ADR 0066 excludes concurrent writers, and this ADR converges repeated executions
of the same writer while rejecting every other writer.
