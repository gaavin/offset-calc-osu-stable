//go:build windows

package stable

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func registryOsuDirs() []string {
	keys := []struct {
		root registry.Key
		path string
	}{
		{registry.CLASSES_ROOT, `osu!\shell\open\command`},
		{registry.CLASSES_ROOT, `osu\shell\open\command`},
		{registry.CURRENT_USER, `Software\Classes\osu!\shell\open\command`},
		{registry.CURRENT_USER, `Software\Classes\osu\shell\open\command`},
	}
	var out []string
	seen := map[string]bool{}
	for _, k := range keys {
		key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		val, _, err := key.GetStringValue("")
		key.Close()
		if err != nil || val == "" {
			continue
		}
		exe := extractQuotedExe(val)
		if exe == "" {
			continue
		}
		dir := filepath.Dir(exe)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	return out
}

func extractQuotedExe(command string) string {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, `"`) {
		if i := strings.Index(command[1:], `"`); i >= 0 {
			return command[1 : 1+i]
		}
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
