---
parent: Decisions
nav_order: 59
status: accepted
date: 12026-07-29
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0057
    scope: "the release-asset mode public workflow_call interface is a minimal user-intent surface"
  - type: see-also
    target: ADR-0060
---

# Define the Public npm Release-Asset Mode Interface

## Context and Problem Statement

ADR 0057 selects an explicit GitHub Release asset publication mode on the existing public JS/TS npm
workflow for publishing one JS/TS npm package and attaching the same verified package tarball plus
producer provenance sidecar to a GitHub Release. ADR 0058 then constrains the GitHub Release
publisher to the caller repository's existing release, separates release mutation authority from
signing and metadata authority, and rejects cross-repo, custom-token, release-creation, and
overwrite paths.

Those decisions leave one public contract question before the architecture specifications can define
the exact reusable workflow schema: what level of `workflow_call` interface should the public npm
workflow expose when callers enable release-asset mode?

The workflow's public API becomes part of the verifier-relevant release surface. GitHub reusable
workflow inputs, secrets, permissions, and outputs are caller-visible compatibility contracts. SLSA
verification also depends on stable expectations for builder identity, `buildType`,
`externalParameters`, artifact digest binding, and provenance distribution. If the public interface
exposes internal handoff details, raw artifact paths, cross-repository target controls, or custom
tokens, the project creates a broader trust boundary than the same-run composition selected in ADR
0057 and the same-repository publisher boundary selected in ADR 0058.

Should release-asset mode expose a minimal user-intent interface, a broadly configurable release
orchestration interface, or the low-level producer-to-publisher composition primitives directly?

## Decision Drivers

- Preserve the single public npm entrypoint selected by ADR 0022 and refined by ADR 0057.
- Preserve the same-run internal handoff boundary selected by ADR 0057.
- Preserve the same-repository existing-release mutation boundary selected by ADR 0058.
- Give normal callers a stable, simple `workflow_call` contract for one public npm package release.
- Avoid exposing internal artifact names, handoff manifest names, manifest digests, upload URLs, or
  raw artifact paths as caller-owned trust inputs.
- Prefer npm trusted publishing and GitHub OIDC over long-lived npm tokens.
- Avoid custom GitHub tokens, personal access tokens, or GitHub App tokens in the initial production
  release-asset mode surface.
- Keep verifier expectations narrow enough to compare `externalParameters` strictly.
- Keep SLSA provenance bound to the package tarball and distributed as an artifact-related sidecar.
- Keep release asset publication fail-closed for missing permissions, missing releases, duplicate
  assets, digest mismatches, provenance mismatches, and unsupported configuration attempts.
- Leave exact input names, types, defaults, examples, and fixture schemas to architecture specs.

## Considered Options

- Expose a minimal user-intent `workflow_call` interface for release-asset mode.
- Expose a broadly configurable release orchestration interface.
- Expose low-level producer-to-publisher composition inputs directly.

## Decision Outcome

Chosen option: "Expose a minimal user-intent `workflow_call` interface for release-asset mode",
because it gives callers a safe default release surface while keeping internal handoff mechanics,
release mutation authority, and verifier expectations aligned with ADR 0057 and ADR 0058.

The public npm workflow's release-asset mode must expose a minimal interface for one JS/TS npm
package release to the caller repository's existing GitHub Release. Its public inputs should
describe release intent, not internal transport or trust state. The public input categories are:

- package selection, such as the package directory;
- npm publish intent, such as public access and dist-tag behavior;
- release-asset mode enablement;
- release selection, such as the existing release tag in the caller repository;
- optional provenance sidecar publication behavior;
- optional linked artifact metadata publication behavior.

The exact names, types, defaults, validation rules, and examples for those inputs belong in the
architecture specifications. The ADR-level contract is that the input set stays user-intent oriented
and does not expose security-sensitive internal wiring.

Release-asset mode must not expose public inputs for:

- target repository owner, repository name, upload URL, release URL, or cross-repository release
  target selection;
- custom GitHub token, personal access token, or GitHub App token for release mutation;
- raw artifact path, arbitrary local file upload, asset bytes supplied outside the producer handoff,
  or caller-supplied asset digest as the trust source;
- internal workflow artifact names, provenance bundle artifact names, handoff manifest artifact
  names, or handoff manifest digests;
- release creation, tag creation, draft publication, prerelease flag mutation, latest-marker
  mutation, asset overwrite, asset delete, or asset replacement;
- multi-package or multi-primary-asset batch publication in one workflow invocation.

The public npm workflow should require no npm publish secret for the normal production path. It
should use npm trusted publishing with GitHub OIDC for public package publication. If private
dependency installation or token-based package publication is needed later, that is a separate trust
boundary and requires a future ADR.

Release-asset mode must not require or accept a custom GitHub release mutation secret in the initial
production path. The release upload job uses caller-granted `GITHUB_TOKEN` permissions scoped to the
caller repository as constrained by ADR 0058.

The public permission contract must document caller-granted least privilege. At minimum, the
architecture specifications and examples must distinguish:

- read access needed by checkout, verification, mapping, and metadata preparation;
- `id-token: write` needed for OIDC-backed npm trusted publishing and signing or attestation jobs;
- `attestations: write` where GitHub artifact attestations are produced;
- `contents: write` only for the job that uploads release assets to the existing caller-repository
  GitHub Release;
- `artifact-metadata: write` only for an explicitly enabled linked-artifact metadata job.

The public outputs should report publication results and verifier-relevant locators and digests, not
internal handoff mechanics. Output categories should include:

- package name and version;
- npm package tarball filename and digest;
- npm registry publication locator or registry metadata locator;
- GitHub Release primary asset name, download URL, and digest when release-asset mode is enabled;
- producer provenance sidecar name, download URL, and digest when sidecar publication is enabled;
- optional linked artifact metadata locator when that feature is enabled.

Release-asset mode must fail closed before release mutation or optional metadata publication when
the caller omits required permissions, the selected tag or GitHub Release is missing, the selected
release target resolves outside the caller repository, a release asset with the target name already
exists, producer provenance is missing or unverifiable, the tarball digest does not match the
producer handoff, or callers attempt unsupported cross-repo, raw-artifact, custom-token, overwrite,
or multi-asset configurations.

### Consequences

- Good, because ordinary callers get a small stable public workflow API for the intended release
  path without learning a second npm workflow entrypoint.
- Good, because internal handoff details remain internal to the same-run trusted workflow graph.
- Good, because verifier expectations remain centered on package identity, release tag, workflow
  identity, artifact digests, and publication locators rather than arbitrary caller wiring.
- Good, because tokenless npm trusted publishing remains the default and no GitHub custom token is
  introduced.
- Good, because duplicate assets and overwrite attempts remain fail-closed, preserving provenance
  and sidecar immutability.
- Neutral, because architecture specs must still define the exact `workflow_call` schema and
  examples.
- Bad, because advanced callers cannot customize release asset naming, custom release repositories,
  multi-package batches, or raw-file publication through release-asset mode.
- Bad, because future support for those advanced cases will require additional ADRs and likely new
  workflow surfaces or advanced composition contracts.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, fixtures, and
documentation define:

- `.github/workflows/js-ts-npm-package-slsa3.yml` as the public reusable workflow entrypoint and its
  complete release-asset mode `workflow_call` schema;
- user-intent inputs for package selection, npm publish intent, release-asset mode enablement,
  existing release tag, optional provenance sidecar publication, and optional linked artifact
  metadata publication;
- absence or rejection of public inputs for target repositories, upload URLs, release URLs, custom
  GitHub tokens, raw artifact paths, internal handoff artifacts, overwrite behavior, release
  creation, multi-package release, and multi-primary-asset publication;
- a normal production path with no npm publish secret and no custom GitHub release mutation secret;
- caller permission examples showing the least privilege needed for npm trusted publishing, signing
  or attestation, release upload, and optional linked artifact metadata publication;
- public outputs for package identity, npm tarball digest, registry locator, release asset locator
  and digest, provenance sidecar locator and digest, and optional metadata locator;
- failure behavior for missing permissions, missing tags, missing releases, duplicate assets,
  provenance verification failures, digest mismatches, unsupported input combinations, cross-repo
  target attempts, raw-artifact attempts, custom-token attempts, overwrite attempts, and batch
  publication attempts;
- fixtures proving that public outputs do not expose internal handoff manifest names or digests as a
  composition API between separately invoked reusable workflows.

Implementation review should verify that release-asset mode exposes only release intent to callers,
derives trust from the producer-owned same-run handoff rather than caller-provided artifact
metadata, and cannot be configured into a broader release publisher than the authority boundary
defined by ADR 0058.

## Pros and Cons of the Options

### Expose a minimal user-intent `workflow_call` interface

Release-asset mode exposes package selection, npm publish intent, release tag selection, optional
sidecar behavior, optional metadata behavior, least-privilege permission requirements, and
publication-result outputs. Internal handoff and release mutation mechanics stay hidden.

- Good, because it follows GitHub reusable workflow guidance to expose typed caller inputs and
  outputs without asking callers to wire internal jobs manually.
- Good, because it matches npm trusted publishing's tokenless OIDC model.
- Good, because it keeps SLSA `externalParameters` and verifier expectations small.
- Good, because it aligns with SLSA guidance that provenance accompany the artifact as an
  artifact-bound sidecar.
- Bad, because it intentionally excludes advanced release orchestration knobs from the public npm
  workflow.

### Expose a broadly configurable release orchestration interface

Release-asset mode would accept more customization, such as asset name templates, node runtime
selection, package manager overrides, release naming behavior, dry-run behavior, overwrite behavior,
or other release lifecycle knobs.

- Good, because callers could adapt the same workflow to more repository layouts and release styles.
- Good, because fewer future workflows might be needed for convenience features.
- Bad, because each public knob becomes a compatibility contract and a verifier expectation.
- Bad, because overwrite or release lifecycle knobs conflict with the existing release and
  no-overwrite boundaries from ADR 0043 and ADR 0058.
- Bad, because the workflow would drift from a SLSA release profile into a general release
  automation tool.

### Expose low-level producer-to-publisher composition inputs directly

Release-asset mode would expose raw artifact paths, artifact digests, provenance paths, handoff
manifest names, release IDs, upload URLs, or token inputs so callers can wire the producer and
publisher directly.

- Good, because expert users get maximal flexibility.
- Good, because the public surface resembles the lower-level producer and publisher primitives.
- Bad, because it turns internal same-run handoff details into stable public API.
- Bad, because caller-supplied artifact metadata can become a confused trust source.
- Bad, because it reopens public-output and cross-run composition that ADR 0057 intentionally leaves
  for a future ADR.
- Bad, because custom token and target repository inputs conflict with ADR 0058's initial authority
  boundary.

## More Information

This decision follows ADR 0057 and ADR 0058. It also preserves the earlier JS/TS npm package
profile, GitHub Release asset publisher, producer-to-publisher handoff, provenance sidecar
distribution, and verification policy decisions that those ADRs compose.
