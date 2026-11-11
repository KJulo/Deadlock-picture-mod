#!/usr/bin/env python3
"""Extract the original soul_container color texture from the local Deadlock install
into vendor/original/ (Valve asset: not committed to the repo)."""
import os
import sys
from pathlib import Path

import vpk

ROOT = Path(__file__).resolve().parent.parent
GAME_PATH = "models/props_gameplay/soul_container/materials/soul_container_color_png_1da67bfe.vtex_c"
CITADEL = Path(os.environ.get(
    "DEADLOCK_CITADEL",
    Path.home() / ".local/share/Steam/steamapps/common/Deadlock/game/citadel"))
OUT = ROOT / "vendor/original" / Path(GAME_PATH).name


def main():
    pak = vpk.open(str(CITADEL / "pak01_dir.vpk"))
    try:
        entry = pak[GAME_PATH]
    except KeyError:
        sys.exit(f"{GAME_PATH} not found in pak01 — the game may have renamed it after an update.")
    OUT.parent.mkdir(parents=True, exist_ok=True)
    entry.save(str(OUT))
    print(f"wrote {OUT} ({OUT.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
