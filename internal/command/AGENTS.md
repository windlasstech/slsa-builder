# Command Adapter Knowledge Base

## OVERVIEW

Typed dispatcher plus subcommand adapters wrapping `internal/` packages behind the single
`cmd/slsa-builder-internal` executable. Adapters preserve core semantics; no policy logic lives
here.

## SUBCOMMANDS

Registered via constructors in the `NewDispatcher(...)` call in `cmd/slsa-builder-internal/main.go`.

| Subcommand               | Delegates to                                         | Purpose                                     |
| ------------------------ | ---------------------------------------------------- | ------------------------------------------- |
| `fixture-check`          | `fixture`                                            | Validate the fixture registry and corpus.   |
| `npm-profile-build`      | `npmprofile` + `handoff` + `digest`                  | Build, pack, and hand off the npm tarball.  |
| `npm-profile-publish`    | `npmprofile` + `attestation` + `policy` + `handoff`  | Publish with provenance and attestation.    |
| `npm-profile-report`     | `diagnostic`                                         | Validate and persist the outcome report.    |
| `npm-profile-sign`       | `npmprofile` + `signing` + `attestation` + `handoff` | Sign and attest built provenance.           |
| `npm-profile-select`     | `npmprofile.Analyze`                                 | Resolve and describe the target package.    |
| `npm-profile-source-ref` | `npmprofile.ResolveSourceRefTag`                     | Resolve the tags-only build source ref.     |
| `verify-handoff`         | `handoff`                                            | Verify a producer-to-publisher handoff.     |
| `verify-attestation`     | `attestation` + `policy` + `provenance`              | Verify a signed attestation against policy. |
| `workflow-check`         | `workflowcheck`                                      | Static workflow conformance check.          |

## WHERE TO LOOK

| Task                         | Location                          | Notes                                                      |
| ---------------------------- | --------------------------------- | ---------------------------------------------------------- |
| Dispatcher and exit mapping  | `command.go`                      | `Command` interface, `Dispatch`, report writers.           |
| Shared I/O adapters          | `io.go`                           | `ReadTypedJSON`, `WriteFileAtomic`, outputs, redaction.    |
| Add a subcommand             | Steps in CONVENTIONS below        | Registration lives in `cmd/slsa-builder-internal/main.go`. |
| Workflow conformance catalog | `internal/workflowcheck/check.go` | Checks enforced by `workflow-check`.                       |
| Shared core rules            | `internal/AGENTS.md`              | Diagnostic registry, JCS reports, strict JSON, exit codes. |
| npm domain detail            | `internal/npmprofile/AGENTS.md`   | Profile behavior the npm adapters delegate to.             |

## CONVENTIONS

- **Registration**: Implement `Command` (`Name()` + `Execute(ctx, args, out) error`), then add a
  `New...Command()` constructor to `NewDispatcher(...)` in `cmd/slsa-builder-internal/main.go`.
  Unknown subcommands exit 2 with primary ID `windlass.verify.error.verifier-execution-failure`. No
  stubs: unimplemented subcommands are simply unregistered.
- **Exit mapping**: Return `ErrVerificationFailure` for exit 1 (a report must already be emitted);
  return `ErrInvocationFailure` for exit 2. Unexpected errors are redacted, then wrapped into an
  invocation report by the dispatcher.
- **Typed input**: `ReadTypedJSON[T]` accepts regular non-symlink files only, capped at 16 MiB,
  decoded as canonical strict JSON with unknown-field rejection and an optional semantic validator.
- **Atomic output**: `WriteFileAtomic` writes a same-dir temp file, fsyncs file and directory, then
  renames with owner-only 0600. An existing target must be regular and non-symlink.
- **`$GITHUB_OUTPUT`**: Each command declares a closed allowlist via `NewOutputAllowlist`.
  `WriteGitHubOutputs` rejects unknown names, newlines or NULs, and secret-like values; output is in
  deterministic sorted order, appended to a 0600 file.
- **Redaction**: `RedactSecrets` scrubs Bearer/Basic values, GitHub and npm token forms, and PEM
  private-key blocks. The dispatcher applies it to unexpected errors before reporting.
- **Flags**: Stdlib `flag` with `ContinueOnError` and discarded output. Validate required flags
  after parsing. `--help` prints usage and exits 0; usage never claims a public consumer CLI.

## ANTI-PATTERNS

- Never put policy or verification logic in adapters; delegate to `internal/` packages.
- Never write outputs non-atomically or follow symlinks for typed inputs or outputs.
- Never emit a free-form `$GITHUB_OUTPUT` name outside the command's allowlist.
- Never print secrets or unredacted wrapped errors.
- Never register placeholder or stub subcommands that succeed without doing the work.
