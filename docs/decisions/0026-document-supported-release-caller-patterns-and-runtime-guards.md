---
parent: Decisions
nav_order: 26
status: accepted
date: 12026-07-01
decision-makers: Yunseo Kim
---

# Document Supported Release Caller Patterns and Runtime Guards

## Context and Problem Statement

ADR 0002 chose an extensible trusted reusable workflow foundation, ADR 0003 refined it into
profile-owned reusable workflows, and ADR 0022 selected
`.github/workflows/js-ts-npm-package-slsa3.yml` as the initial JS/TS npm package profile entrypoint.
ADR 0023 defined the initial `workflow_call` input contract, ADR 0024 selected OIDC trusted
publishing without publish secrets, and ADR 0025 selected the initial workflow outputs.

The next public-contract decision is how the initial JS/TS npm package profile should describe and
enforce supported release caller patterns. The reusable workflow itself is invoked through
`on.workflow_call`; it does not own the top-level GitHub event trigger. Caller repositories decide
whether their local workflow starts from `push`, `release`, `workflow_dispatch`, `schedule`,
`pull_request`, or another event. However, the production SLSA3 profile still needs a documented
caller contract so users know which release patterns are supported, which patterns are outside the
profile's trust story, and which event contexts the implementation should reject when possible.

Should the initial profile merely document examples, define supported caller patterns, or attempt to
enforce allowed caller events at runtime?

## Decision Drivers

- Be precise that GitHub triggers belong to caller workflows, while the profile exposes only
  `workflow_call`.
- Keep the production npm publish path aligned with clear release intent.
- Preserve SLSA Build L3 expectations that external parameters and invocation context are complete
  enough for downstream verification.
- Align with npm trusted publishing, which validates the caller repository and caller workflow
  filename for reusable workflow publishes.
- Avoid treating pull request, pull request target, scheduled, branch-push, or arbitrary dispatch
  events as production npm publish release events by default.
- Provide fail-fast behavior for event contexts that the called workflow can detect as unsupported.
- Keep future support for other release patterns possible through later ADRs or distinct builder
  identities when their trust properties differ.

## Considered Options

- Document tag-push release workflows plus constrained manual dispatch, and add runtime guards.
- Document tag-push release workflows only.
- Document tag push, manual dispatch, and GitHub Release publication as equal production patterns.
- Allow any caller trigger and only document examples.
- Support branch-push or scheduled continuous publishing.
- Support pull request, pull request target, or merge queue publishing.

## Decision Outcome

Chosen option: "Document tag-push release workflows plus constrained manual dispatch, and add
runtime guards", because it is accurate about reusable workflow ownership while still giving the
production SLSA3 npm publish profile a clear, enforceable release-intent boundary.

The initial JS/TS npm package profile should expose only `on.workflow_call`. Caller workflows own
their top-level triggers. The project should document these production caller patterns as supported:

```yaml
on:
  push:
    tags:
      - "v*"
```

and, as an operational retry or manually approved release path:

```yaml
on:
  workflow_dispatch:
```

The tag-push pattern is the primary supported production caller pattern. The manual dispatch pattern
is supported only when the caller uses it for an intentional release ref, preferably a tag ref, and
protects it with repository controls such as GitHub environments, restricted workflow permissions,
protected tags, or maintainer review. The architecture specification should define the exact manual
dispatch constraints, including whether `github.ref_type` must be `tag`, how package version relates
to the selected ref, and how a rerun differs from a new release.

The profile should document these patterns as unsupported production publish caller patterns unless
a later ADR defines a distinct mode, stricter contract, or separate builder identity:

- `pull_request`;
- `pull_request_target`;
- `merge_group`;
- branch `push` publishing;
- `schedule` publishing;
- `workflow_run`, `repository_dispatch`, or other indirect automation used as the publish trigger;
- arbitrary caller triggers not described by the production caller contract.

The called reusable workflow should implement runtime guards for the event context it can observe.
At minimum, the production profile should fail before packing or publishing when `github.event_name`
and `github.ref_type` do not match a documented supported production caller pattern. For example, a
production implementation may accept `push` only when `github.ref_type` is `tag`, and may accept
`workflow_dispatch` only when the selected ref satisfies the architecture specification's
release-ref rules.

These guards are best-effort enforcement, not a substitute for caller documentation. A called
workflow cannot own the caller's YAML trigger, cannot prove every repository governance policy, and
cannot always distinguish whether a supported event was itself caused by a less direct automation
chain. Therefore, documentation and verifier guidance should describe both:

- the supported caller workflow shape users should write; and
- the runtime event checks the reusable workflow applies before publishing.

Caller examples should use a released builder reference such as:

```yaml
jobs:
  publish:
    permissions:
      contents: read
      id-token: write
    uses: windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@vX.Y.Z
    with:
      package-directory: .
```

Documentation should also repeat ADR 0024's npm trusted publisher rule: npm package settings must
name the caller repository and caller workflow filename, not the reusable workflow filename in this
repository.

### Consequences

- Good, because the decision is accurate about GitHub Actions ownership: callers own triggers, the
  profile owns `workflow_call` behavior and runtime validation.
- Good, because tag-push release workflows create a clear source ref for package releases and
  provenance expectations.
- Good, because constrained manual dispatch gives maintainers a practical retry or recovery path
  without making every event shape a supported release path.
- Good, because runtime guards catch obvious unsupported contexts before npm publish mutates the
  registry.
- Good, because unsupported PR and branch contexts are not accidentally documented as production
  publish paths.
- Neutral, because caller repositories still need their own branch, tag, environment, and npm
  trusted publisher configuration.
- Bad, because manual dispatch constraints need further specification before implementation.
- Bad, because runtime guards cannot prove all caller governance or indirect trigger history.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification and
implementation define:

- `on.workflow_call` as the only trigger on the reusable workflow itself;
- tag-push caller workflows as the primary supported production publish pattern;
- constrained `workflow_dispatch` caller workflows as the supported manual retry or approved release
  path;
- unsupported production publish patterns including PR, PR-target, merge queue, branch-push,
  scheduled, workflow-run, repository-dispatch, and arbitrary caller triggers;
- runtime guard rules for `github.event_name`, `github.ref_type`, `github.ref`, and any additional
  release-ref constraints;
- failure behavior for unsupported event contexts before package packing, provenance generation, or
  npm publish;
- caller examples using `contents: read`, `id-token: write`, no publish secrets, and released
  builder refs;
- npm trusted publisher setup instructions that name the caller workflow filename.

Implementation review should verify that unsupported event contexts cannot proceed to `npm publish`
through the production SLSA3 workflow unless a later ADR intentionally broadens the supported caller
contract or creates a distinct builder mode.

## Pros and Cons of the Options

### Document tag-push release workflows plus constrained manual dispatch, and add runtime guards

Document tag push as the primary production caller pattern, document manual dispatch as a
constrained retry or approved release path, and fail fast on unsupported event contexts where
observable.

- Good, because it balances strict release intent with operational recovery.
- Good, because it matches npm trusted publishing's caller-workflow identity model.
- Good, because it avoids pretending the reusable workflow owns the caller's trigger declaration.
- Good, because obvious unsupported events can be rejected before publish.
- Neutral, because manual dispatch requires careful ref and approval documentation.
- Bad, because runtime guards are necessarily incomplete compared with repository governance.

### Document tag-push release workflows only

Support only caller workflows that run on pushed tags.

- Good, because release source identity is simple and verifier-friendly.
- Good, because package version, Git tag, and protected-tag policy can be aligned.
- Bad, because maintainers lack a supported manual retry path for transient registry, runner, or
  trusted-publisher failures.
- Bad, because users may invent unsupported manual or dispatch wrappers anyway.

### Document tag push, manual dispatch, and GitHub Release publication as equal production patterns

Treat tag push, manual dispatch, and `release: published` as equal documented production caller
patterns.

- Good, because GitHub Release publication maps naturally to human release review and release notes.
- Good, because npm provenance documentation includes GitHub Release examples for direct publish
  workflows.
- Neutral, because a later ADR could add this path with explicit tag and release-object validation.
- Bad, because the initial profile would need to define release-object, draft, prerelease,
  retagging, and package version semantics before implementation.
- Bad, because multiple equal trigger patterns increase verifier and documentation complexity.

### Allow any caller trigger and only document examples

Accept that callers own triggers and do not define supported production trigger patterns beyond
examples.

- Good, because it maximizes caller flexibility.
- Good, because the reusable workflow contract remains simple.
- Bad, because dangerous contexts can look supported merely because the reusable workflow runs.
- Bad, because verifier expectations and npm trusted publisher setup become caller-specific.
- Bad, because PR-target, branch, schedule, and dispatch publishes can silently enter the production
  SLSA3 trust story.

### Support branch-push or scheduled continuous publishing

Document `push` to branches or `schedule` as production publish patterns.

- Good, because this can support nightly, canary, or continuous prerelease packages.
- Good, because SLSA GitHub Generator supports several non-tag triggers for generic provenance.
- Bad, because npm package versions are immutable, so continuous publish requires additional version
  generation and dist-tag policy.
- Bad, because moving branch refs and scheduled release intent are weaker and harder to verify than
  tag-based releases.
- Bad, because this is a different product mode from a production package release profile.

### Support pull request, pull request target, or merge queue publishing

Allow PR, PR-target, or merge queue events to call the production publish workflow.

- Good, because these events are useful for validation, dry-run packing, and future non-publish
  quality gates.
- Bad, because production npm publish from PR-shaped events mixes release mutation with review or
  test contexts.
- Bad, because fork PRs, merge refs, and privileged `pull_request_target` contexts create security
  and provenance ambiguity.
- Bad, because SLSA GitHub Generator treats `pull_request` as an exception rather than a generally
  supported trigger for provenance generation.

## More Information

This decision follows ADR 0022, ADR 0023, ADR 0024, and ADR 0025. It decides the documented
production caller patterns and runtime event guard policy, not package versioning, Git tag naming,
GitHub environment protection, release approval policy, or the exact provenance fields for caller
event context.

Reference points considered:

- SLSA v1.2 Build Requirements require the producer to choose an appropriate build platform, follow
  a consistent build process, and distribute provenance.
- SLSA v1.2 Build Provenance treats external parameters as the externally controlled build interface
  and requires completeness at SLSA Build L3.
- GitHub reusable workflows are invoked through `workflow_call`; the caller workflow owns top-level
  event triggers and passes inputs, secrets, permissions, and outputs through the reusable workflow
  interface.
- GitHub event contexts expose event name and ref information that a called workflow can inspect at
  runtime.
- npm trusted publishing for GitHub Actions validates the caller repository and caller workflow
  filename when reusable workflows are involved, and requires OIDC permissions for both parent and
  child workflows.
- npm trusted publishing and provenance generation require supported cloud-hosted CI contexts; npm
  does not support self-hosted runners for trusted publishing.
- SLSA GitHub Generator documents `schedule`, `push` including tags, `release`, and
  `workflow_dispatch` as tested triggers for generic and Go provenance workflows, while calling out
  `pull_request` as an exception; npm publishing has a narrower registry-mutating release boundary
  than generic provenance generation.
