---
parent: Decisions
nav_order: 69
status: accepted
date: 12026-08-02
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0037
    scope:
      "the transparency log, timestamping, and trust root requirements of the documented verifier
      policy and the producer publish gate"
  - type: see-also
    target: ADR-0035
  - type: see-also
    target: ADR-0055
  - type: see-also
    target: ADR-0068
---

# Require Rekor Transparency and Govern the Sigstore Trust Root

## Context and Problem Statement

ADR 0068 pinned who verifiers must bind signatures to. This decision pins the complementary question
ADR 0037 left open: what proof of signing time and public witness a verifier must require, and how
it obtains the trust material to check any of it. Fulcio certificates are short-lived (minutes), so
a signature is only meaningful with evidence that it was created while the certificate was valid,
and every verification ultimately rests on trust material — the Fulcio CA chain, Rekor log keys, and
timestamping keys — whose provenance and freshness must themselves be governed.

Three bundle classes are in scope, all produced today by `actions/attest` against the Sigstore
public good instance: npm package provenance, release asset provenance sidecars, and the signed
release manifest. For all three, the verification options are real and divergent. sigstore-go
verifies bundles offline by construction and never queries Rekor; cosign's legacy `--offline` flag
is deprecated; PyPI requires Rekor and Fulcio CT inclusion proofs for verified attestations; GitHub
documents fully offline verification from a bundle plus a trusted root; SLSA recommends transparency
logs and accepts timestamping when transparency is inappropriate. Timestamping itself is in
transition: Rekor v1 covers `integratedTime` with a signed entry timestamp (SET), while the
ecosystem is moving toward RFC3161 timestamp authorities and Rekor v2.

Trust root distribution has its own failure modes. The TUF-distributed public good root is the
default and self-updating, but offline or reproducible verification needs a pinned
`trusted_root.json`, and a stale pinned root is a documented freeze/replay risk. Legacy component
overrides (`SIGSTORE_ROOT_FILE`, `SIGSTORE_REKOR_PUBLIC_KEY`, `SIGSTORE_CT_LOG_PUBLIC_KEY_FILE`)
bypass the governed root entirely.

What must a verifier require inside a bundle, what may it require from the network, and how must
trust material be obtained and kept fresh — for a public, high-assurance builder whose verification
must also work offline and reproducibly?

## Decision Drivers

- Public artifacts deserve public witness: packages and releases published openly should have their
  signing events publicly logged and checkable.
- Verification must work offline and reproducibly from the bundle plus governed trust material;
  network availability must not be a verification dependency.
- Signing-time evidence must be trustworthy: only times covered by a signature (SET or RFC3161),
  never self-declared times.
- Trust material must be governed and fresh: no unmanaged component overrides, no silently stale
  roots.
- Stay aligned with the tooling the project already uses (sigstore-go semantics,
  `gh attestation verify`, `actions/attest` output) and with high-assurance precedent (PyPI).
- Anticipate the Rekor v2 / TSA transition so this ADR can be revised without touching the identity
  policy of ADR 0068.

## Considered Options

- Require Rekor inclusion from the bundle, verified offline; accept TSA timestamps as additional
  evidence.
- Accept TSA-only bundles; make transparency log inclusion optional.
- Require both Rekor inclusion and a TSA timestamp on every bundle.
- Require an online Rekor query at verification time.

For trust root governance, the options are: TUF-managed root by default with pinned roots permitted
under freshness rules; TUF only; pinned roots only.

## Decision Outcome

Chosen option: "Require Rekor inclusion from the bundle, verified offline; accept TSA timestamps as
additional evidence", with "TUF-managed root by default with pinned roots permitted under freshness
rules", because public packages merit public transparency, offline verification matches both the
tooling and the reproducibility requirement, and governed freshness is the smallest rule that keeps
pinned roots safe.

**Transparency requirement.** For every Windlass-signed bundle class, the producer publish gate and
consumer verifiers must verify, failing closed:

1. the Fulcio certificate chain against the governed trust root, including the certificate's
   embedded signed certificate timestamp (SCT) from the Fulcio CT log;
2. Rekor inclusion for the signature — an inclusion proof or signed entry timestamp (SET) contained
   in the bundle — including consistency between the Rekor entry and the bundle's signature and
   certificate;
3. signing-time evidence: that signing occurred while the Fulcio certificate was valid, established
   through the SET-covered integrated time, an RFC3161 signed timestamp when present, or both. A TSA
   timestamp is welcome as additional evidence but does not replace the Rekor inclusion requirement.

**Offline-by-default verification.** Verification must not require network access to Rekor, Fulcio,
or any log operator: the bundle plus governed trust material must be sufficient. Online lookups are
permitted for monitoring and diagnostics but are never part of the required verification path. This
matches sigstore-go's offline-by-construction bundle verification and GitHub's documented offline
verification flow, and it rejects the online-query option as both a availability risk and a mismatch
with current tooling.

**Trust root governance.** The default trust material is the Sigstore public good instance's
TUF-distributed trusted root, refreshed by the verification tooling. A pinned `trusted_root.json` is
permitted for offline or reproducible verification only when the verification procedure documents
its freshness: a pinned root must be revalidated against the TUF repository whenever verification
runs online, and long-lived verification environments must refresh it on a documented schedule
rather than treating it as permanent. Legacy component overrides — `SIGSTORE_ROOT_FILE`,
`SIGSTORE_REKOR_PUBLIC_KEY`, `SIGSTORE_CT_LOG_PUBLIC_KEY_FILE` — are prohibited in the documented
verification path because they bypass root governance.

**Scope.** This policy covers the Sigstore public good instance, which backs public repository
signing through `actions/attest`. GitHub's private Sigstore instance for private repositories is out
of scope for the production profiles.

**Transition note.** The ecosystem is moving from Rekor v1 SET semantics toward Rekor v2 and RFC3161
timestamping. When `actions/attest` and sigstore-go complete that transition, this ADR — not ADR
0068 — is expected to be revisited, and the identity binding policy remains unaffected.

Deferred: exact bundle field paths, sigstore-go verifier options (for example `WithTransparencyLog`,
`WithSignedTimestamps`, `WithObserverTimestamps`), command sequences, pinned-root freshness schedule
values, and fixtures belong to the architecture specifications.

### Consequences

- Good, because every published signature is publicly witnessed: inclusion in Rekor makes signing
  events auditable by third parties, matching PyPI's high-assurance precedent.
- Good, because verification works offline and reproducibly — a bundle plus governed trust material
  suffices, which keeps CI gates and downstream consumers independent of log operator availability.
- Good, because signing-time evidence is always signature-covered, so short-lived Fulcio
  certificates cannot be replayed outside their validity window.
- Good, because trust material has exactly two governed states — TUF-fresh or pinned with freshness
  rules — and unmanaged overrides are excluded by policy rather than by habit.
- Good, because the Rekor v2 transition is contained: revising this ADR does not disturb the
  identity binding policy.
- Bad, because verification requires Rekor material in the bundle, so any future signing adapter
  that omits inclusion proofs is non-conformant by definition, constraining adapter choice.
- Bad, because pinned-root freshness is a process rule that tooling cannot fully enforce;
  documentation and review checklists must carry it.
- Neutral, because TSA timestamps are accepted but not required; if the ecosystem completes the TSA
  transition quickly, this policy may read as conservative until revised.

### Confirmation

This decision is confirmed when:

- architecture specifications define the required bundle contents, the exact verification procedure
  for each bundle class (chain, SCT, Rekor inclusion, signing-time evidence), the offline rule, and
  the trust root governance rules, tracing to this ADR;
- the producer publish gate performs the full offline verification fail-closed before any registry
  mutation, using only bundle-contained evidence and governed trust material;
- verification documentation shows the complete offline procedure and the pinned-root freshness
  rules, and prohibits legacy component override environment variables;
- fixtures demonstrate acceptance of a compliant bundle and rejection for: missing or mismatched
  Rekor entry, missing SCT, signing time outside certificate validity, and an ungoverned or stale
  trust root;
- review checklists confirm that no documented verification path requires a network call to a log
  operator.

## Pros and Cons of the Options

### Require Rekor inclusion from the bundle, verified offline

- Good, because public artifacts get public transparency at full strength while verification stays
  offline, reproducible, and aligned with sigstore-go and GitHub's offline flow.
- Good, because requiring SCT and signature-consistent Rekor entries closes the legacy gap where
  SETs were trusted without full comparison to the bundle.
- Bad, because signing adapters lose the option of producing log-free bundles, and the verification
  procedure has the most parts to specify and fixture-test.

### Accept TSA-only bundles

- Good, because it accommodates closed-network signing and the emerging Rekor v2 model early.
- Bad, because signing events for public packages would escape public transparency, weakening
  third-party audit for no compensating gain in this project's public profile.
- Bad, because it fragments the evidence shape: verifiers would need two acceptable bundle forms
  from day one.

### Require both Rekor inclusion and a TSA timestamp

- Good, because two independent time sources harden the signing-time proof.
- Bad, because it over-constrains the signing adapter before the ecosystem settles, and may reject
  bundles the current adapter produces if either material is absent.

### Require an online Rekor query at verification time

- Good, because it gives the freshest possible log view.
- Bad, because it makes log operator availability a verification dependency, breaks offline and
  reproducible verification, and contradicts how current tooling actually verifies bundles (offline,
  from included proofs).

### Trust root: TUF default with pinned roots under freshness rules

- Good, because the default path self-updates while offline and reproducible verification remains
  possible with a documented freshness obligation.
- Bad, because the freshness obligation for pinned roots is a process rule that needs documentation
  and checklist enforcement rather than tooling enforcement.

### Trust root: TUF only, or pinned only

- Good, because each is simpler than the hybrid.
- Bad, because TUF-only breaks offline and reproducible verification, and pinned-only
  institutionalizes stale-root risk with no freshness baseline.

## More Information

Verification mechanics and offline semantics: sigstore-go verification API and its transparency log,
signed timestamp, and observer timestamp options
(<https://github.com/sigstore/sigstore-go/blob/main/docs/verification.md>,
<https://github.com/sigstore/sigstore-go/blob/main/pkg/verify/signed_entity.go>); the Sigstore
bundle format's requirement for SET- or RFC3161-covered signing times
(<https://docs.sigstore.dev/about/bundle/>); cosign timestamp guidance and deprecated `--offline`
semantics (<https://docs.sigstore.dev/cosign/verifying/timestamps/>,
<https://docs.sigstore.dev/cosign/verifying/verify/>).

Trust root governance: TUF-distributed trusted root and `trusted_root.json`
(<https://docs.sigstore.dev/about/security/>,
<https://github.com/sigstore/sigstore-go/blob/main/pkg/root/trusted_root.go>); custom components and
legacy override variables (<https://docs.sigstore.dev/cosign/system_config/custom_components/>);
GitHub offline attestation verification with a custom trusted root
(<https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/verify-attestations-offline>,
<https://cli.github.com/manual/gh_attestation_verify>).

High-assurance precedent: PyPI requires Rekor and Fulcio CT inclusion proofs for verified
attestations (<https://docs.pypi.org/attestations/security-model/>); SLSA recommends transparency
logs and accepts timestamping when transparency is inappropriate
(<https://slsa.dev/spec/v1.2/build-requirements>).

This ADR is the transparency and trust root half of the verifier trust policy left open by ADR 0037;
ADR 0068 pins the identity half. Together they complete the verifier policy that ADR 0037 deferred
to specifications.
