---
parent: Decisions
nav_order: 8
status: accepted
date: 12026-06-24
decision-makers: Yunseo Kim
---

# Use Dedicated Formatters for Go, Markdown, and Shell

## Context and Problem Statement

ADR 0004 chose Go as the primary implementation language and limited shell to minimal workflow glue.
ADR 0005 chose a dedicated linting toolchain instead of a universal linter bundle, and later ADRs
separated linting from formatting for Go and shell. The repository now needs a formatting decision
that covers at least Go, Markdown, and shell while keeping formatting behavior explicit and
reviewable.

Which formatter combination should `slsa-builder` use for Go, Markdown, and shell files?

## Decision Drivers

- Cover the initial required formatting surfaces: Go, Markdown, and shell.
- Keep formatting separate from linting, vulnerability scanning, workflow linting, and workflow
  security analysis.
- Prefer ecosystem-standard formatters for each file type.
- Keep local and CI formatting behavior reproducible through pinned tool versions where the
  formatter is not part of the Go toolchain.
- Avoid formatting orchestration decisions in this ADR; orchestration can be decided separately if
  needed.
- Avoid broad universal formatter bundles when focused tools are sufficient.

## Considered Options

- Use `gofmt` and `goimports` for Go, Prettier for Markdown, and `shfmt` for shell.
- Use `gofmt` and `goimports` for Go, dprint for Markdown, and `shfmt` for shell.
- Use `gofmt` and `goimports` for Go, mdformat for Markdown, and `shfmt` for shell.
- Use `gofumpt` and `goimports` for Go, Prettier for Markdown, and `shfmt` for shell.

## Decision Outcome

Chosen option: "Use `gofmt` and `goimports` for Go, Prettier for Markdown, and `shfmt` for shell",
because this combination uses the most standard formatter for Go, a widely adopted Markdown
formatter with strong editor and CI support, and the established shell formatter.

Go source formatting should use `gofmt`. Go import grouping and cleanup should use `goimports` when
import normalization is needed. Markdown formatting should use Prettier. Shell script and shell glue
formatting should use `shfmt`.

Formatter versions outside the Go toolchain should be pinned for CI and local reproducibility.
Prettier should be scoped to Markdown formatting by this decision; formatting YAML, JSON,
JavaScript, TypeScript, or other file types requires either explicit configuration under this
decision's intent or a follow-up decision when the surface becomes important.

This ADR does not choose a formatter orchestrator. Tools such as `treefmt`, `pre-commit`, Lefthook,
Make targets, or custom scripts may later coordinate formatter execution, but they are not formatter
alternatives and are outside this decision.

### Consequences

- Good, because Go formatting follows the official Go formatter and common import-normalization
  practice.
- Good, because Prettier is broadly adopted for Markdown and has strong editor, local, and CI
  integration.
- Good, because `shfmt` is the established formatter for shell scripts and complements ADR 0007's
  ShellCheck linting decision.
- Good, because each selected formatter is focused on a specific surface instead of hiding behavior
  behind a universal bundle.
- Good, because formatting remains clearly separate from linting and vulnerability-scanning
  decisions made in ADR 0006 and ADR 0007.
- Neutral, because Prettier introduces a Node-based developer tool for Markdown formatting even
  though Node is not part of the trusted runtime selected in ADR 0004.
- Bad, because multiple formatter commands or a separate orchestration layer will be needed for
  convenient local and CI usage.
- Bad, because Prettier's Markdown wrapping and normalization may create larger documentation diffs
  when first applied.

### Confirmation

This decision is confirmed when formatting configuration and CI usage:

- run `gofmt` on Go source files;
- run `goimports` where import normalization is required;
- run Prettier on Markdown files;
- run `shfmt` on repository-owned shell scripts and shell glue files;
- pin Prettier and `shfmt` versions or installation sources for CI reproducibility;
- keep formatter checks separate from vulnerability scanning, GitHub Actions workflow analysis, and
  other linting responsibilities;
- avoid treating formatter orchestrators as substitutes for the selected formatters.

Implementation review should verify that formatting failures can be reproduced locally and that new
file types are not silently formatted without an explicit decision or clearly scoped configuration.

## Pros and Cons of the Options

### Use `gofmt` and `goimports` for Go, Prettier for Markdown, and `shfmt` for shell

Use Go-standard formatting for Go source, Prettier for Markdown documentation, and `shfmt` for shell
scripts and glue.

- Good, because `gofmt` is the canonical Go formatter.
- Good, because `goimports` handles common import cleanup without replacing `gofmt`'s formatting
  model.
- Good, because Prettier is widely used for Markdown and well supported by editors and CI
  environments.
- Good, because `shfmt` is purpose-built for shell formatting and pairs well with ShellCheck.
- Good, because the combination is easy for contributors to recognize.
- Neutral, because Prettier can format many other file types, but this decision scopes it to
  Markdown.
- Bad, because Prettier normally brings Node/npm-based developer tooling into the repository's
  formatter setup.
- Bad, because Markdown formatting may produce noisy first-use diffs if existing documents were
  manually wrapped.

### Use `gofmt` and `goimports` for Go, dprint for Markdown, and `shfmt` for shell

Use dprint for Markdown instead of Prettier while keeping Go and shell formatters unchanged.

- Good, because dprint can provide Markdown formatting without requiring a Node/npm toolchain.
- Good, because dprint's CLI and WebAssembly plugin model can be pinned explicitly.
- Good, because it can reduce JavaScript ecosystem dependency exposure for a Go-first supply-chain
  project.
- Neutral, because dprint still introduces an external formatter binary and Markdown plugin
  artifact.
- Bad, because dprint is less familiar than Prettier for many Markdown contributors.
- Bad, because plugin URL, cache, checksum, or vendoring policy would need careful documentation for
  strict reproducibility.
- Bad, because Prettier has broader editor and contributor familiarity for Markdown formatting.

### Use `gofmt` and `goimports` for Go, mdformat for Markdown, and `shfmt` for shell

Use mdformat as a Markdown-specific formatter while keeping Go and shell formatters unchanged.

- Good, because mdformat is focused specifically on Markdown formatting.
- Good, because it avoids using a broad JavaScript formatter for Markdown.
- Neutral, because it may fit Python-heavy documentation workflows.
- Bad, because it introduces Python tooling for a repository whose primary implementation language
  is Go.
- Bad, because it is less common than Prettier in general GitHub Actions and open-source Markdown
  workflows.
- Bad, because contributor and editor support is likely weaker than Prettier for this repository's
  expected audience.

### Use `gofumpt` and `goimports` for Go, Prettier for Markdown, and `shfmt` for shell

Use `gofumpt` as a stricter Go formatter while keeping Prettier and `shfmt` for Markdown and shell.

- Good, because `gofumpt` provides stricter Go formatting while staying close to `gofmt`.
- Good, because stricter formatting can reduce subjective style discussion in Go code review.
- Neutral, because `gofumpt` may be reconsidered later if the Go codebase benefits from stricter
  formatting.
- Bad, because `gofumpt` is more opinionated than the official Go formatter.
- Bad, because the repository is still early and does not need stricter-than-standard Go formatting
  yet.
- Bad, because ADR 0006 already prefers conservative Go tooling decisions before adding stricter
  checks.

## More Information

This decision covers formatter selection only. It intentionally does not choose a command runner,
pre-commit framework, CI job layout, or formatter orchestration tool.

This decision also does not choose formatters for YAML, JSON, GitHub Actions workflow files,
JavaScript, TypeScript, generated files, or future profile-local languages. Those surfaces should be
handled through explicitly scoped configuration or a future ADR when they become part of the
maintained formatting surface.

Linting remains governed by ADR 0005, ADR 0006, and ADR 0007. Formatting failures should not be
treated as linter findings unless a later implementation explicitly defines a combined check command
for developer convenience.
