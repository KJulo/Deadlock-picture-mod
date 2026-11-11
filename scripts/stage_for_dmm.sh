#!/usr/bin/env bash
# Copy the built VPK into the Deadlock Mod Manager Flatpak's own data dir, which the
# sandbox can always read (it can't see ~/Development or ~/Downloads -> "Mod file not found at path").
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$HOME/.var/app/dev.stormix.deadlock-mod-manager/import"
mkdir -p "$DEST"
cp "$ROOT/build/pak09_dir.vpk" "$DEST/catmod_soulcontainer_pak09_dir.vpk"
echo "Listo: en DMM agrega un mod local y elige $DEST/catmod_soulcontainer_pak09_dir.vpk"
