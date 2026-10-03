#!/usr/bin/env python3
"""Regenerate the iOS app icon: remove the CRT side bands, add edge lighting.

FIX-585. The 1024x1024 master artwork (dark squircle + pixel-invader + ">_"
prompt) keeps its design; only the artefacts that do not match the current Apple
icon look are changed:

  * the two thick vertical light bands hugging the left/right inner rim are
    inpainted by horizontal interpolation between the neighbouring clean
    background columns;
  * a specular highlight is painted along the squircle's top edge, plus a weaker
    one along the bottom edge (~50% of the top);
  * the transparent margin around the squircle is flattened onto the artwork's
    own edge colour, because App Store icons must not contain an alpha channel;
  * every other size in AppIcon.appiconset is resampled from the edited master.

Usage:
  python3 mobile/tools/appicon_regen.py --inspect   # print detected geometry
  python3 mobile/tools/appicon_regen.py             # edit master + rebuild sizes
"""

import argparse
import json
import os
import sys

from PIL import Image

HERE = os.path.dirname(os.path.abspath(__file__))
ICONSET = os.path.normpath(
    os.path.join(HERE, "..", "ios", "Runner", "Assets.xcassets", "AppIcon.appiconset")
)
MASTER = os.path.join(ICONSET, "Icon-App-1024x1024@1x.png")

SIDE_ZONE = 0.08  # only the outer 8% of the width is searched for side bands
BAND_REF_OFFSET = 48  # px: baseline column used to judge a band column
BAND_MIN_DELTA = 12  # luminance points above the baseline => band column
GLOW_DEPTH = 20  # px: how far the edge highlight reaches inward
GLOW_CORE = 3  # px: fully saturated part of the highlight
GLOW_COLOR = (0x7A, 0xF0, 0xA8)  # light green highlight colour
GLOW_TOP_STRENGTH = 0.30  # blend factor of the highlight core (top edge)
GLOW_BOTTOM_RATIO = 0.50  # bottom edge is weaker than the top edge
REF_ROWS = 4  # clean rows blended into the artwork depth profile
REF_DEPTH = 1  # px inside the silhouette from which artwork is solid


def luma(px):
    """Rec.601 luminance of an RGB triple."""
    r, g, b = px
    return (r * 299 + g * 587 + b * 114) // 1000


def load_master():
    """Open the 1024 master as RGBA (the current artwork carries an alpha margin)."""
    return Image.open(MASTER).convert("RGBA")


def is_canvas(px):
    """True for the white canvas margin around the artwork (near-white pixel)."""
    r, g, b = px[:3]
    return r > 230 and g > 230 and b > 230


def canvas_contours(img):
    """Return (top, bottom) of the dark artwork per column (white canvas baseline)."""
    w, h = img.size
    px = img.load()
    top = [None] * w
    bottom = [None] * w
    for x in range(w):
        for y in range(h):
            if not is_canvas(px[x, y]):
                top[x] = y
                break
        for y in range(h - 1, -1, -1):
            if not is_canvas(px[x, y]):
                bottom[x] = y
                break
    return top, bottom


def col_luma_profile(img):
    """Mean luminance of every column, measured over the artwork's middle band."""
    w, h = img.size
    px = img.load()
    y0, y1 = int(h * 0.20), int(h * 0.80)
    rows = y1 - y0
    profile = []
    for x in range(w):
        total = 0
        for y in range(y0, y1):
            total += luma(px[x, y][:3])
        profile.append(total / rows)
    return profile


def detect_bands(profile):
    """Return [(x0, x1)] side-band ranges inside the outer SIDE_ZONE of each edge."""
    n = len(profile)
    zone = int(n * SIDE_ZONE)
    bands = []
    for x in range(n):
        if x >= zone and x < n - zone:
            continue
        ref_x = min(n - 1, x + BAND_REF_OFFSET) if x < n // 2 else max(0, x - BAND_REF_OFFSET)
        if abs(x - ref_x) < 8:
            continue
        if profile[x] - profile[ref_x] < BAND_MIN_DELTA:
            continue
        if bands and x == bands[-1][1] + 1:
            bands[-1][1] = x
        else:
            bands.append([x, x])
    return [(x0, x1) for x0, x1 in bands]


def silhouette_edges(top, bottom, w, h):
    """Per-row artwork boundary: leftmost/rightmost column spanning that row."""
    left = [w] * h
    right = [-1] * h
    for x in range(w):
        if top[x] is None or bottom[x] is None:
            continue
        for y in range(top[x], bottom[x] + 1):
            if x < left[y]:
                left[y] = x
            if x > right[y]:
                right[y] = x
    return left, right


def median_colour(samples):
    """Per-channel median of a list of RGBA pixels."""
    out = []
    for i in range(4):
        vals = sorted(sample[i] for sample in samples)
        out.append(vals[len(vals) // 2])
    return tuple(out)


def depth_profile(px, edge, clean_rows, y, right_side, depth_max, alpha_fill):
    """Artwork colour at every depth inside the silhouette, for one band row.

    Sampled from the nearest clean rows (the ones whose two flanking columns
    both belong to the artwork) so the rounded corner keeps the rim look of the
    straight edge instead of a canvas-to-artwork ramp.
    """
    refs = sorted(clean_rows, key=lambda row: abs(row - y))[:REF_ROWS]
    profile = []
    for d in range(depth_max + 1):
        samples = []
        for row in refs:
            x = edge[row] - d if right_side else edge[row] + d
            if 0 <= x < len(edge):
                samples.append(px[x, row])
        profile.append(median_colour(samples) if samples else alpha_fill)
    return profile


def blend_band_row(px, x0, x1, left, right, y):
    """Blend a band row between its two clean neighbours (outside the corners)."""
    lr, lg, lb, la = px[left, y]
    rr, rg, rb, ra = px[right, y]
    span = right - left
    for x in range(x0, x1 + 1):
        t = (x - left) / span if span else 0.0
        px[x, y] = (
            int(round(lr + (rr - lr) * t)),
            int(round(lg + (rg - lg) * t)),
            int(round(lb + (rb - lb) * t)),
            la if la == ra else int(round(la + (ra - la) * t)),
        )


def remove_bands(img, bands, top, bottom):
    """Inpaint each vertical band, keeping the squircle silhouette intact.

    Both flanking columns are canvas for the rows above a rounded corner and
    both are artwork below it, so a horizontal blend is invisible there. Where
    the corner arc passes through the band, a flank lands on (or just outside)
    the anti-aliased silhouette: blending canvas with artwork smears a grey ramp
    across the edge. Those rows follow the silhouette geometry instead - canvas
    outside, artwork at the matching depth inside.

    Depths are measured from the silhouette: depth 0 is the anti-aliased rim
    pixel itself, so only depth >= REF_DEPTH counts as solid artwork.
    """
    px = img.load()
    w, h = img.size
    left_edge, right_edge = silhouette_edges(top, bottom, w, h)
    for x0, x1 in bands:
        left = max(0, x0 - 1)
        right = min(w - 1, x1 + 1)
        right_side = x0 > w // 2
        edge = right_edge if right_side else left_edge
        sign = 1 if right_side else -1
        depths = [((edge[y] - left) * sign, (edge[y] - right) * sign)
                  for y in range(h)]
        # near = the flank facing the silhouette, far = the opposite one
        near = [d[1] if right_side else d[0] for d in depths]
        far = [d[0] if right_side else d[1] for d in depths]
        clean = [y for y in range(h) if near[y] >= REF_DEPTH]
        width = x1 - x0 + 1
        for y in range(h):
            if not clean or near[y] >= REF_DEPTH or far[y] < 0:
                blend_band_row(px, x0, x1, left, right, y)
                continue
            # canvas colour, taken from the flank on the canvas side
            canvas = px[right, y] if right_side else px[left, y]
            profile = depth_profile(px, edge, clean, y, right_side, width, canvas)
            for x in range(x0, x1 + 1):
                d = (edge[y] - x) * sign
                if d < 0:
                    px[x, y] = canvas
                elif d > 0:
                    px[x, y] = profile[d]
                # d == 0 is the silhouette's own anti-aliased rim pixel: keep it    return img


def flatten_alpha(img):
    """Fill the transparent margin with the artwork's own edge colour (no alpha)."""
    w, h = img.size
    px = img.load()
    ring = []
    for x in range(w):
        for y in range(h):
            if px[x, y][3] == 0:
                for dy, dx in ((-1, 0), (1, 0), (0, -1), (0, 1)):
                    nx, ny = x + dx, y + dy
                    if 0 <= nx < w and 0 <= ny < h and px[nx, ny][3] > 0:
                        ring.append(px[nx, ny][:3])
                        break
    if not ring:
        return img.convert("RGB")
    ring.sort(key=luma)
    fill = ring[len(ring) // 2]
    for x in range(w):
        for y in range(h):
            if px[x, y][3] < 255:
                px[x, y] = (fill[0], fill[1], fill[2], 255)
    return img.convert("RGB")


def add_edge_light(img, top, bottom):
    """Paint a specular highlight along the top edge and a weaker one at the bottom."""
    px = img.load()
    h = img.size[1]

    def paint(contour, inward, strength):
        for x, y_edge in enumerate(contour):
            if y_edge is None:
                continue
            for d in range(GLOW_DEPTH):
                y = y_edge + inward * d
                if y < 0 or y >= h:
                    continue
                if d < GLOW_CORE:
                    weight = 1.0
                else:
                    weight = 1.0 - (d - GLOW_CORE) / float(GLOW_DEPTH - GLOW_CORE)
                weight *= strength
                r, g, b = px[x, y][:3]
                px[x, y] = (
                    int(round(r + (GLOW_COLOR[0] - r) * weight)),
                    int(round(g + (GLOW_COLOR[1] - g) * weight)),
                    int(round(b + (GLOW_COLOR[2] - b) * weight)),
                    255,
                )

    paint(top, 1, GLOW_TOP_STRENGTH)
    paint(bottom, -1, GLOW_TOP_STRENGTH * GLOW_BOTTOM_RATIO)
    return img


def band_delta(img, bands):
    """Mean |band - clean neighbour| luminance difference at the mid rows."""
    if not bands:
        return 0.0
    px = img.load()
    w, h = img.size
    deltas = []
    for x0, x1 in bands:
        ref_x = min(w - 1, x1 + 20) if x1 < w // 2 else max(0, x0 - 20)
        for x in range(x0, x1 + 1):
            for y in range(int(h * 0.25), int(h * 0.75), 4):
                deltas.append(abs(luma(px[x, y][:3]) - luma(px[ref_x, y][:3])))
    return sum(deltas) / len(deltas) if deltas else 0.0


def ring_delta(before, after, contour, inward):
    """Mean luminance gain inside the 0..GLOW_CORE px ring along a contour."""
    pb, pa = before.load(), after.load()
    h = before.size[1]
    gains = []
    for x, y_edge in enumerate(contour):
        if y_edge is None:
            continue
        for d in range(GLOW_CORE + 1):
            y = y_edge + inward * d
            if y < 0 or y >= h:
                continue
            gains.append(luma(pa[x, y][:3]) - luma(pb[x, y][:3]))
    return sum(gains) / len(gains) if gains else 0.0


def foreground_delta(before, after):
    """Mean absolute change over the bright green artwork (must stay untouched)."""
    pb, pa = before.load(), after.load()
    w, h = before.size
    diffs = []
    for y in range(h):
        for x in range(w):
            r, g, b = pb[x, y][:3]
            if g > 150 and g - r > 40 and g - b > 40:
                ar, ag, ab = pa[x, y][:3]
                diffs.append((abs(ar - r) + abs(ag - g) + abs(ab - b)) / 3.0)
    return (sum(diffs) / len(diffs) if diffs else 0.0), len(diffs)


def appiconset_targets():
    """Read AppIcon.appiconset/Contents.json and return [(filename, px)] to rebuild."""
    with open(os.path.join(ICONSET, "Contents.json"), "r", encoding="utf-8") as fh:
        contents = json.load(fh)
    targets = []
    for entry in contents.get("images", []):
        name = entry.get("filename")
        if not name:
            continue
        scale = int(entry.get("scale", "1x").rstrip("x"))
        pt = float(entry.get("size", "0x0").split("x")[0])
        targets.append((name, int(round(pt * scale))))
    return targets


def rebuild_sizes(master):
    """Resample every icon size from the edited master (Contents.json untouched)."""
    written = []
    for name, px in appiconset_targets():
        path = os.path.join(ICONSET, name)
        master.resize((px, px), Image.LANCZOS).convert("RGB").save(path)
        written.append((name, px))
    return written


def describe():
    """Print the detected geometry so the edit can be reviewed before it is applied."""
    raw = Image.open(MASTER)
    print("raw: mode=%s size=%s info=%s" % (raw.mode, raw.size, sorted(raw.info.keys())))
    print("raw px(2,2)=%s px(511,511)=%s" % (raw.getpixel((2, 2)), raw.getpixel((511, 511))))
    img = load_master()
    print("rgba px(2,2)=%s px(511,511)=%s" % (img.getpixel((2, 2)), img.getpixel((511, 511))))
    top, bottom = canvas_contours(img)
    known_top = [y for y in top if y is not None]
    known_bottom = [y for y in bottom if y is not None]
    profile = col_luma_profile(img)
    bands = detect_bands(profile)
    print("master: %s %s (alpha present)" % (MASTER, img.size))
    print("opaque columns: %d..%d" % (min(known_top) is not None and 0 or 0,
                                      len([y for y in top if y is not None]) - 1))
    print("top contour:    y=%d..%d, mid=%s" % (min(known_top), max(known_top), top[img.size[0] // 2]))
    print("bottom contour: y=%d..%d, mid=%s" % (min(known_bottom), max(known_bottom), bottom[img.size[0] // 2]))
    print("column luma: min=%.1f max=%.1f" % (min(profile), max(profile)))
    print("detected side bands: %s" % (bands,))
    for x0, x1 in bands:
        ref_x = min(img.size[0] - 1, x1 + 20) if x1 < img.size[0] // 2 else max(0, x0 - 20)
        inside = sum(profile[x0:x1 + 1]) / (x1 - x0 + 1)
        print("  x=%4d..%4d w=%2d inside=%.1f ref(x=%d)=%.1f delta=%+.1f"
              % (x0, x1, x1 - x0 + 1, inside, ref_x, profile[ref_x], inside - profile[ref_x]))
    fg_delta, fg_px = foreground_delta(img, img)
    print("artwork pixels (bright green): %d" % fg_px)


def probe(steps=32):
    """Print a coarse luminance map so the canvas/shape structure is visible."""
    raw = Image.open(MASTER).convert("RGB")
    w, h = raw.size
    small = raw.resize((steps, steps), Image.BOX)
    px = small.load()
    for y in range(steps):
        print(" ".join("%02X" % luma(px[x, y]) for x in range(steps)))
    print("corners: %s %s %s %s"
          % (raw.getpixel((2, 2)), raw.getpixel((w - 3, 2)), raw.getpixel((2, h - 3)), raw.getpixel((w - 3, h - 3))))


def main(argv=None):
    parser = argparse.ArgumentParser(description="Rebuild the iOS app icon (FIX-585)")
    parser.add_argument("--inspect", action="store_true", help="only print the detected geometry")
    parser.add_argument("--probe", action="store_true", help="print a coarse luminance map of the master")
    parser.add_argument("--keep-master", action="store_true", help="edit sizes only, keep the master file")
    args = parser.parse_args(argv)

    if args.inspect:
        describe()
        return 0
    if args.probe:
        probe()
        return 0

    before = load_master()
    master = before.copy()

    top, bottom = canvas_contours(master)
    bands = detect_bands(col_luma_profile(master))
    if not bands:
        print("warning: no side bands detected — nothing to remove", file=sys.stderr)
    delta_before = band_delta(master, bands)
    remove_bands(master, bands, top, bottom)
    flat = flatten_alpha(master)
    add_edge_light(flat, top, bottom)

    delta_after = band_delta(flat, bands)
    top_gain = ring_delta(before, flat, top, 1)
    bottom_gain = ring_delta(before, flat, bottom, -1)
    ratio = (top_gain / bottom_gain) if bottom_gain > 0 else float("inf")
    fg_delta, fg_px = foreground_delta(before, flat)

    print("side bands removed : %s" % (bands,))
    print("side-band delta    : before=%.2f after=%.2f (target <= 3)" % (delta_before, delta_after))
    print("edge light         : top=%.2f bottom=%.2f ratio=%.2f (target > 1.5)"
          % (top_gain, bottom_gain, ratio))
    print("artwork unchanged  : mean=%.2f over %d px (target <= 2)" % (fg_delta, fg_px))
    print("output mode        : %s (no alpha)" % flat.mode)

    if not args.keep_master:
        flat.save(MASTER)
        print("wrote %s" % MASTER)
    for name, px in rebuild_sizes(flat):
        print("wrote %s (%dx%d)" % (name, px, px))

    ok = delta_after <= 3.0 and bottom_gain > 0 and ratio >= 1.5 and fg_delta <= 2.0 and flat.mode == "RGB"
    print("RESULT: %s" % ("OK" if ok else "CHECK"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
