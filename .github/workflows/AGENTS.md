# CI/Workflow Knowledge Base

## OVERVIEW

GitHub Actions workflows for lint, autoformat, dependency review, OSV scanning, and OpenSSF
Scorecard. All workflows are hardened by default; the authoritative security policy lives in
`windlasstech/.github`.

## STRUCTURE

```text
.github/workflows/
├── autofix.yml           # autoformat PRs/pushes with Prettier, golangci-lint fmt, shfmt
├── dependency-review.yml # PR/merge-group dependency review gate
├── lint.yml              # markdownlint, actionlint, golangci-lint
├── osv-scanner.yml       # scheduled + PR/push vulnerability scanning
└── scorecard.yml         # OpenSSF Scorecard on branch-protection/main/schedule events
```

## WHERE TO LOOK

| Task                       | File                    | Notes                                                  |
| -------------------------- | ----------------------- | ------------------------------------------------------ |
| Required code-quality gate | `lint.yml`              | Runs on PR and push to `main`.                         |
| Autoformat a PR            | `autofix.yml`           | Pushes formatter fixes via `autofix.ci`.               |
| Dependency review          | `dependency-review.yml` | Ignores docs/markdown-only changes.                    |
| Vulnerability scanning     | `osv-scanner.yml`       | Reusable org workflow; runs on schedule too.           |
| Supply-chain scorecard     | `scorecard.yml`         | Reusable org workflow.                                 |
| Local hook equivalents     | `lefthook.yml`          | Pre-commit formatters → linters; commit-msg DCO check. |

## CONVENTIONS

- **SHA-pinning**: every third-party action is pinned to a full SHA with a comment tag.
- **Hardened runner**: every job starts with `step-security/harden-runner` in
  `egress-policy: audit`.
- **Minimal permissions**: top-level `permissions: {}` with job-level elevation only where required.
- **Reusable org workflows**: security workflows (`dependency-review.yml`, `osv-scanner.yml`,
  `scorecard.yml`) call `windlasstech/.github` workflows on the main branch.
- **No secrets**: initial workflows avoid publishing credentials; see ADRs for trusted publishing
  plans.
- **Go formatting**: use `golangci-lint fmt`/`golangci-lint run`; there is no standalone `gofmt` CI
  step.

## ANTI-PATTERNS

- Do not use floating tags for third-party actions (e.g., `@v3`). Always pin a SHA.
- Do not add `contents: write` to a job that does not need it.
- Do not add a workflow that bypasses `windlasstech/.github` dependency-review or OSV scanner gates.
- Do not reference reusable workflows by mutable branch or tag; use the org’s main branch refs.
- Do not add long-lived cloud credentials; prefer OIDC where elevation is required.
