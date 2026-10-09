---
parent: Decisions
nav_order: 92
status: accepted
date: 12026-10-08
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0070
    scope:
      "the v1 rejection and deferral recorded in Decision Outcome ('Yarn packageManager hash pinning
      ... are rejected for v1 as acquisition-path and input-requirement changes; either may return
      as a later ADR on the reproducibility roadmap'): this ADR is that later ADR and realizes the
      deferred question in optional form — a digest declaration is accepted but never required, and
      the record model keeps the source-native observed authorities. The rejection of required hash
      pins, the source-native authority model, and every other ADR 0070 clause remain in force"
  - type: see-also
    target: ADR-0017
  - type: see-also
    target: ADR-0088
  - type: see-also
    target: ADR-0091
  - type: partially-supersedes
    target: ADR-0091
    scope:
      "the uniform selection rule's rejection of hash-suffixed descriptors ('shortened forms,
      ranges, tags, URLs, and hash-suffixed descriptors are rejected from both fields'): a
      grammar-valid integrity digest suffix (ADRs 0093, 0094) is accepted for pnpm and Yarn from
      both fields. Non-digest build metadata and every other rejection in the uniform rule remain in
      force"
  - type: amended-by
    target: ADR-0093
    scope:
      "the deferral of the accepted digest algorithm set recorded in Decision Outcome ('This ADR
      deliberately does not define the accepted algorithm set or the digest format ... Those axes
      are decided in the specification phase or a follow-up ADR'): ADR 0093 decides the algorithm
      set, pinning it to the W3C SRI v1 token set (sha256, sha384, sha512). The digest format axis
      is decided separately by ADR 0094, and every other ADR 0092 clause remains in force"
  - type: amended-by
    target: ADR-0094
    scope:
      "the deferral of the digest format axis recorded in Decision Outcome ('This ADR deliberately
      does not define the accepted algorithm set or the digest format (Corepack-style
      algorithm.<hex> versus SRI algorithm-<base64>)'): ADR 0094 decides the format, pinning it to
      the Corepack-style +algorithm.<hex> suffix on the exact version in both manifest fields. Every
      other ADR 0092 clause remains in force"
  - type: see-also
    target: ADR-0095
---

# Accept Optional Integrity Digests in Package-Manager Descriptors

## Context and Problem Statement

Corepack accepts an integrity digest embedded in the package-manager descriptor, in both the
top-level `packageManager` field and `devEngines.packageManager.version` — for example
`yarn@4.18.1+sha512.<hex>`. The behavior is documented in Corepack's README and verified against
Corepack 0.36.0 on 12026-10-08: the suffix is parsed as SemVer build metadata, the algorithm name is
passed to the Node runtime's `crypto.createHash` with no Corepack-side algorithm allow-list, and the
downloaded bytes are hashed and compared, failing on mismatch. Two qualifications matter: Corepack
skips this validation when serving from a warm cache, and when no digest is declared it derives the
expected digest from the npm registry's signature-verified `dist.integrity` instead. The other
toolchain members treat the suffix differently: pnpm strips it before version validation, Yarn's
binary ignores both fields entirely, and npm's `devEngines` check ignores SemVer build metadata — so
verification of a declared digest against downloaded bits is Corepack-only and cache-dependent.

slsa-builder currently rejects hash-suffixed descriptors at the specification level, under ADR
0017's exact-version contract and the descriptor grammar that followed it. ADR 0070 considered
requiring a `packageManager` hash pin so that "distribution integrity is source-declared", and
rejected it for v1 "as acquisition-path and input-requirement changes", while noting that the idea
"may return as a later ADR on the reproducibility roadmap". Read precisely, ADR 0070 rejected making
the pin a requirement and making the declared hash the distribution integrity authority; it did not
address an optional declaration reconciled against the observed authorities it adopted.

An optional digest declaration has real value for consumers who want defense in depth against
registry revisions (issue #105) or registry-level compromise: the digest is committed in the
consumer's repository, reviewed in pull requests, and checked at build time, rather than fetched
from the same registry whose compromise is the threat. Corepack's documented practice is a precedent
consumers may already follow.

Should the profile accept an optional integrity digest in its package-manager descriptors, and if
so, with what semantics?

## Decision Drivers

- Optional hardening, never a requirement: the digest is a consumer-chosen security enhancement, not
  an input requirement — ADR 0070's rejection of required hash pins stands.
- ADR 0070's evidence model stays authoritative: distribution integrity is recorded from observed
  acquisition (registry-integrity / download-hash), not from declarations.
- Fail-closed honesty: the profile must not honor a declaration it cannot check itself; Corepack's
  warm-cache skip means builder-side verification is the only reliable enforcement.
- Interoperability with documented Corepack practice: consumers following Corepack's recommendation
  to pin a hash should not be forced to strip it for release builds.
- ADR atomicity: the accepted algorithm set and digest format are separate axes, decided in the
  specification phase or a follow-up ADR — not here.

## Considered Options

- Keep rejecting all digest-suffixed descriptors, closing the build-metadata acceptance gap tracked
  in #122.
- Accept optional digest declarations and verify the acquired distribution against them, failing
  closed on mismatch, while keeping the observed-authority record model.
- Accept digest declarations and record them as metadata without verification.
- Defer the question to the ADR 0088 registry-tarball mechanism's specification phase.

## Decision Outcome

Chosen option: "Accept optional digest declarations and verify the acquired distribution against
them, failing closed on mismatch", because it adds a consumer-chosen integrity anchor that catches
registry revisions and registry-level compromise without changing the acquisition path, the evidence
model, or any requirement — and because the Corepack precedent means the declaration format already
has ecosystem meaning.

### The optional digest contract

- Both manifest selection sources — the top-level `packageManager` field and
  `devEngines.packageManager` — may carry an optional integrity digest alongside the exact version,
  for pnpm and Yarn. The ADR 0017 exact-version contract and every version bound (ADR 0063,
  ADR 0090) are unchanged: the digest never participates in version selection, and a descriptor
  without one is fully supported.
- Reconciliation is builder-side and fail-closed: the profile computes the declared algorithm's
  digest over the acquired distribution bits and compares it with the declared value before install.
  A mismatch fails the build with a dedicated diagnostic, registered in the specification phase.
  This check must not delegate to Corepack, which skips digest validation on warm-cache hits.
- The record model is unchanged: `package-manager-distribution` keeps its observed authorities
  (registry-integrity for pnpm, download-hash for Yarn, per ADR 0070). A declared digest is not an
  evidence authority and does not alter the recorded descriptor.
- npm is unaffected: the profile uses the builder-owned npm runtime, and the manifest npm version is
  not consumed.
- This ADR deliberately does not define the accepted algorithm set or the digest format
  (Corepack-style `algorithm.<hex>` versus SRI `algorithm-<base64>`). Those axes are decided in the
  specification phase or a follow-up ADR; investigated candidates are recorded in More Information.
  Until the specification phase pins the grammar, descriptors carrying a digest keep rejecting with
  the existing diagnostics, and #122's build-metadata acceptance gap is closed under the current
  grammar.

### Confirmation

This decision is confirmed when:

- the JS/TS npm build-and-pack specification defines the optional digest grammar for both manifest
  fields, including the accepted algorithm set and format;
- a dedicated fail-closed diagnostic for declared-versus-observed digest mismatch is registered,
  with specification parity;
- the fixture corpus covers accepted matching declarations, rejected mismatches, and rejected
  malformed declarations from both manifest sources;
- the README support-window section documents the optional digest declaration for consumers;
- a live verification runs a digest-declaring pnpm and Yarn consumer end to end, including a
  warm-cache run proving builder-side enforcement.

## Pros and Cons of the Options

### Keep rejecting all digest-suffixed descriptors

- Good, because the selection grammar stays a single exact-version shape with no ambiguity between
  the selected value and the executed value.
- Good, because no evidence-model, diagnostic, or fixture surface is added.
- Bad, because consumers following Corepack's documented recommendation to pin a hash must strip it
  for release builds — a manifest valid for local development is invalid for release.
- Bad, because it forecloses the strongest selection-time integrity signal: a digest committed in
  the consumer's repository, which would catch registry revisions (#105) and registry-level
  compromise that observed-authority recording only reports after the fact.

### Accept optional digest declarations with fail-closed verification (chosen)

- Good, because it is defense in depth under consumer control: the digest is version-controlled and
  reviewable, unlike registry metadata fetched at build time.
- Good, because the observed-authority record model (ADR 0070), the exact-version contract (ADR
  0017), and every version bound are untouched — the change is additive and optional.
- Good, because builder-side verification keeps the guarantee honest even where Corepack skips it on
  warm caches, and the Corepack precedent gives the declaration ecosystem meaning.
- Bad, because the profile takes on a verification path it previously delegated: digest computation,
  algorithm handling, a new diagnostic, and fixtures.
- Bad, because two consumer classes exist — with and without declarations — and verifiers must
  understand that the recorded authorities, not the declaration, are the evidence.

### Accept digest declarations without verification

- Good, because interoperability is achieved with minimal implementation.
- Bad, because accepting an integrity-looking declaration without checking it is a placebo:
  consumers believe it is enforced, and readers of the record may misread it — contrary to the
  fail-closed, evidence-bound posture.

### Defer to the ADR 0088 mechanism's specification phase

- Good, because the registry-tarball mechanism already builds acquisition-time verification
  machinery the reconciliation could share.
- Bad, because the value is independent of the provisioning mechanism and consumers can use it
  during the Corepack window today; deferral buys no information, since the semantics decided here
  (optional, observed-authority-preserving, fail-closed) do not depend on the mechanism.

## More Information

- ADR 0070's deferral, verbatim: "Yarn `packageManager` hash pinning and a controlled
  `actions/setup-node` acquisition path with `SHASUMS256.txt` verification are rejected for v1 as
  acquisition-path and input-requirement changes; either may return as a later ADR on the
  reproducibility roadmap." This ADR is the later ADR for the hash-pin item; the `SHASUMS256.txt`
  item remains deferred.
- Corepack README, authoring sections: <https://github.com/nodejs/corepack#when-authoring-packages>
  — digest form in both fields.
- Corepack source: `sources/specUtils.ts` (`parseSpec` — suffix parsed as SemVer build metadata, no
  algorithm allow-list) and `sources/corepackUtils.ts` (download hashing, comparison, warm-cache
  behavior, registry-integrity derivation when no digest is declared).
- Live verification on 12026-10-08 (Corepack 0.36.0, Node.js 24.21.0): `sha1`, `sha224`, `sha256`,
  and `sha512` suffixes were all parsed and validated over downloaded bytes (mismatch fails); a warm
  cache skipped validation; cross-field `packageManager`/`devEngines.packageManager` disagreement
  fails by default.
- pnpm strips the suffix before version validation
  (<https://github.com/pnpm/pnpm/commit/302a2f7d2cd68c0e7e2404418cffb4a7e406597>); Yarn's binary
  ignores both fields (verified on 4.18.1); npm's `devEngines` check ignores SemVer build metadata.
- Registry revisions watch: <https://github.com/windlasstech/slsa-builder/issues/105> — the threat
  an optional declared digest directly addresses.
- Build-metadata acceptance gap: <https://github.com/windlasstech/slsa-builder/issues/122> — closed
  under the current grammar before the specification phase defines the digest grammar.
- Investigated but undecided (specification-phase or follow-up ADR candidates): the accepted
  algorithm set (a fixed reviewed set such as `sha256`/`sha512` versus mirroring Corepack's
  runtime-dependent open set); the digest format (Corepack-style `algorithm.<hex>` versus SRI
  `algorithm-<base64>` used by registry `dist.integrity`); whether the fact of a successful
  declared-digest verification is recorded as diagnostic metadata.
