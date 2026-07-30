---
parent: Decisions
nav_order: 64
status: accepted
date: 12026-07-31
decision-makers: Yunseo Kim
---

# Use npm PURL Subject with SHA-512 and SHA-256 Digests

## Context and Problem Statement

ADR 0029 selected Windlass-generated SLSA provenance as the canonical npm package provenance. ADR
0055 then selected stock `actions/attest` custom mode as the initial signing adapter, with Windlass
supplying the subject inputs, predicate type, and SLSA provenance predicate while the adapter
constructs and signs the in-toto Statement.

The npm profile also needs the signed bundle emitted by `actions/attest` custom mode to work with
`npm publish --provenance-file`. Investigation of the official npm CLI documentation and npm CLI
source shows that `--provenance-file` accepts a Sigstore bundle, extracts the DSSE payload, and
validates the embedded Statement subject before publication. For npm packages, npm expects the
subject name to be the npm Package URL for the published package version, and it verifies the
published tarball's SHA-512 digest.

Earlier composition decisions for GitHub Release assets emphasized a producer subject name matching
the final release asset name, which naturally led to a tarball-filename subject. That shape is
useful for release asset verification, but it is not the subject shape npm CLI validates for
`--provenance-file`. The profile needs one subject format that preserves npm CLI compatibility while
keeping Windlass's SHA-256-centered handoff and release-asset verification model.

What should the JS/TS npm profile use as the in-toto Statement subject for the signed provenance
bundle submitted through `npm publish --provenance-file`?

## Decision Drivers

- Keep `npm publish --provenance-file` on the official npm CLI path.
- Preserve Windlass-generated SLSA predicate semantics, including Windlass-owned `buildType`,
  `builder.id`, and `externalParameters`.
- Continue using stock, full-SHA-pinned `actions/attest` custom mode rather than introducing a
  custom signer immediately.
- Keep the tarball SHA-256 available for cross-job handoff, public outputs, GitHub Release asset
  publication, and consumer verification.
- Avoid creating two canonical provenance bundles for the same npm package release.
- Make the npm package identity explicit in the verifier-visible Statement subject.

## Considered Options

- Use npm Package URL subject naming with both SHA-512 and SHA-256 digests.
- Use npm Package URL subject naming with only SHA-512.
- Keep tarball-filename subject naming with SHA-256.
- Produce separate npm-publish and GitHub-Release provenance bundles.
- Use npm CLI automatic provenance generation instead of Windlass-generated `--provenance-file`
  provenance.
- Bypass npm CLI `--provenance-file` with a custom publish or registry attachment path.

## Decision Outcome

Chosen option: "Use npm Package URL subject naming with both SHA-512 and SHA-256 digests", because
it matches npm CLI's `--provenance-file` subject requirements while preserving Windlass's SHA-256
digest as a verifier-visible claim in the same canonical provenance bundle.

For the initial JS/TS npm package profile, the signed SLSA provenance Statement subject must be the
published npm package version, not the tarball filename. The subject must have exactly one entry
with this shape:

```json
{
  "name": "pkg:npm/%40scope/name@1.2.3",
  "digest": {
    "sha512": "<published tarball sha512 hex>",
    "sha256": "<published tarball sha256 hex>"
  }
}
```

The subject `name` must equal the npm Package URL that npm CLI derives for the package being
published. The `sha512` digest must equal the npm-published tarball bytes using the digest npm CLI
checks for `--provenance-file`. The `sha256` digest must equal the same tarball bytes using the
Windlass handoff digest algorithm.

The tarball filename remains verifier-relevant, but it is no longer the npm provenance Statement
subject name. It must be represented in profile-defined provenance fields and producer-to-publisher
handoff fields, and it must be checked against the actual pack-produced tarball and any GitHub
Release asset copy. The GitHub Release asset publisher must verify the tarball bytes and final asset
name through producer policy and handoff fields rather than requiring the npm provenance subject
name itself to equal the release asset name.

This decision narrows earlier subject-name assumptions only for the JS/TS npm package producer path
that submits provenance through `npm publish --provenance-file`. It does not change the generic
principle that future producer profiles may define subject names appropriate to their publication
surface, nor does it allow checksum files, SBOMs, provenance sidecars, or secondary artifacts as
additional Statement subjects.

### Consequences

- Good, because the same `actions/attest` custom-mode bundle can satisfy npm CLI `--provenance-file`
  validation.
- Good, because Windlass keeps one canonical SLSA provenance bundle for the npm package release.
- Good, because the Statement subject directly identifies the published npm package version.
- Good, because SHA-512 supports npm's registry-facing package integrity check while SHA-256 remains
  available for Windlass handoff and release asset verification.
- Good, because producer-side verification can require both digest algorithms to match the same
  tarball bytes before publication.
- Neutral, because the Statement subject digest map is broader than npm's minimum `sha512` check.
- Bad, because GitHub Release asset composition can no longer use `subject[0].name` as the final
  release asset name for npm tarballs.
- Bad, because the architecture specs and fixtures must distinguish package-version subject identity
  from tarball filename and release asset name.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, tests, and
fixtures define:

- npm Package URL subject naming for the JS/TS npm producer provenance Statement;
- mandatory `sha512` and `sha256` subject digests for the same packed tarball bytes;
- `actions/attest` custom-mode invocation with explicit subject name and digest inputs that can emit
  the required subject;
- producer-side extraction and comparison of the signed Statement payload before
  `npm publish --provenance-file`;
- rejection of tarball-filename npm provenance subjects, missing `sha512`, missing `sha256`, digest
  mismatch, multiple subjects, and raw Statement files used in place of Sigstore bundles;
- profile-defined fields or handoff fields for tarball filename and GitHub Release asset name;
- GitHub Release asset publisher verification that does not require npm `subject[0].name` to equal
  the release asset name when the trusted producer policy identifies the subject as an npm Package
  URL and separately proves the tarball filename and SHA-256 bytes.

Implementation review should verify that the npm publish path never falls back to
`npm publish --provenance`, never omits the Windlass-generated SLSA predicate, and never
reserializes or rewrites the signed bundle bytes between the `actions/attest` output and
`npm publish --provenance-file`.

## Pros and Cons of the Options

### Use npm PURL subject with SHA-512 and SHA-256

Use the npm Package URL as `subject[0].name`, include npm's required tarball `sha512`, and also
include Windlass's tarball `sha256` in the same subject digest map.

- Good, because it satisfies the npm CLI subject name and SHA-512 expectations for
  `--provenance-file`.
- Good, because the SHA-256 digest remains part of the signed Statement and can be used by Windlass
  verification without relying only on profile-specific fields.
- Good, because it avoids duplicate provenance bundles for npm and GitHub Release distribution.
- Bad, because release asset subject-name equality must move from the Statement subject name to
  profile policy and handoff verification for the npm composition.
- Bad, because registry behavior should be covered by compatibility fixtures in case npm narrows
  accepted digest maps in a future CLI or registry change.

### Use npm PURL subject with only SHA-512

Use exactly the npm Package URL subject name and only the digest npm CLI validates.

- Good, because it is the smallest subject shape known to satisfy npm CLI `--provenance-file`.
- Good, because it avoids relying on npm accepting extra digest algorithms.
- Bad, because SHA-256 would be absent from the Statement subject and would need to be trusted only
  through `externalParameters`, handoff fields, or workflow outputs.
- Bad, because GitHub Release asset verification would need more npm-specific policy to bind the
  release asset bytes to the npm provenance.

### Keep tarball-filename subject with SHA-256

Keep the earlier subject shape where `subject[0].name` is the pack-produced `.tgz` basename and
`subject[0].digest.sha256` is the tarball SHA-256.

- Good, because it aligns with existing release asset subject-name equality assumptions.
- Good, because it keeps SHA-256 as the single primary digest algorithm.
- Bad, because npm CLI `--provenance-file` validates the subject against the package Package URL and
  tarball SHA-512, so this shape is not compatible with the official npm publish path.
- Bad, because keeping this shape would force either a custom publish path or separate npm
  provenance generation.

### Produce separate npm and GitHub Release provenance bundles

Generate one signed bundle for npm publication and another for GitHub Release asset distribution.

- Good, because each publication surface can keep its preferred subject naming model.
- Good, because the release asset publisher could keep subject-name equality unchanged.
- Bad, because two provenance bundles for the same tarball create verifier ambiguity and additional
  signing, handoff, fixture, and documentation work.
- Bad, because this weakens the single canonical Windlass provenance model for the npm package
  release.

### Use npm CLI automatic provenance generation

Use `npm publish --provenance` or trusted publishing's automatic provenance rather than submitting a
Windlass-generated bundle through `--provenance-file`.

- Good, because it follows npm's default provenance path.
- Good, because npm controls subject construction and signing.
- Bad, because it does not preserve Windlass-owned `buildType`, `builder.id`, or
  `externalParameters` as the canonical npm provenance predicate.
- Bad, because it conflicts with the Windlass-generated provenance direction selected by ADR 0029.

### Bypass npm CLI provenance-file with a custom publish path

Use a custom registry API or publishing tool to attach provenance without npm CLI's
`--provenance-file` subject checks.

- Good, because it could theoretically preserve the tarball-filename subject.
- Bad, because it leaves the official npm CLI publish path and expands authentication, registry, and
  compatibility risk.
- Bad, because the initial production profile should not depend on undocumented registry attachment
  behavior.

## More Information

This decision follows ADR 0029, ADR 0035, ADR 0036, ADR 0050, ADR 0051, ADR 0052, and ADR 0055. It
keeps stock `actions/attest` custom mode and `npm publish --provenance-file`, but changes the npm
producer's Statement subject semantics to match npm CLI's package-version subject model.

Reference points considered:

- The npm CLI `npm publish` documentation defines `provenance-file` as a path to the provenance
  bundle used during publish and makes it mutually exclusive with `provenance`.
- npm CLI `libnpmpublish` reads the supplied provenance bundle, extracts `dsseEnvelope.payload`,
  requires exactly one subject, checks the subject name against the npm Package URL for the package
  spec, checks `subject.digest.sha512` against the tarball bytes, verifies the Sigstore bundle, and
  attaches the serialized bundle to the publish metadata.
- The `actions/attest` README and `action.yml` document custom attestation inputs as subject inputs,
  `predicate-type`, and `predicate` or `predicate-path`; its output is a JSON-serialized Sigstore
  bundle file.

Primary references:

- <https://docs.npmjs.com/cli/v11/commands/npm-publish>
- <https://docs.npmjs.com/generating-provenance-statements>
- <https://github.com/npm/cli/blob/release/v11/workspaces/libnpmpublish/lib/publish.js>
- <https://github.com/npm/cli/blob/release/v11/workspaces/libnpmpublish/lib/provenance.js>
- <https://github.com/actions/attest>
- <https://github.com/actions/attest/blob/main/action.yml>
