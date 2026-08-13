---
parent: Decisions
nav_order: 68
status: accepted
date: 12026-08-02
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0037
    scope:
      "the identity binding depth of the documented verifier policy and the producer publish gate;
      ADR 0068 pins the required bindings"
  - type: see-also
    target: ADR-0028
  - type: see-also
    target: ADR-0029
  - type: see-also
    target: ADR-0055
  - type: see-also
    target: ADR-0062
  - type: see-also
    target: ADR-0079
  - type: see-also
    target: ADR-0069
  - type: see-also
    target: ADR-0073
  - type: partially-superseded-by
    target: ADR-0080
    scope:
      "the Source content binding (Decision Outcome clause 4): certificate Source Repository Digest
      and Source Repository Ref equality with the expected release commit SHA and tag ref; the
      issuer, signer workflow, source repository identity including numeric IDs, run identity, and
      runner trust bindings remain in force"
---

# Bind Verification to Immutable Builder and Source Identities

## Context and Problem Statement

ADR 0037 established the verification deliverables: a producer-side publish gate, a documented
verifier policy, conformance fixtures, and reference commands. It directed the verifier policy to
describe roots of trust and identity bindings "in terms of" the Sigstore root, the GitHub Actions
reusable workflow signer identity, the SHA-pinned Windlass `builder.id`, and related expectations,
but it did not pin the depth of that binding. Two verifiers can both comply with ADR 0037 and yet
disagree about security: one checking only that some GitHub Actions workflow signed the artifact,
another checking that the exact Windlass workflow at the exact commit SHA signed it from the exact
source repository.

Names are weak identifiers. GitHub itself warns that name-based OIDC subjects can collide when a
namespace is recycled, and GitHub is migrating to an immutable subject format built from numeric
owner and repository IDs (rollout from 12026-07-15 for new, renamed, and transferred repositories).
OpenSSF Trusted Publishers guidance likewise states that name-only trust is too weak and recommends
resolving and persisting numeric IDs. A policy that binds verification to repository or owner names
alone inherits rename, transfer, and recreation risk for both the Windlass builder identity and the
consumer source identity.

The platform already provides stronger material, verified by the 12026-08-02 spike (private
repository `yunseo-kim/slsa-spike-tmp`): the OIDC token issued to a job contains `job_workflow_sha`
— the resolved full commit SHA of the called reusable workflow, present even when the caller pins by
tag — plus numeric `repository_id` and `repository_owner_id`. The same spike confirmed that the
`github` context does not expose the called workflow's SHA: `github.workflow_sha` is the caller
workflow's SHA and must never be used as builder identity. Fulcio maps these claims into signing
certificate extensions: Build Signer URI and Digest (`job_workflow_ref`, `job_workflow_sha`), Source
Repository URI, Digest, and Ref, the immutable Source Repository Identifier and Source Repository
Owner Identifier, the Run Invocation URI (run ID plus attempt), and the deployment environment. The
SAN URI carries the called workflow's ref.

What exactly must the producer publish gate and consumer verifiers bind a signature to, so that "the
trusted builder produced this artifact" is checkable against immutable identity rather than mutable
names?

Transparency log, timestamping, and trust root distribution mechanics — what proof of signing time a
verifier must require and how trust material is obtained — are deliberately out of scope here and
are decided by the follow-up ADR 0069.

## Decision Drivers

- Bind to immutable identity: verification must survive repository rename, transfer, and recreation
  for both builder and source identities.
- Bind the builder to exact code: the signer must be the exact Windlass profile workflow at the
  exact commit SHA recorded in the signed release manifest.
- Fail closed: any missing, mismatched, or malformed identity binding is a verification failure,
  never a warning.
- Use platform-signed claims only: every required value must come from the GitHub OIDC token or the
  Fulcio certificate, never from caller-controlled workflow context or inputs.
- Stay implementable today: the policy must be expressible with `gh attestation verify` plus
  machine-readable post-processing now, and with `sigstore-go` when the trusted Go core lands.
- Align with where GitHub and OpenSSF are going (immutable subjects, numeric IDs) rather than
  freezing a name-based policy that must later be reversed.

## Considered Options

- Registry and tool defaults: document `gh attestation verify` and `npm audit signatures` defaults
  as the verifier policy.
- Standard binding: exact issuer, signer workflow ref, signer workflow SHA, source repository name,
  ref, and SHA, GitHub-hosted runners only.
- Maximal binding: standard binding plus immutable numeric source repository and owner IDs and the
  Run Invocation URI, with expectations carried by the explicit verifier policy.

## Decision Outcome

Chosen option: "Maximal binding", because the immutable identifiers exist in every GitHub
Actions-issued certificate, GitHub and OpenSSF both treat name-only binding as insufficient, and the
one-time cost of custom identity checks buys permanent rename and recreation resistance.

The producer publish gate and the documented consumer verifier policy must both enforce the
following bindings, failing closed on any mismatch, absence, or malformed value:

1. **Issuer**: exactly `https://token.actions.githubusercontent.com`.
2. **Signer workflow**: the certificate SAN URI and Build Signer URI identify the exact Windlass
   profile workflow path, and the Build Signer Digest (`job_workflow_sha`) equals the full commit
   SHA recorded for that profile in the signed release manifest for the release being verified.
   `job_workflow_sha` is the only authoritative source of the called workflow's SHA; the `github`
   context `workflow_sha` value is the caller workflow's SHA and must not be used as builder
   identity (spike-verified).
3. **Source repository**: Source Repository URI equals the expected consumer repository, and the
   immutable Source Repository Identifier (`repository_id`) and Source Repository Owner Identifier
   (`repository_owner_id`) equal the expected numeric IDs. Names may be displayed but IDs decide.
4. **Source content**: Source Repository Digest and Source Repository Ref equal the expected commit
   SHA and tag ref for the release.
5. **Run identity**: the Run Invocation URI is present and well-formed, carrying the producing run's
   ID and attempt; it is recorded in diagnostics for traceability.
6. **Runner trust**: only GitHub-hosted runner identities are accepted; self-hosted runner
   provenance is rejected in the production path.

The explicit verifier policy, or another independently configured verifier-expectation source,
carries the expected values, including the numeric IDs, so that ADR 0062's policy intersection
operates over immutable keys rather than names. The signed release manifest carries no
caller-specific source identity: ADR 0062 closes its schema version 1, and the manifest signer has
no authority to decide which caller repository is canonical for a downstream package. A caller
learns its own repository and owner IDs through the GitHub API and supplies them as policy input;
the architecture specifications define the exact policy schema fields.

Because every identity value comes from the GitHub OIDC token or the Fulcio certificate, no binding
depends on caller-controlled context, job outputs, or workflow inputs.

The producer publish gate enforces this maximal policy itself before any registry mutation,
ratifying ADR 0037's split: the gate fails closed so that a published artifact always carries
maximally bound evidence, and consumers verify independently using the documented policy and
reference commands. `gh attestation verify` covers issuer, signer workflow, and signer digest checks
today; the numeric ID and Run Invocation URI checks require machine-readable post-processing of
verified bundle output (or `sigstore-go` in the future trusted core), and the specifications must
provide both the exact check procedure and negative fixtures.

Deferred: transparency log inclusion, timestamping, and trust root distribution are decided by
ADR 0069. Exact certificate extension OIDs, JSON field paths, command sequences, and policy schema
fields belong to the architecture specifications.

### Consequences

- Good, because verification survives repository rename, transfer, and recreation on both the
  builder side and the consumer source side, matching GitHub's own immutable-subject direction and
  OpenSSF guidance.
- Good, because the builder is bound to exact code: `job_workflow_sha` equals the release manifest's
  recorded workflow SHA, so a different workflow file, a different path, or different builder code
  fails verification even when everything else matches.
- Good, because the spike proved every required claim exists and is platform-signed: no part of the
  policy rests on caller-supplied values.
- Good, because the policy intersection of ADR 0062 now operates over immutable keys, removing an
  entire ambiguity class (name change between manifest issuance and verification).
- Bad, because off-the-shelf commands do not cover the numeric ID checks; the project must document
  and fixture-test custom post-processing until the Go verifier exists.
- Bad, because verifier policy schemas grow numeric ID fields, and callers must learn and supply
  their repository and owner IDs — a documentation and validation cost.
- Neutral, because `gh attestation verify` remains useful but insufficient alone; reference commands
  must show the complete procedure rather than implying one command is enough.

### Confirmation

This decision is confirmed when:

- architecture specifications define the required bindings, the exact certificate extension OIDs and
  claim sources, the expected-value schema fields for the explicit verifier policy, and the failure
  behavior for each binding, tracing to this ADR;
- the producer publish gate implements every binding fail-closed before registry mutation;
- verification documentation provides the complete procedure, including the numeric ID checks, and
  explicitly forbids `github.workflow_sha` as builder identity;
- fixtures demonstrate acceptance of a fully bound bundle and rejection for: wrong issuer, wrong
  signer workflow path, wrong signer SHA, name-matching but ID-mismatched source repository, missing
  numeric IDs, missing or malformed Run Invocation URI, and self-hosted runner identity;
- verifier policy examples show numeric repository and owner IDs as expectations.

## Pros and Cons of the Options

### Registry and tool defaults

- Good, because there is nothing custom to build, document, or fixture-test, and the tools already
  exist.
- Bad, because the defaults are name-based and skip the signer workflow SHA, leaving rename and
  recreation windows open and allowing any code behind a reused name to pass — exactly what OpenSSF
  warns against.
- Bad, because the verifier policy would differ per tool and per default change, so the project
  would not actually control its own trust contract.

### Standard binding (names plus SHA)

- Good, because it pins the builder to exact code and is expressible entirely with existing
  `gh attestation verify` flags.
- Bad, because repository and owner binding stays name-based: a recreated or transferred namespace
  with the same name passes, and there is no run-instance evidence at all.
- Bad, because it deliberately ignores immutable identifiers that are always present in the
  certificates being verified, buying a known weakness to save a one-time documentation cost.

### Maximal binding

- Good, because it binds to identities that cannot be recycled: numeric IDs for repository and
  owner, exact SHA for builder code, and run-instance evidence for traceability.
- Good, because every required value is platform-signed and always present in GitHub Actions-issued
  certificates, so the policy is enforceable today without waiting for any platform change.
- Bad, because no single off-the-shelf command performs all checks, so the project owns custom
  post-processing, fixtures, and teaching callers to supply numeric IDs.
- Bad, because the policy schemas and documentation grow, and a stricter policy always carries a
  small risk of rejecting an otherwise legitimate artifact when IDs are recorded incorrectly —
  mitigated by fail-closed diagnostics that name the mismatched expectation.

## More Information

Spike evidence (12026-08-02, private repository `yunseo-kim/slsa-spike-tmp`, deleted after the
experiment): in runs `30745570800` (caller pinned by SHA) and `30745572730` (caller pinned by tag),
the OIDC token's `job_workflow_sha` was the called workflow's resolved full commit SHA in both
cases, while `github.workflow_sha` and the `workflow_sha` claim carried the caller workflow's SHA;
`repository_id` and `repository_owner_id` were present in the token.

Identity surfaces: GitHub Actions OIDC claims and the issuer
`https://token.actions.githubusercontent.com`
(<https://docs.github.com/en/actions/reference/security/oidc>); Fulcio certificate extension mapping
including Build Signer URI/Digest, Source Repository URI/Digest/Ref/Identifier, Source Repository
Owner Identifier, Run Invocation URI, and Deployment Environment
(<https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md>); GitHub's immutable OIDC subject
format and namespace-recycling warning
(<https://docs.github.com/en/actions/reference/security/oidc>).

Ecosystem guidance: OpenSSF Trusted Publishers on the insufficiency of name-only trust and on
persisting numeric IDs
(<https://repos.openssf.org/trusted-publishers-for-all-package-repositories.html>); PyPI trusted
publishing security model (<https://docs.pypi.org/trusted-publishers/security-model/>);
`gh attestation verify` policy flags and their limits
(<https://cli.github.com/manual/gh_attestation_verify>); GitHub guidance that the reusable workflow
is the signer to validate
(<https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/increase-security-rating>);
npm provenance verification checks (<https://github.com/npm/provenance/blob/main/README.md>).

This ADR pins the identity half of the verifier trust policy left open by ADR 0037; ADR 0069 pins
the transparency and trust root half. Together they complete the verifier policy that ADR 0037
deferred to specifications.
