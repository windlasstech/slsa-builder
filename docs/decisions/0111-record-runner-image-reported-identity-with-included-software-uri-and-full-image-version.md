---
parent: Decisions
nav_order: 111
status: accepted
date: 12026-11-11
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0070
    scope:
      "the runner-image URI and identity/availability semantics: the broad permission to identify
      either a runner image release or its software report is replaced by the verbatim
      platform-supplied Included Software URI, paired with the exact full on-disk image_version as a
      buildType-defined reported identity rather than immutable or image-byte identity; the
      consequence making the descriptor depend on continuing publication is replaced by offline
      structural projection against local evidence, with no public-availability gate. The
      runner-image role, required resolvedDependencies membership and cardinality, annotation-based
      image_version recording, URI-based digest-free representation, no-distribution-digest
      rationale, and all package-manager and runtime recording decisions remain in force"
  - type: see-also
    target: ADR-0071
  - type: amends
    target: ADR-0100
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
  - type: see-also
    target: ADR-0105
  - type: see-also
    target: ADR-0109
  - type: partially-superseded-by
    target: ADR-0112
    scope:
      "the clauses retaining image_os as a required environment-backed annotation and the associated
      capture and Confirmation requirements: image_os is replaced by operating_system, exactly the
      decoded Operating System.detail string from the producer's bounded local metadata snapshot,
      and ImageOS becomes optional diagnostic-only with no required, comparison, or fallback role.
      The exact name/uri/annotations top-level shape, the verbatim Included Software URI plus exact
      full image_version as the sole reported-identity pair, operating_system's non-identifying
      context role, nullable deployment_version and independent runner_label, descriptor cardinality
      and serialization order, offline URI/version projection, no-public-availability behavior,
      baseline consumer independence from raw metadata, and the allowance for a later separately
      governed vendor-prefixed evidence annotation remain in force; ADR 0112 adopts no such
      additional annotation"
---

# Record Runner-Image Reported Identity with the Included Software URI and Full Image Version

## Context and Problem Statement

ADR 0070 requires exactly one `runner-image` entry in `resolvedDependencies` and permits its URI to
identify either a runner image release or its software report. ADR 0100 subsequently made the
on-disk `/imagegeneration/imagedata.json` metadata authoritative for the actual image version, while
keeping the control-plane `ImageVersion` as separate `deployment_version` evidence. The descriptor
already carries the platform-supplied Included Software URI and the full on-disk version, but the
strength and limits of their combined identity are not explicit.

The mutation-free probe on 12026-11-01 captured this concrete pair from the same local Runner Image
metadata snapshot:

- Full `Version:` value: `20261025.353.1`.
- Included Software URI:
  `https://github.com/actions/runner-images/blob/ubuntu24/20261025.353/images/ubuntu/Ubuntu2404-Readme.md`.
- Separate control-plane deployment `ImageVersion`: `20261026.354.1`.

As observed on 12026-11-11, the 353 report and release return HTTP 404, while the 354 report is
available. The local evidence therefore identifies the image that actually hosted the build even
though its public report and release are not retrievable. Treating HTTP availability as an identity
check would discard that valid evidence or substitute the deployment's different image version.
Treating the URI as an immutable or complete image identity would instead claim more than the
platform supplies.

Upstream's `configure-image-data.sh` constructs the report and release references from the major and
minor version components and omits the patch component. Thus the URI contains the release-tag
projection `20261025.353`, while only the annotation retains the full observed `20261025.353.1`.
SLSA and in-toto permit a buildType to define dependency semantics and permit URI-only resource
descriptors, but that permission does not itself provide immutable image bytes, report integrity, or
permanent public availability.

How should the existing `runner-image` resolved dependency express the actual runner image's
identity so that the platform-supplied Included Software URI and exact full on-disk `image_version`
remain useful without claiming immutable image bytes, public report availability, or control-plane
deployment identity?

## Decision Drivers

- Preserve the actual on-disk runner image in the existing dependency record, including during a
  rollout where the control-plane deployment version names a different image.
- Define the identity carried by the Included Software URI and full `image_version` as explicit
  Windlass `buildType` semantics rather than assuming universal SLSA semantics.
- Distinguish identity from retrievability and integrity. A useful reported identifier need not be a
  retrievable locator or a digest of image bytes.
- Preserve both values verbatim from one local metadata snapshot. Do not reconstruct, normalize,
  truncate, substitute, or rewrite either value to obtain a more convenient identity.
- Keep producer and baseline consumer verification deterministic and offline for this descriptor.
  Public DNS, HTTP, tag, release, and report availability must not gate signing or verification.
- Retain the full observed version while recognizing that the URI carries only the upstream
  major/minor release-tag projection, not the patch component.
- Preserve the closed top-level descriptor shape, the names, types, and meanings of the existing
  four annotations, cardinality, deterministic serialization order, acquisition mechanism, and
  runner selection. Allow a vendor-prefixed evidence annotation only through a separate explicit
  decision and closed nested schema, not by arbitrary extension of this contract.
- Keep genuinely missing or malformed local evidence fail closed. An unavailable public report is
  not evidence that the local metadata is missing or malformed.
- Keep `deployment_version` non-authoritative and non-identifying. It is neither an identity
  fallback nor an equality requirement.
- Avoid deciding raw environment evidence placement, serialization, digesting, transport, or
  retention in this identity-focused decision.

## Considered Options

- Included Software URI with full-version reported identity
- Included Software URI with public-availability enforcement
- Version-derived release URI with full-version reported identity
- Digest-bound VM image identity
- Control-plane deployment version as image identity
- No runner-image dependency

## Decision Outcome

Chosen option: "Included Software URI with full-version reported identity", because it preserves the
strongest identity actually present in the local runner metadata without fabricating image-byte
integrity or making valid provenance depend on a public report that may not exist.

### Descriptor and preserved contract

Exactly one `runner-image` descriptor remains in `resolvedDependencies`. Its allowed top-level
members remain exactly `name`, `uri`, and `annotations`:

- `name` remains `runner-image`.
- `uri` is the verbatim, non-empty, platform-supplied Included Software URI read from the local
  Runner Image metadata. It is not reconstructed from a version, replaced with a release URL, or
  converted to a permanent link.
- `annotations.image_version` is the exact full version string on the on-disk Runner Image
  `Version:` line. It retains every component, including the patch component absent from the URI's
  release-tag projection.

Together, `uri` and `annotations.image_version` form the runner image's **reported identity**. For
the probe snapshot, the conforming identity portion is:

```json
{
  "name": "runner-image",
  "uri": "https://github.com/actions/runner-images/blob/ubuntu24/20261025.353/images/ubuntu/Ubuntu2404-Readme.md",
  "annotations": {
    "deployment_version": "20261026.354.1",
    "image_os": "ubuntu24",
    "image_version": "20261025.353.1",
    "runner_label": "ubuntu-24.04"
  }
}
```

The existing four annotations `deployment_version`, `image_os`, `image_version`, and `runner_label`
remain required with their current names, types, and meanings. None is removed, renamed, retyped, or
given a second identity meaning. This decision does not close the annotation map against a
separately governed, vendor-prefixed evidence annotation, including an annotation containing a
nested evidence ResourceDescriptor. Such an annotation and its nested schema are outside this
decision and require their own explicit ADR and specification; arbitrary additional annotations
remain non-conforming. The overall descriptor cardinality remains six for npm selections and seven
for pnpm or Yarn selections, with exactly one `runner-image` in either case. Producer serialization
remains `invocation-source`, `built-source`, `package-manifest`, `lockfile`, `node-runtime`, the
conditional `package-manager-distribution`, then `runner-image`. Semantic verification remains
unordered and name-keyed.

`digest`, `content`, `downloadLocation`, and every other new descriptor member, including
`mediaType`, are prohibited. The decision adds no second runner descriptor and no alternate report,
release, deployment, or image-byte descriptor. Acquisition and runner selection are unchanged.

The `mediaType` exclusion follows from the resource identified by this descriptor, not from a
general SLSA or in-toto prohibition. The descriptor represents the runner image, and the platform
supplies no media type for the VM image or disk snapshot. The Included Software URI's Markdown
report and the on-disk JSON metadata have their own media types, but neither is the media type of
the runner image. Assigning `text/markdown` or `application/json` would describe a related evidence
resource rather than the dependency identified here. The exclusion prevents that substitution and
preserves the existing closed shape; it does not prevent a separate evidence descriptor from
carrying an accurate media type under its own contract.

Identity does not move into `builder.version` or `externalParameters`, and no duplicate runner-image
identity member is introduced there.

This decision does not decide raw environment evidence placement, serialization, digesting,
transport, or retention. Those questions are not prerequisites for the reported-identity semantics
selected here.

### Three identity levels

The descriptor distinguishes three levels that must not be conflated:

| Level                                | Evidence and meaning                                                                                                                                                   | Explicit limit                                                                                                                       |
| ------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| Version-bearing release-tag identity | The Included Software URI contains the upstream release-tag projection, such as `ubuntu24/20261025.353`, and names the corresponding software-report path.             | The URI omits the patch component and is not established to uniquely identify a full version.                                        |
| Full observed reported identity      | The verbatim URI plus the exact full `annotations.image_version` jointly identify the locally reported runner image as `20261025.353.1` with its 353 report reference. | This is a buildType-defined assertion about platform-reported metadata, not an immutable identifier or proof of public availability. |
| Actual image-byte identity           | None is supplied by this descriptor.                                                                                                                                   | No VM image bytes, disk snapshot, or report content is digest-bound, authenticated, or claimed immutable by this decision.           |

Upstream `configure-image-data.sh` uses the major/minor projection, meaning the first two numeric
components of the full version, and drops the patch component when forming these report and release
references. The projection of `20261025.353.1` is `20261025.353`; the final `.1` survives only in
`image_version`. Whether multiple platform-issued full versions can share one tag projection has not
been established. The contract assumes no one-to-one mapping between a release tag and a full image
version, and consumers must not infer uniqueness from the URI.

Identity, retrievability, and integrity have separate meanings:

- **Identity** is the reported URI-plus-full-version pair defined by this buildType.
- **Retrievability** is an independent, time-varying property of a public report, tag, or release.
  The URI remains meaningful as recorded identity even when no resource resolves.
- **Integrity** of the VM image or report is not established. The provenance signature protects the
  statement containing this reported assertion; it does not turn the assertion into a digest of the
  image or report.

This is deliberately weaker than an immutable-URI or digest-bound dependency convention. It is a
permissible ResourceDescriptor and SLSA buildType convention, not a claim that the mutable report
URI satisfies an immutable-URI assumption.

### Offline comparison and failure semantics

The producer captures and checks the descriptor URI and full `image_version` against one local
Runner Image metadata snapshot. It does not independently source either value from the control
plane, a public report, or a release page. Baseline consumers validate the signed URI/version pair
and supported projection offline and are not required to obtain the raw local snapshot. Any
downstream comparison with raw evidence belongs to the later evidence contract, not to baseline
consumer verification.

For the supported GitHub runner-image grammar, producer and consumer conformance also validate the
supported tag projection offline. The full version's first two numeric components are projected to
the release-tag form and compared with the tag component already present in the captured Included
Software URI. In the probe example, the validation-only projection `20261025.353` must agree with
the URI's `ubuntu24/20261025.353` component. The projection is never emitted in place of either
captured value. Neither the URI nor the full version is reconstructed, normalized, truncated,
substituted, or rewritten to force a match. A malformed full version, unsupported URI grammar, or
projection mismatch is rejected rather than repaired.

No DNS, HTTP, tag, release, or report lookup is required for predicate construction, signing, or
baseline verification. In particular, HTTP 404 does not select omission of the `runner-image`
dependency, emit a warning, or reject otherwise valid local evidence. A public 200 response is
likewise not an integrity or identity proof. Network observations do not replace the local
comparison.

The existing fail-closed rules remain unchanged for genuinely deficient local evidence. An
unavailable or malformed metadata file; a missing, malformed, or repeated Runner Image group or
required detail; a missing, empty, or malformed full `Version:` value; or a missing, empty, or
malformed Included Software URI stops capture before predicate construction. On the producer side, a
descriptor that disagrees with the local snapshot is rejected. Producers and consumers reject an
invalid projection or a closed-shape violation. No public lookup can rescue those failures, and no
guessed value can replace them.

`deployment_version` remains separate control-plane evidence. It is non-authoritative,
non-identifying, and outside the reported-identity pair. It can differ from `image_version` during a
normal rollout, and equality is never required. It cannot replace a missing or malformed on-disk
version or URI, serve as an identity fallback, or make the available 354 report a substitute for the
locally observed 353 identity.

### Consequences

- Good, because the signed record preserves both the exact full on-disk version and the platform's
  own report reference for the image that actually hosted the build.
- Good, because the buildType now distinguishes release-tag identity, full reported identity, and
  image-byte identity instead of letting a URI imply stronger guarantees than it carries.
- Good, because valid local evidence remains signable and verifiable when a report or release is
  unavailable, including the observed 353 rollout case, without adding a network dependency.
- Good, because the existing descriptor shape, annotations, cardinality, ordering, acquisition, and
  runner selection remain stable. Existing verification gains clearer semantics rather than a new
  descriptor family.
- Good, because offline structural projection detects corrupted or mismatched URI/version pairs
  without rewriting them or treating a public 404 as local evidence failure.
- Neutral, because the reported identity is useful for correlation and forensic analysis but is
  weaker than immutable or digest-bound identity. Consumers must understand the buildType-specific
  convention rather than apply a generic immutable-URI assumption.
- Neutral, because the Included Software URI can be mutable or dangling. Its presence in signed
  provenance records what the platform supplied, not what a later network observer can retrieve.
- Bad, because the descriptor cannot establish VM image-byte integrity, report integrity, or
  byte-level reproducibility, and it does not satisfy a policy that requires immutable dependency
  identity.
- Bad, because consumers need the upstream major/minor projection rule to validate the pair, and a
  URI-only view can lose the patch component unless `image_version` is evaluated with it.

### Confirmation

After explicit acceptance, architecture review, conformance fixtures, and test-first implementation
must demonstrate the following cases without adding a runner-image network dependency.

Accepted cases:

- The probe-derived descriptor carries the exact 353 Included Software URI and
  `image_version: "20261025.353.1"`. It is accepted even when both the 353 report and release return
  HTTP 404. The result neither warns nor selects omission of the `runner-image` dependency.
- The same descriptor carries `deployment_version: "20261026.354.1"`. The 354/353 deployment
  divergence is accepted as rollout evidence, with no equality check and no change to the reported
  identity.
- Offline projection validation accepts the pair when the full version `20261025.353.1` projects to
  `20261025.353` and the captured URI contains `ubuntu24/20261025.353`. The emitted URI and full
  version remain byte-for-byte the captured values.
- A network-disabled conformance run rejects any attempted runner-image DNS, HTTP, tag, release, or
  report lookup while exact predicate construction, the existing hermetic signing test, and baseline
  verification still succeed for the valid descriptor.
- Existing required-annotation and descriptor-cardinality tests continue to pass with the same four
  required annotations, the same six/seven overall counts, exactly one `runner-image`, and the
  established serialization order. Acquisition and runner-selection tests are unchanged. Arbitrary
  additional annotations remain rejected, but these tests do not preclude a vendor-prefixed evidence
  annotation admitted later under its own explicit ADR and closed nested schema.

Rejected cases:

- The available 354 identity cannot replace the local 353 evidence. On the producer side, changing
  the captured URI to the 354 report, changing `image_version` to `20261026.354.1`, or copying
  `deployment_version` into `image_version` is rejected when the local snapshot contains the 353
  values. Public availability does not make the substitution valid. Consumer-side checks remain
  limited to the signed URI/version pair, its supported projection, and the closed descriptor shape;
  no comparison with the raw local 353 snapshot is required.
- A projection mismatch is rejected. For example, full version `20261025.353.1` paired with an
  Included Software URI containing `ubuntu24/20261026.354` fails offline validation. The
  implementation must not truncate, rewrite, or normalize either value to make the pair pass.
- Missing or malformed local evidence remains fail closed: an absent or malformed metadata file;
  missing, malformed, or repeated Runner Image group or required detail; missing, empty, or
  malformed full version; and missing, empty, or malformed Included Software URI are all rejected.
  `deployment_version`, an HTTP response, and a reconstructed release URI are not fallbacks.
- A `digest`, `content`, `downloadLocation`, `mediaType`, unknown top-level member, additional
  runner-image descriptor, additional identity annotation, or arbitrary ungoverned annotation is
  rejected. Moving the identity into `builder.version` or `externalParameters` is also
  non-conforming. This rejection does not cover a vendor-prefixed evidence annotation admitted by a
  later ADR with its own closed nested schema, provided that annotation does not acquire an identity
  meaning or change the existing four annotations.
- Any conformance path that requires a DNS or HTTP response, treats HTTP 404 as omission of the
  `runner-image` dependency, warns or rejects solely because public evidence is unavailable, or
  requires `deployment_version` to equal `image_version` is non-conforming.

Review must also confirm that no test interprets the URI alone as the full version, asserts a
one-to-one tag/full-version mapping, claims VM image or report integrity, or treats the reported
identity as control-plane deployment identity.

## Pros and Cons of the Options

### Included Software URI with full-version reported identity

Keep the verbatim platform URI and exact full on-disk version as one buildType-defined reported
identity, with offline projection validation and no availability gate.

- Good, because it preserves the actual local 353 evidence even when the corresponding public report
  and release are absent, while retaining the deployment's 354 value only as separate evidence.
- Good, because the pair carries more precise identity than either member alone: the URI supplies
  the release-tag/report reference and the annotation supplies the complete observed version.
- Good, because it changes no acquisition, runner selection, descriptor cardinality, existing
  annotation name, type, or meaning, or serialization order, and signing and baseline verification
  stay offline for this record.
- Good, because its explicit limits avoid false claims of immutable identity, public retention,
  report availability, or image integrity while remaining compatible with URI-only
  ResourceDescriptor semantics.
- Neutral, because consumers must learn the buildType-specific pair and projection semantics rather
  than treating every resolved dependency URI as an immutable resource locator.
- Bad, because there is still no digest-bound identity for the VM image or report and no guarantee
  that the URI will remain retrievable.
- Bad, because the validation rule depends on the supported upstream URI/version grammar and cannot
  infer a one-to-one mapping between a tag and full version.

### Included Software URI with public-availability enforcement

Keep the Included Software URI but require a successful public report, tag, or release lookup before
accepting or verifying the descriptor.

- Good, because a successful lookup can help a consumer inspect the published software inventory at
  verification time and detect some malformed or unexpected references.
- Good, because it makes current public retrievability explicit rather than leaving a consumer to
  discover a dangling report later.
- Neutral, because a 200 response can establish that a resource was available at one observation
  time, but it still does not establish full-version uniqueness, image-byte identity, or future
  retention.
- Bad, because it conflates identity with availability and rejects the valid 353 local evidence on
  exactly the rollout path that motivated the decision.
- Bad, because DNS, HTTP, tag, release, and report behavior makes signing or baseline verification
  nondeterministic and prevents a network-disabled proof.
- Bad, because HTTP 404 would pressure the contract toward omission of the `runner-image`
  dependency, warnings, or substitution of the available 354 deployment identity, all of which
  discard or falsify sound local evidence.

### Version-derived release URI with full-version reported identity

Reconstruct a runner-images release URI from the full on-disk version and pair that derived URI with
`image_version` instead of retaining the platform-supplied Included Software URI.

- Good, because the upstream major/minor projection supplies a deterministic offline rule for
  deriving a release-tag-shaped reference when that reference is all a consumer needs.
- Good, because retaining the full annotation alongside the derived reference would preserve the
  patch component that the release tag omits.
- Neutral, because a derived release URI and an Included Software URI can share a tag projection,
  but they identify different publication surfaces and need not have the same availability.
- Bad, because it replaces the platform's verbatim local evidence with a builder-constructed value,
  hides URI capture defects, and violates the prohibition on reconstruction and substitution.
- Bad, because a syntactically valid derived release URI says nothing about whether that tag,
  release, or report ever existed publicly and adds no integrity to the image.
- Bad, because deriving the URI can falsely suggest a one-to-one tag/full-version mapping even
  though upstream drops the patch component.

### Digest-bound VM image identity

Identify the runner by a cryptographic digest over the actual VM image or disk snapshot instead of
the URI-plus-version reported pair.

- Good, because a trustworthy digest over defined image bytes would provide substantially stronger
  immutable identity and integrity than a mutable report reference.
- Good, because consumers could detect substitution of the digest-covered image representation
  rather than relying only on platform-reported metadata.
- Neutral, because the strength would depend on a precise definition of the hashed representation
  and an authoritative digest source, not merely on adding a digest field to the descriptor.
- Bad, because the current local metadata and acquisition path supply no authenticated VM image or
  disk-snapshot digest, and hashing a report or an arbitrary running filesystem would not identify
  the original image bytes.
- Bad, because obtaining such evidence would require a different acquisition and evidence mechanism
  and could change runner selection, exceeding this record-only decision.
- Bad, because adding `digest` now would either fabricate an unsupported guarantee or redefine the
  descriptor around bytes the builder does not possess.

### Control-plane deployment version as image identity

Use `ImageVersion` or the `deployment_version` annotation as the authoritative runner-image identity
instead of the on-disk URI-plus-version pair.

- Good, because the control-plane value is simple to observe and directly describes the deployment
  target reported by the Actions platform.
- Good, because it can help explain rollout state and correlate a job with the platform's incoming
  deployment, which is why it remains useful as separate evidence.
- Neutral, because deployment identity and actual on-disk image identity can coincide outside a
  rollout window, but their agreement is not a platform invariant.
- Bad, because the probe shows that the control plane reports 354 while the actual local image
  reports 353, so making the deployment value authoritative would sign the wrong image identity.
- Bad, because it would make a non-authoritative observation an identity fallback or impose an
  equality requirement that normal platform rollouts cannot satisfy.
- Bad, because selecting the available 354 report over the local 353 evidence would optimize public
  retrievability at the cost of truth about the build environment.

### No runner-image dependency

Remove the `runner-image` entry from `resolvedDependencies` rather than define the semantics of its
URI and full version.

- Good, because it eliminates report-availability ambiguity, projection validation, and the risk of
  interpreting this descriptor as image-byte integrity.
- Good, because it simplifies the closed descriptor set and avoids runner-image capture failures
  when local metadata changes shape.
- Neutral, because SLSA treats recursive dependency inspection as best effort, but removal would
  still discard evidence the project can record honestly.
- Bad, because consumers would lose the actual runner image's full reported version and report
  reference, weakening vulnerability correlation and rollout forensics.
- Bad, because it breaks the established required role, descriptor cardinality, and serialization
  contract rather than clarifying them.
- Bad, because public 404 is not a valid reason to erase well-formed local evidence, and removal
  would also discard builds whose reports remain available.

## More Information

### Probe and publication evidence

- The
  [mutation-free probe job](https://github.com/windlasstech/vers-js/actions/runs/41326580253/job/124907685958)
  captured full on-disk version `20261025.353.1`, Included Software URI
  `https://github.com/actions/runner-images/blob/ubuntu24/20261025.353/images/ubuntu/Ubuntu2404-Readme.md`,
  and control-plane deployment `ImageVersion` `20261026.354.1`. These values demonstrate why the
  local image record and deployment evidence cannot be collapsed into one identity.
- The upstream
  [`configure-image-data.sh`](https://github.com/actions/runner-images/blob/main/images/ubuntu/scripts/build/configure-image-data.sh)
  generator uses the full version's major/minor components for the Included Software and Image
  Release references and drops the patch component. For the observed values, the URI carries
  `20261025.353`, while the local `Version:` line carries `20261025.353.1`.
- At the time-bounded observation on 12026-11-11, the
  [353 Included Software report](https://github.com/actions/runner-images/blob/ubuntu24/20261025.353/images/ubuntu/Ubuntu2404-Readme.md)
  and [353 release](https://github.com/actions/runner-images/releases/tag/ubuntu24/20261025.353)
  returned HTTP 404, while the
  [354 Included Software report](https://github.com/actions/runner-images/blob/ubuntu24/20261026.354/images/ubuntu/Ubuntu2404-Readme.md)
  resolved. These observations describe availability on that date only. They do not prove that the
  353 resources never existed, that they will remain unavailable, or that the 354 resource will
  remain available.
- The
  [runner-images README's Image Releases section](https://github.com/actions/runner-images/blob/main/README.md#image-releases)
  documents the prerelease and release deployment lifecycle and the distinction between image
  deployment and publication. Related report/release availability behavior is also recorded in
  [runner-images#15197](https://github.com/actions/runner-images/issues/15197). Neither source turns
  a local Included Software URI into a permanent public-retention guarantee.
- GitHub's [Git refs documentation](https://docs.github.com/en/rest/git/refs?apiVersion=2022-11-28)
  documents the ref lookup and listing surfaces used for time-bounded tag checks. Its
  [permanent-link documentation](https://docs.github.com/en/repositories/working-with-files/using-files/getting-permanent-links-to-files)
  explains why commit-pinned source links are useful when citing moving source such as `main`. Those
  documentation practices do not change the runner's verbatim Included Software URI into an
  immutable identifier.

### Specification and community evidence

- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance) defines
  `resolvedDependencies` as build-time dependency records and gives the buildType the role of
  defining their profile-specific meaning. The reported-identity pair in this ADR is therefore a
  Windlass buildType convention, not a claim that SLSA universally assigns this meaning to every
  URI-plus-annotation pair.
- [in-toto ResourceDescriptor v1.2.0](https://github.com/in-toto/attestation/blob/v1.2.0/spec/v1/resource_descriptor.md)
  permits a descriptor with a `uri` and no `digest`, including a URI naming a mutable resource. This
  makes the selected URI-only representation permissible at the descriptor-model level, but it
  supplies neither immutable image-byte identity nor report integrity.
- The
  [SLSA issue 681 comment](https://github.com/slsa-framework/slsa/issues/681#issuecomment-1467999591)
  uses an immutable URI as the example in which a resolved dependency can stand without a digest.
  That immutable-URI example is stronger than this decision's mutable software-report convention.
  The Included Software URI does not satisfy that example's immutability assumption, and this ADR
  does not claim that it does.
- The community GitHub Actions
  [`workflow/v1` README](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/README.md)
  and
  [`example.json`](https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/example.json)
  provide precedent for buildType-specific dependency recording, including runner virtual
  environment context. They do not supply a digest for this runner image or convert a mutable report
  URI into an immutable identity.
