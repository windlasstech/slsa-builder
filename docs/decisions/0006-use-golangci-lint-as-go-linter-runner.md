---
parent: Decisions
nav_order: 6
status: accepted
date: 12026-06-23
decision-makers: Yunseo Kim
---

# Use golangci-lint as the Go Linter Runner

## Context and Problem Statement

ADR 0004 chose Go as the primary implementation language. ADR 0005 chose a dedicated linter
toolchain instead of a universal linter bundle. The project now needs a Go-specific linting decision
that keeps local and CI behavior consistent while remaining explicit enough for a supply-chain
security project.

Should `slsa-builder` use `golangci-lint`, only built-in Go tooling, direct standalone Go linters,
or Staticcheck-centered linting as its Go linter runner strategy?

## Decision Drivers

- Keep the Go lint path explicit, reviewable, and Go-specific.
- Align with the current Go ecosystem without adopting a repository-wide universal linter bundle.
- Use a practical default that developers can run locally and CI can run consistently.
- Start with high-signal checks before adding noisier style or security-pattern rules.
- Preserve precise version pinning and configuration review.
- Separate formatting, vulnerability reachability scanning, and security-pattern scanning
  responsibilities clearly.

## Considered Options

- Use `golangci-lint` as the Go linter runner.
- Use only built-in Go tooling.
- Run standalone Go linters directly.
- Use Staticcheck-centered linting.

## Decision Outcome

Chosen option: "Use `golangci-lint` as the Go linter runner", because it provides a Go-specific,
widely used runner with a conservative default set that already includes `govet`, `staticcheck`,
`errcheck`, `ineffassign`, and `unused`.

The initial configuration should be explicit and conservative rather than enabling every available
linter. The baseline should include high-signal checks such as `govet`, `staticcheck`, `errcheck`,
`ineffassign`, and `unused`. Additional linters may be enabled only when they have a clear purpose
and acceptable false-positive behavior for this repository.

`govulncheck` is not a linter and is not a competing option in this decision. It should be adopted
as a separate Go security gate for vulnerability reachability scanning.

`gosec` is also not a competing linter-runner option in this decision. It may be used later as a
security-pattern check, either through `golangci-lint` configuration or through a separate
SARIF-producing job, when that detail is specified.

Formatting and import normalization should remain separate from linting. Go source formatting should
use Go-standard tooling such as `gofmt`, with `goimports` considered where import management is
needed.

### Consequences

- Good, because one Go-specific command can run multiple high-signal Go linters consistently.
- Good, because `golangci-lint` supports explicit configuration and version pinning.
- Good, because this fits ADR 0005 without adopting MegaLinter, Super-Linter, Trunk Check, or
  another universal bundle.
- Good, because the project can start with conservative checks and add stricter checks deliberately.
- Good, because `govulncheck` remains clearly separated as a security gate rather than being
  mislabeled as linting.
- Neutral, because `golangci-lint` is still an aggregation layer over individual linters.
- Bad, because upgrades may change available linters, defaults, or deprecation status and will
  require review.
- Bad, because too many enabled linters could create noise if configuration discipline is not
  maintained.

### Confirmation

This decision is confirmed when Go linting configuration and CI usage:

- install or invoke a pinned `golangci-lint` version;
- use an explicit configuration rather than relying only on undocumented defaults;
- include a conservative baseline centered on `govet`, `staticcheck`, `errcheck`, `ineffassign`, and
  `unused`;
- keep formatting checks separate from linting;
- keep `govulncheck` as a separate vulnerability reachability security gate;
- treat any future `gosec` usage as a security-pattern configuration detail, not as a replacement
  for the Go linter runner decision.

Implementation review should reject broad linter enablement without justification and should verify
that suppressions are narrow, documented, and reviewed.

## Pros and Cons of the Options

### Use `golangci-lint` as the Go linter runner

Use `golangci-lint` as the primary command for Go linting, with an explicit configuration and a
conservative enabled set.

- Good, because it is a Go-specific runner rather than a repository-wide universal linter bundle.
- Good, because it provides one local and CI entrypoint for multiple established Go linters.
- Good, because its default high-signal set includes `govet`, `staticcheck`, `errcheck`,
  `ineffassign`, and `unused`.
- Good, because additional linters can be added later through reviewed configuration changes.
- Good, because it reduces local and CI orchestration compared with running every linter directly.
- Neutral, because it introduces an aggregation layer whose version and behavior must be pinned and
  reviewed.
- Bad, because enabling too many linters can create false-positive noise and slow adoption.

### Use only built-in Go tooling

Use only Go toolchain commands such as `go test`, `go vet`, and Go-standard formatting commands.

- Good, because the trusted tool surface is minimal and tied to the Go toolchain.
- Good, because built-in tools are easy for contributors and CI to run.
- Good, because `go vet` provides official suspicious-code checks.
- Neutral, because this remains a useful baseline even when another linter runner is adopted.
- Bad, because it omits common high-signal ecosystem checks such as `staticcheck`, `errcheck`,
  `ineffassign`, and `unused` beyond what the toolchain already covers.
- Bad, because it is weaker than the linting standard expected for a security-sensitive Go project.

### Run standalone Go linters directly

Run tools such as Staticcheck, Errcheck, Revive, and other Go linters as separate commands without a
runner.

- Good, because each tool is directly visible and independently pinned.
- Good, because there is no aggregation layer between the repository and each linter.
- Good, because individual tools can be updated or removed independently.
- Neutral, because this maximizes transparency at the cost of orchestration effort.
- Bad, because local and CI commands become more fragmented.
- Bad, because output formats, caching, package selection, suppressions, and configuration files
  need separate maintenance.
- Bad, because it recreates coordination that `golangci-lint` already provides for Go linting.

### Use Staticcheck-centered linting

Use Staticcheck as the primary external linter, alongside built-in Go tooling.

- Good, because Staticcheck has strong semantic analysis and low false-positive goals.
- Good, because it is simple to understand and can be run directly in CI.
- Good, because it complements `go vet` well.
- Neutral, because Staticcheck remains part of the selected `golangci-lint` baseline.
- Bad, because it does not cover the broader baseline expected from `errcheck`, `ineffassign`, and
  `unused` as a coordinated set.
- Bad, because future additions would still require either more standalone orchestration or a
  runner.

## More Information

This decision refines ADR 0005 for Go only. It does not choose linters for Markdown, GitHub Actions,
shell, YAML, prose, or any future profile-local language.

`govulncheck` should be documented or configured separately as a Go vulnerability reachability gate
because it answers a different question from linting: whether known vulnerabilities in dependencies
are reachable from this project's code.

`gosec` should be evaluated as a security-pattern check when the project decides how to handle
SARIF, code scanning, and security findings. If enabled through `golangci-lint`, it should still be
understood as a selected linter within the runner, not as a competing runner choice.
