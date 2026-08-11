<div align="center">

<h1>
  <a href="https://slsa-builder.dev">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="assets/logo/logo-dark.svg" />
      <img src="assets/logo/logo.svg" alt="slsa-builder" width="256" height="256" />
    </picture>
    <br>
    slsa-builder
  </a>
</h1>

[![GitHub License](https://img.shields.io/github/license/windlasstech/slsa-builder)](LICENSE)
[![SemVer Versioning](https://img.shields.io/badge/version_scheme-SemVer-0097a7)](https://semver.org/)
[![SLSA Build L3](slsa-build-l3-badge.svg)](https://slsa.dev/spec/v1.2/build-requirements#build-platform)
[![GitHub Release](https://img.shields.io/github/v/release/windlasstech/slsa-builder)](https://github.com/windlasstech/slsa-builder/releases)
[![GitHub Release Date](https://img.shields.io/github/release-date/windlasstech/slsa-builder)](https://github.com/windlasstech/slsa-builder/releases)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-3.0-4baaaa.svg)](https://github.com/windlasstech/.github/blob/main/CODE_OF_CONDUCT.md)
[![GitHub issues](https://img.shields.io/badge/issue_tracking-GitHub-blue.svg)](https://github.com/windlasstech/slsa-builder/issues)

[![Lint](https://github.com/windlasstech/slsa-builder/actions/workflows/lint.yml/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/lint.yml)
[![CodeQL](https://github.com/windlasstech/slsa-builder/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/github-code-scanning/codeql)
[![OSV Scanner](https://github.com/windlasstech/slsa-builder/actions/workflows/osv-scanner.yml/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/osv-scanner.yml)
[![Dependency Review](https://github.com/windlasstech/slsa-builder/actions/workflows/dependency-review.yml/badge.svg)](https://github.com/windlasstech/slsa-builder/actions/workflows/dependency-review.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/windlasstech/slsa-builder/badge)](https://scorecard.dev/viewer/?uri=github.com/windlasstech/slsa-builder)

English | [한국어](README.ko.md)

</div>

A reusable, profile-extensible [SLSA provenance](https://slsa.dev/spec/v1.2/build-provenance)
builder foundation.

## Contents

- [Purpose and goals](#purpose-and-goals)
  - [What is SLSA?](#what-is-slsa)
  - [What is provenance?](#what-is-provenance)
  - [Who it is for](#who-it-is-for)
- [How it works](#how-it-works)
- [Why should I choose slsa-builder?](#why-should-i-choose-slsa-builder)
  - [Limitations of existing alternatives](#limitations-of-existing-alternatives)
  - [Strengths of slsa-builder](#strengths-of-slsa-builder)
- [Features](#features)
  - [Provenance issuance and publishing](#provenance-issuance-and-publishing)
  - [Release-asset mode](#release-asset-mode)
  - [Provenance verification](#provenance-verification)
- [Security and trust model](#security-and-trust-model)
- [Project status and scope](#project-status-and-scope)
- [Spread the word](#spread-the-word)
- [Specifications and ADRs](#specifications-and-adrs)
- [Development setup](#development-setup)
- [Contributors](#contributors)
- [License](#license)

## Purpose and goals

slsa-builder is a clean, modern, profile-extensible foundation for trusted
[SLSA provenance](https://slsa.dev/spec/v1.2/build-provenance) builders with a small, auditable
trusted computing base (TCB). As a spiritual successor to
[slsa-github-generator](https://github.com/slsa-framework/slsa-github-generator), it carries on the
original intent and philosophy without inheriting the legacy surface. Its goal is to freshly
implement and support SLSA Build L3+ release workflows for diverse language and registry ecosystems,
against the latest SLSA v1.2 specification.

slsa-builder started as a project to meet
[Windlass's supply-chain security policies, standards, and goals](https://github.com/windlasstech/.github/blob/main/SECURITY.md#supply-chain-integrity),
and it can be broadly useful beyond Windlass to
[all of the target users described below](#who-it-is-for).

### What is SLSA?

**SLSA ([Supply-chain Levels for Software Artifacts](https://slsa.dev/), "salsa")** is a security
framework: a checklist of standards and controls to prevent tampering, improve integrity, and
protect packages and infrastructure. It is a means of securing the highest feasible resilience at
every step of the supply chain, going beyond merely being "safe enough."

> Supply-chain Levels for Software Artifacts, or SLSA (“salsa”), is a set of incrementally adoptable
> guidelines for supply chain security, established by industry consensus. The specification set by
> SLSA is useful for both software producers and consumers: producers can follow SLSA’s guidelines
> to make their software supply chain more secure, and consumers can use SLSA to make decisions
> about whether to trust a software package.
>
> ― <https://slsa.dev/spec/v1.2/about>

[SLSA is described in terms of tracks and levels](https://slsa.dev/spec/v1.2/about#how-slsa-works).
Each SLSA track focuses on a particular aspect of the supply chain; as of v1.2 there is a
[Build track](https://slsa.dev/spec/v1.2/tracks#build-track) and a
[Source track](https://slsa.dev/spec/v1.2/tracks#source-track).

Within each track, a higher level means a stronger security posture. Higher levels guarantee
stronger defense against supply-chain threats, at a higher implementation cost. Lower SLSA levels
are designed for easy adoption but provide limited guarantees. "SLSA 0" is sometimes used to
describe software that does not yet meet any SLSA level. The SLSA build track currently
[spans Build L1 through L3](https://slsa.dev/spec/v1.2/build-track-basics), and the official SLSA
site states that
[higher levels are planned for future revisions](https://slsa.dev/spec/v1.2/future-directions).

Combining tracks and levels makes it easy to state whether software meets a specific set of
requirements. **Saying that an artifact meets SLSA Build L3 means that the software artifact was
built in accordance with a set of security practices that industry experts recognize as effective at
preventing specific supply-chain compromises.**

### What is provenance?

**Provenance** is metadata about how a software artifact was produced. It can include information
about the source code used, the build system, and the build steps — and even who initiated the build
and why. Provenance can be used to judge the authenticity and trustworthiness of the software
artifacts you use.

SLSA defines a [provenance format](https://slsa.dev/provenance/) for recording this metadata.
slsa-builder is a tooling foundation for building and distributing software artifacts in a way that
meets SLSA Build L3, and that includes automatically issuing and distributing appropriate provenance
under the [in-toto attestation](https://github.com/in-toto/attestation) framework and the
[SLSA build provenance v1 format](https://slsa.dev/spec/v1.2/build-provenance)(`"predicateType": "https://slsa.dev/provenance/v1"`).

> [!CAUTION]  
> As the **Mini Shai-Hulud** supply-chain attack of May 11, 12026 shows, provenance and signatures
> are necessary conditions for safety, not sufficient ones. Provenance is only one component of the
> SLSA framework. If the build platform that builds a software artifact and issues its provenance is
> itself compromised by an attacker, compromised packages can end up being distributed with
> cryptographically "validly signed" provenance. The packages compromised in the **Mini Shai-Hulud**
> incident were using npm's OIDC Trusted Publishing and built-in provenance, which correspond to
> SLSA Build L2, but the build-environment isolation corresponding to SLSA Build L3 was not in
> place.
>
> SLSA Build L2 is by no means meaningless — it is far safer than L0 or L1 — but there is a
> meaningful gap between Build L2 and L3, and it is worth understanding exactly what that gap is and
> the limits it implies. The following articles may help:
>
> - [Signed doesn’t mean safe](https://arew-m.medium.com/signed-doesnt-mean-safe-7261b0763ea0)
> - [Mini Shai-Hulud: Where SLSA's Boundaries Fall](https://slsa.dev/blog/2026/05/mini-shai-hulud-what-slsa-can-and-cannot-do)

### Who it is for

As the official [SLSA overview](https://slsa.dev/spec/v1.2/about) states, SLSA targets software
producers, consumers, and infrastructure providers, and it broadly helps anyone who produces,
supplies, or distributes packages. For producers, it offers protection against supply-chain
tampering, reduced insider risk, and assurance that software reaches consumers as intended — along
with a common vocabulary for communication, an actionable checklist, a measure of
[SSDF](https://csrc.nist.gov/Projects/ssdf) alignment, and clearer shared expectations between
suppliers and consumers.

slsa-builder can help this same broad SLSA audience, and it is especially useful if one or more of
the following applies to you:

1. Repository maintainers, release workflow authors, and package publishers.
2. Advanced users who compose workflows from producer and publisher primitives.
3. Downstream verifiers who use the release manifest, verification policy, and fixtures.

## How it works

A thin trusted core works together with profile-owned
[reusable GitHub Actions workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows).
Producer and publisher profiles — and, within a profile, each task (build, sign, publish, and so on)
— run isolated with distinct permissions. Producers build artifacts and SLSA provenance, then hand
them off to a publisher, which verifies and distributes them. The handoff includes digest and
provenance verification, and the procedure aborts immediately if a problem is detected. A signed
release manifest maps release versions to workflow SHAs, `builder.id` values, and `buildType` URIs.

The exact, observable behavior of each contract outlined in this section is defined in the following
technical specifications:

- [Core profile contract](docs/architecture/core-profile-contract.md): the boundary between the thin
  trusted core and profile-owned reusable workflows.
- [Identity and build types](docs/architecture/identity-and-buildtypes.md): `builder.id`,
  `buildType` URIs, and release-metadata linkage.
- [SLSA provenance v1](docs/architecture/slsa-provenance-v1.md): the common in-toto Statement and
  SLSA v1 predicate contract.
- [Composed workflow internal handoff](docs/architecture/composed-workflow-internal-handoff.md):
  producer-to-publisher handoff within a single run.
- [GitHub Release asset publisher](docs/architecture/github-release-asset-publisher.md): the
  publisher contract that distributes only verified bytes.
- [Release manifest](docs/architecture/release-manifest.md): the signed release manifest and its
  three-job signing boundary.

## Why should I choose slsa-builder?

slsa-builder aims to meaningfully lower the barrier to adopting the SLSA security framework for
members of diverse language and package-registry ecosystems, and to contribute to ecosystem
supply-chain security by more broadly disseminating and encouraging SLSA Build L3+
package-distribution practices (see
[ADR 0001](docs/decisions/0001-start-slsa-builder-as-clean-repository.md) and
[ADR 0002](docs/decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md)). Other
existing tools remain useful within their intended scopes, but slsa-builder's strength is providing
one integrated, profile-owned trust contract — spanning build, provenance, publishing, distribution,
and verification — with a low adoption barrier.

### Limitations of existing alternatives

The [SLSA get-started guide](https://slsa.dev/how-to/get-started) recommends starting at the highest
feasible level from the outset to avoid unnecessary rework. On the GitHub Actions platform, SLSA
Build L3 is achievable with a properly designed and configured trust boundary, but existing
alternatives have the following limitations.

- **`slsa-github-generator` and `slsa-verifier` (maintenance discontinued):**
  - Starting with the
    [first public release in June 12022](https://github.com/slsa-framework/slsa-github-generator/releases/tag/v1.0.0),
    the SLSA framework team developed and maintained
    [`slsa-github-generator`](https://github.com/slsa-framework/slsa-github-generator) and
    [`slsa-verifier`](https://github.com/slsa-framework/slsa-verifier) for roughly three years, and
    for a long time they contributed greatly to lowering the barrier to SLSA Build L3 adoption.
  - However, due to a
    [design limitation of `slsa-verifier`](https://github.com/slsa-framework/slsa-verifier/issues/12),
    the builders provided by `slsa-github-generator` cannot be referenced by digest; only
    tag-version references in the `@vX.Y.Z` form are possible. This contradicts
    [commonly accepted security best practices](https://docs.github.com/en/actions/reference/security/secure-use#using-third-party-actions)
    and
    [Windlass's own security guidance](https://github.com/windlasstech/.github/blob/main/docs/security/workflow-hardening.md#action-references).
    - See: [ADR 0028](./docs/decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md)
  - Crucially, these projects were effectively unmaintained from around July 12025 — after
    [`slsa-github-generator` v2.1.0](https://github.com/slsa-framework/slsa-github-generator/releases/tag/v2.1.0)
    and
    [`slsa-verifier` v2.7.1](https://github.com/slsa-framework/slsa-verifier/releases/tag/v2.7.1) —
    and
    [maintenance was officially discontinued on August 7, 12026](https://github.com/slsa-framework/slsa-github-generator/pull/4515).
  - The latest SLSA specification version is v1.2, but the provenance format supported by
    slsa-github-generator has not been updated since [v0.2](https://slsa.dev/spec/v0.2/provenance).
    Using these tools is therefore no longer recommended.
- **GitHub `actions/attest`:**
  - GitHub Artifact Attestations — the [`attest` action](https://github.com/actions/attest) — make
    it possible to build and distribute packages on the GitHub Actions platform while meeting SLSA
    Build L3 requirements.
  - GitHub Artifact Attestations automatically handle work such as provenance issuance and signing
    backed by a [Sigstore](https://www.sigstore.dev/) instance.
  - However, using GitHub Artifact Attestations on GitHub-hosted runners alone achieves only up to
    SLSA Build L2. Meeting Build L3's
    [provenance-unforgeable](https://slsa.dev/spec/v1.2/build-requirements#provenance-unforgeable)
    and [isolated](https://slsa.dev/spec/v1.2/build-requirements#isolated) requirements additionally
    requires setting up a dedicated
    [reusable GitHub Actions workflow](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)
    for building and signing, so that the build and distribution process runs in an isolated
    environment in a repository different from the caller workflow. This is not solved by the
    [`attest` action](https://github.com/actions/attest) alone, and it is a factor that
    comparatively raises the barrier to adoption.
    - [GitHub Docs: Artifact attestations concepts](https://docs.github.com/en/actions/concepts/security/artifact-attestations)
    - [GitHub Blog: Enhance build security and reach SLSA Level 3 with GitHub Artifact Attestations](https://github.blog/enterprise-software/devsecops/enhance-build-security-and-reach-slsa-level-3-with-github-artifact-attestations/)
    - [GitHub Docs: Increase your security rating with artifact attestations](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/increase-security-rating)
  - In addition, the [`attest` action](https://github.com/actions/attest) supports only SHA-256
    digest output in Provenance mode. This is fine in many cases, but for package managers and
    registries like npm that accept only SHA-512 input, using `--provenance-file` or similar options
    may not be possible.

### Strengths of slsa-builder

- **Low-barrier SLSA adoption meeting Build L3 requirements:** As covered above, meeting Build L3's
  provenance-forgery-resistance and build-isolation requirements calls for an isolated reusable
  workflow in a repository separate from the target package, and designing and maintaining that
  trust boundary yourself can feel like a burden for each repository maintainer. slsa-builder
  designs this boundary for you as profile-owned reusable workflows, offered as a paved road to SLSA
  Build L3. From a caller workflow, a `uses:` reference and a handful of inputs such as
  `package-directory` bring build, provenance issuance, signing, publishing, and verification in as
  a single contract. Workflow references can be pinned to commit SHAs instead of tags — avoiding the
  tag-based-reference limitation noted earlier — and provenance follows the latest SLSA v1.2
  specification. The goal is to carry forward, on top of the current specification, the low-barrier
  model that slsa-github-generator demonstrated on GitHub
  ([ADR 0002](docs/decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md),
  [ADR 0003](docs/decisions/0003-use-thin-core-with-profile-owned-reusable-workflows.md),
  [ADR 0023](docs/decisions/0023-use-package-directory-as-required-js-ts-npm-package-selector.md),
  [ADR 0028](docs/decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md); spec:
  [JS/TS npm package profile](docs/architecture/js-ts-npm-package-profile.md)).
- **Minimized trust surface:** Rather than inheriting slsa-github-generator's broad legacy and BYOB
  framework surface, slsa-builder chose a completely fresh start and a smaller, deliberately
  selected trust surface ([ADR 0001](docs/decisions/0001-start-slsa-builder-as-clean-repository.md),
  [ADR 0002](docs/decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md),
  [ADR 0003](docs/decisions/0003-use-thin-core-with-profile-owned-reusable-workflows.md); spec:
  [Core profile contract](docs/architecture/core-profile-contract.md)).
- **Canonical provenance semantics:** slsa-builder records the profile's `builder.id`, `buildType`,
  `externalParameters`, subject, digest, publish, and verification semantics, and assembles and
  signs exact Statement bytes with a Go-native Sigstore DSSE signer. A single subject can also carry
  both SHA-256 and SHA-512 digests over the same tarball bytes, which provides better compatibility
  than tools that support only a single digest output per subject
  ([ADR 0029](docs/decisions/0029-use-windlass-generated-slsa-provenance-for-npm-publish.md),
  [ADR 0042](docs/decisions/0042-use-acquired-domains-for-buildtype-uris.md),
  [ADR 0064](docs/decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md),
  [ADR 0077](docs/decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md);
  spec: [SLSA provenance v1](docs/architecture/slsa-provenance-v1.md),
  [Identity and build types](docs/architecture/identity-and-buildtypes.md)).
- **End-to-end release trust:** Strict signed-JSON handling, immutable builder and source binding,
  Rekor-backed offline verification with a governed trust root, a signed release manifest,
  provenance-gated release-asset publishing, and controlled release mutation — all connected
  end-to-end from source build to release distribution
  ([ADR 0031](docs/decisions/0031-use-sigstore-signed-in-toto-release-manifest.md),
  [ADR 0049](docs/decisions/0049-separate-artifact-production-from-github-release-asset-publication.md),
  [ADR 0050](docs/decisions/0050-define-producer-to-publisher-handoff-contract.md),
  [ADR 0051](docs/decisions/0051-distribute-producer-provenance-with-release-assets.md),
  [ADR 0053](docs/decisions/0053-use-three-job-release-manifest-signing-boundary.md),
  [ADR 0061](docs/decisions/0061-reject-duplicate-json-members-in-signed-slsa-statements.md),
  [ADR 0066](docs/decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [ADR 0067](docs/decisions/0067-converge-repeated-runs-within-run-identity.md),
  [ADR 0068](docs/decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md),
  [ADR 0069](docs/decisions/0069-require-rekor-transparency-and-govern-sigstore-trust-root.md),
  [ADR 0072](docs/decisions/0072-use-sidecar-first-pair-binding-for-release-asset-run-ownership.md),
  [ADR 0073](docs/decisions/0073-require-published-attestation-run-identity-for-npm-same-run-convergence.md),
  [ADR 0074](docs/decisions/0074-use-single-job-mutation-segments-with-detection-based-cross-run-safety.md),
  [ADR 0075](docs/decisions/0075-queue-mutation-segment-contenders-with-queue-max.md),
  [ADR 0076](docs/decisions/0076-use-observation-preflights-and-first-mutation-classification.md);
  spec: [Release manifest](docs/architecture/release-manifest.md),
  [GitHub Release asset publisher](docs/architecture/github-release-asset-publisher.md),
  [Verification policy and fixtures](docs/architecture/verification-policy-and-fixtures.md)).

## Features

slsa-builder currently provides issuance, distribution, and verification of SLSA provenance for
JS/TS packages, GitHub Releases, and npm. Choose the profile that matches your ecosystem and
distribution target, and reference it from a caller workflow. Support for additional ecosystems and
distribution targets will continue to be added over time.

### Provenance issuance and publishing

| Ecosystem | Profile                                                                     | Description                                                             | Status      |
| :-------- | :-------------------------------------------------------------------------- | :---------------------------------------------------------------------- | :---------- |
| JS/TS npm | [JS/TS npm package profile](docs/architecture/js-ts-npm-package-profile.md) | npm package build, SLSA v1 provenance issuance and signing, npm publish | Pre-release |

- **Exactly one package per run:** Select the target package with the required `package-directory`
  input. The public contract consists of this one required input and seven optional inputs.
- **Manifest-first package manager selection:** Supports npm, pnpm, and Yarn Berry v4+ through
  Corepack, and runs build scripts only when declared (see
  [JS/TS npm build and pack](docs/architecture/js-ts-npm-build-pack.md)).
- **Secretless trusted publishing:** Authenticates with npm OIDC trusted publishing, so no
  long-lived publish secrets are needed. The SLSA v1 provenance slsa-builder generates carries both
  SHA-512 and SHA-256 digests of the same tarball bytes in a single npm Package URL subject, is
  signed with the Go-native Sigstore DSSE signer, and is published through a three-job publish graph
  (see [JS/TS npm provenance and publish](docs/architecture/js-ts-npm-provenance-publish.md)).

### Release-asset mode

Uploads the built tarball and its provenance sidecar (`tarball.intoto.jsonl`) to an existing GitHub
Release after digest verification (see
[npm-to-release-asset composition](docs/architecture/npm-to-release-asset-composition.md)).

### Provenance verification

For downstream verifiers, slsa-builder provides a verification policy schema, a fixture taxonomy,
and reference commands. The initial profiles do not include a standalone verifier CLI (see
[Verification policy and fixtures](docs/architecture/verification-policy-and-fixtures.md)).

## Security and trust model

slsa-builder's trust model starts from
[the premise that "signed provenance is a necessary condition for safety, not a sufficient one"](#what-is-provenance).
This section summarizes which threats slsa-builder defends against, which external parties it must
trust, and where the limits of its defense lie.

### Key management and signing

- **No long-lived secrets:** npm publish authentication uses OIDC trusted publishing, so no publish
  tokens need to be stored in the repository
  ([ADR 0024](docs/decisions/0024-use-oidc-trusted-publishing-without-publish-secrets.md)).
- **Keyless signing:** Signing uses Sigstore (Fulcio short-lived certificates and OIDC workload
  identity), so the operational burden of issuing, storing, and rotating private keys disappears
  entirely. Assembling and signing the exact Statement bytes is handled by the Go-native Sigstore
  DSSE signer
  ([ADR 0077](docs/decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md)).
- **Strict signed bytes:** Duplicate members in signed JSON are rejected, preventing parser
  differentials from splitting the signed byte payload
  ([ADR 0061](docs/decisions/0061-reject-duplicate-json-members-in-signed-slsa-statements.md)).

### Verification model

- **Immutable identity binding:** Verification binds to the `builder.id` of a commit-SHA-pinned
  workflow and to an immutable source identity, not to movable tags
  ([ADR 0068](docs/decisions/0068-bind-verification-to-immutable-builder-and-source-identities.md)).
- **Transparency log and governed trust root:** Every signature must be recorded in the Rekor
  transparency log, and the Sigstore trust root uses a pinned copy governed by the project, enabling
  offline verification
  ([ADR 0069](docs/decisions/0069-require-rekor-transparency-and-govern-sigstore-trust-root.md)).
- **Dual digests:** A single subject carries both SHA-512 and SHA-256 over the same tarball bytes,
  supporting verification paths that require different digest algorithms
  ([ADR 0064](docs/decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md)).
- The verification policy schema, fixture taxonomy, and reference commands are defined in
  [Verification policy and fixtures](docs/architecture/verification-policy-and-fixtures.md).

### Release integrity

- **Signed release manifest:** A manifest mapping release versions to workflow SHAs, `builder.id`,
  and `buildType` is signed across a three-job signing boundary
  ([ADR 0031](docs/decisions/0031-use-sigstore-signed-in-toto-release-manifest.md),
  [ADR 0053](docs/decisions/0053-use-three-job-release-manifest-signing-boundary.md)).
- **Provenance-gated publishing:** The publisher distributes only after verifying the producer's
  provenance and digests; unverified bytes are never distributed
  ([ADR 0049](docs/decisions/0049-separate-artifact-production-from-github-release-asset-publication.md),
  [ADR 0050](docs/decisions/0050-define-producer-to-publisher-handoff-contract.md),
  [ADR 0051](docs/decisions/0051-distribute-producer-provenance-with-release-assets.md)).
- **Controlled release mutation:** Release mutations are serialized with job-class concurrency, and
  repeated runs within the same run converge to the same result
  ([ADR 0066](docs/decisions/0066-serialize-release-mutations-with-job-class-concurrency.md),
  [ADR 0067](docs/decisions/0067-converge-repeated-runs-within-run-identity.md)).

### Trust boundary and dependencies

slsa-builder places trust in GitHub Actions (hosted runners and the OIDC provider), Sigstore
(Fulcio, Rekor), and the npm registry. It satisfies SLSA Build L3's isolation requirement by
separating build and signing into reusable workflows in a repository different from the caller
workflow ([ADR 0028](docs/decisions/0028-use-sha-pinned-reusable-workflow-builder-identity.md)). The
project is SLSA Build L3-oriented.

### Reporting vulnerabilities

Please report security vulnerabilities through the channel described in the
[Windlass security policy](https://github.com/windlasstech/.github/blob/main/SECURITY.md), not via
public issues.

## Project status and scope

Pre-release. The repository contains a real Go implementation, but the project does not yet claim a
stable release; workflow interfaces may change before the first stable version.

Initial non-goals:

- Generic files or containers.
- A standalone verifier CLI.
- Custom-token or PAT release mutation.
- Raw file uploads without producer provenance.
- First publication of a brand-new npm package identity.
- Private dependency credentials as publish credentials.

## Spread the word

Whether it is an introduction, an endorsement, or constructive criticism pointing out problems and
areas for improvement, the more content — articles, videos, and the like — about slsa-builder and
the SLSA framework, the more we can raise awareness of the framework and these tools across the
ecosystem and drive adoption. Contributions and feedback are always welcome
([contributing guidelines](https://github.com/windlasstech/.github/blob/main/CONTRIBUTING.md),
[issue tracker](https://github.com/windlasstech/slsa-builder/issues)).

There is also a much simpler way to help slsa-builder. If your project uses or supports
slsa-builder, spread the word by linking to the project from your README or other project pages. We
provide several kinds of badges for inclusion in a README or similar project documentation.

The badges are static SVGs that include the project logo, stored in
[`assets/badges/`](assets/badges/). Because this repository serves them directly instead of going
through an external badge service such as shields.io, the full-color logo is preserved and the badge
assets are versioned together with the project. Copy the snippets below as-is, or copy the SVG files
and host them yourself. Replace `main` in the snippet URLs with a release tag or commit SHA to pin
an immutable reference.

### built with slsa-builder

For projects that build and distribute artifacts with slsa-builder.

[![built with slsa-builder](assets/badges/built-with-slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)

Markdown:

```markdown
[![built with slsa-builder](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/built-with-slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)
```

HTML:

```html
<a href="https://github.com/windlasstech/slsa-builder"
  ><img
    src="https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/built-with-slsa-builder.svg"
    alt="built with slsa-builder"
/></a>
```

### verified with slsa-builder

For downstream verifiers that verify releases with slsa-builder's verification policy and fixtures.

[![verified with slsa-builder](assets/badges/verified-with-slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)

Markdown:

```markdown
[![verified with slsa-builder](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/verified-with-slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)
```

HTML:

```html
<a href="https://github.com/windlasstech/slsa-builder"
  ><img
    src="https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/verified-with-slsa-builder.svg"
    alt="verified with slsa-builder"
/></a>
```

### slsa-builder logo badges

Single-background logo badges for simply pointing to the project, provided in two colors: gray
(default) and green.

[![slsa-builder](assets/badges/slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)
[![slsa-builder (green)](assets/badges/slsa-builder-green.svg)](https://github.com/windlasstech/slsa-builder)

Markdown:

```markdown
[![slsa-builder](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/slsa-builder.svg)](https://github.com/windlasstech/slsa-builder)
[![slsa-builder (green)](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/slsa-builder-green.svg)](https://github.com/windlasstech/slsa-builder)
```

HTML:

```html
<a href="https://github.com/windlasstech/slsa-builder"
  ><img
    src="https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/slsa-builder.svg"
    alt="slsa-builder"
/></a>
<a href="https://github.com/windlasstech/slsa-builder"
  ><img
    src="https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/slsa-builder-green.svg"
    alt="slsa-builder"
/></a>
```

### Style variants

Each badge is provided in four styles, similar to shields.io. The default `flat` is provided without
a suffix; the other styles are distinguished by a filename suffix.

| Style            | Filename example                            | Preview                                                                                             |
| ---------------- | ------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `flat` (default) | `built-with-slsa-builder.svg`               | ![built with slsa-builder — flat](assets/badges/built-with-slsa-builder.svg)                        |
| `flat-square`    | `built-with-slsa-builder-flat-square.svg`   | ![built with slsa-builder — flat-square](assets/badges/built-with-slsa-builder-flat-square.svg)     |
| `plastic`        | `built-with-slsa-builder-plastic.svg`       | ![built with slsa-builder — plastic](assets/badges/built-with-slsa-builder-plastic.svg)             |
| `for-the-badge`  | `built-with-slsa-builder-for-the-badge.svg` | ![built with slsa-builder — for-the-badge](assets/badges/built-with-slsa-builder-for-the-badge.svg) |

The same suffix rule applies to the `verified-with-slsa-builder` and `slsa-builder` badges. For the
green variant of the logo badge, the color suffix comes before the style suffix (e.g.,
`slsa-builder-green-for-the-badge.svg`). For example, a `for-the-badge` style Markdown snippet looks
like this:

```markdown
[![built with slsa-builder](https://raw.githubusercontent.com/windlasstech/slsa-builder/main/assets/badges/built-with-slsa-builder-for-the-badge.svg)](https://github.com/windlasstech/slsa-builder)
```

## Specifications and ADRs

See the [architecture index](docs/architecture/README.md) and the
[ADR index](docs/decisions/README.md). ADRs record the rationale in MADR format, while architecture
specifications define exactly observable behavior.

## Development setup

This repository uses [mise](https://mise.jdx.dev/) to install and pin the development-tool runtime
versions. Go is the primary implementation language; Node.js and pnpm are used only for development
tooling such as Prettier and Lefthook.

### Prerequisites

- [mise](https://mise.jdx.dev/getting-started.html) installed
- Git with a configured user name and email

### Bootstrap

```bash
mise install
pnpm install
```

This installs the pinned versions of Go, Node.js, pnpm, and the CLI tools defined in `mise.toml`.
Lefthook hooks are installed automatically as a `postinstall` step when mise installs Lefthook. The
`pnpm install` step then installs the project-local development dependencies declared in
`package.json`.

In CI, run mise with locked mode to avoid API calls to registries:

```bash
MISE_LOCKED=1 mise install
pnpm install
```

After bootstrap, the following commands are available through mise:

```bash
go version
node --version
pnpm --version
golangci-lint --version
shellcheck --version
shfmt --version
lefthook --version
actionlint --version
```

### What mise installs versus what pnpm installs

mise installs language runtimes and standalone CLI binaries:

- Go, Node.js, and pnpm
- `golangci-lint`, `shellcheck`, `shfmt`, `lefthook`, `actionlint`

Go source formatting and import normalization is handled by `golangci-lint` formatters (`gofmt`,
`goimports`), configured in `.golangci.yml`, rather than by standalone formatter binaries.

pnpm installs Node.js-based development dependencies that are coupled to repository configuration
files:

- `prettier` (configured by `.prettierrc`)
- `markdownlint-cli2` (configured by `.markdownlint-cli2.jsonc`)

Keeping Prettier and `markdownlint-cli2` as project-local pnpm dependencies preserves their full
dependency graph in `pnpm-lock.yaml` and keeps them aligned with editor integrations and the
organization's dependency-review workflow.

### Tool versions

Tool versions are declared in `mise.toml`. A `mise.lock` file is committed to ensure reproducible
installs across platforms. If you change a tool version in `mise.toml`, regenerate the lockfile
with:

```bash
mise lock
```

### Commit conventions and sign-off

This project requires a `Signed-off-by:` trailer on every commit (DCO). Lefthook is configured to
enforce this locally, while CI and branch protection perform the authoritative verification.

## Contributors

Thanks to everyone who has contributed to this project. You can find the list of contributors on the
[GitHub contributors graph](https://github.com/windlasstech/slsa-builder/graphs/contributors).

[![Contributors](https://contrib.rocks/image?repo=windlasstech/slsa-builder)](https://github.com/windlasstech/slsa-builder/graphs/contributors)

## License

Distributed under the [Apache License 2.0](LICENSE).
