---
parent: Decisions
nav_order: 98
status: accepted
date: 12026-10-13
decision-makers: Yunseo Kim
relations:
  - type: partially-supersedes
    target: ADR-0085
    scope:
      "the clauses under which the pinned setup action, the GitHub-hosted runner toolcache, and its
      download fallback supply the build-stage Node.js runtime, and the premise that the bundled npm
      CLI is authenticated as part of the Node.js artifact through the pinned setup mechanism's
      checksum verification; the Node.js 24 major selection, the exact-patch pin requirement, the
      pair assertion as an observational guard, the npm floor check, and the runner and OS image
      constraints remain in force"
  - type: see-also
    target: ADR-0084
  - type: see-also
    target: ADR-0082
  - type: see-also
    target: ADR-0070
---

# Provision the Build-Stage Node.js Runtime from a Digest-Verified Release Tarball

## Context and Problem Statement

ADR 0085 records that "the bundled npm is authenticated as part of the Node.js artifact, through the
pinned setup mechanism's own checksum verification." A 12026-10-13 review of the pinned
`actions/setup-node` (v7.0.0, commit `820762786026740c76f36085b0efc47a31fe5020`) found this premise
factually incorrect: the action performs no checksum or signature verification of the Node.js
artifact on either acquisition path. The pinned execution bundle contains no checksum-verification
code (case-insensitive searches for `shasums`, `checksum`, and `verifyChecksum` yield no related
matches), the source at the same commit contains no verification logic in the acquisition modules,
and the acquisition flow is a toolcache lookup used as-is (`tc.find`) or a plain TLS download
(`tc.downloadTool`, which accepts no expected digest). Tracked in issue #132.

Consequently the build-stage Node.js runtime (and the npm CLI bundled with it) is authenticated only
transitively: by trust in the GitHub-hosted runner image on the toolcache path (baked at image build
time) and by TLS to nodejs.org on the download path. The pre-install pair assertion fails closed on
version drift, but a replaced binary that reports the expected version passes it. The toolcache path
is also structurally unverifiable at run time: there is no downloaded archive to hash, and hashing
the installed tree cannot be compared against the published `SHASUMS256.txt` archive checksums.

The threat boundary for this decision excludes compromise of the GitHub platform itself: any GitHub
Actions build already fully trusts the runner, the Actions control plane, and OIDC issuance, and no
in-workflow control can defend against them. The decision axis is narrower: whether the Node.js
artifact supplied by the distribution channel, at runner-image bake time or at run time, is
authenticated against a trust anchor independent of that channel. A checksum file fetched from the
same endpoint as the artifact is corruption detection, not authentication; qualifying anchors are
the Node.js release team's OpenPGP signature over the checksum manifest or a digest committed to
this repository.

How should the build-stage Node.js runtime be supplied and authenticated against an independent
trust anchor, given that the pinned setup mechanism performs no artifact verification and the
toolcache path is structurally unverifiable at run time?

## Decision Drivers

- The publish stage already authenticates its npm CLI against a repository-committed digest (ADR
  0082/0084); the runtime that executes the build should meet the same anchor standard.
- The verification anchor must be independent of the artifact channel; run-time-fetched keys and
  same-channel checksum files do not qualify.
- The existing provisioning machinery — digest-verified download, provisioning boundary, shim root
  discipline, exec boundary, toolchain pin records, closed diagnostics — bounds the new design
  surface to one more artifact class.
- SLSA does not require this control; the decision must be recorded as hardening, not compliance.
- Once the runtime is provisioned, the build-stage npm question (bundled vs independently
  provisioned) becomes decidable at the specification level; this ADR should not foreclose either
  answer.

## Considered Options

- Correct the record only: keep the pinned setup action and fix the premise in this ADR.
- Replace the setup action with a verification-capable installer (for example mise with GPG
  verification enabled).
- Verify at workflow level: manual download plus the nodejs.org documented chain (signed
  `SHASUMS256.txt` verified against release-team keys at run time).
- Provision the Node.js runtime from a digest-verified release tarball through the trusted core,
  extending the ADR 0084 mechanism.
- Run the build in digest-pinned official Node.js container images.

## Decision Outcome

Chosen option: "Provision the Node.js runtime from a digest-verified release tarball through the
trusted core", because it is the only option that authenticates the artifact against a trust anchor
fully independent of the distribution channel (a SHA-256 digest committed to this repository),
reuses the provisioning mechanism the project already operates for the publish-stage npm CLI, and
removes the structurally unverifiable toolcache path instead of layering checks on top of it.

### Nature of this decision

This is an additional hardening measure, not a conformance requirement. No SLSA Build level (v1.2,
L1-L3) requires producer-side verification of the build toolchain; the SLSA model assigns toolchain
integrity to the hosted build platform's trust boundary, which GitHub Actions already occupies for
this project's targets. The status-quo option below is consistent with prevailing ecosystem practice
and with SLSA, and would have remained a defensible position. This ADR is adopted because the
project already holds its publish stage to a stronger anchor standard than the ecosystem default and
chooses symmetry for the runtime, not because any external standard compels it.

### Mechanism

- The trusted core downloads the official Node.js release tarball for the pinned version and
  platform from nodejs.org, verifies it against the SHA-256 digest recorded in a toolchain pin
  record committed to this repository, and provisions it under the same boundary discipline as the
  publish-stage npm provisioning (exact layout, no symlinks, fail-closed behavior).
- The pinned setup action and the runner toolcache no longer supply the Node.js runtime. The pair
  assertion is re-expressed at the specification level as an observational guard over the
  provisioned runtime: it detects provisioning drift and is no longer the integrity control.
- The trust anchor is the committed digest. The pin-review ceremony moves the key bootstrap problem
  off the run-time path: when a pin record is added or updated, the reviewer obtains the digest by
  verifying the release's signed `SHASUMS256.txt` against the Node.js release-team keyring once, at
  review time, and commits the resulting digest. Run-time verification compares against the
  committed digest only.
- The same mechanism supplies the Node.js runtime of any job in the profile that needs one,
  including the publish job's node-only runtime; job-level wiring is a specification concern.

### Relationship to ADR 0085

This ADR partially supersedes ADR 0085 per the `relations` scope: the provisioning clauses and the
checksum-verification premise no longer hold as written, while the version pinning, pair assertion,
npm floor, and runner/OS constraints remain in force and are re-expressed over the provisioned
runtime at the specification level. The premise's intent is realized rather than discarded: under
this ADR the bundled npm is authenticated as part of the Node.js artifact, because the artifact
itself is now digest-verified.

ADR 0070's runner image capture is unchanged as evidence; the captured image no longer sources the
Node.js runtime claim.

### Build-stage npm flexibility

With the runtime provisioned, the build stage may use the npm CLI bundled inside the verified
Node.js tarball or provision a separate npm CLI through the ADR 0084 mechanism with a distinct build
pin record. This ADR does not choose between them; the choice is a specification-level decision per
stage and per profile, revisitable without a new ADR. The mechanism whose revisit the ADR 0085
escalation trigger reserves is now available on demand: adopting an independently provisioned
build-stage npm is a specification change, not an architectural one.

### Consequences

- Good, because both acquisition paths and the runner-image bake-time exposure are replaced by a
  single digest-verified path with a repository-committed anchor.
- Good, because the distribution-channel compromise grades that survive TLS — partial replacement
  (CDN cache poisoning) and full replacement of artifact plus checksum manifest — fail closed on
  digest mismatch, and even signing-key compromise does not retroactively defeat a digest committed
  before it.
- Good, because the ADR 0085 premise's intent becomes true under the new mechanism, and the
  build-stage npm provisioning question is decoupled from the runtime decision.
- Bad, because implementation cost exceeds the status quo: a new pin record class, provisioning of a
  large multi-platform artifact, workflow contract changes, fixture regeneration, and live dogfood
  re-verification. The cost is bounded by mechanism reuse, not eliminated.
- Bad, because every Node.js patch update now carries a digest review ceremony: the cadence ADR 0085
  already accepted for pair records, extended with review-time signature verification.
- Neutral, because GitHub platform compromise remains outside the defended boundary, as it is for
  every GitHub Actions build.

### Confirmation

Compliance is confirmed when: the workflowcheck contract accepts the provisioning step shape and
rejects toolcache-sourced runtimes in governed jobs; the pin-check drift gate covers the Node.js
runtime pin record; the fixture corpus exercises digest match, digest mismatch, and provisioning
boundary violations; and a live dogfood run of the reusable workflow builds and publishes with the
provisioned runtime with the runbook phases passing.

## Pros and Cons of the Options

### Correct the record only (status quo with honest wording)

- Good, because it is consistent with prevailing ecosystem practice: Electron and Signal Desktop pin
  Node.js versions through `actions/setup-node` with no checksum or signature verification in their
  workflows, and no SLSA Build level requires more. Choosing it would be neither negligent nor
  unusual.
- Good, because it adds no machinery, no new pin cadence, and no workflow contract change.
- Bad, because both acquisition paths remain unauthenticated against any independent anchor: an
  artifact replaced at the distribution channel, at bake time or at run time, is adopted without
  detection, and the pair assertion cannot detect a replaced binary that reports the expected
  version.
- Bad, because the asymmetry the project has already rejected for the publish-stage npm CLI
  (digest-verified publish tooling on an unverified runtime) remains.

### Replace the setup action with a verification-capable installer

- Good, because off-the-shelf checksum verification exists (mise, with optional GPG verification of
  the checksum manifest; nvm, checksum comparison with optional GPG).
- Bad, because the anchor problem moves rather than closes: the installer's run-time key or keyring
  retrieval is dynamic trust, and the installer itself becomes a new pinned supply-chain component.
- Bad, because the workflow contract the project's static gates encode (setup step shape, toolcache
  assumptions, runner image capture semantics) is re-specified for a weaker anchor than a committed
  digest.

### Verify at workflow level with the nodejs.org GPG chain

- Good, because it follows the first-party documented verification chain and the nodejs/docker-node
  official image precedent.
- Bad, because run-time key bootstrap is the weak link: a keyring fetched at run time and trusted on
  arrival inherits the channel it was fetched through, while pinning keys or fingerprints in the
  repository reproduces a committed-anchor design with more machinery than a digest record.
- Bad, because key rotation becomes a run-time operational ceremony instead of a review-time one.

### Provision through the trusted core (chosen)

- Good, because the anchor is a digest committed to this repository, independent of the distribution
  channel and of any run-time key retrieval.
- Good, because the incremental design and implementation surface is small: the digest-verified
  download, provisioning boundary, shim-root discipline, exec boundary, pin record types, and closed
  diagnostics already exist for the publish-stage npm CLI (ADR 0082/0084) and are extended to one
  more artifact class rather than redesigned.
- Good, because it removes the structurally unverifiable toolcache path instead of accepting it.
- Bad, because implementation cost is strictly higher than the status quo, and the Node.js patch
  cadence now carries a review-time digest ceremony.
- Bad, because provisioning a runtime is heavier than provisioning a CLI: larger artifact,
  platform-specific pin records, and an exec boundary that must cover the compiler toolchain's
  invocation surface rather than a single `npm` entry point.

### Digest-pinned container jobs

- Good, because an image digest is content-addressed identity.
- Bad, because digest identity is not provenance: no primary source establishes Sigstore attestation
  of the official Node.js images, and the digest must still be obtained from a trusted channel at
  pin time.
- Bad, because ADR 0085 already declined container jobs, a container job constrains the reusable
  workflow's caller environments, and the runner image capture contract would be replaced rather
  than refined.

## More Information

- Tracking issue: <https://github.com/windlasstech/slsa-builder/issues/132> (evidence detail for the
  setup-node finding, including pinned bundle and source inspection results).
- nodejs.org documented verification chain: signed `SHASUMS256.txt` (`.sig`/`.asc`) against the
  release-team keyring (<https://github.com/nodejs/release-keys>); key bootstrap-trust discussion in
  <https://github.com/nodejs/nodejs.org/issues/7942>.
- Precedent for signature-plus-checksum verification of Node.js release artifacts: the
  nodejs/docker-node official image build templates
  (<https://github.com/nodejs/docker-node/blob/main/Dockerfile-debian.template>).
- SLSA v1.2 Build requirements (<https://slsa.dev/spec/v1.2/build-requirements>): no level requires
  producer-side toolchain verification; the hosted-platform trust model governs.
- Distribution-channel integrity incident class: <https://github.com/nodejs/build/issues/3194>
  (CDN-path corruption detectable by checksum verification against an independent anchor).
- Verification-capable installers surveyed: mise (checksum plus optional GPG, extended to all
  release lines), nvm (checksum comparison, optional GPG), asdf-nodejs (precomputed checksums via
  node-build), Volta and fnm (no documented verification).
- Not decided here (specification-level follow-ons): the runtime pin record schema, provisioning
  boundary rules for the runtime artifact, diagnostic IDs, workflow step shape, multi-platform pin
  expansion beyond the production runner platform, the bundled-vs-provisioned build-stage npm
  choice, and GHES implications.

### Scenario coverage by verification tier

The options differ less in whether they "verify" than in which compromise grade their trust anchor
survives. The tiers map to the options as follows: the status quo performs no artifact verification;
a same-channel checksum comparison is the strongest check available to a workflow that keeps the
setup action (and is structurally unavailable on the toolcache path, which has no archive to hash);
run-time signature verification is what the installer-replacement and workflow-level GPG options
provide; a repository-committed digest is the chosen option's anchor (the container-digest variant
shares its anchor properties and is rejected on other grounds).

| Scenario                                                                          | No verification (status quo)                                            | Same-channel checksum                                                    | Run-time signature verification                    | Committed digest (chosen)                                                                               |
| --------------------------------------------------------------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------ | -------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| Transport corruption or single-file CDN cache poisoning (nodejs/build#3194 class) | Not detected                                                            | Detected                                                                 | Detected                                           | Detected                                                                                                |
| Distribution compromise replacing artifact and checksum manifest together         | Not detected                                                            | Not detected: the checksum file arrives through the compromised channel  | Detected: signing keys are a separate trust domain | Detected: the anchor sits in git history                                                                |
| Full compromise including the release signing keys                                | Not detected                                                            | Not detected                                                             | Not detected                                       | Detected for digests committed before the compromise; pin updates re-verify through the review ceremony |
| MITM on the download path                                                         | Not detected by the workflow; TLS is the only control                   | Marginal: an attacker who defeats TLS replaces the checksum file as well | Detected                                           | Detected                                                                                                |
| Compromised artifact baked into the runner image toolcache                        | Not detected                                                            | Structurally unavailable: no archive to hash                             | Eliminated: the toolcache path is not used         | Eliminated: the toolcache path is not used                                                              |
| GitHub platform compromise (runner, control plane, OIDC issuance)                 | Out of scope: no in-workflow control defends against it, for any option | Same                                                                     | Same                                               | Same                                                                                                    |

Run-time signature verification holds only when the trusted keys are not themselves bootstrapped
through the same channel at run time; see the key bootstrap discussion under the corresponding
options above.
