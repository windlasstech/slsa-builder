# Architecture Specifications

<div align="center">

English | [한국어](README.ko.md)

</div>

Architecture specifications for `slsa-builder`. This directory holds the **what** of the system:
observable behavior, contracts, schemas, and verification criteria. The **why** lives in
`docs/decisions/`, which contains the architecture decision records (ADRs) that drive these specs.

> [!Note]  
> This project follows Spec-Driven Development (SDD): decide first in ADRs, specify exact behavior
> here, then implement against the specifications.

## SDD model

| Layer          | Location             | Answers                  | May contain                                        |
| -------------- | -------------------- | ------------------------ | -------------------------------------------------- |
| Decisions      | `docs/decisions/`    | Why did we choose this?  | Rationale, trade-offs, consequences                |
| Specifications | `docs/architecture/` | What must the system do? | Contracts, schemas, invariants, examples, fixtures |
| Implementation | source tree          | How does it work?        | Code, workflows, tests, fixtures                   |

Implementation may not contradict an accepted ADR. If a spec draft reveals a contradiction with an
accepted ADR, stop and write a new ADR rather than editing the accepted ADR body.

## Spec inventory

| Spec                                                                        | Purpose                                                      | Source ADRs                                                |
| --------------------------------------------------------------------------- | ------------------------------------------------------------ | ---------------------------------------------------------- |
| [Core profile contract](core-profile-contract.md)                           | Thin core vs. profile-owned reusable workflow boundaries     | 0002, 0003, 0004, 0035, 0042                               |
| [Identity and build types](identity-and-buildtypes.md)                      | `builder.id`, `buildType` URI, and release metadata linkage  | 0028, 0031, 0042, 0049, 0053                               |
| [SLSA provenance v1](slsa-provenance-v1.md)                                 | Common in-toto Statement and SLSA v1 predicate contract      | 0002, 0003, 0028, 0029, 0035, 0037, 0042                   |
| [JS/TS npm package profile](js-ts-npm-package-profile.md)                   | Public reusable workflow contract for npm packages           | 0013, 0018, 0022, 0023, 0024, 0026, 0027, 0030, 0032, 0034 |
| [JS/TS npm build and pack](js-ts-npm-build-pack.md)                         | Package selection, package manager rules, install/build/pack | 0014, 0015, 0016, 0017, 0018, 0019, 0023, 0027, 0033       |
| [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md)         | Provenance, three-job publish graph, npm publish             | 0024, 0025, 0028, 0029, 0030, 0035, 0036, 0037, 0052       |
| [GitHub Release asset publisher](github-release-asset-publisher.md)         | Verified distributor publisher contract                      | 0039, 0043, 0045, 0046, 0048, 0049, 0050, 0051, 0052       |
| [Composed workflow internal handoff](composed-workflow-internal-handoff.md) | Same-run producer-to-publisher internal handoff              | 0036, 0050, 0052                                           |
| [npm-to-release-asset composition](npm-to-release-asset-composition.md)     | First producer-to-publisher composition                      | 0013–0037, 0049, 0050, 0051, 0052                          |
| [Release manifest](release-manifest.md)                                     | Signed release manifest and three-job signing boundary       | 0028, 0031, 0035, 0042, 0053, 0054                         |
| [Verification policy and fixtures](verification-policy-and-fixtures.md)     | Verifier policy, fixture taxonomy, reference commands        | 0028, 0029, 0030, 0036, 0037, 0049, 0050, 0051, 0052–0054  |

## ADR traceability

Every ADR is either mapped to a spec, marked as tooling-only, or marked as superseded, deprecated,
etc. Superseded or deprecated ADRs are historical context and must not drive new specification or
implementation work.

See [`../decisions/README.md`](../decisions/README.md#adr-traceability) for the canonical detailed
ADR traceability tables.

## Spec writing rules

1. **One spec per file.** Keep each file focused on a single contract.
2. **ADR references first.** Every normative section must trace to one or more accepted ADRs.
   Superseded or deprecated ADRs may be cited only as historical context.
3. **Schemas and examples.** Every input, output, and invariant must include a concrete example and
   at least one negative example or rejection case.
4. **No implementation details.** Do not describe internal code structure, variable names, or
   library choices. Specs describe observable behavior.
5. **Failure explicit.** Every "must" must include the failure behavior when the condition is not
   met.
6. **TDD artifacts.** Every spec must identify the test fixtures and review checklists that will
   prove it in implementation.
7. **Cross-links.** Link related specs using relative paths. Avoid duplicating normative text across
   files.
8. **Terminology.** Use the canonical terms defined in this index. Do not introduce synonyms without
   defining them.

## Canonical terminology

| Term                 | Meaning                                                                                                                                                                               |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Core**             | The shared trusted foundation: reusable workflow invariants, provenance construction, digest-verified handoff, signing adapter interface, and verification documentation conventions. |
| **Profile**          | A profile-owned reusable workflow that defines an ecosystem-specific build, pack, subject, digest, publish, and verification contract.                                                |
| **Producer**         | A profile that produces final artifact bytes and source-to-artifact SLSA provenance.                                                                                                  |
| **Publisher**        | The GitHub Release asset publisher profile, which verifies and distributes producer artifacts without claiming to build them.                                                         |
| **Builder**          | The trusted reusable workflow platform that generates source-to-artifact provenance. The GitHub Release asset publisher is not a builder in the default production path.              |
| **Handoff**          | The digest- and provenance-verified contract between a producer profile and the publisher.                                                                                            |
| **Sidecar**          | A release asset that travels alongside the primary asset but is not part of the primary asset's SLSA subject digest.                                                                  |
| **Release manifest** | The Windlass-signed machine-verifiable mapping from release versions to workflow SHAs, `builder.id` values, and `buildType` URIs.                                                     |

## How to add a new architecture spec

1. Open a new ADR if the new spec requires an architectural decision not covered by existing
   accepted ADRs.
2. Add the spec to the inventory table above and update ADR traceability in
   [`../decisions/README.md`](../decisions/README.md#adr-traceability).
3. Update cross-links in related specs.
4. Add the spec to the verification checklist below.
5. Run the documentation quality commands before submitting.

## Verification checklist for docs PRs

- [ ] Every accepted ADR is mapped to a spec or explicitly marked tooling-only.
- [ ] Every superseded ADR is listed in the historical table and not treated as current.
- [ ] Every new spec includes a purpose, source ADRs, target audience, sections,
      inputs/outputs/schemas, and verification criteria.
- [ ] Every normative "must" has a failure behavior.
- [ ] Every schema has at least one valid and one invalid example.
- [ ] Cross-links between specs use relative paths and are valid.
- [ ] The canonical terminology table is respected.
- [ ] Formatting passes:

  ```bash
  pnpm exec prettier --check "docs/**/*.md"
  pnpm exec markdownlint-cli2 "docs/**/*.md"
  ```
