# Platform Contract Spike Fixtures

> [!CAUTION] These files are non-production evidence copies from F03 spikes performed on
> 12026-08-05. The workflow files are retained for review and must not be installed as production
> workflows.

This directory pins the external behavior that blocks implementation of the initial npm profile. The
evidence combines reusable-workflow observations from the private conformance repository with a
successful run in a public, temporary repository. The temporary repository was used only because
GitHub artifact attestation storage is unavailable to private repositories on the active plan.

## Pinned outcomes

- Reusable-workflow OIDC tokens contain nonempty `job_workflow_ref` and 40-hex `job_workflow_sha`
  claims for the immutable called workflow. The raw token was never persisted.
- `actions/attest@v4.2.2` custom mode emits `attestation.json` as a Sigstore bundle with media type
  `application/vnd.dev.sigstore.bundle.v0.3+json`.
- Two calls with identical subject and predicate inputs produced different complete bundle bytes but
  identical signed Statement payload bytes. Consumers must preserve the emitted bundle exactly and
  must not require byte-identical signatures.
- GitHub repository attestation listing is digest-scoped. The installed `gh` CLI had no
  `attestation list` subcommand; `gh attestation download` and the digest REST endpoint returned the
  two records.
- `queue: max` materialized on github.com with `cancel-in-progress: false`. GitHub documents a
  100-pending limit and cancellation after the queue is full. Runtime overflow and GHES parity
  remain deferred to L03.
- actionlint 1.7.12 rejects `queue: max`; the fixture records the exact diagnostic and upstream
  tracking issue. A narrow compatibility policy belongs to N04/P05, not this evidence task.
- Node.js `v24.18.0` and npm `11.16.0` accepted an external Sigstore bundle through
  `npm publish --dry-run --provenance-file` with no token or authentication fallback. npm 9.7.0 is
  the minimum version documented by the upstream release that introduced the option.
- npm's attestation endpoint returns an `attestations` array. Select provenance by `predicateType`,
  not array position. Published run identity appears in both certificate OID
  `1.3.6.1.4.1.57264.1.21` and `predicate.runDetails.metadata.invocationId`.

## GitHub SLSA storage constraint

The public spike also found that GitHub attestation storage applies semantic validation when the
predicate type is `https://slsa.dev/provenance/v1`. With storage enabled it rejected the
Windlass-style custom build type as unsupported. The successful adapter probe therefore used a
freeform custom predicate URI to isolate bundle emission and storage read-back.

This does not contradict ADR 0055: that ADR makes GitHub attestation storage optional. Production
signing must use the required SLSA predicate and preserve the emitted bundle, but must not depend on
GitHub storage accepting a Windlass custom-buildType provenance record.

## Evidence map

| File                            | Contract                                                    |
| ------------------------------- | ----------------------------------------------------------- |
| `platform-contract-report.json` | Acceptance summary and source-run split                     |
| `oidc-reusable-workflow.json`   | Reusable-workflow OIDC claims                               |
| `actions-attest-custom.json`    | Action pin, basename, bundle shape, and byte comparison     |
| `valid.intoto.jsonl`            | Exact first bundle emitted by `actions/attest`              |
| `github-attestation-store.json` | Digest REST and `gh` read-back shape                        |
| `queue-max.json`                | github.com syntax acceptance and documented queue behavior  |
| `actionlint-1.7.12.json`        | Current parser incompatibility                              |
| `npm-cli-provenance-file.json`  | npm version, exact argv form, and no-token dry-run          |
| `npm-attestation-readback.json` | npm endpoint shape and run-identity location                |
| `*.spike.yml`                   | Non-production workflow copies used to produce the evidence |

## Official sources

- [GitHub OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc)
- [`actions/attest` v4.2.2](https://github.com/actions/attest/releases/tag/v4.2.2)
- [Artifact attestations REST API](https://docs.github.com/en/rest/attestations/attestations)
- [GitHub Actions concurrency](https://docs.github.com/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/control-the-concurrency-of-workflows-and-jobs)
- [Larger concurrency queues announcement](https://github.blog/changelog/2026-05-07-github-actions-concurrency-groups-now-allow-larger-queues/)
- [actionlint 1.7.12](https://github.com/rhysd/actionlint/releases/tag/v1.7.12) and
  [queue support issue](https://github.com/rhysd/actionlint/issues/705)
- [npm CLI 9.7.0](https://github.com/npm/cli/releases/tag/v9.7.0) and
  [`provenance-file` implementation PR](https://github.com/npm/cli/pull/6490)
- [npm provenance documentation](https://docs.npmjs.com/generating-provenance-statements/)

## Reproduction

Run the copied workflows only in isolated repositories. For the checked-in npm fixture, the local
non-publishing acceptance check is:

```bash
npm publish --dry-run \
  --provenance-file testdata/platform/contracts/valid.intoto.jsonl \
  testdata/npm/packages/unscoped-valid
```

The command must exit zero without publishing or falling back to token authentication.
