---
parent: Decisions
nav_order: 108
status: accepted
date: 12026-11-08
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0057
    scope:
      "the Decision Outcome's shaping of the public release-asset mode surface around a
      caller-facing release tag input; the release target is derived from the effective built tag,
      while the single public entrypoint, internal handoff, standalone publisher release-tag
      contract, release validation, and sidecar distribution decisions remain in force"
  - type: partially-supersedes
    target: ADR-0059
    scope:
      "the public workflow_call categories for independent release selection and optional provenance
      sidecar publication, including their repetitions in sidecar-output conditionality,
      Confirmation, and Pros and Cons; the release target is derived from the effective built tag,
      sidecar publication remains mandatory in release-asset mode, and sidecar locators and all
      other public interface decisions remain in force"
  - type: see-also
    target: ADR-0023
  - type: see-also
    target: ADR-0030
  - type: see-also
    target: ADR-0048
  - type: see-also
    target: ADR-0050
  - type: see-also
    target: ADR-0051
  - type: see-also
    target: ADR-0058
  - type: see-also
    target: ADR-0060
  - type: see-also
    target: ADR-0072
  - type: see-also
    target: ADR-0077
  - type: see-also
    target: ADR-0103
  - type: see-also
    target: ADR-0106
  - type: see-also
    target: ADR-0109
---

# Remove Redundant npm Release-Asset Inputs

## Context and Problem Statement

ADR 0057 places an explicit GitHub Release asset publication mode on the existing public npm
workflow, and ADR 0059 defines that mode's public interface as a minimal user-intent surface. The
resulting public contract has nine caller inputs: one required package selector and eight optional
inputs covering source selection, npm publish intent, and release-asset distribution.

A review against SLSA v1.2's BuildDefinition guidance identified two inputs that do not provide an
independent choice under the current contract. SLSA recommends reducing parameters by moving
configuration into input artifacts whenever possible, so that verifying the artifact also verifies
the configuration. It also recommends making values that are boilerplate and identical for every
build implicit from the `buildType`, while retaining actual top-level interface inputs that are not
derived from another input.

The first redundant input is `release-tag`. The public profile specification says that an omitted
value defaults to the effective built release tag and that a supplied value must equal that same
tag. The composition and publisher contracts likewise require the handoff release tag, producer
release ref, and source ref to agree. Consequently, no valid caller-supplied value can select a
release target different from the target already established by the verified built source.

The second redundant input is `provenance-sidecar`. ADR 0051 requires the production publisher to
redistribute the exact verified producer-generated SLSA provenance bundle as an unchanged GitHub
Release asset sidecar. ADR 0057 selects the same behavior for the public npm composition. The
current profile specification permits the input to be omitted or set to `required`, but normalizes
both to the same mandatory publication behavior. Disabling, renaming, rewriting, re-signing, or
replacing the sidecar is not an accepted alternative.

ADR 0059's phrase `optional provenance sidecar publication behavior`, and its reference to outputs
"when sidecar publication is enabled", are ambiguous against those earlier decisions. ADR 0059 does
not define a disabled state or a successful release path without the sidecar, does not supersede ADR
0051, and says that it preserves the earlier provenance-distribution decisions. The sidecar in all
three ADRs is the same producer-generated SLSA bundle. It is not an npm registry-native provenance
record, a GitHub Artifact Attestation locator, linked-artifact metadata, or a new publisher-owned
attestation. Native registry submission and attestation locators are supplementary discovery paths,
not substitutes for the Release distribution copy selected by ADR 0051.

The review also examined whether the other caller inputs should be removed or moved exclusively into
source metadata:

- ADR 0023 expressly considered deriving all publish intent from source metadata and rejected that
  option because callers may need to select registry, dist-tag, or access at invocation time,
  particularly for release-channel variation. It also acknowledged the costs of duplicate
  configuration, drift, conflicts, and a larger verifier surface. The current contract does not
  silently override explicit `publishConfig`; conflicting values are rejected. The accepted record
  therefore supports the existing optional channels but does not provide a field-by-field proof that
  every channel is indispensable.
- ADR 0030 preserves configurable npm-compatible registry behavior for experimentation while
  guaranteeing production semantics only for npmjs. Its confirmation accepts a registry URL input or
  an equivalent registry configuration surface, so the registry capability must not be confused with
  a requirement for a second input channel.
- `release-asset-mode` selects a distinct execution path, and `linked-artifact-metadata` is ADR
  0048's explicit opt-in to additional metadata writes and their separately scoped permission.
  Neither choice can be derived from the package manifest or the built source.
- The reusable workflow currently implements only the npm-only path; release-asset composition
  remains unimplemented. The guard therefore rejects every non-default release-distribution setting
  solely because that functionality is not yet available, not because rejection is the intended
  permanent contract. This temporary implementation status is not, by itself, a reason to remove the
  accepted future composition interface, and this review does not treat the reserved inputs or their
  current rejection as evidence that release-asset mode has been implemented.

The two inputs and the remaining optional inputs do not have equal necessity or priority. Each
warrants a separate assessment of its independent invocation-time role, the strength of its
documented rationale, and whether its value or behavior is already derivable from verified state or
fixed by the profile. The problem is therefore to determine the appropriate scope of change across
the release-related caller interface without assigning every input the same treatment.

How should the public npm workflow distinguish independent invocation-time intent from derived or
mandatory state across its release-related caller inputs, and what changes, if any, are appropriate
for each input in the current and intended future interface while preserving the existing producer,
publisher, and provenance-distribution guarantees?

## Decision Drivers

- Distinguish independent invocation-time intent from values that can be derived from verified
  inputs or are fixed by the profile's behavior.
- Follow SLSA v1.2's guidance to reduce unnecessary parameters and make constant behavior implicit,
  without removing justified external choices.
- Keep the GitHub Release target bound to the effective built tag and to an existing release in the
  caller repository. Preserve the internal handoff and publisher equality checks.
- Preserve ADR 0051's and ADR 0057's mandatory distribution of the unchanged, verified producer
  provenance bundle as a Release asset sidecar.
- Keep npm registry-native provenance submission, GitHub Artifact Attestation locators, Release
  sidecar distribution, and linked-artifact metadata semantically distinct.
- Account for the distinct behavioral and permission consequences of `release-asset-mode` and
  `linked-artifact-metadata` when evaluating whether their public inputs belong in the current or
  intended future interface.
- Evaluate `registry-url`, `dist-tag`, and `access` on their documented invocation-time rationale,
  the availability of equivalent source-controlled configuration, and the cost of duplicate
  channels, without treating support for a capability as proof that a separate caller input is
  necessary.
- Keep public caller intent separate from internal transport. Removing a public input must not
  remove the standalone publisher's release-tag input, the internal handoff's release tag, sidecar
  locators, digest bindings, or failure behavior.
- Resolve ADR 0059's optional-sidecar interface ambiguity without weakening the earlier mandatory
  sidecar-distribution decisions.
- Keep the decision within ADR 0103's pre-release policy: no buildType version migration and no
  old-bundle compatibility path.

## Considered Options

- Remove `release-tag` and `provenance-sidecar` while retaining the other caller inputs.
- Retain the complete nine-input interface.
- Move registry, dist-tag, and access to source metadata as well.
- Remove all currently reserved release-distribution inputs.

## Decision Outcome

Chosen option: "Remove `release-tag` and `provenance-sidecar` while retaining the other caller
inputs", because the two removed inputs provide no distinct successful behavior, while the retained
inputs continue to represent package selection, source selection, publish intent, distribution mode,
or an explicit permission choice.

### Input-by-input disposition

| Input                      | Disposition | Reason                                                                                                                                                  |
| -------------------------- | ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `package-directory`        | Retain      | It is the required selector for exactly one package and is outside this reduction.                                                                      |
| `source-ref`               | Retain      | It is the independent, tags-only caller selector for fixed-pipeline release retries.                                                                    |
| `registry-url`             | Retain      | Its current optional publish-intent channel and ADR 0030 registry behavior are unchanged; no source-only migration is made.                             |
| `dist-tag`                 | Retain      | Its current release-channel selection, manifest resolution, and conflict behavior are unchanged.                                                        |
| `access`                   | Retain      | Its current publish-access selection, manifest resolution, and conflict behavior are unchanged.                                                         |
| `release-asset-mode`       | Retain      | It selects npm-only versus npm-plus-Release execution and gates the other distribution behavior. Its current reserved-mode rejection is also unchanged. |
| `release-tag`              | **Remove**  | Omission and every valid supplied value identify the same effective built tag, so the input cannot select an independent target.                        |
| `provenance-sidecar`       | **Remove**  | Omission and `required` already normalize to the same mandatory publication behavior, so the input provides no distinct successful outcome.             |
| `linked-artifact-metadata` | Retain      | It is the explicit opt-in to additional metadata writes and `artifact-metadata: write` authority. Its current reserved-mode rejection is unchanged.     |

### Release target derivation

The public `workflow_call` input `release-tag` is removed. The effective GitHub Release target is
derived exclusively from the effective built release tag established by the existing `source-ref`
and invocation-resolution rules. The publisher continues to require an existing GitHub Release in
the caller repository and to validate that target against the verified producer data.

The internal producer-to-publisher handoff continues to carry the derived release tag. The
standalone publisher's required release-tag input, producer/publisher equality checks, missing-tag
and missing-release failures, and same-repository authority boundary are unchanged. This decision
removes only the redundant public npm workflow input; it does not introduce independent release
selection or remove release identity from the internal composition contract.

The direct suppliedness projection `externalParameters.distribution.release_tag_supplied` is removed
with the public input. There is no longer an omitted-versus-explicit distinction to record. Other
derived release identity fields, including `release.ref` and `release.version_tag`, are not changed
by this ADR and remain subject to their separate decisions.

### Mandatory sidecar representation

The public `workflow_call` input `provenance-sidecar` is removed. Release-asset mode itself now
fully expresses the caller's distribution intent: when that mode is enabled, the publisher always
uploads the unchanged, verified producer provenance bundle as the deterministic Release asset
sidecar. There is no public disable, arbitrary-path, rename, rewrite, re-sign, or replacement
option.

The direct normalized-input projection `externalParameters.distribution.provenance_sidecar` is
removed. Consumers derive the mandatory sidecar requirement from `release_asset_mode`; a separate
`required` or `null` assertion is unnecessary. In npm-only mode there is no Release sidecar to
publish.

The following behavior remains unchanged:

- the sidecar contains the exact verified producer bundle bytes, including the same bundle used for
  npm registry provenance submission;
- the publisher does not create, mutate, re-sign, wrap, or regenerate producer provenance;
- deterministic sidecar naming, digest verification, upload ordering, partial-failure handling, and
  sidecar locator outputs remain in force;
- npm registry-native provenance records and GitHub Artifact Attestation locators remain additional
  discovery surfaces and are not substituted for the Release sidecar;
- linked-artifact metadata remains a separate record about the uploaded primary asset, not the
  provenance bundle.

This resolves ADR 0059's optional-sidecar wording in favor of ADR 0051's and ADR 0057's mandatory
distribution behavior. It does not turn a previously accepted optional distribution capability into
a mandatory one; the current contract already provides no successful sidecar-disabled release mode.

### Retained publish-intent and distribution inputs

`registry-url`, `dist-tag`, and `access` retain their current public inputs, raw provenance input
fields, `publishConfig` resolution, documented defaults, and conflict-rejection behavior. This ADR
retains those caller channels rather than moving these settings to a source-only contract. A later
source-only migration would require a new ADR that weighs SLSA's input-artifact guidance against
concrete same-manifest invocation requirements and ADR 0023's release-channel rationale. Removing
the public `registry-url` input is not a prerequisite for preserving configurable registries, and
retaining it here does not broaden ADR 0030's npmjs-only support guarantee.

`release-asset-mode` and `linked-artifact-metadata` retain their current public inputs and
`externalParameters.distribution` members. The former remains the explicit composition-mode choice;
the latter remains the explicit metadata-write opt-in. The current npm-only implementation's
rejection of enabled but unimplemented distribution modes is unchanged. This ADR neither implements
release-asset mode nor advertises a new operational capability.

`package-directory`, `source-ref`, and every other input or behavior not expressly removed here are
unchanged.

### Scope and compatibility

Only the two redundant public inputs and their two direct external-parameter projections are
removed. This ADR does not change low-level publisher inputs, internal handoff fields, release
identity fields other than `distribution.release_tag_supplied`, publish authentication, registry
support boundaries, sidecar bytes, or the retained inputs' behavior.

Because the project remains pre-release and release-asset composition is unimplemented, there are no
currently supported successful callers of these settings and therefore no practical migration
impact. The current npm-only implementation already rejects non-default values for both inputs.
Callers experimenting with the reserved interface must stop supplying the removed inputs, and the
intended future behavior remains preserved by derivation and the mandatory release-asset-mode
policy.

Consistent with ADR 0103's pre-release policy, no buildType version migration or old-bundle
compatibility path is introduced. Statements and workflow calls carrying removed members are
non-conforming under the revised closed contracts.

### Consequences

- Good, because the public interface contains only inputs that can express a distinct caller choice,
  reducing caller confusion and verifier-relevant surface without weakening a release guarantee.
- Good, because release target identity has one authoritative derivation path while the internal
  publisher and handoff continue to verify that identity.
- Good, because mandatory sidecar publication is explicit as profile behavior rather than obscured
  by an input whose two accepted spellings have identical outcomes.
- Good, because the decision preserves the distinction among npm registry submission, native
  attestation locators, Release sidecar distribution, and linked-artifact metadata.
- Good, because meaningful invocation-time controls and the accepted future composition and
  permission model remain intact.
- Neutral, because the workflow schema, provenance schema, specifications, fixtures, and caller
  examples must remove the two inputs and their direct projections in one coordinated change.
- Bad, because callers can no longer explicitly repeat a matching release tag or explicitly request
  an already-mandatory sidecar, even though neither action changed the outcome.
- Bad, because independently selecting a different release target or making Release sidecar
  publication optional would now require a new ADR and corresponding changes to the existing
  distribution and run-ownership decisions.

## Confirmation

This decision is confirmed when specifications, workflow contracts, test-first conformance cases,
and exact-byte fixtures demonstrate all of the following:

- the public npm `workflow_call` schema contains neither `release-tag` nor `provenance-sidecar`, and
  rejects them as unrecognized inputs, while every retained input keeps its existing name, type,
  default, validation, and resolution behavior;
- the effective Release target is derived from the effective built release tag under the existing
  source-selection rules, and the internal handoff and standalone publisher continue to bind that
  derived tag to the same existing caller-repository Release;
- `externalParameters.distribution.release_tag_supplied` and
  `externalParameters.distribution.provenance_sidecar` are absent and rejected by the closed schema,
  with no replacement suppliedness or sidecar-required assertion;
- release-asset mode always publishes the unchanged, digest-verified producer bundle as the
  deterministic sidecar, and existing naming, upload ordering, failure, and sidecar-output behavior
  is preserved;
- native registry provenance records, GitHub Artifact Attestation locators, and linked-artifact
  metadata are not accepted as substitutes for the mandatory Release sidecar;
- `registry-url`, `dist-tag`, and `access` retain their manifest resolution, defaults, conflict
  rejection, and ADR 0030 registry boundaries without a source-only migration;
- `release-asset-mode` and `linked-artifact-metadata` retain their independent behavior and the
  current npm-only rejection of enabled but unimplemented distribution modes;
- the standalone publisher's release-tag input and the internal handoff's release tag remain, and no
  `release.ref`, `release.version_tag`, sidecar-byte, or other out-of-scope contract is changed by
  this decision;
- no buildType version migration, old-bundle compatibility path, independent release-target
  selection, or sidecar-disabled production mode is introduced.

## Pros and Cons of the Options

### Remove `release-tag` and `provenance-sidecar` while retaining the other caller inputs

The public release target is derived from the effective built tag, sidecar publication becomes an
implicit mandatory consequence of release-asset mode, and all other caller inputs retain their
current behavior.

- Good, because it removes only redundant choices and follows SLSA's guidance to minimize parameters
  and make constant behavior implicit.
- Good, because no successful release behavior is lost and the existing source, publisher, and
  sidecar verification guarantees remain in force.
- Good, because it resolves the optional-sidecar interface ambiguity without replacing Release
  sidecar distribution with registry-native provenance or attestation locators.
- Good, because it preserves meaningful publish-intent, distribution-mode, and permission choices.
- Neutral, because specifications, verifier fixtures, and any experimental caller examples must be
  updated to reject the removed members, although no currently supported successful caller requires
  migration while the project remains pre-release and release-asset composition is unimplemented.
- Bad, because callers experimenting with the reserved interface can no longer explicitly repeat a
  matching tag or an already-mandatory sidecar requirement, although this schema change has no
  practical impact on any currently supported successful release path.

### Retain the complete nine-input interface

The workflow would continue accepting `release-tag` and `provenance-sidecar` under the existing
normalization and validation rules.

- Good, because it preserves the current public schema and avoids changes to experimental caller
  configurations, although avoiding migration has no practical benefit for currently supported
  successful callers because release-asset composition is unimplemented.
- Good, because callers experimenting with the reserved interface can explicitly document a matching
  release tag and their expectation of mandatory sidecar publication in the workflow call.
- Bad, because neither input can produce a distinct successful outcome, so the additional caller and
  verifier surface has no corresponding capability.
- Bad, because retaining `optional provenance sidecar publication behavior` leaves ADR 0059's
  wording in tension with ADR 0051's and ADR 0057's mandatory distribution decisions.
- Bad, because suppliedness and normalized-input assertions must continue to be recorded and checked
  even though they do not change release identity or distribution behavior.

### Move registry, dist-tag, and access to source metadata as well

In addition to removing the two redundant release inputs, the workflow would eliminate the three npm
publish-intent inputs and derive registry, tag, and access exclusively from `publishConfig` and npm
defaults.

- Good, because package publish configuration would have one source-controlled home whose verified
  bytes could also establish those settings for consumers.
- Good, because it would eliminate duplicate input channels and their drift and conflict surface.
- Bad, because it would reverse ADR 0023's express rejection of manifest-only publish intent and
  remove the release-channel flexibility that motivated that decision.
- Bad, because the accepted record does not contain enough concrete same-manifest product evidence
  to decide which invocation-time exceptions remain necessary, especially for dist-tag selection.
- Bad, because custom registry support and the registry input channel are separable concerns;
  removing the channel must not silently turn ADR 0030 into an npmjs-only configuration decision.

### Remove all currently reserved release-distribution inputs

The current npm-only workflow would remove `release-asset-mode` and `linked-artifact-metadata` in
addition to `release-tag` and `provenance-sidecar`.

- Good, because the current callable surface would advertise only operational distribution behavior.
- Good, because all non-default values for these reserved inputs are already rejected by the current
  implementation.
- Bad, because it would remove the accepted future composition's explicit execution-mode choice and
  its separate metadata-write opt-in, not merely redundant inputs.
- Bad, because it would require a broader redesign of ADR 0048's least-privilege consent model and
  ADR 0057's and ADR 0060's single-entrypoint composition interface.
- Bad, because the current implementation's reserved status is not evidence that the future
  distribution and permission controls lack independent value.

## More Information

- SLSA v1.2 Build Provenance, especially the BuildDefinition guidance on minimizing parameters and
  moving configuration into input artifacts:
  <https://slsa.dev/spec/v1.2/build-provenance#builddefinition>
- The evidence for this decision comes from ADR 0023's manifest-only alternative, ADR 0030's
  registry configuration boundary, ADR 0048's explicit metadata opt-in, ADR 0051's mandatory sidecar
  distribution, ADR 0057's public composition, ADR 0058's existing-release authority boundary, ADR
  0059's public input categories, and the current profile, composition, and publisher
  specifications.
- Exact workflow schema, external-parameter schema, validation rules, fixtures, and caller examples
  are to be updated only after this ADR is accepted.
