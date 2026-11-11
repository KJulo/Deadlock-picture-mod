#!/usr/bin/env python3
"""Compose the cat photo into the soul_container color texture (2048x2048 UV atlas).

UV regions (pixel coords in the 2048 atlas, found by rendering colour-coded debug textures):
  - DISC   : circle centre (400, 840) r~232 -> the flat discs on top AND bottom of the container
  - FRAME  : right big panel (920..2048, 500..1640) -> ornate square frame on each of the 4 side faces
  - TRIS   : left big panel (0..1060, 1000..2048) -> triangle emblems / inner pieces on the side faces

Variants:
  discs : cat face only on the top/bottom discs, rest of the texture untouched
  full  : discs + the photo also painted over the side frames and emblems

usage: compose.py [--base ORIGINAL.png] [--photo assets/cat.jpg] [--out assets/soul_container_color.png]
                  [--variant discs|full]
Without --base, the original texture is decoded straight from vendor/original/*.vtex_c.
"""
import argparse
from pathlib import Path

import numpy as np
import texture2ddecoder
from PIL import Image, ImageDraw, ImageFilter, ImageOps, ImageEnhance

SIZE = 2048
DISC = (400, 840, 232)
FRAME_BOX = (920, 500, 2048, 1640)
TRIS_BOX = (0, 1000, 1060, 2048)
# square crop (in the EXIF-corrected 1536x2048 photo) framing eyes + nose
FACE_CROP = (250, 180, 1490, 1420)


ROOT = Path(__file__).resolve().parent.parent
ORIGINAL_VTEX = ROOT / 'vendor/original/soul_container_color_png_1da67bfe.vtex_c'


def decode_original():
    """Decode mip 0 (BC7, stored last) of the original vtex_c into an RGBA image."""
    mip0 = ORIGINAL_VTEX.read_bytes()[-SIZE * SIZE:]
    bgra = np.frombuffer(texture2ddecoder.decode_bc7(mip0, SIZE, SIZE), np.uint8).reshape(SIZE, SIZE, 4)
    return Image.fromarray(bgra[:, :, [2, 1, 0, 3]].copy(), 'RGBA')


def load_photo(path):
    img = ImageOps.exif_transpose(Image.open(path)).convert('RGB')
    # the game's NPR lighting flattens colours a bit; give the photo a small punch
    img = ImageEnhance.Contrast(img).enhance(1.08)
    return ImageEnhance.Color(img).enhance(1.10)


def circle_mask(size, cx, cy, r, feather=4):
    m = Image.new('L', size, 0)
    ImageDraw.Draw(m).ellipse((cx - r, cy - r, cx + r, cy + r), fill=255)
    return m.filter(ImageFilter.GaussianBlur(feather)) if feather else m


def paste_cover(dst, photo, box, mask=None):
    """Paste `photo` scaled to cover `box` (center-cropped)."""
    w, h = box[2] - box[0], box[3] - box[1]
    tile = ImageOps.fit(photo, (w, h), Image.LANCZOS)
    dst.paste(tile, box[:2], mask)


def compose(base_path, photo_path, variant):
    base = Image.open(base_path).convert('RGBA') if base_path else decode_original()
    assert base.size == (SIZE, SIZE), base.size
    rgb = base.convert('RGB')
    photo = load_photo(photo_path)
    face = photo.crop(FACE_CROP)

    if variant == 'full':
        paste_cover(rgb, photo, FRAME_BOX)
        paste_cover(rgb, photo, TRIS_BOX)

    cx, cy, r = DISC
    bleed = r + 10  # paint slightly past the disc edge so mip levels don't bleed teal in
    disc_img = face.resize((bleed * 2, bleed * 2), Image.LANCZOS)
    mask = circle_mask((bleed * 2, bleed * 2), bleed, bleed, bleed - 2)
    rgb.paste(disc_img, (cx - bleed, cy - bleed), mask)

    out = rgb.convert('RGBA')
    out.putalpha(base.getchannel('A'))  # keep the original alpha channel untouched
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--base', default=None)
    ap.add_argument('--photo', default=str(ROOT / 'assets/cat.jpg'))
    ap.add_argument('--out', default=str(ROOT / 'assets/soul_container_color.png'))
    ap.add_argument('--variant', choices=['discs', 'full'], default='discs')
    a = ap.parse_args()
    compose(a.base, a.photo, a.variant).save(a.out)
    print('wrote', a.out)


if __name__ == '__main__':
    main()
