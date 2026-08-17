# Architecture Spec Knowledge Base

## OVERVIEW

Exact observable-behavior specifications for the SLSA builder. This is the **what** layer of SDD,
between ADRs in `docs/decisions/` and the implementation in `internal/`.

## STRUCTURE

```text
docs/architecture/
├── core-profile-contract.md                 # Core and profile workflow trust boundaries.
├── identity-and-buildtypes.md               # Builder identity, buildType URIs, release metadata.
├── slsa-provenance-v1.md                    # Shared in-toto Statement and SLSA v1 predicate.
├── js-ts-npm-package-profile.md             # Public npm reusable-workflow contract.
├── js-ts-npm-build-pack.md                  # Package selection, toolchains, build, and pack.
├── js-ts-npm-provenance-publish.md          # Provenance, signing, npm publish, three-job graph.
├── github-release-asset-publisher.md        # Verified release-asset distributor contract.
├── composed-workflow-internal-handoff.md    # Same-run producer-to-publisher handoff.
├── npm-to-release-asset-composition.md      # npm producer and release-publisher composition.
├── release-manifest.md                      # Signed manifest and signing boundary.
├── verification-policy-and-fixtures.md      # Verifier policy, fixtures, reference commands.
├── README.md / README.ko.md                 # Inventory, terminology, writing rules.
└── AGENTS.md
```

## WHERE TO LOOK

| Task or topic                      | Spec                                                                           | Notes                                         |
| ---------------------------------- | ------------------------------------------------------------------------------ | --------------------------------------------- |
| Core and profile responsibilities  | `core-profile-contract.md`                                                     | Trust boundaries and shared invariants.       |
| Builder identity and URI contracts | `identity-and-buildtypes.md`                                                   | `builder.id`, `buildType`, release linkage.   |
| Common provenance shape            | `slsa-provenance-v1.md`                                                        | Statement, predicate, material rules.         |
| npm public workflow surface        | `js-ts-npm-package-profile.md`                                                 | Inputs, outputs, caller modes, guards.        |
| npm build and package bytes        | `js-ts-npm-build-pack.md`                                                      | Manager selection, install, build, pack.      |
| npm provenance and publish graph   | `js-ts-npm-provenance-publish.md`                                              | Digest handoff, signing, publish convergence. |
| Release asset distribution         | `github-release-asset-publisher.md`                                            | Publisher verification and upload behavior.   |
| Producer-publisher composition     | `composed-workflow-internal-handoff.md`, `npm-to-release-asset-composition.md` | Internal mapping and public composition.      |
| Release metadata signing           | `release-manifest.md`                                                          | Manifest schema and three-job boundary.       |
| Verification behavior              | `verification-policy-and-fixtures.md`                                          | Trust roots, rejection cases, fixtures.       |
| pnpm settings-only workspace mode  | `js-ts-npm-build-pack.md`                                                      | ADR 0078 root-only resolution.                |
| Caller-specified source ref retry  | `js-ts-npm-package-profile.md`                                                 | ADRs 0079/0080 dispatch-retry contract.       |
| npm OIDC exchange response shape   | `js-ts-npm-provenance-publish.md`                                              | ADR 0081 union timestamps, 15-minute token.   |
| Live operational evidence          | `../dogfood/`                                                                  | M1 dogfood record/procedure; not normative.   |

## CONVENTIONS

- **One contract per file**: Keep each specification focused on one observable boundary.
- **Accepted ADRs govern**: Every normative section traces to accepted ADRs. A contradiction means
  stop and write a new ADR, never rewrite an accepted ADR body.
- **Historical ADRs do not govern**: Superseded and deprecated ADRs may provide context only.
- **Behavior, not rationale**: Specify contracts, schemas, invariants, examples, fixtures, and
  failure behavior. Put rationale and trade-offs in ADRs.
- **Examples prove contracts**: Each input, output, and invariant needs a concrete valid example and
  a negative or rejection case. Every normative `must` states its failure behavior.
- **Implementation stays out**: No internal structure, variable names, library choices, or workflow
  mechanics beyond externally observable behavior.
- **Traceability stays current**: When an ADR changes observable behavior, update affected specs in
  the same change. Maintain ADR-to-spec mappings in `docs/decisions/README.md`.
- **Links and terms**: Use relative links, avoid duplicated normative text, and preserve the
  canonical terminology in `README.md`.
- **README pair**: Edit `README.md` and `README.ko.md` together.
- **Dates**: Use Holocene Era dates in human-facing prose, for example `12026-08-05`.

## ANTI-PATTERNS

- Do not put rationale or trade-offs here. They belong in ADRs.
- Do not contradict an accepted ADR.
- Do not let superseded or deprecated ADRs drive new specification content.
- Do not describe implementation internals or the how.
- Do not edit `README.md` without updating `README.ko.md`.
