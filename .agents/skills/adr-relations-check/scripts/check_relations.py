#!/usr/bin/env python3
"""Check ADR lifecycle metadata in docs/decisions/.

Verifies, for every ADR (docs/decisions/0*.md):

1. Closed status grammar (ADR 0065): status is exactly one of `proposed`,
   `rejected`, `accepted`, `deprecated`, or `superseded by ADR-XXXX`.
2. Relations symmetry: every forward `supersedes` / `partially-supersedes` /
   `amends` / `see-also` edge has the matching reverse edge on the target ADR.
3. Required `scope`: partial supersession and amendment relations (both
   directions) carry a non-empty `scope`.

Exit code 0 when no problems are found, 1 otherwise. Run from anywhere; the
repository root is resolved from this script's location unless --root is given.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

STATUS_OK = re.compile(r"^(proposed|rejected|accepted|deprecated|superseded by ADR-\d{4})$")
REVERSE = {
    "supersedes": "superseded-by",
    "partially-supersedes": "partially-superseded-by",
    "amends": "amended-by",
}
SCOPE_REQUIRED = "partially-supersedes|amends|partially-superseded-by|amended-by"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--root",
        type=Path,
        default=Path(__file__).resolve().parents[4],
        help="repository root (default: resolved from script location)",
    )
    args = parser.parse_args()
    decisions = args.root / "docs" / "decisions"

    files = sorted(decisions.glob("0*.md"))
    if not files:
        print(f"no ADR files found under {decisions}", file=sys.stderr)
        return 1

    fwd: list[tuple[str, str, str]] = []
    problems: list[str] = []
    for f in files:
        text = f.read_text(encoding="utf-8")
        fm_match = re.match(r"^---\n(.*?)\n---", text, re.S)
        if not fm_match:
            problems.append(f"{f.name}: missing frontmatter")
            continue
        fm = fm_match.group(1)
        num = re.search(r"(\d{4})-", f.name).group(1)  # type: ignore[union-attr]
        st_match = re.search(r"^status:\s*(.+)$", fm, re.M)
        if not st_match:
            problems.append(f"ADR-{num}: missing status")
            continue
        st = st_match.group(1).strip().strip('"')
        if not STATUS_OK.match(st):
            problems.append(f"ADR-{num}: bad status {st!r}")
        for rm in re.finditer(r'- type:\s*(\S+)\s*\n\s*target:\s*"?ADR-(\d{4})"?', fm):
            fwd.append((num, rm.group(1), rm.group(2)))
        for bm in re.finditer(
            rf'- type:\s*({SCOPE_REQUIRED})\s*\n\s*target:\s*"?ADR-(\d{4})"?\s*\n(.*?)(?=\n  - type:|\n---|\Z)',
            fm,
            re.S,
        ):
            if not re.search(r"scope:\s*\n?\s*\S", bm.group(3)):
                problems.append(f"{f.name}: {bm.group(1)} ADR-{bm.group(2)} missing scope")

    by_target: dict[str, list[tuple[str, str]]] = {}
    for s, ty, t in fwd:
        if ty in REVERSE:
            by_target.setdefault(t, []).append((s, REVERSE[ty]))
        if ty == "see-also":
            by_target.setdefault(t, []).append((s, "see-also"))
    for t, lst in by_target.items():
        targets = list(decisions.glob(f"{t}-*.md"))
        if not targets:
            problems.append(f"ADR-{t}: relation target file not found")
            continue
        ttext = targets[0].read_text(encoding="utf-8")
        for s, rev in lst:
            rev_pattern = rf'- type:\s*{re.escape(rev)}\s*\n\s*target:\s*"?ADR-{s}"?'
            if not re.search(rev_pattern, ttext):
                problems.append(f"ADR-{s} -> ADR-{t}: MISSING {rev}")

    summary = f"{len(files)} ADRs, {len(fwd)} edges"
    if problems:
        print(f"{summary} — PROBLEMS:")
        for p in problems:
            print(f"  - {p}")
        return 1
    print(f"{summary} — NO PROBLEMS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
