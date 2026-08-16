# ADR Knowledge Base

## OVERVIEW

Architecture decision records for the SLSA builder. Each ADR is a MADR 4.0.0 document with a
sequential four-digit number and a kebab-case title. The sequence currently runs from `0000` through
`0084`.

## STRUCTURE

```text
docs/decisions/
├── 0000-use-markdown-architectural-decision-records.md  # MADR template
├── 0001-start-slsa-builder-as-clean-repository.md
├── 0002-use-extensible-trusted-reusable-workflow-foundation.md
├── ...                                                  # 0003–0082
├── 0083-defer-npm-m1-publish-remediation-to-the-upstream-provenance-file-fix.md
├── 0084-provision-the-publish-stage-npm-cli-from-a-digest-verified-registry-tarball.md
├── README.md / README.ko.md
└── AGENTS.md
```

ADR numbering is sequential from `0000` through `0084`. See the WHERE TO LOOK table below for topic
groupings.

## WHERE TO LOOK

| Topic                         | ADR                                         | Notes                                                                        |
| ----------------------------- | ------------------------------------------- | ---------------------------------------------------------------------------- |
| Why the repo exists           | `0001`                                      | Clean-repository foundation.                                                 |
| Trusted workflow architecture | `0002`, `0003`                              | Core vs. profile-owned reusable workflows.                                   |
| Implementation language       | `0004`                                      | Go for trusted core; shell stays glue.                                       |
| Linter choices                | `0005`, `0006`, `0007`                      | golangci-lint, ShellCheck, no universal bundle.                              |
| Formatter choices             | `0008`                                      | gofmt/goimports, shfmt, Prettier for Markdown.                               |
| Dev tooling runtime           | `0009`, `0010`, `0011`, `0012`              | Node/pnpm, Lefthook, mise bootstrap.                                         |
| JS/TS npm package profile     | `0013`–`0037`, `0055`–`0064`, `0077`–`0078` | Package manager selection, OIDC publishing, SLSA3 npm workflow.              |
| GitHub release asset profile  | `0038`–`0052`, `0057`–`0062`                | Release asset subject handling, attestation distribution.                    |
| Release manifest metadata     | `0053`, `0054`, `0062`                      | Signing boundary, predicate URI, producer policy conflicts.                  |
| ADR lifecycle metadata        | `0065`                                      | Closed status grammar and relations field.                                   |
| Release run ownership         | `0066`                                      | Job-class concurrency and mutation segment serialization.                    |
| Repeated run recovery         | `0067`                                      | Run-identity convergence, outcome states, binding proofs.                    |
| Release asset run ownership   | `0072`                                      | Sidecar-first pair binding and custody non-attribution.                      |
| npm run-ownership proof       | `0073`                                      | Published-attestation run identity for same-run convergence.                 |
| Mutation segment atomicity    | `0074`                                      | Single-job segments and detection-based cross-run safety.                    |
| Mutation queue policy         | `0075`                                      | `queue: max` FIFO waiting for mutation segment contenders.                   |
| Preflight and classification  | `0076`                                      | Observation preflights; first-mutation classification otherwise.             |
| Verifier identity binding     | `0068`                                      | Immutable builder/source identities for verification.                        |
| Transparency and trust root   | `0069`                                      | Rekor inclusion, offline verification, trust root governance.                |
| Build-environment recording   | `0070`, `0071`                              | Toolchain distributions, runner image, builder.version, builderDependencies. |
| Windlass provenance signing   | `0077`                                      | Go-native exact-byte DSSE signing for all profiles and release manifests.    |
| pnpm settings-only workspaces | `0078`                                      | Missing `packages` selects only the root package.                            |
| Caller-specified source ref   | `0079`                                      | Tags-only `source-ref` input for fixed-pipeline release retries.             |
| Source binding model          | `0080`                                      | Signed provenance fields vs certificate invocation claims.                   |
| npm OIDC exchange contract    | `0081`                                      | Observed response shape; 15-minute exchange token lifetime.                  |
| Publish-stage npm version pin | `0082`                                      | Integrity-verified provisioning and a reviewed allowlist.                    |
| npm M1 remediation deferral   | `0083`                                      | Wait for npm/cli#9882; fixed release opens the allowlist.                    |
| Publish npm provisioning      | `0084`                                      | Digest-verified npm registry tarball with a committed SHA-512.               |

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
