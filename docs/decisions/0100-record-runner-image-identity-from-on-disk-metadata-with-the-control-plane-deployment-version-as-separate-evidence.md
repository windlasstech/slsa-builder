---
parent: Decisions
nav_order: 100
status: accepted
date: 12026-11-01
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0070
    scope:
      "the clause identifying the runner image's honest observable identity with the environment
      observation ('the honest observable identity is the runner image version and its software
      report, exposed at runtime through ImageOS and ImageVersion') and the matching Confirmation
      clause ('the runtime observation of ImageOS/ImageVersion'): the runner-image descriptor's
      version identity is recorded from the on-disk /imagegeneration/imagedata.json metadata, and
      the ImageVersion environment value is recorded separately as non-authoritative control-plane
      deployment evidence. The recording obligation, the runner-image descriptor's presence in the
      closed descriptor set, the no-distribution-digest rationale, and the source-native
      digest-authority discipline remain in force"
  - type: see-also
    target: ADR-0098
  - type: see-also
    target: ADR-0099
  - type: partially-superseded-by
    target: ADR-0109
    scope:
      "the runner-image descriptor's closed shape: annotations.node_version is removed in favor of
      the single builder.version.nodejs record, and annotations.runner_label is added"
  - type: partially-superseded-by
    target: ADR-0112
    scope:
      "the clauses sourcing image_os from $ImageOS, treating it as required signed capture evidence,
      and requiring ImageOS in capture diagnostics and associated Confirmation coverage:
      operating_system instead records the exact decoded Operating System.detail string from the
      same bounded local metadata snapshot as the URI and image_version; $ImageOS is optional
      diagnostic-only and never required, compared, or used as a fallback. On-disk URI and full
      image_version authority, their sole reported-identity pair, nullable non-authoritative
      non-identifying deployment_version evidence and accepted rollout divergence, independent
      runner_label, the name/uri/annotations shape, descriptor cardinality and serialization order,
      no-public-availability behavior, baseline consumer independence, and all other fail-closed
      local evidence rules remain in force"
  - type: amended-by
    target: ADR-0111
    scope:
      "the identity-strength and verification semantics of the on-disk runner-image record: the
      Included Software URI and exact full Version-line image_version jointly carry a
      buildType-defined reported identity, not immutable URI or image-byte identity and not an
      assertion of public report/release availability; descriptor conformance uses offline
      structural projection, HTTP 404 does not trigger omission of the `runner-image` dependency,
      and missing or malformed local evidence remains fail closed. On-disk authority, verbatim URI
      and image_version capture, separate non-identifying deployment_version evidence,
      rollout-divergence acceptance, the existing annotation set and descriptor cardinality, and all
      unrelated capture and failure rules remain in force"
---

# Record Runner Image Identity from On-Disk Metadata with the Control-Plane Deployment Version as Separate Evidence

## Context and Problem Statement

ADR 0070 records the GitHub-hosted runner image in the closed `resolvedDependencies` schema as the
`runner-image` descriptor, on the premise that "the honest observable identity is the runner image
version and its software report, exposed at runtime through `ImageOS` and `ImageVersion`." The
build-and-pack specification's runner-image capture clause implements this by reading both the
`ImageOS`/`ImageVersion` environment variables and the on-disk `/imagegeneration/imagedata.json`,
and by failing closed with `windlass.verify.error.input-unavailable` when the file's Runner Image
`Version:` line differs from `$ImageVersion`, treating exact equality of the two sources as a
platform invariant.

The fifth live M1 dogfood on 12026-11-01 (run
[41324911503](https://github.com/windlasstech/vers-js/actions/runs/41324911503), recorded in
[issue #30](https://github.com/windlasstech/slsa-builder/issues/30)) falsified that invariant. The
build job failed closed before any install, build, or pack execution with "runner image metadata
version disagrees with ImageVersion"; no registry mutation occurred. A mutation-free probe on the
same runner fleet (run
[41326580253](https://github.com/windlasstech/vers-js/actions/runs/41326580253), temporary
throwaway-branch workflow, deleted afterwards) captured the real bytes and reproduced the
disagreement:

| Source                                            | Observed value                                           |
| ------------------------------------------------- | -------------------------------------------------------- |
| `ImageVersion` environment variable               | `20261026.354.1`                                         |
| `/imagegeneration/imagedata.json` `Version:` line | `20261025.353.1`                                         |
| Job-header Runner Image `Version:` line           | `20261025.353.1` (follows the file, not the environment) |

During an image rollout window the Actions control plane injects the newer deployment's version into
`ImageVersion` while the virtual machine's baked image metadata still describes the previous image.
The two sources therefore name different images by design of the rollout process, and the job log
header follows the file. The sources also disagree on the OS label form (`ubuntu24` in the
environment versus `ubuntu-24.04` in the file), though both denote the same image line. Supporting
upstream facts: no public `ubuntu24/20261025.353` tag exists in `actions/runner-images` (the tag
sequence runs `ubuntu24/20261023.334` then `ubuntu24/20261026.354`), and the image-data document at
`ubuntu24/20261026.354` records `Image Version: 20261026.354.1`, matching the injected environment
value. The full mechanics of why the two sources diverge, and why no equality invariant can hold
between them, are laid out under Rollout-window mechanics in More Information.

The defect is latent since the capture's introduction (PR #68), not a regression: dogfood attempts 2
through 4 passed the same check only because their runners were not in a rollout window. The current
contract has two failure modes during every rollout window, which recurs on GitHub's roughly weekly
image cadence: honest builds fail closed on a non-defect, and had the environment been authoritative
instead, provenance would have recorded an image the virtual machine did not contain.

Which source should be authoritative for the runner-image identity recorded in provenance, and how
should the control-plane deployment version be treated?

## Decision Drivers

- Provenance must testify to the build environment that actually executed the workload; recording a
  version the virtual machine did not contain is a false statement under signature.
- A platform rollout is normal operation, not an anomaly: honest builds must not fail closed on a
  planned fleet transition, and the drift must not be silently erased either.
- Keep the `resolvedDependencies` schema closed and keep fail-closed behavior for evidence that is
  genuinely absent or malformed (missing file, missing group or detail lines, empty Included
  Software URL).
- Minimize contract churn: the v1 `buildType` contract is not yet publicly frozen, but unrelated
  descriptor axes (`image_os`, the digest-free rationale) should not move without cause.

## Considered Options

- On-disk metadata authoritative, environment as separate evidence
- Record both, downgrade disagreement to a warning
- Keep strict equality, reclassify as retryable
- Environment variable authoritative
- Do not record the runner image

## Decision Outcome

Chosen option: "On-disk metadata authoritative, environment as separate evidence", because it is the
only option under which the signed record always describes the image whose bits actually executed
the build, while still preserving the rollout drift as inspectable evidence instead of a build
failure or a silent erasure.

Concretely, the runner-image capture clause of the build-and-pack specification is amended so that:

- The `runner-image` descriptor's `image_version` annotation carries the on-disk file's Runner Image
  `Version:` line, verbatim. The descriptor `uri` remains the file's Included Software URL,
  unchanged.
- A new `deployment_version` annotation carries the `ImageVersion` environment value verbatim when
  the platform supplies it. A disagreement between `image_version` and `deployment_version` is a
  recorded fact, not a failure: it is exactly the rollout-window signature and carries forensic
  value.
- The `image_os` annotation continues to carry `$ImageOS`, unchanged; the drift observed in practice
  is confined to the version axis, and the environment and file OS labels denote the same image line
  under different surface forms.
- Fail-closed behavior is retained where evidence is genuinely missing or malformed: an unavailable
  or malformed file, a missing or repeated Runner Image group or detail line, or an absent or empty
  Included Software URL still stops the build before predicate construction with
  `windlass.verify.error.input-unavailable` (exit code `2`), and a descriptor shape violation still
  stops with `windlass.verify.error.resolved-dependencies-runner-image` (exit code `1`). The exact
  equality cross-check between the file version and the environment variable is removed.
- Capture-unavailable diagnostics carry the observed `ImageOS` and `ImageVersion` environment
  values, so a failure no longer requires a probe round trip to diagnose.

The change rides the established observe → ADR → specification → TDD path: the specification
amendment and conformance fixtures (including an accepted divergence fixture and the retained
fail-closed rejections) land with the implementation, after which the vers-js caller pin is bumped
and the `v0.1.3` dogfood retries.

### Consequences

- Good, because the signed runner-image record always describes the image whose bits executed the
  build, including during rollout windows.
- Good, because rollout drift becomes visible in provenance as a recorded fact rather than a build
  failure or an erased discrepancy.
- Good, because fail-closed posture is preserved on every failure mode that indicates real evidence
  loss or corruption.
- Good, because the diagnosis of any future failure needs no probe: the diagnostic carries both
  operands.
- Neutral, because the descriptor schema gains one annotation and the closed-schema fixtures grow
  accordingly.
- Bad, because the recorded `image_version` can no longer be assumed to equal the platform-reported
  deployment version; consumers comparing the two must read the annotations' semantics from the
  specification.

## Confirmation

This decision is confirmed when:

- the build-and-pack specification's runner-image capture clause names the on-disk file as the
  authority for `image_version`, defines the `deployment_version` annotation, and removes the
  equality invariant while retaining the fail-closed rejection cases;
- the trusted core records the file version and the environment value as specified, and the
  failure-path diagnostics carry both operands;
- conformance fixtures cover the accepted divergence case (environment ahead of the file) and every
  retained rejection (missing or malformed file, missing group or detail lines, empty Included
  Software URL, descriptor shape violation);
- the `v0.1.3` dogfood retry after the caller pin bump passes the runner-image capture on the live
  fleet, including during a rollout window if one is in progress.

## Pros and Cons of the Options

### On-disk metadata authoritative, environment as separate evidence

The `runner-image` descriptor takes `image_version` from the baked file verbatim, the `uri` stays
the file's Included Software URL, and the `ImageVersion` environment value moves to a separate
non-authoritative `deployment_version` annotation.

- Good, because the signed record always describes the image whose bits actually executed the build:
  the file is baked into the virtual machine, so it cannot drift away from the workload it
  describes, while the environment value demonstrably can and does during rollouts.
- Good, because the rollout signature becomes self-describing in provenance: anyone comparing the
  two annotations sees a fleet mid-transition, and the divergence is preserved as inspectable
  evidence rather than failing the build or being erased.
- Good, because the failure taxonomy is untouched: no new diagnostic IDs, no warning class, and no
  verifier-side tolerance language. The closed schema gains exactly one annotation, which is the
  smallest possible contract delta that carries the evidence.
- Good, because fail-closed behavior survives exactly where it matters: absent, unreadable, or
  malformed evidence still stops the build before signing, so the change removes only the falsified
  invariant and nothing else.
- Neutral, because the two annotations agree outside rollout windows and diverge inside them by
  design; consumers must read the specification to learn which field is authoritative instead of
  assuming the platform reports a single version.
- Bad, because downstream automation that compares the record against GitHub's environment reporting
  must compare against `deployment_version`, not `image_version`; any such consumer written against
  the old semantics reads rollout windows as mismatches.
- Bad, because the schema, the specification, and the fixture corpus each grow by one annotation and
  its accepted and rejected cases, including a live-fleet divergence fixture.

### Record both, downgrade disagreement to a warning

Both values are recorded and a disagreement emits a warning diagnostic while the build continues.

- Good, because both values are preserved and the build survives rollout windows.
- Good, because a warning is more visible to a human reading the run log than a silent annotation
  difference.
- Bad, because it combines the weakest properties of both enforcement postures: the record's
  integrity floor drops to the weaker of the two sources on exactly the occasions when they
  disagree, while the verifier contract must additionally answer whether a warned divergence is
  acceptable, a question with no principled answer that every downstream policy would have to
  re-litigate.
- Bad, because warnings are operationally invisible: nobody pages on them, so a genuinely suspicious
  divergence, such as a tampered metadata file, rides through under the same warning as a benign
  rollout and is indistinguishable from it at triage time.
- Bad, because the cost is higher than the chosen option for a weaker guarantee: a new warning
  diagnostic ID, its registry and specification-table entries, fixture cases, and verifier-policy
  tolerance language all add surface that the chosen option does not need.

### Keep strict equality, reclassify as retryable

The check is unchanged, but the failure is documented as a transient infrastructure condition to be
retried after the rollout settles.

- Good, because the trust model changes the least and, when the check passes, the recorded
  consistency between the two sources is the strongest form of the signal.
- Good, because no specification, schema, or fixture work is required.
- Bad, because it misclassifies a specification error as an infrastructure condition: the platform
  never promised the two sources agree, so the build fails on a contract the platform does not
  offer, and the failure recurs on every image rollout, which follows GitHub's roughly weekly image
  cadence.
- Bad, because a rollout window lasts days and its end is not observable from inside a runner, so
  "retry later" has no bounded ETA: releases and dogfoods landing in a window fail
  nondeterministically, and repeated retries can re-enter the same window.
- Bad, because the failure still names neither operand, so every recurrence costs the same
  diagnostic round trip that this dogfood paid, unless the observability improvement rides along
  anyway.
- Neutral, because during the long stable periods between rollouts the check passes and the record
  happens to be consistent; consistency by coincidence is not a verification property.

### Environment variable authoritative

The `ImageVersion` environment value is recorded as the image identity and the file is demoted to a
cross-check or dropped.

- Good, because it matches the platform's own reported deployment version, the value GitHub's
  tooling and status surfaces describe.
- Good, because the capture code is simplest: no file read, no parsing, no detail-line validation.
- Bad, and disqualifying, because during precisely the windows at issue the environment value
  describes an image whose bits are not in the virtual machine: the signed provenance would testify
  to a fiction, which violates the project's standing rule that it does not issue provenance it
  cannot stand behind.
- Bad, because environment variables are control-plane-injected and comparatively easy to influence
  relative to the baked filesystem; making them authoritative lowers the evidentiary floor of the
  record for every build, not only during rollouts.
- Neutral, because outside rollout windows the two sources agree and the defect is invisible; a
  trust record whose defect surfaces exactly when the platform is in transition is the worst kind of
  latent behavior.

### Do not record the runner image

The `runner-image` descriptor is removed from the closed schema and the capture surface disappears.

- Good, because this entire failure class ceases to exist and the schema shrinks.
- Bad, because it reopens the traceability gap ADR 0070 closed at the v1 contract freeze: the runner
  image is part of the build environment, and without the record a vulnerable or compromised image
  revision can no longer be correlated with the artifacts it produced after the fact.
- Bad, because removal is a regression on the frozen v1 descriptor set: once v1 ships without the
  descriptor, re-adding it becomes a `buildType` versioning event, which is exactly the cost ADR
  0070 paid specification and fixture surface to avoid.
- Neutral, because SLSA treats `resolvedDependencies` as best effort, so no conformance level is
  formally lost; the project's evidentiary standard is deliberately higher than that floor.

## More Information

This decision follows the observe → ADR → specification → TDD remediation path established by
ADR 0081. Evidence:

- Dogfood attempt 5 run: <https://github.com/windlasstech/vers-js/actions/runs/41324911503>
  (fail-closed, zero registry mutation; evidence record in `docs/dogfood/npm-m1.md`).
- Mutation-free probe run capturing the disagreement:
  <https://github.com/windlasstech/vers-js/actions/runs/41326580253>.
- Issue thread: <https://github.com/windlasstech/slsa-builder/issues/30>.
- Upstream tag sequence in `actions/runner-images`: `ubuntu24/20261023.334` →
  `ubuntu24/20261026.354`, with no public `ubuntu24/20261025.353` tag; the image-data document at
  `ubuntu24/20261026.354` records `Image Version: 20261026.354.1`.
- Latency of the defect: the equality check entered with PR #68 and was byte-identical at the
  attempt-4 pin; attempts 2 through 4 passed because their runners were outside rollout windows.

### Predecessor precedent: slsa-github-generator

The predecessor project recorded runner image information from environment variables only and never
cross-checked it against the on-disk image metadata. The Go builder recorded `RUNNER_ARCH` and
`ImageOS` into the invocation environment and built a runner image release material URL from
`ImageOS` and `ImageVersion` (`internal/builders/go/pkg/provenance.go`), with no read of
`/imagegeneration/imagedata.json` and no equality or drift check.

Its later history is directly relevant. Issue
[slsa-github-generator#1817](https://github.com/slsa-framework/slsa-github-generator/issues/1817)
("Don't record RUNNER_XXX") observed that these values came from the caller and might not represent
the runner that actually executed the build, and
[PR #2010](https://github.com/slsa-framework/slsa-github-generator/pull/2010) removed `IMAGE_OS`,
`IMAGE_VERSION`, `RUNNER_ARCH`, `RUNNER_NAME`, and `RUNNER_OS` from the generated predicates.
Reintroduction was deferred until a trustworthy way to retrieve the build job's values exists
([slsa-github-generator#2027](https://github.com/slsa-framework/slsa-github-generator/issues/2027),
closed unresolved).

The predecessor therefore answered the "environment values may misrepresent the build runner"
concern by removal. The probe evidence behind this ADR is a concrete instance of exactly that
misrepresentation: during a rollout window the environment names an image the virtual machine does
not contain. This ADR instead keeps the record and repairs its authority: the on-disk metadata is
authoritative for the image identity, and the environment value is demoted to separate,
non-authoritative evidence. No handling of per-deployment rollout-window drift exists in the
predecessor; the file-authoritative mechanism is new ground relative to it.

### Rollout-window mechanics

This section records why the two version sources diverge, separating what the probe pinned
empirically from the platform mechanism the evidence supports. The structural core is that the two
values are not one truth written twice: they have different producers and different write times.

|            | `/imagegeneration/imagedata.json` (file)                   | `ImageVersion` (environment variable)              |
| ---------- | ---------------------------------------------------------- | -------------------------------------------------- |
| Producer   | The image build pipeline (the image-data report generator) | The Actions control plane                          |
| Written at | Image bake time; immutable with the snapshot afterwards    | Virtual machine provisioning and registration time |
| Lifecycle  | Bound to the image snapshot                                | Bound to the virtual machine instance              |

The images in `actions/runner-images` are prebuilt snapshots, and the file is baked into the
snapshot. The environment value is not read from the guest; it is injected from outside. In steady
state the version the control plane injects matches the version deployed to the pool, so the two
sources happen to agree. That agreement is not a contract the platform offers.

A rollout window then unfolds as follows:

1. A new image version is registered and the pool's current target version flips to it (for the
   observed window: `20261026.354.1`).
2. For the duration of the rollout, which takes days, virtual machines created from the previous
   snapshot and virtual machines created from the new snapshot coexist in the pool. The control
   plane injects the new target version into every virtual machine that takes a job, regardless of
   the snapshot the virtual machine was actually created from.
3. When the rollout completes, every virtual machine runs the new image and the two sources agree
   again.

The probe's three observations land exactly on this timeline. The environment value `20261026.354.1`
is the pool's target version at provisioning time. The file's `Version: 20261025.353.1` line says
the virtual machine's disk was baked from the previous snapshot. And the job log header printing
`20261025.353.1` shows that the header is rendered from the guest-side metadata, not from the
environment variable; reading the header as the environment value is a diagnostic trap, which is why
the failure-path diagnostics must carry both operands.

The version anatomy adds a second crack in the surface. Public release tags are three-component
(`ubuntu24/20261026.354`) while the file and the environment value are four-component
(`20261026.354.1`: date, build number, deployment revision). And no public `ubuntu24/20261025.353`
tag, branch, or release exists at all, so the image the probe ran on was deployed to the fleet
without a published counterpart. This absence was verified exhaustively at ADR time: the exact git
ref endpoints for the tag and the branch return 404, the full paginated listing of all 104
`ubuntu24/*` tags contains no `353`, and the release list (including prereleases) runs
`ubuntu24/20261023.334` (published 2026-10-23) directly to `ubuntu24/20261026.354` (published
2026-10-26). Note the exact strength of this evidence: it proves that no public tag or release
exists now, not that none ever existed, since tags and releases can be deleted.

That deletion path is not speculative; it is the documented release lifecycle. The
`actions/runner-images` README states that a pre-release is created when a deployment starts and is
converted to a release only when the deployment finishes, that deployment usually takes 2-3 days on
a weekly cadence, and that the `main` branch documentation is updated only when deployment is
finished. The most probable reading of `20261025.353.1` is therefore that the 353 deployment began
and propagated to part of the fleet, was superseded by 354 before completion, and its pre-release
was never converted and was removed. Whether that pre-release ever existed is not provable from
public data after the fact, and it does not need to be: the load-bearing facts are pinned by the
probe bytes and the API lookups, namely that the fleet served a file version disagreeing with the
injected environment value, and that the served image has no public release.

Serving an image version with no published release is also a known, previously reported platform
behavior rather than a novel anomaly. In
[runner-images#11574](https://github.com/actions/runner-images/issues/11574) users observed jobs
landing on image versions absent from the releases list with dangling release links, and the
maintainer answer was that rollouts take 2-3 days with both versions coexisting, in effect that this
is how deployment works. In
[runner-images#12720](https://github.com/actions/runner-images/issues/12720) the maintainers confirm
the progressive rollout (3 days minimum), name the completion signals (the prerelease tag
disappears, the documentation pull request merges), and acknowledge pool inconsistency during an
internal change to the image distribution mechanism.

A practical consequence of the same observation: during such a window the file's Included Software
and Image Release URLs dangle, so even the file-authoritative record can point at documentation that
does not (or no longer) exist. The record's purpose is after-the-fact correlation, and a dangling
URL still carries evidentiary value in that it names the image, but consumers should not expect
every recorded URL to resolve.

The equality invariant this ADR removes asked an immutable snapshot artifact written at bake time to
always equal a control-plane view written at provisioning time. As long as a deployment event can
land between those two write times, the equality is an occasional coincidence, not a platform
invariant; and with GitHub's observed roughly weekly image cadence, any release or dogfood run has a
standing chance of landing inside a rollout window.
