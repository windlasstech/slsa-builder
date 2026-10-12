---
parent: Decisions
nav_order: 96
status: accepted
date: 12026-10-12
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0016
  - type: see-also
    target: ADR-0070
  - type: see-also
    target: ADR-0086
  - type: see-also
    target: ADR-0088
  - type: see-also
    target: ADR-0095
---

# Neutralize the Project .corepack.env with a Pinned COREPACK_ENV_FILE=0 and Warn on Its Presence

## Context and Problem Statement

The JS/TS npm profile's build stage runs pnpm and Yarn through Corepack (ADR 0016; npm is the pinned
Node.js toolchain's npm and never touches Corepack). Corepack 0.32.0 added `.corepack.env`
([nodejs/corepack#642](https://github.com/nodejs/corepack/pull/642)): a project-local env file that
Corepack loads from the first directory containing it on the upward traversal from the process
working directory, honoring any `COREPACK_`-prefixed key except `COREPACK_ENABLE_DOWNLOAD_PROMPT`
and `COREPACK_ENV_FILE`. Process environment variables take precedence over file values, and
`COREPACK_ENV_FILE=0` disables file loading entirely.

PR #68 prohibited any `.corepack.env` at the package-manager root, failing closed with
`project .corepack.env is prohibited for release builds`, and PR #127 documented that behavior in
the build-pack specification. The prohibition was never decided in an ADR; issue #128 asks for a
dedicated decision on this single axis: how the builder treats a project `.corepack.env` during
release builds.

Research for that issue established the relevant facts:

- The file's attack surface is wider than a registry redirect. Settable keys include
  `COREPACK_NPM_REGISTRY` (package-manager distribution download endpoint),
  `COREPACK_ENABLE_UNSAFE_CUSTOM_URLS` (custom URLs for known package managers),
  `COREPACK_INTEGRITY_KEYS` (an empty or `0` value skips integrity verification),
  `COREPACK_ON_UNVERIFIED_DOWNLOAD`, and the `COREPACK_NPM_TOKEN` / `COREPACK_NPM_USERNAME` /
  `COREPACK_NPM_PASSWORD` credential set. Future Corepack releases may add more.
- The builder spawns every package-manager command with a fully enumerated child environment and the
  package-manager root as its working directory, so a pinned `COREPACK_ENV_FILE=0` in that
  environment is unconditional for builder-invoked processes: Corepack cannot see a file value that
  overrides it, because file loading is skipped before any merge. These semantics were verified by
  execution on Corepack 0.36.0 (file values redirect fetches when unpinned; real environment wins
  over the file; `COREPACK_ENV_FILE=0` disables loading; a file-set `COREPACK_ENV_FILE` is ignored).
  The kill switch has existed since 0.32.0, and the pinned Node.js 24 toolchain (ADR 0027, hardened
  per-patch in ADR 0085) bundles a Corepack that supports it.
- Ecosystem usage is rare and benign: a GitHub code search census found 15 committed `.corepack.env`
  files across as many repositories (four independent projects and one eleven-repository template
  family), dominated by `COREPACK_ENABLE_AUTO_PIN=0` and `COREPACK_ENABLE_DOWNLOAD_PROMPT=0`, with
  no committed occurrence of a registry override. The commonly committed values duplicate pins the
  builder's controlled environment already sets, so the typical real-world file is harmless under
  builder semantics. Registry-mirror usage of `COREPACK_NPM_REGISTRY` exists but lives in
  Dockerfiles and shell environments, not committed files, and its enterprise audience
  (nodejs/corepack#741) is outside the github.com-only production profile (ADRs 0027, 0068, 0075).
- The prohibition's cost is a reverse divergence: a harmless committed file builds fine locally but
  fails the release build outright. The conflict is explicit, but it is a hard failure for a
  configuration that almost never matters.
- Corepack is a deprecating dependency on every front: Node.js 25 no longer bundles it, pnpm removed
  it from its recommended install paths, ADR 0086 removed it from this repository's own development
  tooling, and ADR 0088 (accepted, not yet implemented) replaces it on the production build stage
  with digest-verified registry tarballs. Any policy for this file has a short half-life, which
  argues against high-investment options.
- ADR 0095 established the pattern of surfacing policy-relevant divergences as registered, non-fatal
  warning diagnostics instead of failures.

How should the builder treat a project `.corepack.env` at the package-manager root?

## Decision Drivers

- Preserve the security invariant: a project-controlled file must never influence the
  package-manager distribution the builder executes, including via keys added by future Corepack
  releases.
- Keep the invariant cheap: no parser on attacker-controlled bytes, which under
  `docs/testing-guide.md` would require fuzz and negative-test coverage.
- Eliminate the hard failure for harmless files; consumers should not have to delete a benign
  committed file to release.
- Avoid silent divergence between local and builder behavior: when the file is present and ignored,
  that fact must be visible in the build output.
- Prefer a mechanism whose semantics are pinned by the builder-owned toolchain, so upstream drift
  arrives only through a reviewed toolchain bump.
- Stay consistent with the ADR 0088 trajectory and keep the decision easy to retire when Corepack
  leaves the production path.

## Considered Options

- Keep the prohibition (fail closed at preflight and before each manager command).
- Pin `COREPACK_ENV_FILE=0` in the controlled environment and warn, without failing, when the file
  is present.
- Pin `COREPACK_ENV_FILE=0` silently, with no warning.
- Parse the file and accept only an allowlisted key set.
- Read and parse the file, rejecting a denylist of security-relevant keys.
- Honor the file only when the consumer opts in through a workflow input.
- Leave the file's parsing and behavior to Corepack: the builder never reads the file itself, but
  pins known security-relevant keys in the controlled environment so environment precedence
  overrides their file values, while Corepack still parses the file and honors every remaining key.

## Decision Outcome

Chosen option: "Pin `COREPACK_ENV_FILE=0` in the controlled environment and warn, without failing,
when the file is present", because it preserves the security invariant with a mechanism the builder
fully controls, removes the hard failure for harmless files, keeps the divergence visible through
the ADR 0095 warning surface, and adds no parser to the trust boundary.

Behavioral definition:

- `COREPACK_ENV_FILE=0` is added to the controlled environment applied to every Corepack-path
  process, including toolchain provisioning invocations. This is the entire safety mechanism; it
  does not depend on any filesystem inspection.
- At the credential-configuration preflight, the profile inspects the package-manager root for
  `.corepack.env` as today. Presence of any entry, including a symlink, now produces a registered
  warning diagnostic stating that the file is ignored in release builds, and the build proceeds.
  Because the invariant no longer depends on the inspection, an inspection failure other than
  absence also degrades to a warning rather than stopping the build.
- The warning is emitted once at the preflight. The per-command re-check of the file is withdrawn:
  the environment pin applies to every command unconditionally, so repeated filesystem inspection
  buys nothing.
- The warning diagnostic follows the closed registry's warning namespace
  (`windlass.verify.warning.*`), consistent with the ADR 0095 precedent; the concrete category name
  is delegated to the specification.
- The npm path is unaffected: npm builds never invoke Corepack.
- Until the implementation lands, the existing prohibition remains in force; this ADR's confirmation
  criteria gate the change.

### Consequences

- Good, because the invariant is enforced by a mechanism the builder owns end to end: the child
  environment is fully enumerated, file loading is disabled before any value merge, and the
  semantics have been stable since Corepack 0.32.0 and were verified by execution on 0.36.0.
- Good, because future `COREPACK_*` keys are neutralized by construction: the file is never read, so
  an unknown key cannot fail open the way an enumeration-based defense can.
- Good, because consumers with harmless committed files no longer face a release-blocking failure,
  removing the reverse divergence that the prohibition created.
- Good, because ignoring the file is never silent: the warning diagnostic surfaces the file's
  presence and its non-effect in the build report.
- Good, because no parser is added to the trust boundary, avoiding the fuzz and negative-test
  obligations that `docs/testing-guide.md` attaches to parsing attacker-controlled input.
- Neutral, because a consumer who genuinely needs file-based Corepack configuration (a registry
  mirror, in the nodejs/corepack#741 pattern) receives a build that visibly ignores it; the
  supported consumption path is github.com-only (ADRs 0027, 0068, 0075), so this audience is
  currently out of scope.
- Neutral, because the mechanism's correctness depends on upstream preserving `COREPACK_ENV_FILE=0`
  semantics; that drift is gated by the pinned Node.js toolchain (ADRs 0027, 0085), under which a
  Corepack behavior change can only arrive through a reviewed toolchain bump.
- Bad, because the explicit conflict signal is weakened from an error to a warning: a consumer who
  believed the file influenced the release build learns otherwise only by reading warnings. The
  visibility requirement in the confirmation criteria exists to mitigate this.

### Confirmation

This decision is confirmed when:

- the controlled environment pins `COREPACK_ENV_FILE=0` for every Corepack-path process, including
  toolchain provisioning, and a test asserts the pin's presence;
- the build-pack specification's credential-configuration preflight section is rewritten from
  prohibition to neutralization plus warning, with specification parity tests;
- a warning diagnostic for the ignored `.corepack.env` is registered in the closed diagnostic
  registry and covered by unit tests for presence, absence, symlink, and inspection-failure cases;
- the ADR relations check passes for this ADR's `see-also` edges;
- the build report surfaces the warning in a way a consumer can find without reading debug logs.

## Pros and Cons of the Options

### Keep the prohibition

- Good, because the conflict is maximally explicit and the current code, specification, and tests
  already implement it.
- Bad, because a harmless file fails the release build, producing the reverse divergence of "works
  locally, fails in the builder" for a configuration that real-world usage shows is almost always
  benign.
- Bad, because it spends consumer-facing friction on a mechanism that ADR 0088 is already scheduled
  to make meaningless on the production path.

### Pin `COREPACK_ENV_FILE=0` and warn on presence (chosen)

Described in the Decision Outcome. It is the only option that is simultaneously future-key-proof,
parser-free, non-blocking for harmless files, and non-silent.

### Pin `COREPACK_ENV_FILE=0` silently

- Good, because it shares the chosen option's safety mechanism and simplicity.
- Bad, because ignoring project configuration without saying so creates opaque divergence between
  local and builder behavior; the pnpm ecosystem's 2026 trust-source filtering work
  (GHSA-3qhv-2rgh-x77r and its migration guidance) shows that behavior changes without visible
  signaling generate real confusion cost.

### Parse the file and accept an allowlisted key set

- Good, because it is default-deny for unknown keys and preserves flexibility for benign ones.
- Bad, because it places a parser on attacker-controlled bytes at the trust boundary, incurring the
  fuzz and negative-test obligations of `docs/testing-guide.md`.
- Bad, because every allowed key's semantics must be re-reviewed on each Corepack upgrade, a
  version-coupled maintenance burden that outlives the file's usefulness under ADR 0088.

### Read and parse the file, rejecting a denylist of security-relevant keys

- Bad, because it pays the same parser cost as the allowlist while failing open for dangerous keys
  added by future Corepack releases. Strictly dominated by the allowlist option.

### Honor the file only on an explicit workflow input

- Good, because it follows the explicit-trust precedent of direnv's `allow` gate and Nix's
  `accept-flake-config=false`, and the opt-in could be recorded as an external parameter in the SLSA
  Build L3 sense.
- Bad, because it expands the workflow's API surface for a feature with near-zero observed demand;
  the census found no committed file that needs honoring. If demand materializes, a follow-up ADR
  can add the opt-in without revisiting this decision.

### Leave the file to Corepack: the builder never reads it and pins known security-relevant keys

- Good, because environment precedence is verified to beat file values, so pinning
  `COREPACK_NPM_REGISTRY`, `COREPACK_ENABLE_UNSAFE_CUSTOM_URLS`, and peers would neutralize known
  vectors while Corepack still parses the file and honors the remaining benign keys.
- Bad, because it fails open for future keys: precedence protects only the keys the builder
  enumerates, which is exactly the enumerability limit that motivated this decision.
- Bad, because some keys resist pinning: `COREPACK_INTEGRITY_KEYS` has no "use the defaults" value,
  so pinning it would mean embedding a copy of upstream key material that can drift.

## More Information

- Issue: [#128](https://github.com/windlasstech/slsa-builder/issues/128) (decision request);
  introduced by [#68](https://github.com/windlasstech/slsa-builder/pull/68); documented by
  [#127](https://github.com/windlasstech/slsa-builder/pull/127).
- Upstream mechanics: [Corepack README](https://github.com/nodejs/corepack/blob/main/README.md) and
  [Node.js Corepack API docs](https://nodejs.org/api/corepack.html) for the variable set, the two
  file-ignored keys, and precedence; [PR #642](https://github.com/nodejs/corepack/pull/642) for the
  feature and its security-adjacent review; [PR #891](https://github.com/nodejs/corepack/pull/891)
  (Corepack 0.36.0) for closest-file loading on every command.
- Upstream risk history: [nodejs/corepack#616](https://github.com/nodejs/corepack/issues/616)
  (registry signing-key rotation; `COREPACK_INTEGRITY_KEYS=0` as the documented verification-skip
  workaround), [#741](https://github.com/nodejs/corepack/issues/741) (enterprise mirror motivation),
  [#884](https://github.com/nodejs/corepack/issues/884) /
  [#885](https://github.com/nodejs/corepack/pull/885) (re-signing registries versus bundled-key
  verification, open).
- Execution verification, 12026-10-12, Corepack 0.36.0 on Node.js 24.21.0: file-set
  `COREPACK_NPM_REGISTRY` redirected the package-manager metadata fetch (fail); real-environment
  `COREPACK_NPM_REGISTRY` overrode the file (pass); `COREPACK_ENV_FILE=0` disabled loading with the
  file present (pass); a file-set `COREPACK_ENV_FILE` was ignored (pass); the loaded file is the
  first `.corepack.env` on the upward path from the working directory, independent of which manifest
  supplies `packageManager` (two cases). Because the builder runs every manager command with the
  package-manager root as its working directory, the preflight inspection point matches the load
  point, and a nested-package `.corepack.env` is unreachable from manager commands.
- Ecosystem census, 12026-10-12. Method: the committed-file population was enumerated with
  filename-scoped GitHub code search (`gh search code` with `--filename .corepack.env`), then
  decomposed by content-restricted queries that pair a key name with the same filename scope (for
  example `COREPACK_NPM_REGISTRY --filename .corepack.env`), while an unrestricted
  `COREPACK_NPM_REGISTRY` query served as a contrast group to locate where that key is actually
  used. A sample of the matched files was fetched and read directly to classify their contents, and
  grep.app was used as a cross-check. Results, reproduced by a second independent run on the same
  day: 15 distinct repositories carry a committed `.corepack.env`, consisting of four independent
  projects and one author's eleven-repository GitHub Actions template family that propagates a
  shared template; the observed keys are `COREPACK_ENABLE_AUTO_PIN=0`,
  `COREPACK_ENABLE_DOWNLOAD_PROMPT=0`, and one project-level bundle also setting
  `COREPACK_DEFAULT_TO_LATEST=0`, `COREPACK_ENABLE_PROJECT_SPEC=1`, and `COREPACK_ENABLE_STRICT=1`.
  The registry-key query returned zero matches in both runs. Registry-mirror usage of
  `COREPACK_NPM_REGISTRY` appears in Dockerfiles and shell environments (for example npmmirror
  configurations), not in committed env files. Limits: GitHub code search covers indexed default
  branches only, so the count is a lower bound rather than a complete census; gitignored or
  differently named env files are invisible to it; and a zero result is bounded evidence of absence
  within the index, not proof of nonexistence. The commonly committed values duplicate pins the
  builder's controlled environment already sets, so the typical real-world file is harmless under
  builder semantics; the enterprise registry-mirror audience (nodejs/corepack#741) is outside the
  github.com-only production profile (ADRs 0027, 0068, 0075).
- Comparable systems: slsa-github-generator performs no Corepack or `.npmrc` neutralization but
  scopes its threat model to provenance non-forgeability ("The content of the source code is out of
  scope."), which does not transfer to this builder's fail-closed contract; pnpm's 2026 advisories
  (CVE-2026-50017, CVE-2026-55180) demonstrate registry-redirection credential-exposure risk and the
  trust-source-aware filtering response.
- Investigated but undecided, per the atomicity convention: an opt-in workflow input to honor the
  file; a dedicated GHES or self-hosted mirror policy (blocked on the structural exclusions in ADRs
  0027, 0068, and 0075); whether the warning also appears in provenance external parameters;
  fixture-corpus coverage for the warning path.
- Revisit triggers: the ADR 0088 rollout completing (Corepack leaves the production build path and
  this policy becomes a candidate for deprecation), or a future ADR admitting GHES or self-hosted
  runners (reopening the mirror-configuration question).
- Pre-existing gaps noted while drafting, not decided here: the current prohibition error falls
  through to the generic verifier-execution-failure diagnostic rather than a dedicated policy
  diagnostic, and the rule has unit-test but no fixture-corpus coverage.
