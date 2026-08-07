---
parent: Decisions
nav_order: 77
status: accepted
date: 12026-08-06
decision-makers: Yunseo Kim
relations:
  - type: supersedes
    target: ADR-0035
  - type: supersedes
    target: ADR-0055
  - type: partially-supersedes
    target: ADR-0071
    scope:
      "the signing-adapter descriptor for every producer profile identifies the governed sigstore-go
      module instead of actions/attest; field classification and closed-set policy remain"
---

# Use a Go-Native Sigstore DSSE Signer for Windlass Provenance Signing

## Context and Problem Statement

Windlass owns the exact in-toto Statement bytes that bind each profile's subject and predicate. Its
shared subject convention is one subject with every profile-required digest in that subject's digest
map; the npm profile demonstrates the strictest current instance through ADR 0064: one npm Package
URL subject with SHA-512 and SHA-256 over the same packed tarball bytes. P01 now assembles those
exact Statement bytes in the trusted Go core.

The initial adapter selected by ADR 0035 and ADR 0055 cannot sign a caller-owned complete Statement
through its stock public interface. The demonstrated failure is the npm subject required by
ADR 0064. In `actions/attest` v4.2.2, `subject-digest` represents one algorithm, `subject-path`
computes SHA-256 only, and `subject-checksums` creates one subject per checksum line. Supplying
separate SHA-512 and SHA-256 checksum lines therefore produces two subjects rather than one subject
with a two-member digest map.

That platform-real two-subject shape cannot be rescued by entry ordering or by npm behavior. npm
CLI's `libnpmpublish/lib/provenance.js` rejects a bundle whose signed payload has
`subject.length > 1` with `Found more than one subject in the sigstore bundle payload` before it
contacts the registry. The stock adapter therefore cannot satisfy the unchanged ADR 0064 contract on
the official `npm publish --provenance-file` path. This is a proven instance of a structural adapter
defect, not an npm-local quirk: an adapter that cannot accept Windlass-owned complete Statement
bytes cannot preserve the shared single-subject, profile-defined digest contract in general.

Should Windlass replace the proven-unusable adapter only for npm, or adopt one signing adapter now
for npm, release-manifest signing, and every future profile while preserving existing Sigstore
verification and SLSA Build L3 isolation?

## Decision Drivers

- Treat `actions/attest`'s inability to accept a complete Statement and construct one subject with
  multiple digests as a proven structural defect, demonstrated by v4.2.2 and npm CLI rejection, not
  an npm-local quirk.
- Preserve Windlass's shared single-subject, profile-defined digest convention, including ADR 0064's
  npm Package URL with SHA-512 and SHA-256 over the same tarball bytes.
- Sign the exact Statement bytes already assembled and validated by the trusted Go core rather than
  reconstructing the Statement in an adapter.
- Keep GitHub Actions OIDC, ephemeral signing keys, Fulcio certificates, SCTs, and bundle-contained
  Rekor evidence under the ADR 0068 and ADR 0069 verification policy.
- Preserve signing-job isolation: caller-controlled build steps and publish credentials must not
  enter the signing boundary.
- Keep the official `npm publish --provenance-file` and registry read-back paths for the first
  production adoption.
- Record the dependency that actually performs signing rather than claiming that `actions/attest`
  produced the bundle.
- Reuse the verified npm bundle unchanged as the release-asset provenance sidecar.
- Reuse one governed signer for Wave 4 release-manifest signing and future profiles instead of
  preserving a known-unusable default.
- Do not invert risk logic by keeping a proven-unusable default merely because its replacement is
  not yet production-proven; transition now, then evaluate remediation or rollback through follow-up
  ADRs if controlled dogfood surfaces defects.
- Minimize total transition risk and cost by avoiding an npm-only adoption, proof cycle, and second
  generalization ADR.

## Considered Options

- Use a Go-native DSSE signer backed by `sigstore-go` v1.3.0 for all Windlass signing.
- Use the Go-native signer only for npm, retain `actions/attest` elsewhere, and generalize later.
- Keep stock `actions/attest` and submit one checksum line per digest algorithm.
- Keep stock `actions/attest` and weaken ADR 0064 to one digest.
- Fork or wrap `actions/attest` to add a complete-Statement input.
- Replace Windlass provenance with npm automatic provenance.
- Bypass npm CLI and attach provenance through a custom registry path.

## Decision Outcome

Chosen option: "Use a Go-native DSSE signer backed by `sigstore-go` v1.3.0 for all Windlass
signing", because it signs exact trusted-core Statement bytes, satisfies npm CLI's single-subject
validation, and preserves the existing keyless Sigstore and isolated signing boundary without
retaining a proven-unusable default or paying for a second generalization transition.

The Go-native signer is **the** Windlass signing adapter. The npm profile adopts it now; Wave 4
release-manifest signing in M01/M02 reuses it; every future profile defaults to it. `actions/attest`
is retired as a Windlass signing adapter. It may remain in historical evidence or be used outside
the Windlass signing-adapter role, but no production Windlass profile may select it for provenance
or release-manifest signing.

The trusted Go core remains responsible for assembling and validating each complete in-toto
Statement. The signing job must pass those exact preassembled bytes to `sigstore-go`'s DSSE signing
path as `sign.DSSEData` with payload type `application/vnd.in-toto+json`. It must not reconstruct,
normalize, or reserialize the Statement before signing. For npm, the Statement contains exactly one
npm Package URL subject whose digest object contains lowercase hexadecimal `sha512` and `sha256`
values computed over identical packed tarball bytes. For the release manifest and future profiles,
the same exact-byte rule applies to their ADR-backed Statement contracts as those profiles arrive.

The signing job obtains a GitHub Actions OIDC token, creates an ephemeral signing key, obtains the
corresponding Fulcio certificate and embedded SCT, and emits a Sigstore bundle containing the DSSE
envelope and Rekor evidence required by ADR 0069. No signing key may persist beyond the job. The
signing boundary must not accept caller-provided signing keys, arbitrary OIDC tokens, npm tokens,
publish credentials, or caller-controlled steps.

Every producer profile's `runDetails.builder.builderDependencies` contains exactly one closed
descriptor for the dependency that performs DSSE signing:

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
version and checksum used by the signing binary. No producer profile may retain the
`git+https://github.com/actions/attest@...` descriptor. Each profile's `resolvedDependencies` set
remains the artifact-affecting set defined by its contract; the signer remains in
`builderDependencies` under ADR 0071's SLSA field classification. Release-manifest Statements do not
use the SLSA provenance `builderDependencies` field, but M01/M02 must use the same governed signer.

Before publication, C06 must verify every provenance bundle offline under the existing ADR 0068 and
ADR 0069 policy and compare the extracted DSSE payload byte-for-byte with the preassembled
Statement. The npm publish job then submits the exact verified bundle through
`npm publish --provenance-file`. A controlled pre-P06 publication must prove real npm publish
success, registry attestation read-back, and pacote consumer verification before the production path
is accepted. The release-manifest path applies the same exact-payload verification before upload.

The same verified producer bundle bytes remain the Wave 5 provenance sidecar. The publisher must not
re-sign, reconstruct, or normalize them. GitHub artifact attestation storage is optional and
disabled for the npm Go-signer path while GitHub rejects the Windlass custom `buildType`; it may be
reconsidered only if that restriction changes. Storage integration is separate from the signing
adapter and does not justify retaining `actions/attest` as a signer.

### Consequences

- Good, because one signer accepts every Windlass-owned complete Statement and preserves the shared
  single-subject, profile-defined digest convention.
- Good, because the npm production signer can emit exactly one npm Package URL subject with both
  required digests without weakening ADR 0064.
- Good, because the trusted core's exact Statement bytes become the DSSE payload instead of being
  reconstructed by a JavaScript action.
- Good, because GitHub Actions OIDC, ephemeral keys, Fulcio identity, SCT validation, Rekor
  transparency, and offline verification remain unchanged in security effect.
- Good, because every producer's `builderDependencies` honestly identifies the Go signing dependency
  used to produce the bundle.
- Good, because the verified bundle remains reusable byte-for-byte for npm publication and the Wave
  5 release sidecar.
- Good, because the isolated signing job and authenticated handoff preserve the organization's SLSA
  Build L3 target.
- Good, because the builder identity is unchanged: the signer identity (GitHub Actions OIDC), bundle
  format, `builder.id` meaning, and verifier expectations are identical to the superseded adapter
  path, so ADR 0035's distinct-builder-identity migration criterion does not trigger.
- Neutral, because C06's cryptographic verification core is unchanged; it verifies a different
  conforming producer bundle and adds exact payload comparison.
- Good, because npm, Wave 4 release-manifest signing, and future profiles share one adapter contract
  and avoid a second migration ADR and implementation transition.
- Neutral, because P01's Statement assembly is largely reused and P02 becomes the first
  implementation effort; generalization applies as release-manifest signing and future profiles
  arrive.
- Neutral, because Go-signer contract failures retain the registered
  `windlass.verify.error.actions-attest-adapter-contract` diagnostic ID as a historical machine
  name; any rename or signer-specific replacement is deferred to P02 so code and specification
  change atomically.
- Bad, because Windlass now owns Fulcio, SCT, Rekor, bundle assembly, and DSSE signing integration
  that the action previously encapsulated.
- Bad, because GitHub attestation storage remains unavailable for this path while its custom
  `buildType` restriction applies.
- Bad, because the full transition precedes production dogfood; any defect requires remediation or a
  rollback decision through a follow-up ADR.
- Bad, because production acceptance now requires a controlled real npm publication and consumer
  verification rather than only a dry-run compatibility check.

The F03 two-subject fixture is retained unchanged as historical evidence of stock `actions/attest`
behavior. Implementation adds a separate production-signer bundle fixture; it must not rewrite the
F03 evidence to resemble the chosen signer. A non-blocking upstream feature request should ask
`actions/attest` to support a single subject with multiple digest algorithms, but Windlass
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
- if dogfood evidence from the controlled publish, P06, or pacote verification surfaces signer
  defects, a follow-up ADR evaluates remediation or rollback;
- Wave 5 redistributes the same verified bundle bytes as the provenance sidecar without re-signing
  or rewriting them;
- M01/M02 release-manifest signing reuses the same Go-native adapter, signs exact preassembled
  release-manifest Statement bytes, and verifies the payload byte-for-byte before upload;
- each future profile uses this adapter by default and applies the same exact-byte contract as its
  profile-specific Statement and verification rules are admitted;
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

### Use the Go-native signer only for npm and generalize later

- Good, because it would limit the first implementation surface until npm dogfood completes.
- Bad, because it retains a proven-unusable default solely because the replacement is not yet
  production-proven, which inverts the risk logic.
- Bad, because it creates two adapter contracts, two transition points, and a second generalization
  ADR even though the structural defect and replacement boundary are already known.

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

This decision supersedes ADR 0035 in full: its only self-owned decisions were the adapter selection
and adapter-specific guidance, and its anticipated `sigstore-go` migration is realized here; the
isolation and verifier requirements it cited were inherited from ADR 0029 and SLSA v1.2 rather than
decided there. It supersedes ADR 0055 in full for the same reason: custom-mode Statement
construction was its sole decision, while Windlass-owned semantics belong to ADR 0029 and post-sign
payload verification is absorbed into the ADR 0069 verification policy. It partially supersedes ADR
0071's signing-adapter descriptor for every producer profile while preserving ADR 0071's field
classification and closed-set policy. ADR 0064 is unchanged; its npm subject remains the
demonstrated instance that proved the structural defect.

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
