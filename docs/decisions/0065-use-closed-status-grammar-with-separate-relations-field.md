---
parent: Decisions
nav_order: 65
status: accepted
date: 12026-07-31
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0000
    scope:
      "MADR 4.0.0 template usage: the status field is restricted to a closed grammar, and a
      relations frontmatter field is added to the project-local ADR format"
---

# Use a Closed Status Grammar and a Separate Relations Field for ADR Lifecycle Metadata

## Context and Problem Statement

ADR 0000 adopted MADR 4.0.0 as the decision record format, and the repository conventions added a
local immutability rule: after acceptance, only the `status` field may change. Neither MADR nor the
local conventions define what values `status` may contain. MADR 4.0.0 shows an open example list
(`proposed | rejected | accepted | deprecated | … | superseded by ADR-0123`) and deliberately leaves
the grammar to each project.

A pre-implementation review of the ADR set (0000–0064) found that this undefined grammar has already
produced defects:

- ADR 0042 carries the composite prose status `accepted; partially updated by ADR-0049`, which does
  not identify which clauses were updated. An implementer reading the accepted ADR cannot tell that
  its GitHub Release asset `buildType` URI assignment is void while its namespace decision stands.
- ADR 0050 and ADR 0052 carry `status: amended by 0064`, a lifecycle value with no defined meaning.
- ADR 0022 and ADR 0029 remain plain `accepted` even though ADR 0028 and ADR 0042 replaced specific
  normative clauses in them (the tag-ref `builder.id` form and the repository-path `buildType` URI).
- ADR 0042's "Superseded decisions" section omitted ADR 0029, and ADR 0049's supersession list
  omitted ADR 0045, leaving traceability gaps that index tables silently reproduce.
- Because accepted ADR bodies are immutable, none of these defects can be repaired by editing the
  affected ADRs' text.

Research into established practice (MADR templates and issues, the original Nygard ADR article,
adr-tools, log4brains, AWS Prescriptive Guidance, Microsoft Learn, PEP 1, the Kubernetes KEP
process, and IETF RFC header conventions) found a consistent pattern:

1. The recognized lifecycle vocabulary is small: `proposed`, `rejected`, `accepted`, `deprecated`,
   and `superseded by <reference>`.
2. Keywords such as `amended`, `updated`, or `partially superseded` are not lifecycle statuses
   anywhere in the surveyed sources. Where they exist, they are relation labels between documents
   (adr-tools `Amends`/`Amended by` links; RFC `Updates`/`Obsoletes` headers and the proposed
   `Amended by`/`Extended by` tag pairs; PEP `Replaces`/`Superseded-By` metadata).
3. Partial changes to an accepted decision are handled by a narrow follow-up ADR plus explicit
   bidirectional relation metadata, while the earlier ADR remains accepted.
4. When a superseding document forgets to list a predecessor, the established remedy is to repair
   the trace graph (add the missing reciprocal relation), not to invent a new lifecycle state.

The project therefore needs a local decision: what closed grammar governs the `status` field, how
are partial supersessions and amendments represented, and how are existing traceability defects
repaired without violating ADR body immutability?

## Decision Drivers

- Keep the `status` field machine-validatable so that index generation, lint rules, and review
  checklists can enforce it mechanically.
- Preserve the immutability of accepted ADR bodies; minimize and precisely enumerate the fields that
  may change after acceptance.
- Represent partial supersession and amendment without inventing additional lifecycle states.
- Require bidirectional traceability: a superseding or amending ADR must enumerate every predecessor
  it affects, and omissions must be repairable.
- Stay aligned with MADR 4.0.0 and the broader ADR ecosystem so that the format remains familiar and
  tooling-compatible.
- Give the six currently defective ADRs (0022, 0029, 0042, 0045, 0050, 0052) a conformant repair
  path.

## Considered Options

- Use a closed `status` grammar and record relations in a new structured `relations` frontmatter
  field.
- Use a closed `status` grammar and record relations as links in the body `More Information`
  section, following adr-tools and MADR linking conventions.
- Keep the open prose `status` field (status quo) and rely on review discipline.
- Extend the lifecycle vocabulary with additional statuses such as `amended` and
  `partially superseded`.

## Decision Outcome

Chosen option: "Use a closed `status` grammar and record relations in a new structured `relations`
frontmatter field", because it keeps lifecycle state machine-validatable, expresses partial changes
as relation metadata consistent with ecosystem practice, and confines post-acceptance edits to two
explicitly named frontmatter fields.

### Status grammar

The `status` field must use exactly one of the following forms:

```text
proposed
rejected
accepted
deprecated
superseded by ADR-XXXX
```

No other value, composite form, or prose annotation is permitted. `deprecated` means the decision is
no longer recommended and no replacing ADR exists or is required. `superseded by ADR-XXXX` means the
named ADR replaces the decision in full; the target reference is mandatory. Full supersession is the
only relation that also changes the `status` field.

### Relations field

ADR frontmatter may include a `relations` list. Each entry must have:

- `type`: exactly one of the eight relation labels below;
- `target`: an existing ADR number in `ADR-XXXX` form;
- `scope`: required for `partially-supersedes`, `partially-superseded-by`, `amends`, and
  `amended-by`; identifies the affected clauses or sections. Optional otherwise.

The relation vocabulary is closed and consists of four directional pairs:

| Forward (recorded on the newer ADR) | Reverse (recorded on the earlier ADR) | Meaning                                        | Effect on earlier ADR status        |
| ----------------------------------- | ------------------------------------- | ---------------------------------------------- | ----------------------------------- |
| `supersedes`                        | `superseded-by`                       | The newer ADR replaces the earlier one in full | Changes to `superseded by ADR-XXXX` |
| `partially-supersedes`              | `partially-superseded-by`             | Named clauses are replaced; the rest stands    | Remains `accepted`                  |
| `amends`                            | `amended-by`                          | Details are narrowed, adjusted, or excepted    | Remains `accepted`                  |
| `see-also`                          | `see-also`                            | Informational cross-reference                  | None                                |

### Distinguishing partial supersession from amendment

The boundary between `partially-supersedes` and `amends` is the vitality of the earlier clause:

- `partially-supersedes` **kills** the named clause. The clause no longer has effect; the newer ADR
  supplies replacement normative content or voids the clause outright.
- `amends` **keeps** the named clause in effect. The earlier decision still governs; the newer ADR
  qualifies it by narrowing its scope, adjusting details, or adding exceptions.

The normative discriminator, applied from the implementer's perspective:

> May an implementer still follow the earlier clause as written?
>
> - If not—following it as written would violate the newer decision—the relation is
>   `partially-supersedes`.
> - If yes, provided the newer ADR's qualification is also applied, the relation is `amends`.

A supporting test: if the newer ADR were retracted, would the earlier clause resume its original
meaning? A clause that was killed but has a restoration point is partially superseded; a clause that
never lost effect is amended.

Voiding a clause without replacement, as when ADR 0049 voided ADR 0042's Release asset `buildType`
assignment, is classified as `partially-supersedes`, because an implementer can no longer follow the
earlier clause as written.

Reviewers must apply this discriminator when classifying relations. When a case remains ambiguous
after both tests, it is resolved in favor of `partially-supersedes`, the stronger statement, because
it warns readers not to rely on the earlier clause.

### Symmetry and completeness rules

- Relation edges are bidirectional. When a new ADR declares `supersedes`, `partially-supersedes`, or
  `amends` against a target, the same change must add the matching reverse entry to the target ADR's
  `relations` field.
- A new ADR must enumerate every accepted ADR it supersedes, partially supersedes, or amends in its
  `relations` field. An omission is a traceability defect, not a new lifecycle state.
- A traceability defect is repaired by adding the missing reverse relation entry to the affected
  earlier ADR. Body text is not edited, and a new ADR is not required for the repair alone.
- Post-acceptance edits are permitted only to the `status` and `relations` frontmatter fields. All
  other content remains immutable after acceptance.

### Retroactive application

The existing defective statuses are repaired by rewriting them into this grammar without changing
any accepted ADR body:

- ADR 0022 and ADR 0029 keep `status: accepted` and gain `partially-superseded-by` entries naming
  ADR 0028 and ADR 0042 respectively, with `scope` identifying the replaced clauses.
- ADR 0042 keeps `status: accepted`; its composite status is removed and its
  `partially-superseded-by ADR-0049` relation gains a `scope` naming the void Release asset
  `buildType` assignment. ADR 0029 gains the missing reverse edge for ADR 0042's omitted
  supersession.
- ADR 0050 and ADR 0052 keep `status: accepted`; `amended by 0064` becomes `amended-by ADR-0064`
  relation entries with scope.
- ADR 0045's subject-name decision concerns a publisher model that ADR 0049 replaced. Whether ADR
  0045 is superseded in full or narrowed to its asset-name normalization guidance is a substantive
  decision and must be made by a separate follow-up ADR, not by this metadata repair.
- ADR 0000 gains the reverse `amended-by ADR-0065` entry required by this ADR's own symmetry rule.

### Consequences

- Good, because `status` becomes a closed, machine-validatable grammar and ad-hoc composite values
  are eliminated.
- Good, because partial supersession and amendment become representable without corrupting the
  lifecycle vocabulary, matching adr-tools, RFC, PEP, and KEP practice.
- Good, because the supersession-omission defects in ADR 0042 and ADR 0049 become repairable through
  reverse relation entries without touching immutable ADR bodies.
- Good, because the traceability index in `docs/decisions/README.md` can be generated or validated
  from structured frontmatter instead of maintained by hand.
- Good, because this ADR exercises its own mechanism immediately: it amends ADR 0000's template
  usage and declares that relation in its frontmatter.
- Neutral, because the project-local format now extends the stock MADR 4.0.0 frontmatter and must
  document the extension for contributors familiar with upstream MADR.
- Bad, because every future superseding or amending ADR must update reverse edges on its targets,
  adding a small mechanical burden to each decision PR.
- Bad, because scopes are prose and cannot be fully machine-validated; review must check that
  `scope` precisely identifies the affected clauses.

### Confirmation

This decision is confirmed when:

- `docs/decisions/AGENTS.md` and `docs/decisions/README.md` document the closed status grammar, the
  relation vocabulary, the partial-supersession-versus-amendment discriminator, the symmetry and
  completeness rules, and the expanded post-acceptance edit allowance (`status` and `relations`
  only);
- the six defective ADRs (0022, 0029, 0042, 0050, 0052, and 0000's reverse edge) carry conformant
  `status` and `relations` values, with ADR 0045 left for its follow-up decision;
- review tooling or the ADR PR checklist validates the status grammar, relation types, target
  existence, reverse-edge symmetry, and `scope` presence for partial and amendment relations;
- the ADR traceability tables in `docs/decisions/README.md` and `README.ko.md` reflect the repaired
  relations.

## Pros and Cons of the Options

### Closed status grammar with a frontmatter relations field

Keep lifecycle state in a five-form closed `status` grammar and record all relationships in a
structured YAML list validated by schema.

- Good, because both fields become mechanically enforceable in review and lint.
- Good, because frontmatter structure enables symmetry checks and index generation.
- Good, because post-acceptance edits remain confined to two named frontmatter fields, preserving
  body immutability without exceptions for body sections.
- Good, because partial changes and amendments map directly to established relation-label practice
  in adr-tools, RFC metadata, PEP, and KEP processes.
- Bad, because the format extends upstream MADR 4.0.0 frontmatter.
- Bad, because reverse-edge maintenance adds work to every decision PR.

### Closed status grammar with relations in the body More Information section

Keep the closed status grammar but record relationships as Markdown links in the body, as adr-tools
does in its Status section.

- Good, because it matches upstream MADR and adr-tools conventions exactly.
- Good, because no frontmatter extension is needed.
- Bad, because adding reverse links to an accepted ADR edits its body, conflicting with the local
  immutability rule or forcing a broader exception to it.
- Bad, because prose links resist machine validation, symmetry checks, and index generation.

### Keep the open prose status field

Continue allowing free-form status values and rely on reviewer discipline.

- Good, because no convention or tooling change is required.
- Bad, because composite and invented values like the current three cases are unverifiable and will
  recur.
- Bad, because index tables and review checklists cannot mechanically classify ADRs.

### Extend the lifecycle vocabulary

Add statuses such as `amended` and `partially superseded` alongside the five recognized forms.

- Good, because the status line alone would show that something changed.
- Bad, because no surveyed methodology or tool recognizes these as lifecycle states; they conflict
  with ecosystem vocabulary.
- Bad, because they blur the distinction between "the decision stands" and "the decision is
  replaced", and still fail to record which clauses changed without a separate mechanism.
- Bad, because each new state multiplies transition rules that tooling must understand.

## More Information

This decision follows ADR 0000's adoption of MADR 4.0.0 and the local immutability convention, and
it responds to defects found during the pre-implementation specification review of ADRs 0022, 0029,
0042, 0045, 0050, and 0052.

Reference points considered:

- MADR 4.0.0 template, which presents `status` as an open example list and leaves the grammar to
  each project: <https://github.com/adr/madr/blob/develop/template/adr-template.md>
- MADR project discussions treating additional states and link history as open design questions:
  <https://github.com/adr/madr/issues/2> and <https://github.com/adr/madr/issues/9>
- Michael Nygard's original ADR article, which defines the keep-and-mark-superseded model:
  <https://www.cognitect.com/blog/2011/11/15/documenting-architecture-decisions>
- adr-tools, which implements `Amends`/`Amended by` as bidirectional link labels rather than
  statuses: <https://github.com/npryce/adr-tools>
- log4brains, which states the immutability rule as "an ADR is immutable. Only its status can
  change": <https://github.com/thomvaill/log4brains>
- AWS Prescriptive Guidance, which makes accepted ADRs immutable and transitions old ADRs to
  Superseded:
  <https://docs.aws.amazon.com/prescriptive-guidance/latest/architectural-decision-records/adr-process.html>
- Microsoft Learn, which describes the ADR log as append-only:
  <https://learn.microsoft.com/en-us/azure/well-architected/architect-role/architecture-decision-record>
- PEP 1, which separates lifecycle status from `Replaces`/`Superseded-By` metadata:
  <https://peps.python.org/pep-0001/>
- Kubernetes KEP process, which uses `replaces`/`superseded-by` metadata:
  <https://github.com/kubernetes/enhancements/blob/master/keps/sig-architecture/0000-kep-process/README.md>
- RFC 7322 `Updates`/`Obsoletes` headers and the IETF proposal for `Amended by`/`Extended by` tag
  pairs: <https://www.rfc-editor.org/rfc/rfc7322.html> and
  <https://datatracker.ietf.org/doc/html/draft-kuehlewind-rswg-updates-tag-03>
