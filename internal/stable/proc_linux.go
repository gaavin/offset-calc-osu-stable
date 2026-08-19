//go:build linux

package stable

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func runningOsuDirs() []string {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, e := range ents {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		base := "/proc/" + e.Name()
		comm, _ := os.ReadFile(base + "/comm")
		name := strings.TrimSpace(string(comm))
		cmdline, _ := os.ReadFile(base + "/cmdline")
		cmd := strings.ToLower(strings.ReplaceAll(string(cmdline), "\x00", " "))
		if !strings.EqualFold(name, "osu!.exe") && !strings.Contains(cmd, "osu!.exe") {
			continue
		}
		if cwd, err := os.Readlink(base + "/cwd"); err == nil && hasOsuExe(cwd) && !seen[cwd] {
			seen[cwd] = true
			out = append(out, cwd)
			continue
		}
		for _, field := range strings.Fields(strings.ReplaceAll(string(cmdline), "\x00", " ")) {
			if !strings.Contains(strings.ToLower(field), "osu!.exe") {
				continue
			}
			dir := filepath.Dir(field)
			if hasOsuExe(dir) && !seen[dir] {
				seen[dir] = true
				out = append(out, dir)
			}
		}
	}
	return out
}
