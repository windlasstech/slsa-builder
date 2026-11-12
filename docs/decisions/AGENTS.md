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

| Topic                                                | ADR                            | Notes                                                                                                                                                                                                                                     |
| ---------------------------------------------------- | ------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Why the repo exists                                  | `0001`                         | Clean-repository foundation.                                                                                                                                                                                                              |
| Trusted workflow architecture                        | `0002`, `0003`                 | Core vs. profile-owned reusable workflows.                                                                                                                                                                                                |
| Implementation language                              | `0004`                         | Go for trusted core; shell stays glue.                                                                                                                                                                                                    |
| Linter choices                                       | `0005`, `0006`, `0007`         | golangci-lint, ShellCheck, no universal bundle.                                                                                                                                                                                           |
| Formatter choices                                    | `0008`                         | gofmt/goimports, shfmt, Prettier for Markdown.                                                                                                                                                                                            |
| Dev tooling runtime                                  | `0009`, `0010`, `0011`, `0012` | Node/pnpm, Lefthook, mise bootstrap.                                                                                                                                                                                                      |
| JS/TS npm package profile                            | `0013`–`0037`                  | Package selection, manifest-first managers, build/pack, workflow entrypoint, OIDC publishing, provenance, initial signing adapter and historical decisions.                                                                               |
| GitHub Release asset profile                         | `0038`–`0052`                  | Release asset subjects, buildType URIs, publisher model, producer handoff, sidecar distribution, npm composition and historical decisions.                                                                                                |
| Release manifest and trusted producer policy         | `0053`–`0054`, `0062`          | Three-job signing boundary, release manifest predicate URI, trusted producer policy intersection and conflict handling.                                                                                                                   |
| Signing adapter Statement construction               | `0055`                         | `actions/attest` custom mode and post-sign Statement checks, later superseded by the Go-native signer.                                                                                                                                    |
| Non-selected lockfile diagnostics                    | `0056`                         | Stale lockfile handling under manifest-selected package managers.                                                                                                                                                                         |
| Public npm release interface and publisher authority | `0057`–`0060`, `0108`          | Single public entrypoint, minimal `workflow_call` interface, same-repository least-privilege publisher, separated internal authority, removed `release-tag` and `provenance-sidecar` inputs, built-tag derivation and mandatory sidecars. |
| Signed Statement JSON parsing                        | `0061`                         | Duplicate JSON member rejection before semantic validation of signed SLSA Statements.                                                                                                                                                     |
| Consumer pnpm/Yarn support boundaries                | `0063`, `0090`                 | Yarn Berry support metadata and Corepack history, consumer pnpm range `[10.0.0, 12.0.0)` and support-table boundary.                                                                                                                      |
| npm provenance subject compatibility                 | `0064`                         | npm PURL subject with SHA-512 and SHA-256 over the same tarball bytes.                                                                                                                                                                    |
| ADR lifecycle metadata                               | `0065`                         | Closed status grammar, separate relations field and bidirectional traceability.                                                                                                                                                           |
| Common run convergence and mutation policy           | `0066`–`0067`, `0074`–`0076`   | Job-class concurrency, run-identity idempotency, single-job mutation segments, detection-based safety, `queue: max` FIFO, observation preflights and first-mutation classification.                                                       |
| Verifier identity and transparency trust             | `0068`–`0069`                  | Immutable builder/source identities, Rekor inclusion, offline verification and governed Sigstore trust root.                                                                                                                              |
| Build-environment and builder recording              | `0070`–`0071`                  | Package-manager distributions, initial runner-image descriptor, `builder.version` and `builderDependencies`.                                                                                                                              |
| Release asset run ownership binding                  | `0072`                         | Sidecar-first pair binding and custody non-attribution for release assets.                                                                                                                                                                |
| npm publish convergence and read-back classification | `0073`, `0102`                 | Published-attestation run identity, E422 reduced read-back budget, determinate absence and full budget when the version exists.                                                                                                           |
| Windlass provenance signing adapter                  | `0077`                         | Go-native exact-byte Sigstore DSSE signing for all profiles, superseding `0035` and `0055`.                                                                                                                                               |
| pnpm settings-only root-package mode                 | `0078`                         | Missing `packages` in `pnpm-workspace.yaml` selects root-only mode rather than malformed workspace data.                                                                                                                                  |
| Caller source retry and invocation binding           | `0079`–`0080`                  | Tags-only `source-ref`, fixed-pipeline release retries, source policy bound to signed fields and certificate claims as invocation context.                                                                                                |
| npm OIDC exchange response contract                  | `0081`                         | Observed success response shape and 15-minute exchange-token lifetime.                                                                                                                                                                    |
| Publish-stage npm CLI provisioning                   | `0082`–`0084`                  | Reviewed allowlist, initial pin containing npm/cli#9882, digest-verified npm registry tarball and committed SHA-512.                                                                                                                      |
| Consumer build-stage Node.js runtime                 | `0085`, `0098`–`0099`          | Exact Node.js 24 patch and bundled npm pair, SHA-256-verified release tarball, runtime recording in `resolvedDependencies`.                                                                                                               |
| Repository development Node.js and pnpm tooling      | `0086`–`0087`, `0089`          | Repository-only mise field resolution, `package.json` pins, `devEngines.runtime`, `devEngines.packageManager` and pnpm 12 development tooling.                                                                                            |
| Consumer package-manager provisioning and selection  | `0088`, `0091`                 | Build-stage pnpm/Yarn registry tarballs replacing Corepack, rollout boundary and uniform consumer selection through `devEngines.packageManager`.                                                                                          |
| Descriptor integrity digest grammar                  | `0092`–`0094`                  | Optional declared digests verified fail closed, W3C SRI algorithm tokens, `+algorithm.<hex>` suffixes, SRI base64 rejection and unchanged record model.                                                                                   |
| npm descriptor semantics                             | `0095`                         | Declared-version mismatch warnings, npm digest rejection and the separate npm CLI self-enforcement boundary.                                                                                                                              |
| Project and packed `.corepack.env` policy            | `0096`–`0097`                  | `COREPACK_ENV_FILE=0`, non-fatal project-file warning and parser-free post-pack rejection.                                                                                                                                                |
| Runner-image evidence and reported identity          | `0100`, `0111`–`0112`          | On-disk image metadata, Included Software URI, exact full `image_version`, non-authoritative `deployment_version`, decoded `Operating System.detail`, diagnostic-only `$ImageOS` and offline projection.                                  |
| External CLI stderr hygiene relay                    | `0101`                         | Allowlisted subprocess stderr, spec-owned hygiene filter, classification as enrichment and sealed canonical contract.                                                                                                                     |
| Invocation identity and external parameters          | `0103`–`0107`, `0109`          | Common workflow identity, builder triplet removal, `invocation-source` and `built-source`, source consolidation, `event_name` in `internalParameters.github`, package-manifest descriptor, actual interface inputs and recompute rules.   |
| Non-runner dependency media types                    | `0110`                         | Deterministic `mediaType` for concrete artifacts, untyped source descriptors and absent `downloadLocation`.                                                                                                                               |

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
