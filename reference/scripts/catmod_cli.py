#!/usr/bin/env python3
"""Build the Deadlock soul-container photo mod.

  catmod_cli.py fetch                       # extract template files from the local game install
  catmod_cli.py build PHOTO [options]       # build VPK (+ previews, + install / copy to Downloads)
  catmod_cli.py calibrate PHOTO [options]   # VPK with labelled alpha/roughness bands for in-game tests
  catmod_cli.py uvtest PHOTO [options]      # diagnostic: half sphere f16 UVs (orange bg), half snorm (teal)
  catmod_cli.py uninstall                   # remove the directly installed pak09

Outputs go next to -o (default build/): pak01_dir.vpk, preview.png (renders), preview.html (3D).
"""
import argparse
import hashlib
import shutil
import sys
import webbrowser
from pathlib import Path

REF = Path(__file__).resolve().parent.parent  # reference/
ROOT = REF.parent                            # repo root (build/ lives here)
sys.path.insert(0, str(REF))

from catmod import templates  # noqa: E402
from catmod.build import build_mod  # noqa: E402
from catmod.photo import PhotoParams  # noqa: E402
from catmod.preview import contact_sheet  # noqa: E402
from catmod.webpreview import preview_html  # noqa: E402

ADDONS = templates.DEFAULT_CITADEL / "addons"
INSTALL_NAME = "pak09_dir.vpk"
MARKER = ".catmod_pak09"
DOWNLOADS = Path.home() / "Downloads"


def hexrgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i + 2], 16) for i in (0, 2, 4))


def add_photo_args(b):
    b.add_argument("photo", type=Path)
    b.add_argument("-o", "--output", type=Path, default=ROOT / "build/pak01_dir.vpk")
    b.add_argument("--layout", choices=["cube6", "twice", "wrap"], default="cube6",
                   help="cube6: 6 photo stickers (readable from any angle); twice: front+back; wrap: one photo")
    b.add_argument("--center", type=float, nargs=2, default=(0.5, 0.5), metavar=("X", "Y"))
    b.add_argument("--radius", type=float, default=0.5)
    b.add_argument("--projection", choices=["ortho", "equidistant"], default="equidistant")
    b.add_argument("--sticker-deg", type=float, default=40.0, help="sticker angular radius")
    b.add_argument("--ring-deg", type=float, default=3.0, help="ring width around stickers (0 = none)")
    b.add_argument("--bg-color", default="18788c", help="hex colour between stickers")
    b.add_argument("--ring-color", default="ebebeb", help="hex colour of the sticker ring")
    b.add_argument("--color-alpha", type=float, default=1.0,
                   help="colour texture alpha (packed 'metal'); 0 renders black in game")
    b.add_argument("--roughness", type=float, default=0.69)
    b.add_argument("--back", choices=["mirror", "flip", "color"], default="mirror", help="wrap layout only")
    b.add_argument("--rotate", type=float, default=0.0)
    b.add_argument("--uv-encoding", choices=["auto", "snorm", "f16", "split"], default="auto",
                   help="how vertex UVs are stored: auto = the format each LOD declares (diagnostic)")
    b.add_argument("--no-preview", action="store_true", help="skip preview.png / preview.html")
    b.add_argument("--open", action="store_true", help="open preview.html in the browser")
    b.add_argument("--install", action="store_true",
                   help=f"copy to addons/{INSTALL_NAME} directly (no DMM; restart the game to see it)")
    b.add_argument("--downloads", action="store_true", help="also copy the VPK to ~/Downloads")


def params_from(a) -> PhotoParams:
    return PhotoParams(layout=a.layout, center=tuple(a.center), radius=a.radius,
                       projection=a.projection, sticker_deg=a.sticker_deg, ring_deg=a.ring_deg,
                       bg_color=hexrgb(a.bg_color), ring_color=hexrgb(a.ring_color), back=a.back,
                       color_alpha=a.color_alpha, roughness=a.roughness, rotation_deg=a.rotate)


def install(vpk_path: Path):
    dst, marker = ADDONS / INSTALL_NAME, ADDONS / MARKER
    if dst.exists() and not marker.exists():
        sys.exit(f"{dst} exists and is not ours (DMM?) — not overwriting")
    shutil.copyfile(vpk_path, dst)
    marker.write_text(hashlib.sha256(dst.read_bytes()).hexdigest())
    print(f"installed {dst}")
    print("  ojo: si DMM tiene otro mod del soul container activo (número menor), ese gana. Desactívalo.")


def uninstall():
    dst, marker = ADDONS / INSTALL_NAME, ADDONS / MARKER
    if not marker.exists():
        sys.exit("nothing installed by catmod")
    if dst.exists() and hashlib.sha256(dst.read_bytes()).hexdigest() != marker.read_text().strip():
        sys.exit(f"{dst} changed since install — refusing, remove it manually")
    dst.unlink(missing_ok=True)
    marker.unlink()
    print("uninstalled")


def finish(a, result, roughness):
    a.output.parent.mkdir(parents=True, exist_ok=True)
    a.output.write_bytes(result.vpk_bytes())
    print(f"wrote {a.output}")
    if not a.no_preview:
        out = a.output.parent
        pos, uv, tris = result.preview_mesh()
        contact_sheet(pos, uv, tris, result.texture).save(out / "preview.png")
        html = out / "preview.html"
        html.write_text(preview_html(result.texture, roughness=roughness), encoding="utf-8")
        print(f"wrote {out / 'preview.png'} and {html}")
        if a.open:
            webbrowser.open(html.as_uri())
    if a.downloads:
        dst = DOWNLOADS / ("catmod_" + a.output.parent.name + ".vpk")
        shutil.copyfile(a.output, dst)
        print(f"copied to {dst}")
    if a.install:
        install(a.output)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    f = sub.add_parser("fetch")
    f.add_argument("--citadel", type=Path, default=templates.DEFAULT_CITADEL)
    add_photo_args(sub.add_parser("build"))
    add_photo_args(sub.add_parser("calibrate"))
    add_photo_args(sub.add_parser("uvtest"))
    sub.add_parser("uninstall")
    a = ap.parse_args()

    if a.cmd == "fetch":
        for p in templates.fetch(a.citadel):
            print("wrote", p)
    elif a.cmd == "uninstall":
        uninstall()
    elif a.cmd == "build":
        params = params_from(a)
        finish(a, build_mod(a.photo, params, uv_encoding=a.uv_encoding), params.roughness)
    elif a.cmd == "uvtest":
        import dataclasses
        import numpy as np
        from PIL import Image
        from catmod.photo import sphere_texture
        params = params_from(a)
        teal =np.asarray(sphere_texture(a.photo, params))
        orange = np.asarray(sphere_texture(a.photo, dataclasses.replace(params, bg_color=(255, 120, 0))))
        half = teal.shape[1] // 2
        tex = Image.fromarray(np.concatenate([orange[:, :half], teal[:, half:]], 1), "RGBA")
        finish(a, build_mod(None, params, texture=tex, uv_encoding="split"), params.roughness)
    elif a.cmd == "calibrate":
        from catmod.calibrate import calibration_maps
        params = params_from(a)
        texture, rough = calibration_maps(a.photo, params)
        finish(a, build_mod(None, params, texture=texture, roughness_map=rough), params.roughness)


if __name__ == "__main__":
    main()
