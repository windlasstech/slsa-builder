---
parent: Decisions
nav_order: 22
status: accepted
date: 12026-07-01
decision-makers: Yunseo Kim
---

# Use `js-ts-npm-package-slsa3.yml` as the Initial JS/TS npm Package Workflow Entrypoint

## Context and Problem Statement

ADR 0002 chose an extensible trusted reusable workflow foundation. ADR 0003 refined that foundation
into a thin shared core with profile-owned reusable workflows. ADR 0013 through ADR 0018 scoped the
initial JS/TS profile to one npm registry package per profile run, and ADR 0020 decided that
production `builder.id` values identify the released reusable workflow file plus full Git tag ref.
ADR 0021 then separated that versioned workflow identity from the profile-specific `buildType` URI.

The next public-contract decision is the concrete reusable workflow filename and caller entrypoint
for the initial JS/TS npm package profile. Because the workflow filename appears in the caller's
`jobs.<job_id>.uses` reference and in the SLSA provenance `runDetails.builder.id`, the filename is
not just an internal implementation detail. It is part of the verifier-visible trusted builder
identity.

What reusable workflow filename and entrypoint should the initial JS/TS npm package profile expose?

## Decision Drivers

- Make the public entrypoint identify the profile scope: JS/TS npm package releases.
- Make the SLSA Build L3-oriented trust mode visible in the workflow identity.
- Keep the filename aligned with ADR 0020's reusable-workflow-ref `builder.id` pattern.
- Keep the filename aligned with ADR 0021's `js-ts-npm-package` build type profile segment without
  confusing `builder.id` and `buildType`.
- Preserve a clear naming pattern for future profile-owned reusable workflows.
- Avoid a generic publish-oriented filename that could hide the trusted builder boundary.
- Avoid upstream-compatible naming that overstates compatibility with
  `slsa-framework/slsa-github-generator`.

## Considered Options

- Use `.github/workflows/js-ts-npm-package-slsa3.yml`.
- Use `.github/workflows/npm-package-slsa3.yml`.
- Use `.github/workflows/builder_nodejs_slsa3.yml`.
- Use `.github/workflows/publish-npm-package.yml`.
- Use `.github/workflows/js-ts-npm-package.yml`.

## Decision Outcome

Chosen option: "Use `.github/workflows/js-ts-npm-package-slsa3.yml`", because it names the initial
profile and artifact scope directly, makes the SLSA Build L3-oriented trust mode visible, and
matches the `builder.id` example already selected in ADR 0020.

The initial JS/TS npm package profile should expose this reusable workflow entrypoint:

```text
.github/workflows/js-ts-npm-package-slsa3.yml
```

Callers should invoke the released workflow using the GitHub reusable workflow `uses` syntax:

```yaml
jobs:
  publish:
    uses: windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@vX.Y.Z
```

The production SLSA provenance emitted by this profile should use a `builder.id` shaped like:

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/tags/vX.Y.Z
```

The workflow should be the public `workflow_call` entrypoint for the initial profile. Caller
workflows own their local trigger choices, such as `push`, `release`, or `workflow_dispatch`, but
they interact with this trusted profile through declared workflow inputs and secrets only. The
workflow filename decision does not by itself decide the full input, output, secret, runner,
permission, or version-pinning contract; those details remain part of the forthcoming reusable
workflow contract specification or follow-up ADRs.

If a future workflow has materially different security properties, isolation guarantees, signing
behavior, runner trust, artifact scope, or claimed SLSA Build level, it should use a distinct
workflow filename and therefore a distinct `builder.id`. Preview, experimental, SLSA Build L2, or
non-claiming workflows should not reuse this production SLSA3 entrypoint name unless they satisfy
the same trust contract.

### Consequences

- Good, because the filename is explicit about the JS/TS npm package profile selected by ADR 0013.
- Good, because the `slsa3` suffix makes the intended trust mode visible in caller workflows,
  provenance, and verifier policy.
- Good, because the chosen name matches ADR 0020's example `builder.id` and ADR 0021's build type
  profile segment.
- Good, because future profile filenames can follow the same pattern of `<profile>-slsa3.yml`.
- Good, because generic publish helpers, baseline attestation workflows, and experimental workflows
  can be given separate names and builder identities.
- Neutral, because the filename is longer than shorter alternatives such as `npm-package-slsa3.yml`.
- Bad, because including `slsa3` in the filename commits the production entrypoint to satisfying the
  documented SLSA Build L3-oriented trust contract before release.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification and
implementation define:

- `.github/workflows/js-ts-npm-package-slsa3.yml` as the public reusable workflow entrypoint;
- `on.workflow_call` as the supported entry mechanism for callers;
- caller examples using
  `windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@vX.Y.Z`;
- production `builder.id` values using the workflow file path plus full `refs/tags/vX.Y.Z` ref;
- separate filenames for preview, experimental, lower-assurance, or non-production workflows;
- review checks that prevent arbitrary caller-defined steps, services, defaults, or environment from
  becoming part of this trusted workflow entrypoint.

Implementation review should verify that production provenance does not emit a different workflow
filename in `builder.id` unless a later ADR changes this public entrypoint decision.

## Pros and Cons of the Options

### Use `.github/workflows/js-ts-npm-package-slsa3.yml`

Expose the initial profile as a profile-scoped reusable workflow whose name includes the JS/TS npm
package artifact scope and the SLSA3 trust mode.

- Good, because it is the most direct match for the existing ADR language: JS/TS profile, npm
  package artifact, and SLSA Build L3-oriented builder boundary.
- Good, because it keeps `builder.id` verifier policies readable and profile-specific.
- Good, because it avoids implying that this workflow is a generic npm publisher or a generic
  Node.js builder.
- Good, because it leaves room for later filenames such as `github-release-asset-slsa3.yml`,
  `oci-image-slsa3.yml`, or lower-assurance variants.
- Neutral, because the name is long.
- Bad, because the `slsa3` suffix raises the bar for implementation readiness and documentation.

### Use `.github/workflows/npm-package-slsa3.yml`

Expose the initial profile using a shorter npm-package-focused filename.

- Good, because it is concise and still names the npm package artifact scope.
- Good, because it includes the SLSA3 trust mode.
- Neutral, because it may be adequate if the project wants the artifact type to dominate the profile
  name.
- Bad, because it drops the JS/TS profile context used by the existing ADRs and build type segment.
- Bad, because future non-JS/TS paths that produce npm packages, such as a Deno-to-npm adapter,
  could make the name ambiguous.

### Use `.github/workflows/builder_nodejs_slsa3.yml`

Follow the broad naming style used by `slsa-framework/slsa-github-generator` language builders.

- Good, because it resembles established SLSA GitHub Generator filenames such as
  `builder_go_slsa3.yml` and `builder_nodejs_slsa3.yml`.
- Good, because it clearly signals that the workflow is a builder rather than a post-build
  attestation helper.
- Neutral, because it may ease recognition for users familiar with SLSA GitHub Generator.
- Bad, because `nodejs` is runtime-oriented and less precise than the selected npm package profile.
- Bad, because using the upstream-style filename could imply compatibility with an upstream builder
  contract that this project deliberately did not inherit wholesale.
- Bad, because it is less aligned with the chosen `js-ts-npm-package` build type segment.

### Use `.github/workflows/publish-npm-package.yml`

Use a caller-friendly release or publishing workflow name.

- Good, because it is easy for package maintainers to understand at first glance.
- Good, because it emphasizes the user-facing release action.
- Bad, because it hides the SLSA3 trust boundary from the workflow identity.
- Bad, because it could be confused with a normal caller-controlled publishing workflow.
- Bad, because it is a weak `builder.id` name for verifier policy and security documentation.

### Use `.github/workflows/js-ts-npm-package.yml`

Use the profile name without encoding the SLSA trust mode in the filename.

- Good, because it avoids over-claiming before the implementation is ready to document and satisfy a
  SLSA Build L3-oriented contract.
- Good, because it is shorter than the selected filename.
- Neutral, because the build type URI already carries the `js-ts-npm-package` profile schema name.
- Bad, because it makes security mode separation less visible in `builder.id`.
- Bad, because future SLSA2, preview, or baseline workflows could be tempted to share the same
  filename and blur verifier expectations.

## More Information

This decision complements ADR 0020 and ADR 0021. ADR 0020 makes the reusable workflow file path part
of the production `builder.id`; this ADR fixes that path for the initial JS/TS npm package profile.
ADR 0021 keeps the profile's `buildType` URI separate from the released workflow filename.

Reference points considered:

- GitHub reusable workflows must live directly under `.github/workflows/`, must include
  `on.workflow_call`, and are invoked by callers through
  `{owner}/{repo}/.github/workflows/{filename}@{ref}`.
- SLSA v1.2 provenance defines `runDetails.builder.id` as the trusted build platform identity and
  requires different builder IDs for modes with different security attributes or SLSA Build levels.
- SLSA v1.2 Build L3 requires hosted, isolated builds and provenance that tenants cannot forge or
  tamper with, so the production entrypoint name should not be shared with lower-assurance modes.
- SLSA GitHub Generator uses reusable workflow filenames such as `builder_go_slsa3.yml`,
  `builder_nodejs_slsa3.yml`, and `generator_generic_slsa3.yml`, demonstrating the convention of
  making builder/generator role and SLSA3 mode visible in the workflow filename.
