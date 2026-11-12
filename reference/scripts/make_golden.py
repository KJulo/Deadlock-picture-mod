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
from catmod.vmdl import SPHERE_RADIUS, build_sphere_model  # noqa: E402
import meshoptimizer as mo  # noqa: E402
import numpy as np  # noqa: E402
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


def _enc_vertices(vb: np.ndarray, vertex_size: int) -> bytes:
    mo.encode_vertex_version(0)
    return bytes(mo.encode_vertex_buffer(vb, len(vb), vertex_size))


def _enc_indices(idx: np.ndarray, vertex_count: int) -> bytes:
    return bytes(mo.encode_index_buffer(idx.astype(np.uint32), len(idx), vertex_count))


def meshopt_cases(mesh_blob: bytes, mesh_meta: dict):
    """Inputs and python-meshoptimizer outputs: the sphere LODs plus synthetic edge cases."""
    rng = np.random.default_rng(1234)
    cases = []

    def vcase(name, vb, size):
        cases.append({"name": name, "kind": "vertex", "vertex_size": size,
                      "input": vb.tobytes(), "output": _enc_vertices(vb, size)})

    def icase(name, idx, vcount):
        cases.append({"name": name, "kind": "index", "vertex_count": vcount,
                      "input": idx.astype("<u4").tobytes(), "output": _enc_indices(idx, vcount)})

    for n, lod in enumerate(mesh_meta["lods"]):
        o, vc, ic = lod["offset"], lod["vertex_count"], lod["index_count"]
        vb = np.frombuffer(mesh_blob[o:o + vc * 24], np.uint8).reshape(vc, 24)
        idx = np.frombuffer(mesh_blob[o + vc * 24:o + vc * 24 + ic * 4], "<u4")
        vcase(f"sphere_lod{n}_vertices", vb, 24)
        icase(f"sphere_lod{n}_indices", idx, vc)

    # smooth data (small deltas -> 0/2/4-bit groups) mixed with noise (8-bit groups)
    smooth = np.cumsum(rng.integers(-3, 4, size=(1000, 24)), 0).astype(np.uint8)
    noise = rng.integers(0, 256, size=(1000, 24), dtype=np.uint8)
    mixed = np.where(rng.random((1000, 24)) < 0.15, noise, smooth).astype(np.uint8)
    vcase("vertices_mixed_1000x24", mixed, 24)
    vcase("vertices_noise_77x8", rng.integers(0, 256, size=(77, 8), dtype=np.uint8), 8)
    vcase("vertices_zero_300x4", np.zeros((300, 4), np.uint8), 4)
    vcase("vertices_one_1x24", rng.integers(0, 256, size=(1, 24), dtype=np.uint8), 24)

    # grid mesh with shuffled triangles and random rotations (edge/vertex fifo misses, free indices)
    g = 20
    tris = []
    for j in range(g):
        for i in range(g):
            a = j * (g + 1) + i
            tris += [(a, a + g + 1, a + 1), (a + 1, a + g + 1, a + g + 2)]
    tris = np.array(tris)
    icase("grid_ordered", tris.ravel(), (g + 1) ** 2)
    shuffled = tris[rng.permutation(len(tris))]
    rot = rng.integers(0, 3, len(shuffled))
    shuffled = np.stack([np.roll(t, r) for t, r in zip(shuffled, rot)])
    icase("grid_shuffled", shuffled.ravel(), (g + 1) ** 2)
    rand = rng.integers(0, 100000, size=(300, 3))
    rand = np.concatenate([rand, [[0, 1, 2]], rand[:5], [[0, 1, 2], [3, 4, 5]]])  # resets
    icase("random_large_indices", rand.ravel(), 100000)
    return cases


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
    model, _ = build_sphere_model(templates.load(templates.MODEL), SPHERE_RADIUS, "f16")
    (OUT / "model_golden.json").write_text(json.dumps(
        {"sphere_model_sha256": hashlib.sha256(model).hexdigest()}, indent=2) + "\n")
    print("wrote", OUT / "model_golden.json")
    cases = meshopt_cases(blob, meta)
    index, data = [], bytearray()
    for c in cases:
        entry = {k: v for k, v in c.items() if k not in ("input", "output")}
        entry.update(input=[len(data), len(c["input"])])
        data += c["input"]
        entry.update(output=[len(data), len(c["output"])])
        data += c["output"]
        index.append(entry)
    (OUT / "meshopt_cases.json").write_text(json.dumps(index, indent=1) + "\n")
    (OUT / "meshopt_cases.bin.gz").write_bytes(gzip.compress(bytes(data), mtime=0))
    print("wrote", OUT / "meshopt_cases.json", "and", OUT / "meshopt_cases.bin.gz")


if __name__ == "__main__":
    main()
