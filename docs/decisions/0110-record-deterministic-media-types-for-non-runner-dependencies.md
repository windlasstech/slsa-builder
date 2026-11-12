---
parent: Decisions
nav_order: 110
status: accepted
date: 12026-11-10
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0109
    scope:
      "the Decision Driver and Preserved contracts clauses deferring mediaType for lockfile,
      node-runtime, and package-manager-distribution: those concrete artifact descriptors now
      require deterministic mediaType values selected from their actual digest-covered
      representations, while package-manifest retains application/json and invocation-source and
      built-source remain without mediaType. Identity, URI, digest, acquisition, annotations,
      descriptor cardinality and serialization order, the package-manifest no-content rule, and
      every runner-image decision remain in force"
  - type: see-also
    target: ADR-0105
  - type: see-also
    target: ADR-0106
  - type: see-also
    target: ADR-0088
  - type: see-also
    target: ADR-0099
---

# Record Deterministic Media Types for Non-Runner Dependencies

## Context and Problem Statement

ADR 0109 gave `package-manifest` an explicit `mediaType` of `application/json` but deferred media
typing for the other non-runner dependency descriptors. The remaining concrete artifact descriptors
therefore require consumers to infer representation from a URI suffix, a lockfile name, a package
manager, or out-of-band acquisition knowledge. That inference is especially unreliable for
compressed archives versus extracted content and for a package-manager distribution whose actual
digest-covered representation can be a registry tarball or the current single `yarn.js` bundle.

The existing contracts already pin the identities and digest targets needed for a deterministic
choice. ADR 0099 records the exact compressed Node.js `.tar.gz` bytes covered by the provisioning
digest. ADR 0088 verifies npm registry `.tgz` bytes before extraction and defines pnpm dual-artifact
semantics without making every package-manager representation identical. ADR 0105 and ADR 0106
record invocation and built sources as Git commit or tree identities, not as one serialized source
archive. ADR 0056 distinguishes the selected lockfile from stale non-selected diagnostics, while ADR
0085 records npm 12's removal of `npm-shrinkwrap.json` support.

The sole decision axis is representation typing for these existing non-runner dependencies. It does
not change descriptor identity, URI or digest targets, acquisition, annotations, descriptor
structure or cardinality, runner-image evidence, or verification network behavior. Baseline in-toto
ResourceDescriptor validity does not require `mediaType`; a deterministic closed rule is a Windlass
buildType choice, not a claim about the baseline specification.

Which existing non-runner dependency descriptors should require deterministic `mediaType` values,
which descriptors must forbid the field, and how should each value describe the actual
digest-covered representation without introducing generic fallbacks, new locators, or acquisition
changes?

## Decision Drivers

- Describe the exact artifact representation identified and digest-covered by each existing concrete
  artifact descriptor, rather than typing by filename suffix alone, package-manager name, or a
  remote server's response header.
- Require `mediaType` deterministically for `package-manifest`, `lockfile`, `node-runtime`, and the
  conditional `package-manager-distribution`. Forbid it for `invocation-source` and `built-source`,
  whose Git commit or tree identity is not one serialized artifact representation.
- Keep a closed matrix with no generic fallback and no producer-optional omission. An unknown future
  representation requires a follow-up decision before that representation is supported.
- Apply lockfile typing only to the selected lockfile. Preserve the three selected lockfile names
  `package-lock.json`, `pnpm-lock.yaml`, and `yarn.lock`, limit Yarn YAML typing to supported Berry
  v4/v5, and add no `npm-shrinkwrap.json` support or stale-diagnostic descriptor typing.
- Preserve ADR 0088's registry-tarball verification and pnpm dual-artifact semantics without
  deciding or changing descriptor structure or cardinality. Type every represented registry `.tgz`
  as `application/gzip`, and distinguish the exact current single `yarn.js` bytes as
  `text/javascript`.
- Preserve identity, URI, digest targets, acquisition, annotations, six/seven descriptor counts,
  deterministic serialization order, name-keyed verification, the `package-manifest` no-content
  rule, and every runner-image decision.
- Add no `downloadLocation`. Commit-pinned raw manifest and lockfile URLs remain deferred until a
  real consumer or evidence-collector requirement exists, with exact-byte/digest verification and no
  verifier network fallback as prerequisites for any later decision.
- Treat `mediaType` as descriptive metadata, not as a trust signal, parser authorization, or remote
  Content-Type check. Existing digest verification, source identity, bounded parsing, and
  fail-closed acquisition remain the security controls.

## Considered Options

- Deterministic media types for concrete artifact descriptors
- Producer-optional media types
- MediaType on every descriptor including sources
- Deterministic media types plus downloadLocation for manifest/lockfile
- No new media types

## Decision Outcome

Chosen option: "Deterministic media types for concrete artifact descriptors", because the current
contracts already determine every supported representation, consumers benefit from one closed typing
rule, and source descriptors have a fundamentally different identity model that should not be forced
into a serialized-artifact media type. The decision adds no acquisition path, locator, digest
target, or runner-image change.

### Deterministic descriptor matrix

`mediaType` is required on `package-manifest`, `lockfile`, `node-runtime`, and the conditional
`package-manager-distribution`, selected exactly as follows. It is forbidden on `invocation-source`
and `built-source`.

| Descriptor                     | Actual digest-covered representation                                    | `mediaType`        | Closed rule                                                                                           |
| ------------------------------ | ----------------------------------------------------------------------- | ------------------ | ----------------------------------------------------------------------------------------------------- |
| `package-manifest`             | Selected `package.json` source bytes                                    | `application/json` | Required; retains ADR 0109's value and no-content rule                                                |
| `lockfile`                     | Selected `package-lock.json` bytes                                      | `application/json` | Required for the selected npm lockfile only                                                           |
| `lockfile`                     | Selected `pnpm-lock.yaml` bytes                                         | `application/yaml` | Required for the selected pnpm lockfile only                                                          |
| `lockfile`                     | Selected supported Yarn Berry v4/v5 `yarn.lock` bytes                   | `application/yaml` | Required for the selected Berry lockfile only                                                         |
| `node-runtime`                 | Exact compressed Node.js `.tar.gz` bytes covered by the existing digest | `application/gzip` | Required; never describes the extracted runtime tree                                                  |
| `package-manager-distribution` | Exact npm registry `.tgz` bytes                                         | `application/gzip` | Required for every represented registry tarball, including any represented pnpm dual-artifact tarball |
| `package-manager-distribution` | Exact current single `yarn.js` bytes                                    | `text/javascript`  | Required only when this is the actual acquired and digest-covered representation                      |
| `invocation-source`            | Git commit or tree identity, not one serialized artifact                | none               | `mediaType` is forbidden                                                                              |
| `built-source`                 | Git commit or tree identity, not one serialized artifact                | none               | `mediaType` is forbidden                                                                              |

For the concrete artifact rows, the matrix is exhaustive. There is no generic media type, no
fallback such as `application/octet-stream` or `text/plain`, and no producer option to omit a
required value. A future artifact representation not enumerated here requires a follow-up decision
that extends the closed matrix deliberately.

Selection uses the existing validated artifact and digest-covered representation, not a suffix-only
lookup. Lockfile rows are selected through the existing package-manager selection, lockfile parsing,
and digest contract, not by scanning repository filenames. Distribution rows are selected from the
actual acquired representation covered by the descriptor's digest, never from the manager name
alone.

### Lockfile evidence and support boundaries

The Yarn Berry v4/v5 `application/yaml` assignment is grounded in Yarn's own format evidence. The
official Yarn 2.0 release notes describe the new lockfile format as pure YAML, and the pinned Berry
parser source shows `parseSyml` delegating to `js-yaml`. The IANA `application/yaml` registration
and RFC 9512 provide the corresponding media-type vocabulary.

This decision applies that evidence only to the supported Yarn Berry v4/v5 `yarn.lock`. It does not
call Yarn Classic `yarn.lock` YAML, type Yarn Classic, or expand Yarn support. Similarly,
`application/json` for `package-lock.json` and `application/yaml` for `pnpm-lock.yaml` describe the
selected parsed artifact bytes, not arbitrary stale files sharing those names.

The selected lockfile names remain exactly `package-lock.json`, `pnpm-lock.yaml`, and `yarn.lock`.
No `npm-shrinkwrap.json` support is added, consistent with ADR 0085's record that npm 12 drops
shrinkwrap support. Non-selected supported lockfiles remain stale diagnostics under ADR 0056; they
do not become selected dependencies, acquire descriptors, or receive media-type assignments from
this decision.

### Distribution representation, not manager-name typing

`application/gzip` describes compressed bytes, not an extracted archive tree. For `node-runtime`, it
covers the exact `.tar.gz` representation whose existing SHA-256 is verified before extraction. For
a `package-manager-distribution` entry representing npm registry `.tgz` bytes, it covers those exact
compressed registry bytes and their existing digest target.

ADR 0088's pnpm dual-artifact semantics are preserved. This ADR does not decide whether a future
contract uses one descriptor, multiple descriptors, or another already specified dual-artifact
record, and it changes no descriptor structure or cardinality. Wherever a represented registry
`.tgz` appears in the conditional package-manager distribution record, its media type is
`application/gzip`.

`text/javascript` applies only to the exact current single `yarn.js` representation when those
bytes, rather than a containing `.tgz`, are what the descriptor acquires or records and covers with
its digest. An `@yarnpkg/cli-dist` registry tarball remains `application/gzip`; extracting `yarn.js`
does not retroactively change the tarball descriptor's type. Conversely, a descriptor over the
single `yarn.js` bytes must not inherit `application/gzip` merely because Yarn was the selected
manager or because the bundle arrived inside an archive before extraction.

### Source exclusion and descriptive-only semantics

`invocation-source` and `built-source` continue to identify Git commits or trees through their
existing `uri`, `digest.gitCommit`, role names, and, for `built-source`, `annotations.ref_type`.
Those identities do not select one serialized artifact representation such as a tarball, zip
archive, or JSON document. Both descriptors therefore forbid `mediaType`; adding it would
misdescribe their identity rather than improve it.

For every descriptor, `mediaType` is descriptive only. It does not authenticate bytes, authorize a
parser, replace digest or source verification, establish that content is safe, or permit execution.
Consumers do not fetch a remote Content-Type header to select, validate, or override the value, and
producers do not infer it from such a header. Existing exact-byte digest checks and bounded,
fail-closed parsers remain mandatory. Baseline in-toto does not require the field, while this closed
Windlass contract requires the matrix above.

### Locator, cardinality, and preserved-contract boundaries

No descriptor gains `downloadLocation` in this decision. Existing `uri` values and their identity
semantics remain unchanged. Commit-pinned raw manifest and lockfile URLs could help a future
evidence collector retrieve exact source bytes, but no current consumer or evidence-collector
requirement justifies adding those locators and their distribution semantics now. Any later proposal
must bind retrieved bytes to the existing exact-byte digest and source identity and must not create
a verifier network fallback.

The descriptor counts remain six for npm selections and seven for pnpm or Yarn selections, with
exactly one conditional `package-manager-distribution` role under the existing contract. Producer
serialization remains `invocation-source`, `built-source`, `package-manifest`, `lockfile`,
`node-runtime`, the conditional `package-manager-distribution`, then `runner-image`. Semantic
verification remains unordered and name-keyed, with duplicate or unknown names rejected, and npm's
index-0 `invocation-source` emission rule remains unchanged.

`package-manifest` retains its existing source-bound URI, digest, `workspace_root` annotation, and
no-content rule. No descriptor gains `content`, and no identity, URI, digest target, acquisition
mechanism, annotation, or verification expectation changes. Every runner-image decision and the
complete `runner-image` descriptor remain outside this decision and unchanged.

### Consequences

- Good, because consumers can determine the representation of every supported concrete non-runner
  artifact from a closed signed rule rather than inferring it from filenames, manager names, or
  acquisition implementation details.
- Good, because compressed `.tar.gz` and `.tgz` bytes are distinguished from extracted trees and
  from the single `yarn.js` representation, keeping each media type aligned with the existing digest
  target.
- Good, because source descriptors avoid a false serialized-artifact type, while stale lockfiles,
  shrinkwrap, Yarn Classic, unknown future representations, and remote Content-Type headers cannot
  silently expand the matrix.
- Good, because no URI, identity, digest, acquisition, annotation, descriptor cardinality, ordering,
  runner-image, or network behavior changes. The decision is limited to descriptive representation
  metadata.
- Neutral, because baseline in-toto permits descriptors without `mediaType`, while the Windlass
  closed contract deliberately makes the field mandatory for the concrete artifact roles and
  forbidden for sources.
- Neutral, because `downloadLocation` remains absent. A future evidence collector could benefit from
  commit-pinned raw manifest or lockfile URLs, but there is no current consumer requirement to
  justify their added contract surface.
- Bad, because producers, consumers, exact-byte fixtures, and verification policy must migrate to
  the new required and forbidden field matrix even though no artifact bytes or acquisition behavior
  change.
- Bad, because a future representation cannot enter the supported contract through a generic type or
  producer-local choice. It requires a follow-up decision and corresponding conformance cases before
  support.

### Confirmation

After explicit acceptance, the architecture specifications, test-first implementation, exact-byte
fixtures, and verification policy must demonstrate the following cases.

Accepted cases:

- `package-manifest` retains `mediaType: application/json`, its source-bound identity and digest,
  `workspace_root`, and no `content`.
- A selected `package-lock.json` lockfile carries `application/json`; a selected `pnpm-lock.yaml`
  carries `application/yaml`; and a selected supported Yarn Berry v4/v5 `yarn.lock` carries
  `application/yaml`. Each value is checked against the selected parsed artifact representation and
  its existing digest.
- `node-runtime` carries `application/gzip` for the exact compressed `.tar.gz` bytes covered by its
  existing digest. The digest continues to authenticate those compressed bytes rather than the
  extracted runtime tree.
- A `package-manager-distribution` record representing exact npm registry `.tgz` bytes carries
  `application/gzip`. This applies to every represented registry tarball under ADR 0088, including
  pnpm dual-artifact tarballs, without changing the record's structure or cardinality.
- A `package-manager-distribution` record representing the exact current single `yarn.js` bytes
  carries `text/javascript` only when those bytes are its actual acquired and digest-covered
  representation. A registry `.tgz` and a single `yarn.js` record are accepted under their
  respective representations without inferring either value from the manager name.
- `invocation-source` and `built-source` are accepted without `mediaType`, with their existing Git
  identity, URI, `gitCommit` digest, role names, and `built-source.annotations.ref_type` unchanged.
- All accepted descriptors omit `downloadLocation`. Existing `uri` values, exact-byte and digest
  verification, descriptor counts of six and seven, serialization order, name-keyed verification,
  package-manifest no-content behavior, and runner-image fixtures remain unchanged.
- Stale non-selected lockfiles remain accepted only under ADR 0056's existing diagnostic behavior,
  without becoming selected descriptors or acquiring media-type rules from this decision.

Rejected cases:

- A missing or incorrect `mediaType` on `package-manifest`, the selected `lockfile`, `node-runtime`,
  or a present `package-manager-distribution` is rejected. Producer-optional omission is not
  conforming.
- Any `mediaType` on `invocation-source` or `built-source` is rejected, including a value claiming a
  Git commit or tree is one serialized tarball, archive, JSON document, or generic byte stream.
- A media type that disagrees with the actual digest-covered representation is rejected. Examples
  include a Node.js extracted tree labeled `application/gzip`, registry `.tgz` bytes labeled
  `text/javascript`, single `yarn.js` bytes labeled `application/gzip`, `package-lock.json` labeled
  `application/yaml`, or Berry `yarn.lock` labeled `application/json`.
- Selection logic based only on a filename suffix, package-manager name, or remote Content-Type
  header is non-conforming, even when it happens to produce the expected value. Verification may not
  fetch a header or artifact to repair or infer the media type, and no verifier network fallback is
  allowed.
- `npm-shrinkwrap.json` as a selected lockfile, descriptor, or newly supported lockfile name is
  rejected. Typing Yarn Classic `yarn.lock` as YAML or otherwise expanding beyond supported Berry
  v4/v5 is rejected. Assigning this decision's media types to stale non-selected diagnostics or
  adding descriptors for them is also rejected.
- Any `downloadLocation` on these non-runner descriptors is rejected under the revised closed
  shapes. Commit-pinned raw manifest or lockfile URLs may not be introduced through implementation
  without a separate consumer-motivated decision.
- A generic or unknown fallback, including `application/octet-stream`, `text/plain`, or a
  producer-selected value for an unenumerated representation, is rejected. A future representation
  requires an explicit follow-up decision before support.
- Any change to identity, URI, digest targets, acquisition, annotations, descriptor structure or
  six/seven cardinalities, serialization order, package-manifest no-content behavior, ADR 0088 pnpm
  dual-artifact semantics, or runner-image behavior is outside this decision and rejected as
  non-conforming scope expansion.

Review must also confirm that no consumer treats `mediaType` as a trust signal or parser
authorization, no baseline in-toto requirement is misstated, and no verifier fetches a remote
Content-Type or substitutes network evidence for exact-byte and digest verification.

## Pros and Cons of the Options

### Deterministic media types for concrete artifact descriptors

Require the closed matrix for concrete artifact descriptors and forbid `mediaType` for Git source
identities.

- Good, because every supported value is determined by the existing validated representation and
  digest target, giving producers and consumers the same complete conformance rule.
- Good, because the matrix distinguishes compressed archives, extracted content, selected lockfile
  formats, the current single `yarn.js`, and non-serialized Git identity without inventing new
  acquisition or descriptor structure.
- Good, because unknown representations fail closed for a later decision rather than acquiring a
  misleading generic type or producer-local meaning.
- Neutral, because the required matrix exceeds baseline in-toto validity requirements, but it does
  so as explicit Windlass closed-contract policy rather than misrepresenting a standard obligation.
- Bad, because the schema and fixtures must migrate across several descriptors, and future
  representation changes require deliberate matrix maintenance.

### Producer-optional media types

Allow producers to include or omit `mediaType` on non-runner descriptors, using the deterministic
values only as recommendations.

- Good, because existing producers could adopt typing incrementally and unknown representations
  would not immediately require a closed-matrix decision.
- Neutral, because optional presence could provide useful hints during a transition, but consumers
  would still need every existing inference path for omitted values.
- Bad, because the same artifact could validly carry a type or omit it, preventing deterministic
  consumer behavior and making omission semantics unclear.
- Bad, because optional values weaken conformance coverage and invite generic fallbacks or
  producer-specific types instead of solving representation ambiguity.

### MediaType on every descriptor including sources

Require deterministic media types on concrete artifacts and also assign a media type to
`invocation-source` and `built-source` for uniform field presence.

- Good, because a uniform member list could make superficial descriptor traversal appear simpler.
- Neutral, because consumers could special-case source values, but they would still need to
  understand that those values do not describe the same kind of resource as the concrete artifact
  rows.
- Bad, because a Git commit or tree identity is not one serialized artifact representation. Any
  assigned media type would falsely imply a selected tarball, archive, document, or byte encoding.
- Bad, because source descriptor shapes would change without improving their existing `uri`,
  `gitCommit`, role, or annotation semantics, conflating identity with representation.

### Deterministic media types plus downloadLocation for manifest/lockfile

Adopt the deterministic matrix and add commit-pinned raw URLs as `downloadLocation` for
`package-manifest` and the selected `lockfile`.

- Good, because exact raw URLs could help future consumers or evidence collectors retrieve the
  selected manifest and lockfile bytes at the built commit without reconstructing repository paths
  independently.
- Good, because a later design could require those retrieved bytes to match the existing exact-byte
  digest and source identity, making the locator useful without weakening authentication.
- Neutral, because the utility is real for evidence collection, but there is no current consumer or
  evidence-collector requirement that needs these locators now.
- Bad, because adding `downloadLocation` introduces a second locator family and retrieval semantics,
  duplicate path evidence, availability questions, and a potential verifier network fallback before
  any consumer justifies them.
- Bad, because offline verification would still need an exact-byte delivery contract. A URL alone
  neither supplies the bytes nor guarantees future retrieval, so the added surface does not solve
  the current representation-typing problem.

### No new media types

Retain `application/json` only on `package-manifest` and leave all other descriptor shapes
unchanged.

- Good, because there is no new schema migration, fixture matrix, or future media-type maintenance
  burden, and the descriptors remain valid under baseline in-toto rules.
- Good, because all existing identity, digest, acquisition, and verification behavior remains
  untouched, including source descriptors and package-manager dual-artifact semantics.
- Bad, because consumers must continue inferring representations from filenames, manager names, URI
  suffixes, or private acquisition knowledge, with compressed and extracted representations
  especially easy to confuse.
- Bad, because the contract forfeits deterministic typing even though every current representation
  is already known and digest-bound, leaving a solvable ambiguity in the signed record.

## More Information

Decision and evidence references:

- [ADR 0056](0056-treat-non-selected-lockfiles-as-stale-diagnostics.md): the selected-lockfile
  boundary and stale non-selected diagnostic behavior.
- [ADR 0085](0085-pin-the-node-24-patch-version-and-assert-the-expected-bundled-npm-pair.md): the
  toolchain review recording npm 12's removal of `npm-shrinkwrap.json` support.
- [ADR 0088](0088-provision-build-stage-pnpm-and-yarn-from-digest-verified-registry-tarballs.md):
  compressed registry `.tgz` verification, the `@yarnpkg/cli-dist` and `yarn.js` acquisition
  relationship, and pnpm dual-artifact semantics.
- [ADR 0099](0099-record-the-digest-verified-nodejs-runtime-in-resolveddependencies.md): the exact
  Node.js release tarball descriptor and its existing digest target.
- [ADR 0105](0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md): Git
  source identity, source descriptor shapes, cardinality, serialization order, and name-keyed
  verification.
- [ADR 0106](0106-consolidate-external-source-parameters-onto-workflow-and-descriptor-bindings.md):
  consolidated source bindings and the `built-source.annotations.ref_type` shape.
- [ADR 0109](0109-minimize-external-parameters-to-actual-interface-inputs.md): `package-manifest`
  media type and no-content rule, six/seven descriptor cardinalities, serialization order, and the
  prior deferral of other non-runner media types.
- [Yarn 2.0 release notes, New lockfile format](https://yarnpkg.com/blog/release/2.0#new-lockfile-format):
  the official description of the Berry lockfile format as pure YAML.
- [Pinned Berry `syml.ts` parser source](https://github.com/yarnpkg/berry/blob/b6921541fd8bf13b0531ddfd7e811d8b99bf81a8/packages/yarnpkg-parsers/sources/syml.ts):
  the `parseSyml` implementation delegating to `js-yaml`.
- [IANA `application/yaml` registration](https://www.iana.org/assignments/media-types/application/yaml)
  and [RFC 9512](https://www.rfc-editor.org/rfc/rfc9512): the YAML media-type definition and its
  fragment and interoperability considerations.
- [in-toto ResourceDescriptor v1.2.0](https://github.com/in-toto/attestation/blob/v1.2.0/spec/v1/resource_descriptor.md):
  the optional `mediaType` and `downloadLocation` field vocabulary. The deterministic requirements
  selected here are Windlass policy, not baseline in-toto obligations.
- Investigated but undecided:
  [ResourceDescriptor `downloadLocation`](https://github.com/in-toto/attestation/blob/v1.2.0/spec/v1/resource_descriptor.md)
  for commit-pinned raw manifest and lockfile URLs. Such URLs could serve a future evidence
  collector, but no current consumer requirement justifies them. Any later adoption requires
  exact-byte/digest verification and no verifier network fallback.
