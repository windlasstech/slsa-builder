---
parent: Decisions
nav_order: 55
status: accepted
date: 12026-07-08
decision-makers: Yunseo Kim
---

# Use actions/attest Custom Mode for Statement Construction

## Context and Problem Statement

ADR 0029 selected Windlass-generated SLSA provenance as the canonical npm package provenance. ADR
0035 selected GitHub `actions/attest` as the initial Sigstore signing adapter, while preserving a
future migration path toward a Go-native signing adapter.

The architecture specs assumed that Windlass would construct a complete in-toto Statement and pass
that exact Statement payload to `actions/attest` for signing. The current stock `actions/attest`
public interface does not accept a complete Statement payload. Its custom attestation mode accepts
subject inputs plus a predicate type and predicate, then constructs the in-toto Statement before
signing it as a Sigstore bundle.

Should the initial production profile keep `actions/attest` and use its custom mode to construct the
Statement, replace it with a signer that accepts complete Statement bytes, or fall back to
`actions/attest` default provenance?

## Decision Drivers

- Preserve ADR 0029's Windlass-owned canonical SLSA provenance semantics.
- Preserve ADR 0035's initial use of stock, full-SHA-pinned `actions/attest` for Sigstore signing.
- Avoid relying on `actions/attest` default provenance, because that would not emit Windlass-owned
  `buildType`, `builder.id`, and `externalParameters` semantics.
- Match the current supported `actions/attest` interface rather than documenting a non-existent full
  Statement input mode.
- Keep verifier-visible Statement fields deterministic and testable even when the adapter assembles
  the Statement.
- Fail closed before publication if the signed bundle payload differs from the Statement implied by
  Windlass-verified subject inputs, predicate type, and predicate.
- Leave a future path to a signer that accepts complete Statement bytes without changing the initial
  production builder identity silently.

## Considered Options

- Use stock `actions/attest` custom attestation mode: Windlass supplies subject inputs, predicate
  type, and predicate; `actions/attest` constructs and signs the Statement.
- Replace `actions/attest` with a signer that accepts complete in-toto Statement bytes.
- Use `actions/attest` default provenance mode.
- Fork or wrap `actions/attest` to add a complete Statement input mode.

## Decision Outcome

Chosen option: "Use stock `actions/attest` custom attestation mode", because it matches the current
supported `actions/attest` interface while preserving the initial adapter decision from ADR 0035 and
the Windlass-owned provenance semantics from ADR 0029.

The initial production JS/TS npm package profile must invoke `actions/attest` in custom attestation
mode. Windlass is responsible for generating and validating:

- the expected subject name;
- the expected subject digest map;
- `predicate-type: https://slsa.dev/provenance/v1`;
- the complete SLSA provenance predicate, including `buildDefinition.buildType`,
  `buildDefinition.externalParameters`, `runDetails.builder.id`, and profile-defined fields.

`actions/attest` is responsible for constructing the in-toto Statement from those validated inputs,
signing the Statement with GitHub Actions OIDC-backed Sigstore identity, emitting the Sigstore
bundle, and optionally uploading the attestation to GitHub attestation storage.

The producer-side verification gate must inspect the signed bundle payload before publication. It
must reject the bundle if the emitted Statement does not match the Statement implied by the
Windlass-verified subject inputs, predicate type, and predicate. The gate must also perform the
existing signature, signer identity, `builder.id`, `buildType`, subject, and `externalParameters`
checks before `npm publish --provenance-file`.

The initial specs must not describe stock `actions/attest` as accepting a complete Statement
payload. They may still define the exact Statement shape that must appear in the signed bundle,
because that shape remains verifier-visible and must be checked after signing.

If a future adapter signs complete Statement bytes directly, a later ADR must define whether that is
a compatible adapter change under the same builder identity or a materially different signing mode
requiring distinct verifier guidance and possibly a distinct builder identity.

### Consequences

- Good, because the initial production profile stays on stock `actions/attest` without relying on an
  unsupported full-Statement input mode.
- Good, because Windlass still owns the verifier-relevant SLSA predicate and subject semantics.
- Good, because the emitted signed Statement remains testable by extracting and comparing the bundle
  payload before publication.
- Good, because this keeps GitHub OIDC, Fulcio/Rekor, Sigstore bundle emission, and optional GitHub
  attestation storage in the maintained GitHub action path.
- Neutral, because Statement JSON serialization is performed by `actions/attest`, not by Windlass,
  but the verifier-visible payload is checked after signing.
- Bad, because the signing adapter now participates in Statement assembly, not only cryptographic
  signing.
- Bad, because producer-side verification must include an explicit emitted-Statement comparison to
  prevent silent adapter drift.
- Bad, because full-SHA-pinned `actions/attest` upgrades must be reviewed for Statement assembly and
  bundle payload behavior, not only signing behavior.

### Confirmation

This decision is confirmed when the architecture specifications, implementation, fixtures, and
review guidance define:

- `actions/attest` custom attestation mode as the initial production invocation mode;
- Windlass-owned generation and validation of subject inputs, predicate type, and the SLSA
  provenance predicate;
- `actions/attest` responsibility for constructing and signing the in-toto Statement;
- producer-side extraction and comparison of the signed Statement payload before publication;
- rejection behavior for Statement mismatch, wrong `_type`, wrong `predicateType`, subject mismatch,
  wrong `builder.id`, wrong `buildType`, unexpected `externalParameters`, and adapter drift;
- fixtures proving that the emitted bundle payload matches the Statement implied by the validated
  inputs;
- documentation that default `actions/attest` provenance mode is not the canonical production npm
  provenance path.

## Pros and Cons of the Options

### Use stock `actions/attest` custom attestation mode

Windlass supplies subject inputs, `predicate-type`, and `predicate`/`predicate-path`;
`actions/attest` constructs the in-toto Statement and signs it as a Sigstore bundle.

- Good, because it matches the supported stock action interface.
- Good, because Windlass can still control the SLSA predicate, `buildType`, `builder.id`,
  `externalParameters`, and subject input values.
- Good, because it preserves the GitHub-native OIDC and Sigstore integration selected by ADR 0035.
- Good, because the signed bundle payload can be verified after signing and before publication.
- Bad, because Windlass does not control the final Statement serialization bytes before invoking the
  adapter.
- Bad, because action upgrades must be tested for Statement construction compatibility.

### Replace `actions/attest` with a signer that accepts complete Statement bytes

Windlass constructs the full in-toto Statement and passes those bytes to a signing tool or library.

- Good, because the signing adapter would only sign exact Windlass-generated Statement bytes.
- Good, because it matches the earlier spec wording more directly.
- Bad, because it conflicts with ADR 0035's initial adapter choice unless superseded or narrowed by
  a later ADR.
- Bad, because the project would need to own more Sigstore, GitHub OIDC, bundle, and storage
  behavior immediately.
- Bad, because the initial implementation risk is higher than needed for the current documentation
  and tooling scaffold.

### Use `actions/attest` default provenance mode

The workflow supplies only the subject and lets `actions/attest` generate GitHub Actions workflow
build provenance.

- Good, because it is the simplest `actions/attest` invocation.
- Good, because it aligns with GitHub artifact attestation defaults.
- Bad, because it does not preserve Windlass-owned `buildType`, `builder.id`, and
  `externalParameters` as the canonical npm provenance contract.
- Bad, because it conflicts with ADR 0029's Windlass-generated provenance decision.

### Fork or wrap `actions/attest` to add a complete Statement input mode

The project maintains a custom action or wrapper that accepts complete Statement bytes and delegates
to Sigstore signing behavior.

- Good, because it could combine exact Statement-byte control with a familiar GitHub Actions
  integration point.
- Neutral, because it may be a future compatibility strategy if upstream `actions/attest` does not
  add a full Statement input.
- Bad, because maintaining a fork or wrapper expands the trusted dependency and vulnerability
  response surface.
- Bad, because it is unnecessary for the initial profile when post-sign Statement verification can
  close the adapter-drift gap.

## More Information

Reference points considered:

- SLSA v1.2 Build Provenance defines `predicateType: https://slsa.dev/provenance/v1`, separates
  `buildDefinition` and `runDetails`, treats `externalParameters` as verifier-relevant external
  inputs, and requires consumers to accept only expected signer-builder pairs.
- SLSA v1.2 Software Attestations describe the attestation layers: envelope, Statement, and
  predicate. The Statement binds subjects to predicate metadata.
- The in-toto Statement v1 specification defines `_type: https://in-toto.io/Statement/v1`,
  `subject`, `predicateType`, and `predicate`.
- The `actions/attest` README and `action.yml` document custom attestation inputs as subject inputs
  plus `predicate-type` and either `predicate` or `predicate-path`; they do not document a complete
  Statement input.
- The `@actions/attest` implementation constructs the in-toto Statement from `subjects` and
  `{predicateType, predicate}`, serializes it as `application/vnd.in-toto+json`, signs it, and emits
  a Sigstore bundle.

Primary references:

- <https://slsa.dev/spec/v1.2/build-provenance>
- <https://slsa.dev/spec/v1.2/attestation-model>
- <https://github.com/in-toto/attestation/blob/7aefca35a0f74a6e0cb397a8c4a76558f54de571/spec/v1/statement.md>
- <https://github.com/actions/attest>
- <https://github.com/actions/attest/blob/main/action.yml>
- <https://github.com/actions/toolkit/blob/main/packages/attest/src/attest.ts>
- <https://github.com/actions/toolkit/blob/main/packages/attest/src/intoto.ts>
