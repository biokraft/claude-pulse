#!/usr/bin/env python3
"""Print the CoreGraphics window id of the Connect IQ simulator window.

Needs pyobjc-framework-Quartz. Used by shoot-pages.sh so `screencapture -l`
can grab the simulator window directly — it works even when the window sits
on another display or behind other windows, which `-R` region capture does not.
"""
import sys

import Quartz


def main() -> int:
    windows = Quartz.CGWindowListCopyWindowInfo(
        Quartz.kCGWindowListOptionAll, Quartz.kCGNullWindowID
    )
    for w in windows:
        name = w.get("kCGWindowName") or ""
        if name.startswith("CIQ Simulator"):
            print(w.get("kCGWindowNumber"))
            return 0
    print("simulator window not found", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
