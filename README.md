<div align="center">

# osu-offset

**Recommend a universal Offset for osu!stable** — from **live hit error**.

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/gaavin/offset-calc-osu-stable?label=release)](https://github.com/gaavin/offset-calc-osu-stable/releases)
[![Nix](https://img.shields.io/badge/Nix-flake-success?logo=NixOS)](https://nixos.org)

Works with [nix-osu-stable](https://github.com/gaavin/nix-osu-stable) on NixOS, native Windows, and Wine on Linux.

</div>

## ⚡ Quick Start

**Already have a release binary or Nix?**

```bash
osu-offset          # watch osu!.exe; print after every usable play
```

1. Launch osu!stable (or start `osu-offset` first — it waits for `osu!.exe`).
2. Play a map with at least **50 timed hits** (standard mode).
3. When the play ends, copy the printed Offset into **Options → Audio → Offset**.

> **What it reads:** live hit-error values and your current universal Offset from process memory — not `.osr` replays or config files on disk.

---

## 🎯 How It Works

<div>
<strong>Each poll while you play:</strong><br/>
• Attach to running <code>osu!.exe</code> (Windows, or Wine on Linux / NixOS)<br/>
• Read the current universal Offset from memory (updates when you change it in-game)<br/>
• Read the current play's hit-error list from memory<br/>
• Take the <strong>median</strong> error (≥ 50 timed hits)<br/>
• Print: <code>currentOffset − medianError</code><br/>
• Keep watching — re-attach if osu! restarts
</div>

That reflects the Offset and audio setup you have **right now**.

On Wine (including nix-osu-stable), audio is usually late, so the suggestion is often a **negative** Offset.

| Platform | Memory reading |
| --- | --- |
| Windows | ✅ |
| Linux / NixOS (Wine) | ✅ |
| macOS | ❌ (binaries build; no memory attach) |

---

## 📦 Install

### Nix (standalone)

```bash
nix run github:gaavin/offset-calc-osu-stable
```

Or add to Home Manager:

```nix
# flake inputs
offset-calc-osu-stable.url = "github:gaavin/offset-calc-osu-stable";

# home.nix
home.packages = [
  offset-calc-osu-stable.packages.${pkgs.stdenv.hostPlatform.system}.osu-offset
];
```

### With nix-osu-stable (recommended on NixOS)

No extra flake input — enable it in the same Home Manager module:

```nix
programs.osu-stable = {
  enable = true;
  offsetCalculator.enable = true;
};
```

Then run `osu-offset` alongside `osu-wine`. See the [nix-osu-stable README](https://github.com/gaavin/nix-osu-stable#-essential-set-audio-offset) for the usual **−40 to −35 ms** ballpark; this tool replaces guessing with your current session.

### Release binaries

Every push to `master` publishes a GitHub Release tagged `vYYYY-MM-DD-HHMMSS` (UTC commit time). Tags and assets are never overwritten.

Download from the [latest release](https://github.com/gaavin/offset-calc-osu-stable/releases):

| Platform | Artifact |
| --- | --- |
| Windows | `osu-offset-windows-amd64.exe`, `osu-offset-windows-arm64.exe` |
| macOS | `osu-offset-darwin-amd64`, `osu-offset-darwin-arm64` |
| Linux | `osu-offset-x86_64.AppImage`, `osu-offset-aarch64.AppImage` |
| Linux (Flatpak) | `osu-offset-x86_64.flatpak`, `osu-offset-aarch64.flatpak` |

Plain `osu-offset-<os>-<arch>` binaries (`amd64` + `arm64`) are attached as well.

```bash
# AppImage
chmod +x osu-offset-x86_64.AppImage
./osu-offset-x86_64.AppImage

# Flatpak (sideload; not on Flathub)
flatpak install --user ./osu-offset-x86_64.flatpak
flatpak run io.github.gaavin.osu-offset
```

### From source

```bash
go build -o osu-offset ./cmd/osu-offset

# Cross-compile (CGO_ENABLED=0)
GOOS=windows GOARCH=amd64 go build -o osu-offset.exe ./cmd/osu-offset
GOOS=darwin  GOARCH=arm64 go build -o osu-offset ./cmd/osu-offset
```

---

## 🎵 Set Your Offset

In-game: **Options → Audio → Offset**.

<div>
• Hitting <strong>early</strong> (error bar left) → <strong>raise</strong> Offset<br/>
• Hitting <strong>late</strong> (error bar right) → <strong>lower</strong> Offset
</div>

Change the Offset in-game while `osu-offset` is running — the live read updates immediately.

---

## 📋 Commands

| Command | Purpose |
| --- | --- |
| <span>osu-offset</span> | Watch processes; print after every usable play |
| <span>osu-offset -once</span> | Exit after the first recommendation |
| <span>osu-offset -json</span> | Machine-readable output |

### Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-min-hits` | 50 | Min timed hits before recommending |
| `-watch` | on | Keep monitoring after each play |
| `-once` | off | Exit after the first usable play |
| `-poll` | 50ms | Memory sample interval |
| `-json` | off | Machine-readable output |

---

## 🔧 Troubleshooting

| Issue | Solution |
| --- | --- |
| Hits feel late on Wine | Run `osu-offset` while you play; set the printed Offset (often negative) |
| "Waiting for osu!.exe" | Start the game, or launch `osu-offset` first — it attaches when osu! appears |
| Not enough hits | Play a longer standard map (need ≥ 50 timed hits) |
| Attach fails on Linux | Run as the same user as the game; try `kernel.yama.ptrace_scope=0` |
| Recommendation stops after game update | Signatures may have moved; file an issue with your osu! version |

---

## 📝 Notes

- Only **osu!standard** hit errors are used.
- Replay *watching* in the client is ignored (those hits are not yours with the current Offset). Old `.osr` files are never scanned.
- Signatures match current osu!stable (same family as [tosu](https://github.com/tosuapp/tosu) / gosumemory). A game update can move them.

---

## 🙏 Related

- [nix-osu-stable](https://github.com/gaavin/nix-osu-stable) — osu! on NixOS with Wine + Steam Runtime
- [osu-winello](https://github.com/NelloKudo/osu-winello) — upstream Wine stack inspiration
- [tosu](https://github.com/tosuapp/tosu) — memory signature reference
