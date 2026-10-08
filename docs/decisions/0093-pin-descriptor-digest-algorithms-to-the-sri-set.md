---
parent: Decisions
nav_order: 93
status: accepted
date: 12026-10-08
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0092
    scope:
      "the deferral of the accepted digest algorithm set recorded in Decision Outcome ('This ADR
      deliberately does not define the accepted algorithm set or the digest format ... Those axes
      are decided in the specification phase or a follow-up ADR'): this ADR decides the algorithm
      set, pinning it to the W3C SRI v1 token set (sha256, sha384, sha512). The digest format axis
      is decided separately by ADR 0094, and every other ADR 0092 clause remains in force"
  - type: see-also
    target: ADR-0070
  - type: see-also
    target: ADR-0094
---

# Pin Descriptor Digest Algorithms to the W3C SRI Set

## Context and Problem Statement

ADR 0092 accepted optional integrity digests in package-manager descriptors with fail-closed
reconciliation, and deliberately deferred two axes: the accepted algorithm set and the digest
format. This ADR decides the algorithm set; the format axis (Corepack-style `algorithm.<hex>` versus
SRI `algorithm-<base64>`) remains deferred to the specification phase.

A wide survey (12026-10-08) established the candidate landscape:

- Ecosystem usage concentrates on two algorithms: SHA-256 (W3C SRI baseline, OCI canonical and
  mandatory-to-verify, Go modules `h1:`, crates.io, PyPI, Gradle verification) and SHA-512 (npm
  `dist.integrity`'s current value, SRI, OCI optional, Gradle; also this project's own recorded
  authorities and dual-digest SLSA subjects). SHA-384 appears only in the SRI token set, with no
  confirmed artifact-integrity use beyond it.
- The W3C Subresource Integrity recommendation defines exactly three algorithm tokens — `sha256`,
  `sha384`, `sha512` — ordered weaker to stronger; it enumerates no others.
- SHA-1 and MD5 are broken for adversarial integrity (SHAttered; NIST's transition away from SHA-1;
  RFC 6151) and appear only as legacy metadata. SHA-224, SHA-384, and the SHA-512/t variants see no
  meaningful package-integrity use; SHA-224 lingers only in Corepack's older built-in examples while
  current `corepack use` writes SHA-512.
- No mainstream package-registry adoption of SHA-3 or BLAKE3 for artifact integrity was found;
  PyPI's published BLAKE2b-256 digests are the one production exception outside the SHA-2 family.
- Implementation cost is flat across the SRI set: every SRI token is supported by the Go standard
  library and by Node.js 24's `crypto.createHash` with zero new dependencies, so the declared digest
  also validates under Corepack during the Corepack window.
- Monotonicity favors starting narrow: widening an accepted algorithm set is backward-compatible
  (existing declarations keep working), while narrowing is a breaking change.

Which algorithms may a declared descriptor digest use?

## Decision Drivers

- Anchor to a reviewed external standard where one fits, rather than inventing a project-local set.
- Cover the algorithms ecosystems actually use for artifact integrity — no more, no less than a
  standard set justifies.
- Keep the grammar closed and deterministic across platforms and runtime versions; no
  runtime-dependent open sets.
- Zero new trusted-core dependencies and full Corepack-window interoperability.
- Preserve a clean, documented extension path for future demand.

## Considered Options

- Pin the set to SHA-512 only.
- Pin the set to the W3C SRI v1 token set (`sha256`, `sha384`, `sha512`).
- Pin the set to the pragmatic pair (`sha256`, `sha512`).
- Adopt a wide modern set adding SHA-3 and BLAKE2-family algorithms.
- Mirror Corepack's open, runtime-dependent algorithm set.

## Decision Outcome

Chosen option: "Pin the set to the W3C SRI v1 token set (`sha256`, `sha384`, `sha512`)", because it
anchors the grammar to an existing reviewed standard, covers every algorithm with confirmed
production use in package-artifact integrity, costs nothing to implement on either runtime, and
keeps the door to widening explicit and cheap.

### The accepted algorithm set

- A declared descriptor digest must use exactly one of `sha256`, `sha384`, or `sha512` — the
  algorithm tokens defined by the W3C Subresource Integrity recommendation. Token names follow SRI
  regardless of which digest format the specification phase later selects.
- Declarations using any other algorithm — including SHA-1, MD5, SHA-224, SHA-512/t variants,
  SHA-3-family algorithms, BLAKE2, BLAKE3, and non-cryptographic hashes — are rejected before
  install with a dedicated diagnostic, registered in the specification phase.
- The digest format axis is unchanged from ADR 0092: Corepack-style `algorithm.<hex>` versus SRI
  `algorithm-<base64>` remains undecided here and is pinned in the specification phase.
- The set's SHA-384 member is accepted for SRI conformance even though no artifact-integrity use
  beyond SRI was confirmed; its cost is one enumeration entry, not new machinery.

### Demand-driven expansion

If ecosystem demand for additional algorithms — for example SHA-3-family or BLAKE2-family digests —
emerges through tooling support, registry adoption, or consumer requests, widening the accepted set
may be revisited as a follow-up decision (a specification restatement or an ADR, as the change
warrants). Widening is backward-compatible: extending the accepted set never invalidates existing
declarations, so starting with the SRI set forecloses nothing.

### Confirmation

This decision is confirmed when:

- the JS/TS npm build-and-pack specification defines the descriptor digest grammar with the accepted
  algorithm set pinned to `sha256`, `sha384`, and `sha512`;
- a dedicated diagnostic for unsupported digest algorithms is registered, with specification parity;
- the fixture corpus covers accepted declarations in all three algorithms and rejected declarations
  for SHA-1, SHA-224, SHA-3, BLAKE2, and unknown algorithm tokens, from both manifest sources.

## Pros and Cons of the Options

### Pin the set to SHA-512 only

- Good, because it matches npm's `dist.integrity` and this project's recorded authorities, so a
  declaration can be compared in the same algorithm end to end.
- Bad, because it rejects SHA-256 declarations — the broadest cross-ecosystem baseline (SRI, OCI, Go
  modules, crates.io, PyPI) — and leaves no algorithm agility at a single point.

### Pin the set to the W3C SRI v1 token set (chosen)

- Good, because the set is an existing reviewed standard: one citation replaces a project-local
  algorithm review, and conformance tooling already exists.
- Good, because it covers every algorithm with confirmed production artifact-integrity use while
  staying closed, deterministic, and zero-cost on both runtimes.
- Bad, because SHA-384 is accepted without confirmed ecosystem demand — accepted for standard
  conformance at near-zero cost, but an entry no consumer may ever use.

### Pin the set to the pragmatic pair (`sha256`, `sha512`)

- Good, because it is the minimal set covering all confirmed production usage.
- Bad, because a project-local subset of the standard invites the recurring question of why SHA-384
  is missing, and the SRI anchor is the stronger citation for the same machinery.

### Adopt a wide modern set adding SHA-3 and BLAKE2

- Good, because algorithm diversity hedges a future SHA-2 break with a differently constructed
  primitive, at low implementation cost.
- Bad, because it institutionalizes contracts nobody can use today: no package-registry adoption, no
  consumer tooling that produces such declarations, and BLAKE2 naming/output-length ambiguity —
  permanent spec, fixture, and fuzz surface for hypothetical demand that the monotonic extension
  path already covers.

### Mirror Corepack's open set

- Good, because descriptor interoperability during the Corepack window is maximal.
- Bad, because the contract drifts with the Node.js/OpenSSL version, admits broken algorithms
  (SHA-1, MD5) unless a blocklist is layered on, and contradicts the project's closed-grammar,
  fail-closed discipline.

## More Information

- W3C Subresource Integrity: <https://www.w3.org/TR/sri/> — the `sha256`/`sha384`/`sha512` token
  set, ordered weaker to stronger; "no other hashing algorithms are currently supported".
- NIST hash-functions policy:
  <https://csrc.nist.gov/projects/hash-functions/nist-policy-on-hash-functions> — SHA-256 as the
  interoperability minimum; no need to migrate from SHA-2 to SHA-3; transition away from SHA-1.
- Ecosystem usage survey (12026-10-08): npm `dist.integrity` (SHA-512 SRI), OCI descriptor spec
  (SHA-256 canonical and mandatory), Go modules (`h1:` SHA-256), crates.io (`cksum` SHA-256), PyPI
  (SHA-256 with BLAKE2b-256 also published), Gradle verification (SHA-256/SHA-512 recommended),
  Corepack (`corepack use` writes `sha512.<hex>`; older built-in examples use SHA-224).
- Runtime support matrix (verified 12026-10-08): Go standard library covers the full SHA-2 family;
  Go 1.24+ adds `crypto/sha3`; Node.js 24 `crypto.createHash` supports all three SRI tokens plus
  SHA-3 and BLAKE2. The builder's Go 1.27 toolchain needs no new dependency for the SRI set.
- Security exclusions: SHAttered (<https://shattered.io/static/shattered.pdf>), NIST SHA-1
  transition
  (<https://www.nist.gov/news-events/news/2022/12/nist-transitioning-away-sha-1-all-applications>),
  RFC 6151 for MD5 (<https://www.rfc-editor.org/rfc/rfc6151.html>).
