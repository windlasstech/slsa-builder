---
parent: Decisions
nav_order: 97
status: accepted
date: 12026-10-12
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0096
    scope:
      "ADR 0096 closed the build-influence channel of a project .corepack.env and left tarball
      inclusion unaddressed; this ADR adds a post-pack gate rejecting the file when it would ship in
      the packed artifact. The COREPACK_ENV_FILE=0 pin and the presence warning are unchanged."
  - type: see-also
    target: ADR-0034
---

# Reject a Packed .corepack.env at the Post-Pack Artifact Gate

## Context and Problem Statement

ADR 0096 neutralized the build-influence channel of a project `.corepack.env`: every Corepack-path
process runs with `COREPACK_ENV_FILE=0`, and the file's presence at the package-manager root
produces a non-fatal warning. That decision deliberately introduced no parser and scoped itself to
whether the file can influence the build. It did not address a second, independent channel: the file
can ship inside the published tarball.

Empirical verification (npm 11.19.0, pnpm 12.10.1, 12026-10-12): when the manifest declares no
`files` allowlist, both managers pack `.corepack.env` into the tarball, and no package manager's
built-in ignore list covers it. npm-packlist's defaults ignore `**/.npmrc` but neither `.env` nor
`.corepack.env`; Yarn Berry's `plugin-pack` `ALWAYS_IGNORE` list excludes neither `.npmrc` nor
`.corepack.env`; pnpm documents no built-in exclusion. A `files` allowlist excludes the file.

The file is credential-designated by Corepack's own documentation: `COREPACK_NPM_TOKEN` "sets a
Bearer token authorization header when connecting to a npm type registry", and
`COREPACK_NPM_USERNAME` / `COREPACK_NPM_PASSWORD` carry Basic-auth credentials in plain text.
Loading is prefix-based ("Only keys that start with `COREPACK_`"), so the set of security-relevant
keys is not fixed: keys added by future Corepack releases load automatically. The tarball channel is
broader still than the loader: a tarball ships the file's bytes verbatim, including comments,
non-`COREPACK_` lines, and lines a parser would reject, content no key-level inspection can bound.

cubic review of PR #130 flagged the open channel (High): credential-bearing `.corepack.env` files
pass the preflight uninspected, and if the manifest's `files` includes the file, npm packs it,
exposing any `COREPACK_NPM_TOKEN` it contains. Before ADR 0096, the preflight prohibition failed
closed on any `.corepack.env`, so no build, and no tarball, ever contained one; the warn-only
transition opened this channel. ADR 0088's removal of Corepack from provisioning does not close it:
the file remains on disk, and pack inclusion is independent of whether Corepack runs.

The ecosystem census (ADR 0096, 12026-10-12) found 15 repositories with a committed `.corepack.env`,
all carrying benign toggles and zero registry keys, a lower bound within GitHub's index rather than
proof of absence. Documented adjacent precedent exists for tool-configuration dotfiles leaking
secrets through npm tarballs (Mobb, 12026: `.claude/settings.local.json` files containing bearer
tokens found in published tarballs).

How does the builder keep a `.corepack.env` out of the published tarball?

## Decision Drivers

- The trusted core fails closed on release-mutation paths; a credential-bearing release artifact is
  a mutation-class defect.
- No parser on attacker-controlled bytes (ADR 0096 driver, still binding).
- Three pack implementations with divergent, partially undocumented ignore semantics; a solution
  keyed to manager behavior drifts with them.
- Protection must not fail open for future `COREPACK_*` keys or for content no parser recognizes.
- ADR 0034's posture: credentials are neither accepted nor forwarded by the initial profile, and a
  published tarball is a forward.
- Minimal consumer friction: a harmless file that stays out of the artifact keeps the ADR 0096
  warning path.

## Considered Options

- Accept the residual risk: document the channel and leave the artifact ungated.
- Reject a packed `.corepack.env` at the post-pack artifact gate (filename match on the packed entry
  list).
- Post-pack advisory warning only.
- Parse `.corepack.env` and reject when credential-designated keys are present.
- Predict pack inclusion at preflight by reimplementing each manager's
  `files`/`.npmignore`/`.gitignore` semantics.
- Mutate the consumer workspace before pack (move or delete the file, or inject ignore rules).

## Decision Outcome

Chosen option: "Reject a packed `.corepack.env` at the post-pack artifact gate", because it closes
the entire channel, not merely the keys a parser recognizes, with no parser, no workspace mutation,
and no dependence on any manager's pack semantics.

Behavioral definition:

- After the pack step completes and the packed artifact's identity is re-verified against the
  manifest (ADR 0019), and before packed metadata is written, the build stage inspects the packed
  tarball's entry list, which the existing archive inspection already path-validates and collects.
- If any regular-file entry's final path segment is `.corepack.env`, the build fails closed with the
  registered error `windlass.verify.error.packed-corepack-env-prohibited` (verification phase, exit
  code 1, no registry mutation possible).
- The check is a filename comparison over already-validated entry paths. The file is never opened,
  read, or parsed; archive-entry content inspection is unchanged.
- The rule applies uniformly to the npm, pnpm, and Yarn pack paths.
- The ADR 0096 presence warning is unchanged: a `.corepack.env` that does not enter the artifact
  continues to produce `windlass.verify.warning.project-corepack-env-ignored` and the build
  proceeds.
- The error message instructs the consumer to declare a `files` allowlist that excludes the file or
  to remove it before publishing.

### Consequences

- Good, because the tarball channel is closed by construction for any file content, including
  comments, non-`COREPACK_` lines, and keys future Corepack releases have not defined yet.
- Good, because no parser is added to the trust boundary, so the decision incurs no fuzz or
  negative-test obligations under docs/testing-guide.md.
- Good, because the gate evaluates the artifact rather than npm-packlist, pnpm, or Yarn
  `plugin-pack` behavior, so divergent or drifting pack semantics cannot weaken it.
- Good, because the consumer remediation is deterministic and documented in the error: declare a
  `files` allowlist that excludes the file, or remove it.
- Bad, because a consumer who intentionally ships `.corepack.env` sees a hard publish failure. No
  such use case is known, the file configures local package-manager downloads and has no function
  inside a published artifact, and the census shows none, but the failure mode is a rejection rather
  than an advisory.
- Neutral, because the fixture corpus gains no rejected-fixture entry for this gate until the
  fixture harness supports post-pack rejection; coverage rests on unit and hermetic end-to-end
  tests.

### Confirmation

- Unit tests cover the entry-list predicate (absent, root entry, nested entry, similarly suffixed
  names pass).
- Hermetic end-to-end tests drive the npm, pnpm, and Yarn shim packers to produce tarballs
  containing `.corepack.env` and assert the registered error; the accepted `.corepack.env` fixtures
  declare a `files` allowlist so their warn-and-proceed behavior is pinned unchanged.
- No rejected-fixture corpus entry accompanies this ADR: the fixture harness runs rejected fixtures
  through the Analyze stage only, and this gate fires after pack. A rejected-fixture corpus entry is
  deferred until the harness supports post-pack rejection.

## Pros and Cons of the Options

### Accept the residual risk

- Good, because it spends nothing: the census observed no credential-bearing file.
- Bad, because the channel stays open indefinitely (ADR 0088 does not close it) and the census is a
  lower bound the ADR itself qualifies as bounded evidence of absence, not proof of nonexistence.
  Documented adjacent incidents (tool dotfiles with bearer tokens in published tarballs) show the
  accident class is real.
- Bad, because the builder's only remaining defense would be consumers remembering to exclude the
  file, which is the configuration npm itself refuses to rely on for `.npmrc`.

### Reject a packed .corepack.env at the post-pack artifact gate (chosen)

- Good, because it matches the threat model exactly: the hazard is the file entering the artifact,
  and the gate fires on precisely that event, for any content, including content no parser would
  read.
- Good, because it adds no parser to the trust boundary, honoring ADR 0096's driver and avoiding the
  fuzz and negative-test obligations of docs/testing-guide.md.
- Good, because it is manager-agnostic by construction: it evaluates the tarball, not npm-packlist,
  pnpm, or plugin-pack behavior, so divergent or drifting ignore semantics cannot weaken it.
- Good, because it cannot fail open for future `COREPACK_*` keys: filename membership is independent
  of what any future Corepack release reads.
- Good, because it composes with ADR 0096's lifecycle: warn when the file is present but excluded,
  fail when it would ship.
- Bad, because it breaks a publish for a consumer who intentionally ships `.corepack.env`. No such
  use case is known, and the census shows none, but the failure mode is a hard rejection rather than
  an advisory.
- Neutral, because it costs one registered error ID, one spec row, and tests; the entry list it
  consumes already exists.

### Post-pack advisory warning only

- Good, because it never breaks a publish.
- Bad, because nothing has been neutralized: ADR 0096 could afford a warning because
  `COREPACK_ENV_FILE=0` removes the hazard outright; here the hazard (publication of the bytes)
  completes while the warning scrolls by. In an unattended release workflow a warning is not a
  control.
- Bad, because it is inconsistent with the fail-closed contract applied to every other mutation-path
  defect.

### Parse .corepack.env and reject credential-designated keys

- Good, because it would permit publishing files that contain only benign keys.
- Bad, because ADR 0096 already rejected this family: a parser on attacker-controlled bytes incurs
  fuzz and negative-test obligations, and a denylist fails open for keys added by future Corepack
  releases, a structural property now confirmed by prefix-based loading.
- Bad, because it protects the wrong boundary: the tarball ships bytes a parser ignores or rejects
  (comments, non-`COREPACK_` keys, malformed lines), so even a perfect key parser cannot bound what
  leaks.
- Bad, because the existing `.npmrc`/`.yarnrc.yml` credential heuristic does not recognize
  `COREPACK_NPM_TOKEN` (no matching substring), so reuse would still require a second key
  enumeration to maintain.

### Predict pack inclusion at preflight

- Good, because rejection would happen before pack.
- Bad, because it reimplements three divergent packlist semantics (npm-packlist; pnpm; Yarn
  `plugin-pack`, which does not even default-ignore `.npmrc`) and drifts with each manager release.
  Strictly dominated by evaluating the artifact itself.

### Mutate the consumer workspace before pack

- Good, because exclusion would be guaranteed regardless of artifact inspection.
- Bad, because the builder does not modify consumer sources: moving or deleting the file, or writing
  `.npmignore`/`files` rules, violates workspace immutability, can race build scripts that read the
  file, and differs per manager. Strictly dominated.

## More Information

- Trigger: cubic review issue `03c36334-ac45-486c-8398-3d4d2c52f589` on
  [PR #130](https://github.com/windlasstech/slsa-builder/pull/130) (High, security_privacy);
  predecessor decision request [#128](https://github.com/windlasstech/slsa-builder/issues/128).
- Primary sources: Corepack README and env-file loader at revision `d4dcb1f` (prefix-based loading;
  `COREPACK_ENV_FILE=0`; the npm auth variables);
  [npm-packlist](https://github.com/npm/npm-packlist/blob/main/lib/index.js) built-in ignores; Yarn
  Berry
  [plugin-pack](https://github.com/yarnpkg/berry/blob/master/packages/plugin-pack/sources/packUtils.ts)
  `ALWAYS_IGNORE`. `.corepack.env` support landed in Corepack 0.32.0
  ([nodejs/corepack#642](https://github.com/nodejs/corepack/pull/642), 12025-02-28); Corepack is
  unbundled from Node.js 25 onward.
- Adjacent incident reporting: Mobb, 12026, on `.claude/settings.local.json` credentials found in
  published npm tarballs (secondary report; counts not independently verified).
- Watch item, out of scope: Yarn Berry's built-in ignore list excludes neither `.npmrc` nor
  `.yarnrc.yml`, so non-credential package-manager configuration can also ship in tarballs. The ADR
  0034 preflight already rejects credential material in those files; whether to extend an artifact
  gate to them is a separate decision.
