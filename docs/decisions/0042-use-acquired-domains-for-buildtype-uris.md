---
parent: Decisions
nav_order: 42
status: partially updated by ADR-0049
date: 12026-07-06
decision-makers: Yunseo Kim
---

# Use Acquired Domains for `buildType` URIs: `buildtype.dev` as Identifier, `slsa-builder.dev` as Documentation Host

## Context and Problem Statement

ADR 0021 selected Windlass-owned, profile-specific, major-versioned repository-path `buildType` URIs
for profile-owned reusable workflows. ADR 0041 applied that convention to the GitHub Release asset
profile. Both decisions intentionally left open a future move to a stable Windlass documentation
domain once such infrastructure existed.

Windlass has now acquired the relevant domains for this project and future build type namespace
work:

- `slsa-builder.dev` – the project documentation site.
- `buildtype.dev` – the canonical namespace for `buildType` URI identifiers.
- `buildtypes.dev` – a registry, index, or alias surface for build type discovery.
- `buildtype.com` and `buildtypes.com` – defensive domains that redirect to the `.dev` surfaces.

The project is still in the architecture and decision phase; no build type specifications, workflow
implementations, or provenance attestations have been published. Because no provenance has been
issued, there is no existing corpus to migrate, no verifier policy to update for legacy URIs, and no
need for a grace period or alias chain. Switching the `buildType` URI family now carries the lowest
possible cost and prevents a disruptive migration later.

The remaining question is how to assign roles to the two acquired domains and what exact URI shape
the project should adopt for profile `buildType` values going forward.

## Decision Drivers

- Separate the stable machine-readable `buildType` identifier from the human-readable documentation
  hosting location.
- Replace repository-path `buildType` URIs before any provenance is emitted, avoiding future
  migration or alias maintenance.
- Keep the `buildType` identifier stable and independent of GitHub repository layout, hosting
  provider, or future repository renames.
- Preserve a clear organizational and project namespace so Windlass can register additional build
  types under the same identifier domain without collision.
- Ensure `buildType` URIs still resolve to human-readable specifications, as recommended by SLSA
  provenance v1.
- Follow SLSA community conventions for build type URI shape (`buildtypes/<profile>/v<major>`) on
  the documentation side.
- Minimize operational complexity while achieving the above goals.

## Considered Options

- Keep the repository-path `buildType` URIs selected in ADR 0021 and ADR 0041.
- Use `slsa-builder.dev` as both the `buildType` identifier and the documentation host.
- Use `buildtype.dev` as both the `buildType` identifier and the documentation host.
- Use `buildtype.dev` as the `buildType` identifier and `slsa-builder.dev` as the documentation
  host, with redirects from the identifier domain to the documentation domain.
- Use `slsa-builder.dev` as the `buildType` identifier and `buildtype.dev` as a registry/index site.

## Decision Outcome

Chosen option: "Use `buildtype.dev` as the `buildType` identifier and `slsa-builder.dev` as the
documentation host, with redirects from the identifier domain to the documentation domain", because
it separates identifier stability from documentation presentation, decouples the build type
namespace from the project documentation site, and avoids migration cost by switching before any
provenance is published.

### Canonical `buildType` URI shape

The canonical `buildType` URI value that profile-owned reusable workflows must emit in provenance
predicates is:

```text
https://buildtype.dev/<org>/<project>/<profile>/v<major>
```

For the Windlass `slsa-builder` project, the concrete URIs are:

- JS/TS npm package profile:

  ```text
  https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1
  ```

- GitHub Release asset profile:

  ```text
  https://buildtype.dev/windlass/slsa-builder/github-release-asset/v1
  ```

The `<org>/<project>` segments identify the owner of the build type schema and isolate the namespace
from other Windlass projects or third parties. The `<profile>` segment identifies the artifact and
profile semantics. The `<major>` segment identifies the major version of the build type schema.
Backward-compatible schema clarifications and additional optional fields may remain on the same
major version. Breaking changes to meanings, required fields, verification expectations, or
supported build templates must use a new major version such as `/v2`.

### Documentation host and redirect behavior

The human-readable build type specification for each URI is hosted on `slsa-builder.dev` at:

```text
https://slsa-builder.dev/buildtypes/<profile>/v<major>
```

Concrete documentation URLs:

```text
https://slsa-builder.dev/buildtypes/js-ts-npm-package/v1
https://slsa-builder.dev/buildtypes/github-release-asset/v1
```

Requests to `https://buildtype.dev/<org>/<project>/<profile>/v<major>` must resolve through a
permanent redirect, preferably HTTP 308 or otherwise HTTP 301, to the corresponding
`https://slsa-builder.dev/buildtypes/<profile>/v<major>` documentation page. This lets
`buildtype.dev` act as a stable, opaque identifier namespace while still satisfying the SLSA
recommendation that `buildType` URIs resolve to human-readable specifications. Temporary redirects
such as HTTP 302 or HTTP 307 should not be used for canonical build type specification resolution
because they imply that the documentation target is not stable.

The adjacent domains are supporting surfaces, not separate `buildType` URI authorities for this
decision. `buildtypes.dev` may serve as a registry, index, or alias surface that points users and
tools to canonical `buildtype.dev` identifiers. It may redirect to `buildtype.dev` or host a hub UI
that lists known build type specifications. `buildtype.com` and `buildtypes.com` are defensive
registrations and should redirect to the corresponding `.dev` surfaces rather than emit canonical
Windlass `buildType` URIs.

Verifiers should compare the exact canonical `buildtype.dev` URI recorded in provenance. Redirects
exist for human-readable specification discovery and ecosystem navigation; they do not make redirect
targets or alias domains equivalent `buildType` identifiers.

### Superseded decisions

This decision supersedes the repository-path `buildType` URI choices in:

- ADR 0021 – Use Profile-Specific `buildType` URIs
- ADR 0041 – Use `github-release-asset` as the GitHub Release Asset `buildType` Profile URI

Those ADRs remain valid historical context but their chosen `buildType` URI values are no longer
authoritative. The existing repository-path URIs will not be retained as aliases because no
provenance has been issued and the cost of immediate replacement is zero.

### Consequences

- Good, because `buildType` identifiers are now stable domain-based URIs rather than repository-path
  URIs.
- Good, because the identifier namespace is decoupled from GitHub repository layout and hosting
  changes.
- Good, because `buildtype.dev` can host identifiers for other Windlass projects without mixing them
  into the `slsa-builder.dev` documentation site.
- Good, because adjacent domains can support registry discovery and defensive redirects without
  multiplying canonical `buildType` URI authorities.
- Good, because `slsa-builder.dev` remains a clean, branded project documentation site.
- Good, because redirecting from the identifier domain preserves the SLSA expectation that
  `buildType` URIs resolve to specifications.
- Good, because no provenance has been published, so the switch requires no legacy alias or
  migration period.
- Neutral, because two domains must be maintained instead of one.
- Neutral, because the URI is slightly longer than a single-domain project URI.
- Bad, because the redirect from `buildtype.dev` to `slsa-builder.dev` must be kept operational for
  the identifier resolution contract to hold.
- Bad, because the `<org>/<project>` prefix assumes Windlass will continue to use GitHub-style org
  names; a different organizational structure would require revisiting the namespace design.

### Confirmation

This decision is confirmed when:

- ADR 0021 and ADR 0041 are updated to `status: superseded by ADR-0042`.
- This ADR is added to `docs/decisions/` with `status: accepted`.
- Architecture specifications for the JS/TS npm package profile and GitHub Release asset profile
  list the new `buildtype.dev` URIs as their `buildType` values.
- Reusable workflow implementations emit the new `buildType` URIs in production provenance.
- Verifier documentation and example policies reference the new `buildType` URIs.
- `buildtype.dev` is configured to issue permanent HTTP 308 or HTTP 301 redirects from
  `/<org>/<project>/<profile>/v<major>` to the matching
  `slsa-builder.dev/buildtypes/<profile>/v<major>` documentation page.
- `buildtypes.dev`, `buildtype.com`, and `buildtypes.com` are documented or configured as registry,
  alias, or defensive redirect surfaces rather than canonical `buildType` URI namespaces.
- Tests assert that emitted provenance predicates use the new `buildType` URI format and do not
  reference the superseded repository-path URIs.

## Pros and Cons of the Options

### Keep the repository-path `buildType` URIs

Continue using `https://github.com/windlasstech/slsa-builder/buildtypes/<profile>/v<major>`.

- Good, because no new infrastructure or redirect configuration is required.
- Good, because existing ADRs remain authoritative without change.
- Bad, because the URI is tied to GitHub repository path and ownership; a repository rename or move
  would break or complicate the identifier.
- Bad, because it wastes the acquired domains and misses the opportunity to switch before any
  provenance is published.
- Bad, because it conflates source hosting with build type schema ownership.

### Use `slsa-builder.dev` as both identifier and documentation host

Use `https://slsa-builder.dev/buildtypes/<profile>/v<major>` for both `buildType` values and
specifications.

- Good, because it is a single domain to operate and explain.
- Good, because it strongly brands each build type with the project name.
- Good, because it matches the future-documentation-domain example in ADR 0021.
- Bad, because it does not leverage `buildtype.dev` for its intended purpose as a build type
  namespace.
- Bad, because the identifier namespace is tied to one project's documentation site, making
  organization-wide reuse harder.
- Bad, because a future Windlass project would need a separate domain or a subpath under
  `slsa-builder.dev`, which is semantically awkward.

### Use `buildtype.dev` as both identifier and documentation host

Use `https://buildtype.dev/<org>/<project>/<profile>/v<major>` for both identifiers and rendered
specifications.

- Good, because it keeps build type identifiers in a dedicated namespace.
- Good, because organizational and project scoping is clear.
- Bad, because the documentation site loses the `slsa-builder.dev` project branding and URL.
- Bad, because users following a `buildType` URI would land on a generic `buildtype.dev` page rather
  than the project documentation site.
- Bad, because it does not separate the concerns of identifier stability and human-readable
  documentation presentation as cleanly as the chosen option.

### Use `buildtype.dev` as identifier and `slsa-builder.dev` as documentation host

Use `https://buildtype.dev/<org>/<project>/<profile>/v<major>` as the canonical `buildType`
identifier and `https://slsa-builder.dev/buildtypes/<profile>/v<major>` as the human-readable
specification, with redirects from the former to the latter.

- Good, because it cleanly separates stable identifier namespace from project documentation hosting.
- Good, because `buildtype.dev` can scale to other Windlass projects under the same namespace
  convention.
- Good, because `slsa-builder.dev` remains the branded, user-facing documentation site.
- Good, because redirects preserve SLSA's expectation that `buildType` URIs resolve to
  specifications.
- Good, because the switch happens before any provenance is issued, eliminating migration cost.
- Neutral, because two domains must be configured and renewed.
- Bad, because redirect availability becomes part of the build type resolution contract.

### Use `slsa-builder.dev` as identifier and `buildtype.dev` as registry/index

Use `https://slsa-builder.dev/buildtypes/<profile>/v<major>` as the identifier and
`https://buildtype.dev/` as a cross-project build type registry or index.

- Good, because it brands the identifier with the project.
- Bad, because it ties the identifier to a single project documentation domain, reintroducing the
  coupling the dedicated `buildtype.dev` domain was meant to avoid.
- Bad, because a registry/index role for `buildtype.dev` is speculative and not currently needed.
- Bad, because it does not use `buildtype.dev` for its natural purpose as the identifier namespace.

## More Information

This decision supersedes the `buildType` URI choices in ADR 0021 and ADR 0041. Those ADRs retain
their reasoning about why `buildType` should be separate from `builder.id`, why the profile segment
should name artifact semantics rather than workflow filenames, and why major-versioned schema URIs
are preferable to tag-churning identities. Only the URI authority and namespace shape change here.

Reference points considered:

- SLSA provenance v1 requires `buildDefinition.buildType` and recommends that the URI resolve to a
  human-readable specification with parameter schemas, initiation instructions, and a complete
  example.
- SLSA provenance v1 treats `buildType` as the schema identity for `externalParameters`,
  `internalParameters`, and `resolvedDependencies`.
- ADR 0021 separated `builder.id` (released reusable workflow identity) from `buildType` (profile
  schema identity) and explicitly noted a future move to a stable documentation domain.
- ADR 0041 applied the ADR 0021 convention to the GitHub Release asset profile and likewise left
  open a future documentation-domain move.
- No provenance, verifier policy, or published specification currently references the superseded
  repository-path URIs, making immediate replacement cost-free.

Future decisions may define:

- the exact repository-local file path for build type specifications before they are published to
  `slsa-builder.dev`;
- the redirect and hosting mechanics for `buildtype.dev` (e.g., static site generator, edge redirect
  rules, DNS configuration);
- the registry, index, alias, or defensive redirect behavior for `buildtypes.dev`, `buildtype.com`,
  and `buildtypes.com`;
- whether future Windlass projects follow the same `https://buildtype.dev/<org>/<project>/...`
  convention;
- how to version and publish build type specifications alongside project releases.
