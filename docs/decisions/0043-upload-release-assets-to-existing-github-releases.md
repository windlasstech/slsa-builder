---
parent: Decisions
nav_order: 43
status: accepted
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Upload GitHub Release Assets to Existing Releases

## Context and Problem Statement

ADR 0038 selected Windlass-generated SLSA provenance with `actions/attest` as the initial signing
and storage adapter for GitHub Release assets. ADR 0039 scoped the low-level profile to one
explicitly named GitHub Release asset per profile run. ADR 0040 selected the reusable workflow
entrypoint for that one-asset profile.

The remaining lifecycle policy question is how much GitHub Release state the low-level profile
should own. GitHub Releases can be created from existing tags, can create tags when the tag is
missing in some tooling flows, can be drafts or prereleases, can become immutable after publication,
and can contain existing assets with the same name. The profile needs a failure-closed policy that
preserves artifact-bound SLSA provenance without turning the one-asset profile into a whole-release
orchestrator.

Should the initial GitHub Release asset profile create or update GitHub Releases, or should it
upload one asset only to an existing release selected by an existing tag?

## Decision Drivers

- Preserve ADR 0039's one-asset profile boundary and keep whole-release lifecycle concerns in an
  orchestration layer.
- Keep SLSA provenance artifact-bound rather than treating a GitHub Release as the build subject.
- Avoid giving the low-level profile responsibility for creating, moving, signing, deleting, or
  selecting release tags.
- Avoid GitHub tooling behavior that can create a missing tag from a default branch or selected
  target commit during release creation.
- Support GitHub immutable release workflows, where the recommended flow is draft release creation,
  asset upload, then publication.
- Fail clearly when the target release or asset slot is not in the expected state.
- Avoid overwriting, deleting, or replacing existing release assets under the same production
  profile invocation.

## Considered Options

- Upload one asset to an existing GitHub Release selected by an existing tag, failing when the tag
  or release is missing.
- Create the GitHub Release when it is missing, then upload the asset.
- Create a draft GitHub Release, upload the asset, and publish the release from the same profile
  run.
- Allow asset overwrite or clobber behavior for existing asset names.
- Treat release creation, asset upload, and publish as one high-level orchestration workflow.

## Decision Outcome

Chosen option: "Upload one asset to an existing GitHub Release selected by an existing tag, failing
when the tag or release is missing", because it keeps the low-level profile focused on one
artifact-bound release asset while leaving release lifecycle, draft creation, publication, notes,
and multi-asset coordination to a higher-level orchestration layer.

The initial GitHub Release asset profile should require both:

- an existing Git tag matching the requested release tag; and
- an existing GitHub Release associated with that tag.

The profile should fail before provenance signing or asset upload when either the tag or release is
missing. It should not create a tag, create a release, select a default branch target, move a
release tag, publish a draft release, update release notes, change the prerelease flag, change the
latest release marker, or otherwise mutate release lifecycle metadata.

Draft releases are valid upload targets. This supports the immutable-release-friendly flow where an
orchestration layer creates a draft release, invokes the one-asset profile once per asset, and then
publishes the release after all required assets are attached. Published releases are also valid
upload targets when GitHub and repository policy allow adding assets after publication, but the
recommended production flow is to upload through draft releases when immutable releases are enabled
or desired.

Prerelease GitHub Releases are valid upload targets. The low-level profile should record the
observed release state if the architecture specification makes it verifier-relevant, but it should
not decide or change whether the release is a prerelease.

The profile should reject an existing release asset with the requested asset name. It should not
delete, overwrite, replace, or clobber an existing asset as part of the production path. If a
previous attempt uploaded the asset but later failed, the correct follow-up is verification or
manual inspection, not silent replacement under the same tag and asset name. Retrying after a
failure is supported only when the asset was not uploaded or when the caller chooses a distinct
asset name under an explicit new release policy outside this low-level profile.

The profile's required GitHub token permissions should not exceed what is needed to read repository
contents, attest provenance, and upload the selected release asset. Any additional permissions
needed to create releases, publish drafts, edit release notes, mark prereleases, or manage tags
belong to the orchestration layer or caller workflow that performs those lifecycle operations.

### Consequences

- Good, because the low-level profile remains a one-asset upload and provenance contract rather than
  a whole-release lifecycle manager.
- Good, because missing tags and releases fail closed instead of being created implicitly from a
  default branch or caller-selected target.
- Good, because draft releases can still be used for GitHub immutable release publication flows.
- Good, because existing asset names cannot be silently replaced with different bytes or provenance.
- Good, because release notes, draft publication, prerelease flags, latest markers, and multi-asset
  completeness checks stay in the orchestration layer that owns whole-release user experience.
- Neutral, because callers must create the release before invoking the low-level profile.
- Neutral, because published mutable releases may still accept new assets when repository policy
  allows it.
- Bad, because a single call to the low-level profile cannot create a complete GitHub Release from
  scratch.
- Bad, because callers need orchestration when they want draft creation, multiple asset uploads, and
  publish as one workflow.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification,
workflow implementation, tests, and documentation define:

- a required release tag input that must identify an existing Git tag;
- failure before provenance signing or upload when the release tag does not exist;
- failure before provenance signing or upload when no GitHub Release exists for the tag;
- upload of exactly one explicitly named asset to the existing release;
- support for draft release targets without publishing them;
- support for prerelease targets without changing prerelease state;
- no tag creation, tag movement, release creation, draft publication, release note editing,
  prerelease flag editing, or latest marker editing in the low-level profile;
- failure when an existing release asset already uses the requested asset name;
- no overwrite, delete, replace, or clobber behavior for existing release assets;
- documentation that draft release creation, multi-asset sequencing, immutable release publication,
  release notes, and whole-release completeness checks belong to an orchestration layer.

Implementation review should verify that the production path cannot silently create missing tags or
releases, upload to the wrong release target, publish a draft release, modify release metadata, or
replace an existing asset under the same release tag and asset name.

## Pros and Cons of the Options

### Upload one asset to an existing release and fail when tag or release is missing

The caller or orchestration layer creates the tag and release first. The low-level profile validates
that both exist, uploads one named asset, signs or stores the associated provenance, and fails if
the asset name already exists.

- Good, because it matches ADR 0039's low-level one-asset scope.
- Good, because release governance remains outside the trusted asset upload profile.
- Good, because it avoids implicit tag creation during release creation flows.
- Good, because it composes cleanly with draft releases and immutable release publication.
- Good, because duplicate asset handling is simple and failure-closed.
- Neutral, because an orchestration layer is required for a one-command whole-release workflow.
- Bad, because first-time users must create the GitHub Release before the asset profile can run.

### Create the release when it is missing

The profile accepts release metadata, creates the GitHub Release when the selected tag has no
release, and then uploads the asset.

- Good, because it reduces setup for simple single-asset releases.
- Good, because the release and asset upload can happen in one profile invocation.
- Bad, because release creation brings release notes, title, target commit, latest marker, draft,
  and prerelease policy into the low-level profile.
- Bad, because some tooling can create a missing tag as part of release creation unless explicitly
  guarded.
- Bad, because it blurs the boundary between artifact-bound provenance and whole-release lifecycle
  management.

### Create a draft release, upload the asset, and publish it

The profile owns the full immutable-release-friendly sequence for one release: create draft, upload
the selected asset, and publish.

- Good, because it matches GitHub's recommended immutable release publication flow for complete
  releases.
- Good, because users get a convenient single workflow for simple releases.
- Bad, because publishing a release is a whole-release decision that must know whether all required
  assets, checksums, SBOMs, and notes are complete.
- Bad, because multi-asset releases would need partial-success and publish-order handling inside the
  low-level one-asset profile.
- Bad, because the profile would need broader `contents: write` behavior and release metadata
  inputs.

### Allow asset overwrite or clobber behavior

The profile deletes or replaces an existing release asset when the requested asset name already
exists.

- Good, because retries after partial failure can be operationally convenient.
- Good, because it resembles common mutable release upload tooling that supports clobber behavior.
- Bad, because deleting the old asset before uploading the new one can lose the original asset if
  the new upload fails.
- Bad, because replacing bytes under the same release tag and asset name weakens artifact identity
  and consumer expectations.
- Bad, because it conflicts with immutable release behavior and SLSA's preference for immutable
  attestations corresponding to artifacts.

### Treat release creation, asset upload, and publish as one orchestration workflow

A higher-level workflow creates or selects a release, invokes the one-asset profile for each asset,
checks whole-release completeness, and publishes the draft when appropriate.

- Good, because it can provide the user-friendly whole-release flow that the low-level profile
  avoids.
- Good, because it can own release notes, draft status, prerelease status, latest marker, checksums,
  SBOMs, and multi-asset completeness checks together.
- Good, because it composes the one-asset profile without changing the artifact-bound SLSA subject
  contract.
- Neutral, because this is a separate product surface with a distinct lifecycle contract.
- Bad, because it requires additional design work and possibly a distinct workflow entrypoint or
  builder identity.

## More Information

This decision follows ADR 0038, ADR 0039, and ADR 0040. It decides only the low-level GitHub Release
asset profile's release creation and upload policy. It does not decide the future orchestration
workflow interface, release notes schema, checksum manifest policy, immutable release requirement,
release asset sidecar naming, or standalone verifier CLI behavior.

Reference points considered:

- SLSA v1.2 Distributing Provenance says attestations should be bound to artifacts rather than
  releases, and that provenance should accompany the artifact at publish time.
- SLSA v1.2 Distributing Provenance says attestations should be immutable and should not be
  overwritten later with a different attestation for the same artifact.
- GitHub CLI `gh release create` can automatically create a matching tag when one does not exist;
  `--verify-tag` aborts release creation when the tag is missing.
- GitHub CLI `gh release upload --clobber` deletes existing assets before uploading replacements and
  warns that the original asset is lost if the upload fails.
- GitHub immutable releases protect release tags and assets from modification or deletion after
  publication, and GitHub recommends creating the release as a draft, attaching all assets, and then
  publishing it.
- GitHub artifact attestations and `actions/attest` bind named artifact subjects and digests to
  signed predicates; they complement GitHub release attestations but do not require the low-level
  profile to own release lifecycle operations.
