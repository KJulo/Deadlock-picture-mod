#!/usr/bin/env bash
# Copy build/pak09_dir.vpk into Deadlock's addons folder.
# Deadlock Mod Manager uses pak01-pak08 there; we use pak09.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ADDONS="${DEADLOCK_ADDONS:-$HOME/.local/share/Steam/steamapps/common/Deadlock/game/citadel/addons}"
SRC="$ROOT/build/pak09_dir.vpk"
DST="$ADDONS/pak09_dir.vpk"
MARKER="$ADDONS/.catmod_pak09"
FORCE=0; [[ "${1:-}" == "--force" ]] && FORCE=1

[[ -f "$SRC" ]] || { echo "Missing $SRC — run scripts/build.py first" >&2; exit 1; }
[[ -d "$ADDONS" ]] || { echo "Addons dir not found: $ADDONS" >&2; exit 1; }
if [[ -f "$DST" && ! -f "$MARKER" && $FORCE -eq 0 ]]; then
  echo "$DST exists and was not installed by this mod (Deadlock Mod Manager?). Use --force to overwrite." >&2
  exit 1
fi
cp "$SRC" "$DST"
sha256sum "$DST" | cut -d' ' -f1 > "$MARKER"
echo "Installed $DST"
