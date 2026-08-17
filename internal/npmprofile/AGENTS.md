# npm Profile Knowledge Base

## OVERVIEW

Owns the JS/TS npm profile end to end: package and manager selection, build/pack, provenance inputs,
OIDC exchange, registry observation, and publish convergence, per `docs/architecture/js-ts-npm-*.md`
specs. Largest and most security-sensitive package in the Go trusted core. Root `AGENTS.md` and
`internal/AGENTS.md` shared rules apply; this file covers only npmprofile-specific knowledge.

## STRUCTURE

```text
internal/npmprofile/
├── Selection and resolution:  types.go, resolution.go, workspace.go, selection.go
│     Entry: Analyze(Config) (Result, error)
├── Build-pack execution:      build_pack.go, build_pack_types.go, packed.go, toolchain.go, exec_boundary.go
│     Entry: BuildPack(ctx, BuildPackConfig)
├── Provenance inputs:         provenance_input.go, provenance_types.go, provenance_validate.go, workflow_metadata.go
│     Entries: NewProvenanceSigningInput, ValidateNPMStatement,
│     EncodeExternalParameters/DecodeExternalParameters, NPMPackagePURL,
│     ResolveWorkflowPublishIntent
├── OIDC + HTTP + secrets:     oidc_client.go, http_client.go, secret.go
│     Entries: NewOIDCClient, AcquireIdentity/Preflight/Exchange, SecretToken
├── Registry client:           registry_client.go
│     Entries: NewRegistryClient, Preflight (packument), Attestations
├── Publish state machine:     publish.go, publish_verifier.go
│     Entries: Publish, NewSigstorePublishBundleVerifier
│     States: committed-as-expected / absent / foreign-conflict / indeterminate
└── Source-ref (ADR 0079/0080): source_ref.go
      Entries: NormalizeSourceRefInput, ValidateSourceRefInput, ResolveSourceRefTag
      (tags-only, isolated fetch, peel-to-commit proof)
```

Tests sit beside sources; live-tool tests are confined to `build_pack_live_test.go`.

## WHERE TO LOOK

| Task or topic                                        | Location                                                                   |
| ---------------------------------------------------- | -------------------------------------------------------------------------- |
| Manager selection rules                              | `selection.go` + `docs/architecture/js-ts-npm-build-pack.md`               |
| OIDC exchange contract (ADR 0081)                    | `oidc_client.go` — union epoch/RFC3339 timestamps, 15-min token lifetime   |
| Publish convergence (ADR 0073/0076)                  | `publish.go` + `js-ts-npm-provenance-publish.md`                           |
| Source-ref validation and resolution (ADR 0079/0080) | `source_ref.go`                                                            |
| Packument/attestation registry observation           | `registry_client.go`                                                       |
| Fixtures                                             | repo-root `testdata/npm/` + registration in `testdata/fixtures/index.json` |

## CONVENTIONS

- **Manifest-first manager selection**: `packageManager`, then `devEngines.packageManager`. pnpm
  requires an exact version. Yarn only Berry v4+ via Corepack. Lockfile inference applies to npm
  only. Stale non-selected lockfiles are diagnostics, never silently used.
- **Settings-only `pnpm-workspace.yaml`** (no `packages` member) means standalone root-package mode
  (ADR 0078); a present-but-invalid `packages` member stays malformed.
- **Exec boundary**: only allowlisted absolute tool basenames under trusted roots; argv arrays,
  never shell strings.
- **Build and pack**: build scripts run only when declared (`scripts.build`); exactly one tarball
  per pack; packed name and version are re-verified against source metadata.
- **Publish mutation discipline**: at most one argv-only npm mutation after all preflights pass;
  ambient npm token and env config are stripped first; same-run retry converges; foreign evidence is
  never adopted.
- **Hermetic tests**: inject fake toolchains and fetchers. Real-tool tests live only in
  `build_pack_live_test.go` behind `-short` plus `SLSA_BUILDER_LIVE_TOOLCHAIN=1`.
- **Provenance goldens**: byte-exact JCS; regenerate only via the documented `UPDATE_*` gates.

## ANTI-PATTERNS

- Never add an npm token, OTP, or `NPM_CONFIG_*` fallback for OIDC failures. Fail closed per spec
  (`trusted-publisher-mismatch`, `npm-oidc-exchange-indeterminate`).
- Never follow redirects, credential-bearing URLs or proxies, or custom TLS hooks in HTTP clients;
  bound response sizes.
- Never weaken `SecretToken` redaction or log token material.
- Never run a fixture through `npm publish`; the fixture test asserts no fixture command contains
  it.
- Never accept a source-ref that is not a full `refs/tags/<name>` ref; never create, move, or delete
  tags during resolution; never let whitespace-only input mean omission (exact empty string only).
- Never publish the initial version of a brand-new package identity.
- Never let npm automatic provenance substitute for the Windlass-signed bundle; publish uses the
  exact verified bundle bytes (`--provenance-file`). See ADRs 0082-0085 for the pinned npm CLI
  provisioning this depends on.
