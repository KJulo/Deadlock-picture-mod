#!/usr/bin/env bash
set -euo pipefail
ADDONS="${DEADLOCK_ADDONS:-$HOME/.local/share/Steam/steamapps/common/Deadlock/game/citadel/addons}"
DST="$ADDONS/pak09_dir.vpk"
MARKER="$ADDONS/.catmod_pak09"
if [[ ! -f "$MARKER" ]]; then echo "Mod not installed by install.sh (no marker); nothing removed." >&2; exit 1; fi
if [[ -f "$DST" && "$(sha256sum "$DST" | cut -d' ' -f1)" != "$(cat "$MARKER")" ]]; then
  echo "$DST changed since install (not ours anymore?); refusing. Remove manually." >&2; exit 1
fi
rm -f "$DST" "$MARKER"
echo "Uninstalled."
