# PROJECT KNOWLEDGE BASE

- **Generated:** 12026-07-07
- **Commit:** e40a91e
- **Branch:** docs/adr-npm-and-release-asset-profiles

## OVERVIEW

Reusable, profile-extensible SLSA provenance builder foundation. Currently a docs/tooling scaffold;
the Go implementation tree has not been added yet.

## STRUCTURE

```text
.
├── AGENTS.md              # this file
├── README.md / README.ko.md
├── docs/decisions/        # architecture ADRs (see docs/decisions/AGENTS.md)
├── .github/workflows/     # CI/CD workflows (see .github/workflows/AGENTS.md)
├── .golangci.yml          # Go format/lint policy
├── lefthook.yml           # git hooks
├── mise.toml / mise.lock  # pinned runtimes and tools
├── package.json / pnpm-*  # dev-only Node tooling
└── .cursor/ .claude/ .gemini/ .kiro/  # editor/AI tool configs
```

## WHERE TO LOOK

| Task                    | Location                                                   | Notes                                    |
| ----------------------- | ---------------------------------------------------------- | ---------------------------------------- |
| Why a decision was made | `docs/decisions/`                                          | MADR 4.0.0 ADRs, numbered `0000`–`0054`. |
| Bootstrap / dev setup   | `README.md`, `mise.toml`                                   | `mise install` + `pnpm install`.         |
| CI / workflow security  | `.github/workflows/`                                       | See `.github/workflows/AGENTS.md`.       |
| Lint/format policy      | `.golangci.yml`, `.prettierrc`, `.markdownlint-cli2.jsonc` | Go, Markdown, shell.                     |
| Git hooks / DCO         | `lefthook.yml`                                             | Commit-msg `Signed-off-by:` check.       |
| Dependency security     | `pnpm-workspace.yaml`                                      | Cooldown, trust policy, frozen lockfile. |

## CONVENTIONS

### Spec-Driven Development (SDD)

Do not implement before reading the specs.

1. **ADRs first** (`docs/decisions/`): understand _why_ architecture was chosen.
2. **Specs second** (`docs/architecture/`): define _exact observable behavior_.
3. **Implementation third**: build against the specifications.

### Repo Conventions

- **ADRs**: Use MADR 4.0.0 format. Store in `docs/decisions/` with sequential numbering
  (`0001-title.md`).
- **ADR immutability**: Existing accepted ADRs are immutable. Never edit the body of an accepted ADR
  after the fact. The only permitted post-acceptance change is updating the `status` field (e.g., to
  `superseded`, `deprecated`). If a decision changes, write a new ADR rather than rewriting history.
- **Dates in human-facing documents**: Use Holocene Era / Human Era year format for prose, changelog
  headings, ADR dates, and other human-reader dates (e.g., `12026-06-23`). Machine-readable
  timestamps and protocol/schema fields, such as JSON `generated_at`, SLSA `startedOn`, and
  `finishedOn`, must preserve the applicable technical standard format, normally ISO 8601 with a
  standard four-digit Gregorian year (e.g., `2026-06-23T12:00:00Z`).
- **Bilingual README updates**: When editing any `README.md`, update the corresponding
  `README.ko.md` in the same directory as part of the same change.
- **CodeGraph MCP**: `opencode.jsonc` configures a local CodeGraph MCP server. Other AI tool configs
  (`.cursor/`, `.claude/`, `.kiro/`, `.gemini/`) also reference CodeGraph.

## Commits

- **DCO sign-off required**: Every commit must include a `Signed-off-by:` line. Use `git commit -s`
  (or `git commit --signoff`) for all commits. Lefthook enforces this in the commit-msg hook.
- **Commit message body format**: When a commit needs a multi-line body, prefer
  `git commit -s -F - <<'EOF'` (heredoc) over repeating `-m`. Repeating `-m` inserts blank lines
  between each argument and hurts readability; use heredoc unless the blank lines are intentional
  paragraph breaks.

## Changelog Management

- Maintain `CHANGELOG.md` according to
  [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), but use the organization's Human
  Era date convention for release headings (for example, `## [0.1.0] - 12026-06-13`).
- Changelog entries are for users and downstream integrators. Summarize notable upgrade-relevant
  behavior; do not generate changelog entries by dumping commit logs.
- For every PR, complete the organization PR template's `Changelog` section with:
  - **Category**: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`, or `None`
  - **User-facing note**: a short impact summary, or why no note is needed
- Use `None` for changes with no direct user-facing impact, such as test cleanup, internal
  refactoring, formatting, or CI-only maintenance.
- During development, update only the `[Unreleased]` section when a PR has user-facing impact. Group
  entries by `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, and `Security`; do not create
  empty category sections.
- For release PRs, move `[Unreleased]` entries into the new version section, recreate an empty
  `[Unreleased]` section at the top, update comparison links at the bottom of `CHANGELOG.md`, and
  use the finalized version section as the GitHub Release body.

## CI / Security

- Reusable workflows from `windlasstech/.github`:
  - Scorecard supply-chain security
  - OSV Scanner (full scan on schedule + push to main; PR scan on PRs + merge groups)
  - Dependency Review (on PRs + merge groups)
- Do not add build/test CI that bypasses these security checks.
- **Always reference** `windlasstech/.github` main branch security docs before making
  security-relevant changes:
  - Primary security policy:
    <https://raw.githubusercontent.com/windlasstech/.github/refs/heads/main/SECURITY.md>
  - Dependency security:
    <https://raw.githubusercontent.com/windlasstech/.github/refs/heads/main/docs/security/dependency-security.md>
  - SLSA compliance framework:
    <https://raw.githubusercontent.com/windlasstech/.github/refs/heads/main/docs/security/slsa-compliance-framework.md>
  - Workflow hardening:
    <https://raw.githubusercontent.com/windlasstech/.github/refs/heads/main/docs/security/workflow-hardening.md>
- Supply-chain baseline from the organization policy:
  - SLSA Build L1/L2 are required; Build L3+ is the target wherever feasible.
  - SLSA Source L1/L2 are required; Source L3 controls are followed where feasible; Source L4 is
    structurally blocked for a 1-person organization.
  - Release source integrity uses GPG-signed annotated tags, GPG-signed commits on `main`, protected
    branches/tags, linear history, and required CI gates.
  - Released binaries and container images must include signed SPDX and CycloneDX SBOM attestations
    when the build can generate them; public releases should publish the same SBOM files as release
    assets when possible.
  - Registry-published release artifacts should upload linked artifacts storage metadata with
    `artifact-metadata: write` when supported.
  - Dependency security is layered: committed lockfiles, Dependabot, cooldowns, Dependency Review,
    and OSV Scanner. Security updates bypass cooldowns; normal version updates use cooldowns.
  - Workflow hardening requires SHA-pinned third-party actions, hardened runners, explicit minimal
    top-level permissions, job-level elevation only when required, OIDC instead of long-lived cloud
    credentials, and protected production environments.
- GitHub Actions permission reminders:
  - Artifact attestations with `actions/attest`: `contents: read`, `id-token: write`,
    `attestations: write`.
  - Linked artifacts storage records: add `artifact-metadata: write` and use registry artifact
    subjects by immutable digest.
  - Container registry pushes: add `packages: write` only on the job that pushes images.
  - Release asset upload: add `contents: write` only on the release job.
  - PR comments: add `pull-requests: write` only for jobs that write comments.

## Pull Requests

- PRs must follow the template defined in `windlasstech/.github`:
  - <https://raw.githubusercontent.com/windlasstech/.github/refs/heads/main/.github/PULL_REQUEST_TEMPLATE.md>
- **Always fetch the template content** and write the PR body to match it. Do not rely on
  `gh pr create` to auto-populate the template; if it does not, manually compose the body using the
  fetched template structure.

<!-- CODEGRAPH_START -->

## CodeGraph

In repositories indexed by CodeGraph (a `.codegraph/` directory exists at the repo root), reach for
it BEFORE grep/find or reading files when you need to understand or locate code:

- **MCP tools** (when available): `codegraph_explore` answers most code questions in one call — the
  relevant symbols' verbatim source plus the call paths between them. `codegraph_node` returns one
  symbol's source + callers, or reads a whole file with line numbers. If the tools are listed but
  deferred, load them by name via tool search.
- **Shell** (always works): `codegraph explore "<symbol names or question>"` and
  `codegraph node <symbol-or-file>` print the same output.

If there is no `.codegraph/` directory, skip CodeGraph entirely — indexing is the user's decision.

<!-- CODEGRAPH_END -->
