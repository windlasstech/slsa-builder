---
run_id: 41666974638
attempt: 1
tag: refs/tags/v0.1.3
commit: ae5bb155d447bbaeeab2c40f79434d4760440546
operator: yunseo-kim
date: 2026-11-03T15:44:18Z
---

# npm M1 dogfood evidence

## Attempt 9 run identity

The frontmatter and this section identify the latest attempt-9 run. Older attempt sections carry
their own run identities.

- Caller: `windlasstech/vers-js/.github/workflows/publish.yml`
- Reusable workflow pin: `e7c55d9192e4493eb21d40d2b4dbf0bdfb1cbfc2` (carries the ADR 0101 stderr
  relay from PR #145 and the five-minute read-back budget from PR #146)
- Invocation ref and revision: `refs/heads/main`, `bd8b7ab116046e2c9bf8ee8682606ce643452c26`
- Built source ref and revision: `refs/tags/v0.1.3`, `ae5bb155d447bbaeeab2c40f79434d4760440546`
- Tag object and peeled commit: `ede69038d5878dcc53e29396237537da0a7f7b75`,
  `ae5bb155d447bbaeeab2c40f79434d4760440546`
- Caller workflow blob SHA (supporting evidence): `21f174854a9c5405a0504a7d4f17c97a2b93251e`
- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/41666974638>
- Event: `workflow_dispatch` (fixed-pipeline retry from `main` with `source-ref: refs/tags/v0.1.3`,
  per ADR 0079), attempt 1
- Pre-run registry state: `npm view @windlass/vers-js@0.1.3 version` returned `E404` (version
  absent)

## Outcome: fail-closed after npm E422 — full registry rejection captured (attempt 9, 12026-11-03)

Run <https://github.com/windlasstech/vers-js/actions/runs/41666974638>, `Publish npm package` job
<https://github.com/windlasstech/vers-js/actions/runs/41666974638/job/126026791130>. The caller-side
`Verify release` job, trusted-core `Build npm package` job, and `Sign npm provenance` job all
passed. The publish job passed pinned runtime and npm CLI provisioning and both handoff
verifications, then failed at `Publish with trusted-publisher convergence` with npm code `E422`
after 2.83 seconds, exactly as attempt 8. The caller-side `Publish GitHub Release` job was skipped.

This attempt exercised the ADR 0101 stderr relay for the first time. The relay captured the complete
npm CLI-rendered registry rejection message with no instrumentation change, satisfying the ADR 0101
confirmation criterion. The rejection is the routed predicate-mismatch form, and the registry now
names nine expected-value sources — the six catalogued workflow/GitHub fields plus two
`resolvedDependencies[0]` lines that pre-attempt-9 evidence had not shown:

```text
npm error code E422
npm error 422 Unprocessable Entity - PUT https://registry.npmjs.org/@windlass%2fvers-js - Error verifying sigstore provenance bundle: Invalid GitHub Actions SLSA v1 provenance predicate:
npm error buildDefinition.externalParameters.workflow.ref: <empty> does not match expected value for SourceRepositoryRef: refs/heads/main,
npm error buildDefinition.externalParameters.workflow.repository: <empty> does not match expected value for SourceRepositoryURI: https://github.com/windlasstech/vers-js,
npm error buildDefinition.externalParameters.workflow.path: .github/workflows/js-ts-npm-package-slsa3.yml does not match expected value for BuildConfigURI: .github/workflows/publish.yml,
npm error buildDefinition.internalParameters.github.event_name: <empty> does not match expected value for BuildTrigger: workflow_dispatch,
npm error buildDefinition.internalParameters.github.repository_id: <empty> does not match expected value for SourceRepositoryIdentifier: 1261293345,
npm error buildDefinition.internalParameters.github.repository_owner_id: <empty> does not match expected value for SourceRepositoryOwnerIdentifier: 257660296,
npm error buildDefinition.resolvedDependencies[0].uri: git+https://github.com/windlasstech/vers-js@ae5bb155d447bbaeeab2c40f79434d4760440546#pnpm-lock.yaml does not match expected value for SourceRepositoryURI: git+https://github.com/windlasstech/vers-js@refs/heads/main,
npm error buildDefinition.resolvedDependencies[0].digest.gitCommit: <empty> does not match expected value for SourceRepositoryDigest: bd8b7ab116046e2c9bf8ee8682606ce643452c26
```

First-party findings from this message:

- **`buildType` and `builder.id` are absent from the complaint.** The registry did not list the
  custom Windlass buildType (`https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`) or
  the Windlass builder identity among the mismatches, consistent with the post-12023-08 relaxation
  recorded in the research note. This is suggestive, not conclusive: the validator may evaluate
  those fields only after the reported mismatches are corrected, so acceptance remains unproven
  until a run with conforming fields reaches the registry.
- **Missing fields render as `<empty>`**, confirming the attempt-8 bundle lacks the fields rather
  than carrying mismatched values.
- **`workflow.path` must equal the certificate's BuildConfigURI**: the caller workflow path
  (`.github/workflows/publish.yml`), not the reusable workflow's own path.
- **`workflow.ref` is the invocation ref** (`refs/heads/main`), not the built tag — consistent with
  the ADR 0079/0080 dispatch-retry model in which certificate claims prove the invocation.
- **`resolvedDependencies[0]` is validated against the invocation source**: expected URI
  `git+https://github.com/windlasstech/vers-js@refs/heads/main` (SourceRepositoryURI at
  SourceRepositoryRef, with no lockfile fragment) and `digest.gitCommit` equal to the invocation
  head commit (`bd8b7ab...`), not the peeled tag commit. This requirement was not in any catalogued
  form and directly shapes the #143 projection.
- The failure summary emitted `code=E422 http_status=422` but **no `family=`**: the recognizer's
  complete-template matcher rejected the live form because the two `resolvedDependencies[0]` lines
  are not in its binding table. The relay carried the full text regardless, which is the ADR 0101
  enrichment fallback working as designed. Widening the family template is follow-up work under
  issue #142.

Post-mutation read-back made 21 observations over the new five-minute budget; the version and
attestation sets never changed, so the state machine returned `indeterminate` and emitted
`windlass.verify.error.registry-linkage-mismatch`, as in attempt 8. The run completed in
approximately 9 minutes 46 seconds, down from attempt 8's 20 minutes 25 seconds.

Canonical outcome report (no external text; the sealed contract is unchanged):

```json
{
  "diagnostic_metadata": { "publish_npm_version": "11.21.1" },
  "diagnostics": [
    {
      "category": "registry-linkage-mismatch",
      "check": "npm.publish.readback",
      "id": "windlass.verify.error.registry-linkage-mismatch",
      "message": "publication outcome is indeterminate",
      "severity": "error"
    }
  ],
  "exit_code": 1,
  "primary_id": "windlass.verify.error.registry-linkage-mismatch",
  "result": "fail",
  "run_invocation": "https://github.com/windlasstech/vers-js/actions/runs/41666974638/attempts/1",
  "schema_version": "1"
}
```

Post-run registry state confirms zero mutation: `npm view @windlass/vers-js@0.1.3 version` returns
`E404` and the attestations endpoint returns HTTP 404.

Preserved evidence (local): full publish-job log, outcome report, build metadata, package tarball,
and provenance bundle artifacts under the attempt-9 evidence directory; the rejection message is
quoted verbatim above.

## Attempt 8 run identity

The frontmatter and this section identify the latest attempt-8 run. Older attempt sections carry
their own run identities.

- Caller: `windlasstech/vers-js/.github/workflows/publish.yml`
- Reusable workflow pin: `033575798d70da3ae060d039767ad896f383ef70`
- Invocation ref and revision: `refs/heads/main`, `3687faf773ab1091dc189bf417aab5717a1a0769`
- Built source ref and revision: `refs/tags/v0.1.3`, `ae5bb155d447bbaeeab2c40f79434d4760440546`
- Tag object and peeled commit: `ede69038d5878dcc53e29396237537da0a7f7b75`,
  `ae5bb155d447bbaeeab2c40f79434d4760440546`
- Caller workflow blob SHA (supporting evidence): `40fa08efb9e23e9751daad54ab7d459a31865719`
- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/41488152839>
- Event: `workflow_dispatch` (fixed-pipeline retry from `main` with `source-ref: refs/tags/v0.1.3`,
  per ADR 0079), attempt 1
- Pre-run registry state: `npm view @windlass/vers-js@0.1.3 version` returned `E404` (version
  absent)

## Outcome: fail-closed after npm E422 — no registry publication observed (attempt 8, 12026-11-02)

Run <https://github.com/windlasstech/vers-js/actions/runs/41488152839>, `Publish npm package` job
<https://github.com/windlasstech/vers-js/actions/runs/41488152839/job/125425905149>. The caller-side
`Verify release` job, trusted-core `Build npm package` job, and `Sign npm provenance` job all
passed. The publish job passed pinned Node.js runtime provisioning, pinned npm CLI 11.21.1
provisioning, tarball handoff verification, and provenance bundle handoff verification, then failed
at `Publish with trusted-publisher convergence`. The caller-side `Publish GitHub Release` job was
skipped.

The new operational diagnostics established the exact stage and npm result. The pre-publish registry
observation was `package=present version=absent integrity=unknown attestations=absent`: the package
identity `@windlass/vers-js` already existed from earlier releases, while target version `0.1.3` was
absent. The pinned npm version check exited 0, and the actual publish command exited 1 after 2.93
seconds with npm code `E422` and HTTP status `422`. Post-mutation read-back made 61 observations
over 15 minutes; the version and attestation sets never changed, so the publish state machine
correctly returned `indeterminate` and emitted `windlass.verify.error.registry-linkage-mismatch`.

Operational diagnostics excerpt:

```text
windlass operational: readback after_mutation=false observations=1 changes=0 first="package=present version=absent integrity=unknown attestations=absent errors=none" final="package=present version=absent integrity=unknown attestations=absent errors=none" decision=absent
windlass operational: operation=npm start
windlass operational: operation=npm started=true outcome=exit exit=0 elapsed=68.199448ms output_truncated=false success=true
windlass operational: operation=npm start
windlass operational: operation=npm started=true outcome=exit exit=1 elapsed=2.932497713s output_truncated=false success=false
windlass operational: npm_failure code=E422 http_status=422
windlass operational: readback after_mutation=true observations=61 changes=0 first="package=present version=absent integrity=unknown attestations=absent errors=none" final="package=present version=absent integrity=unknown attestations=absent errors=none" decision=indeterminate
```

Canonical outcome report:

```json
{
  "diagnostic_metadata": {
    "publish_npm_version": "11.21.1"
  },
  "diagnostics": [
    {
      "category": "registry-linkage-mismatch",
      "check": "npm.publish.readback",
      "id": "windlass.verify.error.registry-linkage-mismatch",
      "message": "publication outcome is indeterminate",
      "severity": "error"
    }
  ],
  "exit_code": 1,
  "primary_id": "windlass.verify.error.registry-linkage-mismatch",
  "result": "fail",
  "run_invocation": "https://github.com/windlasstech/vers-js/actions/runs/41488152839/attempts/1",
  "schema_version": "1"
}
```

Post-run registry state confirms that no publication became visible:
`npm view @windlass/vers-js@0.1.3 version dist.integrity dist.shasum --json` returned `E404`, and
`https://registry.npmjs.org/-/npm/v1/attestations/%40windlass%2Fvers-js@0.1.3` returned HTTP 404. No
retry was attempted.

### Artifact and signer verification

The preserved tarball and Windlass-signed bundle passed online GitHub attestation verification with
the exact tarball, caller repository, issuer, SLSA v1 predicate type, and SHA-pinned reusable
workflow. This separates npm registry admission from bundle signature validity.

- Tarball SHA-256: `e3de6be684ff97651c09361d67680fddf1d5fe96d673433a88e2ddd3c03fa898`
- Tarball SHA-512:
  `e89f2c710fb587775984e7365bd80018df6d9b00a78b35c0838fe088b09f08747c6b04e568df06f48820b60c4ffb98d4897dc91a75ad741eabe89919dc5955af`
- Bundle SHA-256: `38cde7fbfb4e036991a8905b2af386fd298d604b6e8284d1cc2206d14d93da02`
- Build metadata SHA-256: `bfef3d1c1011ebbcf07c7dce088b95829943c4c35784e343cfe1dbe75f16a99f`
- Outcome report SHA-256: `498e334adfc2dbdf1f0666b1cfc8dfddd998753d834458fdccbd0d42533033de`

The signing certificate's public claims include issuer
`https://token.actions.githubusercontent.com`, source repository
`https://github.com/windlasstech/vers-js`, source repository ref `refs/heads/main`, build config
`https://github.com/windlasstech/vers-js/.github/workflows/publish.yml@refs/heads/main`, build
trigger `workflow_dispatch`, repository ID `1261293345`, owner ID `257660296`, and runner
environment `github-hosted`.

### Failure analysis

The registry response body was not captured, so the final server-side error message remains
unobserved. The evidence nevertheless narrows the likely cause substantially. npm CLI 11.21.1
supports `--provenance-file` and locally checks that the bundle has one subject, the expected npm
PURL, a matching SHA-512 digest, and a valid Sigstore signature. The preserved bundle satisfies
those checks. Registry admission applies additional npm-specific provenance policy beyond generic
Sigstore validity.

Comparing the registry-accepted npm-generated provenance for `@windlass/vers-js@0.1.2` with the
rejected attempt-8 bundle exposes a concrete compatibility gap:

| Field family                                             | Registry-accepted 0.1.2 bundle                    | Attempt-8 Windlass bundle                       |
| -------------------------------------------------------- | ------------------------------------------------- | ----------------------------------------------- |
| `buildDefinition.externalParameters.workflow.ref`        | `refs/heads/main`                                 | absent                                          |
| `buildDefinition.externalParameters.workflow.repository` | `https://github.com/windlasstech/vers-js`         | absent                                          |
| `buildDefinition.externalParameters.workflow.path`       | `.github/workflows/publish.yml`                   | `.github/workflows/js-ts-npm-package-slsa3.yml` |
| `buildDefinition.internalParameters.github`              | event, repository ID, and owner ID present        | `{}`                                            |
| Subject digest algorithms                                | `sha512`                                          | `sha256` and `sha512`                           |
| `buildDefinition.buildType`                              | GitHub workflow build type                        | Windlass npm package build type                 |
| `runDetails.builder.id`                                  | `https://github.com/actions/runner/github-hosted` | SHA-pinned Windlass reusable workflow           |

The leading hypothesis is therefore npm's certificate-to-predicate consistency validation for
GitHub-specific workflow fields, not a signing failure. The attempt-8 signing certificate carries
the repository, caller workflow, trigger, repository IDs, and runner environment listed above, but
the Windlass predicate does not expose npm's expected `workflow.ref`, `workflow.repository`, caller
workflow `path`, or `internalParameters.github` projection. Upstream evidence in
[slsa-github-generator#2499](https://github.com/slsa-framework/slsa-github-generator/issues/2499)
shows npm returning E422 for exactly this family of missing or mismatched fields.

The extra SHA-256 subject digest is a lower-confidence hypothesis. npm 11.21.1's client-side custom
provenance path checks `digest.sha512` and does not reject additional digest algorithms, so the
accepted 0.1.2 bundle's single digest is correlation rather than proof of a server-side
single-algorithm rule. A custom Windlass `buildType` or SHA-pinned `builder.id` is likewise not
proven: the upstream thread records that npm relaxed its earlier builder and build-type checks.

The missing predicate paths in the operational log are an instrumentation limitation, not proof that
rejection happened before predicate validation. The current recognizer only preserves a narrow
subset of npm's historical output forms; it does not retain all continuation-line, colon-delimited,
or `buildDefinition.internalParameters` paths. It also omits the request phase, so `E422` alone does
not prove which HTTP request npm considered failed.

### Resolution path

Per the runbook safety rules, no blind retry was attempted. The next step should follow the normal
observe → ADR → spec → TDD path. First extend the diagnostics contract with bounded,
fixed-vocabulary classification for the npm rejection family, request phase when positively
identified, and allowlisted field identifiers, using the upstream multiline E422 forms and attempt 8
as fixtures. Recover the exact registry rejection message before changing signed provenance
semantics. If the GitHub workflow-field hypothesis is confirmed, decide a compatibility mapping that
preserves the immutable Windlass builder identity and the signed distinction between the selected
source tag and the dispatch invocation ref. Do not remove SHA-256 or weaken builder identity
speculatively.

## Outcome: fail-closed at publish read-back — registry outcome indeterminate (attempt 7, 12026-11-02)

Run <https://github.com/windlasstech/vers-js/actions/runs/41427318706>, `Publish npm package` job
<https://github.com/windlasstech/vers-js/actions/runs/41427318706/job/125218381039>. The run was
dispatched from `main` at `02dfb25b0627462ac8580a224f7936513ee0a4d4`, with caller workflow blob
`d566674f363dd736e3feae459c664a8d924339f7` and reusable-workflow pin
`643077bd9a035a6bc943a56c1cb10ab6590e3beb` (the Corepack acquisition fix). The built source remained
`refs/tags/v0.1.3` at `ae5bb155d447bbaeeab2c40f79434d4760440546`.

The caller-side `Verify release` job, trusted-core `Build npm package` job, and
`Sign npm provenance` job all passed. This was the first `v0.1.3` attempt to reach the publish
mutation stage after the Corepack fix. The publish job passed pinned Node.js runtime provisioning,
pinned npm CLI 11.21.1 provisioning, tarball handoff verification, and provenance bundle handoff
verification, then failed at `Publish with trusted-publisher convergence` after approximately 15
minutes.

Canonical outcome report:

```json
{
  "diagnostic_metadata": {
    "publish_npm_version": "11.21.1"
  },
  "diagnostics": [
    {
      "category": "registry-linkage-mismatch",
      "check": "npm.publish.readback",
      "id": "windlass.verify.error.registry-linkage-mismatch",
      "message": "publication outcome is indeterminate",
      "severity": "error"
    }
  ],
  "exit_code": 1,
  "primary_id": "windlass.verify.error.registry-linkage-mismatch",
  "result": "fail",
  "run_invocation": "https://github.com/windlasstech/vers-js/actions/runs/41427318706/attempts/1",
  "schema_version": "1"
}
```

This run predates PR #141's operational diagnostics, so the log does not retain the npm error code,
HTTP status, or the read-back observation summary. The preserved outcome report proves that the
publish command reached the post-mutation read-back classification and returned `indeterminate`; it
does not identify the npm response that led there. Current registry reads after attempts 7 and 8
still show no visible `@windlass/vers-js@0.1.3` version or attestation.

Preserved attempt-7 artifacts:

- Package tarball: `js-ts-npm-package-tarball-41427318706-1` (artifact ID `13133095544`, SHA-256
  `e3de6be684ff97651c09361d67680fddf1d5fe96d673433a88e2ddd3c03fa898`, SHA-512
  `e89f2c710fb587775984e7365bd80018df6d9b00a78b35c0838fe088b09f08747c6b04e568df06f48820b60c4ffb98d4897dc91a75ad741eabe89919dc5955af`)
- Build metadata: `js-ts-npm-build-metadata-41427318706-1` (artifact ID `13132502338`, SHA-256
  `5a716ad6aaf3dc7186fdf132037afd99c706142daa1c11f7599333d386418563`)
- Provenance bundle: `js-ts-npm-provenance-bundle-41427318706-1` (artifact ID `13132895912`, SHA-256
  `8bc14c2030709f8682be18f79abb68b12fa807c8936d90e0d1eba2a13412afd8`)
- Outcome report: `js-ts-npm-outcome-report-41427318706-1` (artifact ID `13133197951`, SHA-256
  `d8a1cfa32c9e63cd7234519f303d6c863a307942df25d3d8d08005f068b5a4ea`)

The preserved bundle passed online GitHub attestation verification with the exact tarball, caller
repository, issuer, SLSA v1 predicate type, and the attempt-7 SHA-pinned signer workflow. Its
statement already carried the same npm compatibility gap later visible in attempt 8:
`externalParameters.workflow` contained only the Windlass builder path, SHA, and builder ID;
`internalParameters` was `{}`; the npm-accepted GitHub workflow projection was absent. The signing
certificate carried source repository `https://github.com/windlasstech/vers-js`, source ref
`refs/heads/main`, build config
`https://github.com/windlasstech/vers-js/.github/workflows/publish.yml@refs/heads/main`, trigger
`workflow_dispatch`, repository ID `1261293345`, owner ID `257660296`, and runner environment
`github-hosted`.

Attempt 7 therefore established that the remaining blocker was in the publish stage rather than
build, signing, provisioning, or handoff. Its exact npm response is unavailable because the run
predates PR #141. The attempt-8 pin differs from attempt 7 only by PR #141's diagnostics-only
change, and both preserved statements carry the same GitHub workflow compatibility gap. The same
underlying registry rejection is therefore likely for attempt 7, but that inference is not a
substitute for the missing attempt-7 server response. PR #141 added the operational diagnostics used
by attempt 8, which captured the missing npm result as `E422` / HTTP 422.

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

Not accepted by these runs: attempts 7, 8, and 9 advanced through build and provenance signing,
attempted one npm publication each, and failed closed in post-mutation read-back. Attempts 8 and 9
established the npm result as `E422`; attempt 9 captured the registry's complete rejection message
through the ADR 0101 stderr relay; attempt 7 predates the operational diagnostics. No registry
publication became visible, so all publication-side acceptance items remain open.

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

Attempt 9 is not applicable as a Phase 2 propagation measurement for the same reason as attempt 8:
npm returned `E422`, so no attestation propagation window applies. Its post-mutation read-back made
21 observations over the five-minute budget introduced by PR #146 and never observed the version or
its attestations.

Attempt 8 is not applicable as a Phase 2 propagation measurement: npm returned `E422`, no
publish-completion success was observed, and no attestation propagation window applies. The publish
command's own post-mutation read-back made 61 observations over 15 minutes and never observed the
version or its attestations.

Attempt 7's propagation status is unknown and was not measured. That run predates the operational
diagnostics, so the preserved evidence does not establish whether npm returned `E422`; it only
records that post-mutation read-back exhausted its budget with `indeterminate` and that no `0.1.3`
publication or attestation is visible now.

## Command outputs and preserved evidence

Attempt 9:

- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/41666974638>
- Publish-job URL:
  <https://github.com/windlasstech/vers-js/actions/runs/41666974638/job/126026791130>
- Publish failure diagnostics: the complete registry rejection message, quoted verbatim in the
  attempt-9 Outcome section above
- Post-run registry responses: `E404` on `npm view @windlass/vers-js@0.1.3 version`; HTTP 404 on
  `https://registry.npmjs.org/-/npm/v1/attestations/%40windlass%2Fvers-js@0.1.3`
- Package tarball artifact: `js-ts-npm-package-tarball-41666974638-1` (artifact ID `13246517763`,
  SHA-256 `e3de6be684ff97651c09361d67680fddf1d5fe96d673433a88e2ddd3c03fa898`, SHA-512
  `e89f2c710fb587775984e7365bd80018df6d9b00a78b35c0838fe088b09f08747c6b04e568df06f48820b60c4ffb98d4897dc91a75ad741eabe89919dc5955af`).
  Both digests are byte-identical to attempt 8's tarball, reconfirming deterministic build
  reproduction across runs.
- Build metadata artifact: `js-ts-npm-build-metadata-41666974638-1` (artifact ID `13246392710`,
  SHA-256 `a9a35eb9ee2243522a1fc98dfdfe39a38223872dbe5644471479d6508038bdff`)
- Provenance bundle artifact: `js-ts-npm-provenance-bundle-41666974638-1` (artifact ID
  `13246027941`, SHA-256 `98fc2325d3dcd26798b82214198509732acf136507ebf0f977bb6262c34735f3`)
- Outcome report artifact: `js-ts-npm-outcome-report-41666974638-1` (artifact ID `13245314214`,
  SHA-256 `cfee1692293e5799d943ff770946b46283e21fe793b509dce11916ead647e2d9`)

Attempt 8:

- Run URL: <https://github.com/windlasstech/vers-js/actions/runs/41488152839>
- Publish-job URL:
  <https://github.com/windlasstech/vers-js/actions/runs/41488152839/job/125425905149>
- Publish failure diagnostics: quoted verbatim in the attempt-8 Outcome section above
- Post-run registry responses: `E404` on
  `npm view @windlass/vers-js@0.1.3 version dist.integrity dist.shasum --json`; HTTP 404 on
  `https://registry.npmjs.org/-/npm/v1/attestations/%40windlass%2Fvers-js@0.1.3`
- Package tarball artifact: `js-ts-npm-package-tarball-41488152839-1` (artifact ID `13163735404`,
  SHA-256 `e3de6be684ff97651c09361d67680fddf1d5fe96d673433a88e2ddd3c03fa898`, SHA-512
  `e89f2c710fb587775984e7365bd80018df6d9b00a78b35c0838fe088b09f08747c6b04e568df06f48820b60c4ffb98d4897dc91a75ad741eabe89919dc5955af`)
- Build metadata artifact: `js-ts-npm-build-metadata-41488152839-1` (artifact ID `13163685299`,
  SHA-256 `bfef3d1c1011ebbcf07c7dce088b95829943c4c35784e343cfe1dbe75f16a99f`)
- Provenance bundle artifact: `js-ts-npm-provenance-bundle-41488152839-1` (artifact ID
  `13162589843`, SHA-256 `38cde7fbfb4e036991a8905b2af386fd298d604b6e8284d1cc2206d14d93da02`)
- Outcome report artifact: `js-ts-npm-outcome-report-41488152839-1` (artifact ID `13163438563`,
  SHA-256 `498e334adfc2dbdf1f0666b1cfc8dfddd998753d834458fdccbd0d42533033de`)
- Preserved bundle verification: `gh attestation verify` passed online for the exact tarball and
  bundle, with repository, issuer, SLSA v1 predicate, and SHA-pinned signer-workflow constraints
- X.509 post-processing result: public certificate claims quoted in the attempt-8 Outcome section
- npm consumer verification JSON: not applicable (nothing published)
- Foreign-conflict diagnostic: not applicable (nothing published)
