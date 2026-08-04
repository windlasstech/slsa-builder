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
python3 .claude/skills/adr-relations-check/scripts/check_relations.py
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

## Repair rules

- A missing reverse edge is a traceability defect: add the reverse entry to the target ADR's
  `relations` field in the same change. No new ADR is required for the repair, and accepted ADR
  bodies stay immutable — only `status` and `relations` frontmatter may change after acceptance.
- Use `partially-supersedes` when an implementer may no longer follow the earlier clause as written;
  use `amends` when the earlier clause still governs and the newer ADR only qualifies it. Resolve
  ambiguous cases in favor of `partially-supersedes`.
- Only full supersession changes the earlier ADR's `status` (to `superseded by ADR-XXXX`); partial
  supersession and amendment leave it `accepted`.

Full conventions:
[ADR 0065](../../../docs/decisions/0065-use-closed-status-grammar-with-separate-relations-field.md)
and [docs/decisions/AGENTS.md](../../../docs/decisions/AGENTS.md).
