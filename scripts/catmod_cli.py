#!/usr/bin/env python3
"""Build the Deadlock soul-container photo mod.

  catmod_cli.py fetch                         # extract template files from the local game install
  catmod_cli.py build PHOTO [-o out.vpk] [--preview preview.png] [photo options]
"""
import argparse
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

from catmod import templates  # noqa: E402
from catmod.build import build_mod  # noqa: E402
from catmod.photo import PhotoParams  # noqa: E402
from catmod.preview import contact_sheet  # noqa: E402


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    f = sub.add_parser("fetch")
    f.add_argument("--citadel", type=Path, default=templates.DEFAULT_CITADEL)
    b = sub.add_parser("build")
    b.add_argument("photo", type=Path)
    b.add_argument("-o", "--output", type=Path, default=ROOT / "build/pak01_dir.vpk")
    b.add_argument("--preview", type=Path, help="write a contact-sheet render of the result")
    b.add_argument("--texture", type=Path, help="write the composed color texture")
    b.add_argument("--center", type=float, nargs=2, default=(0.5, 0.5), metavar=("X", "Y"))
    b.add_argument("--radius", type=float, default=0.5)
    b.add_argument("--projection", choices=["ortho", "equidistant"], default="equidistant")
    b.add_argument("--back", choices=["mirror", "flip", "color"], default="mirror")
    b.add_argument("--rotate", type=float, default=0.0)
    a = ap.parse_args()

    if a.cmd == "fetch":
        for p in templates.fetch(a.citadel):
            print("wrote", p)
        return

    params = PhotoParams(center=tuple(a.center), radius=a.radius, projection=a.projection,
                         back=a.back, rotation_deg=a.rotate)
    result = build_mod(a.photo, params)
    a.output.parent.mkdir(parents=True, exist_ok=True)
    a.output.write_bytes(result.vpk_bytes())
    print(f"wrote {a.output}  (sphere grid per LOD: {result.grids})")
    if a.texture:
        result.texture.save(a.texture)
    if a.preview:
        pos, uv, tris = result.preview_mesh()
        contact_sheet(pos, uv, tris, result.texture).save(a.preview)
        print("wrote", a.preview)


if __name__ == "__main__":
    main()
