---
parent: Decisions
nav_order: 5
status: accepted
date: 12026-06-23
decision-makers: Yunseo Kim
---

# Use a Dedicated Linter Toolchain Instead of a Universal Linter Bundle

## Context and Problem Statement

ADR 0004 chose Go as the primary implementation language and limited shell to minimal workflow glue.
The repository must still lint multiple surfaces, including Markdown documentation, Go code, GitHub Actions workflows, and future profile-local wrapper code.
The first linting decision is whether to use a dedicated set of focused linters or adopt a universal linter bundle that covers many languages through one tool.

Should `slsa-builder` use a dedicated linter toolchain, MegaLinter, Super-Linter, or Trunk Check as its primary linting strategy?

## Decision Drivers

- Cover Markdown, Go, and GitHub Actions as required repository surfaces.
- Keep linting behavior explicit, auditable, and easy to reason about in a supply-chain security project.
- Avoid a large default linting container or orchestrator when the repository currently has a small, deliberate language surface.
- Allow precise tool version pinning, updates, and security review for each linter.
- Preserve room to add profile-local linters later without changing the core linting strategy.
- Make local developer usage and CI usage consistent enough to support spec-driven development.
- Prefer focused checks that understand their target domain over broad default coverage.

## Considered Options

- Use a dedicated linter toolchain.
- Use MegaLinter.
- Use Super-Linter.
- Use Trunk Check.

## Decision Outcome

Chosen option: "Use a dedicated linter toolchain", because it keeps linting explicit and reviewable while allowing the project to select best-fit tools for Markdown, Go, GitHub Actions, shell glue, and future profile-specific code.

This ADR decides the linting strategy only.
Concrete tool choices for each required surface, such as Markdown, Go, and GitHub Actions, should be recorded separately before implementation.
The expected follow-up is to choose focused linters for each language or file type rather than enabling a broad bundle by default.

### Consequences

- Good, because each linter can be selected for its domain-specific accuracy and maintained independently.
- Good, because the repository can pin and review each linting dependency instead of inheriting a large bundled toolchain.
- Good, because CI behavior can stay small and transparent for a security-sensitive project.
- Good, because future ecosystem profiles can add profile-local linters without forcing all repositories or profiles into one universal linter configuration.
- Neutral, because multiple configuration files and commands will be required.
- Bad, because initial setup requires more deliberate decisions than adopting one universal linter bundle.
- Bad, because the project must maintain a small amount of orchestration for running the selected linters consistently.

### Confirmation

This decision is confirmed when follow-up documentation or configuration:

- selects specific linters for Markdown, Go, and GitHub Actions;
- documents how each selected linter is installed, pinned, and run locally and in CI;
- keeps universal linter bundles out of the default lint path unless a future ADR changes this decision;
- defines how additional profile-local linters may be added without expanding the core linting surface unnecessarily.

Implementation review should verify that linting additions remain focused and that broad linter bundles are not introduced as the primary linting path without a new decision record.

## Pros and Cons of the Options

### Use a dedicated linter toolchain

Select focused tools for each relevant surface, such as documentation, Go code, GitHub Actions workflows, shell glue, and any future profile-local wrapper language.

- Good, because each tool can be chosen for the exact file type or language it understands best.
- Good, because version pinning, dependency review, and update cadence remain visible per tool.
- Good, because this fits ADR 0004's preference for a small, explicit toolchain around a Go-first implementation.
- Good, because specialized GitHub Actions linters can check workflow semantics and security properties that generic YAML linters cannot fully understand.
- Neutral, because this strategy requires a small wrapper command, Make target, script, or CI job layout to run all checks consistently.
- Bad, because developers must know more than one lint command unless the repository provides a convenience entrypoint.

### Use MegaLinter

Use MegaLinter as the primary linting entrypoint and configure it to run the relevant embedded linters.

- Good, because it supports many languages and formats and can scale to broad multi-language repositories.
- Good, because it can run many established linters through one CI integration.
- Good, because it provides reporting and configuration features for large repositories.
- Neutral, because it may become useful later if this repository grows many unrelated profile-local languages.
- Bad, because it adds a large linting image and broad tool surface before the project needs it.
- Bad, because the effective linter versions and defaults are mediated through the bundle.
- Bad, because it is less aligned with the project's preference for small, explicit, security-reviewed tooling.

### Use Super-Linter

Use Super-Linter as the primary linting entrypoint and configure its supported linters for the repository.

- Good, because it is widely used in GitHub Actions and supports many languages, including Markdown, Go, GitHub Actions, and shell.
- Good, because it provides a quick path to broad lint coverage.
- Neutral, because it may be appropriate for repositories that value fast broad coverage over precise tool ownership.
- Bad, because it is still a large bundled linting surface relative to this repository's initial needs.
- Bad, because focused tools run directly are easier to audit, configure, and pin for a supply-chain security project.
- Bad, because default bundle behavior may include checks unrelated to the current architecture or profile set.

### Use Trunk Check

Use Trunk Check as the primary lint orchestrator for local and CI linting.

- Good, because it provides a strong developer experience for running multiple linters consistently.
- Good, because it can manage linter versions and editor integration across many languages.
- Good, because it can reduce friction as the number of linters grows.
- Neutral, because it could be reconsidered later if local developer workflow becomes more important than direct tool transparency.
- Bad, because it introduces an additional orchestration platform instead of directly documenting and running the selected linters.
- Bad, because the repository's linting policy becomes coupled to Trunk configuration semantics.
- Bad, because it is more abstraction than the current repository needs for Markdown, Go, GitHub Actions, and minimal shell glue.

## More Information

This decision intentionally does not choose the concrete linters for Markdown, Go, GitHub Actions, shell, YAML, or prose.
Those choices should follow this ADR and should prefer focused tools with clear local and CI invocation.

The decision is consistent with earlier ADRs that keep the trusted project surface small and explicit: ADR 0003 chooses profile-owned reusable workflows, and ADR 0004 chooses Go as the primary implementation language with shell limited to glue.
