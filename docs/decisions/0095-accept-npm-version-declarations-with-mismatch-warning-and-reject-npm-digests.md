---
parent: Decisions
nav_order: 95
status: accepted
date: 12026-10-09
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0017
    scope:
      "the npm clause ('record npm's actual version'): declared npm versions remain accepted and
      non-authoritative, but a declared npm version that does not match the toolchain npm now
      produces a warning diagnostic; the recording rule itself is unchanged"
  - type: see-also
    target: ADR-0016
  - type: see-also
    target: ADR-0085
  - type: see-also
    target: ADR-0092
  - type: partially-supersedes
    target: ADR-0091
    scope:
      "the uniform selection rule as applied to npm ('shortened forms, ranges, tags, URLs, and
      hash-suffixed descriptors are rejected from both fields' for any supported manager): npm
      version declarations, including ranges, are accepted as non-authoritative input with a
      mismatch warning, while npm digest suffixes remain rejected. pnpm and Yarn semantics are
      unchanged by this edge"
---

# Accept npm Version Declarations with a Mismatch Warning and Reject npm Digest Declarations

## Context and Problem Statement

ADR 0016 binds npm to the builder-owned Node.js toolchain: the profile uses the npm CLI bundled with
the pinned Node.js runtime and does not provision npm through Corepack. ADR 0017 accordingly does
not require npm version pins and records npm's actual version, and ADR 0085 hardens the binding by
pinning the Node.js patch line per builder release and asserting the expected bundled npm pair. ADR
0092 accepted optional integrity digest declarations scoped to pnpm and Yarn, and stated only that
npm is unaffected: it did not define what happens when an npm descriptor carries a digest suffix,
nor what a declared npm version means when it diverges from the toolchain npm.

Empirical investigation (12026-10-08 and 12026-10-09, Corepack 0.36.0 and npm 11.5.1) established
the tool behavior these declarations meet:

- Corepack provisions npm from either the top-level `packageManager` field or
  `devEngines.packageManager`, and enforces `+algorithm.<hex>` descriptor hashes over the downloaded
  registry tarball on a cold cache (warm-cache hits skip validation). npm hash pins are therefore
  meaningful in Corepack flows: npm's registry tarball carries SRI integrity just like pnpm's.
- The npm CLI ignores the top-level `packageManager` field entirely — a mismatched or hash-suffixed
  value passes — but enforces `devEngines.packageManager.version` as a SemVer range against the
  running npm, failing with `EBADDEVENGINES` on mismatch by default. `onFail: "warn"` or `"ignore"`
  lets a mismatch pass, npm does not implement `onFail: "download"`, and SemVer build metadata — the
  digest suffix — is ignored: npm never verifies descriptor hashes.

Under the toolchain-bound model, slsa-builder never acquires an npm distribution: the executed npm
is fixed at invocation time by the toolchain. Two gaps follow. First, a declared npm version that
diverges from the toolchain npm is silently accepted today; npm's own devEngines self-enforcement
may fail the build or pass it under `onFail` weakening, but that behavior is npm's, not the
builder's, and the divergence itself is invisible in slsa-builder's diagnostics. Second, an npm
digest declaration is inexpressible in the profile's semantics: there is no acquired npm
distribution to reconcile it against, and no enforcement agent exists for it.

## Decision Drivers

- Keep the toolchain-bound npm model (ADRs 0016, 0085); do not provision npm as a distribution.
- Never leave a declared integrity expectation silently unenforced.
- Take responsibility only where slsa-builder controls the outcome. For npm, the executed version is
  already determined at invocation time by the toolchain, so the npm CLI's own version
  interpretation — its top-level field ignoring, its devEngines range check, and its `onFail`
  effects — lies outside the builder's responsibility scope; `onFail`-driven weakening is the
  project author's own decision and is respected.
- Make npm version divergence between the manifest and the toolchain visible without breaking
  builds: warn, never fail.
- Document the responsibility boundary clearly so users can tell which behaviors are npm's own.

## Considered Options

- Accept declared npm versions with a mismatch warning, stay out of npm CLI self-enforcement, and
  reject npm digest declarations.
- Accept and silently ignore npm version and digest declarations (the status quo ante for versions).
- Take over npm version enforcement: fail closed on any declared npm version that does not
  range-match the toolchain npm, and reject `onFail: "warn"` or `"ignore"` for npm.
- Provision npm like the other managers — via Corepack, or via the ADR 0084 digest-verified
  registry-tarball pattern — so npm pins and digests become enforceable.
- Reject npm descriptors carrying any version or digest suffix.

## Decision Outcome

Chosen option: "Accept declared npm versions with a mismatch warning, stay out of npm CLI
self-enforcement, and reject npm digest declarations", because npm is the one package manager whose
executed version is fixed by the toolchain before any invocation, so policing npm's own field
interpretation would take responsibility for behavior the builder does not control, while a digest
declaration on npm would promise an integrity check that no component performs.

The initial JS/TS npm package profile should:

- keep using the npm CLI bundled with the pinned Node.js toolchain for npm projects, unchanged;
- accept an npm descriptor version declared in the top-level `packageManager` field or in
  `devEngines.packageManager`, compare it with the toolchain npm's actual version, and emit a
  registered warning diagnostic when they do not match — never failing on this account; the declared
  version remains non-authoritative, and provenance keeps recording the actual npm version;
- not verify, override, or otherwise intervene in the npm CLI's own handling of these fields —
  including its `devEngines.packageManager` range check, its `onFail` semantics, and its ignoring of
  the top-level field — and document this responsibility boundary in the user-facing documentation;
- reject an npm descriptor carrying an integrity digest suffix, because under the toolchain-bound
  model no npm distribution is acquired and the declaration is therefore inexpressible and
  unverifiable.

### Consequences

- Good, because no integrity declaration can be silently inert: a declared digest is always
  reconciled (pnpm and Yarn, per ADR 0092) or rejected (npm).
- Good, because npm version divergence between the manifest and the toolchain becomes visible in
  diagnostics without breaking builds.
- Good, because the responsibility boundary is explicit: users can tell which behaviors are npm's
  own — devEngines self-enforcement, `onFail` effects, top-level ignoring — from the documented
  contract.
- Good, because the settled toolchain model (ADRs 0016, 0085) is untouched, and rejection keeps the
  fail-closed-to-relax direction open: a future ADR can provision npm (for example with the ADR 0084
  pattern) and admit npm digests without breaking any consumer.
- Bad, because consumers carrying Corepack-style npm hash pins are rejected even though Corepack
  would honor them. The project judges this acceptable because npm-adopting projects plausibly
  invoke npm directly — npm ships with Node.js — rather than routing it through Corepack. This is a
  hypothesis, not a measurement.
- Bad, because a new warning diagnostic and its fixtures must be maintained, and the specification's
  `onFail` clause needs an explicit npm carve-out describing where the builder does not intervene.
- Neutral, because npm version declarations remain non-authoritative, as ADR 0017 decided.

### Confirmation

This decision is confirmed when:

- the JS/TS npm build-and-pack specification defines the npm version-mismatch warning and the npm
  digest rejection, with a registered warning diagnostic and specification parity;
- the fixture corpus covers accepted npm version declarations (matching, and mismatching with a
  warning) and rejected npm digest suffixes in both manifest fields;
- the README documents the npm responsibility boundary, including the npm CLI's own devEngines
  enforcement and `onFail` semantics;
- the implementation aligns both manifest fields, closing the gap where a
  `devEngines.packageManager` npm version carrying a `+` suffix was silently accepted.

## Pros and Cons of the Options

### Accept with a mismatch warning, out-of-scope self-enforcement, reject digests

- Good and bad as listed in the consequences above.

### Accept and silently ignore (status quo ante)

- Good, because it is zero-change.
- Bad, because a digest declaration would be silently unenforced — a false-assurance channel — and
  version divergence would stay invisible.

### Take over npm version enforcement

- Good, because declared npm versions would be guaranteed meaningful.
- Bad, because it reverses ADR 0017's npm clause without new evidence, duplicates the npm CLI's own
  check with a second policy, makes the builder answer for npm behavior it does not control, and
  would have to forbid `onFail` values that are the project author's own choice.

### Provision npm as a distribution (Corepack or the ADR 0084 pattern)

- Good, because npm pins and digests would become genuinely enforceable and npm would gain a
  recorded distribution descriptor.
- Bad, because it reverses ADR 0016's settled scope and ADR 0085's deliberate hardening of the
  toolchain-pair model ("the heavier alternatives are either inconsistent with builder-owned npm or
  buy version freedom the build stage does not currently need"), adds a trusted-core provisioning
  path with new failure modes, and answers a demand no consumer has recorded. Reconsider if real
  demand emerges; the ADR 0084 publish-stage pattern is the ready vehicle.

### Reject npm descriptors carrying any version or digest

- Good, because it is the simplest uniform rejection.
- Bad, because it breaks today's accepted npm version declarations for no integrity gain: a version
  declaration is harmless metadata the toolchain already overrides, unlike a digest, which asserts a
  check that nothing performs.

## More Information

- Empirical matrices (12026-10-08 and 12026-10-09, Corepack 0.36.0, npm 11.5.1): Corepack provisions
  npm from either manifest field and enforces descriptor hashes over the registry tarball on cold
  cache (warm cache skips validation); the npm CLI ignores the top-level `packageManager` field,
  enforces `devEngines.packageManager.version` as a SemVer range against itself with
  `EBADDEVENGINES` on mismatch (default `onFail` is `error`; `warn` and `ignore` pass; `download` is
  unsupported), and ignores SemVer build metadata, never verifying descriptor hashes.
- Corepack npm shims are opt-in: `corepack enable` installs no npm shim unless explicitly requested,
  because npm is distributed with Node.js (ADR 0016's original context). The profile's explicit
  Corepack invocation does not rely on shims.
- ADR 0084 (publish-stage npm from a digest-verified registry tarball) is the established pattern if
  npm provisioning is ever revisited.
- The comparison semantics for declared npm versions that are ranges or unparseable strings are
  specified at the specification level.
