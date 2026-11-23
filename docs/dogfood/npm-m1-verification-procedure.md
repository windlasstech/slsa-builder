# npm M1 dogfood verification procedure

Use this runbook for the live P06 M1 dogfood of the fixed target `@windlass/vers-js@0.1.3`, from
pre-dispatch review through post-dispatch verification. It is a reusable procedure, not an attempt
record. Attempt outcomes, run IDs, timestamps, logs, and other run-specific evidence belong only in
[npm-m1.md](npm-m1.md). Do not retell attempt history or copy attempt outcomes into this runbook.
[npm-registry-provenance-requirements.md](npm-registry-provenance-requirements.md) is supporting
research, not a normative procedure.

The fixed historical target `@windlass/vers-js@0.1.2` is retained only for the new-run
foreign-conflict check. See [npm-m1.md](npm-m1.md) for the attempt history that makes it useful. The
caller workflow is `windlasstech/vers-js/.github/workflows/publish.yml`.

Phase 0 ends with a hard stop immediately before dispatch. An operator performing preparation-only
work must stop there. The dispatch command is isolated in Phase 1 so that no earlier step implies
that dispatch or publication occurred.

## Fixed target

| Item                              | Value                                                             |
| --------------------------------- | ----------------------------------------------------------------- |
| Caller repository                 | `windlasstech/vers-js`                                            |
| Caller workflow                   | `.github/workflows/publish.yml`                                   |
| Reusable workflow                 | `.github/workflows/js-ts-npm-package-slsa3.yml`                   |
| Reusable workflow pin             | One full 40-hex builder SHA selected and verified in Phase 0      |
| Package                           | `@windlass/vers-js@0.1.3`                                         |
| Package directory                 | `.`                                                               |
| Built source ref                  | `refs/tags/v0.1.3`                                                |
| Annotated tag object              | Recorded in Phase 0                                               |
| Peeled built-source commit        | Recorded in Phase 0                                               |
| Invocation ref                    | `refs/heads/main`                                                 |
| Invocation commit                 | The dispatched run's `headSha`, not the peeled tag commit         |
| Publish inputs                    | Registry omitted, `dist-tag: latest`, `access: public`            |
| Distribution inputs               | `release-asset-mode: false`, `linked-artifact-metadata: false`    |
| Windlass predicate type           | `https://slsa.dev/provenance/v1`                                  |
| npm publish-attestation predicate | `https://github.com/npm/attestation/tree/main/specs/publish/v0.1` |
| Foreign-conflict target           | Existing `@windlass/vers-js@0.1.2`, used only in the final check  |

The tag's `package.json` declares version `0.1.3`. Its settings-only `pnpm-workspace.yaml` has no
`packages` member, which exercises issue #22 root-package resolution. The npm Trusted Publisher
binding names the caller repository and caller workflow filename, not the reusable builder workflow.
That configuration is a prerequisite, not evidence that a future publish will succeed.

## Normative references

Read the selected builder revision of these contracts before dispatch. Links here select the current
document for orientation; Phase 0 requires inspection at the selected SHA.

- [JS/TS npm package profile](../architecture/js-ts-npm-package-profile.md)
- [JS/TS npm build and pack](../architecture/js-ts-npm-build-pack.md)
- [JS/TS npm provenance and publish](../architecture/js-ts-npm-provenance-publish.md)
- [Common SLSA provenance v1](../architecture/slsa-provenance-v1.md)
- [Complete verification procedure](../architecture/verification-policy-and-fixtures.md#complete-verification-procedure)
- [ADR 0101](../decisions/0101-relay-external-cli-stderr-to-logs-under-hygiene-bounds-with-classification-as-enrichment.md)
- [ADR 0102](../decisions/0102-classify-npm-e422-publish-rejection-as-determinate-under-reduced-readback-budget.md)
- [ADR 0109](../decisions/0109-minimize-external-parameters-to-actual-interface-inputs.md)
- [ADR 0113](../decisions/0113-define-the-common-builderdependencies-descriptor-shape.md)
- [ADR 0114](../decisions/0114-record-the-publish-stage-npm-cli-in-builderdependencies.md)

## Safety rules

- Run phases in order. Record preparation timestamps in preparation notes and run timestamps in the
  attempt evidence log after dispatch, never in this runbook.
- Do not dispatch, publish, rerun, or retry during preparation. Do not blindly retry after an
  `indeterminate` result. Fail closed and preserve the evidence.
- Never use an npm token, OTP, credential fallback, or npm automatic-provenance fallback. The
  successful path is npm OIDC trusted publishing with the exact Windlass-signed bundle only.
- Stop and request an operator decision on a follow-up ADR for remediation or rollback if publish,
  read-back, or pacote consumer verification exposes a signer defect. Do not repair implementation,
  ADRs, specifications, fixtures, or caller state as an undocumented side effect of this procedure.
- Passing local or static checks is necessary preparation, not proof of live registry acceptance.
  Only a conforming live run can establish npm's acceptance of the custom `buildType` and
  `builder.id`.
- The optional real-toolchain precheck uses the network. It is never a silent prerequisite and never
  publishes or exchanges an OIDC token.

## Phase 0: pre-dispatch review and static gates

Dispatch eligibility and diagnostic work are different outcomes. A failed dispatch gate blocks
Phase 1. After such a failure, later non-publishing static checks MAY run only to collect
blocked-preparation evidence; green results cannot clear the blocker or make preparation complete.
Do not convert an inconclusive check, a later green check, or an acceptance limitation into
permission to publish.

Keep preparation results in preparation notes under the neutral scratch directory established below.
Results without an actual run ID and dispatch timestamp do not belong in `npm-m1.md`. A blocked
preparation that never dispatches creates no attempt entry. After a real dispatch, the new attempt
entry may reference or incorporate those preparation notes.

Treat the builder dependency review in Phase 0.6 and the caller Sigstore-bundle parser review in
Phase 0.4 as independent dispatch gates. Keep both distinct from the full-verification readiness
limitation in Phase 0.7, which is not an implementation defect by itself. Record observed blockers
in preparation notes rather than turning them into runbook-specific revisions.

### 0.1 Select the builder SHA and establish caller state

1. Select the full 40-hex `windlasstech/slsa-builder` commit that carries the intended ADR
   0082/0084/0085/0098 and ADR 0099 through ADR 0114 behavior. Establish an absolute builder
   checkout path and a neutral preparation directory outside every package root. Keep all generated
   review outputs and preparation notes outside the builder and caller repositories.

   ```bash
   BUILDER_CHECKOUT='<ABSOLUTE_PATH_TO_SELECTED_BUILDER_CHECKOUT>'
   BUILDER_SHA='<SELECTED_40_HEX_BUILDER_SHA>'
   PREPARATION_DIR=$(mktemp -d)
   PREPARATION_NOTES="$PREPARATION_DIR/preparation-notes.md"
   : > "$PREPARATION_NOTES"
   cd "$PREPARATION_DIR"

   [[ "$BUILDER_SHA" =~ ^[0-9a-f]{40}$ ]] || {
     printf 'BUILDER_SHA must be a full lowercase 40-hex commit\n' >&2
     exit 1
   }
   git -C "$BUILDER_CHECKOUT" rev-parse --show-toplevel
   git -C "$BUILDER_CHECKOUT" rev-parse HEAD
   git -C "$BUILDER_CHECKOUT" status --short
   git -C "$BUILDER_CHECKOUT" fetch origin main
   git -C "$BUILDER_CHECKOUT" merge-base --is-ancestor "$BUILDER_SHA" origin/main
   gh api repos/windlasstech/slsa-builder/commits/main --jq '.sha'
   ```

   Prove that the selected SHA is contained in `origin/main`; `git -C ... merge-base --is-ancestor`
   must exit `0`. Record the remote builder `main` SHA as supporting evidence. A movable branch or
   tag is not an acceptable builder pin.

   Run local static gates only when `git -C "$BUILDER_CHECKOUT" rev-parse HEAD` prints the selected
   SHA. The content under test must otherwise equal that revision. Use a separate checkout if
   necessary. Record any local documentation-only difference instead of silently treating an edited
   implementation as the selected revision. `mktemp` creates the neutral directory outside any
   package root, preventing the builder checkout's own `devEngines.packageManager` policy from
   affecting registry npm commands. Do not weaken that policy or set an npm configuration override.

2. Record the caller's current `main` commit and workflow-file blob SHA, and preserve the exact
   workflow bytes used for review.

   ```bash
   CALLER_MAIN_SHA=$(gh api repos/windlasstech/vers-js/commits/main --jq '.sha')
   CALLER_BLOB_SHA=$(gh api \
     "repos/windlasstech/vers-js/contents/.github/workflows/publish.yml?ref=${CALLER_MAIN_SHA}" \
     --jq '.sha')
   gh api \
     "repos/windlasstech/vers-js/contents/.github/workflows/publish.yml?ref=${CALLER_MAIN_SHA}" \
     --jq '.content' | base64 -d > caller-publish.yml
   printf 'caller_main=%s\ncaller_workflow_blob=%s\n' \
     "$CALLER_MAIN_SHA" "$CALLER_BLOB_SHA"
   date -u +%Y-%m-%dT%H:%M:%SZ
   ```

   Record the operator identity and UTC timestamp with these values. The caller `main` commit is
   invocation context only. It is not the expected built-source commit.

3. Inspect recent caller runs and require no active duplicate before proceeding.

   ```bash
   gh run list -R windlasstech/vers-js --workflow publish.yml --limit 20 \
     --json databaseId,attempt,conclusion,createdAt,event,headBranch,headSha,status \
     > recent-caller-runs.json
   jq '.[] | {databaseId,attempt,event,status,conclusion,headBranch,headSha,createdAt}' \
     recent-caller-runs.json
   jq -e '[.[] | select(.status != "completed")] | length == 0' recent-caller-runs.json
   ```

   The final assertion must return `true`. If any run is active, stop rather than dispatching a
   contender for the same release intent. Review the recent completed runs for relevant context, but
   record their outcomes only in the evidence log when they are directly relevant to the new run.

### 0.2 Prove the target version absent with paired registry reads

The package identity must already exist. First publication of a package identity is outside the M1
profile. Run every registry npm command from the neutral preparation directory, never from the
builder checkout or another package root.

```bash
cd "$PREPARATION_DIR"
npm view @windlass/vers-js version

packument_exit=0
npm view @windlass/vers-js@0.1.3 version dist.integrity dist.shasum --json \
  > pre-dispatch-packument.json 2> pre-dispatch-packument.stderr || packument_exit=$?
printf 'packument_exit=%s\n' "$packument_exit"
printf '%s\n' '--- packument stderr ---'
cat pre-dispatch-packument.stderr

attestation_exit=0
attestation_http=$(curl --silent --show-error \
  --output pre-dispatch-attestation.json --write-out '%{http_code}' \
  'https://registry.npmjs.org/-/npm/v1/attestations/%40windlass%2Fvers-js@0.1.3') \
  || attestation_exit=$?
printf 'attestation_exit=%s attestation_http=%s\n' "$attestation_exit" "$attestation_http"
cat pre-dispatch-attestation.json
```

Require all of the following before continuing:

- The unversioned package-metadata read succeeds, proving the package identity exists.
- The exact-version packument read fails with npm `E404` and no version value. Any version value,
  different exit class, malformed response, or unreadable packument surface stops the procedure.
- The attestation request itself succeeds at the transport level and returns HTTP `404` with the
  expected `{"error":"Not found"}` body. Validate that body with
  `jq -e '.error == "Not found"' pre-dispatch-attestation.json`.
- Both absence observations are present. An attestation `404` alone proves neither version absence
  nor attestation absence. Any `2xx`, `5xx`, transport error, unexpected body, or unreadable surface
  is fail closed, not an inferred absence.

Preserve both responses and their timestamps as pre-dispatch evidence.

### 0.3 Verify the annotated tag and release binding

1. Resolve the annotated tag object and peel it to its terminal commit. Require GitHub's tag
   signature verification to report `verified: true`.

   ```bash
   gh api repos/windlasstech/vers-js/git/ref/tags/v0.1.3 > tag-ref.json
   jq -e '.object.type == "tag"' tag-ref.json
   TAG_OBJECT_SHA=$(jq -r '.object.sha' tag-ref.json)
   gh api "repos/windlasstech/vers-js/git/tags/${TAG_OBJECT_SHA}" > annotated-tag.json
   jq '{sha, tag, object, verification}' annotated-tag.json
   jq -e '.object.type == "commit" and .verification.verified == true' annotated-tag.json
   PEELED_SOURCE_REVISION=$(jq -r '.object.sha' annotated-tag.json)
   [[ "$PEELED_SOURCE_REVISION" =~ ^[0-9a-f]{40}$ ]]
   printf 'tag_object=%s\npeeled_commit=%s\n' \
     "$TAG_OBJECT_SHA" "$PEELED_SOURCE_REVISION"
   ```

   Record the annotated tag object, peeled commit, signature-verification result, and verification
   reason. A lightweight tag, missing signature, `verified: false`, missing verification evidence,
   non-commit target, or unreadable API surface stops the procedure. Do not confuse the tag object
   with the commit it annotates.

2. Preserve the release files at the peeled commit, not at caller `main`.

   ```bash
   for file in package.json pnpm-workspace.yaml pnpm-lock.yaml CHANGELOG.md; do
     gh api "repos/windlasstech/vers-js/contents/${file}?ref=${PEELED_SOURCE_REVISION}" \
       --jq '.content' | base64 -d > "tag-${file}"
   done
   jq '{name,version,private,repository,packageManager,devEngines,publishConfig}' \
     tag-package.json
   jq -e '.name == "@windlass/vers-js" and .version == "0.1.3" and .private != true' \
     tag-package.json
   cat tag-pnpm-workspace.yaml
   grep -nE '^## \[0\.1\.3\] - 12[0-9]{3}-[0-9]{2}-[0-9]{2}[[:space:]]*$' \
     tag-CHANGELOG.md
   ```

   Review and record all of the following:

   - `repository` normalizes to `https://github.com/windlasstech/vers-js` under the
     [repository identity rules](../architecture/js-ts-npm-build-pack.md#repository-identity-validation).
   - `packageManager`, or any higher-priority `devEngines.packageManager` selection, follows the
     selected SHA's exact package-manager and optional-digest rules and agrees with
     `pnpm-lock.yaml`. A shortened, unsupported, or conflicting selection stops the procedure.
   - `publishConfig` does not disable provenance or set `directory`. Its `registry`, `tag`, and
     `access` values, when present, agree with the raw caller inputs: omitted registry, `latest`,
     and `public`. Any conflict stops the procedure.
   - `pnpm-workspace.yaml` is a YAML object with no `packages` member. Its other settings are not
     workspace declarations. This is the settings-only root-package case, not a malformed file.
   - `CHANGELOG.md` contains the exact Human Era `0.1.3` heading expected by the caller's release
     guard and a non-empty notes section for that release.

### 0.4 Review the caller workflow and immutable pin

Inspect `caller-publish.yml` and record the checks below.

```bash
grep -nE 'release_tag|permissions:|contents:|id-token:|uses:|package-directory:|source-ref:|access:|dist-tag:|secrets:' \
  caller-publish.yml
grep -nF \
  "uses: windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@${BUILDER_SHA}" \
  caller-publish.yml
grep -nE 'dsseEnvelope|envelope\.payload|JSON\.parse|gh release create' caller-publish.yml
```

Require all of the following:

- The `uses:` reference is exactly the selected full 40-hex builder SHA, with no branch, tag, or
  shortened SHA.
- The trusted-publisher caller path is `.github/workflows/publish.yml`. The publish job grants only
  `contents: read` and `id-token: write` to the reusable workflow and inherits no broad secrets. The
  top-level permission reduction remains intact. Any separate caller-owned release job keeps
  `contents: write` off the reusable publish job.
- `package-directory` is `.`. Release-asset and linked-artifact options are absent or `false`.
- On `workflow_dispatch`, `release_tag` accepts only a release-tag shape and maps to
  `source-ref: refs/tags/<release_tag>`; on a tag push, `source-ref` is the exact empty string. The
  dispatch input cannot select an arbitrary ref, build command, runtime, registry credential, or
  second package. This procedure dispatches `v0.1.3` for the fixed target and `v0.1.2` only for the
  Phase 8 foreign-conflict check.
- The raw publish inputs are omitted registry, `dist-tag: latest`, and `access: public`.
- The caller workflow contains the release guard that binds the tag, package version, and changelog
  reviewed above.
- The caller-owned release-job Sigstore parser addresses the actual bundle field
  `dsseEnvelope.payload`, decodes those bytes as the in-toto Statement, and applies strict
  duplicate-rejecting JSON parsing to both the original Sigstore bundle and decoded Statement before
  trusting subject fields. A top-level `envelope.payload` lookup is not the Sigstore bundle path.

Stop if the selected pin differs or if any caller mapping is ambiguous.

Caller parser incompatibility is a hard dispatch blocker independent of builder or npm publish
checks. If the release job reads top-level `envelope.payload` instead of `dsseEnvelope.payload`,
record preparation as **blocked**, with the observed caller `main` and workflow blob in preparation
notes, not complete or eligible. Do not modify the caller as part of this runbook, and do not let
green npm publish or static checks clear the blocker.

Review the release job's unconditional `gh release create` independently of that parser defect.
Convergence of the reusable npm publish job does not prove convergence of the caller-owned release
job. Require a separate assessment of parser behavior, existing-release handling, and the command's
behavior on first publication and retry. This belongs to caller release acceptance, not a third
Windlass signer defect. Do not treat an unconditional create as evidence of idempotent caller
release behavior, and do not repair it through this procedure.

### 0.5 Inspect the selected builder workflow, specifications, and pin records

Read these files at `$BUILDER_SHA`, not from a movable branch.

```bash
cd "$PREPARATION_DIR"
git -C "$BUILDER_CHECKOUT" show "$BUILDER_SHA:.github/workflows/js-ts-npm-package-slsa3.yml" \
  > selected-js-ts-npm-package-slsa3.yml
git -C "$BUILDER_CHECKOUT" show "$BUILDER_SHA:internal/toolchainpins/node-runtime.json"
git -C "$BUILDER_CHECKOUT" show "$BUILDER_SHA:internal/toolchainpins/publish-npm.json"
git -C "$BUILDER_CHECKOUT" show "$BUILDER_SHA:docs/architecture/js-ts-npm-build-pack.md" \
  > selected-js-ts-npm-build-pack.md
git -C "$BUILDER_CHECKOUT" show "$BUILDER_SHA:docs/architecture/js-ts-npm-provenance-publish.md" \
  > selected-js-ts-npm-provenance-publish.md
git -C "$BUILDER_CHECKOUT" show "$BUILDER_SHA:docs/architecture/slsa-provenance-v1.md" \
  > selected-slsa-provenance-v1.md
grep -nE 'setup-node|RUNNER_TOOL_CACHE|runner.tool_cache|/opt/hostedtoolcache' \
  selected-js-ts-npm-package-slsa3.yml
```

Resolve the exact Node.js patch, bundled npm pair, runtime URL and SHA-256, plus the publish npm
version, tarball URL, and SHA-512 SRI from the selected SHA's `internal/toolchainpins/*.json`.
Record those resolved values as values to verify in the run. Runbook literals are not a timeless
substitute for the selected records.

Review the selected workflow against the
[digest-verified runtime contract](../architecture/js-ts-npm-build-pack.md#digest-verified-runtime-provisioning)
and
[publish npm provisioning contract](../architecture/js-ts-npm-provenance-publish.md#publish-stage-npm-provisioning):

- Every governed job that uses Node.js provisions the digest-verified runtime before any `node`,
  `npm`, `npx`, or Corepack invocation, including diagnostics. No setup action or runner toolcache
  supplies Node.js. The toolcache grep above must produce no matches.
- Build and provenance-sign jobs assert the pinned Node.js/bundled npm pair after provisioning and
  PATH binding, before subsequent package operations.
- The publish job asserts the provisioned Node.js runtime with `--node-only`, then independently
  provisions the selected SHA's digest-verified publish npm before every npm invocation. It never
  invokes the runtime-bundled npm. Its build-time npm record and publish-time npm record remain
  distinct.
- The three-job graph, artifact handoffs, permissions, and mutation concurrency match the selected
  specifications.

### 0.6 Review builder dependency consistency

The selected SHA's governed `runDetails.builder.builderDependencies` must contain exactly the
`signing-adapter` and `publish-cli` descriptors, selected by `name`. Their values must agree with
the actual signing module and publish npm pin in `go.mod`, `go.sum`, and
`internal/toolchainpins/publish-npm.json`, as required by ADR 0113 and ADR 0114. Agreement between a
fixture and a specification alone is insufficient.

Run the consistency review at the selected checkout and preserve the outputs.

```bash
cd "$PREPARATION_DIR"
SIGSTORE_PATH=github.com/sigstore/sigstore-go
SIGSTORE_VERSION=$(go -C "$BUILDER_CHECKOUT" list -m -f '{{.Version}}' "$SIGSTORE_PATH")
SIGSTORE_GO_LIST_H1=$(go -C "$BUILDER_CHECKOUT" list -m -f '{{.Sum}}' "$SIGSTORE_PATH")
SIGSTORE_GO_SUM_H1=$(awk -v path="$SIGSTORE_PATH" -v version="$SIGSTORE_VERSION" \
  '$1 == path && $2 == version {print $3}' "$BUILDER_CHECKOUT/go.sum")
printf 'sigstore_version=%s\ngo_list_h1=%s\ngo_sum_h1=%s\n' \
  "$SIGSTORE_VERSION" "$SIGSTORE_GO_LIST_H1" "$SIGSTORE_GO_SUM_H1"
test "$SIGSTORE_VERSION" = "v1.4.0"
test "$SIGSTORE_GO_LIST_H1" = "$SIGSTORE_GO_SUM_H1"
[[ "$SIGSTORE_GO_LIST_H1" =~ ^h1:[A-Za-z0-9+/]{43}=$ ]]
SIGSTORE_DIRHASH1=$(printf '%s' "${SIGSTORE_GO_LIST_H1#h1:}" \
  | base64 -d | xxd -p -c 256)
printf 'signing_adapter_dirHash1=%s\n' "$SIGSTORE_DIRHASH1"
test "$SIGSTORE_DIRHASH1" = \
  "0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac"

git -C "$BUILDER_CHECKOUT" show "$BUILDER_SHA:internal/toolchainpins/publish-npm.json" \
  > publish-npm.json
PUBLISH_CLI_VERSION=$(jq -r '.version' publish-npm.json)
PUBLISH_CLI_SRI=$(jq -r '.integrity' publish-npm.json)
test "$PUBLISH_CLI_VERSION" = "11.21.1"
[[ "$PUBLISH_CLI_SRI" =~ ^sha512-[A-Za-z0-9+/]{86}==$ ]]
PUBLISH_CLI_SHA512=$(printf '%s' "${PUBLISH_CLI_SRI#sha512-}" \
  | base64 -d | xxd -p -c 256)
printf 'publish_cli_version=%s\npublish_cli_sha512=%s\n' \
  "$PUBLISH_CLI_VERSION" "$PUBLISH_CLI_SHA512"
test "$PUBLISH_CLI_SHA512" = \
  "613ccbe90044257c87713bb426251ced128ca7e9b3949b65eeb18b98445a5d441e14720c73d6ad76be86f4cf42f2f0d66efe347a9a13400ccd2f6fcfb5954385"

grep -n 'github.com/sigstore/sigstore-go' \
  "$BUILDER_CHECKOUT/go.mod" "$BUILDER_CHECKOUT/go.sum"
git -C "$BUILDER_CHECKOUT" grep -n -A12 -B8 \
  -e 'signing-adapter' -e 'publish-cli' -e 'dirHash1' "$BUILDER_SHA" -- \
  internal/npmprofile internal/provenance \
  docs/architecture/js-ts-npm-provenance-publish.md \
  docs/architecture/slsa-provenance-v1.md \
  docs/architecture/verification-policy-and-fixtures.md
```

The `go list` `.Sum` and the exactly matched `go.sum` row above are the module-content `h1`. Never
use the separate `github.com/sigstore/sigstore-go v1.4.0/go.mod` checksum for this comparison.
Strictly base64-decode the module-content `h1` payload and lowercase-hex-encode the bytes to compare
with the signing adapter's `dirHash1`. Require URI
`pkg:golang/github.com/sigstore/sigstore-go@v1.4.0`, exactly the `name`, `uri`, and `digest`
members, and no signer `annotations` member.

Separately compare the `publish-cli` descriptor with the selected SHA's npm pin. Require name
`publish-cli`, URI `pkg:npm/npm@11.21.1`, exactly the pin-derived lowercase-hex `sha512` above, and
`mediaType: application/gzip`. Its `digest_authority: builder-pinned-record` annotation is
descriptive only and is excluded from trust verification. Do not compare it with the build-stage
bundled npm or place an `npm-cli` descriptor in `resolvedDependencies`.

Compare every governed producer, validator, and specification descriptor at the selected SHA with
these module and pin results. All of them must agree. Any name, URI, version, digest, media-type, or
closed-shape disagreement is a hard stop before dispatch. Do not codify a mismatch as expected, edit
the implementation or fixtures to make the review pass, or proceed on the basis of an earlier
reviewed revision.

A dependency disagreement found by this review is a hard dispatch blocker, independent of whether
tests are green. Preserve the actual `go list`, `go.sum`, decoded digest, and pin results and every
disagreeing descriptor in preparation notes. No static test, fixture, or later evidence-only check
can clear the disagreement.

### 0.7 Assess full-verification input readiness

The
[complete verification procedure](../architecture/verification-policy-and-fixtures.md#complete-verification-procedure)
requires more than a producer bundle and a certificate checklist. Before dispatch, name and preserve
the following inputs when available, recording each path, source, digest, and availability in
preparation notes:

- The explicit verifier policy.
- The verifier-facing Windlass release-manifest expectation.
- The authenticated Windlass release-manifest bundle, including its signer and predicate identity
  evidence. It must be authenticated before any of its fields can enter policy intersection.
- The strict duplicate-rejecting JSON/X.509 post-processor, including its invocation and
  typed-output contract. `jq` alone is not this post-processor.
- The destination for the final canonical diagnostic report, which is produced only after policy
  intersection and all required checks, not prefilled as an acceptance claim.

Use placeholders only to establish the paths and readiness record. Replace each with an actual input
path or `unavailable`; do not invent evidence to fill the list.

```bash
cd "$PREPARATION_DIR"
FULL_VERIFICATION_DIR="$PREPARATION_DIR/full-verification-inputs"
mkdir -p "$FULL_VERIFICATION_DIR"
VERIFIER_POLICY='<PATH_OR_UNAVAILABLE>'
RELEASE_MANIFEST_EXPECTATION='<PATH_OR_UNAVAILABLE>'
RELEASE_MANIFEST_BUNDLE='<PATH_OR_UNAVAILABLE>'
STRICT_POSTPROCESSOR='<PATH_OR_UNAVAILABLE>'
FINAL_DIAGNOSTIC_REPORT="$FULL_VERIFICATION_DIR/final-diagnostic-report.json"
printf 'policy=%s\nexpectation=%s\nrelease_manifest_bundle=%s\npostprocessor=%s\nreport=%s\n' \
  "$VERIFIER_POLICY" "$RELEASE_MANIFEST_EXPECTATION" "$RELEASE_MANIFEST_BUNDLE" \
  "$STRICT_POSTPROCESSOR" "$FINAL_DIAGNOSTIC_REPORT" \
  | tee "$FULL_VERIFICATION_DIR/input-readiness.txt"
```

For every available file input, copy it into `$FULL_VERIFICATION_DIR` and record its SHA-256 with
`shasum -a 256`. Preserve the original provenance of each input rather than relabeling an unrelated
file. The authenticated original `package.json` carried by `build-metadata.json.manifest_evidence`
is the npm package's source-manifest evidence. It is **not** the Windlass release manifest, its
expectation, or a release-manifest Sigstore bundle, and cannot substitute for any of them.

Missing readiness inputs are an acceptance limitation, not a third implementation defect. If any
required input is unavailable, the corresponding full consumer-side Windlass policy verification and
M1 acceptance remain unresolved. Record that limitation explicitly. Do not claim that the
certificate checklist is complete policy verification, silently substitute `gh attestation verify`,
`npm audit signatures`, or package-manifest evidence, or mark M1 accepted. After the two concrete
dispatch blockers are cleared by authorized changes, an operator must acknowledge this limitation
rather than treating it as permission to downgrade verification. Any diagnostic publish authorized
under that limitation is still not M1 acceptance and cannot be represented as full verification.

### 0.8 Run the established static gates

Run these commands after `cd "$BUILDER_CHECKOUT"`. Every dispatch-gating command must succeed before
dispatch. When a blocker has already been found, later commands are evidence-only and cannot clear
it or make preparation complete.

```bash
cd "$BUILDER_CHECKOUT"
go test ./...
go test -race ./...
golangci-lint run ./...
actionlint
go run ./cmd/slsa-builder-internal fixture-check --index testdata/fixtures/index.json
go run ./cmd/slsa-builder-internal workflow-check \
  --workflow .github/workflows/js-ts-npm-package-slsa3.yml --profile npm-only
go run ./cmd/slsa-builder-internal pin-check --publish-pin \
  internal/toolchainpins/publish-npm.json --pair-record \
  internal/toolchainpins/node-runtime.json --publish-spec \
  docs/architecture/js-ts-npm-provenance-publish.md --build-spec \
  docs/architecture/js-ts-npm-build-pack.md
pnpm exec prettier --check docs/dogfood/npm-m1-verification-procedure.md
pnpm exec markdownlint-cli2 docs/dogfood/npm-m1-verification-procedure.md
cd "$PREPARATION_DIR"
```

Whether these gates pass, fail, or abort the shell early, return to `$PREPARATION_DIR` before any
dispatch or post-dispatch npm operation. If the final `cd` was not reached, perform it explicitly
before continuing. Do not run registry npm commands from the builder checkout.

The multiline `pin-check` is the same check and argument sequence used by
`.github/workflows/lint.yml`. Run `actionlint` with no directory argument from the selected builder
checkout root; its workflow discovery is rooted there. `fixture-check` proves fixture governance,
not runtime behavior; the Go tests remain required.

Optionally, and only with explicit operator approval for network-using real tools, run the gated
`Live` build/pack tests without `-short`:

```bash
cd "$BUILDER_CHECKOUT"
SLSA_BUILDER_LIVE_TOOLCHAIN=1 go test ./internal/npmprofile -run 'Live' -count=1
cd "$PREPARATION_DIR"
```

This precheck may download and execute real npm/pnpm/Yarn toolchains. It performs no publish and is
not required silently. Record its selection and result separately from the hermetic gates.

### 0.9 Final freshness check and hard stop

Immediately before dispatch, from `$PREPARATION_DIR`:

1. Re-read caller `main` and the caller workflow blob. If either differs from the values recorded in
   Phase 0.1, stop and repeat the caller and pin review. If caller `main` changed, also repeat the
   recent-run review.
2. Repeat the paired packument and attestation absence checks in Phase 0.2 and the active-run
   assertion in Phase 0.1 step 3. The active-run assertion is unconditional: a competing dispatch
   can start from an unchanged caller `main`, so its result is required even when step 1 observed no
   caller change. Any visible target version, attestation, newly unreadable surface, or active
   caller run stops the procedure.
3. Confirm that `git -C "$BUILDER_CHECKOUT" rev-parse HEAD` still equals `$BUILDER_SHA`, the caller
   `uses:` still names that SHA, and every dispatch-gating check succeeded.
4. Record the final caller workflow blob SHA, caller `main` SHA for invocation only, operator, and
   UTC timestamp in `$PREPARATION_NOTES`, not in `npm-m1.md`. If either concrete blocker remains,
   record preparation as blocked and do not claim preparation completed. Evidence-only static
   results gathered afterward stay in the same preparation notes.

> [!IMPORTANT]
>
> **Hard stop before dispatch.** Preparation-only execution ends here. Do not run Phase 1 until an
> authorized operator explicitly accepts dispatch eligibility: every dispatch blocker must be
> cleared by authorized changes outside this runbook, every dispatch-gating check must have passed,
> and any full-verification acceptance limitation must be explicit. Later green checks are
> blocked-preparation evidence only and are not clearance. No successful static check is proof of
> registry acceptance.

## Phase 1: dispatch the fixed target

This is the first step that can lead to a registry mutation. Do not run it during preparation.

```bash
gh workflow run publish.yml -R windlasstech/vers-js --ref main -f release_tag=v0.1.3
```

GitHub accepting the dispatch is not publish success and not registry acceptance. Record the
dispatch timestamp only after the command actually succeeds. The caller maps this dispatch to
`source-ref: refs/tags/v0.1.3` while the invocation remains `refs/heads/main`.

## Phase 2: run observation and early log checks

1. Identify the new `workflow_dispatch` run by its creation time and invocation `headSha`, then
   watch it. Replace `<RUN_ID>` only with the run actually created by Phase 1.

   ```bash
   gh run list -R windlasstech/vers-js --workflow publish.yml --limit 5 \
     --json databaseId,attempt,conclusion,createdAt,event,headSha,status
   gh run watch <RUN_ID> -R windlasstech/vers-js --exit-status
   ```

   Record the run ID, attempt, event, invocation `headSha`, and conclusion in the evidence log. The
   `headSha` is the invocation commit, not the built tag commit.

2. Preserve the job log and canonical outcome report. The report artifact name includes the original
   run ID and attempt.

   ```bash
   gh run view <RUN_ID> -R windlasstech/vers-js --log > run-<RUN_ID>-<ATTEMPT>.log
   gh run download <RUN_ID> -R windlasstech/vers-js \
     -n js-ts-npm-outcome-report-<RUN_ID>-<ATTEMPT>
   jq '.' windlass-npm-outcome-report.json
   ```

   Under ADR 0101, the log preserves npm's rendered stderr through the hygiene filter. Preserve that
   sanitized operational evidence and any `windlass operational:` summary. Rejection-family
   recognition enriches the log only. It never determines classification, retry, mutation, report
   contents, or the primary diagnostic. The canonical outcome report remains free of external text.
   A missing outcome report is a conformance failure and cannot be presented as success.

3. Check the early OIDC preflight and toolchain ordering in the log:

   - Digest-verified Node.js provisioning precedes every Node.js-family invocation in each governed
     job. Build and signing assert the selected pair. Publish performs the node-only assertion and
     then independently provisions the digest-verified publish npm before any npm invocation.
   - The early, non-mutating npm OIDC exchange preflight succeeds before the first `npm publish`.
     Its token is discarded and never appears in logs or evidence. npm performs its own publish-time
     trusted-publishing exchange; no token, OTP, or credential fallback is used.
   - Missing OIDC capability fails before mutation with
     `windlass.verify.error.oidc-capability-unavailable`. An exchange `401` or `404` means
     `windlass.verify.error.trusted-publisher-mismatch`. An unreadable, malformed, or `5xx` exchange
     surface means `windlass.verify.error.npm-oidc-exchange-indeterminate`; fail closed and do not
     blindly retry.
   - Record the actual outcome state and primary diagnostic from the canonical report, the relevant
     sanitized log excerpt, and whether a publish mutation was attempted. Do not infer these from a
     rejection-family label.

## Phase 3: publication outcome and registry propagation

### 3.1 Successful-publish external propagation measurement

Run this measurement only after the publish mutation succeeded. It does not replace the workflow's
internal classification and does not apply to an `E422` or any other failed publish call.

Record the UTC publish-completion timestamp. Then read the packument and attestation endpoint
together once immediately and every 15 seconds until 300 seconds have elapsed from the first
request. Stop early only when both the exact version and a Windlass SLSA v1 candidate are visible.
The attestation URL uses uppercase percent-encoding for `@` and `/`.

```bash
cd "$PREPARATION_DIR"
first_request_epoch=$(date -u +%s)
request_count=0
while :; do
  observed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  request_count=$((request_count + 1))
  packument=$(npm view @windlass/vers-js@0.1.3 version dist.integrity dist.shasum --json 2>&1) \
    || true
  attestation=$(curl --silent --show-error --write-out '\nHTTP_STATUS:%{http_code}\n' \
    'https://registry.npmjs.org/-/npm/v1/attestations/%40windlass%2Fvers-js@0.1.3')
  attestation_body=${attestation%%$'\nHTTP_STATUS:'*}
  printf 'observed_at=%s request=%s\npackument=%s\nattestation=%s\n' \
    "$observed_at" "$request_count" "$packument" "$attestation" \
    | tee -a propagation-observations.log

  if printf '%s' "$packument" \
      | jq -e '.version == "0.1.3" and (.["dist.integrity"] | type == "string")' \
        > /dev/null 2>&1 \
    && printf '%s' "$attestation_body" \
      | jq -e '[.attestations[]? | select(.predicateType == "https://slsa.dev/provenance/v1")] | length > 0' \
        > /dev/null 2>&1; then
    break
  fi

  now_epoch=$(date -u +%s)
  [ $((now_epoch - first_request_epoch)) -ge 300 ] && break
  sleep 15
done
```

The multi-field `npm view ... --json` response uses the literal requested-field key
`"dist.integrity"`, so the jq check intentionally uses `.["dist.integrity"]`, not nested `.dist`.

Record every observation, the first candidate visibility, the first authenticated Windlass
attestation visibility established by later verification, convergence time, and request count. A
visible candidate is not yet accepted; later phases verify it. A `404` from the attestation endpoint
remains ambiguous without the paired packument result. One observation does not tighten the
five-minute propagation bound.

| Observation timestamp | Request | Packument version/integrity | Attestation HTTP status | Windlass candidate visible | Notes      |
| --------------------- | ------: | --------------------------- | ----------------------: | -------------------------: | ---------- |
| `<ISO-8601>`          |   `<n>` | `<value or error>`          |              `<status>` |                 `<yes/no>` | `<detail>` |

### 3.2 Failed-publish classification, including ADR 0102

Do not substitute the external measurement above for the workflow's internal read-back. Preserve the
canonical outcome and apply these rules without another publish attempt:

- A failed publish whose npm error code is exactly `E422` uses the internal reduced read-back
  budget: one immediate request, then one every 15 seconds through 60 seconds from the first
  read-back request. The observation issued at 60 seconds is inside the window.
- Determinate absence requires no clean packument observation of the target version and at least one
  clean packument observation of its absence. It ends as outcome state `absent` with
  `windlass.verify.error.publish-rejected-unprocessable`. An attestation error does not invalidate
  an otherwise clean packument absence. This grants no same-run retry.
- If the reduced window has no clean packument absence, including an all-error window,
  classification continues under the original five-minute deadline measured from the first read-back
  request. No new five-minute budget starts. A clean absence first seen after the reduced window is
  not the ADR 0102 determinate-absence shortcut.
- Any clean packument observation of the target version at any post-mutation poll, even after
  reduced expiry or paired with an attestation error, adds
  `windlass.verify.warning.publish-committed-after-rejection`. Custody continues under the original
  full-budget classification: `committed-as-expected`, `foreign-conflict`, or `indeterminate`, with
  the warning preserved alongside any final result. It does not turn a foreign or indeterminate
  result into success.
- The rejection-family recognizer never drives this classification. The trigger is the independently
  observed npm error code and registry evidence, not message enrichment.
- Every non-`E422` failure keeps its standing mapping, and every other ambiguous outcome keeps the
  full read-back budget. An `indeterminate` result still forbids blind retry because publication may
  already have committed.

## Phase 4: evidence preservation and semantic bundle comparison

Perform this phase after a successful publish and candidate visibility. Preserve artifacts before
running later convergence checks.

Define the strict duplicate-rejecting JSON helper once, at the top level so that its heredoc
terminator is valid shell. Step 3 of this phase and Phase 7 call it.

```bash
strict_json() {
  python3 - "$@" <<'PY'
import json
import sys

def reject_duplicates(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON member: {key!r}")
        result[key] = value
    return result

def reject_constant(value):
    raise ValueError(f"non-JSON numeric constant: {value}")

for path in sys.argv[1:]:
    with open(path, "rb") as source:
        raw = source.read(64 * 1024 * 1024 + 1)
    if len(raw) > 64 * 1024 * 1024:
        raise ValueError(f"JSON input exceeds 64 MiB bound: {path}")
    json.loads(raw, object_pairs_hook=reject_duplicates, parse_constant=reject_constant)
PY
}
```

1. Download the preserved bundle, package tarball, and build metadata. Record each filename and
   SHA-256. Capture the evidence directory as an absolute path so later consumer commands cannot
   accidentally read another working directory's files.

   ```bash
   cd "$PREPARATION_DIR"
   EVIDENCE_DIR="$PREPARATION_DIR"
   gh run download <RUN_ID> -R windlasstech/vers-js \
     -n js-ts-npm-provenance-bundle-<RUN_ID>-<ATTEMPT>
   gh run download <RUN_ID> -R windlasstech/vers-js \
     -n js-ts-npm-package-tarball-<RUN_ID>-<ATTEMPT>
   gh run download <RUN_ID> -R windlasstech/vers-js \
     -n js-ts-npm-build-metadata-<RUN_ID>-<ATTEMPT>
   TARBALL=windlass-vers-js-0.1.3.tgz
   PRESERVED_BUNDLE=windlass-vers-js-0.1.3.tgz.intoto.jsonl
   shasum -a 256 "$TARBALL" "$PRESERVED_BUNDLE" build-metadata.json
   shasum -a 512 "$TARBALL"
   ```

   Verify the build-metadata SHA-256 against the digest recorded by the run's handoff evidence, not
   merely against a filename.

2. Preserve and use `build-metadata.json` and its `manifest_evidence`. Complete verification
   requires the authenticated original selected-manifest bytes. They are in the digest-bound build
   metadata, not embedded as `content` in the signed `package-manifest` descriptor.

   ```bash
   jq '{schema_version,primary_artifact,manifest_evidence,build_script_disposition}' \
     build-metadata.json
   jq -r '.manifest_evidence.content_base64' build-metadata.json \
     | base64 -d > authenticated-original-package.json
   shasum -a 256 authenticated-original-package.json
   cmp --silent authenticated-original-package.json tag-package.json
   ```

   Require the evidence path to be `package.json`, the decoded bytes to match the Phase 0 tag
   manifest, and their SHA-256 to match the signed `package-manifest.digest.sha256` checked later.
   Also verify the descriptor's source repository and built commit against this evidence. Missing,
   mismatched, malformed, or network-substituted evidence is a verification failure, not permission
   to skip manifest-dependent checks.

3. Fetch the final registry response and select candidates by predicate type, never array position.
   The registry's publish attestation is not Windlass provenance. Before any jq inspection or
   normalization, strictly parse the original `attestations.json` and preserved Sigstore bundle with
   duplicate rejection. Strictly parse each decoded Statement separately as well. jq must not see a
   security-relevant JSON value before duplicate rejection has succeeded.

   ```bash
   curl --fail --silent --show-error \
     'https://registry.npmjs.org/-/npm/v1/attestations/%40windlass%2Fvers-js@0.1.3' \
     -o attestations.json
   strict_json attestations.json "$PRESERVED_BUNDLE"
   jq -r '.dsseEnvelope.payload' "$PRESERVED_BUNDLE" | base64 -d > expected-statement.json
   strict_json expected-statement.json
   jq '.attestations[] | select(.predicateType == "https://slsa.dev/provenance/v1")' \
     attestations.json

   match_count=0
   : > registry-payload-matches.jsonl
   while IFS= read -r candidate; do
     printf '%s\n' "$candidate" | jq -r '.dsseEnvelope.payload' | base64 -d \
       > candidate-statement.json
     strict_json candidate-statement.json
     if cmp --silent candidate-statement.json expected-statement.json; then
       match_count=$((match_count + 1))
       printf '%s\n' "$candidate" >> registry-payload-matches.jsonl
     fi
   done < <(jq -c '.attestations[]
     | select(.predicateType == "https://slsa.dev/provenance/v1") | .bundle' attestations.json)
   [ "$match_count" -eq 1 ]
   cp registry-payload-matches.jsonl registry-bundle.json
   ```

   Selection compares decoded DSSE payload bytes, not base64 spelling or array position. Exactly one
   candidate must match. This is a convenience for identifying the expected candidate, not
   acceptance by itself. Full signature, identity, subject, and Windlass policy verification remains
   mandatory. Delaying duplicate rejection until Phase 6 would be too late because jq normalization
   could discard the duplicate evidence first.

4. Compare semantics, not serialized bundle files. The signed Statement bytes are the base64 DSSE
   `payload`; compare those bytes and the signature set. Full-file byte equality is diagnostic only.

   ```bash
   jq -r '.dsseEnvelope.payload' registry-bundle.json | base64 -d > registry-statement.json
   jq -r '.dsseEnvelope.payload' "$PRESERVED_BUNDLE" | base64 -d > preserved-statement.json
   cmp --silent registry-statement.json preserved-statement.json
   jq -S '.dsseEnvelope.signatures | sort_by(.keyid, .sig)' registry-bundle.json \
     > registry-signatures.json
   jq -S '.dsseEnvelope.signatures | sort_by(.keyid, .sig)' "$PRESERVED_BUNDLE" \
     > preserved-signatures.json
   cmp --silent registry-signatures.json preserved-signatures.json
   cmp --silent registry-bundle.json "$PRESERVED_BUNDLE"
   ```

   The first two comparisons must succeed. Record `yes` or `no` for the last comparison without
   treating `no` as failure, because npm may reserialize the bundle.

## Phase 5: npm-native consumer verification

Use npm 11.12.0 or later for `--json --include-attestations`. This is a local consumer check, not a
publish operation.

```bash
workdir=$(mktemp -d)
cd "$workdir"
npm --version
npm init -y
npm install @windlass/vers-js@0.1.3
npm view @windlass/vers-js@0.1.3 version dist.integrity dist.shasum --json
npm audit signatures --json --include-attestations
```

Installation and `npm audit signatures` must succeed. On failure, preserve the command's JSON,
including `invalid` and `missing`, and its exit code. Recompute the preserved tarball digest, bind
it to the build metadata, and compare the registry's complete SHA-512 SRI with those same bytes:

```bash
tarball_sha512=$(shasum -a 512 "$EVIDENCE_DIR/$TARBALL" | cut -d' ' -f1)
metadata_sha512=$(jq -r '.primary_artifact.sha512' "$EVIDENCE_DIR/build-metadata.json")
[ "$tarball_sha512" = "$metadata_sha512" ]
expected_sri="sha512-$(printf '%s' "$tarball_sha512" | xxd -r -p | openssl base64 -A)"
observed_sri=$(npm view @windlass/vers-js@0.1.3 dist.integrity --json | jq -r .)
printf 'expected=%s\nobserved=%s\n' "$expected_sri" "$observed_sri"
[ "$expected_sri" = "$observed_sri" ]
```

npm checks registry ECDSA packument signatures and provenance attestations, bundle validity, and the
attestation subject's package name and SHA-512 integrity. These are necessary compatibility checks,
not complete Windlass policy verification. `gh attestation verify` in the next phase is also
necessary but insufficient on its own.

## Phase 6: online reference-procedure and identity verification

Use online verification only. Resolve online mode, reject duplicate JSON members before trusting
data, and acquire the current Sigstore public-good TUF trust root. Never fall back to an offline
pinned root when online TUF acquisition fails. Follow the
[complete verification procedure](../architecture/verification-policy-and-fixtures.md#complete-verification-procedure);
the commands below do not replace its strict post-processing.

1. Download the exact published tarball and verify the selected bundle with every GitHub CLI
   constraint it supports.

   ```bash
   npm pack @windlass/vers-js@0.1.3
   cmp --silent "$EVIDENCE_DIR/$TARBALL" windlass-vers-js-0.1.3.tgz
   gh attestation verify windlass-vers-js-0.1.3.tgz \
     --bundle "$EVIDENCE_DIR/registry-bundle.json" \
     --repo windlasstech/vers-js \
     --signer-workflow \
       "windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@${BUILDER_SHA}" \
     --cert-oidc-issuer https://token.actions.githubusercontent.com \
     --predicate-type https://slsa.dev/provenance/v1 \
     --format json > gh-verified.json
   ```

   Require command success and JSON bound to the exact tarball and supplied bundle. Record the
   tarball SHA-256 and SHA-512, output, and exact signer-workflow ref.

2. Use a strict duplicate-rejecting JSON parser and an X.509 parser to post-process the verified
   bundle and `gh-verified.json`. `jq` can inspect ordinary JSON values but cannot authoritatively
   decode certificate extensions. Require all of the following from the same verified bundle and
   certificate:

   - Issuer exactly `https://token.actions.githubusercontent.com`; Runner Environment exactly
     `github-hosted`.
   - URI SAN, Build Signer URI, and Build Signer Digest identify the full-SHA-pinned reusable
     workflow. Build Signer URI equals `runDetails.builder.id` byte-for-byte and Build Signer Digest
     equals its full SHA suffix.
   - Source Repository URI is `https://github.com/windlasstech/vers-js` and matches the normalized
     authenticated manifest repository plus both source descriptors' repository components. Numeric
     repository and owner IDs match the caller and signed `internalParameters.github` strings.
   - Build Trigger equals `internalParameters.github.event_name`, which is `workflow_dispatch` for
     this run. Build Config URI identifies caller `workflow.path`, `.github/workflows/publish.yml`,
     never the reusable builder path.
   - Source Repository Ref equals signed `workflow.ref` and the `invocation-source` URI ref, namely
     `refs/heads/main`. Source Repository Digest equals `invocation-source.digest.gitCommit` and the
     run's invocation `headSha`.
   - The effective built source ref is `refs/tags/v0.1.3`, and `built-source.digest.gitCommit` is
     the peeled tag commit from Phase 0. Do not compare that peeled commit with the certificate
     Source Repository Digest: the certificate digest binds the invocation source, not the built
     source.
   - `runDetails.metadata.invocationId` is byte-equal to the Fulcio Run Invocation URI and
     identifies `/actions/runs/<RUN_ID>/attempts/<ATTEMPT>`.
   - Bundle-contained SCT, Rekor inclusion proof, SET, and integrated-time evidence pass the online
     governed-root checks. No missing certificate claim is filled from logs, artifact names, or
     unsigned GitHub metadata.

3. Complete the full Windlass consumer-side policy procedure only when every Phase 0.7 input is
   available. The certificate checklist above is necessary but is not complete policy verification.

   - Strictly parse the explicit verifier policy, verifier-facing release-manifest expectation,
     release-manifest bundle, and producer bundle with duplicate rejection.
   - Authenticate the Windlass release-manifest bundle by repeating online bundle verification and
     strict JSON/X.509 post-processing with the release-manifest predicate and identity
     expectations. An unauthenticated bundle contributes no policy values.
   - Compute the field-by-field intersection of the authenticated release-manifest expectations and
     explicit verifier policy. Then perform the Statement, schema, descriptor, digest, and
     authenticated-original-manifest checks in Phase 7. The npm `manifest_evidence` is not the
     release manifest and cannot replace this input.
   - Invoke `$STRICT_POSTPROCESSOR` only through its documented interface, preserve its typed
     output, and serialize the final diagnostic report to `$FINAL_DIAGNOSTIC_REPORT`. Acceptance
     requires every required check to pass and no `error` diagnostic. Warnings do not convert
     failure to acceptance.

   If any required policy, expectation, release-manifest bundle, or strict post-processor input is
   unavailable, record full consumer-side Windlass policy verification and M1 acceptance as
   unresolved. Do not silently substitute the lesser certificate checklist or claim M1 acceptance.
   This is an acceptance limitation, separate from the two concrete implementation dispatch
   blockers, not a fabricated third defect.

## Phase 7: current Statement and issue acceptance checks

Decode the verified Statement and inspect it with strict JSON and the selected SHA's canonical
schemas.

```bash
jq -r '.dsseEnvelope.payload' "$EVIDENCE_DIR/registry-bundle.json" \
  | base64 -d > verified-statement.json
strict_json verified-statement.json
```

The summary below is a dogfood checklist, not a replacement for the
[externalParameters schema](../architecture/js-ts-npm-provenance-publish.md#jsts-npm-externalparameters-schema),
[resolvedDependencies schema](../architecture/js-ts-npm-provenance-publish.md#jsts-npm-resolveddependencies-schema),
[common provenance contract](../architecture/slsa-provenance-v1.md), or
[complete verification procedure](../architecture/verification-policy-and-fixtures.md#complete-verification-procedure).

### 7.1 Build definition and invocation identity

Require all of the following:

- `buildType` is exactly `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1`.
- `externalParameters.workflow` has exactly `ref`, `repository`, and `path`, with `refs/heads/main`,
  `https://github.com/windlasstech/vers-js`, and `.github/workflows/publish.yml`. These are
  invocation claims, not built-source or builder identity.
- Optional `source` is present only when the caller supplied a selector, and then contains exactly
  `ref`. For this explicit dispatch its value is `refs/tags/v0.1.3`; it is omitted entirely when no
  selector was supplied, never emitted as `{}`. The effective built ref is
  `source.ref ?? workflow.ref`; no old `source.repository`, `revision`, `input_ref`, `invocation_*`,
  `event_name`, or `release.*` shape is accepted.
- `package.directory` is `.`. The `publish` group has exactly the raw nullable inputs
  `input_registry_url: null`, `input_dist_tag: "latest"`, and `input_access: "public"`. The
  `distribution` group has exactly `release_asset_mode: false` and
  `linked_artifact_metadata: false`. No removed group or member is present.
- `internalParameters` is exactly `github`, whose only members are the strings `event_name`,
  `repository_id`, and `repository_owner_id`. It is not `{}` and contains no actor fields. The event
  is `workflow_dispatch`; numeric IDs agree with the certificate and trusted policy.
- The certificate Build Trigger, Build Config URI, and numeric-ID comparisons in Phase 6 pass.
  Invocation claims remain distinct from the built tag and peeled commit.

### 7.2 Resolved dependencies and media types

The general cardinality is six descriptors for an npm selection and seven for pnpm or Yarn. The
fixed target uses pnpm, so require exactly seven in this producer order: `invocation-source`,
`built-source`, `package-manifest`, `lockfile`, `node-runtime`, `package-manager-distribution`,
`runner-image`. `invocation-source` is at producer index 0 for npm compatibility, but consumers
select every descriptor by `name`, never position. Reordered valid sets are semantically acceptable
to consumers, while changing signed bytes is not.

- `invocation-source` identifies the caller at `refs/heads/main` and invocation `headSha`, with
  exactly `name`, `uri`, and `digest`, and no annotations. `built-source` is a distinct descriptor
  at the effective built ref `refs/tags/v0.1.3`, with the peeled tag commit and exactly
  `annotations.ref_type: "tag"`. Each digest contains only `gitCommit`. Both are required even when
  their values coincide; neither carries `mediaType`.
- `package-manifest` identifies `package.json` at the built commit, uses
  `mediaType: application/json`, carries the original manifest's SHA-256 and only the
  `workspace_root` annotation, and agrees with `build-metadata.json.manifest_evidence`.
- `lockfile` identifies `pnpm-lock.yaml` at the built commit, uses `application/yaml`, and carries
  exactly `package_manager`, `package_manager_root`, `selection_source`, `selection_manifest_path`,
  `selection_lockfile_path`, and `stale_non_selected_lockfiles`. Its package-manager root and
  selection evidence agree with the settings-only pnpm root-package build.
- `node-runtime` equals the selected SHA's runtime URL and compressed-tarball SHA-256, uses
  `application/gzip`, and has exactly `digest_authority: builder-pinned-record` and
  `platform: linux-x64`. It transitively covers the build-stage bundled npm; there is no `npm-cli`
  descriptor.
- Exactly one `package-manager-distribution` is present for this pnpm target; it would be forbidden
  for a direct npm selection. Its actual Corepack URL, selected version, SHA-512, and
  `application/gzip` media type agree with the build evidence. Annotations are exactly
  `digest_authority: registry-integrity`, `package_manager`, `package_manager_version`, and
  `acquisition_source: corepack`. It is not the separately provisioned publish npm.
- `runner-image` has exactly `name`, `uri`, and `annotations`, with no digest or media type. Its
  annotations are exactly `deployment_version`, `operating_system`, `image_version`, and
  `runner_label`. The Included Software URI and full image version pass offline projection;
  `operating_system` is the exact decoded on-disk detail, deployment divergence is allowed, and no
  `image_os` or `node_version` appears.
- No descriptor carries `content` or `downloadLocation`. Concrete descriptors follow the closed
  [deterministic media-type matrix](../architecture/slsa-provenance-v1.md#deterministic-media-types);
  unknown entries, duplicates, and generic media types are rejected.

### 7.3 Run details, subject, and signed outcomes

- `runDetails.builder.id` is exactly the canonical npm reusable-workflow URI at `$BUILDER_SHA`, and
  `buildType` is the authorized Windlass URI. Certificate Build Signer URI/Digest bind to it.
- `builder.version` is exactly `nodejs`, `npm`, and conditional `corepack`. Node.js and npm are the
  observed build-stage pinned pair; `npm` is not the independently provisioned publish npm. Corepack
  is present and exact for this pnpm build and would be absent for direct npm, with no runner-image
  key or other extra member.
- `builderDependencies` contains exactly two descriptors, selected by `name`: `signing-adapter` and
  `publish-cli`. The signing adapter has exactly `name`, `uri`, and `digest`, with no `annotations`.
  Its URI is `pkg:golang/github.com/sigstore/sigstore-go@v1.4.0`, and `digest.dirHash1` is exactly
  `0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac`, agreeing with the strict
  module-content `h1` conversion reviewed in Phase 0.6. The publish CLI has exactly `name`, `uri`,
  `digest`, `mediaType`, and `annotations`: URI `pkg:npm/npm@11.21.1`, only `digest.sha512` equal to
  `613ccbe90044257c87713bb426251ced128ca7e9b3949b65eeb18b98445a5d441e14720c73d6ad76be86f4cf42f2f0d66efe347a9a13400ccd2f6fcfb5954385`,
  and `mediaType: application/gzip`, all agreeing with the selected publish pin. Its only annotation
  is `digest_authority: builder-pinned-record`, which is descriptive only and excluded from trust
  verification. No build action or additional builder dependency is listed.
- `runDetails.metadata.windlass_build_script_disposition` is `executed` or `skipped-absent` in exact
  agreement with the authenticated original manifest. Build timestamps follow the canonical
  whole-second UTC contract, and `metadata.invocationId` equals the Fulcio Run Invocation URI.
- The single subject is exactly `pkg:npm/%40windlass/vers-js@0.1.3`, with both SHA-256 and SHA-512
  over the exact downloaded tarball bytes.

Inspect the build log for issue #22 acceptance: settings-only pnpm root-package resolution passes
and no `windlass.verify.error.package-resolution-invalid` diagnostic appears.

The eight registry comparisons observed through issue #30 and issue #143 are necessary evidence for
this checklist. They are not proof of npm's complete acceptance predicate. In particular, acceptance
of the custom Windlass `buildType` and `builder.id` remains unproven until a conforming run succeeds
end to end. Do not promote registry silence about any field into a general acceptance claim.

## Phase 8: convergence checks after preserving evidence

Do not run either mutation-capable command in this phase during preparation or before preserving the
first run's evidence.

1. Test reusable npm publish same-run convergence with the supported rerun command only after
   explicit operator approval. Do not add an unsupported `gh run rerun --all` flag.

   ```bash
   gh run rerun <RUN_ID> -R windlasstech/vers-js
   gh run watch <RUN_ID> -R windlasstech/vers-js --exit-status
   ```

   Record the new attempt, which jobs were actually rerun, the npm publish job's canonical outcome,
   and the overall run conclusion separately. The reusable npm publish result is the convergence
   subject: when that job is exercised, require no second publish and full integrity plus
   authenticated same-run attestation verification. If the supported rerun does not exercise it,
   mark npm same-run convergence as not exercised rather than claiming success. Rerunning has no
   broader adoption rights; inability to recompute the expected evidence is `indeterminate`, not
   success. Do not promise that the overall all-jobs run succeeds unconditionally.

   Evaluate the caller-owned release job independently. Its parser must use `dsseEnvelope.payload`
   and strict duplicate-rejecting JSON parsing, and its `gh release create` behavior must be
   evaluated for first-run creation and retry behavior against any existing release. Record
   creation, convergence, or conflict separately; do not overwrite or delete state to manufacture
   convergence. A known non-conforming parser means preparation remains blocked, not a future
   success awaiting confirmation. npm publish convergence never proves caller release-job
   convergence, and a caller release failure does not erase an independently valid npm publish
   result.

2. Dispatch a new run against the existing historical `@windlass/vers-js@0.1.2` publication to test
   foreign conflict. This is intentionally a different run identity and must not adopt the existing
   publication, even if bytes or metadata appear compatible.

   ```bash
   gh workflow run publish.yml -R windlasstech/vers-js --ref main -f release_tag=v0.1.2
   ```

   Expected result: outcome state `foreign-conflict`, with the current pre-mutation primary
   diagnostic `windlass.verify.error.already-published-version`. No second publish mutation is
   authorized. Record the new run ID, outcome state, primary diagnostic, canonical report, and log
   excerpt. The obsolete `windlass.verify.error.foreign-conflict` string is not the canonical
   diagnostic ID for this check. Do not retry the foreign-conflict run.

## Evidence record template

Preparation results without a run ID and dispatch timestamp belong in `$PREPARATION_NOTES`, not in
[npm-m1.md](npm-m1.md). A blocked preparation with no dispatch creates no attempt entry and must not
be described as completed preparation. After an actual dispatch, append a new attempt section to
[npm-m1.md](npm-m1.md), preserving all earlier attempt history and linking or incorporating the
preparation notes. Do not overwrite earlier attempts or store attempt outcomes in this runbook. Fill
placeholders only with observed evidence from the corresponding preparation and run.

```markdown
## Attempt <ATTEMPT_NUMBER> evidence

### Run and preparation identity

- Run ID and attempt: `<RUN_ID>`, `<ATTEMPT>`
- Run URL: `<URL>`
- Operator: `<OPERATOR>`
- Preparation notes path: `<PREPARATION_NOTES>`
- Preparation review and dispatch timestamps: `<ISO-8601 UTC>`, `<ISO-8601 UTC>`
- Dispatch eligibility: `<blocked/eligible, with reasons>`
- Selected-SHA builder dependency blocker: `<blocked/cleared, with evidence>`
- Caller Sigstore-bundle parser blocker: `<blocked/cleared, with evidence>`
- Caller release-job `gh release create` convergence review: `<result>`
- Full-verification readiness and acceptance limitations: `<result>`
- Caller: `windlasstech/vers-js/.github/workflows/publish.yml`
- Caller workflow blob SHA: `<CALLER_WORKFLOW_BLOB_SHA>`
- Caller main SHA, invocation only: `<CALLER_MAIN_SHA>`
- Reusable workflow pin: `<FULL_40_HEX_BUILDER_SHA>`
- Builder main SHA and selected-SHA containment result: `<BUILDER_MAIN_SHA>`,
  `<contained/not-contained>`
- Invocation ref and revision: `refs/heads/main`, `<headSha>`
- Built source ref and revision: `refs/tags/v0.1.3`, `<PEELED_SOURCE_REVISION>`
- Annotated tag object and signature verification: `<TAG_OBJECT_SHA>`, `<verified/reason>`
- Tag manifest repository/packageManager/publishConfig: `<observed values>`
- Settings-only pnpm workspace check and changelog binding: `<observed results>`
- Recent-run active-duplicate check: `<result>`
- Pre-dispatch packument and attestation absence: `<responses and timestamps>`
- Selected Node.js/npm pair and runtime artifact pin: `<values from selected SHA>`
- Selected publish npm and distribution pin: `<values from selected SHA>`
- Actual signing dependency from `go list` and `go.sum`: `<path, version, module-content h1>`
- Governed signing-adapter descriptor consistency: `<URI, dirHash1 conversion, shape, and result>`
- Governed publish-cli descriptor consistency:
  `<npm version, pin-derived sha512, application/gzip, shape, and result>`
- Static gates, including pin-check and runbook lint: `<command results and evidence-only status>`
- Optional network-using Live precheck, if explicitly selected: `<selected/not-selected and result>`
- Explicit verifier policy path and SHA-256: `<path, digest, or unavailable>`
- Verifier-facing release-manifest expectation path and SHA-256: `<path, digest, or unavailable>`
- Authenticated Windlass release-manifest bundle path, SHA-256, and verification result:
  `<path, digest, result, or unavailable>`
- Strict duplicate-rejecting JSON/X.509 post-processor and invocation: `<path/version/invocation>`
- Policy intersection result: `<result or unresolved>`
- Final canonical diagnostic report path and error summary: `<path and summary, or unresolved>`

### Early run and publication evidence

- Toolchain provisioning and pair/node-only assertions: `<log excerpts>`
- Early OIDC preflight and no-token behavior: `<result and log excerpt>`
- Publish mutation attempted: `<yes/no>`
- Canonical outcome state, primary diagnostic, and report path: `<values>`
- Sanitized ADR 0101 stderr and operational summaries: `<log paths or excerpts>`
- Publish completion timestamp, if successful: `<ISO-8601 UTC>`

### P06/M1 acceptance checklist

- [ ] The real `@windlass/vers-js@0.1.3` version exists in npm after a successful publish.
- [ ] Downloaded tarball bytes match recorded SHA-256 and SHA-512 values and registry SRI.
- [ ] The Statement subject is the expected npm PURL and carries both digests.
- [ ] Preserved and registry bundles have equal DSSE payload bytes and equal signature sets.
- [ ] Original attestation JSON and preserved bundle passed duplicate-rejecting parsing before jq,
      and every decoded Statement passed separately.
- [ ] Full bundle-file byte equality: `<yes/no, diagnostic only>`.
- [ ] `build-metadata.json` and its authenticated `manifest_evidence` are preserved and verified,
      without mistaking that package manifest for the Windlass release manifest.
- [ ] Sigstore online verification used the current public-good TUF root with no offline fallback.
- [ ] Strict JSON/X.509 post-processing passed issuer, signer, workflow, source, numeric-ID, runner,
      Build Trigger, Build Config URI, and Run Invocation URI checks; this was not treated as
      complete Windlass policy verification by itself.
- [ ] Explicit policy, release-manifest expectation, authenticated release-manifest bundle, policy
      intersection, and final diagnostic report completed full consumer-side verification, or the
      acceptance limitation was recorded as unresolved without substituting a lesser check.
- [ ] Invocation claims are distinguished from built tag and peeled commit; the peeled commit was
      not compared with certificate Source Repository Digest.
- [ ] Minimized externalParameters and the closed internalParameters.github triple pass.
- [ ] Seven name-keyed `resolvedDependencies` descriptors, producer order, media types, runner
      annotations, and absence of `npm-cli` there pass.
- [ ] `builder.version`, exactly the `signing-adapter` and `publish-cli` `builderDependencies`
      descriptors, signer `dirHash1` conversion, publish pin comparison, and signed build-script
      disposition pass. The signer has no annotations, and publish annotations are descriptive only.
- [ ] The early npm OIDC exchange preflight succeeded before mutation with no token fallback.
- [ ] Issue #22 settings-only pnpm root-package resolution passed with no
      `windlass.verify.error.package-resolution-invalid` diagnostic.
- [ ] `npm audit signatures` consumer verification passed; npm and gh checks were not treated as
      complete Windlass policy verification.
- [ ] Issue #143's eight registry comparisons were recorded as necessary evidence, not as a complete
      npm acceptance predicate.
- [ ] Reusable npm publish same-run convergence was exercised without a second publish or broader
      adoption rights, or marked not exercised; no unconditional all-jobs success was claimed.
- [ ] Caller-owned release-job bundle parsing and `gh release create` behavior were evaluated
      independently of npm publish convergence.
- [ ] New-run duplicate dispatch ended in outcome state `foreign-conflict` with primary diagnostic
      `windlass.verify.error.already-published-version`.

### Propagation observations after successful publish only

| Observation timestamp | Request | Packument version/integrity | Attestation HTTP status | Windlass candidate visible | Notes      |
| --------------------- | ------: | --------------------------- | ----------------------: | -------------------------: | ---------- |
| `<ISO-8601>`          |   `<n>` | `<value or error>`          |              `<status>` |                 `<yes/no>` | `<detail>` |

- Publish completion: `<ISO-8601 UTC>`
- First authenticated Windlass attestation visibility: `<ISO-8601 UTC>`
- Convergence duration: `<duration>`
- Requests: `<count>`

### Command outputs and preserved evidence

- Publish, build, signing, rerun, and foreign-conflict log excerpts: `<links or quoted excerpts>`
- Canonical outcome reports: `<paths or links>`
- Registry response and selected bundle: `<paths or links>`
- Preserved bundle name and SHA-256: `<name and digest>`
- Package tarball name, SHA-256, and SHA-512: `<name and digests>`
- Build metadata name and SHA-256: `<name and digest>`
- Authenticated original manifest bytes and descriptor comparison: `<path and result>`
- Builder dependency review outputs, decoded digests, and descriptor comparisons:
  `<paths or excerpts>`
- `gh attestation verify` JSON: `<path or excerpt>`
- Strict JSON and X.509 post-processing result: `<path or excerpt>`
- npm consumer verification JSON: `<path or excerpt>`
- Reusable npm publish same-run convergence evidence: `<run attempt, report, and conclusion>`
- Caller-owned release-job parser and `gh release create` evidence: `<logs and conclusion>`
- Foreign-conflict report and diagnostic: `<run URL, outcome state, primary ID, and excerpt>`
```

## Failure handling

Treat a signer defect in live publish, registry read-back, or pacote consumer verification as a stop
condition. Preserve the evidence and request an operator decision on a follow-up ADR for remediation
or rollback. Treat every `indeterminate` classification as fail closed. Never retry blindly, because
the registry may already contain an ambiguous partial mutation. Record attempt-specific failures and
outcomes only in [npm-m1.md](npm-m1.md).
