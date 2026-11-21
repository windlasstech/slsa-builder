# Dependabot companion knowledge base

## Scope

Repository maintenance only. `Planner.DryRun` owns the closed local projection and deterministic
generator. Remote integration must reuse it, not reimplement descriptor or fixture transformations.
See [dependency selection](../../docs/dependency-selection.md#dependabot-companion-integration).

## Boundaries

- `remote_event.go` binds actual completed Lint payloads to one PR. Titles are not dependency
  identity.
- `github.go` owns authenticated, bounded, redirect-free GitHub HTTP. Production has no injection
  API; internal tests use `httptest` through unexported fields.
- `remote_projection.go` re-reads PR eligibility and exact base/head files. `remote_contents.go`
  supplies release/tagged module/license evidence. Inputs are data, never candidate execution.
- `remote.go` reuses the local planner, regenerates before apply, and compares plan hashes.
- `app_identity.go` binds the installation token to the App bot before replacing the DCO
  placeholder.
- `graphql_commit.go` sends exactly one `createCommitOnBranch` with expected head and exact
  contents. `reconcile.go` checks OID, parent, message, content, verified signature, and branch ref.
  An ambiguous mutation/read-back stops with no retry or rollback.
- `workflow_security_test.go` rejects unsafe workflow mutations. Both existing fuzz targets are
  registered in the smoke and weekly matrices; the event target also covers the raw webhook parser.

## Rules

Keep production and test files below 250 nonblank, noncomment lines. Preserve the output allowlist
and local dry-run contract. Do not touch accepted ADRs, dogfood evidence, or current generated
fixtures to make planning work. Never perform live GitHub writes during tests. The workflow and CLI
apply gates must both remain explicit. Local tests do not prove live App signing, CI retriggering,
or branch-protection enforcement.
