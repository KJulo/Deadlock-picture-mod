"""High-level API: photo in -> mod files / VPK bytes out. This is what a web backend would call."""
from __future__ import annotations

import io
import tempfile
from dataclasses import dataclass
from pathlib import Path

import vpk
from PIL import Image

from . import templates, vtex
from .photo import PhotoParams, sphere_texture
from .vmdl import build_sphere_model, decode_lod

# compiled NormalRoughness layout (raw texels): R,G = normal XY (128 = flat), B = roughness, A = 255.
# 177 = the original texture's average roughness. (Source2Viewer shows it re-arranged as RGB normal + A rough.)
NORMAL_ROUGH_RGBA = (128, 128, 177, 255)


@dataclass
class ModResult:
    files: dict[str, bytes]       # game path -> compiled file
    texture: Image.Image          # the composed color texture (for previews)
    grids: list                   # sphere resolution per LOD

    def vpk_bytes(self) -> bytes:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "root"
            for path, data in self.files.items():
                p = root / path
                p.parent.mkdir(parents=True, exist_ok=True)
                p.write_bytes(data)
            out = Path(tmp) / "pak01_dir.vpk"
            vpk.new(str(root)).save(str(out))
            return out.read_bytes()

    def preview_mesh(self, lod: int = 0):
        return decode_lod(self.files[templates.MODEL.game_path], lod)


def build_mod(photo, params: PhotoParams | None = None, shape: str = "sphere") -> ModResult:
    """shape='sphere': sphere geometry + photo wrapped around it.

    `photo` is a path, file-like object or PIL image.
    """
    if shape != "sphere":
        raise NotImplementedError(shape)
    if not isinstance(photo, (str, Path, Image.Image)):
        photo = Image.open(io.BytesIO(photo.read() if hasattr(photo, "read") else photo))
    texture = sphere_texture(photo, params)

    model, grids = build_sphere_model(templates.load(templates.MODEL))
    color_tpl = templates.load(templates.COLOR)
    nr_tpl = templates.load(templates.NORMAL_ROUGH)
    ao_tpl = templates.load(templates.AO)

    size = vtex.info(color_tpl)["width"]
    files = {
        templates.MODEL.game_path: model,
        templates.COLOR.game_path: vtex.rebuild(color_tpl, texture.convert("RGBA").resize((size, size))),
        templates.NORMAL_ROUGH.game_path: vtex.rebuild(
            nr_tpl, Image.new("RGBA", (size, size), NORMAL_ROUGH_RGBA)),
        templates.AO.game_path: vtex.rebuild(ao_tpl, constant=255),  # no baked occlusion
    }
    return ModResult(files, texture, grids)
