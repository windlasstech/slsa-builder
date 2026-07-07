---
parent: Decisions
nav_order: 40
status: superseded by ADR-0049
date: 12026-07-06
decision-makers: Yunseo Kim
---

# Use `github-release-asset-slsa3.yml` as the GitHub Release Asset Workflow Entrypoint

## Context and Problem Statement

ADR 0002 chose an extensible trusted reusable workflow foundation, and ADR 0003 refined that
foundation into a thin shared core with profile-owned reusable workflows. ADR 0022 selected
`.github/workflows/js-ts-npm-package-slsa3.yml` as the initial JS/TS npm package profile entrypoint.
ADR 0028 later made the reusable workflow file path plus full commit SHA part of the production
`builder.id` trust anchor. ADR 0038 added the GitHub Release asset profile direction, and ADR 0039
scoped that profile to one explicitly named GitHub Release asset per profile run.

The GitHub Release asset profile now needs a concrete reusable workflow filename before its
architecture specification fixes `runDetails.builder.id`, caller examples, and verifier policy.
Since the workflow filename appears in both the caller's `jobs.<job_id>.uses` reference and the SLSA
provenance `runDetails.builder.id`, this is a verifier-visible public contract, not an internal YAML
detail.

What reusable workflow filename and entrypoint should the initial GitHub Release asset SLSA3 profile
expose?

## Decision Drivers

- Make the public entrypoint identify the selected profile scope: one GitHub Release asset per run.
- Keep the workflow path aligned with ADR 0028's SHA-pinned reusable workflow `builder.id` pattern.
- Preserve consistency with ADR 0022's `<profile>-slsa3.yml` naming style for production SLSA3
  profile-owned reusable workflows.
- Make the SLSA Build L3-oriented trust mode visible in caller workflows, provenance, and verifier
  policy.
- Avoid implying that the low-level profile publishes or verifies multiple release assets in one
  run.
- Avoid generic artifact naming that would hide GitHub Release-specific parameters such as release
  tag, asset name, upload target, immutable release behavior, and GitHub attestation storage.
- Leave room for a separate higher-level orchestration workflow for multi-asset releases.

## Considered Options

- Use `.github/workflows/github-release-asset-slsa3.yml`.
- Use `.github/workflows/release-asset-slsa3.yml`.
- Use `.github/workflows/github-release-assets-slsa3.yml`.
- Use `.github/workflows/generic-artifact-slsa3.yml`.
- Use `.github/workflows/github-release-asset.yml`.

## Decision Outcome

Chosen option: "Use `.github/workflows/github-release-asset-slsa3.yml`", because it names the GitHub
Release asset profile directly, uses singular `asset` to reinforce ADR 0039's one-asset-per-run
scope, and follows the existing production profile naming pattern selected for the JS/TS npm package
profile.

The initial GitHub Release asset profile should expose this reusable workflow entrypoint:

```text
.github/workflows/github-release-asset-slsa3.yml
```

Production callers should invoke the released workflow using a full 40-character commit SHA as
selected by ADR 0028:

```yaml
jobs:
  release-asset:
    uses: windlasstech/slsa-builder/.github/workflows/github-release-asset-slsa3.yml@0123456789abcdef0123456789abcdef01234567
```

The production SLSA provenance emitted by this profile should use a `builder.id` shaped like:

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/github-release-asset-slsa3.yml@0123456789abcdef0123456789abcdef01234567
```

The workflow should be the public `workflow_call` entrypoint for the low-level one-asset GitHub
Release asset profile. Caller workflows own their local triggers and orchestration choices, but they
interact with this trusted profile through declared workflow inputs and secrets only. The workflow
filename decision does not decide the full input, output, secret, runner, permission, job graph,
release creation, immutable release, upload, or verification contract; those details belong in the
release asset architecture specification or follow-up ADRs.

A higher-level multi-asset release orchestration workflow, if added later, should use a distinct
workflow filename and should compose this one-asset profile rather than sharing its `builder.id`.
Any preview, experimental, lower-assurance, baseline attestation, generic artifact, or full-release
workflow should also use a distinct filename if it has materially different security properties,
artifact scope, isolation guarantees, signing behavior, runner trust, or claimed SLSA Build level.

### Consequences

- Good, because the filename directly matches the profile selected in ADR 0038 and scoped in
  ADR 0039.
- Good, because singular `asset` makes the one-profile-run-one-asset verification unit visible in
  the builder identity.
- Good, because the `slsa3` suffix clearly marks the production SLSA Build L3-oriented reusable
  workflow contract.
- Good, because the chosen name stays consistent with the existing
  `.github/workflows/js-ts-npm-package-slsa3.yml` profile entrypoint pattern.
- Good, because later multi-asset orchestration, generic artifact, and lower-assurance workflows can
  receive distinct filenames and builder identities.
- Neutral, because the filename is longer than `release-asset-slsa3.yml`.
- Bad, because including `github` and `slsa3` makes the entrypoint intentionally GitHub-specific and
  commits the implementation to satisfying the documented SLSA3-oriented trust contract before
  release.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification
and implementation define:

- `.github/workflows/github-release-asset-slsa3.yml` as the public reusable workflow entrypoint;
- `on.workflow_call` as the supported entry mechanism for callers;
- production caller examples using
  `windlasstech/slsa-builder/.github/workflows/github-release-asset-slsa3.yml@<full-commit-sha>`;
- production `builder.id` values using the workflow file path plus the same full commit SHA used for
  reusable workflow execution;
- exactly one explicitly named GitHub Release asset as the primary output and verification unit for
  this workflow;
- separate filenames for multi-asset orchestration, preview, experimental, lower-assurance,
  non-production, generic artifact, or full-release workflows;
- review checks that prevent arbitrary caller-defined steps, services, defaults, or environment from
  becoming part of this trusted workflow entrypoint.

Implementation review should verify that production release asset provenance does not emit a
different workflow filename in `builder.id` unless a later ADR changes this public entrypoint
decision.

## Pros and Cons of the Options

### Use `.github/workflows/github-release-asset-slsa3.yml`

Expose the low-level release asset profile as a GitHub-specific, one-asset reusable workflow whose
name includes the SLSA3 trust mode.

- Good, because it is the most direct match for GitHub Release asset scope.
- Good, because singular `asset` avoids implying multi-asset handling in one profile run.
- Good, because it keeps `builder.id` verifier policies readable and profile-specific.
- Good, because it matches the established `<profile>-slsa3.yml` production profile naming pattern.
- Neutral, because the name is long.
- Bad, because the entrypoint is explicitly GitHub-specific rather than reusable for arbitrary file
  artifacts.

### Use `.github/workflows/release-asset-slsa3.yml`

Expose the profile with a shorter release-asset-focused filename.

- Good, because it is concise and still names the release asset artifact scope.
- Good, because it includes the SLSA3 trust mode.
- Bad, because it does not identify GitHub Release as the distribution surface.
- Bad, because future release surfaces could make the name ambiguous.
- Bad, because GitHub-specific verifier parameters are less visible in `builder.id`.

### Use `.github/workflows/github-release-assets-slsa3.yml`

Use a plural filename to reflect that GitHub Releases commonly contain multiple assets.

- Good, because it names the GitHub Release asset domain clearly.
- Good, because it may feel natural to users thinking about complete releases.
- Bad, because plural `assets` conflicts with ADR 0039's one-asset-per-run verification unit.
- Bad, because users may expect one invocation to publish, sign, or verify multiple release assets.
- Bad, because documentation would need to explain why a plural builder identity emits exactly one
  primary SLSA subject.

### Use `.github/workflows/generic-artifact-slsa3.yml`

Expose the profile as a generic artifact SLSA3 workflow and treat GitHub Release upload as one use
case.

- Good, because it appears more reusable for future artifact surfaces.
- Good, because SLSA subjects are artifact-bound and can be represented by generic name-and-digest
  pairs.
- Bad, because it broadens the profile beyond ADR 0038 and ADR 0039 before the GitHub Release asset
  contract is specified.
- Bad, because it hides release tag, asset name, upload target, immutable release, and GitHub
  attestation storage semantics.
- Bad, because generic naming would make a future truly generic artifact profile harder to
  distinguish from this GitHub-specific profile.

### Use `.github/workflows/github-release-asset.yml`

Expose the profile without encoding the SLSA trust mode in the filename.

- Good, because it is shorter than the selected filename.
- Good, because it avoids putting the SLSA level directly in the workflow path.
- Bad, because it is inconsistent with the existing JS/TS npm package production entrypoint name.
- Bad, because it makes security mode separation less visible in caller workflows and `builder.id`.
- Bad, because future preview, SLSA2, baseline attestation, or non-claiming workflows could be
  tempted to share the same filename and blur verifier expectations.

## More Information

This decision follows ADR 0038 and ADR 0039. It decides only the reusable workflow filename and
public entrypoint for the initial GitHub Release asset SLSA3 profile. It does not decide the exact
public input contract, output names, job graph, build or pack command policy, release creation
policy, immutable release requirement, asset upload behavior, checksum or SBOM handling, provenance
sidecar naming, orchestration workflow interface, or standalone verifier CLI surface.

Reference points considered:

- GitHub reusable workflows must live directly under `.github/workflows/`, must include
  `on.workflow_call`, and are invoked by callers through
  `{owner}/{repo}/.github/workflows/{filename}@{ref}`.
- ADR 0028 requires production consumers to pin reusable workflows by full commit SHA and requires
  production `builder.id` values to include the same full commit SHA.
- SLSA v1.2 provenance defines `runDetails.builder.id` as the trusted build platform identity and
  requires different builder IDs for modes with different security attributes or SLSA Build levels.
- ADR 0039 scopes the initial GitHub Release asset profile to exactly one explicitly named GitHub
  Release asset per profile run.
