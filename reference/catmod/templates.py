"""Original game files the mod is built from (Valve assets: extracted locally, never committed).

Each template is pinned by sha256 so a game update that changes a file fails loudly instead of
producing a broken mod.
"""
from __future__ import annotations

import hashlib
import os
from dataclasses import dataclass
from pathlib import Path

# repo root: templates/ is shared by the Python reference and the Go backend
ROOT = Path(__file__).resolve().parents[2]
TEMPLATE_DIR = Path(os.environ.get("CATMOD_TEMPLATES", ROOT / "templates"))
DEFAULT_CITADEL = Path.home() / ".local/share/Steam/steamapps/common/Deadlock/game/citadel"

MAT = "models/props_gameplay/soul_container/materials/"


@dataclass(frozen=True)
class Template:
    game_path: str
    sha256: str | None = None  # None = not pinned


MODEL = Template("models/props_gameplay/soul_container/soul_container.vmdl_c",
                 "20e3946fb0272c7b2740d04112374c643d5c51150d01789fd34a0f7461a134e6")
COLOR = Template(MAT + "soul_container_color_png_1da67bfe.vtex_c")
NORMAL_ROUGH = Template(MAT + "soul_container_rough_png_59a0e409.vtex_c")
AO = Template(MAT + "soul_container_ao_png_e9d127e1.vtex_c")
ALL = (MODEL, COLOR, NORMAL_ROUGH, AO)

# Per-LOD (vertex block index, index block index, vertex count, index count), from the model's
# MDAT/CTRL data (dumped with Source2Viewer-CLI -a). Counts are kept identical when patching.
MODEL_LODS = [
    (0, 1, 6084, 17997),
    (3, 4, 3196, 9000),
    (6, 7, 1531, 3576),
    (9, 10, 1444, 3264),
]

# TEXCOORD format declared by each LOD's vertex layout (m_inputLayoutFields): LODs 0-2 use
# R16G16_SNORM (format 37), LOD3 uses R16G16_FLOAT (format 34). The game honours the declaration,
# so each LOD must store its UVs in its own format. (An earlier in-game test looked like "the game
# always reads f16" only because the small soul container is usually drawn with LOD3.)
MODEL_LOD_UV = ["snorm", "snorm", "snorm", "f16"]


def local_path(t: Template) -> Path:
    return TEMPLATE_DIR / t.game_path


def load(t: Template) -> bytes:
    p = local_path(t)
    if not p.exists():
        raise FileNotFoundError(f"missing template {p} — run scripts/fetch_templates.py")
    data = p.read_bytes()
    if t.sha256 and hashlib.sha256(data).hexdigest() != t.sha256:
        raise ValueError(f"{t.game_path} differs from the version this mod was written for "
                         "(game update?). Re-check the layout before trusting the output.")
    return data


def fetch(citadel: Path = DEFAULT_CITADEL) -> list[Path]:
    import vpk

    pak = vpk.open(str(citadel / "pak01_dir.vpk"))
    out = []
    for t in ALL:
        p = local_path(t)
        p.parent.mkdir(parents=True, exist_ok=True)
        pak[t.game_path].save(str(p))
        out.append(p)
    return out
