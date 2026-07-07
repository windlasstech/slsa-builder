---
parent: Decisions
nav_order: 48
status: accepted
date: 12026-07-07
decision-makers: Yunseo Kim
---

# Make Linked Artifacts Storage Records Explicit Opt-In for Release Assets

## Context and Problem Statement

ADR 0038 selected Windlass-generated SLSA provenance for GitHub Release assets with `actions/attest`
as the initial Sigstore signing and storage adapter. ADR 0044 selected a three-job permission
boundary and allowed `artifact-metadata: write` only when linked artifact metadata is intentionally
created. ADR 0047 then selected GitHub artifact attestation storage as the canonical release asset
provenance distribution channel, with release asset bundle sidecars only as an optional export path.

GitHub's linked artifacts page can display storage and deployment records for artifacts built by an
organization. GitHub recommends uploading attested assets to the linked artifacts page so teams can
connect vulnerable artifacts to owning repositories, build runs, storage locations, and deployment
context. GitHub `actions/attest` can automatically create linked-artifact storage records, but the
documented automatic path is gated by `push-to-registry: true` and `artifact-metadata: write`, which
fits registry-published artifacts such as container images. GitHub Release assets are not pushed to
a package registry by the release asset profile.

Should the initial GitHub Release asset profile upload linked-artifact storage records by default,
make them optional through the artifact metadata REST API, rely on `actions/attest` automatic
storage records, or leave the feature to higher-level orchestration?

## Decision Drivers

- Preserve the low-level one-release-asset profile boundary from ADR 0039.
- Avoid granting `artifact-metadata: write` unless linked artifact metadata is explicitly requested.
- Avoid treating a GitHub Release asset as a registry-pushed artifact just to trigger
  `actions/attest` automatic storage record creation.
- Keep the default release asset path focused on signed provenance, producer-side verification, and
  upload to an existing GitHub Release.
- Allow organizations that use GitHub linked artifacts to record release asset storage metadata.
- Ensure storage record metadata describes the uploaded release asset URL, name, digest, version,
  and repository accurately.
- Avoid rollback or clobber behavior when linked metadata creation fails after the release asset has
  already been uploaded.
- Keep deployment records and whole-release inventory policy outside the low-level asset profile.

## Considered Options

- Do not upload linked-artifact storage records by default; support explicit REST API opt-in after
  verified release asset upload.
- Never support linked-artifact storage records in the low-level release asset profile.
- Require linked-artifact storage records for every release asset upload.
- Use `actions/attest` automatic storage records through `push-to-registry: true`.
- Leave all linked artifact storage and deployment records to a higher-level orchestration workflow.

## Decision Outcome

Chosen option: "Do not upload linked-artifact storage records by default; support explicit REST API
opt-in after verified release asset upload", because it preserves the default least-privilege
release asset path while allowing organizations to populate GitHub's linked artifacts inventory when
they intentionally accept the extra permission and metadata lifecycle.

The initial GitHub Release asset profile should not create linked-artifact storage records by
default. The default production path should not request `artifact-metadata: write`, should not call
the artifact metadata REST API, and should not configure `actions/attest` only to make linked
artifact metadata appear.

The profile may support an explicit opt-in input for linked-artifact storage record creation. When
enabled, storage record creation should happen only after the `release-upload` job has verified the
signed provenance bundle and successfully uploaded the selected release asset to the existing GitHub
Release. The implementation should use GitHub's artifact metadata REST API rather than
`actions/attest` `push-to-registry: true`, because the subject is a GitHub Release asset rather than
a registry-pushed package or container image.

The linked-artifact storage record should describe the uploaded release asset, not the provenance
bundle, checksum file, SBOM, or whole GitHub Release. The architecture specification should define
the exact mapping, including at least:

- `name` from the final GitHub Release asset name;
- `digest` as `sha256:<lowercase-hex-release-asset-digest>`;
- `version` from the release tag or a profile-defined normalized version;
- `artifact_url` from the uploaded GitHub Release asset URL when available;
- `registry_url` as the profile-defined GitHub Release storage surface base URL;
- `repository` as the profile-defined repository or release asset storage path component;
- `github_repository` only when required by the API because an associated provenance attestation is
  not available or not discoverable.

When linked-artifact upload is enabled, the workflow should grant `artifact-metadata: write` only to
the job that creates the storage record. That job should not have `id-token: write`,
`attestations: write`, or GitHub Release mutation authority unless a later ADR explicitly combines
those responsibilities. Prefer a post-upload metadata job or a tightly scoped post-upload step over
adding metadata permissions to caller-controlled build steps.

If the selected release asset uploads successfully but linked-artifact storage record creation
fails, the workflow should fail clearly without deleting, replacing, or clobbering the uploaded
release asset. The failure message and outputs should make the partial state explicit: the release
asset was uploaded, but linked artifact metadata was not recorded. Operators can then retry metadata
creation or use the artifact metadata REST API manually.

Deployment records are out of scope for the low-level release asset profile. They describe runtime
or environment deployment state and should be handled by deployment systems, integrations, custom
REST API automation, or a higher-level orchestration workflow.

### Consequences

- Good, because the default release asset profile remains least-privilege and does not require
  `artifact-metadata: write`.
- Good, because GitHub Release assets are not forced into the `actions/attest` registry-push model.
- Good, because organizations that use linked artifacts can still opt into accurate storage records.
- Good, because metadata creation happens after the signed asset has been verified and uploaded.
- Good, because metadata permissions are isolated from caller-controlled build steps and signing
  authority.
- Neutral, because projects that want linked artifacts must provide additional inputs and accept a
  post-upload metadata step.
- Bad, because linked artifacts inventory is incomplete for default release asset profile users.
- Bad, because an opt-in metadata failure can leave an uploaded release asset without its linked
  storage record and requires manual or automated follow-up.

### Confirmation

This decision is confirmed when the initial GitHub Release asset profile architecture specification,
workflow implementation, tests, and documentation define:

- linked-artifact storage record creation as disabled by default;
- no `artifact-metadata: write` permission in the default production path;
- an explicit opt-in input for storage record creation, if supported;
- artifact metadata REST API usage for GitHub Release asset storage records rather than
  `actions/attest` `push-to-registry: true`;
- release asset metadata field mapping for `name`, `digest`, `version`, `artifact_url`,
  `registry_url`, `repository`, and `github_repository` when applicable;
- storage record creation after verified release asset upload;
- clear failure behavior that does not delete, overwrite, or clobber uploaded release assets after
  metadata failure;
- job-level permission isolation for `artifact-metadata: write`;
- documentation that deployment records and whole-release inventory policy belong outside the
  low-level one-asset profile.

Implementation review should verify that no default path grants artifact metadata write permission,
that no path pretends a GitHub Release asset is a registry-pushed artifact to trigger automatic
storage records, and that metadata failures cannot cause release asset overwrite or deletion.

## Pros and Cons of the Options

### Do not upload by default; support explicit REST API opt-in

The low-level profile defaults to no linked-artifact storage record. When the caller opts in, a
post-upload step or job calls the artifact metadata REST API with metadata for the uploaded GitHub
Release asset.

- Good, because it keeps the common path minimal and least-privilege.
- Good, because release asset metadata can use the final uploaded asset URL and digest.
- Good, because the feature composes with GitHub organizations that use linked artifacts for alert
  prioritization.
- Good, because metadata permissions can be isolated to a narrow job or step.
- Neutral, because the profile must define a GitHub Release storage mapping for a registry-oriented
  API shape.
- Bad, because the optional path requires more implementation and failure-state documentation.

### Never support linked-artifact storage records in the low-level profile

The profile produces signed provenance and uploads release assets, but it never records linked
artifact storage metadata.

- Good, because it is the simplest low-level profile boundary.
- Good, because no additional permission or API lifecycle is needed.
- Bad, because organizations using GitHub linked artifacts cannot see release asset storage context
  from this profile.
- Bad, because every repository would need custom follow-up automation for storage inventory.

### Require linked-artifact storage records for every release asset upload

Every production release asset upload also creates or updates a linked-artifact storage record.

- Good, because organization inventory is complete for all release assets produced by the profile.
- Good, because security alert prioritization can rely on consistent storage metadata.
- Bad, because the default profile requires broader permissions and a GitHub organization feature.
- Bad, because metadata API failure becomes a required release failure after asset upload.
- Bad, because this adds lifecycle complexity before the initial release asset profile needs it.

### Use `actions/attest` automatic storage records through `push-to-registry: true`

The profile configures `actions/attest` to create storage records automatically by enabling
`push-to-registry` and granting `artifact-metadata: write`.

- Good, because it follows GitHub's automatic storage record path for registry artifacts.
- Good, because storage record IDs can be exposed from the `actions/attest` outputs.
- Bad, because `push-to-registry` is documented for registry-published subjects with fully qualified
  image or package names, not ordinary GitHub Release assets.
- Bad, because it can misrepresent the release asset as a registry artifact and blur storage
  semantics.
- Bad, because it couples signing adapter configuration to linked-artifact inventory behavior.

### Leave all linked artifact records to a higher-level orchestration workflow

A release orchestration workflow creates storage and deployment records after all one-asset profile
invocations have completed.

- Good, because whole-release inventory and deployment context naturally belong above the low-level
  one-asset profile.
- Good, because the orchestrator can handle multi-asset releases, checksum files, SBOMs, and release
  manifests together.
- Good, because the low-level profile remains simpler.
- Neutral, because this can coexist with a low-level opt-in storage record mode if scopes are clear.
- Bad, because users of only the low-level profile do not get linked artifact storage records
  without additional tooling.

## More Information

This decision follows ADR 0038, ADR 0039, ADR 0043, ADR 0044, ADR 0045, ADR 0046, and ADR 0047. It
decides only linked-artifact storage record policy for the initial GitHub Release asset profile. It
does not decide deployment record policy, whole-release inventory aggregation, linked artifact UI
usage, alert prioritization rules, sidecar bundle naming, or release manifest schema.

Reference points considered:

- GitHub's artifact attestation guide recommends uploading attested assets to the organization's
  linked artifacts page for build history, deployment records, storage details, and security alert
  prioritization.
- GitHub's `actions/attest` automatic storage record path requires both `push-to-registry: true` and
  `artifact-metadata: write`.
- GitHub's linked artifacts guide describes artifact metadata storage records as creatable through
  artifact attestations, JFrog integration, or the artifact metadata REST API.
- GitHub's artifact metadata storage record REST API accepts artifact `name`, `digest`, `version`,
  `artifact_url`, `registry_url`, `repository`, `status`, and `github_repository` metadata.
- GitHub's linked artifacts guide says unwanted records cannot be deleted from the linked artifacts
  page, though storage or deployment records can be updated to reflect artifact status.
- Windlass workflow hardening guidance requires explicit minimal permissions and job-level elevation
  only where required, including `artifact-metadata: write` only for linked artifact metadata.
