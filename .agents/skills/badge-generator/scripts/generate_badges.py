#!/usr/bin/env python3
"""Regenerate slsa-builder's shields-style static badge SVGs.

Writes 12 badges (3 types x 4 styles) to assets/badges/, embedding the nested
contents of assets/logo/logo-dark.svg (light-stroke variant for dark segments).
Run from anywhere: the repo root is resolved from the script location
(override with --root <path>).

Styles mirror shields.io conventions:
  flat          rx=3, soft gradient, text shadow, 20px, Verdana 11
  flat-square   square corners, no gradient, text shadow, 20px, Verdana 11
  plastic       rx=4, strong gloss gradient, text shadow, 20px, Verdana 11
  for-the-badge 28px, UPPERCASE, Verdana 10, bold message only, 12px padding,
                per-character letterspacing compensation, crispEdges

Two-segment badges carry the logo at the left edge of the right (brand-colored)
segment, adjacent to the "slsa-builder" message text. Flat is the canonical
unsuffixed filename; other styles add a suffix. Requires Pillow and a system
Verdana + Verdana Bold (paths below are the macOS Supplemental fonts).
"""

import argparse
import math
import re
from pathlib import Path

from PIL import ImageFont

FONT = "/System/Library/Fonts/Supplemental/Verdana.ttf"
FONT_BOLD = "/System/Library/Fonts/Supplemental/Verdana Bold.ttf"

fonts = {
    (110, False): ImageFont.truetype(FONT, 110),
    (100, False): ImageFont.truetype(FONT, 100),
    (100, True): ImageFont.truetype(FONT_BOLD, 100),
}


def text_len(s: str, size: int, bold: bool = False) -> float:
    return round(fonts[(size, bold)].getlength(s), 1)


# Empirical shields.io for-the-badge width correction: their textLength runs
# ~11.5 badge-units (1.15px) wider per character than raw Verdana advances
# (fitted from real shields badges: BUILD/LICENSE/COVERAGE/TESTED WITH/
# PASSING/MIT/100%, max residual < 0.5px). textLength pins glyphs to this
# width, reproducing shields' characteristic letterspaced look.
FTB_PER_CHAR = 11.5


def ftb_len(s: str, bold: bool = False) -> float:
    return round(fonts[(100, bold)].getlength(s) + FTB_PER_CHAR * len(s), 1)


def extract_logo_inner(repo: Path) -> str:
    raw = (repo / "assets/logo/logo-dark.svg").read_text()
    m = re.search(r"<svg[^>]*>(.*)</svg>", raw, re.S)
    assert m is not None, "logo-dark.svg has no <svg> root"
    inner = m.group(1)
    inner = re.sub(r"<title[^>]*>.*?</title>", "", inner, flags=re.S)
    inner = re.sub(r"<!--.*?-->", "", inner, flags=re.S)
    inner = re.sub(r"\n\s*\n", "\n", inner).strip()
    return inner


FLAT_GRADIENT = (
    '<stop offset="0" stop-color="#bbb" stop-opacity=".1"/>'
    '<stop offset="1" stop-opacity=".1"/>'
)
PLASTIC_GRADIENT = (
    '<stop offset="0" stop-color="#fff" stop-opacity=".7"/>'
    '<stop offset=".1" stop-color="#aaa" stop-opacity=".1"/>'
    '<stop offset=".9" stop-color="#000" stop-opacity=".3"/>'
    '<stop offset="1" stop-color="#000" stop-opacity=".5"/>'
)

# Type icons at the left edge of the label segment, recolored from the #000000
# sources to #fff for contrast on the #555 segment.
ICON_PACKAGE = (
    '<path d="M20.5 7.27783L12 12.0001M12 12.0001L3.49997 7.27783M12 12.0001L12 21.5001'
    "M14 20.889L12.777 21.5684C12.4934 21.726 12.3516 21.8047 12.2015 21.8356C12.0685 "
    "21.863 11.9315 21.863 11.7986 21.8356C11.6484 21.8047 11.5066 21.726 11.223 "
    "21.5684L3.82297 17.4573C3.52346 17.2909 3.37368 17.2077 3.26463 17.0893C3.16816 "
    "16.9847 3.09515 16.8606 3.05048 16.7254C3 16.5726 3 16.4013 3 16.0586V7.94153C3 "
    "7.59889 3 7.42757 3.05048 7.27477C3.09515 7.13959 3.16816 7.01551 3.26463 "
    "6.91082C3.37368 6.79248 3.52345 6.70928 3.82297 6.54288L11.223 2.43177C11.5066 "
    "2.27421 11.6484 2.19543 11.7986 2.16454C11.9315 2.13721 12.0685 2.13721 12.2015 "
    "2.16454C12.3516 2.19543 12.4934 2.27421 12.777 2.43177L20.177 6.54288C20.4766 "
    "6.70928 20.6263 6.79248 20.7354 6.91082C20.8318 7.01551 20.9049 7.13959 20.9495 "
    "7.27477C21 7.42757 21 7.59889 21 7.94153L21 12.5001M7.5 4.50008L16.5 9.50008M16 "
    '18.0001L18 20.0001L22 16.0001" stroke="#fff" stroke-width="2" stroke-linecap="round"'
    ' stroke-linejoin="round" fill="none"/>'
)
ICON_SHIELD = (
    '<path fill-rule="evenodd" clip-rule="evenodd" d="M12.4472 1.10557C12.1657 0.964809 '
    "11.8343 0.964809 11.5528 1.10557L3.55279 5.10557C3.214 5.27496 3 5.62123 3 6V12C3 "
    "14.6622 3.86054 16.8913 5.40294 18.7161C6.92926 20.5218 9.08471 21.8878 11.6214 "
    "22.9255C11.864 23.0248 12.136 23.0248 12.3786 22.9255C14.9153 21.8878 17.0707 "
    "20.5218 18.5971 18.7161C20.1395 16.8913 21 14.6622 21 12V6C21 5.62123 20.786 "
    "5.27496 20.4472 5.10557L12.4472 1.10557ZM5 12V6.61803L12 3.11803L19 6.61803V12C19 "
    "14.1925 18.305 15.9635 17.0696 17.425C15.8861 18.8252 14.1721 19.9803 12 "
    "20.9156C9.82786 19.9803 8.11391 18.8252 6.93039 17.425C5.69502 15.9635 5 14.1925 5 "
    "12ZM16.7572 9.65323C17.1179 9.23507 17.0714 8.60361 16.6532 8.24284C16.2351 7.88207 "
    "15.6036 7.9286 15.2428 8.34677L10.7627 13.5396L8.70022 11.5168C8.30592 11.1301 "
    "7.67279 11.1362 7.28607 11.5305C6.89935 11.9248 6.90549 12.5579 7.29978 "
    "12.9446L10.1233 15.7139C10.3206 15.9074 10.5891 16.0106 10.8651 15.9991C11.1412 "
    '15.9876 11.4002 15.8624 11.5807 15.6532L16.7572 9.65323Z" fill="#fff"/>'
)

STYLE = {
    "flat": dict(height=20, rx=3, gradient=FLAT_GRADIENT, size=110, y=140, shadow=True),
    "flat-square": dict(height=20, rx=0, gradient=None, size=110, y=140, shadow=True),
    "plastic": dict(height=20, rx=4, gradient=PLASTIC_GRADIENT, size=110, y=140, shadow=True),
    "for-the-badge": dict(height=28, rx=0, gradient=None, size=100, y=175, shadow=False),
}


def logo_svg(x: int, height: int, logo: str) -> str:
    side = 14 if height == 20 else 18
    y = (height - side) // 2
    return (
        f'<svg x="{x}" y="{y}" width="{side}" height="{side}" viewBox="0 0 512 512">'
        f"{logo}</svg>"
    )


def icon_svg(x: int, height: int, icon: str) -> str:
    side = 14 if height == 20 else 18
    y = (height - side) // 2
    return (
        f'<svg x="{x}" y="{y}" width="{side}" height="{side}" viewBox="0 0 24 24">'
        f"{icon}</svg>"
    )


def text_group(x10: int, tl: float, s: str, st: dict, bold: bool = False) -> str:
    y = st["y"]
    weight = ' font-weight="bold"' if bold else ""
    if st["shadow"]:
        return (
            f'<g transform="scale(.1)">'
            f'<g aria-hidden="true" fill="#010101">'
            f'<text x="{x10}" y="{y + 10}" fill-opacity=".8" filter="url(#blur)" textLength="{tl}">{s}</text>'
            f'<text x="{x10}" y="{y + 10}" fill-opacity=".3" textLength="{tl}">{s}</text>'
            f"</g>"
            f'<text x="{x10}" y="{y}" textLength="{tl}"{weight}>{s}</text>'
            f"</g>"
        )
    return f'<text transform="scale(.1)" x="{x10}" y="{y}" textLength="{tl}"{weight}>{s}</text>'


def chrome(width: int, height: int, st: dict, rects: str) -> tuple:
    defs = ""
    if st["shadow"]:
        defs += '<filter id="blur"><feGaussianBlur stdDeviation="16"/></filter>'
    if st["gradient"]:
        defs += (
            f'<linearGradient id="s" x2="0" y2="100%">{st["gradient"]}</linearGradient>'
        )
    body = rects
    if st["rx"]:
        defs += f'<clipPath id="r"><rect width="{width}" height="{height}" rx="{st["rx"]}"/></clipPath>'
        body = f'<g clip-path="url(#r)">{rects}</g>'
    if st["gradient"]:
        body += f'<rect width="{width}" height="{height}" fill="url(#s)"/>'
        if st["rx"]:  # gradient rect must sit inside the clip group
            body = (
                f'<g clip-path="url(#r)">{rects}'
                f'<rect width="{width}" height="{height}" fill="url(#s)"/></g>'
            )
    return defs, body


def two_segment(
    label: str, msg: str, color: str, style: str, logo: str, icon: str
) -> str:
    st = STYLE[style]
    size, h = st["size"], st["height"]
    upper = style == "for-the-badge"
    label_s, msg_s = (label.upper(), msg.upper()) if upper else (label, msg)
    pad = 12 if upper else 5  # shields for-the-badge: 12px side padding
    logo_side = 18 if upper else 14
    logo_gap = 5 if upper else 4

    label_tl = ftb_len(label_s) if upper else text_len(label_s, size)
    msg_tl = ftb_len(msg_s, bold=True) if upper else text_len(msg_s, size)

    label_w, msg_w = label_tl / 10.0, msg_tl / 10.0
    label_start = pad + logo_side + logo_gap
    left = math.ceil(label_start + label_w + pad)
    logo_x = left + pad
    text_start = logo_x + logo_side + logo_gap
    right = math.ceil((text_start - left) + msg_w + pad)
    width = left + right

    label_cx = round((label_start + label_w / 2) * 10)
    msg_cx = round((text_start + msg_w / 2) * 10)

    crisp = ' shape-rendering="crispEdges"' if upper else ""
    rects = (
        f"<g{crisp}>"
        f'<rect width="{left}" height="{h}" fill="#555"/>'
        f'<rect x="{left}" width="{right}" height="{h}" fill="{color}"/>'
        f"</g>"
    )
    defs, body = chrome(width, h, st, rects)

    aria = f"{label}: {msg}"
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{h}" role="img" aria-label="{aria}">'
        f"<title>{aria}</title>{defs}{body}"
        f'<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="{size}">'
        f"{icon_svg(pad, h, icon)}"
        f"{logo_svg(logo_x, h, logo)}"
        f"{text_group(label_cx, label_tl, label_s, st)}"
        f"{text_group(msg_cx, msg_tl, msg_s, st, bold=upper)}"
        f"</g></svg>\n"
    )


def single_segment(text: str, color: str, style: str, logo: str) -> str:
    st = STYLE[style]
    size, h = st["size"], st["height"]
    upper = style == "for-the-badge"
    s = text.upper() if upper else text
    pad = 12 if upper else 5
    logo_side = 18 if upper else 14
    logo_gap = 5 if upper else 4

    tl = ftb_len(s, bold=True) if upper else text_len(s, size)  # message role
    tw = tl / 10.0
    text_start = pad + logo_side + logo_gap
    width = math.ceil(text_start + tw + pad)
    cx = round((text_start + tw / 2) * 10)

    crisp = ' shape-rendering="crispEdges"' if upper else ""
    rects = f"<g{crisp}>" f'<rect width="{width}" height="{h}" fill="{color}"/>' f"</g>"
    defs, body = chrome(width, h, st, rects)

    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{h}" role="img" aria-label="{text}">'
        f"<title>{text}</title>{defs}{body}"
        f'<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="{size}">'
        f"{logo_svg(pad, h, logo)}"
        f"{text_group(cx, tl, s, st, bold=upper)}"
        f"</g></svg>\n"
    )


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Regenerate slsa-builder's shields-style static badge SVGs."
    )
    parser.add_argument(
        "--root",
        type=Path,
        default=Path(__file__).resolve().parents[4],
        help="repository root (default: resolved from script location)",
    )
    repo = parser.parse_args().root
    logo = extract_logo_inner(repo)

    types = {
        "built-with-slsa-builder": lambda st: two_segment(
            "built with", "slsa-builder", "#3f8f4f", st, logo, ICON_PACKAGE
        ),
        "verified-with-slsa-builder": lambda st: two_segment(
            "verified with", "slsa-builder", "#3f8f4f", st, logo, ICON_SHIELD
        ),
        "slsa-builder": lambda st: single_segment("slsa-builder", "#555", st, logo),
        "slsa-builder-green": lambda st: single_segment(
            "slsa-builder", "#3f8f4f", st, logo
        ),
    }

    out = repo / "assets/badges"
    out.mkdir(exist_ok=True)
    for base, make in types.items():
        for style in STYLE:
            name = f"{base}.svg" if style == "flat" else f"{base}-{style}.svg"
            svg = make(style)
            (out / name).write_text(svg)
            wm = re.search(r'width="(\d+)"', svg)
            hm = re.search(r'height="(\d+)"', svg)
            assert wm is not None and hm is not None
            print(f"wrote {name} ({wm.group(1)}x{hm.group(1)})")


if __name__ == "__main__":
    main()
