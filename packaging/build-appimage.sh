#!/usr/bin/env bash
set -euo pipefail

binary="${1:?usage: build-appimage.sh BINARY VERSION OUT_DIR APPIMAGE_ARCH}"
version="${2:?}"
out_dir="${3:?}"
arch="${4:?}"

root="$(cd "$(dirname "$0")/.." && pwd)"
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

appdir="$workdir/AppDir"
mkdir -p "$appdir/usr/bin" "$appdir/usr/share/icons/hicolor/scalable/apps"

install -m 0755 "$binary" "$appdir/usr/bin/osu-offset"
chmod +x "$appdir/usr/bin/osu-offset"
install -m 0644 "$root/packaging/io.github.gaavin.osu-offset.desktop" "$appdir/osu-offset.desktop"
install -m 0644 "$root/packaging/io.github.gaavin.osu-offset.svg" \
  "$appdir/usr/share/icons/hicolor/scalable/apps/io.github.gaavin.osu-offset.svg"
cp "$appdir/usr/share/icons/hicolor/scalable/apps/io.github.gaavin.osu-offset.svg" \
  "$appdir/io.github.gaavin.osu-offset.svg"

if command -v rsvg-convert >/dev/null 2>&1; then
  rsvg-convert -w 256 -h 256 \
    "$root/packaging/io.github.gaavin.osu-offset.svg" \
    -o "$appdir/io.github.gaavin.osu-offset.png"
fi

cat > "$appdir/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/osu-offset" "$@"
EOF
chmod +x "$appdir/AppRun"

tool="$workdir/appimagetool.AppImage"
curl -fsSL -o "$tool" \
  "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-${arch}.AppImage"
chmod +x "$tool"
(cd "$workdir" && ./appimagetool.AppImage --appimage-extract)

mkdir -p "$out_dir"
out="$out_dir/osu-offset-${arch}.AppImage"
ARCH="$arch" VERSION="$version" "$workdir/squashfs-root/AppRun" "$appdir" "$out"
(cd "$out_dir" && sha256sum "$(basename "$out")" > "$(basename "$out").sha256")
