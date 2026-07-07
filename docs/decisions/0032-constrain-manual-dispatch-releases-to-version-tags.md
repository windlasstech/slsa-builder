---
parent: Decisions
nav_order: 32
status: accepted
date: 12026-07-05
decision-makers: Yunseo Kim
---

# Constrain Manual Dispatch Releases to Version Tags

## Context and Problem Statement

ADR 0026 selected tag-push caller workflows plus constrained `workflow_dispatch` as the supported
production release caller patterns for the initial JS/TS npm package SLSA3 profile. It intentionally
left the exact manual-dispatch constraints open, including whether the selected ref must be a tag,
how the selected ref relates to the package version, and how reruns differ from new releases.

ADR 0028 later required production consumers to execute the Windlass reusable workflow by full
commit SHA and to use SHA-based `builder.id` values. That decision does not weaken the caller
repository's release-ref requirements: package provenance still needs a clear source ref, release
intent, and external-parameter contract for downstream verification.

Manual dispatch is operationally useful for recovering from transient runner, registry, or trusted
publishing failures. It is also risky if treated as an arbitrary release button, because a
maintainer could accidentally publish from a moving branch ref, provide a version input that
disagrees with the package metadata, or rerun a workflow after npm has already accepted an immutable
package version.

Should production `workflow_dispatch` publishes require a tag ref, require the tag name to match the
package version, and define rerun semantics before npm publish?

## Decision Drivers

- Preserve ADR 0026's constrained manual dispatch support without broadening manual dispatch into an
  arbitrary branch-based release mechanism.
- Keep package releases tied to immutable, protected, signed release tags rather than moving branch
  refs or free-form version inputs.
- Preserve SLSA Build L3 expectations that externally controlled release inputs and invocation
  context are complete and verifier-meaningful.
- Align with npm's immutable package name/version model: the same package version cannot be
  republished as a new release.
- Distinguish a retry of the same release tag from a new package release.
- Ensure provenance can distinguish separate build invocations through GitHub run identity and run
  attempt values.
- Avoid giving the production workflow permission or responsibility to create release tags.

## Considered Options

- Require manual dispatch to run on a tag ref, require `v${package.json.version}` tag matching, and
  treat reruns or repeated dispatches as distinct invocations for the same release tag.
- Require manual dispatch to run on a tag ref, but only warn when the tag and package version
  differ.
- Allow manual dispatch from branch refs with a required version input.
- Allow manual dispatch from branch refs and let the workflow create the release tag.
- Allow manual dispatch only for retry after a pre-publish failure.
- Remove manual dispatch support and allow tag-push releases only.

## Decision Outcome

Chosen option: "Require manual dispatch to run on a tag ref, require `v${package.json.version}` tag
matching, and treat reruns or repeated dispatches as distinct invocations for the same release tag",
because it preserves a practical manual recovery path while keeping production release identity tied
to immutable Git tags and npm package versions.

The production JS/TS npm package SLSA3 profile should accept `workflow_dispatch` only when the
selected ref is a tag. The called workflow should fail before package packing, provenance
generation, or npm publish when `github.event_name == "workflow_dispatch"` and
`github.ref_type != "tag"`. Branch refs, default-branch manual runs, arbitrary commit-SHA dispatches
without a tag ref, and manual version inputs are unsupported for production publish under this
profile.

The selected tag name must match the package version using the initial profile's release tag
convention:

```text
v${package.json.version}
```

For example, tag `v1.2.3` must publish package version `1.2.3`, and tag `v1.2.3-beta.1` must publish
package version `1.2.3-beta.1`. The workflow should read the packed package metadata, not only the
working-tree `package.json`, when checking the final publish version. A mismatch between the
selected tag and the package version should fail clearly.

Manual dispatch is a supported manual retry or approved release path for an existing release tag. It
is not a tag-creation path. The production profile should not create, move, sign, or delete release
tags. Tag creation, tag signing, protected tag policy, and release approval happen outside this
reusable workflow before the publish attempt.

GitHub reruns and repeated manual dispatches for the same tag are distinct build invocations for the
same release tag, not new releases. Provenance and workflow outputs should distinguish invocations
with the GitHub run id and run attempt where available. The profile should include or derive a build
invocation identifier equivalent to:

```text
${github.run_id}-${github.run_attempt}
```

The profile should fail clearly when the target npm package name and version are already published
in the selected registry. It should not silently skip publish, overwrite provenance, treat the rerun
as a successful new release, or attempt to republish the immutable npm version. If a previous
attempt failed before registry mutation, a rerun or repeated manual dispatch may publish the same
tag and package version. If the registry already contains that package version, verification or
inspection is the correct operation, not another production publish.

### Consequences

- Good, because manual dispatch keeps a supported operational recovery path without weakening the
  release-ref boundary.
- Good, because package version, Git tag, and provenance source ref remain aligned.
- Good, because branch refs and free-form version inputs cannot become production release sources in
  the initial SLSA3 profile.
- Good, because reruns are represented as distinct build invocations instead of ambiguous duplicate
  releases.
- Good, because the production publish workflow does not need `contents: write` permission to create
  or move tags.
- Neutral, because callers still need repository-level protections such as signed/protected tags and
  protected environments where appropriate.
- Bad, because users must create the release tag before manual dispatch.
- Bad, because custom release tag conventions are out of scope until a later ADR or profile revision
  changes the tag/version contract.
- Bad, because checking already-published package versions requires registry interaction and careful
  error reporting.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification,
implementation, documentation, and verifier guidance define:

- `workflow_dispatch` production publishes as tag-ref-only manual retries or approved release paths;
- failure before pack, provenance generation, or publish when manual dispatch is not running on a
  tag ref;
- the initial release tag convention `v${package.json.version}`;
- package-version checks against the packed artifact metadata;
- failure behavior for tag/package-version mismatches;
- no workflow-created, moved, signed, or deleted release tags in the production publish profile;
- provenance or byproduct fields that include event name, ref, ref type, ref SHA, run id, run
  attempt, actor, and triggering actor where available;
- build invocation identity that distinguishes reruns, such as
  `${github.run_id}-${github.run_attempt}`;
- clear failure when the package name/version is already published;
- documentation that verification or inspection, not republish, is the correct path after a package
  version has already been accepted by the registry.

Implementation review should verify that branch-based manual dispatch, arbitrary version inputs,
workflow-created tags, and already-published package versions cannot proceed to production npm
publish through the initial SLSA3 profile.

## Pros and Cons of the Options

### Require tag ref, require tag/version match, and define reruns as same-release invocations

Manual dispatch is allowed only when the selected ref is a tag whose name equals
`v${package.json.version}`. Reruns and repeated dispatches produce distinct build invocations for
the same release tag.

- Good, because this preserves tag-push release semantics while allowing manual recovery.
- Good, because verifier expectations can rely on a stable tag-to-package-version relationship.
- Good, because npm's immutable version model is respected.
- Good, because run id and run attempt can uniquely identify each build invocation.
- Bad, because it reduces flexibility for projects with nonstandard tag naming.
- Bad, because manual dispatch cannot create a release from a branch without prior tag creation.

### Require tag ref but only warn on tag/version mismatch

Manual dispatch must use a tag ref, but the workflow only warns when the tag and package version do
not match.

- Good, because it supports projects whose Git tags and package versions intentionally differ.
- Good, because it is easier to adopt during migration.
- Bad, because accidental mismatches can still publish a wrong package version.
- Bad, because release notes, npm version, Git tag, and verifier expectations can diverge.
- Bad, because a warning is too weak for a production registry mutation in the SLSA3 profile.

### Allow branch refs with a required version input

Manual dispatch may run from a branch ref when the caller provides an explicit version input.

- Good, because the GitHub UI flow is convenient.
- Good, because maintainers can initiate releases before creating tags.
- Bad, because branch refs are moving release sources.
- Bad, because version input becomes another externally controlled parameter that can disagree with
  package metadata.
- Bad, because tag protection and signed-tag release integrity can be bypassed.

### Allow branch refs and let the workflow create the tag

Manual dispatch runs on a branch, derives or accepts a version, creates the release tag, then
publishes.

- Good, because it can provide a one-button release workflow.
- Good, because tag creation and publish can be ordered by automation.
- Bad, because the publish workflow needs source-control write permissions.
- Bad, because tag signing, protected tags, release approval, and source mutation become coupled to
  the package publish path.
- Bad, because this gives the reusable workflow a broader trust boundary than registry publishing.

### Allow manual dispatch only after a pre-publish failure

Manual dispatch is accepted only when it can prove an earlier attempt failed before registry
mutation.

- Good, because this is the narrowest retry semantics.
- Good, because it minimizes duplicate release attempts.
- Bad, because proving exact prior failure state across GitHub Actions and the registry is brittle.
- Bad, because registry/network failures can leave ambiguous publish status.
- Bad, because the workflow still needs a simple already-published-version failure rule.

### Remove manual dispatch and support tag-push only

Only pushed tags can publish production npm packages.

- Good, because the release trigger model is simplest.
- Good, because verifier expectations and documentation are concise.
- Good, because manual misuse risk is minimized.
- Bad, because maintainers lack a supported recovery path for transient release infrastructure
  failures.
- Bad, because users may create unsupported wrapper workflows to recover from failures.

## More Information

This decision follows ADR 0026 and decides the remaining manual-dispatch constraints for the initial
production JS/TS npm package profile. It does not change the reusable workflow entrypoint, the
SHA-pinned builder identity requirement from ADR 0028, the release manifest decision from ADR 0031,
npm trusted publishing requirements, registry scope, or future non-publish validation workflows.

Reference points considered:

- SLSA v1.2 Build Requirements require a consistent build process and complete, trustworthy
  provenance generation for the chosen build level.
- SLSA v1.2 Verifying Artifacts recommends checking trusted `builder.id`, `buildType`, and expected
  `externalParameters` against verifier expectations.
- SLSA GitHub Generator's Node.js builder documents `workflow_dispatch` as a supported trigger while
  showing release-oriented examples that guard on tag refs.
- SLSA GitHub Generator's Node.js provenance records GitHub event context such as event name, ref,
  ref type, run id, and run attempt, and uses run id plus run attempt to distinguish reruns.
- GitHub manual `workflow_dispatch` runs can be started against a selected ref, so a production
  release profile must define which selected refs are valid.
- npm package versions are immutable after publication, so rerunning a successful publish should not
  be treated as a new production release.
