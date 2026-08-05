# Trusted Runtime Delivery

- Date: 12026-08-05
- Plan task: F01, Wave 0
- Decision: Proven; use platform-resolved exact-SHA source checkout with OIDC cross-binding

## Decision

Production reusable workflows will deliver the Go trusted core from the same `slsa-builder` commit
that defines the called workflow:

1. A caller invokes the reusable workflow with a full 40-character commit SHA. GitHub documents a
   commit SHA as the safest reusable-workflow reference for stability and security.
2. Inside the called job, GitHub's `job.workflow_repository`, `job.workflow_sha`,
   `job.workflow_file_path`, and `job.workflow_ref` contexts identify the repository, commit, path,
   and full ref of the workflow file that defines that job. Unlike the corresponding `github`
   context values, these properties identify the called reusable workflow.
3. A full-SHA-pinned `actions/checkout` checks out `job.workflow_repository` at `job.workflow_sha`,
   with `persist-credentials: false`. The workflow reads the resulting Git `HEAD` and rejects any
   value other than the platform-resolved SHA before invoking Go.
4. The first delivered Go operation requests a GitHub OIDC token, verifies its RS256 signature
   against GitHub's discovered JWKS, validates issuer, audience, and time bounds, and requires:
   - `job_workflow_sha == job.workflow_sha == checked-out HEAD`;
   - `job_workflow_ref == job.workflow_ref`; and
   - a full lowercase 40-hex SHA suffix in the workflow ref.
5. Only after those checks may the Go process enter provenance, digest, policy, signing-input, or
   other trusted payload logic. The authenticated SHA becomes the suffix of the ADR 0028
   `builder.id`:

   ```text
   https://github.com/windlasstech/slsa-builder/.github/workflows/<workflow>.yml@<job_workflow_sha>
   ```

`job.workflow_sha` is the pre-execution delivery selector. The OIDC token's signed
`job_workflow_sha` remains the authoritative value for emitted builder identity, as required by the
[identity and build types specification](architecture/identity-and-buildtypes.md#runtime-identity-acquisition-and-pin-validation).
The workflow must fail rather than fall back if either surface is unavailable or they disagree.

The production workflow will not expose the spike-only `delivery_sha` input. It will set both the
checkout ref and expected revision directly from `job.workflow_sha`.

## Why This Is Non-Circular

The delivered Go source does not select or authenticate itself. Before any delivered Go code runs,
the bootstrap trust base is limited to:

- GitHub's reusable-workflow resolver, which selects the callee workflow at the caller's full SHA;
- GitHub-hosted runner and `job.workflow_*` context generation;
- full-SHA-pinned `step-security/harden-runner` and `actions/checkout` actions;
- GitHub repository transport and Git's exact commit checkout; and
- the hosted Go toolchain that compiles already authenticated source.

Those components are part of the hosted build platform or independently immutable action pins. The
selected Go source cannot influence `job.workflow_sha`, the checkout `repository` or `ref`, or the
pre-execution `HEAD` comparison. The Go code then ratifies the same identity cryptographically by
verifying GitHub's signed OIDC token before trusted payload behavior.

The phrase “before code execution” in this decision means before execution of the delivered builder
runtime. GitHub runner bootstrap and the independently SHA-pinned hardening and checkout actions are
necessarily prior platform operations; they are not code delivered from the builder repository.

## Threat Model

### In scope

- A caller supplies a false SHA input or attempts to select different builder source.
- A mutable branch or tag points to different source after review.
- Default checkout silently selects the caller repository.
- Downloaded source differs from the platform-resolved called-workflow commit.
- A workflow context for the caller is confused with the called workflow's identity.
- An OIDC token has a wrong issuer, audience, signature, validity interval, workflow path, or SHA.
- A compromised delivery endpoint returns bytes from a different Git commit.

Each case fails before trusted payload logic. Source selection uses no caller-supplied repository or
ref in production, and the checked-out commit, platform context, and signed OIDC claim must converge
on one value.

### Trusted platform assumptions

- GitHub correctly resolves full-SHA reusable workflow and action references.
- GitHub-hosted runners correctly supply the documented `job.workflow_*` contexts and OIDC request
  capability.
- GitHub repository transport returns the Git object identified by the requested commit SHA.
- GitHub's OIDC discovery document and JWKS are authentic over TLS.

These are hosted build-platform assumptions already accepted by ADRs 0002 and 0003. The mechanism is
github.com-only because GitHub documents the `job.workflow_*` properties as unavailable on GitHub
Enterprise Server.

## Live Spike Evidence

The live spike ran in the private repository
[`yunseo-kim/slsa-builder-conformance`](https://github.com/yunseo-kim/slsa-builder-conformance),
whose visibility was verified as `PRIVATE` before use. The called workflow was pinned to:

```text
e8cb8528dcd6a69bc9da93f6b5930a9aa5b09304
```

### Matching revision

- Run:
  [`30995562119`](https://github.com/yunseo-kim/slsa-builder-conformance/actions/runs/30995562119)
- `gh run watch --exit-status`: exit `0`
- Artifact: `runtime-delivery-report`

| Report field            | Value                                      |
| ----------------------- | ------------------------------------------ |
| `result`                | `pass`                                     |
| `requested_sha`         | `e8cb8528dcd6a69bc9da93f6b5930a9aa5b09304` |
| `executed_sha`          | `e8cb8528dcd6a69bc9da93f6b5930a9aa5b09304` |
| `job_workflow_sha`      | `e8cb8528dcd6a69bc9da93f6b5930a9aa5b09304` |
| `platform_workflow_sha` | `e8cb8528dcd6a69bc9da93f6b5930a9aa5b09304` |
| `payload_executed`      | `true`                                     |

The OIDC-authenticated `job_workflow_ref` and the platform `job.workflow_ref` were both:

```text
yunseo-kim/slsa-builder-conformance/.github/workflows/runtime-delivery-callee.yml@e8cb8528dcd6a69bc9da93f6b5930a9aa5b09304
```

### Tampered delivered revision

- Run:
  [`30995688342`](https://github.com/yunseo-kim/slsa-builder-conformance/actions/runs/30995688342)
- `gh run watch --exit-status`: exit `1`, as required
- Requested and platform workflow SHA: `e8cb8528dcd6a69bc9da93f6b5930a9aa5b09304`
- Injected delivered revision: `c9ab8faf089093b2e1e00fbd5b9e3bd4492ba01a`
- Failure phase: `pre-execution`
- `payload_executed`: `false`
- Payload marker: absent from the downloaded artifact
- Go verification/payload step: skipped

In the failure report, `executed_sha` records the delivered candidate revision observed by the
pre-execution gate; `payload_executed: false` proves it was not run. `job_workflow_sha` is `null`
because the gate intentionally stopped before requesting OIDC or invoking any delivered Go. The
independent platform value remained the expected callee SHA.

The exact non-production workflow copies and report schema are under
[`testdata/platform/runtime-delivery/`](../testdata/platform/runtime-delivery/).

## Alternatives

### Default or caller-repository checkout

Rejected. GitHub documents that reusable-workflow jobs retain caller workflow context, and
`actions/checkout` defaults to the repository that triggered the workflow. It does not deliver the
called workflow's co-located Go source.

### Caller-supplied builder SHA

Rejected as a trust source. A declared input can be useful only as an expectation that is checked
against platform identity. Letting the caller choose both the expected SHA and checkout ref would be
self-asserting and would break ADR 0028's binding.

### Decode OIDC in shell before checkout

Rejected. It could recover `job_workflow_sha`, but it would move JWT parsing and identity policy
into shell before the Go trust boundary, contradicting the approved implementation plan's Go/shell
division. The documented `job.workflow_sha` context removes that bootstrap need.

### Digest-pinned prebuilt binary

Not selected for the initial implementation. An expected binary digest embedded in the pinned
workflow could provide non-circular integrity, but a release or attestation by itself cannot: its
verifier must first be delivered through an independent trust path. This option also adds binary
build provenance, platform variants, immutable hosting, digest rotation, and a separate
source-to-binary release mapping before the core exists. Exact-SHA source checkout provides the same
workflow identity binding with one repository revision and a smaller bootstrap surface.

### Hybrid source and prebuilt delivery

Rejected for now because two runtime channels and fallback rules enlarge the trusted surface. A
future performance optimization may add a digest-pinned binary only if it preserves the exact
workflow-SHA/OIDC convergence and fails closed without source or mutable-tag fallback.

## Platform Documentation Verified

- [GitHub Actions contexts: `job.workflow_*`](https://docs.github.com/en/actions/reference/workflows-and-actions/contexts#job-context)
  documents the called job's workflow repository, SHA, path, and ref, plus the exact-source checkout
  example and GHES limitation.
- [Reuse workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows#calling-a-reusable-workflow)
  documents SHA, tag, and branch refs, calls commit SHA the safest option, and states that contexts
  or expressions cannot compute `jobs.<job_id>.uses` dynamically.
- [OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
  defines `job_workflow_ref` as the reusable workflow ref path and `job_workflow_sha` as the
  reusable workflow file's commit SHA.
- [OIDC with reusable workflows](https://docs.github.com/en/actions/how-tos/secure-your-work/security-harden-deployments/oidc-with-reusable-workflows#how-the-token-works-with-reusable-workflows)
  distinguishes caller claims from the called workflow's `job_workflow_ref`.
- [`actions/checkout`](https://github.com/actions/checkout) documents explicit `repository` and
  branch, tag, or SHA `ref` inputs, one-commit default fetch depth, and
  `persist-credentials: false`.
- [Windlass workflow hardening](https://raw.githubusercontent.com/windlasstech/.github/refs/heads/main/docs/security/workflow-hardening.md)
  requires full-SHA action pins, harden-runner first, and least-privilege permissions.

## ADR And Specification Check

No contradiction was found:

- ADRs 0002 and 0003 allow the hosted GitHub platform and pinned actions in the bootstrap trust
  base.
- ADR 0004 remains satisfied because JWT, identity, and policy checks are performed by delivered Go;
  shell only invokes tools and compares the checked-out Git revision to platform context.
- ADR 0028 remains satisfied because the caller pin, platform callee SHA, OIDC claim, executed
  source, and resulting `builder.id` converge on one immutable commit.
- ADR 0068 remains satisfied because the OIDC claim, not caller input or `github.workflow_sha`, is
  authoritative for builder identity.
- The core profile and identity specifications require fail-closed checks that the spike exercised.

ADR 0077 and specification deltas are therefore not required. Any future platform observation that
removes or changes `job.workflow_*`, `job_workflow_sha`, or exact-SHA checkout behavior is a stop
condition and requires a new ADR before production implementation continues.
