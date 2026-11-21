---
parent: Decisions
nav_order: 114
status: accepted
date: 12026-11-15
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0071
    scope:
      "the closed 'exactly one signing-adapter descriptor' rule for builderDependencies; the field
      classification rationale remains in force, and the signing-adapter descriptor's shape is
      governed by ADR 0113"
  - type: partially-supersedes
    target: ADR-0077
    scope:
      "the clause that every producer's builderDependencies contains exactly one descriptor; the
      sigstore-go signing-adapter selection, URI, digest values, and validation remain in force,
      with the descriptor's member shape governed by ADR 0113"
  - type: partially-supersedes
    target: ADR-0082
    scope:
      "the recording-location clause placing the publish npm version only in outcome evidence; the
      pinning, integrity-verified provisioning, reviewed allowlist, and bump-procedure clauses
      remain in force"
  - type: partially-supersedes
    target: ADR-0084
    scope:
      "the clause that the publish npm version 'is not merged into the build-time record', insofar
      as it excluded any signed-provenance record; the digest-verified registry tarball provisioning
      mechanism and the committed SHA-512 pin remain in force"
  - type: partially-supersedes
    target: ADR-0109
    scope:
      "the clause leaving publish-stage npm recording to outcome evidence only; the
      builder.version.npm build-stage-only clause and the externalParameters minimization remain in
      force"
  - type: see-also
    target: ADR-0098
  - type: see-also
    target: ADR-0099
  - type: see-also
    target: ADR-0104
  - type: partially-superseded-by
    target: ADR-0115
    scope:
      "the consumer expectation source for the publish-cli descriptor for producer revisions
      represented by an authenticated signed release manifest; actual-used provisioning agreement,
      descriptor shape and emission, annotation handling, and governed bump procedure remain in
      force"
---

# Record the Publish-Stage npm CLI in `builderDependencies`

## Context and Problem Statement

ADR 0082 pinned the publish-stage npm CLI and ADR 0084 refined its provisioning to a digest-verified
npm registry tarball, with the pin committed in `internal/toolchainpins/publish-npm.json`. ADR 0109
confirmed that `builder.version.npm` records only the build-stage bundled npm. As a result, the
publish-stage npm CLI — the tool whose behavior determines the registry-visible provenance outcome,
as the npm/cli#9882 incident demonstrated — is recorded nowhere in the signed SLSA Statement. Its
observed version appears only as `diagnostic_metadata.publish_npm_version` in the publish job's
unsigned outcome report.

SLSA provenance v1 defines `runDetails.builder.builderDependencies` as "dependencies used by the
orchestrator that are not run within the workload and that do not affect the build, but might affect
the provenance generation or security guarantees." The publish-stage npm CLI satisfies every clause
of that definition: the profile's publish job (orchestrator) invokes it, it runs outside the
consumer workload build, it does not alter the built tarball bytes, and its internal provenance
behavior (OIDC detection, `--provenance-file` handling, bundled sigstore-js) directly affects the
provenance and security guarantees of the published package.

Today a consumer reading only the signed Statement cannot learn which publish npm executed. The
version is determined transitively — the `builder.id` commit SHA fixes the embedded pin file — but
recovering it requires fetching the builder repository at that SHA and reading the pin record, out
of band. Enumerating artifacts affected by a future publish-npm defect (an npm/cli#9882-class event)
becomes a mapping exercise across builder releases instead of a provenance query.

Should the JS/TS npm profile record the publish-stage npm CLI inside the signed provenance, and if
so, in which field and with what content?

## Decision Drivers

- Record orchestrator dependencies that affect provenance generation or security guarantees in the
  field SLSA v1.2 defines for exactly that purpose.
- Bind the record to the digest-verified pin, not merely a version string, so the signed evidence
  carries the same integrity anchor the provisioning path verifies.
- Preserve ADR 0109's single-home principle: no publish npm version in `builder.version`, no return
  to `externalParameters`, no duplicate builder identity records (ADR 0104).
- Preserve the ADR 0077 signer selection, URI form, underlying digest bytes, and digest-agreement
  and validation obligations, with the descriptor's member shape and DigestSet encoding governed by
  ADR 0113; the closed-set relaxation must admit profile-declared descriptors without weakening
  signer checks.
- Fail closed: a missing, extra, malformed, or pin-inconsistent publish-npm descriptor is a
  verification error, consistent with the repository's existing closed-shape stance.
- Scope the change to the JS/TS npm profile; other present or future profiles are unaffected unless
  their own ADRs declare profile descriptors.

## Considered Options

- Record a name-keyed ResourceDescriptor for the publish-stage npm CLI in
  `runDetails.builder.builderDependencies`.
- Record the publish npm version as an additional `builder.version` key.
- Record a role-named descriptor in `buildDefinition.resolvedDependencies`.
- Record the version in a vendor-prefixed extension field (for example
  `runDetails.metadata.windlass_publish_npm_version`).
- Keep outcome-evidence-only recording and document the transitive pinning through `builder.id`.

## Decision Outcome

Chosen option: "Record a name-keyed ResourceDescriptor in `builderDependencies`", because SLSA v1.2
defines that field for dependencies used by the orchestrator outside the workload that do not affect
the build but might affect provenance generation or security guarantees, matching the publish npm
CLI's role. Both `builderDependencies` and `resolvedDependencies` use ResourceDescriptor and can
carry a versioned URI with a digest; `resolvedDependencies` is rejected because it would misclassify
the publish CLI as a build-time input, not because it cannot carry a digest. The descriptor's shape
follows ADR 0113's common `builderDependencies` descriptor shape.

### Common contract change

The shared `builderDependencies` rule changes from "exactly one signing-adapter descriptor" to:
exactly one signing-adapter descriptor per ADR 0077 as qualified by ADR 0113, preserving the signer
selection, URI form, digest agreement with the governed module graph, and validation obligations,
plus zero or more profile-declared orchestrator-dependency descriptors. Each profile-declared
descriptor must be defined by an accepted ADR for that profile, carry the unique `name` assigned by
that ADR per ADR 0113's common descriptor shape, and fail closed on unknown names, duplicates, or
extra entries. Profile admission still must not substitute or alter the signing-adapter descriptor.

### npm profile descriptor

The JS/TS npm profile emits exactly one additional descriptor, with exactly these members:

```json
{
  "name": "publish-cli",
  "uri": "pkg:npm/npm@<version from the embedded pin record>",
  "digest": { "sha512": "<lowercase hex SHA-512 of the registry tarball>" },
  "mediaType": "application/gzip",
  "annotations": { "digest_authority": "builder-pinned-record" }
}
```

This template fixes the descriptor's shape, not its values. The concrete version and digest at any
time are read from `internal/toolchainpins/publish-npm.json` (npm 11.21.1 at the time of drafting).
A version-and-digest bump under ADR 0082's bump procedure changes only those values: it is not a
change to this decision, and it does not require a new ADR. Only a change to the shape — the member
set, the digest algorithm or encoding, the `name` value, the annotation keys, or the cardinality of
one descriptor — is a new decision requiring its own ADR.

- `name` is exactly `publish-cli`, the descriptor's identity key per ADR 0113.
- `uri` is the npm Package URL of the pinned publish-stage CLI; its version equals the version in
  `internal/toolchainpins/publish-npm.json`.
- The only digest member is `sha512`, lowercase hex, of the digest-verified registry tarball — the
  encoding in-toto's DigestSet mandates for named standard algorithms. The value is the
  deterministic lowercase-hex rendering of the same digest bytes that the embedded pin record stores
  in registry-native SRI form per ADR 0084 and that the registry publishes as `dist.integrity`; the
  two forms are interconvertible by base64 decoding. The recorded value is the one verified against
  the embedded pin before first use.
- `mediaType` is exactly `application/gzip`, matching the digest-covered representation: the npm
  registry distributes the CLI as a gzipped tarball, the same representation class as the
  `node-runtime` and `package-manager-distribution` descriptors under ADR 0110's matrix.
- The only annotation is `digest_authority: "builder-pinned-record"`, the same vocabulary the
  `node-runtime` descriptor uses when the digest's authority is a builder-embedded pin record. The
  annotation is descriptive metadata only: the integrity control is the digest's comparison against
  the embedded pin, and verification policy does not evaluate the annotation. No `platform`
  annotation is defined: unlike the platform-specific Node.js runtime tarball, the npm CLI tarball
  is platform-independent. No version annotation is defined: the `uri` carries the version.
- The descriptor records the publish-stage CLI only. The build-stage bundled npm remains in
  `builder.version.npm` (ADR 0109); the build-stage Node.js runtime and any package-manager
  distribution remain in `resolvedDependencies` (ADR 0099); no `npm-cli` descriptor is introduced
  there.

### Failure behavior

Producer-side, a provisioning result whose version or tarball digest disagrees with the embedded pin
fails closed before first use (existing ADR 0082/0084 behavior) and no Statement is emitted.
Verifier-side, a missing publish-cli descriptor, more than one, a wrong URI version, a digest member
set other than exactly `sha512`, a digest unequal to the embedded pin, a missing or wrong `name`, a
missing or wrong `mediaType`, or any additional descriptor member other than `annotations` is
rejected with `windlass.verify.error.builder-dependencies-publish-cli-mismatch`. The `annotations`
member is excluded from verification: consumers ignore it per the in-toto baseline parsing rule,
since it carries descriptive metadata and no trust content. Consumer verification compares the
received descriptor against the embedded pin before checking equality with any accompanying
metadata, so a descriptor and metadata corrupted to the same wrong values still fail.

### Unchanged surfaces

The publish job's unsigned outcome report continues to carry
`diagnostic_metadata.publish_npm_version` as operational diagnostic metadata. The ADR 0082 allowlist
and bump procedure are unchanged, except that a bump pull request now also refreshes the descriptor
expectations and fixtures for the new pin.

### Consequences

- Good, because the signed Statement becomes self-contained about every orchestrator tool that
  shapes the registry-visible provenance outcome.
- Good, because impact enumeration for a future publish-npm defect becomes a provenance query rather
  than an out-of-band builder-release mapping.
- Good, because the digest-bound descriptor extends the ADR 0084 integrity anchor from provisioning
  time into the signed record consumers actually verify.
- Good, because the common rule's relaxation is name-gated and fail-closed, so it cannot silently
  weaken the signing-adapter check or admit undeclared descriptors.
- Neutral, because `builder.id` already determined the pin transitively; this change buys visibility
  and queryability, not a new trust property.
- Bad, because the common provenance contract, the npm provenance and publish specification, the
  verifier policy, the diagnostic registry, and the fixture corpus all gain a governed member and
  must change together.
- Bad, because consumers running an older verifier policy will reject provenance carrying the new
  descriptor until their policy is updated, so rollout must be release-coordinated.

### Confirmation

This decision is confirmed when:

- the common SLSA provenance v1 specification states the relaxed closed-set rule and the npm
  provenance and publish specification defines the publish-cli descriptor shape above, each with a
  valid example and rejection cases;
- the diagnostic registry contains `windlass.verify.error.builder-dependencies-publish-cli-mismatch`
  and the specification tables match it;
- the producer emits the descriptor and the validator enforces the closed shape, with fixtures
  covering valid, missing, duplicated, wrong-version, wrong-digest, wrong-name, wrong-mediaType, and
  extra-member cases, plus annotation-varied cases proving annotations are ignored;
- `fixture-check` and the workflow-check gates pass with the updated corpus;
- the M1 dogfood verification procedure reads back the descriptor from a live published attestation.

## Pros and Cons of the Options

### `builderDependencies` name-keyed descriptor

- Good, because the SLSA v1.2 definition of the field matches the publish npm CLI clause for clause:
  orchestrator-used, outside the workload, no effect on the build, direct effect on provenance and
  security guarantees.
- Good, because ResourceDescriptor carries the digest, keeping the signed record bound to the
  digest-verified provisioning pin.
- Good, because generic SLSA consumers parse it as a standard field.
- Bad, because it relaxes the common "exactly one" closed rule and requires a coordinated
  specification, validator, diagnostic, and fixture change.

### Additional `builder.version` key

- Good, because the structural change is smallest.
- Bad, because the field is defined as versions of build-platform components, while the publish npm
  CLI is an orchestrator-invoked tool rather than a platform component.
- Bad, because the map carries no digest, dropping the integrity anchor.
- Bad, because it contradicts ADR 0109's clause reserving `builder.version.npm` for the build-stage
  bundled npm and would invite confusion between two npm version keys.

### `resolvedDependencies` role-named descriptor

- Good, because role-named descriptors have precedent (ADR 0099's `node-runtime`) and carry digests.
- Bad, because `resolvedDependencies` records artifacts needed at build time and sufficient to
  initialize the build; the publish npm CLI is an input to the publish step, not the build, so the
  placement misstates its role to consumers reconstructing build inputs.

### Vendor-prefixed extension field

- Good, because it requires no change to closed standard-field validation and is explicitly
  permitted by SLSA's extension-field rules.
- Bad, because generic verifiers ignore it, so the record would not serve ecosystem-wide audit.
- Bad, because a plain version string loses the digest, and an object-valued extension duplicates
  what ResourceDescriptor already standardizes.

### Outcome-evidence-only with documented transitive pinning

- Good, because nothing changes and ADR 0104/0109's minimalism is preserved; `builder.id` already
  fixes the pin.
- Bad, because the evidence remains unsigned and out of band, and affected-artifact enumeration
  after a publish-npm defect requires repository archaeology instead of a provenance query.

## More Information

- Publish npm pinning, allowlist, and bump procedure:
  [ADR 0082](0082-pin-publish-stage-npm-cli-version-with-integrity-verified-provisioning-and-reviewed-allowlist.md),
  [ADR 0084](0084-provision-the-publish-stage-npm-cli-from-a-digest-verified-registry-tarball.md).
- Signing-adapter descriptor and closed-set origin:
  [ADR 0071](0071-activate-builder-version-and-builderdependencies-for-platform-components.md),
  [ADR 0077](0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md).
- Version-recording locations:
  [ADR 0109](0109-minimize-external-parameters-to-actual-interface-inputs.md); build-stage runtime
  descriptor: [ADR 0099](0099-record-the-digest-verified-nodejs-runtime-in-resolveddependencies.md).
- The `digest_authority` annotation vocabulary originates at the specification phase: ADR 0099
  explicitly deferred the node-runtime descriptor's name, annotation keys, and authority wording to
  the specification phase, where the value `builder-pinned-record` was defined. This decision reuses
  that vocabulary at ADR level for the descriptor it governs.
- Common descriptor shape and identity-key rule:
  [ADR 0113](0113-define-the-common-builderdependencies-descriptor-shape.md).
- Motivating upstream behavior: [npm/cli#9882](https://github.com/npm/cli/pull/9882) and the fourth
  M1 dogfood evidence in [issue #30](https://github.com/windlasstech/slsa-builder/issues/30).
- SLSA v1.2 field definition: <https://slsa.dev/spec/v1.2/build-provenance#builder>.
