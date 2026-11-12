---
parent: Decisions
nav_order: 112
status: accepted
date: 12026-11-11
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0070
    scope:
      "the runner-image descriptor's image_os annotation and the associated ImageOS capture and
      Confirmation clauses: image_os is replaced by operating_system, whose value is the exact
      decoded Operating System.detail string from the producer's local metadata snapshot; runtime
      ImageOS becomes optional diagnostic-only and is never required, compared, or used as a
      fallback. The runner-image role and required resolvedDependencies membership, the
      name/uri/annotations top-level shape, the verbatim Included Software URI and exact full
      image_version as the sole reported-identity pair, descriptor cardinality and serialization
      order, no-public-availability behavior, baseline consumer independence, and every unrelated
      recording decision remain in force"
  - type: partially-supersedes
    target: ADR-0100
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
  - type: partially-supersedes
    target: ADR-0109
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
  - type: partially-supersedes
    target: ADR-0111
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

# Record Runner-Image Operating System from On-Disk Metadata

## Context and Problem Statement

Accepted ADR 0111 defines the `runner-image` descriptor's reported identity as the verbatim Included
Software URI together with the exact full on-disk `image_version`. It preserves the closed top-level
shape `name`, `uri`, and `annotations`, validates the URI/version projection offline, and keeps
public report or release availability outside signing and baseline verification.

Raw metadata preservation is unnecessary for this decision. The URI and full `image_version` already
provide the complete reported identity selected by ADR 0111, and neither adding raw metadata nor
digesting it would strengthen that buildType-defined identity. The existing offline projection also
needs no raw document. This decision therefore preserves no raw metadata and adds no raw evidence
representation.

The remaining gap is OS annotation authority. The existing `image_os` annotation records the
control-plane `ImageOS` environment value, even though the producer's local metadata snapshot
contains an `Operating System` group with its own `detail` string. The mutation-free probe records
that decoded detail as `"Ubuntu\n24.04.5\nLTS"`, while the environment label is `ubuntu24` and the
independent runner label is `ubuntu-24.04`. These are different surface forms, not interchangeable
authorities. Requiring `ImageOS` can also reject a capture when the local OS detail is present and
well formed, and retaining it as signed OS evidence leaves a second OS authority alongside the
on-disk metadata.

Only that OS source and representation gap is decided here. The descriptor's reported identity,
baseline consumer independence, other annotation meanings, and public-availability behavior remain
as ADR 0111 established them. How should the `runner-image` descriptor source and represent OS
evidence from the producer's local metadata snapshot without raw metadata preservation or a second
OS authority?

## Decision Drivers

- Use the on-disk `Operating System` detail as the sole authority for the signed OS annotation, with
  no environment-derived OS value retained alongside it.
- Preserve the exact decoded detail string, including multiline structure and meaningful whitespace,
  without creating a component grammar or compact-label mapping.
- Keep the verbatim Included Software URI and exact full `image_version` as the sole reported-
  identity pair. `operating_system` is context, not identity.
- Preserve the exact top-level descriptor shape and required four-annotation set, replacing
  `image_os` rather than adding another OS annotation.
- Preserve no raw metadata and introduce no metadata content, metadata digest, raw-evidence
  annotation, or additional evidence descriptor.
- Capture the URI, full `image_version`, and OS detail from one bounded local metadata snapshot
  before caller-controlled install, build, or pack scripts, with fail-closed local evidence handling
  and no environment or network substitution.
- Keep baseline consumers independent of the local snapshot. Their checks are limited to the signed
  descriptor shape, annotation schema and values, and the existing offline URI/version projection.
- Make `$ImageOS` optional diagnostic-only context. It is never required, never used in any
  comparison, and never a fallback.
- Preserve nullable, non-authoritative, non-identifying `deployment_version` rollout evidence and
  independent `runner_label` observation, with deployment divergence accepted.
- Preserve descriptor cardinality and serialization order, no-public-availability behavior, and ADR
  0111's allowance for a later separately governed vendor-prefixed evidence annotation without
  adopting such an annotation here.

## Considered Options

- Exact on-disk Operating System detail
- Retain environment ImageOS
- Derive compact ImageOS through a closed mapping
- Record both on-disk detail and environment ImageOS

## Decision Outcome

Chosen option: "Exact on-disk Operating System detail", because it gives the signed OS annotation
one on-disk authority, preserves the complete decoded value without a mapping contract, and removes
the unnecessary environment capture requirement while leaving ADR 0111's identity and verification
model unchanged.

### Closed descriptor and annotation contract

The `runner-image` descriptor's allowed top-level members remain exactly `name`, `uri`, and
`annotations`:

- `name` remains exactly `runner-image`.
- `uri` remains the verbatim, non-empty, platform-supplied Included Software URI from the local
  Runner Image metadata. It is not reconstructed, normalized, replaced, or rewritten.
- `annotations.image_version` remains the exact full version string from the on-disk Runner Image
  `Version:` detail, including every version component.

The verbatim `uri` and exact full `image_version` together remain the sole reported-identity pair.
`operating_system` supplies OS context only. It does not join, replace, strengthen, or fallback to
that identity pair, and it is not an assertion of VM image-byte identity or independent host-OS
proof.

The required annotation set is exactly `deployment_version`, `operating_system`, `image_version`,
and `runner_label`. `image_os` is rejected, including under a changed meaning. A conforming example
using the probe values is:

```json
{
  "name": "runner-image",
  "uri": "https://github.com/actions/runner-images/blob/ubuntu24/20261025.353/images/ubuntu/Ubuntu2404-Readme.md",
  "annotations": {
    "deployment_version": "20261026.354.1",
    "operating_system": "Ubuntu\n24.04.5\nLTS",
    "image_version": "20261025.353.1",
    "runner_label": "ubuntu-24.04"
  }
}
```

The annotations have separate meanings:

- `operating_system` is the exact decoded `Operating System` `detail` string extracted by the
  producer from the local metadata snapshot.
- `image_version` remains the exact full on-disk version and half of the reported-identity pair.
- `deployment_version` remains the control-plane `ImageVersion` environment value when supplied and
  null otherwise. It is nullable, non-authoritative, non-identifying rollout evidence. Divergence
  from `image_version` is accepted, and it is never an identity or evidence fallback.
- `runner_label` remains the independently observed Actions runner label. It is not derived from the
  OS detail, environment `ImageOS`, URI, or version, and it has no identity-fallback role.

No `content` member, base64 or other raw metadata content, metadata `digest`, `mediaType`,
`downloadLocation`, unknown top-level member, or raw-evidence annotation is permitted. No additional
runner-image, metadata-evidence, report, release, deployment, or image descriptor is introduced. The
decision carries no raw transport size check and no consumer decoded-content check.

Arbitrary additional annotations are rejected. This ADR adopts no additional evidence annotation,
but it does not close ADR 0111's existing allowance for a later separately governed vendor-prefixed
evidence annotation under its own explicit ADR and closed nested schema. That later allowance is not
a permission for arbitrary extensions or for changing the four annotations and their meanings here.

### Exact decoded Operating System detail

The producer's local metadata snapshot must contain exactly one group whose `group` value is
`Operating System`. That group must contain exactly one `detail` member, and its value must be a
string. All of the following reject capture before predicate construction:

- a missing, duplicate, or malformed `Operating System` group;
- a missing or duplicate `detail` member in that group;
- a non-string `detail` value;
- an empty string; or
- a whitespace-only string.

The retained annotation is the decoded string exactly as returned by the JSON parser. The producer
does not trim it, apply Unicode normalization, convert line endings, split or parse components,
compact it, or map it to `ImageOS`. It is not derived from the Runner Image `Image:` detail,
`runner_label`, `deployment_version`, or `$ImageOS`. Empty and whitespace-only checks are validation
predicates only. Leading or trailing whitespace in an otherwise non-whitespace value, multiline
structure, and decoded line-ending characters remain unchanged.

Decoded-value equivalence is distinct from source-token spelling. For example, the JSON escape
spellings `\n` and `\u000a` both decode to the newline character. Thus `"Ubuntu\n24.04.5\nLTS"` and
`"Ubuntu\u000a24.04.5\u000aLTS"` yield the same decoded `operating_system` value. The producer
compares decoded values, not source-token spellings. No raw metadata content or metadata digest is
retained or compared for this equivalence. Baseline consumers do not repeat the extraction or
decoded-content comparison.

For the probe evidence, the exact annotation is the three-line value represented in JSON as
`"Ubuntu\n24.04.5\nLTS"`. It is not `ubuntu24`, `ubuntu-24.04`, `Ubuntu 24.04.5 LTS`, or any other
single-line compaction.

### One bounded producer snapshot and fail-closed capture

Before any caller-controlled install, build, or pack script executes, the producer takes one bounded
logical snapshot of the local `/imagegeneration/imagedata.json` metadata. The verbatim Included
Software URI, exact full `image_version`, and exact decoded OS `detail` all come from that same
snapshot. The producer must not re-read the file for different descriptor fields, combine values
from different reads, or substitute an environment or network value for any of them. Producer-side
validation rejects a descriptor whose URI, full version, or OS annotation disagrees with that
snapshot.

Existing fail-closed local evidence behavior remains in force. An unavailable or malformed metadata
file; a missing, duplicate, or malformed Runner Image group or required detail; a missing, empty, or
malformed full `Version:` value; or a missing, empty, or malformed Included Software URI rejects
capture. The new Operating System failures are additional conjunctive requirements. No environment
value, public report, release page, HTTP response, or other network observation can rescue malformed
or missing local evidence. No raw transport representation or raw transport size check is created by
this local capture rule.

### Baseline consumer limits and environment separation

Baseline consumers receive no raw metadata snapshot and cannot independently recheck the producer's
extraction of `operating_system`. They validate only:

1. the signed descriptor's closed top-level shape;
2. the exact annotation set and each annotation's schema and value requirements, including a
   non-empty, non-whitespace string for `operating_system`; and
3. the existing offline URI/version projection from ADR 0111.

They do not compare `operating_system` with metadata, do not require raw or decoded metadata
content, and do not perform a raw transport size check. The signature authenticates the statement
containing the producer's assertion. It does not make the OS annotation independently
consumer-verifiable against the local metadata, prove that the metadata is truthful about the host,
or authenticate the VM image bits.

`$ImageOS` is optional diagnostic-only context when available. It is not required for capture, is
not moved into another signed field, is never used in any comparison, and is never a fallback for
missing or malformed on-disk evidence. Its absence cannot invalidate an otherwise conforming
capture, and its presence cannot rescue an invalid one.

No DNS, HTTP, tag, release, or report lookup is required for predicate construction, signing, or
baseline verification. HTTP 404 does not omit the `runner-image` descriptor, emit a warning, reject
otherwise valid local evidence, or substitute another identity. In particular, an available
deployment report or the `deployment_version` value cannot replace the captured URI or full
`image_version`. A public success response is likewise not an identity or integrity proof.

### Preserved descriptor set and compatibility

Exactly one `runner-image` descriptor remains mandatory. Overall descriptor cardinality remains six
for npm selections and seven for pnpm or Yarn selections. Producer serialization remains
`invocation-source`, `built-source`, `package-manifest`, `lockfile`, `node-runtime`, the conditional
`package-manager-distribution`, then `runner-image`. Semantic verification remains unordered and
name-keyed, and npm's index-0 `invocation-source` emission rule is unchanged. No other descriptor
changes shape, and no additional descriptor is introduced for OS or metadata evidence.

Consistent with the existing pre-release policy, there is no `buildType` version migration and no
old-bundle compatibility path. Statements carrying `image_os`, omitting `operating_system`, or
otherwise violating this closed contract are non-conforming under the revised contract.

### Consequences

- Good, because the signed OS annotation has one on-disk authority and retains the complete decoded
  detail rather than an environment label or lossy compact mapping.
- Good, because a valid local OS detail is sufficient when `$ImageOS` is absent. An environment
  omission no longer rejects complete local evidence, and no comparison between different OS surface
  forms is needed.
- Good, because the URI plus exact full `image_version` remains the sole reported-identity pair. OS
  context cannot silently become an identity component or fallback.
- Good, because no raw metadata, metadata digest, raw transport, or additional evidence descriptor
  is added. Statement size and baseline consumer responsibilities do not grow to carry evidence that
  is unnecessary for ADR 0111's reported identity.
- Good, because one bounded producer snapshot keeps URI, version, and OS extraction consistent and
  occurs before caller-controlled scripts, while existing local evidence failures remain fail
  closed.
- Good, because deployment divergence, independent runner-label observation, descriptor cardinality
  and order, offline URI/version projection, and no-public-availability behavior remain intact.
- Neutral, because different JSON escape spellings can decode to the same OS annotation. This is
  decoded-value equivalence only, with no raw content or digest comparison.
- Neutral, because baseline consumers authenticate and validate the signed assertion but cannot
  independently verify its extraction against the producer's local metadata. That limitation is
  explicit rather than hidden behind a content-binding claim.
- Bad, because multiline annotations require fixtures, displays, and policy tooling to preserve
  embedded newlines and meaningful whitespace without implicit trimming or normalization.
- Bad, because captures with a usable environment `ImageOS` now fail when the local Operating System
  group or detail is missing, duplicate, malformed, non-string, empty, or whitespace-only. These are
  intentional local evidence requirements.
- Bad, because a baseline consumer cannot detect a producer that extracted the wrong OS detail from
  an otherwise unavailable local snapshot. Its assurance rests on the trusted producer contract and
  signature, not independent metadata verification.

### Confirmation

After explicit acceptance, architecture review, conformance fixtures, and test-first implementation
must demonstrate the following cases without adding raw metadata preservation or a runner-image
network dependency.

Accepted cases:

- The probe-shaped snapshot with decoded `Operating System` detail `"Ubuntu\n24.04.5\nLTS"` produces
  exactly that multiline `operating_system` annotation. The descriptor has exactly the required four
  annotations `deployment_version`, `operating_system`, `image_version`, and `runner_label`, and its
  top level has exactly `name`, `uri`, and `annotations`.
- The valid descriptor contains no `content`, metadata `digest`, `mediaType`, `downloadLocation`,
  raw-evidence annotation, or additional metadata descriptor. No raw transport size check or
  consumer decoded-content check occurs.
- The Included Software URI, exact full `image_version`, and decoded OS detail come from one bounded
  snapshot before caller-controlled install, build, and pack scripts. Same-snapshot consistency
  checks pass without environment or network substitution.
- Source strings using `\n` and `\u000a` for newlines produce the same decoded `operating_system`
  value. The test compares decoded values only and neither retains nor compares raw metadata content
  or metadata digests.
- A capture with no `$ImageOS` environment value is accepted when the local OS evidence and all
  other required inputs are valid. When `$ImageOS` is present, it appears only as diagnostic context
  and is never used in any comparison or as a fallback.
- A rollout fixture with `deployment_version: "20261026.354.1"` and
  `image_version: "20261025.353.1"` is accepted. `deployment_version` remains nullable,
  non-authoritative, non-identifying evidence, and `runner_label` remains independent.
- The probe URI and full version remain accepted when the corresponding report and release return
  HTTP 404. The result does not omit the descriptor, warn, reject, or substitute the deployment
  identity or another public report.
- Baseline verification succeeds without a raw metadata snapshot by checking only signed descriptor
  shape, annotation schema and values, and the offline URI/version projection. The test and consumer
  documentation explicitly recognize that the consumer cannot independently verify OS extraction
  against metadata.
- npm and pnpm/Yarn fixtures retain six and seven descriptors respectively, exactly one
  `runner-image`, the established producer serialization order, npm's index-0 `invocation-source`,
  and unordered name-keyed semantic verification. Every non-runner-image descriptor passes its
  existing conformance cases unchanged.

Rejected cases:

- A missing, duplicate, or malformed `Operating System` group is rejected. A missing or duplicate
  `detail`, non-string detail, empty string, or whitespace-only string is also rejected, even when
  `$ImageOS` is present.
- An `operating_system` value produced by trimming, Unicode normalization, line-ending conversion,
  component parsing, compaction, or mapping to `ImageOS` is rejected at producer extraction. A value
  taken from `$ImageOS`, `runner_label`, `deployment_version`, or the Runner Image `Image:` detail
  is likewise non-conforming.
- Mixed reads, re-reading the metadata for a different URI, version, or OS value, and environment or
  network substitution are rejected. Missing or malformed Runner Image evidence and Included
  Software URI evidence retain their existing fail-closed behavior.
- The presence of `image_os`, absence of `operating_system`, or an arbitrary fifth annotation is
  rejected. The arbitrary-fifth-annotation test does not close ADR 0111's allowance for a later
  separately governed vendor-prefixed evidence annotation; this ADR simply adopts no such
  annotation.
- A `content` member, base64 or other raw metadata content, metadata `digest`, `mediaType`,
  `downloadLocation`, unknown top-level member, raw-evidence annotation, or any additional
  descriptor for this evidence is rejected. Consumer requirements for raw snapshots, decoded-content
  checks, or raw transport size checks are non-conforming.
- Any producer requirement for `$ImageOS`, any producer or consumer comparison involving `$ImageOS`,
  and any use of `$ImageOS` as a fallback are non-conforming. Requiring
  `deployment_version == image_version`, deriving `runner_label` from another annotation, or using
  either annotation as an identity fallback is also non-conforming.
- Any consumer claim to have independently verified `operating_system` against metadata without the
  producer's local snapshot is non-conforming. Baseline consumers perform no such extraction check
  and do not treat the annotation as proof of host-OS truth or VM image integrity.
- Any path that treats HTTP 404 as a reason to omit the descriptor, warn, reject valid local
  evidence, or substitute another identity is non-conforming. A public response cannot rescue
  invalid local evidence.
- Duplicate or unknown descriptors, additional metadata descriptors, wrong six/seven cardinalities,
  producer order changes, loss of npm's index-0 `invocation-source`, and any `buildType` migration
  or old-bundle compatibility path remain rejected at their respective layers.

Review must also confirm that no normalization is hidden in OS extraction, no diagnostic requires or
compares `$ImageOS`, no public availability gate is introduced, and no text describes
`operating_system` as content-bound or independently consumer-verifiable against metadata.

## Pros and Cons of the Options

### Exact on-disk Operating System detail

Replace `image_os` with `operating_system`, carrying the exact decoded `Operating System` `detail`
string from the producer's bounded local metadata snapshot.

- Good, because the signed OS record has one on-disk authority and one exact representation, with no
  second environment-backed OS value or compact mapping authority.
- Good, because multiline structure and meaningful whitespace survive without trimming, Unicode
  normalization, line-ending conversion, or component parsing.
- Good, because `$ImageOS` leaves signed evidence and capture requirements. Its absence cannot fail
  an otherwise valid local capture, and it remains available only as optional diagnostic context.
- Good, because the URI plus full `image_version` remains the sole reported-identity pair, while no
  raw metadata, metadata digest, raw transport, or additional descriptor is needed for the OS
  record.
- Neutral, because JSON escape-equivalent strings produce the same decoded annotation without any
  raw content or digest comparison.
- Bad, because exact multiline values are less convenient for displays and require disciplined
  whitespace handling.
- Bad, because baseline consumers cannot independently recheck extraction without the local
  snapshot, and strict local OS validation adds intentional capture failures.

### Retain environment ImageOS

Keep `image_os` sourced from `$ImageOS`, with the on-disk metadata supplying only the URI and full
`image_version`.

- Good, because the annotation remains compact and the producer needs no Operating System group or
  decoded-detail validation.
- Good, because existing environment-oriented consumers require no migration to a new annotation
  name or multiline value semantics.
- Bad, because signed OS evidence retains a second authority instead of using the OS detail already
  present in the local metadata snapshot.
- Bad, because `$ImageOS` remains required capture evidence, so its absence can reject a build with
  complete and well-formed on-disk OS evidence.
- Bad, because environment and on-disk labels use different surface forms. There is no lossless
  exact comparison between them, and preserving both authorities would invite unsupported equality
  or precedence rules.

### Derive compact ImageOS through a closed mapping

Decode the Operating System detail, parse its components, and map them to a compact ImageOS-style
value under a closed mapping before recording the annotation.

- Good, because consumers receive a short, uniform label rather than a multiline source string.
- Good, because the source input comes from the local metadata and a deterministic mapping could
  make the derivation reproducible at the producer.
- Neutral, because a closed mapping can be specified and tested, but it remains a transformation
  contract with its own versioning and unknown-input behavior.
- Bad, because the annotation is no longer the exact decoded Operating System `detail` string.
  Component parsing and compact mapping discard multiline structure, whitespace, and unselected
  source information.
- Bad, because the mapping becomes another derivation authority that can drift from the source
  parser and must define behavior for future metadata generator forms.
- Bad, because strict source-group validation is still necessary, so the mapping adds complexity and
  information loss without removing the local evidence failure surface.

### Record both on-disk detail and environment ImageOS

Add the exact decoded Operating System detail while retaining the environment `image_os` annotation
alongside it.

- Good, because consumers would receive both the complete source string and a compact environment
  label for display or diagnostics.
- Neutral, because the values could sometimes corroborate one another, although their different
  producers and surface forms have no general exact equality rule.
- Bad, because retaining both creates an additional annotation and preserves two signed OS
  authorities instead of selecting the on-disk detail as the sole OS source.
- Bad, because two signed OS representations invite equality, precedence, or fallback rules that the
  platform's different surface forms cannot support.
- Bad, because the environment value would either remain a capture requirement or need a new
  optional signed-field contract. Both alternatives add schema and verification complexity without
  improving the exact on-disk OS record.

## More Information

Decision baseline and evidence:

- [Accepted ADR 0111](0111-record-runner-image-reported-identity-with-included-software-uri-and-full-image-version.md):
  the verbatim Included Software URI plus exact full `image_version` as reported identity, the
  `name`/`uri`/`annotations` shape, offline URI/version projection, no-public-availability behavior,
  baseline consumer independence, and the separately governed future evidence-annotation allowance.
- The
  [mutation-free probe job](https://github.com/windlasstech/vers-js/actions/runs/41326580253/job/124907685958)
  records the `Operating System` evidence whose decoded `detail` value is represented in JSON as
  `"Ubuntu\n24.04.5\nLTS"`, the local full version `20261025.353.1`, the corresponding Included
  Software URI, and the distinct control-plane deployment version `20261026.354.1`.
- [SLSA v1.2 Build Provenance](https://slsa.dev/spec/v1.2/build-provenance) defines
  `resolvedDependencies` as build-time dependency records whose profile-specific meaning is defined
  by the buildType. The separation between the URI/version reported-identity pair and
  non-identifying OS context is therefore a Windlass buildType rule, not a universal SLSA identity
  assignment.
