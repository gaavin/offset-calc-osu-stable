#!/usr/bin/env bash
set -euo pipefail

binary="${1:?usage: build-flatpak.sh BINARY VERSION OUT_DIR FLATPAK_ARCH}"
version="${2:?}"
out_dir="${3:?}"
arch="${4:?}"

root="$(cd "$(dirname "$0")/.." && pwd)"
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

cp "$root/packaging/io.github.gaavin.osu-offset.yml" "$workdir/"
cp "$root/packaging/io.github.gaavin.osu-offset.desktop" "$workdir/"
cp "$root/packaging/io.github.gaavin.osu-offset.svg" "$workdir/"
cp "$root/packaging/io.github.gaavin.osu-offset.metainfo.xml" "$workdir/"
install -m 0755 "$binary" "$workdir/osu-offset"
chmod +x "$workdir/osu-offset"

flatpak remote-add --if-not-exists --user flathub https://flathub.org/repo/flathub.flatpakrepo
flatpak install --user -y "runtime/org.freedesktop.Platform/${arch}/24.08" \
  "runtime/org.freedesktop.Sdk/${arch}/24.08"

flatpak-builder --user --force-clean --disable-rofiles-fuse \
  --install-deps-from=flathub --arch="$arch" --repo="$workdir/repo" \
  "$workdir/build" "$workdir/io.github.gaavin.osu-offset.yml"

mkdir -p "$out_dir"
out="$out_dir/osu-offset-${arch}.flatpak"
flatpak build-bundle --arch="$arch" "$workdir/repo" "$out" io.github.gaavin.osu-offset
(cd "$out_dir" && sha256sum "$(basename "$out")" > "$(basename "$out").sha256")

# version is stamped on the wrapped binary; keep the arg so callers stay consistent
: "$version"
