#!/usr/bin/env python3
"""Regenerate app/macos/AppIcon.icns from the co-shell 1024 master artwork.

The macOS shell (co-shell.app) needs an .icns; the artwork itself lives with the
iOS app, where it is delivered as a full-bleed 1024 square (no alpha, no baked
rounded corners — iOS masks icons itself). macOS does not mask app icons, so this
script re-lays the artwork onto Apple's macOS icon grid and bakes the rounded
shape in:

    * artwork scaled to 824x824 (Apple's macOS grid) and centred on 1024x1024
    * continuous rounded corners with radius 185.4 (≈22.5% of 824), antialiased
    * everything outside the rounded shape stays fully transparent

It then writes the ten .iconset members macOS expects and converts them with
`iconutil`. The resulting AppIcon.icns is committed, so the app build itself
needs neither Python nor PIL.

Usage: python3 app/macos/make-appicon.py
"""
import os
import shutil
import subprocess
import sys
import tempfile

from PIL import Image, ImageDraw

SIZE = 1024                     # master canvas
CONTENT = 824                   # artwork size inside the macOS icon grid
RADIUS = 185.4                  # ≈22.5% of CONTENT (macOS corner radius)
SUPERSAMPLE = 4                 # antialiasing factor for the mask

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
ARTWORK = os.path.join(
    REPO_ROOT, "mobile/ios/Runner/Assets.xcassets/AppIcon.appiconset/Icon-App-1024x1024@1x.png"
)
OUTPUT = os.path.join(HERE, "AppIcon.icns")

ICONSET = [
    ("icon_16x16.png", 16),
    ("icon_16x16@2x.png", 32),
    ("icon_32x32.png", 32),
    ("icon_32x32@2x.png", 64),
    ("icon_128x128.png", 128),
    ("icon_128x128@2x.png", 256),
    ("icon_256x256.png", 256),
    ("icon_256x256@2x.png", 512),
    ("icon_512x512.png", 512),
    ("icon_512x512@2x.png", 1024),
]


def rounded_mask():
    """Antialiased rounded-rect mask on the macOS icon grid."""
    big = Image.new("L", (SIZE * SUPERSAMPLE, SIZE * SUPERSAMPLE), 0)
    draw = ImageDraw.Draw(big)
    inset = (SIZE - CONTENT) * SUPERSAMPLE // 2
    draw.rounded_rectangle(
        [inset, inset, inset + CONTENT * SUPERSAMPLE, inset + CONTENT * SUPERSAMPLE],
        radius=RADIUS * SUPERSAMPLE,
        fill=255,
    )
    return big.resize((SIZE, SIZE), Image.LANCZOS)


def master():
    art = Image.open(ARTWORK).convert("RGB").resize((CONTENT, CONTENT), Image.LANCZOS)
    canvas = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    offset = (SIZE - CONTENT) // 2
    canvas.paste(art, (offset, offset))
    canvas.putalpha(rounded_mask())
    return canvas


def main():
    if not os.path.exists(ARTWORK):
        print("error: artwork not found: %s" % ARTWORK, file=sys.stderr)
        return 1

    workdir = tempfile.mkdtemp(prefix="co-shell-iconset-")
    try:
        iconset = os.path.join(workdir, "AppIcon.iconset")
        os.makedirs(iconset)
        icon = master()
        for name, size in ICONSET:
            icon.resize((size, size), Image.LANCZOS).save(os.path.join(iconset, name))
        subprocess.run(["iconutil", "-c", "icns", iconset, "-o", OUTPUT], check=True)
    finally:
        shutil.rmtree(workdir, ignore_errors=True)

    print("wrote %s (%d bytes)" % (OUTPUT, os.path.getsize(OUTPUT)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
