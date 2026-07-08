# Core Trust Model And Profile Contract

This document defines the shared trust model of `slsa-builder`: the boundary between the thin
trusted **core** and **profile-owned reusable workflows**.

- Source ADRs: [0002](../decisions/0002-use-extensible-trusted-reusable-workflow-foundation.md),
  [0003](../decisions/0003-use-thin-core-with-profile-owned-reusable-workflows.md),
  [0004](../decisions/0004-use-go-as-primary-implementation-language.md),
  [0035](../decisions/0035-use-actions-attest-as-initial-sigstore-signing-adapter.md),
  [0042](../decisions/0042-use-acquired-domains-for-buildtype-uris.md)
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
   - The core does not define ecosystem-specific subject names or digest rules.
4. **Builder identity and build type conventions**
   - The core defines how `builder.id` and `buildType` URIs are documented and how release metadata
     maps versions to workflow SHAs.
5. **Validation of profile `externalParameters`**
   - The core must ensure that every profile declares complete, explicit, and verifier-relevant
     `externalParameters` and rejects unexpected fields when the verifier policy requires strict
     matching.
6. **Signing adapter interface**
   - The core defines the interface between profile workflows and signing adapters such as
     `actions/attest`.
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
   - The canonical `buildType` URI and the complete schema of `externalParameters` a verifier must
     check.
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
  or command matrix.
- A profile must not inherit broad secrets or `GITHUB_TOKEN` permissions from the caller.

### Minimal permissions

- Top-level workflow permissions must be minimal.
- Job-level permissions must be elevated only when required and only for the narrowest scope (for
  example, `id-token: write` and `attestations: write` only on the signing job, `contents: write`
  only on the upload/publish job).

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

The initial production handoff schema is a same-workflow-run GitHub Actions artifact handoff.
Profile specs may add profile-specific fields, but each payload that crosses a job boundary must
include at least this semantic shape:

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

## Signing adapter boundary

- The initial signing adapter is full-SHA-pinned `actions/attest`.
- The signing adapter is responsible for:
  - Receiving a subject name and digest.
  - Receiving a predicate type and predicate JSON.
  - Constructing the in-toto Statement from those verified inputs.
  - Producing a Sigstore-backed bundle.
  - Optionally uploading the bundle to GitHub artifact attestation storage.
- The signing adapter is **not** responsible for:
  - Defining what `builder.id`, `buildType`, or `externalParameters` mean.
  - Validating ecosystem-specific subject or digest semantics.
  - Deciding whether an artifact is safe to publish.
- The initial stock `actions/attest` adapter must not be invoked or documented as accepting a
  complete in-toto Statement payload. The trusted core and profile own the subject, predicate type,
  and predicate semantics, then the producer-side verification gate must extract the emitted
  Statement from the signed bundle and prove that it matches those verified signing inputs.
- A future ADR may replace `actions/attest` with direct Sigstore tooling, `sigstore-go`, or a
  dedicated reusable workflow. Any migration must preserve the verifier-visible trust contract.

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
2. A complete `externalParameters` schema and a verifier policy that rejects unexpected fields.
3. Subject naming and digest algorithm rules.
4. Build, signing, and publish or upload job separation, unless a later ADR explicitly changes that
   boundary for the producer.
5. SLSA provenance v1 generation with documented `builder.id`, `buildType`, and
   `externalParameters`.

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
  broad permission, self-hosted runner, unexpected `externalParameters`).
- A negative fixture proving that trusted provenance, digest, policy, or release manifest logic is
  not delegated to Node.js, npm, pnpm, Yarn, or shell parsing.
- A review checklist that a human can run against the profile YAML.

Producer profile conformance tests must additionally include source-to-artifact provenance fixtures
with valid and rejected `builder.id`, `buildType`, subject, digest, and `externalParameters` values.
Publisher profile conformance tests must instead include producer handoff fixtures proving that raw
artifacts, missing producer provenance, publisher-owned source-to-artifact `buildType` claims, and
release mutation without producer provenance verification are rejected.

## Failure behavior

If a profile workflow does not satisfy the core invariants, the implementation must fail at workflow
parse time or at the earliest runtime check. A profile must never silently fall back to
caller-controlled build steps, untrusted runners, or broad permissions.
