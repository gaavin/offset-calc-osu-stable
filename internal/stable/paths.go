package stable

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func detectGOOS() string { return runtime.GOOS }

func detectHome() string {
	home, _ := os.UserHomeDir()
	return home
}

func xdgDataHome(home string) string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share")
}

// Candidates is the auto-detect search list, highest priority first.
func Candidates() []string {
	var out []string
	for _, c := range debugCandidates() {
		out = append(out, c.Path)
	}
	return out
}

func debugCandidates() []Candidate {
	home := detectHome()
	xdg := xdgDataHome(home)
	var out []Candidate
	seen := map[string]bool{}
	add := func(source, p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		key := filepath.Clean(p)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Candidate{Path: p, Source: source})
	}

	for _, p := range runningOsuDirs() {
		add("running osu!.exe", p)
	}
	for _, p := range registryOsuDirs() {
		add("windows file association", p)
	}
	for _, p := range osuWineInfoPaths() {
		add("osu-wine --info (nix-osu-stable / osu-winello)", p)
	}
	for _, p := range winelloOsuPaths(xdg) {
		add("osu-winello osupath", p)
	}

	switch runtime.GOOS {
	case "windows":
		for _, p := range windowsStaticCandidates(
			os.Getenv("LOCALAPPDATA"),
			os.Getenv("USERPROFILE"),
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			home,
		) {
			add("windows default", p)
		}
	case "darwin":
		for _, p := range darwinStaticCandidates(home) {
			add("macOS default", p)
		}
		for _, p := range globDirs(darwinGlobPatterns(home)) {
			add("macOS Wine/CrossOver/Whisky", p)
		}
	default:
		for _, p := range unixStaticCandidates(home, xdg) {
			add("linux / NixOS default", p)
		}
		for _, p := range globDirs(unixGlobPatterns(home, xdg)) {
			add("linux Wine/Lutris/Bottles", p)
		}
	}

	if v := os.Getenv("WINEPREFIX"); v != "" {
		for _, p := range winePrefixOsuDirs(v) {
			add("WINEPREFIX", p)
		}
	}

	return out
}

func unixStaticCandidates(home, xdg string) []string {
	var out []string
	add := func(p string) {
		if p != "" {
			out = append(out, p)
		}
	}
	if xdg != "" {
		add(filepath.Join(xdg, "nix-osu-stable", "osu"))
		add(filepath.Join(xdg, "nix-osu-stable"))
		add(filepath.Join(xdg, "osu-wine"))
		add(filepath.Join(xdg, "osu-winello"))
		add(filepath.Join(xdg, "osu!"))
		add(filepath.Join(xdg, "osu"))
	}
	if home != "" {
		homeXdg := filepath.Join(home, ".local", "share")
		if xdg == "" || homeXdg != xdg {
			add(filepath.Join(home, ".local", "share", "nix-osu-stable", "osu"))
			add(filepath.Join(home, ".local", "share", "nix-osu-stable"))
			add(filepath.Join(home, ".local", "share", "osu-wine"))
		}
		add(filepath.Join(home, "osu!"))
		add(filepath.Join(home, "osu"))
		add(filepath.Join(home, ".osu"))
		add(filepath.Join(home, "Games", "osu!"))
		add(filepath.Join(home, "Games", "osu"))
		add(filepath.Join(home, "games", "osu"))
	}
	return out
}

func windowsStaticCandidates(localAppData, userProfile, programFiles, programFilesX86, home string) []string {
	var out []string
	add := func(p string) {
		if p != "" {
			out = append(out, p)
		}
	}
	if localAppData != "" {
		add(filepath.Join(localAppData, "osu!"))
	}
	if userProfile != "" {
		add(filepath.Join(userProfile, "AppData", "Local", "osu!"))
		add(filepath.Join(userProfile, "osu!"))
		add(filepath.Join(userProfile, "Games", "osu!"))
	}
	if programFiles != "" {
		add(filepath.Join(programFiles, "osu!"))
	}
	if programFilesX86 != "" {
		add(filepath.Join(programFilesX86, "osu!"))
	}
	if home != "" {
		add(filepath.Join(home, "osu!"))
		add(filepath.Join(home, "Games", "osu!"))
	}
	return out
}

func darwinStaticCandidates(home string) []string {
	var out []string
	add := func(p string) {
		if p != "" {
			out = append(out, p)
		}
	}
	add("/Applications/osu!.app/Contents/Resources/drive_c/osu!")
	add("/Applications/osu!.app/Contents/SharedSupport/prefix/drive_c/osu!")
	if home != "" {
		add(filepath.Join(home, "Applications", "osu!.app", "Contents", "Resources", "drive_c", "osu!"))
		add(filepath.Join(home, "Library", "Application Support", "osu!"))
		add(filepath.Join(home, "osu!"))
		add(filepath.Join(home, ".local", "share", "nix-osu-stable", "osu"))
		add(filepath.Join(home, ".local", "share", "osu-wine"))
	}
	return out
}

func darwinGlobPatterns(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, "Library", "Application Support", "CrossOver", "Bottles", "*", "drive_c", "osu!"),
		filepath.Join(home, "Library", "Application Support", "CrossOver", "Bottles", "*", "drive_c", "users", "*", "AppData", "Local", "osu!"),
		filepath.Join(home, "Library", "Application Support", "Whisky", "Bottles", "*", "drive_c", "osu!"),
		filepath.Join(home, "Library", "Application Support", "Whisky", "Bottles", "*", "drive_c", "users", "*", "AppData", "Local", "osu!"),
		filepath.Join(home, "Library", "Application Support", "com.isaacmarovitz.Whisky", "Bottles", "*", "drive_c", "osu!"),
		filepath.Join(home, "Library", "Containers", "*", "Data", "Bottles", "*", "drive_c", "users", "*", "AppData", "Local", "osu!"),
		filepath.Join(home, "Library", "Application Support", "PlayOnMac", "Bottles", "*", "drive_c", "osu!"),
		filepath.Join("/Applications", "osu!.app", "Contents", "*", "drive_c", "osu!"),
	}
}

func unixGlobPatterns(home, xdg string) []string {
	var pats []string
	add := func(p string) {
		if p != "" {
			pats = append(pats, p)
		}
	}
	if xdg != "" {
		add(filepath.Join(xdg, "wineprefixes", "*", "drive_c", "osu!"))
		add(filepath.Join(xdg, "wineprefixes", "*", "drive_c", "users", "*", "AppData", "Local", "osu!"))
		add(filepath.Join(xdg, "nix-osu-stable", "wineprefix", "drive_c", "users", "*", "AppData", "Local", "osu!"))
		add(filepath.Join(xdg, "bottles", "bottles", "*", "drive_c", "osu!"))
		add(filepath.Join(xdg, "bottles", "bottles", "*", "drive_c", "users", "*", "AppData", "Local", "osu!"))
	}
	if home != "" {
		add(filepath.Join(home, ".wine", "drive_c", "osu!"))
		add(filepath.Join(home, ".wine", "drive_c", "users", "*", "AppData", "Local", "osu!"))
		add(filepath.Join(home, "Games", "*", "osu!.exe"))
		add(filepath.Join(home, "Games", "*", "osu", "osu!.exe"))
		add(filepath.Join(home, "Games", "osu!", "osu!.exe"))
	}
	return pats
}

func winePrefixOsuDirs(prefix string) []string {
	return globDirs([]string{
		filepath.Join(prefix, "drive_c", "osu!"),
		filepath.Join(prefix, "drive_c", "users", "*", "AppData", "Local", "osu!"),
	})
}

func globDirs(patterns []string) []string {
	var out []string
	for _, pat := range patterns {
		matches, err := filepath.Glob(pat)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if strings.EqualFold(filepath.Base(m), "osu!.exe") {
				out = append(out, filepath.Dir(m))
				continue
			}
			out = append(out, m)
		}
	}
	return out
}

func osuWineInfoPaths() []string {
	bin, err := exec.LookPath("osu-wine")
	if err != nil {
		return nil
	}
	out, err := exec.Command(bin, "--info").Output()
	if err != nil {
		return nil
	}
	var paths []string
	var state string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "osu! path:"):
			paths = append(paths, strings.TrimSpace(line[len("osu! path:"):]))
		case strings.HasPrefix(lower, "osu path:"):
			paths = append(paths, strings.TrimSpace(line[len("osu path:"):]))
		case strings.HasPrefix(lower, "state:"):
			state = strings.TrimSpace(line[len("state:"):])
		}
	}
	if state != "" {
		paths = append(paths, filepath.Join(state, "osu"), state)
	}
	return paths
}

func winelloOsuPaths(xdg string) []string {
	if xdg == "" {
		return nil
	}
	file := filepath.Join(xdg, "osuconfig", "osupath")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	p := strings.TrimSpace(string(data))
	if p == "" {
		return nil
	}
	return []string{p}
}
