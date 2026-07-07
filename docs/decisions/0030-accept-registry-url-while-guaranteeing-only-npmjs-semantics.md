---
parent: Decisions
nav_order: 30
status: accepted
date: 12026-07-05
decision-makers: Yunseo Kim
---

# Accept Registry URL While Guaranteeing Only npmjs Semantics

## Context and Problem Statement

ADR 0013 scoped the initial JS/TS profile to npm package releases, ADR 0024 selected OIDC trusted
publishing without publish secrets, and ADR 0029 selected Windlass-generated SLSA provenance
submitted with `npm publish --provenance-file` as the canonical production provenance path.

The remaining registry-scope question is how the profile should treat its npm registry input. The
npm CLI can publish to the public npm registry by default, and it can also be configured with a
different registry URL or a scope-specific registry. However, npm trusted publishing, npm registry
provenance linkage, and `provenance-file` behavior are ecosystem features whose production semantics
are documented for npmjs.com, not as a portable guarantee for every npm-compatible registry API.

The profile therefore needs to decide whether it should hard-code `registry.npmjs.org`, accept any
npm-compatible registry URL, support token fallback for registries that do not implement OIDC, or
split registry families into separate profiles. The choice affects the public `workflow_call` input
contract, the provenance subject and registry linkage rules, verifier expectations, and whether the
same production SLSA3 builder identity would accidentally imply support for registries whose
authentication and provenance behavior Windlass has not validated.

Should the production JS/TS npm package SLSA3 profile accept configurable registry URLs, and if so,
which registry behaviors does Windlass guarantee?

## Decision Drivers

- Preserve ADR 0024's no-publish-secret invariant and avoid token fallback in the production SLSA3
  profile.
- Preserve ADR 0029's canonical `npm publish --provenance-file` submission path and npm registry
  provenance linkage expectations.
- Avoid claiming production compatibility for npm-compatible registries whose OIDC authentication,
  provenance-file acceptance, digest semantics, or provenance discovery behavior Windlass has not
  validated.
- Keep the workflow API flexible enough for users to test compatible registries without requiring a
  new input shape later.
- Make `https://registry.npmjs.org/` the only registry with guaranteed production semantics for the
  initial profile.
- Keep non-npm package registries, including JSR, outside the initial npm package profile while
  leaving room for future dedicated profiles.
- Avoid overloading one `builder.id` and `buildType` with materially different package ecosystem
  semantics.

## Considered Options

- Accept a configurable npm registry URL, guarantee only npmjs production semantics, and never fall
  back to token-based publishing.
- Require `https://registry.npmjs.org/` and reject every other registry URL.
- Accept arbitrary npm-compatible registry URLs and claim production support for all of them.
- Accept arbitrary npm-compatible registry URLs and use token fallback for registries without OIDC
  trusted publishing.
- Add JSR and other non-npm registries to the initial JS/TS package profile.

## Decision Outcome

Chosen option: "Accept a configurable npm registry URL, guarantee only npmjs production semantics,
and never fall back to token-based publishing", because this preserves the no-secret production
security model while avoiding an unnecessary hard block on users who want to validate npm-compatible
custom registries themselves.

The production JS/TS npm package SLSA3 profile should expose or preserve a registry URL input for
npm publishing. The officially guaranteed production target for that input is
`https://registry.npmjs.org/`. The profile may attempt to publish to another npm-compatible registry
URL when a caller explicitly configures one, but that configuration is outside Windlass's guaranteed
production compatibility surface unless a later ADR and specification add support for that registry
class.

The profile must not use `NPM_TOKEN`, `NODE_AUTH_TOKEN`, OTP, or any other publish-capable token as
a fallback when a non-npmjs registry cannot complete OIDC trusted publishing, external
provenance-file submission, or provenance linkage. A custom registry path must either complete the
same tokenless publish and provenance submission flow or fail clearly.

Documentation for the registry URL input should state that custom npm-compatible registries are
unsupported-but-not-blocked. Users who choose such registries are responsible for validating that
the registry supports all required semantics for their use case, including:

- OIDC or equivalent tokenless publish authentication compatible with the profile's
  no-publish-secret contract;
- `npm publish --provenance-file` or equivalent external provenance bundle submission behavior;
- stable npm package identity, version, tarball URL, and digest semantics compatible with the
  profile's provenance subject rules;
- discoverable registry metadata that can link the package version to the submitted provenance
  bundle;
- verifier-accessible package and provenance data after publication.

Windlass verifier guidance for the initial production profile should define normative registry
linkage checks for npmjs.com. For non-npmjs npm-compatible registries, verifier behavior is
registry-specific unless a later ADR defines a supported registry class and its verification
contract.

JSR and other non-npm package registries are out of scope for the initial JS/TS npm package profile.
They may be supported later through separate profiles, build types, provenance subject rules,
publication flows, and verifier policies. Their lower priority for the initial implementation should
not be interpreted as a rejection of future support; it only keeps the first production profile
focused on npm package publication.

### Consequences

- Good, because the profile remains flexible for callers that want to experiment with npm-compatible
  custom registries.
- Good, because npmjs.com remains the only officially guaranteed production registry target for the
  initial profile.
- Good, because the no-publish-secret and no-token-fallback invariant from ADR 0024 remains intact.
- Good, because Windlass avoids claiming SLSA3 registry compatibility for ecosystems it has not
  specified or validated.
- Good, because custom registry behavior can be added later through a narrower support contract
  rather than by weakening the initial profile.
- Good, because JSR and other non-npm registries can receive dedicated profiles instead of being
  forced into an npm-shaped build type.
- Neutral, because users may still see a `registry-url` input and assume broader support unless
  documentation is explicit.
- Bad, because custom registry users must validate compatibility themselves before relying on the
  profile.
- Bad, because implementation and documentation must distinguish "accepted input" from "guaranteed
  production support".

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification,
implementation, release documentation, and verifier guidance define:

- a registry URL input or equivalent npm registry configuration surface;
- `https://registry.npmjs.org/` as the only officially guaranteed production registry target;
- no token-based publish fallback for any registry URL in the production SLSA3 profile;
- clear failure behavior when OIDC trusted publishing, external provenance-file submission, package
  digest resolution, or provenance linkage is unavailable;
- documentation that custom npm-compatible registries are unsupported-but-not-blocked and require
  user validation;
- normative npmjs.com registry linkage checks for package identity, tarball digest, and provenance
  metadata;
- registry-specific verifier guidance as future work for any custom registry class that becomes
  officially supported;
- JSR and other non-npm registries as out of scope for the initial npm profile and candidates for
  later dedicated profiles.

Implementation review should verify that selecting a custom registry cannot make the production
profile use publish tokens, silently omit provenance, silently switch to npm automatic provenance,
skip provenance registry linkage checks without documenting the weaker support level, or claim npmjs
verification semantics for a registry that Windlass has not specified.

## Pros and Cons of the Options

### Accept registry URL, guarantee only npmjs, and forbid token fallback

The profile accepts a configurable npm registry URL, officially supports npmjs.com production
semantics, and requires all registry choices to use the same no-publish-secret production flow.

- Good, because it keeps the workflow input flexible without over-claiming compatibility.
- Good, because npmjs.com remains the documented and tested production path.
- Good, because security properties stay stable across registry choices: no publish secret and no
  token fallback.
- Good, because custom registry users can test their registry-specific behavior without Windlass
  blessing it as supported.
- Bad, because documentation must be precise to avoid support ambiguity.
- Bad, because custom registries may fail at publish or verification time if they do not implement
  compatible OIDC, provenance-file, or metadata behavior.

### Require npmjs and reject every other registry URL

The profile hard-codes or validates the registry URL to `https://registry.npmjs.org/`.

- Good, because the support boundary is unambiguous.
- Good, because verifier semantics can be fully npmjs-specific.
- Good, because unsupported custom registry failures happen early.
- Bad, because users cannot experiment with registries that may already support compatible behavior.
- Bad, because a future custom-registry profile may need to introduce a new API surface that could
  have been preserved safely from the start.

### Accept arbitrary npm-compatible registries as officially supported

The profile treats any npm-compatible registry URL as part of the production support contract.

- Good, because it maximizes apparent compatibility.
- Good, because it aligns with npm CLI's general ability to target alternate registries.
- Bad, because npm-compatible package APIs do not imply compatible OIDC trusted publishing,
  provenance-file submission, provenance storage, or verifier discovery semantics.
- Bad, because Windlass would be claiming support for behavior it has not specified, tested, or
  secured.
- Bad, because registry-specific behavior could weaken or confuse the same `builder.id` and
  `buildType` trust contract.

### Accept custom registries with token fallback

The profile publishes to npmjs with OIDC but falls back to publish tokens for registries that do not
support tokenless publishing.

- Good, because it matches many existing private registry workflows.
- Good, because it would make custom registries more likely to work operationally.
- Bad, because long-lived publish credentials re-enter the production workflow boundary.
- Bad, because it conflicts with ADR 0024's OIDC-only publish authentication decision.
- Bad, because token and OIDC publish modes have materially different security properties under the
  same production profile unless split by a later ADR and distinct identity.

### Add JSR and other non-npm registries to the initial profile

The initial JS/TS package profile supports npm plus other JavaScript/TypeScript package registries.

- Good, because it acknowledges the broader JS/TS package ecosystem.
- Good, because JSR and similar registries may have modern provenance or OIDC-oriented publishing
  models that deserve support.
- Bad, because non-npm registries have different package identity, publication, provenance, and
  verifier semantics.
- Bad, because combining multiple registry families would enlarge the initial profile and dilute its
  npm-specific `buildType` contract.
- Bad, because npm remains the higher-priority initial ecosystem for this project.

## More Information

This decision follows ADR 0024's publish authentication decision and ADR 0029's provenance
submission decision. It decides only registry URL scope for the initial production JS/TS npm package
profile. It does not decide private dependency registry authentication, staged publishing,
lower-assurance token-based profiles, registry mirrors used only for dependency installation, or
future JSR support.

Reference points considered:

- npm publish defaults to the public npm registry but can be configured with another registry URL or
  scope-specific registry.
- npm trusted publishing is documented for npmjs.com package settings and supported cloud CI/CD
  providers, with OIDC replacing long-lived publish tokens.
- npm provenance and trusted publishing have public package, public repository, supported CI, and
  registry-specific limitations.
- npm publish supports `provenance-file` for submitting an externally generated provenance bundle
  and treats it as mutually exclusive with automatic `provenance` generation.
- Windlass organization policy prefers OIDC over long-lived secrets and targets signed provenance
  for production releases.
- SLSA verification requires checking artifact subject digest, provenance signature,
  `predicateType`, trusted `builder.id`, `buildType`, and expected external parameters; registry
  linkage is ecosystem-specific evidence that must be specified before it can be guaranteed.
