import io

import numpy as np
import pytest
from PIL import Image

from catmod import templates
from catmod.mesh import build_buffers, frisvad_basis, pack_tangent_frame, unpack_normal, uv_sphere
from catmod.photo import PhotoParams, sphere_texture
from catmod.resource import Resource

have_templates = all(templates.local_path(t).exists() for t in templates.ALL)
needs_templates = pytest.mark.skipif(not have_templates, reason="run scripts/catmod_cli.py fetch")


@needs_templates
@pytest.mark.parametrize("tpl", templates.ALL, ids=lambda t: t.game_path.rsplit("/", 1)[-1])
def test_resource_roundtrip(tpl):
    data = templates.local_path(tpl).read_bytes()
    assert Resource.parse(data).serialize() == data


def test_tangent_frame_roundtrip():
    rng = np.random.default_rng(0)
    n = rng.normal(size=(5000, 3))
    n /= np.linalg.norm(n, axis=1, keepdims=True)
    t = np.cross(n, rng.normal(size=(5000, 3)))
    t /= np.linalg.norm(t, axis=1, keepdims=True)
    p = pack_tangent_frame(n, t, np.ones(len(n))).astype(np.int64)
    nd = unpack_normal((p >> 12) & 0x3FF, (p >> 22) & 0x3FF)
    assert np.degrees(np.arccos(np.clip((nd * n).sum(1), -1, 1))).max() < 0.25
    ang = ((p >> 1) & 0x7FF) / 2047 * 2 * np.pi
    b1, b2 = frisvad_basis(nd)
    td = np.cos(ang)[:, None] * b1 + np.sin(ang)[:, None] * b2
    tp = t - (t * nd).sum(1, keepdims=True) * nd
    tp /= np.linalg.norm(tp, axis=1, keepdims=True)
    assert np.degrees(np.arccos(np.clip((td * tp).sum(1), -1, 1))).max() < 0.5
    assert (p & 1).all()


def test_sphere_winding_outward_and_budget():
    pos, _, _, _, tris = uv_sphere(6.3, 20, 10)
    a, b, c = pos[tris[:, 0]], pos[tris[:, 1]], pos[tris[:, 2]]
    cr = np.cross(b - a, c - a)
    ok = np.linalg.norm(cr, axis=1) > 1e-9
    assert ((cr * (a + b + c)).sum(1)[ok] > 0).all()
    for v, i in [(6084, 17997), (1444, 3264)]:
        vb, idx, _ = build_buffers(v, i, 6.3)
        assert vb.shape == (v, 24) and len(idx) == i and idx.max() < v


def test_sphere_texture_shape_and_front():
    photo = Image.new("RGB", (300, 400), (200, 30, 30))
    photo.paste((30, 200, 30), (140, 190, 160, 210))  # green dot at the centre
    for layout in ("cube6", "twice", "wrap"):
        tex = sphere_texture(photo, PhotoParams(layout=layout, saturation=1.0, contrast=1.0), size=256)
        assert tex.size == (256, 256) and tex.mode == "RGBA"
        # front centre of the sphere = u 0 / v 0.5 -> texel column 0, middle row
        r, g, b, a = tex.getpixel((0, 128))
        assert g > r, layout
        assert a == 255  # colour alpha 1.0 by default (0 renders black in game)
    tex = sphere_texture(photo, PhotoParams(color_alpha=0.0), size=64)
    assert tex.getpixel((0, 32))[3] == 0


@needs_templates
def test_build_mod_end_to_end():
    from catmod.build import build_mod
    import vpk

    buf = io.BytesIO()
    Image.new("RGB", (64, 64), (120, 120, 120)).save(buf, "PNG")
    buf.seek(0)
    res = build_mod(buf)
    assert set(res.files) == {t.game_path for t in templates.ALL}
    for t in templates.ALL[1:]:  # textures keep the exact original size
        assert len(res.files[t.game_path]) == templates.local_path(t).stat().st_size
    pos, uv, tris = res.preview_mesh(0)
    r = np.linalg.norm(pos, axis=1)
    assert np.allclose(r, 6.3, atol=1e-3)
    assert uv.min() >= 0 and uv.max() <= 1
    data = res.vpk_bytes()
    assert data[:4] == b"\x34\x12\xaa\x55"
    tmp = io.BytesIO(data)
    assert tmp.getbuffer().nbytes > 1_000_000
    del vpk


def test_uv_stored_as_half_float():
    # the game decodes the TEXCOORD stream as R16G16_FLOAT (confirmed in game), not the declared SNORM
    vb, _, _ = build_buffers(1444, 3264, 6.3)
    uv = vb[:, 12:16].copy().view(np.float16).astype(float)
    assert uv.min() >= 0 and uv.max() <= 1 and uv.max() > 0.9
