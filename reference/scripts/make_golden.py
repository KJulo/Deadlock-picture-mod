#!/usr/bin/env python3
"""Write golden files that the Go backend tests compare against (testdata/ at the repo root).

Only *metadata* derived from the game files is written (block kinds, sizes, alignments, hashes),
never Valve content, so testdata/ can be committed to the public repo.

  .venv/bin/python reference/scripts/make_golden.py
"""
import base64
import gzip
import hashlib
import json
import sys
import tempfile
from pathlib import Path

REF = Path(__file__).resolve().parent.parent
ROOT = REF.parent
sys.path.insert(0, str(REF))

from catmod import templates  # noqa: E402
from catmod.resource import Resource  # noqa: E402
from catmod.mesh import build_buffers  # noqa: E402
from catmod.vmdl import SPHERE_RADIUS  # noqa: E402
import vpk  # noqa: E402

OUT = ROOT / "testdata"


def resource_layout():
    files = {}
    for tpl in templates.ALL:
        data = templates.local_path(tpl).read_bytes()
        res = Resource.parse(data)
        files[tpl.game_path.rsplit("/", 1)[-1]] = {
            "sha256": hashlib.sha256(data).hexdigest(),
            "header_version": res.header_version,
            "version": res.version,
            "blocks": [{"kind": k, "size": len(b), "align": a}
                       for (k, b), a in zip(res.blocks, res.aligns)],
            "trailer_size": len(res.trailer),
        }
    return files


# Synthetic files covering the VPK tree cases: several extensions, nested and shared directories,
# root files, dots in names, an empty file and binary data.
VPK_FILES = {
    "models/props_gameplay/soul_container/soul_container.vmdl_c": b"model\x00\x01\x02" * 50,
    "models/props_gameplay/soul_container/materials/a_color_png_1.vtex_c": bytes(range(256)) * 3,
    "models/props_gameplay/soul_container/materials/b_rough_png_2.vtex_c": b"",
    "models/other/x.vtex_c": b"x",
    "readme.txt": "hola gato \u00f1".encode(),
    "materials/my.file.name.vmat_c": b"dots",
}


def vpk_synthetic():
    """VPK written by the python library, with the tree sorted the way the Go writer sorts it."""
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp) / "root"
        for p, data in VPK_FILES.items():
            f = root / p
            f.parent.mkdir(parents=True, exist_ok=True)
            f.write_bytes(data)
        pak = vpk.new(str(root))
        pak.tree = {ext: {d: sorted(names) for d, names in sorted(dirs.items())}
                    for ext, dirs in sorted(pak.tree.items())}
        out = Path(tmp) / "out.vpk"
        pak.save(str(out))
        return out.read_bytes()


def mesh_buffers():
    """Uncompressed vertex + index buffers of the sphere for every LOD (pure geometry, no game data).

    Returns (meta, blob): blob is vb0 | idx0 | vb1 | idx1 ... with indices as little-endian u32.
    """
    meta, blob = {"radius": SPHERE_RADIUS, "uv_encoding": "f16", "lods": []}, bytearray()
    for _, _, vcount, icount in templates.MODEL_LODS:
        vb, idx, grid = build_buffers(vcount, icount, SPHERE_RADIUS, "f16")
        meta["lods"].append({"vertex_count": vcount, "index_count": icount, "grid": list(grid),
                             "offset": len(blob)})
        blob += vb.tobytes() + idx.astype("<u4").tobytes()
    return meta, bytes(blob)


def main():
    OUT.mkdir(exist_ok=True)
    path = OUT / "resource_layout.json"
    path.write_text(json.dumps(resource_layout(), indent=2) + "\n")
    print("wrote", path)
    files = OUT / "vpk_synthetic_files.json"
    files.write_text(json.dumps({p: base64.b64encode(d).decode() for p, d in VPK_FILES.items()},
                                indent=2) + "\n")
    (OUT / "vpk_synthetic.vpk").write_bytes(vpk_synthetic())
    print("wrote", files, "and", OUT / "vpk_synthetic.vpk")
    meta, blob = mesh_buffers()
    (OUT / "mesh_buffers.json").write_text(json.dumps(meta, indent=2) + "\n")
    # mtime=0 -> the .gz is byte-identical on every run (no spurious git diffs)
    (OUT / "mesh_buffers.bin.gz").write_bytes(gzip.compress(blob, mtime=0))
    print("wrote", OUT / "mesh_buffers.json", "and", OUT / "mesh_buffers.bin.gz")


if __name__ == "__main__":
    main()
