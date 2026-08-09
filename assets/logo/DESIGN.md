# slsa-builder Logo Design Document

<div align="center">

English | [한국어](DESIGN.ko.md)

</div>

**Created:** 12026-08-09 **Target file:** `assets/logo/logo.svg` (final. Refined from logo-3 as the
adopted version; all three drafts are preserved in the `archived/` subdirectory. The dark background
variant is `assets/logo/logo-dark.svg`, see §2.5)

---

## 1. Overview and Concept

SLSA is officially pronounced "salsa." The logo translates that pronunciation into a visual pun: a
bowl of salsa with a tortilla chip stuck into its upper right. It is a single flat-cartoon SVG with
a `0 0 512 512` viewBox.

The most important design constraint is legibility at small sizes. The logo is expected to shrink to
16–64px in contexts such as favicons and app icons, and every silhouette decision is subordinate to
this constraint. Even when scaled down, the "bowl + chip" composition must read instantly.

## 2. Design Language

### 2.1 Outline

A warm dark brown `#3d3029`, 12px thick, with round caps and round joins. These values are not
arbitrary; they are a deliberate midpoint between the two early drafts. logo-1 used a cold
near-black (`#1A1A1A`) 10px outline, while logo-2 used the same brown family at a heavier 14px.
logo-3 intentionally took the middle point, keeping strokes from clumping at small sizes while
preserving the logo's overall warm tone.

### 2.2 Palette

Colors are managed as a 12-color palette with named roles, but the final SVG inlines literal HEX
values. The drafts (logo-2, logo-3) defined the palette as CSS custom properties inside a `<style>`
block; however, some non-browser SVG consumers — certain rasterizers and favicon generation
pipelines — do not support CSS variables and would render broken colors. For portability, the final
version resolves every `var(--*)` reference to its actual value. The names below are palette role
names, not variables in the code.

| Name               | Value     | Role                                                     |
| ------------------ | --------- | -------------------------------------------------------- |
| `--outline`        | `#3d3029` | All outline strokes                                      |
| `--bowl`           | `#36a6a0` | Bowl body base color                                     |
| `--bowl-shadow`    | `#27847f` | Bowl interior surface, side shadow band, foot bottom lip |
| `--bowl-highlight` | `#82d1c9` | Bowl left highlight arc                                  |
| `--salsa`          | `#d94a34` | Salsa mound base color                                   |
| `--salsa-dark`     | `#ad3027` | Tomato chunks                                            |
| `--salsa-light`    | `#f47a4e` | Warm highlight crescent on the salsa surface             |
| `--green`          | `#3f8f4f` | Cilantro leaf and green bits base color                  |
| `--green-dark`     | `#2d6f3d` | Darker variation of the green bits                       |
| `--chip`           | `#f4c654` | Tortilla chip base color                                 |
| `--chip-light`     | `#ffe183` | Chip left-edge highlight                                 |
| `--toast`          | `#b8752f` | Chip toast spots                                         |

### 2.3 Shading Techniques

No gradients, SVG filters, or raster images are used at all. Depth comes solely from overlapping
flat fill layers — four specific techniques:

1. Shadow bands: a darker flat shape from the same color family laid inside the body.
2. Highlight strokes: a light round-cap stroke placed on the left-facing surface.
3. Low-contrast same-family variations: details separated with darker or lighter shades of the same
   family.
4. Inset principle: shadow fills sit flush against the **inner edge** of the outline stroke. Drawing
   a fill along the stroke's centerline makes it cover the inner half of the stroke, producing
   abrupt outline thinning and edge slivers. Both the bowl side shadow and the foot bottom shadow
   use arcs offset inward by half the stroke width.

### 2.4 Code Conventions

- No `<style>` block and no CSS classes. Outline attributes (`stroke`, `stroke-width`,
  `stroke-linecap`, `stroke-linejoin`) are declared as presentation attributes on each outlined
  element. The drafts' `.ink` class approach was dropped because renderers without CSS support would
  lose the outlines entirely.
- The root `<svg>` carries `role="img"` and `aria-labelledby`, with `<title id="logo-title">`
  providing the text alternative.
- Comments (`<!-- Bowl -->`, etc.) delimit element groups so the structure can be followed from
  source alone, without a graphics editor.

### 2.5 Dark Background Variant

`logo-dark.svg` is the variant for use on dark backgrounds. Geometry, shading, and the logo's
interior palette are identical to `logo.svg`; only the outline — the one element that touches the
background — changes from `#3d3029` to a cream `#F5E9DA`. Interior color contrast is a relationship
between the colors themselves and stays valid regardless of background brightness; the only place
that needs a light value is the outline responsible for separation from the background. `#F5E9DA` is
a cream tone in harmony with the warm palette, and the outline does not disappear even on
backgrounds as dark as `#0d1117` (GitHub dark). The variant's `<title>` is
`slsa-builder logo (dark background variant)` to tell it apart.

## 3. Element Details and Intent

### 3.1 Bowl

The form follows the silhouette of a deep pedestal-footed bowl — a compote-style serving bowl
standing on a single foot. Since the subject is salsa, the bowl takes the form of a serving vessel
suited to that context.

The pedestal foot is a separate path drawn **behind** the bowl body — a technique inherited from
logo-1. Merging the body and foot into one closed outline makes the stroke kink or overlap at the
seam; keeping the foot as a separate back layer keeps the junction permanently smooth. Shading is
applied only to the foot's bottom lip as a `--bowl-shadow` band.

A broad `--bowl-shadow` band shades the bowl's side, and the highlight is **a single arc** on the
left. An earlier iteration had two highlight strokes, but the pair of parallel lines read like drip
marks of sauce running down, so they were merged into one.

### 3.2 Chip

A large triangle with straight edges. Small-size legibility comes first: rounded or wavy edges blur
below 32px and stop reading as a triangle, so the silhouette stays sharp and straight.

The chip is tilted slightly toward vertical, widening the chip area visible above the salsa. The
more chip shows, the clearer the composition at small sizes.

There are 4 toast spots, all irregular organic paths. Perfect circles are deliberately avoided —
they read as mechanical and clash with a hand-made, toasted texture. A thin `--chip-light` highlight
shape runs along the left edge.

### 3.3 Chip Insertion Point (Key Detail)

The junction where the chip enters the sauce is the most carefully engineered point in the logo.
There is one rule: the mound's outline must never cut across the face of the chip. An outline
passing over the chip reads as the chip floating in front of the sauce, breaking the sense of depth.

The implementation is a single mound path. The silhouette below the chip swells upward into a
sauce-dollop curve that wraps the chip's submerged edge, so the outline hugs the chip in one
continuous curve without breaking. The draft (logo-3) achieved the same effect by cutting a U-shaped
dip (U-dip) into the mound and layering a separate dollop path with its fill and outline split
across two paths on top; but the overlay boundaries produced tiny spikes and outline gaps, so the
final version merges the dollop bezier directly into the mound silhouette.

The narrative this conveys is "the chip pierces the sauce surface, and the sauce climbs up onto the
chip." A thin bridge stroke instead of a filled dollop was considered and rejected: an empty stroke
reads as a line floating in midair, while only a filled shape reads as sauce.

### 3.4 Salsa Mound

The silhouette is an irregular organic curve. Perfect geometry (symmetric arcs, exact ellipses) is
avoided; it is drawn as a mound whose right side rises slightly higher.

Ingredient details are 4 tomato chunks (`--salsa-dark`) and 3 green bits (`--green`,
`--green-dark`). Spacing is jittered and sizes scattered within ±20–30%. An evenly spaced arithmetic
arrangement was tried and rejected as mechanical-looking — real salsa has ingredients embedded at
random.

The `--salsa-light` highlight crescent sits **inside** the mound surface. In earlier versions this
highlight spilled over the bowl rim or clung to the mound's edge, looking like an edge artifact. Its
current position is moved inward on the southern part of the surface, so it reads only as a sheen.

### 3.5 Cilantro Leaf Accent

A single outlined cilantro leaf rests on top of the mound. Its position was shifted about 35px left
of the chip insertion point. Previously the leaf crowded the junction together with the dollop and
the chip edge, making that spot feel congested; moving it left gave the insertion point visual
breathing room.

## 4. Origin and Design History

The final version is a refinement of logo-3, which itself is a deliberate hybrid of the two earlier
drafts.

Inherited from logo-1:

- The pedestal-footed bowl form
- The sharp, iconic straight-edged triangular chip
- Icon-first orientation toward small-size legibility

Inherited from logo-2:

- The warm palette and brown outline
- The chunky, organic salsa depiction
- The "sauce climbs onto the chip" depth storytelling
- The bowl shading treatment
- The CSS-variable-based palette structure (kept through logo-3; replaced with inlined HEX in the
  final version for portability)
- Accessibility metadata (`role="img"`, `<title>`)

Refinements from logo-3 to the final version:

- Merged the dollop overlay at the chip insertion point into the mound silhouette (§3.3)
- Inset the bowl side shadow and foot bottom shadow to the stroke's inner edge (§2.3)
- Inlined CSS custom properties and the `.ink` class into literal values and attributes (§2.2, §2.4)

logo-1.svg, logo-2.svg, and logo-3.svg are preserved unmodified as alternative drafts in the
`archived/` subdirectory.

## 5. Constraints and Non-Goals

- No text or letterforms.
- No gradients, filters, or raster images.
- A fully self-contained single SVG with no external resource references.
- Every silhouette decision is subordinate to small-size legibility. Only details that survive
  scaling down are kept.
