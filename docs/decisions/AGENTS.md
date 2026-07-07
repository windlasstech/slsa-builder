# ADR Knowledge Base

## OVERVIEW

Architecture decision records for the SLSA builder. Each ADR is a MADR 4.0.0 document with a
sequential four-digit number and a kebab-case title. The sequence currently runs from `0000` through
`0053`.

## STRUCTURE

```text
docs/decisions/
├── 0000-use-markdown-architectural-decision-records.md  # MADR template
├── 0001-start-slsa-builder-as-clean-repository.md
├── 0002-use-extensible-trusted-reusable-workflow-foundation.md
├── ...                                                  # 0003–0052
├── 0053-use-three-job-release-manifest-signing-boundary.md
└── AGENTS.md
```

ADR numbering is sequential from `0000` through `0053`. See the WHERE TO LOOK table below for topic
groupings.

## WHERE TO LOOK

| Topic                         | ADR                            | Notes                                                           |
| ----------------------------- | ------------------------------ | --------------------------------------------------------------- |
| Why the repo exists           | `0001`                         | Clean-repository foundation.                                    |
| Trusted workflow architecture | `0002`, `0003`                 | Core vs. profile-owned reusable workflows.                      |
| Implementation language       | `0004`                         | Go for trusted core; shell stays glue.                          |
| Linter choices                | `0005`, `0006`, `0007`         | golangci-lint, ShellCheck, no universal bundle.                 |
| Formatter choices             | `0008`                         | gofmt/goimports, shfmt, Prettier for Markdown.                  |
| Dev tooling runtime           | `0009`, `0010`, `0011`, `0012` | Node/pnpm, Lefthook, mise bootstrap.                            |
| JS/TS npm package profile     | `0013`–`0035`                  | Package manager selection, OIDC publishing, SLSA3 npm workflow. |
| GitHub release asset profile  | `0036`–`0052`                  | Release asset subject handling, attestation distribution.       |
| Release manifest signing      | `0053`                         | Three-job signing boundary.                                     |

## CONVENTIONS

- **Format**: MADR 4.0.0. Use `0000-title.md` numbering (the template itself is `0000`).
- **Status**: accepted / deprecated / superseded / etc. Only the status field may change after
  acceptance.
- **Dates**: Use Holocene Era year format (e.g., `12026-06-23`).
- **Immutability**: Do not edit the body of an accepted ADR. Write a new ADR instead.

## ANTI-PATTERNS

- Do not invent new numbering schemes; continue the sequence.
- Do not change an accepted ADR's body to reverse a decision.
- Do not put implementation details here; ADRs explain _why_, specs explain _what_.
- Do not use Node.js or pnpm for trusted/runtime logic.
