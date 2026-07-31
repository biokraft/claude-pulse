#!/usr/bin/env python3
"""Build the Connect IQ store upload folder from simulator screenshots.

Run scripts/shoot-pages.sh first — this turns its raw window captures into the
listing assets (screen images + hero) and gathers the static ones alongside them.

    PYTHON=/path/to/venv/bin/python scripts/shoot-pages.sh .screenshots
    /path/to/venv/bin/python scripts/build-store-assets.py .screenshots ~/Desktop/ClaudePulse-store

Needs pillow in the same venv as pyobjc (see CLAUDE.md).
"""
import os
import shutil
import sys

from PIL import Image, ImageDraw, ImageFont

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
STATIC = os.path.join(
    REPO,
    "design",
    "store-assets",
)

# Page captures -> store screen image names, in listing order.
PAGES = [
    ("page-1-rings", "screen-1-rings"),
    ("page-2-detail", "screen-2-detail"),
    ("page-3-cost", "screen-3-cost"),
]

# Where the watch display sits inside a shoot-pages.sh window capture, as a
# fraction of the window. Found empirically; re-check if the simulator window
# size changes, since too small a crop clips text at the display edge.
DISPLAY_CX = 0.5
DISPLAY_CY = 0.492
DISPLAY_SIDE = 0.53

SCREEN_PX = 416
BG = (20, 19, 18)
TEXT = (247, 245, 242)
MUTED = (169, 163, 153)
EDGE = (58, 55, 52)


def font(size):
    for path in ["/System/Library/Fonts/Helvetica.ttc", "/Library/Fonts/Arial.ttf"]:
        if os.path.exists(path):
            try:
                return ImageFont.truetype(path, size)
            except OSError:
                pass
    return ImageFont.load_default()


def circle_mask(size):
    mask = Image.new("L", (size, size), 0)
    ImageDraw.Draw(mask).ellipse((0, 0, size - 1, size - 1), fill=255)
    return mask


def crop_display(shot_path):
    """Cut the round watch display out of a simulator window capture."""
    im = Image.open(shot_path).convert("RGB")
    w, h = im.size
    cx, cy = int(w * DISPLAY_CX), int(h * DISPLAY_CY)
    side = int(w * DISPLAY_SIDE)
    crop = im.crop((cx - side // 2, cy - side // 2, cx + side // 2, cy + side // 2))
    crop = crop.resize((SCREEN_PX, SCREEN_PX), Image.LANCZOS)
    # Mask to a circle so the simulator's bezel corners never reach the listing.
    out = Image.new("RGB", (SCREEN_PX, SCREEN_PX), (0, 0, 0))
    out.paste(crop, (0, 0), circle_mask(SCREEN_PX))
    return out


def build_hero(out_dir):
    """1440x720 banner: the three pages as watch discs under the app name."""
    w, h = 1440, 720
    im = Image.new("RGB", (w, h), BG)
    draw = ImageDraw.Draw(im)

    discs = [("screen-1-rings", 340), ("screen-2-detail", 440), ("screen-3-cost", 340)]
    gap = 64
    x = (w - (sum(s for _, s in discs) + gap * (len(discs) - 1))) // 2
    cy = int(h * 0.56)
    for name, size in discs:
        face = Image.open(os.path.join(out_dir, name + ".png")).convert("RGB")
        face = face.resize((size, size), Image.LANCZOS)
        im.paste(face, (x, cy - size // 2), circle_mask(size))
        draw.ellipse((x, cy - size // 2, x + size - 1, cy + size // 2 - 1), outline=EDGE, width=4)
        x += size + gap

    for text, f, y, color in [
        ("Claude Pulse", font(66), 74, TEXT),
        ("Your Claude Code usage, on your wrist", font(31), 158, MUTED),
    ]:
        draw.text(((w - draw.textlength(text, font=f)) / 2, y), text, font=f, fill=color)

    im.save(os.path.join(out_dir, "hero-1440x720.png"))


def main():
    shots = sys.argv[1] if len(sys.argv) > 1 else os.path.join(REPO, ".screenshots")
    out_dir = sys.argv[2] if len(sys.argv) > 2 else os.path.expanduser("~/Desktop/ClaudePulse-store")
    os.makedirs(out_dir, exist_ok=True)

    for shot, name in PAGES:
        crop_display(os.path.join(shots, shot + ".png")).save(os.path.join(out_dir, name + ".png"))
    build_hero(out_dir)

    for name in ["cover-500.png", "icon-128-24bit.png", "icon-128-64color.png"]:
        shutil.copy(os.path.join(STATIC, name), os.path.join(out_dir, name))

    iq = os.path.expanduser("~/Desktop/ClaudePulse.iq")
    if os.path.exists(iq):
        shutil.copy(iq, out_dir)

    for name in sorted(os.listdir(out_dir)):
        size_kb = os.path.getsize(os.path.join(out_dir, name)) // 1024
        print(f"{name:26} {size_kb:>6} KB")


if __name__ == "__main__":
    main()
