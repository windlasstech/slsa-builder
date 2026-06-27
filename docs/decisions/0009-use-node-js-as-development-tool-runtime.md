---
parent: Decisions
nav_order: 9
status: accepted
date: 12026-06-24
decision-makers: Yunseo Kim
---

# Use Node.js as the Development Tool Runtime

## Context and Problem Statement

ADR 0004 chose Go as the primary trusted implementation language and kept non-Go code outside the trusted core unless a narrow profile-local wrapper requires it.
ADR 0008 chose Prettier for Markdown formatting.
Using Prettier introduces a development-only JavaScript tooling runtime even though the project's trusted runtime remains Go.

Which JavaScript or TypeScript runtime should `slsa-builder` use for development tooling required by Prettier?

## Decision Drivers

- Keep Node.js, Deno, Bun, or any JavaScript runtime out of the trusted runtime and release artifact execution path.
- Use the runtime that best matches Prettier's supported and documented local-install workflow.
- Support exact Prettier version pinning for local development, editor integrations, and CI.
- Preserve clear dependency-review and lockfile workflows for a supply-chain security project.
- Minimize contributor surprise by choosing a widely supported runtime for JavaScript-based developer tools.
- Decide the runtime before deciding the package manager.

## Considered Options

- Use Node.js as the development tool runtime.
- Use Deno as the development tool runtime.
- Use Bun as the development tool runtime.

## Decision Outcome

Chosen option: "Use Node.js as the development tool runtime", because Node.js is the most standard and least surprising runtime for project-local Prettier installation, editor integration, CI execution, lockfile review, and dependency automation.

Node.js is adopted only for development tooling required by Prettier and other future JavaScript-based developer tools that are explicitly accepted by follow-up decisions.
It is not part of the trusted core selected in ADR 0004.
It should not be required to execute released `slsa-builder` binaries, reusable workflow trusted logic, provenance generation, subject or digest handling, or verifier behavior.

The Node.js version should be pinned in local and CI configuration.
Prettier should be installed as an exact project-local development dependency so editors and CI use the repository-selected formatter version.

This ADR does not choose the Node.js package manager.
The choice between npm, pnpm, Yarn, or another package manager should be made in a follow-up decision.

### Consequences

- Good, because Node.js matches Prettier's most common project-local installation and execution model.
- Good, because editor integrations commonly discover and use a project-local Prettier installation from the Node package layout.
- Good, because GitHub Actions setup, cache behavior, dependency review, vulnerability scanning, and Dependabot support are mature for Node package workflows.
- Good, because the runtime boundary is explicit: Node.js is development-only and Go remains the trusted runtime.
- Neutral, because the package manager remains undecided and must be recorded separately before implementation.
- Bad, because the repository gains a JavaScript runtime for development even though its primary implementation language is Go.
- Bad, because Node.js and its package ecosystem require lockfile discipline, exact version pinning, and dependency review.

### Confirmation

This decision is confirmed when development tooling configuration and CI usage:

- pin a Node.js version for development and CI;
- install Prettier as an exact project-local development dependency;
- run Prettier from the project-local installation rather than an implicitly downloaded latest version;
- document that Node.js is development-only and not part of the trusted runtime or release artifact execution path;
- defer package-manager-specific behavior to a follow-up ADR;
- avoid adding JavaScript or TypeScript trusted runtime code without a separate ADR or profile-local justification.

Implementation review should verify that Node.js is introduced only to support development tooling and that the Go-first trusted-runtime boundary from ADR 0004 remains intact.

## Pros and Cons of the Options

### Use Node.js as the development tool runtime

Use Node.js to run Prettier and other explicitly accepted JavaScript-based development tools.

- Good, because Prettier is distributed and documented primarily through project-local package installation workflows in the Node ecosystem.
- Good, because `npm exec`, `pnpm exec`, `yarn exec`, and similar package-manager commands can run the repository's pinned local Prettier version.
- Good, because editor integrations are most likely to use the expected project-local Prettier version with a Node package installation.
- Good, because GitHub Actions and dependency automation have mature support for Node.js, package-manager caches, lockfiles, and dependency updates.
- Good, because contributors are likely to recognize Node.js as the expected runtime for Prettier.
- Neutral, because Node.js still leaves package-manager selection open.
- Bad, because it adds a development runtime that is not otherwise needed by the Go trusted core.
- Bad, because npm ecosystem dependency hygiene becomes part of development-tool maintenance.

### Use Deno as the development tool runtime

Use Deno to run Prettier through Deno tasks, npm compatibility, or another Deno-managed workflow.

- Good, because Deno has an appealing single-binary model and a security-focused runtime design.
- Good, because Deno can reduce reliance on a traditional Node.js installation for some JavaScript tooling workflows.
- Neutral, because Deno's npm compatibility may be sufficient for running Prettier in some setups.
- Bad, because Prettier's most common local install, editor, and CI workflows are not Deno-first.
- Bad, because using Deno only for Prettier would make a simple formatter dependency more specialized for contributors.
- Bad, because dependency automation and lockfile review for this Prettier-only use case would be less conventional than a Node package workflow.
- Bad, because the project is not otherwise using Deno, so Deno's application-runtime advantages would not be exercised.

### Use Bun as the development tool runtime

Use Bun to run Prettier and manage the JavaScript development dependency surface.

- Good, because Prettier documents Bun-based installation and execution commands.
- Good, because Bun can provide both runtime and package-manager behavior in one tool.
- Good, because Bun is fast and increasingly compatible with Node package workflows.
- Neutral, because Bun may become more attractive if a future profile chooses Bun-specific package-manager behavior.
- Bad, because Prettier formatting does not benefit materially from Bun's runtime performance.
- Bad, because choosing Bun would couple the runtime decision closely to a package-manager decision that this ADR intentionally defers.
- Bad, because Node.js remains more familiar and broadly supported for Prettier editor, CI, and dependency automation workflows.
- Bad, because the project is not otherwise building a Bun application or Bun-specific profile at this stage.

## More Information

This decision follows ADR 0008 and exists only because Prettier was selected for Markdown formatting.
It does not change ADR 0004's choice of Go as the primary trusted implementation language.

The package-manager decision should follow this ADR.
That decision should compare npm, pnpm, Yarn, and any other relevant package-manager options for exact Prettier installation, lockfile behavior, CI reproducibility, dependency review, and contributor experience.
