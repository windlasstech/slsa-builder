# npm M1 dogfood verification procedure

Run this procedure for the live P06 M1 dogfood retry on 12026-08-14. It publishes
`@windlass/vers-js@0.1.2` through the SHA-pinned reusable workflow, collects the publication
evidence, and records the completed run in `docs/dogfood/npm-m1.md`. Do not create that evidence
record until the run begins.

The caller workflow is `.github/workflows/publish.yml`. Earlier planning material called it
`release.yml`; that filename is obsolete for this runbook.

## Fixed target

| Item                              | Value                                                             |
| --------------------------------- | ----------------------------------------------------------------- |
| Caller repository                 | `windlasstech/vers-js`                                            |
| Caller workflow                   | `.github/workflows/publish.yml`                                   |
| Reusable workflow pin             | `496e40e5ea983cd6933de0dec6b2658c6ccb4db6`                        |
| Package                           | `@windlass/vers-js@0.1.2`                                         |
| Built source ref                  | `refs/tags/v0.1.2`                                                |
| Annotated tag object              | `518569c61b8f52ddf22b3fa98f2a145a03048f68`                        |
| Peeled source revision            | `68e6f0018d6824167ec490243b17a9edd6ba8fc7`                        |
| Invocation ref                    | `refs/heads/main`                                                 |
| Windlass predicate type           | `https://slsa.dev/provenance/v1`                                  |
| npm publish-attestation predicate | `https://github.com/npm/attestation/tree/main/specs/publish/v0.1` |

The tag's `package.json` declares version `0.1.2`. Its settings-only `pnpm-workspace.yaml` has no
`packages` member, exercising issue #22 root-package resolution. The npm Trusted Publisher binding
to the caller workflow is already confirmed working.

## Safety rules

- Run the phases in order. Record ISO 8601 UTC timestamps in the evidence record.
- Do not dispatch, publish, or retry after an `indeterminate` result. Fail closed and preserve the
  evidence.
- Stop and open a follow-up ADR evaluating remediation or rollback if publish, read-back, or pacote
  verification exposes a signer defect. This is the ADR 0077 dogfood consequence.
- Never use a token, OTP, or credential fallback. The successful path is npm OIDC trusted publishing
  only.

## Phase 0: pre-run checks and dispatch

1. Confirm that the version is unpublished. The command must return an error, normally `E404`.

   ```bash
   npm view @windlass/vers-js@0.1.2 version
   ```

   Expected output: no version value and an npm error indicating the version is absent. Stop if the
   version exists.

2. Record the tag object and its peeled commit.

   ```bash
   gh api repos/windlasstech/vers-js/git/ref/tags/v0.1.2 --jq '.object.sha'
   gh api repos/windlasstech/vers-js/git/tags/518569c61b8f52ddf22b3fa98f2a145a03048f68 --jq '.object.sha'
   ```

   Expected output, in order:

   ```text
   518569c61b8f52ddf22b3fa98f2a145a03048f68
   68e6f0018d6824167ec490243b17a9edd6ba8fc7
   ```

3. Record the caller pin, operator, and start timestamp.

   ```bash
   gh api repos/windlasstech/vers-js/contents/.github/workflows/publish.yml?ref=main --jq '.sha'
   gh api repos/windlasstech/vers-js/commits/main --jq '.sha'
   date -u +%Y-%m-%dT%H:%M:%SZ
   ```

   Record the observed workflow-file blob SHA as supporting evidence, the fixed reusable-workflow
   pin `496e40e5ea983cd6933de0dec6b2658c6ccb4db6`, the operator identity, and the timestamp. The
   `main` commit is invocation context only, not the expected built-source revision.

4. Dispatch the retry from `main`.

   ```bash
   gh workflow run publish.yml -R windlasstech/vers-js --ref main -f release_tag=v0.1.2
   ```

   Expected output: GitHub accepts the dispatch. The caller supplies `source-ref: refs/tags/v0.1.2`
   for dispatch and the exact empty string for tag pushes.

## Phase 1: run observation and early OIDC preflight

1. Capture the run ID and invocation head SHA, then watch the run.

   ```bash
   gh run list -R windlasstech/vers-js --workflow publish.yml --limit 1 --json databaseId,conclusion,headSha,event
   gh run watch <RUN_ID> -R windlasstech/vers-js --exit-status
   ```

   Replace `<RUN_ID>` with the `databaseId` from the first command. Expected result: a
   `workflow_dispatch` run with `conclusion` `success`. Record the run ID, attempt, event, and
   `headSha`; the latter is the expected `source.invocation_revision`.

2. Inspect the publish-job log for the ADR 0076 early npm OIDC exchange preflight.

   ```bash
   gh run view <RUN_ID> -R windlasstech/vers-js --log
   ```

   Expected success: the preflight succeeds before the first `npm publish` mutation, with no token
   fallback. A `trusted-publisher-mismatch` diagnostic means npm rejected the configured caller
   workflow identity, normally as an HTTP `401` or `404`. A
   `windlass.verify.error.npm-oidc-exchange-indeterminate` diagnostic means the exchange surface was
   unreadable, malformed, or returned `5xx`; fail closed and do not blindly retry publication.
   Record the classification, relevant log excerpt, and whether a publish mutation was attempted.

## Phase 2: ADR 0073 propagation measurement

Immediately after the publish job succeeds, record the UTC publish-completion timestamp. Then poll
both the packument and the attestation endpoint once immediately and every 15 seconds until 15
minutes have elapsed from the first request. The attestation URL encodes `@` as `%40` and `/` as
`%2F`, using uppercase hex.

```bash
first_request_epoch=$(date -u +%s)
request_count=0
while :; do
  observed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  request_count=$((request_count + 1))
  packument=$(npm view @windlass/vers-js@0.1.2 version dist.integrity dist.shasum --json 2>&1)
  attestation=$(curl --silent --show-error --write-out '\nHTTP_STATUS:%{http_code}\n' \
    'https://registry.npmjs.org/-/npm/v1/attestations/%40windlass%2Fvers-js@0.1.2')
  printf 'observed_at=%s request=%s\npackument=%s\nattestation=%s\n' \
    "$observed_at" "$request_count" "$packument" "$attestation"
  now_epoch=$(date -u +%s)
  [ $((now_epoch - first_request_epoch)) -ge 900 ] && break
  sleep 15
done
```

Expected result: the packument confirms `0.1.2` and its `dist.integrity`, then the endpoint exposes
an element whose `predicateType` is `https://slsa.dev/provenance/v1`. A `404 {"error":"Not found"}`
does not say whether the version or its provenance is absent. Always pair it with the packument
check. Record every observation, the first authenticated Windlass-attestation visibility, total
convergence time, and request count.

| Observation timestamp | Request | Packument version/integrity | Attestation HTTP status | Windlass provenance visible | Notes      |
| --------------------- | ------: | --------------------------- | ----------------------: | --------------------------- | ---------- |
| `<ISO-8601>`          |   `<n>` | `<value or error>`          |              `<status>` | `<yes/no>`                  | `<detail>` |

One observation does not tighten this 15-minute polling bound. A normative change requires a new ADR
and specification update.

## Phase 3: attestation read-back and semantic bundle comparison

1. Fetch the final registry response and select provenance by predicate type, never array position.
   The registry's own publish attestation is not the Windlass provenance.

   ```bash
   curl --fail --silent --show-error \
     'https://registry.npmjs.org/-/npm/v1/attestations/%40windlass%2Fvers-js@0.1.2' \
     -o attestations.json
   jq '.attestations[] | select(.predicateType == "https://slsa.dev/provenance/v1")' attestations.json
   ```

   Expected output: one or more candidate objects with the Windlass predicate type. Preserve the
   entire response and identify each selected candidate.

2. Discover the preserved artifact name and download the run's bundle. Artifact names include the
   run ID and attempt.

   ```bash
   gh api repos/windlasstech/vers-js/actions/runs/<RUN_ID>/artifacts \
     --jq '.artifacts[] | {name, expired, archive_download_url}'
   gh run download <RUN_ID> -R windlasstech/vers-js \
     -n js-ts-npm-provenance-bundle-<RUN_ID>-<ATTEMPT>
   ```

   Expected output: a downloaded preserved provenance bundle. Record its filename and SHA-256.

3. Compare semantics, not the serialized bundle files. npm may re-serialize JSON on upload and
   read-back. The signed Statement bytes are the base64 DSSE `payload`; compare those bytes and the
   signature set. Full-file byte equality is diagnostic only.

   ```bash
   jq -r '.attestations[] | select(.predicateType == "https://slsa.dev/provenance/v1") | .bundle' \
     attestations.json > registry-bundle.json
   jq -r '.dsseEnvelope.payload' registry-bundle.json | base64 -D > registry-statement.json
   jq -r '.dsseEnvelope.payload' <PRESERVED_BUNDLE>.intoto.jsonl | base64 -D > preserved-statement.json
   cmp --silent registry-statement.json preserved-statement.json
   jq -S '.dsseEnvelope.signatures | sort_by(.keyid, .sig)' registry-bundle.json > registry-signatures.json
   jq -S '.dsseEnvelope.signatures | sort_by(.keyid, .sig)' <PRESERVED_BUNDLE>.intoto.jsonl > preserved-signatures.json
   cmp --silent registry-signatures.json preserved-signatures.json
   cmp --silent registry-bundle.json <PRESERVED_BUNDLE>.intoto.jsonl
   ```

   Expected results: the first two comparisons succeed. The last command may succeed or fail; record
   `yes` or `no` for full-file byte equality without treating `no` as a failure.

## Phase 4: npm-native consumer verification

Use npm 11.12.0 or later for `--json --include-attestations`. npm 9.5.0 is the minimum version with
attestation verification, and npm 9.7.0 added `--provenance-file`.

```bash
workdir=$(mktemp -d)
cd "$workdir"
npm --version
npm init -y
npm install @windlass/vers-js@0.1.2
npm view @windlass/vers-js@0.1.2 version dist.integrity dist.shasum --json
npm audit signatures --json --include-attestations
```

Expected result: installation succeeds through the npm consumer path, then `npm audit signatures`
succeeds. On failure, preserve its JSON, including its `invalid` and `missing` values, and its exit
code `1`. Compare the registry `dist.integrity` with the run's recorded SHA-512 SRI build metadata;
they must match exactly.

npm verifies registry ECDSA packument signatures and provenance attestations. It checks Sigstore
bundle validity and that the attestation subject matches the package name and SHA-512 integrity. It
evidence of npm-path compatibility, but ADR 0068 identity binding still requires the Phase 5
reference procedure. `gh attestation verify` success is also necessary but not sufficient.

## Phase 5: reference-procedure identity verification

Use online verification only. Resolve online mode, reject duplicate JSON members before trusting
data, and acquire the current Sigstore public-good TUF trust root. Do not use an offline pinned-root
fallback if online TUF acquisition fails.

1. Download the exact published tarball and verify the bundle with every GitHub CLI constraint that
   it supports.

   ```bash
   npm pack @windlass/vers-js@0.1.2
   gh attestation verify windlass-vers-js-0.1.2.tgz \
     --bundle registry-bundle.json \
     --repo windlasstech/vers-js \
     --signer-workflow \
       windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@496e40e5ea983cd6933de0dec6b2658c6ccb4db6 \
     --cert-oidc-issuer https://token.actions.githubusercontent.com \
     --predicate-type https://slsa.dev/provenance/v1 \
     --format json > gh-verified.json
   ```

   Expected result: command success and JSON output for the exact tarball and supplied bundle.
   Record the tarball SHA-256 and SHA-512, the command output, and the exact signer-workflow ref.

2. Use a strict JSON parser and an X.509 parser to post-process the verified bundle and
   `gh-verified.json`. `jq` can inspect JSON values but cannot authoritatively decode certificate
   extensions. Record and verify all of the following:

   - Fulcio numeric repository and owner IDs match `windlasstech/vers-js`.
   - The certificate issuer is `https://token.actions.githubusercontent.com`.
   - URI SAN, Build Signer URI, and Build Signer Digest identify the full-SHA-pinned reusable
     workflow.
   - Runner Environment is exactly `github-hosted`.
   - Signed built source ref is `refs/tags/v0.1.2`.
   - Signed built source revision is `68e6f0018d6824167ec490243b17a9edd6ba8fc7`.
   - The certificate invocation claims match signed `source.invocation_ref` and
     `source.invocation_revision`.
   - `metadata.invocationId` and the Fulcio Run Invocation URI are byte-equal and contain
     `/actions/runs/<RUN_ID>`.

## Phase 6: issue acceptance and convergence

1. Decode the verified Statement payload and record these issue #81 fields:

   | Field                                           | Required value                             |
   | ----------------------------------------------- | ------------------------------------------ |
   | `externalParameters.source.ref`                 | `refs/tags/v0.1.2`                         |
   | `externalParameters.source.revision`            | `68e6f0018d6824167ec490243b17a9edd6ba8fc7` |
   | `externalParameters.source.input_ref`           | `refs/tags/v0.1.2`                         |
   | `externalParameters.source.invocation_ref`      | `refs/heads/main`                          |
   | `externalParameters.source.invocation_revision` | Phase 1 `headSha`                          |
   | `run_invocation` and Run Invocation URI         | identify the dispatch run                  |

   Also verify that `internalParameters` is exactly `{}`, the subject PURL is for
   `@windlass/vers-js@0.1.2`, both subject digests match downloaded tarball bytes, and `builder.id`
   contains the full reusable-workflow SHA.

2. Inspect the build-job log for issue #22 acceptance.

   ```bash
   gh run view <RUN_ID> -R windlasstech/vers-js --log
   ```

   Expected result: settings-only pnpm root-package resolution passes and no
   `package-resolution-invalid` diagnostic appears.

3. Re-run the same run's jobs to test same-run convergence. It must conclude successfully without a
   second publish.

   ```bash
   gh run rerun <RUN_ID> -R windlasstech/vers-js
   gh run watch <RUN_ID> -R windlasstech/vers-js --exit-status
   ```

   Record the new attempt, final conclusion, and evidence that the existing registry state was
   adopted only after integrity and authenticated attestation verification.

4. Dispatch a new run with the same release tag to test foreign conflict. This is intentionally a
   different run identity and must not adopt the existing publication.

   ```bash
   gh workflow run publish.yml -R windlasstech/vers-js --ref main -f release_tag=v0.1.2
   ```

   Expected result: failure with `windlass.verify.error.foreign-conflict`. Record the new run ID,
   diagnostic, and log excerpt. Do not keep retrying it.

## Evidence record template

Create `docs/dogfood/npm-m1.md` when the run starts. Fill every placeholder with observed evidence,
preserving command outputs and links where practical.

```markdown
---
run_id: <RUN_ID>
attempt: <ATTEMPT>
tag: refs/tags/v0.1.2
commit: 68e6f0018d6824167ec490243b17a9edd6ba8fc7
operator: <OPERATOR>
date: <ISO-8601 UTC timestamp>
---

# npm M1 dogfood evidence

## Run identity

- Caller: `windlasstech/vers-js/.github/workflows/publish.yml`
- Reusable workflow pin: `496e40e5ea983cd6933de0dec6b2658c6ccb4db6`
- Invocation ref and revision: `<refs/heads/main>`, `<headSha>`
- Tag object and peeled commit: `518569c61b8f52ddf22b3fa98f2a145a03048f68`,
  `68e6f0018d6824167ec490243b17a9edd6ba8fc7`

## P06/M1 acceptance checklist

- [ ] The real `@windlass/vers-js@0.1.2` version exists in npm.
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

- Run URL: `<URL>`
- Publish, build, and re-run log excerpts: `<links or quoted excerpts>`
- Registry response and selected bundle: `<paths or links>`
- Preserved artifact name and digest: `<name and SHA-256>`
- `gh attestation verify` JSON: `<path or excerpt>`
- X.509 post-processing result: `<path or excerpt>`
- npm consumer verification JSON: `<path or excerpt>`
- Foreign-conflict diagnostic: `<run URL and excerpt>`
```

## Failure handling

Treat a signer defect in the live publish, registry read-back, or pacote consumer path as a stop
condition. Preserve the evidence and open a follow-up ADR for remediation or rollback. Treat every
`indeterminate` classification as fail closed. Do not blindly retry publishing, because the registry
may already contain an ambiguous partial mutation.
