---
parent: Decisions
nav_order: 76
status: accepted
date: 12026-08-04
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0024
    scope:
      "the pre-publication authorization guarantee: trusted-publishing misconfiguration is detected
      before registry mutation by an early OIDC token exchange preflight; residual publish-time
      authorization failures are classified, and the absolute must-stop-before-registry-mutation
      wording is scoped to what the exchange and observation preflights can establish"
  - type: amends
    target: ADR-0058
    scope:
      "the runtime permission-verification layer: no side-effect-free write-capability probe exists,
      so the first mutating call is the runtime authority check, with HTTP 403 as the
      permission-failure signal and ADR 0067 read-back on ambiguity"
  - type: see-also
    target: ADR-0066
  - type: see-also
    target: ADR-0067
---

# Use Observation Preflights and First-Mutation Classification

## Context and Problem Statement

The specifications ask for pre-mutation guarantees the platforms cannot always provide. The release
asset publisher validates runtime authority by "probing actual API behavior for the selected path"
before upload, and the npm package profile requires that a trusted-publishing configuration failure
"must stop before registry mutation" and that the caller workflow filename be observed "from the
trusted publishing authorization context". A pre-implementation review flagged all three as
unverifiable as written: no observation interface was pinned, and no probe was known to exist.

Spike verification (12026-08-04, `yunseo-kim/slsa-spike-tmp`@`50b7cbe`, run 30935100751) settled the
GitHub side empirically. The repository `permissions` field returns
`{"admin":false,"maintain":false,"pull":false,"push":false,"triage":false}` identically under
`contents: write` and `contents: read` job permissions: it does not reflect the job-effective
fine-grained `GITHUB_TOKEN` permissions, and no other GitHub API exposes them. There is no
side-effect-free write-capability probe. The same spike confirmed the asymmetry that makes a tiered
model possible: without `id-token: write`, the `ACTIONS_ID_TOKEN_REQUEST_TOKEN` environment variable
is simply absent, so OIDC capability IS probe-able without side effects, and a requested token
carries the `workflow_ref` (caller workflow) and `job_workflow_ref` claims.

The npm side is settled by documentation and source. The OIDC token exchange
(`POST /-/npm/v1/oidc/token/exchange/package/{pkg}`) validates the caller's OIDC token against the
package's trusted publisher configuration and is not itself a registry mutation — it mints a
short-lived publish token (documented TTL "typically 1 hour") and nothing else. Exchange failures
surface as 401/404/500 and mutate nothing; the npm CLI itself performs the exchange immediately
before publish and swallows exchange failure into a fallback path, so an early exchange attempt is
safe. npm validates the CALLING workflow for reusable-workflow publishing (caller-based matching,
the `workflow_ref` side), which is exactly the claim the observation preflight can read. The
ecosystem confirms both halves of the pattern: semantic-release/npm v13.1.0 performs an explicit
OIDC exchange check before publish, while release-please, changesets, GoReleaser, and
slsa-github-generator declare required permissions and simply fail on the first write; a true
write-capability probe exists only where the underlying protocol offers one (`git push --dry-run`),
which GitHub's REST API does not. No release tool performs read-back after ambiguous writes — ADR
0067's classification machine already exceeds ecosystem practice there.

How should the profiles handle authorization and capability requirements that cannot be verified
side-effect-free before mutation — without claiming guarantees the platforms cannot support, and
without manufacturing ambiguous remote state while probing?

## Decision Drivers

- Never create an ambiguous commit by policy (ADR 0066's driver): a probe that itself mutates remote
  state is a defect, not a verification step.
- Fail closed with classified outcomes: every authorization failure gets a named diagnostic; none is
  relayed raw from a platform whose messages mislead.
- Use the strongest side-effect-free observation each surface actually offers; claim no guarantee
  beyond it.
- Keep the specifications implementable: a requirement no implementation can satisfy is a
  conformance defect, not a safety feature.
- Prefer documented or vendor-tooling-backed surfaces; where behavior is undocumented, spike-verify
  it and watch upstream (the ADR 0067 precedent).
- Leave the static YAML conformance layer and the ADR 0066/0067 machinery unchanged.

## Considered Options

- Keep the absolute pre-mutation guarantee wording (status quo).
- Observation preflights plus first-mutation classification: a two-tier model per surface.
- Classification only: delete all preflight language and classify the first mutation's failure.
- Probe emulation: perform harmless-looking dummy writes to test capability before the real write.

## Decision Outcome

Chosen option: "Observation preflights plus first-mutation classification", because it claims
exactly the guarantee each platform can support and no more, catches the dominant misconfiguration
class before any mutation where that is physically possible, and never mutates remote state in the
name of verification.

**The doctrine is two-tier.** Tier 1 — observation preflights — uses every side-effect-free check
the surface offers, as early as possible: the static YAML conformance layer (unchanged), OIDC
capability via `ACTIONS_ID_TOKEN_REQUEST_TOKEN` presence and token request, and the npm early
exchange below. Tier 2 — first-mutation classification — applies where no observation exists: the
first mutating call IS the runtime authority check. An HTTP 403 (or 401) is the permission-failure
signal and fails the run without further mutation; other API or transport failures keep their own
categories; if a mutating request may already have been submitted when the result becomes ambiguous,
ADR 0067 read-back classifies the outcome and the run fails as `indeterminate` unless that
classification proves another outcome.

**Application 1 — release asset publisher (amends ADR 0058).** The "probing actual API behavior"
wording is replaced by the tier-2 rule above: no separate probe exists, and the specification must
not imply one. The static YAML conformance layer is untouched. The spike-documented fact that the
repository `permissions` field does not reflect job-effective token permissions is recorded so the
probe is not reintroduced later.

**Application 2 — npm trusted publishing (amends ADR 0024).** The publish job performs the OIDC
token exchange EARLY, before signing and before any publish attempt. Exchange failure
(authentication or configuration mismatch, observed as 401/404) fails the run before registry
mutation with `windlass.verify.error.trusted-publisher-mismatch` or a narrower diagnostic; an
unreadable or erroring exchange surface (5xx, malformed response) fails as `indeterminate`. Exchange
success validates the trusted publisher configuration at exchange time; the exchanged token's
documented TTL (typically 1 hour) covers the preflight-to-publish gap by a wide margin. Residual
publish-time authorization failures (npm's E404/ENEEDAUTH, whose diagnostics are documented to
mislead) are mapped into this project's own taxonomy rather than relayed. The ADR 0024 prohibition
of token fallback (`NPM_TOKEN`, `NODE_AUTH_TOKEN`, OTP) is unchanged. A `npm trust list`
configuration read requires a registry token and is therefore only meaningful AFTER a successful
exchange; it is demoted to an optional configuration-observation step, never the preflight. The
absolute "must stop before registry mutation" wording is thereby scoped: for trusted-publishing
misconfiguration the guarantee is now genuinely achieved by the early exchange, and for everything
else the classified first mutation is the honest bound.

**Application 3 — caller workflow filename observation (amends ADR 0024).** The observation
interface is pinned: the profile reads the OIDC token's `workflow_ref` claim — the caller workflow
path, whose final path segment is the filename npm validates — and cross-checks it against the
caller-scoped `github.workflow_ref` context. Caller-supplied filename inputs remain forbidden.
Missing or conflicting capture fails before signing and registry mutation with
`windlass.verify.error.trusted-publisher-mismatch`. Because npm's reusable-workflow validation is
caller-based, the `workflow_ref` claim is the same value npm checks; `job_workflow_ref` identifies
the reusable callee and must not be used for this match.

### Consequences

- Good, because every pre-mutation guarantee in the specifications is now achievable: the npm
  trusted-publishing misconfiguration guarantee is genuinely delivered by the early exchange, and
  the GitHub permission guarantee is honestly restated as first-mutation classification.
- Good, because the dominant caller misconfiguration — wrong repository or workflow filename in the
  trusted publisher settings — is caught before any mutation, with a diagnostic written for this
  project rather than npm's misleading E404/ENEEDAUTH.
- Good, because the model matches verified ecosystem practice on both halves: semantic-release/npm
  for the early exchange, and first-write classification for everything else — while the ADR 0067
  read-back keeps this project ahead of ecosystem practice on ambiguity.
- Good, because no probe mutates remote state, so ADR 0066's no-ambiguous-commit-by-policy driver is
  preserved intact.
- Bad, because the early exchange as a standalone preflight is not a documented npm pattern: it is
  safe by construction (no registry mutation, fresh token per call) but must be watched against
  upstream npm changes, as with the ADR 0067 undocumented-behavior precedent.
- Bad, because exchange success validates configuration at exchange time, not at publish time; the
  residual window is bounded by fail-closed classification, not eliminated.
- Bad, because the specifications gain a tiered guarantee vocabulary that must be applied
  consistently: which guarantees are observational and which are first-mutation must be stated per
  surface.
- Neutral, because the platform scope decision (ADR 0075, github.com only) already covers every
  surface this decision touches.

### Confirmation

This decision is confirmed when:

- the publisher specification replaces the probe wording with the first-mutation authority rule and
  records the permissions-field finding, and its fixtures cover 403-on-first-mutation classification
  and ambiguous-write read-back;
- the npm profile specification defines the early exchange preflight (placement, failure
  classifications, TTL margin, residual E404/ENEEDAUTH mapping, no token fallback) and the
  `workflow_ref`-claim filename observation with the `github.workflow_ref` cross-check;
- the shared diagnostic taxonomy registers the preflight diagnostics used above, and fixtures cover
  the exchange-preflight failure matrix (401/404 → `trusted-publisher-mismatch`; 5xx/unreadable →
  `indeterminate`), the id-token env-absence capability probe, and the filename cross-check mismatch
  case;
- caller documentation states the tiered guarantee: which misconfigurations fail before any mutation
  and which are classified at the first mutation;
- the first dogfood publish empirically exercises the early-exchange preflight and its result is
  recorded against this decision.

## Pros and Cons of the Options

### Keep the absolute pre-mutation guarantee wording (status quo)

- Good, because it changes nothing and asks nothing of the platforms.
- Bad, because it is unachievable: the spike proves no GitHub write-capability probe exists, so a
  conforming implementation is impossible and every real implementation either violates the
  specification or pretends — a conformance lie at the foundation of a spec-driven project.
- Bad, because it wastes the observations that DO exist: the OIDC capability probe and the early
  exchange can deliver real pre-mutation failure for the dominant misconfiguration class.

### Observation preflights plus first-mutation classification (chosen)

- Good, because it claims exactly what each platform supports: absolute pre-mutation failure where
  an observation exists (npm trusted-publishing configuration, OIDC capability), classified
  first-mutation failure where none does (GitHub `contents: write`).
- Good, because it has verified precedent on both halves (semantic-release/npm's early exchange;
  release-please/changesets/GoReleaser/slsa-github-generator's first-write classification).
- Good, because it preserves ADR 0066's no-ambiguous-commit driver and reuses ADR 0067's
  classification machine unchanged.
- Bad, because the early exchange is an undocumented standalone pattern requiring upstream watch,
  and the tiered vocabulary adds specification surface that must be kept consistent.
- Bad, because exchange-time validation leaves a bounded residual window before publish, closed by
  classification rather than eliminated.

### Classification only (remove all preflights)

- Good, because it is the simplest honest model and matches most of the release-tooling ecosystem.
- Bad, because it discards achievable pre-mutation failure: the early exchange catches trusted
  publisher misconfiguration before any registry mutation, and the OIDC capability probe catches
  permission misconfiguration before any job does real work — both for free.
- Bad, because it inherits npm's misleading E404/ENEEDAUTH diagnostics as the first and only signal
  for the most common caller error.

### Probe emulation via dummy writes

- Good, because it would produce a true pre-mutation capability answer on surfaces without read
  probes.
- Bad, because a dummy write IS a mutation: it dirties remote state, races the concurrency model,
  and can itself end ambiguously — manufacturing exactly the states ADR 0066 and ADR 0067 exist to
  contain. The probe becomes a contamination source.

## More Information

Spike verification (12026-08-04): repository `yunseo-kim/slsa-spike-tmp` at commit `50b7cbe`, run
30935100751 — repository `permissions` field all-false under both `contents: write` and
`contents: read`; `ACTIONS_ID_TOKEN_REQUEST_TOKEN` absent without `id-token: write`; OIDC token
carries `workflow_ref` and `job_workflow_ref`.

npm platform references: OIDC exchange endpoint and token TTL (<https://api-docs.npmjs.com/>); npm
CLI exchange and publish-call sites
(<https://github.com/npm/cli/blob/4cdcceac047f82571d0ec734e18b87d1d130e042/lib/utils/oidc.js#L117-L142>,
<https://github.com/npm/cli/blob/4cdcceac047f82571d0ec734e18b87d1d130e042/lib/commands/publish.js#L150-L209>);
trusted publishing caller-based validation for reusable workflows
(<https://docs.npmjs.com/trusted-publishers/>, <https://github.com/npm/documentation/issues/1755>);
staged publishing is itself a registry write (<https://docs.npmjs.com/staged-publishing/>);
misleading publish-time diagnostics (<https://github.com/npm/cli/issues/9088>,
<https://github.com/npm/documentation/issues/1960>, <https://github.com/npm/cli/issues/8730>,
<https://github.com/npm/cli/issues/8976>).

Ecosystem precedent: semantic-release/npm explicit OIDC exchange verification before publish
(<https://github.com/semantic-release/npm/commit/e3319f1b2cb07eef8f61f9fa613552fa33bc92ae>,
<https://github.com/semantic-release/npm/commit/c80ecb0404f44fa60c5d9edb1d3424adf8a336f0>,
<https://github.com/semantic-release/npm/releases/tag/v13.1.0>); semantic-release/github read-only
verification without a write-capability probe
(<https://github.com/semantic-release/github/blob/039dc5839a891f05ed89b4a8c4ef8a06cbd51cc4/lib/verify.js#L81-L152>);
`git push --dry-run` as the protocol-supported probe shape
(<https://github.com/semantic-release/semantic-release/blob/d27db1b86fb5938ca8415e155d40a9616828c069/lib/git.js#L200-L215>);
first-write classification in release-please
(<https://github.com/googleapis/release-please/blob/fdaca293b5c9215c7bcae72dd228ba9558120817/src/github-api.ts#L676-L709>)
and changesets
(<https://github.com/changesets/action/blob/cae63535198362487bf613d7940dbc74b12d7313/src/run.ts#L158-L245>);
the community confirmation that token permissions cannot be inspected
(<https://github.com/orgs/community/discussions/73397>).
