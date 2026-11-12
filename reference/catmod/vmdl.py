"""Patch soul_container.vmdl_c: replace every LOD's geometry with a sphere.

Only the meshopt-compressed MVTX (vertices) and MIDX (indices) blocks change. Vertex/index counts,
draw calls, bounds, physics (a sphere of radius 7) and material references stay untouched, so the
binary KV3 metadata does not need rewriting.
"""
from __future__ import annotations

import meshoptimizer as mo
import numpy as np

from . import templates
from .mesh import VERTEX_SIZE, build_buffers
from .resource import Resource

# fits inside every LOD's bounding box closely enough (LOD0 bounds are +-6.34)
SPHERE_RADIUS = 6.3


# meshopt vertex codec version. The game's own buffers are v1, but with v1 streams from the
# python meshoptimizer binding the UVs came out wrong in game while positions survived (suspected
# mismatch between v1 revisions). v0 is decoded identically by every meshopt version.
VERTEX_CODEC_VERSION = 0


def _encode_vertices(vb: np.ndarray) -> bytes:
    mo.encode_vertex_version(VERTEX_CODEC_VERSION)
    return bytes(mo.encode_vertex_buffer(vb, len(vb), VERTEX_SIZE))


def _encode_indices(idx: np.ndarray, vertex_count: int) -> bytes:
    return bytes(mo.encode_index_buffer(idx.astype(np.uint32), len(idx), vertex_count))


def lod_uv_encoding(lod: int, uv_encoding: str = "auto") -> str:
    """'auto' = the format each LOD declares (templates.MODEL_LOD_UV); anything else forces it."""
    return templates.MODEL_LOD_UV[lod] if uv_encoding == "auto" else uv_encoding


def build_sphere_model(template: bytes, radius: float = SPHERE_RADIUS,
                       uv_encoding: str = "auto") -> tuple[bytes, list]:
    res = Resource.parse(template)
    grids = []
    for lod, (vblock, iblock, vcount, icount) in enumerate(templates.MODEL_LODS):
        assert res.blocks[vblock][0] == "MVTX" and res.blocks[iblock][0] == "MIDX"
        vb, idx, grid = build_buffers(vcount, icount, radius, lod_uv_encoding(lod, uv_encoding))
        res.blocks[vblock] = ("MVTX", _encode_vertices(vb))
        res.blocks[iblock] = ("MIDX", _encode_indices(idx, vcount))
        grids.append(grid)
    return res.serialize(), grids


def decode_lod(model: bytes, lod: int = 0, uv_encoding: str = "auto"):
    """Decode positions, uvs and triangles of one LOD (for previews/tests)."""
    res = Resource.parse(model)
    vblock, iblock, vcount, icount = templates.MODEL_LODS[lod]
    vb = np.asarray(mo.decode_vertex_buffer(vcount, VERTEX_SIZE, res.blocks[vblock][1]))
    vb = vb.view(np.uint8).reshape(vcount, VERTEX_SIZE)
    pos = vb[:, :12].copy().view(np.float32).reshape(-1, 3).astype(float)
    raw = vb[:, 12:16].copy()
    if lod_uv_encoding(lod, uv_encoding) == "f16":
        uv = raw.view(np.float16).reshape(-1, 2).astype(float)
    else:
        uv = raw.view(np.int16).reshape(-1, 2) / 32767.0
    # index_size=4: the python binding always decodes into a uint32 array
    tris = np.asarray(mo.decode_index_buffer(icount, 4, res.blocks[iblock][1])).reshape(-1, 3)
    return pos, uv, tris
