# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Release headings use
Human Era five-digit years (e.g., `## [0.1.0] - 12026-06-13`).

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

### Fixed

- Fixed verification policy and release-manifest expectation decoding to classify every JSON parse
  failure as `windlass.verify.error.policy-schema-invalid` (duplicate members remain
  `windlass.verify.error.duplicate-json-member`), matching the documented evaluation contract.
- Fixed canonical repository URI normalization to reject or strip mixed-case `.git` suffixes,
  keeping `CanonicalRepository` output idempotent and acceptable to the canonical-form validator.
- Fixed pnpm package resolution for standalone root packages whose `pnpm-workspace.yaml` contains
  policy settings but omits the optional `packages` member.
- Fixed a panic in the workflow conformance decoder when workflow YAML maps a tagged scalar (for
  example `!!str`) into a sequence field, by pinning goccy/go-yaml to the upstream fix commit
  ([goccy/go-yaml#862](https://github.com/goccy/go-yaml/pull/862)) so such input decodes to an
  ordinary type error; as defense in depth, the decoder also converts any residual decoder panic
  into a bounded `decode workflow: decoder panic` error instead of crashing.

### Security

- Raised the module Go directive to 1.26.6, clearing exposure to
  [GO-2026-4970](https://osv.dev/GO-2026-4970) and the Go standard library advisories
  [GO-2026-5026](https://osv.dev/GO-2026-5026), [GO-2026-5942](https://osv.dev/GO-2026-5942),
  [GO-2026-5972](https://osv.dev/GO-2026-5972), [GO-2026-6088](https://osv.dev/GO-2026-6088),
  [GO-2026-6089](https://osv.dev/GO-2026-6089), [GO-2026-6090](https://osv.dev/GO-2026-6090),
  [GO-2026-6091](https://osv.dev/GO-2026-6091), and [GO-2026-6218](https://osv.dev/GO-2026-6218) for
  consumers building from source with older toolchains.
- Upgraded `golang.org/x/mod` to v0.40.0, clearing [GO-2026-6179](https://osv.dev/GO-2026-6179) and
  [GO-2026-6180](https://osv.dev/GO-2026-6180).
