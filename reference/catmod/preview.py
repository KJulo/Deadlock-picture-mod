"""Tiny numpy software renderer used to preview a textured mesh without the game."""
from __future__ import annotations

import numpy as np
from PIL import Image


def rot(yaw: float, pitch: float) -> np.ndarray:
    """View rotation for Source coordinates (Z up): yaw around Z, then pitch."""
    cy, sy = np.cos(yaw), np.sin(yaw)
    cp, sp = np.cos(pitch), np.sin(pitch)
    # camera looks along -X_cam... map world (x fwd, y left, z up) -> screen (right, up, depth)
    to_screen = np.array([[0, 1, 0], [0, 0, 1], [1, 0, 0]], float)
    Rz = np.array([[cy, -sy, 0], [sy, cy, 0], [0, 0, 1]])
    Rx = np.array([[1, 0, 0], [0, cp, -sp], [0, sp, cp]])
    return Rx @ to_screen @ Rz


def render(pos, uv, tris, texture: Image.Image, R, size: int = 400, bg=(38, 38, 38)) -> Image.Image:
    tex = np.asarray(texture.convert("RGB"), np.float64) / 255
    th, tw = tex.shape[:2]
    c = (pos.max(0) + pos.min(0)) / 2
    q = (pos - c) @ R.T
    s = size * 0.45 / max(np.abs(q[:, :2]).max(), 1e-9)
    X, Y, Z = q[:, 0] * s + size / 2, -q[:, 1] * s + size / 2, q[:, 2]
    img = np.empty((size, size, 3))
    img[:] = np.array(bg) / 255
    zb = np.full((size, size), -np.inf)
    light = np.array([0.3, 0.5, 0.8])
    light /= np.linalg.norm(light)
    for a, b, cc in tris:
        if a == b or b == cc or a == cc:
            continue
        xs, ys = np.array([X[a], X[b], X[cc]]), np.array([Y[a], Y[b], Y[cc]])
        n = np.cross(q[b] - q[a], q[cc] - q[a])
        nn = np.linalg.norm(n)
        if nn == 0:
            continue
        n /= nn
        x0, x1 = int(max(xs.min(), 0)), int(min(xs.max() + 1, size))
        y0, y1 = int(max(ys.min(), 0)), int(min(ys.max() + 1, size))
        if x0 >= x1 or y0 >= y1:
            continue
        gx, gy = np.meshgrid(np.arange(x0, x1) + .5, np.arange(y0, y1) + .5)
        d = (ys[1] - ys[2]) * (xs[0] - xs[2]) + (xs[2] - xs[1]) * (ys[0] - ys[2])
        if abs(d) < 1e-12:
            continue
        w0 = ((ys[1] - ys[2]) * (gx - xs[2]) + (xs[2] - xs[1]) * (gy - ys[2])) / d
        w1 = ((ys[2] - ys[0]) * (gx - xs[2]) + (xs[0] - xs[2]) * (gy - ys[2])) / d
        w2 = 1 - w0 - w1
        m = (w0 >= 0) & (w1 >= 0) & (w2 >= 0)
        if not m.any():
            continue
        z = w0 * Z[a] + w1 * Z[b] + w2 * Z[cc]
        sub = zb[y0:y1, x0:x1]
        m &= z > sub
        if not m.any():
            continue
        u = w0 * uv[a, 0] + w1 * uv[b, 0] + w2 * uv[cc, 0]
        v = w0 * uv[a, 1] + w1 * uv[b, 1] + w2 * uv[cc, 1]
        tx = (np.clip(u, 0, 1) * (tw - 1)).astype(int)
        ty = (np.clip(v, 0, 1) * (th - 1)).astype(int)
        col = tex[ty, tx] * (0.45 + 0.55 * abs(n @ light))
        sub[m] = z[m]
        img[y0:y1, x0:x1][m] = col[m]
    return Image.fromarray((img * 255).clip(0, 255).astype(np.uint8))


VIEWS = [("frente", 0.0, 0.0), ("3/4", 0.8, 0.35), ("lado", np.pi / 2, 0.0),
         ("atrás", np.pi, 0.0), ("arriba", 0.0, 1.3)]


def contact_sheet(pos, uv, tris, texture, size: int = 320) -> Image.Image:
    shots = [render(pos, uv, tris, texture, rot(y, p), size) for _, y, p in VIEWS]
    sheet = Image.new("RGB", (size * len(shots), size))
    for k, im in enumerate(shots):
        sheet.paste(im, (k * size, 0))
    return sheet
