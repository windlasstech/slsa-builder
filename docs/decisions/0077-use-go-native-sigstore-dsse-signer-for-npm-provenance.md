---
parent: Decisions
nav_order: 77
status: accepted
date: 12026-08-06
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0035
    scope:
      "the npm profile's signing adapter becomes the Go-native signer; actions/attest delegation
      remains for other uses"
  - type: partially-supersedes
    target: ADR-0055
    scope:
      "stock actions/attest custom mode is replaced as the npm signing adapter because it cannot
      construct the ADR 0064 subject shape and npm CLI rejects multi-subject bundles"
  - type: partially-supersedes
    target: ADR-0071
    scope:
      "the npm signing-adapter descriptor in builderDependencies/resolvedDependencies identifies the
      Go signer instead of actions/attest"
---

# Use a Go-Native Sigstore DSSE Signer for npm Provenance

## Context and Problem Statement

ADR 0064 requires the npm provenance Statement to contain exactly one subject named by the npm
Package URL, with both the SHA-512 and SHA-256 digests of the same packed tarball bytes. P01 now
assembles those exact Statement bytes in the trusted Go core.

The initial adapter selected by ADR 0035 and ADR 0055 cannot sign that Statement shape through its
stock public interface. In `actions/attest` v4.2.2, `subject-digest` represents one algorithm,
`subject-path` computes SHA-256 only, and `subject-checksums` creates one subject per checksum line.
Supplying separate SHA-512 and SHA-256 checksum lines therefore produces two subjects rather than
one subject with a two-member digest map.

That platform-real two-subject shape cannot be rescued by entry ordering or by npm behavior. npm
CLI's `libnpmpublish/lib/provenance.js` rejects a bundle whose signed payload has
`subject.length > 1` with `Found more than one subject in the sigstore bundle payload` before it
contacts the registry. The stock adapter therefore cannot satisfy the unchanged ADR 0064 contract on
the official `npm publish --provenance-file` path.

How should the npm profile sign the exact one-subject, dual-digest Statement while preserving the
existing Sigstore verification, SLSA Build L3 isolation, official npm publication, and provenance
sidecar contracts?

## Decision Drivers

- Preserve ADR 0064's exact one-subject npm Package URL shape with SHA-512 and SHA-256 over the same
  tarball bytes.
- Sign the exact Statement bytes already assembled and validated by the trusted Go core rather than
  reconstructing the Statement in an adapter.
- Keep GitHub Actions OIDC, ephemeral signing keys, Fulcio certificates, SCTs, and bundle-contained
  Rekor evidence under the ADR 0068 and ADR 0069 verification policy.
- Preserve signing-job isolation: caller-controlled build steps and publish credentials must not
  enter the signing boundary.
- Keep the official `npm publish --provenance-file` and registry read-back paths.
- Record the dependency that actually performs npm signing rather than claiming that
  `actions/attest` produced the bundle.
- Reuse the verified npm bundle unchanged as the release-asset provenance sidecar.

## Considered Options

- Use a Go-native DSSE signer backed by `sigstore-go` v1.3.0.
- Keep stock `actions/attest` and submit one checksum line per digest algorithm.
- Keep stock `actions/attest` and weaken ADR 0064 to one digest.
- Fork or wrap `actions/attest` to add a complete-Statement input.
- Replace Windlass provenance with npm automatic provenance.
- Bypass npm CLI and attach provenance through a custom registry path.

## Decision Outcome

Chosen option: "Use a Go-native DSSE signer backed by `sigstore-go` v1.3.0", because it signs the
exact trusted-core Statement bytes, satisfies npm CLI's single-subject validation, and preserves the
existing keyless Sigstore and isolated signing boundary without weakening ADR 0064.

P01 remains responsible for assembling and validating the complete in-toto Statement. The Statement
must contain exactly one npm Package URL subject whose `digest` object contains lowercase
hexadecimal `sha512` and `sha256` values computed over identical packed tarball bytes. The signing
job must pass those exact preassembled bytes to `sigstore-go`'s DSSE signing path as `sign.DSSEData`
with payload type `application/vnd.in-toto+json`. It must not reconstruct, normalize, or reserialize
the Statement before signing.

The signing job obtains a GitHub Actions OIDC token, creates an ephemeral signing key, obtains the
corresponding Fulcio certificate and embedded SCT, and emits a Sigstore bundle containing the DSSE
envelope and Rekor evidence required by ADR 0069. No signing key may persist beyond the job. The
signing boundary must not accept caller-provided signing keys, arbitrary OIDC tokens, npm tokens,
publish credentials, or caller-controlled steps.

The npm profile's `runDetails.builder.builderDependencies` contains exactly one closed descriptor
for the dependency that performs DSSE signing:

```json
{
  "uri": "pkg:golang/github.com/sigstore/sigstore-go@v1.3.0",
  "digest": {
    "h1": "hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE="
  },
  "annotations": {
    "role": "signing-adapter"
  }
}
```

The `h1` value is the checksum recorded for `github.com/sigstore/sigstore-go v1.3.0` in the governed
Go module dependency graph. The descriptor is verifier-visible and must agree with the module
version and checksum used by the signing binary. The npm profile must not retain the
`git+https://github.com/actions/attest@...` descriptor. The profile's `resolvedDependencies` set
remains the artifact-affecting set defined by ADR 0070; the signer remains in `builderDependencies`
under ADR 0071's SLSA field classification.

Before publication, C06 must verify the bundle offline under the existing ADR 0068 and ADR 0069
policy and compare the extracted DSSE payload byte-for-byte with the preassembled Statement. The npm
publish job then submits the exact verified bundle through `npm publish --provenance-file`. A
controlled pre-P06 publication must prove real npm publish success, registry attestation read-back,
and pacote consumer verification before the production path is accepted.

The same verified bundle bytes remain the Wave 5 provenance sidecar. The publisher must not re-sign,
reconstruct, or normalize them. GitHub artifact attestation storage is optional and disabled for the
npm Go-signer path while GitHub rejects the Windlass custom `buildType`; it may be reconsidered only
if that restriction changes. This partial supersession does not replace `actions/attest` for release
manifest signing or another profile that can satisfy its own subject and storage contracts.

### Consequences

- Good, because the production signer can emit exactly one npm Package URL subject with both
  required digests without weakening ADR 0064.
- Good, because the trusted core's exact Statement bytes become the DSSE payload instead of being
  reconstructed by a JavaScript action.
- Good, because GitHub Actions OIDC, ephemeral keys, Fulcio identity, SCT validation, Rekor
  transparency, and offline verification remain unchanged in security effect.
- Good, because `builderDependencies` honestly identifies the Go signing dependency used to produce
  the bundle.
- Good, because the verified bundle remains reusable byte-for-byte for npm publication and the Wave
  5 release sidecar.
- Good, because the isolated signing job and authenticated handoff preserve the organization's SLSA
  Build L3 target.
- Neutral, because C06's cryptographic verification core is unchanged; it verifies a different
  conforming producer bundle and adds exact payload comparison.
- Neutral, because P01's Statement assembly is largely reused and P02 becomes the primary
  implementation effort.
- Bad, because Windlass now owns Fulcio, SCT, Rekor, bundle assembly, and DSSE signing integration
  that the action previously encapsulated.
- Bad, because GitHub attestation storage remains unavailable for this path while its custom
  `buildType` restriction applies.
- Bad, because production acceptance now requires a controlled real npm publication and consumer
  verification rather than only a dry-run compatibility check.

The F03 two-subject fixture is retained unchanged as historical evidence of stock `actions/attest`
behavior. Implementation adds a separate production-signer bundle fixture; it must not rewrite the
F03 evidence to resemble the chosen signer. A non-blocking upstream feature request should ask
`actions/attest` to support a single subject with multiple digest algorithms, but the npm
implementation does not wait for that request.

### Confirmation

This decision is confirmed when:

- the npm Statement contains exactly one npm Package URL subject with lowercase hexadecimal SHA-512
  and SHA-256 digests over identical packed tarball bytes;
- the exact preassembled Statement bytes are the DSSE payload with media type
  `application/vnd.in-toto+json`;
- signing uses GitHub Actions OIDC, an ephemeral key, a Fulcio certificate with an SCT, and
  bundle-contained Rekor evidence;
- C06 successfully verifies the bundle offline and compares the extracted Statement bytes exactly
  before publication;
- the closed verifier-visible `builderDependencies` descriptor identifies the actual `sigstore-go`
  signing dependency and never claims an `actions/attest` revision;
- no signing key, arbitrary OIDC token, npm token, publish credential, or caller-controlled step can
  enter the signing boundary;
- a controlled pre-P06 run successfully publishes to npm, reads the provenance back from the
  registry, and verifies the package and provenance through pacote as a consumer;
- Wave 5 redistributes the same verified bundle bytes as the provenance sidecar without re-signing
  or rewriting them;
- GitHub attestation storage remains disabled for the npm path unless its custom `buildType`
  restriction changes;
- the F03 two-subject fixture remains unchanged and implementation adds a separate conforming
  Go-signer bundle fixture; and
- an upstream `actions/attest` feature request is filed without blocking P02 implementation.

## Pros and Cons of the Options

### Use a Go-native DSSE signer backed by sigstore-go

- Good, because it accepts the exact Statement bytes and preserves the required subject shape.
- Good, because the existing Go trust root, bundle, and verification dependency can serve signing as
  well as verification.
- Good, because the signing credentials remain ephemeral and isolated in one trusted job.
- Bad, because Windlass assumes direct responsibility for the complete keyless signing protocol.

### Keep stock actions/attest with one checksum line per digest

- Good, because it retains the previously selected action and its maintained GitHub integration.
- Bad, because it emits multiple subjects and npm CLI rejects the bundle before registry contact.
- Bad, because checksum-line order cannot change the subject cardinality.

### Weaken ADR 0064 to one digest

- Good, because either stock `subject-digest` or `subject-path` could express the reduced shape.
- Bad, because dropping SHA-512 breaks npm's integrity validation and dropping SHA-256 breaks the
  Windlass handoff and sidecar binding contract.

### Fork or wrap actions/attest

- Good, because a fork could expose complete-Statement signing while retaining familiar action
  plumbing.
- Bad, because it creates a security-sensitive fork and JavaScript dependency surface when the
  trusted core is already Go and `sigstore-go` is already governed.

### Use npm automatic provenance or bypass npm CLI

- Good, because npm automatic provenance avoids custom bundle construction, while a custom registry
  path could bypass npm CLI's subject check.
- Bad, because automatic provenance loses Windlass-owned predicate semantics, and a custom registry
  path abandons the official no-secret publication contract.

## More Information

This decision partially supersedes only the npm-profile clauses of ADR 0035 and ADR 0055. It leaves
their `actions/attest` delegation available for other uses. It also partially supersedes ADR 0071's
npm signing-adapter descriptor while preserving ADR 0071's field classification and closed-set
policy.

Reference points:

- [`actions/attest` v4.2.2](https://github.com/actions/attest/releases/tag/v4.2.2) and its
  [action inputs](https://github.com/actions/attest/blob/v4.2.2/action.yml);
- npm CLI
  [`libnpmpublish/lib/provenance.js`](https://github.com/npm/cli/blob/release/v11/workspaces/libnpmpublish/lib/provenance.js);
- `sigstore-go` v1.3.0 signing APIs in
  [`pkg/sign`](https://pkg.go.dev/github.com/sigstore/sigstore-go/pkg/sign);
- [Sigstore bundle format](https://docs.sigstore.dev/about/bundle/); and
- [ADR 0069](0069-require-rekor-transparency-and-govern-sigstore-trust-root.md) for the unchanged
  transparency and trust-root contract.
