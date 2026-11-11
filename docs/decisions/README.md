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
sequence currently runs from `0000` through `0112`.

| ADRs                 | Topic                                                | Notes                                                                                                                                                                                                                                     |
| -------------------- | ---------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0000–0012            | Repository foundation and initial development tools  | ADR format, clean repository start, trusted workflow foundation, Go, linting, formatting, Node.js, pnpm, Lefthook, mise.                                                                                                                  |
| 0013–0037            | JS/TS npm package profile                            | Package selection, manifest-first managers, build/pack, workflow entrypoint, OIDC publishing, provenance, initial signing adapter and historical decisions.                                                                               |
| 0038–0052            | GitHub Release asset profile                         | Release asset subjects, buildType URIs, publisher model, producer handoff, sidecar distribution, npm composition and historical decisions.                                                                                                |
| 0053–0054, 0062      | Release manifest and trusted producer policy         | Three-job signing boundary, release manifest predicate URI, trusted producer policy intersection and conflict handling.                                                                                                                   |
| 0055                 | Signing adapter Statement construction               | `actions/attest` custom mode and post-sign Statement checks, later superseded by the Go-native signer.                                                                                                                                    |
| 0056                 | Non-selected lockfile diagnostics                    | Stale lockfile handling under manifest-selected package managers.                                                                                                                                                                         |
| 0057–0060, 0108      | Public npm release interface and publisher authority | Single public entrypoint, minimal `workflow_call` interface, same-repository least-privilege publisher, separated internal authority, removed `release-tag` and `provenance-sidecar` inputs, built-tag derivation and mandatory sidecars. |
| 0061                 | Signed Statement JSON parsing                        | Duplicate JSON member rejection before semantic validation of signed SLSA Statements.                                                                                                                                                     |
| 0063, 0090           | Consumer pnpm/Yarn support boundaries                | Yarn Berry support metadata and Corepack history, consumer pnpm range `[10.0.0, 12.0.0)` and support-table boundary.                                                                                                                      |
| 0064                 | npm provenance subject compatibility                 | npm PURL subject with SHA-512 and SHA-256 over the same tarball bytes.                                                                                                                                                                    |
| 0065                 | ADR lifecycle metadata                               | Closed status grammar, separate relations field and bidirectional traceability.                                                                                                                                                           |
| 0066–0067, 0074–0076 | Common run convergence and mutation policy           | Job-class concurrency, run-identity idempotency, single-job mutation segments, detection-based safety, `queue: max` FIFO, observation preflights and first-mutation classification.                                                       |
| 0068–0069            | Verifier identity and transparency trust             | Immutable builder/source identities, Rekor inclusion, offline verification and governed Sigstore trust root.                                                                                                                              |
| 0070–0071            | Build-environment and builder recording              | Package-manager distributions, initial runner-image descriptor, `builder.version` and `builderDependencies`.                                                                                                                              |
| 0072                 | Release asset run ownership binding                  | Sidecar-first pair binding and custody non-attribution for release assets.                                                                                                                                                                |
| 0073, 0102           | npm publish convergence and read-back classification | Published-attestation run identity, E422 reduced read-back budget, determinate absence and full budget when the version exists.                                                                                                           |
| 0077                 | Windlass provenance signing adapter                  | Go-native exact-byte Sigstore DSSE signing for all profiles, superseding `0035` and `0055`.                                                                                                                                               |
| 0078                 | pnpm settings-only root-package mode                 | Missing `packages` in `pnpm-workspace.yaml` selects root-only mode rather than malformed workspace data.                                                                                                                                  |
| 0079–0080            | Caller source retry and invocation binding           | Tags-only `source-ref`, fixed-pipeline release retries, source policy bound to signed fields and certificate claims as invocation context.                                                                                                |
| 0081                 | npm OIDC exchange response contract                  | Observed success response shape and 15-minute exchange-token lifetime.                                                                                                                                                                    |
| 0082–0084            | Publish-stage npm CLI provisioning                   | Reviewed allowlist, initial pin containing npm/cli#9882, digest-verified npm registry tarball and committed SHA-512.                                                                                                                      |
| 0085, 0098–0099      | Consumer build-stage Node.js runtime                 | Exact Node.js 24 patch and bundled npm pair, SHA-256-verified release tarball, runtime recording in `resolvedDependencies`.                                                                                                               |
| 0086–0087, 0089      | Repository development Node.js and pnpm tooling      | Repository-only mise field resolution, `package.json` pins, `devEngines.runtime`, `devEngines.packageManager` and pnpm 12 development tooling.                                                                                            |
| 0088, 0091           | Consumer package-manager provisioning and selection  | Build-stage pnpm/Yarn registry tarballs replacing Corepack, rollout boundary and uniform consumer selection through `devEngines.packageManager`.                                                                                          |
| 0092–0094            | Descriptor integrity digest grammar                  | Optional declared digests verified fail closed, W3C SRI algorithm tokens, `+algorithm.<hex>` suffixes, SRI base64 rejection and unchanged record model.                                                                                   |
| 0095                 | npm descriptor semantics                             | Declared-version mismatch warnings, npm digest rejection and the separate npm CLI self-enforcement boundary.                                                                                                                              |
| 0096–0097            | Project and packed `.corepack.env` policy            | `COREPACK_ENV_FILE=0`, non-fatal project-file warning and parser-free post-pack rejection.                                                                                                                                                |
| 0100, 0111–0112      | Runner-image evidence and reported identity          | On-disk image metadata, Included Software URI, exact full `image_version`, non-authoritative `deployment_version`, decoded `Operating System.detail`, diagnostic-only `$ImageOS` and offline projection.                                  |
| 0101                 | External CLI stderr hygiene relay                    | Allowlisted subprocess stderr, spec-owned hygiene filter, classification as enrichment and sealed canonical contract.                                                                                                                     |
| 0103–0107, 0109      | Invocation identity and external parameters          | Common workflow identity, builder triplet removal, `invocation-source` and `built-source`, source consolidation, `event_name` in `internalParameters.github`, package-manifest descriptor, actual interface inputs and recompute rules.   |
| 0110                 | Non-runner dependency media types                    | Deterministic `mediaType` for concrete artifacts, untyped source descriptors and absent `downloadLocation`.                                                                                                                               |

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
traceability defect and is repaired by adding the missing reverse entry. No new ADR is required for
the repair alone, and body text is never edited.

After acceptance, only the `status` and `relations` frontmatter fields may change. See
[ADR 0065](0065-use-closed-status-grammar-with-separate-relations-field.md) for the full decision.

## ADR traceability

Every ADR is either mapped to a spec, marked as tooling-only, or marked as superseded, deprecated,
etc. Superseded or deprecated ADRs are historical context and must not drive new specification or
implementation work. Partial supersession and amendment relations (ADR 0065) do not move an ADR out
of this table; they are recorded in each ADR's `relations` frontmatter field.

### Accepted ADRs

| ADR  | Decision                                                                                                          | Spec mapping                                                                                                                                                                                     |
| ---- | ----------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 0000 | Use Markdown Architectural Decision Records                                                                       | Process; no runtime spec needed                                                                                                                                                                  |
| 0001 | Start slsa-builder as a clean repository                                                                          | Foundation; no runtime spec needed                                                                                                                                                               |
| 0002 | Extensible trusted reusable workflow foundation                                                                   | Core profile contract, SLSA provenance, verification policy                                                                                                                                      |
| 0003 | Thin core with profile-owned reusable workflows                                                                   | Core profile contract, SLSA provenance, verification policy                                                                                                                                      |
| 0004 | Go as primary implementation language                                                                             | Core profile contract                                                                                                                                                                            |
| 0005 | Dedicated linter toolchain                                                                                        | Tooling-only                                                                                                                                                                                     |
| 0006 | golangci-lint as Go linter runner                                                                                 | Tooling-only                                                                                                                                                                                     |
| 0007 | ShellCheck for shell glue                                                                                         | Tooling-only                                                                                                                                                                                     |
| 0008 | Dedicated formatters                                                                                              | Tooling-only                                                                                                                                                                                     |
| 0009 | Node.js as development tool runtime                                                                               | Tooling-only; core contract notes trusted logic boundary                                                                                                                                         |
| 0010 | pnpm for Node.js development tooling                                                                              | Tooling-only                                                                                                                                                                                     |
| 0011 | Lefthook for local git hook orchestration                                                                         | Tooling-only                                                                                                                                                                                     |
| 0012 | mise as unified development-tool runtime                                                                          | Tooling-only                                                                                                                                                                                     |
| 0013 | Scope initial JS/TS profile to npm packages                                                                       | JS/TS npm profile, composition spec                                                                                                                                                              |
| 0014 | Support npm, pnpm, and Yarn for initial build stages                                                              | JS/TS npm build and pack                                                                                                                                                                         |
| 0015 | Manifest-first package manager selection                                                                          | JS/TS npm build and pack                                                                                                                                                                         |
| 0016 | Corepack for pnpm and Yarn build stages                                                                           | JS/TS npm build and pack                                                                                                                                                                         |
| 0017 | Explicit package manager version enforcement                                                                      | JS/TS npm build and pack                                                                                                                                                                         |
| 0018 | Publish one JS/TS package per profile run                                                                         | JS/TS npm profile, build and pack                                                                                                                                                                |
| 0019 | Validate package metadata through packed artifacts                                                                | JS/TS npm build and pack                                                                                                                                                                         |
| 0022 | `js-ts-npm-package-slsa3.yml` workflow entrypoint                                                                 | JS/TS npm profile                                                                                                                                                                                |
| 0023 | `package-directory` as required package selector                                                                  | JS/TS npm profile, build and pack                                                                                                                                                                |
| 0024 | OIDC trusted publishing without publish secrets                                                                   | JS/TS npm profile, provenance and publish                                                                                                                                                        |
| 0025 | Return package identity and tarball digest outputs                                                                | JS/TS npm provenance and publish                                                                                                                                                                 |
| 0026 | Document supported release caller patterns and runtime guards                                                     | JS/TS npm profile                                                                                                                                                                                |
| 0027 | GitHub-Hosted Ubuntu 24.04 and Node.js 24 runtime                                                                 | JS/TS npm profile, build and pack                                                                                                                                                                |
| 0028 | SHA-pinned reusable workflow builder identity                                                                     | Identity and build types, common provenance, release manifest, verification policy                                                                                                               |
| 0029 | Windlass-generated SLSA provenance for npm publish                                                                | Common provenance, JS/TS npm provenance and publish, verification policy                                                                                                                         |
| 0030 | Accept registry URL while guaranteeing only npmjs semantics                                                       | JS/TS npm profile, provenance and publish, verification policy                                                                                                                                   |
| 0031 | Sigstore-signed in-toto release manifest                                                                          | Identity and build types, release manifest                                                                                                                                                       |
| 0032 | Constrain manual dispatch releases to version tags                                                                | JS/TS npm profile                                                                                                                                                                                |
| 0033 | Run build script only when declared                                                                               | JS/TS npm build and pack                                                                                                                                                                         |
| 0034 | Do not support private dependency credentials                                                                     | JS/TS npm profile                                                                                                                                                                                |
| 0035 | `actions/attest` as initial Sigstore signing adapter                                                              | Core profile contract, common provenance, JS/TS npm provenance and publish, release manifest                                                                                                     |
| 0036 | Three-job digest-verified publish graph                                                                           | JS/TS npm provenance and publish, verification policy                                                                                                                                            |
| 0037 | Define initial verification deliverables                                                                          | Verification policy and fixtures                                                                                                                                                                 |
| 0039 | Scope release asset profile to one asset per run                                                                  | GitHub Release asset publisher                                                                                                                                                                   |
| 0042 | Use acquired domains for buildType URIs                                                                           | Core profile contract, identity and build types                                                                                                                                                  |
| 0043 | Upload release assets to existing releases                                                                        | GitHub Release asset publisher                                                                                                                                                                   |
| 0045 | Use release asset name as SLSA subject name                                                                       | GitHub Release asset publisher                                                                                                                                                                   |
| 0046 | Keep checksums and SBOMs out of subject digest                                                                    | GitHub Release asset publisher                                                                                                                                                                   |
| 0048 | Make linked artifacts storage records explicit opt-in                                                             | GitHub Release asset publisher                                                                                                                                                                   |
| 0049 | Separate artifact production from GitHub Release asset publication                                                | Identity and build types, GitHub Release asset publisher, verification policy                                                                                                                    |
| 0050 | Define producer-to-publisher handoff contract                                                                     | GitHub Release asset publisher, verification policy                                                                                                                                              |
| 0051 | Distribute producer provenance with release assets                                                                | GitHub Release asset publisher, verification policy                                                                                                                                              |
| 0052 | Compose npm package tarball producer with release asset publisher                                                 | Composition spec, verification policy                                                                                                                                                            |
| 0053 | Three-job release manifest signing boundary                                                                       | Release manifest                                                                                                                                                                                 |
| 0054 | Use `slsa-builder.dev` release manifest predicate URI                                                             | Release manifest, verification policy                                                                                                                                                            |
| 0055 | `actions/attest` custom mode for Statement construction                                                           | Common provenance, JS/TS npm provenance and publish                                                                                                                                              |
| 0056 | Treat non-selected lockfiles as stale diagnostics                                                                 | JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy                                                                                                                  |
| 0057 | Provide public npm release-asset mode                                                                             | Composition spec, JS/TS npm provenance and publish, GitHub Release asset publisher                                                                                                               |
| 0058 | Define GitHub Release asset publisher authority boundary                                                          | GitHub Release asset publisher, composition spec, verification policy                                                                                                                            |
| 0059 | Define public npm release-asset mode interface                                                                    | Composition spec, JS/TS npm provenance and publish, GitHub Release asset publisher                                                                                                               |
| 0060 | Unify npm profile public entrypoint with release-asset mode                                                       | JS/TS npm profile, composition spec, verification policy                                                                                                                                         |
| 0061 | Reject duplicate JSON members in signed SLSA Statements                                                           | Common provenance, JS/TS npm provenance and publish, verification policy                                                                                                                         |
| 0062 | Intersect trusted producer policies                                                                               | Release manifest, GitHub Release asset publisher, verification policy                                                                                                                            |
| 0063 | Limit Yarn support to Berry v4 with Corepack metadata                                                             | JS/TS npm build and pack                                                                                                                                                                         |
| 0064 | Use npm PURL subject with SHA-512 and SHA-256 digests                                                             | Common provenance, JS/TS npm specs, composition and publisher specs, verification policy                                                                                                         |
| 0065 | Use a closed status grammar and a separate relations field                                                        | Process; no runtime spec needed                                                                                                                                                                  |
| 0066 | Serialize release mutations with job-class concurrency                                                            | GitHub Release asset publisher, JS/TS npm provenance and publish, release manifest, JS/TS npm package profile, verification policy                                                               |
| 0067 | Converge repeated runs within run identity                                                                        | Release manifest, GitHub Release asset publisher, JS/TS npm provenance and publish, verification policy                                                                                          |
| 0068 | Bind verification to immutable builder and source identities                                                      | Verification policy, identity and build types, release manifest, JS/TS npm provenance and publish                                                                                                |
| 0069 | Require Rekor transparency and govern the Sigstore trust root                                                     | Verification policy, common provenance, release manifest                                                                                                                                         |
| 0070 | Record package manager distributions and runner image in resolvedDependencies                                     | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                                 |
| 0071 | Activate builder.version and builderDependencies for platform components                                          | SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                                                                                           |
| 0072 | Use sidecar-first pair binding for release asset run ownership                                                    | GitHub Release asset publisher, npm-to-release-asset composition, verification policy and fixtures                                                                                               |
| 0073 | Require published-attestation run identity for npm same-run convergence                                           | JS/TS npm provenance and publish, verification policy and fixtures                                                                                                                               |
| 0074 | Use single-job mutation segments with detection-based cross-run safety                                            | GitHub Release asset publisher, release manifest, npm-to-release-asset composition, verification policy and fixtures                                                                             |
| 0075 | Queue mutation segment contenders with queue: max                                                                 | GitHub Release asset publisher, JS/TS npm provenance and publish, release manifest, verification policy and fixtures                                                                             |
| 0076 | Use observation preflights and first-mutation classification                                                      | GitHub Release asset publisher, JS/TS npm package profile, verification policy and fixtures                                                                                                      |
| 0077 | Use a Go-native Sigstore DSSE signer for Windlass provenance signing                                              | Core profile contract, SLSA provenance v1, JS/TS npm package profile, JS/TS npm provenance and publish, release asset publisher, release manifest, composition, verification policy              |
| 0078 | Treat settings-only pnpm-workspace.yaml as standalone root package mode                                           | JS/TS npm build and pack, verification policy and fixtures                                                                                                                                       |
| 0079 | Support a tags-only caller-specified build source ref for release retries across profiles                         | Core profile contract, JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                             |
| 0080 | Bind source identity policy to signed provenance fields, certificate claims as invocation context                 | Core profile contract, SLSA provenance v1, identity and build types, JS/TS npm provenance and publish, verification policy and fixtures                                                          |
| 0081 | Pin the npm OIDC exchange response contract to the observed shape, correct token lifetime                         | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                                    |
| 0082 | Pin the publish-stage npm CLI version with integrity-verified provisioning and a reviewed allowlist               | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                                    |
| 0083 | Defer the npm M1 publish remediation to the upstream provenance-file fix                                          | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                                    |
| 0084 | Provision the publish-stage npm CLI from a digest-verified registry tarball                                       | JS/TS npm package profile, JS/TS npm provenance and publish, verification policy and fixtures                                                                                                    |
| 0085 | Pin the Node.js 24 patch version and assert the expected bundled npm pair                                         | JS/TS npm package profile, JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy and fixtures                                                                          |
| 0086 | Provision pnpm through mise packageManager-field resolution instead of Corepack                                   | Tooling-only                                                                                                                                                                                     |
| 0087 | Adopt pnpm 12 for Node.js development tooling                                                                     | Tooling-only                                                                                                                                                                                     |
| 0088 | Provision build-stage pnpm and Yarn from digest-verified npm registry tarballs                                    | JS/TS npm build and pack, verification policy and fixtures (only the Corepack-window support boundary is specified; the registry-tarball mechanism is deferred to follow-on spec work, B01/#106) |
| 0089 | Provision the development Node.js runtime through mise package.json field resolution                              | Tooling-only                                                                                                                                                                                     |
| 0090 | Extend the supported pnpm range to the 10.x line                                                                  | JS/TS npm build and pack, verification policy and fixtures (Corepack-window pnpm support-floor revision)                                                                                         |
| 0091 | Accept devEngines.packageManager as a uniform selection source                                                    | JS/TS npm build and pack (package manager selection), verification policy and fixtures                                                                                                           |
| 0092 | Accept optional integrity digests in package-manager descriptors                                                  | JS/TS npm build and pack (descriptor grammar, fail-closed digest reconciliation), verification policy and fixtures                                                                               |
| 0093 | Pin descriptor digest algorithms to the W3C SRI set                                                               | JS/TS npm build and pack (descriptor digest grammar), verification policy and fixtures                                                                                                           |
| 0094 | Pin the descriptor digest format to a hex suffix                                                                  | JS/TS npm build and pack (descriptor digest grammar), verification policy and fixtures                                                                                                           |
| 0095 | Accept npm version declarations with a mismatch warning and reject npm digests                                    | JS/TS npm build and pack (npm descriptor semantics), verification policy and fixtures, README                                                                                                    |
| 0096 | Neutralize the project .corepack.env with a pinned COREPACK_ENV_FILE=0 and warn on its presence                   | JS/TS npm build and pack (credential-configuration preflight), verification policy and fixtures                                                                                                  |
| 0097 | Reject a packed .corepack.env at the post-pack artifact gate                                                      | JS/TS npm build and pack (post-pack artifact gate), verification policy and fixtures                                                                                                             |
| 0098 | Provision the build-stage Node.js runtime from a digest-verified release tarball                                  | JS/TS npm package profile, JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy and fixtures                                                                          |
| 0099 | Record the digest-verified Node.js runtime in resolvedDependencies                                                | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                                 |
| 0100 | Record runner image identity from on-disk metadata with the control-plane deployment version as separate evidence | JS/TS npm build and pack (runner image capture), verification policy and fixtures                                                                                                                |
| 0101 | Relay external CLI stderr to logs under hygiene bounds, with template classification as enrichment                | Core profile contract (external-process operational diagnostics), JS/TS npm provenance and publish (best-effort operational diagnostics)                                                         |
| 0102 | Classify npm publish E422 rejections as determinate pre-commit failures under a reduced read-back budget          | JS/TS npm provenance and publish, verification policy and fixtures                                                                                                                               |
| 0103 | Define external workflow parameters as common invocation identity                                                 | Core profile contract, SLSA provenance v1, identity and build types, JS/TS npm provenance and publish, verification policy and fixtures                                                          |
| 0104 | Remove redundant external-parameter builder record                                                                | Core profile contract, SLSA provenance v1, identity and build types, JS/TS npm provenance and publish, verification policy and fixtures                                                          |
| 0105 | Record invocation and built source as role-named resolved dependencies                                            | Core profile contract, SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                                                                    |
| 0106 | Consolidate external source parameters onto workflow and descriptor bindings                                      | Core profile contract, SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                                                                    |
| 0107 | Record invocation event metadata in internalParameters.github only                                                | Core profile contract, SLSA provenance v1, JS/TS npm provenance and publish, verification policy and fixtures                                                                                    |
| 0108 | Remove redundant npm release-asset inputs                                                                         | JS/TS npm package profile, JS/TS npm provenance and publish, npm-to-release-asset composition, GitHub Release asset publisher, verification policy and fixtures                                  |
| 0109 | Minimize external parameters to actual interface inputs                                                           | Core profile contract, SLSA provenance v1, JS/TS npm package profile, JS/TS npm build and pack, JS/TS npm provenance and publish, verification policy and fixtures                               |
| 0110 | Record deterministic media types for non-runner dependencies                                                      | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                                 |
| 0111 | Record runner-image reported identity with the Included Software URI and full image version                       | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                                 |
| 0112 | Record runner-image operating system from on-disk metadata                                                        | SLSA provenance v1, JS/TS npm provenance and publish, JS/TS npm build and pack, verification policy and fixtures                                                                                 |

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
4. **Use Holocene Era dates.** ADR front matter dates use the `12026-07-08` style.
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
- [ ] The ADR uses MADR 4.0.0 structure and Holocene Era date format.
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
