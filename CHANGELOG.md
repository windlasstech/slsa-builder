# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Release headings use Human
Era five-digit years (e.g., `## [0.1.0] - 12026-06-13`).

## [Unreleased]

### Added

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
- Added mise packageManager-field provisioning for the development pnpm (12.9.0), with the pnpm
  version declared solely in `package.json` (`devEngines.packageManager`), a `devEngines.runtime`
  Node.js 24 declaration, and Corepack removed from the bootstrap; requires mise v2026.8.7 or newer.
