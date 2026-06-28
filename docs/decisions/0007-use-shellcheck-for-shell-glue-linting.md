---
parent: Decisions
nav_order: 7
status: accepted
date: 12026-06-23
decision-makers: Yunseo Kim
---

# Use ShellCheck for Shell Glue Linting

## Context and Problem Statement

ADR 0004 limited shell usage to minimal workflow glue rather than security-critical provenance,
subject, digest, or policy logic. ADR 0005 chose a dedicated linter toolchain instead of a universal
linter bundle. The project still needs linting for any standalone shell scripts and shell snippets
used as glue around the Go implementation and GitHub Actions workflows.

Should `slsa-builder` use ShellCheck, shell built-in syntax checks, checkbashisms, bashate, or
Shellharden as the primary linter for shell glue?

## Decision Drivers

- Reduce shell-specific risk from quoting, word splitting, globbing, masked exit codes, and fragile
  conditionals.
- Keep shell glue short, explicit, and auditable.
- Use a shell-specific analyzer rather than a broad universal linter bundle.
- Support both local development and CI with the same command behavior.
- Allow precise version pinning and reviewed suppressions.
- Keep formatting, workflow semantics, and workflow security analysis separate from shell linting.

## Considered Options

- Use ShellCheck for shell glue linting.
- Use only shell built-in syntax checks.
- Use checkbashisms for POSIX portability checks.
- Use bashate for Bash style checking.
- Use Shellharden as the primary shell hardening tool.

## Decision Outcome

Chosen option: "Use ShellCheck for shell glue linting", because ShellCheck is the mature shell
static analyzer that directly targets the classes of bugs most likely to make shell glue unsafe or
surprising.

ShellCheck should be the primary linter for standalone shell scripts and shell glue snippets that
can be checked directly. It should be installed or invoked at a pinned version in CI. Suppressions
should use specific ShellCheck rule IDs and should include a justification when the reason is not
obvious.

`shfmt` is not a competing linter option in this decision. It should be considered separately as the
formatter for shell scripts and shell snippets.

`actionlint` is not a competing shell linter option in this decision. It should be considered in the
GitHub Actions workflow linting decision, where it can also apply ShellCheck to workflow `run:`
blocks when ShellCheck is available.

`zizmor` is not a competing shell linter option in this decision. It should be considered as a
GitHub Actions workflow security analyzer rather than as shell glue linting.

### Consequences

- Good, because ShellCheck catches high-impact shell mistakes around quoting, globbing, command
  substitution, exit-code handling, and portability.
- Good, because ShellCheck is focused on shell scripts and fits ADR 0005's dedicated-tool strategy.
- Good, because ShellCheck can be used locally, in editors, and in CI.
- Good, because ShellCheck diagnostics are rule-ID based and can support reviewed suppressions.
- Good, because the decision reinforces ADR 0004's constraint that shell remains minimal glue.
- Neutral, because ShellCheck does not replace formatting, workflow linting, or workflow security
  analysis.
- Bad, because ShellCheck can report warnings for intentional shell patterns and therefore needs a
  suppression policy.
- Bad, because ShellCheck is an additional CI/developer tool that must be pinned and maintained.

### Confirmation

This decision is confirmed when shell linting configuration and CI usage:

- run ShellCheck on repository-owned shell scripts and applicable shell glue files;
- install or invoke a pinned ShellCheck version;
- fail on unsuppressed ShellCheck findings at the chosen severity level;
- use specific ShellCheck suppression rule IDs rather than broad blanket disables;
- keep formatting checks separate, for example through `shfmt` if adopted;
- keep GitHub Actions workflow semantics and workflow security checks in separate decisions.

Implementation review should verify that new shell remains small glue and that complex behavior
moves into Go instead of accumulating in shell scripts.

## Pros and Cons of the Options

### Use ShellCheck for shell glue linting

Use ShellCheck as the primary static analyzer for shell scripts and shell glue.

- Good, because it is the de facto standard shell static analyzer.
- Good, because it directly detects common shell hazards such as unquoted variables, unsafe word
  splitting, masked command failures, fragile tests, and non-portable constructs.
- Good, because it supports Bash and POSIX shell analysis based on script syntax and shebangs.
- Good, because it has broad editor, CI, and pre-commit integration.
- Good, because upstream SLSA GitHub workflow tooling has used ShellCheck as part of linting.
- Neutral, because ShellCheck may be applied directly to standalone scripts and indirectly to
  workflow `run:` blocks through workflow-aware tooling.
- Bad, because intentional exceptions require explicit suppressions.
- Bad, because it does not enforce formatting or fully understand GitHub Actions workflow semantics
  by itself.

### Use only shell built-in syntax checks

Use shell parser checks such as `bash -n` or `sh -n` without an external linter.

- Good, because no additional lint dependency is required.
- Good, because syntax-only checks are fast and simple.
- Neutral, because syntax checks can still be useful as an additional sanity check.
- Bad, because syntax checks miss most shell-specific failure modes that matter for secure glue
  code.
- Bad, because they do not catch unquoted variables, masked failures, unsafe globbing, or many
  portability issues.
- Bad, because this option does not sufficiently reduce the shell fragility called out in ADR 0004.

### Use checkbashisms for POSIX portability checks

Use checkbashisms to detect Bash-specific features in scripts intended to run as `/bin/sh`.

- Good, because it is focused on POSIX portability for `/bin/sh` scripts.
- Good, because it can help avoid accidental Bash dependencies when a script claims to be portable
  shell.
- Neutral, because it may become useful if the project explicitly chooses POSIX `/bin/sh`
  compatibility for some scripts.
- Bad, because the project's shell glue can intentionally use Bash where GitHub Actions provides
  Bash semantics.
- Bad, because it overlaps with ShellCheck portability diagnostics for shebang-selected shells.
- Bad, because POSIX portability is not the same as general shell safety linting.

### Use bashate for Bash style checking

Use bashate to enforce Bash style rules such as whitespace, line length, and selected structural
conventions.

- Good, because it provides style-oriented checks for Bash scripts.
- Good, because it may be familiar in OpenStack-derived shell-heavy projects.
- Neutral, because it could supplement a project with a strong Bash style policy.
- Bad, because it has a narrower semantic safety scope than ShellCheck.
- Bad, because style checking is less important than catching fragile shell behavior in this
  project.
- Bad, because formatting should be handled by a formatter such as `shfmt` rather than by a style
  linter when possible.

### Use Shellharden as the primary shell hardening tool

Use Shellharden to highlight or transform shell scripts, especially around quoting.

- Good, because it focuses on hardening unsafe shell quoting patterns.
- Good, because it can help with cleanup of existing scripts after human review.
- Neutral, because it may be useful as an optional manual refactoring helper.
- Bad, because it is a transformer and hardening helper rather than the primary shell linter.
- Bad, because automated transformations can change behavior when scripts intentionally rely on word
  splitting or glob expansion.
- Bad, because ShellCheck remains the stronger default for broad shell static analysis.

## More Information

This decision refines ADR 0005 for shell glue only. It does not choose linters for Markdown, GitHub
Actions workflows, YAML, prose, Go, or profile-local languages.

Shell formatting should be decided separately. If adopted, `shfmt` should be used as a formatter
rather than as a replacement for ShellCheck.

GitHub Actions workflow analysis should also be decided separately. If `actionlint` is adopted for
workflow linting, its ShellCheck integration can complement this decision for inline `run:` blocks.
If `zizmor` is adopted, it should be treated as workflow security analysis rather than shell
linting.
