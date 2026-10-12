# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Release headings use Human
Era five-digit years (e.g., `## [0.1.0] - 12026-06-13`).

## [Unreleased]

### Added

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
  supported package managers are npm, pnpm 10.x and 11.x, and Yarn Berry v4/v5.
- Added support for standalone root packages whose `pnpm-workspace.yaml` contains policy settings
  but omits the optional `packages` member (settings-only workspace files resolve to the root
  package).
- Added the optional tags-only `source-ref` input to the npm producer workflow for fixed-pipeline
  release retries, with built-source provenance, signed invocation context, and ADR 0080
  verification binding.
- Added npm configuration diagnostics logging to the JS/TS npm reusable workflow's build and publish
  jobs: node/npm versions, ambient `NPM_CONFIG_*` variable count, redacted `npm config ls` output,
  and the provenance/registry key view across all config layers, to aid trusted-publishing and
  provenance triage in caller runs. The build job additionally logs the selected non-npm package
  manager's configuration (pnpm `config list` or `yarn config`, resolved from the `packageManager`
  field) with the same redaction of credential-shaped values.
- Added the ADR 0088 Corepack-window package-manager version bounds to the JS/TS npm profile: while
  Corepack remains the production provisioning path, consumer manifests pinning a pnpm version
  outside the supported 10.x or 11.x lines (`[10.0.0, 12.0.0)`, per the ADR 0090 boundary rule) are
  rejected before install with `windlass.verify.error.pnpm-version-unsupported`, and manifests
  pinning Yarn 6 or newer are rejected before install with
  `windlass.verify.error.yarn-version-unsupported`. Yarn externalParameters in signed provenance are
  likewise bounded to exact v4/v5 versions, matching the revised verification policy.
- Added `devEngines.packageManager` as a uniform package-manager selection source (ADR 0091): any
  supported package manager, including pnpm 10.x and Yarn Berry v4/v5, is selectable from either the
  top-level `packageManager` field or `devEngines.packageManager` under each manager's descriptor
  contract (exact versions for pnpm and Yarn; npm declarations stay non-authoritative per ADR 0095),
  while `yarn.lock`-only inference remains rejected.
- Added optional integrity digest declarations to pnpm and Yarn package-manager descriptors (ADRs
  0092-0094): either manifest field may carry one `+<algorithm>.<hex>` suffix (`sha256`, `sha384`,
  or `sha512` with lowercase hex), which the builder verifies against the acquired distribution
  evidence and reconciles fail-closed before install, rejecting a malformed suffix with
  `windlass.verify.error.package-manager-digest-malformed` and a disagreement with
  `windlass.verify.error.package-manager-digest-mismatch`. The recorded
  `package-manager-distribution` evidence keeps its source-native observed authority, and provenance
  renders the declared descriptor verbatim.
- Added npm declaration semantics per ADR 0095: an npm version declared in either manifest field is
  accepted as non-authoritative input, and a declaration that does not match the toolchain npm emits
  a `windlass.verify.warning.npm-version-mismatch` warning without failing the build; an npm
  descriptor carrying a digest suffix is rejected, and SemVer build metadata on pnpm and Yarn
  descriptors that is not a grammar-valid digest suffix is likewise rejected with
  `windlass.verify.error.package-manager-digest-malformed`.
- Added a Go testing and fuzzing guide (`docs/testing-guide.md`) defining test organization,
  security-negative testing, quality gates, and the fuzzing policy for trust-boundary parsers.
- Added property-based fuzz targets for all trust-boundary parsers and validators (attestation
  bundle parsing, verification policy decoding, handoff contracts, npm provenance inputs, registry
  and OIDC response decoding, workspace and package-manager selection parsing, identity and digest
  validators, and workflow decoding), with seed corpora ported from existing negative tests, a
  30-second per-target fuzz smoke job on pull requests, and a scheduled weekly long-run fuzz
  workflow that uploads the fuzz corpus as an artifact.
- Added mise packageManager-field provisioning for the development Node.js runtime (24.21.0) and
  pnpm (12.10.1), with both versions declared solely in `package.json` (`devEngines.runtime` and
  `devEngines.packageManager`) and Corepack removed from the bootstrap; requires mise v2026.8.7 or
  newer.
- Added the credential-configuration preflight to the JS/TS npm build stage (ADR 0034): before
  install, `.npmrc` and `.yarnrc.yml` at the package-manager root and the selected package directory
  are scanned line-wise in both `key=value` and YAML `key: value` forms, and entries with
  credential-shaped keys (`_auth`, `authtoken`, `authident`, or keys ending in `username`,
  `_password`, `httpskey`, or `httpscert`, case-insensitive) fail closed; a project `.corepack.env`
  at the package-manager root is never loaded because every Corepack-path process runs with
  `COREPACK_ENV_FILE=0`, and its presence produces a non-fatal warning (ADR 0096); a `.corepack.env`
  that would ship inside the packed tarball is rejected post-pack with
  `windlass.verify.error.packed-corepack-env-prohibited` (ADR 0097).
- Added the publish-stage npm CLI pin (ADRs 0082-0084): the publish job provisions a pinned npm CLI
  (initially npm 11.21.0, the latest 11-line release carrying the upstream npm/cli#9882
  `--provenance-file` fix) from a digest-verified npm registry tarball before any npm invocation in
  the job, selected from a reviewed allowlist whose single shared pin-data source is
  `internal/toolchainpins/publish-npm.json`; provisioning failures (download, digest mismatch,
  unsafe archive, post-extraction version mismatch) fail closed before any npm execution and before
  any registry mutation.
- Added the build-stage Node.js and npm toolchain pair pin (ADR 0085): the exact Node.js 24 patch
  and its expected bundled npm (currently Node.js 24.21.0 and npm 11.19.0) are recorded in
  `internal/toolchainpins/node-runtime.json` and asserted before the first npm invocation, with a
  mismatch failing closed with `windlass.verify.error.toolchain-pair-mismatch`.
- Added publish-side npm pin validation replacing the prior build/publish npm exact-equality check:
  the publish job's npm version must equal the allowlist pin, a mismatch fails closed with
  `windlass.verify.error.publish-npm-pin-mismatch` before any registry mutation, and the observed
  version is recorded in the publish job's outcome report as
  `diagnostic_metadata.publish_npm_version` on success and on failure whenever observed.
- Added the `pin-check` drift gate to CI, which fails on any drift between the machine-readable
  toolchain pin records (`internal/toolchainpins/publish-npm.json` and
  `internal/toolchainpins/node-runtime.json`) and their human-readable specification tables.

### Security

- [CVE-2026-33818](https://osv.dev/vulnerability/CVE-2026-33818),
  [CVE-2026-39821](https://osv.dev/vulnerability/CVE-2026-39821),
  [CVE-2026-39822](https://osv.dev/vulnerability/CVE-2026-39822),
  [CVE-2026-46600](https://osv.dev/vulnerability/CVE-2026-46600),
  [CVE-2026-56853](https://osv.dev/vulnerability/CVE-2026-56853),
  [CVE-2026-56858](https://osv.dev/vulnerability/CVE-2026-56858),
  [CVE-2026-56859](https://osv.dev/vulnerability/CVE-2026-56859),
  [CVE-2026-56860](https://osv.dev/vulnerability/CVE-2026-56860), and
  [CVE-2026-56862](https://osv.dev/vulnerability/CVE-2026-56862): Pin the module Go directive at
  1.27.2 or newer so that source builds are not exposed to these Go standard library
  vulnerabilities, namely a root escape via symlink plus trailing slash in `os`, a Punycode label
  validation failure in the vendored `golang.org/x/net/idna` copy, a panic on parsing invalid SVCB
  or HTTPS DNS records in the vendored `golang.org/x/net/dns/dnsmessage` copy, missing recursion
  depth limits in `encoding/asn1` and `encoding/xml`, a missing `ReadHeaderTimeout` on the
  unencrypted HTTP/2 upgrade check in `net/http`, unlimited post-handshake messages in `crypto/tls`,
  incorrect JavaScript regexp context tracking in `html/template`, and quadratic complexity in
  `net/url` path resolution. The two x/net-originated flaws (CVE-2026-39821 and CVE-2026-46600) also
  affect the standalone `golang.org/x/net` module; that side is covered by the x/net pin in the next
  entry.
- [CVE-2026-78659](https://osv.dev/vulnerability/CVE-2026-78659),
  [CVE-2026-78660](https://osv.dev/vulnerability/CVE-2026-78660),
  [CVE-2026-78663](https://osv.dev/vulnerability/CVE-2026-78663),
  [CVE-2026-78669](https://osv.dev/vulnerability/CVE-2026-78669), and
  [CVE-2026-97032](https://osv.dev/vulnerability/CVE-2026-97032): Pin the module Go directive at
  1.27.2 or newer and `golang.org/x/net` at v0.60.0 or newer, because these HTTP/2 vulnerabilities
  each affect both `net/http` in the standard library and `golang.org/x/net`, namely server memory
  exhaustion through Trailer headers, the transport accepting malformed framing-related headers,
  excessive CPU consumption from repeated initial window changes, a double flow-control refund on
  server streams, and a server crash from an HPACK encoder race.
- [CVE-2026-56864](https://osv.dev/vulnerability/CVE-2026-56864) and
  [CVE-2026-56865](https://osv.dev/vulnerability/CVE-2026-56865): Pin `golang.org/x/mod` at v0.40.0
  or newer so that builds are not exposed to two checksum-database flaws, namely a transparency-log
  tile verification bypass in `sumdb/tlog` and acceptance of unrelated, unauthenticated hashes in
  `sumdb` lookups.
- [CVE-2026-56855](https://osv.dev/vulnerability/CVE-2026-56855) and
  [CVE-2026-78662](https://osv.dev/vulnerability/CVE-2026-78662): Pin `golang.org/x/crypto` at
  v0.56.0 or newer so that builds are not exposed to two SSH denial-of-service flaws, deadlocks on
  undecided and on established channels in `golang.org/x/crypto/ssh`.
- [CVE-2026-84445](https://osv.dev/vulnerability/CVE-2026-84445): Pin `google.golang.org/grpc` at
  v1.83.2 or newer so that builds are not exposed to an xDS server panic on requests missing the
  authority or Host header; the fix also shipped in the v1.84.0 stable release, which the module now
  carries (with `grpc-ecosystem/grpc-gateway/v2` at v2.31.0).
