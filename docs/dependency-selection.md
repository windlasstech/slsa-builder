# Trusted Core Dependency Selection

This document records the initial third-party Go modules approved for the trusted core. The review
was performed on 12026-08-05 against ADRs 0004, 0061, and 0069 and the
[verification policy](architecture/verification-policy-and-fixtures.md). It selects dependencies; it
does not implement production verification, canonicalization, or workflow checks.

## Wave 2 review on 12026-10-30

### Current scoped result

After clearing the orphaned download-lock owner, the proxy-backed scoped transaction and
`go mod tidy -go=1.27.3` succeeded. Eleven selected versions were upgraded; the twelfth target, xz
v0.5.17, was already retained. No parent upgrade was deferred. Scope is the exact module paths
explicitly required by baseline `HEAD:go.mod`, plus xz; upstream-test-only update hints were not
independent upgrade arguments. The 12-line selection has SHA-256
`f576576a3740f4bbf8dc5fcfc770de0d491f820e105b638bf46c1e6da92a89ca`.

The final graph contains 393 entries including this module (392 dependencies). Compared with the
recovered graph, 67 dependency versions changed, none were added, and
`github.com/hashicorp/go-hclog v1.6.3` left the graph. Tidy retained no new explicit requirement
paths, so no additional in-scope upgrade targets were introduced. All 12 exact scoped targets match.
Every dependency has a recorded timestamp at or before `2026-10-22T18:42:13Z`; the graph contains no
loading errors or replacements. This audit checks the selected graph, not every available
upstream-test-only update.

Current JCS is `v0.0.0-20261011152517-32a48195b9b2`. The goccy pin remains
`v1.19.3-0.20260407131736-edee2f91616c`, and xz remains `v0.5.17`. Go 1.27.3 was selected through
both PATH and GOROOT with automatic toolchain switching disabled. After tidy, read-only graph
loading, unchanged `go mod tidy -diff`, `go mod verify`, and `go build -mod=readonly ./...` passed.
`go test ./...` remains RED with exactly the same top-level failing test names as the
recovered-state run, covering staged production Node/npm, xz decoder, and fixture work. No source or
fixture changes were used to hide those failures.

The separate `go list -m -retracted -json all` query stopped with
`updates to go.sum needed, disabled by -mod=readonly`. Therefore, fresh full-graph retraction review
remains incomplete, despite the complete timestamp audit. Full license, API, vector, Dependency
Review, and OSV review also remain merge gates; successful build and package tests are not
substitutes for them. Proxy cleanup confirmed process exit, no loopback listener, and no owners of
the four previously blocked module cache locks.

### Historical recovery and scoped retry attempts

The failed broad upgrade described below was recovered before the scoped retry. Current initial
upgrade scope is the exact module paths explicitly required by `HEAD:go.mod`, plus
`github.com/ulikunitz/xz`. Upstream-test-only modules are not independent upgrade targets. New
requirements or package providers retained by a successful tidy require their own eligibility review
before becoming additional targets. Full-graph update hints are not upgrade arguments.

Filtering the reviewed selection by this scope produced 12 targets (including the already-retained
xz pin), with SHA-256 `f576576a3740f4bbf8dc5fcfc770de0d491f820e105b638bf46c1e6da92a89ca`. Websocket
was outside the explicit baseline requirements and was not independently targeted. The single-array
scoped `go get` attempts timed out during downloads at 200 and 600 seconds, without applying the
scoped versions. No parent upgrade was deferred without dependency evidence. The last retry logged
downloads for AWS KMS, enterprise-certificate-proxy, gax-go, and Google API.

A subsequent loopback CONNECT-proxy attempt completed the read-only JCS metadata probe, but did not
complete the scoped transaction. Runtime inspection found an orphaned earlier `go get` holding the
download lock files for those four modules. The orphan remained alive after SIGTERM; the later
attempt therefore also waited without completing. SIGKILL was issued to that task-owned orphan after
the second inspection. This is evidence of a local cache-lock blocker, not proof that the proxy
cannot fetch those module archives. Both temporary proxies were stopped and waited for, and their
cleanup logs confirm that their loopback listeners were absent. No scoped upgrade or final
upgraded-graph validation is claimed from these attempts.

The recovered graph has 394 entries including the main module. Recovery validation passed
`go mod tidy -go=1.27.3`, a subsequent unchanged `go mod tidy -diff`, `go mod verify`, and
`go build -mod=readonly ./...`. Build required both PATH and GOROOT to select the installed Go
1.27.3 toolchain; the inherited GOROOT still pointed to 1.27.2. Tests remain RED. These checks
validate the recovered graph, not completion of the scoped upgrades. Scoped target GREEN, full-graph
cutoff review, and upgrade compatibility review remain pending.

At that recovered stage, JCS remained `v0.0.0-20241213102144-19d51d7fe467`; goccy remained
`v1.19.3-0.20260407131736-edee2f91616c`, and xz remains `v0.5.17`.

### Historical all-module attempts (not the current graph)

The historical selection and rationale below remain unchanged. The attempted update uses the fixed
cutoff `2026-10-22T18:42:13Z` and the approved 175-line selection with SHA-256
`769d70c7b03610652f4f522db9627492191259ef8e8c44292a9d5836d2386395`. The combined `go get`
transaction failed before applying the selected upgrades:

- The selection requests `github.com/gorilla/websocket v1.5.3`.
- `github.com/google/trillian v1.8.0` and `github.com/prometheus/prometheus v0.315.1` require
  `github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674`.
- `github.com/sigstore/rekor v1.5.5` also requires that newer pseudo-version indirectly.

These exact pins cannot coexist under minimal version selection. The reviewed remedy replaced only
websocket with the required pseudo-version. Its official proxy timestamp is `2025-03-19T13:29:07Z`,
before the cutoff, and `go list -m -retracted` reported no retraction. It remains in semantic major
v1 and orders above `v1.5.3`. The revised 175-line selection has SHA-256
`b8d0fbcc50a648ea43de772e8690fca806f20e7566563506c864a189d00a08e2`.

The revised `go get` succeeded, and all 175 selected versions matched that attempt's captured graph.
However, the first `go mod tidy` failed, so that combined upgrade was **not resolved**:

- `github.com/ysmood/got v0.43.0` lacks `lib/lcs`, imported by `github.com/go-rod/rod` through the
  Sigstore OAuth-flow tests.
- `go.etcd.io/etcd/tests/v3 v3.7.2` requires `go.etcd.io/etcd/cache/v3 v3.7.2`, but the official
  proxy returns 404 (unknown revision) for that cache version. The dependency's local replacement is
  not inherited by this module; no replacement was added here.

That attempt's error-tolerant graph capture contained 846 entries including the main module and one
module resolution error. A normal full-graph query failed on that error. The fixed-cutoff analyzer
attempt timed out. Zero eligible updates, zero unresolved modules, a stable tidied graph, and a
GREEN target helper have not been established. The four history-derived pprof/genproto pins, the
goccy pin, and xz v0.5.17 are retained. No cutoff or major boundary was widened.

That attempt installed JCS `v0.0.0-20261011152517-32a48195b9b2` before recovery. The goccy
pseudo-version remained `v1.19.3-0.20260407131736-edee2f91616c`. License, API, offline-network,
vector, and full transitive review of the proposed combined graph remain pending, as do Dependency
Review and OSV gates. The successful package tests do not establish full transitive graph
compatibility.

The module Go directive and the three production workflow Go pins are now `1.27.3`. `go test ./...`
compiled the packages on Go 1.27.3, but remains RED for the intentionally staged Node/npm pin
changes, xz decoder implementation, and missing npm compatibility fixtures. Attestation, canonical
JSON, dependency proof, and workflow-check packages passed. The top-level failing test names match
the pre-remedy run; the tidy and graph-loading failures above are new dependency blockers, not
intentional production RED tests. No production source, tests, or fixtures were changed in this wave
to hide those failures.

`github.com/ulikunitz/xz v0.5.17` is retained for the staged behavioral tests. Its module declares
Go 1.20 and no third-party requirements; its downloaded license is BSD-3-Clause. Its documented
Go-native `NewReader` and `NewWriter` APIs avoid a C-library dependency. `go mod tidy` classifies
the existing test import as direct, which does **not** mean the Wave 3 production decoder import is
complete. Decoder integration and its acceptance vectors remain pending.

## Approved direct modules (historical selection)

| Capability            | Module and exact version                                                         | License    |
| --------------------- | -------------------------------------------------------------------------------- | ---------- |
| Sigstore verification | `github.com/sigstore/sigstore-go v1.3.0`                                         | Apache-2.0 |
| RFC 8785 JCS          | `github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467` | Apache-2.0 |
| Workflow YAML parsing | `github.com/goccy/go-yaml v1.19.3-0.20260407131736-edee2f91616c`                 | MIT        |

Apache-2.0 and MIT are OSI-approved licenses. No direct dependency with an unapproved, ambiguous, or
copyleft license was accepted.

The module graph is intentionally pinned by `go.mod` and `go.sum`. At selection time,
`go list -m all` resolves 372 entries including this module, or 371 dependencies. `go.mod` retains
80 indirect requirements needed by the imported Sigstore proof APIs. The JCS module adds no new
module to the build list because this exact version is already required by `sigstore-go`; the JCS
and YAML modules otherwise declare no third-party module requirements. The Sigstore footprint is
large: its `v1.3.0` module declares 88 module requirements, which expand through the cryptographic,
TUF, Rekor, protobuf, and in-toto stack. That cost is accepted only for the official verification
implementation. Dependency Review and OSV Scanner are required gates for the full transitive graph.

## Sigstore: `github.com/sigstore/sigstore-go v1.3.0`

### Rationale

- **License:** Apache-2.0, confirmed by the repository
  [license](https://github.com/sigstore/sigstore-go/blob/v1.3.0/LICENSE).
- **Maintainer and reputation:** this is the official Go verification library maintained by the
  Sigstore project. Its README describes the library as stable, production-ready, and covered by
  Sigstore conformance testing.
- **Release and compatibility:** `v1.3.0` was released on 12026-07-30. The preceding `v1.2.0`,
  `v1.2.1`, and `v1.2.2` releases were published during 12026-06 and 12026-07, showing active
  maintenance. Its module requires Go 1.25.8, which is compatible with the repository's Go 1.26.6
  toolchain.
- **Capability fit:** the library parses Sigstore bundles and verifies message signatures or DSSE
  envelopes, Fulcio certificate chains, embedded SCTs, Rekor inclusion proofs or SETs, observer
  timestamps including SET-covered integrated time, and optional RFC 3161 timestamps. The selected
  verifier options are `WithSignedCertificateTimestamps(1)`, `WithTransparencyLog(1)`, and
  `WithObserverTimestamps(1)`; an RFC 3161 timestamp remains additive rather than a replacement for
  Rekor evidence.
- **Footprint:** this is the only approved direct dependency with a substantial transitive graph.
  Reimplementing its cryptographic and transparency-log behavior would be less auditable and would
  conflict with ADR 0069's alignment with Sigstore semantics.

### Online and offline trust-root paths

The two modes have separate acquisition paths:

1. **Online:** call `root.FetchTrustedRoot()`. The implementation constructs the Sigstore TUF client
   with public-good defaults and authenticates the current `trusted_root.json`. A TUF error is
   fatal; production code must not fall back to a pin.
2. **Offline:** after application-level SHA-256, TUF-repository identity, freshness, and forbidden
   environment-override checks, read the pinned file locally and call `root.NewTrustedRootFromJSON`.
   Pass the returned `root.TrustedMaterial` to `verify.NewVerifier` with the SCT, transparency-log,
   and observer-timestamp options above.

The no-network property is provided by API separation, not by silently trusting a network-capable
mode. `root.NewTrustedRootFromJSON` consumes bytes and `verify.NewVerifier` consumes only
`root.TrustedMaterial`; neither accepts an HTTP client, transport, URL, Rekor client, Fulcio client,
or TUF client. Bundle verification uses the supplied bundle and trusted material. Network-capable
fetching is isolated in `pkg/tuf` and `root.FetchTrustedRoot`, which the offline path never calls.
No transport injection wrapper is therefore required for cryptographic verification. The future
production mode dispatcher must keep acquisition and verification separate and must test that the
offline path cannot reach the TUF package or a log operator.

The API proof test replaces Go's default HTTP transport with a deny transport and constructs a
verifier from local trusted-root JSON without calling TUF. The source-level API separation above is
the complete no-network argument; the transport guard catches an accidental default-HTTP regression.
The test's empty root is deliberately not a successful cryptographic fixture. Later fixture tasks
will use governed public-good root and bundle bytes to prove accepted and rejected verification
results.

Relevant upstream APIs and implementation evidence:

- [`root.NewTrustedRootFromJSON`](https://pkg.go.dev/github.com/sigstore/sigstore-go/pkg/root#NewTrustedRootFromJSON)
  and
  [`root.FetchTrustedRoot`](https://pkg.go.dev/github.com/sigstore/sigstore-go/pkg/root#FetchTrustedRoot)
- [`verify.NewVerifier`](https://pkg.go.dev/github.com/sigstore/sigstore-go/pkg/verify#NewVerifier)
  and its SCT, transparency-log, and timestamp options
- [verification guide](https://github.com/sigstore/sigstore-go/blob/v1.3.0/docs/verification.md)
- [TUF options](https://github.com/sigstore/sigstore-go/blob/v1.3.0/pkg/tuf/options.go), including
  the air-gapped trusted-root-file path

## RFC 8785 JCS: `github.com/cyberphone/json-canonicalization`

### Rationale

- **License:** Apache-2.0, confirmed by the repository
  [license](https://github.com/cyberphone/json-canonicalization/blob/19d51d7fe467d4706a3ff08adf8a748f29fc21e0/LICENSE).
- **Maintainer and reputation:** this is the reference implementation repository maintained by RFC
  8785 editor Anders Rundgren and linked from RFC 8785 Appendix G. The selected pseudo-version pins
  commit `19d51d7fe467` exactly rather than following a branch.
- **Release cadence:** the repository is commit-driven rather than release-tag-driven. That is a
  maintenance disadvantage, but the exact pseudo-version is reproducible and the implementation's
  reference status outweighs the absence of tagged releases for this narrow algorithm.
- **Capability fit:** `jsoncanonicalizer.Transform` accepts raw JSON bytes and produces RFC 8785
  canonical UTF-8 bytes, including ECMAScript-compatible number formatting, UTF-16 property-name
  ordering, and required string escaping. It rejects duplicate object names rather than first
  decoding into a lossy map.
- **Footprint:** it has no third-party module requirements and is already present at the same
  version in the Sigstore dependency graph, so approving it directly adds no new transitive module.

ADR 0061 remains broader than this choice. Signed SLSA Statements are not required to be JCS. Their
raw payload bytes must be scanned for duplicate object names at every depth before semantic parsing;
that production scanner belongs to a later task. Go's `encoding/json` cannot perform that check
because duplicate keys are processed in order and later values replace or merge earlier values. For
values that do require JCS, duplicate rejection must happen before canonicalization as required by
the verification policy. The selected transformer provides defense in depth but does not replace the
general duplicate-aware Statement parser.

## YAML: `github.com/goccy/go-yaml v1.19.3-0.20260407131736-edee2f91616c`

### Rationale

- **License:** MIT, confirmed by the repository
  [license](https://github.com/goccy/go-yaml/blob/edee2f91616c6d73112a13e7c0dbde72ce938877/LICENSE).
- **Maintainer and reputation:** this is an established, actively maintained Go YAML implementation
  with parser and AST APIs used independently of struct unmarshalling.
- **Release and compatibility:** the selected pseudo-version pins upstream commit `edee2f91616c`
  exactly, keeping the pin reproducible under the same convention as the JCS module's pseudo-version
  approval above. It is the earliest upstream revision containing the fix for goccy/go-yaml issue
  #735, merged as PR #862 on 12026-04-07: a fuzz-discovered nil-pointer panic in
  `Decoder.decodeSlice` when a tagged scalar maps into a Go slice field, reachable from
  attacker-controlled workflow YAML. No tagged release contains the fix yet; `v1.19.2` is the latest
  tag and predates it. The pin is five commits ahead of the `v1.19.2` tag, and those commits
  additionally touch parser, token, context, and encoder files, which was accepted after review of
  the upstream diff. The module still requires Go 1.21, which is compatible with this repository's
  Go 1.26.6 toolchain. When a tagged release containing PR #862 ships, the pin should move to that
  release.
- **Capability fit:** `parser.ParseBytes` preserves a syntax tree suitable for static GitHub Actions
  conformance checks. Duplicate mapping keys are rejected by default; the trusted core must not use
  the `AllowDuplicateMapKey` opt-out. The parser can inspect workflow structure without coercing it
  through an application struct first.
- **Footprint:** the module has no third-party module requirements.

This dependency parses YAML syntax only. It does not define GitHub Actions semantics; the future
workflow checker must apply the repository's closed conformance rules to the AST.

The version review required by this document's final paragraph was repeated on 12026-08-17 for this
revision. The license was re-confirmed MIT at the pinned commit, no new module requirements were
introduced, and the Dependency Review and OSV gates are unchanged.

## Official test-vector inventory

### Sigstore

`sigstore-go v1.3.0` provides:

- the Sigstore verifier conformance runner in
  [`test/conformance`](https://github.com/sigstore/sigstore-go/tree/v1.3.0/test/conformance);
- end-to-end verification tests in
  [`test/e2e`](https://github.com/sigstore/sigstore-go/tree/v1.3.0/test/e2e);
- package tests for bundle parsing, trust roots, TUF, SCT, Rekor/tlog, TSA, DSSE, and verifier
  policy under [`pkg`](https://github.com/sigstore/sigstore-go/tree/v1.3.0/pkg);
- bundle, trusted-root, and public-key vectors in
  [`pkg/testing/data`](https://github.com/sigstore/sigstore-go/tree/v1.3.0/pkg/testing/data); and
- public-good bundle and trusted-root examples in
  [`examples`](https://github.com/sigstore/sigstore-go/tree/v1.3.0/examples).

Later fixture work should copy only the upstream vectors needed to prove the accepted bundle and the
required missing-SCT, missing-or-mismatched-Rekor, invalid-signing-time, and ungoverned-root
rejections, preserving source commit and license metadata.

### RFC 8785 JCS

The authoritative inventory is the reference repository's
[`testdata`](https://github.com/cyberphone/json-canonicalization/tree/19d51d7fe467d4706a3ff08adf8a748f29fc21e0/testdata):
paired `input` and `output` vectors cover arrays, French text, structures, Unicode property
ordering, primitive values, and unusual string and number cases. Its `outhex` data records canonical
UTF-8 bytes where rendered text is ambiguous. RFC 8785 itself supplies the section 3.2.2
canonicalization sample used by `dependency_proof_test.go` and the Appendix B IEEE-754 number
serialization samples.

### YAML

`go-yaml v1.19.3-0.20260407131736-edee2f91616c` includes an integration harness for the
community-maintained [`yaml-test-suite`](https://github.com/yaml/yaml-test-suite) in
[`yaml_test_suite_test.go`](https://github.com/goccy/go-yaml/blob/edee2f91616c6d73112a13e7c0dbde72ce938877/yaml_test_suite_test.go).
The repository also carries unit tests for lexer, parser, AST, decoder, encoder, anchors, aliases,
merge keys, comments, and duplicate mapping keys. Workflow-specific accepted and rejected fixtures
will be project-owned because the YAML standard suite does not define GitHub Actions semantics.

## Rejected candidates

| Candidate                                        | Reason rejected                                                                                                                                                                 |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `github.com/sigstore/cosign/v2`                  | CLI-oriented and materially heavier; its legacy offline flag is deprecated, while `sigstore-go` exposes the required library boundary directly.                                 |
| `github.com/sigstore/sigstore` alone             | Lower-level primitives do not replace `sigstore-go`'s bundle and policy verification orchestration. It remains an indirect dependency of the selected official library.         |
| Older `sigstore-go` releases                     | Superseded by compatible stable `v1.3.0`; no benefit justifies starting on an older verifier.                                                                                   |
| `github.com/deszhou/jcs v1.0.0`                  | Strict and tagged, but newly maintained and not the RFC editor's reference repository; it would add a second implementation while Sigstore already brings the reference module. |
| `github.com/gowebpki/jcs v1.0.1`                 | Stable fork, but less direct provenance to the RFC reference implementation and its official vectors.                                                                           |
| `github.com/ucarion/jcs v0.1.2`                  | Operates on already-decoded Go values and documents incomplete surrogate validation, weakening raw-input assurance.                                                             |
| `github.com/lattice-substrate/json-canon v0.3.4` | Pre-v1 and Linux-specific; unnecessary for a portable trusted core.                                                                                                             |
| `go.yaml.in/yaml/v4 v4.0.0-rc.6`                 | Officially maintained but still a release candidate, so it is not suitable for the initial stable pin.                                                                          |
| `go.yaml.in/yaml/v3 v3.0.5`                      | Stable but frozen for legacy/security fixes and less parser/AST-focused for static conformance than `goccy/go-yaml`.                                                            |

Any future version change repeats license, API, offline-network, vector, transitive, Dependency
Review, and OSV review before merge.
