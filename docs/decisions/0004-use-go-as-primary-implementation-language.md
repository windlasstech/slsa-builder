---
parent: Decisions
nav_order: 4
status: accepted
date: 12026-06-23
decision-makers: Yunseo Kim
---

# Use Go as the Primary Implementation Language

## Context and Problem Statement

ADR 0003 chose a thin core with profile-owned reusable workflows. The GitHub Actions workflow YAML
files are the orchestration envelope, but the trusted implementation still needs a primary
programming language for provenance generation, subject and digest handling, profile validation, and
workflow integration. The language choice must keep the trusted computing base small while allowing
ecosystem profiles to remain practical.

Should `slsa-builder` implement its trusted runtime primarily in Go, TypeScript, Rust, Python, or
shell?

## Decision Drivers

- Keep the trusted core small, auditable, and easy to distribute in GitHub Actions workflows.
- Prefer standard-library implementations where practical to reduce third-party dependency risk.
- Align with SLSA, Sigstore, in-toto, provenance, and verifier tooling ecosystems.
- Support multiple ecosystem profiles.
- Preserve a simple execution model inside reusable workflows and composite actions.
- Avoid relying on shell for security-critical JSON, digest, provenance, and error-handling logic.
- Allow profile-specific wrappers in another language only when they materially reduce complexity or
  risk.

## Considered Options

- Use Go as the primary implementation language.
- Use TypeScript as the primary implementation language.
- Use Rust as the primary implementation language.
- Use Python as the primary implementation language.
- Use shell as the primary implementation language.

## Decision Outcome

Chosen option: "Use Go as the primary implementation language", because Go provides a strong fit for
SLSA and supply-chain tooling, simple binary distribution, good standard-library coverage, and a
small runtime surface for trusted workflow execution.

The project should implement as much trusted logic as practical in Go using the standard library
before adding third-party dependencies. Shell should be limited to minimal workflow glue, such as
invoking the Go binary, passing file paths, or connecting GitHub Actions steps. Language or
ecosystem profiles may introduce thin wrappers in another language, such as TypeScript for
JS/TS-specific GitHub Action or package-manager ergonomics, but those wrappers should remain
optional, profile-local, and as small as possible.

The initial implementation should treat Go as the default for:

- SLSA provenance v1 predicate and statement construction;
- subject naming and digest normalization;
- package or artifact manifest parsing where the format is simple and documented;
- profile contract validation;
- GitHub Actions file-command integration, such as outputs, environment files, summaries, and
  annotations;
- verifier helper commands and fixture generation.

### Consequences

- Good, because Go can produce a compact command-line binary that reusable workflows can invoke
  directly.
- Good, because Go keeps the shared core language-neutral across JS/TS, generic artifact, container,
  JVM, or other future profiles.
- Good, because many SLSA, Sigstore, Cosign, and verifier tools already use Go or provide strong Go
  libraries.
- Good, because the standard library covers much of the required trusted core: JSON, hashing,
  archive handling, process execution, HTTP, and CLI-oriented filesystem work.
- Good, because limiting shell to glue reduces quoting, parsing, and error-handling risk in
  security-sensitive paths.
- Neutral, because profile wrappers in TypeScript or another language may still be justified when
  ecosystem-specific tooling integration would otherwise be fragile.
- Bad, because GitHub JavaScript Action ergonomics and Actions Toolkit integration are less direct
  than they would be in TypeScript.
- Bad, because some JS/TS package-manager behavior may require carefully invoking and parsing
  external tools rather than reusing npm ecosystem libraries directly.

### Confirmation

This decision is confirmed when the initial implementation:

- contains a Go command or library as the primary trusted runtime;
- keeps shell scripts short and limited to workflow glue rather than provenance, subject, digest, or
  policy logic;
- documents any non-Go profile wrapper with a reason it is needed and the boundary of what trusted
  logic it may contain;
- requires explicit review before adding third-party Go dependencies to trusted core code;
- avoids adding TypeScript, Python, Rust, or shell implementations for core behavior without a
  follow-up ADR or documented profile-local justification.

Implementation review should verify that security-critical behavior remains in Go unless an
exception is narrow, profile-local, and explicitly documented.

## Pros and Cons of the Options

### Use Go as the primary implementation language

Implement trusted core commands and libraries in Go, with reusable workflows invoking Go binaries
directly or through minimal composite-action glue.

- Good, because Go is widely used by SLSA, Sigstore, Cosign, and verifier tooling.
- Good, because Go binaries are easy to build, test, cross-compile, and run inside GitHub-hosted
  runners.
- Good, because Go avoids requiring Node, Python, Rust toolchains, or package installations at
  action runtime for core behavior.
- Good, because Go's standard library is sufficient for much of the required JSON, hashing, HTTP,
  archive, and process integration work.
- Good, because the core stays neutral across package ecosystems.
- Neutral, because third-party Go dependencies may still be needed for Sigstore, in-toto, or GitHub
  API integration, but they should be added deliberately.
- Bad, because Go is less native than TypeScript for JavaScript Action UX and GitHub Actions Toolkit
  APIs.

### Use TypeScript as the primary implementation language

Implement the trusted runtime as JavaScript or TypeScript actions using the GitHub Actions Toolkit
and npm ecosystem packages.

- Good, because TypeScript is the most natural language for JavaScript Actions and GitHub Actions
  Toolkit integration.
- Good, because the first JS/TS profile could reuse npm ecosystem libraries for package metadata,
  workspaces, semver, and package URLs.
- Good, because GitHub-maintained actions such as `actions/attest` demonstrate this model.
- Neutral, because thin TypeScript wrappers can remain useful for profile-local ergonomics.
- Bad, because using TypeScript for the trusted core increases npm dependency and bundling exposure
  in a project that exists to improve supply-chain trust.
- Bad, because `dist` bundle generation and dependency review become part of the trusted release
  process.
- Bad, because a TypeScript core would make the shared foundation feel JS/TS-specific despite the
  profile-extensible architecture.

### Use Rust as the primary implementation language

Implement the trusted runtime in Rust for strong type and memory safety guarantees.

- Good, because Rust provides memory safety, strong static typing, and efficient single-binary
  distribution.
- Good, because Rust is attractive for cryptographic and verification-heavy tooling.
- Neutral, because Rust may become appropriate for a future standalone verifier or cryptographic
  component.
- Bad, because the SLSA and Sigstore practical ecosystem is currently stronger in Go.
- Bad, because Rust may slow initial implementation and reduce contributor accessibility compared
  with Go.
- Bad, because GitHub Actions integration still needs the same kind of wrapper or CLI invocation
  model as Go.

### Use Python as the primary implementation language

Implement the trusted runtime in Python scripts or packages.

- Good, because Python is fast for prototyping and has strong JSON, YAML, CLI, and testing
  ergonomics.
- Good, because Python can be useful for local analysis scripts or fixture generation.
- Bad, because Python runtime and packaging environments add more moving parts to trusted GitHub
  Actions execution.
- Bad, because producing a simple, portable runtime artifact is harder than with Go.
- Bad, because dependency isolation, lockfile management, and package installation increase trusted
  surface area.

### Use shell as the primary implementation language

Implement most logic directly in shell scripts embedded in workflows or composite actions.

- Good, because shell is always available on Linux GitHub-hosted runners and is convenient for short
  glue steps.
- Good, because it can invoke external ecosystem tools with minimal setup.
- Bad, because shell is fragile for structured JSON, provenance predicates, subject normalization,
  digest maps, and policy validation.
- Bad, because quoting and error handling are difficult to audit in security-critical paths.
- Bad, because shell-heavy implementations make cross-platform behavior, tests, and maintenance
  harder.

## More Information

This decision complements ADR 0003. Profile-owned reusable workflows remain the architecture
boundary, while Go becomes the default implementation language for the trusted code those workflows
invoke.

Use of another language is not forbidden, but it should be treated as a narrow profile-local wrapper
decision rather than a change to the shared core language. For example, a JS/TS profile may later
add a small TypeScript wrapper if GitHub Actions Toolkit or package-manager integration makes that
safer or simpler, but provenance, subject, digest, and profile contract logic should remain in Go by
default.
