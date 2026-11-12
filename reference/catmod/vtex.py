"""Rebuild vtex_c textures by keeping the original resource header and replacing the pixel data.

A vtex_c is a resource (RED2 + DATA blocks) followed by the mip chain, smallest mip first,
uncompressed. Supported formats: BC7 (20) and ATI1N/BC4 (27).
"""
from __future__ import annotations

import struct

import etcpak
import numpy as np
from PIL import Image

from .resource import Resource

FMT_BC7 = 20
FMT_BC4 = 27


def info(data: bytes) -> dict:
    res = Resource.parse(data)
    d = res.blocks[res.block_index("DATA")][1]
    # u16 version, u16 flags, f32 reflectivity[4], u16 width, u16 height, u16 depth, u8 format, u8 mips
    width, height, _depth = struct.unpack_from("<HHH", d, 20)
    fmt, mips = d[26], d[27]
    return {"width": width, "height": height, "format": fmt, "mips": mips, "res": res}


def _mip_sizes(width: int, height: int, mips: int):
    return [(max(width >> i, 4), max(height >> i, 4)) for i in range(mips)]


def encode_bc7(img: Image.Image) -> bytes:
    rgba = np.ascontiguousarray(np.asarray(img.convert("RGBA")))
    return etcpak.compress_bc7(rgba.tobytes(), img.width, img.height)  # expects RGBA order


def bc4_constant(value: int, w: int, h: int) -> bytes:
    block = bytes([value, value]) + b"\x00" * 6
    return block * ((w // 4) * (h // 4))


def rebuild(template: bytes, image: Image.Image | None = None, constant: int | None = None) -> bytes:
    """New vtex_c with the template's header; pixels from `image` (BC7) or `constant` (BC4)."""
    meta = info(template)
    res, w, h, fmt, mips = meta["res"], meta["width"], meta["height"], meta["format"], meta["mips"]
    chain = []
    for mw, mh in _mip_sizes(w, h, mips):
        if fmt == FMT_BC7:
            if image is None:
                raise ValueError("BC7 texture needs an image")
            src = image.convert("RGBA")
            mip = src if src.size == (mw, mh) else src.resize((mw, mh), Image.LANCZOS)
            chain.append(encode_bc7(mip))
        elif fmt == FMT_BC4:
            if constant is None:
                raise ValueError("BC4 texture: only constant fill is supported")
            chain.append(bc4_constant(constant, mw, mh))
        else:
            raise NotImplementedError(f"vtex format {fmt}")
    pixels = b"".join(reversed(chain))  # smallest mip first
    if len(pixels) != len(res.trailer):
        raise ValueError(f"pixel data size {len(pixels)} != original {len(res.trailer)}")
    res.trailer = pixels
    return res.serialize()
