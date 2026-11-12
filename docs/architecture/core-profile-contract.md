# Core Trust Model And Profile Contract

This document defines the shared trust model of `slsa-builder`: the boundary between the thin
trusted **core** and **profile-owned reusable workflows**.

- Source ADRs: [0002](../decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md),
  [0003](../decisions/0003-use-thin-core-with-profile-owned-reusable-workflows.md),
  [0004](../decisions/0004-use-go-as-primary-implementation-language.md),
  [0042](../decisions/0042-use-acquired-domains-for-buildtype-uris.md),
  [0077](../decisions/0077-use-go-native-sigstore-dsse-signer-for-windlass-provenance-signing.md),
  [0079](../decisions/0079-support-tags-only-caller-specified-build-source-ref-for-release-retries-across-profiles.md),
  [0080](../decisions/0080-bind-source-identity-policy-to-signed-provenance-fields-and-treat-certificate-source-claims-as-invocation-context.md),
  [0101](../decisions/0101-relay-external-cli-stderr-to-logs-under-hygiene-bounds-with-classification-as-enrichment.md),
  [0103](../decisions/0103-define-external-workflow-parameters-as-common-invocation-identity.md),
  [0104](../decisions/0104-remove-redundant-external-parameter-builder-record.md),
  [0105](../decisions/0105-record-invocation-and-built-source-as-role-named-resolved-dependencies.md),
  [0106](../decisions/0106-consolidate-external-source-parameters-onto-workflow-and-descriptor-bindings.md),
  [0107](../decisions/0107-record-invocation-event-metadata-in-internalparameters-github.md), and
  [0109](../decisions/0109-minimize-external-parameters-to-actual-interface-inputs.md)
- Related specs: [Identity and build types](identity-and-buildtypes.md),
  [SLSA provenance v1](slsa-provenance-v1.md)

## Scope and non-goals

**In scope:**

- Responsibilities that belong to the shared core.
- Responsibilities that belong to each profile-owned reusable workflow.
- Trusted workflow invariants that every profile must preserve.
- The contract for adding a new ecosystem profile.
- The signing adapter boundary.

**Out of scope:**

- Ecosystem-specific install, build, pack, or publish commands (profile specs).
- Exact `builder.id`, `buildType`, or release manifest schema (identity spec).
- Consumer verifier implementation details (verification policy spec).
- Development-tool choices such as linters, formatters, or package managers (tooling-only ADRs).

## Core responsibilities

The shared core owns the following:

1. **Trusted workflow invariants**
   - Every production reusable workflow must run on GitHub-hosted runners.
   - Build, provenance/signing, and publish/release-mutation authorities must be separated into
     distinct jobs with least-privilege permissions.
   - Callers must interact with a profile only through declared `workflow_call` inputs, secrets, and
     outputs.
2. **Digest-verified artifact handoff**
   - The core must define how artifacts, provenance bundles, and metadata move between isolated jobs
     using digest comparison.
   - Any job that receives an artifact from a previous job must recompute its digest and compare it
     against the expected value.
3. **Common SLSA provenance v1 construction**
   - The core defines the shared in-toto Statement and SLSA provenance v1 predicate shape used by
     all producer profiles.
   - The [common provenance contract](slsa-provenance-v1.md) defines the common source and
     invocation model. Producer provenance records caller invocation identity in the required,
     closed `externalParameters.workflow` object with exactly `ref`, `repository`, and `path`, and
     records the invocation and built source in the role-named `invocation-source` and
     `built-source` `resolvedDependencies` descriptors. A profile that exposes a caller-selected
     source ref records that actual selector only in the optional `externalParameters.source.ref`
     member, under the common contract's presence and closed-schema rules. `workflow.path` is the
     caller workflow path, never the reusable builder path. Builder `sha` or `builder_id` members
     and use of the reusable builder path as `workflow.path` are rejected; canonical builder
     identity remains only in `runDetails.builder.id`.
   - The effective built ref is `source.ref` when present and `workflow.ref` otherwise. Producer
     profiles apply this derivation before constructing provenance, and consumers independently
     recompute it. A statement whose `built-source` URI or resolved commit disagrees with the
     recomputed identity is rejected.
   - Invocation event metadata lives only in `internalParameters.github`. The object is required and
     closed, with exactly the string members `event_name`, `repository_id`, and
     `repository_owner_id`.
   - Producer profiles compare all three values fail-closed with the corresponding
     signing-certificate claims before any registry or release mutation. Missing, additional,
     non-string, or certificate-mismatched members are rejected.
   - Actor fields, including `actor_id` and `triggering_actor_id`, are forbidden. An
     `externalParameters` duplicate or alias of the invocation event is also rejected.
   - The core does not define ecosystem-specific subject names or digest rules.
4. **Builder identity and build type conventions**
   - The core defines how `builder.id` and `buildType` URIs are documented and how release metadata
     maps versions to workflow SHAs.
5. **Validation of profile `externalParameters`**
   - The core must ensure that every profile declares a complete, closed `externalParameters` schema
     limited to actual values passed through its interface and rejects missing or unexpected fields.
     Derived values, resolved artifact metadata, environment observations, manifest content, and
     behavior fixed by the `buildType` are rejected rather than accepted as external parameters.
     Their verification evidence instead uses the common invocation record, resolved dependencies,
     builder versions, signed run metadata, authenticated input artifacts, or a profile-specified
     recompute rule.
6. **Signing adapter interface**
   - The core defines the interface between profile workflows and the Go-native `sigstore-go`
     signing adapter selected by ADR 0077.
   - The signing adapter is not the source of provenance semantics; it only signs the material
     produced by the trusted core and profile.
7. **Shared verification documentation conventions**
   - The core defines how verifier expectations, policy, and reference commands are documented for
     each profile.

## Profile-owned responsibilities

Each profile-owned reusable workflow owns the following:

1. **Workflow entrypoint and triggers**
   - The public reusable workflow file name and the supported caller triggers.
2. **Ecosystem toolchain setup and build or pack steps**
   - How the profile installs dependencies, runs build scripts, and produces the final artifact.
3. **Package or artifact subject naming**
   - The `subject[0].name` value and any ecosystem-specific naming rules.
4. **Digest algorithm semantics**
   - Which digest algorithms are recorded and where (for example, npm tarball `sha512` integrity and
     `sha256` sidecar digest).
5. **Profile-specific `buildType` and `externalParameters` schema**
   - The canonical `buildType` URI and the complete, closed schema of actual interface inputs a
     verifier must check. Missing or unexpected inputs and derived, observed, manifest-sourced, or
     constant values projected into `externalParameters` are rejected.
   - Input suppliedness, spelling, conflict, and consumer-recompute rules. A profile must not
     redefine the common invocation object, event object, or source descriptors; it references the
     common provenance contract for their closed shapes and verification behavior.
6. **Registry publish and verification behavior**
   - How the profile publishes artifacts and what verification steps run before publication.
7. **Profile fixtures, examples, and downstream verification commands**
   - Concrete examples and test fixtures that demonstrate correct and incorrect behavior.

## Trusted workflow invariants

Every profile-owned reusable workflow must satisfy these invariants. A workflow that violates any
invariant is not a valid `slsa-builder` profile.

### Hosted runner requirement

- Production jobs must use GitHub-hosted runners only.
- Self-hosted runners, container-based runners under caller control, or runner labels that resolve
  to non-GitHub infrastructure are prohibited.

### Job isolation

- Source-to-artifact producer profiles must run build, signing/attestation, and publication concerns
  in separate jobs unless a later ADR explicitly changes this boundary.
- Publisher profiles that do not build or sign artifacts are not required to add empty build or
  signing jobs. They must still separate any authority they do hold, such as release mutation,
  linked-artifact metadata mutation, and producer provenance verification, as defined by the
  publisher spec.
- No job may combine signing authority with release mutation authority.

### No caller-injected arbitrary behavior

- A reusable workflow must not accept or execute arbitrary shell commands, scripts, services,
  composite actions, or workflow defaults supplied by the caller.
- A profile must not expose inputs that let callers override the runtime environment, tool versions,
  or command matrix. The producer `source-ref` input defined by ADR 0079 is not a runtime override:
  it is a declared, closed-schema input constrained to the profile's release tag, and it changes
  only which existing repository content is built, never how the build runs.
- A profile must not inherit broad secrets or `GITHUB_TOKEN` permissions from the caller.

### Built source identity (producer profiles)

- Every producer profile must offer the optional `source-ref` input decided by ADR 0079: a
  tags-only, fail-closed selector for the Git ref whose content the profile builds, attests, and
  publishes. Publisher profiles do not build source and must not define it.
- The effective built ref is `source.ref` when present and `workflow.ref` otherwise. The optional
  `source` group is emitted exactly when the caller supplied a non-empty `source-ref`, contains the
  exact full-ref selector in `ref`, and is omitted otherwise. An empty `source: {}` object,
  malformed or conflicting selector, or input-suppliedness mismatch is rejected.
- Runtime release guards must evaluate the effective built ref rather than always using the
  invocation ref. The effective built ref must satisfy the profile's release tag convention before
  install, pack, signing, or publish, and a violation rejects the run before those operations.
- The profile must resolve the effective built ref to a full commit SHA before building and must
  carry that resolved SHA through the digest-verified handoff so that signing and publish jobs
  cannot disagree with the build job about what was built. A missing, malformed, or mismatched
  resolution fails closed.
- Signed provenance records caller invocation through `externalParameters.workflow` and the
  `invocation-source` descriptor, and records built content through the `built-source` descriptor.
  The common provenance contract owns their exact closed shapes and certificate and policy bindings.
  Verification recomputes the effective built ref, binds release ref and commit expectations to
  `built-source`, and binds certificate invocation claims to `workflow` and `invocation-source`.
  Missing, duplicate, malformed, or mismatched records are rejected; a certificate invocation claim
  is not a substitute for built-source identity.
- Producer-side validation and post-sign verification compare the invocation repository, ref,
  commit, caller workflow path, and event metadata with their corresponding certificate claims
  fail-closed before any registry or release mutation.
- Both source descriptors are required on every producer run that uses this common source model,
  including when the identities coincide. When `source-ref` is omitted, the effective built ref
  equals the invocation ref and the descriptors carry identical URI and digest values under their
  distinct role names. Omitting either descriptor or substituting one role for the other is
  rejected.

For example, a dispatch on `refs/heads/main` with `source-ref` set to `refs/tags/v1.2.3` records
`refs/heads/main` in `workflow.ref` and `invocation-source`, records the exact tag selector in
`source.ref`, and derives `refs/tags/v1.2.3` as the effective built ref recorded by `built-source`.
A tag push on `refs/tags/v1.2.3` without `source-ref` omits `source` and records the same tag and
commit in both descriptors. Replacing the dispatch ref with the built tag in the invocation record,
emitting an empty `source` object, or omitting either descriptor is rejected.

### Minimal permissions

- Top-level workflow permissions must be minimal.
- Job-level permissions must be elevated only when required and only for the narrowest scope (for
  example, `id-token: write` only on the signing job, optional `attestations: write` only when a
  profile separately enables GitHub attestation storage, and `contents: write` only on the
  upload/publish job).

### Digest-verified handoff

- Every artifact, bundle, or manifest that crosses a job boundary must be accompanied by an expected
  digest.
- The receiving job must recompute the digest and fail closed on mismatch.
- The initial production handoff transport is same-run GitHub Actions artifacts plus trusted job
  outputs that carry artifact names and expected lowercase SHA-256 digests.
- A handoff artifact must contain exactly one payload file unless the profile spec explicitly
  defines a multi-file artifact schema.
- Artifact names, file names, job outputs, logs, and locators are transport metadata only; none of
  them prove trust without digest verification and, when applicable, signed provenance verification.
- File-path-only, URL-only, artifact-ID-only, cross-run artifact, or raw-bytes-in-output handoffs
  are outside the initial production contract unless a profile spec explicitly adds them under the
  same trust boundary.

### Initial handoff schema

The initial production handoff schema is a same-workflow-run GitHub Actions artifact handoff. The
shape below is the semantic minimum every profile-specific handoff must satisfy. Profile specs may
expose different public field names or add profile-specific fields, but they must map those names to
this shape explicitly before the receiving job trusts the payload:

```json
{
  "transport": "github-actions-artifact",
  "artifact_name": "profile-payload-123456789-1",
  "payload_file_name": "artifact.tar.gz",
  "payload_kind": "primary-artifact",
  "digest": {
    "algorithm": "sha256",
    "value": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }
}
```

#### Handoff field rules

- `transport` must be `github-actions-artifact` for the initial production contract.
- `artifact_name` must identify an artifact uploaded earlier in the same workflow run. It must not
  be a URL, local file path, artifact numeric ID, or cross-run artifact reference.
- `payload_file_name` must be the basename of the single file contained in the artifact. It must not
  contain `/`, `\\`, NUL, or path traversal segments.
- `payload_kind` must be one of `primary-artifact`, `provenance-bundle`, `release-manifest`, or a
  profile-defined value documented in that profile's spec.
- `digest.algorithm` must be `sha256` for cross-job handoff.
- `digest.value` must be a 64-character lowercase hexadecimal SHA-256 digest.
- The artifact must contain exactly one payload file unless the profile spec defines a stricter
  multi-file schema with a digest for each file and for the canonical archive or manifest that binds
  them.

When a profile uses public fields such as `primary-artifact-name` or `producer-provenance-sha256`,
those fields are aliases for the core semantic fields above, not a separate handoff model. A
profile-specific alias must not omit `transport`, `payload_file_name`, `payload_kind`, or
`digest.algorithm`; the profile may set those fields to fixed values in its mapping table when they
are not public caller inputs.

The receiving job must fail before using the payload when any required field is missing, the
transport is not allowed, the artifact cannot be retrieved from the same workflow run, the artifact
contains the wrong number of files, the payload file name is unsafe or unexpected, the digest field
is malformed, or the recomputed SHA-256 does not match `digest.value`.

## Go trusted core boundary

- The primary implementation language for the trusted core is Go.
- The trusted core owns security-relevant normalization and policy logic, including provenance
  Statement construction, subject and digest normalization, `externalParameters` schema validation,
  handoff digest verification, release manifest canonicalization, and verifier fixture generation.
- That trusted core logic must not depend on Node.js, npm, pnpm, Yarn, JavaScript ecosystem
  libraries, or shell parsing in the production path.
- Ecosystem tools such as Node.js, npm, pnpm, Yarn, and Corepack may run in profile-owned build,
  pack, and publish steps when the profile spec requires them. Their outputs are inputs to trusted
  validation, not the source of trusted invariants.
- Node.js, pnpm, and other JavaScript-based tooling may also be used for local development tooling
  such as Prettier, `markdownlint-cli2`, and Lefthook.
- Shell scripts may be used only as glue, such as invoking the Go trusted core and ecosystem tools,
  not as the source of provenance, digest, policy, or release manifest invariants.

### External-process operational diagnostics

The trusted core may emit best-effort, human-readable operational diagnostics for allowlisted
external processes. These diagnostics are stderr-only transport metadata, not trust proof, verifier
input, or a stable consumer interface. Consumers must not treat them as a protocol or as evidence
that establishes an artifact's integrity or provenance.

Operation labels are fixed to the existing executable allowlist: `node`, `npm`, `npx`, `corepack`,
`pnpm`, and `yarn`. Lifecycle diagnostics are limited to that label, start and completion notices,
whether the process started, the observed exit code or signal, timeout or cancellation state,
elapsed duration, and whether captured output was truncated. A start failure reports `started=false`
and `outcome=start-failed`, without a fabricated exit code or signal. Timeout and cancellation are
reported only when observed.

A profile may supplement these process diagnostics with a closed, explicitly specified allowlist of
profile-specific categorical facts. The profile specification must define those facts and their
bounds, and every extension remains subject to the non-authoritative, secret-exclusion, and
non-interference rules below. Categorical facts are identifiers, allowlisted field paths, statuses,
and counts; they never carry external message text.

#### Relayed stderr and its hygiene filter

In addition to trusted-core-authored lifecycle and categorical lines, the trusted core relays the
stderr of allowlisted external processes to the operational channel. The relay exists so that an
ecosystem registry or tool rejection is diagnosable from the workflow log without
re-instrumentation. ADR 0101 fixes the relay's existence and purpose; this specification owns the
concrete hygiene-filter conditions below and may strengthen or relax them by specification amendment
without a new ADR.

The relayed text is the only external text that may appear in operational diagnostics, and it
crosses only through the hygiene filter. The filter's initial conditions are:

- **Exact-value redaction first.** Every exact occurrence of each secret value known to the trusted
  core is replaced with `[REDACTED]` on the raw stream, before any other filtering. The redaction
  set for a process is the value of the ambient `ACTIONS_ID_TOKEN_REQUEST_TOKEN` environment
  variable when present in that process's environment, plus any OIDC material the trusted core holds
  for the operation. For the npm profile that material is the GitHub OIDC identity token and the npm
  trusted-publishing exchange token; neither is ever injected into the subprocess environment, and
  both are redacted purely as defense in depth. Redaction is exact byte matching, including
  occurrences split across stream chunks; it is never heuristic or pattern-based, and it applies to
  no other values. Empty values and values containing a line break are excluded from the redaction
  set; the named token types contain no line breaks by construction. The guarantee is exact-value
  only: transformed, encoded, truncated, or partial representations of a secret are out of scope by
  decision (ADR 0101), on the strength of the default-verbosity constraint below and the observed
  default-level message classes of the allowlisted tools.
- **Control-character stripping.** After redaction, ASCII control bytes other than LF and TAB, and
  DEL, are stripped, and each invalid UTF-8 byte sequence is replaced with the Unicode replacement
  character (U+FFFD). C1 control characters (U+0080 through U+009F) and the Unicode bidirectional
  control characters (U+061C, U+200E, U+200F, U+202A through U+202E, U+2066 through U+2069) are
  likewise stripped. The relay never emits a character that can drive a terminal or log-viewer
  control function, and its emitted payload is always valid UTF-8 text.
- **Runner-command neutralization.** The legacy GitHub Actions runner command marker `##[` is never
  emitted: every `[` that would leave `##[` in the emitted text is stripped, wherever it appears in
  the line. Neutralization runs after control-character stripping, on the filtered post-redaction
  stream, so a marker exposed by an earlier removal (for example `##[[` or `##` followed by a
  stripped control byte and `[`) and a marker composed by a redaction replacement (for example text
  ending in `##` immediately before a redacted value) are stripped as well. The legacy marker is the
  only workflow-command syntax the runner matches anywhere in a line; the modern `::command::`
  syntax executes only when a line begins with `::`, and every relayed line begins with the
  operational prefix. Between the prefix and this rule, no runner workflow-command surface reaches
  the log.
- **Bounded volume.** At most 65536 filtered payload bytes (64 KiB) and at most 512 non-empty
  relayed lines per process. Bounds are measured after redaction and stripping; the operational line
  prefixes and the truncation marker do not count against them. A byte truncation never splits a
  UTF-8 multibyte character: the relayed prefix ends at the last complete character within the byte
  bound. When either bound is exceeded, relaying stops and exactly one `stderr_truncated=true`
  marker line is emitted; the marker carries no child text.
- **Line format and ordering.** Relayed lines are emitted when the process completes, after the
  start notice and before the completion notice, one operational line per non-empty filtered stderr
  line, in stream order, each in the form
  `windlass operational: operation=<label> stderr: <filtered text>`. A final unterminated line is
  flushed. Empty filtered lines are omitted. Every relayed byte carries the operational prefix, so
  child output cannot forge an operational line or escape the prefix.
- **Default verbosity only.** A profile must not elevate an external tool's log verbosity (for npm:
  no `--loglevel` argument and no loglevel configuration), so that relayed content stays within the
  message classes the tool emits at its default level.

The relay applies to every allowlisted process regardless of exit status. The canonical outcome
report, the diagnostic registry, and signed artifacts remain sealed: no external text ever reaches
them.

Trusted-core-authored lines must not contain raw argv, environment values, file paths, child or
trusted-core error strings, credentials, or URLs; only the filtered stderr relay may carry
tool-rendered text, and only under the hygiene conditions above. Logging and stderr write failures
are discarded; they cannot change returned output or errors, process behavior, or workflow outcomes.

Valid illustrative lines are:

```text
windlass operational: operation=node start
windlass operational: operation=npm stderr: npm error code E422
windlass operational: operation=npm stderr: token value: [REDACTED]
windlass operational: operation=npm stderr_truncated=true
windlass operational: operation=node started=true outcome=exit exit=0 elapsed=12ms output_truncated=false
windlass operational: operation=npm started=false outcome=start-failed elapsed=3ms output_truncated=false
```

The following lines are invalid because they disclose prohibited values in trusted-core-authored
records, carry external text outside the filtered relay format, or bypass the filter entirely:

```text
windlass operational: operation=npm argv="publish /tmp/package.tgz" registry="https://registry.npmjs.org/"
windlass operational: operation=npm child_error="raw child error text"
windlass operational: npm_failure message="expected X, actual Y"
npm error code E422
```

## Signing adapter boundary

- The Go-native `sigstore-go` v1.3.0 signer defined by ADR 0077 is the Windlass signing adapter for
  every production profile. The npm profile adopts it first, Wave 4 release-manifest signing reuses
  it, and future profiles use it by default as their contracts are admitted. `actions/attest` is not
  a Windlass signing adapter.
- The signing adapter is responsible for:
  - Receiving the profile's validated signing material.
  - Preserving the exact in-toto Statement semantics owned by the trusted core and profile.
  - Producing a Sigstore-backed bundle.
  - Optionally uploading the bundle to GitHub artifact attestation storage only when the profile
    explicitly permits that path.
- The signing adapter is **not** responsible for:
  - Defining what `builder.id`, `buildType`, or `externalParameters` mean.
  - Validating ecosystem-specific subject or digest semantics.
  - Deciding whether an artifact is safe to publish.
- The adapter receives each profile's exact preassembled Statement bytes, signs them as
  `sign.DSSEData` with payload type `application/vnd.in-toto+json`, and must not reconstruct,
  normalize, or reserialize them. The npm GitHub attestation storage path is disabled while that
  service rejects the profile's custom `buildType`; storage integration is separate from signer
  selection.
- Any future adapter migration must preserve the verifier-visible trust contract and update the
  profile's closed `builderDependencies` descriptor through a follow-up ADR.

## Profile extension contract

A new profile is admitted only when it satisfies the common profile requirements and one concrete
role contract. The initial roles are **producer profile** and **publisher profile**.

### Common profile requirements

Every production profile must define all of the following:

1. A public reusable workflow entrypoint under `.github/workflows/`.
2. Supported caller triggers and runtime guards.
3. A complete `workflow_call` input and output schema.
4. Publish, upload, or distribution behavior and verification expectations for the authorities the
   profile actually holds.
5. Fixtures, examples, and reference verification commands for its public contract.
6. A review checklist proving that the profile does not bypass hosted runners, job isolation,
   digest-verified handoff, minimal permissions, or its role-specific trusted verification boundary.

### Producer profile requirements

A producer profile creates final artifact bytes from source and emits source-to-artifact SLSA
provenance. In addition to the common requirements, every producer profile must define all of the
following:

1. A canonical `buildType` URI under the `buildtype.dev` namespace.
2. A complete, closed `externalParameters` schema limited to actual interface inputs, including the
   required common `workflow` invocation group. The verifier policy rejects missing or unexpected
   fields and any derived, observed, manifest-sourced, or constant value projected into
   `externalParameters`.
3. Subject naming and digest algorithm rules.
4. Build, signing, and publish or upload job separation, unless a later ADR explicitly changes that
   boundary for the producer.
5. SLSA provenance v1 generation with documented `builder.id`, `buildType`, `externalParameters`,
   and the closed `internalParameters.github` object containing exactly `event_name`,
   `repository_id`, and `repository_owner_id` as strings. Missing, additional, non-string, actor, or
   certificate-mismatched members are rejected before any registry or release mutation.
6. A complete, unordered, name-keyed `resolvedDependencies` schema with deterministic producer
   serialization, including the required `invocation-source` and `built-source` descriptors under
   the common source model. Duplicate or unknown names, missing required descriptors, malformed
   shapes or annotations, and identity mismatches are rejected before publication.
7. The optional `source-ref` input, the effective built-ref derivation and runtime guards, and the
   built-versus-invocation recording required by the built source identity invariant above. The
   profile rejects a selector or statement that violates the derivation, suppliedness, conflict,
   descriptor, or certificate-binding rules (ADRs 0079, 0080, and 0103 through 0107).

### Publisher profile requirements

A publisher profile distributes artifacts produced by producer profiles and does not claim to build
those artifacts from source. In addition to the common requirements, every publisher profile must
define all of the following:

1. A producer-to-publisher handoff contract for artifact bytes, expected digests, producer
   provenance, trusted producer policy, and the target publication slot.
2. Mandatory producer provenance verification before publication or release mutation.
3. The release, registry, or storage mutation authority it holds and the job boundary that isolates
   that authority from signing authority and producer-controlled build steps.
4. Sidecar, locator, or metadata distribution behavior for producer provenance.
5. Explicit confirmation that the default publisher path does not define a source-to-artifact
   `buildType`, `builder.id`, or publisher-owned SLSA provenance unless a later ADR adds a distinct
   evidence model.

### Review criteria for new profiles

Before a new profile is accepted, reviewers must verify that it:

- Runs only on GitHub-hosted runners in production.
- Does not accept arbitrary commands, scripts, or runtime overrides from the caller.
- Uses minimal top-level permissions and job-level elevation only where required.
- Uses digest-verified handoff between jobs.
- Preserves the role-specific authority boundary: producer profiles separate build, signing, and
  publish/upload concerns, while publisher profiles separate release or storage mutation from
  signing authority and producer provenance verification.
- Documents whether it is a producer, publisher, or a later ADR-defined role.
- Includes accepted and rejected test fixtures for every normative behavior.

## TDD and fixture expectations

Each profile conformance test must include:

- A positive fixture showing the profile producing a valid artifact, provenance, and publish output.
- A negative fixture for each prohibited caller injection (arbitrary command, runtime override,
  broad permission, self-hosted runner, unexpected `externalParameters`) and for derived, observed,
  manifest-sourced, or constant values misplaced in `externalParameters`.
- A negative fixture proving that trusted provenance, digest, policy, or release manifest logic is
  not delegated to Node.js, npm, pnpm, Yarn, or shell parsing.
- A review checklist that a human can run against the profile YAML.

Producer profile conformance tests must additionally include source-to-artifact provenance fixtures
with valid and rejected `builder.id`, `buildType`, subject, digest, and actual-interface
`externalParameters` values. They must include:

- Accepted tag-push and dispatch-retry fixtures proving the required `workflow` caller identity,
  caller workflow path, exact `source.ref` selector when supplied, complete omission of `source`
  otherwise, effective built-ref derivation, and required `invocation-source` and `built-source`
  descriptors. A coincident-identity fixture retains both descriptors with identical URI and digest
  values under their distinct names.
- Rejected fixtures for missing or additional `workflow` members, builder `sha` or `builder_id`
  members, reusable builder path substitution for `workflow.path`, an empty `source` object,
  malformed or conflicting selectors, effective built-ref mismatches, missing, duplicate, unknown,
  malformed, or identity-mismatched source descriptors and annotations, and certificate invocation
  mismatches.
- An accepted fixture with exactly the three string members in `internalParameters.github` and
  supported invocation events. Rejected fixtures cover missing, additional, non-string, or actor
  members, an `externalParameters` event duplicate or alias, an unsupported event, Build Trigger
  disagreement, and numeric repository or owner identifier disagreement. Producer-side cases prove
  that each certificate mismatch stops the run before any registry or release mutation.

Publisher profile conformance tests must instead include producer handoff fixtures proving that raw
artifacts, missing producer provenance, publisher-owned source-to-artifact `buildType` claims, and
release mutation without producer provenance verification are rejected.

## Failure behavior

If a profile workflow does not satisfy the core invariants, the implementation must fail at workflow
parse time or at the earliest runtime check. A profile must never silently fall back to
caller-controlled build steps, untrusted runners, or broad permissions.
