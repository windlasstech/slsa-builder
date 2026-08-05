# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Release headings use
Human Era five-digit years (e.g., `## [0.1.0] - 12026-06-13`).

## [Unreleased]

### Security

- Raised the module Go directive to 1.26.5, clearing exposure to
  [GO-2026-4970](https://osv.dev/GO-2026-4970) for consumers building from source with older
  toolchains.
