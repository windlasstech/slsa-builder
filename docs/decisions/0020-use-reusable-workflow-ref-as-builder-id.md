---
parent: Decisions
nav_order: 20
status: superseded by ADR-0028
date: 12026-06-29
decision-makers: Yunseo Kim
---

# Use Reusable Workflow References as `builder.id`

## Context and Problem Statement

ADR 0002 chose an extensible trusted reusable workflow foundation with SLSA provenance v1 as the
baseline provenance format. ADR 0003 refined that architecture into a thin shared core with
profile-owned reusable workflows. ADR 0013 through ADR 0019 then scoped the initial JS/TS npm
package profile and its package-manager, package-selection, publishing, and metadata-validation
boundaries.

The next provenance schema decision is the `runDetails.builder.id` URI format. SLSA provenance v1
defines `builder.id` as the URI that identifies the transitive closure of the trusted build
platform. Verifiers use the recognized signer identity and `builder.id` together to decide whether
provenance comes from a trusted builder and what SLSA Build level, if any, they trust it to satisfy.

For this project, the trusted build platform is not merely GitHub Actions as a generic runner. It is
the Windlass-owned reusable workflow profile running on GitHub-hosted runners with the trust,
isolation, handoff, provenance, signing, and publish constraints defined by the profile contract.
The `builder.id` format therefore needs to identify the reusable workflow identity precisely enough
for downstream verification while still resolving to useful documentation for humans.

What URI format should `slsa-builder` use for `builder.id` values emitted by profile-owned reusable
workflows?

## Decision Drivers

- Align with SLSA provenance v1's model: `builder.id` identifies the trusted build platform, while
  `buildType` identifies the parameterized build template.
- Make verifier roots of trust practical by using an identity that can be matched with the Sigstore
  or GitHub workflow signer identity.
- Follow the ecosystem convention established by SLSA GitHub Generator reusable workflows.
- Avoid treating all GitHub Actions runner executions as equivalent to the Windlass trusted builder
  profile.
- Preserve version-specific trust decisions so a verifier can trust one released builder version
  without implicitly trusting all future workflow changes.
- Keep security modes with different SLSA Build level claims or trust boundaries distinguishable.
- Keep room for human-readable documentation that explains the builder security model and field
  guarantees.

## Considered Options

- Use the reusable workflow file path and full Git ref as `builder.id`.
- Use the reusable workflow file path without a ref as `builder.id`.
- Use a stable product or documentation URI as `builder.id`.
- Use a generic GitHub Actions runner or platform URI as `builder.id`.
- Use a stable mode URI as `builder.id` and record the reusable workflow ref only in
  `builder.version`.

## Decision Outcome

Chosen option: "Use the reusable workflow file path and full Git ref as `builder.id`", because it
matches the GitHub reusable workflow identity shape used by SLSA GitHub Generator, gives verifiers a
specific builder version to trust, and keeps the Windlass profile-owned reusable workflow boundary
visible in the provenance.

Profile-owned reusable workflows should emit `builder.id` values in this form:

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/<profile-workflow>.yml@refs/tags/vX.Y.Z
```

For example, the initial JS/TS npm package profile should use a value shaped like:

```text
https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/tags/v0.1.0
```

The exact workflow filename and release tag are determined by the profile workflow and released
builder version. The URI is intentionally not the GitHub web UI URL with `/blob/`; it is the
reusable workflow identity plus the full Git reference, following the ecosystem convention used by
builders such as SLSA GitHub Generator.

Release provenance should not use branch refs, short refs, floating refs, or caller-controlled refs
as `builder.id`. Release provenance should use a released builder tag ref. Development, test, or
pre-release provenance may use non-release identities only if those identities do not claim the same
SLSA Build level or production trust boundary as the released builder.

Different profile workflows should have different `builder.id` values. Any mode with materially
different security properties, isolation guarantees, signing behavior, runner trust, or claimed SLSA
Build level should also use a different `builder.id`; it may not reuse a production profile
`builder.id` merely because it shares implementation code.

The documentation for each `builder.id` should explain:

- the reusable workflow and release ref represented by the ID;
- the profile and artifact type it applies to;
- the claimed SLSA Build level;
- the trusted control-plane boundary and GitHub-hosted runner assumptions;
- which provenance fields are generated or verified by trusted workflow logic;
- any fields, other than the subject, that originate from tenant-controlled build steps or package
  manager outputs;
- the associated `buildType` URI and verification expectations.

The provenance should also record builder implementation details in `runDetails.builder.version` or
`runDetails.builder.builderDependencies` where useful, but those fields do not replace the
version-specific `builder.id`.

### Consequences

- Good, because downstream verifiers can bind trust to a concrete Windlass reusable workflow
  release.
- Good, because the URI shape aligns with the SLSA GitHub Generator convention and Sigstore workflow
  identity matching patterns.
- Good, because a caller-controlled workflow using GitHub Actions directly is not confused with the
  Windlass trusted profile workflow.
- Good, because profile and security-mode changes can produce distinct builder identities when their
  trust properties differ.
- Good, because the `builder.id` remains separate from `buildType`, allowing one workflow identity
  to point to a profile-specific build template URI.
- Neutral, because verifiers may need wildcard or release-pattern trust policies for multiple
  released builder versions.
- Bad, because every released builder tag changes the exact `builder.id`, requiring trust-policy
  updates or carefully scoped tag patterns.
- Bad, because GitHub's reusable workflow identity URI is not the same as the human-facing `/blob/`
  URL, so documentation must explain how to inspect the workflow source.

### Confirmation

This decision is confirmed when the initial architecture specification defines:

- the concrete `builder.id` value pattern for each profile-owned reusable workflow;
- release-only use of full `refs/tags/vX.Y.Z` refs for production builder identities;
- prohibition of branch, short, floating, or caller-controlled refs for production `builder.id`
  values;
- distinct `builder.id` values for profiles or modes with different trust boundaries or SLSA Build
  level claims;
- the documentation page or section that each `builder.id` resolves to or is explained by;
- how `builder.id`, `buildType`, `builder.version`, and `builderDependencies` relate to one another;
- verifier examples that match the selected builder identity and expected signer identity.

Implementation review should verify that production provenance never emits a generic GitHub Actions
runner URI, an unversioned workflow URI, a GitHub `/blob/` UI URL, or a caller workflow identity as
the Windlass profile `builder.id` unless a later ADR explicitly changes this decision.

## Pros and Cons of the Options

### Use the reusable workflow file path and full Git ref

Use a GitHub reusable workflow identity with a full release ref, such as
`https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/tags/v0.1.0`.

- Good, because it follows the ecosystem convention used by SLSA GitHub Generator reusable
  workflows.
- Good, because it can be matched against Sigstore or GitHub workflow signer identities and verifier
  root-of-trust patterns.
- Good, because the exact builder release is visible in the provenance.
- Good, because trust can be granted to a specific builder version or a controlled release-tag
  pattern.
- Good, because it keeps the profile-owned reusable workflow boundary explicit.
- Neutral, because a stable documentation page is still needed to explain the security model.
- Bad, because exact `builder.id` values change on each released builder tag.

### Use the reusable workflow file path without a ref

Use a stable workflow URI without a version ref, such as
`https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml`.

- Good, because verifier configuration is simpler when the builder identity is stable.
- Good, because the identity names the profile workflow rather than a generic platform.
- Bad, because it hides which builder version actually produced the provenance.
- Bad, because mutable workflow changes can alter trust properties without changing `builder.id`.
- Bad, because it is less aligned with common GitHub reusable workflow builder identity patterns.

### Use a stable product or documentation URI

Use a stable URI controlled by Windlass, such as
`https://slsa-builder.windlass.dev/builders/js-ts-npm-package-slsa3`.

- Good, because it can resolve directly to human-readable security model documentation.
- Good, because the URI can remain stable if workflow filenames or repository layout change.
- Good, because it could abstract over future non-GitHub builder backends.
- Bad, because it is less directly tied to the actual GitHub workflow identity in signer
  certificates and verifier policies.
- Bad, because an additional mapping is required to prove which workflow implementation corresponds
  to the documented builder identity.
- Bad, because it is more abstraction than the initial GitHub reusable workflow profile needs.

### Use a generic GitHub Actions runner or platform URI

Use a broad platform identity, such as `https://github.com/actions/runner`.

- Good, because it names the hosted CI/CD platform involved in the build.
- Good, because it resembles some npm provenance contexts where the CI provider is the main platform
  identity.
- Bad, because it does not identify the Windlass reusable workflow profile or its trust contract.
- Bad, because it treats direct caller workflow builds and Windlass profile builds as too similar.
- Bad, because it cannot distinguish profiles, security modes, or SLSA Build level claims.

### Use a stable mode URI and record workflow ref in `builder.version`

Use a stable mode-specific URI as `builder.id`, then record the exact workflow ref in
`runDetails.builder.version`.

- Good, because it can separate security modes while keeping a stable builder identity.
- Good, because `builder.version` can carry detailed implementation version information.
- Bad, because verifier roots of trust usually key on `builder.id`, not only on `builder.version`.
- Bad, because the exact workflow identity becomes secondary instead of the main trusted identity.
- Bad, because it diverges from the strongest GitHub reusable workflow convention for SLSA builders.

## More Information

This decision complements ADR 0002 and ADR 0003 by making the profile-owned reusable workflow the
visible `builder.id` trust boundary in SLSA provenance v1.

Reference points considered:

- SLSA provenance v1 defines `predicateType: https://slsa.dev/provenance/v1`, places builder
  identity at `runDetails.builder.id`, and treats that URI as the identifier for the trusted build
  platform.
- SLSA v1.2 verification guidance expects verifiers to trust specific signer and `builder.id` pairs
  and to compare `buildType` and `externalParameters` against expectations.
- SLSA GitHub Generator uses GitHub reusable workflow identities such as
  `https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@refs/tags/v2.1.0`
  as builder IDs.
- GitHub artifact attestations bind attestations to GitHub Actions workflow identities and provide
  verification through GitHub CLI and Sigstore-backed attestations.
- npm provenance requires supported cloud CI/CD providers and cloud-hosted runners, but npm package
  provenance alone does not replace this project's profile-specific trusted builder identity.

Future decisions may define:

- the concrete initial JS/TS npm package profile workflow filename;
- the exact `buildType` URI scheme for each profile;
- the `runDetails.builder.version` and `builderDependencies` schema;
- the verifier policy format for trusted Windlass builder releases;
- how pre-release, test, or experimental builders identify themselves without claiming production
  builder trust.
