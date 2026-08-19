//go:build darwin

package stable

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func runningOsuDirs() []string {
	out, err := exec.Command("ps", "-axo", "command=").Output()
	if err != nil {
		return nil
	}
	var dirs []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(strings.ToLower(line), "osu!.exe") {
			continue
		}
		for _, field := range strings.Fields(line) {
			if !strings.Contains(strings.ToLower(field), "osu!.exe") {
				continue
			}
			dir := filepath.Dir(strings.Trim(field, `"'`))
			if hasOsuExe(dir) && !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs
}
