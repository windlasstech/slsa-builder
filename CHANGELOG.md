# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Release headings use
Human Era five-digit years (e.g., `## [0.1.0] - 12026-06-13`).

## [Unreleased]

### Added

- Added the optional tags-only `source-ref` input to the npm producer workflow: a failed release can
  be retried by dispatching from a ref that carries the fixed pipeline with
  `source-ref: refs/tags/vX.Y.Z`, building and attesting the existing tag's content while the signed
  invocation record preserves the dispatch context, per ADR 0079 and ADR 0080.
- Specified the optional tags-only `source-ref` input that lets a manual dispatch build an existing
  release tag, extending the npm profile contract to nine inputs and rebinding source-identity
  verification to signed provenance fields per ADR 0079 and ADR 0080.
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

- Fixed pnpm package resolution for standalone root packages whose `pnpm-workspace.yaml` contains
  policy settings but omits the optional `packages` member.

### Security

- Raised the module Go directive to 1.26.5, clearing exposure to
  [GO-2026-4970](https://osv.dev/GO-2026-4970) for consumers building from source with older
  toolchains.
