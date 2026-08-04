# Architecture Decision Records

<div align="center">

English | [한국어](README.ko.md)

</div>

Architecture decision records (ADRs) for `slsa-builder`. This directory holds the **why** of the
system: accepted decisions, rejected alternatives, trade-offs, and consequences. The exact
observable behavior selected by these decisions is specified in
[`docs/architecture/`](../architecture/).

> [!Note]  
> This project follows Spec-Driven Development (SDD): decide first in ADRs, specify exact behavior
> in architecture docs, then implement against the specifications.

## SDD model

| Layer          | Location             | Answers                  | May contain                                        |
| -------------- | -------------------- | ------------------------ | -------------------------------------------------- |
| Decisions      | `docs/decisions/`    | Why did we choose this?  | Rationale, trade-offs, consequences                |
| Specifications | `docs/architecture/` | What must the system do? | Contracts, schemas, invariants, examples, fixtures |
| Implementation | source tree          | How does it work?        | Code, workflows, tests, fixtures                   |

Implementation may not contradict an accepted ADR. If a spec draft reveals a contradiction with an
accepted ADR, stop and write a new ADR rather than editing the accepted ADR body.

## ADR inventory

ADR files are MADR 4.0.0 documents with sequential four-digit numbers and kebab-case titles. The
sequence currently runs from `0000` through `0074`.

| Range     | Topic                                       | Notes                                                            |
| --------- | ------------------------------------------- | ---------------------------------------------------------------- |
| 0000–0012 | Repository foundation and development tools | ADR format, repository start, Go, linting, formatting, tooling.  |
| 0013–0037 | JS/TS npm package profile                   | Package selection, build/pack, OIDC publishing, provenance.      |
| 0038–0052 | GitHub Release asset profile                | Release asset subjects, publisher model, sidecar distribution.   |
| 0053–0054 | Release manifest metadata                   | Signing boundary and release manifest predicate URI.             |
| 0055      | Signing adapter Statement construction      | `actions/attest` custom mode and post-sign Statement checks.     |
| 0056      | Non-selected lockfile diagnostics           | Stale lockfile handling under manifest-selected managers.        |
| 0057      | Public npm release-asset mode               | Single npm entrypoint plus advanced composition primitives.      |
| 0058      | Release publisher authority boundary        | Same-repository target and least-privilege mutation topology.    |
| 0059      | Public release-asset mode interface         | Minimal user-intent `workflow_call` surface and fail-closed API. |
| 0060      | Unified npm public entrypoint               | One public API with separated internal authority boundaries.     |
| 0061      | Duplicate JSON member rejection             | Strict signed SLSA Statement parsing before semantic validation. |
| 0062      | Trusted producer policy intersection        | Conflict handling for manifest and explicit verifier policy.     |
| 0063      | Yarn Berry v4+ support boundary             | Corepack and `packageManager` requirements for Yarn support.     |
| 0064      | npm provenance subject compatibility        | npm PURL subject with SHA-512 and SHA-256 tarball digests.       |
| 0065      | ADR lifecycle metadata                      | Closed status grammar and relations field.                       |
| 0066      | Release mutation run ownership              | Job-class concurrency and serialized mutation segment.           |
| 0067      | Repeated run convergence                    | Run-identity idempotency, outcome states, binding proofs.        |
| 0068      | Verifier identity binding                   | Immutable builder and source identities for verification.        |
| 0069      | Transparency and trust root policy          | Rekor inclusion, offline verification, trust root governance.    |
| 0070–0071 | Provenance build-environment recording      | Package-manager distributions, runner image, builder fields.     |
| 0072      | Release asset run ownership binding         | Sidecar-first pair binding and custody non-attribution.          |
| 0073      | npm same-run attestation binding            | Published-attestation run identity required for npm convergence. |
| 0074      | Mutation segment atomicity                  | Single-job segments and detection-based cross-run safety.        |

## ADR status and relations

ADR 0065 defines the lifecycle and traceability metadata grammar for every ADR in this directory.

### Status grammar

The `status` frontmatter field must use exactly one of these forms:

```text
proposed
rejected
accepted
deprecated
superseded by ADR-XXXX
```

Composite or prose status values (for example `accepted; partially updated by ...` or
`amended by ...`) are invalid. `deprecated` means the decision is no longer recommended and no
replacing ADR exists or is required. `superseded by ADR-XXXX` means the named ADR replaces the
decision in full.

### Relations field

ADR-to-ADR relationships are recorded in the frontmatter `relations` field, not in `status`:

```yaml
relations:
  - type: partially-superseded-by
    target: ADR-0049
    scope: "Release asset profile buildType URI assignment and related confirmation criteria"
```

Each entry has a `type`, a `target` ADR number, and a `scope` (required for partial supersession and
amendment relations; identifies the affected clauses). The relation vocabulary is closed:

| Forward (newer ADR)    | Reverse (earlier ADR)     | Meaning                                        | Earlier ADR status effect      |
| ---------------------- | ------------------------- | ---------------------------------------------- | ------------------------------ |
| `supersedes`           | `superseded-by`           | The newer ADR replaces the earlier one in full | Changes to `superseded by ...` |
| `partially-supersedes` | `partially-superseded-by` | Named clauses are replaced; the rest stands    | Remains `accepted`             |
| `amends`               | `amended-by`              | Details are narrowed, adjusted, or excepted    | Remains `accepted`             |
| `see-also`             | `see-also`                | Informational cross-reference                  | None                           |

Use `partially-supersedes` when an implementer may no longer follow the earlier clause as written;
use `amends` when the earlier clause still governs and the newer ADR only qualifies it. Resolve
ambiguous cases in favor of `partially-supersedes`.

Relation edges are bidirectional: when a new ADR declares `supersedes`, `partially-supersedes`, or
`amends` against a target, the same change must add the matching reverse entry to the target ADR's
`relations` field. A new ADR must enumerate every accepted ADR it affects. An omission is a
traceability defect and is repaired by adding the missing reverse entry — no new ADR is required for
the repair alone, and body text is never edited.

After acceptance, only the `status` and `relations` frontmatter fields may change. See
[ADR 0065](0065-use-closed-status-grammar-with-separate-relations-field.md) for the full decision.

## ADR traceability

Every ADR is either mapped to a spec, marked as tooling-only, or marked as superseded, deprecated,
etc. Superseded or deprecated ADRs are historical context and must not drive new specification or
implementation work. Partial supersession and amendment relations (ADR 0065) do not move an ADR out
of this table; they are recorded in each ADR's `relations` frontmatter field.

### Accepted ADRs

| ADR  | Decision                                                                      | Spec mapping                                                                                                                       |
| ---- | ----------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| 0000 | Use Markdown Architectural Decision Records                                   | Process; no runtime spec needed                                                                                                    |
| 0001 | Start slsa-builder as a clean repository                                      | Foundation; no runtime spec needed                                                                                                 |
| 0002 | Extensible trusted reusable workflow foundation                               | Core profile contract, SLSA provenance, verification policy                                                                        |
| 0003 | Thin core with profile-owned reusable workflows                               | Core profile contract, SLSA provenance, verification policy                                                                        |
| 0004 | Go as primary implementation language                                         | Core profile contract                                                                                                              |
| 0005 | Dedicated linter toolchain                                                    | Tooling-only                                                                                                                       |
| 0006 | golangci-lint as Go linter runner                                             | Tooling-only                                                                                                                       |
| 0007 | ShellCheck for shell glue                                                     | Tooling-only                                                                                                                       |
| 0008 | Dedicated formatters                                                          | Tooling-only                                                                                                                       |
| 0009 | Node.js as development tool runtime                                           | Tooling-only; core contract notes trusted logic boundary                                                                           |
| 0010 | pnpm for Node.js development tooling                                          | Tooling-only                                                                                                                       |
| 0011 | Lefthook for local git hook orchestration                                     | Tooling-only                                                                                                                       |
| 0012 | mise as unified development-tool runtime                                      | Tooling-only                                                                                                                       |
| 0013 | Scope initial JS/TS profile to npm packages                                   | JS/TS npm profile, composition spec                                                                                                |
| 0014 | Support npm, pnpm, and Yarn for initial build stages                          | JS/TS npm build and pack                                                                                                           |
| 0015 | Manifest-first package manager selection                                      | JS/TS npm build and pack                                                                                                           |
| 0016 | Corepack for pnpm and Yarn build stages                                       | JS/TS npm build and pack                                                                                                           |
| 0017 | Explicit package manager version enforcement                                  | JS/TS npm build and pack                                                                                                           |
| 0018 | Publish one JS/TS package per profile run                                     | JS/TS npm profile, build and pack                                                                                                  |
| 0019 | Validate package metadata through packed artifacts                            | JS/TS npm build and pack                                                                                                           |
| 0022 | `js-ts-npm-package-slsa3.yml` workflow entrypoint                             | JS/TS npm profile                                                                                                                  |
| 0023 | `package-directory` as required package selector                              | JS/TS npm profile, build and pack                                                                                                  |
| 0024 | OIDC trusted publishing without publish secrets                               | JS/TS npm profile, provenance and publish                                                                                          |
| 0025 | Return package identity and tarball digest outputs                            | JS/TS npm provenance and publish                                                                                                   |
| 0026 | Document supported release caller patterns and runtime guards                 | JS/TS npm profile                                                                                                                  |
| 0027 | GitHub-Hosted Ubuntu 24.04 and Node.js 24 runtime                             | JS/TS npm profile, build and pack                                                                                                  |
| 0028 | SHA-pinned reusable workflow builder identity                                 | Identity and build types, common provenance, release manifest, verification policy                                                 |
| 0029 | Windlass-generated SLSA provenance for npm publish                            | Common provenance, JS/TS npm provenance and publish, verification policy                                                           |
| 0030 | Accept registry URL while guaranteeing only npmjs semantics                   | JS/TS npm profile, provenance and publish, verification policy                                                                     |
| 0031 | Sigstore-signed in-toto release manifest                                      | Identity and build types, release manifest                                                                                         |
| 0032 | Constrain manual dispatch releases to version tags                            | JS/TS npm profile                                                                                                                  |
| 0033 | Run build script only when declared                                           | JS/TS npm build and pack                                                                                                           |
| 0034 | Do not support private dependency credentials                                 | JS/TS npm profile                                                                                                                  |
| 0035 | `actions/attest` as initial Sigstore signing adapter                          | Core profile contract, common provenance, JS/TS npm provenance and publish, release manifest                                       |
| 0036 | Three-job digest-verified publish graph                                       | JS/TS npm provenance and publish, verification policy                                                                              |
| 0037 | Define initial verification deliverables                                      | Verification policy and fixtures                                                                                                   |
| 0039 | Scope release asset profile to one asset per run                              | GitHub Release asset publisher                                                                                                     |
| 0042 | Use acquired domains for buildType URIs                                       | Core profile contract, identity and build types                                                                                    |
| 0043 | Upload release assets to existing releases                                    | GitHub Release asset publisher                                                                                                     |
| 0045 | Use release asset name as SLSA subject name                                   | GitHub Release asset publisher                                                                                                     |
| 0046 | Keep checksums and SBOMs out of subject digest                                | GitHub Release asset publisher                                                                                                     |
| 0048 | Make linked artifacts storage records explicit opt-in                         | GitHub Release asset publisher                                                                                                     |
| 0049 | Separate artifact production from GitHub Release asset publication            | Identity and build types, GitHub Release asset publisher, verification policy                                                      |
| 0050 | Define producer-to-publisher handoff contract                                 | GitHub Release asset publisher, verification policy                                                                                |
| 0051 | Distribute producer provenance with release assets                            | GitHub Release asset publisher, verification policy                                                                                |
| 0052 | Compose npm package tarball producer with release asset publisher             | Composition spec, verification policy                                                                                              |
| 0053 | Three-job release manifest signing boundary                                   | Release manifest                                                                                                                   |
| 0054 | Use `slsa-builder.dev` release manifest predicate URI                         | Release manifest, verification policy                                                                                              |
| 0055 | `actions/attest` custom mode for Statement construction                       | Common provenance, JS/TS npm provenance and publish                                                                                |
| 0056 | Treat non-selected lockfiles as stale diagnostics                             | JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy                                                    |
| 0057 | Provide public npm release-asset mode                                         | Composition spec, JS/TS npm provenance and publish, GitHub Release asset publisher                                                 |
| 0058 | Define GitHub Release asset publisher authority boundary                      | GitHub Release asset publisher, composition spec, verification policy                                                              |
| 0059 | Define public npm release-asset mode interface                                | Composition spec, JS/TS npm provenance and publish, GitHub Release asset publisher                                                 |
| 0060 | Unify npm profile public entrypoint with release-asset mode                   | JS/TS npm profile, composition spec, verification policy                                                                           |
| 0061 | Reject duplicate JSON members in signed SLSA Statements                       | Common provenance, JS/TS npm provenance and publish, verification policy                                                           |
| 0062 | Intersect trusted producer policies                                           | Release manifest, GitHub Release asset publisher, verification policy                                                              |
| 0063 | Limit Yarn support to Berry v4 with Corepack metadata                         | JS/TS npm build and pack                                                                                                           |
| 0064 | Use npm PURL subject with SHA-512 and SHA-256 digests                         | Common provenance, JS/TS npm specs, composition and publisher specs, verification policy                                           |
| 0065 | Use a closed status grammar and a separate relations field                    | Process; no runtime spec needed                                                                                                    |
| 0066 | Serialize release mutations with job-class concurrency                        | GitHub Release asset publisher, JS/TS npm provenance and publish, release manifest, JS/TS npm package profile, verification policy |
| 0067 | Converge repeated runs within run identity                                    | Release manifest, GitHub Release asset publisher, JS/TS npm provenance and publish, verification policy                            |
| 0068 | Bind verification to immutable builder and source identities                  | Verification policy, identity and build types, release manifest, JS/TS npm provenance and publish                                  |
| 0069 | Require Rekor transparency and govern the Sigstore trust root                 | Verification policy, common provenance, release manifest                                                                           |
| 0070 | Record package manager distributions and runner image in resolvedDependencies | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                   |
| 0071 | Activate builder.version and builderDependencies for platform components      | SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                             |
| 0072 | Use sidecar-first pair binding for release asset run ownership                | GitHub Release asset publisher, npm-to-release-asset composition, verification policy and fixtures                                 |
| 0073 | Require published-attestation run identity for npm same-run convergence       | JS/TS npm provenance and publish, verification policy and fixtures                                                                 |
| 0074 | Use single-job mutation segments with detection-based cross-run safety        | GitHub Release asset publisher, release manifest, npm-to-release-asset composition, verification policy and fixtures               |

### Superseded or deprecated ADRs (historical only)

| ADR  | Superseded by | Reason                                                                   |
| ---- | ------------- | ------------------------------------------------------------------------ |
| 0020 | 0028          | Reusable workflow ref identity replaced by SHA-pinned identity           |
| 0021 | 0042          | Profile-specific buildType URIs replaced by acquired-domain URIs         |
| 0038 | 0049          | Release asset provenance model changed from builder to distributor       |
| 0040 | 0049          | `github-release-asset-slsa3.yml` entrypoint removed from production path |
| 0041 | 0042          | Release asset buildType URI replaced by acquired-domain namespace        |
| 0044 | 0049          | Three-job release asset build graph replaced by publisher model          |
| 0047 | 0051          | Canonical attestation storage replaced by sidecar distribution model     |

## ADR writing rules

1. **One decision per ADR.** Keep the decision narrow enough to supersede or reference later.
2. **Use the next number.** Continue the sequential `0000-title.md` naming scheme.
3. **Use MADR 4.0.0.** Preserve the established sections unless the decision clearly does not need
   one.
4. **Use Human Era dates.** ADR front matter dates use the `12026-07-08` style.
5. **Keep accepted ADRs immutable.** After acceptance, update only the `status` and `relations`
   frontmatter fields; write a new ADR for changed decisions.
6. **Use the closed status grammar.** `status` must be exactly one of `proposed`, `rejected`,
   `accepted`, `deprecated`, or `superseded by ADR-XXXX` (ADR 0065).
7. **Declare relations, both directions.** If the new ADR supersedes, partially supersedes, or
   amends an earlier ADR, declare the forward entry in its own `relations` field, add the reverse
   entry to each target, and write a `scope` for partial and amendment relations. Enumerate every
   affected predecessor; omissions are traceability defects.
8. **Update traceability.** Add new ADRs to the inventory above and update architecture spec links
   when they define observable behavior.

## How to add a new ADR

1. Copy the structure from an existing ADR or from
   `0000-use-markdown-architectural-decision-records.md`.
2. Assign the next four-digit number and a kebab-case title.
3. Record the decision context, options, outcome, consequences, and confirmation criteria.
4. Fill in `status` using the closed grammar and declare any `relations` entries, including the
   reverse edges on target ADRs.
5. Add the ADR to the traceability table above.
6. Update affected files in [`docs/architecture/`](../architecture/) when the decision changes
   observable behavior.
7. Run the documentation quality commands before submitting.

## Verification checklist for ADR PRs

- [ ] The ADR number is sequential and the title is kebab-case.
- [ ] The ADR uses MADR 4.0.0 structure and Human Era date format.
- [ ] Accepted ADR bodies were not edited retroactively; only `status` and `relations` changed.
- [ ] `status` uses the closed grammar (no composite or prose values).
- [ ] Relation types, targets, and required `scope` values follow ADR 0065, and every forward
      relation has its reverse edge on the target ADR.
- [ ] The traceability table maps the ADR to specs, tooling-only, or historical status.
- [ ] Architecture specs were updated when the ADR changes observable behavior.
- [ ] Formatting passes:

  ```bash
  pnpm exec prettier --check "docs/**/*.md"
  pnpm exec markdownlint-cli2 "docs/**/*.md"
  ```
