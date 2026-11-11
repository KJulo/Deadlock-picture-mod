"""Sphere mesh generation in the soul_container vertex layout.

Vertex layout (24 bytes, from the model's m_inputLayoutFields):
  POSITION      R32G32B32_FLOAT  12 B   (Source units, Z up)
  TEXCOORD      R16G16_SNORM      4 B
  NORMAL        R32_UINT          4 B   CompressedTangentFrame (see pack_tangent_frame)
  BLENDINDICES  R8G8B8A8_UINT     4 B   all zero (single bone)

UV convention (shared with catmod.photo): direction d(theta, phi) = (sin t cos p, sin t sin p, cos t)
  u = phi / 2pi, v = theta / pi  (v = 0 at the top pole, +Z).
"""
from __future__ import annotations

import numpy as np

VERTEX_SIZE = 24


def pack_tangent_frame(n: np.ndarray, t: np.ndarray, sign: np.ndarray) -> np.ndarray:
    """Pack unit normals/tangents into Source 2's 32-bit CompressedTangentFrame.

    bit 0      : bitangent sign (1 -> +1)
    bits 1-11  : tangent angle in the Frisvad/Duff ONB around n, quantised as angle / 2pi * 2047
    bits 12-21 : octahedral normal X, (x + 1) / 2 * 1023
    bits 22-31 : octahedral normal Y
    Reverse-engineered against Source2Viewer's decode of the original model (exact match).
    """
    n = n / np.linalg.norm(n, axis=1, keepdims=True)
    # octahedral encode
    p = n[:, :2] / np.abs(n).sum(1, keepdims=True)
    neg = n[:, 2] < 0
    fold = (1 - np.abs(p[:, ::-1])) * np.where(p >= 0, 1.0, -1.0)
    p = np.where(neg[:, None], fold, p)
    xb = np.clip(np.rint((p[:, 0] + 1) / 2 * 1023), 0, 1023).astype(np.uint32)
    yb = np.clip(np.rint((p[:, 1] + 1) / 2 * 1023), 0, 1023).astype(np.uint32)
    # tangent angle in the ONB of the *quantised* normal (what the shader will reconstruct)
    nq = unpack_normal(xb, yb)
    b1, b2 = frisvad_basis(nq)
    t = t - (t * nq).sum(1, keepdims=True) * nq
    ang = np.mod(np.arctan2((t * b2).sum(1), (t * b1).sum(1)), 2 * np.pi)
    tb = np.rint(ang / (2 * np.pi) * 2047).astype(np.uint32) % 2048
    tb = np.minimum(tb, 2047)
    sb = (np.asarray(sign) > 0).astype(np.uint32)
    return sb | (tb << 1) | (xb << 12) | (yb << 22)


def unpack_normal(xb, yb) -> np.ndarray:
    x = xb / 1023 * 2 - 1
    y = yb / 1023 * 2 - 1
    z = 1 - np.abs(x) - np.abs(y)
    neg = z < 0
    x2 = np.where(neg, (1 - np.abs(y)) * np.sign(x), x)
    y2 = np.where(neg, (1 - np.abs(x)) * np.sign(y), y)
    n = np.stack([x2, y2, z], 1)
    return n / np.linalg.norm(n, axis=1, keepdims=True)


def frisvad_basis(n: np.ndarray):
    s = np.where(n[:, 2] >= 0, 1.0, -1.0)
    a = -1 / (s + n[:, 2])
    b = n[:, 0] * n[:, 1] * a
    b1 = np.stack([1 + s * n[:, 0] ** 2 * a, s * b, -s * n[:, 0]], 1)
    b2 = np.stack([b, s + n[:, 1] ** 2 * a, -n[:, 1]], 1)
    return b1, b2


def sphere_grid(max_vertices: int, max_triangles: int) -> tuple[int, int]:
    """Largest UV-sphere (n_u segments around, n_v rings, n_u = 2 n_v) that fits the budgets."""
    best = (4, 2)
    for nv in range(2, 400):
        nu = 2 * nv
        if (nu + 1) * (nv + 1) <= max_vertices and 2 * nu * nv <= max_triangles:
            best = (nu, nv)
    return best


def uv_sphere(radius: float, nu: int, nv: int):
    """Positions, uvs, normals, tangents, triangle indices of a UV sphere (seam duplicated)."""
    j, i = np.meshgrid(np.arange(nv + 1), np.arange(nu + 1), indexing="ij")
    u = (i / nu).ravel()
    v = (j / nv).ravel()
    th, ph = v * np.pi, u * 2 * np.pi
    n = np.stack([np.sin(th) * np.cos(ph), np.sin(th) * np.sin(ph), np.cos(th)], 1)
    t = np.stack([-np.sin(ph), np.cos(ph), np.zeros_like(ph)], 1)  # dP/du
    a = (j[:-1, :-1] * (nu + 1) + i[:-1, :-1]).ravel()
    b, c = a + 1, a + nu + 1
    d = c + 1
    # same winding as the original model: cross(B-A, C-A) points outward
    tris = np.concatenate([np.stack([a, c, b], 1), np.stack([b, c, d], 1)])
    return n * radius, np.stack([u, v], 1), n, t, tris


def build_buffers(vertex_count: int, index_count: int, radius: float):
    """Vertex bytes (vertex_count * 24) and uint32 indices (index_count) for a padded sphere.

    The counts must equal the original LOD's so the model's metadata stays valid: unused vertices
    duplicate vertex 0 and unused indices form degenerate (invisible) triangles.
    """
    nu, nv = sphere_grid(vertex_count, index_count // 3)
    pos, uv, n, t, tris = uv_sphere(radius, nu, nv)
    nvert = len(pos)
    packed = pack_tangent_frame(n, t, np.ones(nvert))
    vb = np.zeros((vertex_count, VERTEX_SIZE), np.uint8)
    rec = np.zeros((nvert, VERTEX_SIZE), np.uint8)
    rec[:, 0:12] = pos.astype(np.float32).view(np.uint8).reshape(nvert, 12)
    rec[:, 12:16] = np.rint(np.clip(uv, 0, 1) * 32767).astype(np.int16).view(np.uint8).reshape(nvert, 4)
    rec[:, 16:20] = packed.astype(np.uint32).view(np.uint8).reshape(nvert, 4)
    vb[:nvert] = rec
    vb[nvert:] = rec[0]
    idx = np.zeros(index_count, np.uint32)
    flat = tris.ravel()
    idx[:len(flat)] = flat
    return vb, idx, (nu, nv)
