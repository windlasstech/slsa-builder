---
parent: Decisions
nav_order: 109
status: accepted
date: 12026-11-08
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0070
    scope:
      "the retention of the actual package-manager version recording in
      externalParameters.package_manager and the runner-image descriptor's node_version annotation;
      the manager version continues in the package-manager-distribution descriptor's annotations,
      the bundled npm version moves to builder.version.npm, and the Node.js version is single-homed
      in builder.version.nodejs"
  - type: partially-supersedes
    target: ADR-0071
    scope:
      "the builder.version closed key set, which now admits npm for the bundled npm record, and the
      reference to externalParameters.package_manager version recording"
  - type: partially-supersedes
    target: ADR-0080
    scope:
      "the externalParameters.release.* portion of the built-release policy binding surface; release
      expectations now bind to the effective built ref derivation and the built-source descriptor"
  - type: partially-supersedes
    target: ADR-0082
    scope:
      "the externalParameters.runtime.npm_version recording location; the bundled npm version moves
      to builder.version.npm"
  - type: partially-supersedes
    target: ADR-0084
    scope:
      "the externalParameters.runtime.npm_version recording location; the bundled npm version moves
      to builder.version.npm"
  - type: partially-supersedes
    target: ADR-0085
    scope:
      "the externalParameters.runtime.npm_version recording location; the bundled npm version moves
      to builder.version.npm"
  - type: see-also
    target: ADR-0033
  - type: see-also
    target: ADR-0079
  - type: partially-supersedes
    target: ADR-0100
    scope:
      "the runner-image descriptor's closed shape: annotations.node_version is removed in favor of
      the single builder.version.nodejs record, and annotations.runner_label is added"
  - type: partially-supersedes
    target: ADR-0103
    scope:
      "the sentence retaining externalParameters.release.* as built-release policy surface; the
      workflow group and the internalParameters.github placement are unchanged"
  - type: partially-supersedes
    target: ADR-0105
    scope:
      "the closed resolvedDependencies set membership and cardinalities, extended by the
      package-manifest descriptor, and the preservation of existing descriptor shapes where the
      runner-image descriptor changes as scoped in ADR 0100's relation; the source descriptors,
      index-0 serialization, and name-keyed verification remain in force, while the deterministic
      producer serialization order gains the package-manifest descriptor at index 2, between
      built-source and lockfile"
  - type: partially-supersedes
    target: ADR-0106
    scope:
      "the package.repository identity-chain operand and the retention of
      externalParameters.release.*; repository identity binds to the normalized manifest repository
      and workflow.repository, and release identity to the built-source descriptor"
  - type: see-also
    target: ADR-0107
  - type: see-also
    target: ADR-0108
  - type: see-also
    target: ADR-0028
  - type: see-also
    target: ADR-0029
  - type: see-also
    target: ADR-0030
  - type: partially-supersedes
    target: ADR-0064
    scope:
      "the requirement to represent the tarball filename in profile-defined provenance fields; the
      npm PURL subject, SHA-512 and SHA-256 subject digests, actual-tarball and Release-asset
      verification, and the producer-to-publisher handoff filename binding remain in force"
  - type: see-also
    target: ADR-0068
  - type: see-also
    target: ADR-0076
  - type: see-also
    target: ADR-0099
  - type: see-also
    target: ADR-0104
  - type: partially-superseded-by
    target: ADR-0112
    scope:
      "the exact four-key runner-image annotation set and associated Confirmation clause insofar as
      they require image_os: the required set becomes exactly deployment_version, operating_system,
      image_version, and runner_label, with operating_system equal to the exact decoded Operating
      System.detail string and image_os rejected. The name/uri/annotations top-level shape, the
      verbatim Included Software URI and exact full image_version as the sole reported-identity
      pair, nullable deployment_version and independent runner_label semantics, descriptor
      cardinality and serialization order, no-public-availability behavior, baseline consumer
      independence, external-parameter minimization, and every unrelated descriptor and observation
      rule remain in force. Arbitrary additional annotations remain rejected, while ADR 0111's
      allowance for a later separately governed vendor-prefixed evidence annotation is neither
      adopted nor closed"
  - type: partially-superseded-by
    target: ADR-0110
    scope:
      "the Decision Driver and Preserved contracts clauses deferring mediaType for lockfile,
      node-runtime, and package-manager-distribution: those concrete artifact descriptors now
      require deterministic mediaType values selected from their actual digest-covered
      representations, while package-manifest retains application/json and invocation-source and
      built-source remain without mediaType. Identity, URI, digest, acquisition, annotations,
      descriptor cardinality and serialization order, the package-manifest no-content rule, and
      every runner-image decision remain in force"
  - type: see-also
    target: ADR-0111
---

# Minimize External Parameters to Actual Interface Inputs

## Context and Problem Statement

ADR 0103 through ADR 0107 realigned the source and workflow groups and introduced role-named source
descriptors. ADR 0108 subsequently removes the redundant public `release-tag` and
`provenance-sidecar` inputs and their direct external-parameter projections. Taking that
public-interface simplification as the baseline for this decision, the remaining externalParameters
groups still carry values that do not follow SLSA v1.2's recommended parameter model. Its guidance
is explicit: parameters are the top-level inputs to the build template, meaning inputs not derived
from another input; they should contain only the actual values passed through the build platform's
interface; the build platform should minimize the size and complexity of externalParameters, making
boilerplate values implicit from the buildType; and configuration should move into input artifacts
wherever possible, so that verifying the artifact verifies the configuration.

Measured against that model, the remaining groups contain three kinds of misplacement. First,
derived values: `package.workspace_root`, `source_manifest`, `package_url`, `tarball_name`,
`packed_name`, `packed_version`, the `publish.resolved_*` resolutions, and `effective_access` are
all computable from other recorded inputs. Second, values that belong to input artifacts:
`package.name`, `version`, `private`, `repository`, `publish_config_raw`, and `build.script_present`
are package.json content, and the `package_manager` selection metadata duplicates what the lockfile
descriptor's annotations already record. Third, observed or constant values: `runtime.runner`,
`node_version`, `npm_version`, `caller.workflow_filename`, `build.script_result`, and the publish
pre-existing flags are environment observations or outcomes rather than inputs, while
`publish.trusted_publishing` and `provenance_file` are constants by design.

Two constraints make naive deletion wrong. Values like the runner label must not become implicit
buildType semantics: a runner upgrade from ubuntu-24.04 to a later image must change a recorded
observation, not the buildType's meaning. And deleting manifest-derived fields removes their
verification basis unless the manifest itself is recorded as a dependency with a digest, which the
current descriptor set does not include. The current statement also already carries duplicate
recordings: the Node.js version appears in `runtime.node_version`, `builder.version.nodejs`, and the
runner-image descriptor's `node_version` annotation, and the npm version appears in both
`runtime.npm_version` and `package_manager.version`.

Finally, `release.ref` and `release.version_tag` restate the effective built ref, which ADR 0106's
derivation rule and the `built-source` descriptor already record; retaining the group keeps a
derived release record alongside its own inputs.

How should the remaining externalParameters groups be minimized to actual interface inputs, while
every verification guarantee is preserved through re-homed observations, a manifest dependency
record, or specified recompute rules?

## Decision Drivers

- Keep only actual interface inputs in `externalParameters`: values the caller passes through the
  workflow interface, not values derived from other inputs, read from input artifacts, observed from
  the environment, or fixed by design.
- Lose no evidence worth keeping. Observed values must move to their semantic homes —
  `builder.version` for toolchain component versions, descriptor annotations for environment
  attributes, and signed `runDetails.metadata` fields for consequential run outcomes. Derived values
  must be recomputable by a specified rule. Where a value has only one permitted state and the
  trusted builder already enforces that state, the signed assertion is redundant and may be demoted
  to unsigned diagnostics with its consumer-side signed check explicitly retired.
- Never turn a versioned observation into implicit buildType semantics. Runner, Node.js, and npm
  upgrades must change recorded values only.
- Record the package manifest as a digest-bound dependency before removing manifest-derived fields,
  so verifying the artifact verifies the configuration those fields restated.
- Use descriptor annotations only for policy-relevant metadata as in-toto defines them, and avoid
  re-duplicating data already carried by the subject or other descriptors.
- Add `mediaType` only to the `package-manifest` descriptor created by this decision. Do not add it
  to `runner-image`: its URI identifies a Markdown software report, while its authoritative on-disk
  evidence is JSON, so a descriptor-wide media type would be ambiguous. Adding `mediaType` to other
  descriptors is a separate decision.
- Keep this decision atomic. It does not reopen ADR 0103's standing pre-release policy: no buildType
  version migration and no old-bundle compatibility path.

## Considered Options

- Consolidate the remaining groups to actual interface inputs with re-homed records and recompute
  rules.
- Remove derived values only, keeping observed values in externalParameters.
- Retain all remaining groups unchanged.
- Delete derived and observed values without re-homing them.

## Decision Outcome

Chosen option: "Consolidate the remaining groups to actual interface inputs with re-homed records
and recompute rules", because it satisfies SLSA v1.2's parameter model, removes duplicate and
derived recordings without losing any verification target, and avoids coupling future runner or
toolchain upgrades to buildType semantics.

### Final externalParameters shape

After this decision, the npm profile's external parameters contain only:

- `source`: unchanged — optional, and present exactly when the caller supplied `source-ref`, with
  the required full-ref `ref` member (ADR 0106, ADR 0107).
- `workflow`: unchanged — `{ref, repository, path}` (ADR 0103).
- `package`: exactly `directory`, the caller's required package selector.
- `publish`: exactly the nullable raw inputs `input_registry_url`, `input_dist_tag`, and
  `input_access`.
- `distribution`: exactly `release_asset_mode` and `linked_artifact_metadata`, the caller's boolean
  options.

The `runtime`, `package_manager`, `release`, `caller`, and `build` groups are removed. No removed
member is recreated under another external-parameter name.

### The package-manifest descriptor

The `resolvedDependencies` set gains one descriptor that records the selected package manifest as an
input artifact:

```json
{
  "name": "package-manifest",
  "uri": "git+https://github.com/example/project@0123456789abcdef0123456789abcdef01234567#package.json",
  "digest": { "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" },
  "mediaType": "application/json",
  "annotations": { "workspace_root": null }
}
```

- `uri` follows the lockfile descriptor's construction:
  `git+<workflow.repository>@<built revision>#<repository-root-relative manifest path>`, with the
  path percent-encoded by the same rule. The built revision is the effective built ref's verified
  resolved commit.
- `digest.sha256` covers the selected manifest's bytes.
- `mediaType` is exactly `application/json`.
- `annotations` contains exactly `workspace_root`, which is `null` for standalone packages or the
  repository-root-relative workspace root for workspace packages.
- The descriptor carries no `content`. The architecture specification must define the offline
  manifest-evidence delivery contract: how the original selected source-manifest bytes are supplied
  to a consumer, how they are bound to this descriptor's digest and source identity, and which
  failures reject that evidence. This ADR requires an explicit, mandatory contract wherever
  manifest-dependent checks run, but does not select its transport or interface shape. Missing or
  mismatched evidence fails closed, with no network fallback and no silent skipping of those checks.

Consumers use bytes delivered and authenticated under that specified contract when evaluating the
manifest's content for the identity and configuration checks formerly aimed at external parameters.
The closed-set cardinalities become six descriptors for npm and seven for pnpm or Yarn. In the
producer's deterministic serialization, `package-manifest` is emitted immediately after
`built-source` and before `lockfile`, so the full order is `invocation-source`, `built-source`,
`package-manifest`, `lockfile`, `node-runtime`, the conditional `package-manager-distribution`, then
`runner-image`; ADR 0105's index-0 rule for `invocation-source` is unchanged.

### Re-homed observed values

- `builder.version` becomes `{nodejs, npm, corepack?}`: `nodejs` remains the single home for the
  observed Node.js version; `npm` is admitted and records the bundled npm version from the pinned
  toolchain pair; `corepack` retains its conditional meaning. The former `runtime.node_version` and
  `runtime.npm_version` members, and the runner-image descriptor's `node_version` annotation, are
  removed. For pnpm and Yarn, the selected manager's version continues to be recorded in the
  `package-manager-distribution` descriptor's annotations. The publish-stage npm's separate
  provisioning and outcome evidence is unchanged, as ADR 0082 and ADR 0084 require;
  `builder.version.npm` records only the build-stage bundled npm.
- The `runner-image` descriptor's annotations become exactly `deployment_version`, `image_os`,
  `image_version`, and `runner_label`, where `runner_label` records the observed Actions runner
  label (currently `ubuntu-24.04`). A runner or image upgrade changes these observed values only.
- `runDetails.metadata` gains signed run-outcome fields, using vendor-prefixed names per SLSA's
  extension-field guidance:
  - `windlass_build_script_disposition` is **required** for the npm profile (it may remain optional
    in the common cross-profile schema), with values `executed` or `skipped-absent`. The name
    records execution disposition — what the builder did about the declared script, including the
    skip reason — not a success/failure result. This preserves ADR 0033's executed-versus-skipped
    distinction, a consequential, variable outcome that consumers cannot infer from the builder's
    contract. Producer and consumer checks connect the value to the verified source manifest, and a
    missing or inconsistent outcome is rejected.
- `publish.package_identity_preexisting` and `publish.package_version_preexisting` are demoted to
  the unsigned `diagnostic_metadata` report surface as preflight-time operational observations, and
  their consumer-side signed expectations are explicitly retired. On the supported path each flag
  has exactly one permitted value, and that value is already a mandatory preflight condition of the
  SHA-pinned builder implementation, so the signed assertions duplicated what trusting the builder's
  enforced contract already establishes. A signature authenticates the assertion, not the registry's
  historical state, and "absent at preflight" never proved "absent at mutation". The producer's
  preflight gates are unchanged, and consumers rely on the accepted builder revision's enforced
  behavior rather than on any replacement field. This deliberately retires a narrow defect-detection
  opportunity and an authenticated historical record; it does not weaken the intended
  publication-policy guarantee, and the substantive artifact checks — subject digests, builder and
  source identity, and registry artifact binding — are unaffected.

### Removals and recompute rules

- `package.name`, `version`, `private`, `repository`, `publish_config_raw`, and
  `build.script_present` are verified from the package-manifest descriptor's content. The digest
  covers the original selected source-manifest bytes, not a build-mutated or packed manifest, and
  offline verification uses the manifest-evidence delivery contract defined by the specification,
  with no network fallback and no silent skipping of manifest-dependent checks. Package name and
  version additionally remain in the subject's npm Package URL. The repository identity chain
  becomes: the normalized manifest repository equals `workflow.repository`, which equals the
  certificate's Source Repository URI and both source descriptors' URI repository components.
- `package.tarball_name` is an observation whose authoritative home remains the existing pack output
  and handoff binding; no filename derivation is introduced. `package.package_url` is recomputed by
  its existing specified rule. `package.packed_name` and `packed_version` are recovered from the
  digest-verified tarball and compared with the source-manifest identity and the subject PURL,
  preserving the packed-artifact check. `package.packed_files` and `package.consumer_surface` are
  likewise recomputed from the verified tarball's contents, not from the source manifest.
- `package_manager.*` is removed entirely; selection provenance remains in the lockfile descriptor's
  annotations, and the observed distribution version in the distribution descriptor's annotations.
  `yarn_install_mode` becomes implicit in the buildType: immutable install is a fixed Yarn behavior
  of this profile, so the constant assertion and its consumer-side field check retire together.
- `publish.publish_config` is verified from the manifest's `publishConfig` content.
- `publish.resolved_registry_url`, `resolved_dist_tag`, `publish_access_option`, and
  `effective_access` are removed; the specification defines the resolution from the raw inputs, the
  manifest's `publishConfig`, and the documented defaults, and consumers recompute it. The
  input-versus-manifest conflict rejection is unchanged.
- `publish.trusted_publishing`, `provenance_file`, and `custom_registry_support` are constants by
  design and become implicit from the buildType, with the trusted-publishing and ADR 0030 registry
  boundary behavior unchanged.
- `release.ref` and `release.version_tag` are removed. Release identity is the effective built ref,
  reconstructible from `source.ref` and `workflow.ref` by ADR 0106's derivation rule and recorded in
  the `built-source` descriptor; release tag and commit policy expectations bind there.
- ADR 0108 removes `distribution.provenance_sidecar` and `distribution.release_tag_supplied` with
  their public inputs. This decision does not recreate either member: mandatory sidecar publication
  remains derived from `release_asset_mode`, and the public Release target remains derived from the
  effective built ref. Internal handoff and standalone publisher release-tag equality checks are
  unchanged.
- `caller.workflow_filename` is removed; trusted-publisher authorization derives the filename from
  `workflow.path`.

### Preserved contracts

- ADR 0105's source descriptors, index-0 npm emission order, and name-keyed verification, ADR 0106's
  selector and derivation rule, and ADR 0107's event placement and conditional source-group emission
  are unchanged. The lockfile, node-runtime, and package-manager-distribution descriptors keep their
  existing shapes; adding `mediaType` to them is not part of this decision.
- ADR 0028's canonical builder identity, ADR 0104's builder-record removal, the signer descriptor in
  `builderDependencies`, exact-byte signing, subject digest rules, and digest-rebound handoffs are
  unchanged.
- The supported-event set, source-ref validation and resolution, pack-time identity checks, publish
  preflights, and convergence behavior are unchanged; only where their results are recorded moves.
- Consistent with ADR 0103's pre-release policy, there is no buildType version migration and no
  old-bundle compatibility path; statements carrying removed members are non-conforming under the
  revised closed schema.

### Consequences

- Good, because every surviving external parameter is an actual interface input, and every derived,
  manifest-sourced, observed, or constant value has exactly one specified home or a recompute rule.
- Good, because the Node.js and npm versions are recorded once each, in `builder.version`, and the
  runner label is recorded as an observation, so future runner and toolchain upgrades change no
  buildType semantics.
- Good, because the package manifest becomes a digest-bound dependency, letting verifiers check
  package identity and publish configuration against the input artifact rather than against restated
  fields.
- Neutral, because the verification policy, producer validation, fixtures, and goldens must all
  migrate to the new comparison targets in one coordinated change set.
- Neutral, because demoting the pre-existing package flags explicitly retires their consumer-side
  signed expectations. That removes a narrow defect-detection opportunity and an authenticated
  historical record, while the publication-policy guarantee continues to rest on the accepted
  builder revision's enforced preflight contract.
- Bad, because this is the largest statement-schema change in the series: several ADR-scoped field
  locations are partially superseded, strict verifiers reject pre-change statements, and the
  conformance corpus must be regenerated.
- Bad, because consumers must now implement the specified recompute rules for publish resolution and
  caller filename, and fetch or otherwise obtain the manifest content for identity checks.
- Bad, because a controlled dogfood attempt remains necessary to confirm that the minimized
  statement still satisfies the registry's observed comparisons and exposes no new rejection family.

## Confirmation

This decision is confirmed when specifications, test-first conformance cases, exact-byte fixtures,
and controlled live evidence demonstrate all of the following:

- the common provenance contract and the npm provenance and publish specification enumerate only the
  surviving external-parameter members, and reject every removed group and member under the closed
  schema;
- the `package-manifest` descriptor is emitted with its exact shape — digest-bound URI with the
  manifest path fragment, `mediaType: "application/json"`, and only the `workspace_root` annotation
  — with cardinality six for npm and seven for pnpm or Yarn, and negative fixtures reject a wrong
  digest, path, mediaType, or unknown annotation;
- `builder.version` carries exactly `nodejs`, `npm`, and conditional `corepack`, and the removed
  `runtime.node_version`, `runtime.npm_version`, and runner-image `node_version` annotation are
  rejected, while the runner-image descriptor accepts exactly its four annotations;
- `runDetails.metadata.windlass_build_script_disposition` is required for the npm profile and
  distinguishes `executed` from `skipped-absent`, with negative fixtures for a missing or
  manifest-inconsistent outcome; the pre-existing package flags appear only in the unsigned
  diagnostic report, and the verification policy explicitly records that their consumer-side signed
  expectations are retired in favor of relying on the accepted builder revision's enforced preflight
  contract;
- verification tests prove that package identity and publish configuration checks evaluate the
  manifest content, that the repository identity chain binds the normalized manifest repository to
  `workflow.repository` and the certificate's Source Repository URI, and that release tag and commit
  expectations bind to the effective built ref and the `built-source` descriptor with no `release.*`
  member remaining;
- publish-resolution fixtures prove that consumers recompute the resolved registry, dist-tag, and
  access from the raw inputs, the manifest's `publishConfig`, and the defaults, including the
  conflict-rejection cases, and that the trusted-publisher filename is derived from `workflow.path`;
- ADR 0108's public-input removal remains intact: no `release-tag` input or omitted-versus-explicit
  suppliedness case is reintroduced, and internal handoff and standalone publisher checks continue
  to validate the derived release tag;
- manifest-evidence fixtures prove that the digest covers the original selected source-manifest
  bytes and that the specification defines an explicit offline delivery contract for acquiring and
  binding those bytes, with missing or mismatched evidence rejected, no network fallback, and no
  silent skipping of manifest-dependent checks;
- scope review confirms that no actor field was introduced, and that no old-bundle compatibility
  path or buildType version migration was introduced;
- a subsequent controlled dogfood attempt submits a minimized bundle and records the registry's
  response in `docs/dogfood/`, with any remaining rejection family staying fail-closed and recorded.

## Pros and Cons of the Options

### Consolidate the remaining groups to actual interface inputs with re-homed records and recompute rules

- Good, because it follows SLSA v1.2's distinction between actual top-level interface inputs,
  input-artifact content, derived values, observations, and constants. The closed external-parameter
  schema can then reject misplaced values rather than merely describing them.
- Good, because single-homing Node.js and npm versions, runner observations, and run outcomes
  removes duplicate equality surfaces while keeping their evidence. Runner and toolchain upgrades
  change recorded observations rather than appearing to change the buildType's meaning.
- Good, because the digest-bound package manifest makes package identity and publish configuration
  verifiable against the input artifact itself. Recompute rules preserve publish resolution and
  caller-filename checks without keeping pre-computed duplicates in the statement.
- Good, because `windlass_build_script_disposition` preserves a consequential run outcome in signed
  execution metadata, while constants become implicit buildType behavior instead of repeated caller
  inputs.
- Neutral, because the change requires coordinated partial supersession, specification and
  implementation changes, fixture regeneration, and consumer support for recompute rules and locally
  supplied digest-verified manifest bytes. These are migration and consumer-complexity costs in
  exchange for a more authoritative verification basis.
- Bad, because strict verifiers reject pre-change statements under the closed schema, with no
  old-bundle compatibility path or buildType version migration.
- Bad, because demoting the pre-existing package flags retires a narrow defect-detection opportunity
  and an authenticated historical record, even though the producer's enforced preflight remains.
- Bad, because manifest-dependent offline verification needs a specified evidence-delivery contract
  with no network fallback, and the minimized statement still needs a controlled dogfood attempt to
  expose any registry rejection family not anticipated by the design.

### Remove derived values only, keeping observed values in externalParameters

- Good, because it removes obvious duplication such as resolved publish settings and derived release
  identity without requiring the full re-homing design. The migration is smaller and touches fewer
  predecessor decisions.
- Good, because runtime versions, runner observations, build outcomes, and pre-existing package
  flags remain signed and directly readable in the parameter object, so consumers need no new
  metadata locations or unsigned diagnostic contract for those values.
- Good, because no new package-manifest descriptor or manifest-byte distribution requirement is
  introduced, and consumers can continue evaluating retained fields without a new
  content-acquisition path.
- Good, because it still reduces some drift and verifier surface while avoiding the operational risk
  of changing every field's semantic home at once.
- Neutral, because removing derived members still changes the closed statement schema and requires
  fixture and verifier updates, although the predecessor-relation and implementation surface is
  smaller than full consolidation.
- Bad, because observed and constant values remain misclassified as external inputs, and duplicate
  recordings such as the Node.js and npm versions persist with their equality and drift burden.
- Bad, because keeping `runtime.runner` and other environment observations as parameters still blurs
  the distinction between caller input and platform observation, and risks treating versioned values
  as buildType-fixed configuration.
- Bad, because manifest-derived values continue to restate package.json content without a
  digest-bound manifest dependency, leaving a second cleanup and another schema migration for a
  later decision.

### Retain all remaining groups unchanged

- Good, because no statement-schema migration is needed beyond ADR 0107. Existing statements remain
  valid under the current closed schema, avoiding a new old-bundle rejection family.
- Good, because current producer and consumer implementations, fixtures, and verification policies
  can remain unchanged, with no new descriptor, metadata field, recompute rule, or offline
  manifest-byte requirement to implement and test.
- Good, because all recorded observations and pre-computed values stay directly readable, including
  the signed pre-existing package flags and their narrow defect-detection opportunity.
- Good, because it avoids introducing a new registry comparison or rejection surface and therefore
  needs no dogfood attempt specifically for a minimized statement shape.
- Neutral, because runner and toolchain upgrades continue to update several duplicated fields and
  their equality checks, which is ongoing maintenance rather than a one-time migration cost.
- Bad, because derived, duplicated, observed, and constant values remain in the external interface,
  contrary to SLSA's parameter model and minimization guidance.
- Bad, because every duplicate equality rule remains a permanent drift and rejection surface, and
  consumers must continue checking values whose meaning is split across several locations.
- Bad, because the package manifest remains unrecorded as a digest-bound dependency, so package
  identity and publish configuration continue to rely on restated fields rather than verification
  against the input artifact itself.

### Delete derived and observed values without re-homing them

- Good, because it yields the smallest possible external-parameter surface and eliminates the
  duplicate equality checks associated with the deleted fields.
- Good, because it requires no new package-manifest descriptor, signed metadata field, descriptor
  annotation, unsigned diagnostic surface, or specified recompute rule.
- Good, because consumers no longer need to interpret several differently located representations of
  the same value, and the producer has fewer records to construct and keep consistent.
- Good, because it forces an explicit review of which recorded values are genuinely required for a
  verification decision rather than preserving evidence solely because it was previously emitted.
- Neutral, because removing members still changes the closed statement schema and requires fixture
  and implementation migration. It avoids re-homing work, but does not avoid old-bundle rejection or
  consumer-policy changes.
- Bad, because it discards verifier-relevant evidence such as toolchain versions, runner identity,
  and the executed-versus-skipped build-script distinction, weakening incident response and
  reproducibility rather than merely simplifying the statement.
- Bad, because deleting manifest-derived fields without recording the manifest removes the
  independent basis for package identity and publish-configuration checks.
- Bad, because removing verification targets can turn previous rejections into acceptance. That
  weakens the fail-closed verification model and does not satisfy this decision's requirement to
  preserve guarantees while minimizing the statement.

## More Information

Local decisions, contracts, and evidence:

- [ADR 0070](0070-record-package-manager-distributions-and-runner-image-in-resolved-dependencies.md)
  and [ADR 0071](0071-activate-builder-version-and-builderdependencies-for-platform-components.md):
  the descriptor conventions and `builder.version` component map this decision extends.
- [ADR 0080](0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md),
  [ADR 0103](0103-define-external-workflow-parameters-as-common-invocation-identity.md), and
  [ADR 0106](0106-consolidate-external-source-parameters-onto-workflow-and-descriptor-bindings.md):
  the built-release binding surface whose remaining `release.*` retention this decision removes.
- [ADR 0082](0082-pin-publish-stage-npm-cli-version-with-integrity-verified-provisioning-and-reviewed-allowlist.md),
  [ADR 0084](0084-provision-the-publish-stage-npm-cli-from-a-digest-verified-registry-tarball.md),
  and [ADR 0085](0085-pin-the-node-24-patch-version-and-assert-the-expected-bundled-npm-pair.md):
  the toolchain pins whose observed versions move to `builder.version`.
- [ADR 0100](0100-record-runner-image-identity-from-on-disk-metadata-with-the-control-plane-deployment-version-as-separate-evidence.md):
  the runner-image descriptor whose annotation set changes.
- [ADR 0105](0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md): the
  closed descriptor set extended by `package-manifest`.
- [ADR 0033](0033-run-build-script-only-when-declared.md): the executed-versus-skipped distinction
  preserved in `runDetails.metadata`.
- [ADR 0107](0107-record-invocation-event-metadata-in-internalparameters-github.md): the event
  placement and conditional source-group emission this decision leaves intact.
- [SLSA provenance v1](../architecture/slsa-provenance-v1.md),
  [JS/TS npm provenance and publish](../architecture/js-ts-npm-provenance-publish.md), and
  [verification policy and fixtures](../architecture/verification-policy-and-fixtures.md): the
  parameter schema, descriptor shapes, and fixture contracts to migrate after acceptance.
- Current implementation migration evidence:
  [npm dependency construction](../../internal/npmprofile/workflow_metadata.go) and
  [npm statement validation](../../internal/npmprofile/provenance_validate.go). These references
  explain the current recordings; they do not prescribe the future implementation.

Primary external sources:

- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance), especially
  [BuildDefinition](https://slsa.dev/spec/v1.2/build-provenance#builddefinition): top-level inputs,
  actual interface values, minimization of external parameters, and configuration in input
  artifacts.
- [in-toto ResourceDescriptor](https://github.com/in-toto/attestation/blob/main/spec/v1/resource_descriptor.md)
  and [DigestSet](https://github.com/in-toto/attestation/blob/main/spec/v1/digest_set.md):
  descriptor fields, including `mediaType` and policy-relevant `annotations`, and digest
  conventions.
- [GitHub Actions `workflow/v1` README](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/README.md)
  and
  [example.json](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/example.json):
  the distinction between workflow interface inputs and platform-recorded context.
