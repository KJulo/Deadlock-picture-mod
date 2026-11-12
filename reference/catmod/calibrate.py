"""Calibration texture: one in-game test shows many material values at once.

Latitude bands vary the colour texture's alpha (packed "metal" channel), longitude sectors vary
roughness. Every cell is labelled ("A128 R35" = alpha 128, roughness 0.35) on top of the photo
stickers, so a single screenshot tells which combination renders the photo best.
"""
from __future__ import annotations

import numpy as np
from PIL import Image, ImageDraw, ImageFont

from .photo import PhotoParams, sphere_texture

ALPHAS = [0, 64, 128, 192, 255]
ROUGHNESS = [0.0, 0.35, 0.69, 1.0]
SECTORS = 8  # roughness values repeated twice around the sphere


def calibration_maps(photo, params: PhotoParams | None = None, size: int = 2048):
    base = sphere_texture(photo, params, size).convert("RGB")
    rgb = np.asarray(base).copy()
    alpha = np.zeros((size, size), np.uint8)
    rough = np.zeros((size, size), np.uint8)
    band_h = size / len(ALPHAS)
    sector_w = size / SECTORS
    for b, a in enumerate(ALPHAS):
        alpha[int(b * band_h):int((b + 1) * band_h)] = a
    for s in range(SECTORS):
        rough[:, int(s * sector_w):int((s + 1) * sector_w)] = round(ROUGHNESS[s % len(ROUGHNESS)] * 255)

    img = Image.fromarray(rgb)
    draw = ImageDraw.Draw(img)
    font = ImageFont.load_default(size=56)
    for b, a in enumerate(ALPHAS):
        for s in range(SECTORS):
            r = ROUGHNESS[s % len(ROUGHNESS)]
            label = f"A{a} R{int(r * 100)}"
            cx, cy = (s + 0.5) * sector_w, (b + 0.5) * band_h
            draw.text((cx, cy), label, font=font, anchor="mm", fill=(255, 255, 255),
                      stroke_width=5, stroke_fill=(0, 0, 0))
        # band separators
        y = int(b * band_h)
        draw.line([(0, y), (size, y)], fill=(255, 0, 255), width=6)
    for s in range(SECTORS):
        x = int(s * sector_w)
        draw.line([(x, 0), (x, size)], fill=(255, 255, 0), width=6)

    texture = Image.merge("RGBA", (*img.split(), Image.fromarray(alpha)))
    return texture, Image.fromarray(rough)
