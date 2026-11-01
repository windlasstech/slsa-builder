---
run_id: 41348956569
attempt: 1
tag: refs/tags/v0.1.3
commit: ae5bb155d447bbaeeab2c40f79434d4760440546
operator: yunseo-kim
date: 2026-11-01T20:51:04Z
---

# npm M1 dogfood evidence

## Run identity

- Caller: `windlasstech/vers-js/.github/workflows/publish.yml`
- Reusable workflow pin: `b2b0d7af1493035346168ae7fdc83f70672ac50e`
- Invocation ref and revision: `refs/heads/main`, `790ec48f4f8c39ae662208a7e7b2b75278e4703d`
- Built source ref and revision: `refs/tags/v0.1.3`, `ae5bb155d447bbaeeab2c40f79434d4760440546`
- Tag object and peeled commit: `ede69038d5878dcc53e29396237537da0a7f7b75`,
  `ae5bb155d447bbaeeab2c40f79434d4760440546`
- Caller workflow blob SHA (supporting evidence): `8139850c75363d716f63faef33bcbaf75701690d`
- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/41348956569>
- Event: `workflow_dispatch` (fixed-pipeline retry from `main` with `source-ref: refs/tags/v0.1.3`,
  per ADR 0079), attempt 1
- Pre-run registry state: `npm view @windlass/vers-js@0.1.3 version` returned `E404` (version
  absent)

## Outcome: fail-closed at build-and-pack — zero mutation (attempt 6, 12026-11-01)

Run <https://github.com/windlasstech/vers-js/actions/runs/41348956569>, `Build npm package` job
`124974970580`. The run failed in the `Build npm package` job at the
`Build and pack selected package` step, after the toolchain assertions and during Corepack
acquisition capture. The `Sign npm provenance` job was skipped, the `Publish npm package` job
reported success with every mutation step skipped (no bundle handoff), and the caller-side
`Publish GitHub Release` job was skipped. Post-run registry state confirmed no mutation:
`npm view @windlass/vers-js@0.1.3 version` returns `E404`.

Failure diagnostic (verbatim, `npm-profile-build`):

```json
{
  "diagnostics": [
    {
      "category": "verifier-execution-failure",
      "check": "command.dispatch",
      "id": "windlass.verify.error.verifier-execution-failure",
      "message": "subcommand \"npm-profile-build\" failed: corepack did not report the acquired distribution URL",
      "severity": "error"
    }
  ],
  "exit_code": 2,
  "primary_id": "windlass.verify.error.verifier-execution-failure",
  "result": "fail",
  "run_invocation": null,
  "schema_version": "1"
}
```

### Newly exercised and passed before the failure

- ADR 0100 runner-image capture (the attempt-5 failure point): the run progressed past
  `captureRunner` with the on-disk metadata authoritative and the control-plane deployment version
  recorded as separate evidence — first live pass of the fix.
- `npm-profile-provision-node` (ADR 0098) and the pre-install toolchain pair assertion (ADR 0085)
  passed again on the provisioned Node.js 24.22.1 runtime.
- The Corepack acquisition path was reached for the first time since attempt 4, and the first live
  exercise of an integrity-suffixed descriptor: `v0.1.3` declares `pnpm@11.28.8+sha512.f6988a32...`
  (ADR 0092), whereas attempt 4's `v0.1.2` declared the plain `pnpm@11.17.0`. The executed-manager
  version assertion passed: Corepack resolved and executed pnpm 11.28.8 correctly.

### Root cause (reproduced locally with the provisioned Corepack)

The descriptor carries an integrity suffix, and the acquisition parser keyed Corepack's `Installing`
debug line on the full descriptor (`pnpm@11.28.8+sha512.f6988a32...`), while Corepack prints the
plain version in that line. A local reproduction with a fresh `COREPACK_HOME` and `DEBUG=corepack`
shows the exact line:

```text
corepack Installing pnpm@11.28.8 from https://registry.npmjs.org/pnpm/-/pnpm-11.28.8.tgz
```

The reproduction also shows the Corepack cache directory keys on the plain version
(`v1/pnpm/11.28.8/`), while the `.corepack` metadata's `locator.reference` keeps the full descriptor
including the hash. Two of the implementation's three keying assumptions therefore disagree with
live Corepack behavior: the `Installing` line and the cache path are plain-version keyed, and only
the locator reference is descriptor keyed. The version assertion and the descriptor's own integrity
still hold, so the failure is a parser/lookup keying defect, not an acquisition or signing defect.

Observability note: the failure diagnostic carries neither Corepack's acquisition output nor the
expected key, so the root cause required a local reproduction; recording the observed output (or at
least the expected prefix) in the diagnostic would remove that round trip.

### Resolution path

Same observe → ADR → spec → TDD path as the earlier attempts: record the live Corepack keying
behavior, decide the parser/lookup correction (candidate: key the `Installing` line and the cache
path on the plain version, keep the locator `reference` exact match on the full descriptor as the
integrity binding, and keep the exact acquired-URL equality check), amend the specification, fix
with TDD, bump the vers-js caller pin, and retry. Per the runbook's safety rules no blind retry was
attempted; the registry state is untouched and `v0.1.3` remains the retry target (no republication
occurred, so no version conflict exists).

## Outcome: fail-closed at build-and-pack — zero mutation (attempt 5, 12026-11-01)

Run <https://github.com/windlasstech/vers-js/actions/runs/41324911503>, `Build npm package` job
`124902911522`. The run failed in the `Build npm package` job at the
`Build and pack selected package` step, before any install, build, or pack execution. The
`Sign npm provenance` job was skipped, the `Publish npm package` job reported success with every
mutation step skipped (no bundle handoff), and the caller-side `Publish GitHub Release` job was
skipped. Post-run registry state confirmed no mutation: `npm view @windlass/vers-js@0.1.3 version`
returns `E404`, and the registry attestations endpoint for `@windlass/vers-js@0.1.3` returns
HTTP 404.

Failure diagnostic (verbatim, `npm-profile-build`):

```json
{
  "diagnostics": [
    {
      "category": "verifier-execution-failure",
      "check": "command.dispatch",
      "id": "windlass.verify.error.verifier-execution-failure",
      "message": "subcommand \"npm-profile-build\" failed: runner image metadata version disagrees with ImageVersion",
      "severity": "error"
    }
  ],
  "exit_code": 2,
  "primary_id": "windlass.verify.error.verifier-execution-failure",
  "result": "fail",
  "run_invocation": null,
  "schema_version": "1"
}
```

### Newly exercised and passed before the failure

- `npm-profile-provision-node` (ADR 0098): provisioned the pinned Node.js 24.22.1 runtime from the
  digest-verified release tarball on the live runner — first live exercise, passed.
- Pre-install toolchain pair assertion (ADR 0085): the pinned Node.js 24.22.1 / bundled npm 11.19.1
  pair assertion passed on the provisioned runtime — first live exercise, passed.
- The caller's `Verify release` job passed, including the tag ↔ `package.json` ↔ CHANGELOG binding
  for `v0.1.3`.

### Root cause (pinned against upstream and a live probe)

`captureRunner` (`internal/npmprofile/toolchain.go`) requires the `Version:` line of the
`Runner Image` group in `/imagegeneration/imagedata.json` to equal the `ImageVersion` environment
variable exactly. A mutation-free probe (temporary push-triggered workflow on a throwaway vers-js
branch, run [41326580253](https://github.com/windlasstech/vers-js/actions/runs/41326580253), branch
deleted afterwards) captured the real bytes on the same runner fleet and reproduced the mismatch:

- `ImageVersion` environment variable: **`20261026.354.1`**
- `/imagegeneration/imagedata.json`, Runner Image group `Version:` line: **`20261025.353.1`**
- The job header's Runner Image group prints `Version: 20261025.353.1`, matching the file, not the
  environment variable.

The probe captured one concrete divergence on one runner fleet: the control plane injected the newer
deployment's version into `ImageVersion` while the VM's baked imagedata.json still carried the
previous image's version. Read together with the upstream rollout-window reports
([runner-images#11574](https://github.com/actions/runner-images/issues/11574),
[runner-images#12720](https://github.com/actions/runner-images/issues/12720)) and the tag absence
below, the observation is consistent with an image rollout in flight; whatever the mechanism, the
two sources demonstrably diverged, so strict equality between them cannot be assumed. The
environment variable and the file also disagreed on the OS label form (`ubuntu24` vs
`ubuntu-24.04`); the implementation at that run compared only the version. Supporting upstream
facts:

- The public tag sequence around the deployment is `ubuntu24/20261023.334` →
  `ubuntu24/20261026.354`; no public `ubuntu24/20261025.353` tag exists, so the on-disk image
  `20261025.353.1` has no corresponding published image-data snapshot.
- The image-data document at the newer tag (`ubuntu24/20261026.354`) records
  `Image Version: 20261026.354.1`, matching the injected environment variable.

Observability note: the failure diagnostic carries neither operand, so the probe above was needed to
pin the file-side value; including both observed values in the diagnostic would remove that round
trip (compare the `statement-sha256` observability precedent of #95).

### Resolution path

The fix followed the same observe → ADR → spec → TDD path as the earlier attempts: the probe above
pinned the observed relationship, ADR 0100 recorded the authority split, the
`js-ts-npm-build-pack.md` runner-capture clause was amended, and the TDD implementation landed on
the fix branch for PR #139. Remaining after that branch merges: bump the vers-js caller pin and
retry `v0.1.3`. Per the runbook's safety rules no blind retry was attempted; the registry state is
untouched and `v0.1.3` remains the retry target (no republication occurred, so no version conflict
exists).

## P06/M1 acceptance checklist

Not advanced by this run: the run failed before provenance signing and before any registry mutation.
All publication-side acceptance items remain open.

- [ ] The real `@windlass/vers-js@0.1.3` version exists in npm.
- [ ] Downloaded tarball bytes match recorded SHA-256 and SHA-512 values.
- [ ] The Statement subject is the expected npm Package URL and carries both digests.
- [ ] Preserved and registry bundles have equal DSSE payload bytes and equal signature sets.
- [ ] Full bundle-file byte equality: `<yes/no, diagnostic only>`.
- [ ] Sigstore online verification used governed public-good TUF roots with no offline fallback.
- [ ] `builder.id`, URI SAN, Build Signer URI, and Build Signer Digest use the full pinned SHA.
- [ ] ADR 0068 bindings pass: issuer, numeric caller repository and owner IDs, runner environment,
      built source identity, invocation context, and Run Invocation URI.
- [ ] `internalParameters` is exactly `{}`.
- [ ] The early npm OIDC exchange preflight succeeded before mutation; no token fallback occurred.
- [ ] Issue #22 settings-only pnpm root-package resolution passed with no
      `package-resolution-invalid` diagnostic.
- [ ] Issue #81 source and invocation fields match the required values.
- [ ] `npm audit signatures` consumer verification passed.
- [ ] Same-run re-run converged without a second publish.
- [ ] New-run duplicate dispatch failed with `windlass.verify.error.foreign-conflict`.

## Propagation observations

Not applicable: nothing was published, so there is no publish-completion timestamp and no
attestation propagation to measure (ADR 0073).

## Command outputs and preserved evidence

- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/41348956569>
- Build-job failure diagnostic: quoted verbatim in the attempt-6 Outcome section above; the full job
  log was retrieved via the Actions API for job `124974970580` at review time. Earlier attempts
  carry their run URL and job ID inline in their own Outcome sections
- Post-run registry responses: `E404` on `npm view @windlass/vers-js@0.1.3 version`; HTTP 404 on
  `https://registry.npmjs.org/-/npm/v1/attestations/@windlass%2fvers-js@0.1.3`
- Preserved artifact name and digest: not applicable (no handoff artifacts were produced)
- `gh attestation verify` JSON: not applicable (nothing published)
- X.509 post-processing result: not applicable (nothing published)
- npm consumer verification JSON: not applicable (nothing published)
- Foreign-conflict diagnostic: not applicable (nothing published)
