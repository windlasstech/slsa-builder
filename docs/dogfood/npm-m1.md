---
run_id: 31840088262
attempt: 1
tag: refs/tags/v0.1.3
commit: <PEELED_SOURCE_REVISION>
operator: yunseo-kim
date: 2026-08-14T20:54:42Z
---

# npm M1 dogfood evidence

## Run identity

- Caller: `windlasstech/vers-js/.github/workflows/publish.yml`
- Reusable workflow pin: `5964b19c5e4fa704595acbbe2ae30cc99fcbeb41`
- Invocation ref and revision: `refs/heads/main`, `817e15b3686665f51774cb029c585be2cded765e`
- Tag object and peeled commit: `518569c61b8f52ddf22b3fa98f2a145a03048f68`,
  `68e6f0018d6824167ec490243b17a9edd6ba8fc7`
- Caller workflow blob SHA (supporting evidence): `3d5bc7b55375a50571ebab8db69b854a48276c9b`
- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/31840088262>
- Event: `workflow_dispatch`, attempt 1, `release_tag=v0.1.2`
- Pre-run registry state: `npm view @windlass/vers-js@0.1.2 version` returned `E404` (version
  absent)

## P06/M1 acceptance checklist

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

| Observation timestamp | Request | Packument version/integrity | Attestation HTTP status | Windlass provenance visible | Notes      |
| --------------------- | ------: | --------------------------- | ----------------------: | --------------------------- | ---------- |
| `<ISO-8601>`          |   `<n>` | `<value or error>`          |              `<status>` | `<yes/no>`                  | `<detail>` |

- Publish completion: `<ISO-8601>`
- First authenticated Windlass attestation visibility: `<ISO-8601>`
- Convergence duration: `<duration>`
- Requests: `<count>`

## Command outputs and preserved evidence

- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/31840088262>
- Publish, build, and re-run log excerpts: `<links or quoted excerpts>`
- Registry response and selected bundle: `<paths or links>`
- Preserved artifact name and digest: `<name and SHA-256>`
- `gh attestation verify` JSON: `<path or excerpt>`
- X.509 post-processing result: `<path or excerpt>`
- npm consumer verification JSON: `<path or excerpt>`
- Foreign-conflict diagnostic: `<run URL and excerpt>`
