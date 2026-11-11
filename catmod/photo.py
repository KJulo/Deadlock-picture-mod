"""Project a user photo onto the sphere's equirectangular texture.

Every texel (u, v) is a direction d on the sphere (convention in catmod.mesh, +Z up). The photo is
"printed" on the front hemisphere (+X) facing outward; the back hemisphere is filled per `back`.

Parameters (all exposed so a UI can tweak them):
  center      (x, y) of the photo pixel that lands on the front of the sphere, fractions 0..1
  radius      radius of the photo disc that wraps the front hemisphere, as a fraction of the
              photo's shorter side (0.5 = the largest centred disc)
  projection  'ortho'       -> looks exactly like the photo when seen straight on
              'equidistant' -> spreads the photo more evenly, less squashed towards the edges
  back        'mirror' -> back hemisphere = same disc (seamless, appears mirrored from behind)
              'flip'   -> back hemisphere = disc flipped so it reads correctly from behind
              'color'  -> solid `back_color`
"""
from __future__ import annotations

from dataclasses import dataclass, field

import numpy as np
from PIL import Image, ImageEnhance, ImageOps

TEX_SIZE = 2048


@dataclass
class PhotoParams:
    center: tuple[float, float] = (0.5, 0.5)
    radius: float = 0.5
    projection: str = "equidistant"
    back: str = "mirror"
    back_color: tuple[int, int, int] = (40, 40, 40)
    contrast: float = 1.05
    saturation: float = 1.10
    rotation_deg: float = 0.0  # rotate the photo before projecting

    def validate(self):
        assert self.projection in ("ortho", "equidistant"), self.projection
        assert self.back in ("mirror", "flip", "color"), self.back
        assert 0.05 <= self.radius <= 1.5, self.radius
        return self


def load_photo(src, params: PhotoParams) -> Image.Image:
    img = src if isinstance(src, Image.Image) else Image.open(src)
    img = ImageOps.exif_transpose(img).convert("RGB")
    if params.rotation_deg:
        img = img.rotate(-params.rotation_deg, Image.BICUBIC, expand=True)
    img = ImageEnhance.Contrast(img).enhance(params.contrast)
    return ImageEnhance.Color(img).enhance(params.saturation)


def _bilinear(img: np.ndarray, x: np.ndarray, y: np.ndarray) -> np.ndarray:
    h, w = img.shape[:2]
    x = np.clip(x, 0, w - 1.001)
    y = np.clip(y, 0, h - 1.001)
    x0, y0 = x.astype(int), y.astype(int)
    fx, fy = (x - x0)[..., None], (y - y0)[..., None]
    a, b = img[y0, x0], img[y0, x0 + 1]
    c, d = img[y0 + 1, x0], img[y0 + 1, x0 + 1]
    return (a * (1 - fx) + b * fx) * (1 - fy) + (c * (1 - fx) + d * fx) * fy


def sphere_texture(photo, params: PhotoParams | None = None, size: int = TEX_SIZE) -> Image.Image:
    params = (params or PhotoParams()).validate()
    img = load_photo(photo, params)
    arr = np.asarray(img, np.float32)
    h, w = arr.shape[:2]
    cx, cy = params.center[0] * w, params.center[1] * h
    r = params.radius * min(w, h)

    u = (np.arange(size) + 0.5) / size
    v = (np.arange(size) + 0.5) / size
    ph, th = np.meshgrid(u * 2 * np.pi, v * np.pi)
    dx = np.sin(th) * np.cos(ph)  # towards the viewer of the front face
    dy = np.sin(th) * np.sin(ph)  # viewer's right
    dz = np.cos(th)               # up

    lateral = np.hypot(dy, dz)
    if params.projection == "ortho":
        rho = lateral  # 0 at the front centre, 1 on the silhouette
    else:
        alpha = np.arctan2(lateral, np.abs(dx))  # angle from the front/back axis
        rho = alpha / (np.pi / 2)
    with np.errstate(invalid="ignore", divide="ignore"):
        scale = np.where(lateral > 1e-9, rho / lateral, 0.0)
    sx, sy = dy * scale, dz * scale  # unit-disc coordinates

    back = dx < 0
    if params.back == "flip":
        sx = np.where(back, -sx, sx)
    out = _bilinear(arr, cx + sx * r, cy - sy * r)
    if params.back == "color":
        out[back] = params.back_color
    return Image.fromarray(np.clip(out, 0, 255).astype(np.uint8), "RGB")
