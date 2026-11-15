---
name: adr-relations-check
description:
  Verify ADR lifecycle metadata in docs/decisions/ for the slsa-builder repository — closed status
  grammar, bidirectional relations symmetry (supersedes / partially-supersedes / amends / see-also
  reverse edges), and required scope fields per ADR 0065. MUST run after adding a new ADR, flipping
  an ADR status, or editing any relations frontmatter, and before committing ADR changes. Triggers
  on "relations check", "ADR symmetry", "reverse edge", new ADR numbering, or ADR traceability
  repair.
---

# ADR Relations Check

## Quick start

```bash
python3 .agents/skills/adr-relations-check/scripts/check_relations.py
```

Exit code 0 and `NO PROBLEMS` mean the ADR set is consistent. Any other result lists every defect;
exit code is 1. Run from anywhere — the repo root is resolved from the script location (override
with `--root <path>`).

## What it checks

1. **Status grammar** (ADR 0065): `status` is exactly one of `proposed`, `rejected`, `accepted`,
   `deprecated`, `superseded by ADR-XXXX`. Composite or prose values are defects.
2. **Relations symmetry**: every forward `supersedes` / `partially-supersedes` / `amends` /
   `see-also` edge in a `relations` frontmatter field has the matching reverse edge on the target
   ADR (`superseded-by` / `partially-superseded-by` / `amended-by` / `see-also`).
3. **Required scope**: `partially-supersedes`, `amends`, and their reverse forms carry a non-empty
   `scope` identifying the affected clauses.
4. **Scope symmetry**: mirrored `partially-supersedes` / `amends` pairs carry identical `scope`
   content on both edges (compared after whitespace and YAML quoting normalization).

The script validates mechanics only. Whether a replacement is full or partial is a judgment call
governed by the repair rules below; the script cannot make it.

## Repair rules

- A missing reverse edge is a traceability defect: add the reverse entry to the target ADR's
  `relations` field in the same change. No new ADR is required for the repair, and accepted ADR
  bodies stay immutable — only `status` and `relations` frontmatter may change after acceptance.
- Use `partially-supersedes` when an implementer may no longer follow the earlier clause as written;
  use `amends` when the earlier clause still governs and the newer ADR only qualifies it. Resolve
  ambiguous cases in favor of `partially-supersedes`.
- Only full supersession changes the earlier ADR's `status` (to `superseded by ADR-XXXX`); partial
  supersession and amendment leave it `accepted`.
- Choose `supersedes` only when the newer ADR replaces the earlier decision in full: enumerate the
  earlier ADR's normative clauses and confirm each is replaced, re-carried by a newer accepted ADR,
  or spent (realized or moot). If any self-owned clause still governs and no accepted ADR or
  specification carries it, the relation is `partially-supersedes`, with a `scope` naming the
  replaced clauses and the standing remainder. Write the same `scope` content on the forward and
  reverse entries of a partial or amendment relation, and cite ADRs by number rather than as
  `this ADR`. Content inherited from external standards or other accepted ADRs does not count as a
  surviving self-owned clause. Pre-ADR-0065 bodies used "supersedes" loosely, so the word alone is
  not evidence of full replacement; scoped wording in a Decision Outcome ("supersedes ADR X's
  \<clause\>") is evidence of partial scope. Cross-check co-predecessors of the same superseding
  ADR: do not classify one as fully superseded while a co-predecessor with strictly less surviving
  content is classified as partially superseded. When clause survival is undecidable from the
  accepted texts, prefer `partially-supersedes`. This preference covers only genuinely undecidable
  survival: when the enumeration finds no standing clause, `supersedes` is the required
  classification, because an accepted-but-dead ADR misleads readers into treating obsolete clauses
  as governing.
- Every ADR has exactly one core decision subject, read from its own title, its Context and Problem
  Statement question, and its Considered Options set. The dimension along which the options vary is
  the decision axis; a parameter chosen within the winning option is a derivative clause. Identify
  the core subject from the earlier ADR's own framing, never from the newer ADR's diff. When
  authoring a new ADR that replaces a predecessor's core subject and its decision, use `supersedes`
  and account for every derivative clause that is not spent at writing time, whether by re-carrying
  it unchanged or by replacing it with new normative content; never leave a core-replaced
  predecessor `accepted` on the strength of surviving subsidiary clauses, and never leave a live
  derivative clause behind in the superseded predecessor.

Full conventions:
[ADR 0065](../../../docs/decisions/0065-use-closed-status-grammar-with-separate-relations-field.md)
and [docs/decisions/AGENTS.md](../../../docs/decisions/AGENTS.md).
