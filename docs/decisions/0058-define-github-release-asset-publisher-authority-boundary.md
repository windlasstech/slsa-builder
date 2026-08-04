---
parent: Decisions
nav_order: 58
status: accepted
date: 12026-07-29
decision-makers: Yunseo Kim
relations:
  - type: see-also
    target: ADR-0066
  - type: see-also
    target: ADR-0067
  - type: amended-by
    target: ADR-0074
    scope:
      "the publisher mutation topology: all release asset uploads for one run live in exactly one
      upload job per remote surface"
---

# Define the GitHub Release Asset Publisher Authority Boundary

## Context and Problem Statement

ADR 0043 requires the GitHub Release asset publisher to upload one named asset to an existing Git
tag and existing GitHub Release without creating releases, moving tags, editing release metadata, or
overwriting existing assets. ADR 0050 then requires the publisher to verify producer artifact bytes,
expected digests, and producer-generated SLSA provenance before release mutation. ADR 0057 selects a
release-asset mode on the public npm workflow while retaining the standalone publisher as an
advanced composition primitive.

Those decisions still leave the publisher's mutation authority boundary incomplete. The architecture
must decide which repository the publisher may mutate, how release upload authority is separated
from verification, signing, and optional linked-artifact metadata authority, what permissions the
caller must grant to reusable workflows, and how boundary violations fail before any GitHub Release
is modified.

Should the initial production publisher mutate only the caller repository, allow selected
cross-repository publication, support arbitrary target repositories through caller-provided tokens,
or collapse verification and release upload into one privileged job?

## Decision Drivers

- Keep the initial production trust boundary small enough to specify, verify, and test before adding
  cross-repository release distribution.
- Preserve ADR 0043's existing-release and no-overwrite publication model.
- Preserve ADR 0050's requirement that release mutation happens only after producer provenance and
  artifact digests are verified.
- Preserve ADR 0057's public npm release-asset mode while making caller permissions explicit.
- Avoid confused-deputy behavior where a reusable workflow called from one repository mutates a
  different repository without a separate authority model.
- Follow GitHub Actions reusable workflow constraints: called and nested reusable workflows can only
  maintain or reduce caller-granted `GITHUB_TOKEN` permissions.
- Follow workflow hardening guidance: minimal top-level permissions, job-level elevation only where
  required, full-SHA-pinned third-party actions, and no unnecessary long-lived credentials.
- Keep signing or attestation authority separate from release mutation authority.
- Keep linked artifact metadata publication explicit and isolated from ordinary release asset
  upload.

## Considered Options

- Restrict the publisher to the caller repository's existing release with strict job-level authority
  separation.
- Allow publication to another repository in the same owner or organization.
- Allow arbitrary cross-repository publication through caller-provided GitHub App tokens or personal
  access tokens.
- Let the publisher create missing releases or tags before uploading assets.
- Combine verification, signing or attestation, linked metadata, and release upload in one
  privileged job.

## Decision Outcome

Chosen option: "Restrict the publisher to the caller repository's existing release with strict
job-level authority separation", because it keeps the initial production mutation boundary aligned
with ADR 0043, ADR 0050, ADR 0053, and ADR 0057 while avoiding a new cross-repository authorization
model.

The initial production GitHub Release asset publisher must target only the repository that owns the
caller workflow run. In GitHub Actions terms, the release target repository is `github.repository`
for the top-level caller context. The publisher must not accept a public `owner`, `repo`,
`target-repository`, upload URL, release URL, or custom token input that lets callers redirect the
production upload path to another repository.

The publisher must upload only to an existing GitHub Release associated with the verified release
tag inside that caller repository. The publisher must fail before release mutation when the release
tag is missing, the GitHub Release is missing, the release lookup resolves outside the caller
repository, or any caller-provided target value attempts cross-repository publication.

The production authority topology should separate jobs by privilege:

- verification, mapping, and metadata preparation jobs should run with read-only permissions and no
  release mutation authority;
- producer signing or attestation jobs should have only the signing permissions they require, such
  as `contents: read`, `id-token: write`, and `attestations: write`, and must not have release
  mutation authority;
- the release upload job should have the release mutation permission it needs, normally
  `contents: write`, and must not have signing authority, attestation authority, package publication
  authority, or linked-artifact metadata authority;
- optional linked artifact metadata publication should run in a distinct job with
  `artifact-metadata: write` only when the caller explicitly enables that feature.

The top-level reusable workflow and standalone publisher workflow should declare minimal default
permissions and rely on explicit job-level elevation. Because reusable workflows cannot elevate
permissions beyond what the caller grants, caller documentation and examples must state the minimal
permissions required for the selected surface. Missing or insufficient permissions must fail before
release mutation or before any optional metadata publication that requires the missing permission.

The production publisher must not support release creation, tag creation, release metadata updates,
draft publication, prerelease flag changes, latest-marker updates, asset overwrite, asset delete,
asset replacement, cross-run artifact handoff, public-output composition between separately invoked
workflows, arbitrary raw-file upload, or multi-asset batch publication unless a future ADR defines a
new trust boundary and verifier expectations for that capability.

The same caller-repository boundary applies to both the public npm workflow's release-asset mode and
the standalone low-level publisher when used as an advanced composition primitive. Advanced users
may still assemble custom same-repository compositions, but they must preserve the documented
digest-verified handoff, producer provenance verification, least-privilege job separation, and
same-repository release target rule.

### Consequences

- Good, because the initial production publisher cannot become a confused deputy for mutating other
  repositories.
- Good, because release target verification, permission examples, and verifier expectations remain
  simple and testable.
- Good, because signing authority and GitHub Release mutation authority stay separated.
- Good, because optional linked artifact metadata writes cannot silently broaden the ordinary
  release upload job.
- Good, because the decision preserves the existing no-create and no-overwrite release asset model.
- Neutral, because caller workflows must grant enough permissions explicitly; called workflows
  cannot grant them for the caller.
- Neutral, because same-repository release publication still supports the default npm-to-release
  asset use case selected by ADR 0057.
- Bad, because projects that publish artifacts into a central release repository, distribution
  repository, or same-organization release repository need a later ADR and separate workflow
  surface.
- Bad, because custom GitHub App or PAT based publication remains unsupported in the production
  publisher path.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, fixtures, and
documentation define:

- the caller repository as the only valid production release target repository;
- absence or rejection of public `owner`, `repo`, `target-repository`, release URL, upload URL, and
  custom token inputs for redirecting production release asset uploads;
- release tag and GitHub Release lookup scoped to `github.repository`;
- failure before upload when the release target resolves outside the caller repository;
- read-only verification, mapping, and metadata-preparation jobs without release mutation authority;
- signing or attestation jobs without `contents: write` or release mutation authority;
- a release upload job with only the required release mutation permission, normally
  `contents: write`;
- optional linked artifact metadata publication in a separate explicitly enabled job with
  `artifact-metadata: write`;
- caller workflow examples that grant exactly the permissions needed for release-asset mode and the
  standalone publisher;
- insufficient-permission fixtures proving failures happen before release mutation;
- rejection fixtures for cross-repository target attempts, missing tags, missing releases, duplicate
  assets, unverified producer provenance, digest mismatch, and unsupported custom token publication.

Implementation review should verify that no production path can upload to a repository other than
the caller repository, that no job combines signing authority with release mutation authority, that
optional metadata writes are isolated, and that all release target, digest, and provenance checks
complete before any GitHub Release mutation.

## Pros and Cons of the Options

### Restrict the publisher to the caller repository with strict authority separation

The publisher uploads to an existing release in `github.repository` only. Verification and
preparation jobs are read-only, signing jobs cannot mutate releases, the upload job cannot sign, and
optional linked metadata publication uses a separate permission boundary.

- Good, because it matches GitHub's default `GITHUB_TOKEN` repository scope and reusable workflow
  permission model.
- Good, because it avoids cross-repository confused-deputy and token-scope questions in the first
  production contract.
- Good, because it follows this repository's existing digest-verified handoff and three-job
  least-privilege patterns.
- Good, because it keeps verifier expectations centered on one source and release repository.
- Bad, because centralized distribution repositories and same-organization release repositories are
  not supported initially.

### Allow same-owner or same-organization cross-repository publication

The publisher accepts a target repository input when the target shares the caller repository's owner
or organization.

- Good, because it supports organizations that centralize downloads or release assets in a dedicated
  repository.
- Good, because it is narrower than arbitrary cross-repository publication.
- Bad, because GitHub's default `GITHUB_TOKEN` is repository-scoped; many callers would need a
  GitHub App token or personal access token.
- Bad, because verifier expectations must distinguish source repository, caller workflow repository,
  producer artifact repository, and release target repository.
- Bad, because same-owner does not by itself prove that the caller should have authority over the
  target repository.

### Allow arbitrary cross-repository publication through custom tokens

The publisher accepts target owner/repository inputs and a caller-provided GitHub App token or
personal access token for release mutation.

- Good, because it supports mirrors, external distribution repositories, and complex release
  topologies.
- Good, because GitHub Apps can express scoped installation authority when designed carefully.
- Bad, because it introduces long-lived or separately minted credential handling into the publisher
  surface.
- Bad, because token scope, target repository policy, auditability, and verifier expectations become
  a separate authorization system.
- Bad, because it expands the product from a SLSA publisher profile into a more general deployment
  tool.

### Let the publisher create missing releases or tags

The publisher creates the tag or GitHub Release when it cannot find the selected target, then
uploads the asset.

- Good, because first-time caller setup is simpler.
- Good, because a single workflow invocation can create a release from scratch.
- Bad, because it conflicts with ADR 0043's existing-release and no-release-lifecycle boundary.
- Bad, because tag creation, release notes, draft status, prerelease status, target commit, and
  latest-marker policy belong to a higher-level release orchestration surface.
- Bad, because implicit target creation can publish artifacts from an unintended ref or default
  branch when not guarded perfectly.

### Combine all authority in one privileged job

One job verifies producer provenance, signs or attests metadata, uploads release assets, and
publishes linked artifact metadata with all required permissions.

- Good, because implementation and artifact handoff are simpler.
- Good, because fewer jobs means fewer workflow artifacts and fewer digest handoff checks.
- Bad, because a compromised step gains signing authority and release mutation authority together.
- Bad, because it conflicts with the existing release manifest boundary selected by ADR 0053.
- Bad, because it makes optional metadata publication broaden the ordinary release upload authority.

## More Information

This decision follows ADR 0043, ADR 0048, ADR 0049, ADR 0050, ADR 0051, ADR 0052, ADR 0053, and
ADR 0057. It decides only the production publisher's release target repository and job-level
authority boundary. It does not decide a future cross-repository publisher, GitHub App token model,
release creation workflow, multi-asset release orchestration, or linked-artifact metadata schema.

Reference points considered:

- SLSA v1.2 Distributing Provenance says attestations should be bound to artifacts rather than
  releases, should accompany the artifact at publish time, and should be immutable once published
  for an artifact.
- SLSA v1.2 Verifying Artifacts expects verification of provenance signatures, artifact subject
  digests, trusted `builder.id`, `buildType`, and expected `externalParameters`.
- The SLSA GitHub Generator generic workflow can upload provenance to GitHub Releases and documents
  `actions: read`, `id-token: write`, and `contents: write` permissions for that path.
- The SLSA GitHub Generator Node.js builder separates package/provenance creation from npm publish
  through digest-checked package and provenance downloads.
- GitHub Actions reusable workflows require explicit `workflow_call` inputs, secrets, and outputs;
  nested reusable workflows can only maintain or reduce caller-granted permissions.
- GitHub recommends least-privilege `GITHUB_TOKEN` permissions, job-level permission elevation, and
  full-SHA pinning for third-party actions and reusable workflows.
- GitHub release asset upload uses repository-scoped release endpoints; duplicate asset names fail
  rather than overwrite unless callers separately delete or replace existing assets.
- Windlass workflow hardening requires minimal top-level permissions, job-level elevation only when
  required, OIDC instead of long-lived credentials, and explicit permissions for release asset
  upload and linked artifact metadata writes.
