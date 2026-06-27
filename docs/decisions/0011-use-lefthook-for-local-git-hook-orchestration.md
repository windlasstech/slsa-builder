---
parent: Decisions
nav_order: 11
status: accepted
date: 12026-06-24
decision-makers: Yunseo Kim
---

# Use Lefthook for Local Git Hook Orchestration

## Context and Problem Statement

ADR 0005 chose a dedicated linter toolchain, ADR 0006 chose `golangci-lint` for Go linting, ADR 0007 chose ShellCheck for shell glue linting, ADR 0008 chose dedicated formatters for Go, Markdown, and shell, and ADR 0010 chose pnpm for Node.js development tooling.
The repository now needs a local Git hook orchestration tool that can run the selected linters and formatters before commits and enforce the DCO `Signed-off-by:` trailer in commit messages.

Which local Git hook tool should `slsa-builder` use for commit-time DCO checks and pre-commit linting and formatting?

## Decision Drivers

- Enforce DCO sign-off locally through a `commit-msg` hook while keeping CI or PR-level DCO enforcement as the authoritative gate.
- Run the selected linting and formatting tools automatically at commit time without replacing the dedicated toolchain decisions.
- Keep the hook orchestrator small, explicit, and suitable for a Go-first supply-chain security project.
- Support Go, Markdown, shell, GitHub Actions, and future profile-local surfaces without becoming language-specific.
- Work cleanly with the chosen Node.js development-tooling path and pnpm lockfile discipline.
- Support fast local feedback through staged-file filtering and parallel execution where appropriate.
- Avoid adding unrelated runtime ecosystems only to manage Git hooks.

## Considered Options

- Use Lefthook for local Git hook orchestration.
- Use the `pre-commit` framework.
- Use Husky with lint-staged.
- Use native Git hooks with `core.hooksPath`.
- Use Trunk Check hooks.
- Use Overcommit.

## Decision Outcome

Chosen option: "Use Lefthook for local Git hook orchestration", because it is a fast, polyglot Git hook manager distributed as a Go binary, supports both `pre-commit` and `commit-msg` hooks, and can be pinned through the existing pnpm development-tooling path without making Git hooks a JavaScript-specific concern.

Lefthook should orchestrate local hooks only.
It should not replace the selected linters, formatters, vulnerability scanners, workflow analyzers, or CI gates.

The initial hook policy should include:

- a `commit-msg` hook that rejects commit messages without a valid `Signed-off-by:` trailer;
- a `pre-commit` hook that runs the repository-selected formatters and linters on applicable staged files;
- commands that invoke pinned project-local or otherwise pinned tool versions;
- configuration that keeps Node.js and pnpm usage development-only and outside the trusted runtime selected in ADR 0004.

Local DCO checks are a developer feedback mechanism, not the final compliance boundary.
Because client-side hooks can be bypassed with `git commit --no-verify`, DCO compliance should also be verified by CI, branch protection, a GitHub App, or an equivalent PR-level required check before changes are merged.

### Consequences

- Good, because Lefthook fits the Go-first project direction better than hook managers centered on Python, Ruby, PHP, or JavaScript runtime ecosystems.
- Good, because one committed configuration can manage both commit-message checks and pre-commit quality checks.
- Good, because staged-file filtering and parallel execution can keep commit-time feedback fast.
- Good, because Lefthook can call the already-selected tools directly instead of hiding them behind a universal linter or formatter bundle.
- Good, because DCO failures can be reported before the commit is created, reducing contributor friction.
- Neutral, because Lefthook still needs installation or bootstrap documentation for contributors after clone.
- Bad, because pnpm may require explicit allowance for Lefthook's install-time hook setup behavior when installed as a development dependency.
- Bad, because local hooks are bypassable and must not be treated as a substitute for CI or PR enforcement.

### Confirmation

This decision is confirmed when local hook configuration and documentation:

- add Lefthook as a pinned development dependency or otherwise document a pinned installation path;
- commit a `lefthook.yml` configuration for `pre-commit` and `commit-msg` hooks;
- make the `commit-msg` hook fail when the commit message lacks a `Signed-off-by:` trailer;
- run the selected formatters and linters through their pinned commands rather than through a broad universal bundle;
- preserve separation between formatting, linting, vulnerability scanning, workflow analysis, and DCO checks;
- document that CI or PR-level DCO verification remains required because local hooks can be bypassed;
- keep Lefthook out of the trusted runtime and release artifact execution path.

Implementation review should verify that hook commands are narrow, reproducible, and aligned with the tool choices recorded in earlier ADRs.

## Pros and Cons of the Options

### Use Lefthook for local Git hook orchestration

Use Lefthook as the repository's local Git hook manager, with configuration committed in `lefthook.yml` and hooks installed into each developer clone.

- Good, because it is implemented in Go and distributed as a compact binary.
- Good, because it is polyglot and can orchestrate Go, Markdown, shell, Node-based, and future profile-local checks without making the repository hook model language-specific.
- Good, because it supports `pre-commit`, `commit-msg`, staged-file filtering, parallel execution, and automatic restaging of formatter changes where configured.
- Good, because it can be installed through pnpm as a development dependency while still keeping Node.js out of the trusted runtime.
- Good, because it allows DCO enforcement to remain a small explicit commit-message check rather than adopting a separate DCO-only tool.
- Neutral, because it introduces one additional development tool and configuration file.
- Bad, because contributors must install hooks locally, and bypass remains possible through standard Git behavior.

### Use the `pre-commit` framework

Use the Python-based `pre-commit` framework to manage repository hooks through `.pre-commit-config.yaml`.

- Good, because it is mature, widely used, language-agnostic, and has a large hook ecosystem.
- Good, because it handles staged-file workflows and can manage multiple hook types, including `commit-msg`, when installed for those stages.
- Good, because many common linting and formatting tools have existing pre-commit integrations.
- Neutral, because it remains a reasonable choice for repositories that already use Python tooling.
- Bad, because this repository has not otherwise selected Python as a development runtime.
- Bad, because DCO enforcement would still require a custom or third-party hook.
- Bad, because hook dependencies may be mediated through pre-commit-managed environments rather than the explicit tool installation paths chosen in earlier ADRs.

### Use Husky with lint-staged

Use Husky to install Git hooks and lint-staged to run commands on staged files.

- Good, because it integrates naturally with Node.js and pnpm development tooling.
- Good, because lint-staged provides a strong staged-file UX for formatters and linters.
- Good, because Husky is familiar in JavaScript and TypeScript repositories.
- Neutral, because it may become more attractive if a future profile introduces substantial TypeScript development tooling.
- Bad, because it makes the hook layer feel JavaScript-specific in a Go-first repository.
- Bad, because DCO enforcement still requires a custom `commit-msg` script.
- Bad, because it would add more Node-specific hook machinery than needed for a repository whose current Node usage exists only to run Prettier.

### Use native Git hooks with `core.hooksPath`

Commit hook scripts directly in the repository and instruct contributors to configure Git's `core.hooksPath` to point at them.

- Good, because it minimizes third-party dependencies.
- Good, because every hook script is directly visible and auditable.
- Good, because DCO checking can be implemented as a small shell script.
- Neutral, because this can remain a useful fallback for very small repositories.
- Bad, because initial setup depends on every contributor running a Git configuration command after clone.
- Bad, because staged-file filtering, parallel execution, formatter restaging, and cross-platform polish must be implemented manually.
- Bad, because shell hook scripts can grow into the kind of glue-heavy behavior ADR 0004 tried to avoid.

### Use Trunk Check hooks

Use Trunk's Git hook integration and `trunk check` as the local quality gate.

- Good, because Trunk provides a cohesive developer experience for installing tools, running checks, and integrating hooks.
- Good, because it can manage many linting and formatting tools consistently.
- Neutral, because Trunk may be useful for broader repositories that prefer a single quality platform.
- Bad, because ADR 0005 already rejected Trunk Check as the primary linting strategy in favor of a dedicated, explicit linter toolchain.
- Bad, because it would blur the boundary between hook orchestration and linter or formatter selection.
- Bad, because it adds more orchestration platform surface than this repository currently needs.

### Use Overcommit

Use the Ruby-based Overcommit hook manager.

- Good, because it is a mature Git hook manager with support for pre-commit and commit-message checks.
- Good, because it can centralize hook configuration in the repository.
- Neutral, because it can be appropriate in Ruby-heavy repositories.
- Bad, because this project has not selected Ruby for implementation or development tooling.
- Bad, because adding Ruby and gem management solely for Git hooks conflicts with the project's preference for small, explicit tooling.
- Bad, because it offers no clear advantage over Lefthook for this repository's Go, Markdown, shell, and pnpm surfaces.

## More Information

This decision is an orchestration decision only.
It does not change the selected linters, formatters, package manager, or trusted runtime:

- ADR 0004 keeps the trusted implementation runtime in Go.
- ADR 0005 keeps linting as a dedicated toolchain rather than a universal bundle.
- ADR 0006 selects `golangci-lint` for Go linting.
- ADR 0007 selects ShellCheck for shell glue linting.
- ADR 0008 selects `gofmt`, `goimports`, Prettier, and `shfmt` as formatters.
- ADR 0009 and ADR 0010 limit Node.js and pnpm to development tooling.

Future implementation should decide the exact hook commands and staged-file patterns in configuration, not in this ADR.
The DCO hook should prefer Git trailer-aware validation where practical, such as parsing commit trailers, rather than relying only on an unanchored substring match.
