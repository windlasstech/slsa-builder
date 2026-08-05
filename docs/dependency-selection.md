# Trusted Core Dependency Selection

This document records the initial third-party Go modules approved for the trusted core. The review
was performed on 12026-08-05 against ADRs 0004, 0061, and 0069 and the
[verification policy](architecture/verification-policy-and-fixtures.md). It selects dependencies; it
does not implement production verification, canonicalization, or workflow checks.

## Approved direct modules

| Capability            | Module and exact version                                                         | License    |
| --------------------- | -------------------------------------------------------------------------------- | ---------- |
| Sigstore verification | `github.com/sigstore/sigstore-go v1.3.0`                                         | Apache-2.0 |
| RFC 8785 JCS          | `github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467` | Apache-2.0 |
| Workflow YAML parsing | `github.com/goccy/go-yaml v1.19.2`                                               | MIT        |

Apache-2.0 and MIT are OSI-approved licenses. No direct dependency with an unapproved, ambiguous, or
copyleft license was accepted.

The module graph is intentionally pinned by `go.mod` and `go.sum`. At selection time,
`go list -m all` resolves 369 entries including this module, or 368 dependencies. `go.mod` retains
69 indirect requirements needed by the imported Sigstore proof APIs. The JCS module adds no new
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
  maintenance. Its module requires Go 1.25.8, which is compatible with the repository's Go 1.26.4
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

## YAML: `github.com/goccy/go-yaml v1.19.2`

### Rationale

- **License:** MIT, confirmed by the repository
  [license](https://github.com/goccy/go-yaml/blob/v1.19.2/LICENSE).
- **Maintainer and reputation:** this is an established, actively maintained Go YAML implementation
  with parser and AST APIs used independently of struct unmarshalling.
- **Release and compatibility:** `v1.19.2` is a stable release and its module requires Go 1.21,
  which is compatible with Go 1.26.4.
- **Capability fit:** `parser.ParseBytes` preserves a syntax tree suitable for static GitHub Actions
  conformance checks. Duplicate mapping keys are rejected by default; the trusted core must not use
  the `AllowDuplicateMapKey` opt-out. The parser can inspect workflow structure without coercing it
  through an application struct first.
- **Footprint:** the module has no third-party module requirements.

This dependency parses YAML syntax only. It does not define GitHub Actions semantics; the future
workflow checker must apply the repository's closed conformance rules to the AST.

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

`go-yaml v1.19.2` includes an integration harness for the community-maintained
[`yaml-test-suite`](https://github.com/yaml/yaml-test-suite) in
[`yaml_test_suite_test.go`](https://github.com/goccy/go-yaml/blob/v1.19.2/yaml_test_suite_test.go).
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
