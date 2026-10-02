#!/usr/bin/env python3
"""Regenerate the iOS launch-screen logo images (LaunchLogo.imageset).  FIX-585.

The mosaic art MUST stay identical to `CoShellLogoView.art`
(mobile/ios/Runner/CoShellLogoView.swift). Rendering it here to PNG lets
LaunchScreen.storyboard show the same logo without duplicating the pixel math
in a second place.

Canvas: 12x12 cells, 6pt per cell => 72pt (same size as the settings-screen
logo). Colour: systemTeal dark-appearance value, matching the in-app logo on a
dark background.

Usage: python3 mobile/tools/launch_logo_gen.py
"""

import json
import os

from PIL import Image

# Must match CoShellLogoView.art exactly ("#" = filled cell).
ART = [
    "",
    "",
    "",
    "   #####",
    "  ########",
    " ##########",
    "## ## ## ###",
    "   ## ## #",
    " ##########",
    "  ########",
    "",
    "",
]

COLOR = (0x40, 0xC8, 0xE0, 255)  # systemTeal (dark) — visible on the dark launch screen
CELL_PT = 6  # 12 * 6pt = 72pt canvas

OUT_DIR = os.path.normpath(
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "ios", "Runner", "Assets.xcassets", "LaunchLogo.imageset")
)


def render(scale):
    """Render the mosaic at the given scale factor as an RGBA image."""
    cols = max(len(row) for row in ART)
    rows = len(ART)
    cell = CELL_PT * scale
    img = Image.new("RGBA", (cols * cell, rows * cell), (0, 0, 0, 0))
    px = img.load()
    for r, line in enumerate(ART):
        for c, ch in enumerate(line):
            if ch != "#":
                continue
            x0, y0 = c * cell, r * cell
            for y in range(y0, y0 + cell):
                for x in range(x0, x0 + cell):
                    px[x, y] = COLOR
    return img


def contents_json():
    """Build the Xcode imageset Contents.json for the three scales."""
    return {
        "images": [
            {"filename": "launch-logo.png", "idiom": "universal", "scale": "1x"},
            {"filename": "launch-logo@2x.png", "idiom": "universal", "scale": "2x"},
            {"filename": "launch-logo@3x.png", "idiom": "universal", "scale": "3x"},
        ],
        "info": {"author": "xcode", "version": 1},
    }


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    images = {1: "launch-logo.png", 2: "launch-logo@2x.png", 3: "launch-logo@3x.png"}
    for scale, name in images.items():
        img = render(scale)
        path = os.path.join(OUT_DIR, name)
        img.save(path)
        print("wrote %s %dx%d" % (path, img.size[0], img.size[1]))
    with open(os.path.join(OUT_DIR, "Contents.json"), "w", encoding="utf-8") as fh:
        json.dump(contents_json(), fh, indent=2)
        fh.write("\n")
    print("wrote %s" % os.path.join(OUT_DIR, "Contents.json"))


if __name__ == "__main__":
    main()
