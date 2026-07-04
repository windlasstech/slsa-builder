---
parent: Decisions
nav_order: 24
status: accepted
date: 12026-07-01
decision-makers: Yunseo Kim
---

# Use OIDC Trusted Publishing Without Publish Secrets

## Context and Problem Statement

ADR 0002 chose an extensible trusted reusable workflow foundation, ADR 0003 refined it into
profile-owned reusable workflows, ADR 0013 scoped the initial JS/TS profile to npm registry
packages, and ADR 0022 selected `.github/workflows/js-ts-npm-package-slsa3.yml` as the initial JS/TS
npm package profile entrypoint. ADR 0023 kept authentication, trusted publishing, and
provenance-file behavior outside the package selection input decision.

The next public-contract decision is how callers authenticate the npm publish operation. The profile
can require a long-lived npm token secret, rely on npm trusted publishing with GitHub Actions OIDC,
support both, or split the modes. The authentication choice affects the reusable workflow's public
`workflow_call.secrets` contract, the caller workflow permissions, the npm package setup
requirements, the provenance security story, and whether the resulting builder mode can credibly
target SLSA Build L3.

Should the initial JS/TS npm package profile require an npm publish secret, support token fallback,
or publish only through npm trusted publishing with OIDC?

## Decision Drivers

- Prefer short-lived, workflow-scoped publish credentials over long-lived npm write tokens.
- Keep the profile's `workflow_call.secrets` surface small and avoid required publish secrets.
- Align with SLSA Build L3 expectations that secret material used for provenance or platform trust
  is not exposed to tenant-controlled build steps.
- Align with Windlass workflow hardening policy, which prefers OIDC, least-privilege permissions,
  hosted ephemeral runners, and clear release artifact provenance.
- Preserve ADR 0020's requirement that modes with materially different security properties use
  distinct builder identities.
- Keep npm package provenance generation compatible with npm's supported trusted-publishing path.
- Make reusable workflow caller requirements explicit, including GitHub Actions `id-token: write`
  and npm trusted publisher setup.
- Keep private dependency installation concerns separate from publish authentication.

## Considered Options

- Use OIDC trusted publishing only, with no publish secret.
- Use OIDC trusted publishing by default with an optional npm token fallback secret.
- Require an npm token publish secret and do not require trusted publishing.
- Provide separate OIDC and token publishing workflow modes.

## Decision Outcome

Chosen option: "Use OIDC trusted publishing only, with no publish secret", because it removes
long-lived npm publish credentials from the reusable workflow contract while matching npm's
recommended trusted publishing path and the profile's SLSA Build L3 target.

The initial JS/TS npm package profile should not define a required or optional `workflow_call`
secret for npm publish authentication. In particular, it should not define `npm-token`, `NPM_TOKEN`,
`NODE_AUTH_TOKEN`, `otp`, or equivalent publish credentials as part of the production SLSA3 profile
contract.

Caller workflows should publish through npm trusted publishing with GitHub Actions OIDC. The caller
job that invokes the reusable workflow must grant the permissions needed for the called workflow to
obtain an OIDC token, at minimum:

```yaml
permissions:
  contents: read
  id-token: write
```

The called reusable workflow should also request only the permissions it needs, including
`id-token: write` for npm trusted publishing. The profile should run on GitHub-hosted runners for
the production SLSA3 mode unless a later ADR defines a different trusted runner boundary.

Each package published through the profile must have npm trusted publishing configured for the
caller repository and caller workflow filename. When a caller workflow invokes the reusable profile
through `workflow_call`, npm trusted publishing validates the calling workflow identity rather than
only the called reusable workflow file that contains the `npm publish` command. Therefore, profile
documentation must tell users to configure npm's trusted publisher workflow filename to the consumer
repository's release workflow file, not to `.github/workflows/js-ts-npm-package-slsa3.yml` in this
repository.

The profile should use npm CLI and Node.js versions that satisfy npm trusted publishing
requirements. The profile should fail clearly when OIDC trusted publishing is unavailable or
misconfigured rather than falling back to token-based publish inside the same SLSA3 builder
identity.

Private dependency installation is separate from publish authentication. If the profile later
supports credentials for installing private dependencies, those credentials should be read-only,
should not authorize npm publish, and should be documented as dependency-fetch credentials rather
than publish credentials.

### Consequences

- Good, because the production profile has no long-lived npm write token to leak, rotate, scope, or
  pass through reusable workflow secrets.
- Good, because publish authentication uses short-lived credentials scoped to the workflow identity.
- Good, because the caller-facing secrets contract stays empty for publish authentication.
- Good, because npm trusted publishing can automatically generate npm provenance attestations for
  supported public packages from supported public repositories.
- Good, because token-based publish cannot silently downgrade the security properties of the same
  `builder.id`.
- Neutral, because every package needs npm trusted publisher setup before the first publish.
- Neutral, because private dependency installation may still need a separate read-only credential
  decision.
- Bad, because packages or repositories that cannot use npm trusted publishing are outside the
  initial production SLSA3 profile.
- Bad, because reusable workflow users must understand npm's caller-workflow filename validation and
  configure npm package settings accordingly.

### Confirmation

This decision is confirmed when the JS/TS npm package profile architecture specification and
implementation define:

- no `workflow_call.secrets` entry for npm publish authentication;
- no accepted publish token, OTP, or token fallback path in the production SLSA3 profile;
- caller workflow permission requirements including `contents: read` and `id-token: write`;
- called workflow permission requirements including `id-token: write`;
- GitHub-hosted runner requirements for the production trusted publishing mode;
- npm trusted publisher setup instructions that identify the caller repository and caller workflow
  filename;
- clear failures for missing OIDC permission, unsupported runner context, unsupported npm CLI or
  Node.js version, and npm trusted publisher mismatch;
- separate treatment of any future private dependency read token from publish authentication.

Implementation review should verify that a caller cannot publish with `NPM_TOKEN`,
`NODE_AUTH_TOKEN`, OTP, or another long-lived publish credential through the production SLSA3
reusable workflow unless a later ADR creates a distinct mode with a distinct builder identity.

## Pros and Cons of the Options

### Use OIDC trusted publishing only, with no publish secret

The profile publishes only through npm trusted publishing backed by GitHub Actions OIDC and exposes
no publish secret in `workflow_call.secrets`.

- Good, because it eliminates long-lived npm publish tokens from the production profile.
- Good, because it aligns with npm's recommendation to prefer trusted publishing over tokens.
- Good, because the required caller contract is permission-based rather than secret-based.
- Good, because it keeps the SLSA3 builder mode's security properties clear and stable.
- Good, because npm provenance generation is on the supported trusted-publishing path for public
  packages from public repositories.
- Neutral, because callers must configure npm trusted publisher settings per package.
- Bad, because unsupported registries, self-hosted runners, private repository provenance cases, or
  packages not yet configured for trusted publishing cannot use the initial production profile.

### Use OIDC trusted publishing by default with an optional npm token fallback secret

The profile prefers OIDC but accepts an optional npm publish token when trusted publishing is
unavailable.

- Good, because migration from traditional npm token publishing is easier.
- Good, because more repositories can publish before configuring npm trusted publishing.
- Neutral, because token use could be limited to granular, expiring, package-scoped tokens.
- Bad, because a long-lived write token re-enters the trusted workflow boundary.
- Bad, because silent fallback can make two different security modes look like the same builder.
- Bad, because token provenance behavior and trusted-publishing provenance behavior differ enough to
  complicate verifier expectations.

### Require an npm token publish secret and do not require trusted publishing

The profile requires callers to pass a publish-capable npm token secret such as `NPM_TOKEN` or
`NODE_AUTH_TOKEN`.

- Good, because it matches many existing npm publish workflows.
- Good, because it avoids npm trusted publisher setup and caller workflow filename matching.
- Bad, because it depends on long-lived write credentials stored as GitHub secrets.
- Bad, because token leakage, rotation, scoping, and 2FA bypass become part of the profile's
  security model.
- Bad, because it is weaker than the npm and Windlass OIDC-preferred direction.
- Bad, because it is a poor fit for the production SLSA3 builder identity selected for this profile.

### Provide separate OIDC and token publishing workflow modes

The project provides one OIDC-only SLSA3 workflow and a separate token-based workflow or mode with a
distinct builder identity.

- Good, because it preserves a strong OIDC production path while leaving room for token-based
  migration or unsupported cases.
- Good, because ADR 0020 can keep different security modes distinguishable by `builder.id`.
- Neutral, because token mode could be documented as lower trust or non-SLSA3.
- Bad, because the initial public API, documentation, and verification story become larger.
- Bad, because offering token mode creates operational pressure to support and secure it.

## More Information

This decision follows ADR 0023's package selector input contract and decides only publish
authentication for the initial production JS/TS npm package profile. It does not decide private
dependency installation credentials, staged publishing, caller environment protection, runner image
pinning, Node.js version policy, or package-manager install/build/pack behavior.

Reference points considered:

- SLSA v1.2 Build Provenance defines `externalParameters` as externally controlled build interface
  values that must be complete at SLSA Build L3, recommends minimizing them for verifier usability,
  and treats `builder.id` as the trusted build platform identity.
- SLSA v1.2 Build Requirements require SLSA Build L3 provenance to be strongly resistant to tenant
  forgery and require secret material used for provenance authenticity to be inaccessible to
  tenant-controlled build steps.
- npm trusted publishing uses OIDC to publish from supported CI/CD providers without long-lived npm
  tokens and recommends trusted publishing over traditional tokens.
- npm trusted publishing for GitHub Actions requires `id-token: write`, GitHub-hosted runners, npm
  CLI 11.5.1 or later, and Node.js 22.14.0 or later.
- npm trusted publishing automatically generates npm provenance attestations for supported public
  packages published from supported public repositories.
- npm documents that reusable workflow trusted publishing validates the calling workflow name and
  that `id-token: write` must be granted to both parent and child workflows.
- GitHub Actions OIDC tokens for reusable workflows include `job_workflow_ref`, allowing reusable
  workflow identity to be part of trust policy design even though npm trusted publisher setup is
  package-side and caller-workflow-sensitive.
- Windlass workflow hardening policy prefers OIDC over long-lived secrets and requires
  least-privilege workflow permissions with job-level elevation only where needed.
