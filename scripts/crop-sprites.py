#!/usr/bin/env python3
"""Crop the mascot drawables down to the pixels that actually contain the mascot.

The exported art is 96x96 with the character bottom-anchored and tiny — 57-69% of every
sprite is transparent space above it. Drawn as-is, a 72px box renders a ~17px mascot and
reserves 55px of nothing above it, which also drags any centred layout downward.

Every sprite is cropped to the *union* of all their content boxes, not to its own. A shared
crop keeps the relative size and baseline consistent between poses, so the mascot doesn't
jump around when the pose changes. Re-run this after adding a new pose.

    python3 scripts/crop-sprites.py [--dry-run]
"""
import glob
import os
import sys

from PIL import Image

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SPRITES = os.path.join(REPO, "watch", "resources", "drawables", "clawd_*.png")


def main():
    dry_run = "--dry-run" in sys.argv
    paths = sorted(glob.glob(SPRITES))
    if not paths:
        print("no sprites found", file=sys.stderr)
        return 1

    boxes = {}
    for path in paths:
        with Image.open(path) as im:
            box = im.convert("RGBA").getbbox()
        if box is None:
            print(f"{os.path.basename(path)}: fully transparent, skipping", file=sys.stderr)
            continue
        boxes[path] = box

    union = (
        min(b[0] for b in boxes.values()),
        min(b[1] for b in boxes.values()),
        max(b[2] for b in boxes.values()),
        max(b[3] for b in boxes.values()),
    )
    print(f"union content box: {union}  ->  {union[2] - union[0]}x{union[3] - union[1]}")

    for path in boxes:
        with Image.open(path) as im:
            before = im.size
            cropped = im.convert("RGBA").crop(union)
        if not dry_run:
            cropped.save(path)
        print(f"  {os.path.basename(path):30} {before[0]}x{before[1]} -> {cropped.size[0]}x{cropped.size[1]}")

    if dry_run:
        print("dry run: nothing written")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
