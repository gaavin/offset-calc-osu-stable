# osu-offset

Recommend a **universal Offset** for osu!stable from **live hit error**, not old replays.

1. Attach to a running `osu!.exe` (Windows, or Wine on Linux / NixOS).
2. Read the current play’s hit-error list from process memory (the same values as the in-game error bar).
3. Take the **median** error (need at least 50 timed hits).
4. Print the exact Offset to set: `currentOffset - medianError`.

That uses the Offset, audio device, and Wine latency you have **right now**. Old `.osr` files can have been played with a different Offset.

On Wine (including [nix-osu-stable](https://github.com/gaavin/nix-osu-stable)), audio is usually late, so the suggestion is often a **negative** Offset.

Memory reading works on **Windows** and **Linux/NixOS (Wine)**. macOS binaries can still be built; they cannot read osu! memory.

## Install

### Nix (NixOS / Home Manager)

```bash
nix run . --
```

Put it on your PATH next to nix-osu-stable:

```nix
# flake.nix inputs
offset-calc-osu-stable.url = "path:/path/to/offset-calc-osu-stable";

# home.nix
home.packages = [
  offset-calc-osu-stable.packages.${pkgs.stdenv.hostPlatform.system}.osu-offset
];
```

Cross-compile from source (`CGO_ENABLED=0`, no extra deps):

```bash
GOOS=windows GOARCH=amd64 go build -o osu-offset.exe ./cmd/osu-offset
GOOS=darwin  GOARCH=arm64 go build -o osu-offset ./cmd/osu-offset
```

### Release binaries

Every push to `master` publishes a GitHub Release tagged `vYYYY-MM-DD-HHMMSS` (UTC commit time). Tags and assets are never overwritten.

Download from the [latest release](https://github.com/gaavin/offset-calc-osu-stable/releases):

| Platform | Artifact |
| --- | --- |
| Windows | `osu-offset-windows-amd64.exe`, `osu-offset-windows-arm64.exe` |
| macOS | `osu-offset-darwin-amd64`, `osu-offset-darwin-arm64` |
| Linux | `osu-offset-x86_64.AppImage`, `osu-offset-aarch64.AppImage` |
| Linux (Flatpak) | `osu-offset-x86_64.flatpak`, `osu-offset-aarch64.flatpak` |

```bash
# AppImage
chmod +x osu-offset-x86_64.AppImage
./osu-offset-x86_64.AppImage

# Flatpak (sideload; not on Flathub)
flatpak install --user ./osu-offset-x86_64.flatpak
flatpak run io.github.gaavin.osu-offset
```

Plain `osu-offset-<os>-<arch>` binaries (`amd64` + `arm64`) are attached as well. Nix stays `nix run .` / Home Manager above.

From source:

```bash
go build -o osu-offset ./cmd/osu-offset
# Windows:
GOOS=windows GOARCH=amd64 go build -o osu-offset.exe ./cmd/osu-offset
```

## Run

Start osu!stable first, then:

```bash
osu-offset
osu-offset -watch          # print after every play
osu-offset -json
osu-offset -debug-paths    # show install-path candidates and exit
osu-offset -apply          # write Offset into the osu! config (close the game first)
osu-offset -dir /path/to/osu
```

Play a map. When the play ends, it prints the Offset to set in **Options → Audio → Offset**.

On Linux, the reader needs to inspect osu!’s Wine process (`process_vm_readv`). If attach fails, run as the same user as the game; you may need `kernel.yama.ptrace_scope=0` (or a cap that allows ptrace).

## Path detection

Used for the **current Offset** in `osu!.<user>.cfg` and for `-apply`. First match wins:

1. `-dir` (folder with `osu!.exe`, the exe itself, or a nix-osu-stable **location** dir)
2. `OSU_STABLE_DIR` / `OSU_DIR` / `OSUPATH`
3. A running `osu!.exe` process
4. Windows file associations (`osu!` / `osu` in the registry)
5. `osu-wine --info` — [nix-osu-stable](https://github.com/gaavin/nix-osu-stable) and osu-winello (including a custom `programs.osu-stable.location`)
6. osu-winello `~/.local/share/osuconfig/osupath`
7. Platform defaults:

| Platform | Defaults |
| --- | --- |
| Windows | `%LOCALAPPDATA%\osu!` (wiki default), `%USERPROFILE%\osu!`, Program Files |
| macOS | `/Applications/osu!.app/Contents/Resources/drive_c/osu!`, CrossOver / Whisky / PlayOnMac bottles |
| Linux | `~/.local/share/osu-wine` (osu-winello), `~/.wine/…/osu!`, Bottles, `~/Games/osu` |
| NixOS | `~/.local/share/nix-osu-stable/osu` (and `$XDG_DATA_HOME/nix-osu-stable/osu`) |

If you set a custom `programs.osu-stable.location`, keep `osu-wine` on PATH (Home Manager already does) or pass `-dir`.

`osu-offset -debug-paths` prints the full probe list.

## What to do with the number

In-game: **Options → Audio → Offset**.

- Hitting **early** (error bar left) → **raise** Offset.
- Hitting **late** (error bar right) → **lower** Offset.

nix-osu-stable’s README ballpark is about **−40 to −35 ms** in normal mode; this tool replaces guessing with your current session.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-dir` | auto | osu!stable folder (config / `-apply`) |
| `-min-hits` | 50 | min timed hits before recommending |
| `-watch` | off | keep going after each play |
| `-poll` | 50ms | memory sample interval |
| `-apply` | off | write `Offset = …` into the user cfg |
| `-json` | off | machine-readable output |
| `-debug-paths` | off | print path candidates and exit |

## Notes

- Only **osu!standard** hit errors are used.
- Replay watching is ignored (those hits are not yours with the current Offset).
- Signatures match current osu!stable (same family as [tosu](https://github.com/tosuapp/tosu) / gosumemory). A game update can move them.
- `-apply` while osu! is running is often undone when the client exits — close it first.
