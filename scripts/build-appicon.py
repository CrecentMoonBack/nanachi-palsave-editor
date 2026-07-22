#!/usr/bin/env python3
"""Builds the application icon from the Nanachi portrait.

Wails wants two things: build/appicon.png, which it uses for the window and
for platforms that take a PNG, and build/windows/icon.ico for the executable
and the taskbar.

The portrait is a 512px RGBA square with a transparent background. An .ico
holding several sizes lets Windows pick per context — 16px in the title bar,
256px in the "large icons" view — instead of scaling one bitmap badly.

Run from the repo root:  python scripts/build-appicon.py
"""

from pathlib import Path
from PIL import Image

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "frontend" / "src" / "assets" / "nanachi_face.png"
PNG_OUT = ROOT / "build" / "appicon.png"
ICO_OUT = ROOT / "build" / "windows" / "icon.ico"

# Windows picks the nearest size from the .ico rather than resampling, so the
# small ones are worth shipping even though they are cheap.
ICO_SIZES = [(n, n) for n in (16, 24, 32, 48, 64, 128, 256)]

# Wails' appicon is documented as 512 or 1024 square; 1024 keeps it sharp on
# a HiDPI display.
PNG_SIZE = 1024


def main() -> None:
    if not SRC.exists():
        raise SystemExit(f"missing source portrait: {SRC}")

    src = Image.open(SRC).convert("RGBA")
    if src.width != src.height:
        raise SystemExit(f"portrait must be square, got {src.size}")

    PNG_OUT.parent.mkdir(parents=True, exist_ok=True)
    ICO_OUT.parent.mkdir(parents=True, exist_ok=True)

    src.resize((PNG_SIZE, PNG_SIZE), Image.LANCZOS).save(PNG_OUT)
    print(f"{PNG_OUT.relative_to(ROOT)}  {PNG_SIZE}x{PNG_SIZE}")

    # Pillow builds every requested size into the one file.
    src.save(ICO_OUT, format="ICO", sizes=ICO_SIZES)
    print(f"{ICO_OUT.relative_to(ROOT)}  {', '.join(str(w) for w, _ in ICO_SIZES)}")


if __name__ == "__main__":
    main()
