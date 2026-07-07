---
parent: Decisions
nav_order: 21
status: superseded by ADR-0042
date: 12026-06-29
decision-makers: Yunseo Kim
---

# Use Profile-Specific `buildType` URIs

## Context and Problem Statement

ADR 0002 chose SLSA provenance v1 as the baseline provenance format for the reusable workflow
foundation. ADR 0003 assigned profile-specific subject, digest, publish, and verification semantics
to profile-owned reusable workflows. ADR 0013 through ADR 0019 then scoped the initial JS/TS npm
package profile. ADR 0020 decided that `runDetails.builder.id` should identify the released
profile-owned reusable workflow using the workflow path plus full Git tag ref.

The next provenance schema decision is the `buildDefinition.buildType` URI convention. In SLSA
provenance v1, `builder.id` identifies the trusted build platform, while `buildType` identifies the
parameterized build template and tells verifiers how to interpret `externalParameters`,
`internalParameters`, and `resolvedDependencies`.

The initial JS/TS npm package profile needs a `buildType` that can describe npm package release
semantics: package selection, package-manager selection, pack behavior, publish intent, npm package
identity, tarball digest expectations, and verifier-relevant parameters. A generic GitHub Actions
workflow build type does not capture those package semantics, and reusing an npm CLI-owned build
type would misrepresent the Windlass-owned profile contract.

What URI convention should `slsa-builder` use for `buildType` values emitted by profile-owned
reusable workflows?

## Decision Drivers

- Preserve SLSA provenance v1's separation between `builder.id` as the trusted platform identity and
  `buildType` as the build template and parameter schema identity.
- Give verifiers a stable, profile-specific schema URI for interpreting `externalParameters` and
  rejecting unexpected fields.
- Keep builder release versions separate from build type schema versions so patch releases do not
  create unnecessary `buildType` churn.
- Make profile semantics first-class, especially npm package identity, packed artifact behavior, and
  publish-time verification expectations.
- Use a URI shape that can be documented in this repository before a separate documentation domain
  or site exists.
- Keep future profile expansion straightforward without overloading one generic build type.
- Preserve the option to move to a stable Windlass documentation domain later if the project has the
  documentation infrastructure to support it.

## Considered Options

- Use Windlass-owned, profile-specific, major-versioned repository-path `buildType` URIs.
- Use the community GitHub Actions workflow `buildType`.
- Reuse or emulate the npm CLI GitHub Actions `buildType`.
- Use the released reusable workflow identity as both `builder.id` and `buildType`.
- Use a stable Windlass documentation-domain `buildType` URI.
- Use a GitHub Pages or SLSA-community-style build types URI.

## Decision Outcome

Chosen option: "Use Windlass-owned, profile-specific, major-versioned repository-path `buildType`
URIs", because the build type is a schema and verification contract owned by `slsa-builder`, not by
GitHub Actions, npm CLI, or a specific released workflow implementation.

Profile-owned reusable workflows should emit `buildType` values in this form:

```text
https://github.com/windlasstech/slsa-builder/buildtypes/<profile>/v<major>
```

The initial JS/TS npm package profile should use:

```text
https://github.com/windlasstech/slsa-builder/buildtypes/js-ts-npm-package/v1
```

The `<profile>` segment identifies the artifact and profile semantics, not the workflow filename.
The `<major>` segment identifies the major version of the build type schema. Backward-compatible
schema clarifications and additional optional fields may remain on the same major version. Breaking
changes to the meaning, required fields, verification expectations, or supported build template must
use a new major version such as `/v2`.

The `buildType` URI is intentionally separate from the released workflow `builder.id` chosen in
ADR 0020. For example, a released workflow identity may change from tag to tag while the build type
stays stable:

```text
builder.id: https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/tags/v0.1.0
buildType:  https://github.com/windlasstech/slsa-builder/buildtypes/js-ts-npm-package/v1
```

The repository should provide a human-readable build type specification for each `buildType` URI. A
future architecture specification may choose the exact file layout, but the URI must map clearly to
a repository-owned specification path such as:

```text
docs/architecture/buildtypes/js-ts-npm-package-v1.md
```

or:

```text
docs/buildtypes/js-ts-npm-package/v1.md
```

Each build type specification should define:

- the build type URI;
- the profile and artifact scope;
- supported and unsupported invocation modes;
- `externalParameters` schema and verification expectations;
- `internalParameters` schema, if any;
- `resolvedDependencies` rules;
- `runDetails.metadata.invocationId` convention;
- subject and digest expectations related to the build type;
- relationship to `builder.id`, `builder.version`, and `builderDependencies`;
- a complete example predicate;
- version history.

The stable documentation-domain option remains a valid future direction. If Windlass later provides
a stable project documentation domain or versioned documentation site, a follow-up ADR may revisit
this decision and move future `buildType` URIs to a domain such as:

```text
https://slsa-builder.windlass.dev/buildtypes/js-ts-npm-package/v1
```

That future move should define compatibility and transition rules for already-published provenance.

### Consequences

- Good, because `buildType` is owned by the project that owns the profile schema.
- Good, because the JS/TS npm package profile can define npm-specific parameters rather than forcing
  them into a generic GitHub Actions workflow schema.
- Good, because verifier expectations can key on a stable profile schema while `builder.id` remains
  version-specific to a released workflow.
- Good, because future profiles can add their own build type URIs without changing the core URI
  convention.
- Good, because major-versioned build type URIs make breaking schema changes visible to verifiers.
- Neutral, because the URI is repository-hosted rather than hosted on a dedicated documentation
  domain.
- Bad, because Windlass must maintain clear build type specifications and examples for each profile.
- Bad, because third-party verifiers will not recognize these custom build type URIs unless their
  policies or documentation are updated.

### Confirmation

This decision is confirmed when the initial architecture specification defines:

- `https://github.com/windlasstech/slsa-builder/buildtypes/js-ts-npm-package/v1` as the initial
  JS/TS npm package profile `buildType`;
- a repository-local specification document that explains that build type;
- a schema for the profile's `externalParameters`, `internalParameters`, and `resolvedDependencies`;
- versioning rules for backward-compatible and breaking changes;
- examples showing the distinction between `builder.id` and `buildType`;
- verifier examples that reject unexpected `externalParameters` for the selected build type;
- a future-decision boundary for moving to a stable Windlass documentation-domain URI.

Implementation review should verify that production provenance does not reuse the community GitHub
Actions workflow build type, npm CLI build type, generic runner identity, or released workflow
identity as the Windlass profile `buildType` unless a later ADR explicitly changes this decision.

## Pros and Cons of the Options

### Use Windlass-owned repository-path `buildType` URIs

Use project-owned, profile-specific, major-versioned URIs such as
`https://github.com/windlasstech/slsa-builder/buildtypes/js-ts-npm-package/v1`.

- Good, because the URI identifies the Windlass profile schema rather than a generic platform or a
  specific workflow implementation.
- Good, because the convention mirrors the SLSA GitHub Generator pattern of project-owned build type
  URIs while making the build type specification path explicit.
- Good, because the schema version is decoupled from builder release tags.
- Good, because custom npm package release semantics can be documented directly.
- Neutral, because URI-to-file mapping must be documented until a dedicated documentation site
  exists.
- Bad, because Windlass must maintain these specifications and examples.

### Use the community GitHub Actions workflow `buildType`

Use `https://slsa-framework.github.io/github-actions-buildtypes/workflow/v1`.

- Good, because it is a documented community-maintained build type.
- Good, because it describes top-level GitHub Actions workflow execution and common GitHub event
  parameters.
- Bad, because it is not intended to describe a reusable workflow, action, or job subset.
- Bad, because it does not describe JS/TS npm package profile semantics, package-manager selection,
  pack behavior, publish intent, or npm registry verification expectations.
- Bad, because profile-specific data would have to be forced into an inappropriate schema or hidden
  elsewhere.

### Reuse or emulate the npm CLI GitHub Actions `buildType`

Use a URI such as `https://github.com/npm/cli/gha/v2`.

- Good, because it is close to npm package provenance and npm registry verification contexts.
- Good, because the initial profile publishes npm packages.
- Bad, because that URI is owned by npm CLI and describes npm CLI provenance behavior, not the
  Windlass reusable workflow profile.
- Bad, because Windlass profile validation and artifact handling are broader than direct
  `npm publish --provenance` behavior.
- Bad, because reusing another tool's build type would mislead verifiers about which schema and
  semantics apply.

### Use the released reusable workflow identity as `buildType`

Use the same versioned reusable workflow identity for both `builder.id` and `buildType`.

- Good, because the implementation and schema appear to be one thing.
- Good, because it is easy to generate from workflow context.
- Bad, because it collapses SLSA's distinction between trusted platform identity and build template
  identity.
- Bad, because every builder release tag would appear to create a new build type even when the
  parameter schema is unchanged.
- Bad, because verifier expectations would churn unnecessarily on patch releases.

### Use a stable Windlass documentation-domain URI

Use a dedicated documentation-domain URI such as
`https://slsa-builder.windlass.dev/buildtypes/js-ts-npm-package/v1`.

- Good, because it can resolve directly to a human-readable specification independent of repository
  layout.
- Good, because it provides the cleanest long-term documentation and verifier UX.
- Good, because it allows future non-GitHub implementation details without changing the build type
  URI.
- Neutral, because it is a good future target if Windlass has stable documentation hosting.
- Bad, because the current repository does not yet have that documentation domain or publication
  workflow.
- Bad, because introducing it now would add operational requirements before the profile schema is
  implemented.

### Use a GitHub Pages or SLSA-community-style build types URI

Use a URI such as `https://windlasstech.github.io/slsa-builder/buildtypes/js-ts-npm-package/v1`.

- Good, because it resembles the SLSA community build type documentation style.
- Good, because it can resolve to rendered documentation without a custom domain.
- Bad, because it requires GitHub Pages or another docs publishing workflow.
- Bad, because source and published documentation synchronization becomes part of the trust and
  documentation process.
- Bad, because it is less direct than a repository-owned URI at the current scaffold stage.

## More Information

This decision complements ADR 0020. ADR 0020 identifies the released reusable workflow as the
`builder.id`; this ADR identifies the profile schema as the `buildType`.

Reference points considered:

- SLSA provenance v1 requires `buildDefinition.buildType` and `externalParameters` for SLSA Build L1
  and treats `buildType` as the schema for interpreting build parameters and dependencies.
- SLSA provenance v1 recommends that `buildType` URIs resolve to human-readable specifications with
  parameter schemas, initiation instructions, and a complete example.
- The community GitHub Actions workflow build type documents top-level workflow execution and is not
  intended to describe reusable workflows, actions, or jobs.
- SLSA GitHub Generator uses project-owned build type URIs such as
  `https://github.com/slsa-framework/slsa-github-generator/generic@v1` and
  `https://github.com/slsa-framework/slsa-github-generator/go@v1`.
- npm provenance examples use npm-owned build type URIs such as `https://github.com/npm/cli/gha/v2`,
  which should not be reused for a Windlass-owned builder schema.

Future decisions may define:

- the exact repository-local file path for build type specifications;
- the complete JS/TS npm package profile `externalParameters` schema;
- `internalParameters`, `resolvedDependencies`, and `byproducts` conventions;
- when a schema change requires a new major build type version;
- whether future `buildType` URIs should move to a stable Windlass documentation domain.
