# CI/Workflow Knowledge Base

## OVERVIEW

GitHub Actions workflows: one production reusable workflow (JS/TS npm SLSA3 profile) plus quality
gates (lint, autofix, fuzz) and org-reusable security scans (Dependency Review, OSV, Scorecard). All
workflows are hardened by default; the authoritative security policy lives in
`windlasstech/.github`.

## STRUCTURE

```text
.github/workflows/
├── js-ts-npm-package-slsa3.yml # production reusable workflow: build -> provenance-sign -> publish
├── lint.yml                    # markdownlint, actionlint, golangci-lint, go test, fuzz smoke matrix
├── fuzz.yml                    # scheduled/manual deep fuzz matrix (10 min/target), corpus upload
├── autofix.yml                 # autoformat PRs/pushes with Prettier, golangci-lint fmt, shfmt
├── dependency-review.yml       # PR/merge-group dependency review gate (org reusable)
├── osv-scanner.yml             # PR/merge-group scan + full scan on push to main and weekly schedule
└── scorecard.yml               # OpenSSF Scorecard on branch-protection/main/schedule events
```

Also: `.github/actionlint.yaml` — scoped suppressions, see below.

## WHERE TO LOOK

| Task                        | File                          | Notes                                                                          |
| --------------------------- | ----------------------------- | ------------------------------------------------------------------------------ |
| Production npm profile      | `js-ts-npm-package-slsa3.yml` | `workflow_call` only; 9 inputs / 7 outputs; three-job graph.                   |
| Required code-quality gate  | `lint.yml`                    | PR + push to `main` + dispatch; includes `go test ./...` and 30s fuzz smoke.   |
| Deep fuzz conformance       | `fuzz.yml`                    | Weekly Mon 04:00 UTC + dispatch; same matrix as lint smoke, 10 min per target. |
| Autoformat a PR             | `autofix.yml`                 | Pushes formatter fixes via `autofix.ci`; never touches `testdata/` fixtures.   |
| Dependency review           | `dependency-review.yml`       | Ignores docs/markdown-only changes on PRs; merge groups unfiltered.            |
| Vulnerability scanning      | `osv-scanner.yml`             | Org reusable workflows; separate PR and full-scan jobs.                        |
| Supply-chain scorecard      | `scorecard.yml`               | Org reusable workflow; top-level `permissions: read-all` is intentional.       |
| Local hook equivalents      | `lefthook.yml`                | Pre-commit formatters → linters; commit-msg DCO check.                         |
| Workflow conformance checks | `internal/workflowcheck/`     | Static checker for the production workflow's contract.                         |

## CONVENTIONS

- **SHA-pinning**: every third-party action is pinned to a full SHA with a comment tag. The
  sanctioned exception is `windlasstech/.github` org reusable workflows referenced at `@main`.
- **Hardened runner**: every locally stepped job starts with `step-security/harden-runner` in
  `egress-policy: audit`. Org reusable-workflow jobs cannot carry local steps; their hardening is
  delegated to `windlasstech/.github`.
- **Minimal permissions**: top-level `permissions: {}` with job-level elevation only where required
  (exceptions: `autofix.yml`/`osv-scanner.yml` use top-level `contents: read`; `scorecard.yml` uses
  `read-all` for its publishing model).
- **Production workflow permissions split**: `build` gets `contents: read` only; `provenance-sign`
  and `publish` add `id-token: write` (OIDC). No job ever gets `contents: write`, `packages: write`,
  `attestations: write`, or `artifact-metadata: write` in the npm-only profile.
- **Trusted self-checkout**: the production workflow checks out its own repository with
  `job.workflow_repository` at `job.workflow_sha`, credentials disabled — this is the F01 runtime
  delivery mechanism, and the two `job.workflow_*` expressions are intentional.
- **Mutation serialization**: the `publish` job uses concurrency group
  `release-mutation-${{ github.repository }}-${{ inputs.source-ref || github.ref }}` with
  `cancel-in-progress: false` and `queue: max` (ADRs 0066/0074/0075). Pre-mutation jobs use
  `cancel-in-progress: true` and no `queue`.
- **actionlint suppressions**: `.github/actionlint.yaml` suppresses exactly three diagnostics, all
  scoped to `js-ts-npm-package-slsa3.yml`: dynamic `job.workflow_repository`, dynamic
  `job.workflow_sha`, and the `queue` concurrency extension. Do not "fix" the flagged behavior.
- **Go invocation glue**: workflow `run:` steps only invoke
  `go run ./cmd/slsa-builder-internal <subcommand>` with flags; shell never parses trusted JSON/JWT.
- **No long-lived credentials**: publishing authenticates via npm OIDC trusted publishing; there is
  no `NPM_TOKEN`/`NODE_AUTH_TOKEN` path anywhere.
- **Go formatting**: use `golangci-lint fmt`/`golangci-lint run`; there is no standalone `gofmt` CI
  step.
- **Signing**: provenance signing is Go-native (ADR 0077); `actions/attest` is forbidden in the
  production workflow and `workflow-check` rejects it.

## ANTI-PATTERNS

- Do not use floating tags for third-party actions (e.g., `@v3`). Always pin a SHA.
- Do not add `contents: write` (or any write permission) to a job that does not need it.
- Do not add a workflow that bypasses `windlasstech/.github` dependency-review or OSV scanner gates.
- Do not reference third-party reusable workflows by mutable branch or tag; the
  `windlasstech/.github` `@main` refs are the sanctioned org exception.
- Do not add long-lived cloud or registry credentials; prefer OIDC where elevation is required.
- Do not remove or broaden the three scoped actionlint suppressions without sufficient evidence;
  they pin intentional platform behavior. If a scope change appears justified (for example, a future
  actionlint release natively supporting the suppressed syntax), report the evidence to a human
  maintainer and obtain approval before modifying them.
- Do not change the publish job's `cancel-in-progress`/`queue: max` pair; mutation ordering depends
  on it.
- Do not add `actions/attest` to the production workflow (superseded signing adapter, ADR 0077).
