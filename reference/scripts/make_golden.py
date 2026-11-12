#!/usr/bin/env python3
"""Write golden files that the Go backend tests compare against (testdata/ at the repo root).

Only *metadata* derived from the game files is written (block kinds, sizes, alignments, hashes),
never Valve content, so testdata/ can be committed to the public repo.

  .venv/bin/python reference/scripts/make_golden.py
"""
import hashlib
import json
import sys
from pathlib import Path

REF = Path(__file__).resolve().parent.parent
ROOT = REF.parent
sys.path.insert(0, str(REF))

from catmod import templates  # noqa: E402
from catmod.resource import Resource  # noqa: E402

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


def main():
    OUT.mkdir(exist_ok=True)
    path = OUT / "resource_layout.json"
    path.write_text(json.dumps(resource_layout(), indent=2) + "\n")
    print("wrote", path)


if __name__ == "__main__":
    main()
