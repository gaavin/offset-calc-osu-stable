# osu-offset

Recommend a **universal Offset** for osu!stable from your recent plays, using the same idea as osu!lazer:

1. Rebuild hit error from local `.osr` replays (circles + slider heads).
2. Take the **median** error of each play (need at least 50 timed hits, like lazer).
3. Average those into one suggested global offset: `currentOffset - medianError`.
4. Print it. Optionally write it to `osu!.<user>.cfg`.

On Wine (including [nix-osu-stable](https://github.com/gaavin/nix-osu-stable)), audio is usually late, so the suggestion is often a **negative** Offset.

Works on **Windows**, **macOS**, **Linux**, and **NixOS**.

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

GitHub Actions builds `linux`, `darwin`, and `windows` (`amd64` + `arm64`) on each `v*` tag. Download `osu-offset-<os>-<arch>` (`.exe` on Windows).

From source:

```bash
go build -o osu-offset ./cmd/osu-offset
# Windows:
GOOS=windows GOARCH=amd64 go build -o osu-offset.exe ./cmd/osu-offset
```

## Run

```bash
osu-offset
osu-offset -verbose
osu-offset -json
osu-offset -debug-paths    # show every install path that would be tried
osu-offset -apply          # write Offset into the osu! config (close the game first)
osu-offset -dir /path/to/osu
```

## Path detection

First match wins:

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

nix-osu-stable’s README ballpark is about **−40 to −35 ms** in normal mode; this tool replaces guessing with your own replays.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-dir` | auto | osu!stable folder (the one with `osu!.exe`) |
| `-plays` | 50 | max recent plays (lazer’s history cap) |
| `-min-hits` | 50 | min timed hits per play |
| `-apply` | off | write `Offset = …` into the user cfg |
| `-json` | off | machine-readable output |
| `-verbose` | off | print skipped plays |
| `-debug-paths` | off | print path candidates and exit |
| `-ur-dampen` | off | lazer’s *per-beatmap* UR shrink (not used for global offset) |

## Notes

- Only **osu!standard**. Relax / autopilot / auto replays are ignored.
- Replays are read from `Data/r/` and `Replays/`.
- Hit reconstruction is a replay analyser, not the live client. Median over many notes is stable enough for offset; a few stacked or slider-head mismatches will not move the suggestion much.
- `-apply` while osu! is running is often undone when the client exits — close it first.
