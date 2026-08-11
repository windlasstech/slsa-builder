---
name: badge-generator
description:
  Regenerate the slsa-builder spread-the-word badge SVGs in assets/badges/ — 3 types (built-with,
  verified-with, plain logo badge) x 4 shields.io-compatible styles (flat, flat-square, plastic,
  for-the-badge), with the molcajete logo embedded next to the brand text. MUST run when the logo
  artwork changes, a badge type or style is added/removed, brand colors change, or badge text
  changes. Triggers on "badge", "배지", "regenerate badges", "spread the word", "shields style", or
  README badge snippet updates.
---

# Badge Generator

## Quick start

```bash
python3 .agents/skills/badge-generator/scripts/generate_badges.py
```

Regenerates all 12 SVGs in `assets/badges/` deterministically from `assets/logo/logo-dark.svg`. Run
from anywhere — the repo root is resolved from the script location (override with `--root <path>`).
Requires Python 3 with Pillow and the macOS Supplemental Verdana fonts (adapt the `FONT` /
`FONT_BOLD` constants on other platforms).

After regenerating, render and visually inspect before committing (see QA below). If badge types,
filenames, or text changed, update the badge sections in `README.ko.md` and `README.md` (both, per
the bilingual README rule) and the `CHANGELOG.md` entry.

## Design decisions (do not regress these)

- **Static SVG, not shields.io URLs.** Badges are repo-versioned assets hotlinked via
  `raw.githubusercontent.com` (verified to serve `image/svg+xml`). shields.io's `logo=` parameter
  cannot carry the full-color logo and bloats URLs with base64.
- **Logo variant.** Always embed `logo-dark.svg` (light-stroke `#F5E9DA` variant); badge segments
  are dark, so the default dark-stroke `logo.svg` would be low-contrast. The logo is nested as an
  inner `<svg viewBox="0 0 512 512">`, never rasterized or base64-inlined.
- **Logo position.** The logo sits at the left edge of the message segment, adjacent to the
  `slsa-builder` brand text — not in the gray label segment.
- **Type icons.** `built-with` carries a package-with-check icon and `verified-with` a
  shield-with-check icon at the left edge of the label segment (`ICON_PACKAGE` / `ICON_SHIELD`
  constants, recolored from the black sources to `#fff` for contrast on `#555`). The plain logo
  badge has no type icon.
- **Color semantics.** Message segment is brand green `#3f8f4f`; label is shields gray `#555`. The
  plain logo badge ships in both gray (canonical) and green. Do not use the logo's salsa red
  `#d94a34` on badges — red reads as CI failure at badge size.
- **Text metrics.** Widths are measured with PIL + real Verdana and pinned via `textLength`; never
  hand-estimate. `for-the-badge` additionally compensates +11.5 units/character (`FTB_PER_CHAR`,
  empirically fitted to real shields badges) for the letterspaced look.

## shields.io style contract

| Style           | Geometry                                                                  |
| --------------- | ------------------------------------------------------------------------- |
| `flat`          | 20px, rx=3, soft gradient, text shadow, Verdana 11, 5px padding           |
| `flat-square`   | 20px, square corners, matte (no gradient), text shadow, Verdana 11        |
| `plastic`       | 20px, rx=4, gloss gradient (white .7 → black .5), text shadow, Verdana 11 |
| `for-the-badge` | 28px, UPPERCASE, Verdana 10, bold message only, 12px padding, crispEdges  |

Naming: canonical `flat` is unsuffixed (`built-with-slsa-builder.svg`); other styles append
`-flat-square`, `-plastic`, `-for-the-badge`. The plain badge's green variant places the color
suffix before the style suffix (`slsa-builder-green-for-the-badge.svg`).

## QA

1. XML-parse all 12 SVGs.
2. Render each with `qlmanage -t -s 700 -o <dir> <file>.svg` and inspect: logo crisp and correctly
   positioned, text centered with no clipping or squish, style geometry correct (rounded vs square
   corners, plastic gloss, for-the-badge caps/bold).
3. Re-run the script after any fix — output must stay byte-identical unless a design input changed.
