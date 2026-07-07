---
parent: Decisions
nav_order: 34
status: accepted
date: 12026-07-05
decision-makers: Yunseo Kim
---

# Do Not Support Private Dependency Credentials in the Initial Profile

## Context and Problem Statement

ADR 0024 selected npm trusted publishing with GitHub Actions OIDC and no npm publish secret for the
initial production JS/TS npm package SLSA3 profile. That decision intentionally left private
dependency installation credentials undecided. npm trusted publishing authenticates `npm publish`;
it does not authenticate `npm install`, `npm ci`, `pnpm install`, or `yarn install` when those
commands need to fetch private packages.

The npm ecosystem commonly handles private dependency installation in CI with read-only granular
access tokens. npm documentation recommends trusted publishing for publish operations, while also
recommending read-only granular tokens when CI needs to install private npm packages. SLSA v1.2 does
not require hermetic builds, and dependency fetching during a build is allowed. However, the
production SLSA3 builder identity selected for this project has a small, verifier-oriented public
contract: workflow inputs are intentionally constrained, publish tokens are forbidden, and modes
with materially different security properties require distinct builder identities.

Allowing dependency-fetch credentials inside the initial reusable workflow would add a
secret-bearing mode to the same production profile. Even if the credential is intended to be
read-only, install-time lifecycle scripts and package-manager behavior can observe the install
environment. The workflow also cannot prove every caller-provided token is read-only, short-lived,
registry-scoped, or unable to publish. Private dependency usage can also make downstream verifier
checks less reproducible because dependency metadata or tarballs may not be publicly resolvable.

Should the initial production JS/TS npm package SLSA3 profile support read-only private dependency
credentials, reject them entirely, or split private-dependency support into a future distinct
profile or builder mode?

## Decision Drivers

- Preserve ADR 0024's no-publish-secret invariant and avoid confusing dependency-fetch credentials
  with publish credentials.
- Keep the initial production profile's `workflow_call.secrets` contract empty.
- Preserve a small and easily verifiable `externalParameters` and secret surface for the initial
  SLSA3 builder identity.
- Avoid exposing long-lived or caller-controlled credentials to install-time package lifecycle
  scripts in the initial production profile.
- Keep private dependency and private registry semantics out of the first npmjs-focused production
  compatibility surface.
- Align with SLSA v1.2's guidance that `externalParameters` should be complete and minimized, and
  that different security modes should be distinguishable through builder identity.
- Leave a deliberate future path for read-only dependency credential support under a distinct trust
  contract if needed.

## Considered Options

- Do not support private dependency credentials in the initial production profile, and define a
  future path for a distinct credential-bearing profile or mode.
- Allow one optional read-only npm dependency token secret in the initial production profile.
- Allow registry- or scope-specific read-only credential maps for npm-compatible registries.
- Support only OIDC or brokered short-lived credentials for dependency reads.
- Allow `secrets: inherit` or arbitrary environment passthrough for dependency installation.

## Decision Outcome

Chosen option: "Do not support private dependency credentials in the initial production profile, and
define a future path for a distinct credential-bearing profile or mode", because it keeps the first
production SLSA3 builder identity small, tokenless, and straightforward to verify while leaving room
for a separately specified private-dependency mode later.

The initial production JS/TS npm package SLSA3 profile should not define any `workflow_call.secrets`
for dependency installation. It should not accept `npm-read-token`, `NPM_READ_TOKEN`, `NPM_TOKEN`,
`NODE_AUTH_TOKEN`, GitHub Packages tokens, registry tokens, cloud credentials, `.npmrc` secret
contents, `.yarnrc.yml` secret contents, or equivalent dependency-fetch credentials through the
production reusable workflow contract.

The profile should not support `secrets: inherit`, arbitrary environment passthrough, arbitrary
registry credential maps, or caller-provided package-manager auth configuration as part of the
initial production SLSA3 profile. If the selected package's install step requires private dependency
authentication, the workflow should fail clearly rather than prompting users to pass a token,
falling back to a publish token, or silently changing the builder security mode.

This decision does not claim that private dependency credentials are incompatible with SLSA in
general. SLSA v1.2 does not require hermetic builds, and read-only dependency credentials can be a
reasonable CI practice when they are scoped, protected, and documented. The decision is narrower:
the initial Windlass production JS/TS npm package profile will not include that secret-bearing
behavior under the same builder identity.

A future ADR may add private dependency support through a separate reusable workflow, separate mode,
or revised profile contract. If the future mode exposes dependency-fetch credentials to install-time
commands, it should use a distinct builder identity when its security properties, verifier
expectations, or secret exposure differ materially from the initial tokenless profile. That future
decision should define at least:

- which registries and package managers are supported;
- whether credentials are long-lived read-only tokens, OIDC-exchanged short-lived tokens, or
  brokered credentials;
- how credential names avoid confusion with publish credentials;
- how credentials are scoped to install only and removed before build, pack, provenance generation,
  and publish stages where possible;
- what credential mode, registry scope, and dependency-fetch policy are recorded in provenance or
  byproducts without recording secret values;
- how verifier policy can accept or reject private-dependency credential mode;
- how package-manager lifecycle scripts, logs, caches, and artifact handoff avoid leaking secrets.

### Consequences

- Good, because the initial production profile keeps an empty `workflow_call.secrets` surface.
- Good, because dependency-fetch credentials cannot be confused with publish credentials in the
  initial reusable workflow contract.
- Good, because install-time lifecycle scripts cannot access caller-provided registry tokens through
  this profile.
- Good, because verifier expectations for the first builder identity remain smaller and easier to
  document.
- Good, because future private-dependency support can receive its own ADR, tests, documentation, and
  possibly distinct builder identity.
- Neutral, because SLSA itself permits non-hermetic dependency fetching and best-effort dependency
  recording; this decision is a product-scope constraint rather than a SLSA prohibition.
- Bad, because packages that require private dependency authentication cannot use the initial
  production profile without changing their dependency model.
- Bad, because some users may need to wait for a future credential-bearing profile or publish
  through a less constrained workflow outside the initial SLSA3 profile.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification,
implementation, documentation, and verifier guidance define:

- no `workflow_call.secrets` entries for dependency installation credentials;
- no accepted `npm-read-token`, `NPM_READ_TOKEN`, `NPM_TOKEN`, `NODE_AUTH_TOKEN`, GitHub Packages
  token, registry token, cloud credential, `.npmrc` secret, `.yarnrc.yml` secret, or equivalent
  dependency-fetch credential in the production profile;
- no `secrets: inherit`, arbitrary environment passthrough, or arbitrary registry credential map in
  caller examples or supported workflow contract;
- clear failure behavior when dependency installation requires credentials not available through the
  selected public dependency configuration;
- documentation that private dependency installation is unsupported in the initial production
  profile and may require a future distinct profile or mode;
- verifier guidance that does not treat private dependency credential behavior as part of the
  initial tokenless `buildType` contract.

Implementation review should verify that dependency installation cannot receive caller-provided
secrets through the production SLSA3 workflow API and that no implementation path falls back to an
npm publish token or inherited secret for installing private dependencies.

## Pros and Cons of the Options

### Do not support private dependency credentials in the initial production profile

The production reusable workflow exposes no dependency credential secrets. Packages whose install
stage requires private dependency authentication fail clearly under the initial profile.

- Good, because the secret surface stays empty and easy to audit.
- Good, because the initial profile remains aligned with ADR 0024's tokenless publish direction.
- Good, because verifier policy does not need to reason about secret-bearing dependency-fetch modes.
- Good, because a future credential-bearing mode can be isolated under a distinct trust contract.
- Bad, because private-dependency packages are outside the first production compatibility surface.
- Bad, because adoption is harder for organizations with internal npm packages.

### Allow one optional read-only npm dependency token secret

The profile accepts a single optional secret intended only for installing private npm dependencies.

- Good, because it matches npm's documented CI guidance for private dependencies.
- Good, because many real npm packages could adopt the profile without changing dependency hosting.
- Good, because publish authentication can remain OIDC-only with no publish token fallback.
- Bad, because install-time scripts and package-manager behavior can observe the token environment.
- Bad, because the workflow cannot fully prove the token is read-only, short-lived, or narrowly
  scoped.
- Bad, because the same builder identity would cover both tokenless and secret-bearing install
  behavior unless split by a later decision.

### Allow registry- or scope-specific read-only credential maps

The profile supports multiple private registries or scopes through explicit credential mappings.

- Good, because it fits enterprise npm, GitHub Packages, Artifactory, Verdaccio, and mixed-scope
  dependency layouts.
- Good, because registry-specific policy can be represented more precisely than a single token.
- Bad, because initial public API and verifier expectations become much larger.
- Bad, because custom registry semantics are outside ADR 0030's guaranteed npmjs production surface.
- Bad, because multiple credentials increase leakage and misconfiguration risks.

### Support only OIDC or brokered short-lived dependency credentials

The profile supports private dependency reads only through OIDC exchange or a credential broker
rather than long-lived read tokens.

- Good, because it follows the same short-lived credential direction as trusted publishing.
- Good, because it can reduce long-lived secret exposure if registry support exists.
- Bad, because npm private dependency install does not have the same simple trusted-publishing OIDC
  path as `npm publish`.
- Bad, because registry support and broker design would require a larger separate architecture
  decision.
- Bad, because it is too broad for the initial profile's first production specification.

### Allow `secrets: inherit` or arbitrary environment passthrough

The caller can pass all secrets or arbitrary environment variables into the reusable workflow so its
normal install commands can access whatever credentials the caller uses.

- Good, because it is highly flexible for existing workflows.
- Good, because it can support almost any private dependency setup.
- Bad, because the workflow cannot enumerate or verify all external influences on the build.
- Bad, because publish tokens, cloud tokens, and unrelated secrets may enter the build environment.
- Bad, because this conflicts with the small input and secret surface required for the initial
  production SLSA3 profile.

## More Information

This decision follows ADR 0024 and decides only private dependency installation credentials for the
initial production JS/TS npm package profile. It does not change publish authentication,
Windlass-generated provenance, supported package managers, package-manager install command policy,
registry URL scope, release trigger policy, or SHA-pinned builder identity.

Reference points considered:

- SLSA v1.2 Build Requirements require security best practices for secrets and require SLSA Build L3
  provenance to be strongly resistant to tenant forgery. Secret material used for provenance
  authenticity must not be accessible to user-defined build steps.
- SLSA v1.2 Build Requirements do not require hermetic builds. Dependency fetching is allowed, and
  resolved dependency completeness is best effort at Build L3.
- SLSA v1.2 Build Provenance defines `externalParameters` as the externally controlled build
  interface, requires those parameters to be complete at Build L3, and recommends minimizing their
  size and complexity for verifier usability.
- SLSA v1.2 Build Provenance says modes with materially different security attributes or SLSA Build
  levels must have different `builder.id` values and should have different signer identities.
- npm trusted publishing documentation recommends trusted publishing for publish operations while
  recommending read-only granular access tokens when CI needs to install private npm packages.
- npm documentation notes that trusted publishing applies to publish operations; private dependency
  install commands still require traditional authentication when private packages are used.
- GitHub Actions documentation notes that secrets are not automatically passed to reusable
  workflows, that callers can pass named secrets or use `secrets: inherit`, and that commit SHA
  references are safest for reusable workflows.
- The SLSA GitHub Generator Node.js builder exposes inputs such as `run-scripts` but does not define
  a dependency-read token secret on its reusable workflow. Its publish examples pass publish tokens
  in a separate publishing path rather than making private dependency credentials part of the
  builder contract.
