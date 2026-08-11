# slsa-builder Logo Design Document

<div align="center">

English | [한국어](DESIGN.ko.md)

</div>

**Created:** 12026-08-09  
**Target files:** `assets/logo/logo.svg` (adopted) and `assets/logo/logo-dark.svg` (dark-background
variant). Earlier drafts are preserved unmodified in `assets/logo/archived/` (see §4).

---

## 1. Overview and Concept

SLSA is officially pronounced "salsa." The logo turns that pronunciation — and the project name
slsa-builder — into a visual pun: a _molcajete_, the vesicular-basalt mortar traditionally used in
Mexico to make salsa, its matching stone pestle the _tejolote_, and the _salsa roja_ being made
inside it. It is a single flat-cartoon-style SVG with a `0 0 512 512` viewBox.

The most important design constraint is legibility at small sizes. The logo is expected to shrink to
16–64px in contexts such as favicons and app icons, and every silhouette decision is subordinate to
this constraint. Even scaled down, the "stone mortar + stubby pestle + red contents" composition
must read instantly.

## 2. Design Language

### 2.1 Outline

A warm dark brown `#3d3029`, 12px thick, round caps and round joins. The same values also outline
the bowl's inner opening — a second stroked contour is what makes the mortar's wall thickness
visible (§3.1).

### 2.2 Palette

Colors are managed as a named-role palette but written into the SVG as literal HEX values. The
drafts once defined the palette as CSS custom properties in a `<style>` block, but non-browser SVG
consumers (some rasterizers, favicon pipelines) do not support CSS variables and colors can break,
so the final files resolve every `var(--*)` reference to its literal value for portability. The
names below are palette role names, not variables in the code.

| Name               | Value     | Role                                        |
| ------------------ | --------- | ------------------------------------------- |
| `--outline`        | `#3d3029` | All outline strokes                         |
| `--stone`          | `#756F68` | Basalt body base color (bowl, legs, pestle) |
| `--stone-shadow`   | `#57514B` | Stone shadow bands, recessed center leg     |
| `--stone-interior` | `#4E4842` | Bowl inner wall                             |
| `--pore-dark`      | `#4B4641` | Dark vesicle pits                           |
| `--pore-light`     | `#8C847C` | Light vesicle pits (surface brightness)     |
| `--salsa`          | `#d94a34` | Salsa base color                            |
| `--salsa-dark`     | `#ad3027` | Tomato chunks, ground particles             |
| `--salsa-light`    | `#f47a4e` | Warm sauce accents                          |
| `--green`          | `#3f8f4f` | Cilantro leaf and green bits                |
| `--green-dark`     | `#2d6f3d` | Darker green variation                      |
| `--cream`          | `#F5E9DA` | Onion/garlic pieces                         |

### 2.3 Shading Techniques

No gradients, SVG filters, or raster images. Depth comes solely from overlapping flat fill layers —
four techniques:

1. Shadow bands: a darker flat shape from the same color family laid inside the body.
2. Low-contrast same-family variations: details separated with darker or lighter shades of the same
   family.
3. Inset principle: shadow fills sit flush against the **inner edge** of the outline stroke. Drawing
   a fill along the stroke's centerline makes it cover the inner half of the stroke, producing
   abrupt outline thinning and edge slivers; every shadow arc is offset inward by half the stroke
   width.
4. Texture by pits, never by gloss: rough vesicular stone has no specular highlight. Brightness
   variation on the bowl comes from scattered light pore pits; no smooth highlight arcs are drawn on
   stone (§3.1).

### 2.4 Code Conventions

- No `<style>` block and no CSS classes. Outline attributes (`stroke`, `stroke-width`,
  `stroke-linecap`, `stroke-linejoin`) are declared as presentation attributes on each outline
  element, so the file renders identically in any consumer.
- The root `<svg>` carries `role="img"` and `aria-labelledby`, with `<title id="logo-title">` as the
  text alternative.
- Comments (`<!-- Bowl -->`, etc.) delimit element groups so the structure can be followed from
  source alone, without a graphics editor.
- A single wrapper `<g transform="...">` handles optical fit: the adopted file scales the
  composition 1.12× about the pixel-measured artwork center so the viewBox margins balance on all
  sides.

### 2.5 Dark Theme Variant

`logo-dark.svg` is the variant for use on dark backgrounds. Geometry, shading, and the logo's
internal palette are identical to `logo.svg`; only the outline that touches the background changes
from `#3d3029` to cream `#F5E9DA`. The logo's internal color contrast is a relationship among its
own colors, so it holds regardless of background brightness — the only place a light value is needed
is the outline that separates the artwork from the background. `#F5E9DA` is a cream tone that
harmonizes with the warm palette and keeps the outline from disappearing even on backgrounds as dark
as `#0d1117` (GitHub dark). The variant's `<title>` reads
`slsa-builder logo (dark background variant)` to distinguish it.

## 3. Element Details and Intent

### 3.1 _Molcajete_ Bowl

The bowl follows the form of a traditional _molcajete_, not a generic bowl:

- **Medium-deep wide mortar.** A _molcajete_ is fundamentally a mortar that can also double as
  serveware — a wide mouth with real depth. It is neither a shallow serving dish nor a narrow, deep
  European apothecary mortar.
- **Three short legs (tripod).** Two side legs in the body color, one center leg drawn in the shadow
  color and slightly shorter to read as recessed behind. Legs are drawn behind the bowl body so the
  junction stays smooth.
- **Thick wall.** The rim reads as a visible band: an outer silhouette plus a smaller inner opening
  carrying its own outline. A single shared ellipse for body top and interior would make the wall
  mathematically zero-thick — the inner opening must be a distinct, smaller stroked ellipse.
- **Vesicular texture.** Small irregular dark and light pore pits scattered over the body and legs —
  organic blob paths, never perfect circles.
- **No gloss.** Rough porous stone does not produce specular arcs; a smooth highlight arc
  appropriate to glazed ceramic was removed for this reason.

### 3.2 _Tejolote_

The pestle follows the authentic tool's proportions and form:

- **Stubby truncated cone.** Real _tejolotes_ measure roughly 10–13cm long with a 6–7.5cm grinding
  face and a 4–5cm handle — a heavy stone stump, not a long thin western pestle. Length-to-width
  must land in **1.3–2.0:1**, and this is verified by objective pixel measurement of the rendered
  silhouette (PCA extents), never by eyeballing. The adopted _tejolote_ measures 1.63:1.
- **Blunt hemispherical end.** The lower end is a worn, rounded fingertip-like bulb — not a
  mathematically flat cut plane, and not a long shaft with a ball tip.
- **Blunt domed handle end.** The upper (grip) end is a cubic-Bézier dome whose end tangents
  continue the side lines — no corner where the cap meets the sides.
- **About half outside the bowl**, leaning diagonally from the upper right (≈35° in the adopted
  file). A _tejolote_ grinds by rolling along the mortar wall under a full palm grip, so a steep
  pounding pose is avoided.

### 3.3 Pestle–Sauce Junction (Key Detail)

One rule: the sauce outline must never cross the pestle's face at an acute angle. Acute crossings
between two dark outlines produce wedge/sliver artifacts, and a straight contour crossing the
submerged end reads as the pestle tucked behind the sauce rather than immersed in it.

The implementation is a single merged sauce contour: it climbs nearly parallel up the pestle's left
side, crests onto the face, and descends on the right, while the sauce fill covers the submerged
portion. Separate overlay or "splash" paths were tried and rejected — they reintroduce acute stroke
crossings (hooks, notches) at their boundaries.

### 3.4 _Salsa Roja_

- **Fill level below the rim**, so the mortar's dark inner back wall is visibly exposed above the
  sauce. This sells both "mortar" and "making."
- **Ground particle cluster** directly under the pestle head, staging the ingredients being crushed
  between _molcajete_ and _tejolote_ in real time.
- **One half-smashed tomato** plus whole ingredients (dark-red tomato chunks, green bits, one or two
  cream onion/garlic pieces), scattered with jitter. Even spacing reads mechanical; real salsa is
  irregular, if anything.

### 3.5 Candidate Comparison and Final Combination

logo-6 and logo-7 were independent interpretations of the same brief (§1–§3.4 binding on both). The
adopted `logo.svg` (drafted as logo-8) combines their verified strengths:

| Aspect                | logo-6                             | logo-7                       | Adopted                                                         |
| --------------------- | ---------------------------------- | ---------------------------- | --------------------------------------------------------------- |
| Pestle angle          | ~45°                               | ~35°, placed further right   | logo-7 (local coords + transform)                               |
| Bowl curvature        | Deeper ellipse from logo-4         | Wider side-to-side           | logo-7                                                          |
| Back rim band         | Present                            | Absent (front-lip sag)       | Present — raised top arc (~16px)                                |
| Side shadow           | Segmented crescent                 | Single continuous sweep      | Segmented crescent, lower arc flush to the outline's inner edge |
| Pores                 | 13, including legs                 | 4                            | logo-6 density, re-placed                                       |
| Salsa contour         | Merged wrap                        | Single merged base+wrap path | logo-7 path verbatim                                            |
| Sauce volume          | Reads sparse                       | Generous                     | logo-7                                                          |
| Ingredient detail     | Particles, smashed tomato, garnish | Sparser, stroked bits        | logo-6 set, repositioned                                        |
| Measured pestle ratio | 1.46:1                             | 1.63:1                       | 1.63:1                                                          |

Post-combination review refinements:

- The crescent shadow's lower arc rides the bowl outline's inner edge — no bright gap (§2.3 inset
  principle).
- Every small ingredient sits ≥8px inside the sauce outline; nothing crosses onto the inner wall (a
  perspective-impossible placement).
- Ground particles and the half-smashed tomato cluster at this _tejolote_'s grinding point — they
  had kept logo-6's coordinates after the pestle moved.
- The handle end was rounded into the tangent-continuous dome (§3.2).
- The composition was scaled 1.12× and re-centered from pixel-measured bounds.

## 4. Origin and Design History

- **logo-1, logo-2, logo-3** — early drafts of the serving concept (salsa bowl with a dipped
  tortilla chip); archived.
- **logo-4, logo-4-dark** (originally `logo.svg` / `logo-dark.svg`) — the first adopted serving
  design: teal pedestal-footed bowl (compote form) with a dipped chip, plus the first dark variant.
  Archived when the making concept was adopted; its dark-variant convention (cream outline swap,
  fills unchanged) carries over to the new variant.
- **logo-5** — serving concept re-cast as a basalt _molcajete_: introduced the stone palette, tripod
  legs, vesicular pores, and the no-gloss rule. Archived.
- **logo-6** — the making concept: _tejolote_ replaces the chip, sauce level lowered to expose the
  inner wall. First to introduce proportion measurement via pixel measurement (PCA extents) of the
  rendered silhouette. Archived.
- **logo-7** — a second, independent interpretation of the logo-6 brief, produced separately with
  the same full requirement set for candidate comparison, mirroring how logo-3 emerged from
  combining logo-1 and logo-2. Archived.
- **logo-8 → adopted as `assets/logo/logo.svg`** — the final combination of logo-6 and logo-7
  (§3.5), refined during review and adopted.
- **`logo-dark.svg`** — dark-background variant of the adopted logo (§2.5).

`archived/` preserves logo-1–7 and logo-4-dark unmodified.

## 5. Constraints and Non-Goals

- No text or letterforms.
- No gradients, filters, or raster images.
- A fully self-contained single SVG with no external resource references.
- Every silhouette decision is subordinate to small-size legibility.
- No specular/gloss arcs on stone surfaces.
- No long-thin pestle silhouettes; _tejolote_ proportions must be objectively measurable within
  1.3–2.0:1.
- No flat, cleanly-cut pestle ends.
- The mortar's wall thickness must be explicitly expressed (outer silhouette plus smaller stroked
  inner opening).
- The dark variant changes only the outline color; geometry and fills stay identical to the light
  file.
