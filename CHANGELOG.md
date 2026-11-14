# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Release headings use Human
Era five-digit years (e.g., `## [0.1.0] - 12026-06-13`).

## [Unreleased]

### Added

- Added Go-native keyless DSSE signing for npm provenance, with digest-verified handoffs and offline
  statement verification before bundle upload.
- Added the public npm-only reusable workflow with trusted-publisher preflights, serialized
  publishes, immutable artifact handoffs, persistent outcome reports, and bounded registry
  read-back. The workflow distinguishes delayed attestations, rejected publications, and unexpected
  registry state, and converges safely on re-run.
- Added official built-with, verified-with, and logo badge SVGs under `assets/badges/`, each in four
  shields.io-compatible styles, with copy-ready Markdown and HTML snippets in the README.
- Expanded and synchronized the English and Korean READMEs with SLSA and provenance primers,
  security and trust guidance, profile comparisons, badge usage, and contributor and license
  information.
- Added a fail-closed package-manager support boundary: npm uses the pinned Node.js toolchain, while
  Corepack provisions pnpm 10.x and 11.x and Yarn Berry v4/v5; pnpm 12+ and Yarn 6+ are rejected
  before install.
- Added support for standalone root packages with settings-only `pnpm-workspace.yaml` files that
  omit `packages`.
- Added an optional tags-only `source-ref` input for fixed-pipeline release retries, with
  built-source provenance and signed invocation context.
- Added redacted Node.js and npm configuration diagnostics to build and publish jobs, plus non-npm
  package-manager configuration diagnostics in the build job, to aid trusted-publishing and
  provenance triage.
- Added `devEngines.packageManager` as an alternative package-manager selection source, with exact
  versions for pnpm and Yarn; `yarn.lock`-only inference remains rejected.
- Added optional SHA-256, SHA-384, and SHA-512 integrity digests for pnpm and Yarn descriptors.
  Malformed or mismatched digests fail closed before install, while provenance preserves the
  declared descriptor. Digest suffixes on npm descriptors and malformed digest-like metadata are
  rejected.
- Added non-authoritative npm version declarations, with a warning rather than a build failure when
  the declared and toolchain versions differ.
- Added a Go testing and fuzzing guide in `docs/testing-guide.md`, plus property-based fuzzing for
  trust-boundary parsers and validators, PR smoke tests, and weekly long-run fuzzing with corpus
  artifacts.
- Added mise provisioning for the development Node.js and pnpm versions declared in `package.json`,
  removing Corepack from the development bootstrap; mise v2026.8.7 or newer is required.
- Added a fail-closed credential-configuration preflight for `.npmrc` and `.yarnrc.yml` before
  install. Corepack environment files are never loaded, and `.corepack.env` is rejected if it would
  be packed into the published tarball.
- Added digest-verified provisioning and validation of a pinned npm CLI for publishes. Provisioning
  or pin mismatches fail closed before registry mutation, and the observed npm version is recorded
  in outcome reports.
- Added digest-verified Node.js runtime provisioning and a Node.js/npm pair check for governed build
  jobs, failing closed on unsafe archives, unexpected layouts, or toolchain mismatches before build,
  install, pack, or publish commands run.
- Added the finalized JS/TS npm SLSA provenance contract, binding workflow and invocation context
  and recording source, manifest, lockfile, runtime, package-manager, and runner-image dependencies.
  It includes embedded manifest evidence for offline verification, observed tool versions, and
  build-script dispositions. Invalid publish inputs and conflicts with authenticated `publishConfig`
  produce classified diagnostics.
- Added redacted human-readable diagnostics for external processes, npm publish failures, and
  registry read-back, with bounded stderr relay and recognized publish-rejection summaries.
  Diagnostics remain non-authoritative and cannot affect verification or publish outcomes.

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
- [CVE-2026-78661](https://osv.dev/vulnerability/CVE-2026-78661) and
  [CVE-2026-97045](https://osv.dev/vulnerability/CVE-2026-97045): Pin `golang.org/x/net` at v0.62.0
  or newer so that builds are not exposed to quadratic complexity when coalescing HTML text nodes or
  incomplete ancestor namespace checks when rendering raw text in `html`.
- [CVE-2026-116587](https://osv.dev/vulnerability/CVE-2026-116587): Pin
  `github.com/go-jose/go-jose/v4` at v4.1.6 or newer so that builds are not exposed to excessive CPU
  consumption from crafted RSA JWKs.
- [CVE-2026-105481](https://osv.dev/vulnerability/CVE-2026-105481),
  [CVE-2026-105482](https://osv.dev/vulnerability/CVE-2026-105482),
  [CVE-2026-105483](https://osv.dev/vulnerability/CVE-2026-105483),
  [CVE-2026-106069](https://osv.dev/vulnerability/CVE-2026-106069),
  [CVE-2026-106072](https://osv.dev/vulnerability/CVE-2026-106072),
  [CVE-2026-106073](https://osv.dev/vulnerability/CVE-2026-106073),
  [CVE-2026-106074](https://osv.dev/vulnerability/CVE-2026-106074), and
  [CVE-2026-111497](https://osv.dev/vulnerability/CVE-2026-111497): Pin the production and
  development Node.js runtime at 24.22.1 or newer so that builds and releases are not exposed to
  keep-alive socket response confusion, DNS hostname truncation on NUL characters, three TLS
  certificate hostname-validation bypasses involving non-canonical IPv4 forms, a permission-model
  bypass in `fs.realpathSync()`, reused cryptographic entropy caches in startup snapshots and single
  executable applications, and an out-of-bounds write in the Brotli decoder.
