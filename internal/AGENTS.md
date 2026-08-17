# Trusted-Core Go Knowledge Base

## OVERVIEW

Go trusted core for SLSA provenance: module `github.com/windlasstech/slsa-builder`. All packages are
internal, no public Go API. Sole executable is `cmd/slsa-builder-internal`.

## STRUCTURE

```text
internal/
├── attestation/      # Sigstore bundle + in-toto Statement verification; Fulcio/SCT/Rekor/SET; TUF online + offline pins.
├── canonicaljson/    # Duplicate-member-rejecting strict JSON + RFC 8785 JCS canonicalization.
├── command/          # Typed dispatcher + subcommand adapters. Has its own AGENTS.md.
├── dependencyproof/  # Dependency-proof model and validation.
├── diagnostic/       # Closed diagnostic registry + canonical reports.
├── digest/           # Typed SHA-256/SHA-512 digests.
├── fixture/          # Executable conformance-fixture registry and loader.
├── handoff/          # Digest-bound one-file artifact handoff verification.
├── identity/         # Immutable builder/source/runner identity.
├── npmprofile/       # npm profile domain. Has its own AGENTS.md.
├── policy/           # Closed verifier-policy decode + producer-policy intersection.
├── provenance/       # Profile-neutral in-toto/SLSA v1 models.
├── signing/          # Go-native Sigstore DSSE signing for GitHub Actions (ADR 0077).
├── workflowcheck/    # Static workflow conformance: permissions, SHA pins, harden-runner-first, job graph, queue:max.
└── AGENTS.md         # this file
```

## WHERE TO LOOK

| Task                       | Location                                                                        | Notes                                                     |
| -------------------------- | ------------------------------------------------------------------------------- | --------------------------------------------------------- |
| Diagnostic taxonomy change | `internal/diagnostic` + `docs/architecture/verification-policy-and-fixtures.md` | `registry_spec_test.go` enforces registry/spec parity.    |
| Add a subcommand           | `internal/command/AGENTS.md`                                                    | Dispatcher and adapter conventions live there.            |
| npm domain behavior        | `internal/npmprofile/AGENTS.md`                                                 | Profile-specific rules live there.                        |
| Fixture registration       | `testdata/AGENTS.md` + `internal/fixture`                                       | Fixture layout and loader contract.                       |
| Test policy                | `docs/testing-guide.md`                                                         | Organization, fuzzing, quality gates, gated `Live` tests. |

## CONVENTIONS

- **Closed diagnostic registry**: IDs are lowercase kebab-case under `windlass.verify.error.*` /
  `windlass.verify.warning.*`. Severity, phase, exit code, mutation-possibility, and precedence are
  registry-controlled in `internal/diagnostic/registry.go`. Any taxonomy change must update the spec
  tables; `registry_spec_test.go` enforces parity by parsing the spec.
- **Canonical reports**: Reports are RFC 8785 JCS canonical bytes. The first ordered error
  determines result, exit code, and primary ID. Evidence values are secret-safe scalars only.
- **Strict JSON everywhere**: Duplicate object members are rejected at every depth BEFORE semantic
  validation (`internal/canonicaljson`). Typed decoders reject unknown fields.
- **Offline means offline**: Offline verification makes zero network calls; tests replace
  `http.DefaultTransport` to prove it. Online mode never falls back to pinned roots.
- **Hermetic tests**: Fake servers via `httptest`, fake toolchains on a private PATH. Live tests are
  `*Live`-named, skipped under `-short`, gated by env (`WINDLASS_TEST_ONLINE=1`,
  `SLSA_BUILDER_LIVE_TOOLCHAIN=1`), never in CI. Golden updates are gated by `UPDATE_*_GOLDEN=1` env
  vars. New trust-boundary parsers require fuzz coverage; every fuzz target is registered in BOTH
  `.github/workflows/lint.yml` (30s smoke matrix) and `.github/workflows/fuzz.yml` (10min weekly).
- **Exit codes**: 0 pass, 1 verification/policy failure, 2 invocation failure.
- **Secret handling**: Tokens are wrapped in redacting types (`String`/`GoString` return
  `[REDACTED]`). Command adapters re-redact unexpected errors.

## ANTI-PATTERNS

- Never parse trusted JSON/JWT in shell or YAML. Go owns all trusted parsing; shell is invocation
  glue only.
- Never weaken: redaction, credential stripping (`NPM_TOKEN`/`NODE_AUTH_TOKEN`/`NPM_CONFIG_*`
  ambient env stripped before publish), redirect refusal, TLS/proxy hardening, executable
  allowlists, digest rebinding on handoff receipt.
- Never accept a handoff without recomputing the digest. Fail closed on mismatch.
- Never mutate remote state after an ambiguous read-back. Foreign-conflict and indeterminate both
  stop mutation.
- Never add a diagnostic ID without registering it and updating the spec tables. Never emit
  non-canonical report bytes.
- Never use Node.js/pnpm/Yarn or JS libraries for trusted logic.
