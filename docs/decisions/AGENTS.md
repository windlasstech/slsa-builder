# ADR Knowledge Base

## OVERVIEW

Architecture decision records for the SLSA builder. Each ADR is a MADR 4.0.0 document with a
sequential four-digit number and a kebab-case title. The sequence currently runs from `0000` through
`0112`.

## STRUCTURE

```text
docs/decisions/
├── 0000-use-markdown-architectural-decision-records.md  # MADR template
├── 0001-start-slsa-builder-as-clean-repository.md
├── 0002-use-extensible-trusted-reusable-workflow-foundation.md
├── ...                                                  # 0003–0082
├── 0083-defer-npm-m1-publish-remediation-to-the-upstream-provenance-file-fix.md
├── 0084-provision-the-publish-stage-npm-cli-from-a-digest-verified-registry-tarball.md
├── 0085-pin-the-node-24-patch-version-and-assert-the-expected-bundled-npm-pair.md
├── 0086-provision-pnpm-through-mise-packagemanager-field-resolution-instead-of-corepack.md
├── 0087-adopt-pnpm-12-for-node-js-development-tooling.md
├── 0088-provision-build-stage-pnpm-and-yarn-from-digest-verified-registry-tarballs.md
├── 0089-provision-the-development-node-js-runtime-through-mise-package-json-field-resolution.md
├── 0090-extend-the-supported-pnpm-range-to-the-10x-line.md
├── 0091-accept-devengines-package-manager-as-a-uniform-selection-source.md
├── 0092-accept-optional-integrity-digests-in-package-manager-descriptors.md
├── 0093-pin-descriptor-digest-algorithms-to-the-sri-set.md
├── 0094-pin-descriptor-digest-format-to-a-hex-suffix.md
├── 0095-accept-npm-version-declarations-with-mismatch-warning-and-reject-npm-digests.md
├── 0096-neutralize-project-corepack-env-via-kill-switch-pin-and-warn-on-presence.md
├── 0097-reject-a-packed-corepack-env-at-the-post-pack-artifact-gate.md
├── 0098-provision-the-build-stage-nodejs-runtime-from-a-digest-verified-release-tarball.md
├── 0099-record-the-digest-verified-nodejs-runtime-in-resolveddependencies.md
├── 0100-record-runner-image-identity-from-on-disk-metadata-with-the-control-plane-deployment-version-as-separate-evidence.md
├── 0101-relay-external-cli-stderr-to-logs-under-hygiene-bounds-with-classification-as-enrichment.md
├── 0102-classify-npm-e422-publish-rejection-as-determinate-under-reduced-readback-budget.md
├── 0103-define-external-workflow-parameters-as-common-invocation-identity.md
├── 0104-remove-redundant-external-parameter-builder-record.md
├── 0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md
├── 0106-consolidate-external-source-parameters-onto-workflow-and-descriptor-bindings.md
├── 0107-record-invocation-event-metadata-in-internalparameters-github.md
├── 0108-remove-redundant-npm-release-asset-inputs.md
├── 0109-minimize-external-parameters-to-actual-interface-inputs.md
├── 0110-record-deterministic-media-types-for-non-runner-dependencies.md
├── 0111-record-runner-image-reported-identity-with-included-software-uri-and-full-image-version.md
├── 0112-record-runner-image-operating-system-from-on-disk-metadata.md
├── README.md / README.ko.md
└── AGENTS.md
```

ADR numbering is sequential from `0000` through `0112`. See the WHERE TO LOOK table below for topic
groupings.

Live operational evidence (the npm M1 dogfood records and verification procedure in `docs/dogfood/`)
confirms ADR/spec behavior but is not decision material and does not live here.

## WHERE TO LOOK

| Topic                             | ADR                                                        | Notes                                                                                                                                                                                    |
| --------------------------------- | ---------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Why the repo exists               | `0001`                                                     | Clean-repository foundation.                                                                                                                                                             |
| Trusted workflow architecture     | `0002`, `0003`                                             | Core vs. profile-owned reusable workflows.                                                                                                                                               |
| Implementation language           | `0004`                                                     | Go for trusted core; shell stays glue.                                                                                                                                                   |
| Linter choices                    | `0005`, `0006`, `0007`                                     | golangci-lint, ShellCheck, no universal bundle.                                                                                                                                          |
| Formatter choices                 | `0008`                                                     | gofmt/goimports, shfmt, Prettier for Markdown.                                                                                                                                           |
| Dev tooling runtime               | `0009`, `0010`, `0011`, `0012`                             | Node/pnpm, Lefthook, mise bootstrap.                                                                                                                                                     |
| JS/TS npm package profile         | `0013`–`0037`, `0055`–`0064`, `0077`–`0085`, `0090`–`0096` | Package manager selection, OIDC publishing, SLSA3 npm workflow.                                                                                                                          |
| GitHub release asset profile      | `0038`–`0052`, `0057`–`0062`                               | Release asset subject handling, attestation distribution.                                                                                                                                |
| Release manifest metadata         | `0053`, `0054`, `0062`                                     | Signing boundary, predicate URI, producer policy conflicts.                                                                                                                              |
| ADR lifecycle metadata            | `0065`                                                     | Closed status grammar and relations field.                                                                                                                                               |
| Release run ownership             | `0066`                                                     | Job-class concurrency and mutation segment serialization.                                                                                                                                |
| Repeated run recovery             | `0067`                                                     | Run-identity convergence, outcome states, binding proofs.                                                                                                                                |
| Release asset run ownership       | `0072`                                                     | Sidecar-first pair binding and custody non-attribution.                                                                                                                                  |
| npm run-ownership proof           | `0073`                                                     | Published-attestation run identity for same-run convergence.                                                                                                                             |
| Mutation segment atomicity        | `0074`                                                     | Single-job segments and detection-based cross-run safety.                                                                                                                                |
| Mutation queue policy             | `0075`                                                     | `queue: max` FIFO waiting for mutation segment contenders.                                                                                                                               |
| Preflight and classification      | `0076`                                                     | Observation preflights; first-mutation classification otherwise.                                                                                                                         |
| Verifier identity binding         | `0068`                                                     | Immutable builder/source identities for verification.                                                                                                                                    |
| Transparency and trust root       | `0069`                                                     | Rekor inclusion, offline verification, trust root governance.                                                                                                                            |
| Build-environment recording       | `0070`, `0071`                                             | Toolchain distributions, runner image, builder.version, builderDependencies.                                                                                                             |
| Windlass provenance signing       | `0077`                                                     | Go-native exact-byte DSSE signing for all profiles and release manifests. Supersedes `0035`/`0055`, which are historical only.                                                           |
| pnpm settings-only workspaces     | `0078`                                                     | Missing `packages` selects only the root package.                                                                                                                                        |
| Caller-specified source ref       | `0079`                                                     | Tags-only `source-ref` input for fixed-pipeline release retries.                                                                                                                         |
| Source binding model              | `0080`                                                     | Signed provenance fields vs certificate invocation claims.                                                                                                                               |
| npm OIDC exchange contract        | `0081`                                                     | Observed response shape; 15-minute exchange token lifetime.                                                                                                                              |
| Publish-stage npm version pin     | `0082`                                                     | Integrity-verified provisioning and a reviewed allowlist.                                                                                                                                |
| npm M1 remediation deferral       | `0083`                                                     | Wait for npm/cli#9882; fixed release opens the allowlist.                                                                                                                                |
| Publish npm provisioning          | `0084`                                                     | Digest-verified npm registry tarball with a committed SHA-512.                                                                                                                           |
| Build toolchain pair pin          | `0085`                                                     | Exact Node.js 24 patch per builder release; bundled npm asserted pre-use.                                                                                                                |
| pnpm provisioning via mise        | `0086`                                                     | mise packageManager-field resolution replaces Corepack; single pin source in package.json.                                                                                               |
| pnpm 12 for dev tooling           | `0087`                                                     | Adopt the pnpm 12 line for Node.js development tooling.                                                                                                                                  |
| Build-stage PM provisioning       | `0088`                                                     | Digest-verified npm registry tarballs replace Corepack for pnpm/Yarn; rollout release is open.                                                                                           |
| Dev Node.js runtime pin           | `0089`                                                     | package.json devEngines.runtime is the sole declaration; mise resolves it via idiomatic package.json resolution.                                                                         |
| pnpm 10.x support extension       | `0090`                                                     | Supported pnpm range extended to the 10.x line.                                                                                                                                          |
| Uniform selection source          | `0091`                                                     | devEngines.packageManager selects any supported package manager.                                                                                                                         |
| Descriptor digest declarations    | `0092`–`0094`                                              | Optional declared digests, SRI algorithm set, hex-suffix format.                                                                                                                         |
| npm descriptor semantics          | `0095`                                                     | Declared npm versions warn on mismatch; npm digest suffixes are rejected.                                                                                                                |
| Project .corepack.env policy      | `0096`–`0097`                                              | Neutralized by a pinned COREPACK_ENV_FILE=0; presence warns. A packed file is rejected at the post-pack gate (0097).                                                                     |
| Build-stage Node.js runtime       | `0098`                                                     | Digest-verified release tarball with a committed SHA-256 replaces setup-action and toolcache selection; bundled vs provisioned build npm is a spec-level choice.                         |
| Node.js runtime recording         | `0099`                                                     | Digest-verified runtime tarball as a `resolvedDependencies` descriptor (accepted).                                                                                                       |
| Runner image identity             | `0100`                                                     | On-disk image metadata is authoritative for `image_version`; the control-plane deployment version is recorded separately as `deployment_version` evidence.                               |
| External-process stderr relay     | `0101`                                                     | Allowlisted tool stderr reaches the log through a spec-owned hygiene filter; classification is enrichment; the canonical contract stays sealed.                                          |
| npm E422 rejection classification | `0102`                                                     | E422 rejections read back under a reduced budget and fail determinately when no publication appears within it; the full budget is retained when the version is present.                  |
| Invocation identity fields        | `0103`                                                     | Common external workflow identity for producer profiles; ADR 0080 byte compatibility is partially superseded.                                                                            |
| Builder record representation     | `0104`                                                     | Removes the redundant external-parameter builder triplet while retaining canonical `runDetails.builder.id`, authenticated signer identity, and trusted-policy verification.              |
| Source dependency roles           | `0105`                                                     | Mandatory role-named `invocation-source` and `built-source` descriptors with `gitCommit` digests; npm index-0 serialization, unordered name-keyed verification.                          |
| External source consolidation     | `0106`                                                     | Derived source fields removed; optional caller-selector `source.ref` retained; verification re-pointed to workflow fields and descriptors; `ref_type` becomes a built-source annotation. |
| Invocation event metadata         | `0107`                                                     | `event_name` recorded only in `internalParameters.github`; the `source` group is emitted only when an explicit selector is present.                                                      |
| Redundant npm release inputs      | `0108`                                                     | Public `release-tag` and `provenance-sidecar` removed; the release target derives from the effective built tag and mandatory sidecar distribution remains.                               |
| External parameter minimization   | `0109`                                                     | Actual interface inputs only; a package-manifest descriptor is added, observations are re-homed, and derived values are recomputed.                                                      |
| Non-runner dependency media types | `0110`                                                     | Deterministic `mediaType` values for concrete artifacts; source descriptors remain untyped and `downloadLocation` stays absent.                                                          |
| Runner-image reported identity    | `0111`                                                     | Included Software URI and exact full `image_version`; offline projection without public-availability gating, digest, content, or media typing.                                           |
| Runner-image operating system     | `0112`                                                     | Exact decoded `Operating System.detail` recorded as `operating_system`; `$ImageOS` is diagnostic-only and raw metadata is not preserved.                                                 |

## CONVENTIONS

- **Format**: MADR 4.0.0. Use `0000-title.md` numbering (the template itself is `0000`).
- **Status**: Closed grammar per ADR 0065. Exactly one of `proposed`, `rejected`, `accepted`,
  `deprecated`, or `superseded by ADR-XXXX`. Composite or prose status values are invalid.
- **Relations**: ADR-to-ADR relationships live in the frontmatter `relations` field (ADR 0065), not
  in `status`. Four directional pairs: `supersedes`/`superseded-by`,
  `partially-supersedes`/`partially-superseded-by`, `amends`/`amended-by`, `see-also`. Partial and
  amendment relations require a `scope` identifying the affected clauses. Only full supersession
  changes the earlier ADR's `status`; partial supersession and amendment leave it `accepted`.
- **Discriminator**: `partially-supersedes` when an implementer may no longer follow the earlier
  clause as written; `amends` when the earlier clause still governs and the newer ADR only qualifies
  it. Resolve ambiguous cases in favor of `partially-supersedes`.
- **Symmetry**: Relation edges are bidirectional. A new ADR must declare every accepted ADR it
  supersedes, partially supersedes, or amends, and the same change must add the matching reverse
  entry to each target. An omission is a traceability defect, repaired by adding the missing reverse
  entry without a new ADR.
- **Dates**: Use Holocene Era year format (e.g., `12026-06-23`).
- **Immutability**: Do not edit the body of an accepted ADR. Write a new ADR instead. After
  acceptance, only the `status` and `relations` frontmatter fields may change.

## ANTI-PATTERNS

- Do not invent new numbering schemes; continue the sequence.
- Do not change an accepted ADR's body to reverse a decision.
- Do not invent new lifecycle statuses (for example `amended` or `partially updated`); use the
  closed status grammar and `relations` field instead.
- Do not write composite or prose `status` values.
- Do not declare a forward relation without adding the reverse edge to the target ADR.
- Do not put implementation details here; ADRs explain _why_, specs explain _what_.
- Do not use Node.js or pnpm for trusted/runtime logic.
