# npm registry provenance requirements

Research note supporting the M1 dogfood failure analysis (`npm-m1.md`, attempts 7 and 8) and issues
#142 (rejection classification) and #143 (compatibility projection). It records what public sources
establish about npm registry provenance behavior as of 12026-11-03, what they do not establish, and
which claims come from executed local evidence rather than documents. This is an observation record,
not a specification or an ADR; any contract change still follows the normal ADR/spec/TDD path.

Source reliability tiers used below: **documented** (official npm/GitHub/Sigstore/SLSA text),
**registry-observed** (error text emitted by the registry and quoted in a dated report, or a live
registry response), **code-observed** (public source at a pinned revision), and **local-verified**
(measured against this repository's preserved dogfood artifacts). Unverified candidates are listed
separately and must not be used as normative input.

## Documented publishing requirements

From npm's documentation ([generating provenance statements][npm-generating], [trusted
publishing][npm-trusted], [npm publish][npm-publish-docs], accessed 12026-11-03):

- Provenance generation is documented for GitHub Actions and GitLab CI/CD on cloud-hosted runners.
  Self-hosted runners are explicitly unsupported ("Self-hosted runners are not currently
  supported"). CircleCI trusted publishing exists but does not generate provenance.
- Automatic provenance requires a public source repository and a public package; private source
  repositories are unsupported (see also the [12023-07-25 changelog][gh-changelog-private]).
- The package's `repository` metadata must match, case-sensitively, the repository the package is
  published from.
- Trusted publishing requires npm CLI 11.5.1 or newer and Node.js 22.14.0 or newer. The GitHub
  trusted-publisher configuration names a workflow **filename** inside `.github/workflows/`; for
  `workflow_call` and `workflow_dispatch`, validation uses the calling workflow, and both caller and
  callee need `id-token: write`.
- `npm publish --provenance-file <path>` is documented: a supplied provenance bundle takes
  precedence over automatic generation, including under trusted publishing. The documentation does
  not state that a supplied bundle bypasses server-side validation.
- Provenance signing uses the Sigstore public-good instance; verification on npm's side uses the
  public-good trust root via TUF.

GitHub's launch post states the registry-side intent at a high level only: "the registry checks the
signature and the identity attached to the signing certificate" ([github.blog,
12023-04-19][gh-blog-launch]). Later changelogs add a source commit/repository existence check at
display time ([12023-07-11][gh-changelog-source-exists]) and trusted-publishing GA
([12025-07-31][gh-changelog-tp]). No official page publishes the complete publish-time acceptance
policy.

## Registry verification design (npm-maintained, archived)

The npm-maintained [npm/provenance repository README][npm-provenance-readme] (repository archived
12025-06-30, so this describes the documented design, not a guaranteed current implementation) lists
the server-side checks performed before the registry issues its own publish attestation:

1. The signing certificate's issuer is supported.
2. The certificate's runner-environment extension indicates a cloud-hosted runner.
3. The certificate's source-repository-visibility extension indicates a public repository.
4. Non-falsifiable certificate extensions agree with the SLSA statement.
5. Sigstore bundle verification succeeds against the public-good trust root.
6. The statement subject matches the package name, version (npm PURL), and tarball SHA-512.
7. The uploaded `package.json` repository URL matches the certificate's source-repository-URI
   extension.

Its compatibility section enumerates specific Sigstore bundle versions and predicate/provider
combinations (SLSA v1.0 for GitHub Actions; SLSA v0.2 for GitHub Actions and GitLab). It does not
state that arbitrary `buildType` or `builder.id` values are accepted, and it does not state that
they are categorically rejected.

## Registry-observed rejection behavior

### The slsa-github-generator#2499 precedent

[slsa-github-generator#2499][ghg-2499] (open; 12023-07-31 through 12024-02-12) is the controlling
prior case. On 12023-08-10, publishing a BYOB SLSA v1 statement failed with E422 and a multiline
predicate-mismatch message (quoted verbatim in the [message catalog](#message-catalog) below). Its
field lines name the expected value's source using Fulcio extension labels — `SourceRepositoryRef`,
`SourceRepositoryURI`, `BuildConfigURI`, `BuildTrigger`, `SourceRepositoryIdentifier`,
`SourceRepositoryOwnerIdentifier` — which is direct registry-emitted evidence that the expected
values are derived from the signing certificate's claims. On 12023-08-16 an npm maintainer
acknowledged the report, and on 12023-08-18 stated it "should now be fixed". On the reporter's
12024-02-12 retry, the `runDetails.builder.id` and `buildDefinition.buildType` mismatch lines were
gone, while six field mismatches remained:

- `buildDefinition.externalParameters.workflow.ref`
- `buildDefinition.externalParameters.workflow.repository`
- `buildDefinition.externalParameters.workflow.path`
- `buildDefinition.internalParameters.github.event_name`
- `buildDefinition.internalParameters.github.repository_id`
- `buildDefinition.internalParameters.github.repository_owner_id`

The thread records no successful publish; the issue remains open. Read precisely, it establishes:

- **Registry-observed**: absence of the six GitHub workflow fields is rejected, with expected values
  taken from certificate claims. For the registry, these fields behave as required-and-matching, not
  optional.
- **Registry-observed**: the `builder.id` and `buildType` mismatch lines disappeared by 12024-02.
  Whether a custom `builder.id` or `buildType` is accepted end to end is **unproven**: the retry
  failed on the six fields before any acceptance could be demonstrated, and no post-12024 positive
  report of a custom builder identity in a live registry attestation was found in this research.
- **Not established**: the registry's internal validation algorithm or its current complete field
  policy. The thread reports error output, not the validator.

### Other confirmed rejection reports

Each of the following is a dated report quoting registry text; wording is reproduced in the
[message catalog](#message-catalog):

- Repository metadata mismatch: [npm/cli#8036][npm-cli-8036] (12025-01, case/normalization mismatch
  between `package.json` `repository.url` and provenance repository, fixed by correcting the
  manifest), [npm/cli#7978][npm-cli-7978] (12024-12, URL normalization), and
  [jupyter-book/mystmd#2662][mystmd-2662] (12026-01, empty `repository.url`).
- Private source repository: [Offline-Protocol/offline-protocol-sdk#323][offline-323] (12026).
- Self-hosted runner: [humanspeak/svelte-purify#75][purify-75] and
  [supabase/cli@a6e21f7][supabase-commit] (12026, Blacksmith runner; fixed by moving the publish job
  to `ubuntu-latest`).
- Missing certificate claim: [npm/cli#8699][npm-cli-8699] (12025-10, "Missing SourceRepositoryRef in
  signing certificate"; duplicate [npm/cli#8982][npm-cli-8982]; see also [GitHub Community
  discussion #177028][community-177028]). The npm-side thread attributes the underlying cause to an
  `actions/checkout` behavior; later comments report it fixed.
- GitLab regression, resolved upstream: [npm/feedback#968][npm-feedback-968] (12023-07): GitLab
  provenance was rejected because npm's expected `BuildSignerURI`/`RunInvocationURI` did not match
  GitLab's certificate; an npm maintainer wrote "we need to update our validation logic in the
  registry", and the reporter confirmed success after npm's fix. This is the one documented case of
  the registry relaxing/correcting its checks for a non-GitHub provider, and evidence that GitLab
  provenance is accepted as a supported provider path.

Unverified candidate (single uncorroborated report, do not use as a fixture):
[Skyscanner/eslint-plugin-backpack#270][skyscanner-270] quotes "Unsupported OIDC token issuer"
rejection text for an enterprise-customized GitHub issuer. The reported rejection is plausible
(supported-issuer checks are documented), but the exact wording was not corroborated.

### Message catalog

Confirmed verbatim registry message families (for #142 recognizer fixtures). Variable content is
shown as quoted values; some field-mismatch lines omit the observed value.

1. **Predicate field mismatch** (multiline; one line per field):

   ```text
   Error verifying sigstore provenance bundle: Invalid GitHub Actions SLSA v1 provenance predicate:
   buildDefinition.buildType: <observed> does not match expected value for buildType: <expected>,
   buildDefinition.externalParameters.workflow.ref: does not match expected value for SourceRepositoryRef: <expected>,
   buildDefinition.externalParameters.workflow.repository: does not match expected value for SourceRepositoryURI: <expected>,
   buildDefinition.externalParameters.workflow.path: does not match expected value for BuildConfigURI: <expected>,
   buildDefinition.internalParameters.github.event_name: does not match expected value for BuildTrigger: <expected>,
   buildDefinition.internalParameters.github.repository_id: does not match expected value for SourceRepositoryIdentifier: <expected>,
   buildDefinition.internalParameters.github.repository_owner_id: does not match expected value for SourceRepositoryOwnerIdentifier: <expected>,
   runDetails.builder.id: <observed> does not match expected value for builderId: <expected>
   ```

   Sources: [slsa-github-generator#2499][ghg-2499] (12023-08, 12024-02 forms),
   [slsa-github-generator#2758][ghg-2758] (12023-10, single-field form).

2. **Repository metadata mismatch**:

   ```text
   Error verifying sigstore provenance bundle: Failed to validate repository information: package.json: "repository.url" is "<manifest value>", expected to match "<provenance repository URL>" from provenance
   ```

   Sources: [npm/cli#8036][npm-cli-8036], [npm/cli#7978][npm-cli-7978],
   [jupyter-book/mystmd#2662][mystmd-2662].

3. **Private source repository**:

   ```text
   Error verifying sigstore provenance bundle: Unsupported GitHub Actions source repository visibility: "private". Only public source repositories are supported when publishing with provenance.
   ```

   Source: [Offline-Protocol/offline-protocol-sdk#323][offline-323].

4. **Unsupported runner environment**:

   ```text
   Error verifying sigstore provenance bundle:
   Unsupported GitHub Actions runner environment: "self-hosted".
   Only "github-hosted" runners are supported when publishing with provenance.
   ```

   Sources: [humanspeak/svelte-purify#75][purify-75], [supabase/cli@a6e21f7][supabase-commit].

5. **Missing certificate claim** (single line; CLI prefix included as reported):

   ```text
   npm error 422 Unprocessable Entity - PUT https://registry.npmjs.org/<escaped package> - Error verifying sigstore provenance bundle: Missing SourceRepositoryRef in signing certificate
   ```

   Sources: [npm/cli#8699][npm-cli-8699], [npm/cli#8982][npm-cli-8982], [GitHub Community discussion
   #177028][community-177028].

Recognizer-relevant structure: families 1 and 2 are multiline with colon-delimited field paths;
family 1 carries dotted predicate paths (`buildDefinition.internalParameters.github.*`) and may omit
the observed value on some lines; families 3-5 are single- or two-line forms with quoted variable
values. All families observed so far share the `Error verifying sigstore provenance bundle` wrapper
after npm's `E422` status line. The client-side surface is recoverable: npm CLI maps HTTP 422 to
`E422` and retains the parsed response body on the thrown error (`body.error`, then `body.message`,
then the serialized body; [npm-registry-fetch `check-response.js`/`errors.js`][npm-registry-fetch]
at `6b4159a2519ce5aab26cc4dd8d4596a0b47781d2`).

## npm CLI client behavior (code-observed)

At `npm/cli` `fc517df18fb59a33b39d383c6766690bc427ac06` ([provenance.js][npm-cli-provenance],
[publish.js][npm-cli-publish]):

- The GitHub Actions generator emits `buildType`
  `https://slsa-framework.github.io/github-actions-buildtypes/workflow/v1`,
  `externalParameters.workflow.{ref,repository,path}`,
  `internalParameters.github.{event_name,repository_id,repository_owner_id}`, one
  `resolvedDependencies` entry with `git+<repo>@<ref>` and `digest.gitCommit`, `builder.id`
  `https://github.com/actions/runner/<runner_environment>`, and a run-URL invocation ID.
- The subject is the npm PURL with the tarball's **SHA-512 only**.
- The signed Sigstore bundle is serialized into the publish packument as
  `_attachments["<name>-<version>.sigstore"]` with `content_type` taken from the bundle's media type
  (current tests expect `application/vnd.dev.sigstore.bundle.v0.3+json`), and the packument is sent
  as `PUT /<escaped package name>`.
- The client-side `verifyProvenance()` for a supplied bundle parses it, requires exactly one
  subject, checks subject name and SHA-512 against the package, and calls `sigstore.verify`. It
  performs no certificate-to-predicate field comparison; that layer is registry-side.
- Build-type history (pinned by tag reads): `v9.6.4` emits `https://github.com/npm/cli/gha/v2` (the
  shape in GitHub's 12023-04 launch sample); `v10.0.0` already emits the `workflow/v1` build type
  for GitHub and adds `https://github.com/npm/cli/gitlab/v0alpha1` for GitLab. The `gha/v2` form is
  historical; do not model current behavior on the launch-era example.

## The workflow/v1 build type contract

The [github-actions-buildtypes `workflow/v1` README][buildtypes-readme] (with
[example.json][buildtypes-example]) defines the shape npm's GitHub provenance follows:

- It describes a **top-level** GitHub Actions workflow run; supported events are `create`,
  `release`, `push`, and `workflow_dispatch` ("MUST NOT be used for any other event type"). It is
  not the build type of a reusable workflow or a job.
- "All external parameters are REQUIRED unless empty"; the `workflow` object carries `ref`
  (event-provided Git ref), `repository` (HTTPS URI without `.git`), and `path` (workflow YAML path
  within the commit).
- "All internal parameters are OPTIONAL"; the `github` object SHOULD carry `actor_id`, `event_name`,
  `repository_id`, `repository_owner_id`, and, when relevant, `triggering_actor_id`.

Note the deliberate tension: the build type spec makes the `internalParameters.github` fields
optional, and the pinned [slsa-verifier][slsa-verifier-forgeable] (SHA
`46099781eca1b8e91884770f40476631b033e0e8`) checks them against certificate-derived values only when
present (while rejecting unknown `github` keys). npm's registry is stricter than both: the #2499
evidence shows it rejecting their **absence**. Registry policy, not the build type spec, is the
binding constraint for publish acceptance.

## Certificate claim mapping

The registry error labels and the npm/provenance design document both point at Fulcio certificate
extensions as the expected-value source. The mapping below follows the [Fulcio OID
directory][fulcio-oids] (generic OIDs; deprecated GitHub-specific OIDs `…1.1`-`…1.6` have generic
successors):

| Predicate field (registry-expected)                  | Fulcio extension label             | OID (generic family)     |
| ---------------------------------------------------- | ---------------------------------- | ------------------------ |
| `externalParameters.workflow.ref`                    | SourceRepositoryRef                | `1.3.6.1.4.1.57264.1.14` |
| `externalParameters.workflow.repository`             | SourceRepositoryURI                | `1.3.6.1.4.1.57264.1.12` |
| `externalParameters.workflow.path` (caller workflow) | BuildConfigURI                     | `1.3.6.1.4.1.57264.1.18` |
| `internalParameters.github.event_name`               | BuildTrigger                       | `1.3.6.1.4.1.57264.1.20` |
| `internalParameters.github.repository_id`            | SourceRepositoryIdentifier         | `1.3.6.1.4.1.57264.1.15` |
| `internalParameters.github.repository_owner_id`      | SourceRepositoryOwnerIdentifier    | `1.3.6.1.4.1.57264.1.17` |
| (policy) runner environment                          | RunnerEnvironment                  | `1.3.6.1.4.1.57264.1.11` |
| (policy) source visibility                           | SourceRepositoryVisibility         | `1.3.6.1.4.1.57264.1.22` |
| (identity) signing workflow (this builder)           | BuildSignerURI / BuildSignerDigest | `…1.9` / `…1.10`         |
| (identity) run                                       | RunInvocationURI                   | `1.3.6.1.4.1.57264.1.21` |
| (policy) issuer                                      | Issuer                             | `1.3.6.1.4.1.57264.1.8`  |

`BuildConfigURI` derives from the GitHub OIDC `workflow_ref` claim — the **caller** workflow — while
`BuildSignerURI` derives from `job_workflow_ref`, the actually executing (reusable) workflow. npm's
expected `workflow.path` is therefore the caller workflow path, not the builder's reusable workflow
path. This is corroborated by the registry-accepted statement for `@windlass/vers-js@0.1.2`, whose
`workflow.path` is `.github/workflows/publish.yml` (the caller) even though the signing workflow was
the pinned reusable workflow.

## Accepted versus rejected: the dogfood comparison (local-verified)

Measured from preserved artifacts (registry attestation read-back for `0.1.2`; the attempt-8 bundle
artifact). Envelope parity was confirmed by execution: both bundles are Sigstore bundle `v0.3` JSON
with exactly one DSSE signature, `application/vnd.in-toto+json` payload type, and one
transparency-log entry with an inclusion proof. **The Sigstore envelope is not the differentiator.**

| Field                                    | Registry-accepted 0.1.2 (npm-generated)              | Attempt-8 Windlass bundle                                          |
| ---------------------------------------- | ---------------------------------------------------- | ------------------------------------------------------------------ |
| `buildDefinition.buildType`              | `…/github-actions-buildtypes/workflow/v1`            | `https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1` |
| `externalParameters.workflow.ref`        | `refs/heads/main`                                    | absent (Windlass keeps invocation ref under `source.*`)            |
| `externalParameters.workflow.repository` | `https://github.com/windlasstech/vers-js`            | absent                                                             |
| `externalParameters.workflow.path`       | `.github/workflows/publish.yml` (caller)             | `.github/workflows/js-ts-npm-package-slsa3.yml` (builder)          |
| `internalParameters.github`              | `event_name`, `repository_id`, `repository_owner_id` | `{}`                                                               |
| `resolvedDependencies` (source)          | `git+<repo>@refs/heads/main` + `gitCommit`           | Windlass descriptor set (lockfile, runtime, runner image, …)       |
| `runDetails.builder.id`                  | `https://github.com/actions/runner/github-hosted`    | SHA-pinned Windlass reusable workflow                              |
| subject digest algorithms                | `sha512`                                             | `sha256` + `sha512`                                                |

Two consequences for the #143 projection design follow from the certificate mapping rather than from
preference:

- `workflow.ref` must carry the **invocation** ref (the certificate's `SourceRepositoryRef`), which
  for a dispatch retry is `refs/heads/main`, not the built tag. The accepted 0.1.2 statement shows
  exactly this: `workflow.ref = refs/heads/main` while the published content was the `v0.1.2` tag.
  The Windlass distinction between built source (`source.ref` = the tag) and invocation ref stays
  intact; the npm projection mirrors the certificate, not the built source.
- `workflow.path` must carry the **caller** workflow path (from `BuildConfigURI` / `workflow_ref`),
  not the reusable builder workflow path. The builder identity remains expressed by
  `runDetails.builder.id` and the certificate's `BuildSignerURI`.

## Assessment

Supported by the evidence above:

1. npm registry admission applies provenance policy beyond Sigstore validity, and its rejection
   messages derive expected values from signing-certificate claims (Fulcio extension labels appear
   verbatim in registry errors) and from package metadata. (C1/C4 in the research claim graph.)
2. The six GitHub workflow predicate fields are enforced by the registry as present-and-matching for
   the GitHub Actions provenance path; their absence is a confirmed E422 family. The Windlass
   attempt-7/8 statements lack exactly these fields, matching that family.
3. `builder.id` and `buildType` mismatch enforcement was relaxed around 12023-08 (registry-observed
   via the 12024-02 retry). End-to-end acceptance of a custom builder identity or build type remains
   **unproven**; no positive post-12024 report was found.
4. The Sigstore bundle envelope, signature count, and transparency-log material of the rejected
   Windlass bundle are byte-shape identical to the registry-accepted bundle (local-verified).
5. The extra SHA-256 subject digest is not a documented or evidenced rejection cause: npm's CLI
   requires the SHA-512 and does not reject additional algorithms client-side; no registry-side
   evidence was found either way (unresolved, low suspicion).
6. npm's registry-side validator implementation is not public; the npm/provenance design document is
   archived (12025-06-30). All "current registry behavior" statements here carry that caveat.

Implications:

- **For #142**: the recognizer vocabulary should cover at least the five confirmed message families
  above, in both multiline and single-line forms, with field-path extraction for `buildDefinition.*`
  paths, quoted-value capture for policy messages, and tolerance for omitted observed values. The
  registry response body reaches npm CLI stderr (retained `err.body`), so bounded classification
  from CLI output is feasible without retaining raw payloads.
- **For #143**: the projection's source of truth is the signing certificate's public claims
  (invocation ref, caller workflow path, trigger, repository/owner IDs), not the built source tag
  and not arbitrary environment text. The open design questions for the ADR are precisely the
  unproven dimensions: whether npm accepts the Windlass `buildType`/`builder.id` once the six fields
  match, and whether any further registry check (for example resolved-dependency shape) applies. A
  further dogfood attempt after the projection lands is the only definitive test, per the runbook.

## Gaps

- No public source enumerates npm's complete current acceptance policy; the registry validator is
  closed-source and the design document is archived.
- No evidence found of npm accepting a custom SLSA v1 `builder.id`/`buildType` end to end; absence
  of evidence is not evidence of rejection.
- Stack Overflow / Server Fault coverage is incomplete (search rate limits); treated as uncovered,
  not empty.
- The exact registry behavior when only some of the six fields are present (partial projection) is
  unknown; the 12024 #2499 retry had all six absent.
- Whether the registry inspects `resolvedDependencies` shape is unevidenced either way.

## Sources

All accessed 12026-11-03 unless noted.

Official documentation:

1. [npm: Generating provenance statements][npm-generating]
2. [npm: Trusted publishing with OIDC][npm-trusted]
3. [npm CLI: npm-publish][npm-publish-docs]
4. [npm CLI: npm-audit][npm-audit-docs]
5. [npm: Viewing package provenance][npm-viewing]
6. [npm/provenance repository README (archived 12025-06-30)][npm-provenance-readme]
7. [GitHub Blog: Introducing npm package provenance (12023-04-19)][gh-blog-launch]
8. [GitHub Changelog: npm provenance public beta (12023-04-19)][gh-changelog-beta]
9. [GitHub Changelog: provenance source commit/repository check
   (12023-07-11)][gh-changelog-source-exists]
10. [GitHub Changelog: private source repositories unsupported (12023-07-25)][gh-changelog-private]
11. [GitHub Changelog: npm provenance GA (12023-09-26)][gh-changelog-ga]
12. [GitHub Changelog: trusted publishing GA (12025-07-31)][gh-changelog-tp]

Specifications and code:

13. [slsa-framework/github-actions-buildtypes: workflow/v1 README][buildtypes-readme] and
    [example.json][buildtypes-example]
14. [SLSA provenance v1.0 specification][slsa-v1]
15. [Sigstore bundle protobuf schema][sigstore-bundle-proto]
16. [Fulcio OID directory][fulcio-oids]
17. [npm/cli `workspaces/libnpmpublish/lib/provenance.js` at `fc517df1`][npm-cli-provenance] and
    [`publish.js`][npm-cli-publish]; build-type history pinned at tags `v9.6.4` and `v10.0.0`
18. [npm/npm-registry-fetch at `6b4159a2`][npm-registry-fetch]
19. [slsa-verifier `verifiers/internal/gha/provenance_forgeable.go` and `builder.go` at
    `46099781`][slsa-verifier-forgeable]; [slsa-verifier npm docs][slsa-verifier-npm-docs]
20. [slsa-github-generator Node.js builder README at `bb91a050`][ghg-nodejs-readme]

Issue reports and discussions:

21. [slsa-framework/slsa-github-generator#2499 (registry E422 field mismatches; relaxation;
    open)][ghg-2499]
22. [slsa-framework/slsa-github-generator#2758 (single-field mismatch form)][ghg-2758]
23. [slsa-framework/slsa#940 (externalParameters standardization discussion)][slsa-940]
24. [npm/feedback#968 (GitLab validation bug, fixed upstream)][npm-feedback-968]
25. [npm/cli#8036 (repository.url case mismatch)][npm-cli-8036]
26. [npm/cli#7978 (repository URL normalization)][npm-cli-7978]
27. [npm/cli#8699 (missing SourceRepositoryRef) and #8982 (duplicate)][npm-cli-8699]
28. [GitHub Community discussion #177028][community-177028]
29. [npm/cli#9879 (silent bundle substitution under trusted publishing; not a
    rejection)][npm-cli-9879]
30. [Offline-Protocol/offline-protocol-sdk#323 (private repository rejection)][offline-323]
31. [humanspeak/svelte-purify#75 (self-hosted runner rejection)][purify-75]
32. [supabase/cli@a6e21f7 (self-hosted runner rejection, fix commit)][supabase-commit]
33. [Skyscanner/eslint-plugin-backpack#270 (unverified issuer message)][skyscanner-270]
34. [jupyter-book/mystmd#2662 (empty repository.url form)][mystmd-2662]
35. [Hacker News launch discussion (12023-04)][hn-launch]

Live registry responses (this research): `/-/npm/v1/attestations/gundam-visor@1.0.1` (200, standard
shape); `/-/npm/v1/attestations/@ianlewis/actions-test@0.1.162` (404). Local artifacts: preserved
attempt-8 bundle and the `0.1.2` registry attestation read-back recorded in `npm-m1.md`.

[npm-generating]: https://docs.npmjs.com/generating-provenance-statements/
[npm-trusted]: https://docs.npmjs.com/trusted-publishers/
[npm-publish-docs]: https://docs.npmjs.com/cli/v12/commands/npm-publish/
[npm-audit-docs]: https://docs.npmjs.com/cli/v12/commands/npm-audit/
[npm-viewing]: https://docs.npmjs.com/viewing-package-provenance/
[npm-provenance-readme]: https://github.com/npm/provenance/blob/main/README.md
[gh-blog-launch]:
  https://github.blog/security/supply-chain-security/introducing-npm-package-provenance/
[gh-changelog-beta]: https://github.blog/changelog/2023-04-19-npm-provenance-public-beta/
[gh-changelog-source-exists]:
  https://github.blog/changelog/2023-07-11-warn-when-the-npm-provenance-source-commit-or-repository-cannot-be-found/
[gh-changelog-private]:
  https://github.blog/changelog/2023-07-25-publishing-with-npm-provenance-from-private-source-repositories-is-no-longer-supported/
[gh-changelog-ga]: https://github.blog/changelog/2023-09-26-npm-provenance-general-availability/
[gh-changelog-tp]:
  https://github.blog/changelog/2025-07-31-npm-trusted-publishing-with-oidc-is-generally-available/
[buildtypes-readme]:
  https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/README.md
[buildtypes-example]:
  https://github.com/slsa-framework/github-actions-buildtypes/blob/main/workflow/v1/example.json
[slsa-v1]: https://slsa.dev/spec/v1.0/provenance
[sigstore-bundle-proto]:
  https://github.com/sigstore/protobuf-specs/blob/main/protos/sigstore_bundle.proto
[fulcio-oids]: https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md
[npm-cli-provenance]:
  https://github.com/npm/cli/blob/fc517df18fb59a33b39d383c6766690bc427ac06/workspaces/libnpmpublish/lib/provenance.js
[npm-cli-publish]:
  https://github.com/npm/cli/blob/fc517df18fb59a33b39d383c6766690bc427ac06/workspaces/libnpmpublish/lib/publish.js
[npm-registry-fetch]:
  https://github.com/npm/npm-registry-fetch/tree/6b4159a2519ce5aab26cc4dd8d4596a0b47781d2/lib
[slsa-verifier-forgeable]:
  https://github.com/slsa-framework/slsa-verifier/blob/46099781eca1b8e91884770f40476631b033e0e8/verifiers/internal/gha/provenance_forgeable.go
[slsa-verifier-npm-docs]:
  https://github.com/slsa-framework/slsa-verifier/blob/46099781eca1b8e91884770f40476631b033e0e8/docs/npm.md
[ghg-nodejs-readme]:
  https://github.com/slsa-framework/slsa-github-generator/blob/bb91a05077afa6601a3d7538c4cbbdbf0abe7ed9/internal/builders/nodejs/README.md
[ghg-2499]: https://github.com/slsa-framework/slsa-github-generator/issues/2499
[ghg-2758]: https://github.com/slsa-framework/slsa-github-generator/pull/2758
[slsa-940]: https://github.com/slsa-framework/slsa/issues/940
[npm-feedback-968]: https://github.com/npm/feedback/discussions/968
[npm-cli-8036]: https://github.com/npm/cli/issues/8036
[npm-cli-7978]: https://github.com/npm/cli/issues/7978
[npm-cli-8699]: https://github.com/npm/cli/issues/8699
[npm-cli-8982]: https://github.com/npm/cli/issues/8982
[community-177028]: https://github.com/orgs/community/discussions/177028
[npm-cli-9879]: https://github.com/npm/cli/issues/9879
[offline-323]: https://github.com/Offline-Protocol/offline-protocol-sdk/pull/323
[purify-75]: https://github.com/humanspeak/svelte-purify/pull/75
[supabase-commit]: https://github.com/supabase/cli/commit/a6e21f7b9fca85419aadcdc0dfcadc04b7991e8a
[skyscanner-270]: https://github.com/Skyscanner/eslint-plugin-backpack/pull/270
[mystmd-2662]: https://github.com/jupyter-book/mystmd/issues/2662
[hn-launch]: https://news.ycombinator.com/item?id=35630241
