---
parent: Decisions
nav_order: 80
status: accepted
date: 12026-08-13
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0068
    scope:
      "the Source content binding (Decision Outcome clause 4): certificate Source Repository Digest
      and Source Repository Ref equality with the expected release commit SHA and tag ref; the
      issuer, signer workflow, source repository identity including numeric IDs, run identity, and
      runner trust bindings remain in force"
  - type: see-also
    target: ADR-0079
---

# Bind Source Identity Policy to Signed Provenance Fields and Treat Certificate Source Claims as Invocation Context

## Context and Problem Statement

ADR 0079 decided that every producer profile supports a caller-specified, tags-only build source
ref, so a failed release can be retried with a fixed pipeline: the caller dispatches from a ref
carrying the fixed workflow (for example `main`), while the profile builds, attests, and publishes
the signed release tag's content. That decision explicitly deferred the binding-model question it
creates: when the built source ref differs from the invocation ref, what exactly do the producer
publish gate and consumer verifiers bind source identity to?

The pre-existing contract, decided by ADR 0068, requires the Fulcio signing certificate's Source
Repository Digest and Source Repository Ref extensions to equal the expected release commit SHA and
tag ref (Decision Outcome clause 4). Those certificate extensions map from the GitHub OIDC token's
`sha` and `ref` claims, which the platform issues for the run's invocation context and the signer
cannot override. For a tag-push release the invocation ref is the release tag, so the clause is
satisfiable; for an ADR 0079 dispatch retry it is structurally unsatisfiable — the certificate
provably records `refs/heads/main` (or whatever the dispatcher selected) while the release identity
is `refs/tags/vX.Y.Z`. Keeping clause 4 as written would reject every legitimate dispatch retry,
which is exactly the scenario ADR 0079 exists to enable.

At the same time, ADR 0068's binding had a real security function: it anchored the source identity
claim to platform-signed evidence rather than to builder-recorded fields, so a compromised or
defective builder could not silently attest a different source than the run actually started from.
Redefining the binding must preserve as much of that function as the platform allows, must keep the
SLSA v1 provenance model intact (externally controlled inputs are complete, signed
`externalParameters`; resolved immutable revisions are `resolvedDependencies`), and must remain
consistent with the ecosystem precedent: slsa-verifier's user-facing source expectations
(`--source-uri`, `--source-tag`, `--source-versioned-tag`) are matched against signed provenance
fields, while certificate claims serve as internal consistency evidence.

How should the producer publish gate and the consumer verifier policy bind source identity when the
built ref and the invocation ref can legitimately differ, so that verification remains
cryptographically anchored, fail-closed, and honest about which identity each piece of evidence
proves?

## Decision Drivers

- Respect platform facts: certificate source claims are platform-issued invocation-context values;
  no signer or workflow can make them describe the built ref when the two differ.
- Preserve a cryptographic anchor for every identity the verifier relies on: nothing security
  relevant may rest on unauthenticated caller input or on unverifiable builder assertion alone.
- Keep the built-source identity — the release tag and its resolved commit — as the value release
  policy actually cares about, consistent with ADR 0079's trust anchor and ADR 0068's immutable
  identity direction.
- Keep the invocation context verifiable and explicit: "the workflow logic came from ref X, the
  built content came from tag Y" must be checkable, not merely logged.
- Follow the SLSA v1 model: caller-supplied `source-ref` is an external parameter; the resolved
  commit is a resolved dependency; verifier policy binds to signed `externalParameters` and
  `buildType`.
- Stay within the ecosystem precedent (slsa-verifier matches source expectations against provenance
  fields) so downstream tooling concepts transfer.
- Change nothing when the capability is unused: an omitted `source-ref` must produce bundles and
  verification outcomes byte-compatible with the pre-existing contract.

## Considered Options

- Keep certificate source-claim equality with the signed built-source fields.
- Bind policy to the signed built-source fields, and bind certificate source claims to a signed
  invocation-context record.
- Bind policy to the signed built-source fields, and stop verifying certificate source claims.

## Decision Outcome

Chosen option: "Bind policy to the signed built-source fields, and bind certificate source claims to
a signed invocation-context record", because it is the only option that is both implementable under
platform-fixed OIDC claims and preservative of a cryptographic anchor for both identities — the
built release source and the invocation origin.

This is a foundation-level binding model: it lives in the common provenance and verification
contracts and applies to every profile, with the JS/TS npm package profile as the first adopter,
matching ADR 0079's scope.

The model has three parts:

1. **Built-source identity binds to signed provenance fields.** The producer publish gate and the
   consumer verifier policy compare their source expectations — canonical repository, release tag
   ref, and full commit SHA — against the signed `externalParameters.source.*` and
   `externalParameters.release.*` fields, not against certificate source extensions. The bundle
   signature is verified against the trusted, SHA-pinned builder identity (ADR 0068 clauses 1, 2,
   and 6 are unchanged), so these fields carry the trusted builder's attestation of what it
   resolved, checked out, built, and packed. The resolved commit SHA additionally appears in
   `resolvedDependencies`, keeping the immutable anchor explicit in the SLSA v1 model.
2. **Certificate source claims become invocation-context evidence, cryptographically bound to a
   signed invocation record.** The provenance carries a signed invocation-context record — at
   minimum the invocation ref and its commit SHA, with the exact field names and shape defined by
   the architecture specifications. The certificate's Source Repository Ref and Source Repository
   Digest extensions must equal that signed record, failing closed on any mismatch. The builder
   therefore cannot misreport the invocation context: a forged or misrecorded invocation ref or SHA
   breaks the certificate comparison. The Run Invocation URI binding (`metadata.invocationId`)
   continues to identify the producing run unchanged.
3. **Repository identity is context-independent and stays bound as before.** The Source Repository
   URI and the immutable numeric repository and owner identifiers describe the same caller
   repository in both the invocation and the built context, so ADR 0068 clause 3 applies unchanged
   and continues to compare certificate values against expected numeric IDs.

Under this model, each piece of evidence proves exactly what the platform allows it to prove: the
certificate proves where the run was triggered and which workflow code executed (signer workflow
identity plus invocation context); the signed predicate, backed by the SHA-pinned builder identity,
proves what source content was built. A verifier's release policy expresses expectations about the
release — repository, tag, commit — and binds them to the signed built-source fields.

Backward compatibility is structural: when `source-ref` is omitted, the invocation ref and the built
ref are the same value, the signed invocation record equals `source.ref`/`source.revision`, and
every comparison reduces to the pre-existing contract byte-for-byte.

The honest cost, stated plainly: the built-source identity is now builder-attested rather than
platform-claim-bound. It joins the same trust class as every other signed external parameter
(package directory, registry URL, distribution intent), whose honesty rests on the trusted,
SHA-pinned builder code and its conformance fixtures. The platform continues to anchor the builder's
identity, the caller repository, the invocation origin, and the run identity; it no longer anchors
the built commit, because it physically cannot when the two contexts differ.

Exact field names, certificate extension OIDs, policy schema rows, and diagnostic IDs — including
the redefinition of the existing source digest and ref mismatch diagnostics and any new diagnostics
for built-source policy mismatches — belong to the architecture specifications.

### Consequences

- Good, because the ADR 0079 retry scenario becomes verifiable end-to-end: a dispatch from the
  caller's default branch building a signed release tag produces a bundle that passes the producer
  gate and consumer verification honestly, with both identities on record.
- Good, because no security-relevant value rests on unauthenticated input: the built-source identity
  is signed by the trusted builder, and the invocation record is cross-checked against
  platform-issued certificate claims.
- Good, because the model matches the ecosystem precedent: slsa-verifier already binds user source
  expectations to provenance fields rather than directly to certificate OIDs, so verifier concepts
  and documentation transfer without invention.
- Good, because the SLSA v1 external-parameter completeness story stays coherent: `source-ref` joins
  the signed `externalParameters`, and the resolved commit joins `resolvedDependencies`, instead of
  creating a shadow channel outside the signed contract.
- Good, because omitted-input flows are byte-compatible with the pre-existing bundles and
  verification outcomes, so existing callers and fixtures are unaffected.
- Neutral, because the verification policy and fixtures specification, the common provenance
  contract's signer-identity section, and the npm provenance and publish specification's signer
  identity table must be rewritten, and the producer publish gate and verifier code must be updated
  to the new comparison targets.
- Neutral, because diagnostic semantics for source digest and ref mismatches move from "certificate
  claim versus expected release identity" to "certificate claim versus signed invocation record";
  the specifications must register the redefined and new diagnostic IDs explicitly.
- Bad, because the platform no longer independently anchors the built commit for dispatch retries: a
  compromise of the trusted builder itself could attest a different built ref — a residual risk
  inherent to the SLSA external-parameter model, mitigated by the SHA-pinned builder identity, the
  signed release manifest binding builder code to releases, and conformance fixtures.
- Bad, because verifier policy authors must now understand two source identities (built and
  invocation) instead of one; documentation must state precisely which field each expectation binds.

### Confirmation

This decision is confirmed when:

- the verification policy and fixtures specification binds source expectations (repository, tag ref,
  commit SHA) to the signed provenance fields, and binds certificate Source Repository Ref and
  Digest extensions to the signed invocation-context record, with exact OIDs, JSON field paths, and
  failure behavior, tracing to this ADR;
- the common SLSA provenance v1 contract's signer-identity section and each adopting profile's
  signer-identity table (npm first) define the built-source rows against signed fields and the
  invocation rows against certificate claims;
- the producer publish gate enforces every binding fail-closed before any registry or release
  mutation;
- fixtures demonstrate: acceptance of a tag-push bundle where invocation equals built (byte
  compatibility with the pre-existing contract); acceptance of a dispatch-retry bundle where the
  invocation record and the built source differ legitimately; rejection when the certificate
  invocation claims disagree with the signed invocation record; rejection when policy source
  expectations disagree with the signed built-source fields;
- verifier documentation teaches the two-identity model explicitly, including which field each
  policy expectation binds and why the certificate cannot describe the built ref on a dispatch
  retry;
- dogfood evidence from `vers-js`: the ADR 0079 retry run for `v0.1.2` passes the producer gate and
  the documented consumer verification procedure under the new binding model.

## Pros and Cons of the Options

### Keep certificate source-claim equality with the signed built-source fields

- Good, because it changes nothing in the verification policy, the gate, or the fixtures.
- Bad, because it is structurally impossible once ADR 0079 exists: the platform fixes the
  certificate's source claims to the invocation context, so a dispatch retry can never satisfy the
  equality. Retaining the clause would silently veto the decided capability.
- Bad, because the only way to keep it satisfiable is to never let the two contexts differ, which is
  precisely the pre-ADR-0079 contract already judged unacceptable.

### Bind policy to the signed built-source fields, and bind certificate source claims to a signed invocation-context record

- Good, because it is implementable: every comparison targets a value the platform or the trusted
  builder can actually produce.
- Good, because both identities stay cryptographically anchored — the built source through the
  trusted builder's signature, the invocation origin through the platform-issued certificate — and
  the divergence between them is explicit and auditable rather than hidden.
- Good, because it matches slsa-verifier's established split between user-facing provenance-field
  expectations and internal certificate consistency checks.
- Neutral, because it requires rewriting the verification policy specification, the signer-identity
  sections, the gate, the verifier code, and their fixtures.
- Bad, because the built commit loses its platform-issued anchor in the dispatch-retry case and
  relies on the trusted builder's honest recording, as do all external parameters.

### Bind policy to the signed built-source fields, and stop verifying certificate source claims

- Good, because the verifier logic is simplest and the specification change is smallest.
- Bad, because it discards the platform's attestation of the invocation origin: nothing would detect
  a builder that misreports which ref triggered the run, weakening incident forensics and the "logic
  from X, content from Y" audit story that ADR 0079 requires to stay explicit.
- Bad, because it removes a check that costs little and catches real defects (misconfigured
  dispatch, wrong checkout wiring), converting a fail-closed surface into silent acceptance.

## More Information

This decision completes the two-ADR split that began with ADR 0079: ADR 0079 decided that producer
profiles support a tags-only caller-specified build source ref; this ADR decides how verification
binds identity when the built and invocation contexts differ. It partially supersedes ADR 0068
clause 4 only; ADR 0068's issuer, signer workflow, repository identity (including numeric IDs), run
identity, and runner trust bindings remain in force, as does its prohibition on using
`github.workflow_sha` as builder identity.

Reference points considered:

- [Fulcio OID information](https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md): Source
  Repository Digest (`.13`) and Source Repository Ref (`.14`) map from the platform-issued `sha` and
  `ref` claims; Build Signer URI/Digest map from `job_workflow_ref`/`job_workflow_sha`.
- [GitHub OIDC with reusable workflows](https://docs.github.com/en/actions/how-tos/secure-your-work/security-harden-deployments/oidc-with-reusable-workflows):
  standard claims describe the caller invocation context; only `job_workflow_ref` and
  `job_workflow_sha` describe the called reusable workflow.
- [slsa-verifier provenance verification](https://github.com/slsa-framework/slsa-verifier/blob/30d0be3bbab553fc51557377baba2f7572dfc212/verifiers/internal/gha/provenance.go):
  user-facing `--source-uri`, `--source-branch`, `--source-tag`, and `--source-versioned-tag`
  expectations are matched against provenance fields, with certificate claims used for internal
  consistency.
- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance) and
  [Build Requirements](https://slsa.dev/spec/v1.2/build-requirements): `externalParameters` are the
  externally controlled interface and must be complete at Build L3; resolved immutable revisions
  belong in `resolvedDependencies`.
