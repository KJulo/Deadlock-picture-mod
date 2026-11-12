"""Project a user photo onto the sphere's equirectangular texture.

Every texel (u, v) is a direction d on the sphere (convention in catmod.mesh: +Z up, u = phi/2pi,
v = theta/pi). In game the container's orientation is arbitrary, so the default layout repeats the
photo as round "stickers" so that a complete, undistorted photo is visible from every angle.

Layouts:
  'cube6'  6 stickers at the cube face centres (+-X, +-Y, +-Z)      <- default, readable from anywhere
  'twice'  2 stickers, front (+X) and back (-X)
  'wrap'   one photo wrapped around the whole sphere (front +X, back per `back`)

Sticker parameters:
  sticker_deg   angular radius of each sticker (cube6 max ~45 before they touch)
  ring_deg      width of the coloured ring around each sticker (0 = none)
  ring_color / bg_color

Photo framing (shared by all layouts):
  center      photo point (fractions 0..1) placed at each sticker's centre
  radius      photo disc radius as a fraction of the photo's shorter side (0.5 = largest centred disc)
  projection  'equidistant' (even spread) or 'ortho' (exact when seen straight on, squashed at edges)

Material:
  roughness    0..1, written to the NormalRoughness texture (blue channel)
  color_alpha  0..1, the colour texture's alpha. The compiler packs soul_container_metal.png there
               (original = 255). With Deadlock's NPR shader alpha 0 renders the sphere black, so
               its exact meaning is still being calibrated (see `catmod_cli.py calibrate`).
"""
from __future__ import annotations

from dataclasses import dataclass

import numpy as np
from PIL import Image, ImageEnhance, ImageOps

TEX_SIZE = 2048

S = 1 / np.sqrt(2)
# (centre direction, up direction) per sticker. Up for the top/bottom stickers is arbitrary.
LAYOUTS = {
    "cube6": [((1, 0, 0), (0, 0, 1)), ((-1, 0, 0), (0, 0, 1)), ((0, 1, 0), (0, 0, 1)),
              ((0, -1, 0), (0, 0, 1)), ((0, 0, 1), (-1, 0, 0)), ((0, 0, -1), (1, 0, 0))],
    "twice": [((1, 0, 0), (0, 0, 1)), ((-1, 0, 0), (0, 0, 1))],
}


@dataclass
class PhotoParams:
    layout: str = "cube6"
    center: tuple[float, float] = (0.5, 0.5)
    radius: float = 0.5
    projection: str = "equidistant"
    sticker_deg: float = 40.0
    ring_deg: float = 3.0
    ring_color: tuple[int, int, int] = (235, 235, 235)
    bg_color: tuple[int, int, int] = (24, 120, 140)  # soul teal
    back: str = "mirror"  # wrap layout only: mirror | flip | color
    color_alpha: float = 1.0  # original texture: ~1.0. 0 renders BLACK in game (tested)
    roughness: float = 0.69   # original cage average (177/255)
    contrast: float = 1.05
    saturation: float = 1.10
    rotation_deg: float = 0.0

    def validate(self):
        assert self.layout in ("cube6", "twice", "wrap"), self.layout
        assert self.projection in ("ortho", "equidistant"), self.projection
        assert self.back in ("mirror", "flip", "color"), self.back
        assert 0.05 <= self.radius <= 1.5, self.radius
        assert 5 <= self.sticker_deg <= 90, self.sticker_deg
        assert 0 <= self.color_alpha <= 1, self.color_alpha
        assert 0 <= self.roughness <= 1, self.roughness
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


def directions(size: int) -> np.ndarray:
    u = (np.arange(size) + 0.5) / size
    ph, th = np.meshgrid(u * 2 * np.pi, u * np.pi)
    return np.stack([np.sin(th) * np.cos(ph), np.sin(th) * np.sin(ph), np.cos(th)], -1)


def _disc_coords(d, centre, up, max_angle, projection):
    """Unit-disc coords (sx right, sy up) of directions d in a sticker; rho=1 at max_angle."""
    c, up = np.asarray(centre, float), np.asarray(up, float)
    right = np.cross(up, c)  # viewer outside looking at the sticker: right = up x centre
    x, y, z = d @ right, d @ up, d @ c
    lateral = np.hypot(x, y)
    ang = np.arctan2(lateral, z)
    rho = (ang if projection == "equidistant" else np.sin(np.minimum(ang, np.pi / 2))) / (
        max_angle if projection == "equidistant" else np.sin(max_angle))
    with np.errstate(invalid="ignore", divide="ignore"):
        k = np.where(lateral > 1e-9, rho / lateral, 0.0)
    return x * k, y * k, ang


def sphere_texture(photo, params: PhotoParams | None = None, size: int = TEX_SIZE) -> Image.Image:
    """RGBA equirect texture; alpha = params.color_alpha."""
    p = (params or PhotoParams()).validate()
    arr = np.asarray(load_photo(photo, p), np.float32)
    h, w = arr.shape[:2]
    cx, cy, r = p.center[0] * w, p.center[1] * h, p.radius * min(w, h)
    d = directions(size)

    if p.layout == "wrap":
        sx, sy, _ = _disc_coords(d, (1, 0, 0), (0, 0, 1), np.pi / 2, p.projection)
        back = d[..., 0] < 0
        if p.back == "flip":
            sx = np.where(back, -sx, sx)
        out = _bilinear(arr, cx + sx * r, cy - sy * r)
        if p.back == "color":
            out[back] = p.bg_color
    else:
        stickers = LAYOUTS[p.layout]
        cos = np.stack([d @ np.asarray(c, float) for c, _ in stickers])
        nearest = cos.argmax(0)
        out = np.empty(d.shape[:2] + (3,), np.float32)
        out[:] = p.bg_color
        R, ring = np.radians(p.sticker_deg), np.radians(p.ring_deg)
        for i, (c, up) in enumerate(stickers):
            sx, sy, ang = _disc_coords(d, c, up, R, p.projection)
            mine = nearest == i
            inside = mine & (ang <= R)
            out[inside] = _bilinear(arr, cx + sx[inside] * r, cy - sy[inside] * r)
            out[mine & (ang > R) & (ang <= R + ring)] = p.ring_color

    alpha = np.full(out.shape[:2] + (1,), round(p.color_alpha * 255), np.float32)
    rgba = np.concatenate([np.clip(out, 0, 255), alpha], -1).astype(np.uint8)
    return Image.fromarray(rgba, "RGBA")
