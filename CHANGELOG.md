# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Release headings use Human
Era five-digit years (e.g., `## [0.1.0] - 12026-06-13`).

## [Unreleased]

### Added

- Added the ADR 0088 Corepack-window package-manager version bounds to the JS/TS npm profile: while
  Corepack remains the production provisioning path, consumer manifests pinning a pnpm version
  outside the 11.x line are rejected before install with
  `windlass.verify.error.pnpm-version-unsupported` (enforced from both the top-level
  `packageManager` field and `devEngines.packageManager`), and manifests pinning Yarn 6 or newer are
  rejected before install with `windlass.verify.error.yarn-version-unsupported`. Yarn
  externalParameters in signed provenance are likewise bounded to exact v4/v5 versions, matching the
  revised verification policy.
- Added npm configuration diagnostics logging to the JS/TS npm reusable workflow's build and publish
  jobs: node/npm versions, ambient `NPM_CONFIG_*` variable count, redacted `npm config ls` output,
  and the provenance/registry key view across all config layers, to aid trusted-publishing and
  provenance triage in caller runs. The build job additionally logs the selected non-npm package
  manager's configuration (pnpm `config list` or `yarn config`, resolved from the `packageManager`
  field) with the same redaction of credential-shaped values.
- Added a Go testing and fuzzing guide (`docs/testing-guide.md`) defining test organization,
  security-negative testing, quality gates, and the fuzzing policy for trust-boundary parsers.
- Added property-based fuzz targets for all trust-boundary parsers and validators (attestation
  bundle parsing, verification policy decoding, handoff contracts, npm provenance inputs, registry
  and OIDC response decoding, workspace and package-manager selection parsing, identity and digest
  validators, and workflow decoding), with seed corpora ported from existing negative tests, a
  30-second per-target fuzz smoke job on pull requests, and a scheduled weekly long-run fuzz
  workflow that uploads the fuzz corpus as an artifact.
- Added the optional tags-only `source-ref` input to the npm producer workflow for fixed-pipeline
  release retries, with built-source provenance, signed invocation context, and ADR 0080
  verification binding.
- Added Go-native keyless DSSE signing for npm provenance, with digest-verified handoffs and offline
  exact-Statement verification before bundle upload.
- Added the public npm-only reusable workflow with trusted-publisher preflights, serialized publish
  convergence, immutable artifact handoffs, and persistent outcome reports.
- Added official slsa-builder badge SVGs (`built with` with a package-check icon, `verified with`
  with a shield-check icon, and a plain logo badge in gray or green) under `assets/badges/`, each in
  four shields.io-compatible styles (flat, flat-square, plastic, and for-the-badge), with copy-ready
  Markdown and HTML snippets in the README.
- Expanded and synchronized the bilingual READMEs with a logo header, table of contents, SLSA and
  provenance primers (including the Mini Shai-Hulud caution), an alternatives comparison, profile
  feature tables, an expanded security and trust model, badge usage documentation, contributing
  links, and a license section.
- Added the documented fail-closed package-manager support boundary: consumer manifests pinning pnpm
  12+ or Yarn 6+ are rejected with a diagnostic while Corepack is the production provisioning path;
  supported package managers are npm, pnpm 11.x, and Yarn Berry v4/v5.
- Added support for standalone root packages whose `pnpm-workspace.yaml` contains policy settings
  but omits the optional `packages` member (settings-only workspace files resolve to the root
  package).
- Added mise packageManager-field provisioning for the development Node.js runtime (24.21.0) and
  pnpm (12.10.1), with both versions declared solely in `package.json` (`devEngines.runtime` and
  `devEngines.packageManager`) and Corepack removed from the bootstrap; requires mise v2026.8.7 or
  newer.

### Security

- Pin the module Go directive at 1.27.2 or newer so that source builds are not exposed to
  [GO-2026-4970](https://osv.dev/GO-2026-4970), the Go standard library advisories
  [GO-2026-5026](https://osv.dev/GO-2026-5026), [GO-2026-5942](https://osv.dev/GO-2026-5942),
  [GO-2026-5972](https://osv.dev/GO-2026-5972), [GO-2026-6088](https://osv.dev/GO-2026-6088),
  [GO-2026-6089](https://osv.dev/GO-2026-6089), [GO-2026-6090](https://osv.dev/GO-2026-6090),
  [GO-2026-6091](https://osv.dev/GO-2026-6091), and [GO-2026-6218](https://osv.dev/GO-2026-6218), or
  the standard-library HTTP/2 components of [GO-2026-6603](https://osv.dev/GO-2026-6603),
  [GO-2026-6610](https://osv.dev/GO-2026-6610), [GO-2026-6611](https://osv.dev/GO-2026-6611),
  [GO-2026-6612](https://osv.dev/GO-2026-6612), and [GO-2026-6617](https://osv.dev/GO-2026-6617).
- Pin `golang.org/x/mod` at v0.40.0 or newer so that builds are not exposed to
  [GO-2026-6179](https://osv.dev/GO-2026-6179) and [GO-2026-6180](https://osv.dev/GO-2026-6180).
- Pin `golang.org/x/crypto` at v0.56.0 or newer so that builds are not exposed to the SSH
  denial-of-service advisories [GO-2026-6354](https://osv.dev/GO-2026-6354) and
  [GO-2026-6355](https://osv.dev/GO-2026-6355).
- Pin `golang.org/x/net` at v0.60.0 or newer so that builds are not exposed to the
  `golang.org/x/net` components of [GO-2026-6603](https://osv.dev/GO-2026-6603),
  [GO-2026-6610](https://osv.dev/GO-2026-6610), [GO-2026-6611](https://osv.dev/GO-2026-6611),
  [GO-2026-6612](https://osv.dev/GO-2026-6612), and [GO-2026-6617](https://osv.dev/GO-2026-6617).
- Pin `google.golang.org/grpc` at v1.83.2 or newer so that builds are not exposed to the xDS server
  panic advisory [GO-2026-6443](https://osv.dev/GO-2026-6443); the fix also shipped in the v1.84.0
  stable release, which the module now carries (with `grpc-ecosystem/grpc-gateway/v2` at v2.31.0).
- Pin the development-tooling transitive dependency `katex` at 0.18.2 or newer via a pnpm override
  (resolving to 0.19.0, because `micromark-extension-math` caps its declared range at ^0.16.0) so
  that lint tooling is not exposed to [GHSA-238p-pmpm-9mq7](https://osv.dev/GHSA-238p-pmpm-9mq7).
- Pin the development-tooling transitive dependency `smol-toml` at 1.9.0 or newer via a pnpm
  override (resolving to 1.9.1, because `markdownlint-cli2` pins 1.8.0 exactly) so that lint tooling
  is not exposed to [GHSA-r4xh-jqrq-34v2](https://osv.dev/GHSA-r4xh-jqrq-34v2).
- Record the development-tooling transitive dependency `braces` 3.0.3 as a known-unfixable finding:
  [GHSA-vfj7-8cjw-p6xm](https://osv.dev/GHSA-vfj7-8cjw-p6xm) has no fixed release (its last affected
  version equals the latest published release), so the advisory is ignored in `osv-scanner.toml`
  until 2027-01-10; the package is reached only through `micromatch` glob matching in
  markdownlint-cli2 lint runs over repo-controlled patterns.
