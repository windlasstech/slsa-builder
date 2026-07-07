---
parent: Decisions
nav_order: 35
status: accepted
date: 12026-07-06
decision-makers: Yunseo Kim
---

# Use actions/attest as the Initial Sigstore Signing Adapter

## Context and Problem Statement

ADR 0029 selected Windlass-generated SLSA provenance as the canonical provenance for the initial
production JS/TS npm package SLSA3 profile. That provenance must be signed as a Sigstore DSSE bundle
that can be submitted through npm's external `provenance-file` path. ADR 0029 intentionally left the
implementation boundary for signing open.

The signing adapter choice affects the trusted core dependency set, GitHub Actions permissions, SLSA
Build L3 signing-key isolation story, verifier compatibility, and the long-term path for a Go
implementation. The project needs a production-friendly initial adapter without foreclosing a later
move to an in-process Go Sigstore implementation.

Should the initial production profile use GitHub `actions/attest`, the `cosign` CLI, `sigstore-go`,
npm automatic provenance, or a custom Sigstore protocol implementation to sign Windlass-generated
SLSA provenance?

## Decision Drivers

- Preserve ADR 0029's Windlass-generated SLSA provenance as the canonical provenance record.
- Produce a Sigstore DSSE bundle using short-lived GitHub Actions OIDC identity rather than a
  long-lived signing key.
- Keep signing-key material inaccessible to caller-controlled build steps, as required for SLSA
  Build L3 provenance unforgeability.
- Minimize initial implementation risk while the repository is still defining architecture
  specifications.
- Use a signing path compatible with GitHub reusable workflow signer identity verification.
- Keep workflow permissions explicit and narrowly scoped to the signing job.
- Avoid making npm automatic provenance the canonical builder provenance for the Windlass profile.
- Leave a clear future path toward a Go-native `sigstore-go` signing adapter when the trusted core
  is ready to own that complexity.

## Considered Options

- Use GitHub `actions/attest` as the initial Sigstore signing adapter, with a future migration path
  toward `sigstore-go`.
- Use `sigstore-go` directly in the initial Go trusted core.
- Invoke the `cosign` CLI from the reusable workflow.
- Rely on npm trusted publishing or `npm publish --provenance` automatic provenance.
- Implement the Sigstore protocol directly without a maintained client or action.

## Decision Outcome

Chosen option: "Use GitHub `actions/attest` as the initial Sigstore signing adapter, with a future
migration path toward `sigstore-go`", because it gives the initial GitHub Actions reusable workflow
a well-supported OIDC and Sigstore bundle path while deferring Go-native signing implementation work
until the trusted core has enough specification and test coverage to own it safely.

The initial production JS/TS npm package SLSA3 profile should treat `actions/attest` as the signing
adapter for Windlass-generated SLSA provenance. Windlass remains responsible for generating and
validating the SLSA provenance statement selected by ADR 0029, including `subject`, `predicateType`,
`buildType`, `externalParameters`, `builder.id`, and other profile-defined fields. `actions/attest`
is responsible only for creating the Sigstore-signed attestation bundle and, when configured,
uploading it to GitHub's attestation storage.

The reusable workflow should pin `actions/attest` to a full commit SHA, not a mutable tag. The
signing job should grant only the permissions required for the selected signing and storage
behavior:

- `contents: read` for repository access;
- `id-token: write` for GitHub Actions OIDC token minting;
- `attestations: write` for GitHub artifact attestation persistence when that storage path is used;
- `artifact-metadata: write` only when linked artifact storage metadata is intentionally created.

The production profile should not grant signing permissions to caller-controlled install, build,
pack, or publish-script steps. If signing and build execution are split across jobs, artifacts and
provenance inputs handed to the signing job must be authenticated by the trusted workflow design
described in the architecture specification. The signing adapter must not accept caller-provided
signing keys, long-lived Sigstore credentials, arbitrary OIDC tokens, or npm publish tokens as
provenance-signing credentials for the initial profile.

Verifier guidance for the initial profile should include GitHub artifact attestation verification
where applicable, including reusable workflow signer identity checks such as `--signer-workflow` or
`--signer-repo`. npm package verification remains governed by ADR 0029's canonical package
provenance requirements: the verifier must check the Sigstore signature, expected GitHub Actions
signer identity, SLSA predicate type, Windlass `builder.id`, Windlass `buildType`, expected
`externalParameters`, and published package tarball digest.

Long term, the project should prefer a `sigstore-go` adapter when the Go trusted core is ready to
implement and test Sigstore signing directly. A future migration ADR should define how to preserve
or intentionally change signer identity, bundle format, verifier behavior, npm `provenance-file`
compatibility, GitHub attestation storage behavior, and workflow permissions. If the migration
materially changes security properties, verifier expectations, or `builder.id` meaning, it should
use a distinct builder identity.

### Consequences

- Good, because the initial production profile gets a maintained GitHub-native Sigstore signing
  path.
- Good, because the workflow can use short-lived GitHub Actions OIDC identity instead of managing a
  long-lived signing key.
- Good, because GitHub artifact attestation verification can validate reusable workflow signer
  identity directly.
- Good, because `actions/attest` keeps Fulcio, Rekor, timestamp, bundle, and GitHub attestation API
  integration out of the first Go implementation milestone.
- Good, because a later `sigstore-go` implementation remains aligned with ADR 0004's Go trusted core
  direction.
- Neutral, because GitHub artifact attestations may be secondary evidence for npm package releases
  unless a later ADR changes the canonical distribution model.
- Bad, because `actions/attest` becomes part of the initial trusted core dependency set and must be
  SHA-pinned, monitored, and updated deliberately.
- Bad, because the initial signing path is GitHub-specific and does not by itself support GitHub
  Enterprise Server or non-GitHub build platforms.
- Bad, because a later `sigstore-go` migration may require verifier documentation and compatibility
  testing even if the emitted bundle shape stays compatible.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification,
implementation, workflow documentation, and verifier guidance define:

- `actions/attest` as the initial Sigstore signing adapter for Windlass-generated SLSA provenance;
- full-SHA pinning for the `actions/attest` action reference;
- job-level permissions limited to the required `contents: read`, `id-token: write`,
  `attestations: write`, and optional `artifact-metadata: write` permissions;
- no caller-provided signing keys, Sigstore credentials, arbitrary OIDC tokens, npm publish tokens,
  or inherited secrets for provenance signing;
- the exact handoff boundary between provenance statement generation, subject digest verification,
  signing, npm `provenance-file` submission, and optional GitHub attestation upload;
- verifier guidance for expected GitHub Actions signer identity, Windlass `builder.id`, Windlass
  `buildType`, SLSA predicate type, `externalParameters`, and package tarball digest;
- a documented future path for a `sigstore-go` adapter without requiring that implementation in the
  initial profile.

Implementation review should verify that signing permissions are not available to caller-controlled
build steps and that production releases cannot silently fall back to npm automatic provenance,
unsigned provenance, long-lived key signing, or a different signer identity under the same
production builder identity.

## Pros and Cons of the Options

### Use GitHub `actions/attest` as the initial signing adapter

The reusable workflow uses a SHA-pinned `actions/attest` step to sign the Windlass-generated SLSA
statement with GitHub Actions OIDC-backed Sigstore identity and to produce a Sigstore bundle.

- Good, because it is a GitHub-native path with first-party support for GitHub artifact
  attestations.
- Good, because it uses short-lived OIDC identity and avoids local signing-key management.
- Good, because it provides a practical initial path for DSSE bundle generation before the Go
  trusted core implements Sigstore signing directly.
- Good, because `gh attestation verify` supports reusable workflow signer checks.
- Bad, because it adds a pinned third-party action dependency to the trusted builder boundary.
- Bad, because its storage and verification conveniences are GitHub-specific.

### Use `sigstore-go` directly in the initial Go trusted core

The Go implementation calls Sigstore libraries directly to create and verify Sigstore bundles.

- Good, because it aligns with ADR 0004's Go trusted core direction.
- Good, because it gives Windlass direct control over signing, bundle generation, error handling,
  and tests.
- Good, because it can reduce workflow shell/action glue over time.
- Bad, because it requires the initial implementation to own OIDC acquisition, Fulcio/Rekor/TSA/TUF
  behavior, failure handling, and compatibility testing immediately.
- Bad, because GitHub attestation storage and linked artifact metadata integration would still need
  separate design.

### Invoke the `cosign` CLI from the reusable workflow

The workflow installs or vendors `cosign` and signs the provenance through `cosign attest-blob` or
an equivalent keyless command.

- Good, because `cosign` is the widely recognized Sigstore CLI and supports keyless signing,
  bundles, and broad verification workflows.
- Good, because it is less tied to GitHub artifact attestation storage than `actions/attest`.
- Bad, because installing, pinning, and verifying a CLI binary adds operational and supply-chain
  surface.
- Bad, because `cosign` has a broad container- and registry-oriented feature set beyond the initial
  npm package profile's needs.
- Bad, because GitHub artifact attestation API integration would be separate.

### Rely on npm automatic provenance

npm trusted publishing or `npm publish --provenance` creates npm ecosystem provenance automatically
instead of Windlass signing its own canonical provenance.

- Good, because it is npm ecosystem-native and naturally discoverable through npm tooling.
- Good, because trusted publishing can generate provenance without an npm publish token.
- Bad, because it conflicts with ADR 0029's Windlass-generated canonical provenance decision.
- Bad, because it does not make Windlass `builder.id`, `buildType`, and `externalParameters` the
  canonical verifier contract.
- Bad, because it would couple the production profile's SLSA semantics to npm's generated predicate
  shape rather than the Windlass build type specification.

### Implement the Sigstore protocol directly

The project implements Fulcio, Rekor, timestamp, bundle, and trust-root interactions without using a
maintained Sigstore client or GitHub action.

- Good, because it gives maximum implementation control.
- Bad, because it duplicates security-sensitive client behavior that maintained Sigstore tools
  already provide.
- Bad, because direct protocol implementation increases compatibility, maintenance, and
  vulnerability response burden.
- Bad, because it is unnecessary for the initial profile and would slow specification-driven
  development.

## More Information

This decision follows ADR 0029 and decides only the initial implementation adapter for signing
Windlass-generated SLSA provenance. It does not change the canonical provenance contents, package
manager policy, npm trusted publishing authentication decision, private dependency credential
policy, or SHA-pinned reusable workflow builder identity.

Reference points considered:

- SLSA v1.2 Build Requirements require authentic provenance at Build L2 and unforgeable provenance
  at Build L3. Secret material used to authenticate provenance must not be accessible to
  user-defined build steps.
- SLSA v1.2 Build Provenance separates signer identity from `builder.id` and requires consumers to
  accept only expected signer-builder pairs.
- GitHub `actions/attest` generates signed in-toto attestations with Sigstore-issued short-lived
  certificates, writes Sigstore bundle JSON, and can upload attestations to GitHub's attestation
  API.
- GitHub `actions/attest-build-provenance` v4 is a wrapper around `actions/attest`; new
  implementations should use `actions/attest` directly.
- GitHub `gh attestation verify` documents reusable workflow verification through signer workflow or
  signer repository checks and warns that predicate contents can be falsified if the workflow
  context is compromised.
- `sigstore-go` is production-ready and provides Go APIs for signing and verifying Sigstore bundles,
  but adopting it directly requires the Windlass trusted core to own more Sigstore behavior.
- `cosign` remains a stable Sigstore CLI with keyless signing, bundles, and attestation support, but
  its CLI installation and broad feature surface are larger than the initial adapter needs.
