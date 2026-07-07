---
parent: Decisions
nav_order: 54
status: accepted
date: 12026-07-08
decision-makers: Yunseo Kim
---

# Use slsa-builder.dev for the Release Manifest Predicate URI

## Context and Problem Statement

ADR 0031 selected a Windlass-defined release manifest carried in an in-toto Statement and signed as
a Sigstore DSSE bundle. ADR 0053 later selected a three-job signing boundary for producing and
publishing that signed release manifest. Together, those decisions require the architecture
specification to define the exact release manifest `predicateType` URI that verifiers compare when
they authenticate release metadata.

ADR 0042 selected `buildtype.dev` as the canonical identifier namespace for Windlass `buildType`
values and `slsa-builder.dev` as the human-readable documentation host. During specification review,
the draft release manifest specification used a URI shaped like a `buildType` value for the release
manifest predicate:

```text
https://buildtype.dev/windlass/slsa-builder/release-manifest/v1
```

That shape risks conflating two different verifier concepts. Producer profile `buildType` values
identify source-to-artifact build templates. The release manifest predicate identifies Windlass
release metadata that maps release versions to trusted workflow SHAs, `builder.id` values, and
producer `buildType` values. The project needs a stable predicate URI that keeps this release
metadata contract separate from the `buildType` namespace.

What URI should the release manifest use as its in-toto Statement `predicateType`?

## Decision Drivers

- Preserve ADR 0031's model of a Windlass-owned release metadata predicate.
- Avoid treating the release manifest as a producer profile `buildType`.
- Keep `buildtype.dev` reserved for canonical `buildType` identifiers selected by ADR 0042.
- Use a URI that can resolve to human-readable project documentation.
- Keep the initial predicate namespace small and operationally simple.
- Leave room for future project-owned predicates without requiring a new domain or registry.
- Make verifier policy easy to implement by comparing one exact predicate URI.

## Considered Options

- Use `https://slsa-builder.dev/predicates/release-manifest/v1`.
- Use a dedicated predicate identifier domain, such as
  `https://predicate.dev/windlass/slsa-builder/release-manifest/v1`.
- Use a GitHub repository URI, such as
  `https://github.com/windlasstech/slsa-builder/predicate/release-manifest/v1`.
- Continue using `https://buildtype.dev/windlass/slsa-builder/release-manifest/v1`.

## Decision Outcome

Chosen option: "Use `https://slsa-builder.dev/predicates/release-manifest/v1`", because it clearly
separates the release manifest predicate from the `buildtype.dev` `buildType` namespace while using
the project's documentation domain as a stable, human-readable authority.

The release manifest in-toto Statement must use this exact `predicateType`:

```text
https://slsa-builder.dev/predicates/release-manifest/v1
```

Verifiers must compare this URI exactly. Redirects, aliases, GitHub repository URLs, and
`buildtype.dev` URLs are not equivalent predicate identifiers.

The documentation page for this predicate should live at the same URI or an equivalent rendered page
under `slsa-builder.dev`. That documentation should define the predicate schema, canonicalization
rules, subject digest rules, signer identity expectations, release manifest schema version, and
verification policy.

The `buildtype.dev` namespace remains reserved for producer `buildType` identifiers. A release
manifest verifier must not treat `https://slsa-builder.dev/predicates/release-manifest/v1` as a
producer `buildType`, and a producer provenance verifier must not accept release manifest predicate
URIs as profile `buildType` values.

Future Windlass predicates for this project may use the same project-local predicate namespace:

```text
https://slsa-builder.dev/predicates/<predicate-name>/v<major>
```

If Windlass later needs an organization-wide predicate registry or a dedicated predicate identifier
domain, that change should be recorded in a new ADR. Such a migration must preserve existing
predicate verifier behavior or explicitly define compatibility and deprecation rules.

### Consequences

- Good, because release manifest metadata is no longer shaped like a producer `buildType`.
- Good, because `buildtype.dev` retains one clear role as the canonical `buildType` identifier
  namespace.
- Good, because the selected URI can resolve directly to project documentation.
- Good, because verifier policy can check a single exact predicate URI without consulting redirect
  or alias equivalence rules.
- Good, because the URI remains stable across GitHub repository path changes.
- Neutral, because `slsa-builder.dev` now serves as both the project documentation host and the
  release manifest predicate authority.
- Neutral, because future non-`slsa-builder` Windlass predicates may still need a broader namespace
  decision.
- Bad, because this diverges from the GitHub repository URI example shown in ADR 0031.
- Bad, because predicate URI documentation must be hosted and maintained under `slsa-builder.dev`.

### Confirmation

This decision is confirmed when architecture specifications, workflow implementations, tests, and
verifier guidance define:

- `https://slsa-builder.dev/predicates/release-manifest/v1` as the exact release manifest
  `predicateType`;
- rejection of `https://buildtype.dev/windlass/slsa-builder/release-manifest/v1` as a release
  manifest `predicateType`;
- rejection of release manifest predicate URIs when a producer profile `buildType` is expected;
- release manifest reference commands and fixtures using the selected predicate URI;
- documentation under `slsa-builder.dev` that explains the release manifest predicate schema and
  verifier expectations.

Implementation review should verify that no release manifest workflow, fixture, verifier policy, or
documentation example treats `buildtype.dev` as the release manifest predicate namespace.

## Pros and Cons of the Options

### Use `https://slsa-builder.dev/predicates/release-manifest/v1`

The project documentation domain hosts and identifies the release manifest predicate.

- Good, because it separates release metadata predicates from producer `buildType` values.
- Good, because the URI is project-specific and can resolve to project documentation.
- Good, because no new domain or registry is required.
- Good, because it is stable across GitHub repository moves or renames.
- Neutral, because the documentation host also becomes an identifier authority for this predicate.
- Bad, because a later organization-wide predicate strategy may require additional namespace design.

### Use a dedicated predicate identifier domain

Use a domain dedicated to custom predicate identifiers, such as
`https://predicate.dev/windlass/slsa-builder/release-manifest/v1`.

- Good, because `buildType`, documentation, and predicate identifiers would each have distinct
  namespaces.
- Good, because an organization-wide predicate registry could scale across projects.
- Bad, because it requires acquiring, operating, and documenting another namespace before the
  project has more than one custom predicate.
- Bad, because the ADR would expand from a release manifest predicate decision into a broader
  predicate registry strategy.

### Use a GitHub repository URI

Use a URI under the source repository, such as
`https://github.com/windlasstech/slsa-builder/predicate/release-manifest/v1`.

- Good, because it is close to the example URI in ADR 0031.
- Good, because the repository owner is visible in the identifier.
- Good, because no separate documentation-domain routing is required to explain ownership.
- Bad, because the identifier is tied to GitHub repository layout and ownership.
- Bad, because it may not resolve to a useful rendered specification page.
- Bad, because it is less aligned with ADR 0042's move away from repository-path identifiers for
  stable verifier-visible URIs.

### Continue using the `buildtype.dev` URI

Use `https://buildtype.dev/windlass/slsa-builder/release-manifest/v1` as the release manifest
predicate URI.

- Good, because it requires the smallest edit to the current draft specification.
- Good, because it reuses an already selected domain namespace.
- Bad, because it makes release manifest metadata look like a producer `buildType`.
- Bad, because it weakens ADR 0042's clean separation between `buildType` identifiers and project
  documentation.
- Bad, because verifier implementers could confuse predicate allowlists with producer `buildType`
  allowlists.

## More Information

This decision follows ADR 0031, ADR 0042, and ADR 0053. It decides only the release manifest
predicate URI. It does not change the release manifest JSON schema, signing workflow boundary,
Sigstore signer identity, release manifest artifact names, producer profile `buildType` values, or
the signed release manifest's role as the canonical version-to-SHA trust metadata.

Reference points considered:

- SLSA provenance v1 uses `buildType` to identify the parameterized build template for a producer
  profile.
- in-toto Statements use `predicateType` to identify the schema and semantics of the Statement
  predicate.
- The Windlass release manifest is release metadata, not source-to-artifact build provenance.
- ADR 0042 reserves `buildtype.dev` for canonical Windlass `buildType` identifiers and uses
  `slsa-builder.dev` as the project documentation host.
