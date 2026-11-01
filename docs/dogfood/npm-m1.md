---
run_id: 41324911503
attempt: 1
tag: refs/tags/v0.1.3
commit: ae5bb155d447bbaeeab2c40f79434d4760440546
operator: yunseo-kim
date: 2026-11-01T16:10:07Z
---

# npm M1 dogfood evidence

## Run identity

- Caller: `windlasstech/vers-js/.github/workflows/publish.yml`
- Reusable workflow pin: `e044243a5cffb8f2cd2bc1bdb685b28edb8c5cfd`
- Invocation ref and revision: `refs/tags/v0.1.3`, `ae5bb155d447bbaeeab2c40f79434d4760440546`
- Tag object and peeled commit: `ede69038d5878dcc53e29396237537da0a7f7b75`,
  `ae5bb155d447bbaeeab2c40f79434d4760440546`
- Caller workflow blob SHA (supporting evidence): `605fae9f1c5a58af30740f34fab1f0b748084861`
- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/41324911503>
- Event: `push` (tag push of `v0.1.3`), attempt 1
- Pre-run registry state: `npm view @windlass/vers-js@0.1.3 version` returned `E404` (version
  absent)

## Outcome: fail-closed at build-and-pack — zero mutation (attempt 5, 12026-11-01)

The run failed in the `Build npm package` job at the `Build and pack selected package` step, before
any install, build, or pack execution. The `Sign npm provenance` job was skipped, the
`Publish npm package` job reported success with every mutation step skipped (no bundle handoff), and
the caller-side `Publish GitHub Release` job was skipped. Post-run registry state confirmed no
mutation: `npm view @windlass/vers-js@0.1.3 version` returns `E404`, and the registry attestations
endpoint for `@windlass/vers-js@0.1.3` returns HTTP 404.

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

The probe pins the platform behavior precisely: during an image rollout window the control plane
injects the newer deployment's version into `ImageVersion` while the VM's baked imagedata.json still
carries the previous image's version, so strict equality between the two is not a platform
invariant. The environment variable and the file also disagree on the OS label form (`ubuntu24` vs
`ubuntu-24.04`); the current implementation only compares the version. Supporting upstream facts:

- The public tag sequence around the deployment is `ubuntu24/20261023.334` →
  `ubuntu24/20261026.354`; no public `ubuntu24/20261025.353` tag exists, so the on-disk image
  `20261025.353.1` has no corresponding published image-data snapshot.
- The image-data document at the newer tag (`ubuntu24/20261026.354`) records
  `Image Version: 20261026.354.1`, matching the injected environment variable.

Observability note: the failure diagnostic carries neither operand, so the probe above was needed to
pin the file-side value; including both observed values in the diagnostic would remove that round
trip (compare the `statement-sha256` observability precedent of #95).

### Resolution path

Same observe → ADR → spec → TDD path as the earlier attempts: pin the empirically observed
relationship between `ImageVersion` and the imagedata.json Runner Image version (with a
mutation-free probe capturing the file bytes if a live observation is wanted), amend
`js-ts-npm-build-pack.md`'s runner-capture clause, fix with TDD, bump the vers-js caller pin, and
retry. Per the runbook's safety rules no blind retry was attempted; the registry state is untouched
and `v0.1.3` remains the retry target (no republication occurred, so no version conflict exists).

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

- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/41324911503>
- Build-job failure diagnostic: quoted verbatim in the Outcome section above; the full job log was
  retrieved via the Actions API for job `124902911522` at review time
- Post-run registry responses: `E404` on `npm view @windlass/vers-js@0.1.3 version`; HTTP 404 on
  `https://registry.npmjs.org/-/npm/v1/attestations/@windlass%2fvers-js@0.1.3`
- Preserved artifact name and digest: not applicable (no handoff artifacts were produced)
- `gh attestation verify` JSON: not applicable (nothing published)
- X.509 post-processing result: not applicable (nothing published)
- npm consumer verification JSON: not applicable (nothing published)
- Foreign-conflict diagnostic: not applicable (nothing published)
