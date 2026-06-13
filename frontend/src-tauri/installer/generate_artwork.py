#!/usr/bin/env python3
"""
Generate the Sempa NSIS installer artwork (sidebar.bmp + header.bmp).

Everything is drawn from vector geometry (the brand "cradle" mark from
static/sempa-mark.svg) at a large supersample and downscaled with LANCZOS, so
the output is genuinely antialiased rather than an upscaled raster. The targets
are 4x the NSIS base sizes (welcome 164x314, header 150x57) so they stay crisp
on 4K / 200-400% DPI displays, where Tauri's NSIS template stretches the
bitmaps up to the DPI-scaled control size.

Run:  python3 generate_artwork.py
"""
import math
import os
from PIL import Image, ImageDraw, ImageFont

HERE = os.path.dirname(os.path.abspath(__file__))
FONT = os.path.join(
    HERE, "..", "..", "android", "app", "src", "main", "res", "font",
    "plus_jakarta_sans.ttf",
)

# ── Brand palette ───────────────────────────────────────────────────────────
CREAM = (247, 243, 235)
AMBER = (218, 159, 98)
GTOP  = (182, 97, 57)    # gradient top
GBOT  = (157, 74, 37)    # gradient bottom
DARK  = (43, 34, 27)     # #2b221b wordmark on light
TILE  = (179, 89, 46)    # #b3592e terracotta

SS = 4  # supersample factor


def font(px):
    return ImageFont.truetype(FONT, int(px))


def draw_word(draw, xy, text, px, color, anchor, weight=0.028):
    """Wordmark with a faux-medium weight (same-colour stroke) since the bundled
    variable font only exposes its ExtraLight default instance here."""
    draw.text(xy, text, font=font(px), fill=color, anchor=anchor,
              stroke_width=max(1, round(px * weight)), stroke_fill=color)


def draw_mark(draw, cx, cy, scale, color):
    """The sempa 'cradle' mark (lower semicircle + dot) from sempa-mark.svg,
    centred on (cx, cy). `scale` = pixels per SVG unit. The mark's content spans
    SVG y in [27.5, 72.5] (dot top to arc bottom incl. half stroke); we centre
    that band on cy. Strokes are stamped as overlapping discs for round caps and
    smooth joints."""
    def U(ux, uy):
        return (cx + (ux - 50) * scale, cy + (uy - 50) * scale)

    stroke_r = (9 / 2) * scale  # stroke-width 9, round caps
    # Arc: lower semicircle centred (50,40) r=28, theta 0..pi (sweeps downward).
    n = 480
    for i in range(n + 1):
        th = math.pi * i / n
        ux = 50 + 28 * math.cos(th)
        uy = 40 + 28 * math.sin(th)
        px_, py_ = U(ux, uy)
        draw.ellipse([px_ - stroke_r, py_ - stroke_r, px_ + stroke_r, py_ + stroke_r],
                     fill=color)
    # Dot: circle cx=50 cy=35 r=7.5
    dr = 7.5 * scale
    dx, dy = U(50, 35)
    draw.ellipse([dx - dr, dy - dr, dx + dr, dy + dr], fill=color)


def vgradient(w, h, top, bot):
    img = Image.new("RGB", (w, h))
    px = img.load()
    for y in range(h):
        t = y / max(1, h - 1)
        r = round(top[0] + (bot[0] - top[0]) * t)
        g = round(top[1] + (bot[1] - top[1]) * t)
        b = round(top[2] + (bot[2] - top[2]) * t)
        for x in range(w):
            px[x, y] = (r, g, b)
    return img


def build_sidebar():
    W, H = 164 * 4, 314 * 4          # 656 x 1256 (4x NSIS welcome bitmap)
    w, h = W * SS, H * SS
    img = vgradient(w, h, GTOP, GBOT)
    d = ImageDraw.Draw(img)

    # Mark — cream, upper third.
    mark_cy = h * 0.30
    draw_mark(d, w / 2, mark_cy, scale=3.6 * SS, color=CREAM)

    # Wordmark.
    draw_word(d, (w / 2, h * 0.50), "sempa", px=120 * SS, color=CREAM, anchor="mm",
              weight=0.03)

    # Bottom: thin amber rule + spaced tagline.
    rule_y = int(h * 0.86)
    rw = int(w * 0.30)
    d.rectangle([w / 2 - rw / 2, rule_y, w / 2 + rw / 2, rule_y + max(1, 2 * SS)],
                fill=AMBER)
    tag = ImageFont.truetype(FONT, int(20 * SS))
    label = "S E L F - H O S T E D"
    d.text((w / 2, rule_y + 34 * SS), label, font=tag, fill=AMBER, anchor="mm",
           stroke_width=max(1, round(20 * SS * 0.03)), stroke_fill=AMBER)

    img = img.resize((W, H), Image.LANCZOS)
    img.save(os.path.join(HERE, "sidebar.bmp"))
    img.save(os.path.join(HERE, "sidebar_preview.png"))
    print("sidebar.bmp", img.size)


def build_header():
    W, H = 150 * 4, 57 * 4           # 600 x 228 (4x NSIS header bitmap)
    w, h = W * SS, H * SS
    img = Image.new("RGB", (w, h), CREAM)
    d = ImageDraw.Draw(img)

    # Logo right-aligned (left side stays clear for NSIS page title text).
    pad = 28 * SS
    word_px = 64 * SS
    # Wordmark anchored to the right edge.
    draw_word(d, (w - pad, h / 2), "sempa", px=word_px, color=DARK, anchor="rm",
              weight=0.03)
    # Mark to the left of the wordmark.
    wbox = d.textbbox((w - pad, h / 2), "sempa", font=font(word_px), anchor="rm")
    mark_scale = 1.45 * SS
    mark_cx = wbox[0] - (28 * mark_scale) - 18 * SS
    draw_mark(d, mark_cx, h / 2, scale=mark_scale, color=TILE)

    img = img.resize((W, H), Image.LANCZOS)
    img.save(os.path.join(HERE, "header.bmp"))
    img.save(os.path.join(HERE, "header_preview.png"))
    print("header.bmp", img.size)


if __name__ == "__main__":
    build_sidebar()
    build_header()
