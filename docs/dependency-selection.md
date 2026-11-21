# Trusted Core Dependency Selection

This document records the current toolchain artifacts and third-party Go modules approved for the
trusted core, together with the standing policy for selecting and reviewing dependencies. It selects
dependencies; it does not implement production verification, canonicalization, workflow checks, or
runtime provisioning. ADRs 0004, 0061, 0069, 0077, 0113, and 0114 and the
[verification policy](architecture/verification-policy-and-fixtures.md) remain governing context.

## Current Selection

### Toolchain and Runtime Archive

The trusted core uses Go 1.27.3. The JS/TS npm build toolchain pairs Node.js 24.22.1 with its
bundled npm 11.19.1 and asserts that pair before first use. The publish-stage npm CLI is selected
independently as npm 11.21.1; it does not inherit or replace the build toolchain's bundled npm. ADR
0114 records it in signed `builderDependencies` as `publish-cli`, with URI `pkg:npm/npm@11.21.1`,
tarball SHA-512
`613ccbe90044257c87713bb426251ced128ca7e9b3949b65eeb18b98445a5d441e14720c73d6ad76be86f4cf42f2f0d66efe347a9a13400ccd2f6fcfb5954385`,
and media type `application/gzip`. Its `digest_authority` annotation is descriptive only.

The Linux x64 Node.js runtime archive is pinned to the official Node.js distribution:

- URL: `https://nodejs.org/download/release/v24.22.1/node-v24.22.1-linux-x64.tar.gz`
- SHA-256: `eb6728e77c576a35162c122c45ccadf224d89ff00abc8dae46cc2897c6002d7c`

Go's standard-library `compress/gzip` package decodes the `.tar.gz` runtime archive.
`github.com/ulikunitz/xz` is not selected and is absent from `go.mod` and `go.sum`.

### Direct Go Modules

| Capability                  | Module and exact version                                                         | License      |
| --------------------------- | -------------------------------------------------------------------------------- | ------------ |
| Sigstore verification       | `github.com/sigstore/sigstore-go v1.4.0`                                         | Apache-2.0   |
| RFC 8785 JCS                | `github.com/cyberphone/json-canonicalization v0.0.0-20261011152517-32a48195b9b2` | Apache-2.0   |
| Workflow YAML parsing       | `github.com/goccy/go-yaml v1.19.3-0.20260407131736-edee2f91616c`                 | MIT          |
| SemVer and module selection | `golang.org/x/mod v0.42.0`                                                       | BSD-3-Clause |

Apache-2.0, MIT, and BSD-3-Clause are OSI-approved licenses. No direct dependency with an
unapproved, ambiguous, or copyleft license is accepted.

Exact versions in `go.mod` and cryptographic checksums in `go.sum` pin the module graph. The JCS and
YAML modules declare no third-party module requirements. Sigstore has a substantial transitive
footprint across cryptographic, TUF, Rekor, protobuf, and in-toto packages. That cost is accepted
only for the official verification implementation, and the full transitive graph remains subject to
Dependency Review and OSV Scanner gates.

## Dependency Selection and Review Policy

Dependency selection and every subsequent version change follow these rules:

1. **Standard library and smallest TCB first:** use the Go standard library when it provides the
   required assurance and behavior. Add third-party code only for a concrete capability gap, and
   prefer the candidate with the smallest auditable trust surface that meets that gap.
2. **Exact, reproducible pins:** record direct requirements at exact versions in `go.mod` and retain
   their authenticated checksums in `go.sum`. A pseudo-version must identify the intended upstream
   commit exactly; branch following is not a selection mechanism.
3. **Complete selection review:** review capability fit, license, maintainer and reputation, release
   and maintenance state, API compatibility, offline-network behavior, official test vectors, and
   the transitive dependency graph. A successful build or package test does not replace any review
   axis.
4. **Mandatory security gates:** Dependency Review and OSV Scanner are mandatory for the complete
   direct and transitive graph before merge. Their results supplement, rather than replace, the
   selection review.
5. **Independent justification for direct additions:** every new direct requirement needs its own
   capability and trust-boundary justification, including requirements or package providers first
   exposed by tidying. It must not become selected merely because another dependency's tests can use
   it.
6. **No selection by update hint alone:** an upstream-test-only update hint is not a selection
   argument. Eligibility, necessity, compatibility, and the full review above must independently
   support the selected version.
7. **Synchronize descriptor-coupled bumps:** a version-only sigstore-go bump must update the module
   graph, compiled signing-adapter descriptor expectations, normative documentation, and byte-exact
   fixtures together. A publish-cli version-and-digest bump must likewise update its pin, compiled
   descriptor expectations, normative documentation, and fixtures together. Any drift between the
   selected dependency, signed descriptor, and governed expectations blocks the change.

## Dependabot companion integration

The internal `dependabot-companion` command synchronizes descriptor-coupled files for a narrow
sigstore-go update. It does not approve a dependency, merge a PR, or replace security review.
Dependency Review remains an independent required check. OSV Scanner, code review, and branch
protection remain unchanged.

### Eligibility and release review

Remote modes accept the actual `workflow_run` JSON payload, not a local dry-run projection. The
source run must be a completed `Lint` pull-request run, with any nonempty conclusion, including
failure. The command binds the PR through the run's single `pull_requests` entry or, when that array
is empty, a unique same-repository open PR for the exact head branch and SHA. PR titles, logs, and
artifacts never identify a dependency or authorize an operation.

Authenticated API reads must establish all of the following:

- The PR is open, authored by `dependabot[bot]`, and has a `dependabot/` head branch in
  `windlasstech/slsa-builder`. Its base is `main`.
- The current trusted checkout SHA equals the PR base SHA. A moved base or head stops the operation.
- The complete changed-file list contains only modified `go.mod` and/or `go.sum` files. Renames,
  duplicate paths, forks, and companion-file edits are rejected. Pagination is bounded at 200 items.
- Exact base/head module bytes pass the existing local transition parser. It admits increases to
  existing Sigstore/in-toto group requirements, including sigstore-go, but not new requirements,
  unrelated transitive changes, directive changes, or replacement directives.
- The candidate has a published, non-draft, non-prerelease GitHub release with a matching tag and
  valid publication date. Tagged commits supply the candidate `go.mod` and both versions' `LICENSE`
  bytes. GitHub must identify both licenses as Apache-2.0, their bytes must be identical, and the
  candidate's declared Go version must fit the trusted base toolchain.
- Remote `plan` and `apply` must also authenticate candidate module content at the release tag's
  peeled commit. Authenticated GitHub commit, tree, and blob objects must bind the complete module
  file set and bytes to that commit, with object identities and tree membership verified. Apply Go
  module-content hashing rules to those bytes and compare the resulting `h1` checksum with the
  candidate's `go.sum` module-content record before deriving `dirHash1`. The separate `/go.mod`
  checksum, a syntactically valid PR-supplied checksum, and release metadata alone are insufficient.
  Missing, truncated, ambiguous, or unauthenticated objects, unsupported module layouts, and
  checksum disagreement fail closed before generation or mutation. Candidate code is never executed.

These checks supply release evidence; they are not a compatibility or security approval. A
maintainer must still review release notes, API compatibility, maintenance state, and the transitive
graph under the selection policy above. Unsupported module transitions produce
`manual-action-required` with no generated files and an empty patch. An unchanged signer produces
`skip`. Invalid event bindings, unavailable API evidence, or unconfirmed remote release/license
evidence fail closed without mutation. Resolve those cases manually rather than broadening the
companion's allowlist to make a PR pass.

### Local dry-run and automatic read-only plan

Local `dry-run` retains its closed event/snapshot contract and does not access GitHub. Remote `plan`
uses `GITHUB_TOKEN` for authenticated reads only. Both modes use the same generator and write only
`plan.json`, `patch.diff`, and allowlisted full file contents under `files/` in a separate output
directory. Existing outputs must match exactly; they are not overwritten with different plans.

Local dry-run remains offline and non-authoritative: its supplied release and checksum snapshots can
exercise deterministic planning but cannot establish remote candidate-content authenticity or
authorize apply. Remote modes must independently obtain the authenticated evidence above.

### Bounded generation and complete file accounting

Generation must enforce aggregate limits before buffering or copying the committed trusted tree, in
addition to per-file limits. The input tree is limited to 10,000 regular files and 128 MiB of
aggregate file contents, with a 16 MiB per-file ceiling. The same limits apply to the complete
post-generation tree. Tree enumeration and generation have finite timeouts; exceeding any limit,
duplicate paths, symlinks, unsupported entries, or incomplete enumeration aborts the plan without
publishing partial outputs or mutating the PR. Only committed trusted-base bytes may seed the
scratch tree, not dirty working-tree content.

After generation, compare complete before/after path inventories and bytes, including untracked,
ignored, and newly created files. Every addition, modification, deletion, or type change must be
accounted for. Include each permitted new regular file and its full contents in the plan and patch;
reject any change outside the exact generated-output allowlist. Reject deletions or type changes
unless the output contract explicitly supports them. Never report success while silently omitting a
new fixture. Temporary output must stay outside the compared tree or be removed before this
comparison. These are implementation requirements, not a claim that the current helper enforces
every bound or accounting check.

Required regression cases include a tree just within each aggregate bound, a tree exceeding each
bound despite individually small files, an allowlisted new fixture included in both plan and patch,
an ignored new file outside the allowlist rejected, and an unavailable or mismatched candidate
module object rejected before generation. Remote authentication tests must reject a well-formed but
false `h1` value; local dry-run tests must prove no network access.

### Plan outputs and workflow permissions

The generator also synchronizes `CHANGELOG.md` only within `[Unreleased]`. If the current signer
version is already present there, it advances that reference to the candidate in place. Otherwise,
the generator appends an old-to-new sigstore-go version-bump entry to the end of `### Changed`,
creating that category after `Added` when necessary. A missing existing signer reference is not an
error. Later release sections remain unchanged.

```bash
go run ./cmd/slsa-builder-internal dependabot-companion \
  --mode dry-run --event "$LOCAL_EVENT" --snapshot "$LOCAL_SNAPSHOT" \
  --trusted-root "$TRUSTED_ROOT" --trusted-sha "$TRUSTED_SHA" --output "$PLAN_OUTPUT"

go run ./cmd/slsa-builder-internal dependabot-companion \
  --mode plan --event "$GITHUB_EVENT_PATH" \
  --trusted-root "$TRUSTED_ROOT" --trusted-sha "$TRUSTED_SHA" --output "$PLAN_OUTPUT"
```

`.github/workflows/dependabot-companion.yml` automatically runs the read-only plan after eligible
`Lint` completions. The plan job checks out trusted `main`, never the PR head, and grants only
`contents: read` and `pull-requests: read`. The plan job has no App secret. Both jobs use SHA-pinned
actions, block-mode Harden Runner first, bounded job timeouts, disabled checkout credential
persistence, and branch-scoped concurrency with a run-ID fallback and `cancel-in-progress: false`.
The plan job uploads only `plan.json` and `patch.diff` as data-only review evidence, using an
artifact name scoped to the source workflow run and current attempt. It grants artifact-upload
egress only to that job. Reviewers can download this artifact from the companion workflow run. Apply
never downloads or consumes it. Both jobs run on `ubuntu-24.04`, cache trusted base modules with
`go mod download`, and invoke `go run -mod=readonly ./cmd/slsa-builder-internal` directly. They
download no workflow artifacts and execute no candidate dependencies. Only trusted base modules are
compiled; candidate module files remain generator data.

### Disabled apply and activation prerequisites

Apply is disabled unless the repository variable `DEPENDABOT_COMPANION_APPLY_ENABLED` equals `true`.
The CLI independently requires the environment variable of the same name to equal `true`, plus
`APP_SLUG` and a repository-scoped App installation token in `GITHUB_TOKEN`. Local snapshot inputs
cannot authorize apply. The workflow's apply job depends on plan, recomputes its own plan, and never
consumes an uploaded plan or patch.

Before enabling the variable, the operator must:

1. Review the automation's Developer Certificate of Origin (DCO) authority and explicitly authorize
   the configured App to certify these generated changes. A bot account alone is not authorization.
2. Configure `DEPENDABOT_COMPANION_APP_ID` and `DEPENDABOT_COMPANION_APP_PRIVATE_KEY` for the
   dedicated App. Restrict its installation token to this repository, `contents: write`, and
   `pull-requests: read`. Keep the token action's default post-job revocation enabled.
3. Confirm, in a separately approved activation exercise, that GitHub resolves the installation
   token's GraphQL viewer to that App's bot identity, creates a verified commit, and records the
   expected DCO identity. A human identity is never a fallback.
4. Confirm that the App-authored commit retriggers the required CI checks and is subject to the
   existing signed-commit, DCO, review, and branch-protection rules. Do not grant a required-check
   bypass or enable automatic merge.

The command authenticates installation scope and the GraphQL viewer, then cross-checks the bot's
numeric user ID. Only after that authorization does it replace the plan's DCO placeholder with
`<app-slug>[bot] <numeric-id>+<app-slug>[bot]@users.noreply.github.com` in an explicit
`Signed-off-by: Name <email>` trailer. It never uses a human signatory. The commit headline retains
`[dependabot skip]`.

### Single mutation and reconciliation

Apply regenerates the plan twice from fresh API projections and compares the exact plan hashes,
including patch and file hashes. It validates each output's full contents and the remote preimage,
then re-reads PR state, author, base/head identity, and the complete changed-file list immediately
before mutation. It calls GraphQL `createCommitOnBranch` exactly once, with `expectedHeadOid` and
base64-encoded full file contents. It sends no custom author, committer, or signature fields.

Read-back must confirm the returned commit OID, exactly one parent equal to the expected PR head,
the commit message, `verification.verified`, exact contents for every planned file, and the branch
ref pointing to the returned OID. A mutation timeout, missing response, missing/wrong read-back, or
unverified signature is **post-mutation indeterminate**. The command does not retry mutation or roll
back. Inspect the branch and checks manually before any further action. A later run whose PR already
contains companion edits is ineligible, rather than an excuse to append another commit.

The production client accepts only HTTPS `api.github.com`, with bounded response bodies and
timeouts, duplicate-member-rejecting JSON decoding, and no redirects, custom proxies, or custom TLS
hooks. There is no API-base CLI flag. Tests inject only local `httptest` servers.

**Live App, GraphQL signing, CI-retrigger, and branch-protection behavior is not proven locally.**
Hermetic API tests and static workflow tests establish local request/response and gating behavior,
not GitHub's live installation or policy configuration. No live mutation or App activation is part
of local validation.

## Selection Rationale and Trust Boundaries

### Sigstore: `github.com/sigstore/sigstore-go v1.4.0`

#### Rationale

- **License:** Apache-2.0, confirmed by the repository
  [license](https://github.com/sigstore/sigstore-go/blob/v1.4.0/LICENSE).
- **Maintainer and reputation:** this is the official Go verification library maintained by the
  Sigstore project. Its README describes the library as stable, production-ready, and covered by
  Sigstore conformance testing.
- **Release and compatibility:**
  [`v1.4.0`](https://github.com/sigstore/sigstore-go/releases/tag/v1.4.0) was released on
  12026-10-29, following `v1.3.0` on 12026-07-30 and the `v1.2.x` releases during 12026-06 and
  12026-07. Its tagged [`go.mod`](https://github.com/sigstore/sigstore-go/blob/v1.4.0/go.mod)
  declares Go 1.27.0, which is compatible with the repository's Go 1.27.3 toolchain.
- **Capability fit:** the library parses Sigstore bundles and verifies message signatures or DSSE
  envelopes, Fulcio certificate chains, embedded SCTs, Rekor inclusion proofs or SETs, observer
  timestamps including SET-covered integrated time, and optional RFC 3161 timestamps. The selected
  verifier options are `WithSignedCertificateTimestamps(1)`, `WithTransparencyLog(1)`, and
  `WithObserverTimestamps(1)`; an RFC 3161 timestamp remains additive rather than a replacement for
  Rekor evidence.
- **Footprint:** this is the only approved direct dependency with a substantial transitive graph.
  Reimplementing its cryptographic and transparency-log behavior would be less auditable and would
  conflict with ADR 0069's alignment with Sigstore semantics.

#### Module Evidence and Signed Descriptor

The selected module's `go.sum` module-content checksum is
`h1:CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw=`. It remains valid source evidence for the module
directory. The separate `github.com/sigstore/sigstore-go v1.4.0/go.mod` checksum covers only
`go.mod` and must not be substituted for the module-content checksum.

ADR 0113's signed signing-adapter descriptor carries the same module-content digest bytes in
DigestSet `dirHash1` form, rendered as lowercase hexadecimal:

```json
{
  "name": "signing-adapter",
  "uri": "pkg:golang/github.com/sigstore/sigstore-go@v1.4.0",
  "digest": {
    "dirHash1": "0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac"
  }
}
```

Strictly base64-decoding the `h1` payload and encoding the resulting bytes as lowercase hexadecimal
produces this `dirHash1` value. The `h1` and `dirHash1` names identify different encodings of the
same module-directory hash, not different digest evidence. The descriptor has no `annotations`
member; its identity key is `name`.

#### Online and Offline Trust-Root Paths

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
No transport-injection wrapper is therefore required for cryptographic verification. The production
mode dispatcher must keep acquisition and verification separate and must test that the offline path
cannot reach the TUF package or a log operator.

The API proof test replaces Go's default HTTP transport with a deny transport and constructs a
verifier from local trusted-root JSON without calling TUF. The source-level API separation above is
the complete no-network argument; the transport guard catches an accidental default-HTTP regression.
The test's empty root is deliberately not a successful cryptographic fixture. Accepted and rejected
verification results require governed public-good root and bundle bytes.

Relevant upstream APIs and implementation evidence:

- [`root.NewTrustedRootFromJSON`](https://pkg.go.dev/github.com/sigstore/sigstore-go/pkg/root#NewTrustedRootFromJSON)
  and
  [`root.FetchTrustedRoot`](https://pkg.go.dev/github.com/sigstore/sigstore-go/pkg/root#FetchTrustedRoot)
- [`verify.NewVerifier`](https://pkg.go.dev/github.com/sigstore/sigstore-go/pkg/verify#NewVerifier)
  and its SCT, transparency-log, and timestamp options
- [verification guide](https://github.com/sigstore/sigstore-go/blob/v1.4.0/docs/verification.md)
- [TUF options](https://github.com/sigstore/sigstore-go/blob/v1.4.0/pkg/tuf/options.go), including
  the air-gapped trusted-root-file path

### RFC 8785 JCS: `github.com/cyberphone/json-canonicalization v0.0.0-20261011152517-32a48195b9b2`

#### Rationale

- **License:**
  [Apache-2.0](https://github.com/cyberphone/json-canonicalization/blob/32a48195b9b2/LICENSE).
- **Maintainer and reputation:** this is the reference implementation repository maintained by RFC
  8785 editor Anders Rundgren and linked from RFC 8785 Appendix G. The selected pseudo-version pins
  commit `32a48195b9b2` exactly rather than following a branch.
- **Release cadence:** the repository is commit-driven rather than release-tag-driven. That is a
  maintenance disadvantage, but the exact pseudo-version is reproducible and the implementation's
  reference status outweighs the absence of tagged releases for this narrow algorithm.
- **Capability fit:** `jsoncanonicalizer.Transform` accepts raw JSON bytes and produces RFC 8785
  canonical UTF-8 bytes, including ECMAScript-compatible number formatting, UTF-16 property-name
  ordering, and required string escaping. It rejects duplicate object names rather than first
  decoding into a lossy map.
- **Footprint:** it has no third-party module requirements and adds no new module path beyond the
  graph already brought by Sigstore.

ADR 0061 remains broader than this choice. Signed SLSA Statements are not required to be JCS. Their
raw payload bytes must be scanned for duplicate object names at every depth before semantic parsing.
Go's `encoding/json` cannot perform that check because duplicate keys are processed in order and
later values replace or merge earlier values. For values that do require JCS, duplicate rejection
must happen before canonicalization as required by the verification policy. The selected transformer
provides defense in depth but does not replace the general duplicate-aware Statement parser.

Original-selection reference, not current version evidence: the initial review approved
`v0.0.0-20241213102144-19d51d7fe467`, pinning commit `19d51d7fe467`. Its
[license at the originally selected commit](https://github.com/cyberphone/json-canonicalization/blob/19d51d7fe467d4706a3ff08adf8a748f29fc21e0/LICENSE)
documents that original review. The reference-implementation, exact-pinning, and capability
rationale survives the version change, but the original version does not describe the current pin.

### Workflow YAML: `github.com/goccy/go-yaml v1.19.3-0.20260407131736-edee2f91616c`

#### Rationale

- **License:** MIT, confirmed by the repository
  [license](https://github.com/goccy/go-yaml/blob/edee2f91616c6d73112a13e7c0dbde72ce938877/LICENSE).
- **Maintainer and reputation:** this is an established, actively maintained Go YAML implementation
  with parser and AST APIs used independently of struct unmarshalling.
- **Release and compatibility:** the selected pseudo-version pins upstream commit `edee2f91616c`
  exactly. The original-selection review identified it as the earliest upstream revision containing
  the fix for goccy/go-yaml issue #735, merged as PR #862 on 12026-04-07: a fuzz-discovered
  nil-pointer panic in `Decoder.decodeSlice` when a tagged scalar maps into a Go slice field,
  reachable from attacker-controlled workflow YAML. At that time, no tagged release contained the
  fix, `v1.19.2` was the latest tag, and the pin was five commits ahead of it. Those commits also
  touched parser, token, context, and encoder files, which the original review accepted after
  inspecting the upstream diff. The module requires Go 1.21, which is compatible with this
  repository's Go 1.27.3 toolchain. Selecting any tagged release containing PR #862 requires the
  standing review policy above.
- **Capability fit:** `parser.ParseBytes` preserves a syntax tree suitable for static GitHub Actions
  conformance checks. Duplicate mapping keys are rejected by default; the trusted core must not use
  the `AllowDuplicateMapKey` opt-out. The parser can inspect workflow structure without coercing it
  through an application struct first.
- **Footprint:** the module has no third-party module requirements.

This dependency parses YAML syntax only. It does not define GitHub Actions semantics; the workflow
checker applies the repository's closed conformance rules to the AST.

### SemVer and Module Selection: `golang.org/x/mod v0.42.0`

#### Rationale

- **License:** [BSD-3-Clause](https://go.googlesource.com/mod/+/refs/tags/v0.42.0/LICENSE).
- **Maintainer and reputation:** `golang.org/x/mod` is the Go project's module for semantic-version
  and module-path operations.
- **Release and compatibility:** `v0.42.0` was tagged on 12026-10-09. Its tagged `go.mod` declares
  Go 1.26.0, compatible with the repository's Go 1.27.3 toolchain; this does not assert a support
  guarantee.
- **Capability fit:** the `golang.org/x/mod/semver` API validates and compares semantic-version
  strings used in package-manager descriptors and toolchain pins at untrusted-input boundaries. It
  does not validate the entire descriptor. Input validation completes before any associated package
  manager or toolchain is executed.
- **Trust boundary:** the package performs in-memory version processing only. It neither accesses
  the network nor executes tools, and a syntactically valid version does not by itself authorize
  provisioning, download, or execution.

## Official Test Vector Inventory

### SemVer and Module Selection

The [`semver_test.go`](https://go.googlesource.com/mod/+/refs/tags/v0.42.0/semver/semver_test.go)
file's in-file tests table provides `IsValid`, `Canonical`, `Compare`, and related accepted and
rejected cases. The upstream package does not provide a separate vector directory.

### Sigstore

`sigstore-go v1.4.0` provides:

- the Sigstore verifier conformance runner in
  [`test/conformance`](https://github.com/sigstore/sigstore-go/tree/v1.4.0/test/conformance);
- end-to-end verification tests in
  [`test/e2e`](https://github.com/sigstore/sigstore-go/tree/v1.4.0/test/e2e);
- package tests for bundle parsing, trust roots, TUF, SCT, Rekor/tlog, TSA, DSSE, and verifier
  policy under [`pkg`](https://github.com/sigstore/sigstore-go/tree/v1.4.0/pkg);
- bundle, trusted-root, and public-key vectors in
  [`pkg/testing/data`](https://github.com/sigstore/sigstore-go/tree/v1.4.0/pkg/testing/data); and
- public-good bundle and trusted-root examples in
  [`examples`](https://github.com/sigstore/sigstore-go/tree/v1.4.0/examples).

Fixture selection copies only the upstream vectors needed to prove the accepted bundle and the
required missing-SCT, missing-or-mismatched-Rekor, invalid-signing-time, and ungoverned-root
rejections, preserving source commit and license metadata.

### RFC 8785 JCS

Original-selection vector reference: the initial authoritative inventory was the reference
repository's
[`testdata` at commit `19d51d7fe467`](https://github.com/cyberphone/json-canonicalization/tree/19d51d7fe467d4706a3ff08adf8a748f29fc21e0/testdata).
Its paired `input` and `output` vectors covered arrays, French text, structures, Unicode property
ordering, primitive values, and unusual string and number cases. Its `outhex` data recorded
canonical UTF-8 bytes where rendered text was ambiguous. This pinned inventory belongs to the
original `19d51d7fe467` selection and is not presented as the inventory of the current
`32a48195b9b2` pin.

RFC 8785 itself supplies the section 3.2.2 canonicalization sample used by
`dependency_proof_test.go` and the Appendix B IEEE-754 number serialization samples.

### Workflow YAML

`go-yaml v1.19.3-0.20260407131736-edee2f91616c` includes an integration harness for the
community-maintained [`yaml-test-suite`](https://github.com/yaml/yaml-test-suite) in
[`yaml_test_suite_test.go`](https://github.com/goccy/go-yaml/blob/edee2f91616c6d73112a13e7c0dbde72ce938877/yaml_test_suite_test.go).
The repository also carries unit tests for lexer, parser, AST, decoder, encoder, anchors, aliases,
merge keys, comments, and duplicate mapping keys. Workflow-specific accepted and rejected fixtures
are project-owned because the YAML standard suite does not define GitHub Actions semantics.

## Rejected Candidates

These are original-selection decisions. Version-specific observations describe the candidates as
reviewed then, not current upstream release claims. Reconsidering a candidate requires the standing
selection and review policy.

| Candidate                                        | Reason rejected at original selection                                                                                                                                                   |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `github.com/sigstore/cosign/v2`                  | CLI-oriented and materially heavier; its legacy offline flag was deprecated, while `sigstore-go` exposed the required library boundary directly.                                        |
| `github.com/sigstore/sigstore` alone             | Lower-level primitives did not replace `sigstore-go`'s bundle and policy verification orchestration. It remains an indirect dependency of the selected official library.                |
| Older `sigstore-go` releases                     | Superseded by compatible stable `v1.4.0`; no benefit justified starting on an older verifier.                                                                                           |
| `github.com/deszhou/jcs v1.0.0`                  | Strict and tagged, but newly maintained and not the RFC editor's reference repository; it would have added a second implementation while Sigstore already brought the reference module. |
| `github.com/gowebpki/jcs v1.0.1`                 | Stable fork, but less direct provenance to the RFC reference implementation and its official vectors.                                                                                   |
| `github.com/ucarion/jcs v0.1.2`                  | Operated on already-decoded Go values and documented incomplete surrogate validation, weakening raw-input assurance.                                                                    |
| `github.com/lattice-substrate/json-canon v0.3.4` | Pre-v1 and Linux-specific; unnecessary for a portable trusted core.                                                                                                                     |
| `go.yaml.in/yaml/v4 v4.0.0-rc.6`                 | Officially maintained but still a release candidate at review time, so it was not suitable for the initial stable pin.                                                                  |
| `go.yaml.in/yaml/v3 v3.0.5`                      | Stable but frozen for legacy/security fixes and less parser/AST-focused for static conformance than `goccy/go-yaml`.                                                                    |
