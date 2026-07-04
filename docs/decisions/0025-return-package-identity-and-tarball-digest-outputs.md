---
parent: Decisions
nav_order: 25
status: accepted
date: 12026-07-01
decision-makers: Yunseo Kim
---

# Return Package Identity and Tarball Digest Outputs

## Context and Problem Statement

ADR 0002 chose an extensible trusted reusable workflow foundation, ADR 0003 assigned
artifact-specific semantics to profile-owned reusable workflows, and ADR 0022 selected
`.github/workflows/js-ts-npm-package-slsa3.yml` as the initial JS/TS npm package profile entrypoint.
ADR 0019 requires the profile to validate package metadata through the packed artifact. ADR 0023
selected `package-directory` plus optional publish-intent inputs as the caller-facing input
contract, and ADR 0024 selected OIDC trusted publishing without publish secrets.

The next public-contract decision is which `on.workflow_call.outputs` values the reusable workflow
returns to the caller. Workflow outputs are not the signed SLSA provenance itself, but they become a
stable caller-facing API that downstream jobs can consume through `needs.<job_id>.outputs.*`.
Exposing too little makes release workflows awkward to integrate; exposing too much duplicates
provenance fields, creates drift risk, and encourages callers to treat unsigned workflow outputs as
a substitute for signed provenance verification.

Which outputs should the initial JS/TS npm package profile return?

## Decision Drivers

- Keep the reusable workflow output surface small and stable.
- Return values that downstream release, notification, and verification jobs commonly need.
- Keep signed provenance and registry attestations as the source of truth for SLSA verification.
- Avoid duplicating the full SLSA predicate, `builder.id`, `buildType`, or `externalParameters` into
  unsigned workflow outputs.
- Align outputs with ADR 0019's packed-artifact validation and one npm package per run.
- Preserve compatibility with SLSA provenance, which represents artifact digests as maps keyed by
  algorithm name and can include multiple algorithms for the same artifact.
- Preserve compatibility with npm provenance verification behavior, which commonly relies on
  `sha512` package integrity, while still making `sha256` available for SLSA and ecosystem tooling
  that commonly expects it.

## Considered Options

- Return no workflow outputs.
- Return only package identity outputs.
- Return package identity plus `sha256` and `sha512` published tarball digest outputs.
- Return provenance or attestation locator outputs.
- Return SLSA provenance schema fields as workflow outputs.
- Return soft-failure outcome outputs.

## Decision Outcome

Chosen option: "Return package identity plus `sha256` and `sha512` published tarball digest
outputs", because it gives callers useful handles for the package version that was published without
turning workflow outputs into a duplicate provenance format, and because SLSA and npm verification
contexts benefit from different digest algorithms.

The initial JS/TS npm package profile should return these workflow outputs:

```yaml
outputs:
  package-name:
    description: The validated npm package name that was published.
  package-version:
    description: The validated npm package version that was published.
  package-registry-url:
    description: The resolved registry URL used for publishing.
  package-url:
    description: The resolved registry package-version URL or package page URL.
  package-tarball-name:
    description: The published package tarball name.
  package-tarball-sha256:
    description: The SHA-256 digest value for the published package tarball.
  package-tarball-sha512:
    description: The SHA-512 digest value for the published package tarball.
```

The digest output names intentionally include both `sha256` and `sha512`. SLSA's recommended
attestation suite mentions SHA-256 for the DSSE envelope signature suite, but that recommendation is
about the envelope layer rather than a requirement that SLSA predicate artifact digests use only
`sha256`. In SLSA provenance and verification summary schemas, artifact digests are represented as
digest maps keyed by algorithm name, and examples show algorithm-specific keys such as `sha256`.
Those maps can carry more than one digest algorithm for the same artifact when useful. npm package
integrity and provenance verification commonly rely on `sha512`, so the public workflow API should
return both the broadly useful `sha256` digest and the npm-aligned `sha512` digest.

This ADR does not decide the exact digest encoding or every place where each digest is recorded. A
follow-up architecture specification or ADR should decide:

- whether `package-tarball-sha256` and `package-tarball-sha512` are hexadecimal, base64, SRI-style,
  or another documented encoding;
- whether both digests are included in the SLSA subject digest map, in byproducts, or in other
  provenance fields;
- how the `package-tarball-sha512` output relates to npm registry integrity values;
- how the profile verifies that the packed tarball, published tarball, npm registry metadata, and
  signed provenance subject refer to the same package artifact.

The workflow should not expose full provenance JSON, `builder-id`, `build-type`,
`external-parameters-json`, npm attestation URLs, GitHub artifact URLs, `provenance-name`,
`provenance-sha256`, or `outcome` as initial public outputs. Those values either belong inside
signed provenance, depend on a future provenance-distribution decision, are unstable as long-term
verifier handles, or encourage soft-failure handling that the production SLSA3 publish profile
should not support by default.

### Consequences

- Good, because callers can easily report, notify, or consume the published package identity.
- Good, because downstream jobs get a cryptographic handle for the published package tarball without
  needing to parse workflow logs.
- Good, because the output contract stays aligned with one package per profile run.
- Good, because returning both digest algorithms aligns with SLSA's digest-map model and npm's
  `sha512`-oriented package integrity behavior.
- Good, because signed provenance remains the source of truth for verifier policy.
- Neutral, because callers that want a provenance artifact locator must wait for a separate
  provenance distribution decision.
- Bad, because the architecture specification still needs to decide digest encoding and registry
  verification details before implementation.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification and
implementation define:

- `package-name`, `package-version`, `package-registry-url`, `package-url`, `package-tarball-name`,
  `package-tarball-sha256`, and `package-tarball-sha512` as workflow outputs;
- the exact source of each output value after package metadata validation, packing, publishing, and
  registry resolution;
- the digest encoding used by `package-tarball-sha256` and `package-tarball-sha512`;
- how both output digests relate to SLSA subject digests and npm registry integrity values;
- review checks that prevent unsigned outputs from being documented as substitutes for signed
  provenance or npm registry attestation verification.

Implementation review should verify that the workflow emits both digest outputs from the same
published package artifact, and that neither value is derived from an unverified intermediate file
without a documented equivalence check.

## Pros and Cons of the Options

### Return no workflow outputs

Expose no `on.workflow_call.outputs` values.

- Good, because the public API is smallest.
- Good, because callers cannot accidentally confuse workflow outputs with signed provenance.
- Bad, because downstream release notes, notifications, and verification jobs cannot easily consume
  the published package result.
- Bad, because callers would need to re-derive package identity or digest values from source files,
  logs, or registry lookups.

### Return only package identity outputs

Return package identity values such as `package-name`, `package-version`, `package-registry-url`,
and `package-url`.

- Good, because these are stable and useful release outputs.
- Good, because they are independent of npm provenance distribution details.
- Neutral, because the exact package URL format still needs specification.
- Bad, because downstream verification jobs lack a digest handle for the package artifact.

### Return package identity plus `sha256` and `sha512` tarball digest outputs

Return package identity outputs plus `package-tarball-name`, `package-tarball-sha256`, and
`package-tarball-sha512`.

- Good, because it provides the package identity and cryptographic artifact handle callers commonly
  need.
- Good, because callers that need `sha256` do not need to compute it themselves.
- Good, because callers and tools that need npm-aligned `sha512` package integrity do not need to
  compute it themselves.
- Good, because it aligns with SLSA ResourceDescriptor digest-map flexibility and npm registry
  integrity behavior.
- Neutral, because the workflow output API is slightly larger than a single digest plus algorithm
  field.
- Bad, because the implementation still needs careful validation to ensure the digest describes the
  published package artifact rather than an intermediate file.

### Return provenance or attestation locator outputs

Return values such as `provenance-name`, `provenance-sha256`, `provenance-artifact-url`,
`npm-provenance-url`, or `npm-attestation-count`.

- Good, because these values could help a caller find generated attestations.
- Good, because SLSA GitHub Generator workflows expose provenance artifact names in some modes.
- Bad, because ADR 0024 currently relies on npm trusted publishing for registry-side provenance
  behavior and does not yet decide a Windlass-owned attestation distribution contract.
- Bad, because GitHub artifact URLs are run-scoped and retention-scoped rather than stable verifier
  handles.
- Bad, because npm provenance lookup behavior is not the same as a stable reusable workflow output
  contract.

### Return SLSA provenance schema fields as workflow outputs

Return values such as `builder-id`, `build-type`, `invocation-id`, `source-repository`,
`source-ref`, `source-digest`, or `external-parameters-json`.

- Good, because these outputs could make logs and ad hoc debugging more convenient.
- Bad, because they duplicate fields that should be verified inside signed provenance.
- Bad, because JSON outputs introduce escaping, size, and masking concerns.
- Bad, because `builder.id` and `buildType` are already defined by ADR 0020 and ADR 0021 and should
  not need to be rediscovered through unsigned caller outputs.

### Return soft-failure outcome outputs

Return an `outcome` output such as `success` or `failure`, usually paired with a `continue-on-error`
input.

- Good, because this pattern can support optional provenance generation or migration workflows.
- Bad, because the production JS/TS npm package SLSA3 profile should fail the release job when
  validation, packing, publishing, or provenance-relevant checks fail.
- Bad, because soft-failure outputs create pressure for callers to ignore release integrity
  failures.

## More Information

This decision follows ADR 0019, ADR 0023, and ADR 0024. It decides only the initial reusable
workflow outputs, not the signed SLSA provenance schema, npm trusted publishing setup, provenance
distribution mechanism, or exact digest encoding.

Reference points considered:

- SLSA provenance v1 records output artifacts in the in-toto statement `subject` and can record
  build-time artifacts using ResourceDescriptor digest maps.
- SLSA v1.2 treats `externalParameters` as the verifier-relevant external interface and recommends
  minimizing externally verified surfaces for usability.
- SLSA's recommended attestation suite mentions SHA-256 for the DSSE envelope signature suite; that
  recommendation is about the envelope layer, not a restriction on predicate artifact digest keys.
- SLSA v1.2 provenance and verification summary schemas use digest maps rather than requiring one
  universal digest algorithm for every artifact.
- GitHub reusable workflow outputs are a stable caller-facing API exposed through
  `on.workflow_call.outputs` and consumed through downstream job `needs` outputs.
- SLSA GitHub Generator workflows keep reusable workflow outputs focused on artifact or provenance
  handles rather than duplicating full predicate contents.
- npm provenance and registry package integrity commonly use `sha512`, while SLSA examples commonly
  show algorithm-specific digest keys such as `sha256`; the initial profile should expose both
  tarball digests instead of forcing callers to choose one ecosystem's default.
