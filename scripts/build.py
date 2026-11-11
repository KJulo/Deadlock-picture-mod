#!/usr/bin/env python3
"""Build pak09_dir.vpk replacing the soul_container color texture.

The output vtex_c = original 2132-byte resource header (vendor/original) +
10 BC7 mip levels (2048 -> 4), stored smallest mip first, which is the layout
the original file uses (verified: its last 4 MiB decode exactly to mip 0).

Alpha: the original texture's alpha is effectively constant (254-255, unused).
If the input PNG has a non-opaque alpha channel it is used; otherwise the
original alpha is preserved.
"""
import argparse
import shutil
import sys
from pathlib import Path

import etcpak
import numpy as np
import texture2ddecoder
import vpk
from PIL import Image

ROOT = Path(__file__).resolve().parent.parent
GAME_PATH = "models/props_gameplay/soul_container/materials/soul_container_color_png_1da67bfe.vtex_c"
ORIGINAL = ROOT / "vendor/original/soul_container_color_png_1da67bfe.vtex_c"
HEADER_SIZE = 2132
SIZE = 2048
NUM_MIPS = 10  # 2048 .. 4


def original_alpha() -> np.ndarray:
    data = ORIGINAL.read_bytes()
    mip0 = data[-SIZE * SIZE:]  # BC7 = 1 byte/pixel; largest mip is last
    bgra = np.frombuffer(texture2ddecoder.decode_bc7(mip0, SIZE, SIZE), np.uint8)
    return bgra.reshape(SIZE, SIZE, 4)[:, :, 3].copy()


def load_input(path: Path) -> Image.Image:
    img = Image.open(path)
    has_alpha = "A" in img.getbands()
    img = img.convert("RGBA")
    if img.size != (SIZE, SIZE):
        print(f"warning: resizing {img.size} -> {SIZE}x{SIZE}", file=sys.stderr)
        img = img.resize((SIZE, SIZE), Image.LANCZOS)
    arr = np.array(img)
    if not has_alpha or arr[:, :, 3].min() == 255:
        arr[:, :, 3] = original_alpha()
    return Image.fromarray(arr, "RGBA")


def encode_bc7(img: Image.Image) -> bytes:
    # etcpak expects RGBA byte order (verified by round-trip).
    return etcpak.compress_bc7(np.ascontiguousarray(np.array(img)).tobytes(), img.width, img.height)


def build_vtex(img: Image.Image) -> bytes:
    header = ORIGINAL.read_bytes()[:HEADER_SIZE]
    mips = []
    for level in range(NUM_MIPS):
        s = SIZE >> level
        mip = img if level == 0 else img.resize((s, s), Image.LANCZOS)
        enc = encode_bc7(mip)
        assert len(enc) == s * s, (level, len(enc))
        mips.append(enc)
    out = header + b"".join(reversed(mips))  # smallest first
    assert len(out) == ORIGINAL.stat().st_size
    return out


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("png", nargs="?", default=str(ROOT / "assets/soul_container_color.png"))
    ap.add_argument("-o", "--output", default=str(ROOT / "build/pak09_dir.vpk"))
    args = ap.parse_args()

    vtex = build_vtex(load_input(Path(args.png)))
    build = ROOT / "build"
    staging = build / "staging"
    shutil.rmtree(staging, ignore_errors=True)
    target = staging / GAME_PATH
    target.parent.mkdir(parents=True)
    target.write_bytes(vtex)
    vpk.new(str(staging)).save(args.output)
    print(f"wrote {args.output}")


if __name__ == "__main__":
    main()
