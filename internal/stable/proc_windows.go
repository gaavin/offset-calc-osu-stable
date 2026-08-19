//go:build windows

package stable

import (
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func runningOsuDirs() []string {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return nil
	}

	var out []string
	seen := map[string]bool{}
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(name, "osu!.exe") {
			if dir := processDir(entry.ProcessID); dir != "" && !seen[dir] {
				seen[dir] = true
				out = append(out, dir)
			}
		}
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return out
}

func processDir(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	var n uint32 = windows.MAX_PATH
	buf := make([]uint16, n)
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return ""
	}
	exe := windows.UTF16ToString(buf)
	dir := filepath.Dir(exe)
	if hasOsuExe(dir) {
		return dir
	}
	return ""
}
